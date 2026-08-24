package calendar

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

var metaTextConfirmationRe = regexp.MustCompile(`(?i)^CONFIRMAR( GASTO)? META ([0-9a-f][0-9a-f-]{7,35})$`)

var (
	errMetaTextActionNotFound  = errors.New("calendar: meta text action not found")
	errMetaTextActionAmbiguous = errors.New("calendar: meta text action prefix is ambiguous")
)

type metaTextConfirmationCommand struct {
	Prefix           string
	AcknowledgeSpend bool
}

type pendingMetaTextAction struct {
	MessageID string
	Proposal  StoredProposal
}

func parseMetaTextConfirmationCommand(value string) (metaTextConfirmationCommand, bool) {
	matches := metaTextConfirmationRe.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 3 {
		return metaTextConfirmationCommand{}, false
	}
	return metaTextConfirmationCommand{
		Prefix:           strings.ToLower(matches[2]),
		AcknowledgeSpend: strings.TrimSpace(matches[1]) != "",
	}, true
}

func resolvePendingMetaTextAction(
	messages []ChatMessage,
	prefix string,
) (pendingMetaTextAction, error) {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	var matches []pendingMetaTextAction
	for _, message := range messages {
		for _, proposal := range message.Proposals {
			meta := proposal.Fields.MetaAction
			if proposal.Status != "pending" || proposal.Kind != "metaAction" || meta == nil {
				continue
			}
			actionProposalID := strings.ToLower(strings.TrimSpace(meta.ActionProposalID))
			if uuidRe.MatchString(actionProposalID) && strings.HasPrefix(actionProposalID, prefix) {
				matches = append(matches, pendingMetaTextAction{MessageID: message.ID, Proposal: proposal})
			}
		}
	}
	switch len(matches) {
	case 0:
		return pendingMetaTextAction{}, errMetaTextActionNotFound
	case 1:
		return matches[0], nil
	default:
		return pendingMetaTextAction{}, errMetaTextActionAmbiguous
	}
}

func (s *Service) handleMetaTextConfirmation(
	ctx context.Context,
	account string,
	principal auth.Principal,
	conv ChatConversation,
	capabilities []AssistantCapability,
	question string,
	command metaTextConfirmationCommand,
) (ChatAskResult, error) {
	if assistantCapabilityMode(capabilities, "meta_ads") != assistantModeWrite ||
		!canManageMetaActions(principal) {
		return ChatAskResult{}, ErrForbidden
	}
	if _, err := s.store.AppendMessage(ctx, account, conv.ID, ChatMessageInput{
		Role: chatRoleUser, Content: question, ContextModules: []string{"meta_ads"},
	}); err != nil {
		return ChatAskResult{}, err
	}
	messages, err := s.store.ListMessages(ctx, account, conv.ID)
	if err != nil {
		return ChatAskResult{}, err
	}
	candidate, resolveErr := resolvePendingMetaTextAction(messages, command.Prefix)
	answer := ""
	switch {
	case errors.Is(resolveErr, errMetaTextActionNotFound):
		answer = "Nao encontrei uma proposta Meta pendente com esse codigo nesta conversa. Copie o comando exibido no cartao."
	case errors.Is(resolveErr, errMetaTextActionAmbiguous):
		answer = "Esse codigo identifica mais de uma proposta Meta pendente. Use um prefixo maior do ID exibido no cartao."
	case resolveErr != nil:
		return ChatAskResult{}, resolveErr
	default:
		answer, err = s.executeMetaTextConfirmation(ctx, account, principal, conv, candidate, command)
		if err != nil {
			return ChatAskResult{}, err
		}
	}
	messageID, err := newAssistantMessageID()
	if err != nil {
		return ChatAskResult{}, err
	}
	assistant, err := s.store.AppendMessage(ctx, account, conv.ID, ChatMessageInput{
		ID: messageID, Role: chatRoleAssistant, Content: answer,
		ContextModules: []string{"meta_ads"},
	})
	if err != nil {
		return ChatAskResult{}, err
	}
	title := conv.Title
	if strings.TrimSpace(title) == "" {
		title = deriveChatTitle(question)
	}
	if err := s.store.TouchConversation(ctx, account, conv.ID, title); err != nil {
		return ChatAskResult{}, err
	}
	return ChatAskResult{
		Answer: answer, ConversationID: conv.ID, Title: title,
		Surface: conv.EntrySurface, Capabilities: capabilities,
		Message: messageViewFrom(assistant),
	}, nil
}

