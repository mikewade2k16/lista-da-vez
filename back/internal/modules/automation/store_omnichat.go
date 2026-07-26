package automation

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// OmniChatConfig e a fonte autoritativa por account. Nao carrega segredo: a
// credencial e resolvida server-side no cofre global somente durante a execucao.
type OmniChatConfig struct {
	AccountID     string
	Enabled       bool
	SystemPrompt  string
	CredentialID  string
	Provider      string
	Model         string
	Temperature   float64
	HistoryWindow int
	UpdatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func defaultOmniChatConfig(accountID string) OmniChatConfig {
	return OmniChatConfig{
		AccountID: accountID, Enabled: true, Provider: "openai",
		Model: "gpt-4.1-mini", Temperature: 0.2, HistoryWindow: defaultHistoryWindow,
	}
}

func (s *Store) GetOmniChatConfig(ctx context.Context, accountID string) (OmniChatConfig, error) {
	const q = `select account_id::text, enabled, system_prompt,
			coalesce(credential_id::text, ''), provider, model,
			temperature::float8, history_window, coalesce(updated_by::text, ''),
			created_at, updated_at
		from automation.omni_chat_configs
		where account_id = $1::uuid`
	var config OmniChatConfig
	err := s.pool.QueryRow(ctx, q, accountID).Scan(
		&config.AccountID, &config.Enabled, &config.SystemPrompt,
		&config.CredentialID, &config.Provider, &config.Model,
		&config.Temperature, &config.HistoryWindow, &config.UpdatedBy,
		&config.CreatedAt, &config.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultOmniChatConfig(accountID), nil
	}
	return config, err
}

func (s *Store) SaveOmniChatConfig(ctx context.Context, config OmniChatConfig) (OmniChatConfig, error) {
	const q = `insert into automation.omni_chat_configs (
			account_id, enabled, system_prompt, credential_id, provider, model,
			temperature, history_window, updated_by
		) values (
			$1::uuid, $2, $3, nullif($4, '')::uuid, $5, $6, $7, $8,
			nullif($9, '')::uuid
		)
		on conflict (account_id) do update set
			enabled = excluded.enabled,
			system_prompt = excluded.system_prompt,
			credential_id = excluded.credential_id,
			provider = excluded.provider,
			model = excluded.model,
			temperature = excluded.temperature,
			history_window = excluded.history_window,
			updated_by = excluded.updated_by,
			updated_at = now()
		returning account_id::text, enabled, system_prompt,
			coalesce(credential_id::text, ''), provider, model,
			temperature::float8, history_window, coalesce(updated_by::text, ''),
			created_at, updated_at`
	var saved OmniChatConfig
	err := s.pool.QueryRow(ctx, q,
		config.AccountID, config.Enabled, config.SystemPrompt, config.CredentialID,
		config.Provider, config.Model, config.Temperature, config.HistoryWindow,
		config.UpdatedBy,
	).Scan(
		&saved.AccountID, &saved.Enabled, &saved.SystemPrompt,
		&saved.CredentialID, &saved.Provider, &saved.Model,
		&saved.Temperature, &saved.HistoryWindow, &saved.UpdatedBy,
		&saved.CreatedAt, &saved.UpdatedAt,
	)
	return saved, err
}
