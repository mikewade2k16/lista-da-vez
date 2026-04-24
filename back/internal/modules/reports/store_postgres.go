package reports

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) ListHistory(
	ctx context.Context,
	storeID string,
	filters repositoryFilters,
) ([]operations.ServiceHistoryEntry, error) {
	return repository.listHistoryQuery(ctx, []string{storeID}, filters)
}

func (repository *PostgresRepository) ListHistoryByStores(
	ctx context.Context,
	storeIDs []string,
	filters repositoryFilters,
) ([]operations.ServiceHistoryEntry, error) {
	return repository.listHistoryQuery(ctx, storeIDs, filters)
}

func (repository *PostgresRepository) listHistoryQuery(
	ctx context.Context,
	storeIDs []string,
	filters repositoryFilters,
) ([]operations.ServiceHistoryEntry, error) {
	if len(storeIDs) == 0 {
		return []operations.ServiceHistoryEntry{}, nil
	}

	query := strings.Builder{}
	query.WriteString(`
		select
			h.service_id,
			h.store_id::text,
			coalesce(s.name, '') as store_name,
			h.person_id::text,
			h.person_name,
			h.started_at,
			h.finished_at,
			h.duration_ms,
			h.finish_outcome,
			h.start_mode,
			h.queue_position_at_start,
			h.queue_wait_ms,
			h.skipped_people_json,
			h.skipped_count,
			h.is_window_service,
			h.is_gift,
			h.product_seen,
			h.product_closed,
			h.product_details,
			h.products_seen_json,
			h.products_closed_json,
			h.products_seen_none,
			h.visit_reasons_not_informed,
			h.customer_sources_not_informed,
			h.customer_name,
			h.customer_phone,
			h.customer_email,
			h.is_existing_customer,
			h.visit_reasons_json,
			h.visit_reason_details_json,
			h.customer_sources_json,
			h.customer_source_details_json,
			h.loss_reasons_json,
			h.loss_reason_details_json,
			h.loss_reason_id,
			h.loss_reason,
			h.sale_amount,
			h.customer_profession,
			h.queue_jump_reason,
			h.notes,
			h.campaign_matches_json,
			h.campaign_bonus_total
		from operation_service_history h
		join stores s
		  on s.id = h.store_id
		where h.store_id::text = any($1)
	`)

	args := []any{storeIDs}
	position := 2

	if filters.FinishedAtFrom != nil {
		query.WriteString(fmt.Sprintf(" and h.finished_at >= $%d", position))
		args = append(args, *filters.FinishedAtFrom)
		position++
	}

	if filters.FinishedAtTo != nil {
		query.WriteString(fmt.Sprintf(" and h.finished_at <= $%d", position))
		args = append(args, *filters.FinishedAtTo)
		position++
	}

	if len(filters.ConsultantIDs) > 0 {
		query.WriteString(fmt.Sprintf(" and h.person_id::text = any($%d)", position))
		args = append(args, filters.ConsultantIDs)
		position++
	}

	if len(filters.Outcomes) > 0 {
		query.WriteString(fmt.Sprintf(" and h.finish_outcome = any($%d)", position))
		args = append(args, filters.Outcomes)
		position++
	}

	if len(filters.StartModes) > 0 {
		query.WriteString(fmt.Sprintf(" and h.start_mode = any($%d)", position))
		args = append(args, filters.StartModes)
		position++
	}

	if filters.IsExistingCustomer != nil {
		query.WriteString(fmt.Sprintf(" and h.is_existing_customer = $%d", position))
		args = append(args, *filters.IsExistingCustomer)
		position++
	}

	if filters.MinSaleAmount != nil {
		query.WriteString(fmt.Sprintf(" and h.sale_amount >= $%d", position))
		args = append(args, *filters.MinSaleAmount)
		position++
	}

	if filters.MaxSaleAmount != nil {
		query.WriteString(fmt.Sprintf(" and h.sale_amount <= $%d", position))
		args = append(args, *filters.MaxSaleAmount)
		position++
	}

	query.WriteString(" order by h.finished_at desc, h.created_at desc;")

	rows, err := repository.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]operations.ServiceHistoryEntry, 0)
	for rows.Next() {
		var entry operations.ServiceHistoryEntry
		var skippedRaw []byte
		var seenProductsRaw []byte
		var closedProductsRaw []byte
		var visitReasonsRaw []byte
		var visitReasonDetailsRaw []byte
		var customerSourcesRaw []byte
		var customerSourceDetailsRaw []byte
		var lossReasonsRaw []byte
		var lossReasonDetailsRaw []byte
		var campaignMatchesRaw []byte

		if err := rows.Scan(
			&entry.ServiceID,
			&entry.StoreID,
			&entry.StoreName,
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
			&entry.ProductDetails,
			&seenProductsRaw,
			&closedProductsRaw,
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
		); err != nil {
			return nil, err
		}

		entry.SkippedPeople = decodeSkippedPeople(skippedRaw)
		entry.ProductsSeen = decodeProducts(seenProductsRaw)
		entry.ProductsClosed = decodeProducts(closedProductsRaw)
		entry.VisitReasons = decodeStringSlice(visitReasonsRaw)
		entry.VisitReasonDetails = decodeStringMap(visitReasonDetailsRaw)
		entry.CustomerSources = decodeStringSlice(customerSourcesRaw)
		entry.CustomerSourceDetails = decodeStringMap(customerSourceDetailsRaw)
		entry.LossReasons = decodeStringSlice(lossReasonsRaw)
		entry.LossReasonDetails = decodeStringMap(lossReasonDetailsRaw)
		entry.CampaignMatches = decodeCampaignMatches(campaignMatchesRaw)
		items = append(items, entry)
	}

	return items, rows.Err()
}

