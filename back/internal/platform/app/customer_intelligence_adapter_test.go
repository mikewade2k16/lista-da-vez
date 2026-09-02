package app

import (
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel"
)

func TestCustomerIntelligenceRuntimeRequestPropagatesHistorySuppression(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	request := omnichannel.CustomerIntelligenceInteractionRequest{
		AccountID:               "11111111-1111-4111-8111-111111111111",
		ClientAccountID:         "22222222-2222-4222-8222-222222222222",
		ConversationID:          "33333333-3333-4333-8333-333333333333",
		DispatchID:              "dispatch-safe",
		Generation:              4,
		Channel:                 "WHATSAPP",
		DerivedMemorySuppressed: true,
	}
	got := customerIntelligenceRuntimeRequest(
		request,
		"44444444-4444-4444-8444-444444444444",
		"55555555-5555-4555-8555-555555555555",
		now,
	)
	if !got.SuppressStoredContext || got.AsOf != now ||
		got.DeadlineAt != now.Add(20*time.Second) || got.Channel != "whatsapp" ||
		got.AIGeneration != request.Generation || got.ConversationID != request.ConversationID {
		t.Fatalf("runtime request lost operational suppression/scope: %+v", got)
	}
}
