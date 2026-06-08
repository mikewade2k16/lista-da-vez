package core

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterAdminRoutes monta os endpoints /v1/admin/accounts* no mux.
// Todas as rotas exigem papel platform_admin — verificado em requirePlatformAdmin.
func RegisterAdminRoutes(mux *http.ServeMux, svc *AdminService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !requirePlatformAdmin(w, r) {
				return
			}
			h(w, r)
		}))
	}

	mux.Handle("GET /v1/admin/accounts", wrap(handleListAccounts(svc)))
	mux.Handle("POST /v1/admin/accounts", wrap(handleCreateAccount(svc)))
	mux.Handle("GET /v1/admin/accounts/{id}", wrap(handleGetAccount(svc)))
	mux.Handle("PATCH /v1/admin/accounts/{id}", wrap(handleUpdateAccount(svc)))
	mux.Handle("DELETE /v1/admin/accounts/{id}", wrap(handleDeleteAccount(svc)))
	mux.Handle("GET /v1/admin/accounts/{id}/modules", wrap(handleGetModules(svc)))
	mux.Handle("PUT /v1/admin/accounts/{id}/modules", wrap(handleSetModules(svc)))
	mux.Handle("GET /v1/admin/accounts/{id}/stores", wrap(handleGetStores(svc)))
	mux.Handle("PUT /v1/admin/accounts/{id}/stores", wrap(handleSetStorePricing(svc)))
	mux.Handle("POST /v1/admin/accounts/{id}/webhook/rotate", wrap(handleRotateWebhook(svc)))
}

func requirePlatformAdmin(w http.ResponseWriter, r *http.Request) bool {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok || principal.Role != auth.RolePlatformAdmin {
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Esta rota exige papel platform_admin.")
		return false
	}
	return true
}

// ============================================================================
// Handlers
// ============================================================================

func handleListAccounts(svc *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		perPage, _ := strconv.Atoi(q.Get("perPage"))

		filter := AdminListFilter{
			Q:              strings.TrimSpace(q.Get("q")),
			Status:         strings.TrimSpace(q.Get("status")),
			OrganizationID: strings.TrimSpace(q.Get("organizationId")),
			Page:           page,
			PerPage:        perPage,
		}

		resp, err := svc.ListAccounts(r.Context(), filter)
		if err != nil {
			writeAdminError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

func handleGetAccount(svc *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		account, err := svc.GetAccount(r.Context(), id)
		if err != nil {
			writeAdminError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, account)
	}
}

func handleCreateAccount(svc *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input AdminCreateAccountInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}

		account, err := svc.CreateAccount(r.Context(), input)
		if err != nil {
			writeAdminError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, account)
	}
}

func handleUpdateAccount(svc *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var input AdminUpdateAccountInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}

		account, err := svc.UpdateAccount(r.Context(), id, input)
		if err != nil {
			writeAdminError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, account)
	}
}

func handleDeleteAccount(svc *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := svc.DeleteAccount(r.Context(), id); err != nil {
			writeAdminError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleGetModules(svc *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		resp, err := svc.GetModules(r.Context(), id)
		if err != nil {
			writeAdminError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

func handleSetModules(svc *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var input AdminSetModulesInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}

		resp, err := svc.SetModules(r.Context(), id, input)
		if err != nil {
			writeAdminError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

func handleGetStores(svc *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		resp, err := svc.GetStores(r.Context(), id)
		if err != nil {
			writeAdminError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

func handleSetStorePricing(svc *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var input AdminSetStorePricingInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}

		resp, err := svc.SetStorePricing(r.Context(), id, input)
		if err != nil {
			writeAdminError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

func handleRotateWebhook(svc *AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		resp, err := svc.RotateWebhook(r.Context(), id)
		if err != nil {
			writeAdminError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

// ============================================================================
// Error dispatcher
// ============================================================================

func writeAdminError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrAccountNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "account_not_found", "Account nao encontrada.")
	case errors.Is(err, ErrAccountSlugConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "slug_conflict", "Ja existe uma account com esse slug.")
	case errors.Is(err, ErrAdminUserNotFound):
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "user_not_found",
			"Usuario com o e-mail informado nao existe em core.users. Crie o usuario primeiro via convite.")
	case IsValidationError(err):
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "validation_error", err.Error())
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro interno ao processar a requisicao.")
	}
}
