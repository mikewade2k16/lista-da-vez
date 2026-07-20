package realtime

import (
	"context"
	"testing"
	"time"

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

// TestPublishOmnichannelEvent_DeliversToAccountTopic prova o transporte da F5: um evento
// publicado pelo call-site chega no canal omnichannel:account:{id} com Type/ResourceID/Payload
// preservados, a midia data: e sanitizada para null (cinto e suspensorio) e outra conta NAO
// recebe nada (isolamento). Sem rede, sem credencial: exercita o hub em memoria direto.
func TestPublishOmnichannelEvent_DeliversToAccountTopic(t *testing.T) {
	hub := NewHub()
	service := NewService(nil, nil, nil, nil, hub)

	subA := hub.Subscribe(omnichannelAccountTopic("acc-1"), tasksSubscriptionBuffer)
	defer subA.Close()
	subB := hub.Subscribe(omnichannelAccountTopic("acc-2"), tasksSubscriptionBuffer)
	defer subB.Close()

	service.PublishOmnichannelEvent(context.Background(), omnichannelmodule.RealtimeEvent{
		Type:       omnichannelmodule.RealtimeEventMessageCreated,
		AccountID:  "acc-1",
		ResourceID: "msg-1",
		Payload: map[string]any{
			"id":             "msg-1",
			"conversationId": "conv-1",
			"direction":      "INBOUND",
			"mediaUrl":       "data:image/png;base64,SECRETBASE64",
		},
	})

	ev, ok := waitEvent(t, subA)
	if !ok {
		t.Fatal("acc-1 nao recebeu o evento publicado")
	}
	if ev.Type != EventTypeOmnichannelMessageCreated {
		t.Errorf("Type = %q, queria %q", ev.Type, EventTypeOmnichannelMessageCreated)
	}
	if ev.AccountID != "acc-1" {
		t.Errorf("AccountID = %q, queria acc-1", ev.AccountID)
	}
	if ev.ResourceID != "msg-1" {
		t.Errorf("ResourceID = %q, queria msg-1", ev.ResourceID)
	}
	if got := ev.Payload["mediaUrl"]; got != nil {
		t.Errorf("mediaUrl = %v, queria nil (data URL sanitizada — nunca base64 no WS)", got)
	}
	if got := ev.Payload["conversationId"]; got != "conv-1" {
		t.Errorf("conversationId = %v, queria conv-1", got)
	}

	// Isolamento: a conta acc-2 nunca recebe o evento de acc-1.
	if _, ok := waitEvent(t, subB); ok {
		t.Error("acc-2 recebeu evento de acc-1 (vazamento cross-tenant)")
	}
}

// TestPublishOmnichannelEvent_NoopGuards prova os no-ops: Type ou AccountID vazio nao publica.
func TestPublishOmnichannelEvent_NoopGuards(t *testing.T) {
	hub := NewHub()
	service := NewService(nil, nil, nil, nil, hub)
	sub := hub.Subscribe(omnichannelAccountTopic("acc-1"), tasksSubscriptionBuffer)
	defer sub.Close()

	service.PublishOmnichannelEvent(context.Background(), omnichannelmodule.RealtimeEvent{Type: "", AccountID: "acc-1"})
	service.PublishOmnichannelEvent(context.Background(), omnichannelmodule.RealtimeEvent{Type: omnichannelmodule.RealtimeEventMessageCreated, AccountID: ""})

	if _, ok := waitEvent(t, sub); ok {
		t.Error("evento com Type/AccountID vazio nao deveria publicar")
	}
}
