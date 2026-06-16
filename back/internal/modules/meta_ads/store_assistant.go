package metaads

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ============================================================================
// assistant_messages
// ============================================================================

// ListAssistantMessages retorna as ULTIMAS `limit` mensagens do chat da account,
// reordenadas em ordem cronologica (created_at asc) para exibicao/contexto:
// subselect pega as N mais recentes (desc) e o select externo reordena (asc).
func (s *Store) ListAssistantMessages(ctx context.Context, accountID string, limit int) ([]AssistantMessage, error) {
	const q = `select id, account_id, role, content, actions, created_at
		from (
			select id, account_id, role, content, actions, created_at
			from meta_ads.assistant_messages
			where account_id = $1
			order by created_at desc, id desc
			limit $2
		) recent
		order by created_at asc, id asc`
	rows, err := s.pool.Query(ctx, q, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AssistantMessage
	for rows.Next() {
		m, scanErr := scanAssistantMessage(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// InsertAssistantMessage persiste uma mensagem do chat. actions e o jsonb cru
// ([]AssistantAction serializado); nil/vazio grava NULL.
func (s *Store) InsertAssistantMessage(ctx context.Context, accountID, role, content string, actions []byte) (AssistantMessage, error) {
	const q = `insert into meta_ads.assistant_messages (account_id, role, content, actions)
		values ($1, $2, $3, $4)
		returning id, account_id, role, content, actions, created_at`
	var actionsParam any
	if len(actions) > 0 {
		actionsParam = actions
	}
	return scanAssistantMessage(s.pool.QueryRow(ctx, q, accountID, role, content, actionsParam))
}

// DeleteAssistantMessages apaga todo o historico do chat da account (botao
// "limpar conversa"). Escopado por account_id.
func (s *Store) DeleteAssistantMessages(ctx context.Context, accountID string) error {
	const q = `delete from meta_ads.assistant_messages where account_id = $1`
	_, err := s.pool.Exec(ctx, q, accountID)
	return err
}

// GetAssistantSettings le model + system_prompt da account. found=false se a
// account ainda nao customizou (sem linha).
func (s *Store) GetAssistantSettings(ctx context.Context, accountID string) (model, systemPrompt string, found bool, err error) {
	const q = `select model, system_prompt from meta_ads.assistant_settings where account_id = $1`
	err = s.pool.QueryRow(ctx, q, accountID).Scan(&model, &systemPrompt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return model, systemPrompt, true, nil
}

// UpsertAssistantSettings grava model + system_prompt da account.
func (s *Store) UpsertAssistantSettings(ctx context.Context, accountID, model, systemPrompt string) error {
	const q = `insert into meta_ads.assistant_settings (account_id, model, system_prompt, updated_at)
		values ($1, $2, $3, now())
		on conflict (account_id) do update
		set model = excluded.model, system_prompt = excluded.system_prompt, updated_at = now()`
	_, err := s.pool.Exec(ctx, q, accountID, model, systemPrompt)
	return err
}

func scanAssistantMessage(row rowScanner) (AssistantMessage, error) {
	var m AssistantMessage
	err := row.Scan(&m.ID, &m.AccountID, &m.Role, &m.Content, &m.Actions, &m.CreatedAt)
	return m, err
}
