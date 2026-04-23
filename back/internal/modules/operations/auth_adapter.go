package operations

import "github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"

func AccessContextFromPrincipal(principal auth.Principal) AccessContext {
	storeIDs := make([]string, 0, len(principal.StoreIDs))
	for _, storeID := range principal.StoreIDs {
		if storeID != "" {
			storeIDs = append(storeIDs, storeID)
		}
	}

	return AccessContext{
		UserID:   principal.UserID,
		TenantID: principal.TenantID,
		Role:     string(principal.Role),
		StoreIDs: storeIDs,
	}
}
