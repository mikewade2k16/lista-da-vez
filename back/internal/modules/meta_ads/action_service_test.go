package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	actionTestAccount      = "11111111-1111-4111-8111-111111111111"
	actionTestResource     = "22222222-2222-4222-8222-222222222222"
	actionTestUser         = "33333333-3333-4333-8333-333333333333"
	actionTestAdAccount    = "44444444-4444-4444-8444-444444444444"
	actionTestCampaign     = "55555555-5555-4555-8555-555555555555"
	actionTestProposal     = "66666666-6666-4666-8666-666666666666"
	actionTestConversation = "77777777-7777-4777-8777-777777777777"
	actionTestMessage      = "88888888-8888-4888-8888-888888888888"
)

func TestCreateAssistantProposalUsesAuthoritativeScopeAndDerivedKey(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	service := &ActionService{repository: repository, scope: scope}
	input := AssistantActionProposalInput{
		ConversationID: actionTestConversation, MessageID: actionTestMessage,
		ProposalIndex: 2, AllowedAdAccountIDs: []string{actionTestAdAccount},
		Action: ActionPauseCampaign, AdAccountID: actionTestAdAccount,
		Payload: json.RawMessage(`{"campaignId":"` + actionTestCampaign + `"}`),
	}

	view, created, err := service.CreateProposalFromAssistant(
		context.Background(), actionTestAccount, actionTestUser, input,
	)
	if err != nil || !created {
		t.Fatalf("CreateProposalFromAssistant() = created %v, err %v", created, err)
	}
	if view.Source != ActionSourceAssistant || view.IdempotencyKey != "assistant:"+actionTestMessage+":2" {
		t.Fatalf("assistant proposal = %#v", view)
	}
	if repository.lastInsert.AccountID != actionTestAccount ||
		repository.lastInsert.ResourceAccountID != actionTestResource ||
		repository.lastInsert.SourceConversationID != actionTestConversation ||
		repository.lastInsert.SourceMessageID != actionTestMessage {
		t.Fatalf("insert scope = %#v", repository.lastInsert)
	}

	input.AllowedAdAccountIDs = []string{"99999999-9999-4999-8999-999999999999"}
	if _, _, err := service.CreateProposalFromAssistant(
		context.Background(), actionTestAccount, actionTestUser, input,
	); !errors.Is(err, ErrActionValidation) {
		t.Fatalf("out-of-registry ad account error = %v", err)
	}
}

func TestActionProposalCreationIsIdempotentAndRejectsPayloadDrift(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	service := &ActionService{repository: repository, scope: scope}
	input := ActionProposalInput{
		Action: ActionUpdateCampaign, AdAccountID: actionTestAdAccount,
		Payload: json.RawMessage(`{"campaignId":"` + actionTestCampaign + `","name":"Nome A"}`),
	}

	first, created, err := service.CreateProposalFromUser(
		context.Background(), actionTestAccount, actionTestUser, "request:pause:0001", input,
	)
	if err != nil || !created {
		t.Fatalf("first create = created %v, err %v", created, err)
	}
	second, created, err := service.CreateProposalFromUser(
		context.Background(), actionTestAccount, actionTestUser, "request:pause:0001", input,
	)
	if err != nil || created || second.ID != first.ID {
		t.Fatalf("replay = %#v, created %v, err %v", second, created, err)
	}

	input.Payload = json.RawMessage(`{"campaignId":"` + actionTestCampaign + `","name":"Nome B"}`)
	if _, _, err := service.CreateProposalFromUser(
		context.Background(), actionTestAccount, actionTestUser, "request:pause:0001", input,
	); !errors.Is(err, ErrActionIdempotencyConflict) {
		t.Fatalf("drifted dangerous action error = %v", err)
	}
}

func TestConfirmProposalExecutesOnceAndReplaysTerminalResult(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	executor := &fakeActionExecutor{supported: map[ActionKind]bool{ActionPauseCampaign: true}, outcome: ActionExecutionOutcome{
		Status: ActionSucceeded, ExternalEntityID: "123456789", Result: json.RawMessage(`{"status":"PAUSED"}`),
	}}
	service := &ActionService{repository: repository, scope: scope, executor: executor}
	proposal := createActionTestProposal(t, service, ActionPauseCampaign,
		json.RawMessage(`{"campaignId":"`+actionTestCampaign+`"}`), "request:pause:0002")

	first, err := service.ConfirmProposal(
		context.Background(), actionTestAccount, actionTestUser, proposal.ID, "confirm:pause:0002", false,
	)
	if err != nil || first.Status != ActionSucceeded {
		t.Fatalf("first confirm = %#v, err %v", first, err)
	}
	second, err := service.ConfirmProposal(
		context.Background(), actionTestAccount, actionTestUser, proposal.ID, "confirm:pause:other", false,
	)
	if err != nil || second.Status != ActionSucceeded || second.ExternalEntityID != first.ExternalEntityID {
		t.Fatalf("replay confirm = %#v, err %v", second, err)
	}
	if executor.executeCalls != 1 || repository.executionStarts != 1 || repository.completions != 1 {
		t.Fatalf("calls execute=%d begin=%d complete=%d", executor.executeCalls, repository.executionStarts, repository.completions)
	}
}

