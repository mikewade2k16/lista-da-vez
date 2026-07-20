package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultTable e a tabela do omnichannel (criada pela 0200_messaging_schema.sql, F2).
// O engine e generico: outro consumidor passa a sua tabela, desde que satisfaca o
// contrato de colunas de OMNI-F3.2.
const DefaultTable = "messaging.outbox"

// Store e a persistencia do outbox. O engine so fala com esta interface — a tabela
// concreta e do consumidor. Toda operacao por id TAMBEM recebe accountID: o filtro
// por conta se repete no repositorio mesmo o service ja tendo validado (defesa em
// profundidade, principio 2).
type Store interface {
	// Enqueue insere um job. created=false => a (account_id, idempotency_key) ja
	// existia e o insert foi dedupado (nao e erro).
	Enqueue(ctx context.Context, job NewJob) (id string, created bool, err error)

	// Claim reivindica ate limit jobs elegiveis e os marca processing/attempts+1.
	// Garante NO MAXIMO UM job em voo por (account_id, ordering_key) — e o coracao
	// do FIFO (risco 5 do canonico).
	Claim(ctx context.Context, workerID string, limit int) ([]Job, error)

	// MarkDone finaliza o job com sucesso.
	MarkDone(ctx context.Context, accountID, id string) error

	// Reschedule devolve o job a pending com run_after no futuro (backoff).
	Reschedule(ctx context.Context, accountID, id string, runAfter time.Time, lastError string) error

	// MarkDead manda o job para a dead-letter (esgotou tentativas ou unrecoverable).
	MarkDead(ctx context.Context, accountID, id, lastError string) error

	// StuckAccounts lista as contas que tem presas anteriores a threshold. E o passo
	// de DESCOBERTA do monitor: bounded, so devolve account_id.
	StuckAccounts(ctx context.Context, threshold time.Time, limit int) ([]string, error)

	// ReleaseStuck devolve a pending ate limit presas DA CONTA. O filtro de conta e
	// obrigatorio: o legado varre a tabela inteira sem tenant e isso NAO e portado.
	ReleaseStuck(ctx context.Context, accountID string, threshold time.Time, limit int) (int, error)
}

