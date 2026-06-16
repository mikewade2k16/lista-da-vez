package automation

import (
	"context"
	"encoding/json"
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
// site (site.products). Leitura escopada por account_id e obrigatoria (multi-tenant):
// account_id e resolvido pela sessao, NUNCA do query do n8n. Espelha o "ativo" do
// proprio site (status='active' AND is_active=true). LIMIT 5; projecao lean.
func (s *Store) SearchSiteProducts(ctx context.Context, accountID, q string, limit int) ([]ProductHit, error) {
	const query = `select p.name, coalesce(p.code, ''), coalesce(p.price, 0)
		from site.products p
		where p.account_id = $1::uuid
		  and p.status = 'active'
		  and p.is_active = true
		  and p.name ilike '%' || $2 || '%'
		order by lower(p.name) asc
		limit $3`
	rows, err := s.pool.Query(ctx, query, accountID, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := make([]ProductHit, 0, limit)
	for rows.Next() {
		var h ProductHit
		if err := rows.Scan(&h.Name, &h.Code, &h.Price); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}
