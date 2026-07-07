package core

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterAppearanceRoutes monta os endpoints da aparência GLOBAL da plataforma
// (tema visual + overrides + toggles de Page Header). Config de nível
// plataforma, desacoplada do módulo queue.
//
//	GET /v1/platform/appearance — RequireAuth (todos os autenticados leem; pinta o painel)
//	PUT /v1/platform/appearance — RequireAuth + platform_admin (só platform_admin escreve)
func RegisterAppearanceRoutes(mux *http.ServeMux, svc *AppearanceService, middleware *auth.Middleware) {
	mux.Handle("GET /v1/platform/appearance", middleware.RequireAuth(handleGetAppearance(svc)))
	mux.Handle("PUT /v1/platform/appearance", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requirePlatformAdmin(w, r) {
			return
		}
		handlePutAppearance(svc)(w, r)
	})))
}

// putAppearanceRequest é o body do PUT: { "appearance": <Appearance> }.
type putAppearanceRequest struct {
	Appearance Appearance `json:"appearance"`
}

func handleGetAppearance(svc *AppearanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := svc.GetAppearance(r.Context())
		if err != nil {
			writePlatformSettingsError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

func handlePutAppearance(svc *AppearanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input putAppearanceRequest
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}

		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		resp, err := svc.SaveAppearance(r.Context(), input.Appearance, principal.UserID)
		if err != nil {
			writePlatformSettingsError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}
