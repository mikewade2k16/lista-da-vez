package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var metaCampaignIDPattern = regexp.MustCompile(`^[0-9]{5,32}$`)

type graphActionExecutor struct {
	tokens          actionTokenProvider
	client          actionGraphClient
	steps           actionStepRepository
	instagramScopes actionInstagramScopeStore
}

type actionInstagramScopeStore interface {
	ValidateActionInstagramIdentityScope(context.Context, string, string, string, string, string, string) error
}

type actionTokenProvider interface {
	WithDecryptedTokenAtRevision(context.Context, string, string, string, func(string) error) error
}

type actionGraphClient interface {
	UpdateCampaignAction(context.Context, string, string, url.Values) (graphActionMutation, error)
	CreateCampaignAction(context.Context, string, string, url.Values) (graphActionCreate, error)
	CopyCampaignAction(context.Context, string, string, url.Values) (graphActionCopy, error)
	CreateAdSetAction(context.Context, string, string, url.Values) (graphActionCreate, error)
	CreateAdCreativeAction(context.Context, string, string, url.Values) (graphActionCreate, error)
	CreateAdAction(context.Context, string, string, url.Values) (graphActionCreate, error)
	GetCampaignAction(context.Context, string, string) (graphActionCampaign, error)
	ListPagesWithInstagram(context.Context, string) ([]GraphInstagramPage, error)
	ListInstagramMedia(context.Context, string, string, int) ([]GraphInstagramMedia, error)
}

func defaultActionExecutor(service *Service) ActionExecutor {
	if service == nil || !metaWritesEnabled() {
		return nil
	}
	return &graphActionExecutor{
		tokens: service.store, client: service.client, steps: service.store,
		instagramScopes: service.store,
	}
}

func metaWritesEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("META_ADS_WRITES_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (e *graphActionExecutor) Supports(action ActionKind) bool {
	if e == nil || e.tokens == nil || e.client == nil {
		return false
	}
	switch action {
	case ActionCreateCampaign, ActionDuplicateCampaign, ActionUpdateCampaign,
		ActionPauseCampaign, ActionResumeCampaign:
		return true
	case ActionPromoteInstagramPost:
		return e.steps != nil && e.instagramScopes != nil
	default:
		return false
	}
}

