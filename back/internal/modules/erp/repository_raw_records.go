package erp

import (
	"context"
	"fmt"
	"strings"
)

func resolveRawRecordsSortColumn(dataType, sortBy string) string {
	allowed := map[string]map[string]struct{}{
		DataTypeCustomer: {
			"name": {}, "cpf": {}, "city": {}, "source_batch_date": {}, "registered_at_raw": {},
		},
		DataTypeOrder: {
			"order_date_raw": {}, "total_amount_cents": {}, "source_batch_date": {}, "customer_id": {},
		},
		DataTypeOrderCanceled: {
			"order_date_raw": {}, "total_amount_cents": {}, "source_batch_date": {},
		},
		DataTypeEmployee: {
			"name": {}, "source_batch_date": {},
		},
	}
	if cols, ok := allowed[dataType]; ok {
		if _, valid := cols[sortBy]; valid {
			return sortBy
		}
	}
	return "source_batch_date"
}

func (repository *PostgresRepository) ListRawRecords(ctx context.Context, store StoreScope, query RawRecordsQuery) (RawRecordsListResponse, error) {
	var (
		tableName         string
		selectColumns     string
		searchCondition   string
		specificCondition string
	)

	switch query.DataType {
	case DataTypeCustomer:
		tableName = "erp_customer_raw"
		selectColumns = `
			id::text as id,
			store_cnpj,
			name,
			nickname,
			cpf,
			email,
			phone,
			mobile,
			gender,
			birthday_raw,
			street,
			number,
			complement,
			neighborhood,
			city,
			uf,
			country,
			zipcode,
			employee_id,
			store_id_raw,
			registered_at_raw,
			original_id,
			identifier,
			tags,
			source_batch_date,
			source_line_number,
			raw_values,
			raw_payload`
		searchCondition = `(
			name ilike $4
			or nickname ilike $4
			or cpf ilike $4
			or email ilike $4
			or phone ilike $4
			or mobile ilike $4
			or city ilike $4
			or uf ilike $4
			or zipcode ilike $4
			or employee_id ilike $4
			or store_id_raw ilike $4
			or original_id ilike $4
			or identifier ilike $4
			or tags ilike $4
		)`
		specificCondition = `(cpf ilike $6 or ($7 <> '' and regexp_replace(cpf, '\D', '', 'g') ilike $8))`
	case DataTypeEmployee:
		tableName = "erp_employee_raw"
		selectColumns = `
			id::text as id,
			store_cnpj,
			name,
			store_id_raw,
			original_id,
			city,
			uf,
			street,
			complement,
			zipcode,
			is_active_raw,
			source_batch_date,
			source_line_number,
			raw_values,
			raw_payload`
		searchCondition = `(
			name ilike $4
			or store_id_raw ilike $4
			or original_id ilike $4
			or city ilike $4
			or uf ilike $4
			or street ilike $4
			or zipcode ilike $4
			or is_active_raw ilike $4
		)`
		specificCondition = `(original_id ilike $6 or ($7 <> '' and original_id ilike $8))`
	case DataTypeOrder:
		tableName = "erp_order_raw"
		selectColumns = `
			id::text as id,
			store_cnpj,
			order_id,
			identifier,
			store_id_raw,
			customer_id,
			order_date_raw,
			total_amount_raw,
			total_amount_cents,
			product_return_raw,
			product_return_cents,
			sku,
			amount_raw,
			amount_cents,
			quantity_raw,
			quantity,
			employee_id,
			payment_type,
			total_exclusion_raw,
			total_exclusion_cents,
			total_debit_raw,
			total_debit_cents,
			source_batch_date,
			source_line_number,
			raw_values,
			raw_payload`
		searchCondition = `(
			order_id ilike $4
			or identifier ilike $4
			or store_id_raw ilike $4
			or customer_id ilike $4
			or order_date_raw ilike $4
			or total_amount_raw ilike $4
			or sku ilike $4
			or amount_raw ilike $4
			or quantity_raw ilike $4
			or employee_id ilike $4
			or payment_type ilike $4
			or total_exclusion_raw ilike $4
			or total_debit_raw ilike $4
		)`
		specificCondition = `(order_id ilike $6 or ($7 <> '' and order_id ilike $8))`
	case DataTypeOrderCanceled:
		tableName = "erp_order_canceled_raw"
		selectColumns = `
			id::text as id,
			store_cnpj,
			order_id,
			identifier,
			store_id_raw,
			customer_id,
			order_date_raw,
			total_amount_raw,
			total_amount_cents,
			product_return_raw,
			product_return_cents,
			sku,
			amount_raw,
			amount_cents,
			quantity_raw,
			quantity,
			employee_id,
			payment_type,
			total_exclusion_raw,
			total_exclusion_cents,
			total_debit_raw,
			total_debit_cents,
			source_batch_date,
			source_line_number,
			raw_values,
			raw_payload`
		searchCondition = `(
			order_id ilike $4
			or identifier ilike $4
			or store_id_raw ilike $4
			or customer_id ilike $4
			or order_date_raw ilike $4
			or total_amount_raw ilike $4
			or sku ilike $4
			or amount_raw ilike $4
			or quantity_raw ilike $4
			or employee_id ilike $4
			or payment_type ilike $4
			or total_exclusion_raw ilike $4
			or total_debit_raw ilike $4
		)`
		specificCondition = `(order_id ilike $6 or ($7 <> '' and order_id ilike $8))`
	default:
		return RawRecordsListResponse{}, ErrUnsupportedDataType
	}

	search := strings.TrimSpace(query.Search)
	likeSearch := "%" + search + "%"
	specificSearch := strings.TrimSpace(query.SpecificSearch)
	specificLike := specificSearch + "%"
	specificDigits := onlyDigits(specificSearch)
	specificDigitsLike := specificDigits + "%"
	dateFrom := strings.TrimSpace(query.DateFrom)
	dateTo := strings.TrimSpace(query.DateTo)

	orderCol := resolveRawRecordsSortColumn(query.DataType, query.SortBy)
	orderDir := "desc"
	if query.SortDir == "asc" {
		orderDir = "asc"
	}
	outerOrderClause := fmt.Sprintf("order by %s %s, source_line_number desc", orderCol, orderDir)

	var countSQL string
	var listSQL string
	offset := (query.Page - 1) * query.PageSize

	// count uses $9/$10 for dates; list uses $9/$10 for limit/offset and $11/$12 for dates.
	countDateFilter := `and ($9 = '' or source_batch_date::date >= $9::date)
				  and ($10 = '' or source_batch_date::date <= $10::date)`
	listDateFilter := `and ($11 = '' or source_batch_date::date >= $11::date)
				  and ($12 = '' or source_batch_date::date <= $12::date)`

	switch {
	case query.Dedup && (query.DataType == DataTypeOrder || query.DataType == DataTypeOrderCanceled):
		countSQL = fmt.Sprintf(`
			with filtered as not materialized (
				select *
				from %s
				where tenant_id = $1::uuid
				  and store_id = $2::uuid
				  and ($3 = '' or %s)
				  and ($5 = '' or %s)
				  %s
			), latest_order_files as (
				select distinct on (order_id)
					order_id,
					file_id
				from filtered
				where nullif(trim(order_id), '') is not null
				order by order_id, source_batch_date desc, created_at_imported desc, source_file_name desc, file_id desc
			)
			select count(*) from latest_order_files;`, tableName, searchCondition, specificCondition, countDateFilter)
		listSQL = fmt.Sprintf(`
			with filtered as not materialized (
				select *
				from %s
				where tenant_id = $1::uuid
				  and store_id = $2::uuid
				  and ($3 = '' or %s)
				  and ($5 = '' or %s)
				  %s
			), latest_orders as (
				select distinct on (order_id)
					order_id,
					file_id,
					source_batch_date,
					source_line_number,
					order_date_raw,
					total_amount_cents,
					customer_id
				from filtered
				where nullif(trim(order_id), '') is not null
				order by order_id, source_batch_date desc, created_at_imported desc, source_file_name desc, file_id desc
			), paged_orders as (
				select *
				from latest_orders
				%s
				limit $9 offset $10
			), latest_order_lines as (
				select filtered.*
				from filtered
				join paged_orders latest
				  on latest.order_id = filtered.order_id
				 and latest.file_id = filtered.file_id
			), grouped_orders as (
				select
					(array_agg(id::text order by source_batch_date desc, created_at_imported desc, source_line_number desc, id desc))[1] as id,
					(array_agg(store_cnpj order by source_batch_date desc, created_at_imported desc, source_line_number desc, id desc))[1] as store_cnpj,
					order_id,
					(array_agg(identifier order by source_batch_date desc, created_at_imported desc, source_line_number desc, id desc))[1] as identifier,
					(array_agg(store_id_raw order by source_batch_date desc, created_at_imported desc, source_line_number desc, id desc))[1] as store_id_raw,
					(array_agg(customer_id order by source_batch_date desc, created_at_imported desc, source_line_number desc, id desc))[1] as customer_id,
					(array_agg(order_date_raw order by source_batch_date desc, created_at_imported desc, source_line_number desc, id desc))[1] as order_date_raw,
					coalesce(
						nullif(max(coalesce(total_amount_cents, 0)), 0),
						sum(coalesce(amount_cents, 0)),
						0
					)::text as total_amount_raw,
					coalesce(
						nullif(max(coalesce(total_amount_cents, 0)), 0),
						sum(coalesce(amount_cents, 0)),
						0
					)::bigint as total_amount_cents,
					coalesce(max(coalesce(product_return_cents, 0)), 0)::text as product_return_raw,
					coalesce(max(coalesce(product_return_cents, 0)), 0)::bigint as product_return_cents,
					coalesce(string_agg(distinct nullif(trim(sku), ''), ', ' order by nullif(trim(sku), '')), '') as sku,
					coalesce(sum(coalesce(amount_cents, 0)), 0)::text as amount_raw,
					coalesce(sum(coalesce(amount_cents, 0)), 0)::bigint as amount_cents,
					coalesce(sum(case when coalesce(quantity, 0) > 0 then quantity else 1 end), 0)::text as quantity_raw,
					coalesce(sum(case when coalesce(quantity, 0) > 0 then quantity else 1 end), 0)::bigint as quantity,
					(array_agg(employee_id order by source_batch_date desc, created_at_imported desc, source_line_number desc, id desc))[1] as employee_id,
					(array_agg(payment_type order by source_batch_date desc, created_at_imported desc, source_line_number desc, id desc))[1] as payment_type,
					coalesce(max(coalesce(total_exclusion_cents, 0)), 0)::text as total_exclusion_raw,
					coalesce(max(coalesce(total_exclusion_cents, 0)), 0)::bigint as total_exclusion_cents,
					coalesce(max(coalesce(total_debit_cents, 0)), 0)::text as total_debit_raw,
					coalesce(max(coalesce(total_debit_cents, 0)), 0)::bigint as total_debit_cents,
					max(source_batch_date) as source_batch_date,
					max(source_line_number) as source_line_number,
					(array_agg(raw_values order by source_batch_date desc, created_at_imported desc, source_line_number desc, id desc))[1] as raw_values,
					(array_agg(raw_payload order by source_batch_date desc, created_at_imported desc, source_line_number desc, id desc))[1] as raw_payload
				from latest_order_lines
				group by order_id
			)
			select %s
			from grouped_orders
			%s
			`, tableName, searchCondition, specificCondition, listDateFilter, outerOrderClause, selectColumns, outerOrderClause)
	case query.Dedup && query.DataType == DataTypeCustomer:
		countSQL = fmt.Sprintf(`
			select count(*) from (
				select distinct on (coalesce(nullif(cpf,''), id::text)) id
				from %s
				where tenant_id = $1::uuid
				  and store_id = $2::uuid
				  and ($3 = '' or %s)
				  and ($5 = '' or %s)
				  %s
				order by coalesce(nullif(cpf,''), id::text), source_batch_date desc, source_line_number desc
			) _dedup;`, tableName, searchCondition, specificCondition, countDateFilter)
		listSQL = fmt.Sprintf(`
			select * from (
				select distinct on (coalesce(nullif(cpf,''), id::text))
					%s
				from %s
				where tenant_id = $1::uuid
				  and store_id = $2::uuid
				  and ($3 = '' or %s)
				  and ($5 = '' or %s)
				  %s
				order by coalesce(nullif(cpf,''), id::text), source_batch_date desc, source_line_number desc
			) _dedup
			%s
			limit $9 offset $10;`, selectColumns, tableName, searchCondition, specificCondition, listDateFilter, outerOrderClause)
	default:
		countSQL = fmt.Sprintf(`
			select count(*)
			from %s
			where tenant_id = $1::uuid
			  and store_id = $2::uuid
			  and ($3 = '' or %s)
			  and ($5 = '' or %s)
			  %s;`, tableName, searchCondition, specificCondition, countDateFilter)
		listSQL = fmt.Sprintf(`
			select %s
			from %s
			where tenant_id = $1::uuid
			  and store_id = $2::uuid
			  and ($3 = '' or %s)
			  and ($5 = '' or %s)
			  %s
			%s
			limit $9 offset $10;`, selectColumns, tableName, searchCondition, specificCondition, listDateFilter, outerOrderClause)
	}

	var total int
	if err := repository.pool.QueryRow(ctx, countSQL, store.TenantID, store.StoreID, search, likeSearch, specificSearch, specificLike, specificDigits, specificDigitsLike, dateFrom, dateTo).Scan(&total); err != nil {
		return RawRecordsListResponse{}, err
	}

	rows, err := repository.pool.Query(ctx, listSQL, store.TenantID, store.StoreID, search, likeSearch, specificSearch, specificLike, specificDigits, specificDigitsLike, query.PageSize, offset, dateFrom, dateTo)
	if err != nil {
		return RawRecordsListResponse{}, err
	}
	defer rows.Close()

	items := make([]map[string]any, 0, query.PageSize)
	fieldDescriptions := rows.FieldDescriptions()
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return RawRecordsListResponse{}, err
		}
		item := make(map[string]any, len(values))
		for index, value := range values {
			name := fieldDescriptions[index].Name
			item[name] = value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return RawRecordsListResponse{}, err
	}

	return RawRecordsListResponse{
		Store:          store,
		DataType:       query.DataType,
		Search:         search,
		SpecificSearch: specificSearch,
		Page:           query.Page,
		PageSize:       query.PageSize,
		Total:          total,
		Items:          items,
	}, nil
}
