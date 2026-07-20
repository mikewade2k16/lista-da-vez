// Package llm e o client LLM nativo da plataforma (OMNI-F3.3): adapters openai/gemini/
// glm, structured output VALIDADO contra schema versionado e Usage exportado.
//
// Por que nasce em platform/: o 2o consumidor ja e previsivel (o calendario, hoje
// despachando via n8n). Reaproveita o que calendar/ai_models.go:40-44 ja provou — os
// tres provedores falam a camada OpenAI-compatible, entao UM adapter cobre os tres,
// parametrizado por baseURL.
//
// Provider/modelo/chave vem do PAINEL/BANCO, nunca de env, nunca supostos: quem
// resolve e o caller (padrao resolveDispatchKey, calendar/ai_dispatch.go:105).
//
// SSRF (risco novo da D-C): no calendario quem chamava o provedor era o n8n; com o LLM
// no Go quem faz o request de saida e o CONTAINER DA API. Como a BaseURL vem do
// painel, uma base apontando para host interno viraria SSRF — dai a allowlist
// obrigatoria (allowlist.go).
//
// Log: provider, modelo, tokens, latencia, account_id. NUNCA prompt, resposta ou chave.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Schema e o contrato de saida estruturada, VERSIONADO: schema mudou => Version sobe.
// Sem versao, um schema alterado silenciosamente quebra o consumidor sem rastro.
type Schema struct {
	Name       string
	Version    int
	Definition json.RawMessage // JSON Schema
}

// Usage sao os tokens da chamada. Devolvido por Complete; PERSISTIR em ai_runs e da
// F9 — este pacote nao cria nem escreve nessa tabela.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Request e a chamada ao provedor. Provider/Model/BaseURL/APIKey vem do painel/banco,
// resolvidos pelo caller — nunca de env, nunca supostos.
type Request struct {
	Provider string // openai | gemini | glm
	Model    string
	BaseURL  string // validado contra a allowlist (SSRF)

	SystemPrompt string
	UserPrompt   string
	Temperature  float64

	// Schema nil = texto livre. Preenchido = JSON mode + validacao no Go.
	Schema *Schema

	// APIKey e a chave CRUA. NUNCA logada, nunca ecoada em erro.
	APIKey string

	// AccountID e so para log/observabilidade (campo explicito, sem PII).
	AccountID string
}

// Response e o resultado da chamada.
type Response struct {
	Text      string
	JSON      json.RawMessage // preenchido so com Schema != nil, e so apos validar
	Usage     Usage
	Model     string
	LatencyMs int
}

// Client e o contrato do pacote.
type Client interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

var (
	// ErrKeyMissing: chave nao configurada. O caller mapeia em 409 ACIONAVEL
	// ("configure a chave do provider"), nunca 500 — nao e falha do servidor.
	ErrKeyMissing = errors.New("llm: chave do provider nao configurada")

	// ErrSchemaViolation: o provedor respondeu fora do schema. O caller decide
	// (repetir, cair para regra default, registrar). NUNCA entregar JSON nao validado
	// ao dominio — strict mode do provedor nao e prova.
	ErrSchemaViolation = errors.New("llm: resposta viola o schema")

	// ErrProviderUnavailable: o provedor falhou (rede, 5xx, resposta ilegivel). O
	// caller mapeia em 502.
	ErrProviderUnavailable = errors.New("llm: provider indisponivel")

	// ErrInvalidProvider: provider fora do enum openai|gemini|glm.
	ErrInvalidProvider = errors.New("llm: provider invalido")

	// ErrInvalidModel: model vazio. O painel manda o modelo escolhido no SELECT.
	ErrInvalidModel = errors.New("llm: modelo nao informado")

	// ErrBaseURLNotAllowed: BaseURL fora da allowlist — bloqueio de SSRF.
	ErrBaseURLNotAllowed = errors.New("llm: base URL nao permitida")
)

// StatusError carrega o status HTTP do provedor, para o caller classificar retry
// (platform/jobs.Classify le o seu proprio StatusError; aqui o status fica acessivel
// para quem quiser decidir). Nunca carrega corpo cru de resposta.
type StatusError struct {
	StatusCode int
	Err        error
}

// Error implementa error sem interpolar struct nem corpo de resposta.
func (e *StatusError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("llm: provider respondeu status %d", e.StatusCode)
	}
	return fmt.Sprintf("llm: status %d: %s", e.StatusCode, e.Err.Error())
}

// Unwrap deixa errors.Is achar ErrProviderUnavailable/ErrKeyMissing embaixo.
func (e *StatusError) Unwrap() error { return e.Err }

// validate checa o que da para checar antes de gastar rede: provider no enum, modelo
// informado, chave presente e BaseURL na allowlist.
func (r Request) validate() (string, error) {
	provider := normalizeProvider(r.Provider)
	if provider == "" {
		return "", fmt.Errorf("%w: %q", ErrInvalidProvider, r.Provider)
	}
	if r.Model == "" {
		return "", ErrInvalidModel
	}
	if r.APIKey == "" {
		// 409 acionavel, nao 500: falta configuracao, nao e defeito do servidor.
		return "", ErrKeyMissing
	}
	baseURL, err := ResolveBaseURL(provider, r.BaseURL)
	if err != nil {
		return "", err
	}
	return baseURL, nil
}

// elapsedMs mede a latencia da chamada.
func elapsedMs(start time.Time) int {
	return int(time.Since(start).Milliseconds())
}
