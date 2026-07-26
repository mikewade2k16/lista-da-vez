package omnichannel

import (
	"encoding/json"
	"testing"
)

func TestCustomerIntelligenceMessageSnapshotKeepsProviderNameInUserJSON(t *testing.T) {
	t.Parallel()
	raw, err := customerIntelligenceMessageSnapshot(
		"message-1",
		[]SimMessage{{ID: "message-1", Role: "contact", Text: "Oi"}},
		"  Ana do WhatsApp  ",
	)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		CurrentMessageID string `json:"currentMessageId"`
		Contact          struct {
			DisplayName string `json:"displayName"`
		} `json:"contact"`
		Messages []SimMessage `json:"messages"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.CurrentMessageID != "message-1" ||
		payload.Contact.DisplayName != "Ana do WhatsApp" ||
		len(payload.Messages) != 1 {
		t.Fatalf("snapshot inesperado: %#v", payload)
	}
}
