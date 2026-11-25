package operations

import "context"

// ListRecentServices pagina o historico de atendimentos encerrados das lojas
// informadas, do mais recente para o mais antigo. Usa o indice
// (store_id, finished_at desc). Cancelados entram: a auditoria mostra o status.
func (repository *PostgresRepository) ListRecentServices(ctx context.Context, storeIDs []string, limit, offset int) ([]RecentServiceItem, int, error) {
	var total int
	if err := repository.pool.QueryRow(ctx, `
		select count(*)
		from queue.operation_service_history
		where store_id = any($1::uuid[])
	`, storeIDs).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := repository.pool.Query(ctx, `
		select
			service_id,
			store_id::text,
			person_id::text,
			person_name,
			started_at,
			finished_at,
			duration_ms,
			finish_outcome,
			start_mode,
			queue_wait_ms,
			skipped_count,
			sale_amount,
			customer_name,
			is_existing_customer,
			coalesce(close_reason, 'manual'),
			coalesce(validation_status, 'validated')
		from queue.operation_service_history
		where store_id = any($1::uuid[])
		order by finished_at desc, created_at desc
		limit $2 offset $3
	`, storeIDs, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]RecentServiceItem, 0, limit)
	for rows.Next() {
		var item RecentServiceItem
		if err := rows.Scan(
			&item.ServiceID,
			&item.StoreID,
			&item.PersonID,
			&item.PersonName,
			&item.StartedAt,
			&item.FinishedAt,
			&item.DurationMs,
			&item.FinishOutcome,
			&item.StartMode,
			&item.QueueWaitMs,
			&item.SkippedCount,
			&item.SaleAmount,
			&item.CustomerName,
			&item.IsExistingCustomer,
			&item.CloseReason,
			&item.ValidationStatus,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	return items, total, rows.Err()
}
