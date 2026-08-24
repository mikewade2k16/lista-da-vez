package calendar

import (
	"context"
	"errors"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func TestParseMetaTextConfirmationCommandIsExact(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value string
		ok    bool
		spend bool
	}{
		{"CONFIRMAR META 66666666", true, false},
		{"confirmar gasto meta 66666666-6666", true, true},
		{" sim, CONFIRMAR META 66666666", false, false},
		{"CONFIRMAR META", false, false},
		{"sim", false, false},
		{"confirmar", false, false},
	}
	for _, test := range tests {
		command, ok := parseMetaTextConfirmationCommand(test.value)
		if ok != test.ok || (ok && command.AcknowledgeSpend != test.spend) {
			t.Fatalf("parse %q = %#v, %v", test.value, command, ok)
		}
	}
}

func TestResolvePendingMetaTextActionRequiresUniqueConversationPrefix(t *testing.T) {
	t.Parallel()
	messages := []ChatMessage{
		metaTextTestMessage("message-a", "proposal-a", "66666666-6666-4666-8666-666666666666"),
		metaTextTestMessage("message-b", "proposal-b", "66666666-7777-4777-8777-777777777777"),
	}
	if _, err := resolvePendingMetaTextAction(messages, "66666666"); err != errMetaTextActionAmbiguous {
		t.Fatalf("ambiguous prefix error = %v", err)
	}
	resolved, err := resolvePendingMetaTextAction(messages, "66666666-7777")
	if err != nil || resolved.MessageID != "message-b" || resolved.Proposal.ID != "proposal-b" {
		t.Fatalf("resolved = %#v, err = %v", resolved, err)
	}
	if _, err := resolvePendingMetaTextAction(messages, "aaaaaaaa"); err != errMetaTextActionNotFound {
		t.Fatalf("unknown prefix error = %v", err)
	}
}

func TestResolvePendingMetaTextActionIgnoresResolvedCards(t *testing.T) {
	t.Parallel()
	message := metaTextTestMessage(
		"message-a", "proposal-a", "66666666-6666-4666-8666-666666666666",
	)
	message.Proposals[0].Status = "accepted"
	if _, err := resolvePendingMetaTextAction([]ChatMessage{message}, "66666666"); err != errMetaTextActionNotFound {
		t.Fatalf("resolved card error = %v", err)
	}
}

func TestMetaTextActionStatusAnswerNeverSuggestsBlindRetry(t *testing.T) {
	t.Parallel()
	answer := metaTextActionStatusAnswer(MetaAssistantActionResult{Status: "unknown"})
	if answer == "" || answer == "Acao Meta Ads executada e cartao confirmado com sucesso." {
		t.Fatalf("unknown answer = %q", answer)
	}
	if got := metaActionTextPrefix("66666666-6666-4666-8666-666666666666"); got != "66666666" {
		t.Fatalf("prefix = %q", got)
	}
}

func TestMetaTextConfirmationFailsClosedWhenManagePermissionWasRevoked(t *testing.T) {
	t.Parallel()
	service := &Service{}
	_, err := service.handleMetaTextConfirmation(
		context.Background(), "tenant-1",
		auth.Principal{
			UserID: "user-1", PermissionsResolved: true,
			Permissions: []string{"meta_ads.view"},
		},
		ChatConversation{ID: "conversation-1", EntrySurface: AssistantSurfaceMetaAds},
		[]AssistantCapability{{Module: "meta_ads", EffectiveMode: assistantModeWrite, Available: true}},
		"CONFIRMAR META 66666666",
		metaTextConfirmationCommand{Prefix: "66666666"},
	)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("revoked manage permission error = %v", err)
	}
}

