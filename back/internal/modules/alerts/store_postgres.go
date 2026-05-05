package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

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

func (repository *PostgresRepository) LoadRules(ctx context.Context, tenantID string) (RulesView, error) {
	row := repository.pool.QueryRow(ctx, `
		select tenant_id::text,
		       long_open_service_minutes,
		       idle_store_minutes,
		       after_closing_grace_minutes,
		       notify_dashboard,
		       notify_operation_context,
		       notify_external,
		       updated_at
		from tenant_operational_alert_rules
		where tenant_id = $1::uuid
		limit 1;
	`, strings.TrimSpace(tenantID))

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
			nullif($8, '')::uuid,
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
		strings.TrimSpace(updatedByUserID),
	).Scan(&updatedAt)
	if err != nil {
		return RulesView{}, err
	}

	current.Source = RulesSourceDatabase
	current.UpdatedAt = &updatedAt
	return current, nil
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

func (repository *PostgresRepository) LoadOperationalRules(ctx context.Context, storeID string) (OperationalRules, error) {
	row := repository.pool.QueryRow(ctx, `
		select s.tenant_id::text,
		       coalesce(r.long_open_service_minutes, $2),
		       coalesce(r.notify_dashboard, true),
		       coalesce(r.notify_operation_context, true),
		       coalesce(r.notify_external, false)
		from stores s
		left join tenant_operational_alert_rules r on r.tenant_id = s.tenant_id
		where s.id = $1::uuid
		limit 1;
	`, strings.TrimSpace(storeID), defaultLongOpenMinutes)

	var rules OperationalRules
	err := row.Scan(
		&rules.TenantID,
		&rules.LongOpenServiceMinutes,
		&rules.NotifyDashboard,
		&rules.NotifyOperationContext,
		&rules.NotifyExternal,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OperationalRules{
				LongOpenServiceMinutes: defaultLongOpenMinutes,
				NotifyDashboard:        true,
				NotifyOperationContext: true,
			}, nil
		}
		return OperationalRules{}, err
	}

	if rules.LongOpenServiceMinutes < 1 {
		rules.LongOpenServiceMinutes = defaultLongOpenMinutes
	}

	return rules, nil
}

