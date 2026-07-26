package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerdata"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerintelligence"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel"
)

// customerIntelligenceScopeAdapter keeps account/client authorization in the
// composition root. Customer Intelligence never needs to query core.* or know
// how agency and standalone accounts are related.
type customerIntelligenceScopeAdapter struct {
	pool *pgxpool.Pool
}

type customerIntelligenceRelationshipScopeAdapter struct {
	service func() *customerdata.Service
}

func (a customerIntelligenceRelationshipScopeAdapter) AuthorizeRelationshipScope(
	ctx context.Context,
	accountID, clientAccountID, subjectID, relationshipID string,
) error {
	service := a.service()
	if service == nil {
		return customerintelligence.ErrForbidden
	}
	profile, err := service.GetDeterministicProfile(
		ctx,
		customerdata.DeterministicProfileRequest{
			AccountID:       accountID,
			ClientAccountID: clientAccountID,
			RelationshipID:  relationshipID,
		},
	)
	if err != nil {
		if errors.Is(err, customerdata.ErrNotFound) {
			return customerintelligence.ErrNotFound
		}
		return err
	}
	if subjectID != "" && profile.Subject.ID != subjectID {
		return customerintelligence.ErrNotFound
	}
	return nil
}

func (a customerIntelligenceScopeAdapter) AuthorizeClientScope(
	ctx context.Context,
	accountID, clientAccountID string,
) error {
	if a.pool == nil {
		return customerintelligence.ErrForbidden
	}
	var allowed bool
	err := a.pool.QueryRow(ctx, `
		select exists (
			select 1
			from core.accounts owner
			join core.accounts client on client.id = $2::uuid
			where owner.id = $1::uuid
			  and owner.is_active = true
			  and client.is_active = true
			  and (
				(owner.is_agency = false and client.id = owner.id)
				or (
					owner.is_agency = true
					and client.is_agency = false
					and owner.organization_id is not null
					and client.organization_id = owner.organization_id
				)
			  )
		)`, accountID, clientAccountID,
	).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return customerintelligence.ErrNotFound
	}
	return nil
}

func (a customerIntelligenceScopeAdapter) AuthorizePortfolioScope(
	ctx context.Context,
	accountID, targetClientAccountID string,
) error {
	return a.AuthorizeClientScope(ctx, accountID, targetClientAccountID)
}

// omnichannelCustomerIntelligenceAdapter is the only place that knows both
// module contracts. It resolves the deterministic customer first and sends a
// minimized, typed request to the headless intelligence runtime.
type omnichannelCustomerIntelligenceAdapter struct {
	customerData func() *customerdata.Service
	runtime      func() customerintelligence.Runtime
}

