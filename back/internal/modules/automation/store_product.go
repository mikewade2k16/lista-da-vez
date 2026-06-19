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
// linha do ERP (queue.erp_item_current, mesmo tenant), preferindo a com preco > 0 — porque
// na pratica (Perola) o site.products vem com nome generico e preco 0, e o ERP tem o nome
// real, a marca e o preco (price_cents -> reais). O match e' por PRIMEIRO SEGMENTO do code: o
// code do site pode ser multi-parte ("368145_360856"), entao casa sku == split_part(code,'_',1)
// (igualdade ESCALAR, ponto no indice; cobre ~511/773 vs ~378 no code inteiro). Performance: a PK
// do ERP e' (tenant_id, store_id, sku), entao buscar (tenant_id, sku) SEM store varria os ~360k
// itens do tenant por lookup (query ~8s). A migration 0165 adiciona o indice (tenant_id, sku) ->
// ~60ms. Por isso usamos a tabela real queue.erp_item_current (onde o indice vive), nao a view.
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
	// O site.products as vezes tem varias linhas do MESMO produto (variantes de codigo);
	// dedup por nome exibido (row_number), mantendo a melhor (com preco e imagem).
	const query = `select name, code, price, brand, image from (
			select
				coalesce(nullif(e.name, ''), p.name) as name,
				coalesce(p.code, '') as code,
				round(coalesce(e.price_cents, 0)::numeric / 100.0, 2)::float8 as price,
				case when e.brandname ~ '^[0-9]+$' then '' else coalesce(e.brandname, '') end as brand,
				coalesce(p.image, '') as image,
				row_number() over (
					partition by lower(coalesce(nullif(e.name, ''), p.name))
					order by (e.price_cents > 0) desc, e.price_cents desc, (coalesce(p.image, '') <> '') desc
				) as rn
			from site.products p
			left join lateral (
				select name, price_cents, brandname
				from queue.erp_item_current
				where tenant_id = p.account_id::uuid and sku = split_part(p.code, '_', 1)
				order by (price_cents > 0) desc, price_cents desc
				limit 1
			) e on true
			where p.account_id = $1::uuid
			  and p.status = 'active'
			  and p.is_active = true
			  and (coalesce(p.name, '') || ' ' || coalesce(e.name, '') || ' ' || coalesce(e.brandname, '')) ilike all($2::text[])
		) t
		where t.rn = 1
		order by (t.price > 0) desc, lower(t.name) asc
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

// SampleSiteProducts devolve uma AMOSTRA de produtos reais da account (SEM filtro de
// nome), escopada por account_id (multi-tenant, igual SearchSiteProducts). Usada no
// modo "listar/sugerir" do Omni Chat: quando o usuario pede para ver o catalogo ou
// sugerir algo sem especificar um produto, ou quando uma busca especifica nao acha
// nada (fallback). So traz itens ENRIQUECIDOS pelo ERP (price > 0 => nome real do ERP
// + preco; evita mostrar item com nome = codigo) e com imagem na frente, deduplicados
// por nome. Ordem ALEATORIA (random()) para VARIAR a cada chamada — senao o bot ficaria
// repetindo sempre os mesmos itens em pedidos genericos/sugestoes. Mesmo enrich/dedup da busca.
func (s *Store) SampleSiteProducts(ctx context.Context, accountID string, limit int) ([]ProductHit, error) {
	const query = `select name, code, price, brand, image from (
			select
				coalesce(nullif(e.name, ''), p.name) as name,
				coalesce(p.code, '') as code,
				round(coalesce(e.price_cents, 0)::numeric / 100.0, 2)::float8 as price,
				case when e.brandname ~ '^[0-9]+$' then '' else coalesce(e.brandname, '') end as brand,
				coalesce(p.image, '') as image,
				row_number() over (
					partition by lower(coalesce(nullif(e.name, ''), p.name))
					order by (e.price_cents > 0) desc, e.price_cents desc, (coalesce(p.image, '') <> '') desc
				) as rn
			from site.products p
			left join lateral (
				select name, price_cents, brandname
				from queue.erp_item_current
				where tenant_id = p.account_id::uuid and sku = split_part(p.code, '_', 1)
				order by (price_cents > 0) desc, price_cents desc
				limit 1
			) e on true
			where p.account_id = $1::uuid
			  and p.status = 'active'
			  and p.is_active = true
		) t
		where t.rn = 1 and t.price > 0
		order by (t.image <> '') desc, random()
		limit $2`
	rows, err := s.pool.Query(ctx, query, accountID, limit)
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
