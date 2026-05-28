package users

import (
	"context"
	"strings"
	"time"

	accesscontrol "github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type Service struct {
	repository         Repository
	password           auth.PasswordHasher
	invites            InvitationIssuer
	notifier           ContextPublisher
	consultantProfiles ConsultantProfileSync
}

type ContextPublisher interface {
	PublishContextEvent(ctx context.Context, tenantID string, resource string, action string, resourceID string, savedAt time.Time)
}

type ConsultantProfileSync interface {
	SyncLinkedAccess(ctx context.Context, user User) error
}

type InvitationIssuer interface {
	Issue(ctx context.Context, input auth.InvitationIssueInput) (auth.InvitationIssueResult, error)
}

func NewService(repository Repository, password auth.PasswordHasher, invites InvitationIssuer, notifier ContextPublisher, consultantProfiles ConsultantProfileSync) *Service {
	return &Service{
		repository:         repository,
		password:           password,
		invites:            invites,
		notifier:           notifier,
		consultantProfiles: consultantProfiles,
	}
}

func (service *Service) ListAccessible(ctx context.Context, principal auth.Principal, input ListInput) ([]UserView, error) {
	if !canManageUsers(principal) {
		return nil, ErrForbidden
	}

	normalizedInput := input
	normalizedInput.TenantID = resolveTenantFilter(principal, input.TenantID)
	normalizedInput.StoreID = strings.TrimSpace(input.StoreID)

	users, err := service.repository.ListAccessible(ctx, principal, normalizedInput)
	if err != nil {
		return nil, err
	}

	views := make([]UserView, 0, len(users))
	for _, user := range users {
		views = append(views, user.View())
	}

	return views, nil
}

func (service *Service) Create(ctx context.Context, principal auth.Principal, input CreateInput) (CreateResult, error) {
	if !canManageUsers(principal) {
		return CreateResult{}, ErrForbidden
	}

	displayName := strings.TrimSpace(input.DisplayName)
	email := normalizeEmail(input.Email)
	employeeCode := strings.TrimSpace(input.EmployeeCode)
	password := strings.TrimSpace(input.Password)
	role := input.Role

	if displayName == "" || email == "" || !auth.IsValidRole(role) {
		return CreateResult{}, ErrValidation
	}
	if role == auth.RoleConsultant {
		return CreateResult{}, ErrConsultantManaged
	}
	if password != "" && !canManageUserPasswords(principal) {
		return CreateResult{}, ErrPasswordForbidden
	}

	active := true
	if input.Active != nil {
		active = *input.Active
	}

	tenantID, storeIDs, err := service.resolveScopedAccess(ctx, principal, role, input.TenantID, input.StoreIDs)
	if err != nil {
		return CreateResult{}, err
	}

	var passwordHash *string
	if password != "" {
		hashedPassword, err := service.password.Hash(password)
		if err != nil {
			return CreateResult{}, err
		}

		passwordHash = &hashedPassword
	}

	created, err := service.repository.Create(ctx, User{
		DisplayName:        displayName,
		Email:              email,
		EmployeeCode:       employeeCode,
		JobTitle:           defaultJobTitleForRole(role),
		Role:               role,
		TenantID:           tenantID,
		StoreIDs:           storeIDs,
		Active:             active,
		MustChangePassword: shouldRequirePasswordRotation(role, passwordHash != nil),
	}, passwordHash)
	if err != nil {
		return CreateResult{}, err
	}

	result := CreateResult{
		User: created.View(),
	}

	if service.invites != nil && passwordHash == nil && created.Active {
		invitation, err := service.invites.Issue(ctx, auth.InvitationIssueInput{
			User: auth.User{
				ID:           created.ID,
				DisplayName:  created.DisplayName,
				Email:        created.Email,
				PasswordHash: "",
				Role:         created.Role,
				TenantID:     created.TenantID,
				StoreIDs:     cloneStringSlice(created.StoreIDs),
				Active:       created.Active,
				CreatedAt:    created.CreatedAt,
			},
			InvitedByUserID: principal.UserID,
		})
		if err != nil {
			return CreateResult{}, err
		}

		refreshed, err := service.repository.FindAccessibleByID(ctx, principal, created.ID)
		if err != nil {
			return CreateResult{}, err
		}

		result.User = refreshed.View()
		result.Invitation = &invitation
	}

	service.publishContextEvent(ctx, created.TenantID, "user", "created", created.ID)
	return result, nil
}

