package omnichannel

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestValidateAIToolArgumentsUsesAllowlistedSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"sku":{"type":"string","maxLength":8},"quantity":{"type":"integer"}},"required":["sku"],"additionalProperties":false}`)
	if err := validateAIToolArguments(schema, json.RawMessage(`{"sku":"ABC"}`)); err != nil {
		t.Fatalf("argumento válido rejeitado: %v", err)
	}
	for _, raw := range []string{`{}`, `{"sku":"ABCDEFGHI"}`, `{"sku":"ABC","extra":true}`, `{"sku":4}`} {
		if err := validateAIToolArguments(schema, json.RawMessage(raw)); err == nil {
			t.Fatalf("argumento inválido aceito: %s", raw)
		}
	}
}

func TestMaskAIToolJSONDoesNotPersistSecretMarkers(t *testing.T) {
	masked := maskAIToolJSON(json.RawMessage(`{"token":"secret","name":"ok","nested":{"apiKey":"x"}}`), 4096)
	if string(masked) == "" || string(masked) == `{"token":"secret"}` {
		t.Fatalf("máscara inesperada: %s", masked)
	}
	var decoded map[string]any
	if err := json.Unmarshal(masked, &decoded); err != nil {
		t.Fatalf("máscara não é JSON: %v", err)
	}
	if decoded["token"] != "[redacted]" {
		t.Fatalf("token deveria ser redigido: %#v", decoded)
	}
}

func TestValidAIToolTimestamp(t *testing.T) {
	now := time.Now().Unix()
	if !validAIToolTimestamp(now) || !validAIToolTimestamp(now-300) || !validAIToolTimestamp(now+300) {
		t.Fatal("janela de timestamp válida rejeitada")
	}
	if validAIToolTimestamp(now-301) || validAIToolTimestamp(now+301) {
		t.Fatal("timestamp fora da janela aceito")
	}
}

func TestValidAIToolSignatureBindsEveryCallField(t *testing.T) {
	token := "gateway-token"
	in := aiToolCallRequest{Timestamp: time.Now().Unix(), DispatchID: "11111111-1111-4111-8111-111111111111", Generation: 3,
		BindingID: "22222222-2222-4222-8222-222222222222", CallID: "call-1", Operation: "lookup"}
	canonical := strings.Join([]string{strconv.FormatInt(in.Timestamp, 10), in.DispatchID, strconv.FormatInt(in.Generation, 10), in.BindingID, in.CallID, in.Operation}, "\n")
	h := hmac.New(sha256.New, []byte(token))
	_, _ = h.Write([]byte(canonical))
	in.Signature = hex.EncodeToString(h.Sum(nil))
	if !validAIToolSignature(token, in) {
		t.Fatal("assinatura canônica válida foi rejeitada")
	}
	for name, mutate := range map[string]func(*aiToolCallRequest){
		"dispatch":   func(v *aiToolCallRequest) { v.DispatchID = "33333333-3333-4333-8333-333333333333" },
		"generation": func(v *aiToolCallRequest) { v.Generation++ },
		"binding":    func(v *aiToolCallRequest) { v.BindingID = "44444444-4444-4444-8444-444444444444" },
		"call":       func(v *aiToolCallRequest) { v.CallID = "call-2" },
		"operation":  func(v *aiToolCallRequest) { v.Operation = "write" },
		"timestamp":  func(v *aiToolCallRequest) { v.Timestamp++ },
	} {
		copy := in
		mutate(&copy)
		if validAIToolSignature(token, copy) {
			t.Fatalf("assinatura deveria falhar ao alterar %s", name)
		}
	}
}
