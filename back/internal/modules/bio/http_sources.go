package bio

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterSourceRoutes monta os endpoints de fontes de produto do painel
// (JWT + escopo da account, gateados por modulo no Chain como o resto de
// /v1/bio). accountID vem do Principal/X-Account-Id, nunca do body.
func RegisterSourceRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuth(h) }
	mux.Handle("GET /v1/bio/sources", wrap(handleSources(svc)))
	mux.Handle("GET /v1/bio/sources/{type}/facets", wrap(handleSourceFacets(svc)))
	mux.Handle("GET /v1/bio/sources/{type}/resolve", wrap(handleSourceResolve(svc)))
}

// handleSourceResolve resolve os slides da fonte para a PREVIA do editor. Filtros
// na query (category, campaigns=csv, tipo, limit, link, whatsapp). Fonte
// desconhecida => 404.
func handleSourceResolve(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, accountID, ok := principalScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		q := r.URL.Query()
		limit := 0
		if n, err := strconv.Atoi(strings.TrimSpace(q.Get("limit"))); err == nil && n > 0 {
			limit = n
		}
		campaigns := []string{}
		for _, c := range strings.Split(q.Get("campaigns"), ",") {
			if v := strings.TrimSpace(c); v != "" {
				campaigns = append(campaigns, v)
			}
		}
		filter := SourceFilter{
			Category:  strings.TrimSpace(q.Get("category")),
			Campaigns: campaigns,
			Tipo:      strings.TrimSpace(q.Get("tipo")),
			Limit:     limit,
		}
		slides, err := svc.ResolvePreview(
			r.Context(), accountID, strings.TrimSpace(r.PathValue("type")),
			filter, strings.TrimSpace(q.Get("link")), strings.TrimSpace(q.Get("whatsapp")),
		)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"slides": slides})
	}
}

// handleSources lista as fontes de produto disponiveis para a account.
func handleSources(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, accountID, ok := principalScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		sources := svc.Sources(r.Context(), accountID)
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"sources": sources})
	}
}

// handleSourceFacets devolve categorias/campanhas/tipos distintos da fonte para
// popular os selects do editor. Fonte desconhecida => 404.
func handleSourceFacets(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, accountID, ok := principalScope(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		sourceType := strings.TrimSpace(r.PathValue("type"))
		facets, err := svc.Facets(r.Context(), accountID, sourceType)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, facets)
	}
}
