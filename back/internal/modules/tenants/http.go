package tenants

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type listResponse struct {
	Tenants []TenantView `json:"tenants"`
}

type tenantResponse struct {
	Tenant TenantView `json:"tenant"`
}

type createRequest struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	IsActive *bool  `json:"isActive"`
}

type updateRequest struct {
	Slug     *string `json:"slug"`
	Name     *string `json:"name"`
	IsActive *bool   `json:"isActive"`
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("GET /v1/tenants", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		tenants, err := service.ListAccessible(r.Context(), principal, ListInput{
			IncludeInactive: readBoolQuery(r, "includeInactive"),
		})
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao carregar os tenants.")
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, listResponse{
			Tenants: tenants,
		})
	})))

	mux.Handle("POST /v1/tenants", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		var request createRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		tenant, err := service.Create(r.Context(), principal, CreateInput{
			Slug:     request.Slug,
			Name:     request.Name,
			IsActive: request.IsActive,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusCreated, tenantResponse{Tenant: tenant})
	})))

	mux.Handle("PATCH /v1/tenants/{id}", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		tenantID := strings.TrimSpace(r.PathValue("id"))
		if tenantID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_tenant_id", "Cliente invalido.")
			return
		}

		var request updateRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		tenant, err := service.Update(r.Context(), principal, UpdateInput{
			ID:       tenantID,
			Slug:     request.Slug,
			Name:     request.Name,
			IsActive: request.IsActive,
		})
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, tenantResponse{Tenant: tenant})
	})))

	mux.Handle("POST /v1/tenants/{id}/archive", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		tenantID := strings.TrimSpace(r.PathValue("id"))
		if tenantID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_tenant_id", "Cliente invalido.")
			return
		}

		tenant, err := service.Archive(r.Context(), principal, tenantID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, tenantResponse{Tenant: tenant})
	})))

	mux.Handle("POST /v1/tenants/{id}/restore", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		tenantID := strings.TrimSpace(r.PathValue("id"))
		if tenantID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_tenant_id", "Cliente invalido.")
			return
		}

		tenant, err := service.Restore(r.Context(), principal, tenantID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, tenantResponse{Tenant: tenant})
	})))
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para alterar este cliente.")
	case errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados do cliente.")
	case errors.Is(err, ErrTenantNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "tenant_not_found", "Cliente nao encontrado.")
	case errors.Is(err, ErrTenantConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "tenant_conflict", "Ja existe um cliente com este slug.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar o cliente.")
	}
}

func readBoolQuery(r *http.Request, key string) bool {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
