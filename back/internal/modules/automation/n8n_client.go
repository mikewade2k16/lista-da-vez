package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrN8NNotConfigured sinaliza que o webhook interno do n8n nao foi configurado
// (AUTOMATION_N8N_INTERNAL_URL vazio ou AUTOMATION_RUNTIME_TOKEN vazio). Mapeado
// para 503 omnichat_not_configured nos handlers.
var ErrN8NNotConfigured = errors.New("automation: omni-chat n8n nao configurado")

// errN8NFailed marca falhas de transporte/protocolo do webhook (rede, HTTP
// nao-2xx, JSON fora do contrato). Mapeado para 502 omnichat_error. Timeout do
// deadline (context.DeadlineExceeded) NAO e' embrulhado aqui — sobe puro para o
// handler mapear em 504.
var errN8NFailed = errors.New("automation: omni-chat n8n falhou")

// omniChatTimeout e o timeout do HTTP client. O AI Agent do n8n e' sincrono
// (LLM headless), entao a janela e' larga. O front usa AbortController e o
// handler trata context.DeadlineExceeded como 504.
const omniChatTimeout = 60 * time.Second

// omniChatWebhookPath e o path do webhook interno (workflow-omni-chat.json,
// Active). Use /webhook/ (nao /webhook-test/) — o test-path so ouve com o editor
// aberto.
const omniChatWebhookPath = "/webhook/omni-chat"

// omniChatMaxResponseBytes limita o corpo lido da resposta do n8n.
const omniChatMaxResponseBytes = 1 << 20

// N8NClient fala com o webhook interno do n8n (rede do compose; nunca exposto ao
// browser). O navegador passa pela API Go, que monta o systemMessage escopado
// por account e faz o proxy. Auth por token de servico (AUTOMATION_RUNTIME_TOKEN,
// reusado do runtime-config) via Bearer.
type N8NClient struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewN8NClient cria o cliente. baseURL ex.: "http://n8n:5678". baseURL/token
// vazios = nao configurado (Ask retorna ErrN8NNotConfigured).
func NewN8NClient(baseURL, token string) *N8NClient {
	return &N8NClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		http:    &http.Client{Timeout: omniChatTimeout},
	}
}

// configured informa se ha baseURL E token para falar com o webhook.
func (c *N8NClient) configured() bool { return c.baseURL != "" && c.token != "" }

// OmniChatRunRequest e o body enviado ao webhook (contrato congelado do
// OMNI_CHAT_PLAN.md). systemMessage vem pronto do Go; contextToken (Fase 2) e' o
// context token HMAC opaco que escopa as tools de dados — o n8n o reenvia no
// header X-Omni-Context ao chamar as tools (ex.: catalogo). Vazio so quando o
// secret nao esta configurado.
type OmniChatRunRequest struct {
	Question      string `json:"question"`
	Topic         string `json:"topic"`
	SystemMessage string `json:"systemMessage"`
	SessionRef    string `json:"sessionRef"`
	ContextToken  string `json:"contextToken"`
}

// OmniChatRunResult e a resposta do webhook ({ answer }).
type OmniChatRunResult struct {
	Answer string `json:"answer"`
}

// Ask envia a pergunta + systemMessage para o webhook do n8n e retorna a
// resposta do AI Agent. Erros: ErrN8NNotConfigured (sem env), errN8NFailed
// (rede/HTTP nao-2xx/JSON invalido), context.DeadlineExceeded (timeout, puro).
func (c *N8NClient) Ask(ctx context.Context, req OmniChatRunRequest) (OmniChatRunResult, error) {
	if !c.configured() {
		return OmniChatRunResult{}, ErrN8NNotConfigured
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return OmniChatRunResult{}, fmt.Errorf("%w: %v", errN8NFailed, err)
	}

	raw, status, err := c.do(ctx, http.MethodPost, omniChatWebhookPath, bytes.NewReader(payload))
	if err != nil {
		// Timeout/cancelamento sobe puro para o handler mapear em 504.
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return OmniChatRunResult{}, err
		}
		return OmniChatRunResult{}, fmt.Errorf("%w: %v", errN8NFailed, err)
	}
	if status < 200 || status >= 300 {
		return OmniChatRunResult{}, fmt.Errorf("%w: http %d", errN8NFailed, status)
	}

	var out OmniChatRunResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return OmniChatRunResult{}, fmt.Errorf("%w: resposta invalida: %v", errN8NFailed, err)
	}
	return out, nil
}

// do executa a chamada com Bearer token e devolve corpo (limitado) + status.
func (c *N8NClient) do(ctx context.Context, method, path string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, omniChatMaxResponseBytes))
	if err != nil {
		return nil, 0, err
	}
	return raw, resp.StatusCode, nil
}
