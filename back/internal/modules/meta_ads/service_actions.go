package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type actionScopeResolver interface {
	ResolveActionAdAccount(context.Context, string, string) (AdAccount, error)
	ResolveActionCampaign(context.Context, string, string, string) (Campaign, error)
}

type actionInstagramScopeResolver interface {
	ValidatePromotableInstagramPost(context.Context, string, string, json.RawMessage) error
}

// AssistantActionSourceValidator confirma no owner do chat que a conversa e o
// card ainda existem e continuam pending. Propostas manuais nao passam por ela.
type AssistantActionSourceValidator func(
	context.Context, string, string, string, string,
) error

// ActionService concentra proposta, confirmacao e reconciliacao. O browser
// nunca chama a Graph diretamente; o executor e uma dependencia explicita e
// pode permanecer nil para fechar todas as escritas.
type ActionService struct {
	repository               actionRepository
	scope                    actionScopeResolver
	executor                 ActionExecutor
	sync                     func(context.Context, string, string) error
	assistantSourceValidator AssistantActionSourceValidator
}

func NewActionService(service *Service, executor ActionExecutor) *ActionService {
	if service == nil {
		return &ActionService{executor: executor}
	}
	return &ActionService{
		repository:               service.store,
		scope:                    service,
		executor:                 executor,
		assistantSourceValidator: service.assistantActionSourceValidator,
		sync: func(ctx context.Context, accountID, adAccountID string) error {
			_, err := service.Sync(ctx, accountID, adAccountID)
			return err
		},
	}
}

// ResolveActionAdAccount repete o ownership/vinculo usado pelos relatorios.
func (s *Service) ResolveActionAdAccount(ctx context.Context, accountID, adAccountID string) (AdAccount, error) {
	return s.requireAdAccount(ctx, accountID, adAccountID)
}

// ResolveActionCampaign exige account viewer + ad account autorizada + campanha
// cacheada nessa mesma ad account. ID/meta ID do modelo nunca vencem o cache.
func (s *Service) ResolveActionCampaign(
	ctx context.Context,
	accountID, adAccountID, campaignID string,
) (Campaign, error) {
	adAccount, err := s.requireAdAccount(ctx, accountID, adAccountID)
	if err != nil {
		return Campaign{}, err
	}
	return s.store.GetCampaignForAction(ctx, adAccount.AccountID, adAccount.ID, campaignID)
}

func (s *ActionService) CreateProposalFromUser(
	ctx context.Context,
	accountID, actorUserID, idempotencyKey string,
	input ActionProposalInput,
) (ActionProposalView, bool, error) {
	return s.createProposal(ctx, accountID, actorUserID, idempotencyKey,
		ActionSourceManual, "", "", input)
}

// CreateProposalFromAssistant e o caminho interno para linguagem natural. O
// caller owner do chat fornece a allowlist ja filtrada e refs de conversa; o
// modelo nao escolhe account, user nem chave idempotente.
func (s *ActionService) CreateProposalFromAssistant(
	ctx context.Context,
	accountID, actorUserID string,
	input AssistantActionProposalInput,
) (ActionProposalView, bool, error) {
	conversationID := strings.TrimSpace(input.ConversationID)
	messageID := strings.TrimSpace(input.MessageID)
	adAccountID := strings.TrimSpace(input.AdAccountID)
	if !metaAdsUUIDRe.MatchString(conversationID) || !metaAdsUUIDRe.MatchString(messageID) ||
		input.ProposalIndex < 0 || input.ProposalIndex > 20 ||
		!actionAdAccountAllowed(input.AllowedAdAccountIDs, adAccountID) {
		return ActionProposalView{}, false, ErrActionValidation
	}
	idempotencyKey := "assistant:" + messageID + ":" + strconv.Itoa(input.ProposalIndex)
	return s.createProposal(ctx, accountID, actorUserID, idempotencyKey,
		ActionSourceAssistant, conversationID, messageID, ActionProposalInput{
			Action: input.Action, AdAccountID: adAccountID, Payload: input.Payload,
		})
}

// CreateProposalFromAssistant deixa o contrato acessivel pela facade publica do
// modulo sem importar o Store no composition root.
func (s *Service) CreateProposalFromAssistant(
	ctx context.Context,
	accountID, actorUserID string,
	input AssistantActionProposalInput,
) (ActionProposalView, bool, error) {
	return NewActionService(s, defaultActionExecutor(s)).CreateProposalFromAssistant(
		ctx, accountID, actorUserID, input,
	)
}

