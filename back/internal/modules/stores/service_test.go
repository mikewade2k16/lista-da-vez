package stores

import (
	"context"
	"errors"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type repositorySpy struct {
	listInput         ListInput
	findStore         Store
	findErr           error
	deleteDeps        []DeleteDependency
	deleteDepsErr     error
	deleteErr         error
	deleteCalledStore string
}

func (spy *repositorySpy) ListAccessible(_ context.Context, _ auth.Principal, input ListInput) ([]Store, error) {
	spy.listInput = input
	return []Store{}, nil
}

func (spy *repositorySpy) FindAccessibleByID(_ context.Context, _ auth.Principal, _ string) (Store, error) {
	if spy.findErr != nil {
		return Store{}, spy.findErr
	}
	return spy.findStore, nil
}

func (spy *repositorySpy) Create(_ context.Context, store Store) (Store, error) {
	return store, nil
}

func (spy *repositorySpy) Update(_ context.Context, store Store) (Store, error) {
	return store, nil
}

func (spy *repositorySpy) ListDeleteDependencies(_ context.Context, _ string) ([]DeleteDependency, error) {
	if spy.deleteDepsErr != nil {
		return nil, spy.deleteDepsErr
	}
	return spy.deleteDeps, nil
}

func (spy *repositorySpy) Delete(_ context.Context, storeID string) error {
	spy.deleteCalledStore = storeID
	return spy.deleteErr
}

func TestListAccessiblePassesIncludeInactive(t *testing.T) {
	repository := &repositorySpy{}
	service := NewService(repository, nil)

	_, err := service.ListAccessible(context.Background(), auth.Principal{
		UserID:   "user-1",
		Role:     auth.RoleOwner,
		TenantID: "tenant-1",
	}, ListInput{
		IncludeInactive: true,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if !repository.listInput.IncludeInactive {
		t.Fatalf("expected includeInactive to be forwarded")
	}
}

func TestDeleteBlocksWhenDependenciesExist(t *testing.T) {
	repository := &repositorySpy{
		findStore: Store{
			ID:       "store-1",
			TenantID: "tenant-1",
			Name:     "Loja 1",
			Code:     "LOJA1",
		},
		deleteDeps: []DeleteDependency{
			{Key: "consultants", Label: "Consultores cadastrados", Count: 2},
		},
	}
	service := NewService(repository, nil)

	_, err := service.Delete(context.Background(), auth.Principal{
		UserID:   "user-1",
		Role:     auth.RoleOwner,
		TenantID: "tenant-1",
	}, "store-1")
	if err == nil {
		t.Fatalf("expected delete to fail")
	}

	var blocked *DeleteBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("expected DeleteBlockedError, got %v", err)
	}

	if blocked.StoreID != "store-1" {
		t.Fatalf("expected store id store-1, got %s", blocked.StoreID)
	}

	if repository.deleteCalledStore != "" {
		t.Fatalf("expected delete not to be called")
	}
}

func TestDeleteRemovesStoreWhenNoDependencies(t *testing.T) {
	repository := &repositorySpy{
		findStore: Store{
			ID:       "store-1",
			TenantID: "tenant-1",
			Name:     "Loja 1",
			Code:     "LOJA1",
		},
	}
	service := NewService(repository, nil)

	result, err := service.Delete(context.Background(), auth.Principal{
		UserID:   "user-1",
		Role:     auth.RoleOwner,
		TenantID: "tenant-1",
	}, "store-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if !result.Deleted {
		t.Fatalf("expected deleted result")
	}

	if repository.deleteCalledStore != "store-1" {
		t.Fatalf("expected delete to be called for store-1, got %s", repository.deleteCalledStore)
	}
}
