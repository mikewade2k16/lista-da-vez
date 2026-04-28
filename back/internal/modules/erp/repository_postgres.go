package erp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) ResolveStoreScope(ctx context.Context, principal auth.Principal, requestedTenantID string, requestedStoreCode string) (StoreScope, error) {
	normalizedStoreCode := strings.TrimSpace(requestedStoreCode)
	if normalizedStoreCode == "" {
		return StoreScope{}, ErrStoreRequired
	}

	tenantID := strings.TrimSpace(requestedTenantID)
	if tenantID == "" {
		resolvedTenantID, err := repository.ResolveDefaultTenantID(ctx, principal)
		if err != nil {
			return StoreScope{}, err
		}
		tenantID = resolvedTenantID
	}

	allowed, err := repository.CanAccessTenant(ctx, principal, tenantID)
	if err != nil {
		return StoreScope{}, err
	}
	if !allowed {
		return StoreScope{}, ErrForbidden
	}

	if requiresStoreScopedFilter(principal.Role) && len(principal.StoreIDs) == 0 {
		return StoreScope{}, ErrForbidden
	}

	query := `
		select
			s.tenant_id::text,
			s.id::text,
			s.code,
			s.name,
			s.city,
			coalesce(last_file.store_cnpj, '')
		from stores s
		left join lateral (
			select sf.store_cnpj
			from erp_sync_files sf
			where sf.tenant_id = s.tenant_id
			  and sf.store_id = s.id
			order by sf.imported_at desc
			limit 1
		) last_file on true
		where s.tenant_id = $1::uuid
		  and s.code = $2
		  and s.is_active = true
	`
	args := []any{tenantID, normalizedStoreCode}
	if requiresStoreScopedFilter(principal.Role) {
		query += ` and s.id = any($3::uuid[])`
		args = append(args, principal.StoreIDs)
	}
	query += ` limit 1;`

	var scope StoreScope
	err = repository.pool.QueryRow(ctx, query, args...).Scan(
		&scope.TenantID,
		&scope.StoreID,
		&scope.StoreCode,
		&scope.StoreName,
		&scope.StoreCity,
		&scope.StoreCNPJ,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return StoreScope{}, ErrStoreNotFound
		}
		return StoreScope{}, err
	}

	return scope, nil
}

func (repository *PostgresRepository) GetStatus(ctx context.Context, store StoreScope) (StatusResponse, error) {
	status := StatusResponse{
		Store:            store,
		SupportedTypes:   append([]string{}, supportedDataTypes...),
		FunctionalTypes:  []string{DataTypeItem, DataTypeCustomer, DataTypeEmployee, DataTypeOrder, DataTypeOrderCanceled},
		PlaceholderTypes: []string{},
	}
	var customerRows, employeeRows, orderRows, orderCanceledRows int

	if err := repository.pool.QueryRow(ctx, `
		select
			coalesce((select count(*) from erp_item_current where tenant_id = $1::uuid and store_id = $2::uuid), 0),
			coalesce((select count(*) from erp_item_raw where tenant_id = $1::uuid and store_id = $2::uuid), 0),
			coalesce((select count(*) from erp_customer_raw where tenant_id = $1::uuid and store_id = $2::uuid), 0),
			coalesce((select count(*) from erp_employee_raw where tenant_id = $1::uuid and store_id = $2::uuid), 0),
			coalesce((select count(*) from erp_order_raw where tenant_id = $1::uuid and store_id = $2::uuid), 0),
			coalesce((select count(*) from erp_order_canceled_raw where tenant_id = $1::uuid and store_id = $2::uuid), 0);
	`, store.TenantID, store.StoreID).Scan(
		&status.ProductCurrent,
		&status.RawItemRows,
		&customerRows,
		&employeeRows,
		&orderRows,
		&orderCanceledRows,
	); err != nil {
		return StatusResponse{}, err
	}

	typeRows := map[string]struct {
		current int
		raw     int
		total   int
	}{
		DataTypeItem:          {current: status.ProductCurrent, raw: status.RawItemRows, total: status.ProductCurrent},
		DataTypeCustomer:      {raw: customerRows, total: customerRows},
		DataTypeEmployee:      {raw: employeeRows, total: employeeRows},
		DataTypeOrder:         {raw: orderRows, total: orderRows},
		DataTypeOrderCanceled: {raw: orderCanceledRows, total: orderCanceledRows},
	}

	status.TypeStats = make([]TypeStatus, 0, len(supportedDataTypes))
	for _, dataType := range supportedDataTypes {
		lastRun, err := repository.getLastRun(ctx, store, dataType)
		if err != nil {
			return StatusResponse{}, err
		}
		lastFile, err := repository.getLastFile(ctx, store, dataType)
		if err != nil {
			return StatusResponse{}, err
		}
		counts := typeRows[dataType]
		status.TypeStats = append(status.TypeStats, TypeStatus{
			DataType:         dataType,
			TotalRows:        counts.total,
			CurrentRows:      counts.current,
			RawRows:          counts.raw,
			LastRun:          lastRun,
			LastImportedFile: lastFile,
		})
	}

	lastRun, err := repository.getLastRun(ctx, store, DataTypeItem)
	if err != nil {
		return StatusResponse{}, err
	}
	status.LastRun = lastRun

	lastFile, err := repository.getLastFile(ctx, store, DataTypeItem)
	if err != nil {
		return StatusResponse{}, err
	}
	status.LastImportedFile = lastFile
	if status.Store.StoreCNPJ == "" && lastFile != nil {
		status.Store.StoreCNPJ = lastFile.StoreCNPJ
	}

	return status, nil
}