func (a omnichannelCustomerIntelligenceAdapter) ExecuteInteraction(
	ctx context.Context,
	request omnichannel.CustomerIntelligenceInteractionRequest,
) (omnichannel.CustomerIntelligenceDecision, error) {
	dataService := a.customerData()
	runtime := a.runtime()
	if dataService == nil || runtime == nil {
		return omnichannel.CustomerIntelligenceDecision{
			Outcome:    "no_reply",
			ReasonCode: "customer_intelligence_disabled",
		}, nil
	}
	resolved, err := dataService.ResolveRelationship(ctx, customerdata.ResolveRelationshipRequest{
		AccountID:       request.AccountID,
		ClientAccountID: request.ClientAccountID,
		RequestID:       "omnichannel:" + request.DispatchID,
		Source: customerdata.SourceReference{
			SourceModule:     "omnichannel",
			SourceKey:        strings.ToLower(strings.TrimSpace(request.Channel)),
			SourceEntityType: "contact",
			SourceEntityID:   request.ContactSourceID,
			SourceVersion:    "interaction.request.v1",
		},
		Identities:  customerIdentities(request),
		DisplayName: strings.TrimSpace(request.ContactName),
		OccurredAt:  request.OccurredAt,
		Purpose:     "customer_service",
		AllowCreate: true,
	})
	if err != nil {
		return omnichannel.CustomerIntelligenceDecision{}, err
	}
	if resolved.RelationshipID == "" || resolved.SubjectID == "" ||
		(resolved.Status != "resolved" && resolved.Status != "created") {
		return omnichannel.CustomerIntelligenceDecision{}, customerintelligence.ErrNotFound
	}
	asOf := request.OccurredAt
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	decision, err := runtime.ExecuteInteraction(ctx, customerintelligence.InteractionRequest{
		SchemaVersion:       "interaction.request.v1",
		RequestID:           request.DispatchID + ":" + fmt.Sprint(request.Generation),
		InteractionID:       request.DispatchID,
		AccountID:           request.AccountID,
		ClientAccountID:     request.ClientAccountID,
		SubjectID:           resolved.SubjectID,
		RelationshipID:      resolved.RelationshipID,
		ConversationID:      request.ConversationID,
		PipelineKey:         "conversation.respond",
		AIGeneration:        request.Generation,
		Message:             request.Message,
		OperationalState:    request.OperationalState,
		RoutingCatalog:      request.RoutingCatalog,
		ChannelCapabilities: request.ChannelCapabilities,
		Purpose:             "customer_service",
		Locale:              "pt-BR",
		Channel:             strings.ToLower(strings.TrimSpace(request.Channel)),
		AsOf:                asOf,
		// A data da mensagem pode ser antiga em replay/debounce. O prazo de
		// execução começa agora; AsOf continua preservando o tempo do evento.
		DeadlineAt:    time.Now().UTC().Add(20 * time.Second),
		MaxItems:      100,
		MaxTokens:     8000,
		CorrelationID: request.DispatchID,
	})
	if err != nil {
		if shadow, ok := customerintelligence.RuntimeShadowDecision(err); ok {
			if shadow.AccountID != request.AccountID ||
				shadow.ClientAccountID != request.ClientAccountID ||
				shadow.RequestID != request.DispatchID+":"+fmt.Sprint(request.Generation) ||
				shadow.AIGeneration != request.Generation {
				return omnichannel.CustomerIntelligenceDecision{},
					omnichannel.NewCustomerIntelligenceFailure(
						"invalid_result",
						"shadow_scope_mismatch",
						false,
						customerintelligence.ErrInvalidInput,
					)
			}
			return translateIntelligenceDecision(shadow, resolved), nil
		}
		if kind, code, retryable, ok := customerintelligence.RuntimeFailureDetails(err); ok {
			return omnichannel.CustomerIntelligenceDecision{},
				omnichannel.NewCustomerIntelligenceFailure(
					string(kind), code, retryable, err,
				)
		}
		return omnichannel.CustomerIntelligenceDecision{}, err
	}
	if decision.AccountID != request.AccountID ||
		decision.ClientAccountID != request.ClientAccountID ||
		decision.RequestID != request.DispatchID+":"+fmt.Sprint(request.Generation) ||
		decision.AIGeneration != request.Generation {
		return omnichannel.CustomerIntelligenceDecision{}, customerintelligence.ErrInvalidInput
	}
	return translateIntelligenceDecision(decision, resolved), nil
}

