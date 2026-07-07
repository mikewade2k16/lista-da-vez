package calendar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// dispatchTimeout limita o POST ao n8n (o n8n responde imediatamente e processa
// em background; nao esperamos a geracao aqui, so a aceitacao do webhook).
const dispatchTimeout = 15 * time.Second

// markErrorTimeout e a janela curta e DESACOPLADA usada so para marcar o plano
// como error quando o dispatch falha. Nao pode reaproveitar o ctx do POST: no
// caso mais provavel de falha (o webhook estourar dispatchTimeout) esse ctx ja
// esta com DeadlineExceeded, o UPDATE nunca roda e a linha fica presa em pending.
const markErrorTimeout = 5 * time.Second

// AIDispatchConfig sao os parametros do disparo ao n8n (contrato C5), lidos do
// ambiente no Build do modulo. WebhookURL vazio => a IA nao esta configurada (o
// service devolve ErrAINotConfigured antes de criar a linha). ServiceToken vai no
// header X-Service-Token do callback; CallbackBase e a base publica da api que o
// n8n usa para chamar de volta. As CHAVES de API dos provedores NAO ficam aqui —
// vivem nas credentials do n8n.
type AIDispatchConfig struct {
	WebhookURL   string
	ServiceToken string
	CallbackBase string
}

// planContext agrega os insumos do payload C5 montados server-side (nomes/perfis
// dos clientes, feriados do mes e a nota do mes). Montado pelo store para evitar
// N+1 (ANY($1) nos nomes/perfis) e manter a camada http/service fina.
type planContext struct {
	Clients   []planClient
	Holidays  []Holiday
	MonthNote string
}

// planClient e um cliente do payload com o perfil embutido (C3 sem clientId).
type planClient struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Profile planProfile `json:"profile"`
}

// planProfile e o perfil C3 sem o clientId (o id ja vem no planClient).
type planProfile struct {
	Segment     string       `json:"segment"`
	Positioning string       `json:"positioning"`
	Description string       `json:"description"`
	History     string       `json:"history"`
	SiteURL     string       `json:"siteUrl"`
	Instagram   string       `json:"instagram"`
	Address     string       `json:"address"`
	Objectives  string       `json:"objectives"`
	BrandVoice  string       `json:"brandVoice"`
	Extra       ProfileExtra `json:"extra"`
}

// aiPayload e o corpo enviado ao webhook "Calendar Omni" do n8n (contrato C5). As
// chaves batem 1:1 com o workflow. callbackUrl e serviceToken fecham o loop de
// volta (o n8n chama /v1/public/calendar-ai/plans/{id}/result com o token).
type aiPayload struct {
	PlanID       string         `json:"planId"`
	Month        string         `json:"month"`
	Language     string         `json:"language"`
	AI           aiPayloadAI    `json:"ai"`
	Clients      []planClient   `json:"clients"`
	Holidays     []aiPayloadHol `json:"holidays"`
	MonthNotes   string         `json:"monthNotes"`
	CallbackURL  string         `json:"callbackUrl"`
	ServiceToken string         `json:"serviceToken"`
}

type aiPayloadAI struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	BaseURL      string  `json:"baseUrl"`
	SystemPrompt string  `json:"systemPrompt"`
	Temperature  float64 `json:"temperature"`
	// APIKey e a KEY CRUA do provider (SPEC-B2), resolvida server-side por resolveAIKey.
	// O n8n usa ai.apiKey como Authorization Bearer (SPEC-W3) — sem credential/$env. A
	// key so existe server-to-server: NUNCA e logada nem devolvida ao front.
	APIKey string `json:"apiKey"`
}

type aiPayloadHol struct {
	Date string `json:"date"`
	Name string `json:"name"`
	Set  string `json:"set"`
}

// resolveDispatchKey e o gate comum do dispatch de IA (chat/plano/transcricao,
// SPEC-B2): aplica o kill switch e resolve a KEY CRUA do provider via resolveAIKey
// (B1, server-side). enabled=false => ErrAIDisabled; key vazia (provider sem slot de
// secret ou sem key gravada) => ErrAIKeyMissing. Retorno SINCRONO para o caller mapear
// o 409 antes de disparar/criar a linha. A key retornada NUNCA e logada. Wave 3.1: o
// enabled/provider vem da config EFETIVA do cliente (chat) ou da config GERAL (plano/
// transcricao) — o caller decide qual passar, e a key resolve pelo provider EFETIVO.
func (s *Service) resolveDispatchKey(ctx context.Context, accountID string, enabled bool, provider string) (string, error) {
	if !enabled {
		return "", ErrAIDisabled
	}
	apiKey, err := s.resolveAIKey(ctx, strings.TrimSpace(accountID), provider)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(apiKey) == "" {
		return "", ErrAIKeyMissing
	}
	return apiKey, nil
}

