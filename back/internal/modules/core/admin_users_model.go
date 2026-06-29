package core

import (
	"context"
	"time"
)

// AccountMembershipView é um item de "accounts em que o user participa".
// Usado por GET /v1/admin/users/{id}/memberships. Role é o papel coarse do user
// naquela conta (owner/director/marketing/...); IsAgency marca a conta-agencia.
type AccountMembershipView struct {
	AccountID   string    `json:"accountId"`
	AccountSlug string    `json:"accountSlug"`
	AccountName string    `json:"accountName"`
	IsActive    bool      `json:"isActive"`
	JoinedAt    time.Time `json:"joinedAt"`
	Role        string    `json:"role"`
	IsAgency    bool      `json:"isAgency"`
}

// UpdateMembershipRoleInput é o body de PATCH /v1/admin/users/{id}/memberships/{accountId}.
type UpdateMembershipRoleInput struct {
	Role string `json:"role"`
}

// MoveUserAccountInput é o body de PUT /v1/admin/users/{id}/account.
// Move o usuario do(s) cliente(s) atual(is) para a conta-cliente destino:
// remove os vinculos de CLIENTE atuais (account_users NAO-agencia + role
// assignments dessas contas) e matricula no destino. NAO toca vinculos de
// agencia (is_agency=true). Role opcional (default "owner").
type MoveUserAccountInput struct {
	AccountID string `json:"accountId"`
	Role      string `json:"role,omitempty"`
}