func (a omnichannelCustomerIntelligenceAdapter) RecordAcceptedOutcome(
	ctx context.Context,
	outcome omnichannel.CustomerIntelligenceAcceptedOutcome,
) error {
	runtime := a.runtime()
	if runtime == nil {
		return customerintelligence.ErrCapabilityDisabled
	}
	normalizedOutcome := strings.TrimSpace(outcome.Outcome)
	switch normalizedOutcome {
	case "reply", "handoff", "no_reply":
	case "routed":
		normalizedOutcome = "handoff"
	case "close":
		normalizedOutcome = "reply"
	default:
		return customerintelligence.ErrInvalidInput
	}
	processRunRefs := make([]map[string]string, 0, len(outcome.ProcessRuns))
	for _, run := range outcome.ProcessRuns {
		processRunRefs = append(processRunRefs, map[string]string{
			"processKey":              run.ProcessKey,
			"runId":                   run.RunID,
			"status":                  run.Status,
			"executionMode":           run.ExecutionMode,
			"processDefinitionId":     run.ProcessDefinitionID,
			"processConfigVersionId":  run.ProcessConfigVersionID,
			"promptBindingId":         run.PromptBindingID,
			"platformPromptVersionId": run.PlatformPromptVersionID,
			"agencyPromptVersionId":   run.AgencyPromptVersionID,
			"clientPromptVersionId":   run.ClientPromptVersionID,
			"processPromptVersionId":  run.ProcessPromptVersionID,
			"agentVersionId":          run.AgentVersionID,
			"modelId":                 run.ModelID,
			"contextSnapshotId":       run.ContextSnapshotID,
			"outputSchemaVersion":     run.OutputSchemaVersion,
		})
	}
	if len(processRunRefs) == 0 && outcome.RunID != "" {
		processRunRefs = append(processRunRefs, map[string]string{
			"processKey": "conversation.respond",
			"runId":      outcome.RunID,
		})
	}
	payload, err := json.Marshal(map[string]any{
		"reasonCode":        "omnichannel_effect_accepted",
		"subjectId":         outcome.SubjectID,
		"pipelineVersionId": outcome.PipelineVersionID,
		"processRunRefs":    processRunRefs,
		"dispatchId":        outcome.DispatchID,
		"messageId":         outcome.MessageID,
		"generation":        outcome.Generation,
		"effect":            outcome.Outcome,
	})
	if err != nil {
		return err
	}
	typedRuns := make([]customerintelligence.ProcessRunRef, 0, len(outcome.ProcessRuns))
	for _, run := range outcome.ProcessRuns {
		typedRuns = append(typedRuns, customerintelligence.ProcessRunRef{
			ProcessKey:              run.ProcessKey,
			RunID:                   run.RunID,
			Status:                  run.Status,
			ExecutionMode:           run.ExecutionMode,
			ProcessDefinitionID:     run.ProcessDefinitionID,
			ProcessConfigVersionID:  run.ProcessConfigVersionID,
			PromptBindingID:         run.PromptBindingID,
			PlatformPromptVersionID: run.PlatformPromptVersionID,
			AgencyPromptVersionID:   run.AgencyPromptVersionID,
			ClientPromptVersionID:   run.ClientPromptVersionID,
			ProcessPromptVersionID:  run.ProcessPromptVersionID,
			AgentVersionID:          run.AgentVersionID,
			ModelID:                 run.ModelID,
			ContextSnapshotID:       run.ContextSnapshotID,
			OutputSchemaVersion:     run.OutputSchemaVersion,
		})
	}
	claims := make([]customerintelligence.AcceptedClaimRef, 0, len(outcome.Claims))
	for _, claim := range outcome.Claims {
		claims = append(claims, customerintelligence.AcceptedClaimRef{
			Ordinal:                claim.Ordinal,
			FactKey:                claim.FactKey,
			ValueType:              claim.ValueType,
			Confidence:             claim.Confidence,
			EvidenceObservationIDs: append([]string(nil), claim.EvidenceObservationIDs...),
			ValidFrom:              claim.ValidFrom,
			ValidUntil:             claim.ValidUntil,
			ProcessKey:             claim.ProcessKey,
			RuntimeRunID:           claim.RuntimeRunID,
			PromptBindingID:        claim.PromptBindingID,
			OutputSchemaVersion:    claim.OutputSchemaVersion,
		})
	}
	_, err = runtime.RecordOutcome(ctx, customerintelligence.AcceptedOutcome{
		AccountID:       outcome.AccountID,
		ClientAccountID: outcome.ClientAccountID,
		EventID:         outcome.EventID,
		InteractionID:   outcome.DispatchID,
		DecisionID:      outcome.DecisionID,
		SubjectID:       outcome.SubjectID,
		RelationshipID:  outcome.RelationshipID,
		ConversationID:  outcome.ConversationID,
		OutcomeType:     normalizedOutcome,
		Accepted:        true,
		ActorType:       "system",
		ProcessRuns:     typedRuns,
		Claims:          claims,
		Payload:         payload,
		OccurredAt:      time.Now().UTC(),
	})
	return err
}

func customerIdentities(
	request omnichannel.CustomerIntelligenceInteractionRequest,
) []customerdata.IdentityInput {
	return customerDataIdentityInputs(
		request.Channel,
		request.ContactSourceID,
		request.ContactPhone,
		request.ContactExternalID,
		request.OccurredAt,
	)
}

