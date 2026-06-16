package site

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // hash so para nome de arquivo (idempotencia), nao seguranca
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Limites do cache de imagens de produto.
const (
	productImageMaxBytes int64 = 15 << 20 // 15 MB por imagem
	productImageGetTO          = 12 * time.Second
	productImageCacheTO        = 150 * time.Second // teto do passo inteiro no sync
	productImageWorkers        = 5
)

// ImageCache baixa as imagens externas dos produtos UMA vez e as serve
// localmente em /uploads/site/products/{account}/{hash}.{ext}. Desacopla a bio e
// o admin da origem do cliente: sem hotlink (que martelava o CDN/Cloudflare da
// Perola e disparava bloqueio) e sem depender da origem estar no ar a cada view.
type ImageCache struct {
	rootDir    string
	httpClient *http.Client
}

// NewImageCache cria o cache sob rootDir (= UPLOADS_DIR). rootDir vazio => no-op.
func NewImageCache(rootDir string) *ImageCache {
	return &ImageCache{
		rootDir:    strings.TrimSpace(rootDir),
		httpClient: &http.Client{Timeout: productImageGetTO},
	}
}

// CacheItems baixa as imagens externas dos itens e reescreve item.Image para o
// path local quando consegue. Falha (origem fora, 404, timeout) MANTEM a URL
// externa como fallback — nunca aborta o sync por causa de imagem. Concorrencia
// limitada + teto de tempo total. Devolve quantas imagens passaram a ser locais.
func (c *ImageCache) CacheItems(ctx context.Context, accountID string, items []ProductUpsertItem) int {
	if c == nil || c.rootDir == "" || len(items) == 0 {
		return 0
	}
	account := sanitizeImageSegment(accountID)
	if account == "" {
		return 0
	}
	dir := filepath.Join(c.rootDir, "site", "products", account)
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // G703: dir = UPLOADS_DIR + constantes + account sanitizado
		return 0
	}

	cctx, cancel := context.WithTimeout(ctx, productImageCacheTO)
	defer cancel()

	var (
		sem    = make(chan struct{}, productImageWorkers)
		wg     sync.WaitGroup
		mu     sync.Mutex
		cached int
	)
	for i := range items {
		urls := candidateURLs(items[i])
		if len(urls) == 0 {
			continue // ja local ou vazio
		}
		idx := i
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if cctx.Err() != nil {
				return
			}
			// Tenta as candidatas em ordem; a 1a que baixar (200) vence.
			for _, url := range urls {
				if cctx.Err() != nil {
					return
				}
				local, err := c.fetchOne(cctx, dir, account, url, imageExtFromURL(url))
				if err != nil || local == "" {
					continue
				}
				items[idx].Image = local
				mu.Lock()
				cached++
				mu.Unlock()
				return
			}
			// Nenhuma respondeu. Se TODAS eram hotlink da Perola (que bloqueia o
			// browser via Cloudflare), zera a imagem p/ o front mostrar "sem img"
			// em vez de martelar uma URL que da timeout. Outras origens: mantem o
			// fallback externo (podem permitir hotlink).
			if allPerolaHost(urls) {
				items[idx].Image = ""
			}
		}()
	}
	wg.Wait()
	return cached
}

// candidateURLs devolve as URLs http(s) a tentar para um item: a lista explicita
// ImageCandidates (preenchida pela fonte) ou, na ausencia, o proprio Image.
func candidateURLs(item ProductUpsertItem) []string {
	src := item.ImageCandidates
	if len(src) == 0 {
		src = []string{item.Image}
	}
	out := make([]string, 0, len(src))
	for _, u := range src {
		u = strings.TrimSpace(u)
		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			out = append(out, u)
		}
	}
	return out
}

// allPerolaHost informa se todas as URLs apontam para uma fonte que o BROWSER nao
// consegue carregar: o host da Perola (Cloudflare bloqueia o IP) ou o XAMPP local
// via host.docker.internal (resolve so dentro do container). Nesses casos, em vez
// de deixar um fallback que vai dar erro/timeout no front, melhor zerar a imagem.
func allPerolaHost(urls []string) bool {
	if len(urls) == 0 {
		return false
	}
	allPerola, allDocker := true, true
	for _, u := range urls {
		if !strings.HasPrefix(u, perolaBaseHost) {
			allPerola = false
		}
		if !strings.Contains(u, "://"+dockerInternalHost) {
			allDocker = false
		}
	}
	return allPerola || allDocker
}

