package automation

import (
	"context"
	"encoding/json"
	"strings"
)

// GetSources le a config de fontes de produto do settings jsonb da automacao.
// Defaults (catalogEnabled=false, siteUrls=[]) quando a chave nao existe.
func (s *Store) GetSources(ctx context.Context, automationID string) (SourcesView, error) {
	const q = `select coalesce(settings -> $2, '{}'::jsonb)::text
		from automation.automations
		where id = $1`
	var raw string
	if err := s.pool.QueryRow(ctx, q, automationID, sourcesSettingsKey).Scan(&raw); err != nil {
		return SourcesView{}, err
	}
	view := SourcesView{SiteURLs: []string{}}
	if raw != "" && raw != "null" {
		if err := json.Unmarshal([]byte(raw), &view); err != nil {
			return SourcesView{}, err
		}
	}
	if view.SiteURLs == nil {
		view.SiteURLs = []string{}
	}
	return view, nil
}

// SetSources grava a config de fontes no settings jsonb da automacao (merge na chave
// "sources", preservando as demais chaves do jsonb).
func (s *Store) SetSources(ctx context.Context, automationID string, view SourcesView) (SourcesView, error) {
	if view.SiteURLs == nil {
		view.SiteURLs = []string{}
	}
	payload, err := json.Marshal(view)
	if err != nil {
		return SourcesView{}, err
	}
	const q = `update automation.automations
		set settings = jsonb_set(coalesce(settings, '{}'::jsonb), array[$2], $3::jsonb, true),
		    updated_at = now()
		where id = $1`
	if _, err := s.pool.Exec(ctx, q, automationID, sourcesSettingsKey, string(payload)); err != nil {
		return SourcesView{}, err
	}
	return view, nil
}

// SearchSiteProducts faz a busca ESTREITA e escopada por account na tabela do modulo
// site (site.products), ENRIQUECIDA pelo ERP. Leitura escopada por account_id e
// obrigatoria (multi-tenant): account_id e resolvido pela sessao/context token, NUNCA
// do query do n8n. Espelha o "ativo" do proprio site (status='active' AND is_active=true).
//
// Base = site.products (lista + imagem). Para cada produto, um LEFT JOIN LATERAL pega 1
// linha do ERP (public.erp_item_current por sku == code, mesmo tenant), preferindo a com
// preco > 0 — porque na pratica (Perola) o site.products vem com nome generico e preco 0,
// e o ERP tem o nome real, a marca e o preco (price_cents -> reais).
//
// Busca multi-palavra: cada token de q vira um padrao %token% que precisa casar (ilike
// all) no "haystack" = nome do site + nome do ERP + marca. Assim "relogio seiko" casa
// mesmo quando o nome do site e "Relogio 292299" e a marca SEIKO so existe no ERP.
func (s *Store) SearchSiteProducts(ctx context.Context, accountID, q string, limit int) ([]ProductHit, error) {
	tokens := strings.Fields(q)
	if len(tokens) == 0 {
		return []ProductHit{}, nil
	}
	patterns := make([]string, len(tokens))
	for i, t := range tokens {
		patterns[i] = "%" + t + "%"
	}
	const query = `select
			coalesce(nullif(e.name, ''), p.name) as name,
			coalesce(p.code, '') as code,
			round(coalesce(e.price_cents, 0)::numeric / 100.0, 2)::float8 as price,
			coalesce(e.brandname, '') as brand,
			coalesce(p.image, '') as image
		from site.products p
		left join lateral (
			select name, price_cents, brandname
			from public.erp_item_current
			where tenant_id = p.account_id::uuid and sku = p.code
			order by (price_cents > 0) desc, price_cents desc
			limit 1
		) e on true
		where p.account_id = $1::uuid
		  and p.status = 'active'
		  and p.is_active = true
		  and (coalesce(p.name, '') || ' ' || coalesce(e.name, '') || ' ' || coalesce(e.brandname, '')) ilike all($2::text[])
		order by (e.price_cents > 0) desc, lower(coalesce(nullif(e.name, ''), p.name)) asc
		limit $3`
	rows, err := s.pool.Query(ctx, query, accountID, patterns, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := make([]ProductHit, 0, limit)
	for rows.Next() {
		var h ProductHit
		if err := rows.Scan(&h.Name, &h.Code, &h.Price, &h.Brand, &h.Image); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}