func TestAmbiguousExecutionBecomesUnknownAndCanReconcile(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	executor := &fakeActionExecutor{
		supported: map[ActionKind]bool{ActionPauseCampaign: true},
		executeErr: &ActionExecutorError{
			Code: "execution_outcome_unknown", Message: "timeout", Ambiguous: true,
		},
		reconcileOutcome: ActionExecutionOutcome{
			Status: ActionSucceeded, ExternalEntityID: "123456789",
			Result: json.RawMessage(`{"status":"PAUSED"}`),
		},
	}
	service := &ActionService{repository: repository, scope: scope, executor: executor}
	proposal := createActionTestProposal(t, service, ActionPauseCampaign,
		json.RawMessage(`{"campaignId":"`+actionTestCampaign+`"}`), "request:pause:0003")

	unknown, err := service.ConfirmProposal(
		context.Background(), actionTestAccount, actionTestUser, proposal.ID, "confirm:pause:0003", false,
	)
	if err != nil || unknown.Status != ActionUnknown || unknown.ErrorCode != "execution_outcome_unknown" {
		t.Fatalf("unknown outcome = %#v, err %v", unknown, err)
	}
	reconciled, err := service.ReconcileProposal(
		context.Background(), actionTestAccount, actionTestUser, proposal.ID,
	)
	if err != nil || reconciled.Status != ActionSucceeded || reconciled.ReconciledAt == nil {
		t.Fatalf("reconciled = %#v, err %v", reconciled, err)
	}
	if executor.executeCalls != 1 || executor.reconcileCalls != 1 {
		t.Fatalf("executor calls = execute %d reconcile %d", executor.executeCalls, executor.reconcileCalls)
	}
}

func TestDisabledWritesNeverConsumePendingProposal(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	service := &ActionService{repository: repository, scope: scope}
	proposal := createActionTestProposal(t, service, ActionPauseCampaign,
		json.RawMessage(`{"campaignId":"`+actionTestCampaign+`"}`), "request:pause:0004")

	_, err := service.ConfirmProposal(
		context.Background(), actionTestAccount, actionTestUser, proposal.ID, "confirm:pause:0004", false,
	)
	if !errors.Is(err, ErrMetaWritesDisabled) {
		t.Fatalf("ConfirmProposal() error = %v", err)
	}
	stored, err := repository.GetActionProposal(context.Background(), actionTestAccount, proposal.ID)
	if err != nil || stored.Status != ActionPending || repository.executionStarts != 0 {
		t.Fatalf("stored proposal = %#v, starts=%d, err=%v", stored, repository.executionStarts, err)
	}
}

func TestBudgetAndResumeAreClosedByPolicy(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	service := &ActionService{repository: repository, scope: scope}
	budgetPayload := json.RawMessage(`{"campaignId":"` + actionTestCampaign + `","budget":{"type":"daily","amount":101}}`)

	if _, _, err := service.CreateProposalFromUser(context.Background(), actionTestAccount,
		actionTestUser, "request:budget:001", ActionProposalInput{
			Action: ActionUpdateCampaign, AdAccountID: actionTestAdAccount, Payload: budgetPayload,
		}); !errors.Is(err, ErrActionPolicyRequired) {
		t.Fatalf("budget without policy error = %v", err)
	}
	capValue := 100.0
	repository.policy = &ActionPolicy{
		AccountID: actionTestResource, AdAccountID: actionTestAdAccount,
		Currency: "BRL", MaxDailyBudget: &capValue,
	}
	if _, _, err := service.CreateProposalFromUser(context.Background(), actionTestAccount,
		actionTestUser, "request:budget:002", ActionProposalInput{
			Action: ActionUpdateCampaign, AdAccountID: actionTestAdAccount, Payload: budgetPayload,
		}); !errors.Is(err, ErrActionBudgetCapExceeded) {
		t.Fatalf("budget over cap error = %v", err)
	}

	repository.policy.AllowResume = true
	scope.campaign.DailyBudget = nil
	if _, _, err := service.CreateProposalFromUser(context.Background(), actionTestAccount,
		actionTestUser, "request:resume:001", ActionProposalInput{
			Action: ActionResumeCampaign, AdAccountID: actionTestAdAccount,
			Payload: json.RawMessage(`{"campaignId":"` + actionTestCampaign + `"}`),
		}); !errors.Is(err, ErrActionBudgetUnavailable) {
		t.Fatalf("resume without known budget error = %v", err)
	}
}