func (e *graphActionExecutor) Execute(
	ctx context.Context,
	proposal ActionProposal,
) (ActionExecutionOutcome, error) {
	createsFromAdAccount := proposal.Action == ActionCreateCampaign ||
		proposal.Action == ActionPromoteInstagramPost
	if !e.Supports(proposal.Action) ||
		(createsFromAdAccount && !metaCampaignIDPattern.MatchString(proposal.MetaAdAccountID)) ||
		(!createsFromAdAccount && !metaCampaignIDPattern.MatchString(proposal.TargetMetaCampaignID)) {
		return ActionExecutionOutcome{}, &ActionExecutorError{
			Code: "action_not_executable", Message: "Esta acao ainda nao possui executor Graph seguro.",
		}
	}
	connectionID, revision, err := claimedActionConnection(proposal)
	if err != nil {
		return ActionExecutionOutcome{}, err
	}
	if proposal.Action == ActionPromoteInstagramPost {
		return e.executePromoteInstagramPost(ctx, proposal, connectionID, revision)
	}
	values, desired, err := graphActionValues(proposal)
	if err != nil {
		return ActionExecutionOutcome{}, err
	}
	var outcome ActionExecutionOutcome
	err = e.tokens.WithDecryptedTokenAtRevision(
		ctx, proposal.ResourceAccountID, connectionID, revision,
		func(token string) error {
			if proposal.Action == ActionCreateCampaign {
				created, createErr := e.client.CreateCampaignAction(
					ctx, token, proposal.MetaAdAccountID, values,
				)
				if createErr != nil {
					return classifyGraphActionError(createErr, false)
				}
				if !metaCampaignIDPattern.MatchString(created.ID) {
					return unknownActionOutcome("A Meta respondeu sem um ID de campanha valido.")
				}
				result, marshalErr := mergeActionResult(desired, map[string]any{"campaignId": created.ID})
				if marshalErr != nil {
					return unknownActionOutcome("A campanha pode ter sido criada, mas o resultado nao foi persistido.")
				}
				outcome = ActionExecutionOutcome{Status: ActionSucceeded, ExternalEntityID: created.ID, Result: result}
				return nil
			}

			current, currentErr := e.client.GetCampaignAction(
				ctx, token, proposal.TargetMetaCampaignID,
			)
			if currentErr != nil {
				// O GET nao altera estado, mas sem preflight nao e seguro enviar POST.
				return classifyGraphActionError(currentErr, true)
			}
			alreadyApplied, currentResult, preflightErr := preflightGraphAction(proposal, current)
			if preflightErr != nil {
				return preflightErr
			}
			if alreadyApplied {
				outcome = ActionExecutionOutcome{
					Status: ActionSucceeded, ExternalEntityID: current.ID,
					Result: currentResult,
				}
				return nil
			}
			if proposal.Action == ActionDuplicateCampaign {
				copied, copyErr := e.client.CopyCampaignAction(
					ctx, token, proposal.TargetMetaCampaignID, values,
				)
				if copyErr != nil {
					return classifyGraphActionError(copyErr, false)
				}
				if !metaCampaignIDPattern.MatchString(copied.CopiedCampaignID) {
					return unknownActionOutcome("A Meta respondeu sem o ID da campanha duplicada.")
				}
				result, marshalErr := mergeActionResult(desired, map[string]any{
					"campaignId":       copied.CopiedCampaignID,
					"sourceCampaignId": proposal.TargetMetaCampaignID,
				})
				if marshalErr != nil {
					return unknownActionOutcome("A copia pode ter sido criada, mas o resultado nao foi persistido.")
				}
				outcome = ActionExecutionOutcome{
					Status: ActionSucceeded, ExternalEntityID: copied.CopiedCampaignID, Result: result,
				}
				return nil
			}

			mutation, mutationErr := e.client.UpdateCampaignAction(
				ctx, token, proposal.TargetMetaCampaignID, values,
			)
			if mutationErr != nil {
				return classifyGraphActionError(mutationErr, false)
			}
			if !mutation.Success {
				return &ActionExecutorError{
					Code:      "execution_outcome_unknown",
					Message:   "A Meta respondeu sem confirmar que a alteracao foi aplicada.",
					Ambiguous: true,
				}
			}
			outcome = ActionExecutionOutcome{
				Status: ActionSucceeded, ExternalEntityID: proposal.TargetMetaCampaignID,
				Result: desired,
			}
			return nil
		},
	)
	if err != nil {
		var executorErr *ActionExecutorError
		if errors.As(err, &executorErr) {
			return ActionExecutionOutcome{}, executorErr
		}
		if errors.Is(err, ErrConnectionChanged) {
			return ActionExecutionOutcome{}, &ActionExecutorError{
				Code:    "connection_revision_stale",
				Message: "A conexao Meta mudou ou expirou antes da execucao. Prepare uma nova proposta.",
			}
		}
		return ActionExecutionOutcome{}, &ActionExecutorError{
			Code: "execution_outcome_unknown", Message: "Nao foi possivel confirmar o resultado da Meta.", Ambiguous: true,
		}
	}
	return outcome, nil
}

