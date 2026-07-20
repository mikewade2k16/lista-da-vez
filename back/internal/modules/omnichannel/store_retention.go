package omnichannel

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ============================================================================
// F13 — Persistencia do PURGE (deletes/scrubs por classe) + trilha purge_runs
// ============================================================================
//
// SEGURANCA — o purge e o bug mais perigoso desta fase: apagar dado de todos os tenants.
// TODA query carrega account_id = $1 NA propria query (defesa em profundidade, alem da
// validacao do handler) e um cutoff/lista de ids — NUNCA um DELETE cego. Nunca DROP, nunca
// TRUNCATE: so DELETE/UPDATE do que passou do prazo daquela conta.
//
// Cada delete/scrub e UM BATCH (<= purgeBatchSize linhas, sub-select com limit): transacao
// curta, sem lock gigante na tabela quente do inbox (C4). O handler DRENA em loop ate o batch
// vir incompleto (drainBatches), respeitando o teto de tempo do job.

// purgeBatchSize e o teto de linhas por batch (C4).
const purgeBatchSize = 500

// RetentionStore roda os deletes/scrubs do purge.
type RetentionStore struct {
	pool *pgxpool.Pool
}

// NewRetentionStore monta o store sobre o pool da plataforma.
func NewRetentionStore(pool *pgxpool.Pool) *RetentionStore {
	return &RetentionStore{pool: pool}
}

