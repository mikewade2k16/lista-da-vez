package omnichannel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

const brainExecutorTimeout = 75 * time.Second

// BrainMessageV2 is the PII-bearing context sent only to the configured internal
// orchestrator. It never contains provider credentials.
type BrainMessageV2 struct {
	ID    string        `json:"id"`
	Role  string        `json:"role"`
	Type  string        `json:"type"`
	Text  *string       `json:"text"`
	Media *BrainMediaV2 `json:"media"`
}

type BrainMediaV2 struct {
	MimeType     *string `json:"mimeType"`
	FileName     *string `json:"fileName"`
	SizeBytes    *int64  `json:"sizeBytes"`
	AnalysisText *string `json:"analysisText"`
}

type BrainOriginV2 struct {
	Channel      *string `json:"channel"`
	Source       *string `json:"source"`
	LandingPage  *string `json:"landingPageSlug"`
	Campaign     *string `json:"campaign"`
	Referrer     *string `json:"referrer"`
	FirstTouchAt *string `json:"firstTouchAt"`
	LastTouchAt  *string `json:"lastTouchAt"`
}

type BrainTenantV2 struct {
	AccountID string `json:"accountId"`
	Timezone  string `json:"timezone"`
}

type BrainConversationV2 struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Channel string `json:"channel"`
}

type BrainContactV2 struct {
	ID                 string        `json:"id"`
	Name               *string       `json:"name"`
	RelationshipStatus string        `json:"relationshipStatus"`
	Tags               []string      `json:"tags"`
	Origin             BrainOriginV2 `json:"origin"`
	Summary            *string       `json:"summary"`
}

type BrainLocalTimeV2 struct {
	Now                 string `json:"now"`
	InsideBusinessHours bool   `json:"insideBusinessHours"`
}

type BrainAgentV2 struct {
	ID        string         `json:"id"`
	VersionID string         `json:"versionId"`
	Model     string         `json:"model"`
	Layers    map[string]any `json:"layers"`
}

type BrainCapabilitiesV2 struct {
	Tools      []string `json:"tools"`
	Multimodal bool     `json:"multimodal"`
}

type BrainClientV3 struct {
	ID string `json:"id"`
}

// BrainRequestV2 is the stable orchestration envelope. Account scope is explicit
// for auditing, while the gateway token remains the only carrier of credentials.
type BrainRequestV2 struct {
	SchemaVersion   string                     `json:"schemaVersion"`
	DispatchID      string                     `json:"dispatchId"`
	Generation      int64                      `json:"generation"`
	Tenant          BrainTenantV2              `json:"tenant"`
	Conversation    BrainConversationV2        `json:"conversation"`
	Contact         BrainContactV2             `json:"contact"`
	Messages        []BrainMessageV2           `json:"messages"`
	CollectedFields map[string]any             `json:"collectedFields"`
	RequiredFields  []string                   `json:"requiredFields"`
	PendingFields   []string                   `json:"pendingFields"`
	LocalTime       BrainLocalTimeV2           `json:"localTime"`
	Agent           BrainAgentV2               `json:"agent"`
	Capabilities    BrainCapabilitiesV2        `json:"capabilities"`
	Client          *BrainClientV3             `json:"client,omitempty"`
	BusinessContext *AutomationBusinessContext `json:"businessContext,omitempty"`
}

// BrainExecutionV2 is internal Go data. APIKey is deliberately json:"-" and is
// sealed into the short-lived gateway token before the request reaches n8n.
type BrainExecutionV2 struct {
	Provider     string
	Model        string
	BaseURL      string
	Temperature  float64
	SystemPrompt string
	UserPrompt   string
	OutputSchema json.RawMessage
	// ToolBindings is internal orchestration metadata. It maps a logical tool
	// id to the tenant-scoped binding UUID; it is not shown as a model capability
	// and never contains credentials.
	ToolBindings map[string]string
	APIKey       string
}

