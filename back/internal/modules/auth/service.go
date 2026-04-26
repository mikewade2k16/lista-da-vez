package auth

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Service struct {
	users              UserRepository
	password           PasswordHasher
	tokens             TokenManager
	avatars            AvatarStorage
	permissions        PermissionResolver
	notifier           ContextPublisher
	consultantProfiles ConsultantProfileSync
}

type ContextPublisher interface {
	PublishContextEvent(ctx context.Context, tenantID string, resource string, action string, resourceID string, savedAt time.Time)
}

type ConsultantProfileSync interface {
	SyncLinkedProfile(ctx context.Context, userID string, displayName string) error
}

func NewService(users UserRepository, password PasswordHasher, tokens TokenManager, avatars AvatarStorage, permissions PermissionResolver, notifier ContextPublisher, consultantProfiles ConsultantProfileSync) *Service {
	return &Service{
		users:              users,
		password:           password,
		tokens:             tokens,
		avatars:            avatars,
		permissions:        permissions,
		notifier:           notifier,
		consultantProfiles: consultantProfiles,
	}
}

func (service *Service) SetContextPublisher(notifier ContextPublisher) {
	service.notifier = notifier
}

func (service *Service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := input.Password

	if email == "" || password == "" {
		return LoginResult{}, ErrInvalidCredentials
	}

	user, err := service.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return LoginResult{}, ErrInvalidCredentials
		}

		return LoginResult{}, err
	}

	if !user.Active {
		return LoginResult{}, ErrUserInactive
	}

	if strings.TrimSpace(user.PasswordHash) == "" {
		return LoginResult{}, ErrOnboardingRequired
	}

	if err := service.password.Verify(user.PasswordHash, password); err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	session, err := service.tokens.Issue(user)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		User:    user.View(),
		Session: session,
	}, nil
}

func (service *Service) Authenticate(ctx context.Context, authorizationHeader string) (Principal, error) {
	token, err := ExtractBearerToken(authorizationHeader)
	if err != nil {
		return Principal{}, err
	}

	return service.AuthenticateToken(ctx, token)
}

func (service *Service) AuthenticateToken(ctx context.Context, token string) (Principal, error) {
	principal, err := service.tokens.Parse(token)
	if err != nil {
		return Principal{}, err
	}

	user, err := service.users.FindByID(ctx, principal.UserID)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return Principal{}, ErrUnauthorized
		}

		return Principal{}, err
	}

	if !user.Active {
		return Principal{}, ErrUserInactive
	}

	principal.DisplayName = user.DisplayName
	principal.Email = user.Email
	principal.Role = user.Role
	principal.TenantID = user.TenantID
	principal.StoreIDs = append([]string{}, user.StoreIDs...)
	if service.permissions != nil {
		permissionKeys, err := service.permissions.ResolveUserPermissions(ctx, user.ID, user.Role)
		if err != nil {
			return Principal{}, err
		}

		principal.Permissions = append([]string{}, permissionKeys...)
		principal.PermissionsResolved = true
	}

	return principal, nil
}

func (service *Service) CurrentUser(ctx context.Context, principal Principal) (UserView, error) {
	user, err := service.users.FindByID(ctx, principal.UserID)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return UserView{}, ErrUnauthorized
		}

		return UserView{}, err
	}

	if !user.Active {
		return UserView{}, ErrUserInactive
	}

	return user.View(), nil
}

func (service *Service) UpdateProfile(ctx context.Context, principal Principal, input UpdateProfileInput) (UserView, error) {
	displayName := strings.TrimSpace(input.DisplayName)
	email := strings.ToLower(strings.TrimSpace(input.Email))

	if displayName == "" || email == "" {
		return UserView{}, ErrInvalidCredentials
	}

	updated, err := service.users.UpdateProfile(ctx, principal.UserID, displayName, email)
	if err != nil {
		return UserView{}, err
	}

	if service.consultantProfiles != nil {
		if err := service.consultantProfiles.SyncLinkedProfile(ctx, updated.ID, updated.DisplayName); err != nil {
			return UserView{}, err
		}
	}

	service.publishContextEvent(ctx, updated.TenantID, "profile", "updated", updated.ID)
	return updated.View(), nil
}

func (service *Service) ChangePassword(ctx context.Context, principal Principal, input ChangePasswordInput) (UserView, error) {
	currentPassword := strings.TrimSpace(input.CurrentPassword)
	newPassword := strings.TrimSpace(input.NewPassword)

	if currentPassword == "" || len(newPassword) < 8 {
		return UserView{}, ErrInvalidCredentials
	}

	user, err := service.users.FindByID(ctx, principal.UserID)
	if err != nil {
		return UserView{}, err
	}

	if err := service.password.Verify(user.PasswordHash, currentPassword); err != nil {
		return UserView{}, ErrInvalidCredentials
	}

	passwordHash, err := service.password.Hash(newPassword)
	if err != nil {
		return UserView{}, err
	}

	updated, err := service.users.UpdatePassword(ctx, principal.UserID, passwordHash, false)
	if err != nil {
		return UserView{}, err
	}

	return updated.View(), nil
}

func (service *Service) UpdateAvatar(ctx context.Context, principal Principal, input UpdateAvatarInput) (UserView, error) {
	if service.avatars == nil {
		return UserView{}, ErrInvalidAvatar
	}

	user, err := service.users.FindByID(ctx, principal.UserID)
	if err != nil {
		return UserView{}, err
	}

	avatarPath, err := service.avatars.Save(
		ctx,
		user.ID,
		input.FileName,
		input.ContentType,
		input.Content,
		user.AvatarPath,
	)
	if err != nil {
		return UserView{}, err
	}

	updated, err := service.users.UpdateAvatarPath(ctx, user.ID, avatarPath)
	if err != nil {
		return UserView{}, err
	}

	service.publishContextEvent(ctx, updated.TenantID, "profile", "avatar-updated", updated.ID)
	return updated.View(), nil
}

func (service *Service) publishContextEvent(ctx context.Context, tenantID string, resource string, action string, resourceID string) {
	if service.notifier == nil || strings.TrimSpace(tenantID) == "" {
		return
	}

	service.notifier.PublishContextEvent(ctx, tenantID, resource, action, resourceID, time.Now().UTC())
}