func (e *graphActionExecutor) Reconcile(
	ctx context.Context,
	proposal ActionProposal,
) (ActionExecutionOutcome, error) {
	if !e.Supports(proposal.Action) {
		return ActionExecutionOutcome{}, &ActionExecutorError{
			Code: "action_not_executable", Message: "Esta acao ainda nao possui reconciliacao Graph segura.",
		}
	}
	if proposal.Action == ActionCreateCampaign || proposal.Action == ActionDuplicateCampaign {
		return ActionExecutionOutcome{
			Status: ActionUnknown, ErrorCode: "creation_reconciliation_requires_review",
			ErrorMessage: "A Meta nao oferece uma chave idempotente de criacao que permita repetir com seguranca. Revise a conta no painel antes de preparar outra proposta.",
			Result:       json.RawMessage(`{}`),
		}, nil
	}
	if proposal.Action == ActionPromoteInstagramPost {
		return e.reconcilePromoteInstagramPost(ctx, proposal)
	}
	if !metaCampaignIDPattern.MatchString(proposal.TargetMetaCampaignID) {
		return ActionExecutionOutcome{}, &ActionExecutorError{
			Code: "action_not_executable", Message: "Esta acao ainda nao possui reconciliacao Graph segura.",
		}
	}
	connectionID, revision, err := claimedActionConnection(proposal)
	if err != nil {
		return ActionExecutionOutcome{}, err
	}
	if _, _, err := graphActionValues(proposal); err != nil {
		return ActionExecutionOutcome{}, err
	}
	var outcome ActionExecutionOutcome
	err = e.tokens.WithDecryptedTokenAtRevision(
		ctx, proposal.ResourceAccountID, connectionID, revision,
		func(token string) error {
			current, currentErr := e.client.GetCampaignAction(
				ctx, token, proposal.TargetMetaCampaignID,
			)
			if currentErr != nil {
				return classifyGraphActionError(currentErr, true)
			}
			matched, matchErr := graphActionMatches(proposal, current)
			if matchErr != nil {
				return matchErr
			}
			result, marshalErr := graphActionCurrentResult(current, false)
			if marshalErr != nil {
				return marshalErr
			}
			if matched {
				outcome = ActionExecutionOutcome{
					Status: ActionSucceeded, ExternalEntityID: current.ID, Result: result,
				}
				return nil
			}
			outcome = ActionExecutionOutcome{
				Status: ActionUnknown, ExternalEntityID: current.ID, Result: result,
				ErrorCode:    "reconciliation_mismatch",
				ErrorMessage: "O estado atual da campanha nao comprova que a acao foi aplicada.",
			}
			return nil
		},
	)
	if err != nil {
		var executorErr *ActionExecutorError
		if errors.As(err, &executorErr) {
			return ActionExecutionOutcome{}, executorErr
		}
		return ActionExecutionOutcome{}, &ActionExecutorError{
			Code: "reconciliation_unavailable", Message: "A conexao Meta nao esta disponivel para reconciliar a acao.", Ambiguous: true,
		}
	}
	return outcome, nil
}

func claimedActionConnection(proposal ActionProposal) (string, string, error) {
	if proposal.GuardSnapshotVersion != actionGuardSnapshotVersion ||
		proposal.ConnectionIDSnapshot == nil || proposal.ConnectionRevisionSnapshot == nil ||
		proposal.ClaimedConnectionID == nil || proposal.ClaimedConnectionRevision == nil ||
		strings.TrimSpace(*proposal.ClaimedConnectionID) == "" ||
		strings.TrimSpace(*proposal.ClaimedConnectionRevision) == "" ||
		*proposal.ClaimedConnectionID != *proposal.ConnectionIDSnapshot ||
		*proposal.ClaimedConnectionRevision != *proposal.ConnectionRevisionSnapshot {
		return "", "", &ActionExecutorError{
			Code: "proposal_claim_stale", Message: "A proposta nao possui um claim de conexao valido.",
		}
	}
	return *proposal.ClaimedConnectionID, *proposal.ClaimedConnectionRevision, nil
}

