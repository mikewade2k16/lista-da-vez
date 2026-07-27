package cardapio

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	maxBulkProducts   = 500
	maxImportProducts = 2000
)

// BulkProducts aplica uma acao fechada sobre IDs selecionados dentro do mesmo
// restaurante e account. IDs duplicados, vazios ou acima do limite invalidam o
// comando antes de chegar ao repository.
func (s *Service) BulkProducts(ctx context.Context, accountID, restaurantID string, in ProductBulkInput) (ProductBulkResult, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return ProductBulkResult{}, mapStoreErr(err)
	}
	if !validProductBulkAction(in.Action) || len(in.IDs) == 0 || len(in.IDs) > maxBulkProducts {
		return ProductBulkResult{}, ErrValidation
	}

	ids := make([]string, 0, len(in.IDs))
	seen := make(map[string]struct{}, len(in.IDs))
	for _, rawID := range in.IDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return ProductBulkResult{}, ErrValidation
		}
		if _, exists := seen[id]; exists {
			return ProductBulkResult{}, ErrValidation
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	affected, err := s.store.BulkProducts(ctx, accountID, restaurantID, ids, in.Action)
	if err != nil {
		return ProductBulkResult{}, mapStoreErr(err)
	}
	return ProductBulkResult{Affected: affected}, nil
}

func validProductBulkAction(action ProductBulkAction) bool {
	switch action {
	case ProductBulkDelete, ProductBulkEnable, ProductBulkDisable,
		ProductBulkFeature, ProductBulkRemoveFeature:
		return true
	default:
		return false
	}
}

// ExportProducts monta um documento portavel a partir do snapshot autoritativo
// do banco. Variacoes/adicionais sao carregados em lote pelo repository.
func (s *Service) ExportProducts(ctx context.Context, accountID, restaurantID string) (ProductTransferDocument, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return ProductTransferDocument{}, mapStoreErr(err)
	}
	categories, err := s.store.ListCategories(ctx, accountID, restaurantID, false)
	if err != nil {
		return ProductTransferDocument{}, mapStoreErr(err)
	}
	products, err := s.store.ListProductsFull(ctx, accountID, restaurantID)
	if err != nil {
		return ProductTransferDocument{}, mapStoreErr(err)
	}

	categoryByID := make(map[string]Category, len(categories))
	for _, category := range categories {
		categoryByID[category.ID] = category
	}
	items := make([]ProductTransferItem, 0, len(products))
	for _, product := range products {
		item := productTransferItem(product)
		if product.CategoryID != nil {
			if category, ok := categoryByID[*product.CategoryID]; ok {
				item.CategorySlug = category.Slug
				item.CategoryName = category.Name
			}
		}
		items = append(items, item)
	}
	return ProductTransferDocument{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Products:   items,
	}, nil
}

func productTransferItem(product Product) ProductTransferItem {
	variations := make([]VariationInput, len(product.Variations))
	for i, variation := range product.Variations {
		variations[i] = VariationInput{
			Name:            variation.Name,
			PriceDeltaCents: variation.PriceDeltaCents,
			SortOrder:       variation.SortOrder,
		}
	}
	addons := make([]AddonInput, len(product.Addons))
	for i, addon := range product.Addons {
		addons[i] = AddonInput{
			Name:       addon.Name,
			PriceCents: addon.PriceCents,
			SortOrder:  addon.SortOrder,
		}
	}
	return ProductTransferItem{ProductInput: ProductInput{
		Slug:                product.Slug,
		Name:                product.Name,
		ShortDesc:           product.ShortDesc,
		Description:         product.Description,
		Body:                product.Body,
		PriceCents:          product.PriceCents,
		CompareAtPriceCents: product.CompareAtPriceCents,
		ImageURL:            product.ImageURL,
		Gallery:             product.Gallery,
		Weight:              product.Weight,
		CookTime:            product.CookTime,
		Diet:                product.Diet,
		Allergens:           product.Allergens,
		Pairing:             product.Pairing,
		Tags:                product.Tags,
		IsAvailable:         product.IsAvailable,
		IsFeatured:          product.IsFeatured,
		SortOrder:           product.SortOrder,
		Variations:          variations,
		Addons:              addons,
	}}
}

// PreviewProductImport compara o arquivo com as categorias atuais sem escrever
// no banco. A mesma analise e repetida no import para evitar bypass da revisao.
func (s *Service) PreviewProductImport(ctx context.Context, accountID, restaurantID string, in ProductImportInput) (ProductImportPreview, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return ProductImportPreview{}, mapStoreErr(err)
	}
	if len(in.Products) == 0 || len(in.Products) > maxImportProducts {
		return ProductImportPreview{}, ErrValidation
	}
	categories, err := s.store.ListCategories(ctx, accountID, restaurantID, false)
	if err != nil {
		return ProductImportPreview{}, mapStoreErr(err)
	}
	return buildProductImportPreview(categories, in.Products), nil
}

func buildProductImportPreview(categories []Category, products []ProductTransferItem) ProductImportPreview {
	existing := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		existing[normalizeSlug(category.Slug)] = struct{}{}
	}

	items := make([]ProductImportPreviewCategory, 0)
	indexBySlug := make(map[string]int)
	for _, product := range products {
		slug := normalizeSlug(product.CategorySlug)
		if slug == "" {
			continue
		}
		if _, exists := existing[slug]; exists {
			continue
		}
		if index, exists := indexBySlug[slug]; exists {
			items[index].ProductCount++
			if items[index].Name == "" {
				items[index].Name = strings.TrimSpace(product.CategoryName)
			}
			continue
		}
		indexBySlug[slug] = len(items)
		items = append(items, ProductImportPreviewCategory{
			Slug:         slug,
			Name:         strings.TrimSpace(product.CategoryName),
			ProductCount: 1,
		})
	}
	return ProductImportPreview{NewCategories: items}
}

