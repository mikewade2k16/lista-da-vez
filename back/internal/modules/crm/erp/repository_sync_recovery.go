package erp

import (
	"context"
	"time"
)

func (repository *PostgresRepository) HasAutomaticCSVSyncRunSince(ctx context.Context, since time.Time) (bool, error) {
	if since.IsZero() {
		return false, nil
	}

	var completedTypes int
	err := repository.pool.QueryRow(ctx, `
		select count(distinct data_type)
		from erp_sync_runs
		where mode = $1
		  and triggered_by = $2
		  and status = $3
		  and started_at >= $4
		  and data_type in ($5, $6, $7, $8, $9);
	`,
		SyncModeCSVFTP,
		SyncTriggeredByCron,
		SyncStatusSucceeded,
		since,
		DataTypeItem,
		DataTypeCustomer,
		DataTypeEmployee,
		DataTypeOrder,
		DataTypeOrderCanceled,
	).Scan(&completedTypes)
	return completedTypes >= len(supportedDataTypes), err
}

// RecoverOrphanedSyncRuns marks any sync run stuck in "running" for longer than maxAge as failed.
// Call this once on startup to clear runs that were interrupted by a container restart.
func (repository *PostgresRepository) RecoverOrphanedSyncRuns(ctx context.Context, maxAge time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-maxAge)
	tag, err := repository.pool.Exec(ctx, `
		update erp_sync_runs
		set
			status = $1,
			finished_at = now(),
			error_message = 'run_interrupted_by_restart',
			updated_at = now()
		where status = $2
		  and started_at < $3;
	`, SyncStatusFailed, SyncStatusRunning, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
