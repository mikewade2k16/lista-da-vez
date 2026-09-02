package omnichannel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type brainFakeLLM struct {
	response llm.Response
	err      error
	request  llm.Request
}

type countingBrainExecutor struct {
	calls int
}

func (f *countingBrainExecutor) CompleteBrain(context.Context, BrainRequestV2, BrainExecutionV2) (BrainResultV2, llm.Usage, int, error) {
	f.calls++
	return BrainResultV2{}, llm.Usage{}, 1, nil
}

func (f *brainFakeLLM) Complete(_ context.Context, req llm.Request) (llm.Response, error) {
	f.request = req
	return f.response, f.err
}

func testBrainBox(t *testing.T) *secretbox.Box {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	box, err := secretbox.New(key)
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func testBrainRequest() BrainRequestV2 {
	return BrainRequestV2{
		SchemaVersion: "brain.request.v2", DispatchID: "dispatch-1", Generation: 2,
		Tenant:          BrainTenantV2{AccountID: "account-1", Timezone: "America/Sao_Paulo"},
		Conversation:    BrainConversationV2{ID: "conversation-1", State: "ai_active", Channel: "WHATSAPP"},
		Contact:         BrainContactV2{ID: "contact-1", RelationshipStatus: "unknown", Tags: []string{}, Origin: BrainOriginV2{}},
		Messages:        []BrainMessageV2{{ID: "message-1", Role: "contact", Type: "TEXT", Text: stringPtr("oi")}},
		CollectedFields: map[string]any{}, RequiredFields: []string{}, PendingFields: []string{},
		LocalTime:    BrainLocalTimeV2{Now: time.Now().Format(time.RFC3339), InsideBusinessHours: true},
		Agent:        BrainAgentV2{ID: "agent-1", VersionID: "version-1", Model: "model", Layers: map[string]any{}},
		Capabilities: BrainCapabilitiesV2{Tools: []string{}, Multimodal: false},
	}
}

func stringPtr(value string) *string { return &value }

func TestCompleteBrainWithLeaseResetFirstNeverPostsN8N(t *testing.T) {
	executor := &countingBrainExecutor{}
	service := &AIService{
		brain: executor,
		externalEffectLease: func(context.Context, string, string, int64, func() error) (bool, error) {
			return false, nil
		},
	}
	_, _, _, err := service.completeBrainWithLease(context.Background(), triageParams{
		AccountID: "account", DispatchID: "dispatch", AIGeneration: 7,
	}, testBrainRequest(), BrainExecutionV2{})
	if !errors.Is(err, ErrHistoryResetInvalidated) || executor.calls != 0 {
		t.Fatalf("CompleteBrain err=%v calls=%d, want reset invalidation before POST", err, executor.calls)
	}
}

func TestN8NBrainExecutorSealsProviderKeyOutsidePayload(t *testing.T) {
	box := testBrainBox(t)
	result := brainResultFixture(t, string(BrainContinueAI), stringPtr("resposta"))
	result.Generation = 2
	resultJSON, _ := json.Marshal(result)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "api-secret") {
			t.Error("api key leaked in n8n payload")
		}
		if r.Header.Get("X-Omni-Internal-Token") != "internal-token" {
			t.Errorf("internal token = %q", r.Header.Get("X-Omni-Internal-Token"))
		}
		var envelope n8nBrainEnvelope
		if err := json.Unmarshal(body, &envelope); err != nil {
			t.Fatal(err)
		}
		claimsJSON, err := box.Decrypt(envelope.Execution.GatewayToken)
		if err != nil {
			t.Fatal(err)
		}
		var claims brainGatewayClaims
		if err := json.Unmarshal([]byte(claimsJSON), &claims); err != nil {
			t.Fatal(err)
		}
		if claims.APIKey != "api-secret" || claims.DispatchID != "dispatch-1" {
			t.Fatalf("claims do not carry expected sealed config: %+v", claims)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":` + string(resultJSON) + `,"usage":{"promptTokens":2,"completionTokens":1,"totalTokens":3}}`))
	}))
	defer server.Close()

	executor := newN8NBrainExecutor(server.URL, "internal-token", box, nil)
	got, usage, _, err := executor.CompleteBrain(context.Background(), testBrainRequest(), BrainExecutionV2{
		Provider: "openai", Model: "model", UserPrompt: "oi", SystemPrompt: "s", APIKey: "api-secret",
	})
	if err != nil {
		t.Fatalf("CompleteBrain: %v", err)
	}
	if got.DispatchID != "dispatch-1" || got.Decision != BrainContinueAI || usage.TotalTokens != 3 {
		t.Fatalf("result=%+v usage=%+v", got, usage)
	}
}

