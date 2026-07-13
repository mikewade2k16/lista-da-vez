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

func (repository *PostgresRepository) LoadRules(ctx context.Context, tenantID string) (RulesView, error) {
	row := repository.pool.QueryRow(ctx, `
		select tenant_id::text,
		       long_open_service_minutes,
		       idle_store_minutes,
		       after_closing_grace_minutes,
		       notify_dashboard,
		       notify_operation_context,
		       notify_external,
		       coalesce(auto_close_enabled, false),
		       coalesce(auto_close_minutes, $2),
		       coalesce(auto_close_grace_seconds, $3),
		       coalesce(snooze_reprompt_minutes, $4),
		       updated_at
		from tenant_operational_alert_rules
		where tenant_id = $1::uuid
		limit 1;
	`, strings.TrimSpace(tenantID), defaultAutoCloseMinutes, defaultAutoCloseGraceSeconds, defaultSnoozeRepromptMinutes)

	var rules RulesView
	var updatedAt time.Time
	err := row.Scan(
		&rules.TenantID,
		&rules.LongOpenServiceMinutes,
		&rules.IdleStoreMinutes,
		&rules.AfterClosingGraceMinutes,
		&rules.NotifyDashboard,
		&rules.NotifyOperationContext,
		&rules.NotifyExternal,
		&rules.AutoCloseEnabled,
		&rules.AutoCloseMinutes,
		&rules.AutoCloseGraceSeconds,
		&rules.SnoozeRepromptMinutes,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return defaultRules(strings.TrimSpace(tenantID)), nil
		}
		return RulesView{}, err
	}

	rules.Source = RulesSourceDatabase
	rules.UpdatedAt = &updatedAt
	return rules, nil
}

func (repository *PostgresRepository) UpsertRules(ctx context.Context, tenantID string, updatedByUserID string, input UpdateRulesInput) (RulesView, error) {
	current, err := repository.LoadRules(ctx, strings.TrimSpace(tenantID))
	if err != nil {
		return RulesView{}, err
	}

	if input.LongOpenServiceMinutes != nil {
		current.LongOpenServiceMinutes = *input.LongOpenServiceMinutes
	}
	if input.IdleStoreMinutes != nil {
		current.IdleStoreMinutes = *input.IdleStoreMinutes
	}
	if input.AfterClosingGraceMinutes != nil {
		current.AfterClosingGraceMinutes = *input.AfterClosingGraceMinutes
	}
	if input.NotifyDashboard != nil {
		current.NotifyDashboard = *input.NotifyDashboard
	}
	if input.NotifyOperationContext != nil {
		current.NotifyOperationContext = *input.NotifyOperationContext
	}
	if input.NotifyExternal != nil {
		current.NotifyExternal = *input.NotifyExternal
	}
	if input.AutoCloseEnabled != nil {
		current.AutoCloseEnabled = *input.AutoCloseEnabled
	}
	if input.AutoCloseMinutes != nil {
		current.AutoCloseMinutes = *input.AutoCloseMinutes
	}
	if input.AutoCloseGraceSeconds != nil {
		current.AutoCloseGraceSeconds = *input.AutoCloseGraceSeconds
	}
	if input.SnoozeRepromptMinutes != nil {
		current.SnoozeRepromptMinutes = *input.SnoozeRepromptMinutes
	}

	var updatedAt time.Time
	err = repository.pool.QueryRow(ctx, `
		insert into tenant_operational_alert_rules (
			tenant_id,
			long_open_service_minutes,
			idle_store_minutes,
			after_closing_grace_minutes,
			notify_dashboard,
			notify_operation_context,
			notify_external,
			auto_close_enabled,
			auto_close_minutes,
			auto_close_grace_seconds,
			snooze_reprompt_minutes,
			updated_by,
			updated_at
		) values (
			$1::uuid,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11,
			nullif($12, '')::uuid,
			now()
		)
		on conflict (tenant_id) do update
		set
			long_open_service_minutes = excluded.long_open_service_minutes,
			idle_store_minutes = excluded.idle_store_minutes,
			after_closing_grace_minutes = excluded.after_closing_grace_minutes,
			notify_dashboard = excluded.notify_dashboard,
			notify_operation_context = excluded.notify_operation_context,
			notify_external = excluded.notify_external,
			auto_close_enabled = excluded.auto_close_enabled,
			auto_close_minutes = excluded.auto_close_minutes,
			auto_close_grace_seconds = excluded.auto_close_grace_seconds,
			snooze_reprompt_minutes = excluded.snooze_reprompt_minutes,
			updated_by = excluded.updated_by,
			updated_at = now()
		returning updated_at;
	`,
		strings.TrimSpace(tenantID),
		current.LongOpenServiceMinutes,
		current.IdleStoreMinutes,
		current.AfterClosingGraceMinutes,
		current.NotifyDashboard,
		current.NotifyOperationContext,
		current.NotifyExternal,
		current.AutoCloseEnabled,
		current.AutoCloseMinutes,
		current.AutoCloseGraceSeconds,
		current.SnoozeRepromptMinutes,
		strings.TrimSpace(updatedByUserID),
	).Scan(&updatedAt)
	if err != nil {
		return RulesView{}, err
	}

	current.Source = RulesSourceDatabase
	current.UpdatedAt = &updatedAt
	return current, nil
}

