package erp

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type rawMirrorRow struct {
	SourceLineNumber int
	StoreIDRaw       string
	RawValues        []string
	RawPayload       map[string]string
}

func (repository *PostgresRepository) refreshRawMirror(ctx context.Context, tx pgx.Tx, tableName string, fileID string, includeStoreID bool, rows []rawMirrorRow) (int, error) {
	if strings.TrimSpace(fileID) == "" || len(rows) == 0 {
		return 0, nil
	}

	safeTableName, err := rawMirrorTableName(tableName)
	if err != nil {
		return 0, err
	}

	if _, err := tx.Exec(ctx, `drop table if exists pg_temp.erp_raw_refresh_tmp;`); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `
		create temp table erp_raw_refresh_tmp (
			source_line_number integer primary key,
			store_id_raw text not null default '',
			raw_values jsonb not null,
			raw_payload jsonb not null
		) on commit drop;
	`); err != nil {
		return 0, err
	}

	if _, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"erp_raw_refresh_tmp"},
		[]string{"source_line_number", "store_id_raw", "raw_values", "raw_payload"},
		pgx.CopyFromSlice(len(rows), func(index int) ([]any, error) {
			row := rows[index]
			return []any{row.SourceLineNumber, row.StoreIDRaw, row.RawValues, row.RawPayload}, nil
		}),
	); err != nil {
		return 0, err
	}

	setClause := `
		raw_values = tmp.raw_values,
		raw_payload = tmp.raw_payload`
	if includeStoreID {
		setClause = `
		store_id_raw = tmp.store_id_raw,
		raw_values = tmp.raw_values,
		raw_payload = tmp.raw_payload`
	}

	commandTag, err := tx.Exec(ctx, fmt.Sprintf(`
		update %s raw
		set %s
		from erp_raw_refresh_tmp tmp
		where raw.file_id = $1::uuid
		  and raw.source_line_number = tmp.source_line_number;
	`, safeTableName, setClause), fileID)
	if err != nil {
		return 0, err
	}

	return int(commandTag.RowsAffected()), nil
}

func rawMirrorTableName(tableName string) (string, error) {
	switch tableName {
	case "erp_item_raw":
		return "queue.erp_item_raw", nil
	case "erp_customer_raw", "erp_employee_raw", "erp_order_raw", "erp_order_canceled_raw":
		return tableName, nil
	default:
		return "", ErrUnsupportedDataType
	}
}

func rawMirrorIdentifier(tableName string) (pgx.Identifier, error) {
	switch tableName {
	case "erp_item_raw":
		return pgx.Identifier{"queue", tableName}, nil
	case "erp_customer_raw", "erp_employee_raw", "erp_order_raw", "erp_order_canceled_raw":
		return pgx.Identifier{tableName}, nil
	default:
		return nil, ErrUnsupportedDataType
	}
}

func rawMirrorRowsFromItems(rows []ItemRawRecord) []rawMirrorRow {
	result := make([]rawMirrorRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, rawMirrorRow{
			SourceLineNumber: row.SourceLineNumber,
			RawValues:        row.RawValues,
			RawPayload:       row.RawPayload,
		})
	}
	return result
}

func rawMirrorRowsFromCustomers(rows []CustomerRawRecord) []rawMirrorRow {
	result := make([]rawMirrorRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, rawMirrorRow{
			SourceLineNumber: row.SourceLineNumber,
			StoreIDRaw:       row.StoreIDRaw,
			RawValues:        row.RawValues,
			RawPayload:       row.RawPayload,
		})
	}
	return result
}

func rawMirrorRowsFromEmployees(rows []EmployeeRawRecord) []rawMirrorRow {
	result := make([]rawMirrorRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, rawMirrorRow{
			SourceLineNumber: row.SourceLineNumber,
			StoreIDRaw:       row.StoreIDRaw,
			RawValues:        row.RawValues,
			RawPayload:       row.RawPayload,
		})
	}
	return result
}

func rawMirrorRowsFromOrders(rows []OrderRawRecord) []rawMirrorRow {
	result := make([]rawMirrorRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, rawMirrorRow{
			SourceLineNumber: row.SourceLineNumber,
			StoreIDRaw:       row.StoreIDRaw,
			RawValues:        row.RawValues,
			RawPayload:       row.RawPayload,
		})
	}
	return result
}
