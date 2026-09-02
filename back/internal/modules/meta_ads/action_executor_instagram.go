package metaads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

func (e *graphActionExecutor) executePromoteInstagramPost(
	ctx context.Context,
	proposal ActionProposal,
	connectionID, revision string,
) (ActionExecutionOutcome, error) {
	var payload promoteInstagramPostActionPayload
	if err := decodeStrictActionJSON(proposal.Payload, &payload); err != nil {
		return ActionExecutionOutcome{}, invalidPersistedActionPayload()
	}
	if strings.ToUpper(strings.TrimSpace(proposal.Currency)) != "BRL" || payload.Budget == nil ||
		payload.Budget.Type != "daily" {
		return ActionExecutionOutcome{}, &ActionExecutorError{
			Code: "unsupported_budget_currency", Message: "Promover post exige budget diario em BRL nesta fase.",
		}
	}

	ids := map[string]any{"instagramPostId": payload.InstagramPostID, "status": "PAUSED"}
	var final ActionExecutionOutcome
	err := e.tokens.WithDecryptedTokenAtRevision(
		ctx, proposal.ResourceAccountID, connectionID, revision,
		func(token string) error {
			if err := e.validateLiveInstagramPost(ctx, token, proposal, payload); err != nil {
				return err
			}

			campaignValues := url.Values{
				"name":                  {payload.Name},
				"objective":             {"OUTCOME_ENGAGEMENT"},
				"special_ad_categories": {mustActionJSON([]string{"NONE"})},
				"buying_type":           {"AUCTION"},
				"status":                {"PAUSED"},
			}
			campaignID, err := e.executeInstagramCreationStep(
				ctx, proposal, actionStepCampaign, campaignValues,
				func() (graphActionCreate, error) {
					return e.client.CreateCampaignAction(ctx, token, proposal.MetaAdAccountID, campaignValues)
				}, ids,
			)
			if err != nil {
				return err
			}
			ids["campaignId"] = campaignID

			targeting := map[string]any{
				"age_min": payload.AgeMin, "age_max": payload.AgeMax,
				"geo_locations":       map[string]any{"countries": payload.Countries},
				"publisher_platforms": []string{"instagram"},
			}
			adSetValues := url.Values{
				"campaign_id":       {campaignID},
				"name":              {payload.AdSetName},
				"optimization_goal": {"POST_ENGAGEMENT"},
				"billing_event":     {"IMPRESSIONS"},
				"bid_strategy":      {"LOWEST_COST_WITHOUT_CAP"},
				"daily_budget":      {strconv.FormatInt(budgetMinorUnits(payload.Budget.Amount), 10)},
				"targeting":         {mustActionJSON(targeting)},
				"status":            {"PAUSED"},
			}
			adSetID, err := e.executeInstagramCreationStep(
				ctx, proposal, actionStepAdSet, adSetValues,
				func() (graphActionCreate, error) {
					return e.client.CreateAdSetAction(ctx, token, proposal.MetaAdAccountID, adSetValues)
				}, ids,
			)
			if err != nil {
				return err
			}
			ids["adSetId"] = adSetID

			creativeValues := url.Values{
				"name":                      {payload.AdName + " - criativo"},
				"object_id":                 {payload.PageID},
				"source_instagram_media_id": {payload.InstagramPostID},
				"instagram_user_id":         {payload.IGUserID},
			}
			creativeID, err := e.executeInstagramCreationStep(
				ctx, proposal, actionStepCreative, creativeValues,
				func() (graphActionCreate, error) {
					return e.client.CreateAdCreativeAction(ctx, token, proposal.MetaAdAccountID, creativeValues)
				}, ids,
			)
			if err != nil {
				return err
			}
			ids["creativeId"] = creativeID

			adValues := url.Values{
				"name":     {payload.AdName},
				"adset_id": {adSetID},
				"creative": {mustActionJSON(map[string]string{"creative_id": creativeID})},
				"status":   {"PAUSED"},
			}
			adID, err := e.executeInstagramCreationStep(
				ctx, proposal, actionStepAd, adValues,
				func() (graphActionCreate, error) {
					return e.client.CreateAdAction(ctx, token, proposal.MetaAdAccountID, adValues)
				}, ids,
			)
			if err != nil {
				return err
			}
			ids["adId"] = adID
			result, marshalErr := json.Marshal(ids)
			if marshalErr != nil {
				return unknownInstagramTreeError("A arvore foi criada, mas o resultado local ficou inconclusivo.", ids)
			}
			final = ActionExecutionOutcome{Status: ActionSucceeded, ExternalEntityID: adID, Result: result}
			return nil
		},
	)
	if err != nil {
		var executorErr *ActionExecutorError
		if errors.As(err, &executorErr) {
			if len(executorErr.Result) == 0 {
				executorErr.Result, _ = json.Marshal(ids)
			}
			return ActionExecutionOutcome{}, executorErr
		}
		if errors.Is(err, ErrConnectionChanged) {
			return ActionExecutionOutcome{}, &ActionExecutorError{
				Code: "connection_revision_stale", Message: "A conexao Meta mudou ou expirou antes da execucao.",
			}
		}
		return ActionExecutionOutcome{}, unknownInstagramTreeError(
			"Nao foi possivel determinar o resultado completo da arvore de anuncio.", ids,
		)
	}
	return final, nil
}

