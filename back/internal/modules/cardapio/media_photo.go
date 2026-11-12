package cardapio

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Fotos de PRODUTO (aba Produtos do painel). Diferente da midia generica
// (logo/banner/video, media_storage.go), a foto de produto e convertida: o
// original fica guardado ao lado, o site usa um AVIF no tamanho exibido pelo
// TAVOLA e a tabela do painel usa uma miniatura AVIF leve.
const (
	// maxProductPhotoBytes e o teto do upload de foto de produto (so imagem).
	maxProductPhotoBytes = 20 * 1024 * 1024
	// largeProductPhotoBytes marca originais grandes para revisao do admin.
	largeProductPhotoBytes = 15 * 1024 * 1024
	// Caixa maxima da versao do site: a foto do TAVOLA e 4:5 e chega a ~600px
	// CSS de largura; 1200x1500 cobre telas 2x sem ampliar fotos menores.
	productPhotoMaxWidth  = 1200
	productPhotoMaxHeight = 1500
	// Miniatura quadrada da tabela do painel (celula de ~64px CSS, 3x).
	productThumbSize = 192
	// Conversoes simultaneas por processo: protege a CPU da VPS.
	productPhotoConcurrency = 2
	productPhotoTimeout     = 90 * time.Second
)

// ProductPhoto e o resultado do upload de uma foto de produto. URL e a imagem
// usada no site (AVIF convertido; o original quando a conversao falha),
// ThumbURL a miniatura (vazia sem conversao) e OriginalURL o arquivo enviado.
type ProductPhoto struct {
	ID            string `json:"id,omitempty"`
	URL           string `json:"url"`
	ThumbURL      string `json:"thumbUrl"`
	OriginalURL   string `json:"originalUrl"`
	OriginalBytes int64  `json:"originalBytes"`
	Converted     bool   `json:"converted"`
}

// ProductPhotoStorage persiste e converte fotos de produto.
type ProductPhotoStorage interface {
	SaveProductPhoto(ctx context.Context, accountID, fileName, contentType string, content []byte) (ProductPhoto, error)
}

// photoEncoder converte uma imagem em disco para AVIF. Interface para testes.
type photoEncoder interface {
	EncodeAVIF(ctx context.Context, src, dst string, variant photoVariant) error
}

type photoVariant int

const (
	photoVariantSite photoVariant = iota
	photoVariantThumb
)

// ffmpegPhotoEncoder usa o ffmpeg da imagem da API (libaom-av1 + muxer avif).
// O ffmpeg aplica a orientacao EXIF (autorotate) antes dos filtros.
type ffmpegPhotoEncoder struct {
	slots chan struct{}
}

func newFFmpegPhotoEncoder() *ffmpegPhotoEncoder {
	return &ffmpegPhotoEncoder{slots: make(chan struct{}, productPhotoConcurrency)}
}

// Filtros: redimensiona, achata transparencia sobre branco (o AVIF sai em
// yuv420p, sem alpha) e garante dimensoes pares para o 4:2:0.
func photoFilter(variant photoVariant) string {
	resize := fmt.Sprintf(
		"scale=w='min(%d,iw)':h='min(%d,ih)':force_original_aspect_ratio=decrease:flags=lanczos,"+
			"crop=trunc(iw/2)*2:trunc(ih/2)*2",
		productPhotoMaxWidth, productPhotoMaxHeight,
	)
	if variant == photoVariantThumb {
		resize = fmt.Sprintf(
			"scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d",
			productThumbSize, productThumbSize, productThumbSize, productThumbSize,
		)
	}
	return "[0:v]" + resize + ",format=rgba,split[a][b];" +
		"[a]drawbox=c=white@1:t=fill[bg];[bg][b]overlay=format=auto,format=yuv420p[out]"
}

func photoCRF(variant photoVariant) string {
	if variant == photoVariantThumb {
		return "32"
	}
	return "24"
}

