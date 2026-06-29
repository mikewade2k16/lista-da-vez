package cardapio

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxMediaBytes = 5 * 1024 * 1024  // 5MB para imagem
	maxVideoBytes = 60 * 1024 * 1024 // 60MB para video (fundo de hero/CTA/banner)
	// Teto do multipart cobre o maior tipo (video) + folga para os headers do form.
	maxMediaMultipartBytes = maxVideoBytes + 256*1024
)

// MediaStorage abstrai a persistencia de midia do cardapio (testavel via mock).
type MediaStorage interface {
	// Save grava a midia (imagem ou video) em uploads/cardapio/{accountId}/ e
	// devolve o caminho relativo "/uploads/cardapio/{accountId}/{file}".
	// content/contentType ja lidos pelo handler.
	Save(accountID, fileName, contentType string, content []byte) (string, error)
}

// DiskMediaStorage grava no disco local sob rootDir/cardapio/{accountId}/.
type DiskMediaStorage struct {
	rootDir string
}

// NewDiskMediaStorage cria o storage. rootDir vazio => uploads (relativo ao
// processo), espelhando o default da plataforma.
func NewDiskMediaStorage(rootDir string) *DiskMediaStorage {
	root := strings.TrimSpace(rootDir)
	if root == "" {
		root = "uploads"
	}
	return &DiskMediaStorage{rootDir: root}
}

// Save valida tamanho/mime e grava o arquivo com permissoes restritas. O teto de
// tamanho depende do tipo: 5MB para imagem, 60MB para video.
func (s *DiskMediaStorage) Save(accountID, fileName, contentType string, content []byte) (string, error) {
	normalized := detectMediaContentType(content, contentType, fileName)
	extension := mediaExtension(normalized, fileName)
	if normalized == "" || extension == "" {
		return "", ErrInvalidMedia
	}
	limit := maxMediaBytes
	if isVideoMime(normalized) {
		limit = maxVideoBytes
	}
	if len(content) == 0 || len(content) > limit {
		return "", ErrInvalidMedia
	}

	account := sanitizeSegment(accountID)
	if account == "" {
		return "", ErrInvalidMedia
	}

	// account e sanitizado (sem "/", "\\", "..", ":") e name e gerado aleatoriamente
	// (sem input do usuario): nao ha traversal possivel nos paths abaixo.
	dir := filepath.Join(s.rootDir, "cardapio", account)
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // segmento sanitizado, sem traversal
		return "", err
	}

	name := randomSuffix() + extension
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil { //nolint:gosec // path derivado de segmento sanitizado + nome aleatorio
		return "", err
	}
	return "/uploads/cardapio/" + account + "/" + name, nil
}

// detectMediaContentType resolve o mime aceito (imagem OU video).
//
// Imagem: sniffa o conteudo (http.DetectContentType e confiavel para imagem) e
// cai no fallback do header — sempre dentro da allowlist.
//
// Video: http.DetectContentType e fraco para video (mp4 costuma cair em
// application/octet-stream), entao confia no contentType declarado SE estiver no
// allowlist de video E a extensao do fileName casar com o mime declarado. Nunca
// aceita mime arbitrario.
func detectMediaContentType(content []byte, fallback, fileName string) string {
	if len(content) > 0 {
		sniffLen := len(content)
		if sniffLen > 512 {
			sniffLen = 512
		}
		sniffed := http.DetectContentType(content[:sniffLen])
		switch strings.ToLower(strings.TrimSpace(sniffed)) {
		case "image/jpeg", "image/png", "image/webp", "image/gif":
			return normalizeImageMime(sniffed)
		}
	}
	if mime := normalizeImageMime(fallback); mime != "" {
		return mime
	}
	if mime := normalizeVideoMime(fallback); mime != "" && videoExtensionMatches(mime, fileName) {
		return mime
	}
	return ""
}

func normalizeImageMime(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image/jpeg", "image/jpg":
		return "image/jpeg"
	case "image/png":
		return "image/png"
	case "image/webp":
		return "image/webp"
	case "image/gif":
		return "image/gif"
	default:
		return ""
	}
}

func normalizeVideoMime(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "video/mp4":
		return "video/mp4"
	case "video/webm":
		return "video/webm"
	case "video/quicktime":
		return "video/quicktime"
	default:
		return ""
	}
}

func isVideoMime(value string) bool {
	return normalizeVideoMime(value) != ""
}

// videoExtensionMatches garante que a extensao do arquivo corresponde ao mime de
// video declarado (defesa extra: nao basta o header dizer video, o nome precisa
// casar com a allowlist).
func videoExtensionMatches(mime, fileName string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	switch mime {
	case "video/mp4":
		return ext == ".mp4"
	case "video/webm":
		return ext == ".webm"
	case "video/quicktime":
		return ext == ".mov"
	default:
		return false
	}
}

func mediaExtension(contentType, fileName string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	}
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(fileName))) {
	case ".jpg", ".jpeg":
		return ".jpg"
	case ".png":
		return ".png"
	case ".webp":
		return ".webp"
	case ".gif":
		return ".gif"
	case ".mp4":
		return ".mp4"
	case ".webm":
		return ".webm"
	case ".mov":
		return ".mov"
	default:
		return ""
	}
}

// sanitizeSegment limpa um segmento de path (accountId) para uso seguro em disco.
func sanitizeSegment(value string) string {
	replacer := strings.NewReplacer("/", "", "\\", "", "..", "", " ", "", ":", "")
	clean := strings.TrimSpace(replacer.Replace(strings.TrimSpace(value)))
	return clean
}

func randomSuffix() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "media"
	}
	return hex.EncodeToString(b)
}
