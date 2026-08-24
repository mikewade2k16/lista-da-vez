package metaads

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrRunnerNotConfigured sinaliza que o agent-runner nao foi configurado
// (META_ADS_ASSISTANT_RUNNER_URL / META_ADS_ASSISTANT_TOKEN vazios). Mapeado
// para 503 assistant_not_configured nos handlers.
var ErrRunnerNotConfigured = errors.New("meta_ads: assistant runner nao configurado")

// errRunnerFailed marca falhas de transporte/protocolo do runner (rede, HTTP
// nao-2xx, JSON fora do contrato). Mapeado para 502 assistant_error.
var errRunnerFailed = errors.New("meta_ads: assistant runner falhou")

// errAuthSessionGone: o runner respondeu 409 no /auth/complete (a sessao de
// login persistente expirou/sumiu). Mapeado para 409 com pedido de gerar o link
// de novo, em vez do 502 generico.
var errAuthSessionGone = errors.New("meta_ads: sessao de login expirou")

// errOAuthCallbackConflict indica que a porta local do callback OAuth ja esta
// ocupada por outro processo/conta. E um conflito recuperavel, nao falha generica
// do runner.
var errOAuthCallbackConflict = errors.New("meta_ads: callback oauth ocupado")

// runnerTimeout e o timeout do HTTP client. Runs sao lentos (Claude headless +
// tools do MCP da Meta), entao a janela e bem maior que a dos demais clientes.
const runnerTimeout = 150 * time.Second

// runnerMaxResponseBytes limita o corpo lido das respostas do runner.
const runnerMaxResponseBytes = 4 << 20

// RunnerClient fala com o agent-runner (sidecar Node interno na rede do
// compose; contrato CONGELADO com a fase MA1: POST /run e GET /healthz, auth
// Bearer). Nunca exposto ao cliente: o painel passa pela API Go, que persiste o
// historico e faz o proxy escopado por account.
type RunnerClient struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewRunnerClient cria o cliente. baseURL ex.: "http://meta-ads-assistant:8787".
// baseURL/token vazios = runner nao configurado (toda chamada retorna
// ErrRunnerNotConfigured; o connect do painel mostra o estado de setup).
func NewRunnerClient(baseURL, token string) *RunnerClient {
	return &RunnerClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		http:    &http.Client{Timeout: runnerTimeout},
	}
}

// configured informa se ha baseURL E token para falar com o runner.
func (c *RunnerClient) configured() bool { return c.baseURL != "" && c.token != "" }

// RunnerOpts sao as configuracoes da account (modelo + system prompt) que o
// runner aplica na sessao. Vazias = defaults do runner.
type RunnerOpts struct {
	Model        string
	SystemPrompt string
}

// RunnerTurn e um turno do historico enviado ao runner ({role, content}).
type RunnerTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RunnerResult e a resposta de POST /run do runner.
type RunnerResult struct {
	Reply   string            `json:"reply"`
	Actions []AssistantAction `json:"actions"`
}

// Run envia prompt + historico para o runner executar (Claude headless + MCP
// meta-ads) e retorna a resposta com as acoes executadas. adAccountID da o
// contexto da conta de anuncio ativa no painel (pode ser vazio). accountID e o
// nosso ID interno de account — o runner o devolve ao bridge interno do Go
// (/internal/meta-ads/*) para buscar postagens do Instagram da conta certa.
func (c *RunnerClient) Run(ctx context.Context, prompt string, history []RunnerTurn, adAccountID, accountID string, opts RunnerOpts) (RunnerResult, error) {
	if !c.configured() {
		return RunnerResult{}, ErrRunnerNotConfigured
	}
	if history == nil {
		history = []RunnerTurn{}
	}
	payload, err := json.Marshal(struct {
		Prompt       string       `json:"prompt"`
		History      []RunnerTurn `json:"history"`
		AdAccountID  string       `json:"adAccountId"`
		AccountID    string       `json:"accountId"`
		Model        string       `json:"model"`
		SystemPrompt string       `json:"systemPrompt"`
	}{Prompt: prompt, History: history, AdAccountID: adAccountID, AccountID: accountID, Model: opts.Model, SystemPrompt: opts.SystemPrompt})
	if err != nil {
		return RunnerResult{}, fmt.Errorf("%w: %v", errRunnerFailed, err)
	}

	raw, status, err := c.do(ctx, http.MethodPost, "/run", bytes.NewReader(payload))
	if err != nil {
		return RunnerResult{}, fmt.Errorf("%w: %v", errRunnerFailed, err)
	}
	if status < 200 || status >= 300 {
		return RunnerResult{}, fmt.Errorf("%w: http %d", errRunnerFailed, status)
	}
	var out RunnerResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return RunnerResult{}, fmt.Errorf("%w: resposta invalida: %v", errRunnerFailed, err)
	}
	return out, nil
}