func (service *Service) Update(ctx context.Context, principal auth.Principal, input UpdateInput) (UserView, error) {
	if !canManageUsers(principal) {
		return UserView{}, ErrForbidden
	}

	userID := strings.TrimSpace(input.ID)
	if userID == "" {
		return UserView{}, ErrValidation
	}

	existing, err := service.repository.FindAccessibleByID(ctx, principal, userID)
	if err != nil {
		return UserView{}, err
	}
	managedConsultant := isConsultantManaged(existing)
	if managedConsultant && !canOverrideConsultantManagement(principal.Role) {
		return UserView{}, ErrConsultantManaged
	}

	if input.DisplayName != nil {
		existing.DisplayName = strings.TrimSpace(*input.DisplayName)
	}

	if input.Email != nil {
		existing.Email = normalizeEmail(*input.Email)
	}

	if input.EmployeeCode != nil {
		existing.EmployeeCode = strings.TrimSpace(*input.EmployeeCode)
	}

	if input.Active != nil {
		existing.Active = *input.Active
	}

	nextRole := existing.Role
	if input.Role != nil {
		nextRole = *input.Role
	}
	if nextRole == auth.RoleConsultant && !managedConsultant && !canOverrideConsultantManagement(principal.Role) {
		return UserView{}, ErrConsultantManaged
	}

	nextTenantID := existing.TenantID
	if input.TenantID != nil {
		nextTenantID = strings.TrimSpace(*input.TenantID)
	}

	nextStoreIDs := cloneStringSlice(existing.StoreIDs)
	if input.StoreIDs != nil {
		nextStoreIDs = cloneStringSlice(*input.StoreIDs)
	}

	trimmedPassword := ""
	if input.Password != nil {
		trimmedPassword = strings.TrimSpace(*input.Password)
		if trimmedPassword == "" {
			return UserView{}, ErrValidation
		}
		if !canManageUserPasswords(principal) {
			return UserView{}, ErrPasswordForbidden
		}
	}

	if existing.DisplayName == "" || existing.Email == "" || !auth.IsValidRole(nextRole) {
		return UserView{}, ErrValidation
	}

	resolvedTenantID, resolvedStoreIDs, err := service.resolveScopedAccess(ctx, principal, nextRole, nextTenantID, nextStoreIDs)
	if err != nil {
		return UserView{}, err
	}

	existing.Role = nextRole
	existing.JobTitle = reconcileJobTitle(existing.JobTitle, nextRole)
	existing.TenantID = resolvedTenantID
	existing.StoreIDs = resolvedStoreIDs

	var passwordHash *string
	if trimmedPassword != "" {
		hashedPassword, err := service.password.Hash(trimmedPassword)
		if err != nil {
			return UserView{}, err
		}

		passwordHash = &hashedPassword
		existing.MustChangePassword = shouldRequirePasswordRotation(existing.Role, true)
	}

	updated, err := service.repository.Update(ctx, existing, passwordHash)
	if err != nil {
		return UserView{}, err
	}

	if service.consultantProfiles != nil {
		if err := service.consultantProfiles.SyncLinkedAccess(ctx, updated); err != nil {
			return UserView{}, err
		}
	}

	service.publishContextEvents(ctx, uniqueTenantIDs(existing.TenantID, updated.TenantID), "user", "updated", updated.ID)
	return updated.View(), nil
}

func (service *Service) Archive(ctx context.Context, principal auth.Principal, userID string) (UserView, error) {
	if !canManageUsers(principal) {
		return UserView{}, ErrForbidden
	}

	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return UserView{}, ErrValidation
	}

	if trimmedUserID == strings.TrimSpace(principal.UserID) {
		return UserView{}, ErrSelfArchive
	}

	existing, err := service.repository.FindAccessibleByID(ctx, principal, trimmedUserID)
	if err != nil {
		return UserView{}, err
	}
	if isConsultantManaged(existing) && !canOverrideConsultantManagement(principal.Role) {
		return UserView{}, ErrConsultantManaged
	}

	existing.Active = false

	updated, err := service.repository.Update(ctx, existing, nil)
	if err != nil {
		return UserView{}, err
	}

	if service.consultantProfiles != nil {
		if err := service.consultantProfiles.SyncLinkedAccess(ctx, updated); err != nil {
			return UserView{}, err
		}
	}

	service.publishContextEvent(ctx, updated.TenantID, "user", "archived", updated.ID)
	return updated.View(), nil
}

