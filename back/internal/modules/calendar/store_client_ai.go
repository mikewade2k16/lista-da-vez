package calendar

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Persistencia do override de IA por cliente (WAVE 3.1, SEC+). Mora na coluna
// ai_config de calendar.client_profiles (migration 0190), 1:1 por (account, cliente).
// A API key NUNCA entra aqui — o override so guarda COMPORTAMENTO.

// GetClientAIOverride le o override de IA do cliente (account, cliente) de
// client_profiles.ai_config. Segundo retorno = ha algum campo setado (overrideHasValue).
// WHERE por account_id (defesa em profundidade): conta A nunca le override de B. Sem
// linha (perfil inexistente) => override vazio, false.
func (s *Store) GetClientAIOverride(ctx context.Context, accountID, clientID string) (ClientAIOverride, bool, error) {
	const q = `select coalesce(ai_config, '{}'::jsonb)
		from calendar.client_profiles
		where account_id = $1::uuid and client_id = $2::uuid`
	var raw json.RawMessage
	err := s.pool.QueryRow(ctx, q, accountID, clientID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return ClientAIOverride{}, false, nil
	}
	if err != nil {
		return ClientAIOverride{}, false, err
	}
	var ov ClientAIOverride
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &ov)
	}
	return ov, overrideHasValue(ov), nil
}

// PutClientAIOverride faz upsert do override no ai_config do cliente. O perfil pode
// nao existir ainda: o INSERT cria a linha so com account/client/ai_config (as demais
// colunas do perfil ficam no default); no CONFLICT atualiza SO o ai_config (preservando
// o perfil estrategico). updated_by/updated_at acompanham a escrita. A key nunca entra.
func (s *Store) PutClientAIOverride(ctx context.Context, accountID, clientID string, ov ClientAIOverride, updatedBy string) error {
	body, err := json.Marshal(ov)
	if err != nil {
		return err
	}
	const q = `
		insert into calendar.client_profiles (account_id, client_id, ai_config, updated_by, updated_at)
		values ($1::uuid, $2::uuid, $3::jsonb, $4, now())
		on conflict (account_id, client_id) do update
		set ai_config = excluded.ai_config, updated_by = excluded.updated_by, updated_at = now()`
	_, err = s.pool.Exec(ctx, q, accountID, clientID, body, updatedBy)
	return err
}