func TestResumeRequiresReinforcedConfirmation(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	capValue := 100.0
	repository.policy = &ActionPolicy{
		AccountID: actionTestResource, AdAccountID: actionTestAdAccount,
		Currency: "BRL", MaxDailyBudget: &capValue, AllowResume: true,
	}
	executor := &fakeActionExecutor{
		supported: map[ActionKind]bool{ActionResumeCampaign: true},
		outcome: ActionExecutionOutcome{
			Status: ActionSucceeded, ExternalEntityID: "123456789",
			Result: json.RawMessage(`{"status":"ACTIVE"}`),
		},
	}
	service := &ActionService{repository: repository, scope: scope, executor: executor}
	proposal := createActionTestProposal(t, service, ActionResumeCampaign,
		json.RawMessage(`{"campaignId":"`+actionTestCampaign+`"}`), "request:resume:002")

	if _, err := service.ConfirmProposal(
		context.Background(), actionTestAccount, actionTestUser, proposal.ID,
		"confirm:resume:002", false,
	); !errors.Is(err, ErrActionReinforcedConfirm) {
		t.Fatalf("confirmation without spend acknowledgement error = %v", err)
	}
	if repository.executionStarts != 0 || executor.executeCalls != 0 {
		t.Fatalf("unsafe execution started: begin=%d execute=%d", repository.executionStarts, executor.executeCalls)
	}

	confirmed, err := service.ConfirmProposal(
		context.Background(), actionTestAccount, actionTestUser, proposal.ID,
		"confirm:resume:002", true,
	)
	if err != nil || confirmed.Status != ActionSucceeded || executor.executeCalls != 1 {
		t.Fatalf("reinforced confirmation = %#v, calls=%d, err=%v", confirmed, executor.executeCalls, err)
	}
}

func TestBudgetUpdateRequiresReinforcedConfirmationBeforeClaim(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	capValue := 100.0
	repository.policy = &ActionPolicy{
		AccountID: actionTestResource, AdAccountID: actionTestAdAccount,
		Currency: "BRL", MaxDailyBudget: &capValue,
	}
	executor := &fakeActionExecutor{
		supported: map[ActionKind]bool{ActionUpdateCampaign: true},
		outcome: ActionExecutionOutcome{
			Status: ActionSucceeded, ExternalEntityID: "123456789",
			Result: json.RawMessage(`{"dailyBudgetMinor":"7500"}`),
		},
	}
	service := &ActionService{repository: repository, scope: scope, executor: executor}
	proposal := createActionTestProposal(t, service, ActionUpdateCampaign,
		json.RawMessage(`{"campaignId":"`+actionTestCampaign+`","budget":{"type":"daily","amount":75}}`),
		"request:budget:ack")
	if !proposal.RequiresSpendAcknowledgement {
		t.Fatalf("budget proposal view = %#v", proposal)
	}

	if _, err := service.ConfirmProposal(
		context.Background(), actionTestAccount, actionTestUser, proposal.ID,
		"confirm:budget:ack", false,
	); !errors.Is(err, ErrActionReinforcedConfirm) {
		t.Fatalf("budget confirmation without acknowledgement error = %v", err)
	}
	if repository.executionStarts != 0 || executor.executeCalls != 0 {
		t.Fatalf("budget claim before acknowledgement: begin=%d execute=%d",
			repository.executionStarts, executor.executeCalls)
	}

	confirmed, err := service.ConfirmProposal(
		context.Background(), actionTestAccount, actionTestUser, proposal.ID,
		"confirm:budget:ack", true,
	)
	if err != nil || confirmed.Status != ActionSucceeded ||
		repository.executionStarts != 1 || executor.executeCalls != 1 ||
		!repository.lastAcknowledgedSpend {
		t.Fatalf("acknowledged budget confirmation = %#v, begin=%d execute=%d ack=%v err=%v",
			confirmed, repository.executionStarts, executor.executeCalls,
			repository.lastAcknowledgedSpend, err)
	}
}

func TestNameOnlyUpdateDoesNotRequireSpendAcknowledgement(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	service := &ActionService{repository: repository, scope: scope}
	proposal := createActionTestProposal(t, service, ActionUpdateCampaign,
		json.RawMessage(`{"campaignId":"`+actionTestCampaign+`","name":"Nome seguro"}`),
		"request:name:noack")
	if proposal.RequiresSpendAcknowledgement {
		t.Fatalf("name-only proposal view = %#v", proposal)
	}
}