func (e *graphActionExecutor) validateLiveInstagramPost(
	ctx context.Context,
	token string,
	proposal ActionProposal,
	payload promoteInstagramPostActionPayload,
) error {
	if err := e.instagramScopes.ValidateActionInstagramIdentityScope(
		ctx, proposal.ResourceAccountID, proposal.AccountID, proposal.AdAccountID,
		payload.IGUserID, payload.PageID, payload.ClientAccountID,
	); err != nil {
		return staleLiveActionTarget("O vinculo entre conta de anuncio, Page/Instagram e cliente mudou.")
	}
	pages, err := e.client.ListPagesWithInstagram(ctx, token)
	if err != nil {
		return classifyGraphActionError(err, true)
	}
	foundIdentity := false
	for _, page := range pages {
		if page.PageID == payload.PageID && page.IGUserID == payload.IGUserID {
			foundIdentity = true
			break
		}
	}
	if !foundIdentity {
		return staleLiveActionTarget("A Page/Instagram selecionada nao esta mais acessivel pela conexao.")
	}
	posts, err := e.client.ListInstagramMedia(
		ctx, token, payload.IGUserID, actionInstagramMediaLookupLimit,
	)
	if err != nil {
		return classifyGraphActionError(err, true)
	}
	for _, post := range posts {
		if strings.TrimSpace(post.ID) == payload.InstagramPostID {
			return nil
		}
	}
	return staleLiveActionTarget("O post selecionado nao esta mais no feed autoritativo recente.")
}

