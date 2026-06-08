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

func (repository *PostgresRepository) LoadOperationalRules(ctx context.Context, storeID string) (OperationalRules, error) {
	row := repository.pool.QueryRow(ctx, `
		select s.tenant_id::text,
		       coalesce(r.long_open_service_minutes, $2),
		       coalesce(r.notify_dashboard, true),
		       coalesce(r.notify_operation_context, true),
		       coalesce(r.notify_external, false)
		from queue.stores s
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

func (repository *PostgresRepository) processLongOpenTriggeredTx(ctx context.Context, tx pgx.Tx, signal OperationalSignalInput) (*SignalMutation, error) {
	tenantID, err := repository.resolveSignalTenantIDTx(ctx, tx, signal)
	if err != nil {
		return nil, err
	}

	triggerType := strings.TrimSpace(signal.TriggerType)
	if triggerType == "" {
		triggerType = TriggerLongOpenService
	}

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

	consultantName := strings.TrimSpace(signal.ConsultantName)
	if cid := strings.TrimSpace(signal.ConsultantID); cid != "" {
		if consultantName == "" {
			_ = tx.QueryRow(ctx, `SELECT coalesce(name, '') FROM queue.consultants WHERE id = $1::uuid`, cid).Scan(&consultantName)
		}
	}

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
			where id = $1::uuid
			  and tenant_id = $2::uuid;
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

func (repository *PostgresRepository) resolveSignalTenantIDTx(ctx context.Context, tx pgx.Tx, signal OperationalSignalInput) (string, error) {
	if normalizedTenantID := strings.TrimSpace(signal.TenantID); normalizedTenantID != "" {
		return normalizedTenantID, nil
	}

	var tenantID string
	err := tx.QueryRow(ctx, `
		select tenant_id::text
		from queue.stores
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

func buildLongOpenDedupeKey(storeID string, serviceID string) string {
	return fmt.Sprintf("operations:%s:%s:%s", TypeLongOpenService, strings.TrimSpace(storeID), strings.TrimSpace(serviceID))
}