func translateIntelligenceDecision(
	decision customerintelligence.InteractionDecision,
	resolved customerdata.ResolveRelationshipResult,
) omnichannel.CustomerIntelligenceDecision {
	translated := omnichannel.CustomerIntelligenceDecision{
		DecisionID:        decision.DecisionID,
		PipelineVersionID: decision.PipelineVersionID,
		SubjectID:         resolved.SubjectID,
		RelationshipID:    resolved.RelationshipID,
		Outcome:           decision.Outcome,
		NeedsHuman:        decision.NeedsHuman,
		Confidence:        decision.Confidence,
		ReasonCode:        decision.ReasonCode,
		ExtractedFields: map[string]any{
			"intent":     decision.Intent,
			"categories": decision.Categories,
			"lead_stage": decision.LeadStage,
		},
	}
	if decision.ReplyDraft != nil {
		translated.ReplyDraft = strings.TrimSpace(*decision.ReplyDraft)
	}
	if len(decision.ProcessRuns) > 0 {
		translated.RunID = decision.ProcessRuns[len(decision.ProcessRuns)-1].RunID
		translated.ProcessRuns = make(
			[]omnichannel.CustomerIntelligenceProcessRunRef,
			0,
			len(decision.ProcessRuns),
		)
		for _, run := range decision.ProcessRuns {
			translated.ProcessRuns = append(
				translated.ProcessRuns,
				omnichannel.CustomerIntelligenceProcessRunRef{
					ProcessKey:              run.ProcessKey,
					RunID:                   run.RunID,
					Status:                  run.Status,
					ExecutionMode:           run.ExecutionMode,
					ProcessDefinitionID:     run.ProcessDefinitionID,
					ProcessConfigVersionID:  run.ProcessConfigVersionID,
					PromptBindingID:         run.PromptBindingID,
					PlatformPromptVersionID: run.PlatformPromptVersionID,
					AgencyPromptVersionID:   run.AgencyPromptVersionID,
					ClientPromptVersionID:   run.ClientPromptVersionID,
					ProcessPromptVersionID:  run.ProcessPromptVersionID,
					AgentVersionID:          run.AgentVersionID,
					ModelID:                 run.ModelID,
					ContextSnapshotID:       run.ContextSnapshotID,
					OutputSchemaVersion:     run.OutputSchemaVersion,
				},
			)
		}
	}
	translated.OperationalEffectAllowed =
		intelligenceRunsAllowOperationalEffect(decision.ProcessRuns)
	translated.CandidateClaims = acceptedClaimReferences(
		decision.ExtractedClaims,
		translated.ProcessRuns,
	)
	if decision.NeedsHuman || decision.Outcome == "handoff" {
		translated.Outcome = "handoff"
		translated.HandoffReason = firstNonEmptyApp(decision.ReasonCode, "intelligence_handoff")
		translated.HandoffSummary = "A Inteligência do Cliente recomendou continuidade com atendimento humano."
	}
	var closure struct {
		Requested      bool    `json:"requested"`
		Reason         string  `json:"reason"`
		Confidence     float64 `json:"confidence"`
		HumanRequested bool    `json:"humanRequested"`
		SensitiveTopic bool    `json:"sensitiveTopic"`
	}
	if len(decision.Closure) > 0 && json.Unmarshal(decision.Closure, &closure) == nil {
		translated.CloseRequested = closure.Requested
		translated.CloseReason = strings.TrimSpace(closure.Reason)
		translated.HumanRequested = closure.HumanRequested
		translated.SensitiveTopic = closure.SensitiveTopic
		if closure.Confidence > 0 && closure.Confidence < translated.Confidence {
			translated.Confidence = closure.Confidence
		}
	}
	return translated
}

func intelligenceRunsAllowOperationalEffect(
	runs []customerintelligence.ProcessRunRef,
) bool {
	if len(runs) == 0 {
		return false
	}
	for _, run := range runs {
		if run.Status != "succeeded" || run.ExecutionMode != "active" {
			return false
		}
	}
	return true
}