func (e *graphActionExecutor) executeInstagramCreationStep(
	ctx context.Context,
	proposal ActionProposal,
	step actionStepName,
	values url.Values,
	call func() (graphActionCreate, error),
	partial map[string]any,
) (string, error) {
	requestHash := actionStepRequestHash(step, values)
	receipt, started, err := e.steps.BeginActionStep(
		ctx, proposal.AccountID, proposal.ID, step, requestHash,
	)
	if err != nil {
		if errors.Is(err, ErrActionStepUncertain) {
			return "", unknownInstagramTreeError(
				"Uma etapa desta criacao ja foi iniciada e nao sera repetida automaticamente.", partial,
			)
		}
		return "", err
	}
	if !started {
		return receipt.ExternalEntityID, nil
	}
	created, callErr := call()
	if callErr != nil {
		executorErr := classifyGraphActionError(callErr, false)
		outcome := actionOutcomeFromError(executorErr)
		outcome.Result, _ = json.Marshal(partial)
		_, persistErr := e.steps.CompleteActionStep(
			ctx, proposal.AccountID, proposal.ID, step, requestHash, outcome,
		)
		if persistErr != nil {
			return "", unknownInstagramTreeError(
				"A Meta respondeu, mas o recibo local da etapa ficou inconclusivo.", partial,
			)
		}
		executorErr.Result = outcome.Result
		return "", executorErr
	}
	if !metaCampaignIDPattern.MatchString(created.ID) {
		outcome := actionOutcomeFromError(unknownInstagramTreeError(
			"A Meta respondeu sem um ID valido para uma etapa de criacao.", partial,
		))
		_, _ = e.steps.CompleteActionStep(
			ctx, proposal.AccountID, proposal.ID, step, requestHash, outcome,
		)
		return "", unknownInstagramTreeError(
			"A Meta respondeu sem um ID valido para uma etapa de criacao.", partial,
		)
	}
	stepResult := map[string]any{"id": created.ID}
	if step != actionStepCreative {
		stepResult["status"] = "PAUSED"
	}
	result, _ := json.Marshal(stepResult)
	_, err = e.steps.CompleteActionStep(
		ctx, proposal.AccountID, proposal.ID, step, requestHash,
		ActionExecutionOutcome{Status: ActionSucceeded, ExternalEntityID: created.ID, Result: result},
	)
	if err != nil {
		partial[string(step)+"Id"] = created.ID
		return "", unknownInstagramTreeError(
			"A etapa pode ter sido criada, mas o recibo local nao foi confirmado.", partial,
		)
	}
	return created.ID, nil
}

func (e *graphActionExecutor) reconcilePromoteInstagramPost(
	ctx context.Context,
	proposal ActionProposal,
) (ActionExecutionOutcome, error) {
	steps, err := e.steps.ListActionSteps(ctx, proposal.AccountID, proposal.ID)
	if err != nil {
		return ActionExecutionOutcome{}, &ActionExecutorError{
			Code: "reconciliation_unavailable", Message: "Nao foi possivel ler os recibos da arvore de anuncio.", Ambiguous: true,
		}
	}
	ids := map[string]any{}
	statusByStep := make(map[actionStepName]ActionStatus, len(steps))
	for _, step := range steps {
		statusByStep[step.Step] = step.Status
		if step.ExternalEntityID != "" {
			ids[string(step.Step)+"Id"] = step.ExternalEntityID
		}
		if step.Status == ActionFailed {
			result, _ := json.Marshal(ids)
			return ActionExecutionOutcome{
				Status: ActionFailed, Result: result, ErrorCode: step.ErrorCode,
				ErrorMessage: step.ErrorMessage,
			}, nil
		}
	}
	for _, expected := range []actionStepName{actionStepCampaign, actionStepAdSet, actionStepCreative, actionStepAd} {
		if statusByStep[expected] != ActionSucceeded {
			result, _ := json.Marshal(ids)
			return ActionExecutionOutcome{
				Status: ActionUnknown, Result: result,
				ErrorCode:    "creation_step_unconfirmed",
				ErrorMessage: "Nem todas as etapas possuem recibo de sucesso; nenhuma criacao sera repetida automaticamente.",
			}, nil
		}
	}
	adID, _ := ids["adId"].(string)
	result, _ := json.Marshal(ids)
	return ActionExecutionOutcome{Status: ActionSucceeded, ExternalEntityID: adID, Result: result}, nil
}

func actionStepRequestHash(step actionStepName, values url.Values) string {
	digest := sha256.Sum256([]byte(string(step) + "\n" + values.Encode()))
	return hex.EncodeToString(digest[:])
}

func unknownInstagramTreeError(message string, partial map[string]any) *ActionExecutorError {
	result, _ := json.Marshal(partial)
	return &ActionExecutorError{
		Code: "execution_outcome_unknown", Message: message, Ambiguous: true, Result: result,
	}
}