func (s *ActionService) BindAssistantProposal(
	ctx context.Context,
	accountID, actorUserID, proposalID, conversationID, messageID string,
) (ActionProposalView, error) {
	accountID = strings.TrimSpace(accountID)
	actorUserID = strings.TrimSpace(actorUserID)
	proposalID = strings.TrimSpace(proposalID)
	conversationID = strings.TrimSpace(conversationID)
	messageID = strings.TrimSpace(messageID)
	if s.repository == nil || s.assistantSourceValidator == nil ||
		!metaAdsUUIDRe.MatchString(accountID) || !metaAdsUUIDRe.MatchString(actorUserID) ||
		!metaAdsUUIDRe.MatchString(proposalID) || !metaAdsUUIDRe.MatchString(conversationID) ||
		!metaAdsUUIDRe.MatchString(messageID) {
		return ActionProposalView{}, ErrActionSourceUnbound
	}
	if err := s.assistantSourceValidator(ctx, accountID, conversationID, messageID, proposalID); err != nil {
		return ActionProposalView{}, ErrActionSourceUnbound
	}
	proposal, err := s.repository.BindAssistantActionProposal(
		ctx, accountID, proposalID, conversationID, messageID, actorUserID,
	)
	if err != nil {
		return ActionProposalView{}, err
	}
	return s.toActionProposalView(proposal), nil
}

func (s *Service) BindAssistantActionProposal(
	ctx context.Context,
	accountID, actorUserID, proposalID, conversationID, messageID string,
) (ActionProposalView, error) {
	return NewActionService(s, defaultActionExecutor(s)).BindAssistantProposal(
		ctx, accountID, actorUserID, proposalID, conversationID, messageID,
	)
}

func (s *ActionService) ConfirmAssistantProposal(
	ctx context.Context,
	accountID, actorUserID, proposalID, conversationID, messageID, confirmationKey string,
	acknowledgeSpend bool,
) (ActionProposalView, error) {
	proposal, err := s.getProposal(ctx, accountID, proposalID)
	if err != nil {
		return ActionProposalView{}, err
	}
	if proposal.Source != ActionSourceAssistant || proposal.SourceConversationID == nil ||
		proposal.SourceMessageID == nil || *proposal.SourceConversationID != strings.TrimSpace(conversationID) ||
		*proposal.SourceMessageID != strings.TrimSpace(messageID) {
		return ActionProposalView{}, ErrActionValidation
	}
	return s.ConfirmProposal(
		ctx, accountID, actorUserID, proposalID, confirmationKey, acknowledgeSpend,
	)
}

func (s *Service) ConfirmAssistantActionProposal(
	ctx context.Context,
	accountID, actorUserID, proposalID, conversationID, messageID, confirmationKey string,
	acknowledgeSpend bool,
) (ActionProposalView, error) {
	return NewActionService(s, defaultActionExecutor(s)).ConfirmAssistantProposal(
		ctx, accountID, actorUserID, proposalID, conversationID, messageID,
		confirmationKey, acknowledgeSpend,
	)
}