// Health consulta GET /healthz do runner ({ok, claudeAuth, detail?}). Runner de
// pe mas com resposta fora do contrato vira OK=false (sem erro).
func (c *RunnerClient) Health(ctx context.Context, accountID string) (AssistantHealthView, error) {
	if !c.configured() {
		return AssistantHealthView{}, ErrRunnerNotConfigured
	}
	path := "/healthz?accountId=" + url.QueryEscape(strings.TrimSpace(accountID))
	raw, status, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return AssistantHealthView{}, fmt.Errorf("%w: %v", errRunnerFailed, err)
	}
	var view AssistantHealthView
	if err := json.Unmarshal(raw, &view); err != nil {
		return AssistantHealthView{OK: false, Detail: fmt.Sprintf("http %d", status)}, nil
	}
	return view, nil
}

// RunnerAuthStart e a resposta de POST /auth/start (URL de login do Facebook).
type RunnerAuthStart struct {
	URL string `json:"url"`
}

// RunnerAuthComplete e a resposta de POST /auth/complete.
type RunnerAuthComplete struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// AuthStart inicia o OAuth do MCP da Meta no runner (chama a tool authenticate)
// e devolve a URL de autorizacao para o painel exibir.
func (c *RunnerClient) AuthStart(ctx context.Context, accountID string, opts RunnerOpts) (RunnerAuthStart, error) {
	if !c.configured() {
		return RunnerAuthStart{}, ErrRunnerNotConfigured
	}
	payload, err := json.Marshal(struct {
		AccountID    string `json:"accountId"`
		Model        string `json:"model"`
		SystemPrompt string `json:"systemPrompt"`
	}{AccountID: strings.TrimSpace(accountID), Model: opts.Model, SystemPrompt: opts.SystemPrompt})
	if err != nil {
		return RunnerAuthStart{}, fmt.Errorf("%w: %v", errRunnerFailed, err)
	}
	raw, status, err := c.do(ctx, http.MethodPost, "/auth/start", bytes.NewReader(payload))
	if err != nil {
		return RunnerAuthStart{}, fmt.Errorf("%w: %v", errRunnerFailed, err)
	}
	if status == http.StatusConflict && runnerErrorCode(raw) == "oauth_callback_conflict" {
		return RunnerAuthStart{}, errOAuthCallbackConflict
	}
	if status < 200 || status >= 300 {
		return RunnerAuthStart{}, fmt.Errorf("%w: http %d", errRunnerFailed, status)
	}
	var out RunnerAuthStart
	if err := json.Unmarshal(raw, &out); err != nil {
		return RunnerAuthStart{}, fmt.Errorf("%w: resposta invalida: %v", errRunnerFailed, err)
	}
	return out, nil
}

// AuthComplete conclui o OAuth do MCP da Meta com a URL de callback colada no
// painel (chama a tool complete_authentication).
func (c *RunnerClient) AuthComplete(ctx context.Context, accountID, callbackURL string, opts RunnerOpts) (RunnerAuthComplete, error) {
	if !c.configured() {
		return RunnerAuthComplete{}, ErrRunnerNotConfigured
	}
	payload, err := json.Marshal(struct {
		AccountID    string `json:"accountId"`
		CallbackURL  string `json:"callbackUrl"`
		Model        string `json:"model"`
		SystemPrompt string `json:"systemPrompt"`
	}{AccountID: strings.TrimSpace(accountID), CallbackURL: callbackURL, Model: opts.Model, SystemPrompt: opts.SystemPrompt})
	if err != nil {
		return RunnerAuthComplete{}, fmt.Errorf("%w: %v", errRunnerFailed, err)
	}
	raw, status, err := c.do(ctx, http.MethodPost, "/auth/complete", bytes.NewReader(payload))
	if err != nil {
		return RunnerAuthComplete{}, fmt.Errorf("%w: %v", errRunnerFailed, err)
	}
	if status == http.StatusConflict {
		if runnerErrorCode(raw) == "oauth_callback_conflict" {
			return RunnerAuthComplete{}, errOAuthCallbackConflict
		}
		return RunnerAuthComplete{}, errAuthSessionGone
	}
	if status < 200 || status >= 300 {
		return RunnerAuthComplete{}, fmt.Errorf("%w: http %d", errRunnerFailed, status)
	}
	var out RunnerAuthComplete
	if err := json.Unmarshal(raw, &out); err != nil {
		return RunnerAuthComplete{}, fmt.Errorf("%w: resposta invalida: %v", errRunnerFailed, err)
	}
	return out, nil
}

func runnerErrorCode(raw []byte) string {
	var envelope struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return ""
	}
	return strings.TrimSpace(envelope.Error)
}

// do executa a chamada com Bearer token e devolve corpo (limitado) + status.
func (c *RunnerClient) do(ctx context.Context, method, path string, body io.Reader) ([]byte, int, error) {
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
	raw, err := io.ReadAll(io.LimitReader(resp.Body, runnerMaxResponseBytes))
	if err != nil {
		return nil, 0, err
	}
	return raw, resp.StatusCode, nil
}
