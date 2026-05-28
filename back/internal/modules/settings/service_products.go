package settings

import (
	"context"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func (service *Service) SaveProductSection(ctx context.Context, principal auth.Principal, input ProductSectionInput) (MutationAck, error) {
	tenantID, currentProducts, _, err := service.loadWritableProductCatalog(ctx, principal, input.TenantID)
	if err != nil {
		return MutationAck{}, err
	}

	savedAt, err := service.repository.ReplaceProducts(ctx, tenantID, normalizeProducts(input.Items, currentProducts))
	if err != nil {
		return MutationAck{}, err
	}

	ack := MutationAck{
		OK:       true,
		TenantID: tenantID,
		SavedAt:  savedAt,
	}

	return service.finalizeMutation(ctx, ack, nil)
}

func (service *Service) SaveProductItem(ctx context.Context, principal auth.Principal, input ProductItemInput) (MutationAck, error) {
	tenantID, currentProducts, seededDefaults, err := service.loadWritableProductCatalog(ctx, principal, input.TenantID)
	if err != nil {
		return MutationAck{}, err
	}

	normalizedProducts := normalizeProducts([]ProductItem{input.Item}, nil)
	if len(normalizedProducts) != 1 {
		return MutationAck{}, ErrValidation
	}

	var savedAt time.Time
	if seededDefaults {
		nextProducts, _ := upsertProductCatalogItem(currentProducts, normalizedProducts[0])
		savedAt, err = service.repository.ReplaceProducts(ctx, tenantID, nextProducts)
	} else {
		savedAt, err = service.repository.UpsertProduct(ctx, tenantID, normalizedProducts[0])
	}
	if err != nil {
		return MutationAck{}, err
	}

	ack := MutationAck{
		OK:       true,
		TenantID: tenantID,
		SavedAt:  savedAt,
	}

	return service.finalizeMutation(ctx, ack, nil)
}

func (service *Service) DeleteProductItem(ctx context.Context, principal auth.Principal, productID string, requestedTenantID string) (MutationAck, error) {
	tenantID, err := service.resolveWritableTenantID(ctx, principal, requestedTenantID)
	if err != nil {
		return MutationAck{}, err
	}

	normalizedProductID := strings.TrimSpace(productID)
	if normalizedProductID == "" {
		return MutationAck{}, ErrValidation
	}

	savedAt, err := service.repository.DeleteProduct(ctx, tenantID, normalizedProductID)
	if err != nil {
		return MutationAck{}, err
	}

	ack := MutationAck{
		OK:       true,
		TenantID: tenantID,
		SavedAt:  savedAt,
	}

	return service.finalizeMutation(ctx, ack, nil)
}

func (service *Service) loadWritableProductCatalog(ctx context.Context, principal auth.Principal, requestedTenantID string) (string, []ProductItem, bool, error) {
	tenantID, err := service.resolveWritableTenantID(ctx, principal, requestedTenantID)
	if err != nil {
		return "", nil, false, err
	}

	items, err := service.repository.GetProductCatalog(ctx, tenantID)
	if err != nil {
		return "", nil, false, err
	}

	if len(items) > 0 {
		return tenantID, cloneProducts(items), false, nil
	}

	return tenantID, defaultProductCatalogItems(), true, nil
}

func normalizeProducts(products []ProductItem, fallback []ProductItem) []ProductItem {
	if products == nil {
		return cloneProducts(fallback)
	}

	normalized := make([]ProductItem, 0, len(products))
	seen := make(map[string]struct{})
	for _, product := range products {
		id := strings.TrimSpace(product.ID)
		name := strings.TrimSpace(product.Name)
		if id == "" || name == "" {
			continue
		}

		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		normalized = append(normalized, ProductItem{
			ID:        id,
			Name:      name,
			Code:      strings.ToUpper(strings.TrimSpace(product.Code)),
			Category:  fallbackCategory(product.Category),
			BasePrice: maxFloat(product.BasePrice, 0),
		})
	}

	return normalized
}

func upsertProductCatalogItem(items []ProductItem, product ProductItem) ([]ProductItem, bool) {
	normalizedItems := normalizeProducts([]ProductItem{product}, nil)
	if len(normalizedItems) != 1 {
		return nil, false
	}

	nextProducts := cloneProducts(items)
	nextProduct := normalizedItems[0]

	for index, current := range nextProducts {
		if current.ID == nextProduct.ID {
			nextProducts[index] = nextProduct
			return nextProducts, true
		}
	}

	return append(nextProducts, nextProduct), true
}

func removeProductCatalogItem(items []ProductItem, productID string) []ProductItem {
	nextProducts := make([]ProductItem, 0, len(items))
	for _, item := range items {
		if item.ID != productID {
			nextProducts = append(nextProducts, item)
		}
	}

	return nextProducts
}
