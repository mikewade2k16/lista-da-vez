package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const aiDispatchColumns = `id::text, account_id::text, conversation_id::text, agent_version_id::text,
	generation, status, message_ids::text[], run_after, locked_at, completed_at, idempotency_key,
	result_run_id::text, last_error, created_at, updated_at`

type aiDispatchConfig struct {
	DebounceMS         int
	MaxContextMessages int
	MaxAITurns         int
	MinConfidence      float64
	HandoffOnError     bool
	HandoffOnLimit     bool
	WorkflowContract   string
}

func scanAIDispatch(row rowScanner) (AIDispatchRecord, error) {
	var out AIDispatchRecord
	var status string
	if err := row.Scan(
		&out.ID, &out.AccountID, &out.ConversationID, &out.AgentVersionID,
		&out.Generation, &status, &out.MessageIDs, &out.RunAfter, &out.LockedAt,
		&out.CompletedAt, &out.IdempotencyKey, &out.ResultRunID, &out.LastError,
		&out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		return AIDispatchRecord{}, err
	}
	out.Status = AIDispatchStatus(status)
	return out, nil
}

// UpsertAIDispatch agrupa uma mensagem inbound numa janela durável. A conversa é sempre o
// primeiro lock; depois o dispatch. Isso mantém a mesma ordem usada pelo takeover/handoff e
// evita deadlock entre humano, inbound e worker.
//
// A chamada é idempotente por messageID: reentrega não incrementa generation nem cria outro
// dispatch. Uma mensagem nova incrementa conversations.ai_generation na mesma transação,
// invalidando qualquer resultado de IA atrasado.
func (s *Store) UpsertAIDispatch(ctx context.Context, accountID, conversationID, agentVersionID,
	messageID string, runAfter time.Time) (AIDispatchRecord, error) {
	accountID = strings.TrimSpace(accountID)
	conversationID = strings.TrimSpace(conversationID)
	agentVersionID = strings.TrimSpace(agentVersionID)
	messageID = strings.TrimSpace(messageID)
	if accountID == "" || conversationID == "" || agentVersionID == "" || messageID == "" {
		return AIDispatchRecord{}, ErrAIDispatchInvalidInput
	}
	if runAfter.IsZero() {
		runAfter = time.Now().UTC()
	}
	runAfter = runAfter.UTC()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AIDispatchRecord{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var state string
	var currentGeneration int64
	err = tx.QueryRow(ctx, `select state, ai_generation
		from messaging.conversations
		where account_id = $1::uuid and id = $2::uuid
		for update`, accountID, conversationID).Scan(&state, &currentGeneration)
	if errors.Is(err, pgx.ErrNoRows) {
		return AIDispatchRecord{}, ErrNotFound
	}
	if err != nil {
		return AIDispatchRecord{}, err
	}
	if state != string(StateAIActive) {
		return AIDispatchRecord{}, ErrAILeaseInvalid
	}

	var inboundExists bool
	if err := tx.QueryRow(ctx, `select exists(
		select 1 from messaging.messages
		where account_id = $1::uuid and conversation_id = $2::uuid
		  and id = $3::uuid and direction = 'INBOUND'
	)`, accountID, conversationID, messageID).Scan(&inboundExists); err != nil {
		return AIDispatchRecord{}, err
	}
	if !inboundExists {
		return AIDispatchRecord{}, ErrNotFound
	}

	// Primeiro reconhece reentrega de uma mensagem que já passou por qualquer dispatch, inclusive
	// completed/failed. Isso evita uma nova chamada ao modelo quando o webhook é repetido depois
	// que a janela original terminou.
	var existing AIDispatchRecord
	existing, err = scanAIDispatch(tx.QueryRow(ctx, `select `+aiDispatchColumns+`
		from messaging.ai_dispatches
		where account_id = $1::uuid and conversation_id = $2::uuid
		  and $3::uuid = any(message_ids)
		order by created_at desc, id desc limit 1`, accountID, conversationID, messageID))
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return AIDispatchRecord{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return AIDispatchRecord{}, err
	}

	var activeID string
	var activeGeneration int64
	err = tx.QueryRow(ctx, `select id::text, generation
		from messaging.ai_dispatches
		where account_id = $1::uuid and conversation_id = $2::uuid
		  and status in ('buffering','queued','processing')
		order by created_at desc, id desc limit 1
		for update`, accountID, conversationID).Scan(&activeID, &activeGeneration)
	newGeneration := currentGeneration + 1
	idempotencyKey := aiDispatchIdempotencyKey(conversationID, newGeneration)
	if err == nil {
		// O generation é monotônica da conversa; activeGeneration só serve como proteção contra
		// dados antigos caso a conversa tenha sido reparada fora do fluxo normal.
		if activeGeneration >= newGeneration {
			newGeneration = activeGeneration + 1
			idempotencyKey = aiDispatchIdempotencyKey(conversationID, newGeneration)
		}
		if _, err := tx.Exec(ctx, `update messaging.conversations
			set ai_generation = $3, updated_at = now()
			where account_id = $1::uuid and id = $2::uuid`, accountID, conversationID, newGeneration); err != nil {
			return AIDispatchRecord{}, err
		}
		if _, err := tx.Exec(ctx, `update messaging.ai_dispatches
			set generation = $3, message_ids = array_append(message_ids, $4::uuid),
			    run_after = $5, idempotency_key = $6, agent_version_id = $7::uuid, updated_at = now()
			where account_id = $1::uuid and id = $2::uuid`, accountID, activeID,
			newGeneration, messageID, runAfter, idempotencyKey, agentVersionID); err != nil {
			return AIDispatchRecord{}, err
		}
		if err := enqueueAIDispatchJobTx(ctx, tx, accountID, activeID, conversationID, newGeneration, runAfter); err != nil {
			return AIDispatchRecord{}, err
		}
		existing, err := scanAIDispatch(tx.QueryRow(ctx, `select `+aiDispatchColumns+`
			from messaging.ai_dispatches where account_id = $1::uuid and id = $2::uuid`, accountID, activeID))
		if err != nil {
			return AIDispatchRecord{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return AIDispatchRecord{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return AIDispatchRecord{}, err
	}

	if _, err := tx.Exec(ctx, `update messaging.conversations
		set ai_generation = $3, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`, accountID, conversationID, newGeneration); err != nil {
		return AIDispatchRecord{}, err
	}
	created, err := scanAIDispatch(tx.QueryRow(ctx, `insert into messaging.ai_dispatches
		(account_id, conversation_id, agent_version_id, generation, status, message_ids,
		 run_after, idempotency_key)
		values ($1::uuid, $2::uuid, $3::uuid, $4, 'buffering', array[$5::uuid], $6, $7)
		returning `+aiDispatchColumns, accountID, conversationID, agentVersionID,
		newGeneration, messageID, runAfter, idempotencyKey))
	if err != nil {
		return AIDispatchRecord{}, err
	}
	if err := enqueueAIDispatchJobTx(ctx, tx, accountID, created.ID, conversationID, newGeneration, runAfter); err != nil {
		return AIDispatchRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AIDispatchRecord{}, err
	}
	return created, nil
}

// enqueueAIDispatchJobTx writes the generic outbox row in the same transaction as the
// dispatch. The worker is the only executor; this row contains identifiers only.
func enqueueAIDispatchJobTx(ctx context.Context, tx pgx.Tx, accountID, dispatchID, conversationID string, generation int64, runAfter time.Time) error {
	payload, err := json.Marshal(aiDispatchJobPayload{DispatchID: dispatchID, Generation: generation})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `insert into messaging.outbox
		(account_id, ordering_key, idempotency_key, kind, payload, max_attempts, run_after)
		values ($1::uuid, $2, $3, $4, $5::jsonb, 5, $6)
		on conflict (account_id, idempotency_key) do nothing`,
		accountID, conversationID,
		fmt.Sprintf("ai-dispatch:%s:%d", dispatchID, generation),
		AIDispatchJobKind, payload, runAfter)
	return err
}

// GetAIDispatch lê uma linha com escopo explícito. Outra conta é indistinguível de inexistente.
func (s *Store) GetAIDispatch(ctx context.Context, accountID, dispatchID string) (AIDispatchRecord, error) {
	accountID = strings.TrimSpace(accountID)
	dispatchID = strings.TrimSpace(dispatchID)
	if accountID == "" || dispatchID == "" {
		return AIDispatchRecord{}, ErrAIDispatchInvalidInput
	}
	var out AIDispatchRecord
	err := func() error {
		var err error
		out, err = scanAIDispatch(s.pool.QueryRow(ctx, `select `+aiDispatchColumns+`
			from messaging.ai_dispatches where account_id = $1::uuid and id = $2::uuid`,
			accountID, dispatchID))
		return err
	}()
	if errors.Is(err, pgx.ErrNoRows) {
		return AIDispatchRecord{}, ErrNotFound
	}
	return out, err
}

// ClaimAIDispatches reserva jobs elegíveis em uma única operação. O claim não executa n8n;
// apenas muda o estado durável para processing depois de FOR UPDATE SKIP LOCKED.
func (s *Store) ClaimAIDispatches(ctx context.Context, workerID string, limit int, now time.Time) ([]AIDispatchRecord, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" || limit <= 0 {
		return nil, ErrAIDispatchInvalidInput
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	rows, err := s.pool.Query(ctx, `with candidates as (
		select id
		from messaging.ai_dispatches
		where status in ('buffering','queued') and run_after <= $1
		order by run_after, created_at, id
		for update skip locked
		limit $2
	)
	update messaging.ai_dispatches d
	set status = 'processing', locked_at = $1, last_error = '', updated_at = now()
	from candidates c
	where d.id = c.id and d.status in ('buffering','queued')
	returning `+aiDispatchColumns, now.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AIDispatchRecord, 0, limit)
	for rows.Next() {
		row, err := scanAIDispatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// CompleteAIDispatch só aceita o generation ainda em processamento; resultado atrasado vira
// no-op. resultRunID é opcional para permitir o registro do run antes da conclusão.
func (s *Store) CompleteAIDispatch(ctx context.Context, accountID, dispatchID string,
	generation int64, resultRunID *string) (bool, error) {
	accountID = strings.TrimSpace(accountID)
	dispatchID = strings.TrimSpace(dispatchID)
	if accountID == "" || dispatchID == "" || generation < 0 {
		return false, ErrAIDispatchInvalidInput
	}
	result, err := s.pool.Exec(ctx, `update messaging.ai_dispatches
		set status = 'completed', completed_at = now(), result_run_id = $4::uuid,
		    locked_at = null, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid and generation = $3
		  and status = 'processing'`, accountID, dispatchID, generation, resultRunID)
	return result.RowsAffected() == 1, err
}

func (s *Store) AIDispatchSchemaAvailable(ctx context.Context) (bool, error) {
	var relation *string
	if err := s.pool.QueryRow(ctx, "select to_regclass('messaging.ai_dispatches')::text").Scan(&relation); err != nil {
		return false, err
	}
	return relation != nil && strings.TrimSpace(*relation) != "", nil
}

func (s *Store) SetAIDispatchV2Enabled(enabled bool) {
	s.aiDispatchV2.Store(enabled)
}

func (s *Store) AIDispatchV2Enabled() bool {
	return s.aiDispatchV2.Load()
}

func (s *Store) AIDispatchConfig(ctx context.Context, accountID, versionID string) (aiDispatchConfig, error) {
	var cfg aiDispatchConfig
	err := s.pool.QueryRow(ctx, `select debounce_ms, max_context_messages, max_ai_turns,
		min_confidence::float8, handoff_on_error, handoff_on_limit, workflow_contract_version
		from messaging.ai_agent_versions
		where account_id = $1::uuid and id = $2::uuid`, accountID, versionID).Scan(
		&cfg.DebounceMS, &cfg.MaxContextMessages, &cfg.MaxAITurns, &cfg.MinConfidence,
		&cfg.HandoffOnError, &cfg.HandoffOnLimit, &cfg.WorkflowContract)
	if errors.Is(err, pgx.ErrNoRows) {
		return aiDispatchConfig{}, ErrNotFound
	}
	return cfg, err
}

func (s *Store) StartAIDispatch(ctx context.Context, accountID, dispatchID string, generation int64) (bool, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(dispatchID) == "" || generation < 0 {
		return false, ErrAIDispatchInvalidInput
	}
	query := "update messaging.ai_dispatches " +
		"set status = 'processing', locked_at = now(), last_error = '', updated_at = now() " +
		"where account_id = $1::uuid and id = $2::uuid and generation = $3 " +
		"and status in ('buffering','queued')"
	tag, err := s.pool.Exec(ctx, query, accountID, dispatchID, generation)
	return tag.RowsAffected() == 1, err
}

func (s *Store) RequeueAIDispatch(ctx context.Context, accountID, dispatchID, code string) (bool, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(dispatchID) == "" {
		return false, ErrAIDispatchInvalidInput
	}
	code = strings.TrimSpace(code)
	if len(code) > 200 {
		code = code[:200]
	}
	query := "update messaging.ai_dispatches " +
		"set status = 'queued', run_after = now(), locked_at = null, last_error = $3, updated_at = now() " +
		"where account_id = $1::uuid and id = $2::uuid and status = 'processing'"
	tag, err := s.pool.Exec(ctx, query, accountID, dispatchID, code)
	return tag.RowsAffected() == 1, err
}

// CancelAIDispatch invalida uma execução em voo (takeover/fechamento). Idempotente para estados
// já terminais; somente buffering/queued/processing são alterados.
func (s *Store) CancelAIDispatch(ctx context.Context, accountID, dispatchID, reason string) (bool, error) {
	accountID = strings.TrimSpace(accountID)
	dispatchID = strings.TrimSpace(dispatchID)
	if accountID == "" || dispatchID == "" {
		return false, ErrAIDispatchInvalidInput
	}
	reason = strings.TrimSpace(reason)
	if len(reason) > 200 {
		reason = reason[:200]
	}
	result, err := s.pool.Exec(ctx, `update messaging.ai_dispatches
		set status = 'cancelled', last_error = $3, locked_at = null, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid
		  and status in ('buffering','queued','processing')`, accountID, dispatchID, reason)
	return result.RowsAffected() == 1, err
}

// FailAIDispatch grava somente um código/motivo sanitizado e libera a conversa para a policy
// de fallback; o caller decide se deve reagendar ou transferir.
func (s *Store) FailAIDispatch(ctx context.Context, accountID, dispatchID, code string) (bool, error) {
	accountID = strings.TrimSpace(accountID)
	dispatchID = strings.TrimSpace(dispatchID)
	if accountID == "" || dispatchID == "" {
		return false, ErrAIDispatchInvalidInput
	}
	code = strings.TrimSpace(code)
	if len(code) > 200 {
		code = code[:200]
	}
	result, err := s.pool.Exec(ctx, `update messaging.ai_dispatches
		set status = 'failed', last_error = $3, locked_at = null, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid
		  and status in ('buffering','queued','processing')`, accountID, dispatchID, code)
	return result.RowsAffected() == 1, err
}
