package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestN8NCompleteValidatesAndDoesNotSendAccountID(t *testing.T) {
	var body string
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(
			`{"ok":true,"output":{"intent":"vendas"},"model":"gpt-4o-mini","usage":{"promptTokens":4,"completionTokens":2,"totalTokens":6}}`,
		))}, nil
	})}
	client := NewN8NWithHTTPClient("http://n8n:5678/webhook/omnichannel-brain", hc, nil)
	req := baseReq()
	req.AccountID = "account-nao-deve-sair"
	req.Schema = &Schema{Name: "triage", Version: 1, Definition: json.RawMessage(`{
		"type":"object","properties":{"intent":{"type":"string"}},"required":["intent"],"additionalProperties":false
	}`)}

	got, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if string(got.JSON) != `{"intent":"vendas"}` || got.Usage.TotalTokens != 6 {
		t.Fatalf("resposta inesperada: %+v", got)
	}
	if strings.Contains(body, req.AccountID) {
		t.Fatal("account_id vazou para o executor n8n")
	}
	if !strings.Contains(body, `"apiKey":"sk-super-secreta-123"`) {
		t.Fatal("chave runtime nao foi enviada ao executor")
	}
}

func TestN8NCompleteRejectsSchemaInvalid(t *testing.T) {
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(
			`{"ok":true,"output":{"campo_extra":true}}`,
		))}, nil
	})}
	client := NewN8NWithHTTPClient("http://n8n:5678/webhook/omnichannel-brain", hc, nil)
	req := baseReq()
	req.Schema = &Schema{Name: "triage", Version: 1, Definition: json.RawMessage(`{
		"type":"object","properties":{"intent":{"type":"string"}},"required":["intent"],"additionalProperties":false
	}`)}

	_, err := client.Complete(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), ErrSchemaViolation.Error()) {
		t.Fatalf("esperava schema violation, veio %v", err)
	}
}
