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
//
// A autorizacao NAO e mais um gate fixo de platform_admin: cada handler usa
// RequireAuth + decisao de escopo POR-REQUEST (o escopo depende do accountId/
// userId do path). O ator sem NENHUM poder de admin recebe 403 logo no inicio
// (anti-sondagem, via requireAdminActor). O actorUserID vem SEMPRE do Principal.
// Decisoes identity-global (criar/deletar usuario, mover, campos globais no PATCH)
// continuam restritas a platform_admin, validado dentro do service.
func RegisterAdminUsersRoutes(mux *http.ServeMux, svc *AdminUserService, middleware *auth.Middleware) {
	wrap := func(h func(*AdminUserService, http.ResponseWriter, *http.Request)) http.Handler {
		return middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !requireAdminActor(svc, w, r) {
				return
			}
			h(svc, w, r)
		}))
	}

	mux.Handle("GET /v1/admin/users", wrap(handleListUsers))
	mux.Handle("POST /v1/admin/users", wrap(handleCreateUser))
	mux.Handle("GET /v1/admin/users/{id}", wrap(handleGetUser))
	mux.Handle("PATCH /v1/admin/users/{id}", wrap(handleUpdateUser))
	mux.Handle("DELETE /v1/admin/users/{id}", wrap(handleDeleteUser))
	mux.Handle("GET /v1/admin/users/{id}/memberships", wrap(handleGetMemberships))
	mux.Handle("POST /v1/admin/users/{id}/memberships", wrap(handleAddMembership))
	mux.Handle("PATCH /v1/admin/users/{id}/memberships/{accountId}", wrap(handleUpdateMembershipRole))
	mux.Handle("DELETE /v1/admin/users/{id}/memberships/{accountId}", wrap(handleRemoveMembership))
	mux.Handle("POST /v1/admin/users/{id}/organizations/{orgId}", wrap(handleLinkOrganization))
	mux.Handle("DELETE /v1/admin/users/{id}/organizations/{orgId}", wrap(handleUnlinkOrganization))
	mux.Handle("PUT /v1/admin/users/{id}/account", wrap(handleMoveUserAccount))
}

// requireAdminActor responde 403 cedo a quem nao tem NENHUM poder de admin (gate
// barato anti-sondagem). Quem passa daqui ainda pode levar 404 por escopo no
// recurso especifico. actorUserID vem do Principal.
func requireAdminActor(svc *AdminUserService, w http.ResponseWriter, r *http.Request) bool {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthorized", "Autenticacao obrigatoria.")
		return false
	}
	isAdmin, err := svc.scope.IsAdminOfAnything(r.Context(), principal.UserID)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return false
	}
	if !isAdmin {
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Sem permissao de gestao de usuarios.")
		return false
	}
	return true
}

// actorID extrai o id do ator do Principal (ja garantido por RequireAuth +
// requireAdminActor). Centraliza para nenhum handler ler de outra fonte.
func actorID(r *http.Request) string {
	principal, _ := auth.PrincipalFromContext(r.Context())
	return principal.UserID
}

// ============================================================================
// Handlers
// ============================================================================

