package consultants

import (
	"context"
	"strings"

	usersmodule "github.com/mikewade2k16/lista-da-vez/back/internal/modules/users"
)

type ProfileSync struct {
	repository Repository
}

func NewProfileSync(repository Repository) *ProfileSync {
	return &ProfileSync{repository: repository}
}

func (syncer *ProfileSync) SyncLinkedProfile(ctx context.Context, userID string, displayName string) error {
	if syncer == nil || syncer.repository == nil {
		return nil
	}

	trimmedUserID := strings.TrimSpace(userID)
	trimmedDisplayName := strings.TrimSpace(displayName)
	if trimmedUserID == "" || trimmedDisplayName == "" {
		return nil
	}

	return syncer.repository.SyncLinkedIdentity(ctx, trimmedUserID, trimmedDisplayName, buildInitials(trimmedDisplayName))
}

func (syncer *ProfileSync) SyncLinkedAccess(ctx context.Context, user usersmodule.User) error {
	if syncer == nil || syncer.repository == nil {
		return nil
	}

	trimmedUserID := strings.TrimSpace(user.ID)
	trimmedDisplayName := strings.TrimSpace(user.DisplayName)
	if trimmedUserID == "" || trimmedDisplayName == "" {
		return nil
	}

	trimmedStoreID := ""
	if len(user.StoreIDs) > 0 {
		trimmedStoreID = strings.TrimSpace(user.StoreIDs[0])
	}

	return syncer.repository.SyncLinkedAccess(ctx, LinkedAccessSyncInput{
		UserID:       trimmedUserID,
		DisplayName:  trimmedDisplayName,
		EmployeeCode: strings.TrimSpace(user.EmployeeCode),
		TenantID:     strings.TrimSpace(user.TenantID),
		StoreID:      trimmedStoreID,
		Role:         user.Role,
		Active:       user.Active,
	})
}