func (repository *PostgresRepository) ListLiveCounts(
	ctx context.Context,
	storeIDs []string,
) (map[string]StoreLiveCounts, error) {
	result := make(map[string]StoreLiveCounts, len(storeIDs))
	if len(storeIDs) == 0 {
		return result, nil
	}

	valuePlaceholders := make([]string, 0, len(storeIDs))
	args := make([]any, 0, len(storeIDs))
	for index, storeID := range storeIDs {
		valuePlaceholders = append(valuePlaceholders, fmt.Sprintf("($%d::uuid)", index+1))
		args = append(args, storeID)
		result[storeID] = StoreLiveCounts{StoreID: storeID}
	}

	query := fmt.Sprintf(`
		with scoped(store_id) as (
			values %s
		)
		select
			scoped.store_id::text,
			coalesce((select count(*) from consultants c where c.store_id = scoped.store_id and c.is_active = true), 0) as consultants,
			coalesce((select count(*) from operation_queue_entries q where q.store_id = scoped.store_id), 0) as queue_count,
			coalesce((select count(*) from operation_active_services a where a.store_id = scoped.store_id), 0) as active_count,
			coalesce((select count(*) from operation_paused_consultants p where p.store_id = scoped.store_id), 0) as paused_count
		from scoped;
	`, strings.Join(valuePlaceholders, ","))

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var counts StoreLiveCounts
		if err := rows.Scan(
			&counts.StoreID,
			&counts.Consultants,
			&counts.QueueCount,
			&counts.ActiveCount,
			&counts.PausedCount,
		); err != nil {
			return nil, err
		}

		result[counts.StoreID] = counts
	}

	return result, rows.Err()
}

func decodeSkippedPeople(raw []byte) []operations.SkippedPerson {
	if len(raw) == 0 {
		return []operations.SkippedPerson{}
	}

	var items []operations.SkippedPerson
	if err := json.Unmarshal(raw, &items); err != nil {
		return []operations.SkippedPerson{}
	}

	return items
}

func decodeProducts(raw []byte) []operations.ProductEntry {
	if len(raw) == 0 {
		return []operations.ProductEntry{}
	}

	var items []operations.ProductEntry
	if err := json.Unmarshal(raw, &items); err != nil {
		return []operations.ProductEntry{}
	}

	return items
}

func decodeStringSlice(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}

	var items []string
	if err := json.Unmarshal(raw, &items); err != nil {
		return []string{}
	}

	return items
}

func decodeStringMap(raw []byte) map[string]string {
	if len(raw) == 0 {
		return map[string]string{}
	}

	var items map[string]string
	if err := json.Unmarshal(raw, &items); err != nil {
		return map[string]string{}
	}

	if items == nil {
		return map[string]string{}
	}

	return items
}

func decodeCampaignMatches(raw []byte) []operations.CampaignMatch {
	if len(raw) == 0 {
		return []operations.CampaignMatch{}
	}

	var items []operations.CampaignMatch
	if err := json.Unmarshal(raw, &items); err != nil {
		return []operations.CampaignMatch{}
	}

	return items
}
