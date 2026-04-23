package auth

import (
	"context"
	"strings"
	"time"
)

type MemoryUserStore struct {
	byEmail map[string]User
	byID    map[string]User
}

func NewMemoryUserStore(users []User) (*MemoryUserStore, error) {
	store := &MemoryUserStore{
		byEmail: make(map[string]User, len(users)),
		byID:    make(map[string]User, len(users)),
	}

	for _, user := range users {
		if err := ValidateUserScope(user); err != nil {
			return nil, err
		}

		normalizedEmail := strings.ToLower(strings.TrimSpace(user.Email))
		user.Email = normalizedEmail
		user.StoreIDs = append([]string{}, user.StoreIDs...)

		store.byEmail[normalizedEmail] = user
		store.byID[user.ID] = user
	}

	return store, nil
}

func (store *MemoryUserStore) FindByEmail(_ context.Context, email string) (User, error) {
	user, ok := store.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return User{}, ErrInvalidCredentials
	}

	return user, nil
}

func (store *MemoryUserStore) FindByID(_ context.Context, id string) (User, error) {
	user, ok := store.byID[strings.TrimSpace(id)]
	if !ok {
		return User{}, ErrUnauthorized
	}

	return user, nil
}

func SeedDemoUsers(hasher PasswordHasher) ([]User, error) {
	type seed struct {
		id          string
		displayName string
		email       string
		role        Role
		tenantID    string
		storeIDs    []string
	}

	seeds := []seed{
		{
			id:          "usr-consultor-demo",
			displayName: "Consultor Demo",
			email:       "consultor@demo.local",
			role:        RoleConsultant,
			tenantID:    DemoTenantID,
			storeIDs:    []string{StoreRiomarID},
		},
		{
			id:          "usr-gerente-demo",
			displayName: "Gerente Demo",
			email:       "gerente@demo.local",
			role:        RoleManager,
			tenantID:    DemoTenantID,
			storeIDs:    []string{StoreRiomarID},
		},
		{
			id:          "usr-marketing-demo",
			displayName: "Marketing Demo",
			email:       "marketing@demo.local",
			role:        RoleMarketing,
			tenantID:    DemoTenantID,
			storeIDs:    []string{StoreRiomarID, StoreJardinsID},
		},
		{
			id:          "usr-owner-demo",
			displayName: "Proprietario Demo",
			email:       "proprietario@demo.local",
			role:        RoleOwner,
			tenantID:    DemoTenantID,
			storeIDs:    []string{StoreRiomarID, StoreJardinsID},
		},
		{
			id:          "usr-platform-admin-demo",
			displayName: "Platform Admin Demo",
			email:       "plataforma@demo.local",
			role:        RolePlatformAdmin,
			tenantID:    "",
			storeIDs:    nil,
		},
	}

	users := make([]User, 0, len(seeds))
	for _, item := range seeds {
		passwordHash, err := hasher.Hash("dev123456")
		if err != nil {
			return nil, err
		}

		users = append(users, User{
			ID:                 item.id,
			DisplayName:        item.displayName,
			Email:              item.email,
			PasswordHash:       passwordHash,
			MustChangePassword: false,
			Role:               item.role,
			TenantID:           item.tenantID,
			StoreIDs:           append([]string{}, item.storeIDs...),
			Active:             true,
			CreatedAt:          time.Now().UTC(),
		})
	}

	return users, nil
}
