package omnichannel

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Storage de midia em DISCO, raiz PRIVADA (spec F6 C3, D2). Diverge do calendar de
// PROPOSITO: o calendar publica em /uploads/ (http.FileServer sem auth); aqui a raiz fica
// FORA de UPLOADS_DIR e a midia so e servida pelo endpoint autenticado /media (armadilha 1).
// A mecanica (sanitizar segmento, nome aleatorio, allowlist de mime, perms restritas) e
// espelhada de calendar/media_storage.go — o DESTINO e o oposto.

var (
	// ErrMediaUnsupported: mime fora do allowlist ou incompativel com o type. 415.
	ErrMediaUnsupported = errors.New("omnichannel: media type not allowed")
	// ErrMediaTooLarge: passou do teto decodificado da conta. 413.
	ErrMediaTooLarge = errors.New("omnichannel: media too large")
	// ErrMediaInvalid: data URL malformado / corpo vazio / path fora da raiz. 400/404.
	ErrMediaInvalid = errors.New("omnichannel: invalid media")
)

const (
	// defaultMediaDir e a raiz PRIVADA default (fora de data/uploads — nunca sob o FileServer).
	defaultMediaDir = "data/media/omnichannel"
	// defaultMaxMediaBytes e o teto decodificado quando a conta nao configurou um.
	defaultMaxMediaBytes = 60 << 20
)

// mediaCategory casa o type da mensagem (TEXT|IMAGE|AUDIO|VIDEO|DOCUMENT) com o mime.
type mediaCategory string

const (
	catImage    mediaCategory = "IMAGE"
	catAudio    mediaCategory = "AUDIO"
	catVideo    mediaCategory = "VIDEO"
	catDocument mediaCategory = "DOCUMENT"
)

// mediaSpec e a entrada do allowlist: categoria + extensao canonica do mime.
type mediaSpec struct {
	cat mediaCategory
	ext string
}

// allowedMedia e o allowlist de mimes servidos/aceitos. Fora dele => 415. Servir so mimes
// conhecidos com Content-Type explicito + nosniff evita XSS por content-type adivinhado.
var allowedMedia = map[string]mediaSpec{
	"image/jpeg":         {catImage, ".jpg"},
	"image/png":          {catImage, ".png"},
	"image/webp":         {catImage, ".webp"},
	"image/gif":          {catImage, ".gif"},
	"audio/ogg":          {catAudio, ".ogg"},
	"audio/mpeg":         {catAudio, ".mp3"},
	"audio/mp4":          {catAudio, ".m4a"},
	"audio/aac":          {catAudio, ".aac"},
	"audio/wav":          {catAudio, ".wav"},
	"audio/webm":         {catAudio, ".weba"},
	"video/mp4":          {catVideo, ".mp4"},
	"video/webm":         {catVideo, ".webm"},
	"video/quicktime":    {catVideo, ".mov"},
	"video/3gpp":         {catVideo, ".3gp"},
	"application/pdf":    {catDocument, ".pdf"},
	"application/msword": {catDocument, ".doc"},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {catDocument, ".docx"},
	"application/vnd.ms-excel": {catDocument, ".xls"},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": {catDocument, ".xlsx"},
	"text/plain": {catDocument, ".txt"},
	"text/csv":   {catDocument, ".csv"},
}

// StoredMedia e o resultado do Save: o path RELATIVO (storage key, nunca serializado) + o
// mime/nome/tamanho DECODIFICADOS a gravar em messaging.messages.
type StoredMedia struct {
	StorageKey string
	MimeType   string
	FileName   string
	SizeBytes  int64
	SHA256     string
}

// DiskMediaStorage grava sob rootDir/{accountId}/{conversationId}/{random}.{ext}.
type DiskMediaStorage struct {
	rootDir string
}

