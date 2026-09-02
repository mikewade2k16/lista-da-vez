package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	omnichannelmodule "github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel"
)

// waitEvent le um Event da subscription com timeout curto; ok=false quando nada chega.
func waitEvent(t *testing.T, sub *Subscription) (Event, bool) {
	t.Helper()
	select {
	case ev, open := <-sub.Events():
		return ev, open
	case <-time.After(200 * time.Millisecond):
		return Event{}, false
	}
}

func requireOpaqueOmnichannelEvent(t *testing.T, event Event, accountID, reason string) {
	t.Helper()
	if event.Type != omnichannelInvalidateEventType {
		t.Fatalf("Type = %q, queria %q", event.Type, omnichannelInvalidateEventType)
	}
	if event.AccountID != accountID {
		t.Errorf("AccountID = %q, queria %q", event.AccountID, accountID)
	}
	if event.ResourceID != "" {
		t.Errorf("ResourceID = %q, queria vazio no canal account-wide", event.ResourceID)
	}
	if len(event.Payload) != 3 {
		t.Fatalf("payload possui %d campos, queria exatamente 3: %#v", len(event.Payload), event.Payload)
	}
	for key := range event.Payload {
		switch key {
		case "eventId", "reason", "occurredAt":
		default:
			t.Errorf("campo nao permitido no payload opaco: %q", key)
		}
	}
	if got := event.Payload["reason"]; got != reason {
		t.Errorf("reason = %v, queria %q", got, reason)
	}
	eventID, ok := event.Payload["eventId"].(string)
	if !ok || !strings.HasPrefix(eventID, "omi_") || len(eventID) != 36 {
		t.Errorf("eventId nao e opaco/normalizado: %#v", event.Payload["eventId"])
	}
	occurredAt, ok := event.Payload["occurredAt"].(string)
	if !ok {
		t.Fatalf("occurredAt = %#v, queria string RFC3339", event.Payload["occurredAt"])
	}
	if _, err := time.Parse(time.RFC3339Nano, occurredAt); err != nil {
		t.Errorf("occurredAt = %q, queria RFC3339: %v", occurredAt, err)
	}
}

// O produtor legado pode enviar o shape rico inteiro. O boundary deve descarta-lo integralmente,
// publicar apenas invalidacao e manter o isolamento do topico por conta.
func TestPublishOmnichannelEvent_LegacyProducerBecomesOpaqueInvalidation(t *testing.T) {
	hub := NewHub()
	service := NewService(nil, nil, nil, nil, hub)

	subA := hub.Subscribe(omnichannelAccountTopic("acc-1"), tasksSubscriptionBuffer)
	defer subA.Close()
	subB := hub.Subscribe(omnichannelAccountTopic("acc-2"), tasksSubscriptionBuffer)
	defer subB.Close()

	const occurredAt = "2026-08-27T14:15:16.123456789-03:00"
	service.PublishOmnichannelEvent(context.Background(), omnichannelmodule.RealtimeEvent{
		Type:       omnichannelmodule.RealtimeEventMessageCreated,
		AccountID:  " acc-1 ",
		ResourceID: "message-secret-123",
		Payload: map[string]any{
			"messageId":      "message-secret-123",
			"conversationId": "conversation-secret-456",
			"instanceId":     "instance-secret-789",
			"phone":          "+5511999999999",
			"preview":        "conteudo privado",
			"text":           "mensagem confidencial",
			"mediaUrl":       "data:image/png;base64,SECRETBASE64",
			"occurredAt":     occurredAt,
		},
	})

	event, ok := waitEvent(t, subA)
	if !ok {
		t.Fatal("acc-1 nao recebeu a invalidacao")
	}
	requireOpaqueOmnichannelEvent(t, event, "acc-1", omnichannelInvalidateReasonMessageChanged)

	wantTime, err := time.Parse(time.RFC3339Nano, occurredAt)
	if err != nil {
		t.Fatal(err)
	}
	gotTime, _ := time.Parse(time.RFC3339Nano, event.Payload["occurredAt"].(string))
	if !gotTime.Equal(wantTime) {
		t.Errorf("occurredAt = %s, queria %s", gotTime, wantTime)
	}

	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("serializar evento publicado: %v", err)
	}
	serialized := string(encoded)
	for _, forbidden := range []string{
		"messageId", "conversationId", "instanceId", "phone", "preview", "resourceId",
		"message-secret-123", "conversation-secret-456", "instance-secret-789",
		"+5511999999999", "conteudo privado", "mensagem confidencial", "SECRETBASE64",
	} {
		if strings.Contains(serialized, forbidden) {
			t.Errorf("evento publicado vazou %q: %s", forbidden, serialized)
		}
	}

	if _, ok := waitEvent(t, subB); ok {
		t.Error("acc-2 recebeu evento de acc-1 (vazamento cross-tenant)")
	}
}