func (s *ActionService) createProposal(
	ctx context.Context,
	accountID, actorUserID, idempotencyKey string,
	source ActionProposalSource,
	conversationID, messageID string,
	input ActionProposalInput,
) (ActionProposalView, bool, error) {
	accountID = strings.TrimSpace(accountID)
	actorUserID = strings.TrimSpace(actorUserID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	input.AdAccountID = strings.TrimSpace(input.AdAccountID)
	if s.repository == nil || s.scope == nil || !metaAdsUUIDRe.MatchString(accountID) ||
		!metaAdsUUIDRe.MatchString(actorUserID) || !metaAdsUUIDRe.MatchString(input.AdAccountID) ||
		!validActionIdempotencyKey(idempotencyKey) {
		return ActionProposalView{}, false, ErrActionValidation
	}
	normalized, err := normalizeActionPayload(input.Action, input.Payload)
	if err != nil {
		return ActionProposalView{}, false, err
	}
	adAccount, err := s.scope.ResolveActionAdAccount(ctx, accountID, input.AdAccountID)
	if err != nil {
		return ActionProposalView{}, false, err
	}
	adAccount.Currency = strings.ToUpper(strings.TrimSpace(adAccount.Currency))
	if len(adAccount.Currency) != 3 {
		return ActionProposalView{}, false, ErrActionValidation
	}
	if err := validateActionBudgetCurrency(input.Action, normalized, adAccount.Currency); err != nil {
		return ActionProposalView{}, false, err
	}
	var target *Campaign
	if normalized.TargetCampaignID != "" {
		campaign, resolveErr := s.scope.ResolveActionCampaign(
			ctx, accountID, adAccount.ID, normalized.TargetCampaignID,
		)
		if resolveErr != nil {
			return ActionProposalView{}, false, resolveErr
		}
		target = &campaign
	}
	policy, err := s.optionalActionPolicy(ctx, adAccount)
	if err != nil {
		return ActionProposalView{}, false, err
	}
	if err := validateActionAgainstPolicy(input.Action, normalized, target, policy); err != nil {
		return ActionProposalView{}, false, err
	}
	if input.Action == ActionPromoteInstagramPost {
		instagramScope, ok := s.scope.(actionInstagramScopeResolver)
		if !ok {
			return ActionProposalView{}, false, ErrMetaActionUnavailable
		}
		if err := instagramScope.ValidatePromotableInstagramPost(
			ctx, accountID, adAccount.ID, normalized.Raw,
		); err != nil {
			return ActionProposalView{}, false, err
		}
	}
	summary := buildActionSummary(input.Action, normalized, target, adAccount)
	proposal, created, err := s.repository.CreateActionProposal(ctx, actionProposalInsert{
		AccountID: accountID, ResourceAccountID: adAccount.AccountID,
		AdAccount: adAccount, Action: input.Action, Source: source,
		SourceConversationID: conversationID, SourceMessageID: messageID,
		TargetCampaign: target, Payload: normalized.Raw, Summary: summary,
		RequestHash:    actionRequestHash(input.Action, adAccount.ID, normalized.Raw),
		IdempotencyKey: idempotencyKey, CreatedByUserID: actorUserID,
	})
	if err != nil {
		return ActionProposalView{}, false, err
	}
	return s.toActionProposalView(proposal), created, nil
}

func (s *ActionService) ListProposals(ctx context.Context, accountID string, limit int) ([]ActionProposalView, error) {
	accountID = strings.TrimSpace(accountID)
	if s.repository == nil || !metaAdsUUIDRe.MatchString(accountID) {
		return nil, ErrActionValidation
	}
	if limit <= 0 {
		limit = 50
	}
	limit = min(limit, 100)
	rows, err := s.repository.ListActionProposals(ctx, accountID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]ActionProposalView, len(rows))
	for index := range rows {
		out[index] = s.toActionProposalView(rows[index])
	}
	return out, nil
}

func (s *ActionService) GetProposal(ctx context.Context, accountID, proposalID string) (ActionProposalView, error) {
	proposal, err := s.getProposal(ctx, accountID, proposalID)
	if err != nil {
		return ActionProposalView{}, err
	}
	if proposal.Status == ActionPending {
		proposal, _, err = s.repository.ExpireActionProposal(ctx, proposal.AccountID, proposal.ID)
		if err != nil {
			return ActionProposalView{}, err
		}
	}
	return s.toActionProposalView(proposal), nil
}

func (s *Service) GetActionProposal(
	ctx context.Context,
	accountID, proposalID string,
) (ActionProposalView, error) {
	return NewActionService(s, defaultActionExecutor(s)).GetProposal(ctx, accountID, proposalID)
}

func (s *ActionService) ConfirmProposal(
	ctx context.Context,
	accountID, actorUserID, proposalID, confirmationKey string,
	acknowledgeSpend bool,
) (ActionProposalView, error) {
	if !validActionIdempotencyKey(confirmationKey) || !metaAdsUUIDRe.MatchString(strings.TrimSpace(actorUserID)) {
		return ActionProposalView{}, ErrActionValidation
	}
	proposal, err := s.getProposal(ctx, accountID, proposalID)
	if err != nil {
		return ActionProposalView{}, err
	}
	if proposal.Status != ActionPending {
		return s.toActionProposalView(proposal), nil
	}
	if proposal.Status == ActionPending {
		proposal, _, err = s.repository.ExpireActionProposal(ctx, proposal.AccountID, proposal.ID)
		if err != nil {
			return ActionProposalView{}, err
		}
		if proposal.Status == ActionExpired {
			return ActionProposalView{}, ErrActionExpired
		}
	}
	if proposal.Source == ActionSourceAssistant {
		if !proposal.SourceBound || proposal.SourceConversationID == nil ||
			proposal.SourceMessageID == nil || s.assistantSourceValidator == nil {
			return ActionProposalView{}, ErrActionSourceUnbound
		}
		if err := s.assistantSourceValidator(
			ctx, proposal.AccountID, *proposal.SourceConversationID,
			*proposal.SourceMessageID, proposal.ID,
		); err != nil {
			return ActionProposalView{}, ErrActionSourceUnbound
		}
	}
	if actionRequiresSpendAcknowledgement(proposal.Action, proposal.Payload) && !acknowledgeSpend {
		return ActionProposalView{}, ErrActionReinforcedConfirm
	}
	if s.executor == nil {
		return ActionProposalView{}, ErrMetaWritesDisabled
	}
	if !s.executor.Supports(proposal.Action) {
		return ActionProposalView{}, ErrMetaActionUnavailable
	}
	if err := s.revalidateProposal(ctx, proposal); err != nil {
		return ActionProposalView{}, err
	}
	proposal, started, err := s.repository.BeginActionExecution(
		ctx, proposal.AccountID, proposal.ID, strings.TrimSpace(actorUserID), strings.TrimSpace(confirmationKey),
		acknowledgeSpend,
	)
	if err != nil {
		return ActionProposalView{}, err
	}
	if !started {
		return s.toActionProposalView(proposal), nil
	}

	outcome, executionErr := s.executor.Execute(ctx, proposal)
	if executionErr != nil {
		outcome = actionOutcomeFromError(executionErr)
	} else {
		outcome = normalizeActionOutcome(outcome)
	}
	persistCtx, cancelPersist := actionPersistenceContext(ctx)
	defer cancelPersist()
	proposal, err = s.repository.CompleteActionExecution(
		persistCtx, proposal.AccountID, proposal.ID, outcome,
	)
	if err != nil {
		return ActionProposalView{}, err
	}
	if proposal.Status == ActionSucceeded && s.sync != nil {
		_ = s.sync(ctx, proposal.AccountID, proposal.AdAccountID)
	}
	return s.toActionProposalView(proposal), nil
}

