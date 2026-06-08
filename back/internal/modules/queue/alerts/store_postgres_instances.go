package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (repository *PostgresRepository) List(ctx context.Context, input ListInput) ([]Alert, error) {
	query := strings.Builder{}
	query.WriteString(`
		select id::text, tenant_id::text, store_id::text, service_id,
		       consultant_id::text, type, category, severity, status,
		       source_module, dedupe_key, headline, body, metadata,
		       opened_at, last_triggered_at, acknowledged_at, resolved_at,
		       interaction_kind, interaction_response, responded_at, external_notified_at,
		       rule_definition_id::text, display_kind, color_theme, response_options,
		       is_mandatory, consultant_name, created_at, updated_at
		from alert_instances
		where tenant_id = $1::uuid
	`)

	args := []any{strings.TrimSpace(input.TenantID)}
	argIndex := 2
	appendStoreFilter(&query, &args, &argIndex, input.StoreID, input.StoreIDs)

	if normalizedStatus := strings.TrimSpace(input.Status); normalizedStatus != "" {
		query.WriteString(fmt.Sprintf(" and status = $%d", argIndex))
		args = append(args, normalizedStatus)
		argIndex++
	}
	if normalizedType := strings.TrimSpace(input.Type); normalizedType != "" {
		query.WriteString(fmt.Sprintf(" and type = $%d", argIndex))
		args = append(args, normalizedType)
		argIndex++
	}
	if normalizedCategory := strings.TrimSpace(input.Category); normalizedCategory != "" {
		query.WriteString(fmt.Sprintf(" and category = $%d", argIndex))
		args = append(args, normalizedCategory)
		argIndex++
	}

	query.WriteString(" order by last_triggered_at desc, created_at desc;")

	rows, err := repository.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	alerts := make([]Alert, 0)
	for rows.Next() {
		alert, err := scanAlert(rows.Scan)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, *alert)
	}

	return alerts, rows.Err()
}

func (repository *PostgresRepository) Overview(ctx context.Context, input OverviewInput) (Overview, error) {
	query := strings.Builder{}
	query.WriteString(`
		select
			count(*) filter (where status = 'active') as total_active,
			count(*) filter (where status = 'active' and severity = 'critical') as critical_active,
			count(*) filter (where status = 'acknowledged') as acknowledged,
			count(*) filter (where status = 'resolved' and resolved_at >= date_trunc('day', now())) as resolved_today
		from alert_instances
		where tenant_id = $1::uuid
	`)

	args := []any{strings.TrimSpace(input.TenantID)}
	argIndex := 2
	appendStoreFilter(&query, &args, &argIndex, input.StoreID, input.StoreIDs)

	overview := Overview{
		TenantID: strings.TrimSpace(input.TenantID),
		StoreID:  strings.TrimSpace(input.StoreID),
	}

	err := repository.pool.QueryRow(ctx, query.String(), args...).Scan(
		&overview.TotalActive,
		&overview.CriticalActive,
		&overview.Acknowledged,
		&overview.ResolvedToday,
	)
	if err != nil {
		return Overview{}, err
	}

	return overview, nil
}

