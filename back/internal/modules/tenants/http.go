package tenants

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type listResponse struct {
	Tenants []TenantView `json:"tenants"`
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("GET /v1/tenants", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		tenants, err := service.ListAccessible(r.Context(), principal)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao carregar os tenants.")
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, listResponse{
			Tenants: tenants,
		})
	})))
}
