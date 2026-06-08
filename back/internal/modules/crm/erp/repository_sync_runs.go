package erp

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func (repository *PostgresRepository) StartSyncRun(ctx context.Context, store StoreScope, dataType string, mode string, sourcePath string, triggeredBy string) (syncRunStart, error) {
	var started syncRunStart
	err := repository.pool.QueryRow(ctx, `
		insert into erp_sync_runs (
			tenant_id,
			store_id,
			store_code,
			store_cnpj,
			data_type,
			mode,
			triggered_by,
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
			$9,
			now(),
			now(),
			now()
		)
		returning id::text, started_at;
	`, store.TenantID, store.StoreID, store.StoreCode, store.StoreCNPJ, dataType, mode, firstNonEmpty(triggeredBy, SyncTriggeredByManual), sourcePath, SyncStatusRunning).Scan(&started.ID, &started.StartedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "erp_sync_runs_one_running_csv_ftp_per_store_idx" {
			return syncRunStart{}, ErrSyncAlreadyRunning
		}
		return syncRunStart{}, err
	}
	return started, nil
}

func (repository *PostgresRepository) UpdateSyncRunProgress(ctx context.Context, runID string, filesSeen int, filesImported int, filesSkipped int, rowsRead int, rowsImported int, storeCNPJ string) error {
	_, err := repository.pool.Exec(ctx, `
		update erp_sync_runs
		set
			files_seen = $2,
			files_imported = $3,
			files_skipped = $4,
			rows_read = $5,
			raw_rows_imported = $6,
			store_cnpj = coalesce(nullif($7, ''), store_cnpj),
			updated_at = now()
		where id = $1::uuid;
	`, runID, filesSeen, filesImported, filesSkipped, rowsRead, rowsImported, storeCNPJ)
	return err
}

func (repository *PostgresRepository) SyncFileExists(ctx context.Context, store StoreScope, dataType string, sourceName string, checksum string) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists(
			select 1
			from erp_sync_files
			where tenant_id = $1::uuid
			  and store_id = $2::uuid
			  and data_type = $3
			  and source_name = $4
			  and checksum_sha256 = $5
		);
	`, store.TenantID, store.StoreID, dataType, sourceName, checksum).Scan(&exists)
	return exists, err
}

func (repository *PostgresRepository) HasRunningCSVSyncRun(ctx context.Context, store StoreScope) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists(
			select 1
			from erp_sync_runs
			where tenant_id = $1::uuid
			  and store_id = $2::uuid
			  and mode = $3
			  and status = $4
		);
	`, store.TenantID, store.StoreID, SyncModeCSVFTP, SyncStatusRunning).Scan(&exists)
	return exists, err
}

func (repository *PostgresRepository) HasRecentCSVSyncRun(ctx context.Context, store StoreScope, since time.Time) (bool, error) {
	if since.IsZero() {
		return false, nil
	}

	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists(
			select 1
			from erp_sync_runs
			where tenant_id = $1::uuid
			  and store_id = $2::uuid
			  and mode = $3
			  and started_at >= $4
		);
	`, store.TenantID, store.StoreID, SyncModeCSVFTP, since).Scan(&exists)
	return exists, err
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
