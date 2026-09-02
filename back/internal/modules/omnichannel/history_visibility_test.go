package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHistoryCutoffDefaultOnAndStrict(t *testing.T) {
	store := NewStore(nil)
	if !store.HistoryCutoffEnforced() {
		t.Fatal("zero value must enforce instance history cutoff")
	}
	expression := store.effectiveHistoryCutoffExpression("conversation")
	if !strings.Contains(expression, "history_visible_from") ||
		!strings.Contains(expression, "contact_suppressions") ||
		!strings.Contains(expression, "conversation.channel='WHATSAPP'") {
		t.Fatalf("effective cutoff missing max(instance, contact): %s", expression)
	}
	predicate := store.historyVisibleMessagePredicate("message", "conversation")
	if !strings.Contains(predicate, "message.created_at >") || strings.Contains(predicate, ">=") {
		t.Fatalf("cutoff must be strict: %s", predicate)
	}
}

func TestHistoryResetHTTPErrorContract(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "forbidden", err: ErrForbidden, status: http.StatusForbidden, code: "forbidden"},
		{name: "confirmation", err: ErrHistoryResetConfirmationMismatch, status: http.StatusUnprocessableEntity, code: "history_reset_confirmation_mismatch"},
		{name: "revision", err: ErrHistoryResetRevisionConflict, status: http.StatusConflict, code: "history_reset_revision_conflict"},
		{name: "legacy moved", err: ErrHistoryResetMoved, status: http.StatusConflict, code: "history_reset_moved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/history/reset", nil)
			writeHistoryResetError(recorder, request, tc.err)
			if recorder.Code != tc.status || !strings.Contains(recorder.Body.String(), `"code":"`+tc.code+`"`) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestHistoryResetReasonLimitCountsRunes(t *testing.T) {
	svc := &SessionService{}
	revision := int64(0)
	_, err := svc.ResetInstanceHistory(context.Background(), "11111111-1111-4111-8111-111111111111",
		"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Caller{UserID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc"},
		InstanceHistoryResetInput{Confirmation: "instance", Reason: strings.Repeat("界", 240), ExpectedRevision: &revision})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("240 runes should pass validation, got %v", err)
	}
	_, err = svc.ResetInstanceHistory(context.Background(), "11111111-1111-4111-8111-111111111111",
		"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Caller{UserID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc"},
		InstanceHistoryResetInput{Confirmation: "instance", Reason: strings.Repeat("界", 241), ExpectedRevision: &revision})
	if !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("241 runes should fail validation, got %v", err)
	}
}

func TestLegacyClearHandlerIsPermanentlyInert(t *testing.T) {
	const malformed = `{"instanceId":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"`
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/v1/omnichannel/tenant/whatsapp/conversations/clear", strings.NewReader(malformed))
	recorder := httptest.NewRecorder()

	// nil prova que o endpoint nao delega para service/store; qualquer acesso panicaria.
	handleClearConversations(nil).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict ||
		!strings.Contains(recorder.Body.String(), `"code":"history_reset_moved"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	remaining, err := io.ReadAll(request.Body)
	if err != nil || string(remaining) != malformed {
		t.Fatalf("legacy body was decoded: remaining=%q err=%v", remaining, err)
	}
}

func TestHistoryResetViewSerializesUTC(t *testing.T) {
	view := InstanceHistoryResetView{
		InstanceID:    "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		HiddenBefore:  time.Date(2026, 8, 27, 12, 0, 0, 0, time.FixedZone("BRT", -3*60*60)).UTC(),
		ResetRevision: 1,
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"hiddenBefore":"2026-08-27T15:00:00Z"`) {
		t.Fatalf("non-UTC response: %s", raw)
	}
}

func TestHistoryCutoffRollbackKeepsContactPrivacyForEveryChannel(t *testing.T) {
	store := NewStore(nil)
	store.SetHistoryCutoffEnforced(false)
	expression := store.effectiveHistoryCutoffExpression("instagram_conversation")
	if strings.Contains(expression, "history_visible_from") || !strings.Contains(expression, "contact_suppressions") {
		t.Fatalf("rollback must disable only instance cutoff: %s", expression)
	}
	predicate := store.historyVisibleConversationPredicate("instagram_conversation")
	if strings.Contains(predicate, "history_instance") || !strings.Contains(predicate, "contact_suppressions") ||
		!strings.Contains(predicate, "history_contact_message.created_at >") {
		t.Fatalf("contact privacy must remain active for Instagram: %s", predicate)
	}
}

func TestHistoryConversationPredicateKeepsInstagramOutsideInstanceCutoff(t *testing.T) {
	store := NewStore(nil)
	predicate := store.historyVisibleConversationPredicate("conversation")
	if !strings.Contains(predicate, "conversation.channel <> 'WHATSAPP'") ||
		!strings.Contains(predicate, "contact_suppressions") {
		t.Fatalf("channel/contact guards missing: %s", predicate)
	}
}

func TestHistoryCutoffEnvIsFailClosed(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{value: "", want: true},
		{value: "invalid", want: true},
		{value: "true", want: true},
		{value: "false", want: false},
		{value: "0", want: false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("OMNICHANNEL_HISTORY_CUTOFF_ENFORCED", tc.value)
			if got := historyCutoffEnforcedFromEnv(); got != tc.want {
				t.Fatalf("historyCutoffEnforcedFromEnv()=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestHistoryResetInvalidationProducerContainsNoResourceIdentifiers(t *testing.T) {
	event := newInvalidationSignal("account-secret", RealtimeInvalidationReasonHistoryReset,
		time.Date(2026, 8, 27, 12, 0, 0, 0, time.FixedZone("BRT", -3*60*60)))
	if event.Type != RealtimeEventInvalidate || event.AccountID != "account-secret" || event.ResourceID != "" {
		t.Fatalf("event envelope=%+v", event)
	}
	if len(event.Payload) != 2 || event.Payload["reason"] != RealtimeInvalidationReasonHistoryReset ||
		event.Payload["occurredAt"] != "2026-08-27T15:00:00Z" {
		t.Fatalf("opaque payload=%#v", event.Payload)
	}
	for _, forbidden := range []string{"eventId", "accountId", "instanceId", "revision", "resourceId"} {
		if _, exists := event.Payload[forbidden]; exists {
			t.Fatalf("payload exposed %q: %#v", forbidden, event.Payload)
		}
	}
}
