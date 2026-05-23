package erp

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type insertSyncFileInput struct {
	RunID         string
	Store         StoreScope
	DataType      string
	SourceName    string
	SourcePath    string
	SourceKind    string
	BatchDate     string
	ExtractedAt   *time.Time
	DataReference *time.Time
	SizeBytes     int64
	ErrorMessage  string
	Checksum      string
	Rows          int
	ImportedAt    time.Time
	StoreCNPJ     string
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
			source_extracted_at,
			source_data_reference,
			source_size_bytes,
			error_message,
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
			$13,
			$14,
			$15,
			$16,
			'pending',
			$17,
			now(),
			now()
		)
		on conflict (tenant_id, store_id, data_type, source_name, checksum_sha256) do nothing
		returning id::text;
	`, input.RunID, input.Store.TenantID, input.Store.StoreID, input.Store.StoreCode, input.StoreCNPJ, input.DataType, input.SourceName, input.SourcePath, input.SourceKind, input.BatchDate, input.ExtractedAt, input.DataReference, nullIfZeroInt64(input.SizeBytes), input.ErrorMessage, input.Checksum, input.Rows, input.ImportedAt).Scan(&fileID)
	if err != nil {
		if err == pgx.ErrNoRows {
			existingID, existingErr := repository.findSyncFileID(ctx, tx, input)
			if existingErr != nil {
				return "", false, existingErr
			}
			return existingID, false, nil
		}
		return "", false, err
	}
	return fileID, true, nil
}

func (repository *PostgresRepository) findSyncFileID(ctx context.Context, tx pgx.Tx, input insertSyncFileInput) (string, error) {
	var fileID string
	err := tx.QueryRow(ctx, `
		select id::text
		from erp_sync_files
		where tenant_id = $1::uuid
		  and store_id = $2::uuid
		  and data_type = $3
		  and source_name = $4
		  and checksum_sha256 = $5
		order by imported_at desc, created_at desc
		limit 1;
	`, input.Store.TenantID, input.Store.StoreID, input.DataType, input.SourceName, input.Checksum).Scan(&fileID)
	if err != nil {
		return "", err
	}
	return fileID, nil
}

func nullIfZeroInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