func (repository *PostgresRepository) ProcessOperationalSignals(ctx context.Context, signals []OperationalSignalInput) ([]SignalMutation, error) {
	normalized := make([]OperationalSignalInput, 0, len(signals))
	for _, signal := range signals {
		storeID := strings.TrimSpace(signal.StoreID)
		serviceID := strings.TrimSpace(signal.ServiceID)
		signalType := strings.TrimSpace(signal.SignalType)
		if storeID == "" || serviceID == "" || signalType == "" {
			continue
		}

		triggeredAt := signal.TriggeredAt.UTC()
		if triggeredAt.IsZero() {
			triggeredAt = time.Now().UTC()
		}

		normalized = append(normalized, OperationalSignalInput{
			TenantID:       strings.TrimSpace(signal.TenantID),
			StoreID:        storeID,
			ServiceID:      serviceID,
			ConsultantID:   strings.TrimSpace(signal.ConsultantID),
			SignalType:     signalType,
			TriggeredAt:    triggeredAt,
			Metadata:       normalizeMetadata(signal.Metadata),
			ConsultantName: strings.TrimSpace(signal.ConsultantName),
			ElapsedMinutes: signal.ElapsedMinutes,
			TriggerType:    strings.TrimSpace(signal.TriggerType),
		})
	}

	if len(normalized) == 0 {
		return nil, nil
	}

	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	mutations := make([]SignalMutation, 0, len(normalized))
	for _, signal := range normalized {
		var mutation *SignalMutation

		switch signal.SignalType {
		case SignalLongOpenServiceTriggered:
			mutation, err = repository.processLongOpenTriggeredTx(ctx, tx, signal)
		case SignalLongOpenServiceResolved:
			mutation, err = repository.processLongOpenResolvedTx(ctx, tx, signal)
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		if mutation != nil {
			mutations = append(mutations, *mutation)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return mutations, nil
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

func (repository *PostgresRepository) processLongOpenTriggeredTx(ctx context.Context, tx pgx.Tx, signal OperationalSignalInput) (*SignalMutation, error) {
	tenantID, err := repository.resolveSignalTenantIDTx(ctx, tx, signal)
	if err != nil {
		return nil, err
	}

	triggerType := strings.TrimSpace(signal.TriggerType)
	if triggerType == "" {
		triggerType = TriggerLongOpenService
	}

	// Load active rule definition for this trigger type.
	rules, err := repository.LoadActiveRulesForTrigger(ctx, tenantID, triggerType)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}

	requestedRuleID := metadataString(signal.Metadata, "ruleDefinitionId")
	var rule *RuleDefinition
	if requestedRuleID != "" {
		for index := range rules {
			if strings.TrimSpace(rules[index].ID) == requestedRuleID {
				rule = &rules[index]
				break
			}
		}
		if rule == nil {
			return nil, nil
		}
	}
	if rule == nil {
		rule = &rules[0]
	}

	// Look up consultant name within the same connection
	consultantName := strings.TrimSpace(signal.ConsultantName)
	if cid := strings.TrimSpace(signal.ConsultantID); cid != "" {
		if consultantName == "" {
			_ = tx.QueryRow(ctx, `SELECT coalesce(name, '') FROM consultants WHERE id = $1::uuid`, cid).Scan(&consultantName)
		}
	}

	// Compute elapsed minutes from metadata
	elapsedMinutes := signal.ElapsedMinutes
	if sa := metadataInt64(signal.Metadata, "serviceStartedAt"); elapsedMinutes < 1 && sa > 0 {
		startedAt := time.UnixMilli(int64(sa)).UTC()
		elapsedMinutes = ElapsedMinutesSince(startedAt, signal.TriggeredAt)
	}
	thresholdMinutes := metadataInt(signal.Metadata, "thresholdMinutes")
	if thresholdMinutes < 1 && rule != nil {
		thresholdMinutes = rule.ThresholdMinutes
	}

	consultantTemplateName := consultantName
	if consultantTemplateName == "" {
		consultantTemplateName = "Loja"
	}

	templateVars := map[string]string{
		"consultant": consultantTemplateName,
		"elapsed":    FormatElapsed(elapsedMinutes),
		"threshold":  fmt.Sprintf("%d", thresholdMinutes),
	}

	// Resolve display/interaction fields from rule or use sensible defaults
	titleTpl := "Atendimento longo detectado"
	bodyTpl := ""
	displayKind := DisplayKindBanner
	colorTheme := ColorThemeAmber
	severity := SeverityCritical
	interactionKind := InteractionKindConfirmChoice
	responseOptionsJSON := `[{"value":"still_happening","label":"Ainda está acontecendo"},{"value":"forgotten","label":"Esqueci de fechar"}]`
	isMandatory := false
	ruleDefinitionID := ""

	if rule != nil {
		ruleDefinitionID = rule.ID
		if rule.TitleTemplate != "" {
			titleTpl = rule.TitleTemplate
		}
		if rule.BodyTemplate != "" {
			bodyTpl = rule.BodyTemplate
		}
		displayKind = rule.DisplayKind
		colorTheme = rule.ColorTheme
		severity = rule.Severity
		interactionKind = rule.InteractionKind
		isMandatory = rule.IsMandatory
		if (rule.InteractionKind == InteractionKindConfirmChoice || rule.InteractionKind == InteractionKindSelectOption) && len(rule.ResponseOptions) > 0 {
			if b, jerr := json.Marshal(rule.ResponseOptions); jerr == nil {
				responseOptionsJSON = string(b)
			}
		} else {
			responseOptionsJSON = "[]"
		}
	}

	headline := RenderTemplate(titleTpl, templateVars)
	body := RenderTemplate(bodyTpl, templateVars)
	if body == "" {
		if consultantName != "" {
			body = fmt.Sprintf("O atendimento de %s segue aberto acima do tempo configurado.", consultantName)
		} else {
			body = "Atendimento aberto acima do tempo configurado."
		}
	}

	dedupeKey := buildLongOpenDedupeKey(signal.StoreID, signal.ServiceID)
	metadata := normalizeMetadata(signal.Metadata)
	metadata["signalType"] = signal.SignalType

	existing, err := repository.findOpenAlertByDedupeKeyTx(ctx, tx, dedupeKey)
	if err == nil {
		_, err = tx.Exec(ctx, `
			update alert_instances
			set consultant_id = coalesce(nullif($3, '')::uuid, consultant_id),
			    severity = $4,
			    headline = $5,
			    body = $6,
			    metadata = $7::jsonb,
			    last_triggered_at = $8,
			    interaction_kind = $9,
			    display_kind = $10,
			    color_theme = $11,
			    response_options = $12::jsonb,
			    is_mandatory = $13,
			    rule_definition_id = coalesce(nullif($14, '')::uuid, rule_definition_id),
			    consultant_name = $15,
			    updated_at = $8
			where id = $1::uuid;
		`, existing.ID, tenantID, strings.TrimSpace(signal.ConsultantID), severity, headline, body,
			marshalJSONB(metadata), signal.TriggeredAt, interactionKind,
			displayKind, colorTheme, responseOptionsJSON, isMandatory, ruleDefinitionID, consultantName)
		if err != nil {
			return nil, err
		}

		existing.LastTriggeredAt = signal.TriggeredAt
		existing.UpdatedAt = signal.TriggeredAt
		existing.Metadata = metadata
		existing.Headline = headline
		existing.Body = body
		existing.ConsultantName = consultantName
		if err := repository.appendAlertActionTx(ctx, tx, *existing, ActionTriggered, Actor{}, "", metadata); err != nil {
			return nil, err
		}

		return &SignalMutation{
			TenantID: tenantID,
			AlertID:  existing.ID,
			Action:   "upserted",
			SavedAt:  signal.TriggeredAt,
		}, nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	var alertID string
	err = tx.QueryRow(ctx, `
		insert into alert_instances (
			tenant_id,
			store_id,
			service_id,
			consultant_id,
			type,
			category,
			severity,
			status,
			source_module,
			dedupe_key,
			headline,
			body,
			metadata,
			opened_at,
			last_triggered_at,
			interaction_kind,
			display_kind,
			color_theme,
			response_options,
			is_mandatory,
			rule_definition_id,
			consultant_name,
			created_at,
			updated_at
		) values (
			$1::uuid,
			$2::uuid,
			$3,
			nullif($4, '')::uuid,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12,
			$13::jsonb,
			$14,
			$15,
			$16,
			$17,
			$18,
			$19::jsonb,
			$20,
			nullif($21, '')::uuid,
			$22,
			$23,
			$24
		) returning id::text;
	`,
		tenantID,
		signal.StoreID,
		signal.ServiceID,
		strings.TrimSpace(signal.ConsultantID),
		TypeLongOpenService,
		CategoryOperational,
		severity,
		StatusActive,
		SourceModuleOperations,
		dedupeKey,
		headline,
		body,
		marshalJSONB(metadata),
		signal.TriggeredAt,
		signal.TriggeredAt,
		interactionKind,
		displayKind,
		colorTheme,
		responseOptionsJSON,
		isMandatory,
		ruleDefinitionID,
		consultantName,
		signal.TriggeredAt,
		signal.TriggeredAt,
	).Scan(&alertID)
	if err != nil {
		return nil, err
	}

	alert := Alert{
		ID:              alertID,
		TenantID:        tenantID,
		StoreID:         signal.StoreID,
		ServiceID:       signal.ServiceID,
		ConsultantID:    strings.TrimSpace(signal.ConsultantID),
		Type:            TypeLongOpenService,
		Category:        CategoryOperational,
		Severity:        severity,
		Status:          StatusActive,
		SourceModule:    SourceModuleOperations,
		DedupeKey:       dedupeKey,
		Headline:        headline,
		Body:            body,
		OpenedAt:        signal.TriggeredAt,
		LastTriggeredAt: signal.TriggeredAt,
		CreatedAt:       signal.TriggeredAt,
		UpdatedAt:       signal.TriggeredAt,
		Metadata:        metadata,
		DisplayKind:     displayKind,
		ColorTheme:      colorTheme,
		IsMandatory:     isMandatory,
		ConsultantName:  consultantName,
	}
	if err := repository.appendAlertActionTx(ctx, tx, alert, ActionTriggered, Actor{}, "", metadata); err != nil {
		return nil, err
	}

	return &SignalMutation{
		TenantID: tenantID,
		AlertID:  alertID,
		Action:   "opened",
		SavedAt:  signal.TriggeredAt,
	}, nil
}

func (repository *PostgresRepository) processLongOpenResolvedTx(ctx context.Context, tx pgx.Tx, signal OperationalSignalInput) (*SignalMutation, error) {
	tenantID, err := repository.resolveSignalTenantIDTx(ctx, tx, signal)
	if err != nil {
		return nil, err
	}

	dedupeKey := buildLongOpenDedupeKey(signal.StoreID, signal.ServiceID)
	alert, err := repository.findOpenAlertByDedupeKeyTx(ctx, tx, dedupeKey)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		update alert_instances
		set status = $2,
		    resolved_at = $3,
		    last_triggered_at = $3,
		    updated_at = $3
		where id = $1::uuid;
	`, alert.ID, StatusResolved, signal.TriggeredAt)
	if err != nil {
		return nil, err
	}

	alert.Status = StatusResolved
	alert.ResolvedAt = &signal.TriggeredAt
	alert.UpdatedAt = signal.TriggeredAt
	metadata := normalizeMetadata(signal.Metadata)
	metadata["signalType"] = signal.SignalType
	if err := repository.appendAlertActionTx(ctx, tx, *alert, ActionAutoResolved, Actor{}, "", metadata); err != nil {
		return nil, err
	}

	return &SignalMutation{
		TenantID: tenantID,
		AlertID:  alert.ID,
		Action:   "resolved",
		SavedAt:  signal.TriggeredAt,
	}, nil
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

func (repository *PostgresRepository) resolveSignalTenantIDTx(ctx context.Context, tx pgx.Tx, signal OperationalSignalInput) (string, error) {
	if normalizedTenantID := strings.TrimSpace(signal.TenantID); normalizedTenantID != "" {
		return normalizedTenantID, nil
	}

	var tenantID string
	err := tx.QueryRow(ctx, `
		select tenant_id::text
		from stores
		where id = $1::uuid
		limit 1;
	`, strings.TrimSpace(signal.StoreID)).Scan(&tenantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrValidation
		}
		return "", err
	}

	return strings.TrimSpace(tenantID), nil
}

func appendStoreFilter(builder *strings.Builder, args *[]any, argIndex *int, storeID string, storeIDs []string) {
	if normalizedStoreID := strings.TrimSpace(storeID); normalizedStoreID != "" {
		builder.WriteString(fmt.Sprintf(" and store_id = $%d::uuid", *argIndex))
		*args = append(*args, normalizedStoreID)
		*argIndex++
		return
	}

	normalizedStoreIDs := normalizeStringSlice(storeIDs)
	if len(normalizedStoreIDs) == 0 {
		return
	}

	builder.WriteString(fmt.Sprintf(" and store_id::text = any($%d::text[])", *argIndex))
	*args = append(*args, normalizedStoreIDs)
	*argIndex++
}

func normalizeStringSlice(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	return normalized
}

func normalizeMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		cloned[trimmedKey] = value
	}

	return cloned
}

func metadataInt(metadata map[string]any, key string) int {
	value := metadataInt64(metadata, key)
	if value < 0 {
		return 0
	}
	return int(value)
}

func metadataInt64(metadata map[string]any, key string) int64 {
	if len(metadata) == 0 {
		return 0
	}

	switch value := metadata[key].(type) {
	case int:
		return int64(value)
	case int32:
		return int64(value)
	case int64:
		return value
	case float32:
		return int64(value)
	case float64:
		return int64(value)
	case json.Number:
		parsed, err := value.Int64()
		if err == nil {
			return parsed
		}
	case string:
		var parsed json.Number = json.Number(strings.TrimSpace(value))
		parsedValue, err := parsed.Int64()
		if err == nil {
			return parsedValue
		}
	}

	return 0
}

func metadataString(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}

	value, exists := metadata[key]
	if !exists || value == nil {
		return ""
	}

	switch value := value.(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func marshalJSONB(metadata map[string]any) []byte {
	normalized := normalizeMetadata(metadata)
	encoded, err := json.Marshal(normalized)
	if err != nil || len(encoded) == 0 {
		return []byte("{}")
	}

	return encoded
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

func buildLongOpenDedupeKey(storeID string, serviceID string) string {
	return fmt.Sprintf("operations:%s:%s:%s", TypeLongOpenService, strings.TrimSpace(storeID), strings.TrimSpace(serviceID))
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
		Source:                   RulesSourceDefaults,
	}
}

// ListRules retrieves all rule definitions for a tenant with optional filters
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
		query.WriteString(fmt.Sprintf(" and is_active = true"))
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

// GetRule retrieves a single rule definition by ID
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

// CreateRule inserts a new rule definition
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

// UpdateRule modifies an existing rule definition
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

// DeleteRule removes a rule definition
func (repository *PostgresRepository) DeleteRule(ctx context.Context, ruleID string) error {
	_, err := repository.pool.Exec(ctx, `
		delete from alert_rule_definitions
		where id = $1::uuid;
	`, strings.TrimSpace(ruleID))
	return err
}

// LoadActiveRulesForTrigger retrieves active rules by trigger type for a tenant
func (repository *PostgresRepository) LoadActiveRulesForTrigger(ctx context.Context, tenantID string, triggerType string) ([]RuleDefinition, error) {
	return repository.ListRules(ctx, ListRulesInput{
		TenantID:    strings.TrimSpace(tenantID),
		TriggerType: strings.TrimSpace(triggerType),
		OnlyActive:  true,
	})
}

// scanRuleDefinition parses a rule row from the database
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