func (s *ActionService) CancelProposal(
	ctx context.Context,
	accountID, actorUserID, proposalID, cancellationKey string,
) (ActionProposalView, error) {
	accountID = strings.TrimSpace(accountID)
	actorUserID = strings.TrimSpace(actorUserID)
	proposalID = strings.TrimSpace(proposalID)
	cancellationKey = strings.TrimSpace(cancellationKey)
	if s.repository == nil || !metaAdsUUIDRe.MatchString(accountID) ||
		!metaAdsUUIDRe.MatchString(actorUserID) || !metaAdsUUIDRe.MatchString(proposalID) ||
		!validActionIdempotencyKey(cancellationKey) {
		return ActionProposalView{}, ErrActionValidation
	}
	proposal, _, err := s.repository.CancelActionProposal(
		ctx, accountID, proposalID, actorUserID, cancellationKey,
	)
	if err != nil {
		return ActionProposalView{}, err
	}
	return s.toActionProposalView(proposal), nil
}

func (s *ActionService) CancelAssistantProposal(
	ctx context.Context,
	accountID, actorUserID, proposalID, conversationID, messageID, cancellationKey string,
) (ActionProposalView, error) {
	proposal, err := s.getProposal(ctx, accountID, proposalID)
	if err != nil {
		return ActionProposalView{}, err
	}
	if proposal.Source != ActionSourceAssistant || proposal.SourceConversationID == nil ||
		proposal.SourceMessageID == nil || *proposal.SourceConversationID != strings.TrimSpace(conversationID) ||
		*proposal.SourceMessageID != strings.TrimSpace(messageID) {
		return ActionProposalView{}, ErrActionValidation
	}
	if s.assistantSourceValidator == nil {
		return ActionProposalView{}, ErrActionSourceUnbound
	}
	if err := s.assistantSourceValidator(
		ctx, proposal.AccountID, *proposal.SourceConversationID,
		*proposal.SourceMessageID, proposal.ID,
	); err != nil {
		return ActionProposalView{}, ErrActionSourceUnbound
	}
	return s.CancelProposal(ctx, accountID, actorUserID, proposalID, cancellationKey)
}

func (s *Service) CancelAssistantActionProposal(
	ctx context.Context,
	accountID, actorUserID, proposalID, conversationID, messageID, cancellationKey string,
) (ActionProposalView, error) {
	return NewActionService(s, defaultActionExecutor(s)).CancelAssistantProposal(
		ctx, accountID, actorUserID, proposalID, conversationID, messageID, cancellationKey,
	)
}

func (s *ActionService) CancelAssistantConversation(
	ctx context.Context,
	accountID, actorUserID, conversationID string,
) (int, error) {
	accountID = strings.TrimSpace(accountID)
	actorUserID = strings.TrimSpace(actorUserID)
	conversationID = strings.TrimSpace(conversationID)
	if s.repository == nil || !metaAdsUUIDRe.MatchString(accountID) ||
		!metaAdsUUIDRe.MatchString(actorUserID) || !metaAdsUUIDRe.MatchString(conversationID) {
		return 0, ErrActionValidation
	}
	return s.repository.CancelAssistantConversationActions(ctx, accountID, conversationID, actorUserID)
}

