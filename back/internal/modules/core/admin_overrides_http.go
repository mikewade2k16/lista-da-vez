package core

import (
	"errors"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterAdminOverridesRoutes monta os endpoints de overrides de permissao por
// usuario por account. NAO reusa o modulo `access` (LEGADO): opera em
// core.user_permission_overrides. Gate: RequireAuth + requireAdminActor (403 a
// quem nao administra nada); o escopo fino por-account fica no service (404 fora
// de escopo). actorUserID SEMPRE do Principal.
func RegisterAdminOverridesRoutes(mux *http.ServeMux, svc *AdminOverridesService, userSvc *AdminUserService, middleware *auth.Middleware) {
	wrap := func(h func(*AdminOverridesService, http.ResponseWriter, *http.Request)) http.Handler {
		return middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !requireAdminActor(userSvc, w, r) {
				return
			}
			h(svc, w, r)
		}))
	}

	mux.Handle("GET /v1/admin/users/{id}/accounts/{accountId}/overrides", wrap(handleGetOverrides))
	mux.Handle("PUT /v1/admin/users/{id}/accounts/{accountId}/overrides", wrap(handleReplaceOverrides))
}

func handleGetOverrides(svc *AdminOverridesService, w http.ResponseWriter, r *http.Request) {
	resp, err := svc.GetOverrides(r.Context(), actorID(r), r.PathValue("id"), r.PathValue("accountId"))
	if err != nil {
		writeAdminOverridesError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

func handleReplaceOverrides(svc *AdminOverridesService, w http.ResponseWriter, r *http.Request) {
	var input ReplaceOverridesInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	resp, err := svc.ReplaceOverrides(r.Context(), actorID(r), r.PathValue("id"), r.PathValue("accountId"), input)
	if err != nil {
		writeAdminOverridesError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

func writeAdminOverridesError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrAccountNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "account_not_found", "Conta nao encontrada ou fora do seu escopo.")
	case errors.Is(err, ErrNotMember):
		httpapi.WriteError(w, r, http.StatusNotFound, "user_not_found", "Usuario nao encontrado nesta conta.")
	case errors.Is(err, ErrInvalidPermission):
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "invalid_permission", "Permissao invalida, de modulo desabilitado ou de escopo de plataforma.")
	case errors.Is(err, ErrInvalidEffect):
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "invalid_effect", "Effect deve ser allow ou deny.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
	}
}
