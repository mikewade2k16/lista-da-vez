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

// UpsertAIDispatch agrupa uma mensagem inbound numa janela durável. Para WhatsApp, a instância
// é bloqueada antes da conversa, na mesma ordem do reset de histórico; depois vem o dispatch.
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

	// Não bloqueia a conversa nesta consulta. Se um reset estiver em andamento, esperamos a
	// instância sem segurar o lock que ele precisa para incrementar ai_generation.
	var lockedInstanceID string
	lockErr := tx.QueryRow(ctx, `select history_instance.id::text
		from messaging.conversations history_conversation
		join messaging.whatsapp_instances history_instance
		  on history_instance.account_id=history_conversation.account_id
		 and (history_instance.id=history_conversation.instance_id
		   or (history_conversation.instance_id is null
		     and history_instance.instance_name=history_conversation.instance_scope_key))
		where history_conversation.account_id=$1::uuid
		  and history_conversation.id=$2::uuid
		  and history_conversation.channel='WHATSAPP'
		for share of history_instance`, accountID, conversationID).Scan(&lockedInstanceID)
	if lockErr != nil && !errors.Is(lockErr, pgx.ErrNoRows) {
		return AIDispatchRecord{}, lockErr
	}

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

	var inboundExists, inboundVisible bool
	if err := tx.QueryRow(ctx, `select exists(
		select 1 from messaging.messages message
		where message.account_id=$1::uuid and message.conversation_id=$2::uuid
		  and message.id=$3::uuid and message.direction='INBOUND'
	), exists(
		select 1 from messaging.messages message
		join messaging.conversations conversation
		  on conversation.account_id=message.account_id and conversation.id=message.conversation_id
		where message.account_id = $1::uuid and message.conversation_id = $2::uuid
		  and message.id = $3::uuid and message.direction = 'INBOUND'`+
		s.historyVisibleMessagePredicate("message", "conversation")+`
	)`, accountID, conversationID, messageID).Scan(&inboundExists, &inboundVisible); err != nil {
		return AIDispatchRecord{}, err
	}
	if !inboundExists {
		return AIDispatchRecord{}, ErrNotFound
	}
	if !inboundVisible {
		return AIDispatchRecord{}, ErrHistoryResetInvalidated
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
// dispatch. AI execution has its own FIFO namespace: a debounce run_after must
// not head-of-line block a newer inbound intent on the conversation FIFO.
// Generation and lease checks still serialize every operational effect.
func enqueueAIDispatchJobTx(ctx context.Context, tx pgx.Tx, accountID, dispatchID, conversationID string, generation int64, runAfter time.Time) error {
	payload, err := json.Marshal(aiDispatchJobPayload{DispatchID: dispatchID, Generation: generation})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `insert into messaging.outbox
		(account_id, ordering_key, idempotency_key, kind, payload, max_attempts, run_after)
		values ($1::uuid, $2, $3, $4, $5::jsonb, 5, $6)
		on conflict (account_id, idempotency_key) do nothing`,
		accountID, aiDispatchOrderingKey(conversationID),
		fmt.Sprintf("ai-dispatch:%s:%d", dispatchID, generation),
		AIDispatchJobKind, payload, runAfter)
	return err
}