func graphActionValues(proposal ActionProposal) (url.Values, json.RawMessage, error) {
	values := url.Values{}
	desired := map[string]any{"campaignId": proposal.TargetMetaCampaignID}
	switch proposal.Action {
	case ActionCreateCampaign:
		var payload createCampaignActionPayload
		if err := decodeStrictActionJSON(proposal.Payload, &payload); err != nil {
			return nil, nil, invalidPersistedActionPayload()
		}
		values.Set("name", payload.Name)
		values.Set("objective", payload.Objective)
		values.Set("special_ad_categories", mustActionJSON(payload.SpecialAdCategories))
		values.Set("status", "PAUSED")
		desired = map[string]any{
			"name": payload.Name, "objective": payload.Objective,
			"specialAdCategories": payload.SpecialAdCategories, "status": "PAUSED",
		}
		if err := addGraphActionBudget(values, desired, proposal, payload.Budget); err != nil {
			return nil, nil, err
		}
	case ActionDuplicateCampaign:
		var payload duplicateCampaignActionPayload
		if err := decodeStrictActionJSON(proposal.Payload, &payload); err != nil {
			return nil, nil, invalidPersistedActionPayload()
		}
		values.Set("deep_copy", "true")
		values.Set("status_option", "PAUSED")
		values.Set("rename_options", mustActionJSON(map[string]string{"rename_strategy": "NO_RENAME"}))
		values.Set("parameter_overrides", mustActionJSON(map[string]string{"name": payload.Name}))
		desired = map[string]any{
			"sourceCampaignId": proposal.TargetMetaCampaignID,
			"requestedName":    payload.Name, "status": "PAUSED", "deepCopy": true,
		}
	case ActionPauseCampaign:
		values.Set("status", "PAUSED")
		desired["status"] = "PAUSED"
	case ActionResumeCampaign:
		values.Set("status", "ACTIVE")
		desired["status"] = "ACTIVE"
	case ActionUpdateCampaign:
		var payload updateCampaignActionPayload
		if err := decodeStrictActionJSON(proposal.Payload, &payload); err != nil {
			return nil, nil, invalidPersistedActionPayload()
		}
		if payload.Name != "" {
			values.Set("name", payload.Name)
			desired["name"] = payload.Name
		}
		if err := addGraphActionBudget(values, desired, proposal, payload.Budget); err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, &ActionExecutorError{Code: "action_not_executable", Message: "Esta acao ainda nao possui executor Graph seguro."}
	}
	result, err := json.Marshal(desired)
	if err != nil {
		return nil, nil, err
	}
	return values, result, nil
}

