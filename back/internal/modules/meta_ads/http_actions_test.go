package metaads

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManualActionBodyCannotImpersonateAssistantSource(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(context.Background(), "POST", "/v1/meta-ads/action-proposals", strings.NewReader(
		`{"action":"pause_campaign","adAccountId":"`+actionTestAdAccount+`",`+
			`"payload":{"campaignId":"`+actionTestCampaign+`"},`+
			`"sourceConversationId":"`+actionTestConversation+`"}`,
	))
	response := httptest.NewRecorder()
	var input ActionProposalInput

	if err := decodeActionHTTPBody(response, request, &input); err == nil {
		t.Fatal("browser body with assistant source fields must be rejected")
	}
}

func TestConfirmationBodyIsOptionalButStrict(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		wantAck bool
		wantErr bool
	}{
		{name: "empty"},
		{name: "reinforced", body: `{"acknowledgeSpend":true}`, wantAck: true},
		{name: "unknown field", body: `{"acknowledgeSpend":true,"accountId":"` + actionTestAccount + `"}`, wantErr: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(context.Background(), "POST", "/confirm", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			var input ActionConfirmationInput
			err := decodeOptionalActionHTTPBody(response, request, &input)
			if (err != nil) != test.wantErr || (err == nil && input.AcknowledgeSpend != test.wantAck) {
				t.Fatalf("input=%#v err=%v", input, err)
			}
		})
	}
}
