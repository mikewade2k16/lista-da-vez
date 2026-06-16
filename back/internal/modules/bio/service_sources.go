package bio

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
)

// ============================================================================
// B7 — Fontes de produto: catalogo, facets e resolucao no publico
// ============================================================================

// sourceLabels mapeia o type da fonte para o rotulo exibido no editor.
var sourceLabels = map[string]string{
	SourceTypeSiteProducts: "Produtos do site",
}

// Sources devolve as fontes de produto disponiveis para a account
// (GET /v1/bio/sources). Lista as fontes registradas no Service em ordem
// estavel (alfabetica por type). MVP: apenas site_products.
func (s *Service) Sources(_ context.Context, _ string) []SourceInfo {
	types := make([]string, 0, len(s.sources))
	for t := range s.sources {
		types = append(types, t)
	}
	sort.Strings(types)

	out := make([]SourceInfo, 0, len(types))
	for _, t := range types {
		label := sourceLabels[t]
		if label == "" {
			label = t
		}
		out = append(out, SourceInfo{Type: t, Label: label, Available: true})
	}
	return out
}

// Facets devolve os valores distintos da fonte para a account (popula os selects
// do editor). Fonte desconhecida => ErrNotFound (404, nao vaza). accountID e o
// escopo ja resolvido contra o Principal pelo handler.
func (s *Service) Facets(ctx context.Context, accountID, sourceType string) (SourceFacets, error) {
	src, ok := s.sources[strings.TrimSpace(sourceType)]
	if !ok {
		return SourceFacets{}, ErrNotFound
	}
	if strings.TrimSpace(accountID) == "" {
		return SourceFacets{Categories: []string{}, Campaigns: []string{}, Tipos: []string{}}, nil
	}
	return src.Facets(ctx, accountID)
}

// ResolvePreview resolve os slides de uma fonte para a PREVIA do editor (sem
// depender de bio publicada): os filtros vêm direto da query. Fonte desconhecida
// => ErrNotFound. accountID e o escopo ja resolvido contra o Principal.
func (s *Service) ResolvePreview(ctx context.Context, accountID, sourceType string, filter SourceFilter, link, whatsapp string) ([]ResolvedSlide, error) {
	src, ok := s.sources[strings.TrimSpace(sourceType)]
	if !ok {
		return nil, ErrNotFound
	}
	if strings.TrimSpace(accountID) == "" {
		return []ResolvedSlide{}, nil
	}
	return src.Resolve(ctx, accountID, filter, link, whatsapp)
}

// ============================================================================
// Publico (resolve a fonte do slideTop antes de absolutizar a midia)
// ============================================================================

// Public devolve o JSON resolvido (defaults + data_published) com midia
// absolutizada. Quando o slideTop tem uma fonte de produtos (type !=
// manual/ausente), resolve os produtos e injeta em slideTop.slides ANTES de
// absolutizar. Fonte vazia/erro cai nos slides manuais (nao quebra a bio).
// Qualquer falha de lookup/merge vira ErrNotFound (sem vazar existencia).
func (s *Service) Public(ctx context.Context, rawSlug string) (json.RawMessage, error) {
	slug := strings.ToLower(strings.TrimSpace(rawSlug))
	if !slugPattern.MatchString(slug) {
		return nil, ErrNotFound
	}
	published, accountID, err := s.store.PublicLookup(ctx, slug)
	if err != nil {
		return nil, mapNotFound(err)
	}
	defaults, err := s.store.GetDefaults(ctx)
	if err != nil {
		return nil, ErrNotFound
	}
	merged, err := deepMerge(defaults.Data, normalizeRaw(published))
	if err != nil {
		return nil, ErrNotFound
	}
	merged = s.resolveSlideSource(ctx, accountID, merged)
	return absolutizeUploads(merged, s.publicBase)
}

// resolveSlideSource injeta os produtos da fonte em slideTop.slides quando
// slideTop.source.type e uma fonte registrada (!= manual/ausente). Qualquer
// problema (sem fonte, erro na query, zero produtos) DEVOLVE o merged intacto —
// a bio cai nos slides manuais e nunca quebra.
func (s *Service) resolveSlideSource(ctx context.Context, accountID string, merged json.RawMessage) json.RawMessage {
	root, ok := decodeJSON(merged)
	if !ok {
		return merged
	}
	obj, ok := root.(map[string]any)
	if !ok {
		return merged
	}
	slideTop, ok := obj["slideTop"].(map[string]any)
	if !ok {
		return merged
	}
	source, sourceType := parseSlideSource(slideTop["source"])
	if sourceType == "" || sourceType == SourceTypeManual {
		return merged
	}
	src, known := s.sources[sourceType]
	if !known {
		return merged
	}

	whatsapp := whatsappFromMerged(obj)
	slides, err := src.Resolve(ctx, accountID, sourceToFilter(source), source.Link, whatsapp)
	if err != nil || len(slides) == 0 {
		return merged
	}

	slideTop["slides"] = resolvedToAny(slides)
	obj["slideTop"] = slideTop
	out, err := json.Marshal(obj)
	if err != nil {
		return merged
	}
	return out
}

// parseSlideSource le o slideTop.source (any do jsonb) no shape tipado. Devolve
// o type normalizado em minusculas (string vazia quando ausente/invalido).
func parseSlideSource(raw any) (slideTopSource, string) {
	if raw == nil {
		return slideTopSource{}, ""
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return slideTopSource{}, ""
	}
	var src slideTopSource
	if err := json.Unmarshal(encoded, &src); err != nil {
		return slideTopSource{}, ""
	}
	return src, strings.ToLower(strings.TrimSpace(src.Type))
}

func sourceToFilter(src slideTopSource) SourceFilter {
	limit := src.Limit
	if limit < 0 {
		limit = 0
	}
	return SourceFilter{
		Category:  strings.TrimSpace(src.Category),
		Campaigns: src.Campaigns,
		Tipo:      strings.TrimSpace(src.Tipo),
		Limit:     limit,
	}
}

// whatsappFromMerged extrai o numero de WhatsApp da bio (lightbox.whatsappNumber)
// para o link de slide-produto do tipo whatsapp. Ausente => "".
func whatsappFromMerged(obj map[string]any) string {
	lightbox, ok := obj["lightbox"].(map[string]any)
	if !ok {
		return ""
	}
	num, ok := lightbox["whatsappNumber"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(num)
}

// resolvedToAny converte os slides resolvidos no shape que vai para o jsonb
// (src/title/desc/price/href). Mantem o mesmo shape de um BioSlide do front
// (campos vazios ausentes, casando com o que o Lightbox espera). O caminho passa
// pelo absolutizeUploads depois, entao src relativo (ex.: /uploads/...) tambem e
// tratado.
func resolvedToAny(slides []ResolvedSlide) []any {
	out := make([]any, 0, len(slides))
	for _, sl := range slides {
		item := map[string]any{"src": sl.Src}
		if sl.Title != "" {
			item["title"] = sl.Title
		}
		if sl.Desc != "" {
			item["desc"] = sl.Desc
		}
		if sl.Price != "" {
			item["price"] = sl.Price
		}
		if sl.Href != "" {
			item["href"] = sl.Href
		}
		out = append(out, item)
	}
	return out
}