func (service *Service) Invite(ctx context.Context, principal auth.Principal, userID string) (InviteResult, error) {
	if !canManageUsers(principal) {
		return InviteResult{}, ErrForbidden
	}

	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" || service.invites == nil {
		return InviteResult{}, ErrValidation
	}

	user, err := service.repository.FindAccessibleByID(ctx, principal, trimmedUserID)
	if err != nil {
		return InviteResult{}, err
	}
	if isConsultantManaged(user) && !canOverrideConsultantManagement(principal.Role) {
		return InviteResult{}, ErrConsultantManaged
	}

	if !user.Active || user.HasPassword {
		return InviteResult{}, ErrInviteNotAllowed
	}

	invitation, err := service.invites.Issue(ctx, auth.InvitationIssueInput{
		User: auth.User{
			ID:           user.ID,
			DisplayName:  user.DisplayName,
			Email:        user.Email,
			PasswordHash: "",
			Role:         user.Role,
			TenantID:     user.TenantID,
			StoreIDs:     cloneStringSlice(user.StoreIDs),
			Active:       user.Active,
			CreatedAt:    user.CreatedAt,
		},
		InvitedByUserID: principal.UserID,
	})
	if err != nil {
		return InviteResult{}, err
	}

	refreshed, err := service.repository.FindAccessibleByID(ctx, principal, user.ID)
	if err != nil {
		return InviteResult{}, err
	}

	service.publishContextEvent(ctx, refreshed.TenantID, "user", "invited", refreshed.ID)
	return InviteResult{
		User:       refreshed.View(),
		Invitation: invitation,
	}, nil
}

func (service *Service) ResetPassword(ctx context.Context, principal auth.Principal, userID string, password string) (ResetPasswordResult, error) {
	if !canManageUsers(principal) {
		return ResetPasswordResult{}, ErrForbidden
	}
	if !canManageUserPasswords(principal) {
		return ResetPasswordResult{}, ErrPasswordForbidden
	}

	trimmedUserID := strings.TrimSpace(userID)
	trimmedPassword := strings.TrimSpace(password)
	if trimmedUserID == "" || len(trimmedPassword) < 8 {
		return ResetPasswordResult{}, ErrValidation
	}

	user, err := service.repository.FindAccessibleByID(ctx, principal, trimmedUserID)
	if err != nil {
		return ResetPasswordResult{}, err
	}

	passwordHash, err := service.password.Hash(trimmedPassword)
	if err != nil {
		return ResetPasswordResult{}, err
	}

	user.MustChangePassword = shouldRequirePasswordRotation(user.Role, true)
	updated, err := service.repository.Update(ctx, user, &passwordHash)
	if err != nil {
		return ResetPasswordResult{}, err
	}

	service.publishContextEvent(ctx, updated.TenantID, "user", "password-reset", updated.ID)
	return ResetPasswordResult{
		User:              updated.View(),
		TemporaryPassword: trimmedPassword,
	}, nil
}

func (service *Service) resolveScopedAccess(ctx context.Context, principal auth.Principal, role auth.Role, tenantID string, storeIDs []string) (string, []string, error) {
	if !auth.IsValidRole(role) {
		return "", nil, ErrValidation
	}

	normalizedTenantID := strings.TrimSpace(tenantID)
	normalizedStoreIDs := normalizeStoreIDs(storeIDs)

	if principal.Role != auth.RolePlatformAdmin && role == auth.RolePlatformAdmin {
		return "", nil, ErrForbidden
	}

	switch role {
	case auth.RolePlatformAdmin:
		if principal.Role != auth.RolePlatformAdmin {
			return "", nil, ErrForbidden
		}

		return "", nil, nil
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		resolvedTenantID := resolveTenantFilter(principal, normalizedTenantID)
		if resolvedTenantID == "" {
			return "", nil, ErrTenantRequired
		}

		return resolvedTenantID, nil, nil
	case auth.RoleManager, auth.RoleConsultant, auth.RoleStoreTerminal:
		if len(normalizedStoreIDs) == 0 {
			return "", nil, ErrStoreRequired
		}
		if len(normalizedStoreIDs) != 1 {
			return "", nil, ErrInvalidStoreScope
		}

		storeScopes, err := service.repository.ResolveStoreScopes(ctx, normalizedStoreIDs)
		if err != nil {
			return "", nil, err
		}

		if len(storeScopes) != len(normalizedStoreIDs) {
			return "", nil, ErrInvalidStoreScope
		}

		resolvedTenantID := ""
		validatedStoreIDs := make([]string, 0, len(storeScopes))

		for _, storeScope := range storeScopes {
			if !storeScope.Active {
				return "", nil, ErrInvalidStoreScope
			}

			if resolvedTenantID == "" {
				resolvedTenantID = storeScope.TenantID
			}

			if strings.TrimSpace(storeScope.TenantID) != resolvedTenantID {
				return "", nil, ErrInvalidStoreScope
			}

			validatedStoreIDs = append(validatedStoreIDs, storeScope.ID)
		}

		if principal.Role == auth.RoleOwner {
			if resolvedTenantID == "" || principal.TenantID != resolvedTenantID {
				return "", nil, ErrForbidden
			}
		}

		if principal.Role == auth.RolePlatformAdmin && normalizedTenantID != "" && normalizedTenantID != resolvedTenantID {
			return "", nil, ErrValidation
		}

		return resolvedTenantID, validatedStoreIDs, nil
	default:
		return "", nil, ErrValidation
	}
}