// dispatchPlan dispara o n8n em goroutine (a request original nao espera). apiKey e a
// KEY CRUA ja resolvida sincronamente pelo caller (CreateAIPlan), repassada ao payload
// C5 (ai.apiKey). Falha no dispatch marca o plano como error via SetAIPlanResult (mesma
// transicao do callback). Usa contexto proprio com timeout — o contexto da request pode
// ja ter sido cancelado. Log em WARN/ERROR (sem a key; nunca deixa o erro silencioso).
func (s *Service) dispatchPlan(plan AIPlan, cfg CalendarConfig, apiKey string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), dispatchTimeout)
		defer cancel()

		if err := s.sendToN8N(ctx, plan, cfg, apiKey); err != nil {
			s.logAI("error", "dispatch do plano de IA falhou", plan, err)
			// Marca error para o front sair do estado pending (best-effort). Usa um
			// ctx NOVO e curto (nao o ctx do POST): quando o webhook estoura o
			// timeout, o ctx do POST ja esta expirado (DeadlineExceeded) e o UPDATE
			// nao rodaria — a linha ficaria presa em pending para sempre.
			markCtx, markCancel := context.WithTimeout(context.Background(), markErrorTimeout)
			defer markCancel()
			_, markErr := s.store.SetAIPlanResult(markCtx, plan.AccountID, plan.ID,
				planStatusError, AIPlanContent{}, "falha ao acionar o gerador de plano")
			if markErr != nil {
				s.logAI("error", "falha ao marcar plano como error apos dispatch", plan, markErr)
			}
			return
		}
		s.logAI("info", "plano de IA disparado ao n8n", plan, nil)
	}()
}

// sendToN8N monta o payload C5 (com a key crua em ai.apiKey) e faz o POST no webhook.
// Erro de rede/HTTP >= 400 vira erro (o chamador marca o plano como error). Wave 3.1:
// pula os clientes desativados (disabledClientIds) na montagem do payload — a IA nao
// gera plano para quem esta com a IA desligada. O plano usa o ai config GERAL da conta
// (override de comportamento por-cliente no plano fica p/ depois).
func (s *Service) sendToN8N(ctx context.Context, plan AIPlan, cfg CalendarConfig, apiKey string) error {
	clientIDs := filterDisabledClients(plan.ClientIDs, cfg.AI.DisabledClientIDs)
	pctx, err := s.store.planContext(ctx, plan.AccountID, plan.Month, clientIDs)
	if err != nil {
		return err
	}
	payload := s.buildPayload(plan, cfg, pctx, apiKey)
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSpace(s.ai.WebhookURL), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: dispatchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		return errBadDispatchStatus(resp.StatusCode)
	}
	return nil
}

// buildPayload monta o corpo C5 a partir do plano, da config, do contexto e da key
// crua (ai.apiKey). O callbackUrl junta a base publica ao caminho do callback deste plano.
func (s *Service) buildPayload(plan AIPlan, cfg CalendarConfig, pctx planContext, apiKey string) aiPayload {
	holidays := make([]aiPayloadHol, 0, len(pctx.Holidays))
	for _, h := range pctx.Holidays {
		// Holiday e aiPayloadHol tem os mesmos campos/tags: conversao direta.
		holidays = append(holidays, aiPayloadHol(h))
	}
	clients := pctx.Clients
	if clients == nil {
		clients = []planClient{}
	}
	return aiPayload{
		PlanID:   plan.ID,
		Month:    plan.Month,
		Language: "pt-BR",
		AI: aiPayloadAI{
			Provider:     cfg.AI.Provider,
			Model:        cfg.AI.Model,
			BaseURL:      cfg.AI.BaseURL,
			SystemPrompt: cfg.AI.SystemPrompt,
			Temperature:  cfg.AI.Temperature,
			APIKey:       apiKey,
		},
		Clients:      clients,
		Holidays:     holidays,
		MonthNotes:   pctx.MonthNote,
		CallbackURL:  callbackURL(s.ai.CallbackBase, plan.ID),
		ServiceToken: s.ai.ServiceToken,
	}
}

// filterDisabledClients remove os clientes desativados (WAVE 3.1 disabledClientIds) da
// lista de clientes do plano. Preserva a ordem; sem desativados devolve a lista original.
func filterDisabledClients(ids, disabled []string) []string {
	if len(disabled) == 0 {
		return ids
	}
	off := make(map[string]bool, len(disabled))
	for _, d := range disabled {
		off[d] = true
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !off[id] {
			out = append(out, id)
		}
	}
	return out
}

// errBadDispatchStatus rotula uma resposta HTTP >= 400 do webhook do n8n.
func errBadDispatchStatus(code int) error {
	return fmt.Errorf("calendar: webhook respondeu status %d", code)
}

// callbackURL junta a base publica ao caminho do callback do plano. Base vazia =>
// caminho relativo (o n8n pode ter a base configurada do lado dele).
func callbackURL(base, planID string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	return base + "/v1/public/calendar-ai/plans/" + planID + "/result"
}

// logAI emite um log estruturado do fluxo de IA (campos explicitos, sem PII). No-op
// se o logger nao foi injetado.
func (s *Service) logAI(level, msg string, plan AIPlan, err error) {
	if s.logger == nil {
		return
	}
	attrs := []any{"module", "calendar", "op", "ai_plan", "account_id", plan.AccountID, "plan_id", plan.ID}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
	}
	switch level {
	case "error":
		s.logger.Error(msg, attrs...)
	case "warn":
		s.logger.Warn(msg, attrs...)
	default:
		s.logger.Info(msg, attrs...)
	}
}
