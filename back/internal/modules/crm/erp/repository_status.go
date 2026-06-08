package erp

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type syncFileImportState struct {
	SourceName  string
	DataType    string
	SourceKind  string
	Status      string
	RecordCount int
	ImportedAt  *time.Time
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

func (repository *PostgresRepository) ListSyncRuns(ctx context.Context, store StoreScope, query RunsQuery) (SyncRunsListResponse, error) {
	offset := (query.Page - 1) * query.PageSize
	response := SyncRunsListResponse{
		Store:    store,
		DataType: query.DataType,
		Page:     query.Page,
		PageSize: query.PageSize,
		Items:    make([]SyncRunSummary, 0, query.PageSize),
	}

	if err := repository.pool.QueryRow(ctx, `
		select count(*)
		from erp_sync_runs
		where tenant_id = $1::uuid
		  and store_id = $2::uuid
		  and ($3 = '' or data_type = $3);
	`, store.TenantID, store.StoreID, query.DataType).Scan(&response.Total); err != nil {
		return SyncRunsListResponse{}, err
	}

	rows, err := repository.pool.Query(ctx, `
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
		  and ($3 = '' or data_type = $3)
		order by started_at desc
		limit $4 offset $5;
	`, store.TenantID, store.StoreID, query.DataType, query.PageSize, offset)
	if err != nil {
		return SyncRunsListResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var summary SyncRunSummary
		if err := rows.Scan(
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
			return SyncRunsListResponse{}, err
		}
		response.Items = append(response.Items, summary)
	}
	if err := rows.Err(); err != nil {
		return SyncRunsListResponse{}, err
	}

	return response, nil
}

func (repository *PostgresRepository) ListLatestSyncFileStates(ctx context.Context, store StoreScope) (map[string]syncFileImportState, error) {
	rows, err := repository.pool.Query(ctx, `
		select distinct on (source_name)
			source_name,
			data_type,
			source_kind,
			status,
			record_count,
			imported_at
		from erp_sync_files
		where tenant_id = $1::uuid
		  and store_id = $2::uuid
		order by source_name, updated_at desc, imported_at desc, created_at desc;
	`, store.TenantID, store.StoreID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make(map[string]syncFileImportState)
	for rows.Next() {
		var state syncFileImportState
		var importedAt time.Time
		if err := rows.Scan(
			&state.SourceName,
			&state.DataType,
			&state.SourceKind,
			&state.Status,
			&state.RecordCount,
			&importedAt,
		); err != nil {
			return nil, err
		}
		state.ImportedAt = &importedAt
		states[state.SourceName] = state
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return states, nil
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
