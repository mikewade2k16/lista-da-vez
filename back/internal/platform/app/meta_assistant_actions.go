package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/calendar"
	metaads "github.com/mikewade2k16/lista-da-vez/back/internal/modules/meta_ads"
)

type metaAdsServiceProvider func() *metaads.Service

func metaAssistantActionProvider(serviceProvider metaAdsServiceProvider) calendar.MetaAssistantActionProvider {
	return func(
		ctx context.Context,
		req calendar.MetaAssistantActionRequest,
	) (calendar.MetaAssistantActionResult, error) {
		service := serviceProvider()
		if service == nil {
			return calendar.MetaAssistantActionResult{}, calendar.ErrAssistantRuntimeUnavailable
		}
		view, _, err := service.CreateProposalFromAssistant(ctx, req.AccountID, req.ActorUserID,
			metaads.AssistantActionProposalInput{
				ConversationID: req.ConversationID, MessageID: req.MessageID,
				ProposalIndex: req.ProposalIndex, AllowedAdAccountIDs: req.AllowedAdAccountIDs,
				Action: metaads.ActionKind(req.Action), AdAccountID: req.AdAccountID,
				Payload: req.Payload,
			})
		if err != nil {
			if code, message, expected := expectedMetaAssistantActionError(err); expected {
				return calendar.MetaAssistantActionResult{
					Action: req.Action, AdAccountID: req.AdAccountID,
					Status: "pending", ErrorCode: code, ErrorMessage: message,
				}, nil
			}
			return calendar.MetaAssistantActionResult{}, err
		}
		return calendarMetaActionResult(view), nil
	}
}

func metaAssistantActionStatusProvider(serviceProvider metaAdsServiceProvider) calendar.MetaAssistantActionStatusProvider {
	return func(
		ctx context.Context,
		accountID, proposalID string,
	) (calendar.MetaAssistantActionResult, error) {
		service := serviceProvider()
		if service == nil {
			return calendar.MetaAssistantActionResult{}, calendar.ErrAssistantRuntimeUnavailable
		}
		view, err := service.GetActionProposal(ctx, accountID, proposalID)
		if err != nil {
			return calendar.MetaAssistantActionResult{}, err
		}
		return calendarMetaActionResult(view), nil
	}
}

func metaAssistantActionBindProvider(serviceProvider metaAdsServiceProvider) calendar.MetaAssistantActionBindProvider {
	return func(ctx context.Context, req calendar.MetaAssistantActionLifecycleRequest) (calendar.MetaAssistantActionResult, error) {
		service := serviceProvider()
		if service == nil {
			return calendar.MetaAssistantActionResult{}, calendar.ErrAssistantRuntimeUnavailable
		}
		view, err := service.BindAssistantActionProposal(
			ctx, req.AccountID, req.ActorUserID, req.ActionProposalID,
			req.ConversationID, req.MessageID,
		)
		if err != nil {
			return calendar.MetaAssistantActionResult{}, err
		}
		return calendarMetaActionResult(view), nil
	}
}

func metaAssistantActionConfirmProvider(serviceProvider metaAdsServiceProvider) calendar.MetaAssistantActionConfirmProvider {
	return func(ctx context.Context, req calendar.MetaAssistantActionLifecycleRequest) (calendar.MetaAssistantActionResult, error) {
		service := serviceProvider()
		if service == nil {
			return calendar.MetaAssistantActionResult{}, calendar.ErrAssistantRuntimeUnavailable
		}
		view, err := service.ConfirmAssistantActionProposal(
			ctx, req.AccountID, req.ActorUserID, req.ActionProposalID,
			req.ConversationID, req.MessageID, req.IdempotencyKey,
			req.AcknowledgeSpend,
		)
		if err != nil {
			if code, message, expected := expectedMetaAssistantActionError(err); expected {
				current, statusErr := service.GetActionProposal(ctx, req.AccountID, req.ActionProposalID)
				if statusErr != nil {
					return calendar.MetaAssistantActionResult{}, statusErr
				}
				result := calendarMetaActionResult(current)
				result.ErrorCode = code
				result.ErrorMessage = message
				return result, nil
			}
			return calendar.MetaAssistantActionResult{}, err
		}
		return calendarMetaActionResult(view), nil
	}
}

