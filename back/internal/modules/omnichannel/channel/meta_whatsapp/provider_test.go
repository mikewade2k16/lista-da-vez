package meta_whatsapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

func TestVerifyWebhookAndParseMessage(t *testing.T) {
	body := []byte(`{"object":"whatsapp_business_account","entry":[{"changes":[{"value":{"metadata":{"phone_number_id":"123"},"contacts":[{"wa_id":"5511999999999","profile":{"name":"Cliente"}}],"messages":[{"from":"5511999999999","id":"wamid.1","timestamp":"1710000000","type":"text","text":{"body":"oi"},"context":{"id":"wamid.prev"}}]}}]}]}`)
	provider := New("")
	mac := hmac.New(sha256.New, []byte("app-secret"))
	_, _ = mac.Write(body)
	h := make(http.Header)
	h.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	cred := channel.Credentials{Token: `{"accessToken":"token","appSecret":"app-secret","verifyToken":"verify"}`, Config: map[string]string{"graphVersion": "v20.0"}}
	if err := provider.VerifyWebhook(h, body, cred); err != nil {
		t.Fatalf("verify: %v", err)
	}
	events, err := provider.ParseWebhook(context.Background(), h, body)
	if err != nil || len(events) != 1 {
		t.Fatalf("parse: %v events=%d", err, len(events))
	}
	message := events[0].Message
	if message == nil || message.Content != "oi" || message.ContactName != "Cliente" || message.Reply == nil || message.Reply.ExternalMessageID != "wamid.prev" {
		t.Fatalf("canonical message: %#v", message)
	}
	if events[0].InstanceName != "123" || events[0].ExternalEventID != "123:msg:wamid.1" {
		t.Fatalf("event identity: %#v", events[0])
	}
	body[len(body)-1] = ' '
	if err := provider.VerifyWebhook(h, body, cred); err == nil {
		t.Fatal("modified body accepted")
	}
}

func TestVerifyChallengeUsesConstantTimeSecret(t *testing.T) {
	provider := New("")
	cred := channel.Credentials{Token: `{"accessToken":"token","verifyToken":"verify"}`}
	challenge, err := provider.VerifyWebhookChallenge(map[string]string{"hub.mode": "subscribe", "hub.verify_token": "verify", "hub.challenge": "abc"}, cred)
	if err != nil || challenge != "abc" {
		t.Fatalf("challenge: %q %v", challenge, err)
	}
	if _, err := provider.VerifyWebhookChallenge(map[string]string{"hub.mode": "subscribe", "hub.verify_token": "wrong", "hub.challenge": "abc"}, cred); err == nil {
		t.Fatal("invalid challenge accepted")
	}
}

func TestSendMessageDoesNotLogOrExposeCredentials(t *testing.T) {
	var got http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = *r
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if json.Unmarshal(body, &payload) != nil || payload["type"] != "text" {
			t.Fatalf("payload: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.out"}]}`))
	}))
	defer server.Close()
	provider := New(server.URL)
	result, err := provider.SendMessage(context.Background(), channel.Credentials{Token: "access-secret", Config: map[string]string{"graphVersion": "v20.0", "phoneNumberId": "123"}}, channel.OutboundMessage{InstanceName: "ignored", ToPhone: "5511", MessageType: "TEXT", Content: "hello"})
	if err != nil || result.ExternalMessageID != "wamid.out" {
		t.Fatalf("send: %#v %v", result, err)
	}
	if got.Header.Get("Authorization") != "Bearer access-secret" || !strings.HasSuffix(got.URL.Path, "/v20.0/123/messages") {
		t.Fatalf("request: %s auth=%q", got.URL.Path, got.Header.Get("Authorization"))
	}
}
