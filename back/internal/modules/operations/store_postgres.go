package operations

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) StoreExists(ctx context.Context, storeID string) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists(
			select 1
			from stores
			where id = $1::uuid
			  and is_active = true
		);
	`, storeID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (repository *PostgresRepository) GetStoreName(ctx context.Context, storeID string) (string, error) {
	var name string
	err := repository.pool.QueryRow(ctx, `
		select name
		from stores
		where id = $1::uuid
		limit 1;
	`, storeID).Scan(&name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrStoreNotFound
		}

		return "", err
	}

	return strings.TrimSpace(name), nil
}

func (repository *PostgresRepository) GetMaxConcurrentServices(ctx context.Context, storeID string) (int, error) {
	var value int
	err := repository.pool.QueryRow(ctx, `
		select coalesce((
			select max_concurrent_services
			from store_operation_settings
			where store_id = $1::uuid
			limit 1
		), 10);
	`, storeID).Scan(&value)
	if err != nil {
		return 0, err
	}

	if value < 1 {
		return 10, nil
	}

	return value, nil
}

func (repository *PostgresRepository) ListRoster(ctx context.Context, storeID string) ([]ConsultantProfile, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			id::text,
			store_id::text,
			name,
			role_label,
			initials,
			color,
			monthly_goal,
			commission_rate,
			conversion_goal,
			avg_ticket_goal,
			pa_goal
		from consultants
		where store_id = $1::uuid
		  and is_active = true
		order by name asc;
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roster := make([]ConsultantProfile, 0)
	for rows.Next() {
		var consultant ConsultantProfile
		if err := rows.Scan(
			&consultant.ID,
			&consultant.StoreID,
			&consultant.Name,
			&consultant.Role,
			&consultant.Initials,
			&consultant.Color,
			&consultant.MonthlyGoal,
			&consultant.CommissionRate,
			&consultant.ConversionGoal,
			&consultant.AvgTicketGoal,
			&consultant.PAGoal,
		); err != nil {
			return nil, err
		}

		consultant.Name = strings.TrimSpace(consultant.Name)
		consultant.Role = strings.TrimSpace(consultant.Role)
		consultant.Initials = strings.TrimSpace(consultant.Initials)
		consultant.Color = strings.TrimSpace(consultant.Color)
		roster = append(roster, consultant)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return roster, nil
}

func (repository *PostgresRepository) LoadSnapshot(ctx context.Context, storeID string) (SnapshotState, error) {
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

	serviceHistory, err := repository.loadServiceHistory(ctx, storeID)
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

func (repository *PostgresRepository) Persist(ctx context.Context, input PersistInput) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := replaceWaitingList(ctx, tx, input.StoreID, input.WaitingList); err != nil {
		return err
	}

	if err := replaceActiveServices(ctx, tx, input.StoreID, input.ActiveServices); err != nil {
		return err
	}

	if err := replacePausedEmployees(ctx, tx, input.StoreID, input.PausedEmployees); err != nil {
		return err
	}

	if err := replaceCurrentStatus(ctx, tx, input.StoreID, input.CurrentStatus); err != nil {
		return err
	}

	if err := appendSessions(ctx, tx, input.StoreID, input.AppendedSessions); err != nil {
		return err
	}

	if err := appendHistory(ctx, tx, input.StoreID, input.AppendedHistory); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (repository *PostgresRepository) loadWaitingList(ctx context.Context, storeID string) ([]QueueStateItem, error) {
	rows, err := repository.pool.Query(ctx, `
		select consultant_id::text, queue_joined_at
		from operation_queue_entries
		where store_id = $1::uuid
		order by sort_order asc, queue_joined_at asc;
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]QueueStateItem, 0)
	for rows.Next() {
		var item QueueStateItem
		if err := rows.Scan(&item.ConsultantID, &item.QueueJoinedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (repository *PostgresRepository) loadActiveServices(ctx context.Context, storeID string) ([]ActiveServiceState, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			consultant_id::text,
			service_id,
			service_started_at,
			queue_joined_at,
			queue_wait_ms,
			queue_position_at_start,
			start_mode,
			skipped_people_json
		from operation_active_services
		where store_id = $1::uuid
		order by service_started_at asc;
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ActiveServiceState, 0)
	for rows.Next() {
		var item ActiveServiceState
		var skippedPeopleRaw []byte
		if err := rows.Scan(
			&item.ConsultantID,
			&item.ServiceID,
			&item.ServiceStartedAt,
			&item.QueueJoinedAt,
			&item.QueueWaitMs,
			&item.QueuePositionAtStart,
			&item.StartMode,
			&skippedPeopleRaw,
		); err != nil {
			return nil, err
		}

		item.SkippedPeople = decodeSkippedPeople(skippedPeopleRaw)
		items = append(items, item)
	}

	return items, rows.Err()
}

func (repository *PostgresRepository) loadPausedEmployees(ctx context.Context, storeID string) ([]PausedStateItem, error) {
	rows, err := repository.pool.Query(ctx, `
		select consultant_id::text, reason, coalesce(kind, 'pause'), started_at
		from operation_paused_consultants
		where store_id = $1::uuid
		order by started_at asc;
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PausedStateItem, 0)
	for rows.Next() {
		var item PausedStateItem
		if err := rows.Scan(&item.ConsultantID, &item.Reason, &item.Kind, &item.StartedAt); err != nil {
			return nil, err
		}
		item.Reason = strings.TrimSpace(item.Reason)
		item.Kind = normalizePauseKind(item.Kind)
		items = append(items, item)
	}

	return items, rows.Err()
}

func (repository *PostgresRepository) loadCurrentStatus(ctx context.Context, storeID string) (map[string]ConsultantStatus, error) {
	rows, err := repository.pool.Query(ctx, `
		select consultant_id::text, status, started_at
		from operation_current_status
		where store_id = $1::uuid;
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := map[string]ConsultantStatus{}
	for rows.Next() {
		var consultantID string
		var status ConsultantStatus
		if err := rows.Scan(&consultantID, &status.Status, &status.StartedAt); err != nil {
			return nil, err
		}
		items[consultantID] = status
	}

	return items, rows.Err()
}

func (repository *PostgresRepository) loadSessions(ctx context.Context, storeID string) ([]ConsultantSession, error) {
	rows, err := repository.pool.Query(ctx, `
		select consultant_id::text, status, started_at, ended_at, duration_ms
		from operation_status_sessions
		where store_id = $1::uuid
		order by started_at asc, created_at asc;
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ConsultantSession, 0)
	for rows.Next() {
		var item ConsultantSession
		if err := rows.Scan(&item.PersonID, &item.Status, &item.StartedAt, &item.EndedAt, &item.DurationMs); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (repository *PostgresRepository) loadServiceHistory(ctx context.Context, storeID string) ([]ServiceHistoryEntry, error) {
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
			queue_position_at_start,
			queue_wait_ms,
			skipped_people_json,
			skipped_count,
			is_window_service,
			is_gift,
			product_seen,
			product_closed,
			product_details,
			products_seen_json,
			products_closed_json,
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
			campaign_bonus_total
		from operation_service_history
		where store_id = $1::uuid
		order by started_at asc, created_at asc;
	`, storeID)
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

func replaceWaitingList(ctx context.Context, tx pgx.Tx, storeID string, items []QueueStateItem) error {
	if _, err := tx.Exec(ctx, `delete from operation_queue_entries where store_id = $1::uuid;`, storeID); err != nil {
		return err
	}

	for index, item := range items {
		if _, err := tx.Exec(ctx, `
			insert into operation_queue_entries (store_id, consultant_id, queue_joined_at, sort_order)
			values ($1::uuid, $2::uuid, $3, $4);
		`, storeID, item.ConsultantID, item.QueueJoinedAt, index); err != nil {
			return err
		}
	}

	return nil
}

func replaceActiveServices(ctx context.Context, tx pgx.Tx, storeID string, items []ActiveServiceState) error {
	if _, err := tx.Exec(ctx, `delete from operation_active_services where store_id = $1::uuid;`, storeID); err != nil {
		return err
	}

	for _, item := range items {
		skippedRaw, err := json.Marshal(item.SkippedPeople)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			insert into operation_active_services (
				store_id,
				consultant_id,
				service_id,
				service_started_at,
				queue_joined_at,
				queue_wait_ms,
				queue_position_at_start,
				start_mode,
				skipped_people_json
			)
			values ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9::jsonb);
		`,
			storeID,
			item.ConsultantID,
			item.ServiceID,
			item.ServiceStartedAt,
			item.QueueJoinedAt,
			item.QueueWaitMs,
			item.QueuePositionAtStart,
			item.StartMode,
			string(skippedRaw),
		); err != nil {
			return err
		}
	}

	return nil
}

func replacePausedEmployees(ctx context.Context, tx pgx.Tx, storeID string, items []PausedStateItem) error {
	if _, err := tx.Exec(ctx, `delete from operation_paused_consultants where store_id = $1::uuid;`, storeID); err != nil {
		return err
	}

	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			insert into operation_paused_consultants (store_id, consultant_id, reason, kind, started_at)
			values ($1::uuid, $2::uuid, $3, $4, $5);
		`, storeID, item.ConsultantID, item.Reason, normalizePauseKind(item.Kind), item.StartedAt); err != nil {
			return err
		}
	}

	return nil
}

func replaceCurrentStatus(ctx context.Context, tx pgx.Tx, storeID string, items map[string]ConsultantStatus) error {
	if _, err := tx.Exec(ctx, `delete from operation_current_status where store_id = $1::uuid;`, storeID); err != nil {
		return err
	}

	for consultantID, item := range items {
		if _, err := tx.Exec(ctx, `
			insert into operation_current_status (store_id, consultant_id, status, started_at)
			values ($1::uuid, $2::uuid, $3, $4);
		`, storeID, consultantID, item.Status, item.StartedAt); err != nil {
			return err
		}
	}

	return nil
}

func appendSessions(ctx context.Context, tx pgx.Tx, storeID string, items []ConsultantSession) error {
	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			insert into operation_status_sessions (
				store_id,
				consultant_id,
				status,
				started_at,
				ended_at,
				duration_ms
			)
			values ($1::uuid, $2::uuid, $3, $4, $5, $6);
		`, storeID, item.PersonID, item.Status, item.StartedAt, item.EndedAt, item.DurationMs); err != nil {
			return err
		}
	}

	return nil
}

func appendHistory(ctx context.Context, tx pgx.Tx, storeID string, items []ServiceHistoryEntry) error {
	for _, item := range items {
		skippedRaw, err := json.Marshal(item.SkippedPeople)
		if err != nil {
			return err
		}
		productsSeenRaw, err := json.Marshal(item.ProductsSeen)
		if err != nil {
			return err
		}
		productsClosedRaw, err := json.Marshal(item.ProductsClosed)
		if err != nil {
			return err
		}
		visitReasonsRaw, err := json.Marshal(item.VisitReasons)
		if err != nil {
			return err
		}
		visitReasonDetailsRaw, err := json.Marshal(item.VisitReasonDetails)
		if err != nil {
			return err
		}
		customerSourcesRaw, err := json.Marshal(item.CustomerSources)
		if err != nil {
			return err
		}
		customerSourceDetailsRaw, err := json.Marshal(item.CustomerSourceDetails)
		if err != nil {
			return err
		}
		lossReasonsRaw, err := json.Marshal(item.LossReasons)
		if err != nil {
			return err
		}
		lossReasonDetailsRaw, err := json.Marshal(item.LossReasonDetails)
		if err != nil {
			return err
		}
		campaignMatchesRaw, err := json.Marshal(item.CampaignMatches)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			insert into operation_service_history (
				store_id,
				service_id,
				person_id,
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
				product_details,
				products_seen_json,
				products_closed_json,
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
				campaign_bonus_total
			)
			values (
				$1::uuid, $2, $3::uuid, $4, $5, $6, $7, $8, $9, $10,
				$11, $12::jsonb, $13, $14, $15, $16, $17, $18, $19::jsonb, $20::jsonb,
				$21, $22, $23, $24, $25, $26, $27, $28::jsonb, $29::jsonb, $30::jsonb,
				$31::jsonb, $32::jsonb, $33::jsonb, $34, $35, $36, $37, $38, $39, $40::jsonb, $41
			)
			on conflict (store_id, service_id) do nothing;
		`,
			storeID,
			item.ServiceID,
			item.PersonID,
			item.PersonName,
			item.StartedAt,
			item.FinishedAt,
			item.DurationMs,
			item.FinishOutcome,
			item.StartMode,
			item.QueuePositionAtStart,
			item.QueueWaitMs,
			string(skippedRaw),
			item.SkippedCount,
			item.IsWindowService,
			item.IsGift,
			item.ProductSeen,
			item.ProductClosed,
			item.ProductDetails,
			string(productsSeenRaw),
			string(productsClosedRaw),
			item.ProductsSeenNone,
			item.VisitReasonsNotInformed,
			item.CustomerSourcesNotInformed,
			item.CustomerName,
			item.CustomerPhone,
			item.CustomerEmail,
			item.IsExistingCustomer,
			string(visitReasonsRaw),
			string(visitReasonDetailsRaw),
			string(customerSourcesRaw),
			string(customerSourceDetailsRaw),
			string(lossReasonsRaw),
			string(lossReasonDetailsRaw),
			item.LossReasonID,
			item.LossReason,
			item.SaleAmount,
			item.CustomerProfession,
			item.QueueJumpReason,
			item.Notes,
			string(campaignMatchesRaw),
			item.CampaignBonusTotal,
		); err != nil {
			return err
		}
	}

	return nil
}

func decodeSkippedPeople(raw []byte) []SkippedPerson {
	if len(raw) == 0 {
		return []SkippedPerson{}
	}
	var items []SkippedPerson
	if err := json.Unmarshal(raw, &items); err != nil {
		return []SkippedPerson{}
	}
	return items
}

func decodeProducts(raw []byte) []ProductEntry {
	if len(raw) == 0 {
		return []ProductEntry{}
	}
	var items []ProductEntry
	if err := json.Unmarshal(raw, &items); err != nil {
		return []ProductEntry{}
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

func decodeCampaignMatches(raw []byte) []CampaignMatch {
	if len(raw) == 0 {
		return []CampaignMatch{}
	}
	var items []CampaignMatch
	if err := json.Unmarshal(raw, &items); err != nil {
		return []CampaignMatch{}
	}
	return items
}