func (repository *PostgresRepository) ListCurrentItems(ctx context.Context, store StoreScope, query ProductQuery) (ProductListResponse, error) {
	identifierPrefix := strings.TrimSpace(query.IdentifierPrefix)
	search := strings.TrimSpace(query.Search)
	identifierLike := identifierPrefix + "%"
	likeSearch := "%" + search + "%"

	countSQL := `
		select count(*)
		from erp_item_current
		where tenant_id = $1::uuid
		  and store_id = $2::uuid
		  and (
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
		  );`

	var total int
	if err := repository.pool.QueryRow(ctx, countSQL, store.TenantID, store.StoreID, identifierPrefix, identifierLike, search, likeSearch).Scan(&total); err != nil {
		return ProductListResponse{}, err
	}

	offset := (query.Page - 1) * query.PageSize

	listSQL := `
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
		  and (
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
		  )
		order by name asc, sku asc
		limit $7 offset $8;`

	rows, err := repository.pool.Query(ctx, listSQL, store.TenantID, store.StoreID, identifierPrefix, identifierLike, search, likeSearch, query.PageSize, offset)
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
			registered_at_raw,
			original_id,
			identifier,
			tags`
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
			original_id,
			city,
			uf,
			street,
			complement,
			zipcode,
			is_active_raw`
		searchCondition = `(
			name ilike $4
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
			total_debit_cents`
		searchCondition = `(
			order_id ilike $4
			or identifier ilike $4
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
			total_debit_cents`
		searchCondition = `(
			order_id ilike $4
			or identifier ilike $4
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

	countSQL := fmt.Sprintf(`
		select count(*)
		from %s
		where tenant_id = $1::uuid
		  and store_id = $2::uuid
		  and (
			$3 = ''
			or %s
		  )
		  and (
			$5 = ''
			or %s
		  );`, tableName, searchCondition, specificCondition)

	var total int
	if err := repository.pool.QueryRow(ctx, countSQL, store.TenantID, store.StoreID, search, likeSearch, specificSearch, specificLike, specificDigits, specificDigitsLike).Scan(&total); err != nil {
		return RawRecordsListResponse{}, err
	}

	offset := (query.Page - 1) * query.PageSize
	listSQL := fmt.Sprintf(`
		select %s
		from %s
		where tenant_id = $1::uuid
		  and store_id = $2::uuid
		  and (
			$3 = ''
			or %s
		  )
		  and (
			$5 = ''
			or %s
		  )
		order by source_batch_date desc, source_line_number desc
		limit $9 offset $10;`, selectColumns, tableName, searchCondition, specificCondition)

	rows, err := repository.pool.Query(ctx, listSQL, store.TenantID, store.StoreID, search, likeSearch, specificSearch, specificLike, specificDigits, specificDigitsLike, query.PageSize, offset)
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
			name := string(fieldDescriptions[index].Name)
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

func onlyDigits(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if char >= '0' && char <= '9' {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func (repository *PostgresRepository) StartSyncRun(ctx context.Context, store StoreScope, dataType string, mode string, sourcePath string) (syncRunStart, error) {
	var started syncRunStart
	err := repository.pool.QueryRow(ctx, `
		insert into erp_sync_runs (
			tenant_id,
			store_id,
			store_code,
			store_cnpj,
			data_type,
			mode,
			source_path,
			status,
			started_at,
			created_at,
			updated_at
		) values (
			$1::uuid,
			$2::uuid,
			$3,
			nullif($4, ''),
			$5,
			$6,
			$7,
			$8,
			now(),
			now(),
			now()
		)
		returning id::text, started_at;
	`, store.TenantID, store.StoreID, store.StoreCode, store.StoreCNPJ, dataType, mode, sourcePath, SyncStatusRunning).Scan(&started.ID, &started.StartedAt)
	if err != nil {
		return syncRunStart{}, err
	}
	return started, nil
}

func (repository *PostgresRepository) FinishSyncRun(
	ctx context.Context,
	runID string,
	status string,
	filesSeen int,
	filesImported int,
	filesSkipped int,
	rowsRead int,
	rowsImported int,
	storeCNPJ string,
	finishedAt time.Time,
	errorMessage string,
) error {
	_, err := repository.pool.Exec(ctx, `
		update erp_sync_runs
		set
			status = $2,
			files_seen = $3,
			files_imported = $4,
			files_skipped = $5,
			rows_read = $6,
			raw_rows_imported = $7,
			store_cnpj = coalesce(nullif($8, ''), store_cnpj),
			finished_at = $9,
			error_message = nullif($10, ''),
			updated_at = now()
		where id = $1::uuid;
	`, runID, status, filesSeen, filesImported, filesSkipped, rowsRead, rowsImported, storeCNPJ, finishedAt, errorMessage)
	return err
}

func (repository *PostgresRepository) ImportItemBatch(ctx context.Context, input itemBatchImportInput) (itemBatchImportResult, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return itemBatchImportResult{}, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	fileID, inserted, err := repository.insertSyncFile(ctx, tx, insertSyncFileInput{
		RunID:      input.RunID,
		Store:      input.Store,
		DataType:   input.DataType,
		SourceName: input.Batch.SourceFileName,
		SourcePath: input.Batch.SourceFileName,
		SourceKind: SyncModeBootstrapMarkdown,
		BatchDate:  input.Batch.BatchDate,
		Checksum:   input.Batch.ChecksumSHA256,
		Rows:       len(input.Batch.Rows),
		ImportedAt: input.ImportedAt,
		StoreCNPJ:  input.Batch.StoreCNPJ,
	})
	if err != nil {
		return itemBatchImportResult{}, err
	}
	if !inserted {
		if err := tx.Rollback(ctx); err != nil {
			return itemBatchImportResult{}, err
		}
		tx = nil
		return itemBatchImportResult{Imported: false, Rows: 0, StoreCNPJ: input.Batch.StoreCNPJ}, nil
	}

	if len(input.Batch.Rows) > 0 {
		if _, err := tx.CopyFrom(
			ctx,
			pgx.Identifier{"erp_item_raw"},
			[]string{
				"run_id",
				"file_id",
				"tenant_id",
				"store_id",
				"store_code",
				"store_cnpj",
				"source_file_name",
				"source_batch_date",
				"source_line_number",
				"sku",
				"name",
				"description",
				"supplierreference",
				"brandname",
				"seasonname",
				"category1",
				"category2",
				"category3",
				"size",
				"color",
				"unit",
				"price_raw",
				"price_cents",
				"identifier",
				"created_at_raw",
				"updated_at_raw",
				"created_at",
				"updated_at",
				"created_at_imported",
			},
			pgx.CopyFromSlice(len(input.Batch.Rows), func(index int) ([]any, error) {
				row := input.Batch.Rows[index]
				return []any{
					input.RunID,
					fileID,
					input.Store.TenantID,
					input.Store.StoreID,
					row.StoreCode,
					row.StoreCNPJ,
					row.SourceFileName,
					row.SourceBatchDate,
					row.SourceLineNumber,
					row.SKU,
					row.Name,
					row.Description,
					row.SupplierReference,
					row.BrandName,
					row.SeasonName,
					row.Category1,
					row.Category2,
					row.Category3,
					row.Size,
					row.Color,
					row.Unit,
					row.PriceRaw,
					row.PriceCents,
					row.Identifier,
					row.CreatedAtRaw,
					row.UpdatedAtRaw,
					row.CreatedAt,
					row.UpdatedAt,
					input.ImportedAt,
				}, nil
			}),
		); err != nil {
			return itemBatchImportResult{}, err
		}

		if _, err := tx.Exec(ctx, `
			insert into erp_item_current (
				tenant_id,
				store_id,
				store_code,
				store_cnpj,
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
				source_file_name,
				source_batch_date,
				source_line_number,
				source_created_at_raw,
				source_updated_at_raw,
				source_created_at,
				source_updated_at,
				run_id,
				file_id,
				created_at,
				updated_at
			)
			select distinct on (raw.sku)
				raw.tenant_id,
				raw.store_id,
				raw.store_code,
				raw.store_cnpj,
				raw.sku,
				raw.identifier,
				raw.name,
				raw.description,
				raw.supplierreference,
				raw.brandname,
				raw.seasonname,
				raw.category1,
				raw.category2,
				raw.category3,
				raw.size,
				raw.color,
				raw.unit,
				raw.price_raw,
				raw.price_cents,
				raw.source_file_name,
				raw.source_batch_date,
				raw.source_line_number,
				raw.created_at_raw,
				raw.updated_at_raw,
				raw.created_at,
				raw.updated_at,
				raw.run_id,
				raw.file_id,
				now(),
				now()
			from erp_item_raw raw
			where raw.file_id = $1::uuid
			order by raw.sku, coalesce(raw.updated_at, raw.created_at, raw.source_batch_date::timestamp) desc, raw.source_line_number desc
			on conflict (tenant_id, store_id, sku)
			do update
			set
				store_code = excluded.store_code,
				store_cnpj = excluded.store_cnpj,
				identifier = excluded.identifier,
				name = excluded.name,
				description = excluded.description,
				supplierreference = excluded.supplierreference,
				brandname = excluded.brandname,
				seasonname = excluded.seasonname,
				category1 = excluded.category1,
				category2 = excluded.category2,
				category3 = excluded.category3,
				size = excluded.size,
				color = excluded.color,
				unit = excluded.unit,
				price_raw = excluded.price_raw,
				price_cents = excluded.price_cents,
				source_file_name = excluded.source_file_name,
				source_batch_date = excluded.source_batch_date,
				source_line_number = excluded.source_line_number,
				source_created_at_raw = excluded.source_created_at_raw,
				source_updated_at_raw = excluded.source_updated_at_raw,
				source_created_at = excluded.source_created_at,
				source_updated_at = excluded.source_updated_at,
				run_id = excluded.run_id,
				file_id = excluded.file_id,
				updated_at = now()
			where
				coalesce(excluded.source_updated_at, excluded.source_created_at, excluded.source_batch_date::timestamp, to_timestamp(0)) >
					coalesce(erp_item_current.source_updated_at, erp_item_current.source_created_at, erp_item_current.source_batch_date::timestamp, to_timestamp(0))
				or (
					coalesce(excluded.source_updated_at, excluded.source_created_at, excluded.source_batch_date::timestamp, to_timestamp(0)) =
						coalesce(erp_item_current.source_updated_at, erp_item_current.source_created_at, erp_item_current.source_batch_date::timestamp, to_timestamp(0))
					and excluded.source_line_number >= erp_item_current.source_line_number
				);
		`, fileID); err != nil {
			return itemBatchImportResult{}, err
		}
	}

	if _, err := tx.Exec(ctx, `
		update erp_sync_files
		set
			record_count = $2,
			status = 'imported',
			updated_at = now()
		where id = $1::uuid;
	`, fileID, len(input.Batch.Rows)); err != nil {
		return itemBatchImportResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return itemBatchImportResult{}, err
	}
	tx = nil
	return itemBatchImportResult{Imported: true, Rows: len(input.Batch.Rows), FileID: fileID, StoreCNPJ: input.Batch.StoreCNPJ}, nil
}

func (repository *PostgresRepository) ImportCustomerBatch(ctx context.Context, input customerBatchImportInput) (itemBatchImportResult, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return itemBatchImportResult{}, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	fileID, inserted, err := repository.insertSyncFile(ctx, tx, insertSyncFileInput{
		RunID:      input.RunID,
		Store:      input.Store,
		DataType:   input.DataType,
		SourceName: input.Batch.SourceFileName,
		SourcePath: input.Batch.SourceFileName,
		SourceKind: SyncModeBootstrapMarkdown,
		BatchDate:  input.Batch.BatchDate,
		Checksum:   input.Batch.ChecksumSHA256,
		Rows:       len(input.Batch.Rows),
		ImportedAt: input.ImportedAt,
		StoreCNPJ:  input.Batch.StoreCNPJ,
	})
	if err != nil {
		return itemBatchImportResult{}, err
	}
	if !inserted {
		if err := tx.Rollback(ctx); err != nil {
			return itemBatchImportResult{}, err
		}
		tx = nil
		return itemBatchImportResult{Imported: false, Rows: 0, StoreCNPJ: input.Batch.StoreCNPJ}, nil
	}

	if len(input.Batch.Rows) > 0 {
		if _, err := tx.CopyFrom(
			ctx,
			pgx.Identifier{"erp_customer_raw"},
			[]string{
				"run_id", "file_id", "tenant_id", "store_id", "store_code", "store_cnpj", "source_file_name", "source_batch_date", "source_line_number",
				"name", "nickname", "cpf", "email", "phone", "mobile", "gender", "birthday_raw", "street", "number", "complement", "neighborhood",
				"city", "uf", "country", "zipcode", "employee_id", "registered_at_raw", "original_id", "identifier", "tags", "created_at_imported",
			},
			pgx.CopyFromSlice(len(input.Batch.Rows), func(index int) ([]any, error) {
				row := input.Batch.Rows[index]
				return []any{
					input.RunID, fileID, input.Store.TenantID, input.Store.StoreID, row.StoreCode, row.StoreCNPJ, row.SourceFileName, row.SourceBatchDate, row.SourceLineNumber,
					row.Name, row.Nickname, row.CPF, row.Email, row.Phone, row.Mobile, row.Gender, row.BirthdayRaw, row.Street, row.Number, row.Complement, row.Neighborhood,
					row.City, row.UF, row.Country, row.Zipcode, row.EmployeeID, row.RegisteredAtRaw, row.OriginalID, row.Identifier, row.Tags, input.ImportedAt,
				}, nil
			}),
		); err != nil {
			return itemBatchImportResult{}, err
		}
	}

	if _, err := tx.Exec(ctx, `
		update erp_sync_files
		set record_count = $2, status = 'imported', updated_at = now()
		where id = $1::uuid;
	`, fileID, len(input.Batch.Rows)); err != nil {
		return itemBatchImportResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return itemBatchImportResult{}, err
	}
	tx = nil
	return itemBatchImportResult{Imported: true, Rows: len(input.Batch.Rows), FileID: fileID, StoreCNPJ: input.Batch.StoreCNPJ}, nil
}

func (repository *PostgresRepository) ImportEmployeeBatch(ctx context.Context, input employeeBatchImportInput) (itemBatchImportResult, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return itemBatchImportResult{}, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	fileID, inserted, err := repository.insertSyncFile(ctx, tx, insertSyncFileInput{
		RunID:      input.RunID,
		Store:      input.Store,
		DataType:   input.DataType,
		SourceName: input.Batch.SourceFileName,
		SourcePath: input.Batch.SourceFileName,
		SourceKind: SyncModeBootstrapMarkdown,
		BatchDate:  input.Batch.BatchDate,
		Checksum:   input.Batch.ChecksumSHA256,
		Rows:       len(input.Batch.Rows),
		ImportedAt: input.ImportedAt,
		StoreCNPJ:  input.Batch.StoreCNPJ,
	})
	if err != nil {
		return itemBatchImportResult{}, err
	}
	if !inserted {
		if err := tx.Rollback(ctx); err != nil {
			return itemBatchImportResult{}, err
		}
		tx = nil
		return itemBatchImportResult{Imported: false, Rows: 0, StoreCNPJ: input.Batch.StoreCNPJ}, nil
	}

	if len(input.Batch.Rows) > 0 {
		if _, err := tx.CopyFrom(
			ctx,
			pgx.Identifier{"erp_employee_raw"},
			[]string{
				"run_id", "file_id", "tenant_id", "store_id", "store_code", "store_cnpj", "source_file_name", "source_batch_date", "source_line_number",
				"name", "original_id", "street", "complement", "city", "uf", "zipcode", "is_active_raw", "created_at_imported",
			},
			pgx.CopyFromSlice(len(input.Batch.Rows), func(index int) ([]any, error) {
				row := input.Batch.Rows[index]
				return []any{
					input.RunID, fileID, input.Store.TenantID, input.Store.StoreID, row.StoreCode, row.StoreCNPJ, row.SourceFileName, row.SourceBatchDate, row.SourceLineNumber,
					row.Name, row.OriginalID, row.Street, row.Complement, row.City, row.UF, row.Zipcode, row.IsActiveRaw, input.ImportedAt,
				}, nil
			}),
		); err != nil {
			return itemBatchImportResult{}, err
		}
	}

	if _, err := tx.Exec(ctx, `
		update erp_sync_files
		set record_count = $2, status = 'imported', updated_at = now()
		where id = $1::uuid;
	`, fileID, len(input.Batch.Rows)); err != nil {
		return itemBatchImportResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return itemBatchImportResult{}, err
	}
	tx = nil
	return itemBatchImportResult{Imported: true, Rows: len(input.Batch.Rows), FileID: fileID, StoreCNPJ: input.Batch.StoreCNPJ}, nil
}

func (repository *PostgresRepository) ImportOrderBatch(ctx context.Context, input orderBatchImportInput) (itemBatchImportResult, error) {
	tableName := "erp_order_raw"
	if input.DataType == DataTypeOrderCanceled {
		tableName = "erp_order_canceled_raw"
	}

	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return itemBatchImportResult{}, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	fileID, inserted, err := repository.insertSyncFile(ctx, tx, insertSyncFileInput{
		RunID:      input.RunID,
		Store:      input.Store,
		DataType:   input.DataType,
		SourceName: input.Batch.SourceFileName,
		SourcePath: input.Batch.SourceFileName,
		SourceKind: SyncModeBootstrapMarkdown,
		BatchDate:  input.Batch.BatchDate,
		Checksum:   input.Batch.ChecksumSHA256,
		Rows:       len(input.Batch.Rows),
		ImportedAt: input.ImportedAt,
		StoreCNPJ:  input.Batch.StoreCNPJ,
	})
	if err != nil {
		return itemBatchImportResult{}, err
	}
	if !inserted {
		if err := tx.Rollback(ctx); err != nil {
			return itemBatchImportResult{}, err
		}
		tx = nil
		return itemBatchImportResult{Imported: false, Rows: 0, StoreCNPJ: input.Batch.StoreCNPJ}, nil
	}

	if len(input.Batch.Rows) > 0 {
		if _, err := tx.CopyFrom(
			ctx,
			pgx.Identifier{tableName},
			[]string{
				"run_id", "file_id", "tenant_id", "store_id", "store_code", "store_cnpj", "source_file_name", "source_batch_date", "source_line_number",
				"order_id", "identifier", "customer_id", "order_date_raw", "order_date", "total_amount_raw", "total_amount_cents", "product_return_raw", "product_return_cents",
				"sku", "amount_raw", "amount_cents", "quantity_raw", "quantity", "employee_id", "payment_type", "total_exclusion_raw", "total_exclusion_cents", "total_debit_raw", "total_debit_cents", "created_at_imported",
			},
			pgx.CopyFromSlice(len(input.Batch.Rows), func(index int) ([]any, error) {
				row := input.Batch.Rows[index]
				var quantity any
				if row.Quantity != nil {
					quantity = int32(*row.Quantity)
				}
				return []any{
					input.RunID, fileID, input.Store.TenantID, input.Store.StoreID, row.StoreCode, row.StoreCNPJ, row.SourceFileName, row.SourceBatchDate, row.SourceLineNumber,
					row.OrderID, row.Identifier, row.CustomerID, row.OrderDateRaw, row.OrderDate, row.TotalAmountRaw, row.TotalAmountCents, row.ProductReturnRaw, row.ProductReturnCents,
					row.SKU, row.AmountRaw, row.AmountCents, row.QuantityRaw, quantity, row.EmployeeID, row.PaymentType, row.TotalExclusionRaw, row.TotalExclusionCents, row.TotalDebitRaw, row.TotalDebitCents, input.ImportedAt,
				}, nil
			}),
		); err != nil {
			return itemBatchImportResult{}, err
		}
	}

	if _, err := tx.Exec(ctx, `
		update erp_sync_files
		set record_count = $2, status = 'imported', updated_at = now()
		where id = $1::uuid;
	`, fileID, len(input.Batch.Rows)); err != nil {
		return itemBatchImportResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return itemBatchImportResult{}, err
	}
	tx = nil
	return itemBatchImportResult{Imported: true, Rows: len(input.Batch.Rows), FileID: fileID, StoreCNPJ: input.Batch.StoreCNPJ}, nil
}

type insertSyncFileInput struct {
	RunID      string
	Store      StoreScope
	DataType   string
	SourceName string
	SourcePath string
	SourceKind string
	BatchDate  string
	Checksum   string
	Rows       int
	ImportedAt time.Time
	StoreCNPJ  string
}

func (repository *PostgresRepository) insertSyncFile(ctx context.Context, tx pgx.Tx, input insertSyncFileInput) (string, bool, error) {
	var fileID string
	err := tx.QueryRow(ctx, `
		insert into erp_sync_files (
			run_id,
			tenant_id,
			store_id,
			store_code,
			store_cnpj,
			data_type,
			source_name,
			source_path,
			source_kind,
			source_batch_date,
			checksum_sha256,
			record_count,
			status,
			imported_at,
			created_at,
			updated_at
		) values (
			$1::uuid,
			$2::uuid,
			$3::uuid,
			$4,
			nullif($5, ''),
			$6,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12,
			'pending',
			$13,
			now(),
			now()
		)
		on conflict (tenant_id, store_id, data_type, source_name, checksum_sha256) do nothing
		returning id::text;
	`, input.RunID, input.Store.TenantID, input.Store.StoreID, input.Store.StoreCode, input.StoreCNPJ, input.DataType, input.SourceName, input.SourcePath, input.SourceKind, input.BatchDate, input.Checksum, input.Rows, input.ImportedAt).Scan(&fileID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	return fileID, true, nil
}

func (repository *PostgresRepository) getLastRun(ctx context.Context, store StoreScope, dataType string) (*SyncRunSummary, error) {
	row := repository.pool.QueryRow(ctx, `
		select
			id::text,
			data_type,
			mode,
			status,
			files_seen,
			files_imported,
			files_skipped,
			rows_read,
			raw_rows_imported,
			coalesce(source_path, ''),
			coalesce(error_message, ''),
			started_at,
			finished_at,
			coalesce(store_cnpj, '')
		from erp_sync_runs
		where tenant_id = $1::uuid
		  and store_id = $2::uuid
		  and data_type = $3
		order by started_at desc
		limit 1;
	`, store.TenantID, store.StoreID, dataType)

	var summary SyncRunSummary
	if err := row.Scan(
		&summary.ID,
		&summary.DataType,
		&summary.Mode,
		&summary.Status,
		&summary.FilesSeen,
		&summary.FilesImported,
		&summary.FilesSkipped,
		&summary.RowsRead,
		&summary.RowsImported,
		&summary.SourcePath,
		&summary.ErrorMessage,
		&summary.StartedAt,
		&summary.FinishedAt,
		&summary.StoreCNPJ,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &summary, nil
}

func (repository *PostgresRepository) getLastFile(ctx context.Context, store StoreScope, dataType string) (*SyncFileSummary, error) {
	row := repository.pool.QueryRow(ctx, `
		select
			id::text,
			data_type,
			source_name,
			source_kind,
			checksum_sha256,
			record_count,
			imported_at,
			coalesce(store_cnpj, '')
		from erp_sync_files
		where tenant_id = $1::uuid
		  and store_id = $2::uuid
		  and data_type = $3
		order by imported_at desc
		limit 1;
	`, store.TenantID, store.StoreID, dataType)

	var summary SyncFileSummary
	if err := row.Scan(
		&summary.ID,
		&summary.DataType,
		&summary.SourceName,
		&summary.SourceKind,
		&summary.ChecksumSHA256,
		&summary.RecordCount,
		&summary.ImportedAt,
		&summary.StoreCNPJ,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &summary, nil
}

func (repository *PostgresRepository) CanAccessTenant(ctx context.Context, principal auth.Principal, tenantID string) (bool, error) {
	normalizedTenantID := strings.TrimSpace(tenantID)
	if normalizedTenantID == "" {
		return false, nil
	}

	if principalTenantID := strings.TrimSpace(principal.TenantID); principalTenantID != "" {
		return principalTenantID == normalizedTenantID, nil
	}

	var (
		query string
		args  []any
	)

	switch principal.Role {
	case auth.RolePlatformAdmin:
		query = `
			select exists(
				select 1
				from tenants t
				where t.id::text = $1
				  and t.is_active = true
			);
		`
		args = []any{normalizedTenantID}
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		query = `
			select exists(
				select 1
				from tenants t
				join user_tenant_roles utr on utr.tenant_id = t.id
				where t.id::text = $1
				  and utr.user_id::text = $2
				  and t.is_active = true
			);
		`
		args = []any{normalizedTenantID, strings.TrimSpace(principal.UserID)}
	default:
		query = `
			select exists(
				select 1
				from tenants t
				join stores s on s.tenant_id = t.id
				join user_store_roles usr on usr.store_id = s.id
				where t.id::text = $1
				  and usr.user_id::text = $2
				  and t.is_active = true
				  and s.is_active = true
			);
		`
		args = []any{normalizedTenantID, strings.TrimSpace(principal.UserID)}
	}

	var allowed bool
	if err := repository.pool.QueryRow(ctx, query, args...).Scan(&allowed); err != nil {
		return false, err
	}
	return allowed, nil
}

func (repository *PostgresRepository) ResolveDefaultTenantID(ctx context.Context, principal auth.Principal) (string, error) {
	if tenantID := strings.TrimSpace(principal.TenantID); tenantID != "" {
		return tenantID, nil
	}

	var (
		query string
		args  []any
	)

	switch principal.Role {
	case auth.RolePlatformAdmin:
		query = `
			select t.id::text
			from tenants t
			where t.is_active = true
			order by t.name asc, t.created_at asc, t.id asc
			limit 2;
		`
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		query = `
			select distinct t.id::text
			from tenants t
			join user_tenant_roles utr on utr.tenant_id = t.id
			where utr.user_id = $1::uuid
			  and t.is_active = true
			order by t.id asc
			limit 2;
		`
		args = []any{principal.UserID}
	default:
		query = `
			select distinct t.id::text
			from tenants t
			join stores s on s.tenant_id = t.id
			join user_store_roles usr on usr.store_id = s.id
			where usr.user_id = $1::uuid
			  and t.is_active = true
			  and s.is_active = true
			order by t.id asc
			limit 2;
		`
		args = []any{principal.UserID}
	}

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	tenantIDs := make([]string, 0, 2)
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			return "", err
		}
		tenantIDs = append(tenantIDs, tenantID)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(tenantIDs) != 1 {
		return "", ErrTenantRequired
	}
	return tenantIDs[0], nil
}

func requiresStoreScopedFilter(role auth.Role) bool {
	switch role {
	case auth.RoleConsultant, auth.RoleManager, auth.RoleStoreTerminal:
		return true
	default:
		return false
	}
}

type execQueryer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
}