func (repository *PostgresRepository) ListRules(ctx context.Context, input ListRulesInput) ([]RuleDefinition, error) {
	query := strings.Builder{}
	query.WriteString(`
		select id::text, tenant_id::text, name, description, is_active,
		       trigger_type, threshold_minutes, severity,
		       display_kind, color_theme, title_template, body_template,
		       interaction_kind, response_options,
		       is_mandatory, notify_dashboard, notify_operation_context, notify_external,
		       external_channel, created_at, updated_at
		from (
			select *,
			       row_number() over (
			           partition by tenant_id, trigger_type, name
			           order by is_active desc, updated_at desc, created_at desc, id
			       ) as rule_rank
			from alert_rule_definitions
			where tenant_id = $1::uuid
		) ranked_rules
		where rule_rank = 1
	`)

	args := []any{strings.TrimSpace(input.TenantID)}
	argIndex := 2

	if normalizedTrigger := strings.TrimSpace(input.TriggerType); normalizedTrigger != "" {
		query.WriteString(fmt.Sprintf(" and trigger_type = $%d", argIndex))
		args = append(args, normalizedTrigger)
		argIndex++
	}

	if input.OnlyActive {
		query.WriteString(" and is_active = true")
	}

	query.WriteString(" order by updated_at desc;")

	rows, err := repository.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]RuleDefinition, 0)
	for rows.Next() {
		rule, err := scanRuleDefinition(rows.Scan)
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rule)
	}

	return rules, rows.Err()
}