func metaAssistantActionCancelProvider(serviceProvider metaAdsServiceProvider) calendar.MetaAssistantActionCancelProvider {
	return func(ctx context.Context, req calendar.MetaAssistantActionLifecycleRequest) (calendar.MetaAssistantActionResult, error) {
		service := serviceProvider()
		if service == nil {
			return calendar.MetaAssistantActionResult{}, calendar.ErrAssistantRuntimeUnavailable
		}
		view, err := service.CancelAssistantActionProposal(
			ctx, req.AccountID, req.ActorUserID, req.ActionProposalID,
			req.ConversationID, req.MessageID, req.IdempotencyKey,
		)
		if err != nil {
			return calendar.MetaAssistantActionResult{}, err
		}
		return calendarMetaActionResult(view), nil
	}
}

func metaAssistantConversationCancelProvider(serviceProvider metaAdsServiceProvider) calendar.MetaAssistantConversationCancelProvider {
	return func(ctx context.Context, accountID, actorUserID, conversationID string) error {
		service := serviceProvider()
		if service == nil {
			return calendar.ErrAssistantRuntimeUnavailable
		}
		_, err := service.CancelAssistantConversationActions(
			ctx, accountID, actorUserID, conversationID,
		)
		return err
	}
}

func calendarMetaActionResult(view metaads.ActionProposalView) calendar.MetaAssistantActionResult {
	targetCampaignID := ""
	if view.TargetCampaignID != nil {
		targetCampaignID = strings.TrimSpace(*view.TargetCampaignID)
	}
	return calendar.MetaAssistantActionResult{
		ID: view.ID, Action: string(view.Action), AdAccountID: view.AdAccountID,
		AdAccountName: view.AdAccountName, Currency: view.Currency,
		TargetCampaignID: targetCampaignID, Summary: view.Summary,
		Status: string(view.Status), ExecutionAvailable: view.ExecutionAvailable,
		CanConfirm:                   view.CanConfirm,
		RequiresSpendAcknowledgement: view.RequiresSpendAcknowledgement,
		ExpiresAt:                    view.ExpiresAt.UTC().Format(time.RFC3339),
		ErrorCode:                    view.ErrorCode, ErrorMessage: view.ErrorMessage,
	}
}

func expectedMetaAssistantActionError(err error) (string, string, bool) {
	switch {
	case errors.Is(err, metaads.ErrActionPolicyRequired):
		return "action_policy_required", "Configure os limites financeiros desta conta de anuncio antes de confirmar a acao.", true
	case errors.Is(err, metaads.ErrActionPolicyDenied):
		return "action_not_allowed", "A politica financeira bloqueia esta acao.", true
	case errors.Is(err, metaads.ErrActionBudgetCapExceeded):
		return "budget_cap_exceeded", "O orcamento excede o teto configurado para esta conta de anuncio.", true
	case errors.Is(err, metaads.ErrActionBudgetUnavailable):
		return "budget_state_unavailable", "O orcamento atual nao esta disponivel para validar a acao com seguranca.", true
	case errors.Is(err, metaads.ErrActionReinforcedConfirm):
		return "reinforced_confirmation_required", "Esta acao financeira exige a confirmacao explicita de gasto.", true
	case errors.Is(err, metaads.ErrActionSourceUnbound):
		return "assistant_source_unbound", "O card desta proposta nao existe mais ou ainda nao foi vinculado.", true
	case errors.Is(err, metaads.ErrActionNotCancellable):
		return "action_not_cancellable", "A acao ja foi iniciada ou concluida e nao pode ser cancelada.", true
	case errors.Is(err, metaads.ErrActionExpired):
		return "action_expired", "A proposta expirou. Prepare uma nova acao pelo assistente.", true
	case errors.Is(err, metaads.ErrMetaWritesDisabled):
		return "meta_writes_disabled", "As escritas Meta Ads estao desabilitadas no servidor.", true
	case errors.Is(err, metaads.ErrMetaActionUnavailable):
		return "action_unavailable", "Esta acao ainda nao possui executor Graph seguro.", true
	case errors.Is(err, metaads.ErrActionIdempotencyConflict):
		return "idempotency_conflict", "Esta proposta conflita com uma tentativa anterior e nao sera executada.", true
	case errors.Is(err, metaads.ErrActionValidation):
		return "invalid_action_proposal", "A proposta nao passou pela validacao segura do Meta Ads.", true
	case errors.Is(err, metaads.ErrNotConnected):
		return "not_connected", "Conecte uma conta Meta antes de preparar esta acao.", true
	case errors.Is(err, pgx.ErrNoRows):
		return "resource_unavailable", "A conta ou campanha nao esta mais disponivel neste escopo.", true
	default:
		return "", "", false
	}
}
