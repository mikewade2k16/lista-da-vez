package access

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type roleMatrixResponse struct {
	Permissions []PermissionDefinition `json:"permissions"`
	Roles       []RoleMatrixEntry      `json:"roles"`
}

type roleUpdateRequest struct {
	PermissionKeys []string `json:"permissionKeys"`
}

type roleEntryResponse struct {
	Role RoleMatrixEntry `json:"role"`
}

type userAccessResponse struct {
	Access UserAccessView `json:"access"`
}

type userOverrideUpdateRequest struct {
	Overrides []UserOverride `json:"overrides"`
}

func RegisterRoutes(mux *http.ServeMux, service *Service, middleware *auth.Middleware) {
	mux.Handle("GET /v1/access/roles", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		matrix, err := service.ListRoleMatrix(r.Context(), principal)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, roleMatrixResponse{
			Permissions: matrix.Permissions,
			Roles:       matrix.Roles,
		})
	})))

	mux.Handle("PUT /v1/access/roles/", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		roleID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/access/roles/"))
		if roleID == "" {
			http.NotFound(w, r)
			return
		}

		var request roleUpdateRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		entry, err := service.UpdateRolePermissions(r.Context(), principal, auth.Role(roleID), request.PermissionKeys)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, roleEntryResponse{Role: entry})
	})))

	mux.Handle("GET /v1/access/users/", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		userID, action := splitUserSubroute(r.URL.Path)
		if userID == "" || action != "" {
			http.NotFound(w, r)
			return
		}

		accessView, err := service.GetUserAccess(r.Context(), principal, userID)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, userAccessResponse{Access: accessView})
	})))

	mux.Handle("PUT /v1/access/users/", middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.PrincipalFromContext(r.Context())
		if !ok {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
			return
		}

		userID, action := splitUserSubroute(r.URL.Path)
		if userID == "" || action != "overrides" {
			http.NotFound(w, r)
			return
		}

		var request userOverrideUpdateRequest
		if err := httpapi.ReadJSON(r, &request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_json", "Payload invalido.")
			return
		}

		accessView, err := service.UpdateUserOverrides(r.Context(), principal, userID, request.Overrides)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, userAccessResponse{Access: accessView})
	})))
}

func splitUserSubroute(path string) (string, string) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/v1/access/users/"), "/")
	if trimmed == "" {
		return "", ""
	}

	segments := strings.Split(trimmed, "/")
	userID := strings.TrimSpace(segments[0])
	if userID == "" {
		return "", ""
	}

	if len(segments) == 1 {
		return userID, ""
	}

	return userID, strings.TrimSpace(segments[1])
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao para gerenciar esta configuracao de acesso.")
	case errors.Is(err, ErrValidation):
		httpapi.WriteError(w, r, http.StatusBadRequest, "validation_error", "Verifique os dados enviados.")
	case errors.Is(err, ErrNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "user_not_found", "Usuario nao encontrado.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao processar a configuracao de acesso.")
	}
}