func TestPublishOmnichannelEvent_AllLegacyTypesUseMessageChanged(t *testing.T) {
	legacyTypes := []string{
		omnichannelmodule.RealtimeEventMessageCreated,
		omnichannelmodule.RealtimeEventMessageUpdated,
		omnichannelmodule.RealtimeEventConversationUpdated,
	}

	for _, eventType := range legacyTypes {
		t.Run(eventType, func(t *testing.T) {
			hub := NewHub()
			service := NewService(nil, nil, nil, nil, hub)
			sub := hub.Subscribe(omnichannelAccountTopic("acc-1"), tasksSubscriptionBuffer)
			defer sub.Close()

			service.PublishOmnichannelEvent(context.Background(), omnichannelmodule.RealtimeEvent{
				Type: eventType, AccountID: "acc-1", ResourceID: "sensitive-id",
				Payload: map[string]any{"conversationId": "sensitive-conversation"},
			})

			event, ok := waitEvent(t, sub)
			if !ok {
				t.Fatal("evento legado valido foi descartado")
			}
			requireOpaqueOmnichannelEvent(t, event, "acc-1", omnichannelInvalidateReasonMessageChanged)
		})
	}
}

func TestPublishOmnichannelEvent_AcceptsOnlyClosedReasons(t *testing.T) {
	reasons := []string{
		omnichannelInvalidateReasonMessageChanged,
		omnichannelInvalidateReasonHistoryReset,
		omnichannelInvalidateReasonAccessScopeChanged,
	}

	for _, reason := range reasons {
		t.Run(reason, func(t *testing.T) {
			hub := NewHub()
			service := NewService(nil, nil, nil, nil, hub)
			sub := hub.Subscribe(omnichannelAccountTopic("acc-1"), tasksSubscriptionBuffer)
			defer sub.Close()

			service.PublishOmnichannelEvent(context.Background(), omnichannelmodule.RealtimeEvent{
				Type: omnichannelInvalidateEventType, AccountID: "acc-1",
				Payload: map[string]any{
					"eventId":        "producer-id-must-not-pass-through",
					"reason":         reason,
					"occurredAt":     "2026-08-27T17:15:16Z",
					"conversationId": "must-be-dropped",
				},
			})

			event, ok := waitEvent(t, sub)
			if !ok {
				t.Fatal("invalidacao com motivo permitido foi descartada")
			}
			requireOpaqueOmnichannelEvent(t, event, "acc-1", reason)
			if event.Payload["eventId"] == "producer-id-must-not-pass-through" {
				t.Error("eventId do produtor atravessou sem opacificacao")
			}
		})
	}
}

// A mesma publicacao entrega o mesmo ID a todos os assinantes, permitindo dedupe entre abas. Uma
// nova publicacao recebe outro ID e nenhum identificador nasce de dados do dominio.
func TestPublishOmnichannelEvent_EventIDSupportsDedupeWithoutDomainFingerprint(t *testing.T) {
	hub := NewHub()
	service := NewService(nil, nil, nil, nil, hub)
	subA := hub.Subscribe(omnichannelAccountTopic("acc-1"), tasksSubscriptionBuffer)
	defer subA.Close()
	subB := hub.Subscribe(omnichannelAccountTopic("acc-1"), tasksSubscriptionBuffer)
	defer subB.Close()

	event := omnichannelmodule.RealtimeEvent{
		Type:       omnichannelmodule.RealtimeEventMessageUpdated,
		AccountID:  "acc-1",
		ResourceID: "message-1",
		Payload: map[string]any{
			"messageId":  "message-1",
			"status":     "sent",
			"occurredAt": "2026-08-27T17:15:16Z",
		},
	}
	service.PublishOmnichannelEvent(context.Background(), event)

	first, ok := waitEvent(t, subA)
	if !ok {
		t.Fatal("primeira invalidacao nao recebida")
	}
	second, ok := waitEvent(t, subB)
	if !ok {
		t.Fatal("segunda aba nao recebeu a invalidacao")
	}
	if first.Payload["eventId"] != second.Payload["eventId"] {
		t.Errorf("mesma publicacao gerou eventIds diferentes entre assinantes: %v != %v", first.Payload["eventId"], second.Payload["eventId"])
	}

	service.PublishOmnichannelEvent(context.Background(), event)
	third, ok := waitEvent(t, subA)
	if !ok {
		t.Fatal("nova publicacao nao recebida")
	}
	if first.Payload["eventId"] == third.Payload["eventId"] {
		t.Error("duas publicacoes independentes reutilizaram eventId")
	}
}

