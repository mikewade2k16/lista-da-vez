package calendar

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// MediaStorage abstrai a persistencia dos anexos do calendario (testavel via mock).
type MediaStorage interface {
	// Save valida mime/tamanho (contra limits) e grava a midia em
	// uploads/calendar/{accountId}/, devolvendo o MediaItem pronto para anexar a
	// um evento ou dia. content/contentType/fileName ja lidos pelo handler.
	Save(accountID, fileName, contentType string, content []byte, limits MediaLimits) (MediaItem, error)
}

// DiskMediaStorage grava no disco local sob rootDir/calendar/{accountId}/.
type DiskMediaStorage struct {
	rootDir string
}

// NewDiskMediaStorage cria o storage. rootDir vazio => "uploads" (relativo ao
// processo), espelhando o default da plataforma (cfg.UploadsDir).
func NewDiskMediaStorage(rootDir string) *DiskMediaStorage {
	root := strings.TrimSpace(rootDir)
	if root == "" {
		root = "uploads"
	}
	return &DiskMediaStorage{rootDir: root}
}

// Save valida tipo/tamanho e grava com permissoes restritas. O teto depende do
// tipo (limits.ImageMaxBytes vs limits.VideoMaxBytes).
func (s *DiskMediaStorage) Save(accountID, fileName, contentType string, content []byte, limits MediaLimits) (MediaItem, error) {
	normalized := detectMediaContentType(content, contentType, fileName)
	extension := mediaExtension(normalized, fileName)
	if normalized == "" || extension == "" {
		return MediaItem{}, ErrInvalidMedia
	}

	kind := "image"
	limit := limits.ImageMaxBytes
	if isVideoMime(normalized) {
		kind = "video"
		limit = limits.VideoMaxBytes
	}
	if limit <= 0 {
		return MediaItem{}, ErrInvalidMedia
	}
	if len(content) == 0 || int64(len(content)) > limit {
		return MediaItem{}, ErrMediaTooLarge
	}

	account := sanitizeSegment(accountID)
	if account == "" {
		return MediaItem{}, ErrInvalidMedia
	}

	// account e sanitizado (sem "/", "\\", "..", ":") e name e gerado aleatoriamente
	// (sem input do usuario): nao ha traversal possivel nos paths abaixo.
	dir := filepath.Join(s.rootDir, "calendar", account)
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // segmento sanitizado, sem traversal
		return MediaItem{}, err
	}

	id := randomSuffix()
	name := id + extension
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil { //nolint:gosec // path derivado de segmento sanitizado + nome aleatorio
		return MediaItem{}, err
	}

	return MediaItem{
		ID:          id,
		URL:         "/uploads/calendar/" + account + "/" + name,
		Name:        displayName(fileName, name),
		Type:        kind,
		ContentType: normalized,
		SizeBytes:   len(content),
	}, nil
}

// displayName escolhe um rotulo amigavel: o nome original (sanitizado) ou, se
// vazio, o nome gerado no disco.
func displayName(original, fallback string) string {
	clean := strings.TrimSpace(filepath.Base(strings.TrimSpace(original)))
	if clean == "" || clean == "." || clean == string(filepath.Separator) {
		return fallback
	}
	if len(clean) > 120 {
		clean = clean[:120]
	}
	return clean
}

// detectMediaContentType resolve o mime aceito (imagem OU video). Imagem: sniffa o
// conteudo (confiavel) + fallback do header. Video: http.DetectContentType e fraco
// (mp4 vira octet-stream), entao confia no contentType declarado SE estiver no
// allowlist de video E a extensao casar com o mime. Nunca aceita mime arbitrario.
func detectMediaContentType(content []byte, fallback, fileName string) string {
	if len(content) > 0 {
		sniffLen := len(content)
		if sniffLen > 512 {
			sniffLen = 512
		}
		sniffed := http.DetectContentType(content[:sniffLen])
		if mime := normalizeImageMime(sniffed); mime != "" {
			return mime
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
// video declarado (defesa extra: nao basta o header dizer video).
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
	return strings.TrimSpace(replacer.Replace(strings.TrimSpace(value)))
}

func randomSuffix() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "media"
	}
	return hex.EncodeToString(b)
}