type brainExecutor interface {
	CompleteBrain(context.Context, BrainRequestV2, BrainExecutionV2) (BrainResultV2, llm.Usage, int, error)
}

type n8nBrainExecutor struct {
	webhookURL  string
	internalKey string
	box         *secretbox.Box
	hc          *http.Client
	logger      *slog.Logger
}

func newN8NBrainExecutor(webhookURL, internalKey string, box *secretbox.Box, logger *slog.Logger) *n8nBrainExecutor {
	return &n8nBrainExecutor{
		webhookURL: strings.TrimSpace(webhookURL), internalKey: strings.TrimSpace(internalKey),
		box: box, hc: &http.Client{Timeout: brainExecutorTimeout}, logger: logger,
	}
}

type brainGatewayClaims struct {
	Version    string `json:"v"`
	ExpiresAt  int64  `json:"exp"`
	AccountID  string `json:"accountId"`
	DispatchID string `json:"dispatchId"`
	Generation int64  `json:"generation"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	BaseURL    string `json:"baseUrl"`
	APIKey     string `json:"apiKey"`
}

type n8nBrainEnvelope struct {
	Request   BrainRequestV2     `json:"request"`
	Execution brainExecutionWire `json:"execution"`
}

type brainExecutionWire struct {
	Provider     string            `json:"provider"`
	Model        string            `json:"model"`
	Temperature  float64           `json:"temperature"`
	SystemPrompt string            `json:"systemPrompt"`
	UserPrompt   string            `json:"userPrompt"`
	OutputSchema json.RawMessage   `json:"outputSchema"`
	ToolBindings map[string]string `json:"toolBindings,omitempty"`
	GatewayToken string            `json:"gatewayToken"`
}

type n8nBrainResponse struct {
	OK        bool            `json:"ok"`
	Result    json.RawMessage `json:"result"`
	ErrorCode string          `json:"errorCode"`
	Usage     brainUsageWire  `json:"usage"`
}

type brainUsageWire struct {
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	TotalTokens      int    `json:"totalTokens"`
}

func (c *n8nBrainExecutor) CompleteBrain(ctx context.Context, request BrainRequestV2, execution BrainExecutionV2) (BrainResultV2, llm.Usage, int, error) {
	if c == nil || c.box == nil || strings.TrimSpace(c.internalKey) == "" {
		return BrainResultV2{}, llm.Usage{}, 0, fmt.Errorf("%w: gateway n8n nao configurado", llm.ErrProviderUnavailable)
	}
	if err := validateBrainHTTPURL(c.webhookURL); err != nil {
		return BrainResultV2{}, llm.Usage{}, 0, err
	}
	if strings.TrimSpace(request.DispatchID) == "" || request.Generation < 0 || strings.TrimSpace(execution.APIKey) == "" {
		return BrainResultV2{}, llm.Usage{}, 0, fmt.Errorf("%w: identidade brain incompleta", llm.ErrProviderUnavailable)
	}
	claimsJSON, err := json.Marshal(brainGatewayClaims{
		Version: "brain-gateway.v1", ExpiresAt: time.Now().Add(2 * time.Minute).Unix(),
		AccountID:  request.Tenant.AccountID,
		DispatchID: request.DispatchID, Generation: request.Generation,
		Provider: execution.Provider, Model: execution.Model, BaseURL: execution.BaseURL, APIKey: execution.APIKey,
	})
	if err != nil {
		return BrainResultV2{}, llm.Usage{}, 0, fmt.Errorf("%w: token gateway", llm.ErrProviderUnavailable)
	}
	token, err := c.box.Encrypt(string(claimsJSON))
	if err != nil {
		return BrainResultV2{}, llm.Usage{}, 0, fmt.Errorf("%w: cifrar token gateway", llm.ErrProviderUnavailable)
	}
	payload := n8nBrainEnvelope{Request: request, Execution: brainExecutionWire{
		Provider: execution.Provider, Model: execution.Model, Temperature: execution.Temperature,
		SystemPrompt: execution.SystemPrompt, UserPrompt: execution.UserPrompt,
		OutputSchema: execution.OutputSchema, ToolBindings: execution.ToolBindings, GatewayToken: token,
	}}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return BrainResultV2{}, llm.Usage{}, 0, fmt.Errorf("%w: serializar brain", llm.ErrProviderUnavailable)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(rawPayload)) //nolint:gosec // URL vem exclusivamente da configuracao interna do servidor.
	if err != nil {
		return BrainResultV2{}, llm.Usage{}, 0, fmt.Errorf("%w: montar webhook brain", llm.ErrProviderUnavailable)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Omni-Internal-Token", c.internalKey)
	started := time.Now()
	resp, err := c.hc.Do(req) //nolint:gosec // request aponta para o webhook interno configurado no boot.
	if err != nil {
		return BrainResultV2{}, llm.Usage{}, 0, fmt.Errorf("%w: webhook brain indisponivel", llm.ErrProviderUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return BrainResultV2{}, llm.Usage{}, elapsedBrainMs(started), fmt.Errorf("%w: resposta webhook brain invalida", llm.ErrProviderUnavailable)
	}
	var envelope n8nBrainResponse
	if err := json.Unmarshal(raw, &envelope); err != nil || !envelope.OK || len(envelope.Result) == 0 {
		code := strings.TrimSpace(envelope.ErrorCode)
		if code == "" {
			code = "brain_execution_failed"
		}
		return BrainResultV2{}, llm.Usage{PromptTokens: envelope.Usage.PromptTokens, CompletionTokens: envelope.Usage.CompletionTokens, TotalTokens: envelope.Usage.TotalTokens}, elapsedBrainMs(started), fmt.Errorf("%w: %s", llm.ErrProviderUnavailable, code)
	}
	result, err := DecodeBrainResultV2(envelope.Result)
	if err != nil {
		return BrainResultV2{}, llm.Usage{PromptTokens: envelope.Usage.PromptTokens, CompletionTokens: envelope.Usage.CompletionTokens, TotalTokens: envelope.Usage.TotalTokens}, elapsedBrainMs(started), err
	}
	if result.DispatchID != request.DispatchID || result.Generation != request.Generation {
		return BrainResultV2{}, llm.Usage{PromptTokens: envelope.Usage.PromptTokens, CompletionTokens: envelope.Usage.CompletionTokens, TotalTokens: envelope.Usage.TotalTokens}, elapsedBrainMs(started), fmt.Errorf("%w: identity brain divergente", ErrBrainSchemaInvalid)
	}
	usage := llm.Usage{PromptTokens: envelope.Usage.PromptTokens, CompletionTokens: envelope.Usage.CompletionTokens, TotalTokens: envelope.Usage.TotalTokens}
	provider := envelope.Usage.Provider
	if provider == "" {
		provider = result.Usage.Provider
	}
	model := envelope.Usage.Model
	if model == "" {
		model = result.Usage.Model
	}
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 && usage.TotalTokens == 0 {
		usage.PromptTokens = result.Usage.PromptTokens
		usage.CompletionTokens = result.Usage.CompletionTokens
		usage.TotalTokens = result.Usage.PromptTokens + result.Usage.CompletionTokens
	}
	if c.logger != nil {
		c.logger.Info("omnichannel_brain_n8n_complete", "provider", provider, "model", model,
			"total_tokens", usage.TotalTokens, "latency_ms", elapsedBrainMs(started))
	}
	return result, usage, elapsedBrainMs(started), nil
}

func validateBrainHTTPURL(raw string) error {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("%w: webhook brain invalido", llm.ErrProviderUnavailable)
	}
	return nil
}

func elapsedBrainMs(start time.Time) int { return int(time.Since(start).Milliseconds()) }

var _ brainExecutor = (*n8nBrainExecutor)(nil)