// NewDiskMediaStorage cria o storage. rootDir vazio => defaultMediaDir (raiz privada).
func NewDiskMediaStorage(rootDir string) *DiskMediaStorage {
	root := strings.TrimSpace(rootDir)
	if root == "" {
		root = defaultMediaDir
	}
	return &DiskMediaStorage{rootDir: root}
}

// MediaDirFromEnv resolve OMNICHANNEL_MEDIA_DIR (default = raiz privada). Fica no modulo
// porque o app.go/config.go sao de outra fase (wiring listado no AGENT.md); precedente:
// module.go ja le os.Getenv de EVOLUTION_*.
func MediaDirFromEnv() string {
	if v := strings.TrimSpace(os.Getenv("OMNICHANNEL_MEDIA_DIR")); v != "" {
		return v
	}
	return defaultMediaDir
}

// Save grava a midia em disco e devolve o descriptor. mediaURL e um data URL base64 (o
// caminho do front) ou uma URL http(s) (passa pelo anti-SSRF antes do fetch). A decodificacao
// e STREAMING (io.Copy do base64.Decoder para o arquivo) — o arquivo NUNCA vira []byte inteiro
// (spec C3). maxBytes = teto decodificado da conta.
func (s *DiskMediaStorage) Save(ctx context.Context, accountID, conversationID, msgType, mimeHint, fileName, mediaURL string, maxBytes int64) (StoredMedia, error) {
	account := sanitizeSegment(accountID)
	conversation := sanitizeSegment(conversationID)
	if account == "" || conversation == "" {
		return StoredMedia{}, ErrMediaInvalid
	}

	src, mime, err := s.openSource(ctx, mediaURL, mimeHint)
	if err != nil {
		return StoredMedia{}, err
	}
	defer func() { _ = src.Close() }()

	return s.writeMedia(account, conversation, categoryForType(msgType), mime, fileName, src, maxBytes, "")
}

// SaveReader grava bytes JA decodificados (usado na rehidratacao one-shot da midia inbound: o
// provider.DownloadMedia devolve um io.Reader). Sem checagem de categoria — so o allowlist do mime.
func (s *DiskMediaStorage) SaveReader(accountID, conversationID, mimeType, fileName string, src io.Reader, maxBytes int64) (StoredMedia, error) {
	account := sanitizeSegment(accountID)
	conversation := sanitizeSegment(conversationID)
	if account == "" || conversation == "" {
		return StoredMedia{}, ErrMediaInvalid
	}
	return s.writeMedia(account, conversation, "", normalizeMime(mimeType), fileName, src, maxBytes, "")
}

// SaveInboundReader grava uma midia recebida sob uma chave deterministica por mensagem.
// Reexecutar o mesmo job substitui o mesmo arquivo e nao acumula copias orfas.
func (s *DiskMediaStorage) SaveInboundReader(accountID, conversationID, messageID, mimeType, fileName string, src io.Reader, maxBytes int64) (StoredMedia, error) {
	account := sanitizeSegment(accountID)
	conversation := sanitizeSegment(conversationID)
	message := sanitizeSegment(messageID)
	if account == "" || conversation == "" || message == "" {
		return StoredMedia{}, ErrMediaInvalid
	}
	return s.writeMedia(account, conversation, "", normalizeMime(mimeType), fileName, src, maxBytes, message)
}

