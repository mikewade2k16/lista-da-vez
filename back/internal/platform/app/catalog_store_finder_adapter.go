package app

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/catalog"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

type catalogStoreFinderAdapter struct {
	storeService *stores.Service
}

func newCatalogStoreFinderAdapter(storeService *stores.Service) *catalogStoreFinderAdapter {
	return &catalogStoreFinderAdapter{storeService: storeService}
}

func (adapter *catalogStoreFinderAdapter) FindAccessible(ctx context.Context, access catalog.AccessContext, storeID string) (catalog.AccessibleStore, error) {
	principal := auth.Principal{
		UserID:   strings.TrimSpace(access.UserID),
		TenantID: strings.TrimSpace(access.TenantID),
		Role:     auth.Role(strings.TrimSpace(access.Role)),
		StoreIDs: append([]string{}, access.StoreIDs...),
	}

	store, err := adapter.storeService.FindAccessible(ctx, principal, strings.TrimSpace(storeID))
	if err != nil {
		return catalog.AccessibleStore{}, err
	}

	return catalog.AccessibleStore{
		ID:       store.ID,
		TenantID: store.TenantID,
		Code:     store.Code,
		Name:     store.Name,
		City:     store.City,
	}, nil
}
