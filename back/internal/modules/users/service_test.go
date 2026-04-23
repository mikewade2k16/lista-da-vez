package users

import (
	"context"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type serviceTestRepository struct {
	user         User
	createdUser  User
	updatedUser  User
	updateCalled bool
	createCalled bool
	storeScopes  []StoreScope
	findErr      error
	updateErr    error
	createErr    error
	resolveErr   error
}

func (repository *serviceTestRepository) ListAccessible(ctx context.Context, principal auth.Principal, input ListInput) ([]User, error) {
	return nil, nil
}

func (repository *serviceTestRepository) FindAccessibleByID(ctx context.Context, principal auth.Principal, userID string) (User, error) {
	if repository.findErr != nil {
		return User{}, repository.findErr
	}

	return repository.user, nil
}

func (repository *serviceTestRepository) ResolveStoreScopes(ctx context.Context, storeIDs []string) ([]StoreScope, error) {
	if repository.resolveErr != nil {
		return nil, repository.resolveErr
	}

	return repository.storeScopes, nil
}

func (repository *serviceTestRepository) Create(ctx context.Context, user User, passwordHash *string) (User, error) {
	repository.createCalled = true
	repository.createdUser = user
	if repository.createErr != nil {
		return User{}, repository.createErr
	}
	if passwordHash != nil {
		user.HasPassword = true
	}

	return user, nil
}

func (repository *serviceTestRepository) Update(ctx context.Context, user User, passwordHash *string) (User, error) {
	repository.updateCalled = true
	repository.updatedUser = user
	if repository.updateErr != nil {
		return User{}, repository.updateErr
	}

	if passwordHash != nil {
		user.HasPassword = true
	}

	return user, nil
}

type serviceTestHasher struct{}

func (serviceTestHasher) Hash(password string) (string, error) {
	return "hashed:" + password, nil
}

func (serviceTestHasher) Verify(hash, password string) error {
	return nil
}

func TestCreateRejectsConsultantRoleInUsersModule(t *testing.T) {
	repository := &serviceTestRepository{}
	service := NewService(repository, serviceTestHasher{}, nil, nil, nil)

	_, err := service.Create(context.Background(), auth.Principal{
		UserID:   "owner-1",
		Role:     auth.RoleOwner,
		TenantID: "tenant-1",
	}, CreateInput{
		DisplayName: "Consultor",
		Email:       "consultor@demo.local",
		Role:        auth.RoleConsultant,
		StoreIDs:    []string{"store-1"},
	})
	if err != ErrConsultantManaged {
		t.Fatalf("expected ErrConsultantManaged, got %v", err)
	}
	if repository.createCalled {
		t.Fatalf("expected repository.Create not to be called")
	}
}

func TestUpdateRejectsManagedConsultant(t *testing.T) {
	repository := &serviceTestRepository{
		user: User{
			ID:                "user-1",
			DisplayName:       "Consultor",
			Email:             "consultor@demo.local",
			Role:              auth.RoleConsultant,
			TenantID:          "tenant-1",
			StoreIDs:          []string{"store-1"},
			Active:            true,
			HasPassword:       true,
			ManagedBy:         "consultants",
			ManagedResourceID: "consultant-1",
		},
	}
	service := NewService(repository, serviceTestHasher{}, nil, nil, nil)
	newName := "Consultor Editado"

	_, err := service.Update(context.Background(), auth.Principal{
		UserID:   "owner-1",
		Role:     auth.RoleOwner,
		TenantID: "tenant-1",
	}, UpdateInput{
		ID:          "user-1",
		DisplayName: &newName,
	})
	if err != ErrConsultantManaged {
		t.Fatalf("expected ErrConsultantManaged, got %v", err)
	}
	if repository.updateCalled {
		t.Fatalf("expected repository.Update not to be called")
	}
}

func TestUpdateAllowsPlatformAdminToEditManagedConsultant(t *testing.T) {
	repository := &serviceTestRepository{
		user: User{
			ID:                "user-1",
			DisplayName:       "Consultor",
			Email:             "consultor@demo.local",
			EmployeeCode:      "259",
			Role:              auth.RoleConsultant,
			TenantID:          "tenant-1",
			StoreIDs:          []string{"store-1"},
			Active:            true,
			HasPassword:       true,
			ManagedBy:         "consultants",
			ManagedResourceID: "consultant-1",
		},
		storeScopes: []StoreScope{{ID: "store-1", TenantID: "tenant-1", Active: true}},
	}
	service := NewService(repository, serviceTestHasher{}, nil, nil, nil)
	newName := "Consultor Editado"
	newCode := "259A"

	result, err := service.Update(context.Background(), auth.Principal{
		UserID: "platform-1",
		Role:   auth.RolePlatformAdmin,
	}, UpdateInput{
		ID:           "user-1",
		DisplayName:  &newName,
		EmployeeCode: &newCode,
	})
	if err != nil {
		t.Fatalf("expected managed consultant update to succeed, got %v", err)
	}
	if !repository.updateCalled {
		t.Fatalf("expected repository.Update to be called")
	}
	if repository.updatedUser.DisplayName != newName {
		t.Fatalf("expected updated name %q, got %q", newName, repository.updatedUser.DisplayName)
	}
	if repository.updatedUser.EmployeeCode != newCode {
		t.Fatalf("expected updated employee code %q, got %q", newCode, repository.updatedUser.EmployeeCode)
	}
	if result.DisplayName != newName {
		t.Fatalf("expected updated view name %q, got %q", newName, result.DisplayName)
	}
}

func TestArchiveRejectsManagedConsultant(t *testing.T) {
	repository := &serviceTestRepository{
		user: User{
			ID:                "user-1",
			DisplayName:       "Consultor",
			Email:             "consultor@demo.local",
			Role:              auth.RoleConsultant,
			TenantID:          "tenant-1",
			StoreIDs:          []string{"store-1"},
			Active:            true,
			ManagedBy:         "consultants",
			ManagedResourceID: "consultant-1",
		},
	}
	service := NewService(repository, serviceTestHasher{}, nil, nil, nil)

	_, err := service.Archive(context.Background(), auth.Principal{
		UserID:   "owner-1",
		Role:     auth.RoleOwner,
		TenantID: "tenant-1",
	}, "user-1")
	if err != ErrConsultantManaged {
		t.Fatalf("expected ErrConsultantManaged, got %v", err)
	}
	if repository.updateCalled {
		t.Fatalf("expected repository.Update not to be called")
	}
}

func TestArchiveAllowsPlatformAdminToArchiveManagedConsultant(t *testing.T) {
	repository := &serviceTestRepository{
		user: User{
			ID:                "user-1",
			DisplayName:       "Consultor",
			Email:             "consultor@demo.local",
			Role:              auth.RoleConsultant,
			TenantID:          "tenant-1",
			StoreIDs:          []string{"store-1"},
			Active:            true,
			ManagedBy:         "consultants",
			ManagedResourceID: "consultant-1",
		},
	}
	service := NewService(repository, serviceTestHasher{}, nil, nil, nil)

	result, err := service.Archive(context.Background(), auth.Principal{
		UserID: "platform-1",
		Role:   auth.RolePlatformAdmin,
	}, "user-1")
	if err != nil {
		t.Fatalf("expected archive to succeed, got %v", err)
	}
	if !repository.updateCalled {
		t.Fatalf("expected repository.Update to be called")
	}
	if repository.updatedUser.Active {
		t.Fatalf("expected updated user to be archived")
	}
	if result.Active {
		t.Fatalf("expected archived view to be inactive")
	}
}

func TestResetPasswordKeepsAdministrativePathForManagedConsultant(t *testing.T) {
	repository := &serviceTestRepository{
		user: User{
			ID:                "user-1",
			DisplayName:       "Consultor",
			Email:             "consultor@demo.local",
			Role:              auth.RoleConsultant,
			TenantID:          "tenant-1",
			StoreIDs:          []string{"store-1"},
			Active:            true,
			HasPassword:       true,
			ManagedBy:         "consultants",
			ManagedResourceID: "consultant-1",
		},
	}
	service := NewService(repository, serviceTestHasher{}, nil, nil, nil)

	result, err := service.ResetPassword(context.Background(), auth.Principal{
		UserID: "platform-1",
		Role:   auth.RolePlatformAdmin,
	}, "user-1", "NovaSenha123")
	if err != nil {
		t.Fatalf("expected reset to succeed, got %v", err)
	}
	if !repository.updateCalled {
		t.Fatalf("expected repository.Update to be called")
	}
	if !repository.updatedUser.MustChangePassword {
		t.Fatalf("expected managed consultant reset to require password change")
	}
	if result.TemporaryPassword != "NovaSenha123" {
		t.Fatalf("unexpected temporary password result: %q", result.TemporaryPassword)
	}
}

func TestCreateRejectsManualPasswordForOwner(t *testing.T) {
	repository := &serviceTestRepository{}
	service := NewService(repository, serviceTestHasher{}, nil, nil, nil)

	_, err := service.Create(context.Background(), auth.Principal{
		UserID:   "owner-1",
		Role:     auth.RoleOwner,
		TenantID: "tenant-1",
	}, CreateInput{
		DisplayName: "Gerente Manual",
		Email:       "gerente@demo.local",
		Password:    "Senha1234",
		Role:        auth.RoleManager,
		StoreIDs:    []string{"store-1"},
	})
	if err != ErrPasswordForbidden {
		t.Fatalf("expected ErrPasswordForbidden, got %v", err)
	}
	if repository.createCalled {
		t.Fatalf("expected repository.Create not to be called")
	}
}

func TestCreateAllowsManualPasswordForPlatformAdmin(t *testing.T) {
	repository := &serviceTestRepository{}
	service := NewService(repository, serviceTestHasher{}, nil, nil, nil)

	result, err := service.Create(context.Background(), auth.Principal{
		UserID: "platform-1",
		Role:   auth.RolePlatformAdmin,
	}, CreateInput{
		DisplayName: "Diretoria Manual",
		Email:       "diretoria@demo.local",
		Password:    "Senha1234",
		Role:        auth.RoleDirector,
		TenantID:    "tenant-1",
	})
	if err != nil {
		t.Fatalf("expected manual password create to succeed, got %v", err)
	}
	if !repository.createCalled {
		t.Fatalf("expected repository.Create to be called")
	}
	if !repository.createdUser.MustChangePassword {
		t.Fatalf("expected created user to require password change")
	}
	if !result.User.Onboarding.HasPassword {
		t.Fatalf("expected created view to reflect password presence")
	}
	if !result.User.Onboarding.MustChangePassword {
		t.Fatalf("expected created view to require password change")
	}
}

func TestUpdateRejectsManualPasswordForOwner(t *testing.T) {
	repository := &serviceTestRepository{
		user: User{
			ID:          "user-1",
			DisplayName: "Gerente",
			Email:       "gerente@demo.local",
			Role:        auth.RoleManager,
			TenantID:    "tenant-1",
			StoreIDs:    []string{"store-1"},
			Active:      true,
			HasPassword: true,
		},
	}
	service := NewService(repository, serviceTestHasher{}, nil, nil, nil)
	password := "Senha1234"

	_, err := service.Update(context.Background(), auth.Principal{
		UserID:   "owner-1",
		Role:     auth.RoleOwner,
		TenantID: "tenant-1",
	}, UpdateInput{
		ID:       "user-1",
		Password: &password,
	})
	if err != ErrPasswordForbidden {
		t.Fatalf("expected ErrPasswordForbidden, got %v", err)
	}
	if repository.updateCalled {
		t.Fatalf("expected repository.Update not to be called")
	}
}

func TestResetPasswordRejectsOwner(t *testing.T) {
	repository := &serviceTestRepository{
		user: User{
			ID:          "user-1",
			DisplayName: "Gerente",
			Email:       "gerente@demo.local",
			Role:        auth.RoleManager,
			TenantID:    "tenant-1",
			StoreIDs:    []string{"store-1"},
			Active:      true,
			HasPassword: true,
		},
	}
	service := NewService(repository, serviceTestHasher{}, nil, nil, nil)

	_, err := service.ResetPassword(context.Background(), auth.Principal{
		UserID:   "owner-1",
		Role:     auth.RoleOwner,
		TenantID: "tenant-1",
	}, "user-1", "NovaSenha123")
	if err != ErrPasswordForbidden {
		t.Fatalf("expected ErrPasswordForbidden, got %v", err)
	}
	if repository.updateCalled {
		t.Fatalf("expected repository.Update not to be called")
	}
}
