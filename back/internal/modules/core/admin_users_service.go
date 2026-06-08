package core

import (
	"context"
	"errors"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// AdminUserService orquestra regras de negocio dos endpoints /v1/admin/users.
// Valida unicidade de email, hash de senha e safeguard do ultimo platform_admin.
type AdminUserService struct {
	repo   AdminUserRepository
	hasher *auth.BcryptHasher
}

// NewAdminUserService cria o service com as dependencias necessarias.
func NewAdminUserService(repo AdminUserRepository, hasher *auth.BcryptHasher) *AdminUserService {
	return &AdminUserService{repo: repo, hasher: hasher}
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

// GetUser devolve um user pelo id.
func (s *AdminUserService) GetUser(ctx context.Context, userID string) (AdminUserView, error) {
	return s.repo.FindAdminUser(ctx, userID)
}

// CreateUser valida input, hasheia a senha (se fornecida) e cria o user.
func (s *AdminUserService) CreateUser(ctx context.Context, input AdminCreateUserInput) (AdminUserView, error) {
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

// UpdateUser valida o patch e aplica safeguard: nao permitir rebaixar/desativar
// o ultimo platform_admin ativo (evita perda total de acesso).
func (s *AdminUserService) UpdateUser(ctx context.Context, userID string, input AdminUpdateUserInput) (AdminUserView, error) {
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

	return s.repo.UpdateUser(ctx, userID, input)
}

// DeleteUser aplica safeguard antes de soft-delete (chamado direto).
func (s *AdminUserService) DeleteUser(ctx context.Context, userID string) error {
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
	return s.repo.SoftDeleteUser(ctx, userID)
}

// GetMemberships devolve as accounts que o user e membro.
func (s *AdminUserService) GetMemberships(ctx context.Context, userID string) (AdminMembershipsResponse, error) {
	memberships, err := s.repo.GetMemberships(ctx, userID)
	if err != nil {
		return AdminMembershipsResponse{}, err
	}
	return AdminMembershipsResponse{Memberships: memberships}, nil
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