// AccountsWithModule lista as contas com o modulo omnichannel HABILITADO. Fonte do
// enfileirador diario — a lista vem do catalogo, nunca hardcoded (C4).
func (s *RetentionStore) AccountsWithModule(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `select account_id::text from core.account_modules
		where module_id = 'omnichannel' and enabled`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ============================================================================
// Classe `ephemeral` (30d default) — webhook_events + outbox terminal
// ============================================================================

// CountEphemeral conta o que a classe apagaria (dry-run). Filtra por conta.
func (s *RetentionStore) CountEphemeral(ctx context.Context, accountID string, cutoff time.Time) (int64, error) {
	a, err := s.count(ctx, `select count(*) from messaging.webhook_events
		where account_id = $1::uuid and received_at < $2`, accountID, cutoff)
	if err != nil {
		return 0, err
	}
	b, err := s.count(ctx, `select count(*) from messaging.outbox
		where account_id = $1::uuid and status in ('done','dead') and created_at < $2`, accountID, cutoff)
	return a + b, err
}

// DeleteWebhookBatch apaga ate purgeBatchSize webhook_events anteriores ao cutoff.
func (s *RetentionStore) DeleteWebhookBatch(ctx context.Context, accountID string, cutoff time.Time) (int64, error) {
	return s.exec(ctx, `delete from messaging.webhook_events
		where id in (select id from messaging.webhook_events
			where account_id = $1::uuid and received_at < $2
			order by received_at limit $3)`, accountID, cutoff, purgeBatchSize)
}

// DeleteOutboxBatch apaga ate purgeBatchSize outbox em done/dead anteriores ao cutoff. NUNCA
// pending/processing/failed (podem reenviar) — so os terminais.
func (s *RetentionStore) DeleteOutboxBatch(ctx context.Context, accountID string, cutoff time.Time) (int64, error) {
	return s.exec(ctx, `delete from messaging.outbox
		where id in (select id from messaging.outbox
			where account_id = $1::uuid and status in ('done','dead') and created_at < $2
			order by created_at limit $3)`, accountID, cutoff, purgeBatchSize)
}

// ============================================================================
// Classe `ai_io` (90d default) — SCRUB de input/output; a linha SOBREVIVE (C1.2)
// ============================================================================
//
// SCRUB, nunca DELETE: ai_runs carrega o custo por conta (total_tokens/cost_usd) que ESTA
// fase entrega — apagar a linha destruiria a base do custo (C1.2). Zera-se o payload; os
// contadores seguem ate a classe audit.

// CountAIIO conta as linhas com payload a limpar (dry-run). Filtra por conta.
func (s *RetentionStore) CountAIIO(ctx context.Context, accountID string, cutoff time.Time) (int64, error) {
	a, err := s.count(ctx, `select count(*) from messaging.ai_runs
		where account_id = $1::uuid and created_at < $2
		  and (input <> '{}'::jsonb or output <> '{}'::jsonb)`, accountID, cutoff)
	if err != nil {
		return 0, err
	}
	b, err := s.count(ctx, `select count(*) from messaging.routing_decisions
		where account_id = $1::uuid and decided_at < $2 and input <> '{}'::jsonb`, accountID, cutoff)
	return a + b, err
}

// ScrubAIRunsBatch zera input/output de ate purgeBatchSize ai_runs anteriores ao cutoff.
func (s *RetentionStore) ScrubAIRunsBatch(ctx context.Context, accountID string, cutoff time.Time) (int64, error) {
	return s.exec(ctx, `update messaging.ai_runs set input = '{}'::jsonb, output = '{}'::jsonb
		where id in (select id from messaging.ai_runs
			where account_id = $1::uuid and created_at < $2
			  and (input <> '{}'::jsonb or output <> '{}'::jsonb)
			order by created_at limit $3)`, accountID, cutoff, purgeBatchSize)
}

// ScrubRoutingDecisionsBatch zera input de ate purgeBatchSize routing_decisions. Some o
// snapshot de PII; ficam rule_id/outcome/reason e a decisao segue explicavel (F8).
func (s *RetentionStore) ScrubRoutingDecisionsBatch(ctx context.Context, accountID string, cutoff time.Time) (int64, error) {
	return s.exec(ctx, `update messaging.routing_decisions set input = '{}'::jsonb
		where id in (select id from messaging.routing_decisions
			where account_id = $1::uuid and decided_at < $2 and input <> '{}'::jsonb
			order by decided_at limit $3)`, accountID, cutoff, purgeBatchSize)
}

// ============================================================================
// Classe `conversation` (180d default) — DELETE ancorado em last_message_at (C1.1)
// ============================================================================

// ConversationsToPurge devolve ate purgeBatchSize ids de conversas paradas ha mais que o
// cutoff (last_message_at, NUNCA created_at da mensagem: podar conversa ATIVA seria apagar o
// historico no meio do atendimento). Em QUALQUER state (inclusive aberta — senao conversa que
// ninguem fecha vira retencao infinita).
func (s *RetentionStore) ConversationsToPurge(ctx context.Context, accountID string, cutoff time.Time) ([]string, error) {
	rows, err := s.pool.Query(ctx, `select id::text from messaging.conversations
		where account_id = $1::uuid and last_message_at < $2
		order by last_message_at limit $3`, accountID, cutoff, purgeBatchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0, purgeBatchSize)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// CountConversationsToPurge conta as conversas a apagar (dry-run). Filtra por conta.
func (s *RetentionStore) CountConversationsToPurge(ctx context.Context, accountID string, cutoff time.Time) (int64, error) {
	return s.count(ctx, `select count(*) from messaging.conversations
		where account_id = $1::uuid and last_message_at < $2`, accountID, cutoff)
}

// CountMediaToPurge conta as mensagens COM midia em disco das conversas a apagar (dry-run:
// quantos arquivos cairiam). Filtra por conta.
func (s *RetentionStore) CountMediaToPurge(ctx context.Context, accountID string, cutoff time.Time) (int64, error) {
	return s.count(ctx, `select count(*) from messaging.messages m
		where m.account_id = $1::uuid and m.media_storage_key is not null and m.media_storage_key <> ''
		  and exists (select 1 from messaging.conversations c
			where c.id = m.conversation_id and c.account_id = m.account_id and c.last_message_at < $2)`,
		accountID, cutoff)
}

// ScrubRunsForConversations zera input/output dos ai_runs das conversas a apagar. ai_runs NAO
// tem FK para conversations (F9 C9.1) => NAO cascateia: sem este scrub explicito o texto do
// cliente sobreviveria a conversa apagada para sempre (C1.3 — o vazamento mais facil de deixar
// passar). Roda ANTES do DELETE das conversas. Filtra por conta.
func (s *RetentionStore) ScrubRunsForConversations(ctx context.Context, accountID string, convIDs []string) (int64, error) {
	if len(convIDs) == 0 {
		return 0, nil
	}
	return s.exec(ctx, `update messaging.ai_runs set input = '{}'::jsonb, output = '{}'::jsonb
		where account_id = $1::uuid and conversation_id::text = any($2::text[])
		  and (input <> '{}'::jsonb or output <> '{}'::jsonb)`, accountID, convIDs)
}

// DeleteConversations apaga as conversas dadas. O ON DELETE CASCADE derruba messages,
// hidden_messages e routing_decisions (F2/F8). Filtra por conta. Roda DEPOIS do scrub de
// ai_runs e da remocao dos arquivos de midia (C5: arquivo primeiro, linha depois).
func (s *RetentionStore) DeleteConversations(ctx context.Context, accountID string, convIDs []string) (int64, error) {
	if len(convIDs) == 0 {
		return 0, nil
	}
	return s.exec(ctx, `delete from messaging.conversations
		where account_id = $1::uuid and id::text = any($2::text[])`, accountID, convIDs)
}

// CountOrphanContacts conta os contatos sem conversa restante e mais velhos que o cutoff (dry-run).
func (s *RetentionStore) CountOrphanContacts(ctx context.Context, accountID string, cutoff time.Time) (int64, error) {
	return s.count(ctx, `select count(*) from messaging.contacts c `+orphanContactWhere, accountID, cutoff)
}

// DeleteOrphanContactsBatch apaga ate purgeBatchSize contatos SEM conversa restante e mais
// velhos que o cutoff (a mesma janela da classe conversation). O gate por created_at protege o
// contato recem-criado manualmente que ainda nao teve conversa. Filtra por conta.
func (s *RetentionStore) DeleteOrphanContactsBatch(ctx context.Context, accountID string, cutoff time.Time) (int64, error) {
	return s.exec(ctx, `delete from messaging.contacts
		where id in (select c.id from messaging.contacts c `+orphanContactWhere+`
			order by c.created_at limit $3)`, accountID, cutoff, purgeBatchSize)
}

// orphanContactWhere e o predicado do contato orfao (reusado pela contagem e pelo delete).
const orphanContactWhere = `where c.account_id = $1::uuid and c.created_at < $2
	and not exists (select 1 from messaging.conversations v
		where v.contact_id = c.id and v.account_id = c.account_id)`

// ============================================================================
// Classe `audit` (365d default) — DELETE audit_events + purge_runs (auto-poda)
// ============================================================================

// CountAudit conta o que a classe apagaria (dry-run). purge_runs poda a si mesma (C3).
func (s *RetentionStore) CountAudit(ctx context.Context, accountID string, cutoff time.Time) (int64, error) {
	a, err := s.count(ctx, `select count(*) from messaging.audit_events
		where account_id = $1::uuid and created_at < $2`, accountID, cutoff)
	if err != nil {
		return 0, err
	}
	b, err := s.count(ctx, `select count(*) from messaging.purge_runs
		where account_id = $1::uuid and started_at < $2`, accountID, cutoff)
	return a + b, err
}

// DeleteAuditEventsBatch apaga ate purgeBatchSize audit_events anteriores ao cutoff.
func (s *RetentionStore) DeleteAuditEventsBatch(ctx context.Context, accountID string, cutoff time.Time) (int64, error) {
	return s.exec(ctx, `delete from messaging.audit_events
		where id in (select id from messaging.audit_events
			where account_id = $1::uuid and created_at < $2
			order by created_at limit $3)`, accountID, cutoff, purgeBatchSize)
}

// DeletePurgeRunsBatch apaga ate purgeBatchSize purge_runs anteriores ao cutoff (a tabela de
// evidencia poda a si mesma aos 365d — senao cresce para sempre, C3).
func (s *RetentionStore) DeletePurgeRunsBatch(ctx context.Context, accountID string, cutoff time.Time) (int64, error) {
	return s.exec(ctx, `delete from messaging.purge_runs
		where id in (select id from messaging.purge_runs
			where account_id = $1::uuid and started_at < $2
			order by started_at limit $3)`, accountID, cutoff, purgeBatchSize)
}

// ============================================================================
// Midia viva (para a varredura de orfaos) e trilha
// ============================================================================

// KnownMediaKeys devolve o conjunto de media_storage_key vivos da conta (para a varredura de
// orfaos saber quais arquivos AINDA sao referenciados). Filtra por conta.
func (s *RetentionStore) KnownMediaKeys(ctx context.Context, accountID string) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `select media_storage_key from messaging.messages
		where account_id = $1::uuid and media_storage_key is not null and media_storage_key <> ''`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out[key] = true
	}
	return out, rows.Err()
}

