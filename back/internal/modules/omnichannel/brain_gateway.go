package omnichannel

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// BrainGateway is the only Go endpoint that can unwrap an n8n gateway token.
// It is intentionally outside JWT/module middleware: n8n calls it server-to-server,
// and the encrypted short-lived token is the authentication boundary.
type BrainGateway struct {
	box *secretbox.Box
	llm llm.Client
}

func newBrainGateway(box *secretbox.Box, client llm.Client) *BrainGateway {
	return &BrainGateway{box: box, llm: client}
}

type brainGatewayRequest struct {
	Request   BrainRequestV2     `json:"request"`
	Execution brainExecutionWire `json:"execution"`
}

type brainGatewayResponse struct {
	Content string         `json:"content"`
	Model   string         `json:"model"`
	Usage   brainUsageWire `json:"usage"`
}

func registerBrainGatewayRoutes(mux *http.ServeMux, gateway *BrainGateway) {
	if gateway == nil {
		return
	}
	mux.HandleFunc("POST /v1/runtime/omnichannel/llm-gateway", gateway.handle)
}

func (g *BrainGateway) handle(w http.ResponseWriter, r *http.Request) {
	if g == nil || g.box == nil || g.llm == nil {
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "gateway_unavailable", "Gateway indisponivel.")
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "gateway_unauthorized", "Token interno ausente.")
		return
	}
	claimsRaw, err := g.box.Decrypt(token)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "gateway_unauthorized", "Token interno invalido.")
		return
	}
	var claims brainGatewayClaims
	if json.Unmarshal([]byte(claimsRaw), &claims) != nil || claims.Version != "brain-gateway.v1" || claims.ExpiresAt < time.Now().Unix() ||
		strings.TrimSpace(claims.AccountID) == "" || strings.TrimSpace(claims.DispatchID) == "" || claims.Generation < 0 || strings.TrimSpace(claims.Provider) == "" ||
		strings.TrimSpace(claims.Model) == "" || strings.TrimSpace(claims.APIKey) == "" {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "gateway_unauthorized", "Token interno invalido.")
		return
	}
	var in brainGatewayRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
		return
	}
	if !supportedBrainRequestSchema(in.Request.SchemaVersion) || in.Request.Tenant.AccountID != claims.AccountID || in.Request.DispatchID != claims.DispatchID ||
		in.Request.Generation != claims.Generation || in.Execution.Provider != claims.Provider || in.Execution.Model != claims.Model {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "gateway_unauthorized", "Token nao corresponde ao request.")
		return
	}
	if strings.TrimSpace(in.Execution.SystemPrompt) == "" || strings.TrimSpace(in.Execution.UserPrompt) == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Prompt ausente.")
		return
	}
	request := llm.Request{
		Provider: claims.Provider, Model: claims.Model, BaseURL: claims.BaseURL, APIKey: claims.APIKey,
		Temperature: in.Execution.Temperature, SystemPrompt: in.Execution.SystemPrompt,
		UserPrompt: in.Execution.UserPrompt, AccountID: in.Request.Tenant.AccountID,
	}
	if len(in.Execution.OutputSchema) > 0 && string(in.Execution.OutputSchema) != "null" {
		request.Schema = &llm.Schema{Name: "omnichannel_triage", Version: 1, Definition: in.Execution.OutputSchema}
	}
	resp, err := g.llm.Complete(r.Context(), request)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, llm.ErrKeyMissing) || errors.Is(err, llm.ErrInvalidProvider) || errors.Is(err, llm.ErrInvalidModel) {
			status = http.StatusUnprocessableEntity
		}
		httpapi.WriteError(w, r, status, "gateway_provider_error", "Provider indisponivel.")
		return
	}
	content := strings.TrimSpace(string(resp.JSON))
	if content == "" {
		content = strings.TrimSpace(resp.Text)
	}
	httpapi.WriteJSON(w, http.StatusOK, brainGatewayResponse{Content: content, Model: resp.Model, Usage: brainUsageWire{
		PromptTokens: resp.Usage.PromptTokens, CompletionTokens: resp.Usage.CompletionTokens, TotalTokens: resp.Usage.TotalTokens,
	}})
}

func supportedBrainRequestSchema(schema string) bool {
	switch strings.TrimSpace(schema) {
	case "brain.request.v2", "brain.request.v3":
		return true
	default:
		return false
	}
}
