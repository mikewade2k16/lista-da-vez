package cardapio

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Resolve mapeia um host -> slug do restaurante. Ordem:
//  1. normaliza (lowercase, sem porta, sem www.);
//  2. localhost com DevDefaultSlug configurado => esse slug (se ativo);
//  3. termina em "."+BaseDomain => primeiro rotulo e o slug (se ativo);
//  4. caso contrario => busca em restaurant_domains.
//
// Retorna ErrNotFound quando nada resolve (404 uniforme).
func (s *Service) Resolve(ctx context.Context, host string) (string, error) {
	normalized := normalizeHost(host)
	if normalized == "" {
		return "", ErrNotFound
	}

	if normalized == "localhost" && s.cfg.DevDefaultSlug != "" {
		return s.resolveBySlug(ctx, s.cfg.DevDefaultSlug)
	}

	if base := normalizeHost(s.cfg.BaseDomain); base != "" {
		if normalized == base {
			return "", ErrNotFound
		}
		if strings.HasSuffix(normalized, "."+base) {
			label := strings.TrimSuffix(normalized, "."+base)
			if idx := strings.IndexByte(label, '.'); idx >= 0 {
				label = label[:idx]
			}
			if slug, err := s.resolveBySlug(ctx, label); err == nil {
				return slug, nil
			}
			// subdominio sob o base domain mas sem restaurante => 404 (nao cai
			// para domains, que sao hosts custom).
			return "", ErrNotFound
		}
	}

	slug, err := s.store.resolveHostActive(ctx, normalized)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return slug, nil
}

func (s *Service) resolveBySlug(ctx context.Context, slug string) (string, error) {
	resolved, err := s.store.resolveSlugActive(ctx, normalizeSlug(slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return resolved, nil
}

// PublicMenu monta o cardapio completo (restaurante + categorias ativas +
// produtos disponiveis ja com variations/addons). Imagens absolutizadas.
func (s *Service) PublicMenu(ctx context.Context, slug string) (PublicMenu, error) {
	restaurant, accountID, err := s.loadPublicRestaurant(ctx, slug)
	if err != nil {
		return PublicMenu{}, err
	}
	categories, err := s.store.ListCategories(ctx, accountID, restaurant.ID, true)
	if err != nil {
		return PublicMenu{}, err
	}
	products, err := s.store.ListMenuProducts(ctx, accountID, restaurant.ID)
	if err != nil {
		return PublicMenu{}, err
	}
	zones, err := s.store.ListPublicZones(ctx, accountID, restaurant.ID)
	if err != nil {
		return PublicMenu{}, err
	}
	s.absolutizeRestaurant(&restaurant)
	for i := range products {
		s.absolutizeProduct(&products[i])
	}
	// WS-F: productCount derivado (conta os produtos disponiveis ja carregados por
	// categoria, sem query extra) + absolutiza a foto da categoria, como em
	// produto/restaurante. omitempty no DTO: 0 => ausente (o front deriva).
	counts := make(map[string]int, len(categories))
	for i := range products {
		if products[i].CategoryID != nil {
			counts[*products[i].CategoryID]++
		}
	}
	for i := range categories {
		categories[i].ProductCount = counts[categories[i].ID]
		categories[i].ImageURL = s.absolutize(categories[i].ImageURL)
	}
	return PublicMenu{
		Restaurant:    restaurant,
		Categories:    categories,
		Products:      products,
		DeliveryZones: zones,
	}, nil
}

// PublicProduct monta prato + reviews. 404 se o produto nao existe/indisponivel.
func (s *Service) PublicProduct(ctx context.Context, slug, productSlug string) (PublicProduct, error) {
	restaurant, accountID, err := s.loadPublicRestaurant(ctx, slug)
	if err != nil {
		return PublicProduct{}, err
	}
	product, err := s.store.publicProductBySlug(ctx, accountID, restaurant.ID, normalizeSlug(productSlug))
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicProduct{}, ErrNotFound
	}
	if err != nil {
		return PublicProduct{}, err
	}
	reviews, err := s.store.ListReviewsByProduct(ctx, accountID, product.ID)
	if err != nil {
		return PublicProduct{}, err
	}
	s.absolutizeRestaurant(&restaurant)
	s.absolutizeProduct(&product)
	return PublicProduct{Restaurant: restaurant, Product: product, Reviews: reviews}, nil
}

// RecordEvent grava um evento publico, validando a allowlist e o tamanho do
// context (<= 8KB).
func (s *Service) RecordEvent(ctx context.Context, slug string, in PublicEventInput) error {
	name := strings.TrimSpace(in.Name)
	if !isAllowedEvent(name) {
		return ErrValidation
	}
	if len(in.Context) > 8*1024 {
		return ErrValidation
	}
	restaurant, accountID, err := s.loadPublicRestaurant(ctx, slug)
	if err != nil {
		return err
	}
	return s.store.InsertEvent(ctx, accountID, restaurant.ID, name, strings.TrimSpace(in.SessionID), in.Context)
}

// loadPublicRestaurant resolve slug -> restaurante publico (ativo + account
// ativa + modulo habilitado). 404 uniforme caso contrario. Devolve o accountID.
func (s *Service) loadPublicRestaurant(ctx context.Context, slug string) (Restaurant, string, error) {
	restaurant, accountID, err := s.store.publicRestaurant(ctx, normalizeSlug(slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return Restaurant{}, "", ErrNotFound
	}
	if err != nil {
		return Restaurant{}, "", err
	}
	return restaurant, accountID, nil
}

// absolutizeRestaurant troca caminhos /uploads/* por URL absoluta (logo/banner).
func (s *Service) absolutizeRestaurant(r *Restaurant) {
	r.LogoURL = s.absolutize(r.LogoURL)
	r.BannerURL = s.absolutize(r.BannerURL)
}

// absolutizeProduct troca caminhos /uploads/* em image_url e gallery.
func (s *Service) absolutizeProduct(p *Product) {
	p.ImageURL = s.absolutize(p.ImageURL)
	for i := range p.Gallery {
		p.Gallery[i] = s.absolutize(p.Gallery[i])
	}
}

func (s *Service) absolutize(path string) string {
	base := strings.TrimRight(s.cfg.PublicBaseURL, "/")
	if base == "" || !strings.HasPrefix(path, "/uploads/") {
		return path
	}
	return base + path
}