func (s *Service) executeMetaTextConfirmation(
	ctx context.Context,
	account string,
	principal auth.Principal,
	conv ChatConversation,
	candidate pendingMetaTextAction,
	command metaTextConfirmationCommand,
) (string, error) {
	meta := candidate.Proposal.Fields.MetaAction
	if meta == nil || s.metaAssistantActionStatusProvider == nil {
		return "Nao foi possivel validar o estado atual desta proposta Meta Ads.", nil
	}
	actionProposalID := strings.TrimSpace(meta.ActionProposalID)
	current, err := s.metaAssistantActionStatusProvider(ctx, account, actionProposalID)
	if err != nil {
		return "", err
	}
	status := normalizeMetaActionStatus(current.Status)
	if current.RequiresSpendAcknowledgement && !command.AcknowledgeSpend {
		return "Esta acao altera ou retoma investimento. Para confirmar explicitamente, envie: CONFIRMAR GASTO META " + metaActionTextPrefix(actionProposalID), nil
	}
	if status != "pending" && status != "succeeded" {
		return metaTextActionStatusAnswer(current), nil
	}
	if status == "pending" && (!current.ExecutionAvailable || !current.CanConfirm) {
		return metaTextActionStatusAnswer(current), nil
	}
	if s.metaAssistantActionConfirmProvider == nil {
		return "Nao foi possivel confirmar esta proposta Meta Ads com seguranca.", nil
	}
	result, err := s.metaAssistantActionConfirmProvider(ctx, MetaAssistantActionLifecycleRequest{
		AccountID: account, ActorUserID: principal.UserID,
		ConversationID: conv.ID, MessageID: candidate.MessageID,
		ActionProposalID: actionProposalID,
		IdempotencyKey:   "assistant-text-confirm:" + actionProposalID,
		AcknowledgeSpend: command.AcknowledgeSpend,
	})
	if err != nil {
		return "", err
	}
	if normalizeMetaActionStatus(result.Status) != "succeeded" {
		return metaTextActionStatusAnswer(result), nil
	}
	if _, err := s.store.SetProposalStatus(
		ctx, account, conv.ID, candidate.MessageID, candidate.Proposal.ID, "accepted",
	); err != nil {
		if !errors.Is(err, ErrNotFound) {
			return "", err
		}
		currentMessage, getErr := s.store.GetMessage(ctx, account, conv.ID, candidate.MessageID)
		if getErr != nil {
			return "", mapNotFound(getErr)
		}
		proposal, ok := metaActionProposalFromMessage(currentMessage, candidate.Proposal.ID)
		if !ok || proposal.Status != "accepted" {
			return "", ErrNotFound
		}
	}
	return "Acao Meta Ads executada e cartao confirmado com sucesso.", nil
}

func metaActionTextPrefix(actionProposalID string) string {
	actionProposalID = strings.ToLower(strings.TrimSpace(actionProposalID))
	if len(actionProposalID) <= 8 {
		return actionProposalID
	}
	return actionProposalID[:8]
}

func metaTextActionStatusAnswer(result MetaAssistantActionResult) string {
	message := strings.TrimSpace(result.ErrorMessage)
	switch normalizeMetaActionStatus(result.Status) {
	case "executing":
		return "A acao Meta Ads ja esta em execucao. Aguarde e use Reconciliar no cartao se o estado nao atualizar."
	case "unknown":
		return "O resultado da acao Meta Ads esta incerto. Nao repetirei a escrita; use Reconciliar no cartao."
	case "failed":
		if message != "" {
			return "A acao Meta Ads falhou: " + message
		}
		return "A acao Meta Ads falhou. Revise o cartao antes de preparar uma nova proposta."
	case "cancelled":
		return "Esta proposta Meta Ads foi cancelada e nao pode mais ser executada."
	case "expired":
		return "Esta proposta Meta Ads expirou. Peca ao assistente para preparar uma nova acao."
	case "succeeded":
		return "A acao Meta Ads ja foi executada com sucesso."
	default:
		if message != "" {
			return "A proposta Meta Ads nao pode ser confirmada agora: " + message
		}
		return "A proposta Meta Ads nao esta disponivel para confirmacao neste momento."
	}
}
