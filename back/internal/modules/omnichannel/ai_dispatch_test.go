package omnichannel

import (
	"strings"
	"testing"
)

func TestAIDispatchIdempotencyKeyIsStableAndContainsNoPII(t *testing.T) {
	conversationID := "11111111-1111-4111-8111-111111111111"
	got := aiDispatchIdempotencyKey(" "+conversationID+" ", 4)
	if got != "brain:"+conversationID+":4" {
		t.Fatalf("key=%q", got)
	}
	if strings.Contains(got, "@") || strings.Contains(got, "+") {
		t.Fatalf("key contains a PII-like token: %q", got)
	}
	if got == aiDispatchIdempotencyKey(conversationID, 5) {
		t.Fatal("different generations must not share an idempotency key")
	}
}

func TestAIDispatchOrderingKeyDoesNotBlockConversationIngress(t *testing.T) {
	const conversationID = "11111111-1111-4111-8111-111111111111"
	got := aiDispatchOrderingKey(conversationID)
	if got == conversationID {
		t.Fatal("AI dispatch must not share the ingress FIFO head-of-line")
	}
	if got != "ai-dispatch:"+conversationID {
		t.Fatalf("ordering key = %q", got)
	}
}

func TestAIDispatchStatusesAreClosed(t *testing.T) {
	valid := []AIDispatchStatus{
		AIDispatchBuffering, AIDispatchQueued, AIDispatchProcessing,
		AIDispatchCompleted, AIDispatchCancelled, AIDispatchFailed,
	}
	for _, status := range valid {
		if !validAIDispatchStatus(status) {
			t.Errorf("status %q deveria ser aceito", status)
		}
	}
	if validAIDispatchStatus("sending") {
		t.Fatal("status fora do enum foi aceito")
	}
}