func acceptedClaimReferences(
	raw json.RawMessage,
	runs []omnichannel.CustomerIntelligenceProcessRunRef,
) []omnichannel.CustomerIntelligenceAcceptedClaimRef {
	var sourceRun *omnichannel.CustomerIntelligenceProcessRunRef
	for index := range runs {
		if runs[index].ProcessKey == "conversation.triage" ||
			runs[index].ProcessKey == "memory.extract" {
			sourceRun = &runs[index]
			break
		}
	}
	if sourceRun == nil {
		return []omnichannel.CustomerIntelligenceAcceptedClaimRef{}
	}
	var extracted []struct {
		FactKey                string          `json:"factKey"`
		ValueType              string          `json:"valueType"`
		Value                  json.RawMessage `json:"value"`
		Confidence             float64         `json:"confidence"`
		EvidenceObservationIDs []string        `json:"evidenceObservationIds"`
		ValidFrom              *string         `json:"validFrom"`
		ValidUntil             *string         `json:"validUntil"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &extracted) != nil {
		return []omnichannel.CustomerIntelligenceAcceptedClaimRef{}
	}
	claims := make([]omnichannel.CustomerIntelligenceAcceptedClaimRef, 0, len(extracted))
	for ordinal, claim := range extracted {
		claim.FactKey = strings.TrimSpace(claim.FactKey)
		claim.ValueType = strings.TrimSpace(claim.ValueType)
		if len(claim.Value) == 0 ||
			!safeAcceptedClaimKey(claim.FactKey) ||
			!acceptedClaimValueType(claim.ValueType) ||
			claim.Confidence < 0 || claim.Confidence > 1 {
			continue
		}
		reference := omnichannel.CustomerIntelligenceAcceptedClaimRef{
			Ordinal:                ordinal,
			FactKey:                claim.FactKey,
			ValueType:              claim.ValueType,
			Confidence:             claim.Confidence,
			EvidenceObservationIDs: acceptedObservationIDs(claim.EvidenceObservationIDs),
			ProcessKey:             sourceRun.ProcessKey,
			RuntimeRunID:           sourceRun.RunID,
			PromptBindingID:        sourceRun.PromptBindingID,
			OutputSchemaVersion:    sourceRun.OutputSchemaVersion,
		}
		if claim.ValidFrom != nil {
			reference.ValidFrom = strings.TrimSpace(*claim.ValidFrom)
			if !validAcceptedClaimTime(reference.ValidFrom) {
				continue
			}
		}
		if claim.ValidUntil != nil {
			reference.ValidUntil = strings.TrimSpace(*claim.ValidUntil)
			if !validAcceptedClaimTime(reference.ValidUntil) {
				continue
			}
		}
		claims = append(claims, reference)
	}
	return claims
}

func safeAcceptedClaimKey(value string) bool {
	if value == "" || len(value) > 160 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	separator := false
	for _, character := range value[1:] {
		switch {
		case character >= 'a' && character <= 'z':
			separator = false
		case character >= '0' && character <= '9':
			separator = false
		case character == '.', character == '_', character == '-':
			if separator {
				return false
			}
			separator = true
		default:
			return false
		}
	}
	return !separator
}

func acceptedClaimValueType(value string) bool {
	switch value {
	case "string", "integer", "decimal", "boolean", "date", "timestamp",
		"enum", "string_list", "object_closed":
		return true
	default:
		return false
	}
}

func acceptedObservationIDs(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if looksLikeUUID(value) && !seen[value] {
			seen[value] = true
			out = append(out, value)
			if len(out) == 100 {
				break
			}
		}
	}
	return out
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 ||
		value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		isDigit := character >= '0' && character <= '9'
		isLowercaseHex := character >= 'a' && character <= 'f'
		isUppercaseHex := character >= 'A' && character <= 'F'
		if !isDigit && !isLowercaseHex && !isUppercaseHex {
			return false
		}
	}
	return true
}

func validAcceptedClaimTime(value string) bool {
	if value == "" {
		return true
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return true
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

func firstNonEmptyApp(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

var (
	_ customerintelligence.ClientScopeAuthorizer       = customerIntelligenceScopeAdapter{}
	_ customerintelligence.PortfolioScopeAuthorizer    = customerIntelligenceScopeAdapter{}
	_ customerintelligence.RelationshipScopeAuthorizer = customerIntelligenceRelationshipScopeAdapter{}
	_ omnichannel.CustomerIntelligenceBridge           = omnichannelCustomerIntelligenceAdapter{}
)