func (s *Service) CancelAssistantConversationActions(
	ctx context.Context,
	accountID, actorUserID, conversationID string,
) (int, error) {
	return NewActionService(s, defaultActionExecutor(s)).CancelAssistantConversation(
		ctx, accountID, actorUserID, conversationID,
	)
}

func (s *ActionService) ReconcileProposal(
	ctx context.Context,
	accountID, actorUserID, proposalID string,
) (ActionProposalView, error) {
	if !metaAdsUUIDRe.MatchString(strings.TrimSpace(actorUserID)) {
		return ActionProposalView{}, ErrActionValidation
	}
	proposal, err := s.getProposal(ctx, accountID, proposalID)
	if err != nil {
		return ActionProposalView{}, err
	}
	if proposal.Status != ActionExecuting && proposal.Status != ActionUnknown {
		return s.toActionProposalView(proposal), nil
	}
	if s.executor == nil {
		return ActionProposalView{}, ErrMetaWritesDisabled
	}
	if !s.executor.Supports(proposal.Action) {
		return ActionProposalView{}, ErrMetaActionUnavailable
	}
	if _, _, err := s.revalidateProposalAccess(ctx, proposal); err != nil {
		return ActionProposalView{}, err
	}
	outcome, reconcileErr := s.executor.Reconcile(ctx, proposal)
	if reconcileErr != nil {
		outcome = actionOutcomeFromError(reconcileErr)
	} else {
		outcome = normalizeActionOutcome(outcome)
	}
	persistCtx, cancelPersist := actionPersistenceContext(ctx)
	defer cancelPersist()
	proposal, err = s.repository.ReconcileActionExecution(
		persistCtx, proposal.AccountID, proposal.ID, strings.TrimSpace(actorUserID), outcome,
	)
	if err != nil {
		return ActionProposalView{}, err
	}
	if proposal.Status == ActionSucceeded && s.sync != nil {
		_ = s.sync(ctx, proposal.AccountID, proposal.AdAccountID)
	}
	return s.toActionProposalView(proposal), nil
}

func (s *ActionService) GetPolicy(ctx context.Context, accountID, adAccountID string) (ActionPolicyView, error) {
	if s.repository == nil || s.scope == nil || !metaAdsUUIDRe.MatchString(strings.TrimSpace(accountID)) ||
		!metaAdsUUIDRe.MatchString(strings.TrimSpace(adAccountID)) {
		return ActionPolicyView{}, ErrActionValidation
	}
	adAccount, err := s.scope.ResolveActionAdAccount(ctx, accountID, adAccountID)
	if err != nil {
		return ActionPolicyView{}, err
	}
	policy, err := s.repository.GetActionPolicy(ctx, adAccount.AccountID, adAccount.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ActionPolicyView{
			Configured: false, AdAccountID: adAccount.ID,
			Currency: strings.ToUpper(adAccount.Currency),
		}, nil
	}
	if err != nil {
		return ActionPolicyView{}, err
	}
	return toActionPolicyView(policy), nil
}

func (s *ActionService) PutPolicy(
	ctx context.Context,
	accountID, actorUserID, adAccountID string,
	input ActionPolicyInput,
) (ActionPolicyView, error) {
	if s.repository == nil || s.scope == nil || !metaAdsUUIDRe.MatchString(strings.TrimSpace(accountID)) ||
		!metaAdsUUIDRe.MatchString(strings.TrimSpace(actorUserID)) ||
		!metaAdsUUIDRe.MatchString(strings.TrimSpace(adAccountID)) {
		return ActionPolicyView{}, ErrActionValidation
	}
	input, err := normalizeActionPolicyInput(input)
	if err != nil {
		return ActionPolicyView{}, err
	}
	adAccount, err := s.scope.ResolveActionAdAccount(ctx, accountID, adAccountID)
	if err != nil {
		return ActionPolicyView{}, err
	}
	// A politica financeira pertence ao dono da conexao. Cliente que consome uma
	// ad account mapeada herda a politica da agencia e nao pode sobrescreve-la.
	if adAccount.AccountID != strings.TrimSpace(accountID) {
		return ActionPolicyView{}, pgx.ErrNoRows
	}
	if len(strings.TrimSpace(adAccount.Currency)) != 3 {
		return ActionPolicyView{}, ErrActionValidation
	}
	policy, err := s.repository.UpsertActionPolicy(
		ctx, accountID, adAccount, strings.TrimSpace(actorUserID), input,
	)
	if err != nil {
		return ActionPolicyView{}, err
	}
	return toActionPolicyView(policy), nil
}

