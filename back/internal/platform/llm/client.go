package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// httpTimeout limita a chamada ao provedor. Sem timeout, um provedor pendurado
// prenderia o worker (platform/jobs) por tempo indefinido. O ctx do caller ainda
// pode cortar antes.
const httpTimeout = 60 * time.Second

// client e a implementacao OpenAI-compatible do Client. UM adapter cobre os tres
// provedores (openai/gemini/glm) porque os tres falam /chat/completions com Bearer —
// o que muda e a BaseURL, resolvida (e barrada contra SSRF) em Request.validate.
type client struct {
	hc     *http.Client
	logger *slog.Logger // nil => sem log
}

// New devolve o client nativo. logger nil desliga o log (o pacote nunca loga prompt,
// resposta ou chave — so provider/model/tokens/latencia/account_id).
func New(logger *slog.Logger) Client {
	return &client{
		hc:     &http.Client{Timeout: httpTimeout},
		logger: logger,
	}
}

// NewWithHTTPClient injeta o http.Client (testes: transport de mentira sem rede).
func NewWithHTTPClient(hc *http.Client, logger *slog.Logger) Client {
	if hc == nil {
		hc = &http.Client{Timeout: httpTimeout}
	}
	return &client{hc: hc, logger: logger}
}

// chatRequest e o corpo OpenAI-compatible enviado aos tres provedores.
type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	Temperature    float64       `json:"temperature"`
	ResponseFormat *responseFmt  `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFmt struct {
	Type string `json:"type"` // "json_object" quando ha Schema
}

// chatResponse e o subconjunto do retorno que consumimos. Campos extras sao ignorados.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Complete valida a request, chama o provedor e — quando ha Schema — valida a saida
// no Go antes de entregar. O strict mode do provedor NAO e prova (por isso Validate).
func (c *client) Complete(ctx context.Context, req Request) (Response, error) {
	baseURL, err := req.validate()
	if err != nil {
		return Response{}, err // ErrInvalidProvider/ErrInvalidModel/ErrKeyMissing/ErrBaseURLNotAllowed
	}
	provider := normalizeProvider(req.Provider)

	body := chatRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		Messages:    messagesFor(req),
	}
	if req.Schema != nil {
		// JSON mode + validacao no Go. Sem isto, o modelo pode devolver prosa em
		// volta do JSON e o Validate a jusante falha por ruido, nao por conteudo.
		body.ResponseFormat = &responseFmt{Type: "json_object"}
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/chat/completions"
	start := time.Now()
	raw, statusCode, err := c.doChatRequest(ctx, endpoint, req.APIKey, body)
	if err != nil {
		return Response{}, err
	}
	// Alguns modelos OpenAI-compatible legados aparecem em /models e aceitam chat,
	// mas rejeitam response_format=json_object com 400. A validacao autoritativa
	// continua no Go; por isso uma unica repeticao sem a dica do provider e segura.
	if statusCode == http.StatusBadRequest && body.ResponseFormat != nil {
		body.ResponseFormat = nil
		raw, statusCode, err = c.doChatRequest(ctx, endpoint, req.APIKey, body)
		if err != nil {
			return Response{}, err
		}
	}

	if statusCode < 200 || statusCode >= 300 {
		// StatusError expoe o codigo para o caller/jobs.Classify decidir retry.
		// NUNCA anexa o corpo cru (pode ecoar prompt/echo da chave em alguns provedores).
		return Response{}, &StatusError{StatusCode: statusCode, Err: ErrProviderUnavailable}
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Response{}, fmt.Errorf("%w: resposta ilegivel", ErrProviderUnavailable)
	}
	if len(parsed.Choices) == 0 {
		return Response{}, fmt.Errorf("%w: resposta sem choices", ErrProviderUnavailable)
	}

	content := parsed.Choices[0].Message.Content
	out := Response{
		Text:      content,
		Model:     req.Model,
		LatencyMs: elapsedMs(start),
		Usage: Usage{
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		},
	}

	if req.Schema != nil {
		trimmed := json.RawMessage(strings.TrimSpace(content))
		if err := Validate(req.Schema, trimmed); err != nil {
			// Entregar JSON nao validado ao dominio e o que a spec proibe.
			return Response{}, fmt.Errorf("%w: %v", ErrSchemaViolation, err)
		}
		out.JSON = trimmed
	}

	c.logCall(provider, out)
	return out, nil
}

func (c *client) doChatRequest(ctx context.Context, endpoint, apiKey string, body chatRequest) ([]byte, int, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: marshal request: %v", ErrProviderUnavailable, err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: build request: %v", ErrProviderUnavailable, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := c.hc.Do(httpReq)
	if err != nil {
		// Rede/timeout/ctx cancelado: o corpo do erro do net/http nao carrega a chave.
		return nil, 0, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 4 MiB
	if err != nil {
		return nil, 0, fmt.Errorf("%w: ler resposta: %v", ErrProviderUnavailable, err)
	}
	return raw, resp.StatusCode, nil
}

// messagesFor monta a lista de mensagens; system opcional, user sempre.
func messagesFor(req Request) []chatMessage {
	msgs := make([]chatMessage, 0, 2)
	if strings.TrimSpace(req.SystemPrompt) != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: req.SystemPrompt})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: req.UserPrompt})
	return msgs
}

// logCall registra so metadado — nunca prompt, resposta ou chave (comentario do package).
func (c *client) logCall(provider string, resp Response) {
	if c.logger == nil {
		return
	}
	c.logger.Info("llm.complete",
		slog.String("provider", provider),
		slog.String("model", resp.Model),
		slog.Int("prompt_tokens", resp.Usage.PromptTokens),
		slog.Int("completion_tokens", resp.Usage.CompletionTokens),
		slog.Int("total_tokens", resp.Usage.TotalTokens),
		slog.Int("latency_ms", resp.LatencyMs),
	)
}
