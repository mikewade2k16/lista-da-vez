package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// secretStore e a fatia da persistencia dos secrets de IA (Wave 3, SEC) que o Service
// consome. Por conta em calendar.ai_secrets; global em core.platform_settings.
type secretStore interface {
	// GetAccountSecret le a key crua da conta para o provider ("" se nao existe).
	GetAccountSecret(ctx context.Context, accountID, provider string) (string, error)
	// PutAccountSecret grava a key da conta; apiKey vazio = remove a linha (limpar).
	PutAccountSecret(ctx context.Context, accountID, provider, apiKey, updatedBy string) error
	// GetGlobalSecrets le o conjunto de keys globais da plataforma.
	GetGlobalSecrets(ctx context.Context) (GlobalSecrets, error)
	// PutGlobalSecret grava/limpa a key global de um provider (preserva as demais).
	PutGlobalSecret(ctx context.Context, provider, apiKey, updatedBy string) error
}

// GetAccountSecret le a key crua da conta para o provider. Escopo por PK composta
// (account_id, provider): conta A nunca le a de B. Sem linha => "".
func (s *Store) GetAccountSecret(ctx context.Context, accountID, provider string) (string, error) {
	const q = `select api_key from calendar.ai_secrets where account_id = $1::uuid and provider = $2`
	var key string
	err := s.pool.QueryRow(ctx, q, accountID, provider).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return key, err
}

// PutAccountSecret faz upsert da key da conta. apiKey vazio => DELETE (limpar). O
// escopo por account_id na PK garante que a conta so escreve o proprio secret.
func (s *Store) PutAccountSecret(ctx context.Context, accountID, provider, apiKey, updatedBy string) error {
	if strings.TrimSpace(apiKey) == "" {
		const del = `delete from calendar.ai_secrets where account_id = $1::uuid and provider = $2`
		_, err := s.pool.Exec(ctx, del, accountID, provider)
		return err
	}
	const q = `
		insert into calendar.ai_secrets (account_id, provider, api_key, updated_by, updated_at)
		values ($1::uuid, $2, $3, $4, now())
		on conflict (account_id, provider) do update
		set api_key = excluded.api_key, updated_by = excluded.updated_by, updated_at = now()`
	_, err := s.pool.Exec(ctx, q, accountID, provider, apiKey, updatedBy)
	return err
}

// GetGlobalSecrets le as keys globais de core.platform_settings (key
// 'calendar_ai_secrets'), mesmo padrao do media_limits. Sem linha => tudo vazio.
func (s *Store) GetGlobalSecrets(ctx context.Context) (GlobalSecrets, error) {
	const q = `select config from core.platform_settings where key = 'calendar_ai_secrets'`
	var out GlobalSecrets
	var raw json.RawMessage
	err := s.pool.QueryRow(ctx, q).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out, nil
}

// PutGlobalSecret atualiza SO a key do provider no conjunto global e regrava o jsonb
// inteiro (preservando as outras keys). updatedBy = userID (uuid) ou vazio.
func (s *Store) PutGlobalSecret(ctx context.Context, provider, apiKey, updatedBy string) error {
	current, err := s.GetGlobalSecrets(ctx)
	if err != nil {
		return err
	}
	switch provider {
	case "gemini":
		current.Gemini = apiKey
	case "glm":
		current.GLM = apiKey
	case "openai":
		current.OpenAI = apiKey
	}
	body, err := json.Marshal(current)
	if err != nil {
		return err
	}
	const q = `
		insert into core.platform_settings (key, config, updated_at, updated_by)
		values ('calendar_ai_secrets', $1::jsonb, now(), $2::uuid)
		on conflict (key) do update
		set config = excluded.config, updated_at = now(), updated_by = excluded.updated_by`
	_, err = s.pool.Exec(ctx, q, body, nullUUID(updatedBy))
	return err
}