func (s *ActionService) getProposal(ctx context.Context, accountID, proposalID string) (ActionProposal, error) {
	accountID = strings.TrimSpace(accountID)
	proposalID = strings.TrimSpace(proposalID)
	if s.repository == nil || !metaAdsUUIDRe.MatchString(accountID) || !metaAdsUUIDRe.MatchString(proposalID) {
		return ActionProposal{}, ErrActionValidation
	}
	return s.repository.GetActionProposal(ctx, accountID, proposalID)
}

func (s *ActionService) optionalActionPolicy(ctx context.Context, adAccount AdAccount) (*ActionPolicy, error) {
	policy, err := s.repository.GetActionPolicy(ctx, adAccount.AccountID, adAccount.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if policy.Currency != strings.ToUpper(adAccount.Currency) {
		return nil, ErrActionPolicyRequired
	}
	return &policy, nil
}

func (s *ActionService) revalidateProposal(ctx context.Context, proposal ActionProposal) error {
	adAccount, normalized, err := s.revalidateProposalAccess(ctx, proposal)
	if err != nil {
		return err
	}
	if err := validateActionBudgetCurrency(proposal.Action, normalized, adAccount.Currency); err != nil {
		return err
	}
	var target *Campaign
	if normalized.TargetCampaignID != "" {
		campaign, resolveErr := s.scope.ResolveActionCampaign(
			ctx, proposal.AccountID, proposal.AdAccountID, normalized.TargetCampaignID,
		)
		if resolveErr != nil {
			return resolveErr
		}
		if campaign.MetaCampaignID != proposal.TargetMetaCampaignID {
			return pgx.ErrNoRows
		}
		target = &campaign
	}
	policy, err := s.optionalActionPolicy(ctx, adAccount)
	if err != nil {
		return err
	}
	if err := validateActionAgainstPolicy(proposal.Action, normalized, target, policy); err != nil {
		return err
	}
	if proposal.Action == ActionPromoteInstagramPost {
		instagramScope, ok := s.scope.(actionInstagramScopeResolver)
		if !ok {
			return ErrMetaActionUnavailable
		}
		return instagramScope.ValidatePromotableInstagramPost(
			ctx, proposal.AccountID, proposal.AdAccountID, normalized.Raw,
		)
	}
	return nil
}

// revalidateProposalAccess repete o vinculo account/ad account imediatamente
// antes de qualquer leitura Graph. Reconciliacao nao reaplica a politica
// financeira corrente: ela apenas observa uma mutacao que ja foi tentada.
func (s *ActionService) revalidateProposalAccess(
	ctx context.Context,
	proposal ActionProposal,
) (AdAccount, normalizedActionPayload, error) {
	adAccount, err := s.scope.ResolveActionAdAccount(ctx, proposal.AccountID, proposal.AdAccountID)
	if err != nil {
		return AdAccount{}, normalizedActionPayload{}, err
	}
	if adAccount.AccountID != proposal.ResourceAccountID ||
		adAccount.MetaAdAccountID != proposal.MetaAdAccountID ||
		strings.ToUpper(adAccount.Currency) != proposal.Currency {
		return AdAccount{}, normalizedActionPayload{}, pgx.ErrNoRows
	}
	normalized, err := normalizeActionPayload(proposal.Action, proposal.Payload)
	if err != nil || actionRequestHash(proposal.Action, proposal.AdAccountID, normalized.Raw) != proposal.RequestHash {
		return AdAccount{}, normalizedActionPayload{}, ErrActionValidation
	}
	return adAccount, normalized, nil
}

func actionAdAccountAllowed(allowed []string, requested string) bool {
	for _, candidate := range allowed {
		if strings.TrimSpace(candidate) == requested {
			return true
		}
	}
	return false
}

func validateActionAgainstPolicy(
	action ActionKind,
	payload normalizedActionPayload,
	target *Campaign,
	policy *ActionPolicy,
) error {
	switch action {
	case ActionPauseCampaign:
		return nil
	case ActionUpdateCampaign:
		return validateBudgetAgainstPolicy(payload.Budget, policy)
	case ActionCreateCampaign, ActionPromoteInstagramPost:
		if policy == nil {
			return ErrActionPolicyRequired
		}
		if !policy.AllowCreate {
			return ErrActionPolicyDenied
		}
		return validateBudgetAgainstPolicy(payload.Budget, policy)
	case ActionDuplicateCampaign:
		if policy == nil {
			return ErrActionPolicyRequired
		}
		if !policy.AllowDuplicate {
			return ErrActionPolicyDenied
		}
		return validateCachedCampaignBudgets(target, policy, false)
	case ActionResumeCampaign:
		if policy == nil {
			return ErrActionPolicyRequired
		}
		if !policy.AllowResume {
			return ErrActionPolicyDenied
		}
		return validateCachedCampaignBudgets(target, policy, true)
	default:
		return ErrActionValidation
	}
}

// actionRequiresSpendAcknowledgement e calculado no backend a partir do
// payload canonico persistido. Em caso de payload update invalido, fecha do
// lado seguro; a revalidacao posterior devolvera ErrActionValidation.
func actionRequiresSpendAcknowledgement(action ActionKind, payload json.RawMessage) bool {
	if action == ActionResumeCampaign || action == ActionPromoteInstagramPost {
		return true
	}
	if action != ActionUpdateCampaign {
		return false
	}
	var update updateCampaignActionPayload
	if err := decodeStrictActionJSON(payload, &update); err != nil {
		return true
	}
	return update.Budget != nil
}

func validateCachedCampaignBudgets(target *Campaign, policy *ActionPolicy, requireKnown bool) error {
	if target == nil || policy == nil {
		return ErrActionValidation
	}
	known := false
	if target.DailyBudget != nil {
		known = true
		if policy.MaxDailyBudget == nil {
			return ErrActionPolicyRequired
		}
		if budgetMinorUnits(*target.DailyBudget) > budgetMinorUnits(*policy.MaxDailyBudget) {
			return ErrActionBudgetCapExceeded
		}
	}
	if target.LifetimeBudget != nil {
		known = true
		if policy.MaxLifetimeBudget == nil {
			return ErrActionPolicyRequired
		}
		if budgetMinorUnits(*target.LifetimeBudget) > budgetMinorUnits(*policy.MaxLifetimeBudget) {
			return ErrActionBudgetCapExceeded
		}
	}
	if requireKnown && !known {
		return ErrActionBudgetUnavailable
	}
	return nil
}

func buildActionSummary(
	action ActionKind,
	payload normalizedActionPayload,
	target *Campaign,
	adAccount AdAccount,
) string {
	accountName := strings.TrimSpace(adAccount.Name)
	if accountName == "" {
		accountName = adAccount.MetaAdAccountID
	}
	targetName := ""
	if target != nil {
		targetName = target.Name
	}
	switch action {
	case ActionCreateCampaign:
		return fmt.Sprintf("Criar a campanha %q pausada em %s.", payload.Name, accountName)
	case ActionPromoteInstagramPost:
		return fmt.Sprintf(
			"Promover o post selecionado na campanha %q; campanha, conjunto e anuncio nascerao pausados em %s.",
			payload.Name, accountName,
		)
	case ActionDuplicateCampaign:
		return fmt.Sprintf("Duplicar %q como %q, mantendo a copia pausada.", targetName, payload.Name)
	case ActionUpdateCampaign:
		parts := make([]string, 0, 2)
		if payload.Name != "" {
			parts = append(parts, fmt.Sprintf("nome para %q", payload.Name))
		}
		if payload.Budget != nil {
			parts = append(parts, fmt.Sprintf("orcamento %s para %.2f %s",
				payload.Budget.Type, payload.Budget.Amount, strings.ToUpper(adAccount.Currency)))
		}
		return fmt.Sprintf("Atualizar %q: %s.", targetName, strings.Join(parts, " e "))
	case ActionPauseCampaign:
		return fmt.Sprintf("Pausar a campanha %q.", targetName)
	case ActionResumeCampaign:
		return fmt.Sprintf("Ativar a campanha %q e retomar a veiculacao.", targetName)
	default:
		return "Proposta Meta Ads."
	}
}

func actionOutcomeFromError(err error) ActionExecutionOutcome {
	var executorErr *ActionExecutorError
	if errors.As(err, &executorErr) {
		status := ActionFailed
		if executorErr.Ambiguous {
			status = ActionUnknown
		}
		result := executorErr.Result
		if len(result) == 0 || len(result) > maxActionPayloadBytes || !json.Valid(result) {
			result = json.RawMessage(`{}`)
		}
		return ActionExecutionOutcome{
			Status: status, ErrorCode: strings.TrimSpace(executorErr.Code),
			ErrorMessage:     sanitizeActionErrorText(executorErr.Message),
			ExternalEntityID: strings.TrimSpace(executorErr.ExternalEntityID),
			Result:           result,
		}
	}
	return ActionExecutionOutcome{
		Status: ActionUnknown, ErrorCode: "execution_outcome_unknown",
		ErrorMessage: "Nao foi possivel determinar se a Meta aplicou a acao.", Result: json.RawMessage(`{}`),
	}
}

func normalizeActionOutcome(outcome ActionExecutionOutcome) ActionExecutionOutcome {
	if outcome.Status != ActionSucceeded && outcome.Status != ActionFailed && outcome.Status != ActionUnknown {
		outcome.Status = ActionUnknown
		outcome.ErrorCode = "invalid_executor_outcome"
		outcome.ErrorMessage = "O executor devolveu um resultado inconclusivo."
	}
	if len(outcome.Result) == 0 || len(outcome.Result) > maxActionPayloadBytes || !json.Valid(outcome.Result) {
		outcome.Result = json.RawMessage(`{}`)
	} else {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(outcome.Result, &object); err != nil || object == nil {
			outcome.Result = json.RawMessage(`{}`)
		}
	}
	outcome.ExternalEntityID = strings.TrimSpace(outcome.ExternalEntityID)
	if len(outcome.ExternalEntityID) > 160 {
		outcome.ExternalEntityID = outcome.ExternalEntityID[:160]
	}
	outcome.ErrorCode = strings.TrimSpace(outcome.ErrorCode)
	if len(outcome.ErrorCode) > 100 {
		outcome.ErrorCode = outcome.ErrorCode[:100]
	}
	outcome.ErrorMessage = sanitizeActionErrorText(outcome.ErrorMessage)
	return outcome
}

func (s *ActionService) toActionProposalView(proposal ActionProposal) ActionProposalView {
	available := s.executor != nil && s.executor.Supports(proposal.Action)
	bound := proposal.Source == ActionSourceManual || proposal.SourceBound
	viewStatus := proposal.Status
	if proposal.Status == ActionPending && !proposal.ExpiresAt.IsZero() && !proposal.ExpiresAt.After(time.Now()) {
		// A linha e atualizada atomicamente no proximo confirm/get. A projecao de
		// listagem ja fecha o botao no instante do vencimento para nunca oferecer
		// uma confirmacao que o backend necessariamente recusara.
		viewStatus = ActionExpired
	}
	result := proposal.ResultSnapshot
	if len(result) == 0 {
		result = json.RawMessage(`{}`)
	}
	view := ActionProposalView{
		ID: proposal.ID, Action: proposal.Action, Source: proposal.Source,
		AdAccountID: proposal.AdAccountID, MetaAdAccountID: proposal.MetaAdAccountID,
		AdAccountName: proposal.AdAccountName, Currency: proposal.Currency,
		TargetCampaignID:     proposal.TargetCampaignID,
		TargetMetaCampaignID: proposal.TargetMetaCampaignID,
		Payload:              proposal.Payload, Summary: proposal.Summary, Status: viewStatus,
		IdempotencyKey:               proposal.IdempotencyKey,
		ConfirmationIdempotencyKey:   proposal.ConfirmationIdempotencyKey,
		CancellationIdempotencyKey:   proposal.CancellationIdempotencyKey,
		ExecutionAvailable:           available,
		CanConfirm:                   available && bound && viewStatus == ActionPending,
		RequiresSpendAcknowledgement: actionRequiresSpendAcknowledgement(proposal.Action, proposal.Payload),
		ExternalEntityID:             proposal.ExternalEntityID, Result: result,
		ErrorCode: proposal.ErrorCode, ErrorMessage: proposal.ErrorMessage,
		ConfirmedAt: proposal.ConfirmedAt, ExecutionStartedAt: proposal.ExecutionStartedAt,
		CompletedAt: proposal.CompletedAt, ReconciledAt: proposal.ReconciledAt,
		CreatedAt: proposal.CreatedAt, ExpiresAt: proposal.ExpiresAt, UpdatedAt: proposal.UpdatedAt,
	}
	if viewStatus == ActionExpired {
		view.ErrorCode = "action_expired"
		view.ErrorMessage = "Esta proposta expirou. Peca ao assistente para preparar uma nova acao."
	}
	if proposal.Status == ActionPending && proposal.Source == ActionSourceAssistant && !bound {
		view.ErrorCode = "assistant_source_unbound"
		view.ErrorMessage = "O card do assistente ainda nao foi vinculado e nao pode ser confirmado."
	}
	return view
}

func toActionPolicyView(policy ActionPolicy) ActionPolicyView {
	updatedAt := policy.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	return ActionPolicyView{
		Configured: true, AdAccountID: policy.AdAccountID, Currency: policy.Currency,
		MaxDailyBudget: policy.MaxDailyBudget, MaxLifetimeBudget: policy.MaxLifetimeBudget,
		AllowCreate: policy.AllowCreate, AllowDuplicate: policy.AllowDuplicate,
		AllowResume: policy.AllowResume, UpdatedAt: &updatedAt,
	}
}

func actionPersistenceContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
}