func (e *ffmpegPhotoEncoder) EncodeAVIF(ctx context.Context, src, dst string, variant photoVariant) error {
	select {
	case e.slots <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-e.slots }()

	runCtx, cancel := context.WithTimeout(ctx, productPhotoTimeout)
	defer cancel()
	// Argumentos fixos; src/dst sao caminhos gerados pelo proprio modulo.
	cmd := exec.CommandContext(runCtx, "ffmpeg", //nolint:gosec // binario fixo, sem shell, paths internos
		"-v", "error", "-y", "-i", src,
		"-filter_complex", photoFilter(variant), "-map", "[out]",
		"-frames:v", "1", "-c:v", "libaom-av1", "-still-picture", "1",
		"-crf", photoCRF(variant), "-b:v", "0", "-cpu-used", "6",
		dst,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg avif: %w: %s", err, strings.TrimSpace(string(output)))
	}
	info, err := os.Stat(dst)
	if err != nil || info.Size() == 0 {
		return errors.New("ffmpeg avif: saida vazia")
	}
	return nil
}

// SaveProductPhoto grava o original e gera as versoes AVIF (site + miniatura).
// Aceita somente imagem (jpeg/png/webp/gif) ate 20MB. Se a conversao falhar o
// upload nao falha: a foto fica com o original, como no fluxo anterior.
func (s *DiskMediaStorage) SaveProductPhoto(ctx context.Context, accountID, fileName, contentType string, content []byte) (ProductPhoto, error) {
	mime := detectImageContentType(content, contentType)
	extension := mediaExtension(mime, fileName)
	if mime == "" || extension == "" || len(content) == 0 || len(content) > maxProductPhotoBytes {
		return ProductPhoto{}, ErrInvalidMedia
	}
	account := sanitizeSegment(accountID)
	if account == "" {
		return ProductPhoto{}, ErrInvalidMedia
	}
	dir := filepath.Join(s.rootDir, "cardapio", account)
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // segmento sanitizado, sem traversal
		return ProductPhoto{}, err
	}

	base := randomSuffix()
	originalName := base + ".orig" + extension
	originalPath := filepath.Join(dir, originalName)
	if err := os.WriteFile(originalPath, content, 0o600); err != nil { //nolint:gosec // path interno
		return ProductPhoto{}, err
	}
	prefix := "/uploads/cardapio/" + account + "/"
	photo := ProductPhoto{
		URL:           prefix + originalName,
		OriginalURL:   prefix + originalName,
		OriginalBytes: int64(len(content)),
	}
	// GIF animado perderia a animacao: fica como enviado.
	if mime == "image/gif" || s.encoder == nil {
		return photo, nil
	}
	siteName, thumbName, err := convertProductPhoto(ctx, s.encoder, dir, originalPath, base)
	if err != nil {
		s.logConversionFailure(err)
		return photo, nil
	}
	photo.URL = prefix + siteName
	photo.ThumbURL = prefix + thumbName
	photo.Converted = true
	return photo, nil
}

// convertProductPhoto gera {base}.thumb.avif e {base}.avif em dir. A miniatura
// e gerada primeiro: a versao do site so existe quando as duas deram certo.
func convertProductPhoto(ctx context.Context, encoder photoEncoder, dir, src, base string) (string, string, error) {
	siteName := base + ".avif"
	thumbName := base + ".thumb.avif"
	thumbPath := filepath.Join(dir, thumbName)
	sitePath := filepath.Join(dir, siteName)
	if err := encoder.EncodeAVIF(ctx, src, thumbPath, photoVariantThumb); err != nil {
		_ = os.Remove(thumbPath)
		return "", "", err
	}
	if err := encoder.EncodeAVIF(ctx, src, sitePath, photoVariantSite); err != nil {
		_ = os.Remove(thumbPath)
		_ = os.Remove(sitePath)
		return "", "", err
	}
	_ = os.Chmod(thumbPath, 0o600)
	_ = os.Chmod(sitePath, 0o600)
	return siteName, thumbName, nil
}

func (s *DiskMediaStorage) logConversionFailure(err error) {
	if s.logf != nil {
		s.logf("cardapio_product_photo_convert_failed", err)
	}
}

// detectImageContentType aceita somente imagem real: o mime vem do sniff do
// conteudo (nunca so do header declarado), dentro da allowlist de imagem.
func detectImageContentType(content []byte, _ string) string {
	if len(content) == 0 {
		return ""
	}
	sniffLen := min(len(content), 512)
	return normalizeImageMime(http.DetectContentType(content[:sniffLen]))
}