// writeMedia valida o mime (e a categoria, quando informada), grava streaming com teto e
// devolve o descriptor. account/conversation ja sanitizados; nome de arquivo aleatorio.
func (s *DiskMediaStorage) writeMedia(account, conversation string, wantCat mediaCategory, mime, fileName string, src io.Reader, maxBytes int64, stableName string) (StoredMedia, error) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxMediaBytes
	}
	spec, ok := allowedMedia[mime]
	if !ok || (wantCat != "" && spec.cat != wantCat) {
		return StoredMedia{}, ErrMediaUnsupported
	}

	dir := filepath.Join(s.rootDir, account, conversation)
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // segmentos sanitizados, sem traversal
		return StoredMedia{}, err
	}
	name := randomSuffix() + spec.ext
	if stableName != "" {
		name = stableName + spec.ext
	}
	fullPath := filepath.Join(dir, name)
	file, err := os.CreateTemp(dir, ".media-*") //nolint:gosec // dir = raiz + segmentos sanitizados
	if err != nil {
		return StoredMedia{}, err
	}
	tempPath := file.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath) //nolint:gosec // path devolvido por os.CreateTemp no diretorio privado validado.
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return StoredMedia{}, err
	}

	digest := sha256.New()
	size, copyErr := copyCapped(io.MultiWriter(file, digest), src, maxBytes)
	if copyErr == nil && size > 0 {
		copyErr = file.Sync()
	}
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || size == 0 {
		switch {
		case errors.Is(copyErr, ErrMediaTooLarge):
			return StoredMedia{}, ErrMediaTooLarge
		case copyErr != nil:
			return StoredMedia{}, copyErr
		case closeErr != nil:
			return StoredMedia{}, closeErr
		default:
			return StoredMedia{}, ErrMediaInvalid
		}
	}

	// Unix substitui o destino atomicamente. No Windows, os.Rename pode recusar um
	// destino existente; removemos apenas o arquivo deterministico ja validado dentro
	// da raiz e repetimos a publicacao do temporario.
	if err := os.Rename(tempPath, fullPath); err != nil { //nolint:gosec // ambos os paths estao no mesmo diretorio validado
		if _, statErr := os.Stat(fullPath); statErr == nil { //nolint:gosec // path composto apenas por segmentos sanitizados.
			if removeErr := os.Remove(fullPath); removeErr == nil { //nolint:gosec // path contido na raiz privada
				err = os.Rename(tempPath, fullPath) //nolint:gosec // mesmo diretorio validado
			}
		}
		if err != nil {
			return StoredMedia{}, err
		}
	}
	removeTemp = false

	return StoredMedia{
		StorageKey: account + "/" + conversation + "/" + name,
		MimeType:   mime,
		FileName:   mediaDisplayName(fileName, name),
		SizeBytes:  size,
		SHA256:     hex.EncodeToString(digest.Sum(nil)),
	}, nil
}

// openSource devolve um leitor JA decodificado (bytes crus da midia) + o mime resolvido.
// data URL => decoder base64 streaming; http(s) => fetch pelo cliente anti-SSRF.
func (s *DiskMediaStorage) openSource(ctx context.Context, mediaURL, mimeHint string) (io.ReadCloser, string, error) {
	trimmed := strings.TrimSpace(mediaURL)
	switch {
	case strings.HasPrefix(trimmed, "data:"):
		mime, payload, ok := parseDataURL(trimmed)
		if !ok {
			return nil, "", ErrMediaInvalid
		}
		if mime == "" {
			mime = mimeHint
		}
		dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(payload))
		return io.NopCloser(dec), normalizeMime(mime), nil
	case strings.HasPrefix(trimmed, "http://"), strings.HasPrefix(trimmed, "https://"):
		if err := validatePublicURL(ctx, trimmed); err != nil {
			return nil, "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, trimmed, nil)
		if err != nil {
			return nil, "", err
		}
		resp, err := ssrfSafeClient(30 * time.Second).Do(req)
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			_ = resp.Body.Close()
			return nil, "", ErrMediaInvalid
		}
		mime := normalizeMime(resp.Header.Get("Content-Type"))
		if mime == "" {
			mime = normalizeMime(mimeHint)
		}
		return resp.Body, mime, nil
	default:
		return nil, "", ErrMediaInvalid
	}
}

