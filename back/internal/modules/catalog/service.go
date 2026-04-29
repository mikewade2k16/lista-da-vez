package catalog

import (
	"context"
	"strings"
	"unicode/utf8"
)

const (
	defaultProductSearchLimit  = 10
	maxProductSearchLimit      = 25
	minProductSearchTermLength = 3
)

type Service struct {
	repository  Repository
	storeFinder StoreFinder
}

func NewService(repository Repository, storeFinder StoreFinder) *Service {
	return &Service{
		repository:  repository,
		storeFinder: storeFinder,
	}
}

func (service *Service) SearchProducts(ctx context.Context, access AccessContext, input SearchProductsInput) (SearchProductsResponse, error) {
	storeID := strings.TrimSpace(input.StoreID)
	if storeID == "" {
		return SearchProductsResponse{}, ErrStoreRequired
	}

	store, err := service.storeFinder.FindAccessible(ctx, access, storeID)
	if err != nil {
		return SearchProductsResponse{}, err
	}

	return service.searchProductsByQuery(ctx, SearchProductsQuery{
		StoreScope: StoreScope{
			TenantID:  store.TenantID,
			StoreID:   store.ID,
			StoreCode: store.Code,
		},
		SourceKey: input.SourceKey,
		Term:      input.Term,
		Limit:     input.Limit,
	})
}

func (service *Service) searchProductsByQuery(ctx context.Context, query SearchProductsQuery) (SearchProductsResponse, error) {
	normalized := SearchProductsQuery{
		StoreScope: StoreScope{
			TenantID:  strings.TrimSpace(query.TenantID),
			StoreID:   strings.TrimSpace(query.StoreID),
			StoreCode: strings.TrimSpace(query.StoreCode),
		},
		SourceKey: query.SourceKey,
		Term:      strings.ToUpper(strings.TrimSpace(query.Term)),
		Limit:     query.Limit,
	}

	if normalized.TenantID == "" {
		return SearchProductsResponse{}, ErrTenantRequired
	}
	if normalized.StoreID == "" {
		return SearchProductsResponse{}, ErrStoreRequired
	}
	if normalized.SourceKey == "" {
		normalized.SourceKey = ProductSourceERPCurrent
	}
	if utf8.RuneCountInString(normalized.Term) < minProductSearchTermLength {
		return SearchProductsResponse{}, ErrSearchTermTooShort
	}
	switch {
	case normalized.Limit <= 0:
		normalized.Limit = defaultProductSearchLimit
	case normalized.Limit > maxProductSearchLimit:
		normalized.Limit = maxProductSearchLimit
	}

	return service.repository.SearchProducts(ctx, normalized)
}
