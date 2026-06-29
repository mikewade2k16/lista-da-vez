package cardapio

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/stringsx"
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

// maxEventContextBytes e o teto do context jsonb por evento (defesa de payload).
const maxEventContextBytes = 8 * 1024

// RecordEvent grava um evento publico singular (legado/compat), validando a allowlist
// e o tamanho do context (<= 8KB). Passa pelo MESMO sanitize do lote (anti-PII), mas
// sem enriquecimento de UA/referer/ip (o caminho singular nao os recebe).
func (s *Service) RecordEvent(ctx context.Context, slug string, in PublicEventInput) error {
	name := strings.TrimSpace(in.Name)
	if !isAllowedEvent(name) {
		return ErrValidation
	}
	if len(in.Context) > maxEventContextBytes {
		return ErrValidation
	}
	restaurant, accountID, err := s.loadPublicRestaurant(ctx, slug)
	if err != nil {
		return err
	}
	clean := sanitizeContext(name, in.Context)
	return s.store.InsertEvent(ctx, accountID, restaurant.ID, name, strings.TrimSpace(in.SessionID), clean)
}

// RecordEventBatch ingere um lote de eventos do front publico (best-effort). Resolve o
// restaurante 1x (account_id pelo slug, nunca do corpo) e carrega os slugs de produto
// 1x. Para cada evento: valida a allowlist + cap de context; sanitiza PII; clampa o
// occurredAt do cliente; promove product_slug (so se pertencer ao restaurante)/
// page_path/dwell_ms/device_id/utm do context; enriquece device_type/browser/os
// (User-Agent), referrer_host e ip_hash (ja hasheado pelo handler). Eventos invalidos
// nao derrubam o lote (rejected++). Persiste tudo via InsertEventsBatch + um
// UpsertSession agregado. created_at e sempre o relogio do servidor (no store).
func (s *Service) RecordEventBatch(ctx context.Context, slug string, in PublicEventBatchInput, userAgent, referer, ipHash string) (accepted, rejected int, err error) {
	restaurant, accountID, err := s.loadPublicRestaurant(ctx, slug)
	if err != nil {
		return 0, 0, err
	}
	slugs, err := s.store.listProductSlugs(ctx, accountID, restaurant.ID)
	if err != nil {
		return 0, 0, err
	}

	deviceType, browser, os := parseUserAgent(userAgent)
	referrerHost := referrerHostOf(referer)
	now := time.Now().UTC()
	batchDeviceID := strings.TrimSpace(in.DeviceID)
	batchSessionID := strings.TrimSpace(in.SessionID)

	rows := make([]eventInsert, 0, len(in.Events))
	session := sessionUpsert{
		AccountID:    accountID,
		RestaurantID: restaurant.ID,
		SessionID:    batchSessionID,
		DeviceID:     batchDeviceID,
		LastSeenAt:   now,
		DeviceType:   deviceType,
		ReferrerHost: referrerHost,
	}

	rejectedUnknown := make(map[string]struct{})
	for i := range in.Events {
		ev := in.Events[i]
		name := strings.TrimSpace(ev.Name)
		if !isAllowedEvent(name) {
			rejected++
			if name != "" {
				rejectedUnknown[name] = struct{}{}
			}
			continue
		}
		if len(ev.Context) > maxEventContextBytes {
			rejected++
			continue
		}
		clean := sanitizeContext(name, ev.Context)
		promo := promoteContext(clean, slugs)

		sessionID := strings.TrimSpace(ev.SessionID)
		if sessionID == "" {
			sessionID = batchSessionID
		}
		deviceID := batchDeviceID
		if promo.deviceID != "" {
			deviceID = promo.deviceID
		}

		row := eventInsert{
			EventID:      strings.TrimSpace(ev.EventID),
			Name:         name,
			SessionID:    sessionID,
			DeviceID:     deviceID,
			OccurredAt:   clampOccurredAt(ev.OccurredAt, now),
			PagePath:     promo.pagePath,
			ProductSlug:  promo.productSlug,
			DeviceType:   deviceType,
			Browser:      browser,
			OS:           os,
			ReferrerHost: referrerHost,
			UTMSource:    promo.utmSource,
			UTMMedium:    promo.utmMedium,
			UTMCampaign:  promo.utmCampaign,
			IPHash:       ipHash,
			DwellMS:      promo.dwellMS,
			Context:      clean,
		}
		rows = append(rows, row)

		// Agregacao da sessao: pageviews conta page_view; landing/utm vem do 1o evento.
		if name == "page_view" {
			session.Pageviews++
		}
		if session.LandingPath == "" && promo.pagePath != "" {
			session.LandingPath = promo.pagePath
		}
		if session.UTMSource == "" {
			session.UTMSource = promo.utmSource
		}
		if session.UTMMedium == "" {
			session.UTMMedium = promo.utmMedium
		}
		if session.UTMCampaign == "" {
			session.UTMCampaign = promo.utmCampaign
		}
		if promo.dwellMS > int(session.DurationMS) {
			session.DurationMS = int64(promo.dwellMS)
		}
	}

	// Drift de allowlist: nome emitido pelo front que o back nao conhece (deploy
	// fora de ordem). Logado server-side para detectar sem depender do front, que e
	// fire-and-forget e nao trata o rejected da resposta.
	if len(rejectedUnknown) > 0 {
		names := make([]string, 0, len(rejectedUnknown))
		for n := range rejectedUnknown {
			names = append(names, n)
		}
		slog.Warn("cardapio: eventos de telemetria com nome fora da allowlist",
			"restaurantId", restaurant.ID, "names", names)
	}

	if len(rows) == 0 {
		return 0, rejected, nil
	}

	accepted, err = s.store.InsertEventsBatch(ctx, accountID, restaurant.ID, rows)
	if err != nil {
		return 0, rejected, err
	}
	session.Events = accepted
	if session.SessionID != "" {
		if err := s.store.UpsertSession(ctx, session); err != nil {
			return accepted, rejected, err
		}
	}
	return accepted, rejected, nil
}