func TestPromoteInstagramPostRequiresPolicyLiveSourceAndSpendAcknowledgement(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	capValue := 100.0
	repository.policy = &ActionPolicy{
		AccountID: actionTestResource, AdAccountID: actionTestAdAccount,
		Currency: "BRL", MaxDailyBudget: &capValue, AllowCreate: true,
	}
	executor := &fakeActionExecutor{
		supported: map[ActionKind]bool{ActionPromoteInstagramPost: true},
		outcome: ActionExecutionOutcome{
			Status: ActionSucceeded, ExternalEntityID: "44556677",
			Result: json.RawMessage(`{"campaignId":"11","adSetId":"22","creativeId":"33","adId":"44556677"}`),
		},
	}
	service := &ActionService{repository: repository, scope: scope, executor: executor}
	payload := json.RawMessage(`{
		"name":"Campanha post","instagramPostId":"77889900","igUserId":"66778899",
		"pageId":"55667788","adSetName":"Conjunto","adName":"Anuncio",
		"budget":{"type":"daily","amount":25},"countries":["BR"],
		"ageMin":18,"ageMax":65,"status":"PAUSED"
	}`)
	proposal := createActionTestProposal(t, service, ActionPromoteInstagramPost, payload, "request:post:001")
	if !proposal.RequiresSpendAcknowledgement || !proposal.ExecutionAvailable || scope.instagramCalls != 1 {
		t.Fatalf("proposal=%#v instagramCalls=%d", proposal, scope.instagramCalls)
	}
	if _, err := service.ConfirmProposal(
		context.Background(), actionTestAccount, actionTestUser, proposal.ID, "confirm:post:001", false,
	); !errors.Is(err, ErrActionReinforcedConfirm) {
		t.Fatalf("confirm without acknowledgement = %v", err)
	}
	if repository.executionStarts != 0 || executor.executeCalls != 0 || scope.instagramCalls != 1 {
		t.Fatalf("unsafe path starts=%d executor=%d source=%d",
			repository.executionStarts, executor.executeCalls, scope.instagramCalls)
	}
	confirmed, err := service.ConfirmProposal(
		context.Background(), actionTestAccount, actionTestUser, proposal.ID, "confirm:post:001", true,
	)
	if err != nil || confirmed.Status != ActionSucceeded || executor.executeCalls != 1 ||
		scope.instagramCalls != 2 {
		t.Fatalf("confirmed=%#v executor=%d source=%d err=%v",
			confirmed, executor.executeCalls, scope.instagramCalls, err)
	}
}

func TestReconciliationRevalidatesCurrentAdAccountAccess(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	executor := &fakeActionExecutor{
		supported: map[ActionKind]bool{ActionPauseCampaign: true},
		executeErr: &ActionExecutorError{
			Code: "execution_outcome_unknown", Message: "timeout", Ambiguous: true,
		},
		reconcileOutcome: ActionExecutionOutcome{
			Status: ActionSucceeded, ExternalEntityID: "123456789",
			Result: json.RawMessage(`{"status":"PAUSED"}`),
		},
	}
	service := &ActionService{repository: repository, scope: scope, executor: executor}
	proposal := createActionTestProposal(t, service, ActionPauseCampaign,
		json.RawMessage(`{"campaignId":"`+actionTestCampaign+`"}`), "request:pause:scope")
	if _, err := service.ConfirmProposal(
		context.Background(), actionTestAccount, actionTestUser, proposal.ID,
		"confirm:pause:scope", false,
	); err != nil {
		t.Fatal(err)
	}

	scope.blocked = true
	if _, err := service.ReconcileProposal(
		context.Background(), actionTestAccount, actionTestUser, proposal.ID,
	); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("reconciliation after access removal error = %v", err)
	}
	if executor.reconcileCalls != 0 {
		t.Fatalf("Graph reconciliation calls after access removal = %d", executor.reconcileCalls)
	}
}