func TestPublishOmnichannelEvent_FailsClosed(t *testing.T) {
	hub := NewHub()
	service := NewService(nil, nil, nil, nil, hub)
	sub := hub.Subscribe(omnichannelAccountTopic("acc-1"), tasksSubscriptionBuffer)
	defer sub.Close()

	invalidEvents := []omnichannelmodule.RealtimeEvent{
		{Type: "", AccountID: "acc-1"},
		{Type: "message.deleted", AccountID: "acc-1", Payload: map[string]any{"reason": "message_changed"}},
		{Type: omnichannelInvalidateEventType, AccountID: "acc-1", Payload: nil},
		{Type: omnichannelInvalidateEventType, AccountID: "acc-1", Payload: map[string]any{"reason": "all_data_changed"}},
		{Type: omnichannelInvalidateEventType, AccountID: "acc-1", Payload: map[string]any{"reason": 123}},
		{Type: omnichannelmodule.RealtimeEventMessageCreated, AccountID: ""},
	}
	for _, invalid := range invalidEvents {
		service.PublishOmnichannelEvent(context.Background(), invalid)
	}

	if _, ok := waitEvent(t, sub); ok {
		t.Error("boundary publicou evento invalido")
	}

	// Campos ricos ou nao serializaveis sao descartados; nao conseguem bloquear uma invalidacao
	// valida porque o ID opaco nunca e derivado do payload de dominio.
	service.PublishOmnichannelEvent(context.Background(), omnichannelmodule.RealtimeEvent{
		Type: omnichannelInvalidateEventType, AccountID: "acc-1",
		Payload: map[string]any{"reason": "history_reset", "invalid": make(chan int)},
	})
	if event, ok := waitEvent(t, sub); !ok {
		t.Error("campo descartavel bloqueou invalidacao valida")
	} else {
		requireOpaqueOmnichannelEvent(t, event, "acc-1", omnichannelInvalidateReasonHistoryReset)
	}

	// Hub ausente continua sendo no-op seguro.
	NewService(nil, nil, nil, nil, nil).PublishOmnichannelEvent(context.Background(), omnichannelmodule.RealtimeEvent{
		Type: omnichannelmodule.RealtimeEventMessageCreated, AccountID: "acc-1",
	})
}

// A mudanca do payload nao altera o gate existente: outra conta vira 404, e mesmo platform_admin
// precisa validar a existencia/atividade da conta antes do bypass. Pool nil representa indisponivel.
func TestResolveOmnichannelAccount_PreservesAuthorizationBoundary(t *testing.T) {
	service := NewService(nil, nil, nil, nil, NewHub())

	forged := httptest.NewRequestWithContext(context.Background(), "GET", "/v1/realtime/omnichannel?scope=account&accountId=acc-2", nil)
	_, err := service.resolveOmnichannelAccount(forged.Context(), auth.Principal{
		UserID: "user-1", Role: auth.RoleConsultant, AccountID: "acc-1",
	}, forged)
	if !errors.Is(err, errRealtimeNotFound) {
		t.Errorf("outra conta retornou %v, queria errRealtimeNotFound", err)
	}

	own := httptest.NewRequestWithContext(context.Background(), "GET", "/v1/realtime/omnichannel?scope=account&accountId=acc-1", nil)
	_, err = service.resolveOmnichannelAccount(own.Context(), auth.Principal{
		UserID: "user-1", Role: auth.RoleConsultant, AccountID: "acc-1",
	}, own)
	if !errors.Is(err, errRealtimeUnavailable) {
		t.Errorf("conta propria sem backend de autorizacao retornou %v, queria errRealtimeUnavailable", err)
	}

	admin := httptest.NewRequestWithContext(context.Background(), "GET", "/v1/realtime/omnichannel?scope=account&accountId=acc-2", nil)
	_, err = service.resolveOmnichannelAccount(admin.Context(), auth.Principal{
		UserID: "admin-1", Role: auth.RolePlatformAdmin,
	}, admin)
	if !errors.Is(err, errRealtimeUnavailable) {
		t.Errorf("platform_admin contornou validacao da conta: %v", err)
	}

	if err := service.authorizeOmnichannelAccount(context.Background(), auth.Principal{}, ""); !errors.Is(err, errRealtimeValidation) {
		t.Errorf("conta vazia retornou %v, queria errRealtimeValidation", err)
	}
}

func TestOmnichannelSocketClosesBeforeNextWriteWhenAccessChanges(t *testing.T) {
	hub := NewHub()
	service := NewService(nil, nil, nil, nil, hub)
	var allowed atomic.Bool
	allowed.Store(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		service.serveSubscriptionSocket(w, r, "omnichannel:account:acc-1", 2, Event{
			Type: EventTypeConnected, AccountID: "acc-1", SavedAt: time.Now().UTC(),
		}, nil, nil, service.readPumpWithRateLimit, func(context.Context) error {
			if allowed.Load() {
				return nil
			}
			return errRealtimeForbidden
		})
	}))
	defer server.Close()

	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	var connected Event
	if err := connection.ReadJSON(&connected); err != nil {
		t.Fatalf("connected event: %v", err)
	}
	if connected.Type != EventTypeConnected {
		t.Fatalf("connected type=%q", connected.Type)
	}

	allowed.Store(false)
	hub.Publish("omnichannel:account:acc-1", Event{Type: omnichannelInvalidateEventType, AccountID: "acc-1"})
	_ = connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = connection.ReadMessage()
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) || closeErr.Code != websocket.ClosePolicyViolation || closeErr.Text != "access_changed" {
		t.Fatalf("socket close=%v", err)
	}
}
