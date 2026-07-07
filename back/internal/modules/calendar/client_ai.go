package calendar

import (
	"context"
	"strings"
)

// Escopo da IA por cliente (WAVE 3.1, CFG+/SEC+). A config `ai` da conta e o default
// GERAL; este arquivo resolve a config EFETIVA por cliente e o CRUD do override de
// comportamento (calendar.client_profiles.ai_config). A API key NUNCA entra aqui: o
// override so muda comportamento, e a key resolve pelo provider EFETIVO no dispatch.

const (
	// scopeModeGeneral: uma unica config de IA vale para todos os clientes (default).
	scopeModeGeneral = "general"
	// scopeModePerClient: config de IA individual por cliente (override em client_profiles).
	scopeModePerClient = "perClient"
)

// ClientAIOverrideView e a resposta dos endpoints de override por cliente. HasOverride
// diz se ha algum campo setado no ai_config do cliente (o front mostra o badge "usa
// config geral" quando false). Override carrega os campos crus (ponteiros nil = herda).
type ClientAIOverrideView struct {
	ClientID    string           `json:"clientId"`
	HasOverride bool             `json:"hasOverride"`
	Override    ClientAIOverride `json:"override"`
}

// clientAIStore e a fatia da persistencia do override de IA por cliente (WAVE 3.1).
// Mora em calendar.client_profiles.ai_config (PK account_id+client_id, migration
// 0185+0190). WHERE por account_id no store: conta A nunca le/escreve override de B.
type clientAIStore interface {
	GetClientAIOverride(ctx context.Context, accountID, clientID string) (ClientAIOverride, bool, error)
	PutClientAIOverride(ctx context.Context, accountID, clientID string, ov ClientAIOverride, updatedBy string) error
}

// EffectiveAIConfig resolve a config de IA EFETIVA de um cliente (WAVE 3.1). Base = a
// config `ai` da conta (geral). enabled efetivo = ai.Enabled E cliente fora de
// disabledClientIds E (se perClient e ha override com Enabled!=nil, o Enabled do
// override). Em perClient com override, faz merge por campo (override nao-vazio vence
// provider/model/baseUrl/systemPrompt/temperature). A KEY resolve DEPOIS pelo provider
// EFETIVO (resolveAIKey/resolveDispatchKey) — este merge nunca toca a credencial.
func (s *Service) EffectiveAIConfig(ctx context.Context, accountID, clientID string) (AIConfig, error) {
	account := strings.TrimSpace(accountID)
	cfg, err := s.store.GetConfig(ctx, account)
	if err != nil {
		return AIConfig{}, err
	}
	eff := cfg.AI
	client := normalizeUUID(clientID)
	// disabledClientIds desliga a IA para o cliente nos DOIS modos (geral e perClient).
	if client != "" && containsID(eff.DisabledClientIDs, client) {
		eff.Enabled = false
	}
	// So o modo perClient (com cliente valido) consulta o override de comportamento.
	if eff.ScopeMode != scopeModePerClient || client == "" {
		return eff, nil
	}
	ov, found, err := s.store.GetClientAIOverride(ctx, account, client)
	if err != nil {
		return AIConfig{}, err
	}
	if !found {
		return eff, nil
	}
	return mergeOverride(eff, ov), nil
}

// mergeOverride aplica o override de comportamento por cima da config efetiva base:
// campos nao-vazios vencem; Enabled do override compoe por E com o enabled ja
// calculado (kill switch geral/disabledClientIds continuam dominando). A key nunca
// entra no merge — resolve pelo provider EFETIVO resultante.
func mergeOverride(base AIConfig, ov ClientAIOverride) AIConfig {
	out := base
	if ov.Enabled != nil {
		out.Enabled = out.Enabled && *ov.Enabled
	}
	if strings.TrimSpace(ov.Provider) != "" {
		out.Provider = ov.Provider
	}
	if strings.TrimSpace(ov.Model) != "" {
		out.Model = ov.Model
	}
	if strings.TrimSpace(ov.BaseURL) != "" {
		out.BaseURL = ov.BaseURL
	}
	if strings.TrimSpace(ov.SystemPrompt) != "" {
		out.SystemPrompt = ov.SystemPrompt
	}
	if ov.Temperature != nil {
		out.Temperature = *ov.Temperature
	}
	return out
}

// GetClientAIOverride devolve o override de IA do cliente no escopo da account.
// clientId invalido (nao-UUID) => ErrInvalidClient (400). Sem override => HasOverride
// false com Override vazio (o front usa a config geral).
func (s *Service) GetClientAIOverride(ctx context.Context, accountID, clientID string) (ClientAIOverrideView, error) {
	account := strings.TrimSpace(accountID)
	client := normalizeUUID(clientID)
	if client == "" {
		return ClientAIOverrideView{}, ErrInvalidClient
	}
	ov, found, err := s.store.GetClientAIOverride(ctx, account, client)
	if err != nil {
		return ClientAIOverrideView{}, err
	}
	return ClientAIOverrideView{ClientID: client, HasOverride: found, Override: ov}, nil
}

// PutClientAIOverride faz upsert do override (sanitizado) no ai_config do cliente.
// clientId invalido => ErrInvalidClient (400). A key NUNCA entra aqui.
func (s *Service) PutClientAIOverride(ctx context.Context, accountID, clientID string, in ClientAIOverride, updatedBy string) (ClientAIOverrideView, error) {
	account := strings.TrimSpace(accountID)
	client := normalizeUUID(clientID)
	if client == "" {
		return ClientAIOverrideView{}, ErrInvalidClient
	}
	ov := sanitizeOverride(in)
	if err := s.store.PutClientAIOverride(ctx, account, client, ov, strings.TrimSpace(updatedBy)); err != nil {
		return ClientAIOverrideView{}, err
	}
	return ClientAIOverrideView{ClientID: client, HasOverride: overrideHasValue(ov), Override: ov}, nil
}

// sanitizeOverride valida o override (WAVE 3.1): provider no enum de IA (senao "" =
// herda), strings trim, temperature (ponteiro) clamp 0..1. Enabled passa direto (nil =
// herda). NUNCA valida/aceita key — o override so muda comportamento.
func sanitizeOverride(ov ClientAIOverride) ClientAIOverride {
	ov.Provider = strings.ToLower(strings.TrimSpace(ov.Provider))
	if ov.Provider != "" && !aiProviders[ov.Provider] {
		ov.Provider = ""
	}
	ov.Model = strings.TrimSpace(ov.Model)
	ov.BaseURL = strings.TrimSpace(ov.BaseURL)
	ov.SystemPrompt = strings.TrimSpace(ov.SystemPrompt)
	if ov.Temperature != nil {
		t := *ov.Temperature
		switch {
		case t < 0:
			t = 0
		case t > 1:
			t = 1
		}
		ov.Temperature = &t
	}
	return ov
}

// overrideHasValue diz se o override tem algum campo setado (define HasOverride). Row
// so com ai_config '{}' (ou sem nenhum campo) => false (o cliente usa a config geral).
func overrideHasValue(ov ClientAIOverride) bool {
	return ov.Enabled != nil ||
		strings.TrimSpace(ov.Provider) != "" ||
		strings.TrimSpace(ov.Model) != "" ||
		strings.TrimSpace(ov.BaseURL) != "" ||
		strings.TrimSpace(ov.SystemPrompt) != "" ||
		ov.Temperature != nil
}

// containsID diz se id esta na lista (lookup linear; listas de disabledClientIds sao
// curtas, sem necessidade de mapa).
func containsID(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