// promotedContext sao os campos derivados do context que viram coluna em events.
type promotedContext struct {
	productSlug string
	pagePath    string
	deviceID    string
	dwellMS     int
	utmSource   string
	utmMedium   string
	utmCampaign string
}

// promoteContext extrai do context (ja sanitizado) os campos desnormalizados em
// colunas. product_slug so e promovido se pertencer ao restaurante (senao "").
func promoteContext(ctx json.RawMessage, slugs map[string]struct{}) promotedContext {
	var obj struct {
		ProductSlug string `json:"productSlug"`
		PagePath    string `json:"pagePath"`
		Route       string `json:"route"`
		DeviceID    string `json:"deviceId"`
		DwellMS     int    `json:"dwellMs"`
		UTMSource   string `json:"utmSource"`
		UTMMedium   string `json:"utmMedium"`
		UTMCampaign string `json:"utmCampaign"`
	}
	if len(ctx) > 0 {
		_ = json.Unmarshal(ctx, &obj)
	}
	out := promotedContext{
		pagePath:    strings.TrimSpace(stringsx.FirstNonEmpty(obj.PagePath, obj.Route)),
		deviceID:    strings.TrimSpace(obj.DeviceID),
		utmSource:   strings.TrimSpace(obj.UTMSource),
		utmMedium:   strings.TrimSpace(obj.UTMMedium),
		utmCampaign: strings.TrimSpace(obj.UTMCampaign),
	}
	if obj.DwellMS > 0 {
		out.dwellMS = obj.DwellMS
	}
	if slug := normalizeSlug(obj.ProductSlug); slug != "" {
		if _, ok := slugs[slug]; ok {
			out.productSlug = slug
		}
	}
	return out
}

// clampOccurredAt parseia o occurredAt do cliente (RFC3339) e o restringe a
// [now-24h, now+5min]. Invalido/vazio => now do servidor. Nunca alimenta histograma de
// hora (isso e created_at do servidor); serve so para ordenar dentro do lote.
func clampOccurredAt(value string, now time.Time) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return now
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return now
	}
	t = t.UTC()
	if min := now.Add(-24 * time.Hour); t.Before(min) {
		return min
	}
	if max := now.Add(5 * time.Minute); t.After(max) {
		return max
	}
	return t
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
