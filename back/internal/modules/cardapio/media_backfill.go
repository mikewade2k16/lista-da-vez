package cardapio

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/stringsx"
)

// PhotoBackfillReport resume a conversao das fotos de produto ja publicadas.
type PhotoBackfillReport struct {
	DryRun          bool  `json:"dryRun"`
	ProductsScanned int   `json:"productsScanned"`
	PhotosFound     int   `json:"photosFound"`
	Converted       int   `json:"converted"`
	Reused          int   `json:"reused"`
	Skipped         int   `json:"skipped"`
	Failed          int   `json:"failed"`
	ProductsUpdated int   `json:"productsUpdated"`
	BytesBefore     int64 `json:"bytesBefore"`
	BytesAfter      int64 `json:"bytesAfter"`
}

type backfillProduct struct {
	id, accountID, restaurantID, imageURL string
	gallery                               []string
}

type backfillPhoto struct {
	newURL, thumbURL string
	originalBytes    int64
	ok               bool
}

// OptimizeProductPhotos converte para AVIF (site + miniatura) as fotos de produto
// (image_url e galeria) ja gravadas em /uploads/cardapio e aponta os produtos para
// a versao nova. NUNCA apaga o arquivo antigo: sites/caches com a url anterior
// continuam funcionando e o original fica preservado. Idempotente: fotos ja em
// AVIF sao ignoradas e conversoes existentes sao reaproveitadas. dryRun so mede.
func OptimizeProductPhotos(ctx context.Context, pool *pgxpool.Pool, uploadsDir string, dryRun bool, logf func(msg string, args ...any)) (PhotoBackfillReport, error) {
	report := PhotoBackfillReport{DryRun: dryRun}
	root := strings.TrimSpace(uploadsDir)
	if root == "" {
		root = "uploads"
	}
	products, err := loadBackfillProducts(ctx, pool)
	if err != nil {
		return report, err
	}
	report.ProductsScanned = len(products)

	urls := map[string]struct{}{}
	for _, product := range products {
		for _, url := range append([]string{product.imageURL}, product.gallery...) {
			if convertibleUploadURL(url) {
				urls[url] = struct{}{}
			}
		}
	}
	report.PhotosFound = len(urls)
	results := convertBackfillPhotos(ctx, newFFmpegPhotoEncoder(), root, urls, dryRun, &report, logf)
	if dryRun {
		return report, nil
	}
	for _, product := range products {
		updated, err := applyBackfillProduct(ctx, pool, product, results)
		if err != nil {
			return report, err
		}
		if updated {
			report.ProductsUpdated++
		}
	}
	return report, nil
}

func loadBackfillProducts(ctx context.Context, pool *pgxpool.Pool) ([]backfillProduct, error) {
	const q = `select id, account_id, restaurant_id, image_url, gallery
		from cardapio.products
		where image_url like '/uploads/cardapio/%' or gallery::text like '%/uploads/cardapio/%'
		order by account_id, restaurant_id, id`
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]backfillProduct, 0)
	for rows.Next() {
		var item backfillProduct
		var gallery []byte
		if err := rows.Scan(&item.id, &item.accountID, &item.restaurantID, &item.imageURL, &gallery); err != nil {
			return nil, err
		}
		item.gallery = stringsx.DecodeJSONStringSlice(gallery)
		out = append(out, item)
	}
	return out, rows.Err()
}

// convertibleUploadURL aceita so imagens estaticas locais do cardapio ainda nao
// convertidas (GIF fica de fora para nao perder animacao).
func convertibleUploadURL(url string) bool {
	if !strings.HasPrefix(url, "/uploads/cardapio/") || strings.Contains(url, "..") {
		return false
	}
	switch strings.ToLower(filepath.Ext(url)) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	default:
		return false
	}
}

// backfillTargets devolve os caminhos/urls das versoes AVIF ao lado do arquivo.
func backfillTargets(url string) (siteURL, thumbURL string) {
	base := strings.TrimSuffix(url, filepath.Ext(url))
	base = strings.TrimSuffix(base, ".orig")
	return base + ".avif", base + ".thumb.avif"
}

func uploadPath(root, url string) (string, bool) {
	cleanRoot := filepath.Clean(filepath.Join(root, "cardapio"))
	full := filepath.Clean(filepath.Join(root, strings.TrimPrefix(url, "/uploads/")))
	return full, strings.HasPrefix(full, cleanRoot+string(filepath.Separator))
}

func nonEmptyFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

func convertBackfillPhotos(ctx context.Context, encoder photoEncoder, root string, urls map[string]struct{}, dryRun bool, report *PhotoBackfillReport, logf func(string, ...any)) map[string]backfillPhoto {
	results := make(map[string]backfillPhoto, len(urls))
	var mu sync.Mutex
	jobs := make(chan string)
	var wg sync.WaitGroup
	for range productPhotoConcurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range jobs {
				result, outcome := convertBackfillPhoto(ctx, encoder, root, url, dryRun)
				mu.Lock()
				results[url] = result
				switch outcome {
				case "converted":
					report.Converted++
				case "reused":
					report.Reused++
				case "skipped":
					report.Skipped++
				default:
					report.Failed++
				}
				report.BytesBefore += result.originalBytes
				if site, _ := backfillTargets(url); result.ok {
					if sitePath, ok := uploadPath(root, site); ok {
						if info, err := os.Stat(sitePath); err == nil {
							report.BytesAfter += info.Size()
						}
					}
				}
				mu.Unlock()
				if logf != nil && outcome != "converted" && outcome != "reused" {
					logf("cardapio_photo_backfill", "url", url, "outcome", outcome)
				}
			}
		}()
	}
	for url := range urls {
		jobs <- url
	}
	close(jobs)
	wg.Wait()
	return results
}

func convertBackfillPhoto(ctx context.Context, encoder photoEncoder, root, url string, dryRun bool) (backfillPhoto, string) {
	src, ok := uploadPath(root, url)
	if !ok {
		return backfillPhoto{}, "skipped"
	}
	info, err := os.Stat(src)
	if err != nil || info.Size() == 0 {
		return backfillPhoto{}, "missing"
	}
	siteURL, thumbURL := backfillTargets(url)
	sitePath, _ := uploadPath(root, siteURL)
	thumbPath, _ := uploadPath(root, thumbURL)
	result := backfillPhoto{newURL: siteURL, thumbURL: thumbURL, originalBytes: info.Size()}
	if nonEmptyFile(sitePath) && nonEmptyFile(thumbPath) {
		result.ok = true
		return result, "reused"
	}
	if dryRun {
		return result, "converted"
	}
	dir := filepath.Dir(src)
	base := strings.TrimSuffix(filepath.Base(sitePath), ".avif")
	if _, _, err := convertProductPhoto(ctx, encoder, dir, src, base); err != nil {
		return result, "failed"
	}
	result.ok = true
	return result, "converted"
}

// applyBackfillProduct troca image_url/galeria pelas versoes convertidas e
// registra as fotos (miniatura da listagem + lembrete de originais grandes).
func applyBackfillProduct(ctx context.Context, pool *pgxpool.Pool, product backfillProduct, results map[string]backfillPhoto) (bool, error) {
	swap := func(url string) string {
		if result, ok := results[url]; ok && result.ok {
			return result.newURL
		}
		return url
	}
	imageURL := swap(product.imageURL)
	gallery := make([]string, len(product.gallery))
	changed := imageURL != product.imageURL
	for i, url := range product.gallery {
		gallery[i] = swap(url)
		changed = changed || gallery[i] != url
	}
	if !changed {
		return false, nil
	}
	galleryJSON, err := json.Marshal(gallery)
	if err != nil {
		return false, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `update cardapio.products
		set image_url = $1, gallery = $2::jsonb, updated_at = now()
		where id = $3 and account_id = $4`,
		imageURL, string(galleryJSON), product.id, product.accountID); err != nil {
		return false, err
	}
	for _, url := range append([]string{product.imageURL}, product.gallery...) {
		result, ok := results[url]
		if !ok || !result.ok {
			continue
		}
		if _, err := tx.Exec(ctx, `insert into cardapio.product_photos
				(account_id, restaurant_id, url, thumb_url, original_url, original_bytes, source)
			values ($1, $2, $3, $4, $5, $6, 'backfill')
			on conflict (account_id, url) do nothing`,
			product.accountID, product.restaurantID, result.newURL, result.thumbURL, url, result.originalBytes); err != nil {
			return false, err
		}
	}
	return true, tx.Commit(ctx)
}