func aiDispatchOrderingKey(conversationID string) string {
	return "ai-dispatch:" + strings.TrimSpace(conversationID)
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

// WithAIDispatchExternalEffectLease mantem a instancia sob SHARE durante o boundary externo.
// Isso exclui o reset sem bloquear inbound/ai_generation enquanto um modelo lento responde;
// conversa, dispatch e mensagens sao relidos depois do lock da instancia e o apply posterior
// continua responsavel por descartar uma geracao superada por inbound.
func (s *Store) WithAIDispatchExternalEffectLease(ctx context.Context, accountID, dispatchID string,
	generation int64, effect func() error) (bool, error) {
	return s.withAIDispatchExternalEffectLease(ctx, accountID, dispatchID, generation, false, effect)
}

// WithAIDispatchExternalEffectLeaseNowait e a variante dos gateways internos chamados pelo n8n.
// Se um reset exclusivo ja estiver aguardando/adquirido, ela falha fechado sem criar um ciclo
// HTTP -> banco -> HTTP entre o lease exterior e o gateway.
func (s *Store) WithAIDispatchExternalEffectLeaseNowait(ctx context.Context, accountID, dispatchID string,
	generation int64, effect func() error) (bool, error) {
	return s.withAIDispatchExternalEffectLease(ctx, accountID, dispatchID, generation, true, effect)
}

func (s *Store) withAIDispatchExternalEffectLease(ctx context.Context, accountID, dispatchID string,
	generation int64, noWait bool, effect func() error) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var conversationID string
	err = tx.QueryRow(ctx, `select conversation_id::text from messaging.ai_dispatches
		where account_id=$1::uuid and id=$2::uuid and generation=$3`,
		accountID, dispatchID, generation).Scan(&conversationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	lock := lockHistoryExternalEffectScope
	if noWait {
		lock = lockHistoryExternalEffectScopeNowait
	}
	if err := lock(ctx, tx, accountID, conversationID, "none"); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrHistoryResetInvalidated) || isHistoryEffectLockUnavailable(err) {
			return false, nil
		}
		return false, err
	}
	var allowed bool
	err = tx.QueryRow(ctx, `select exists (
		select 1 from messaging.ai_dispatches dispatch
		join messaging.conversations conversation
		  on conversation.account_id=dispatch.account_id and conversation.id=dispatch.conversation_id
		where dispatch.account_id=$1::uuid and dispatch.id=$2::uuid
		  and dispatch.generation=$3 and dispatch.status='processing'
		  and conversation.ai_generation=dispatch.generation and conversation.state='ai_active'
		  and cardinality(dispatch.message_ids)>0
		  and not exists (
			select 1 from unnest(dispatch.message_ids) captured(message_id)
			where not exists (
				select 1 from messaging.messages history_message
				where history_message.account_id=dispatch.account_id
				  and history_message.conversation_id=dispatch.conversation_id
				  and history_message.id=captured.message_id`+
		s.historyVisibleMessagePredicate("history_message", "conversation")+`
			)
		  )
	)`, accountID, dispatchID, generation).Scan(&allowed)
	if err != nil || !allowed {
		return allowed, err
	}
	if effect != nil {
		if err := effect(); err != nil {
			return true, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return true, err
	}
	return true, nil
}

// AIDispatchExternalEffectAllowed permanece como preflight barato para call-sites sem efeito.
// Boundaries externos devem usar WithAIDispatchExternalEffectLease para manter o lock ate o
// callback terminar.
func (s *Store) AIDispatchExternalEffectAllowed(ctx context.Context, accountID, dispatchID string, generation int64) (bool, error) {
	return s.WithAIDispatchExternalEffectLease(ctx, accountID, dispatchID, generation, nil)
}

// WithAIIntelligenceAcceptanceLease protege o bridge de aceite depois que o dispatch ja foi
// completado. Ele nao pode reutilizar o gate de processing: o evento nasce justamente no commit
// que muda o dispatch para completed. Reset-first incrementa ai_generation e oculta as mensagens,
// fazendo a revalidacao abaixo falhar antes do callback.
func (s *Store) WithAIIntelligenceAcceptanceLease(ctx context.Context, accountID, dispatchID string,
	generation int64, effect func() error) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var conversationID string
	err = tx.QueryRow(ctx, `select conversation_id::text from messaging.ai_dispatches
		where account_id=$1::uuid and id=$2::uuid and generation=$3`,
		accountID, dispatchID, generation).Scan(&conversationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := lockHistoryExternalEffectScope(ctx, tx, accountID, conversationID, "none"); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrHistoryResetInvalidated) {
			return false, nil
		}
		return false, err
	}
	var allowed bool
	err = tx.QueryRow(ctx, `select exists (
		select 1 from messaging.ai_dispatches dispatch
		join messaging.conversations conversation
		  on conversation.account_id=dispatch.account_id and conversation.id=dispatch.conversation_id
		where dispatch.account_id=$1::uuid and dispatch.id=$2::uuid
		  and dispatch.generation=$3 and dispatch.status='completed'
		  and conversation.ai_generation=dispatch.generation
		  and cardinality(dispatch.message_ids)>0
		  and not exists (
			select 1 from unnest(dispatch.message_ids) captured(message_id)
			where not exists (
				select 1 from messaging.messages history_message
				where history_message.account_id=dispatch.account_id
				  and history_message.conversation_id=dispatch.conversation_id
				  and history_message.id=captured.message_id`+
		s.historyVisibleMessagePredicate("history_message", "conversation")+`
			)
		  )
	)`, accountID, dispatchID, generation).Scan(&allowed)
	if err != nil || !allowed {
		return allowed, err
	}
	if effect != nil {
		if err := effect(); err != nil {
			return true, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return true, err
	}
	return true, nil
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
