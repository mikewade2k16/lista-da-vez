package erp

import (
	"context"
	"fmt"
	"strings"
)

var productsSortableColumns = map[string]string{
	"name":              "name",
	"sku":               "sku",
	"source_batch_date": "source_batch_date",
	"price_cents":       "price_cents",
	"priceRaw":          "price_cents",
	"source_updated_at": "source_updated_at",
	"sourceUpdatedAt":   "source_updated_at",
}

func resolveProductSortColumn(sortBy string) string {
	if col, ok := productsSortableColumns[sortBy]; ok {
		return col
	}
	return "name"
}

func (repository *PostgresRepository) ListCurrentItems(ctx context.Context, store StoreScope, query ProductQuery) (ProductListResponse, error) {
	identifierPrefix := strings.TrimSpace(query.IdentifierPrefix)
	search := strings.TrimSpace(query.Search)
	identifierLike := identifierPrefix + "%"
	likeSearch := "%" + search + "%"
	dateFrom := strings.TrimSpace(query.DateFrom)
	dateTo := strings.TrimSpace(query.DateTo)

	sortCol := resolveProductSortColumn(query.SortBy)
	sortDir := "asc"
	if query.SortDir == "desc" {
		sortDir = "desc"
	}
	if sortCol == "name" && query.SortDir == "" {
		sortDir = "asc"
	}
	orderClause := fmt.Sprintf("order by %s %s, sku asc", sortCol, sortDir)

	searchFilter := `and (
			$3 = ''
			or sku ilike $4
			or identifier ilike $4
		  )
		  and (
			$5 = ''
			or sku ilike $6
			or identifier ilike $6
			or name ilike $6
			or description ilike $6
			or supplierreference ilike $6
			or brandname ilike $6
			or seasonname ilike $6
			or category1 ilike $6
			or category2 ilike $6
			or category3 ilike $6
			or size ilike $6
			or color ilike $6
			or unit ilike $6
			or price_raw ilike $6
			or cast(price_cents as text) ilike $6
		  )`

	// count uses $7/$8 for dates; list uses $7/$8 for limit/offset, $9/$10 for dates.
	countSQL := fmt.Sprintf(`
		select count(*)
		from erp_item_current
		where tenant_id = $1::uuid
		  and store_id = $2::uuid
		  %s
		  and ($7 = '' or source_batch_date >= $7::date)
		  and ($8 = '' or source_batch_date <= $8::date);`, searchFilter)

	var total int
	if err := repository.pool.QueryRow(ctx, countSQL, store.TenantID, store.StoreID, identifierPrefix, identifierLike, search, likeSearch, dateFrom, dateTo).Scan(&total); err != nil {
		return ProductListResponse{}, err
	}

	offset := (query.Page - 1) * query.PageSize

	listSQL := fmt.Sprintf(`
		select
			sku,
			identifier,
			name,
			description,
			supplierreference,
			brandname,
			seasonname,
			category1,
			category2,
			category3,
			size,
			color,
			unit,
			price_raw,
			price_cents,
			source_created_at,
			source_updated_at,
			source_file_name,
			to_char(source_batch_date, 'YYYY-MM-DD') as source_batch_date
		from erp_item_current
		where tenant_id = $1::uuid
		  and store_id = $2::uuid
		  %s
		  and ($9 = '' or source_batch_date >= $9::date)
		  and ($10 = '' or source_batch_date <= $10::date)
		%s
		limit $7 offset $8;`, searchFilter, orderClause)

	rows, err := repository.pool.Query(ctx, listSQL, store.TenantID, store.StoreID, identifierPrefix, identifierLike, search, likeSearch, query.PageSize, offset, dateFrom, dateTo)
	if err != nil {
		return ProductListResponse{}, err
	}
	defer rows.Close()

	items := make([]ProductRow, 0, query.PageSize)
	for rows.Next() {
		var item ProductRow
		if err := rows.Scan(
			&item.SKU,
			&item.Identifier,
			&item.Name,
			&item.Description,
			&item.SupplierReference,
			&item.BrandName,
			&item.SeasonName,
			&item.Category1,
			&item.Category2,
			&item.Category3,
			&item.Size,
			&item.Color,
			&item.Unit,
			&item.PriceRaw,
			&item.PriceCents,
			&item.SourceCreatedAt,
			&item.SourceUpdatedAt,
			&item.SourceFileName,
			&item.SourceBatchDate,
		); err != nil {
			return ProductListResponse{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ProductListResponse{}, err
	}

	return ProductListResponse{
		Store:            store,
		IdentifierPrefix: identifierPrefix,
		Search:           search,
		Page:             query.Page,
		PageSize:         query.PageSize,
		Total:            total,
		Items:            items,
	}, nil
}
