package core

import (
	"errors"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// admin_role_templates_http.go — endpoints /v1/admin/role-templates*.
// Catalogo GLOBAL de papeis-padrao, gerenciado por platform_admin. Reusa o MESMO
// gate dos demais /v1/admin/* do core (requirePlatformAdmin): RequireAuth resolve
// o Principal do token assinado; requirePlatformAdmin exige principal.Role ==
// auth.RolePlatformAdmin (403 caso contrario). Templates is_system=true sao
// congelados (CONTRACT_FREEZE): PATCH/PUT/DELETE neles -> 409.

// RegisterRoleTemplatesRoutes monta os endpoints de CRUD de role templates.
// Todas as rotas exigem platform_admin (verificado em requirePlatformAdmin).
//
//	GET    /v1/admin/role-templates
//	POST   /v1/admin/role-templates
//	PATCH  /v1/admin/role-templates/{id}
//	PUT    /v1/admin/role-templates/{id}/permissions
//	DELETE /v1/admin/role-templates/{id}
func RegisterRoleTemplatesRoutes(mux *http.ServeMux, svc *RoleTemplateAdminService, middleware *auth.Middleware) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !requirePlatformAdmin(w, r) {
				return
			}
			h(w, r)
		}))
	}

	mux.Handle("GET /v1/admin/role-templates", wrap(handleListRoleTemplates(svc)))
	mux.Handle("POST /v1/admin/role-templates", wrap(handleCreateRoleTemplate(svc)))
	mux.Handle("PATCH /v1/admin/role-templates/{id}", wrap(handlePatchRoleTemplate(svc)))
	mux.Handle("PUT /v1/admin/role-templates/{id}/permissions", wrap(handleReplaceRoleTemplatePermissions(svc)))
	mux.Handle("DELETE /v1/admin/role-templates/{id}", wrap(handleDeleteRoleTemplate(svc)))
}

func handleListRoleTemplates(svc *RoleTemplateAdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := svc.List(r.Context())
		if err != nil {
			writeRoleTemplateError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

func handleCreateRoleTemplate(svc *RoleTemplateAdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in CreateRoleTemplateInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body JSON invalido.")
			return
		}
		t, err := svc.Create(r.Context(), in)
		if err != nil {
			writeRoleTemplateError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, t)
	}
}

func handlePatchRoleTemplate(svc *RoleTemplateAdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in PatchRoleTemplateInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body JSON invalido.")
			return
		}
		t, err := svc.Patch(r.Context(), r.PathValue("id"), in)
		if err != nil {
			writeRoleTemplateError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, t)
	}
}

func handleReplaceRoleTemplatePermissions(svc *RoleTemplateAdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in ReplaceRoleTemplatePermissionsInput
		if err := httpapi.ReadJSON(r, &in); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body JSON invalido.")
			return
		}
		t, err := svc.ReplacePermissions(r.Context(), r.PathValue("id"), in.PermissionKeys)
		if err != nil {
			writeRoleTemplateError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, t)
	}
}

func handleDeleteRoleTemplate(svc *RoleTemplateAdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Delete(r.Context(), r.PathValue("id")); err != nil {
			writeRoleTemplateError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// writeRoleTemplateError traduz os erros do service para HTTP, no padrao httpapi.
func writeRoleTemplateError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrTemplateNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "role_template_not_found", "Papel-padrao nao encontrado.")
	case errors.Is(err, ErrRoleTemplateConflict):
		httpapi.WriteError(w, r, http.StatusConflict, "role_template_conflict", "Ja existe um papel-padrao com este id.")
	case errors.Is(err, ErrRoleTemplateSystem):
		httpapi.WriteError(w, r, http.StatusConflict, "role_template_system", "Papel-padrao de sistema nao pode ser editado nem removido.")
	case errors.Is(err, ErrRoleTemplateInvalidID):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_id", "Id invalido: use apenas minusculas, digitos, ponto, underscore e hifen.")
	case errors.Is(err, ErrRoleTemplateLabelRequired):
		httpapi.WriteError(w, r, http.StatusBadRequest, "label_required", "O nome (label) do papel-padrao e obrigatorio.")
	case errors.Is(err, ErrInvalidPermission):
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "invalid_permission", "Uma ou mais permissoes sao invalidas, deprecated ou de escopo de plataforma.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Erro interno ao processar a requisicao.")
	}
}
