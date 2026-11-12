package cardapio

import (
	"context"
	"strings"
)

// SaveProductPhoto grava a foto de um produto (aba Produtos): guarda o original,
// gera o AVIF do site + miniatura e registra a foto para a listagem e para o
// lembrete de originais grandes. O restaurante e validado na account (404 fora).
func (s *Service) SaveProductPhoto(ctx context.Context, accountID, restaurantID, fileName, contentType string, content []byte) (ProductPhoto, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return ProductPhoto{}, mapStoreErr(err)
	}
	if s.cfg.Photos == nil {
		return ProductPhoto{}, ErrInvalidMedia
	}
	photo, err := s.cfg.Photos.SaveProductPhoto(ctx, accountID, fileName, contentType, content)
	if err != nil {
		return ProductPhoto{}, err
	}
	return s.store.InsertProductPhoto(ctx, accountID, restaurantID, photo, "upload")
}

// ListLargeProductPhotos devolve os originais grandes (>= 15MB) ainda nao
// revisados do restaurante. O gate de admin fica no handler.
func (s *Service) ListLargeProductPhotos(ctx context.Context, accountID, restaurantID string) ([]ProductPhotoReview, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return nil, mapStoreErr(err)
	}
	return s.store.ListLargeProductPhotos(ctx, accountID, restaurantID, largeProductPhotoBytes)
}

// MarkProductPhotoReviewed tira o original do lembrete do admin.
func (s *Service) MarkProductPhotoReviewed(ctx context.Context, accountID, restaurantID, photoID string) error {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return mapStoreErr(err)
	}
	if strings.TrimSpace(photoID) == "" {
		return ErrNotFound
	}
	return s.store.MarkProductPhotoReviewed(ctx, accountID, restaurantID, strings.TrimSpace(photoID))
}
