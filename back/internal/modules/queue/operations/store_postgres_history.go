package operations

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/stringsx"
)

// LoadSnapshot mantem o contrato atual (historico completo) — usado pela
// operacao ao vivo (service.go via loadSnapshotState). Nao janela o historico.
func (repository *PostgresRepository) LoadSnapshot(ctx context.Context, storeID string) (SnapshotState, error) {
	return repository.LoadSnapshotWithHistorySince(ctx, storeID, 0)
}

// LoadSnapshotWithHistorySince carrega o snapshot com o historico janelado por
// finished_at >= historySinceMillis (0 = sem janela). Consumido pelo analytics,
// que so agrega: ler todo o historico da loja a cada request era o gargalo.
func (repository *PostgresRepository) LoadSnapshotWithHistorySince(ctx context.Context, storeID string, historySinceMillis int64) (SnapshotState, error) {
	waitingList, err := repository.loadWaitingList(ctx, storeID)
	if err != nil {
		return SnapshotState{}, err
	}

	activeServices, err := repository.loadActiveServices(ctx, storeID)
	if err != nil {
		return SnapshotState{}, err
	}

	pausedEmployees, err := repository.loadPausedEmployees(ctx, storeID)
	if err != nil {
		return SnapshotState{}, err
	}

	currentStatus, err := repository.loadCurrentStatus(ctx, storeID)
	if err != nil {
		return SnapshotState{}, err
	}

	sessions, err := repository.loadSessions(ctx, storeID)
	if err != nil {
		return SnapshotState{}, err
	}

	serviceHistory, err := repository.loadServiceHistory(ctx, storeID, historySinceMillis)
	if err != nil {
		return SnapshotState{}, err
	}

	return SnapshotState{
		StoreID:                    storeID,
		WaitingList:                waitingList,
		ActiveServices:             activeServices,
		PausedEmployees:            pausedEmployees,
		ConsultantCurrentStatus:    currentStatus,
		ConsultantActivitySessions: sessions,
		ServiceHistory:             serviceHistory,
	}, nil
}

// buildServiceHistoryQuery e' pura para ser testavel sem banco: monta o SQL e os
// args do historico, adicionando a janela `finished_at >= $2` so quando
// sinceMillis > 0.
func buildServiceHistoryQuery(storeID string, sinceMillis int64) (string, []any) {
	query := strings.Builder{}
	query.WriteString(`
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
			queue_position_at_start,
			queue_wait_ms,
			skipped_people_json,
			skipped_count,
			is_window_service,
			is_gift,
			product_seen,
			product_closed,
			purchase_code,
			product_details,
			products_seen_json,
			products_closed_json,
			coalesce(products_not_found_json, '[]'::jsonb) as products_not_found_json,
			products_seen_none,
			visit_reasons_not_informed,
			customer_sources_not_informed,
			customer_name,
			customer_phone,
			customer_email,
			is_existing_customer,
			visit_reasons_json,
			visit_reason_details_json,
			customer_sources_json,
			customer_source_details_json,
			loss_reasons_json,
			loss_reason_details_json,
			loss_reason_id,
			loss_reason,
			sale_amount,
			customer_profession,
			queue_jump_reason,
			notes,
			campaign_matches_json,
			campaign_bonus_total,
			coalesce(parallel_group_id, '') as parallel_group_id,
			parallel_start_index,
			coalesce(sibling_service_ids_json, '[]'::jsonb) as sibling_service_ids_json,
			coalesce(start_offset_ms, 0) as start_offset_ms
		from operation_service_history
		where store_id = $1::uuid
	`)

	args := []any{storeID}
	if sinceMillis > 0 {
		query.WriteString(" and finished_at >= $2")
		args = append(args, sinceMillis)
	}

	query.WriteString(" order by started_at asc, created_at asc;")

	return query.String(), args
}

func (repository *PostgresRepository) loadServiceHistory(ctx context.Context, storeID string, sinceMillis int64) ([]ServiceHistoryEntry, error) {
	query, args := buildServiceHistoryQuery(storeID, sinceMillis)
	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ServiceHistoryEntry, 0)
	for rows.Next() {
		var entry ServiceHistoryEntry
		var skippedRaw []byte
		var seenProductsRaw []byte
		var closedProductsRaw []byte
		var notFoundProductsRaw []byte
		var visitReasonsRaw []byte
		var visitReasonDetailsRaw []byte
		var customerSourcesRaw []byte
		var customerSourceDetailsRaw []byte
		var lossReasonsRaw []byte
		var lossReasonDetailsRaw []byte
		var campaignMatchesRaw []byte
		var siblingServiceIDsRaw []byte
		if err := rows.Scan(
			&entry.ServiceID,
			&entry.StoreID,
			&entry.PersonID,
			&entry.PersonName,
			&entry.StartedAt,
			&entry.FinishedAt,
			&entry.DurationMs,
			&entry.FinishOutcome,
			&entry.StartMode,
			&entry.QueuePositionAtStart,
			&entry.QueueWaitMs,
			&skippedRaw,
			&entry.SkippedCount,
			&entry.IsWindowService,
			&entry.IsGift,
			&entry.ProductSeen,
			&entry.ProductClosed,
			&entry.PurchaseCode,
			&entry.ProductDetails,
			&seenProductsRaw,
			&closedProductsRaw,
			&notFoundProductsRaw,
			&entry.ProductsSeenNone,
			&entry.VisitReasonsNotInformed,
			&entry.CustomerSourcesNotInformed,
			&entry.CustomerName,
			&entry.CustomerPhone,
			&entry.CustomerEmail,
			&entry.IsExistingCustomer,
			&visitReasonsRaw,
			&visitReasonDetailsRaw,
			&customerSourcesRaw,
			&customerSourceDetailsRaw,
			&lossReasonsRaw,
			&lossReasonDetailsRaw,
			&entry.LossReasonID,
			&entry.LossReason,
			&entry.SaleAmount,
			&entry.CustomerProfession,
			&entry.QueueJumpReason,
			&entry.Notes,
			&campaignMatchesRaw,
			&entry.CampaignBonusTotal,
			&entry.ParallelGroupID,
			&entry.ParallelStartIndex,
			&siblingServiceIDsRaw,
			&entry.StartOffsetMs,
		); err != nil {
			return nil, err
		}

		entry.SkippedPeople = decodeSkippedPeople(skippedRaw)
		entry.ProductsSeen = decodeProducts(seenProductsRaw)
		entry.ProductsClosed = decodeProducts(closedProductsRaw)
		entry.ProductsNotFound = decodeProducts(notFoundProductsRaw)
		entry.VisitReasons = stringsx.DecodeJSONStringSlice(visitReasonsRaw)
		entry.VisitReasonDetails = decodeStringMap(visitReasonDetailsRaw)
		entry.CustomerSources = stringsx.DecodeJSONStringSlice(customerSourcesRaw)
		entry.CustomerSourceDetails = decodeStringMap(customerSourceDetailsRaw)
		entry.LossReasons = stringsx.DecodeJSONStringSlice(lossReasonsRaw)
		entry.LossReasonDetails = decodeStringMap(lossReasonDetailsRaw)
		entry.CampaignMatches = decodeCampaignMatches(campaignMatchesRaw)
		entry.SiblingServiceIDs = stringsx.DecodeJSONStringSlice(siblingServiceIDsRaw)
		items = append(items, entry)
	}

	return items, rows.Err()
}