// tablePattern valida o identificador da tabela. O nome entra por interpolacao (nao
// da para parametrizar identificador em SQL), entao so aceitamos schema.tabela em
// minusculas — sem isso o nome viraria vetor de injecao.
var tablePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*$`)

// PostgresStore implementa Store sobre uma tabela que satisfaz o contrato de F3.2.
type PostgresStore struct {
	pool  *pgxpool.Pool
	table string

	claimSQL      string
	enqueueSQL    string
	markDoneSQL   string
	rescheduleSQL string
	markDeadSQL   string
	stuckAccounts string
	releaseStuck  string
}

// NewPostgresStore cria o Store. table vazia => DefaultTable. Nome fora de
// schema.tabela => ErrInvalidTable.
func NewPostgresStore(pool *pgxpool.Pool, table string) (*PostgresStore, error) {
	if table == "" {
		table = DefaultTable
	}
	if !tablePattern.MatchString(table) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidTable, table)
	}
	s := &PostgresStore{pool: pool, table: table}
	s.buildQueries()
	return s, nil
}

// buildQueries monta o SQL uma vez (o nome da tabela e fixo por Store).
func (s *PostgresStore) buildQueries() {
	t := s.table

	// Claim head-of-line — OMNI-F3.2. SKIP LOCKED puro da throughput e INVERTE a
	// ordem: dois jobs da mesma conversa em workers diferentes fariam o cliente ver a
	// resposta antes da pergunta. A garantia vem do `not exists`: so e elegivel o job
	// mais antigo NAO FINALIZADO da chave.
	//
	// O predicado e `in ('pending','processing')` e NAO so 'processing': job em
	// backoff volta para pending com run_after futuro; checando so processing o
	// sucessor passaria na frente — inversao silenciosa, com o provider saudavel.
	// Head-of-line blocking por chave e o comportamento CORRETO aqui.
	//
	// Nao usar DISTINCT ON: o Postgres recusa FOR UPDATE com DISTINCT, e o predicado
	// acima ja garante no maximo um candidato por chave.
	s.claimSQL = `
		with candidates as (
		    select j.id
		    from ` + t + ` j
		    where j.status = 'pending'
		      and j.run_after <= now()
		      and not exists (
		          select 1
		          from ` + t + ` b
		          where b.account_id = j.account_id
		            and b.ordering_key = j.ordering_key
		            and b.status in ('pending', 'processing')
		            and (b.created_at, b.id) < (j.created_at, j.id)
		      )
		    order by j.created_at, j.id
		    limit $1
		    for update skip locked
		)
		update ` + t + ` o
		set status = 'processing', attempts = o.attempts + 1,
		    locked_at = now(), locked_by = $2, updated_at = now()
		from candidates c
		where o.id = c.id
		returning o.id, o.account_id, o.ordering_key, o.kind, o.payload, o.attempts, o.max_attempts`

	// idempotency_key vai CRUA: o unique e (account_id, idempotency_key), por conta.
	// Prefixar com account_id seria redundante e esconderia qual mecanismo vale.
	s.enqueueSQL = `
		insert into ` + t + ` (account_id, ordering_key, idempotency_key, kind, payload, max_attempts, run_after)
		values ($1, $2, $3, $4, $5, $6, coalesce($7, now()))
		on conflict (account_id, idempotency_key) do nothing
		returning id`

	s.markDoneSQL = `
		update ` + t + `
		set status = 'done', locked_at = null, locked_by = '', last_error = '', updated_at = now()
		where id = $1 and account_id = $2`

	s.rescheduleSQL = `
		update ` + t + `
		set status = 'pending', run_after = $3, last_error = $4,
		    locked_at = null, locked_by = '', updated_at = now()
		where id = $1 and account_id = $2`

	s.markDeadSQL = `
		update ` + t + `
		set status = 'dead', last_error = $3, locked_at = null, locked_by = '', updated_at = now()
		where id = $1 and account_id = $2`

	s.stuckAccounts = `
		select distinct account_id
		from ` + t + `
		where status = 'processing' and locked_at is not null and locked_at < $1
		limit $2`

	s.releaseStuck = `
		update ` + t + `
		set status = 'pending', locked_at = null, locked_by = '', updated_at = now()
		where id in (
		    select id from ` + t + `
		    where account_id = $1 and status = 'processing'
		      and locked_at is not null and locked_at < $2
		    order by locked_at
		    limit $3
		    for update skip locked
		)`
}

// Enqueue insere o job. Dedupe por (account_id, idempotency_key) — POR CONTA: com um
// unique GLOBAL a conta A mandaria `abc`, a conta B mandaria `abc` e a colisao
// suprimiria o envio de B. Seria vazamento cross-tenant (principio 2).
func (s *PostgresStore) Enqueue(ctx context.Context, job NewJob) (string, bool, error) {
	if job.AccountID == "" || job.OrderingKey == "" || job.Kind == "" {
		return "", false, ErrInvalidJob
	}
	payload := job.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	maxAttempts := job.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}
	var runAfter *time.Time
	if !job.RunAfter.IsZero() {
		runAfter = &job.RunAfter
	}

	var id string
	err := s.pool.QueryRow(ctx, s.enqueueSQL,
		job.AccountID, job.OrderingKey, job.IdempotencyKey, job.Kind, payload, maxAttempts, runAfter,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// ON CONFLICT DO NOTHING nao devolve linha: ja existia, dedupou.
			return "", false, nil
		}
		return "", false, fmt.Errorf("jobs: enfileirar: %w", err)
	}
	return id, true, nil
}

// Claim reivindica jobs elegiveis. Ver o comentario do claimSQL: e o predicado
// head-of-line que sustenta o FIFO por ordering_key.
func (s *PostgresStore) Claim(ctx context.Context, workerID string, limit int) ([]Job, error) {
	if limit <= 0 {
		limit = 1
	}
	rows, err := s.pool.Query(ctx, s.claimSQL, limit, workerID)
	if err != nil {
		return nil, fmt.Errorf("jobs: claim: %w", err)
	}
	defer rows.Close()

	var claimed []Job
	for rows.Next() {
		var job Job
		if err := rows.Scan(&job.ID, &job.AccountID, &job.OrderingKey, &job.Kind,
			&job.Payload, &job.Attempts, &job.MaxAttempts); err != nil {
			return nil, fmt.Errorf("jobs: ler job reivindicado: %w", err)
		}
		claimed = append(claimed, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("jobs: claim: %w", err)
	}
	return claimed, nil
}

// MarkDone finaliza com sucesso. Filtra por account_id tambem aqui.
func (s *PostgresStore) MarkDone(ctx context.Context, accountID, id string) error {
	return s.exec(ctx, "concluir job", s.markDoneSQL, id, accountID)
}

// Reschedule devolve o job a pending com backoff. lastError ja vem mascarado.
func (s *PostgresStore) Reschedule(ctx context.Context, accountID, id string, runAfter time.Time, lastError string) error {
	return s.exec(ctx, "reagendar job", s.rescheduleSQL, id, accountID, runAfter, lastError)
}

// MarkDead manda para a dead-letter. lastError ja vem mascarado.
func (s *PostgresStore) MarkDead(ctx context.Context, accountID, id, lastError string) error {
	return s.exec(ctx, "dead-letter do job", s.markDeadSQL, id, accountID, lastError)
}

// exec roda um update e converte o erro sem interpolar o payload.
func (s *PostgresStore) exec(ctx context.Context, op, query string, args ...any) error {
	if _, err := s.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("jobs: %s: %w", op, err)
	}
	return nil
}

// StuckAccounts lista contas com presas. Passo de descoberta do monitor — bounded por
// limit e devolve so o account_id (nunca payload).
func (s *PostgresStore) StuckAccounts(ctx context.Context, threshold time.Time, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 1
	}
	rows, err := s.pool.Query(ctx, s.stuckAccounts, threshold, limit)
	if err != nil {
		return nil, fmt.Errorf("jobs: listar contas com presas: %w", err)
	}
	defer rows.Close()

	var accounts []string
	for rows.Next() {
		var accountID string
		if err := rows.Scan(&accountID); err != nil {
			return nil, fmt.Errorf("jobs: ler conta com presas: %w", err)
		}
		accounts = append(accounts, accountID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("jobs: listar contas com presas: %w", err)
	}
	return accounts, nil
}

// ReleaseStuck devolve presas DA CONTA a pending. O update SEMPRE carrega
// account_id — nunca um update cego na tabela toda (comportamento do legado, §8).
func (s *PostgresStore) ReleaseStuck(ctx context.Context, accountID string, threshold time.Time, limit int) (int, error) {
	if accountID == "" {
		return 0, ErrInvalidJob
	}
	if limit <= 0 {
		limit = 1
	}
	tag, err := s.pool.Exec(ctx, s.releaseStuck, accountID, threshold, limit)
	if err != nil {
		return 0, fmt.Errorf("jobs: liberar presas: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
