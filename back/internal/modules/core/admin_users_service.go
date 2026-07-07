package core

import (
	"context"
	"errors"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// AdminUserService orquestra regras de negocio dos endpoints /v1/admin/users.
// Valida unicidade de email, hash de senha e safeguard do ultimo platform_admin.
// scope resolve a autoridade admin por-request (delegacao multi-tenant); links
// cuida dos vinculos de cliente/agencia de um usuario.
type AdminUserService struct {
	repo           AdminUserRepository
	hasher         *auth.BcryptHasher
	scope          *AdminScopeResolver
	links          AdminUserLinksRepository
	principalCache PrincipalCacheInvalidator
}

// SetPrincipalCacheInvalidator liga a invalidacao do PrincipalCache (AC-01) para as
// mutacoes identity-global do painel admin v2 (desativacao / is_platform_admin /
// soft-delete). nil = cache desligado (no-op).
func (s *AdminUserService) SetPrincipalCacheInvalidator(cache PrincipalCacheInvalidator) {
	s.principalCache = cache
}

// NewAdminUserService cria o service com as dependencias necessarias.
func NewAdminUserService(
	repo AdminUserRepository,
	hasher *auth.BcryptHasher,
	scope *AdminScopeResolver,
	links AdminUserLinksRepository,
) *AdminUserService {
	return &AdminUserService{repo: repo, hasher: hasher, scope: scope, links: links}
}

// ListUsers passa filtros para o repositorio e devolve a resposta paginada.
func (s *AdminUserService) ListUsers(ctx context.Context, filter AdminUserListFilter) (AdminUserListResponse, error) {
	users, total, err := s.repo.ListUsers(ctx, filter)
	if err != nil {
		return AdminUserListResponse{}, err
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	return AdminUserListResponse{
		Users:   users,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	}, nil
}

// GetUser devolve um user pelo id, ESCOPADO ao ator: se o ator nao administra o
// usuario (CanManageUser) -> 404 (nao vaza existencia de usuario de outro tenant).
func (s *AdminUserService) GetUser(ctx context.Context, actorUserID, userID string) (AdminUserView, error) {
	can, err := s.scope.CanManageUser(ctx, actorUserID, userID)
	if err != nil {
		return AdminUserView{}, err
	}
	if !can {
		return AdminUserView{}, ErrUserNotFound
	}
	return s.repo.FindAdminUser(ctx, userID)
}

// CreateUser cria uma identidade global — acao identity-global, SO platform_admin
// (mesmo que o ator administre a account/agencia informada no payload). Admin de
// org/cliente vincula um usuario JA existente via POST memberships/organizations.
func (s *AdminUserService) CreateUser(ctx context.Context, actorUserID string, input AdminCreateUserInput) (AdminUserView, error) {
	isAdmin, err := s.scope.IsPlatformAdmin(ctx, actorUserID)
	if err != nil {
		return AdminUserView{}, err
	}
	if !isAdmin {
		return AdminUserView{}, ErrForbidden
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Nick = strings.TrimSpace(input.Nick)
	input.TemporaryPassword = strings.TrimSpace(input.TemporaryPassword)
	input.Role = strings.TrimSpace(input.Role)

	if input.Email == "" || input.DisplayName == "" {
		return AdminUserView{}, errors.New("email and displayName are required")
	}

	// Quando vincula a um cliente (AccountID), o user precisa de papel no tenant
	// (user_tenant_roles legado) para conseguir logar e aparecer na operacao.
	if strings.TrimSpace(input.AccountID) != "" {
		if input.Role == "" {
			input.Role = "owner"
		}
		switch input.Role {
		case "owner", "director", "marketing":
		default:
			return AdminUserView{}, errors.New("role must be owner, director or marketing")
		}
	}

	// Quando vincula a uma agencia (OrganizationID), o cargo define o acesso: o repo
	// matricula o user na conta-agencia (owner para dono, director para membro) para
	// que ele consiga logar. Default agency_member.
	if strings.TrimSpace(input.OrganizationID) != "" {
		input.OrgRole = strings.TrimSpace(input.OrgRole)
		if input.OrgRole == "" {
			input.OrgRole = "agency_member"
		}
		switch input.OrgRole {
		case "agency_owner", "agency_member":
		default:
			return AdminUserView{}, errors.New("orgRole must be agency_owner or agency_member")
		}
	}

	// Auto-gera nick a partir do displayName quando vazio. Mesmo padrao do front
	// (buildNickname em person-display.ts) para consistencia entre camadas.
	if input.Nick == "" {
		input.Nick = BuildNickname(input.DisplayName, 18)
	}

	var passwordHash string
	if input.TemporaryPassword != "" {
		if len(input.TemporaryPassword) < 8 {
			return AdminUserView{}, errors.New("temporaryPassword must be at least 8 chars")
		}
		hash, err := s.hasher.Hash(input.TemporaryPassword)
		if err != nil {
			return AdminUserView{}, err
		}
		passwordHash = hash
	}

	return s.repo.CreateUser(ctx, input, passwordHash)
}

// UpdateUser valida o patch e aplica os gates de delegacao. Todos os campos do
// AdminUpdateUserInput sao identity-global (displayName/nick/email/password/
// isPlatformAdmin/isActive) -> SO platform_admin pode altera-los. Um ator
// nao-platform_admin que administre o usuario (escopo) mas envie QUALQUER campo
// nao-nil -> 403 forbidden_field (a delegacao da poder sobre o vinculo, nao sobre
// a identidade global). Mantem o safeguard do ultimo platform_admin.
func (s *AdminUserService) UpdateUser(ctx context.Context, actorUserID, userID string, input AdminUpdateUserInput) (AdminUserView, error) {
	// Escopo do alvo primeiro (404 antes de qualquer decisao de campo).
	can, err := s.scope.CanManageUser(ctx, actorUserID, userID)
	if err != nil {
		return AdminUserView{}, err
	}
	if !can {
		return AdminUserView{}, ErrUserNotFound
	}

	// Matriz de campo: identity-global so para platform_admin.
	isAdmin, err := s.scope.IsPlatformAdmin(ctx, actorUserID)
	if err != nil {
		return AdminUserView{}, err
	}
	if !isAdmin && hasIdentityGlobalField(input) {
		return AdminUserView{}, ErrForbiddenField
	}

	if err := s.guardLastPlatformAdmin(ctx, userID, input.IsPlatformAdmin, input.IsActive); err != nil {
		return AdminUserView{}, err
	}

	if input.Email != nil {
		normalized := strings.ToLower(strings.TrimSpace(*input.Email))
		if normalized == "" {
			return AdminUserView{}, errors.New("email cannot be blank")
		}
		input.Email = &normalized
	}
	if input.DisplayName != nil {
		trimmed := strings.TrimSpace(*input.DisplayName)
		if trimmed == "" {
			return AdminUserView{}, errors.New("displayName cannot be blank")
		}
		input.DisplayName = &trimmed
	}

	// Senha: SO mexe no hash quando vem uma senha nao-vazia (acao explicita do
	// admin). nil ou string vazia/em-branco => passwordHash "" => repo nao toca
	// no password_hash (regra critica). Nunca logamos a senha nem o hash.
	var passwordHash string
	if input.Password != nil {
		pw := strings.TrimSpace(*input.Password)
		if pw != "" {
			if len(pw) < 8 {
				return AdminUserView{}, errors.New("password must be at least 8 chars")
			}
			hash, err := s.hasher.Hash(pw)
			if err != nil {
				return AdminUserView{}, err
			}
			passwordHash = hash
		}
	}
	// Evita que o texto puro da senha trafegue alem deste ponto.
	input.Password = nil

	view, err := s.repo.UpdateUser(ctx, userID, input, passwordHash)
	if err == nil && s.principalCache != nil {
		// AC-01: cobre desativacao (isActive) e is_platform_admin via painel admin v2.
		s.principalCache.InvalidateUser(userID)
	}
	return view, err
}

// DeleteUser faz soft-delete da identidade global — acao identity-global, SO
// platform_admin. Mantem o safeguard do ultimo platform_admin ativo. Um ator
// nao-platform_admin -> 403 (forbidden); ele deve usar DELETE membership pontual.
func (s *AdminUserService) DeleteUser(ctx context.Context, actorUserID, userID string) error {
	isAdmin, err := s.scope.IsPlatformAdmin(ctx, actorUserID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return ErrForbidden
	}
	current, err := s.repo.FindAdminUser(ctx, userID)
	if err != nil {
		return err
	}
	if current.IsPlatformAdmin && current.IsActive {
		count, err := s.repo.CountActivePlatformAdmins(ctx)
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrLastPlatformAdmin
		}
	}
	if err := s.repo.SoftDeleteUser(ctx, userID); err != nil {
		return err
	}
	// AC-01: soft-delete desativa a identidade — derruba as sessoes cacheadas.
	if s.principalCache != nil {
		s.principalCache.InvalidateUser(userID)
	}
	return nil
}

// GetMemberships devolve as accounts que o user e membro, ESCOPADO ao ator: o
// ator precisa poder administrar o usuario (CanManageUser, senao 404) e a
// resposta so traz as accounts que o ator administra (getMembershipsScoped —
// senao agency_owner veria vinculos do usuario em clientes de outra agencia).
func (s *AdminUserService) GetMemberships(ctx context.Context, actorUserID, userID string) (AdminMembershipsResponse, error) {
	can, err := s.scope.CanManageUser(ctx, actorUserID, userID)
	if err != nil {
		return AdminMembershipsResponse{}, err
	}
	if !can {
		return AdminMembershipsResponse{}, ErrUserNotFound
	}
	return s.getMembershipsScoped(ctx, actorUserID, userID)
}

// UpdateMembershipRole troca o nivel/papel do usuario numa conta (cliente ou
// conta-agencia). Aceita papeis tenant-scoped (owner/director/marketing) — nao
// exigem vinculo de loja. Escopo: CanManageAccount(accountId) senao 404; se a
// conta for agencia, exige autoridade de organizacao (M2). Exige que o user ja
// seja membro da conta. Devolve as memberships escopadas ao ator.
func (s *AdminUserService) UpdateMembershipRole(ctx context.Context, actorUserID, userID, accountID string, input UpdateMembershipRoleInput) (AdminMembershipsResponse, error) {
	accountID = strings.TrimSpace(accountID)
	role := strings.ToLower(strings.TrimSpace(input.Role))
	switch role {
	case "owner", "director", "marketing":
	default:
		return AdminMembershipsResponse{}, ErrInvalidRole
	}

	if err := s.ensureCanMutateAccountLink(ctx, actorUserID, accountID); err != nil {
		return AdminMembershipsResponse{}, err
	}

	member, err := s.repo.IsAccountMember(ctx, accountID, userID)
	if err != nil {
		return AdminMembershipsResponse{}, err
	}
	if !member {
		return AdminMembershipsResponse{}, ErrUserNotFound
	}

	if err := s.repo.SetUserAccountRole(ctx, accountID, userID, role); err != nil {
		return AdminMembershipsResponse{}, err
	}

	return s.getMembershipsScoped(ctx, actorUserID, userID)
}

// MoveUserAccount MOVE o usuario para a conta-cliente destino: o repositorio
// (transacional) remove os vinculos de CLIENTE atuais (nao toca agencia) e
// matricula no destino. Valida accountId obrigatorio e papel (default "owner").
// Retorna o AdminUserView atualizado (mesmo shape do PATCH) para o front
// atualizar a linha sem refetch.
func (s *AdminUserService) MoveUserAccount(ctx context.Context, actorUserID, userID string, input MoveUserAccountInput) (AdminUserView, error) {
	// MOVE e destrutivo (desativa TODOS os vinculos de cliente atuais) — restrito
	// a platform_admin. Admin de org/cliente usa POST/DELETE membership pontual.
	isAdmin, err := s.scope.IsPlatformAdmin(ctx, actorUserID)
	if err != nil {
		return AdminUserView{}, err
	}
	if !isAdmin {
		return AdminUserView{}, ErrForbidden
	}
	accountID := strings.TrimSpace(input.AccountID)
	if accountID == "" {
		return AdminUserView{}, errors.New("accountId is required")
	}
	role := strings.ToLower(strings.TrimSpace(input.Role))
	if role == "" {
		role = "owner"
	}
	switch role {
	case "owner", "director", "marketing":
	default:
		return AdminUserView{}, ErrInvalidRole
	}
	return s.repo.MoveUserAccount(ctx, userID, accountID, role)
}

// guardLastPlatformAdmin bloqueia rebaixar/desativar o ultimo platform_admin
// ativo. Chamado antes do update real.
func (s *AdminUserService) guardLastPlatformAdmin(ctx context.Context, userID string, nextIsAdmin, nextIsActive *bool) error {
	if nextIsAdmin == nil && nextIsActive == nil {
		return nil
	}
	current, err := s.repo.FindAdminUser(ctx, userID)
	if err != nil {
		return err
	}
	if !current.IsPlatformAdmin || !current.IsActive {
		return nil
	}
	willLosePlatformAdmin := nextIsAdmin != nil && !*nextIsAdmin
	willDeactivate := nextIsActive != nil && !*nextIsActive
	if !willLosePlatformAdmin && !willDeactivate {
		return nil
	}
	count, err := s.repo.CountActivePlatformAdmins(ctx)
	if err != nil {
		return err
	}
	if count <= 1 {
		return ErrLastPlatformAdmin
	}
	return nil
}

// hasIdentityGlobalField diz se o patch toca QUALQUER campo identity-global
// (todos os campos do AdminUpdateUserInput sao globais — nome/nick/email/senha/
// is_platform_admin/is_active). Usado para 403 forbidden_field quando o ator nao
// e platform_admin.
func hasIdentityGlobalField(input AdminUpdateUserInput) bool {
	return input.Email != nil ||
		input.DisplayName != nil ||
		input.Nick != nil ||
		input.IsActive != nil ||
		input.IsPlatformAdmin != nil ||
		input.Password != nil
}