func handleListUsers(svc *AdminUserService, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("perPage"))

	// includeAccounts default true (mantem contrato antigo). Apenas o valor
	// explicito "false" ativa a projecao lean sem o agregado de contas.
	includeAccounts := strings.TrimSpace(q.Get("includeAccounts")) != "false"

	filter := AdminUserListFilter{
		Q:             strings.TrimSpace(q.Get("q")),
		Status:        strings.TrimSpace(q.Get("status")),
		PlatformAdmin: strings.TrimSpace(q.Get("platformAdmin")),
		AccountID:     strings.TrimSpace(q.Get("accountId")),
		// ActorUserID SEMPRE do Principal (nunca da query) — o repo escopa a
		// listagem por ele (count + linhas no mesmo where).
		ActorUserID:     actorID(r),
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

func handleGetUser(svc *AdminUserService, w http.ResponseWriter, r *http.Request) {
	user, err := svc.GetUser(r.Context(), actorID(r), r.PathValue("id"))
	if err != nil {
		writeAdminUserError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, user)
}

func handleCreateUser(svc *AdminUserService, w http.ResponseWriter, r *http.Request) {
	var input AdminCreateUserInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	user, err := svc.CreateUser(r.Context(), actorID(r), input)
	if err != nil {
		writeAdminUserError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, user)
}

func handleUpdateUser(svc *AdminUserService, w http.ResponseWriter, r *http.Request) {
	var input AdminUpdateUserInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	user, err := svc.UpdateUser(r.Context(), actorID(r), r.PathValue("id"), input)
	if err != nil {
		writeAdminUserError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, user)
}

func handleDeleteUser(svc *AdminUserService, w http.ResponseWriter, r *http.Request) {
	if err := svc.DeleteUser(r.Context(), actorID(r), r.PathValue("id")); err != nil {
		writeAdminUserError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleGetMemberships(svc *AdminUserService, w http.ResponseWriter, r *http.Request) {
	resp, err := svc.GetMemberships(r.Context(), actorID(r), r.PathValue("id"))
	if err != nil {
		writeAdminUserError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

// handleAddMembership adiciona um vinculo de cliente ao usuario SEM remover os
// outros (POST .../memberships). Escopo CanManageAccount via service.
func handleAddMembership(svc *AdminUserService, w http.ResponseWriter, r *http.Request) {
	var input AddMembershipInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	resp, err := svc.AddMembership(r.Context(), actorID(r), r.PathValue("id"), input)
	if err != nil {
		writeAdminUserError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, resp)
}

func handleUpdateMembershipRole(svc *AdminUserService, w http.ResponseWriter, r *http.Request) {
	var input UpdateMembershipRoleInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	resp, err := svc.UpdateMembershipRole(r.Context(), actorID(r), r.PathValue("id"), r.PathValue("accountId"), input)
	if err != nil {
		writeAdminUserError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

// handleRemoveMembership desativa o vinculo de cliente (DELETE .../memberships/{accountId}).
// Convive com o PATCH no mesmo path (method-aware).
func handleRemoveMembership(svc *AdminUserService, w http.ResponseWriter, r *http.Request) {
	resp, err := svc.RemoveMembership(r.Context(), actorID(r), r.PathValue("id"), r.PathValue("accountId"))
	if err != nil {
		writeAdminUserError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

// handleLinkOrganization vincula uma agencia a um usuario (POST .../organizations/{orgId}).
// Escopo restrito (platform_admin OU agency_owner da org) + confirmacao de acesso amplo.
// Retorna AdminUserView (mesmo shape do PATCH/PUT account) para o front aplicar na linha.
func handleLinkOrganization(svc *AdminUserService, w http.ResponseWriter, r *http.Request) {
	var input LinkOrganizationInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	resp, err := svc.LinkOrganization(r.Context(), actorID(r), r.PathValue("id"), r.PathValue("orgId"), input)
	if err != nil {
		writeAdminUserError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

// handleUnlinkOrganization remove o vinculo de agencia (DELETE .../organizations/{orgId}).
// Safeguard do ultimo agency_owner (409). Retorna AdminUserView (mesmo shape do PATCH).
func handleUnlinkOrganization(svc *AdminUserService, w http.ResponseWriter, r *http.Request) {
	resp, err := svc.UnlinkOrganization(r.Context(), actorID(r), r.PathValue("id"), r.PathValue("orgId"))
	if err != nil {
		writeAdminUserError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

// handleMoveUserAccount MOVE o usuario para a conta-cliente do body. Devolve o
// AdminUserView atualizado (mesmo shape do PATCH) para o front atualizar a linha.
// Acao destrutiva -> platform_admin apenas (validado no service).
func handleMoveUserAccount(svc *AdminUserService, w http.ResponseWriter, r *http.Request) {
	var input MoveUserAccountInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	user, err := svc.MoveUserAccount(r.Context(), actorID(r), r.PathValue("id"), input)
	if err != nil {
		writeAdminUserError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, user)
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
	case errors.Is(err, ErrInvalidRole):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_role", "Papel invalido. Use owner, director ou marketing.")
	case errors.Is(err, ErrAccountNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "account_not_found", "Conta destino nao encontrada ou inativa.")
	case errors.Is(err, ErrAccountIsAgency):
		httpapi.WriteError(w, r, http.StatusBadRequest, "account_is_agency", "A conta destino e uma agencia; este endpoint move apenas para conta-cliente.")
	case errors.Is(err, ErrOrganizationNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "organization_not_found", "Organizacao nao encontrada ou fora do seu escopo.")
	case errors.Is(err, ErrInvalidOrgRole):
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_org_role", "Cargo de agencia invalido. Use agency_owner ou agency_member.")
	case errors.Is(err, ErrConfirmationRequired):
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "confirmation_required", "Vincular a agencia da visao de TODOS os clientes da org. Confirme confirmAgencyWideAccess=true.")
	case errors.Is(err, ErrLastAgencyOwner):
		httpapi.WriteError(w, r, http.StatusConflict, "last_agency_owner", "Nao e possivel remover o ultimo dono (agency_owner) da organizacao.")
	case errors.Is(err, ErrForbiddenField):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden_field", "Apenas platform_admin pode editar dados de identidade global do usuario.")
	case errors.Is(err, ErrForbidden):
		httpapi.WriteError(w, r, http.StatusForbidden, "forbidden", "Esta acao exige papel platform_admin.")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
	}
}
