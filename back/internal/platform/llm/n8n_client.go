package llm

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
)

// n8nTimeout fica acima do timeout do provider: o workflow ainda precisa normalizar e
// responder. O contexto do caller pode encerrar antes.
const n8nTimeout = 75 * time.Second

// n8nClient delega a chamada ao modelo para um workflow stateless. O workflow nunca
// recebe account_id e nunca envia mensagem ao canal; ele apenas devolve a saida do modelo.
// A validacao do schema continua obrigatoriamente no Go antes do dominio consumir a saida.
type n8nClient struct {
	webhookURL string
	hc         *http.Client
	logger     *slog.Logger
}

// NewN8N cria o executor n8n. URL vazia e aceita no boot para permitir que o modulo suba
// sem o profile automation; a chamada falha de forma controlada como provider indisponivel.
func NewN8N(webhookURL string, logger *slog.Logger) Client {
	return NewN8NWithHTTPClient(webhookURL, &http.Client{Timeout: n8nTimeout}, logger)
}

// NewN8NWithHTTPClient permite testar o contrato sem rede real.
func NewN8NWithHTTPClient(webhookURL string, hc *http.Client, logger *slog.Logger) Client {
	if hc == nil {
		hc = &http.Client{Timeout: n8nTimeout}
	}
	return &n8nClient{webhookURL: strings.TrimSpace(webhookURL), hc: hc, logger: logger}
}

type n8nRequest struct {
	AI n8nAIRequest `json:"ai"`
}

type n8nAIRequest struct {
	Provider     string          `json:"provider"`
	Model        string          `json:"model"`
	BaseURL      string          `json:"baseUrl"`
	APIKey       string          `json:"apiKey"`
	Temperature  float64         `json:"temperature"`
	SystemPrompt string          `json:"systemPrompt,omitempty"`
	UserPrompt   string          `json:"userPrompt"`
	Schema       json.RawMessage `json:"schema,omitempty"`
}

type n8nResponse struct {
	OK        bool            `json:"ok"`
	Output    json.RawMessage `json:"output"`
	Model     string          `json:"model"`
	ErrorCode string          `json:"errorCode"`
	Usage     struct {
		PromptTokens     int `json:"promptTokens"`
		CompletionTokens int `json:"completionTokens"`
		TotalTokens      int `json:"totalTokens"`
	} `json:"usage"`
}

// Complete envia a configuracao resolvida do banco ao n8n pela rede server-to-server.
// A chave nao e logada, nao volta na resposta e nao e persistida pelo workflow canonico.
func (c *n8nClient) Complete(ctx context.Context, req Request) (Response, error) {
	baseURL, err := req.validate()
	if err != nil {
		return Response{}, err
	}
	if err := validateN8NWebhookURL(c.webhookURL); err != nil {
		return Response{}, err
	}

	var schema json.RawMessage
	if req.Schema != nil {
		schema = req.Schema.Definition
	}
	payload, err := json.Marshal(n8nRequest{AI: n8nAIRequest{
		Provider: req.Provider, Model: req.Model, BaseURL: baseURL, APIKey: req.APIKey,
		Temperature: req.Temperature, SystemPrompt: req.SystemPrompt,
		UserPrompt: req.UserPrompt, Schema: schema,
	}})
	if err != nil {
		return Response{}, fmt.Errorf("%w: montar request n8n", ErrProviderUnavailable)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return Response{}, fmt.Errorf("%w: montar request n8n", ErrProviderUnavailable)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("%w: executor n8n indisponivel", ErrProviderUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Response{}, fmt.Errorf("%w: resposta n8n ilegivel", ErrProviderUnavailable)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, &StatusError{StatusCode: resp.StatusCode, Err: ErrProviderUnavailable}
	}

	var parsed n8nResponse
	if err := json.Unmarshal(raw, &parsed); err != nil || !parsed.OK {
		return Response{}, fmt.Errorf("%w: executor n8n retornou falha", ErrProviderUnavailable)
	}
	output := bytes.TrimSpace(parsed.Output)
	if len(output) == 0 || bytes.Equal(output, []byte("null")) {
		return Response{}, fmt.Errorf("%w: executor n8n respondeu vazio", ErrProviderUnavailable)
	}
	if req.Schema != nil {
		if err := Validate(req.Schema, output); err != nil {
			return Response{}, err
		}
	}

	text := string(output)
	if len(output) > 0 && output[0] == '"' {
		_ = json.Unmarshal(output, &text)
	}
	out := Response{
		Text: text, JSON: output, Model: firstNonEmptyString(parsed.Model, req.Model),
		LatencyMs: elapsedMs(start),
		Usage: Usage{
			PromptTokens: parsed.Usage.PromptTokens, CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens: parsed.Usage.TotalTokens,
		},
	}
	if c.logger != nil {
		c.logger.Info("llm.complete.n8n", "provider", normalizeProvider(req.Provider),
			"model", out.Model, "total_tokens", out.Usage.TotalTokens, "latency_ms", out.LatencyMs)
	}
	return out, nil
}

func validateN8NWebhookURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("%w: webhook n8n nao configurado", ErrProviderUnavailable)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return fmt.Errorf("%w: webhook n8n invalido", ErrProviderUnavailable)
	}
	return nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
