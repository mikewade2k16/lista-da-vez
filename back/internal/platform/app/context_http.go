package app

import (
	"errors"
	"net/http"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tenants"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type meContextResponse struct {
	User      auth.UserView     `json:"user"`
	Principal principalDTO      `json:"principal"`
	Context   authenticatedView `json:"context"`
}

type principalDTO struct {
	UserID              string    `json:"userId"`
	Role                auth.Role `json:"role"`
	TenantID            string    `json:"tenantId,omitempty"`
	StoreIDs            []string  `json:"storeIds,omitempty"`
	Permissions         []string  `json:"permissions"`
	PermissionsResolved bool      `json:"permissionsResolved"`
	ExpiresAt           time.Time `json:"expiresAt"`
}

type authenticatedView struct {
	ActiveTenantID string               `json:"activeTenantId,omitempty"`
	ActiveStoreID  string               `json:"activeStoreId,omitempty"`
	Tenants        []tenants.TenantView `json:"tenants"`
	Stores         []stores.StoreView   `json:"stores"`
}

func registerContextRoutes(
	mux *http.ServeMux,
	authService *auth.Service,
	authMiddleware *auth.Middleware,
	tenantService *tenants.Service,
	storeService *stores.Service,
) {
	mux.Handle("GET /v1/me/context", authMiddleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		user, err := authService.CurrentUser(r.Context(), principal)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrUnauthorized), errors.Is(err, auth.ErrUserInactive):
				httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Sessao invalida.")
			default:
				httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao carregar o usuario autenticado.")
			}
			return
		}

		tenantViews, err := tenantService.ListAccessible(r.Context(), principal, tenants.ListInput{})
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao carregar o contexto de tenant.")
			return
		}

		storeViews, err := storeService.ListAccessible(r.Context(), principal, stores.ListInput{})
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao carregar o contexto de loja.")
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, meContextResponse{
			User: user,
			Principal: principalDTO{
				UserID:              principal.UserID,
				Role:                principal.Role,
				TenantID:            principal.TenantID,
				StoreIDs:            append([]string{}, principal.StoreIDs...),
				Permissions:         append([]string{}, principal.Permissions...),
				PermissionsResolved: principal.PermissionsResolved,
				ExpiresAt:           principal.ExpiresAt,
			},
			Context: authenticatedView{
				ActiveTenantID: tenants.ResolveDefaultActiveTenantID(principal, tenantViews),
				ActiveStoreID:  stores.ResolveDefaultActiveStoreID(principal, storeViews),
				Tenants:        tenantViews,
				Stores:         storeViews,
			},
		})
	})))
}
