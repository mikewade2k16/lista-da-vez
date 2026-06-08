package erp

import (
	"context"

	"github.com/jackc/pgx/v5"
)

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
		refreshedRows, refreshErr := repository.refreshRawMirror(ctx, tx, "erp_item_raw", fileID, false, rawMirrorRowsFromItems(input.Batch.Rows))
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
		rawTableIdentifier, err := rawMirrorIdentifier("erp_item_raw")
		if err != nil {
			return itemBatchImportResult{}, err
		}

		if _, err := tx.CopyFrom(
			ctx,
			rawTableIdentifier,
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
				"raw_values",
				"raw_payload",
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
					row.RawValues,
					row.RawPayload,
					input.ImportedAt,
				}, nil
			}),
		); err != nil {
			return itemBatchImportResult{}, err
		}

		if _, err := tx.Exec(ctx, `
			insert into queue.erp_item_current (
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
				source_extracted_at,
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
				sync_file.source_extracted_at,
				raw.source_line_number,
				raw.created_at_raw,
				raw.updated_at_raw,
				raw.created_at,
				raw.updated_at,
				raw.run_id,
				raw.file_id,
				now(),
				now()
			from queue.erp_item_raw raw
			join erp_sync_files sync_file on sync_file.id = raw.file_id
			where raw.file_id = $1::uuid
			order by raw.sku, coalesce(sync_file.source_extracted_at, raw.updated_at, raw.created_at, raw.source_batch_date::timestamp) desc, raw.source_line_number desc
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
				source_extracted_at = excluded.source_extracted_at,
				source_line_number = excluded.source_line_number,
				source_created_at_raw = excluded.source_created_at_raw,
				source_updated_at_raw = excluded.source_updated_at_raw,
				source_created_at = excluded.source_created_at,
				source_updated_at = excluded.source_updated_at,
				run_id = excluded.run_id,
				file_id = excluded.file_id,
				updated_at = now()
			where
				coalesce(excluded.source_extracted_at, excluded.source_updated_at, excluded.source_created_at, excluded.source_batch_date::timestamp, to_timestamp(0)) >
					coalesce(erp_item_current.source_extracted_at, erp_item_current.source_updated_at, erp_item_current.source_created_at, erp_item_current.source_batch_date::timestamp, to_timestamp(0))
				or (
					coalesce(excluded.source_extracted_at, excluded.source_updated_at, excluded.source_created_at, excluded.source_batch_date::timestamp, to_timestamp(0)) =
						coalesce(erp_item_current.source_extracted_at, erp_item_current.source_updated_at, erp_item_current.source_created_at, erp_item_current.source_batch_date::timestamp, to_timestamp(0))
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
