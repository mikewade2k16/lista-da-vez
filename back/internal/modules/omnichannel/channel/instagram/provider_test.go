package instagram

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/omnichannel/channel"
)

func TestParseWebhookDMAndComment(t *testing.T) {
	p := New("")
	body := []byte(`{"object":"instagram","entry":[{"id":"ig-1","time":1710000000000,"messaging":[{"sender":{"id":"user-1","username":"ana"},"recipient":{"id":"ig-1"},"timestamp":1710000000123,"message":{"mid":"m-1","text":"oi"}}],"changes":[{"field":"comments","value":{"id":"c-1","text":"preco?","from":{"id":"user-2","username":"bia"},"media":{"id":"media-1"},"timestamp":1710000000456,"is_live":false}}]}]}`)
	events, err := p.ParseWebhook(context.Background(), nil, body)
	if err != nil || len(events) != 2 {
		t.Fatalf("parse: events=%d err=%v", len(events), err)
	}
	if events[0].Message == nil || events[0].Message.Channel != "INSTAGRAM" || events[0].Message.SocialEventKind != "dm" {
		t.Fatalf("unexpected dm: %#v", events[0].Message)
	}
	if events[1].Message == nil || events[1].Message.SocialEventKind != "comment" || events[1].Message.SocialMediaID != "media-1" {
		t.Fatalf("unexpected comment: %#v", events[1].Message)
	}
}

func TestVerifyWebhookAndChallenge(t *testing.T) {
	p := New("")
	cred := channel.Credentials{Token: `{"accessToken":"token","appSecret":"secret","verifyToken":"verify"}`}
	body := []byte(`{"object":"instagram"}`)
	h := hmac.New(sha256.New, []byte("secret"))
	_, _ = h.Write(body)
	hdr := make(http.Header)
	hdr.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(h.Sum(nil)))
	if err := p.VerifyWebhook(hdr, body, cred); err != nil {
		t.Fatalf("verify: %v", err)
	}
	challenge, err := p.VerifyWebhookChallenge(map[string]string{"hub.mode": "subscribe", "hub.verify_token": "verify", "hub.challenge": "123"}, cred)
	if err != nil || challenge != "123" {
		t.Fatalf("challenge=%q err=%v", challenge, err)
	}
}

func TestSendMessage(t *testing.T) {
	var gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message_id":"sent-1"}`))
	}))
	defer server.Close()
	p := New(server.URL)
	result, err := p.SendMessage(context.Background(), channel.Credentials{
		Token:  `{"accessToken":"token","appSecret":"secret","verifyToken":"verify"}`,
		Config: map[string]string{"graphVersion": "v19.0", "igUserId": "ig-1"},
	}, channel.OutboundMessage{ToExternalID: "user-1", Content: "oi"})
	if err != nil || result.ExternalMessageID != "sent-1" {
		t.Fatalf("send=%#v err=%v", result, err)
	}
	if gotPath != "/v19.0/ig-1/messages" || gotAuth != "Bearer token" {
		t.Fatalf("request path=%s auth=%s", gotPath, gotAuth)
	}
}

func TestValidateGraphVersion(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"v19.0", true}, {"v123.0", true}, {"19.0", false},
		{"v1.0", false}, {"v19.1", false}, {" v19.0 ", true},
	}
	for _, test := range tests {
		if ValidateGraphVersion(test.value) != test.expected {
			t.Fatalf("version %q expected %v", test.value, test.expected)
		}
	}
}