func TestMetaTextConfirmationRequiresExplicitSpendPhrase(t *testing.T) {
	t.Parallel()
	confirmCalled := false
	service := &Service{
		store: &metaTextCommandStore{},
		metaAssistantActionStatusProvider: func(context.Context, string, string) (MetaAssistantActionResult, error) {
			return MetaAssistantActionResult{
				Status: "pending", ExecutionAvailable: true, CanConfirm: true,
				RequiresSpendAcknowledgement: true,
			}, nil
		},
		metaAssistantActionConfirmProvider: func(context.Context, MetaAssistantActionLifecycleRequest) (MetaAssistantActionResult, error) {
			confirmCalled = true
			return MetaAssistantActionResult{Status: "succeeded"}, nil
		},
	}
	answer, err := service.executeMetaTextConfirmation(
		context.Background(), "tenant-1", auth.Principal{UserID: "user-1"},
		ChatConversation{ID: "conversation-1"}, metaTextCandidate(),
		metaTextConfirmationCommand{Prefix: "66666666"},
	)
	if err != nil || confirmCalled || answer != "Esta acao altera ou retoma investimento. Para confirmar explicitamente, envie: CONFIRMAR GASTO META 44444444" {
		t.Fatalf("answer=%q confirmCalled=%v err=%v", answer, confirmCalled, err)
	}
}

func TestMetaTextConfirmationSuccessRetryUsesSameDurableKey(t *testing.T) {
	t.Parallel()
	store := &metaTextCommandStore{}
	statusCalls := 0
	var confirmRequests []MetaAssistantActionLifecycleRequest
	service := &Service{
		store: store,
		metaAssistantActionStatusProvider: func(context.Context, string, string) (MetaAssistantActionResult, error) {
			statusCalls++
			status := "pending"
			if statusCalls > 1 {
				status = "succeeded"
			}
			return MetaAssistantActionResult{
				Status: status, ExecutionAvailable: true, CanConfirm: status == "pending",
			}, nil
		},
		metaAssistantActionConfirmProvider: func(_ context.Context, req MetaAssistantActionLifecycleRequest) (MetaAssistantActionResult, error) {
			confirmRequests = append(confirmRequests, req)
			return MetaAssistantActionResult{Status: "succeeded"}, nil
		},
	}
	for attempt := 0; attempt < 2; attempt++ {
		answer, err := service.executeMetaTextConfirmation(
			context.Background(), "tenant-1", auth.Principal{UserID: "user-1"},
			ChatConversation{ID: "conversation-1"}, metaTextCandidate(),
			metaTextConfirmationCommand{Prefix: "66666666"},
		)
		if err != nil || answer != "Acao Meta Ads executada e cartao confirmado com sucesso." {
			t.Fatalf("attempt %d answer=%q err=%v", attempt+1, answer, err)
		}
	}
	if len(confirmRequests) != 2 {
		t.Fatalf("confirm calls = %d", len(confirmRequests))
	}
	for _, req := range confirmRequests {
		if req.IdempotencyKey != "assistant-text-confirm:"+metaActionProposal ||
			req.ActionProposalID != metaActionProposal || req.ConversationID != "conversation-1" ||
			req.MessageID != "message-a" {
			t.Fatalf("confirm request = %#v", req)
		}
	}
	if store.setCalls != 2 || store.getCalls != 1 {
		t.Fatalf("setCalls=%d getCalls=%d", store.setCalls, store.getCalls)
	}
}

type metaTextCommandStore struct {
	calendarStore
	setCalls int
	getCalls int
}

func (s *metaTextCommandStore) SetProposalStatus(
	context.Context, string, string, string, string, string,
) (ChatMessage, error) {
	s.setCalls++
	if s.setCalls > 1 {
		return ChatMessage{}, ErrNotFound
	}
	return metaTextAcceptedMessage(), nil
}

func (s *metaTextCommandStore) GetMessage(
	context.Context, string, string, string,
) (ChatMessage, error) {
	s.getCalls++
	return metaTextAcceptedMessage(), nil
}

func metaTextCandidate() pendingMetaTextAction {
	message := metaTextTestMessage("message-a", "proposal-a", metaActionProposal)
	return pendingMetaTextAction{MessageID: message.ID, Proposal: message.Proposals[0]}
}

func metaTextAcceptedMessage() ChatMessage {
	message := metaTextTestMessage("message-a", "proposal-a", metaActionProposal)
	message.Proposals[0].Status = "accepted"
	return message
}

func metaTextTestMessage(messageID, proposalID, actionProposalID string) ChatMessage {
	return ChatMessage{
		ID: messageID,
		Proposals: []StoredProposal{{
			ID: proposalID, Kind: "metaAction", Status: "pending",
			Fields: ChatProposalFields{MetaAction: &ChatProposalMetaAction{
				ActionProposalID: actionProposalID,
			}},
		}},
	}
}