func TestCreateProposalForcesPausedAndRemainsUnavailableWithoutSafeExecutor(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	repository.policy = &ActionPolicy{
		AccountID: actionTestResource, AdAccountID: actionTestAdAccount,
		Currency: "BRL", AllowCreate: true,
	}
	executor := &fakeActionExecutor{supported: map[ActionKind]bool{ActionPauseCampaign: true}}
	service := &ActionService{repository: repository, scope: scope, executor: executor}

	view, created, err := service.CreateProposalFromUser(
		context.Background(), actionTestAccount, actionTestUser, "request:create:001",
		ActionProposalInput{
			Action: ActionCreateCampaign, AdAccountID: actionTestAdAccount,
			Payload: json.RawMessage(`{"name":"Lancamento","objective":"OUTCOME_TRAFFIC","specialAdCategories":[],"status":"ACTIVE"}`),
		},
	)
	if err != nil || !created {
		t.Fatalf("create proposal = created %v, err %v", created, err)
	}
	var payload createCampaignActionPayload
	if err := json.Unmarshal(view.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Status != "PAUSED" || view.ExecutionAvailable || view.CanConfirm {
		t.Fatalf("create proposal safety = payload %#v view %#v", payload, view)
	}
}

func TestMoneyRejectsFractionBeyondMinorUnit(t *testing.T) {
	t.Parallel()
	_, err := normalizeActionPayload(ActionUpdateCampaign, json.RawMessage(
		`{"campaignId":"`+actionTestCampaign+`","budget":{"type":"daily","amount":10.101}}`,
	))
	if !errors.Is(err, ErrActionValidation) {
		t.Fatalf("fractional minor unit error = %v", err)
	}
}

func TestAssistantProposalCannotExecuteBeforeCardBinding(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	executor := &fakeActionExecutor{
		supported: map[ActionKind]bool{ActionPauseCampaign: true},
		outcome:   ActionExecutionOutcome{Status: ActionSucceeded, Result: json.RawMessage(`{"status":"PAUSED"}`)},
	}
	service := &ActionService{
		repository: repository, scope: scope, executor: executor,
		assistantSourceValidator: func(context.Context, string, string, string, string) error { return nil },
	}
	view, _, err := service.CreateProposalFromAssistant(
		context.Background(), actionTestAccount, actionTestUser,
		AssistantActionProposalInput{
			ConversationID: actionTestConversation, MessageID: actionTestMessage,
			AllowedAdAccountIDs: []string{actionTestAdAccount},
			Action:              ActionPauseCampaign, AdAccountID: actionTestAdAccount,
			Payload: json.RawMessage(`{"campaignId":"` + actionTestCampaign + `"}`),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if view.CanConfirm {
		t.Fatalf("unbound assistant proposal = %#v", view)
	}
	if _, err := service.ConfirmAssistantProposal(
		context.Background(), actionTestAccount, actionTestUser, view.ID,
		actionTestConversation, actionTestMessage, "assistant-confirm:unbound", false,
	); !errors.Is(err, ErrActionSourceUnbound) {
		t.Fatalf("unbound confirm error = %v", err)
	}
	if executor.executeCalls != 0 || repository.executionStarts != 0 {
		t.Fatalf("unbound execution = calls %d starts %d", executor.executeCalls, repository.executionStarts)
	}
}

func TestAssistantBindConfirmAndReplayAreDurable(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	executor := &fakeActionExecutor{
		supported: map[ActionKind]bool{ActionPauseCampaign: true},
		outcome:   ActionExecutionOutcome{Status: ActionSucceeded, Result: json.RawMessage(`{"status":"PAUSED"}`)},
	}
	validatorCalls := 0
	service := &ActionService{
		repository: repository, scope: scope, executor: executor,
		assistantSourceValidator: func(_ context.Context, accountID, conversationID, messageID, proposalID string) error {
			validatorCalls++
			if accountID != actionTestAccount || conversationID != actionTestConversation ||
				messageID != actionTestMessage || proposalID != actionTestProposal {
				return ErrActionValidation
			}
			return nil
		},
	}
	view, _, err := service.CreateProposalFromAssistant(
		context.Background(), actionTestAccount, actionTestUser,
		AssistantActionProposalInput{
			ConversationID: actionTestConversation, MessageID: actionTestMessage,
			AllowedAdAccountIDs: []string{actionTestAdAccount},
			Action:              ActionPauseCampaign, AdAccountID: actionTestAdAccount,
			Payload: json.RawMessage(`{"campaignId":"` + actionTestCampaign + `"}`),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := service.BindAssistantProposal(
		context.Background(), actionTestAccount, actionTestUser, view.ID,
		actionTestConversation, actionTestMessage,
	)
	if err != nil || !bound.CanConfirm {
		t.Fatalf("bound proposal = %#v, err %v", bound, err)
	}
	first, err := service.ConfirmAssistantProposal(
		context.Background(), actionTestAccount, actionTestUser, view.ID,
		actionTestConversation, actionTestMessage, "assistant-text-confirm:"+view.ID, false,
	)
	if err != nil || first.Status != ActionSucceeded {
		t.Fatalf("first confirmation = %#v, err %v", first, err)
	}
	replay, err := service.ConfirmAssistantProposal(
		context.Background(), actionTestAccount, actionTestUser, view.ID,
		actionTestConversation, actionTestMessage, "assistant-text-confirm:"+view.ID, false,
	)
	if err != nil || replay.Status != ActionSucceeded || executor.executeCalls != 1 ||
		repository.executionStarts != 1 || validatorCalls < 2 {
		t.Fatalf("replay = %#v, execute %d, starts %d, validator %d, err %v",
			replay, executor.executeCalls, repository.executionStarts, validatorCalls, err)
	}
}

func TestAssistantCancellationAndConversationDeletionPreventExecution(t *testing.T) {
	t.Parallel()
	for _, cancelConversation := range []bool{false, true} {
		repository, scope := newActionTestDependencies()
		executor := &fakeActionExecutor{
			supported: map[ActionKind]bool{ActionPauseCampaign: true},
			outcome:   ActionExecutionOutcome{Status: ActionSucceeded},
		}
		service := &ActionService{
			repository: repository, scope: scope, executor: executor,
			assistantSourceValidator: func(context.Context, string, string, string, string) error { return nil },
		}
		view, _, err := service.CreateProposalFromAssistant(
			context.Background(), actionTestAccount, actionTestUser,
			AssistantActionProposalInput{
				ConversationID: actionTestConversation, MessageID: actionTestMessage,
				AllowedAdAccountIDs: []string{actionTestAdAccount},
				Action:              ActionPauseCampaign, AdAccountID: actionTestAdAccount,
				Payload: json.RawMessage(`{"campaignId":"` + actionTestCampaign + `"}`),
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.BindAssistantProposal(
			context.Background(), actionTestAccount, actionTestUser, view.ID,
			actionTestConversation, actionTestMessage,
		); err != nil {
			t.Fatal(err)
		}
		if cancelConversation {
			if count, err := service.CancelAssistantConversation(
				context.Background(), actionTestAccount, actionTestUser, actionTestConversation,
			); err != nil || count != 1 {
				t.Fatalf("conversation cancel = %d, %v", count, err)
			}
		} else if cancelled, err := service.CancelAssistantProposal(
			context.Background(), actionTestAccount, actionTestUser, view.ID,
			actionTestConversation, actionTestMessage, "assistant-cancel:"+view.ID,
		); err != nil || cancelled.Status != ActionCancelled {
			t.Fatalf("proposal cancel = %#v, %v", cancelled, err)
		}
		confirmed, err := service.ConfirmProposal(
			context.Background(), actionTestAccount, actionTestUser, view.ID,
			"assistant-confirm:after-cancel", false,
		)
		if err != nil || confirmed.Status != ActionCancelled || executor.executeCalls != 0 {
			t.Fatalf("confirm after cancellation = %#v, calls %d, err %v",
				confirmed, executor.executeCalls, err)
		}
	}
}

func TestExpiredProposalBecomesTerminalBeforeExecution(t *testing.T) {
	t.Parallel()
	repository, scope := newActionTestDependencies()
	executor := &fakeActionExecutor{
		supported: map[ActionKind]bool{ActionPauseCampaign: true},
		outcome:   ActionExecutionOutcome{Status: ActionSucceeded},
	}
	service := &ActionService{repository: repository, scope: scope, executor: executor}
	view := createActionTestProposal(t, service, ActionPauseCampaign,
		json.RawMessage(`{"campaignId":"`+actionTestCampaign+`"}`), "request:expired:001")
	repository.mu.Lock()
	repository.proposal.ExpiresAt = time.Now().Add(-time.Nanosecond)
	repository.mu.Unlock()
	if _, err := service.ConfirmProposal(
		context.Background(), actionTestAccount, actionTestUser, view.ID,
		"confirm:expired:001", false,
	); !errors.Is(err, ErrActionExpired) {
		t.Fatalf("expired confirm error = %v", err)
	}
	stored, err := repository.GetActionProposal(context.Background(), actionTestAccount, view.ID)
	if err != nil || stored.Status != ActionExpired || executor.executeCalls != 0 || repository.executionStarts != 0 {
		t.Fatalf("expired = %#v, calls %d, starts %d, err %v",
			stored, executor.executeCalls, repository.executionStarts, err)
	}
}

func createActionTestProposal(
	t *testing.T,
	service *ActionService,
	action ActionKind,
	payload json.RawMessage,
	key string,
) ActionProposalView {
	t.Helper()
	view, created, err := service.CreateProposalFromUser(
		context.Background(), actionTestAccount, actionTestUser, key,
		ActionProposalInput{Action: action, AdAccountID: actionTestAdAccount, Payload: payload},
	)
	if err != nil || !created {
		t.Fatalf("create test proposal = created %v, err %v", created, err)
	}
	return view
}

type fakeActionScope struct {
	adAccount      AdAccount
	campaign       Campaign
	blocked        bool
	instagramCalls int
	instagramErr   error
}

func (s *fakeActionScope) ResolveActionAdAccount(_ context.Context, accountID, adAccountID string) (AdAccount, error) {
	if s.blocked || accountID != actionTestAccount || adAccountID != s.adAccount.ID {
		return AdAccount{}, pgx.ErrNoRows
	}
	return s.adAccount, nil
}

func (s *fakeActionScope) ResolveActionCampaign(
	_ context.Context,
	accountID, adAccountID, campaignID string,
) (Campaign, error) {
	if accountID != actionTestAccount || adAccountID != s.adAccount.ID || campaignID != s.campaign.ID {
		return Campaign{}, pgx.ErrNoRows
	}
	return s.campaign, nil
}

func (s *fakeActionScope) ValidatePromotableInstagramPost(
	_ context.Context, accountID, adAccountID string, _ json.RawMessage,
) error {
	s.instagramCalls++
	if s.instagramErr != nil {
		return s.instagramErr
	}
	if s.blocked || accountID != actionTestAccount || adAccountID != s.adAccount.ID {
		return pgx.ErrNoRows
	}
	return nil
}

type fakeActionRepository struct {
	mu                    sync.Mutex
	proposal              *ActionProposal
	lastInsert            actionProposalInsert
	policy                *ActionPolicy
	executionStarts       int
	completions           int
	lastAcknowledgedSpend bool
}

func newActionTestDependencies() (*fakeActionRepository, *fakeActionScope) {
	daily := 50.0
	return &fakeActionRepository{}, &fakeActionScope{
		adAccount: AdAccount{
			ID: actionTestAdAccount, AccountID: actionTestResource,
			MetaAdAccountID: "act_987654321", Name: "Conta Principal", Currency: "BRL",
		},
		campaign: Campaign{
			ID: actionTestCampaign, AccountID: actionTestResource,
			AdAccountID: actionTestAdAccount, MetaCampaignID: "123456789",
			Name: "Campanha atual", DailyBudget: &daily,
		},
	}
}

func (r *fakeActionRepository) CreateActionProposal(
	_ context.Context,
	input actionProposalInsert,
) (ActionProposal, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastInsert = input
	if r.proposal != nil {
		if r.proposal.IdempotencyKey == input.IdempotencyKey {
			if r.proposal.RequestHash != input.RequestHash {
				return ActionProposal{}, false, ErrActionIdempotencyConflict
			}
			return *r.proposal, false, nil
		}
	}
	now := time.Now().UTC()
	proposal := ActionProposal{
		ID: actionTestProposal, AccountID: input.AccountID,
		ResourceAccountID: input.ResourceAccountID, AdAccountID: input.AdAccount.ID,
		MetaAdAccountID: input.AdAccount.MetaAdAccountID, AdAccountName: input.AdAccount.Name,
		Currency: input.AdAccount.Currency, Action: input.Action, Source: input.Source,
		Payload: input.Payload, Summary: input.Summary, RequestHash: input.RequestHash,
		IdempotencyKey: input.IdempotencyKey, Status: ActionPending,
		SourceBound:    input.Source == ActionSourceManual,
		ResultSnapshot: json.RawMessage(`{}`), CreatedAt: now,
		ExpiresAt: now.Add(30 * time.Minute), UpdatedAt: now,
	}
	if input.SourceConversationID != "" {
		proposal.SourceConversationID = &input.SourceConversationID
	}
	if input.SourceMessageID != "" {
		proposal.SourceMessageID = &input.SourceMessageID
	}
	if input.TargetCampaign != nil {
		proposal.TargetCampaignID = &input.TargetCampaign.ID
		proposal.TargetMetaCampaignID = input.TargetCampaign.MetaCampaignID
	}
	r.proposal = &proposal
	return proposal, true, nil
}

func (r *fakeActionRepository) GetActionProposal(_ context.Context, accountID, proposalID string) (ActionProposal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.proposal == nil || r.proposal.AccountID != accountID || r.proposal.ID != proposalID {
		return ActionProposal{}, pgx.ErrNoRows
	}
	return *r.proposal, nil
}

func (r *fakeActionRepository) ListActionProposals(_ context.Context, accountID string, _ int) ([]ActionProposal, error) {
	proposal, err := r.GetActionProposal(context.Background(), accountID, actionTestProposal)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []ActionProposal{}, nil
		}
		return nil, err
	}
	return []ActionProposal{proposal}, nil
}

func (r *fakeActionRepository) BindAssistantActionProposal(
	_ context.Context,
	accountID, proposalID, conversationID, messageID, _ string,
) (ActionProposal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.proposal == nil || r.proposal.AccountID != accountID || r.proposal.ID != proposalID {
		return ActionProposal{}, pgx.ErrNoRows
	}
	if r.proposal.Source != ActionSourceAssistant || r.proposal.SourceConversationID == nil ||
		r.proposal.SourceMessageID == nil || *r.proposal.SourceConversationID != conversationID ||
		*r.proposal.SourceMessageID != messageID {
		return ActionProposal{}, ErrActionValidation
	}
	if r.proposal.Status != ActionPending && !r.proposal.SourceBound {
		return ActionProposal{}, ErrActionSourceUnbound
	}
	r.proposal.SourceBound = true
	r.proposal.UpdatedAt = time.Now().UTC()
	return *r.proposal, nil
}

func (r *fakeActionRepository) CancelActionProposal(
	_ context.Context,
	accountID, proposalID, _ string, cancellationKey string,
) (ActionProposal, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.proposal == nil || r.proposal.AccountID != accountID || r.proposal.ID != proposalID {
		return ActionProposal{}, false, pgx.ErrNoRows
	}
	if r.proposal.Status == ActionCancelled {
		if r.proposal.CancellationIdempotencyKey == nil ||
			*r.proposal.CancellationIdempotencyKey != cancellationKey {
			return ActionProposal{}, false, ErrActionIdempotencyConflict
		}
		return *r.proposal, false, nil
	}
	if r.proposal.Status != ActionPending {
		return ActionProposal{}, false, ErrActionNotCancellable
	}
	now := time.Now().UTC()
	r.proposal.Status = ActionCancelled
	r.proposal.CancellationIdempotencyKey = &cancellationKey
	r.proposal.CompletedAt = &now
	r.proposal.UpdatedAt = now
	return *r.proposal, true, nil
}

func (r *fakeActionRepository) CancelAssistantConversationActions(
	_ context.Context,
	accountID, conversationID, _ string,
) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.proposal == nil || r.proposal.AccountID != accountID ||
		r.proposal.SourceConversationID == nil || *r.proposal.SourceConversationID != conversationID ||
		r.proposal.Status != ActionPending {
		return 0, nil
	}
	key := "assistant-conversation-delete:" + conversationID + ":" + r.proposal.ID
	now := time.Now().UTC()
	r.proposal.Status = ActionCancelled
	r.proposal.CancellationIdempotencyKey = &key
	r.proposal.CompletedAt = &now
	r.proposal.UpdatedAt = now
	return 1, nil
}

func (r *fakeActionRepository) ExpireActionProposal(
	_ context.Context,
	accountID, proposalID string,
) (ActionProposal, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.proposal == nil || r.proposal.AccountID != accountID || r.proposal.ID != proposalID {
		return ActionProposal{}, false, pgx.ErrNoRows
	}
	if r.proposal.Status != ActionPending || r.proposal.ExpiresAt.After(time.Now()) {
		return *r.proposal, false, nil
	}
	now := time.Now().UTC()
	r.proposal.Status = ActionExpired
	r.proposal.CompletedAt = &now
	r.proposal.UpdatedAt = now
	return *r.proposal, true, nil
}

func (r *fakeActionRepository) BeginActionExecution(
	_ context.Context,
	accountID, proposalID, actorID, confirmationKey string,
	acknowledgeSpend bool,
) (ActionProposal, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.proposal == nil || r.proposal.AccountID != accountID || r.proposal.ID != proposalID {
		return ActionProposal{}, false, pgx.ErrNoRows
	}
	if r.proposal.Status != ActionPending {
		return *r.proposal, false, nil
	}
	if r.proposal.Source == ActionSourceAssistant && !r.proposal.SourceBound {
		return ActionProposal{}, false, ErrActionSourceUnbound
	}
	if !r.proposal.ExpiresAt.After(time.Now()) {
		now := time.Now().UTC()
		r.proposal.Status = ActionExpired
		r.proposal.CompletedAt = &now
		r.proposal.UpdatedAt = now
		return *r.proposal, false, nil
	}
	now := time.Now().UTC()
	r.proposal.Status = ActionExecuting
	r.proposal.ConfirmationIdempotencyKey = &confirmationKey
	r.proposal.ConfirmedByUserID = &actorID
	r.proposal.ConfirmedAt = &now
	r.proposal.ExecutionStartedAt = &now
	r.executionStarts++
	r.lastAcknowledgedSpend = acknowledgeSpend
	return *r.proposal, true, nil
}

func (r *fakeActionRepository) CompleteActionExecution(
	_ context.Context,
	accountID, proposalID string,
	outcome ActionExecutionOutcome,
) (ActionProposal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.proposal == nil || r.proposal.AccountID != accountID || r.proposal.ID != proposalID {
		return ActionProposal{}, pgx.ErrNoRows
	}
	if r.proposal.Status != ActionExecuting {
		return *r.proposal, nil
	}
	now := time.Now().UTC()
	applyFakeActionOutcome(r.proposal, outcome, now)
	r.completions++
	return *r.proposal, nil
}

func (r *fakeActionRepository) ReconcileActionExecution(
	_ context.Context,
	accountID, proposalID, _ string,
	outcome ActionExecutionOutcome,
) (ActionProposal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.proposal == nil || r.proposal.AccountID != accountID || r.proposal.ID != proposalID {
		return ActionProposal{}, pgx.ErrNoRows
	}
	if r.proposal.Status != ActionExecuting && r.proposal.Status != ActionUnknown {
		return *r.proposal, nil
	}
	now := time.Now().UTC()
	applyFakeActionOutcome(r.proposal, outcome, now)
	r.proposal.ReconciledAt = &now
	return *r.proposal, nil
}

func applyFakeActionOutcome(proposal *ActionProposal, outcome ActionExecutionOutcome, now time.Time) {
	proposal.Status = outcome.Status
	proposal.ExternalEntityID = outcome.ExternalEntityID
	proposal.ResultSnapshot = outcome.Result
	proposal.ErrorCode = outcome.ErrorCode
	proposal.ErrorMessage = outcome.ErrorMessage
	proposal.CompletedAt = &now
	proposal.UpdatedAt = now
}

func (r *fakeActionRepository) GetActionPolicy(_ context.Context, accountID, adAccountID string) (ActionPolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.policy == nil || r.policy.AccountID != accountID || r.policy.AdAccountID != adAccountID {
		return ActionPolicy{}, pgx.ErrNoRows
	}
	return *r.policy, nil
}

func (r *fakeActionRepository) UpsertActionPolicy(
	_ context.Context,
	accountID string,
	adAccount AdAccount,
	actorID string,
	input ActionPolicyInput,
) (ActionPolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	policy := ActionPolicy{
		ID: "99999999-9999-4999-8999-999999999999", AccountID: accountID,
		AdAccountID: adAccount.ID, Currency: adAccount.Currency,
		MaxDailyBudget: input.MaxDailyBudget, MaxLifetimeBudget: input.MaxLifetimeBudget,
		AllowCreate: input.AllowCreate, AllowDuplicate: input.AllowDuplicate,
		AllowResume: input.AllowResume, UpdatedByUserID: &actorID,
		CreatedAt: now, UpdatedAt: now,
	}
	r.policy = &policy
	return policy, nil
}

type fakeActionExecutor struct {
	supported        map[ActionKind]bool
	outcome          ActionExecutionOutcome
	executeErr       error
	reconcileOutcome ActionExecutionOutcome
	reconcileErr     error
	executeCalls     int
	reconcileCalls   int
}

func (e *fakeActionExecutor) Supports(action ActionKind) bool { return e.supported[action] }

func (e *fakeActionExecutor) Execute(_ context.Context, _ ActionProposal) (ActionExecutionOutcome, error) {
	e.executeCalls++
	return e.outcome, e.executeErr
}

func (e *fakeActionExecutor) Reconcile(_ context.Context, _ ActionProposal) (ActionExecutionOutcome, error) {
	e.reconcileCalls++
	return e.reconcileOutcome, e.reconcileErr
}