// purgeRunRecord e uma linha de messaging.purge_runs (uma por conta+classe por execucao).
type purgeRunRecord struct {
	AccountID    string
	Class        string
	Mode         string
	CutoffAt     time.Time
	RowsDeleted  int64
	RowsScrubbed int64
	FilesDeleted int64
	BytesFreed   int64
	StartedAt    time.Time
	FinishedAt   time.Time
	Error        string // JA mascarado (jobs.MaskError) — nunca payload/PII (C6)
}

// RecordPurgeRun grava a evidencia da poda. account_id do produtor (server-side), nunca de body.
func (s *RetentionStore) RecordPurgeRun(ctx context.Context, r purgeRunRecord) error {
	_, err := s.pool.Exec(ctx, `insert into messaging.purge_runs
		(account_id, class, mode, cutoff_at, rows_deleted, rows_scrubbed, files_deleted,
		 bytes_freed, started_at, finished_at, error)
		values ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		r.AccountID, r.Class, r.Mode, r.CutoffAt, r.RowsDeleted, r.RowsScrubbed,
		r.FilesDeleted, r.BytesFreed, r.StartedAt, r.FinishedAt, r.Error)
	return err
}

// count roda um select count(*) escalar.
func (s *RetentionStore) count(ctx context.Context, query string, args ...any) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, query, args...).Scan(&n)
	return n, err
}

// exec roda um update/delete e devolve as linhas afetadas.
func (s *RetentionStore) exec(ctx context.Context, query string, args ...any) (int64, error) {
	tag, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
