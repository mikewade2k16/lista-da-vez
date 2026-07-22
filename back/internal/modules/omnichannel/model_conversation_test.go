package omnichannel

import "testing"

func TestProjectAIStatusUsesAuthoritativeConversationState(t *testing.T) {
	tests := map[string]string{
		string(StateNew):         "idle",
		string(StateAIActive):    "analyzing",
		string(StateRouting):     "transferring",
		string(StateQueued):      "transferring",
		string(StatePending):     "awaiting_client",
		string(StateHumanActive): "human",
		string(StateClosed):      "closed",
		"unexpected":             "idle",
	}
	for state, want := range tests {
		if got := projectAIStatus(state); got != want {
			t.Fatalf("projectAIStatus(%q)=%q, want %q", state, got, want)
		}
	}
}