func (repository *PostgresRepository) GetRule(ctx context.Context, ruleID string) (*RuleDefinition, error) {
	rule, err := scanRuleDefinition(repository.pool.QueryRow(ctx, `
		select id::text, tenant_id::text, name, description, is_active,
		       trigger_type, threshold_minutes, severity,
		       display_kind, color_theme, title_template, body_template,
		       interaction_kind, response_options,
		       is_mandatory, notify_dashboard, notify_operation_context, notify_external,
		       external_channel, created_at, updated_at
		from alert_rule_definitions
		where id = $1::uuid
		limit 1;
	`, strings.TrimSpace(ruleID)).Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return rule, nil
}

func (repository *PostgresRepository) CreateRule(ctx context.Context, input CreateRuleInput, actor Actor) (*RuleDefinition, error) {
	var ruleID string
	responseOptionsJSON, _ := json.Marshal(input.ResponseOptions)

	err := repository.pool.QueryRow(ctx, `
		insert into alert_rule_definitions (
			tenant_id, name, description, is_active, trigger_type, threshold_minutes, severity,
			display_kind, color_theme, title_template, body_template,
			interaction_kind, response_options, is_mandatory,
			notify_dashboard, notify_operation_context, notify_external,
			external_channel, created_by, updated_by, created_at, updated_at
		) values (
			$1::uuid, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11,
			$12, $13::jsonb, $14,
			$15, $16, $17,
			$18, nullif($19, '')::uuid, nullif($20, '')::uuid, now(), now()
		) returning id::text;
	`,
		strings.TrimSpace(input.TenantID),
		strings.TrimSpace(input.Name),
		strings.TrimSpace(input.Description),
		input.IsActive,
		strings.TrimSpace(input.TriggerType),
		input.ThresholdMinutes,
		strings.TrimSpace(input.Severity),
		strings.TrimSpace(input.DisplayKind),
		strings.TrimSpace(input.ColorTheme),
		strings.TrimSpace(input.TitleTemplate),
		strings.TrimSpace(input.BodyTemplate),
		strings.TrimSpace(input.InteractionKind),
		responseOptionsJSON,
		input.IsMandatory,
		input.NotifyDashboard,
		input.NotifyOperationContext,
		input.NotifyExternal,
		strings.TrimSpace(input.ExternalChannel),
		actor.UserID,
		actor.UserID,
	).Scan(&ruleID)

	if err != nil {
		return nil, err
	}

	return repository.GetRule(ctx, ruleID)
}

func (repository *PostgresRepository) UpdateRule(ctx context.Context, ruleID string, input UpdateRuleInput, actor Actor) (*RuleDefinition, error) {
	query := strings.Builder{}
	args := []any{strings.TrimSpace(ruleID)}
	argIndex := 2

	updates := make([]string, 0)

	if input.Name != nil {
		updates = append(updates, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, strings.TrimSpace(*input.Name))
		argIndex++
	}

	if input.Description != nil {
		updates = append(updates, fmt.Sprintf("description = $%d", argIndex))
		args = append(args, strings.TrimSpace(*input.Description))
		argIndex++
	}

	if input.IsActive != nil {
		updates = append(updates, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *input.IsActive)
		argIndex++
	}

	if input.TriggerType != nil {
		updates = append(updates, fmt.Sprintf("trigger_type = $%d", argIndex))
		args = append(args, strings.TrimSpace(*input.TriggerType))
		argIndex++
	}

	if input.ThresholdMinutes != nil {
		updates = append(updates, fmt.Sprintf("threshold_minutes = $%d", argIndex))
		args = append(args, *input.ThresholdMinutes)
		argIndex++
	}

	if input.Severity != nil {
		updates = append(updates, fmt.Sprintf("severity = $%d", argIndex))
		args = append(args, strings.TrimSpace(*input.Severity))
		argIndex++
	}

	if input.DisplayKind != nil {
		updates = append(updates, fmt.Sprintf("display_kind = $%d", argIndex))
		args = append(args, strings.TrimSpace(*input.DisplayKind))
		argIndex++
	}

	if input.ColorTheme != nil {
		updates = append(updates, fmt.Sprintf("color_theme = $%d", argIndex))
		args = append(args, strings.TrimSpace(*input.ColorTheme))
		argIndex++
	}

	if input.TitleTemplate != nil {
		updates = append(updates, fmt.Sprintf("title_template = $%d", argIndex))
		args = append(args, strings.TrimSpace(*input.TitleTemplate))
		argIndex++
	}

	if input.BodyTemplate != nil {
		updates = append(updates, fmt.Sprintf("body_template = $%d", argIndex))
		args = append(args, strings.TrimSpace(*input.BodyTemplate))
		argIndex++
	}

	if input.InteractionKind != nil {
		interactionKind := strings.TrimSpace(*input.InteractionKind)
		updates = append(updates, fmt.Sprintf("interaction_kind = $%d", argIndex))
		args = append(args, interactionKind)
		argIndex++

		if interactionKind == InteractionKindNone || interactionKind == InteractionKindDismiss {
			updates = append(updates, "response_options = '[]'::jsonb")
		}
	}

	if len(input.ResponseOptions) > 0 {
		updates = append(updates, fmt.Sprintf("response_options = $%d::jsonb", argIndex))
		responseOptionsJSON, _ := json.Marshal(input.ResponseOptions)
		args = append(args, responseOptionsJSON)
		argIndex++
	}

	if input.IsMandatory != nil {
		updates = append(updates, fmt.Sprintf("is_mandatory = $%d", argIndex))
		args = append(args, *input.IsMandatory)
		argIndex++
	}

	if input.NotifyDashboard != nil {
		updates = append(updates, fmt.Sprintf("notify_dashboard = $%d", argIndex))
		args = append(args, *input.NotifyDashboard)
		argIndex++
	}

	if input.NotifyOperationContext != nil {
		updates = append(updates, fmt.Sprintf("notify_operation_context = $%d", argIndex))
		args = append(args, *input.NotifyOperationContext)
		argIndex++
	}

	if input.NotifyExternal != nil {
		updates = append(updates, fmt.Sprintf("notify_external = $%d", argIndex))
		args = append(args, *input.NotifyExternal)
		argIndex++
	}

	if input.ExternalChannel != nil {
		updates = append(updates, fmt.Sprintf("external_channel = $%d", argIndex))
		args = append(args, strings.TrimSpace(*input.ExternalChannel))
		argIndex++
	}

	if len(updates) == 0 {
		return repository.GetRule(ctx, ruleID)
	}

	updates = append(updates, fmt.Sprintf("updated_by = nullif($%d, '')::uuid", argIndex))
	args = append(args, actor.UserID)
	argIndex++

	updates = append(updates, "updated_at = now()")

	query.WriteString("update alert_rule_definitions set ")
	query.WriteString(strings.Join(updates, ", "))
	query.WriteString(" where id = $1::uuid;")

	_, err := repository.pool.Exec(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}

	return repository.GetRule(ctx, ruleID)
}

func (repository *PostgresRepository) DeleteRule(ctx context.Context, ruleID string) error {
	_, err := repository.pool.Exec(ctx, `
		delete from alert_rule_definitions
		where id = $1::uuid;
	`, strings.TrimSpace(ruleID))
	return err
}

func (repository *PostgresRepository) LoadActiveRulesForTrigger(ctx context.Context, tenantID string, triggerType string) ([]RuleDefinition, error) {
	return repository.ListRules(ctx, ListRulesInput{
		TenantID:    strings.TrimSpace(tenantID),
		TriggerType: strings.TrimSpace(triggerType),
		OnlyActive:  true,
	})
}

func scanRuleDefinition(scan func(...any) error) (*RuleDefinition, error) {
	var rule RuleDefinition
	var responseOptionsRaw []byte

	err := scan(
		&rule.ID,
		&rule.TenantID,
		&rule.Name,
		&rule.Description,
		&rule.IsActive,
		&rule.TriggerType,
		&rule.ThresholdMinutes,
		&rule.Severity,
		&rule.DisplayKind,
		&rule.ColorTheme,
		&rule.TitleTemplate,
		&rule.BodyTemplate,
		&rule.InteractionKind,
		&responseOptionsRaw,
		&rule.IsMandatory,
		&rule.NotifyDashboard,
		&rule.NotifyOperationContext,
		&rule.NotifyExternal,
		&rule.ExternalChannel,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(responseOptionsRaw) > 0 {
		if err := json.Unmarshal(responseOptionsRaw, &rule.ResponseOptions); err != nil || rule.ResponseOptions == nil {
			rule.ResponseOptions = []ResponseOption{}
		}
	} else {
		rule.ResponseOptions = []ResponseOption{}
	}

	return &rule, nil
}

func defaultRules(tenantID string) RulesView {
	return RulesView{
		TenantID:                 strings.TrimSpace(tenantID),
		LongOpenServiceMinutes:   defaultLongOpenMinutes,
		IdleStoreMinutes:         defaultIdleStoreMinutes,
		AfterClosingGraceMinutes: defaultAfterClosingGraceMinutes,
		NotifyDashboard:          true,
		NotifyOperationContext:   true,
		NotifyExternal:           false,
		AutoCloseEnabled:         false,
		AutoCloseMinutes:         defaultAutoCloseMinutes,
		AutoCloseGraceSeconds:    defaultAutoCloseGraceSeconds,
		SnoozeRepromptMinutes:    defaultSnoozeRepromptMinutes,
		Source:                   RulesSourceDefaults,
	}
}
