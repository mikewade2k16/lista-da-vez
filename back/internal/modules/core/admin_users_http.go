package core

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterAdminUsersRoutes monta os endpoints /v1/admin/users* no mux.
// Todas as rotas exigem papel platform_admin — verificado em requirePlatformAdmin
// (reaproveitado de admin_http.go).
func RegisterAdminUsersRoutes(mux *http.ServeMux, svc *AdminUserService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !requirePlatformAdmin(w, r) {
				return
			}
			h(w, r)
		}))
	}

	mux.Handle("GET /v1/admin/users", wrap(handleListUsers(svc)))
	mux.Handle("POST /v1/admin/users", wrap(handleCreateUser(svc)))
	mux.Handle("GET /v1/admin/users/{id}", wrap(handleGetUser(svc)))
	mux.Handle("PATCH /v1/admin/users/{id}", wrap(handleUpdateUser(svc)))
	mux.Handle("DELETE /v1/admin/users/{id}", wrap(handleDeleteUser(svc)))
	mux.Handle("GET /v1/admin/users/{id}/memberships", wrap(handleGetMemberships(svc)))
}

// ============================================================================
// Handlers
// ============================================================================

func handleListUsers(svc *AdminUserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		perPage, _ := strconv.Atoi(q.Get("perPage"))

		// includeAccounts default true (mantem contrato antigo). Apenas o valor
		// explicito "false" ativa a projecao lean sem o agregado de contas.
		includeAccounts := strings.TrimSpace(q.Get("includeAccounts")) != "false"

		filter := AdminUserListFilter{
			Q:               strings.TrimSpace(q.Get("q")),
			Status:          strings.TrimSpace(q.Get("status")),
			PlatformAdmin:   strings.TrimSpace(q.Get("platformAdmin")),
			Page:            page,
			PerPage:         perPage,
			IncludeAccounts: includeAccounts,
		}

		resp, err := svc.ListUsers(r.Context(), filter)
		if err != nil {
			writeAdminUserError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

func handleGetUser(svc *AdminUserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		user, err := svc.GetUser(r.Context(), id)
		if err != nil {
			writeAdminUserError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, user)
	}
}

func handleCreateUser(svc *AdminUserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input AdminCreateUserInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}

		user, err := svc.CreateUser(r.Context(), input)
		if err != nil {
			writeAdminUserError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, user)
	}
}

func handleUpdateUser(svc *AdminUserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var input AdminUpdateUserInput
		if err := httpapi.ReadJSON(r, &input); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}

		user, err := svc.UpdateUser(r.Context(), id, input)
		if err != nil {
			writeAdminUserError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, user)
	}
}

func handleDeleteUser(svc *AdminUserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := svc.DeleteUser(r.Context(), id); err != nil {
			writeAdminUserError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleGetMemberships(svc *AdminUserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		resp, err := svc.GetMemberships(r.Context(), id)
		if err != nil {
			writeAdminUserError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

// ============================================================================
// Erros
// ============================================================================

func writeAdminUserError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrUserNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "user_not_found", "Usuario nao encontrado.")
	case errors.Is(err, ErrUserEmailConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "email_conflict", "Ja existe um usuario com este email.")
	case errors.Is(err, ErrLastPlatformAdmin):
		httpapi.WriteError(w, r, http.StatusConflict, "last_platform_admin", "Nao e possivel rebaixar ou desativar o ultimo platform admin ativo.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
	}
}
