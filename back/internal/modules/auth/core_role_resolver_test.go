package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCoarseRoleFromCoreRolePreservesLegacyRoles(t *testing.T) {
	cases := []struct {
		name       string
		roleCode   string
		templateID string
		want       Role
	}{
		{"owner", "queue.owner", "queue.supervisor", RoleOwner},
		{"director", "queue.director", "queue.supervisor", RoleDirector},
		{"marketing", "queue.marketing", "queue.consultant", RoleMarketing},
		{"manager", "queue.manager", "queue.supervisor", RoleManager},
		{"consultant", "queue.consultant", "queue.consultant", RoleConsultant},
		{"store terminal", "queue.store_terminal", "queue.supervisor", RoleStoreTerminal},
		{"core owner fallback", "core.owner", "core.owner", RoleOwner},
		{"generic queue supervisor", "queue.supervisor", "queue.supervisor", RoleDirector},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := CoarseRoleFromCoreRole(testCase.roleCode, testCase.templateID)
			if got != testCase.want {
				t.Fatalf("got %q want %q", got, testCase.want)
			}
		})
	}
}

func TestCoreOnlyResolvedUserLogsIn(t *testing.T) {
	user := User{
		ID:           "user-core-only",
		DisplayName:  "Core Only",
		Email:        "core-only@example.test",
		PasswordHash: "hash:secret",
		Role:         CoarseRoleFromCoreRole("queue.owner", "queue.supervisor"),
		TenantID:     "account-1",
		StoreIDs:     []string{"store-1", "store-2"},
		Active:       true,
		CreatedAt:    time.Now(),
	}

	service := NewService(
		&fakeUserRepository{findByEmailUser: user},
		fakePasswordHasher{},
		fakeTokenManager{},
		nil,
		nil,
		nil,
		nil,
	)

	result, err := service.Login(context.Background(), LoginInput{
		Email:    "CORE-ONLY@example.test",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if result.User.Role != RoleOwner {
		t.Fatalf("role = %q, want %q", result.User.Role, RoleOwner)
	}
	if result.User.TenantID != "account-1" {
		t.Fatalf("tenant = %q, want account-1", result.User.TenantID)
	}
	if len(result.User.StoreIDs) != 2 {
		t.Fatalf("store IDs = %v, want two stores", result.User.StoreIDs)
	}
}

func TestEmptyScopeUserLogsIn(t *testing.T) {
	// Authn != authz: usuario ATIVO sem papel-coarse resolvido (so-agencia ou
	// so-papel-custom) NAO pode levar 403 no login. Espelha o platform_admin:
	// autentica com escopo vazio; a autorizacao real vem depois, por account.
	user := User{
		ID:           "user-no-coarse-role",
		DisplayName:  "Agency Only",
		Email:        "agency-only@example.test",
		PasswordHash: "hash:secret",
		Role:         "",
		TenantID:     "",
		StoreIDs:     []string{},
		Active:       true,
		CreatedAt:    time.Now(),
	}

	service := NewService(
		&fakeUserRepository{findByEmailUser: user},
		fakePasswordHasher{},
		fakeTokenManager{},
		nil,
		nil,
		nil,
		nil,
	)

	result, err := service.Login(context.Background(), LoginInput{
		Email:    "agency-only@example.test",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("Login returned error for empty-scope active user: %v", err)
	}
	if result.User.Role != "" {
		t.Fatalf("role = %q, want empty", result.User.Role)
	}
	if result.User.TenantID != "" || len(result.User.StoreIDs) != 0 {
		t.Fatalf("empty scope = tenant %q stores %v, want empty", result.User.TenantID, result.User.StoreIDs)
	}
}

func TestHasEmptyScope(t *testing.T) {
	if !HasEmptyScope(User{Role: ""}) {
		t.Fatal("expected empty Role to be empty scope")
	}
	if HasEmptyScope(User{Role: RoleDirector, TenantID: "t1"}) {
		t.Fatal("expected resolved role to NOT be empty scope")
	}
}

func TestPlatformAdminResolvedFromCoreLogsIn(t *testing.T) {
	user := User{
		ID:           "platform-user",
		DisplayName:  "Platform Admin",
		Email:        "platform@example.test",
		PasswordHash: "hash:secret",
		Role:         RolePlatformAdmin,
		Active:       true,
		CreatedAt:    time.Now(),
	}

	service := NewService(
		&fakeUserRepository{findByEmailUser: user},
		fakePasswordHasher{},
		fakeTokenManager{},
		nil,
		nil,
		nil,
		nil,
	)

	result, err := service.Login(context.Background(), LoginInput{
		Email:    "platform@example.test",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if result.User.Role != RolePlatformAdmin {
		t.Fatalf("role = %q, want %q", result.User.Role, RolePlatformAdmin)
	}
	if result.User.TenantID != "" || len(result.User.StoreIDs) != 0 {
		t.Fatalf("platform scope = tenant %q stores %v, want empty", result.User.TenantID, result.User.StoreIDs)
	}
}

type fakeUserRepository struct {
	findByEmailUser User
}

func (repo *fakeUserRepository) FindByEmail(context.Context, string) (User, error) {
	if repo.findByEmailUser.ID == "" {
		return User{}, ErrInvalidCredentials
	}
	return repo.findByEmailUser, nil
}

func (repo *fakeUserRepository) FindByID(context.Context, string) (User, error) {
	return User{}, ErrUnauthorized
}

func (repo *fakeUserRepository) LoadUserForAuth(context.Context, string) (User, error) {
	return User{}, ErrUnauthorized
}

func (repo *fakeUserRepository) UpdateProfile(context.Context, string, string, string) (User, error) {
	return User{}, ErrUnauthorized
}

func (repo *fakeUserRepository) UpdatePassword(context.Context, string, string, bool) (User, error) {
	return User{}, ErrUnauthorized
}

func (repo *fakeUserRepository) UpdateAvatarPath(context.Context, string, string) (User, error) {
	return User{}, ErrUnauthorized
}

type fakePasswordHasher struct{}

func (fakePasswordHasher) Hash(password string) (string, error) {
	return "hash:" + password, nil
}

func (fakePasswordHasher) Verify(hash string, password string) error {
	if hash != "hash:"+password {
		return errors.New("password mismatch")
	}
	return nil
}

type fakeTokenManager struct{}

func (fakeTokenManager) Issue(string, User) (SessionView, error) {
	return SessionView{
		AccessToken: "token",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
	}, nil
}

func (fakeTokenManager) Parse(string) (Principal, error) {
	return Principal{}, ErrUnauthorized
}