// AdminUserView é o DTO de um user para o painel /manage/users.
// Inclui agregados (accountCount, accountSlugs) computados no backend.
type AdminUserView struct {
	ID                 string `json:"id"`
	Email              string `json:"email"`
	DisplayName        string `json:"displayName"`
	Nick               string `json:"nick"`
	AvatarPath         string `json:"avatarPath,omitempty"`
	IsActive           bool   `json:"isActive"`
	IsPlatformAdmin    bool   `json:"isPlatformAdmin"`
	MustChangePassword bool   `json:"mustChangePassword"`
	AccountCount       int    `json:"accountCount"`
	AccountNames       string `json:"accountNames"`
	// ClientAccountID e o id do UNICO cliente ativo NAO-agencia
	// (core.accounts.is_agency=false) do usuario. "" quando o usuario tem 0 ou
	// mais de 1 cliente nao-agencia. Serve para o front preselecionar a conta e
	// decidir se a celula de "mover cliente" e editavel.
	ClientAccountID string `json:"clientAccountId"`
	// IsAgencyMember marca que o usuario e membro ATIVO de pelo menos uma
	// conta-agencia (core.accounts.is_agency=true). Serve para o painel sinalizar
	// na grade que o usuario ve todos os clientes/modulos da agencia (guard-rail
	// contra "usuario de cliente virou membro de agencia sem querer").
	IsAgencyMember bool      `json:"isAgencyMember"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// AdminUserListFilter parametriza GET /v1/admin/users.
type AdminUserListFilter struct {
	Q             string
	Status        string // "active" | "inactive" | "" (todos)
	PlatformAdmin string // "true" | "false" | "" (todos)
	// AccountID: quando != "", filtra para usuarios que sao membros ATIVOS daquela
	// conta (join em core.account_users + conta ativa). "" = sem filtro de conta.
	AccountID string
	// ActorUserID e o id do ator autenticado (SEMPRE do Principal, nunca da query).
	// O repositorio aplica o predicado de escopo (listUsersScopeWhere) no count(*)
	// e no SELECT: platform_admin ve todos; agency_owner ve usuarios das accounts
	// da org; admin de cliente ve membros das accounts onde tem core.users.manage.
	// AccountID vira "filtro dentro do permitido".
	ActorUserID string
	Page        int
	PerPage     int
	// IncludeAccounts: quando true (default), a listagem agrega accountCount e
	// accountNames por user (lateral join). Quando false, devolve a projecao lean
	// (sem o agregado) — usado pela tela acima-da-dobra, que carrega o detalhe de
	// contas sob interacao (popover de memberships). Espelha "pedir so o necessario".
	IncludeAccounts bool
}

// AdminUserListItem e a projecao LEAN de um user na LISTAGEM /v1/admin/users.
// Espelha o subconjunto de campos que a tabela /manage/users (AdminUsersWorkspace +
// colunas + popover de detalhes/acoes) realmente renderiza — OPT-4/F-26. Em relacao
// ao AdminUserView (detalhe/drawer), OMITE de proposito tres campos que NENHUM
// consumidor da listagem usa: avatarPath, createdAt e updatedAt. O detalhe
// (FindAdminUser/AdminUserView) continua devolvendo o objeto completo.
type AdminUserListItem struct {
	ID                 string `json:"id"`
	Email              string `json:"email"`
	DisplayName        string `json:"displayName"`
	Nick               string `json:"nick"`
	IsActive           bool   `json:"isActive"`
	IsPlatformAdmin    bool   `json:"isPlatformAdmin"`
	MustChangePassword bool   `json:"mustChangePassword"`
	AccountCount       int    `json:"accountCount"`
	AccountNames       string `json:"accountNames"`
	// ClientAccountID: id do UNICO cliente ativo nao-agencia (ou "" p/ 0/>1). Habilita
	// a edicao inline da coluna "Cliente". Mesma semantica do AdminUserView.
	ClientAccountID string `json:"clientAccountId"`
	// IsAgencyMember: membro ativo de ao menos uma conta-agencia. Gateia a celula
	// "Cliente" e o badge "Agencia" na grade. Mesma semantica do AdminUserView.
	IsAgencyMember bool `json:"isAgencyMember"`
}

// AdminUserListResponse e o body de GET /v1/admin/users.
type AdminUserListResponse struct {
	Users   []AdminUserListItem `json:"users"`
	Total   int                 `json:"total"`
	Page    int                 `json:"page"`
	PerPage int                 `json:"perPage"`
}

// AdminCreateUserInput e o body de POST /v1/admin/users.
// TemporaryPassword opcional: se vazio, user fica com password_hash null e
// must_change_password true (precisa fluxo de aceite de convite).
type AdminCreateUserInput struct {
	Email             string `json:"email"`
	DisplayName       string `json:"displayName"`
	Nick              string `json:"nick"`
	IsPlatformAdmin   bool   `json:"isPlatformAdmin"`
	TemporaryPassword string `json:"temporaryPassword"`
	// AccountID opcional: vincula o user a uma account (cliente) via membership
	// em core.account_users. Sem ele, o user fica sem cliente (ex.: platform_admin).
	AccountID string `json:"accountId,omitempty"`
	// OrganizationID opcional: vincula o user a uma organization (agencia) via
	// core.organization_users (org_role 'agency_member').
	OrganizationID string `json:"organizationId,omitempty"`
	// Role: papel no tenant do AccountID (owner/director/marketing). Cria
	// user_tenant_roles legado — NECESSARIO para login (auth resolve papel pelo
	// legado) e para aparecer em /operacao/usuarios. Default 'owner' quando
	// AccountID setado. accountId == tenantId (core.accounts.id == public.tenants.id).
	Role string `json:"role,omitempty"`
	// OrgRole: cargo na agencia (organization) quando OrganizationID setado. Valores
	// 'agency_owner' (acesso total da agencia) ou 'agency_member' (acesso limitado).
	// Default 'agency_member'. O repo tambem matricula o user na conta-agencia com o
	// papel correspondente para que ele consiga logar.
	OrgRole string `json:"orgRole,omitempty"`
}

// AdminUpdateUserInput e o body de PATCH /v1/admin/users/:id.
// Semantica de patch: campos nil sao ignorados.
type AdminUpdateUserInput struct {
	Email           *string `json:"email"`
	DisplayName     *string `json:"displayName"`
	Nick            *string `json:"nick"`
	IsActive        *bool   `json:"isActive"`
	IsPlatformAdmin *bool   `json:"isPlatformAdmin"`
	// Password define/reseta a senha do usuario. Semantica CRITICA: nil ou vazio
	// = NAO mexe no password_hash (regra "nunca sobrescrever senha sem acao
	// explicita"). So quando vem uma senha nao-vazia (minimo 8) o service hasheia
	// e o repo faz SET password_hash + must_change_password = false. Nunca logado.
	Password *string `json:"password"`
}

// AdminMembershipsResponse e o body de GET /v1/admin/users/:id/memberships.
type AdminMembershipsResponse struct {
	Memberships []AccountMembershipView `json:"memberships"`
}

// AddMembershipInput e o body de POST /v1/admin/users/{id}/memberships.
// Adiciona um vinculo de cliente SEM remover os demais (diferente do PUT
// .../account que MOVE). Role default "owner" em {owner,director,marketing}.
type AddMembershipInput struct {
	AccountID string `json:"accountId"`
	Role      string `json:"role,omitempty"`
}

// LinkOrganizationInput e o body de POST /v1/admin/users/{id}/organizations/{orgId}.
// OrgRole em {agency_owner,agency_member}. ConfirmAgencyWideAccess precisa ser true
// (virar membro de agencia da visao de TODOS os clientes da org).
type LinkOrganizationInput struct {
	OrgRole                 string `json:"orgRole"`
	ConfirmAgencyWideAccess bool   `json:"confirmAgencyWideAccess"`
}

// AdminUserLinksRepository abstrai a persistencia dos vinculos de um usuario:
// membership de cliente e cargo de organization. Operacoes destrutivas sao
// transacionais.
type AdminUserLinksRepository interface {
	// FindAccountLinkInfo carrega existe/ativa/agencia da account destino.
	FindAccountLinkInfo(ctx context.Context, accountID string) (AccountLinkInfo, error)
	// AddMembership matricula o usuario na conta-cliente sem remover os outros vinculos.
	AddMembership(ctx context.Context, accountID, userID, role string) error
	// DeactivateMembership desativa o vinculo de cliente (preserva joined_at) e
	// remove os role_assignments daquela conta.
	DeactivateMembership(ctx context.Context, accountID, userID string) error
	// FindOrganizationLinkInfo carrega existe/ativa da org destino.
	FindOrganizationLinkInfo(ctx context.Context, organizationID string) (OrganizationLinkInfo, error)
	// LinkUserToOrganization vincula a org (cargo de agencia) + matricula na conta-agencia.
	LinkUserToOrganization(ctx context.Context, organizationID, userID, orgRole string) error
	// CountAgencyOwners conta os agency_owner ativos da org (safeguard).
	CountAgencyOwners(ctx context.Context, organizationID string) (int, error)
	// IsAgencyOwner diz se o usuario e agency_owner da org.
	IsAgencyOwner(ctx context.Context, organizationID, userID string) (bool, error)
	// UnlinkUserFromOrganization remove o vinculo de agencia + desativa membership
	// na conta-agencia da org.
	UnlinkUserFromOrganization(ctx context.Context, organizationID, userID string) error
}

// AdminUserRepository abstrai persistencia para os endpoints admin de users.
type AdminUserRepository interface {
	// ListUsers devolve a projecao LEAN (AdminUserListItem) — so os campos que a
	// tabela /manage/users renderiza. O detalhe completo (FindAdminUser) usa AdminUserView.
	ListUsers(ctx context.Context, filter AdminUserListFilter) ([]AdminUserListItem, int, error)
	FindAdminUser(ctx context.Context, userID string) (AdminUserView, error)
	CreateUser(ctx context.Context, input AdminCreateUserInput, passwordHash string) (AdminUserView, error)
	// UpdateUser aplica o patch. passwordHash != "" => SET password_hash +
	// must_change_password = false; "" => nao toca no hash (regra: senha so muda
	// com acao explicita). O service e quem hasheia; o repo so persiste.
	UpdateUser(ctx context.Context, userID string, input AdminUpdateUserInput, passwordHash string) (AdminUserView, error)
	SoftDeleteUser(ctx context.Context, userID string) error
	GetMemberships(ctx context.Context, userID string) ([]AccountMembershipView, error)
	// SetUserAccountRole troca o papel do usuario numa conta: remove os
	// user_role_assignments atuais dele naquela conta e atribui o novo papel
	// (clonando o role do template se faltar). Idempotente.
	SetUserAccountRole(ctx context.Context, accountID, userID, role string) error
	// IsAccountMember diz se o usuario ja e membro (account_users) da conta.
	IsAccountMember(ctx context.Context, accountID, userID string) (bool, error)
	CountActivePlatformAdmins(ctx context.Context) (int, error)
	// MoveUserAccount (transacional) MOVE o usuario para a conta-cliente destino:
	// remove os vinculos de CLIENTE atuais (account_users NAO-agencia + os
	// user_role_assignments dessas contas) e matricula no destino reusando o
	// enroll (membership + papel + perms). NAO toca vinculos de agencia
	// (is_agency=true). Valida que o destino existe, esta ativo e NAO e agencia.
	MoveUserAccount(ctx context.Context, userID, targetAccountID, role string) (AdminUserView, error)
}
