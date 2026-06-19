package automation

import (
	"context"
	"strconv"
)

// omniChatPersonaSettingsKey e a chave dentro de automation.automations.settings jsonb
// onde o systemPrompt customizado do Omni Chat vive (por account, na automacao default).
// Mesmo padrao das sources (M5): sem migration nova. String vazia/ausente => usa o
// embed omniChatPersona como default.
const omniChatPersonaSettingsKey = "omniChatPersona"

// GetOmniChatPersonaSetting le o systemPrompt customizado do Omni Chat do settings jsonb
// da automacao (chave omniChatPersona). Retorna string vazia quando a conta nunca salvou
// um custom — o service usa isso para cair no default embutido. Espelha GetSources.
func (s *Store) GetOmniChatPersonaSetting(ctx context.Context, automationID string) (string, error) {
	const q = `select coalesce(settings ->> $2, '')
		from automation.automations
		where id = $1`
	var prompt string
	if err := s.pool.QueryRow(ctx, q, automationID, omniChatPersonaSettingsKey).Scan(&prompt); err != nil {
		return "", err
	}
	return prompt, nil
}

// SetOmniChatPersonaSetting grava o systemPrompt customizado do Omni Chat no settings
// jsonb da automacao (merge na chave omniChatPersona, preservando as demais chaves).
// Espelha SetSources.
func (s *Store) SetOmniChatPersonaSetting(ctx context.Context, automationID, prompt string) error {
	const q = `update automation.automations
		set settings = jsonb_set(coalesce(settings, '{}'::jsonb), array[$2], to_jsonb($3::text), true),
		    updated_at = now()
		where id = $1`
	_, err := s.pool.Exec(ctx, q, automationID, omniChatPersonaSettingsKey, prompt)
	return err
}

// omniChatHistoryWindowKey e a chave do settings jsonb com a janela de memoria do
// Omni Chat (quantas interacoes o n8n mantem). 0/ausente => service usa o default.
const omniChatHistoryWindowKey = "omniChatHistoryWindow"

// GetOmniChatHistoryWindow le a janela de memoria do settings jsonb (numero salvo).
// Retorna 0 quando a conta nunca salvou — o service cai no default. Le como texto e
// converte em Go (evita ::int no SQL quebrar se o valor vier estranho).
func (s *Store) GetOmniChatHistoryWindow(ctx context.Context, automationID string) (int, error) {
	const q = `select coalesce(settings ->> $2, '')
		from automation.automations
		where id = $1`
	var raw string
	if err := s.pool.QueryRow(ctx, q, automationID, omniChatHistoryWindowKey).Scan(&raw); err != nil {
		return 0, err
	}
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, nil
	}
	return n, nil
}

// SetOmniChatHistoryWindow grava a janela de memoria no settings jsonb (numero).
// Espelha SetOmniChatPersonaSetting; preserva as demais chaves do jsonb.
func (s *Store) SetOmniChatHistoryWindow(ctx context.Context, automationID string, n int) error {
	const q = `update automation.automations
		set settings = jsonb_set(coalesce(settings, '{}'::jsonb), array[$2], to_jsonb($3::int), true),
		    updated_at = now()
		where id = $1`
	_, err := s.pool.Exec(ctx, q, automationID, omniChatHistoryWindowKey, n)
	return err
}
