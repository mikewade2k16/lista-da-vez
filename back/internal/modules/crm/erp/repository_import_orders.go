package erp

import (
	"context"

	"github.com/jackc/pgx/v5"
)

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
		refreshedRows, refreshErr := repository.refreshRawMirror(ctx, tx, tableName, fileID, true, rawMirrorRowsFromOrders(input.Batch.Rows))
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
			pgx.Identifier{tableName},
			[]string{
				"run_id", "file_id", "tenant_id", "store_id", "store_code", "store_cnpj", "source_file_name", "source_batch_date", "source_line_number",
				"order_id", "identifier", "store_id_raw", "customer_id", "order_date_raw", "order_date", "total_amount_raw", "total_amount_cents", "product_return_raw", "product_return_cents",
				"sku", "amount_raw", "amount_cents", "quantity_raw", "quantity", "employee_id", "payment_type", "total_exclusion_raw", "total_exclusion_cents", "total_debit_raw", "total_debit_cents", "raw_values", "raw_payload", "created_at_imported",
			},
			pgx.CopyFromSlice(len(input.Batch.Rows), func(index int) ([]any, error) {
				row := input.Batch.Rows[index]
				var quantity any
				if row.Quantity != nil {
					quantity = int32(*row.Quantity)
				}
				return []any{
					input.RunID, fileID, input.Store.TenantID, input.Store.StoreID, row.StoreCode, row.StoreCNPJ, row.SourceFileName, row.SourceBatchDate, row.SourceLineNumber,
					row.OrderID, row.Identifier, row.StoreIDRaw, row.CustomerID, row.OrderDateRaw, row.OrderDate, row.TotalAmountRaw, row.TotalAmountCents, row.ProductReturnRaw, row.ProductReturnCents,
					row.SKU, row.AmountRaw, row.AmountCents, row.QuantityRaw, quantity, row.EmployeeID, row.PaymentType, row.TotalExclusionRaw, row.TotalExclusionCents, row.TotalDebitRaw, row.TotalDebitCents, row.RawValues, row.RawPayload, input.ImportedAt,
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