func canManageUsers(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionUsersEdit)
	}

	return principal.Role == auth.RoleOwner || principal.Role == auth.RolePlatformAdmin
}

func canManageUserPasswords(principal auth.Principal) bool {
	if principal.PermissionsResolved {
		return accesscontrol.HasPermission(principal.Permissions, accesscontrol.PermissionUsersPasswordEdit)
	}

	return principal.Role == auth.RolePlatformAdmin
}

func canOverrideConsultantManagement(role auth.Role) bool {
	return role == auth.RolePlatformAdmin
}

func resolveTenantFilter(principal auth.Principal, tenantID string) string {
	normalizedTenantID := strings.TrimSpace(tenantID)

	if principal.Role == auth.RolePlatformAdmin {
		return normalizedTenantID
	}

	if principal.TenantID == "" {
		return ""
	}

	if normalizedTenantID == "" || normalizedTenantID == principal.TenantID {
		return principal.TenantID
	}

	return ""
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeStoreIDs(storeIDs []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(storeIDs))

	for _, rawStoreID := range storeIDs {
		storeID := strings.TrimSpace(rawStoreID)
		if storeID == "" {
			continue
		}

		if _, ok := seen[storeID]; ok {
			continue
		}

		seen[storeID] = struct{}{}
		normalized = append(normalized, storeID)
	}

	return normalized
}

func cloneStringSlice(values []string) []string {
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			cloned = append(cloned, trimmed)
		}
	}

	return cloned
}

func shouldRequirePasswordRotation(role auth.Role, passwordDefinedByAdmin bool) bool {
	if !passwordDefinedByAdmin {
		return false
	}

	return role != auth.RoleStoreTerminal
}

func defaultJobTitleForRole(role auth.Role) string {
	switch role {
	case auth.RoleConsultant:
		return "Consultor de Atendimento"
	case auth.RoleStoreTerminal:
		return "Terminal da Loja"
	case auth.RoleManager:
		return "Gerente de Loja"
	case auth.RoleMarketing:
		return "Marketing"
	case auth.RoleDirector:
		return "Diretoria"
	case auth.RoleOwner:
		return "Gestao do Tenant"
	case auth.RolePlatformAdmin:
		return "Admin da Plataforma"
	default:
		return ""
	}
}

func reconcileJobTitle(current string, role auth.Role) string {
	trimmed := strings.TrimSpace(current)
	if trimmed == "" {
		return defaultJobTitleForRole(role)
	}

	return trimmed
}

func isConsultantManaged(user User) bool {
	return user.Role == auth.RoleConsultant || strings.TrimSpace(user.ManagedBy) == "consultants"
}

func (service *Service) publishContextEvent(ctx context.Context, tenantID string, resource string, action string, resourceID string) {
	if service.notifier == nil || strings.TrimSpace(tenantID) == "" {
		return
	}

	service.notifier.PublishContextEvent(ctx, tenantID, resource, action, resourceID, time.Now().UTC())
}

func (service *Service) publishContextEvents(ctx context.Context, tenantIDs []string, resource string, action string, resourceID string) {
	for _, tenantID := range uniqueTenantIDs(tenantIDs...) {
		service.publishContextEvent(ctx, tenantID, resource, action, resourceID)
	}
}

func uniqueTenantIDs(values ...string) []string {
	seen := map[string]struct{}{}
	tenantIDs := make([]string, 0, len(values))

	for _, value := range values {
		tenantID := strings.TrimSpace(value)
		if tenantID == "" {
			continue
		}

		if _, ok := seen[tenantID]; ok {
			continue
		}

		seen[tenantID] = struct{}{}
		tenantIDs = append(tenantIDs, tenantID)
	}

	return tenantIDs
}
