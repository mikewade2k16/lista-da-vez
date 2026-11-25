package operations

import (
	"context"
	"errors"
	"testing"
)

func newRecentServicesTestService(repository *testOperationsRepository) *Service {
	return NewService(repository, nil, testStoreScopeProvider{
		stores: []StoreScopeView{
			{ID: "store-a", Name: "Loja A"},
			{ID: "store-b", Name: "Loja B"},
		},
	})
}

func TestRecentServicesRestrictedToPlatformAdmin(t *testing.T) {
	service := newRecentServicesTestService(&testOperationsRepository{})
	for _, role := range []string{RoleOwner, RoleManager, RoleConsultant, RoleDirector} {
		_, err := service.RecentServices(context.Background(), AccessContext{Role: role}, RecentServicesInput{})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("role %s: expected ErrForbidden, got %v", role, err)
		}
	}
}

func TestRecentServicesRejectsStoreOutsideScope(t *testing.T) {
	repository := &testOperationsRepository{}
	service := newRecentServicesTestService(repository)
	_, err := service.RecentServices(context.Background(), AccessContext{Role: RolePlatformAdmin}, RecentServicesInput{StoreID: "store-x"})
	if !errors.Is(err, ErrStoreNotFound) {
		t.Fatalf("expected ErrStoreNotFound, got %v", err)
	}
	if repository.recentStoreIDs != nil {
		t.Fatalf("repository must not be queried for out-of-scope store, got %v", repository.recentStoreIDs)
	}
}

func TestRecentServicesUsesAccessibleStoresAndFillsStoreName(t *testing.T) {
	repository := &testOperationsRepository{
		recentItems: []RecentServiceItem{{ServiceID: "s1", StoreID: "store-b", PersonName: "Diana"}},
	}
	service := newRecentServicesTestService(repository)

	result, err := service.RecentServices(context.Background(), AccessContext{Role: RolePlatformAdmin}, RecentServicesInput{PerPage: 500})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(repository.recentStoreIDs) != 2 {
		t.Fatalf("expected all accessible stores, got %v", repository.recentStoreIDs)
	}
	if result.PerPage != recentServicesMaxPerPage || result.Page != 1 {
		t.Fatalf("expected clamped paging, got page=%d perPage=%d", result.Page, result.PerPage)
	}
	if len(result.Items) != 1 || result.Items[0].StoreName != "Loja B" || result.Total != 1 {
		t.Fatalf("unexpected result %+v", result)
	}

	if _, err := service.RecentServices(context.Background(), AccessContext{Role: RolePlatformAdmin}, RecentServicesInput{StoreID: "store-a"}); err != nil {
		t.Fatalf("expected success for scoped store, got %v", err)
	}
	if len(repository.recentStoreIDs) != 1 || repository.recentStoreIDs[0] != "store-a" {
		t.Fatalf("expected only store-a, got %v", repository.recentStoreIDs)
	}
}
