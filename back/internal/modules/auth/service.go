package auth

import (
	"context"
	"errors"
	"strings"
	"time"
)

// PrincipalCacheStore e a interface minima que o Service usa para cachear Principals.
// Mantida aqui para evitar importar httpapi (ciclo de importacao).
type PrincipalCacheStore interface {
	Get(sessionID string) (Principal, bool)
	Set(sessionID, userID string, p Principal)
}

type Service struct {
	users              UserRepository
	password           PasswordHasher
	tokens             TokenManager
	avatars            AvatarStorage
	permissions        PermissionResolver
	notifier           ContextPublisher
	consultantProfiles ConsultantProfileSync
	sessions           SessionRepository   // opcional; nil = nao persiste sessoes
	principalCache     PrincipalCacheStore // opcional; nil = sem cache
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

// SetSessionRepository habilita criacao e verificacao de sessoes em core.user_sessions.
func (service *Service) SetSessionRepository(sessions SessionRepository) {
	service.sessions = sessions
}

// SetPrincipalCache habilita o cache de Principals com TTL para reduzir DB round-trips.
func (service *Service) SetPrincipalCache(cache PrincipalCacheStore) {
	service.principalCache = cache
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

	var sessionID string
	if service.sessions != nil {
		id, err := service.sessions.Create(ctx, user.ID, "", "")
		if err != nil {
			return LoginResult{}, err
		}
		sessionID = id
	}

	session, err := service.tokens.Issue(sessionID, user)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		User:    user.View(),
		Session: session,
	}, nil
}

// Logout revoga a sessao do principal em core.user_sessions, invalidando o token
// ja emitido (Authenticate passa a retornar 401 para esse `sid`). Idempotente e
// no-op para tokens legados sem `sid` ou quando o repo de sessao nao esta ligado.
func (service *Service) Logout(ctx context.Context, principal Principal) error {
	if service.sessions == nil || strings.TrimSpace(principal.SessionID) == "" {
		return nil
	}

	return service.sessions.Revoke(ctx, principal.SessionID)
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

	// Cache hit: evitar DB se Principal ainda e valido (TTL 2min).
	// Tokens legados (sem SessionID) ignoram o cache.
	if principal.SessionID != "" && service.principalCache != nil {
		if cached, ok := service.principalCache.Get(principal.SessionID); ok {
			return cached, nil
		}
	}

	// Cache miss ou token legado: checar se sessao foi revogada antes de ir ao banco.
	if principal.SessionID != "" && service.sessions != nil {
		revoked, err := service.sessions.IsRevoked(ctx, principal.SessionID)
		if err != nil {
			return Principal{}, err
		}
		if revoked {
			return Principal{}, ErrUnauthorized
		}
	}

	// Hot-path: LoadUserForAuth centraliza user + role + escopo usando a fonte configurada
	// (core, legado ou core com fallback) antes de resolver permissoes efetivas.
	user, err := service.users.LoadUserForAuth(ctx, principal.UserID)
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
	principal.Nick = user.Nick
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

	// Armazenar no cache para proximas requests.
	if principal.SessionID != "" && service.principalCache != nil {
		service.principalCache.Set(principal.SessionID, principal.UserID, principal)
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
