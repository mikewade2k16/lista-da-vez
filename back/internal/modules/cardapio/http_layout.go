package cardapio

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterLayoutRoutes monta as rotas autenticadas do layout de secoes (painel /
// Opcao B). A leitura PUBLICA (GET /v1/public/restaurants/{slug}/layout) fica em
// http_public.go. Mesmo auth dos demais /v1/cardapio/* (JWT + scopedAccountID).
func RegisterLayoutRoutes(mux *http.ServeMux, svc *Service, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(h)
	}
	mux.Handle("GET /v1/cardapio/restaurants/{id}/layout", wrap(handleGetLayout(svc)))
	mux.Handle("PUT /v1/cardapio/restaurants/{id}/layout", wrap(handlePutLayout(svc)))
	mux.Handle("POST /v1/cardapio/restaurants/{id}/layout/publish", wrap(handlePublishLayout(svc)))
}

func handleGetLayout(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		layout, version, err := svc.GetDraftLayout(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.Header().Set("ETag", etagOf(version))
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"layout": layout, "version": version})
	}
}

func handlePutLayout(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxLayoutBytes+1)
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) == 0 {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
			return
		}
		layout, version, err := svc.SaveDraftLayout(r.Context(), accountID, r.PathValue("id"), body, parseIfMatch(r))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.Header().Set("ETag", etagOf(version))
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"layout": layout, "version": version})
	}
}

func handlePublishLayout(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, _, err := scopedAccountID(r, false)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		layout, version, err := svc.PublishLayout(r.Context(), accountID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, r, err)
			return
		}
		w.Header().Set("ETag", etagOf(version))
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"layout": layout, "version": version})
	}
}

// parseIfMatch le o header If-Match como a version esperada (ex.: "12" ou v12).
// Ausente/ invalido => nil (sem checagem de concorrencia).
func parseIfMatch(r *http.Request) *int64 {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	raw = strings.Trim(raw, "\"")
	raw = strings.TrimPrefix(raw, "v")
	if raw == "" {
		return nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

// etagOf formata a version como ETag opaco (string entre aspas).
func etagOf(version int64) string {
	return "\"" + strconv.FormatInt(version, 10) + "\""
}