// ImportProducts cria ou atualiza por slug. O arquivo inteiro e validado antes
// das escritas previsiveis; falhas pontuais de uma linha sao relatadas sem
// esconder quantas linhas ja foram persistidas.
func (s *Service) ImportProducts(ctx context.Context, accountID, restaurantID string, in ProductImportInput) (ProductImportResult, error) {
	result := ProductImportResult{Errors: []ProductImportError{}}
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return result, mapStoreErr(err)
	}
	if len(in.Products) == 0 || len(in.Products) > maxImportProducts {
		return result, ErrValidation
	}

	categories, err := s.store.ListCategories(ctx, accountID, restaurantID, false)
	if err != nil {
		return result, mapStoreErr(err)
	}
	preview := buildProductImportPreview(categories, in.Products)
	acceptedCategorySlugs := make(map[string]struct{}, len(in.AcceptedCategorySlugs))
	for _, slug := range in.AcceptedCategorySlugs {
		if normalized := normalizeSlug(slug); normalized != "" {
			acceptedCategorySlugs[normalized] = struct{}{}
		}
	}
	for _, category := range preview.NewCategories {
		if _, accepted := acceptedCategorySlugs[category.Slug]; !accepted {
			return result, ErrValidation
		}
	}
	categoryBySlug := make(map[string]Category, len(categories))
	nextCategoryOrder := 0
	for _, category := range categories {
		categoryBySlug[normalizeSlug(category.Slug)] = category
		if category.SortOrder >= nextCategoryOrder {
			nextCategoryOrder = category.SortOrder + 1
		}
	}

	existingProducts, err := s.store.ListProductsLean(ctx, accountID, restaurantID)
	if err != nil {
		return result, mapStoreErr(err)
	}
	productBySlug := make(map[string]ProductLean, len(existingProducts))
	for _, product := range existingProducts {
		productBySlug[normalizeSlug(product.Slug)] = product
	}

	seenSlugs := make(map[string]struct{}, len(in.Products))
	for index := range in.Products {
		item := &in.Products[index]
		item.Slug = normalizeSlug(item.Slug)
		item.Name = strings.TrimSpace(item.Name)
		item.CategorySlug = normalizeSlug(item.CategorySlug)
		item.CategoryName = strings.TrimSpace(item.CategoryName)

		if _, duplicate := seenSlugs[item.Slug]; duplicate && item.Slug != "" {
			addProductImportError(&result, index, item.Slug, "Slug duplicado no arquivo.")
			continue
		}
		seenSlugs[item.Slug] = struct{}{}
		if err := validateProductInput(&item.ProductInput); err != nil {
			addProductImportError(&result, index, item.Slug, "Produto invalido: nome, slug e preco devem ser validos.")
			continue
		}
		if item.CategorySlug != "" {
			if _, exists := categoryBySlug[item.CategorySlug]; !exists && item.CategoryName == "" {
				addProductImportError(&result, index, item.Slug, "Categoria inexistente e sem categoryName para criacao.")
			}
		}
	}

	invalidRows := make(map[int]struct{}, len(result.Errors))
	for _, itemError := range result.Errors {
		invalidRows[itemError.Row-1] = struct{}{}
	}

	for index, item := range in.Products {
		if _, invalid := invalidRows[index]; invalid {
			continue
		}

		item.CategoryID = nil
		if item.CategorySlug != "" {
			category, exists := categoryBySlug[item.CategorySlug]
			if !exists {
				category, err = s.store.CreateCategory(ctx, accountID, restaurantID, CategoryInput{
					Slug:      item.CategorySlug,
					Name:      item.CategoryName,
					SortOrder: nextCategoryOrder,
					IsActive:  true,
				})
				if err != nil {
					addProductImportError(&result, index, item.Slug, "Nao foi possivel criar a categoria.")
					continue
				}
				nextCategoryOrder++
				categoryBySlug[item.CategorySlug] = category
			}
			categoryID := category.ID
			item.CategoryID = &categoryID
		}

		existing, exists := productBySlug[item.Slug]
		if exists && !in.UpdateExisting {
			result.Skipped++
			continue
		}
		if exists {
			product, updateErr := s.store.UpdateProduct(ctx, accountID, existing.ID, item.ProductInput)
			if updateErr != nil {
				addProductImportError(&result, index, item.Slug, importPersistenceMessage(updateErr))
				continue
			}
			productBySlug[item.Slug] = ProductLean{ID: product.ID, Slug: product.Slug}
			result.Updated++
			continue
		}

		product, createErr := s.store.CreateProduct(ctx, accountID, restaurantID, item.ProductInput)
		if createErr != nil {
			addProductImportError(&result, index, item.Slug, importPersistenceMessage(createErr))
			continue
		}
		productBySlug[item.Slug] = ProductLean{ID: product.ID, Slug: product.Slug}
		result.Created++
	}
	return result, nil
}

func addProductImportError(result *ProductImportResult, index int, slug, message string) {
	result.Failed++
	result.Errors = append(result.Errors, ProductImportError{
		Row:     index + 1,
		Slug:    slug,
		Message: message,
	})
}

func importPersistenceMessage(err error) string {
	if errors.Is(mapStoreErr(err), ErrSlugConflict) {
		return "Slug em conflito com outro produto."
	}
	return "Nao foi possivel persistir o produto."
}
