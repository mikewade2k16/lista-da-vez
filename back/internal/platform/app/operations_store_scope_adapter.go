package app

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

type operationsStoreScopeAdapter struct {
	storeService *stores.Service
}

func newOperationsStoreScopeAdapter(storeService *stores.Service) *operationsStoreScopeAdapter {
	return &operationsStoreScopeAdapter{storeService: storeService}
}

func (adapter *operationsStoreScopeAdapter) ListAccessible(
	ctx context.Context,
	access operations.AccessContext,
	filter operations.StoreScopeFilter,
) ([]operations.StoreScopeView, error) {
	principal := auth.Principal{
		UserID:   strings.TrimSpace(access.UserID),
		TenantID: strings.TrimSpace(access.TenantID),
		Role:     auth.Role(strings.TrimSpace(access.Role)),
		StoreIDs: append([]string{}, access.StoreIDs...),
	}

	rows, err := adapter.storeService.ListAccessible(ctx, principal, stores.ListInput{
		TenantID: strings.TrimSpace(filter.TenantID),
	})
	if err != nil {
		return nil, err
	}

	result := make([]operations.StoreScopeView, 0, len(rows))
	for _, row := range rows {
		result = append(result, operations.StoreScopeView{
			ID:       row.ID,
			TenantID: row.TenantID,
			Code:     row.Code,
			Name:     row.Name,
			City:     row.City,
		})
	}

	return result, nil
}