func (repository *PostgresRepository) GetByID(ctx context.Context, alertID string) (*Alert, error) {
	row := repository.pool.QueryRow(ctx, `
		select id::text, tenant_id::text, store_id::text, service_id,
		       consultant_id::text, type, category, severity, status,
		       source_module, dedupe_key, headline, body, metadata,
		       opened_at, last_triggered_at, acknowledged_at, resolved_at,
		       interaction_kind, interaction_response, responded_at, external_notified_at,
		       rule_definition_id::text, display_kind, color_theme, response_options,
		       is_mandatory, consultant_name, created_at, updated_at
		from alert_instances
		where id = $1::uuid
		limit 1;
	`, strings.TrimSpace(alertID))

	alert, err := scanAlert(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return alert, nil
}

func (repository *PostgresRepository) Acknowledge(ctx context.Context, alertID string, actor Actor, note string) (*Alert, error) {
	return repository.transitionAlert(ctx, strings.TrimSpace(alertID), actor, strings.TrimSpace(note), func(alert *Alert, now time.Time) (bool, error) {
		switch alert.Status {
		case StatusResolved, StatusClosedByAdmin, StatusAcknowledged:
			return false, nil
		default:
			alert.Status = StatusAcknowledged
			alert.AcknowledgedAt = &now
			alert.UpdatedAt = now
			return true, nil
		}
	}, ActionAcknowledged)
}

func (repository *PostgresRepository) Resolve(ctx context.Context, alertID string, actor Actor, note string) (*Alert, error) {
	return repository.transitionAlert(ctx, strings.TrimSpace(alertID), actor, strings.TrimSpace(note), func(alert *Alert, now time.Time) (bool, error) {
		switch alert.Status {
		case StatusResolved, StatusClosedByAdmin:
			return false, nil
		default:
			alert.Status = StatusResolved
			alert.ResolvedAt = &now
			alert.UpdatedAt = now
			return true, nil
		}
	}, ActionResolved)
}

func (repository *PostgresRepository) RespondToAlert(ctx context.Context, input AlertRespondInput, actor Actor) (*Alert, error) {
	response := strings.TrimSpace(input.Response)

	return repository.transitionAlert(ctx, strings.TrimSpace(input.AlertID), actor, "", func(alert *Alert, now time.Time) (bool, error) {
		if response == "" || !alertAllowsResponse(*alert, response) {
			return false, ErrValidation
		}

		switch alert.Status {
		case StatusResolved, StatusClosedByAdmin:
			return false, nil
		default:
			alert.Status = StatusAcknowledged
			alert.AcknowledgedAt = &now
			alert.InteractionResponse = response
			alert.RespondedAt = &now
			alert.UpdatedAt = now
			return true, nil
		}
	}, "responded")
}

func (repository *PostgresRepository) MarkExternalNotified(ctx context.Context, alertID string) error {
	_, err := repository.pool.Exec(ctx, `
		update alert_instances
		set external_notified_at = now(), updated_at = now()
		where id = $1::uuid
		  and external_notified_at is null;
	`, strings.TrimSpace(alertID))
	return err
}

func (repository *PostgresRepository) transitionAlert(
	ctx context.Context,
	alertID string,
	actor Actor,
	note string,
	apply func(alert *Alert, now time.Time) (bool, error),
	action string,
) (*Alert, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	alert, err := repository.findAlertByIDTx(ctx, tx, alertID, true)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	changed, err := apply(alert, now)
	if err != nil {
		return nil, err
	}

	if changed {
		_, err = tx.Exec(ctx, `
			update alert_instances
			set status = $2,
			    acknowledged_at = $3,
			    resolved_at = $4,
			    interaction_response = nullif($6, ''),
			    responded_at = $7,
			    updated_at = $5
			where id = $1::uuid;
		`, alert.ID, alert.Status, alert.AcknowledgedAt, alert.ResolvedAt, alert.UpdatedAt,
			alert.InteractionResponse, alert.RespondedAt)
		if err != nil {
			return nil, err
		}

		if err := repository.appendAlertActionTx(ctx, tx, *alert, action, actor, note, map[string]any{
			"status": alert.Status,
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return alert, nil
}

func (repository *PostgresRepository) findAlertByIDTx(ctx context.Context, tx pgx.Tx, alertID string, forUpdate bool) (*Alert, error) {
	query := `
		select id::text, tenant_id::text, store_id::text, service_id,
		       consultant_id::text, type, category, severity, status,
		       source_module, dedupe_key, headline, body, metadata,
		       opened_at, last_triggered_at, acknowledged_at, resolved_at,
		       interaction_kind, interaction_response, responded_at, external_notified_at,
		       rule_definition_id::text, display_kind, color_theme, response_options,
		       is_mandatory, consultant_name, created_at, updated_at
		from alert_instances
		where id = $1::uuid
		limit 1`
	if forUpdate {
		query += ` for update`
	}

	alert, err := scanAlert(tx.QueryRow(ctx, query, alertID).Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return alert, nil
}

func (repository *PostgresRepository) findOpenAlertByDedupeKeyTx(ctx context.Context, tx pgx.Tx, dedupeKey string) (*Alert, error) {
	alert, err := scanAlert(tx.QueryRow(ctx, `
		select id::text, tenant_id::text, store_id::text, service_id,
		       consultant_id::text, type, category, severity, status,
		       source_module, dedupe_key, headline, body, metadata,
		       opened_at, last_triggered_at, acknowledged_at, resolved_at,
		       interaction_kind, interaction_response, responded_at, external_notified_at,
		       rule_definition_id::text, display_kind, color_theme, response_options,
		       is_mandatory, consultant_name, created_at, updated_at
		from alert_instances
		where dedupe_key = $1
		  and status in ('active', 'acknowledged')
		order by created_at desc
		limit 1
		for update;
	`, dedupeKey).Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return alert, nil
}

func (repository *PostgresRepository) appendAlertActionTx(ctx context.Context, tx pgx.Tx, alert Alert, action string, actor Actor, note string, metadata map[string]any) error {
	_, err := tx.Exec(ctx, `
		insert into alert_actions (
			alert_id,
			tenant_id,
			store_id,
			action,
			actor_user_id,
			actor_name,
			note,
			metadata,
			created_at
		) values (
			$1::uuid,
			$2::uuid,
			$3::uuid,
			$4,
			nullif($5, '')::uuid,
			$6,
			$7,
			$8::jsonb,
			now()
		);
	`,
		alert.ID,
		alert.TenantID,
		alert.StoreID,
		strings.TrimSpace(action),
		strings.TrimSpace(actor.UserID),
		strings.TrimSpace(actor.DisplayName),
		strings.TrimSpace(note),
		marshalJSONB(metadata),
	)
	return err
}

func scanAlert(scan func(...any) error) (*Alert, error) {
	var alert Alert
	var consultantID *string
	var ruleDefinitionID *string
	var metadataRaw []byte
	var interactionResponse *string
	var responseOptionsRaw []byte
	err := scan(
		&alert.ID,
		&alert.TenantID,
		&alert.StoreID,
		&alert.ServiceID,
		&consultantID,
		&alert.Type,
		&alert.Category,
		&alert.Severity,
		&alert.Status,
		&alert.SourceModule,
		&alert.DedupeKey,
		&alert.Headline,
		&alert.Body,
		&metadataRaw,
		&alert.OpenedAt,
		&alert.LastTriggeredAt,
		&alert.AcknowledgedAt,
		&alert.ResolvedAt,
		&alert.InteractionKind,
		&interactionResponse,
		&alert.RespondedAt,
		&alert.ExternalNotifiedAt,
		&ruleDefinitionID,
		&alert.DisplayKind,
		&alert.ColorTheme,
		&responseOptionsRaw,
		&alert.IsMandatory,
		&alert.ConsultantName,
		&alert.CreatedAt,
		&alert.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if consultantID != nil {
		alert.ConsultantID = strings.TrimSpace(*consultantID)
	}
	if interactionResponse != nil {
		alert.InteractionResponse = strings.TrimSpace(*interactionResponse)
	}
	if ruleDefinitionID != nil {
		alert.RuleDefinitionID = strings.TrimSpace(*ruleDefinitionID)
	}

	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &alert.Metadata); err != nil || alert.Metadata == nil {
			alert.Metadata = map[string]any{}
		}
	} else {
		alert.Metadata = map[string]any{}
	}

	if len(responseOptionsRaw) > 0 {
		if err := json.Unmarshal(responseOptionsRaw, &alert.ResponseOptions); err != nil || alert.ResponseOptions == nil {
			alert.ResponseOptions = []ResponseOption{}
		}
	} else {
		alert.ResponseOptions = []ResponseOption{}
	}

	return &alert, nil
}

func alertAllowsResponse(alert Alert, response string) bool {
	normalizedResponse := strings.TrimSpace(response)
	if normalizedResponse == "" {
		return false
	}
	if normalizedResponse == InteractionResponseStillHappening || normalizedResponse == InteractionResponseForgotten {
		return true
	}

	for _, option := range alert.ResponseOptions {
		if strings.TrimSpace(option.Value) == normalizedResponse {
			return true
		}
	}

	return false
}