func TestBrainGatewayRejectsExpiredToken(t *testing.T) {
	box := testBrainBox(t)
	claims, _ := json.Marshal(brainGatewayClaims{Version: "brain-gateway.v1", ExpiresAt: time.Now().Add(-time.Minute).Unix(), AccountID: "account-1",
		DispatchID: "dispatch-1", Generation: 2, Provider: "openai", Model: "model", APIKey: "secret"})
	token, err := box.Encrypt(string(claims))
	if err != nil {
		t.Fatal(err)
	}
	gateway := allowBrainGatewayForTest(newBrainGateway(box, &brainFakeLLM{}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/runtime/omnichannel/llm-gateway", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	gateway.handle(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBrainGatewayUsesClaimsForProviderRequest(t *testing.T) {
	box := testBrainBox(t)
	claims, _ := json.Marshal(brainGatewayClaims{Version: "brain-gateway.v1", ExpiresAt: time.Now().Add(time.Minute).Unix(), AccountID: "account-1",
		DispatchID: "dispatch-1", Generation: 2, Provider: "openai", Model: "model", APIKey: "secret"})
	token, _ := box.Encrypt(string(claims))
	fake := &brainFakeLLM{response: llm.Response{JSON: json.RawMessage(`{"intent":"sales"}`), Model: "model"}}
	gateway := allowBrainGatewayForTest(newBrainGateway(box, fake))
	payload := `{"request":{"schemaVersion":"brain.request.v2","dispatchId":"dispatch-1","generation":2,"tenant":{"accountId":"account-1","timezone":"America/Sao_Paulo"}},"execution":{"provider":"openai","model":"model","temperature":0.2,"systemPrompt":"s","userPrompt":"u"}}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/runtime/omnichannel/llm-gateway", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	gateway.handle(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.request.APIKey != "secret" || fake.request.Provider != "openai" || fake.request.Model != "model" {
		t.Fatalf("request=%+v", fake.request)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatal("gateway response leaked provider key")
	}
}

func TestBrainGatewayAcceptsSupportedRequestSchemas(t *testing.T) {
	for _, schema := range []string{"brain.request.v2", "brain.request.v3"} {
		t.Run(schema, func(t *testing.T) {
			box := testBrainBox(t)
			claims, _ := json.Marshal(brainGatewayClaims{Version: "brain-gateway.v1", ExpiresAt: time.Now().Add(time.Minute).Unix(), AccountID: "account-1",
				DispatchID: "dispatch-1", Generation: 2, Provider: "openai", Model: "model", APIKey: "secret"})
			token, _ := box.Encrypt(string(claims))
			fake := &brainFakeLLM{response: llm.Response{JSON: json.RawMessage(`{"intent":"sales"}`), Model: "model"}}
			gateway := allowBrainGatewayForTest(newBrainGateway(box, fake))
			payload := `{"request":{"schemaVersion":"` + schema + `","dispatchId":"dispatch-1","generation":2,"tenant":{"accountId":"account-1","timezone":"America/Sao_Paulo"}},"execution":{"provider":"openai","model":"model","temperature":0.2,"systemPrompt":"s","userPrompt":"u"}}`
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/runtime/omnichannel/llm-gateway", strings.NewReader(payload))
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			gateway.handle(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestBrainGatewayRejectsUnsupportedRequestSchema(t *testing.T) {
	if supportedBrainRequestSchema("brain.request.v4") {
		t.Fatal("unknown brain request schema must fail closed")
	}
}

func allowBrainGatewayForTest(gateway *BrainGateway) *BrainGateway {
	gateway.externalEffectLease = func(_ context.Context, _, _ string, _ int64, effect func() error) (bool, error) {
		if effect != nil {
			return true, effect()
		}
		return true, nil
	}
	return gateway
}

func TestBrainGatewayTokenDoesNotUsePlainBase64AsCiphertext(t *testing.T) {
	box := testBrainBox(t)
	claims, _ := json.Marshal(brainGatewayClaims{Version: "brain-gateway.v1", ExpiresAt: time.Now().Add(time.Minute).Unix(), APIKey: "secret"})
	token, _ := box.Encrypt(string(claims))
	if _, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(token, "v1:")); err != nil {
		t.Fatalf("token must remain an authenticated base64 ciphertext: %v", err)
	}
}
