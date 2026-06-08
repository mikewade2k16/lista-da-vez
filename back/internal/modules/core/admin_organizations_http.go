package core

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterAdminOrganizationsRoutes monta os endpoints /v1/admin/organizations*
// no mux. Todas as rotas exigem papel platform_admin.
func RegisterAdminOrganizationsRoutes(mux *http.ServeMux, svc *AdminOrganizationService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !requirePlatformAdmin(w, r) {
				return
			}
			h(w, r)
		}))
	}

	mux.Handle("GET /v1/admin/organizations", wrap(handleListOrganizations(svc)))
	mux.Handle("POST /v1/admin/organizations", wrap(handleCreateOrganization(svc)))
	mux.Handle("GET /v1/admin/organizations/{id}", wrap(handleGetOrganization(svc)))
	mux.Handle("PATCH /v1/admin/organizations/{id}", wrap(handleUpdateOrganization(svc)))
	mux.Handle("DELETE /v1/admin/organizations/{id}", wrap(handleDeleteOrganization(svc)))
}

// ============================================================================
// Handlers
// ============================================================================

func handleListOrganizations(svc *AdminOrganizationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		perPage, _ := strconv.Atoi(q.Get("perPage"))

		filter := AdminOrganizationListFilter{
			Q:       strings.TrimSpace(q.Get("q")),
			Status:  strings.TrimSpace(q.Get("status")),
			Page:    page,
			PerPage: perPage,
		}

		resp, err := svc.ListOrganizations(r.Context(), filter)
		if err != nil {
			writeAdminOrganizationError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

func handleGetOrganization(svc *AdminOrganizationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		org, err := svc.GetOrganization(r.Context(), id)
		if err != nil {
			writeAdminOrganizationError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, org)
	}
}

func handleCreateOrganization(svc *AdminOrganizationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input AdminCreateOrganizationInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}

		org, err := svc.CreateOrganization(r.Context(), input)
		if err != nil {
			writeAdminOrganizationError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, org)
	}
}

func handleUpdateOrganization(svc *AdminOrganizationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var input AdminUpdateOrganizationInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}

		org, err := svc.UpdateOrganization(r.Context(), id, input)
		if err != nil {
			writeAdminOrganizationError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, org)
	}
}

func handleDeleteOrganization(svc *AdminOrganizationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := svc.DeleteOrganization(r.Context(), id); err != nil {
			writeAdminOrganizationError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ============================================================================
// Erros
// ============================================================================

func writeAdminOrganizationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrOrganizationNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "organization_not_found", "Organization nao encontrada.")
	case errors.Is(err, ErrOrganizationSlugConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "slug_conflict", "Ja existe uma organization com este slug.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
	}
}
