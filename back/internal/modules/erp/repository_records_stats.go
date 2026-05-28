package erp

import (
	"context"
	"fmt"
	"strings"
)

func (repository *PostgresRepository) GetRecordsStats(ctx context.Context, store StoreScope, query RecordsStatsQuery) (RecordsStatsResponse, error) {
	resp := RecordsStatsResponse{DataType: query.DataType}
	search := strings.TrimSpace(query.Search)
	likeSearch := "%" + search + "%"
	specificSearch := strings.TrimSpace(query.SpecificSearch)
	specificLike := specificSearch + "%"
	specificDigits := onlyDigits(specificSearch)
	specificDigitsLike := specificDigits + "%"

	switch query.DataType {
	case DataTypeOrder, DataTypeOrderCanceled:
		var tableName string
		if query.DataType == DataTypeOrderCanceled {
			tableName = "erp_order_canceled_raw"
		} else {
			tableName = "erp_order_raw"
		}

		sql := fmt.Sprintf(`
			WITH filtered AS NOT MATERIALIZED (
				SELECT *
				FROM %s
				WHERE tenant_id = $1::uuid AND store_id = $2::uuid
					AND ($3 = '' OR source_batch_date >= $3::date)
					AND ($4 = '' OR source_batch_date <= $4::date)
					AND (
						$5 = ''
						OR order_id ilike $6
						OR identifier ilike $6
						OR store_id_raw ilike $6
						OR customer_id ilike $6
						OR order_date_raw ilike $6
						OR total_amount_raw ilike $6
						OR sku ilike $6
						OR amount_raw ilike $6
						OR quantity_raw ilike $6
						OR employee_id ilike $6
						OR payment_type ilike $6
						OR total_exclusion_raw ilike $6
						OR total_debit_raw ilike $6
					)
					AND (
						$7 = ''
						OR order_id ilike $8
						OR ($9 <> '' AND order_id ilike $10)
					)
			),
			latest_order_files AS (
				SELECT DISTINCT ON (order_id)
					order_id,
					file_id
				FROM filtered
				WHERE nullif(trim(order_id), '') is not null
				ORDER BY order_id, source_batch_date DESC, created_at_imported DESC, source_file_name DESC, file_id DESC
			),
			latest_order_lines AS (
				SELECT filtered.*
				FROM filtered
				JOIN latest_order_files latest
				  ON latest.order_id = filtered.order_id
				 AND latest.file_id = filtered.file_id
			),
			order_totals AS (
				SELECT
					order_id,
					coalesce(
						nullif(max(coalesce(total_amount_cents, 0)), 0),
						sum(coalesce(amount_cents, 0)),
						0
					)::bigint AS order_total_cents,
					coalesce(sum(case when coalesce(quantity, 0) > 0 then quantity else 1 end), 0)::bigint AS total_items
				FROM latest_order_lines
				GROUP BY order_id
			)
			SELECT
				count(*)::bigint AS order_count,
				coalesce(sum(order_total_cents), 0)::bigint AS total_amount_cents,
				CASE
					WHEN count(*) > 0 THEN (coalesce(sum(order_total_cents), 0)::bigint / count(*)::bigint)
					ELSE 0
				END AS avg_amount_cents,
				coalesce(sum(total_items), 0)::bigint AS total_items
			FROM order_totals;
		`, tableName)

		var orderCount, totalAmountCents, avgAmountCents, totalItems int64
		err := repository.pool.QueryRow(ctx, sql,
			store.TenantID,
			store.StoreID,
			query.DateFrom,
			query.DateTo,
			search,
			likeSearch,
			specificSearch,
			specificLike,
			specificDigits,
			specificDigitsLike,
		).Scan(&orderCount, &totalAmountCents, &avgAmountCents, &totalItems)
		if err != nil {
			return RecordsStatsResponse{}, err
		}

		resp.OrderCount = orderCount
		resp.TotalAmountCents = totalAmountCents
		resp.AvgAmountCents = avgAmountCents
		resp.TotalItems = totalItems
		if orderCount > 0 {
			resp.PA = float64(totalItems) / float64(orderCount)
		}

	case DataTypeCustomer:
		sql := `
			SELECT
				count(*),
				count(distinct coalesce(nullif(cpf, ''), id::text))
			FROM erp_customer_raw
			WHERE tenant_id = $1::uuid AND store_id = $2::uuid
				AND ($3 = '' OR source_batch_date >= $3::date)
				AND ($4 = '' OR source_batch_date <= $4::date)
				AND (
					$5 = ''
					OR name ilike $6
					OR nickname ilike $6
					OR cpf ilike $6
					OR email ilike $6
					OR phone ilike $6
					OR mobile ilike $6
					OR city ilike $6
					OR uf ilike $6
					OR zipcode ilike $6
					OR employee_id ilike $6
					OR store_id_raw ilike $6
					OR original_id ilike $6
					OR identifier ilike $6
					OR tags ilike $6
				)
				AND (
					$7 = ''
					OR cpf ilike $8
					OR ($9 <> '' AND regexp_replace(cpf, '\D', '', 'g') ilike $10)
				)
		`

		var totalCount, dedupedCount int64
		err := repository.pool.QueryRow(ctx, sql,
			store.TenantID,
			store.StoreID,
			query.DateFrom,
			query.DateTo,
			search,
			likeSearch,
			specificSearch,
			specificLike,
			specificDigits,
			specificDigitsLike,
		).Scan(&totalCount, &dedupedCount)
		if err != nil {
			return RecordsStatsResponse{}, err
		}

		resp.CustomerCount = dedupedCount

	default:
		return RecordsStatsResponse{}, ErrUnsupportedDataType
	}

	return resp, nil
}
