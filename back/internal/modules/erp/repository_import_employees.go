package erp

import (
	"context"

	"github.com/jackc/pgx/v5"
)

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
		RunID:         input.RunID,
		Store:         input.Store,
		DataType:      input.DataType,
		SourceName:    input.Batch.SourceFileName,
		SourcePath:    firstNonEmpty(input.Batch.SourcePath, input.Batch.SourceFileName),
		SourceKind:    firstNonEmpty(input.Batch.SourceKind, SyncModeBootstrapMarkdown),
		BatchDate:     input.Batch.BatchDate,
		ExtractedAt:   input.Batch.SourceExtractedAt,
		DataReference: input.Batch.SourceDataReference,
		SizeBytes:     input.Batch.SourceSizeBytes,
		ErrorMessage:  input.Batch.ErrorMessage,
		Checksum:      input.Batch.ChecksumSHA256,
		Rows:          len(input.Batch.Rows),
		ImportedAt:    input.ImportedAt,
		StoreCNPJ:     input.Batch.StoreCNPJ,
	})
	if err != nil {
		return itemBatchImportResult{}, err
	}
	if !inserted {
		refreshedRows, refreshErr := repository.refreshRawMirror(ctx, tx, "erp_employee_raw", fileID, true, rawMirrorRowsFromEmployees(input.Batch.Rows))
		if refreshErr != nil {
			return itemBatchImportResult{}, refreshErr
		}
		if err := tx.Commit(ctx); err != nil {
			return itemBatchImportResult{}, err
		}
		tx = nil
		return itemBatchImportResult{Imported: false, Rows: 0, FileID: fileID, StoreCNPJ: input.Batch.StoreCNPJ, RefreshedRows: refreshedRows}, nil
	}

	if len(input.Batch.Rows) > 0 {
		if _, err := tx.CopyFrom(
			ctx,
			pgx.Identifier{"erp_employee_raw"},
			[]string{
				"run_id", "file_id", "tenant_id", "store_id", "store_code", "store_cnpj", "source_file_name", "source_batch_date", "source_line_number",
				"name", "store_id_raw", "original_id", "street", "complement", "city", "uf", "zipcode", "is_active_raw", "raw_values", "raw_payload", "created_at_imported",
			},
			pgx.CopyFromSlice(len(input.Batch.Rows), func(index int) ([]any, error) {
				row := input.Batch.Rows[index]
				return []any{
					input.RunID, fileID, input.Store.TenantID, input.Store.StoreID, row.StoreCode, row.StoreCNPJ, row.SourceFileName, row.SourceBatchDate, row.SourceLineNumber,
					row.Name, row.StoreIDRaw, row.OriginalID, row.Street, row.Complement, row.City, row.UF, row.Zipcode, row.IsActiveRaw, row.RawValues, row.RawPayload, input.ImportedAt,
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
