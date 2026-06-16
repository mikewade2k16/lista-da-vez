package automation

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// registerSourcesRoutes monta os endpoints de painel de fontes de produto (M5).
// AccountID vem do principal (X-Account-Id); o gating por modulo roda no Chain.
func registerSourcesRoutes(mux *http.ServeMux, svc *Service, wrap func(http.HandlerFunc) http.Handler) {
	mux.Handle("GET /v1/automation/sources", wrap(handleSourcesGet(svc)))
	mux.Handle("PUT /v1/automation/sources", wrap(handleSourcesPut(svc)))
}

func handleSourcesGet(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		view, err := svc.Sources(r.Context(), accountID)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao carregar as fontes.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

func handleSourcesPut(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := accountIDFromContext(r)
		if !ok {
			writeNoAccount(w, r)
			return
		}
		var body struct {
			CatalogEnabled bool     `json:"catalogEnabled"`
			SiteURLs       []string `json:"siteUrls"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		view, err := svc.SetSources(r.Context(), accountID, body.CatalogEnabled, sanitizeURLs(body.SiteURLs))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao salvar as fontes.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// sanitizeURLs descarta entradas vazias (apos trim) da lista de URLs do site.
func sanitizeURLs(urls []string) []string {
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if t := strings.TrimSpace(u); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// registerCatalogToolRoute monta a tool runtime de catalogo (M5), consumida pelo n8n.
// Auth por token de servico; account resolvido pela sessao, NUNCA do query.
func registerCatalogToolRoute(mux *http.ServeMux, svc *Service, token string) {
	mux.Handle("GET /v1/runtime/automation/tools/catalog", handleRuntimeCatalogTool(svc, token))
}

func handleRuntimeCatalogTool(svc *Service, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := runtimeAuth(w, r, token)
		if !ok {
			return
		}
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		hits, err := svc.SearchCatalog(r.Context(), session, query)
		if err != nil {
			writeRuntimeErr(w, r, err, "Falha ao buscar no catalogo.")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, hits)
	}
}