func preflightGraphAction(
	proposal ActionProposal,
	current graphActionCampaign,
) (bool, json.RawMessage, error) {
	if current.ID != proposal.TargetMetaCampaignID {
		return false, nil, staleLiveActionTarget("A Meta retornou uma campanha diferente da proposta.")
	}
	currentStatus := strings.ToUpper(strings.TrimSpace(current.ConfiguredStatus))
	snapshotStatus := strings.ToUpper(strings.TrimSpace(proposal.CampaignStatusSnapshot))

	switch proposal.Action {
	case ActionDuplicateCampaign:
		if currentStatus == "" || snapshotStatus == "" || currentStatus != snapshotStatus ||
			current.Name != proposal.CampaignNameSnapshot || !liveCampaignBudgetsMatchSnapshot(proposal, current) {
			return false, nil, staleLiveActionTarget("A campanha original mudou desde a proposta de duplicacao.")
		}
		return false, nil, nil
	case ActionPauseCampaign:
		if currentStatus == "PAUSED" {
			result, err := graphActionCurrentResult(current, true)
			return true, result, err
		}
		if currentStatus == "" || snapshotStatus == "" ||
			currentStatus != snapshotStatus || currentStatus != "ACTIVE" {
			return false, nil, staleLiveActionTarget("O status da campanha mudou desde a proposta.")
		}
		return false, nil, nil

	case ActionUpdateCampaign:
		if currentStatus == "" || snapshotStatus == "" || currentStatus != snapshotStatus {
			return false, nil, staleLiveActionTarget("O status da campanha mudou desde a proposta.")
		}
		var payload updateCampaignActionPayload
		if err := decodeStrictActionJSON(proposal.Payload, &payload); err != nil {
			return false, nil, &ActionExecutorError{Code: "invalid_persisted_payload", Message: "A proposta persistida nao e valida."}
		}
		needsMutation := false
		if payload.Name != "" && current.Name != payload.Name {
			if current.Name != proposal.CampaignNameSnapshot {
				return false, nil, staleLiveActionTarget("O nome da campanha mudou desde a proposta.")
			}
			needsMutation = true
		}
		if payload.Budget != nil {
			actualRaw := current.DailyBudget
			snapshotBudget := proposal.CampaignDailySnapshot
			if payload.Budget.Type == "lifetime" {
				actualRaw = current.LifetimeBudget
				snapshotBudget = proposal.CampaignLifetimeSnapshot
			}
			actualMinor, parseErr := strconv.ParseInt(strings.TrimSpace(actualRaw), 10, 64)
			if parseErr != nil {
				return false, nil, staleLiveActionTarget("A Meta nao informou o orcamento atual da campanha.")
			}
			desiredMinor := budgetMinorUnits(payload.Budget.Amount)
			if actualMinor != desiredMinor {
				if snapshotBudget == nil || actualMinor != budgetMinorUnits(*snapshotBudget) {
					return false, nil, staleLiveActionTarget("O orcamento da campanha mudou desde a proposta.")
				}
				needsMutation = true
			}
		}
		if !needsMutation {
			result, err := graphActionCurrentResult(current, true)
			return true, result, err
		}
		return false, nil, nil
	case ActionResumeCampaign:
		if currentStatus == "ACTIVE" {
			if err := validateLiveResumeBudget(proposal, current); err != nil {
				return false, nil, err
			}
			result, err := graphActionCurrentResult(current, true)
			return true, result, err
		}
		if currentStatus == "" || snapshotStatus == "" || currentStatus != snapshotStatus || currentStatus != "PAUSED" {
			return false, nil, staleLiveActionTarget("O status da campanha mudou desde a proposta de retomada.")
		}
		if err := validateLiveResumeBudget(proposal, current); err != nil {
			return false, nil, err
		}
		return false, nil, nil
	default:
		return false, nil, &ActionExecutorError{Code: "action_not_executable", Message: "Esta acao ainda nao possui executor Graph seguro."}
	}
}

func graphActionCurrentResult(current graphActionCampaign, alreadyApplied bool) (json.RawMessage, error) {
	return json.Marshal(map[string]any{
		"id": current.ID, "name": current.Name,
		"status":              current.ConfiguredStatus,
		"dailyBudgetMinor":    current.DailyBudget,
		"lifetimeBudgetMinor": current.LifetimeBudget,
		"alreadyApplied":      alreadyApplied,
	})
}

func staleLiveActionTarget(message string) *ActionExecutorError {
	return &ActionExecutorError{
		Code: "proposal_stale_live_target", Message: message,
	}
}

func graphActionMatches(proposal ActionProposal, current graphActionCampaign) (bool, error) {
	switch proposal.Action {
	case ActionPauseCampaign:
		return current.ConfiguredStatus == "PAUSED", nil
	case ActionUpdateCampaign:
		var payload updateCampaignActionPayload
		if err := decodeStrictActionJSON(proposal.Payload, &payload); err != nil {
			return false, &ActionExecutorError{Code: "invalid_persisted_payload", Message: "A proposta persistida nao e valida."}
		}
		if payload.Name != "" && current.Name != payload.Name {
			return false, nil
		}
		if payload.Budget != nil {
			actual := current.DailyBudget
			if payload.Budget.Type == "lifetime" {
				actual = current.LifetimeBudget
			}
			actualMinor, err := strconv.ParseInt(strings.TrimSpace(actual), 10, 64)
			if err != nil || actualMinor != budgetMinorUnits(payload.Budget.Amount) {
				return false, nil
			}
		}
		return true, nil
	case ActionResumeCampaign:
		if current.ConfiguredStatus != "ACTIVE" {
			return false, nil
		}
		return validateLiveResumeBudget(proposal, current) == nil, nil
	default:
		return false, errors.New("meta_ads: action cannot be reconciled")
	}
}

