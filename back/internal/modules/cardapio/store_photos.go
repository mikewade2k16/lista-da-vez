package cardapio

import (
	"context"
	"time"
)

// ProductPhotoReview e um original grande aguardando revisao do admin.
type ProductPhotoReview struct {
	ID            string    `json:"id"`
	URL           string    `json:"url"`
	ThumbURL      string    `json:"thumbUrl"`
	OriginalURL   string    `json:"originalUrl"`
	OriginalBytes int64     `json:"originalBytes"`
	ProductNames  []string  `json:"productNames"`
	CreatedAt     time.Time `json:"createdAt"`
}

// InsertProductPhoto registra a foto convertida. Idempotente por (account, url):
// reprocessar a mesma url apenas atualiza miniatura/original.
func (s *Store) InsertProductPhoto(ctx context.Context, accountID, restaurantID string, photo ProductPhoto, source string) (ProductPhoto, error) {
	const q = `insert into cardapio.product_photos
			(account_id, restaurant_id, url, thumb_url, original_url, original_bytes, source)
		values ($1, nullif($2, '')::uuid, $3, $4, $5, $6, $7)
		on conflict (account_id, url) do update
			set thumb_url = excluded.thumb_url,
				original_url = excluded.original_url,
				original_bytes = excluded.original_bytes
		returning id`
	err := s.pool.QueryRow(ctx, q, accountID, restaurantID, photo.URL, photo.ThumbURL,
		photo.OriginalURL, photo.OriginalBytes, source).Scan(&photo.ID)
	return photo, err
}

// ListLargeProductPhotos lista originais acima de minBytes ainda nao revisados,
// com os produtos do restaurante que usam a foto (mesma account).
func (s *Store) ListLargeProductPhotos(ctx context.Context, accountID, restaurantID string, minBytes int64) ([]ProductPhotoReview, error) {
	const q = `select ph.id, ph.url, ph.thumb_url, ph.original_url, ph.original_bytes, ph.created_at,
			coalesce(array_agg(p.name order by p.name) filter (where p.id is not null), '{}')
		from cardapio.product_photos ph
		left join cardapio.products p
			on p.account_id = ph.account_id and p.restaurant_id = ph.restaurant_id and p.image_url = ph.url
		where ph.account_id = $1 and ph.restaurant_id = $2
			and ph.original_bytes >= $3 and ph.reviewed_at is null
		group by ph.id
		order by ph.created_at desc
		limit 200`
	rows, err := s.pool.Query(ctx, q, accountID, restaurantID, minBytes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProductPhotoReview, 0)
	for rows.Next() {
		var item ProductPhotoReview
		if err := rows.Scan(&item.ID, &item.URL, &item.ThumbURL, &item.OriginalURL,
			&item.OriginalBytes, &item.CreatedAt, &item.ProductNames); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// MarkProductPhotoReviewed marca o original como tratado (sai do lembrete).
func (s *Store) MarkProductPhotoReviewed(ctx context.Context, accountID, restaurantID, id string) error {
	const q = `update cardapio.product_photos set reviewed_at = now()
		where id::text = $1 and account_id = $2 and restaurant_id = $3 and reviewed_at is null`
	tag, err := s.pool.Exec(ctx, q, id, accountID, restaurantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
