package bio

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Limites de upload default (bytes), sobrescritos por env no module.go
// (BIO_MAX_VIDEO_MB / BIO_MAX_IMAGE_MB).
const (
	defaultMaxBioVideoBytes int64 = 200 * 1024 * 1024
	defaultMaxBioImageBytes int64 = 10 * 1024 * 1024
	multipartHeadroomBytes  int64 = 8 * 1024 * 1024
)

// Erros do storage de midia.
var (
	ErrInvalidKind  = errors.New("bio: invalid media kind")
	ErrInvalidMedia = errors.New("bio: invalid media file")
	ErrMediaTooBig  = errors.New("bio: media file too large")
	ErrStorageUnset = errors.New("bio: uploads dir not configured")
)

// validKinds sao os tipos de midia aceitos no upload.
var validKinds = map[string]struct{}{
	"video":      {},
	"background": {}, // imagem de fundo (alternativa ao video)
	"poster":     {},
	"logo":       {},
	"favicon":    {},
	"slide":      {},
	"store":      {},
}

// StoredMedia descreve o arquivo gravado.
type StoredMedia struct {
	Path        string // /uploads/bio/{accountId}/{arquivo}
	ContentType string
	SizeBytes   int64
}

// MediaStorage grava arquivos de midia em disco sob UPLOADS_DIR/bio/{accountId}/.
type MediaStorage struct {
	rootDir       string
	maxVideoBytes int64
	maxImageBytes int64
}

// NewMediaStorage cria o storage. rootDir = UPLOADS_DIR; limites <= 0 caem nos
// defaults. Limites configuraveis via env (module.go).
func NewMediaStorage(rootDir string, maxVideoBytes, maxImageBytes int64) *MediaStorage {
	if maxVideoBytes <= 0 {
		maxVideoBytes = defaultMaxBioVideoBytes
	}
	if maxImageBytes <= 0 {
		maxImageBytes = defaultMaxBioImageBytes
	}
	return &MediaStorage{
		rootDir:       strings.TrimSpace(rootDir),
		maxVideoBytes: maxVideoBytes,
		maxImageBytes: maxImageBytes,
	}
}

// MaxUploadBytes e o teto do corpo multipart (maior limite + folga de headers).
func (m *MediaStorage) MaxUploadBytes() int64 {
	return m.maxVideoBytes + multipartHeadroomBytes
}

// Save valida kind/mime/tamanho e grava o arquivo. Diretorios 0o750, arquivos
// 0o600 (lint gosec G301/G306). Retorna o path relativo /uploads/bio/...
func (m *MediaStorage) Save(accountID, kind, fileName, contentType string, content []byte) (*StoredMedia, error) {
	rootDir := strings.TrimSpace(m.rootDir)
	if rootDir == "" {
		return nil, ErrStorageUnset
	}
	if _, ok := validKinds[kind]; !ok {
		return nil, ErrInvalidKind
	}
	account := sanitizeSegment(accountID)
	if account == "" {
		return nil, ErrInvalidMedia
	}

	resolvedType, extension, isVideo := resolveMediaType(content, contentType, fileName)
	if extension == "" {
		return nil, ErrInvalidMedia
	}
	if err := m.checkSize(len(content), isVideo); err != nil {
		return nil, err
	}

	bioDir := filepath.Join(rootDir, "bio", account)
	if err := os.MkdirAll(bioDir, 0o750); err != nil {
		return nil, err
	}

	// name = kind sanitizado + sufixo aleatorio + extensao da allowlist. Nunca usa
	// o filename cru do upload, entao nao ha traversal possivel.
	name := sanitizeSegment(kind) + "-" + randomSuffix() + extension
	if err := os.WriteFile(filepath.Join(bioDir, name), content, 0o600); err != nil { //nolint:gosec // G703: path montado de segmentos sanitizados + sufixo aleatorio
		return nil, err
	}

	return &StoredMedia{
		Path:        "/uploads/bio/" + account + "/" + name,
		ContentType: resolvedType,
		SizeBytes:   int64(len(content)),
	}, nil
}

func (m *MediaStorage) checkSize(size int, isVideo bool) error {
	if size == 0 {
		return ErrInvalidMedia
	}
	limit := m.maxImageBytes
	if isVideo {
		limit = m.maxVideoBytes
	}
	if int64(size) > limit {
		return ErrMediaTooBig
	}
	return nil
}

// resolveMediaType detecta o content-type a partir do conteudo (sniff) com
// fallback no header/extensao. Devolve (mime, extensao, isVideo). Extensao vazia
// quando o tipo nao esta na allowlist (mp4, webm, webp, avif, png, jpeg, svg, ico).
func resolveMediaType(content []byte, fallback, fileName string) (string, string, bool) {
	if mime, ext, isVideo, ok := matchAllowedType(sniffType(content)); ok {
		return mime, ext, isVideo
	}
	if mime, ext, isVideo, ok := matchAllowedType(strings.ToLower(strings.TrimSpace(fallback))); ok {
		return mime, ext, isVideo
	}
	if mime, ext, isVideo, ok := typeFromExtension(fileName); ok {
		return mime, ext, isVideo
	}
	return "", "", false
}

func sniffType(content []byte) string {
	if len(content) == 0 {
		return ""
	}
	sniffLen := len(content)
	if sniffLen > 512 {
		sniffLen = 512
	}
	return strings.ToLower(strings.TrimSpace(http.DetectContentType(content[:sniffLen])))
}

// matchAllowedType resolve um mime para (mime canonico, extensao, isVideo).
func matchAllowedType(mime string) (string, string, bool, bool) {
	switch mime {
	case "video/mp4":
		return "video/mp4", ".mp4", true, true
	case "video/webm":
		return "video/webm", ".webm", true, true
	case "image/webp":
		return "image/webp", ".webp", false, true
	case "image/avif":
		return "image/avif", ".avif", false, true
	case "image/png":
		return "image/png", ".png", false, true
	case "image/jpeg", "image/jpg":
		return "image/jpeg", ".jpg", false, true
	case "image/svg+xml", "text/xml; charset=utf-8", "text/xml":
		return "image/svg+xml", ".svg", false, true
	case "image/x-icon", "image/vnd.microsoft.icon":
		return "image/x-icon", ".ico", false, true
	default:
		return "", "", false, false
	}
}

func typeFromExtension(fileName string) (string, string, bool, bool) {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(fileName))) {
	case ".mp4":
		return "video/mp4", ".mp4", true, true
	case ".webm":
		return "video/webm", ".webm", true, true
	case ".webp":
		return "image/webp", ".webp", false, true
	case ".avif":
		return "image/avif", ".avif", false, true
	case ".png":
		return "image/png", ".png", false, true
	case ".jpg", ".jpeg":
		return "image/jpeg", ".jpg", false, true
	case ".svg":
		return "image/svg+xml", ".svg", false, true
	case ".ico":
		return "image/x-icon", ".ico", false, true
	default:
		return "", "", false, false
	}
}

func sanitizeSegment(value string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-", "..", "")
	return strings.Trim(strings.ToLower(replacer.Replace(strings.TrimSpace(value))), "-")
}

func randomSuffix() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "media"
	}
	return hex.EncodeToString(buf)
}
