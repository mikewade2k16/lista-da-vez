package evolution

import (
	"context"
	"net/http"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

func TestParseWebhookReplyContext(t *testing.T) {
	p := New("", "")
	body := []byte(`{
		"event":"messages.upsert","instance":"main","data":{
			"key":{"remoteJid":"5511999999999@s.whatsapp.net","fromMe":false,"id":"reply-1"},
			"messageTimestamp":1710000000,
			"message":{"extendedTextMessage":{"text":"resposta","contextInfo":{
				"stanzaId":"quoted-1","participant":"5511888888888@s.whatsapp.net",
				"quotedMessage":{"imageMessage":{"caption":"foto original","mimetype":"image/jpeg"}}
			}}}
		}
	}`)
	events, err := p.ParseWebhook(context.Background(), http.Header{}, body)
	if err != nil {
		t.Fatalf("ParseWebhook: %v", err)
	}
	if len(events) != 1 || events[0].Kind != channel.EventMessageReceived || events[0].Message == nil {
		t.Fatalf("evento inesperado: %+v", events)
	}
	reply := events[0].Message.Reply
	if reply == nil {
		t.Fatal("reply ausente")
	}
	if reply.ExternalMessageID != "quoted-1" || reply.ParticipantID != "5511888888888@s.whatsapp.net" {
		t.Fatalf("referencia incorreta: %+v", reply)
	}
	if reply.MessageType != "IMAGE" || reply.Content != "foto original" {
		t.Fatalf("snapshot incorreto: %+v", reply)
	}
}

func TestParseWebhookProviderStatuses(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"SERVER_ACK", "SENT"},
		{"DELIVERY_ACK", "DELIVERED"},
		{"READ", "READ"},
		{"PLAYED", "READ"},
		{"FAILED", "FAILED"},
		{"REVOKED", "DELETED"},
	}
	p := New("", "")
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			body := []byte(`{"event":"messages.update","instance":"main","data":{"key":{"id":"m1"},"messageTimestamp":1710000000,"status":"` + tc.raw + `","errorCode":"RATE_LIMIT"}}`)
			events, err := p.ParseWebhook(context.Background(), http.Header{}, body)
			if err != nil {
				t.Fatalf("ParseWebhook: %v", err)
			}
			if len(events) != 1 || events[0].Status == nil || events[0].Status.Status != tc.want {
				t.Fatalf("status de %s = %+v", tc.raw, events)
			}
			if events[0].Status.ErrorCode != "RATE_LIMIT" {
				t.Fatalf("errorCode = %q", events[0].Status.ErrorCode)
			}
		})
	}
}

func TestSafeProviderErrorCodeRejectsPayload(t *testing.T) {
	if got := safeProviderErrorCode([]byte(`{"message":"telefone 5511999999999"}`)); got != "" {
		t.Fatalf("objeto nao pode virar codigo seguro: %q", got)
	}
	if got := safeProviderErrorCode([]byte(`"invalid token leaked"`)); got != "" {
		t.Fatalf("texto livre nao pode virar codigo seguro: %q", got)
	}
}
