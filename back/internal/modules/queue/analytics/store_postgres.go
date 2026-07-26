package analytics

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/operations"
)

type PostgresRepository struct {
	pool       *pgxpool.Pool
	operations *operations.PostgresRepository
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool:       pool,
		operations: operations.NewPostgresRepository(pool),
	}
}

func (repository *PostgresRepository) LoadSnapshotWithHistorySince(ctx context.Context, storeID string, historySinceMillis int64) (operations.SnapshotState, error) {
	return repository.operations.LoadSnapshotWithHistorySince(ctx, storeID, historySinceMillis)
}

func (repository *PostgresRepository) ListRoster(ctx context.Context, storeID string) ([]operations.ConsultantProfile, error) {
	return repository.operations.ListRoster(ctx, storeID)
}

func (repository *PostgresRepository) LoadSettings(ctx context.Context, storeID string) (StoreSettings, error) {
	settings := StoreSettings{
		VisitReasonLabels:    map[string]string{},
		CustomerSourceLabels: map[string]string{},
	}

	err := repository.pool.QueryRow(ctx, `
		select
			coalesce(timing_fast_close_minutes, 5),
			coalesce(timing_long_service_minutes, 25),
			coalesce(timing_low_sale_amount, 1200),
			coalesce(alert_min_conversion_rate, 0),
			coalesce(alert_max_queue_jump_rate, 0),
			coalesce(alert_min_pa_score, 0),
			coalesce(alert_min_ticket_average, 0)
		from store_operation_settings
		where store_id = $1::uuid
		limit 1;
	`, storeID).Scan(
		&settings.TimingFastCloseMinutes,
		&settings.TimingLongServiceMinutes,
		&settings.TimingLowSaleAmount,
		&settings.AlertMinConversionRate,
		&settings.AlertMaxQueueJumpRate,
		&settings.AlertMinPAScore,
		&settings.AlertMinTicketAverage,
	)
	// Loja sem linha em store_operation_settings nao e' erro: segue com os
	// defaults (reaplicados no build via maxInt/maxFloat). Evita que uma unica
	// loja sem settings derrube o analytics do tenant inteiro com 500.
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return StoreSettings{}, err
	}

	rows, err := repository.pool.Query(ctx, `
		select kind, option_id, label
		from store_setting_options
		where store_id = $1::uuid
		  and kind in ('visit_reason', 'customer_source')
		order by sort_order asc, label asc;
	`, storeID)
	if err != nil {
		return StoreSettings{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var kind string
		var optionID string
		var label string
		if err := rows.Scan(&kind, &optionID, &label); err != nil {
			return StoreSettings{}, err
		}

		switch strings.TrimSpace(kind) {
		case "visit_reason":
			settings.VisitReasonLabels[strings.TrimSpace(optionID)] = strings.TrimSpace(label)
		case "customer_source":
			settings.CustomerSourceLabels[strings.TrimSpace(optionID)] = strings.TrimSpace(label)
		}
	}

	if err := rows.Err(); err != nil {
		return StoreSettings{}, err
	}

	return settings, nil
}

func (repository *PostgresRepository) ResolveUserDisplayNames(ctx context.Context, userIDs []string) (map[string]string, error) {
	names := make(map[string]string, len(userIDs))
	if len(userIDs) == 0 {
		return names, nil
	}

	rows, err := repository.pool.Query(ctx, `
		select
			id::text,
			coalesce(nullif(trim(display_name), ''), nullif(trim(nick), ''), email)
		from core.users
		where id = any($1::uuid[]);
	`, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var userID string
		var displayName string
		if err := rows.Scan(&userID, &displayName); err != nil {
			return nil, err
		}
		names[strings.TrimSpace(userID)] = strings.TrimSpace(displayName)
	}

	return names, rows.Err()
}
