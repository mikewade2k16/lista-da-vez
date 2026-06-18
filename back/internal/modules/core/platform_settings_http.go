package core

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterPlatformSettingsRoutes monta os endpoints da config GLOBAL do menu.
//
//	GET   /v1/platform/menu-layout — RequireAuth (todos os usuários autenticados leem)
//	PATCH /v1/platform/menu-layout — RequireAuth + platform_admin (só platform_admin escreve)
func RegisterPlatformSettingsRoutes(mux *http.ServeMux, svc *PlatformSettingsService, middleware *auth.Middleware) {
	mux.Handle("GET /v1/platform/menu-layout", middleware.RequireAuth(handleGetMenuLayout(svc)))
	mux.Handle("PATCH /v1/platform/menu-layout", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requirePlatformAdmin(w, r) {
			return
		}
		handlePatchMenuLayout(svc)(w, r)
	})))
}

// patchMenuLayoutRequest é o body do PATCH: { "layout": <Layout> }.
type patchMenuLayoutRequest struct {
	Layout MenuLayout `json:"layout"`
}

func handleGetMenuLayout(svc *PlatformSettingsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := svc.GetMenuLayout(r.Context())
		if err != nil {
			writePlatformSettingsError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

func handlePatchMenuLayout(svc *PlatformSettingsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input patchMenuLayoutRequest
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}

		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		resp, err := svc.SaveMenuLayout(r.Context(), input.Layout, principal.UserID)
		if err != nil {
			writePlatformSettingsError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

// writePlatformSettingsError traduz os erros do service para HTTP. Placement
// inválido (validação) → 400, conforme contrato congelado.
func writePlatformSettingsError(w http.ResponseWriter, r *http.Request, err error) {
	if IsValidationError(err) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro interno ao processar a requisicao.")
}
