package calendar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatAskRequestHashIsStableAndScopeBound(t *testing.T) {
	t.Parallel()
	target := chatTarget{mode: chatScopeClient, clientID: "11111111-1111-1111-1111-111111111111", surface: AssistantSurfaceCalendar}
	req := ChatAskRequest{ConversationID: "22222222-2222-2222-2222-222222222222", Month: "2026-08", ViaVoice: true}
	first, err := chatAskRequestHash("33333333-3333-3333-3333-333333333333", "44444444-4444-4444-4444-444444444444", "Crie um post", req, target)
	if err != nil {
		t.Fatal(err)
	}
	second, err := chatAskRequestHash("33333333-3333-3333-3333-333333333333", "44444444-4444-4444-4444-444444444444", "Crie um post", req, target)
	if err != nil {
		t.Fatal(err)
	}
	if !equalHash(first, second) {
		t.Fatal("same ask intent must have the same request hash")
	}
	target.clientID = "55555555-5555-5555-5555-555555555555"
	changed, err := chatAskRequestHash("33333333-3333-3333-3333-333333333333", "44444444-4444-4444-4444-444444444444", "Crie um post", req, target)
	if err != nil {
		t.Fatal(err)
	}
	if equalHash(first, changed) {
		t.Fatal("scope changes must conflict instead of replaying another tenant/client intent")
	}
}

func TestValidChatIdempotencyKeyRejectsWhitespaceAndBounds(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"assistant-ask:123", "assistant-confirm:message:proposal"} {
		if !validChatIdempotencyKey(key) {
			t.Fatalf("expected key %q to be valid", key)
		}
	}
	for _, key := range []string{"short", "has space", "line\nbreak"} {
		if validChatIdempotencyKey(key) {
			t.Fatalf("expected key %q to be rejected", key)
		}
	}
}

func TestProposalExecutionViewsRequireConfiguredTaskBoardAndSafeEventSnapshots(t *testing.T) {
	t.Parallel()
	task := StoredProposal{ID: "task", Kind: "task", Action: "create", Status: "pending"}
	missingSnapshot := StoredProposal{ID: "missing", Kind: "event", Action: "update", Status: "pending", Fields: ChatProposalFields{TargetID: "11111111-1111-1111-1111-111111111111"}}
	linked := StoredProposal{ID: "linked", Kind: "event", Action: "delete", Status: "pending", Fields: ChatProposalFields{TargetID: "22222222-2222-2222-2222-222222222222"}}
	configuredTask := StoredProposal{ID: "configured-task", Kind: "task", Action: "create", Status: "pending", Fields: ChatProposalFields{BoardID: "33333333-3333-3333-3333-333333333333"}}
	views := proposalExecutionViews([]StoredProposal{task, configuredTask, missingSnapshot, linked}, []AIContextEvent{{
		ID: "22222222-2222-2222-2222-222222222222", Version: 3, TaskID: "task-1",
	}})
	for _, index := range []int{0, 2} {
		if views[index].Execution == nil || views[index].Execution.CanConfirm {
			t.Fatalf("proposal %d must be unavailable: %#v", index, views[index].Execution)
		}
	}
	if views[0].Execution.ErrorCode != "tasks_not_configured" {
		t.Fatalf("task error = %q", views[0].Execution.ErrorCode)
	}
	if views[1].Execution == nil || !views[1].Execution.CanConfirm {
		t.Fatalf("configured task should be confirmable: %#v", views[1].Execution)
	}
	if views[2].Execution.ErrorCode != "proposal_snapshot_missing" {
		t.Fatalf("snapshot error = %q", views[2].Execution.ErrorCode)
	}
	if views[3].Execution == nil || !views[3].Execution.CanConfirm {
		t.Fatalf("linked event should preserve production confirmation behavior: %#v", views[3].Execution)
	}
}

func TestEditableProposalOverlayCannotReplaceTargetOrMetaAction(t *testing.T) {
	t.Parallel()
	base := ChatProposalFields{TargetID: "11111111-1111-1111-1111-111111111111", Title: "Original"}
	edited := ChatProposalFields{
		TargetID: "22222222-2222-2222-2222-222222222222",
		Title:    "Editado",
		MetaAction: &ChatProposalMetaAction{
			ActionProposalID: "33333333-3333-3333-3333-333333333333",
		},
	}
	got := overlayEditableProposalFields(base, &edited)
	if got.TargetID != base.TargetID {
		t.Fatalf("target changed to %q", got.TargetID)
	}
	if got.MetaAction != nil {
		t.Fatalf("meta action crossed into local executor: %#v", got.MetaAction)
	}
	if got.Title != "Editado" {
		t.Fatalf("editable title = %q", got.Title)
	}
}

func TestChatProposalIdempotencyConflictMapsToHTTP409(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		context.Background(), http.MethodPatch, "/v1/calendar/chat/proposals/confirm", nil,
	)
	writeChatError(recorder, request, ErrIdempotencyConflict)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
}