// fetchOne baixa uma imagem (idempotente: nome = sha1(url)+ext; ja existe => pula
// o download). Devolve o path relativo /uploads/site/products/...
func (c *ImageCache) fetchOne(ctx context.Context, dir, account, urlStr, ext string) (string, error) {
	sum := sha1.Sum([]byte(urlStr)) //nolint:gosec // só nome de arquivo
	name := hex.EncodeToString(sum[:]) + ext
	full := filepath.Join(dir, name)
	rel := "/uploads/site/products/" + account + "/" + name
	if _, err := os.Stat(full); err == nil { //nolint:gosec // G703: full = dir sanitizado + hash(url)+ext
		return rel, nil // ja cacheado
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", err
	}
	// UA descritivo: o WAF da Perola bloqueia o UA padrao do Go com 406.
	req.Header.Set("User-Agent", "OmniSync/1.0 (+https://omni)")
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/jpeg,image/*,*/*")
	// Fonte local (XAMPP): se a imagem nao existir, o .htaccess cai no index.php,
	// que so usa o banco offline quando HTTP_HOST contem "localhost". Forcamos o
	// Host p/ nao estourar erro de DB online (e a validacao abaixo rejeita o HTML).
	if u, perr := url.Parse(urlStr); perr == nil && u.Hostname() == dockerInternalHost {
		req.Host = "localhost"
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("site: image source status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, productImageMaxBytes))
	if err != nil {
		return "", err
	}
	if len(body) == 0 {
		return "", errors.New("site: empty image body")
	}
	// Valida que o corpo e REALMENTE uma imagem. Sem isso, um 200 com pagina de
	// erro (ex.: o .htaccess da Perola redireciona arquivo ausente p/ index.php,
	// que devolve HTML 200) seria cacheado como "imagem" (bug real: 418 arquivos
	// de 563 bytes com "Fatal error" do PHP). Falha => tenta a proxima candidata.
	if !looksLikeImage(resp.Header.Get("Content-Type"), body) {
		return "", errors.New("site: response is not an image")
	}
	if err := os.WriteFile(full, body, 0o600); err != nil { //nolint:gosec // path = segmentos sanitizados + hash
		return "", err
	}
	return rel, nil
}

// looksLikeImage confirma que o corpo e uma imagem: Content-Type image/* OU magic
// bytes conhecidos (png/jpeg/gif/webp/avif/heic). Rejeita HTML/erros do PHP.
func looksLikeImage(contentType string, body []byte) bool {
	// No sync (download do Apache) o Content-Type e confiavel; basta ele.
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "image/") {
		return true
	}
	return isImageBytes(body)
}

// isImageBytes confere o conteudo SO pelos magic bytes (png/jpeg/gif/webp/avif).
// Usado no upload manual, onde o Content-Type vem do cliente e nao e confiavel.
func isImageBytes(body []byte) bool {
	if len(body) < 12 {
		return false
	}
	switch {
	case bytes.HasPrefix(body, []byte("\x89PNG")):
		return true
	case bytes.HasPrefix(body, []byte{0xFF, 0xD8, 0xFF}): // jpeg
		return true
	case bytes.HasPrefix(body, []byte("GIF8")):
		return true
	case bytes.HasPrefix(body, []byte("RIFF")) && bytes.Equal(body[8:12], []byte("WEBP")):
		return true
	case bytes.Equal(body[4:8], []byte("ftyp")): // ISO-BMFF: avif/heic
		return true
	}
	return false
}

// errImageStorageUnset sinaliza UPLOADS_DIR ausente no upload manual de imagem.
var errImageStorageUnset = errors.New("site: uploads dir not configured")

// SaveUpload grava uma imagem ENVIADA pelo painel (upload manual de produto) em
// /uploads/site/products/{account}/up-{rand}.{ext} e devolve o path relativo.
// Valida que o conteudo e imagem de verdade (allowlist + magic bytes), igual ao
// cache do sync. Diferente de fetchOne, a fonte aqui sao os bytes do multipart.
func (c *ImageCache) SaveUpload(accountID, filename, contentType string, content []byte) (string, error) {
	if c == nil || c.rootDir == "" {
		return "", errImageStorageUnset
	}
	account := sanitizeImageSegment(accountID)
	if account == "" {
		return "", errors.New("site: invalid account for image upload")
	}
	if len(content) == 0 {
		return "", errors.New("site: empty image upload")
	}
	ext := uploadImageExt(filename, contentType)
	// Valida pelos MAGIC BYTES (nao confia no content-type do cliente).
	if ext == "" || !isImageBytes(content) {
		return "", errors.New("site: upload is not a supported image")
	}
	dir := filepath.Join(c.rootDir, "site", "products", account)
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // G703: dir = UPLOADS_DIR + constantes + account sanitizado
		return "", err
	}
	name := "up-" + randomImageSuffix() + ext
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil { //nolint:gosec // path = segmentos sanitizados + sufixo aleatorio
		return "", err
	}
	return "/uploads/site/products/" + account + "/" + name, nil
}

// uploadImageExt deduz a extensao do upload pelo nome do arquivo e, em fallback,
// pelo content-type. Fora da allowlist => "" (rejeita).
func uploadImageExt(filename, contentType string) string {
	switch ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename))); ext {
	case ".avif", ".webp", ".png", ".jpg", ".jpeg", ".gif":
		return ext
	}
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/avif":
		return ".avif"
	case "image/webp":
		return ".webp"
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	}
	return ""
}

func randomImageSuffix() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "img"
	}
	return hex.EncodeToString(buf)
}

// imageExtFromURL extrai a extensao (sem query). Fora da allowlist => ".img".
func imageExtFromURL(rawURL string) string {
	u := rawURL
	if i := strings.IndexByte(u, '?'); i >= 0 {
		u = u[:i]
	}
	switch ext := strings.ToLower(filepath.Ext(u)); ext {
	case ".avif", ".webp", ".png", ".jpg", ".jpeg", ".gif":
		return ext
	default:
		return ".img"
	}
}

// sanitizeImageSegment normaliza um segmento de path (sem traversal).
func sanitizeImageSegment(value string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-", "..", "")
	return strings.Trim(strings.ToLower(replacer.Replace(strings.TrimSpace(value))), "-")
}