// Open abre a midia pelo storage key, com guarda de containment: o path resolvido tem de
// ficar SOB rootDir (defesa extra, alem dos segmentos sanitizados na gravacao).
func (s *DiskMediaStorage) Open(storageKey string) (*os.File, os.FileInfo, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(storageKey)))
	if clean == "" || clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return nil, nil, ErrMediaInvalid
	}
	rootAbs, err := filepath.Abs(s.rootDir)
	if err != nil {
		return nil, nil, err
	}
	fullAbs, err := filepath.Abs(filepath.Join(s.rootDir, clean))
	if err != nil {
		return nil, nil, err
	}
	if fullAbs != rootAbs && !strings.HasPrefix(fullAbs, rootAbs+string(filepath.Separator)) {
		return nil, nil, ErrMediaInvalid
	}
	file, err := os.Open(fullAbs) //nolint:gosec // path contido em rootDir (checado acima)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	return file, info, nil
}

// Remove apaga somente uma chave que passe pela mesma guarda de containment do Open. E usado para
// compensar arquivo preparado quando a transacao mensagem+outbox sofre rollback. Ausencia ja conta
// como removida; nunca recebe path absoluto nem URL.
func (s *DiskMediaStorage) Remove(storageKey string) error {
	file, _, err := s.Open(storageKey)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) { //nolint:gosec // path validado por Open
		return nil
	} else {
		return err
	}
}

// copyCapped copia ate maxBytes; um byte a mais => ErrMediaTooLarge (413). Nao materializa
// o conteudo — streaming direto para o arquivo.
func copyCapped(dst io.Writer, src io.Reader, maxBytes int64) (int64, error) {
	n, err := io.Copy(dst, io.LimitReader(src, maxBytes+1))
	if err != nil {
		return n, err
	}
	if n > maxBytes {
		return n, ErrMediaTooLarge
	}
	return n, nil
}

// parseDataURL extrai (mime, payloadBase64) de "data:<mime>;base64,<payload>". So base64 e
// suportado (o front sempre manda base64 — readFileAsDataUrl).
func parseDataURL(raw string) (string, string, bool) {
	rest := strings.TrimPrefix(raw, "data:")
	comma := strings.IndexByte(rest, ',')
	if comma < 0 {
		return "", "", false
	}
	meta, payload := rest[:comma], rest[comma+1:]
	if !strings.Contains(meta, "base64") {
		return "", "", false
	}
	mime := meta
	if i := strings.IndexByte(meta, ';'); i >= 0 {
		mime = meta[:i]
	}
	return strings.TrimSpace(mime), payload, true
}

// normalizeMime baixa a caixa e descarta parametros (";charset=..."/";codecs=...").
func normalizeMime(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if i := strings.IndexByte(v, ';'); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return v
}

// categoryForType mapeia o type da mensagem para a categoria de midia. "" quando o type nao
// e de midia (TEXT) ou e desconhecido — nesse caso o Save so exige o mime no allowlist.
func categoryForType(t string) mediaCategory {
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case "IMAGE":
		return catImage
	case "AUDIO":
		return catAudio
	case "VIDEO":
		return catVideo
	case "DOCUMENT":
		return catDocument
	default:
		return ""
	}
}

// mediaDisplayName escolhe um rotulo amigavel: o nome original (base, limitado) ou o nome
// gerado no disco. So exibicao — o arquivo real e o nome aleatorio.
func mediaDisplayName(original, fallback string) string {
	clean := strings.TrimSpace(filepath.Base(strings.TrimSpace(original)))
	if clean == "" || clean == "." || clean == string(filepath.Separator) {
		return fallback
	}
	if len(clean) > 120 {
		clean = clean[:120]
	}
	return clean
}

// sanitizeSegment limpa um segmento de path (accountId/conversationId) para uso em disco.
func sanitizeSegment(value string) string {
	replacer := strings.NewReplacer("/", "", "\\", "", "..", "", " ", "", ":", "")
	return strings.TrimSpace(replacer.Replace(strings.TrimSpace(value)))
}

// randomSuffix gera o nome aleatorio do arquivo (sem input do usuario no nome => sem traversal).
func randomSuffix() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "media"
	}
	return hex.EncodeToString(b)
}
