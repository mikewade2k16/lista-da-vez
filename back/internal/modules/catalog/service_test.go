package catalog

import (
	"context"
	"errors"
	"testing"
)

type serviceTestRepository struct {
	lastQuery SearchProductsQuery
	response  SearchProductsResponse
	err       error
}

func (repository *serviceTestRepository) SearchProducts(_ context.Context, query SearchProductsQuery) (SearchProductsResponse, error) {
	repository.lastQuery = query
	return repository.response, repository.err
}

type serviceTestStoreFinder struct {
	lastAccess  AccessContext
	lastStoreID string
	store       AccessibleStore
	err         error
}

func (finder *serviceTestStoreFinder) FindAccessible(_ context.Context, access AccessContext, storeID string) (AccessibleStore, error) {
	finder.lastAccess = access
	finder.lastStoreID = storeID
	return finder.store, finder.err
}

func TestSearchProductsNormalizesQuery(t *testing.T) {
	repository := &serviceTestRepository{
		response: SearchProductsResponse{
			Items: []ProductSearchItem{{ID: "123", Code: "123", Name: "Produto", Price: 19.9}},
		},
	}
	storeFinder := &serviceTestStoreFinder{
		store: AccessibleStore{
			ID:       "store-id",
			TenantID: "tenant-id",
			Code:     "PJ-JAR",
		},
	}
	service := NewService(repository, storeFinder)

	response, err := service.SearchProducts(context.Background(), AccessContext{
		UserID:   "user-1",
		TenantID: "tenant-id",
		Role:     "consultant",
		StoreIDs: []string{"store-id"},
	}, SearchProductsInput{
		StoreID: " store-id ",
		Term:    " 12a ",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if storeFinder.lastStoreID != "store-id" {
		t.Fatalf("expected trimmed store id for store lookup, got %q", storeFinder.lastStoreID)
	}
	if repository.lastQuery.SourceKey != ProductSourceERPCurrent {
		t.Fatalf("expected default source %q, got %q", ProductSourceERPCurrent, repository.lastQuery.SourceKey)
	}
	if repository.lastQuery.Term != "12A" {
		t.Fatalf("expected normalized term %q, got %q", "12A", repository.lastQuery.Term)
	}
	if repository.lastQuery.Limit != defaultProductSearchLimit {
		t.Fatalf("expected default limit %d, got %d", defaultProductSearchLimit, repository.lastQuery.Limit)
	}
	if repository.lastQuery.TenantID != "tenant-id" {
		t.Fatalf("expected resolved tenant id, got %q", repository.lastQuery.TenantID)
	}
	if repository.lastQuery.StoreID != "store-id" {
		t.Fatalf("expected resolved store id, got %q", repository.lastQuery.StoreID)
	}
	if len(response.Items) != 1 {
		t.Fatalf("expected one item, got %d", len(response.Items))
	}
}

func TestSearchProductsRejectsShortTerm(t *testing.T) {
	service := NewService(&serviceTestRepository{}, &serviceTestStoreFinder{
		store: AccessibleStore{
			ID:       "store-id",
			TenantID: "tenant-id",
			Code:     "PJ-JAR",
		},
	})

	_, err := service.SearchProducts(context.Background(), AccessContext{}, SearchProductsInput{
		StoreID: "store-id",
		Term:    "12",
	})
	if !errors.Is(err, ErrSearchTermTooShort) {
		t.Fatalf("expected ErrSearchTermTooShort, got %v", err)
	}
}

func TestSearchProductsRequiresStoreID(t *testing.T) {
	service := NewService(&serviceTestRepository{}, &serviceTestStoreFinder{})

	_, err := service.SearchProducts(context.Background(), AccessContext{}, SearchProductsInput{
		Term: "123",
	})
	if !errors.Is(err, ErrStoreRequired) {
		t.Fatalf("expected ErrStoreRequired, got %v", err)
	}
}
