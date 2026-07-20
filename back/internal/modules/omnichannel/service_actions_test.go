package omnichannel

import (
	"errors"
	"testing"
)

// TestStatusEvent prova a projecao inversa status->evento (F8 Contrato 3): os 3 valores do
// front, o no-op de OPEN em conversa nao-fechada e o 400 para status desconhecido.
func TestStatusEvent(t *testing.T) {
	cases := []struct {
		name    string
		status  string
		current ConversationStatus
		wantEv  Event
		wantNo  bool
		wantErr error
	}{
		{"closed", "CLOSED", StatusOpen, EventConvClose, false, nil},
		{"pending", "PENDING", StatusOpen, EventHumanPending, false, nil},
		{"reopen from closed", "OPEN", StatusClosed, EventConvReopen, false, nil},
		{"open on open is noop", "OPEN", StatusOpen, "", true, nil},
		{"open on pending is noop", "OPEN", StatusPending, "", true, nil},
		{"case-insensitive", "open", StatusClosed, EventConvReopen, false, nil},
		{"trims", "  CLOSED  ", StatusOpen, EventConvClose, false, nil},
		{"unknown status", "ARCHIVED", StatusOpen, "", false, ErrInvalidBody},
		{"empty status", "", StatusOpen, "", false, ErrInvalidBody},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev, noOp, err := statusEvent(c.status, c.current)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
			if c.wantErr != nil {
				return
			}
			if ev != c.wantEv {
				t.Errorf("event = %q, want %q", ev, c.wantEv)
			}
			if noOp != c.wantNo {
				t.Errorf("noOp = %v, want %v", noOp, c.wantNo)
			}
		})
	}
}

// TestSameAssignee cobre a comparacao de responsavel usada para auditar so quando muda.
func TestSameAssignee(t *testing.T) {
	a, b := "user-a", "user-b"
	cases := []struct {
		name string
		x, y *string
		want bool
	}{
		{"both nil", nil, nil, true},
		{"left nil", nil, &a, false},
		{"right nil", &a, nil, false},
		{"equal", &a, &a, true},
		{"different", &a, &b, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sameAssignee(c.x, c.y); got != c.want {
				t.Errorf("sameAssignee = %v, want %v", got, c.want)
			}
		})
	}
}

// TestValidMessageIDs prova o contrato 1..100 e a rejeicao de id vazio.
func TestValidMessageIDs(t *testing.T) {
	long := make([]string, maxForwardMessages+1)
	for i := range long {
		long[i] = "id"
	}
	full := make([]string, maxForwardMessages)
	for i := range full {
		full[i] = "id"
	}
	cases := []struct {
		name string
		ids  []string
		want bool
	}{
		{"empty", nil, false},
		{"one", []string{"a"}, true},
		{"exactly 100", full, true},
		{"over 100", long, false},
		{"blank id", []string{"a", "  "}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := validMessageIDs(c.ids); got != c.want {
				t.Errorf("validMessageIDs = %v, want %v", got, c.want)
			}
		})
	}
}

// TestForwardInput prova a reconstrucao do body a partir da mensagem de origem e a chave de
// idempotencia estavel por (destino, origem).
func TestForwardInput(t *testing.T) {
	url := "https://cdn.example/x.jpg"
	mime := "image/jpeg"
	caption := "legenda"
	src := MessageView{
		MessageType:   "IMAGE",
		Content:       "oi",
		MediaURL:      &url,
		MediaMimeType: &mime,
		MediaCaption:  &caption,
	}
	in := forwardInput(src, "target-1", "msg-9")
	if in.Type != "IMAGE" || in.Content != "oi" || in.MediaURL != url || in.MediaCaption != caption {
		t.Fatalf("campos nao reconstruidos: %+v", in)
	}
	if in.IdempotencyKey != "forward:target-1:msg-9" {
		t.Errorf("idempotencyKey = %q", in.IdempotencyKey)
	}
}

// TestConversationUpdatedPayload prova que o payload traz instanceName (status/assign emitem
// COM instancia) e sanitiza data: URL do preview (nunca base64 no WS).
func TestConversationUpdatedPayload(t *testing.T) {
	instance := "inst-a"
	dataURL := "data:image/png;base64,AAAA"
	view := ConversationView{
		ID:           "conv-1",
		InstanceName: &instance,
		LastMessage:  &LastMessageView{ID: "m1", MediaURL: &dataURL},
	}
	payload := conversationUpdatedPayload(view)
	if payload["instanceName"] != "inst-a" {
		t.Errorf("instanceName ausente/errado: %v", payload["instanceName"])
	}
	lm, ok := payload["lastMessage"].(map[string]any)
	if !ok {
		t.Fatalf("lastMessage nao e objeto: %T", payload["lastMessage"])
	}
	if lm["mediaUrl"] != nil {
		t.Errorf("mediaUrl data: deveria virar null, veio %v", lm["mediaUrl"])
	}

	// URL normal (nao data:) e preservada.
	normalURL := "/v1/omnichannel/conversations/conv-1/messages/m1/media"
	view.LastMessage.MediaURL = &normalURL
	payload = conversationUpdatedPayload(view)
	lm, _ = payload["lastMessage"].(map[string]any)
	if lm["mediaUrl"] != normalURL {
		t.Errorf("mediaUrl normal deveria ser preservada, veio %v", lm["mediaUrl"])
	}
}
