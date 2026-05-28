package core

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// RegisterRBACRoutes monta os endpoints de RBAC do core.
//
// Endpoints:
//
//	GET    /v1/accounts/{accountId}/roles
//	POST   /v1/accounts/{accountId}/roles
//	GET    /v1/accounts/{accountId}/roles/{roleId}
//	PATCH  /v1/accounts/{accountId}/roles/{roleId}
//	DELETE /v1/accounts/{accountId}/roles/{roleId}
//	POST   /v1/accounts/{accountId}/members/{userId}/roles/{roleId}
//	DELETE /v1/accounts/{accountId}/members/{userId}/roles/{roleId}
func RegisterRBACRoutes(mux *http.ServeMux, svc *RBACService, middleware *auth.Middleware) {
	h := &rbacHandler{svc: svc, middleware: middleware}

	mux.Handle("GET /v1/accounts/{accountId}/roles",
		middleware.RequireAuth(http.HandlerFunc(h.listRoles)))

	mux.Handle("POST /v1/accounts/{accountId}/roles",
		middleware.RequireAuth(http.HandlerFunc(h.createRole)))

	mux.Handle("GET /v1/accounts/{accountId}/roles/{roleId}",
		middleware.RequireAuth(http.HandlerFunc(h.getRole)))

	mux.Handle("PATCH /v1/accounts/{accountId}/roles/{roleId}",
		middleware.RequireAuth(http.HandlerFunc(h.updateRole)))

	mux.Handle("DELETE /v1/accounts/{accountId}/roles/{roleId}",
		middleware.RequireAuth(http.HandlerFunc(h.deleteRole)))

	mux.Handle("POST /v1/accounts/{accountId}/members/{userId}/roles/{roleId}",
		middleware.RequireAuth(http.HandlerFunc(h.assignRole)))

	mux.Handle("DELETE /v1/accounts/{accountId}/members/{userId}/roles/{roleId}",
		middleware.RequireAuth(http.HandlerFunc(h.removeRole)))
}

type rbacHandler struct {
	svc        *RBACService
	middleware *auth.Middleware
}

// requireMember verifica que o user autenticado tem membership ativa na account.
func (h *rbacHandler) requireMember(w http.ResponseWriter, r *http.Request, accountID, userID string) bool {
	if err := h.svc.rbac.CheckMembership(r.Context(), accountID, userID); err != nil {
		if errors.Is(err, ErrAccountNotMember) {
			httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem acesso a esta account.")
			return false
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao verificar membership.")
		return false
	}
	return true
}

// ============================================================================
// Handlers
// ============================================================================

func (h *rbacHandler) listRoles(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
		return
	}

	accountID := r.PathValue("accountId")
	if !h.requireMember(w, r, accountID, principal.UserID) {
		return
	}

	roles, err := h.svc.ListRoles(r.Context(), accountID)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao listar roles.")
		return
	}

	summaries := make([]RoleSummary, 0, len(roles))
	for _, ro := range roles {
		summaries = append(summaries, ro.ToSummary())
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"roles": summaries})
}

func (h *rbacHandler) createRole(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
		return
	}

	accountID := r.PathValue("accountId")
	if !h.requireMember(w, r, accountID, principal.UserID) {
		return
	}

	var body struct {
		Code        string `json:"code"`
		Label       string `json:"label"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body JSON invalido.")
		return
	}

	role, err := h.svc.CreateRole(r.Context(), accountID, body.Code, body.Label, body.Description)
	if err != nil {
		writeRBACError(w, r, err)
		return
	}

	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"role": role.ToSummary()})
}

func (h *rbacHandler) getRole(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
		return
	}

	accountID := r.PathValue("accountId")
	if !h.requireMember(w, r, accountID, principal.UserID) {
		return
	}

	role, err := h.svc.GetRole(r.Context(), accountID, r.PathValue("roleId"))
	if err != nil {
		writeRBACError(w, r, err)
		return
	}

	permKeys, err := h.svc.rbac.ListRolePermissions(r.Context(), role.ID)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro ao carregar permissoes.")
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"role":        role.ToSummary(),
		"permissions": permKeys,
	})
}

func (h *rbacHandler) updateRole(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
		return
	}

	accountID := r.PathValue("accountId")
	if !h.requireMember(w, r, accountID, principal.UserID) {
		return
	}

	var body struct {
		Label       string   `json:"label"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body JSON invalido.")
		return
	}

	role, err := h.svc.UpdateRolePermissions(
		r.Context(), accountID, r.PathValue("roleId"),
		body.Label, body.Description, body.Permissions,
	)
	if err != nil {
		writeRBACError(w, r, err)
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"role": role.ToSummary()})
}

func (h *rbacHandler) deleteRole(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
		return
	}

	accountID := r.PathValue("accountId")
	if !h.requireMember(w, r, accountID, principal.UserID) {
		return
	}

	if err := h.svc.DeleteRole(r.Context(), accountID, r.PathValue("roleId")); err != nil {
		writeRBACError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *rbacHandler) assignRole(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
		return
	}

	accountID := r.PathValue("accountId")
	if !h.requireMember(w, r, accountID, principal.UserID) {
		return
	}

	if err := h.svc.AssignRoleToUser(
		r.Context(), accountID,
		r.PathValue("userId"), r.PathValue("roleId"),
	); err != nil {
		writeRBACError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *rbacHandler) removeRole(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
		return
	}

	accountID := r.PathValue("accountId")
	if !h.requireMember(w, r, accountID, principal.UserID) {
		return
	}

	if err := h.svc.RemoveRoleFromUser(
		r.Context(), accountID,
		r.PathValue("userId"), r.PathValue("roleId"),
	); err != nil {
		writeRBACError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeRBACError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrRoleNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "role_not_found", "Cargo nao encontrado.")
	case errors.Is(err, ErrRoleCodeConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "role_code_conflict", "Ja existe um cargo com este code na account.")
	case errors.Is(err, ErrRoleIsLocked):
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "role_locked", "Este cargo e bloqueado e nao pode ser removido.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
	}
}
