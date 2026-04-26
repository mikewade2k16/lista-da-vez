package settings

import (
	"context"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type fakeRepository struct {
	defaultTenantID string
	resolveErr      error
	accessible      map[string]bool
	records         map[string]Record
}

func (repository *fakeRepository) TenantExists(context.Context, string) (bool, error) {
	return true, nil
}

func (repository *fakeRepository) CanAccessTenant(_ context.Context, _ auth.Principal, tenantID string) (bool, error) {
	if repository.accessible == nil {
		return true, nil
	}

	return repository.accessible[tenantID], nil
}

func (repository *fakeRepository) ResolveDefaultTenantID(context.Context, auth.Principal) (string, error) {
	if repository.resolveErr != nil {
		return "", repository.resolveErr
	}

	if repository.defaultTenantID == "" {
		return "", ErrTenantRequired
	}

	return repository.defaultTenantID, nil
}

func (repository *fakeRepository) GetByTenant(_ context.Context, tenantID string) (Record, bool, error) {
	record, ok := repository.records[tenantID]
	return record, ok, nil
}

func (repository *fakeRepository) Upsert(context.Context, Record) (Record, error) {
	return Record{}, nil
}

func (repository *fakeRepository) UpsertConfig(context.Context, Record) (Record, error) {
	return Record{}, nil
}

func (repository *fakeRepository) ReplaceOptionGroup(context.Context, string, string, []OptionItem) (time.Time, error) {
	return time.Time{}, nil
}

func (repository *fakeRepository) UpsertOption(context.Context, string, string, OptionItem) (time.Time, error) {
	return time.Time{}, nil
}

func (repository *fakeRepository) DeleteOption(context.Context, string, string, string) (time.Time, error) {
	return time.Time{}, nil
}

func (repository *fakeRepository) ReplaceProducts(context.Context, string, []ProductItem) (time.Time, error) {
	return time.Time{}, nil
}

func (repository *fakeRepository) UpsertProduct(context.Context, string, ProductItem) (time.Time, error) {
	return time.Time{}, nil
}

func (repository *fakeRepository) DeleteProduct(context.Context, string, string) (time.Time, error) {
	return time.Time{}, nil
}

func TestGetBundleResolvesDefaultTenantForGlobalPrincipal(t *testing.T) {
	service := NewService(&fakeRepository{
		defaultTenantID: "tenant-1",
		records: map[string]Record{
			"tenant-1": {
				TenantID:                    "tenant-1",
				SelectedOperationTemplateID: defaultTemplateID,
				Settings:                    DefaultBundle("tenant-1", defaultTemplateID).Settings,
				ModalConfig:                 DefaultBundle("tenant-1", defaultTemplateID).ModalConfig,
			},
		},
	}, nil)

	bundle, err := service.GetBundle(context.Background(), auth.Principal{
		UserID: "user-1",
		Role:   auth.RolePlatformAdmin,
	}, "")
	if err != nil {
		t.Fatalf("GetBundle returned error: %v", err)
	}

	if bundle.TenantID != "tenant-1" {
		t.Fatalf("expected tenant-1, got %q", bundle.TenantID)
	}
}

func TestGetBundleRejectsAmbiguousGlobalPrincipal(t *testing.T) {
	service := NewService(&fakeRepository{
		resolveErr: ErrTenantRequired,
		records:    map[string]Record{},
	}, nil)

	if _, err := service.GetBundle(context.Background(), auth.Principal{
		UserID: "user-1",
		Role:   auth.RolePlatformAdmin,
	}, ""); err != ErrTenantRequired {
		t.Fatalf("expected ErrTenantRequired, got %v", err)
	}
}

func TestGetBundleUsesRequestedTenantForGlobalPrincipal(t *testing.T) {
	service := NewService(&fakeRepository{
		resolveErr: ErrTenantRequired,
		accessible: map[string]bool{
			"tenant-2": true,
		},
		records: map[string]Record{
			"tenant-2": {
				TenantID:                    "tenant-2",
				SelectedOperationTemplateID: defaultTemplateID,
				Settings:                    DefaultBundle("tenant-2", defaultTemplateID).Settings,
				ModalConfig:                 DefaultBundle("tenant-2", defaultTemplateID).ModalConfig,
			},
		},
	}, nil)

	bundle, err := service.GetBundle(context.Background(), auth.Principal{
		UserID: "user-1",
		Role:   auth.RolePlatformAdmin,
	}, "tenant-2")
	if err != nil {
		t.Fatalf("GetBundle returned error: %v", err)
	}

	if bundle.TenantID != "tenant-2" {
		t.Fatalf("expected tenant-2, got %q", bundle.TenantID)
	}
}
