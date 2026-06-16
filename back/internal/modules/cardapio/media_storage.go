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
	maxMediaBytes          = 5 * 1024 * 1024 // 5MB
	maxMediaMultipartBytes = maxMediaBytes + 256*1024
)

// MediaStorage abstrai a persistencia de imagens do cardapio (testavel via mock).
type MediaStorage interface {
	// Save grava a imagem em uploads/cardapio/{accountId}/ e devolve o caminho
	// relativo "/uploads/cardapio/{accountId}/{file}". content/contentType ja
	// lidos pelo handler.
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

// Save valida tamanho/mime e grava o arquivo com permissoes restritas.
func (s *DiskMediaStorage) Save(accountID, fileName, contentType string, content []byte) (string, error) {
	if len(content) == 0 || len(content) > maxMediaBytes {
		return "", ErrInvalidMedia
	}
	normalized := detectImageContentType(content, contentType)
	extension := imageExtension(normalized, fileName)
	if normalized == "" || extension == "" {
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

// detectImageContentType sniffa o conteudo e cai no fallback do header,
// aceitando so a allowlist de imagens.
func detectImageContentType(content []byte, fallback string) string {
	if len(content) > 0 {
		sniffLen := len(content)
		if sniffLen > 512 {
			sniffLen = 512
		}
		switch strings.ToLower(strings.TrimSpace(http.DetectContentType(content[:sniffLen]))) {
		case "image/jpeg", "image/png", "image/webp", "image/gif":
			return normalizeImageMime(http.DetectContentType(content[:sniffLen]))
		}
	}
	return normalizeImageMime(fallback)
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

func imageExtension(contentType, fileName string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
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
		return "image"
	}
	return hex.EncodeToString(b)
}