func addGraphActionBudget(
	values url.Values,
	desired map[string]any,
	proposal ActionProposal,
	budget *campaignBudgetPayload,
) error {
	if budget == nil {
		return nil
	}
	if strings.ToUpper(strings.TrimSpace(proposal.Currency)) != "BRL" ||
		strings.ToUpper(strings.TrimSpace(proposal.PolicyCurrencySnapshot)) != "BRL" {
		return &ActionExecutorError{
			Code:    "unsupported_budget_currency",
			Message: "Alteracoes de orcamento estao disponiveis apenas para contas em BRL nesta fase.",
		}
	}
	minor := strconv.FormatInt(budgetMinorUnits(budget.Amount), 10)
	values.Set(budget.Type+"_budget", minor)
	desired[budget.Type+"BudgetMinor"] = minor
	return nil
}

func validateLiveResumeBudget(proposal ActionProposal, current graphActionCampaign) error {
	type budgetCheck struct {
		kind     string
		raw      string
		snapshot *float64
		cap      *float64
	}
	checks := []budgetCheck{
		{kind: "diario", raw: current.DailyBudget, snapshot: proposal.CampaignDailySnapshot, cap: proposal.PolicyMaxDailySnapshot},
		{kind: "total", raw: current.LifetimeBudget, snapshot: proposal.CampaignLifetimeSnapshot, cap: proposal.PolicyMaxLifetimeSnapshot},
	}
	known := false
	for _, check := range checks {
		raw := strings.TrimSpace(check.raw)
		if raw == "" || raw == "0" {
			continue
		}
		known = true
		minor, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || check.snapshot == nil || check.cap == nil ||
			minor != budgetMinorUnits(*check.snapshot) || minor > budgetMinorUnits(*check.cap) {
			return staleLiveActionTarget("O orcamento " + check.kind + " atual nao corresponde ao snapshot aprovado ou excede o teto.")
		}
	}
	if !known {
		return &ActionExecutorError{
			Code:    "live_budget_unavailable",
			Message: "A retomada ficou bloqueada porque a Meta nao informou um orcamento CBO verificavel.",
		}
	}
	return nil
}

func liveCampaignBudgetsMatchSnapshot(proposal ActionProposal, current graphActionCampaign) bool {
	return graphBudgetMatchesSnapshot(current.DailyBudget, proposal.CampaignDailySnapshot) &&
		graphBudgetMatchesSnapshot(current.LifetimeBudget, proposal.CampaignLifetimeSnapshot)
}

func graphBudgetMatchesSnapshot(raw string, snapshot *float64) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return snapshot == nil || budgetMinorUnits(*snapshot) == 0
	}
	minor, err := strconv.ParseInt(raw, 10, 64)
	return err == nil && snapshot != nil && minor == budgetMinorUnits(*snapshot)
}

func mergeActionResult(base json.RawMessage, extra map[string]any) (json.RawMessage, error) {
	result := map[string]any{}
	if len(base) > 0 {
		if err := json.Unmarshal(base, &result); err != nil {
			return nil, err
		}
	}
	for key, value := range extra {
		result[key] = value
	}
	return json.Marshal(result)
}

func mustActionJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func invalidPersistedActionPayload() *ActionExecutorError {
	return &ActionExecutorError{Code: "invalid_persisted_payload", Message: "A proposta persistida nao e valida."}
}

func unknownActionOutcome(message string) *ActionExecutorError {
	return &ActionExecutorError{Code: "execution_outcome_unknown", Message: message, Ambiguous: true}
}
