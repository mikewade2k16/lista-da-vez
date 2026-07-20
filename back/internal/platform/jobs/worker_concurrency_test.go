package jobs_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

// Teste de integracao do RISCO 5 do canonico: FIFO por ordering_key com N workers.
//
// Por que este teste existe: `FOR UPDATE SKIP LOCKED` da throughput mas, sozinho,
// INVERTE a ordem — duas mensagens da mesma conversa em workers diferentes fazem o
// cliente ver a resposta antes da pergunta. Sob carga, em producao, e um bug caro e
// invisivel em teste sequencial. A spec OMNI-F3 elege este teste como o coracao da fase.
//
// Para rodar (padrao da casa: platform/database/app_role_ensure_test.go:31):
//
//	TEST_DATABASE_URL="postgres://omni:omni_dev@localhost:5432/omni?sslmode=disable" \
//	  go test ./internal/platform/jobs/ -run TestWorkerFIFO -race -count=5 -v
//
// -race e -count=5 sao parte do criterio: passar uma vez nao prova ausencia de corrida.
//
// A tabela e EFEMERA, criada pelo proprio teste com o contrato de F3.2 — a F3 nao
// depende da F2 (a messaging.outbox real e da migration 0200).

// fifoTestTable e a tabela efemera. Schema proprio para nao colidir com messaging.*.
const fifoTestTable = "jobs_fifo_test.outbox"

// requireTestPool conecta no banco de teste ou pula (padrao da casa).
func requireTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definido — pulando teste de integração do outbox")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("conectar ao banco de teste: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// setupEphemeralOutbox cria a tabela efemera com o contrato de OMNI-F3.2 e uma conta
// de teste em core.accounts (a FK de account_id exige). Derruba tudo no cleanup.
func setupEphemeralOutbox(t *testing.T, pool *pgxpool.Pool) (accountID string) {
	t.Helper()
	ctx := context.Background()

	ddl := []string{
		`create schema if not exists jobs_fifo_test`,
		`drop table if exists ` + fifoTestTable,
		// Contrato coluna a coluna de OMNI-F3.2 (espelha a 0200_messaging_schema.sql).
		`create table ` + fifoTestTable + ` (
			id              uuid primary key default gen_random_uuid(),
			account_id      uuid not null references core.accounts(id) on delete cascade,
			ordering_key    text not null,
			idempotency_key text not null,
			kind            text not null,
			payload         jsonb not null default '{}'::jsonb,
			status          text not null default 'pending'
				check (status in ('pending', 'processing', 'done', 'failed', 'dead')),
			attempts        int not null default 0,
			max_attempts    int not null default 3,
			run_after       timestamptz not null default now(),
			locked_at       timestamptz,
			locked_by       text not null default '',
			last_error      text not null default '',
			created_at      timestamptz not null default now(),
			updated_at      timestamptz not null default now()
		)`,
		`create unique index on ` + fifoTestTable + ` (account_id, idempotency_key)`,
		`create index on ` + fifoTestTable + ` (account_id, ordering_key, created_at, id)
			where status in ('pending', 'processing')`,
		`create index on ` + fifoTestTable + ` (status, run_after)`,
	}
	for _, stmt := range ddl {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("preparar tabela efêmera: %v", err)
		}
	}

	slug := fmt.Sprintf("jobs-fifo-test-%d", time.Now().UnixNano())
	err := pool.QueryRow(ctx,
		`insert into core.accounts (name, slug, is_active) values ($1, $1, false) returning id`,
		slug).Scan(&accountID)
	if err != nil {
		t.Fatalf("criar conta de teste: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		// A tabela cai antes da conta (FK on delete cascade cobriria, mas explicito
		// deixa o banco de teste limpo mesmo se o schema sobreviver).
		if _, err := pool.Exec(cleanupCtx, `drop schema if exists jobs_fifo_test cascade`); err != nil {
			t.Logf("limpar schema efêmero: %v", err)
		}
		if _, err := pool.Exec(cleanupCtx, `delete from core.accounts where id = $1`, accountID); err != nil {
			t.Logf("limpar conta de teste: %v", err)
		}
	})
	return accountID
}

// completionRecorder registra a ordem de CONCLUSAO por ordering_key e quantas vezes
// cada job concluiu com sucesso.
type completionRecorder struct {
	mu        sync.Mutex
	completed map[string][]int // ordering_key -> sequencias concluidas, em ordem
	successes map[string]int   // job payload seq id -> nº de conclusoes com sucesso
}

func newRecorder() *completionRecorder {
	return &completionRecorder{completed: map[string][]int{}, successes: map[string]int{}}
}

func (r *completionRecorder) record(key string, seq int, jobID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.completed[key] = append(r.completed[key], seq)
	r.successes[jobID]++
}

// jobPayload e o corpo do job de teste: a posicao na fila da chave.
type jobPayload struct {
	Seq int `json:"seq"`
}

// decodeJSON le o payload do job de teste.
func decodeJSON(raw []byte, target any) error {
	return json.Unmarshal(raw, target)
}

// TestWorkerFIFO e o teste dedicado do risco 5.
//
// Cenario da spec: 8 workers, 1 conta, 20 ordering_key × 25 jobs, handler com sleep
// aleatorio 0–5 ms e falha transitoria injetada em ~10% (para exercitar o retry).
//
// Assertivas: (a) por ordering_key, a ordem de conclusao e EXATAMENTE a de insercao;
// (b) nenhum job roda duas vezes com sucesso; (c) todos terminam em done ou dead.
func TestWorkerFIFO(t *testing.T) {
	pool := requireTestPool(t)
	accountID := setupEphemeralOutbox(t, pool)
	ctx := context.Background()

	store, err := jobs.NewPostgresStore(pool, fifoTestTable)
	if err != nil {
		t.Fatalf("NewPostgresStore: %v", err)
	}

	const (
		workers     = 8
		keys        = 20
		perKey      = 25
		failPercent = 10
	)

	// Enfileira em ordem: para cada chave, seq 0..perKey-1. created_at é o criterio
	// FIFO, entao a insercao e sequencial de proposito.
	for k := 0; k < keys; k++ {
		orderingKey := fmt.Sprintf("conversation-%02d", k)
		for seq := 0; seq < perKey; seq++ {
			payload := []byte(fmt.Sprintf(`{"seq":%d}`, seq))
			_, created, err := store.Enqueue(ctx, jobs.NewJob{
				AccountID:      accountID,
				OrderingKey:    orderingKey,
				IdempotencyKey: fmt.Sprintf("%s-%03d", orderingKey, seq),
				Kind:           "fifo_test",
				Payload:        payload,
				MaxAttempts:    5, // a classe transitoria pede 5; a linha nao pode limitar abaixo
			})
			if err != nil {
				t.Fatalf("Enqueue(%s,%d): %v", orderingKey, seq, err)
			}
			if !created {
				t.Fatalf("Enqueue(%s,%d) dedupou — idempotency_key repetida", orderingKey, seq)
			}
		}
	}

	recorder := newRecorder()

	// failures conta quantas vezes cada job ja falhou, para a injecao de falha ser
	// determinística por job (falha uma vez, depois passa) — senao um job poderia
	// falhar 5x e morrer, e o teste de ordem ficaria nao-deterministico.
	var failMu sync.Mutex
	failed := map[string]bool{}

	handler := jobs.HandlerFunc(func(ctx context.Context, job jobs.Job) error {
		var payload jobPayload
		if err := decodeJSON(job.Payload, &payload); err != nil {
			return err
		}
		// Sleep aleatorio 0–5 ms: sem isso os workers nao se sobrepoem e o teste nao
		// exerce a corrida.
		time.Sleep(time.Duration(rand.IntN(5)) * time.Millisecond) //nolint:gosec // jitter de teste

		// Falha transitoria em ~10% dos jobs, UMA vez por job: exercita o backoff sem
		// matar o job (a classe transitoria da 5 tentativas).
		failMu.Lock()
		shouldFail := !failed[job.ID] && rand.IntN(100) < failPercent //nolint:gosec // teste
		if shouldFail {
			failed[job.ID] = true
		}
		failMu.Unlock()
		if shouldFail {
			return &jobs.StatusError{StatusCode: 0, Err: context.DeadlineExceeded}
		}

		recorder.record(job.OrderingKey, payload.Seq, job.ID)
		return nil
	})

	// Backoff real e 5s; o teste nao pode esperar isso. Os workers rodam RunOnce em
	// laco apertado e o job em backoff volta quando run_after vence — por isso o
	// deadline generoso abaixo.
	var wg sync.WaitGroup
	runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	for w := 0; w < workers; w++ {
		worker := jobs.New(store, jobs.Config{
			WorkerID:   fmt.Sprintf("fifo-worker-%d", w),
			Batch:      5,
			JobTimeout: 10 * time.Second,
		})
		worker.Register("fifo_test", handler)

		wg.Add(1)
		go func() {
			defer wg.Done()
			for runCtx.Err() == nil {
				n, err := worker.RunOnce(runCtx)
				if err != nil {
					if runCtx.Err() != nil {
						return
					}
					t.Errorf("RunOnce: %v", err)
					return
				}
				if n == 0 {
					// Fila vazia (ou tudo em backoff): nao queima CPU.
					time.Sleep(20 * time.Millisecond)
					if done, err := allSettled(runCtx, pool, accountID); err == nil && done {
						return
					}
				}
			}
		}()
	}
	wg.Wait()

	// (c) todos terminam em done ou dead.
	counts, err := statusCounts(ctx, pool, accountID)
	if err != nil {
		t.Fatalf("contar status: %v", err)
	}
	total := keys * perKey
	if counts["done"] != total {
		t.Fatalf("esperado %d jobs done, veio %v", total, counts)
	}

	// (a) por ordering_key, a ordem de conclusao e EXATAMENTE a de insercao.
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if len(recorder.completed) != keys {
		t.Fatalf("esperado %d ordering_key concluídas, veio %d", keys, len(recorder.completed))
	}
	for key, seqs := range recorder.completed {
		if len(seqs) != perKey {
			t.Fatalf("chave %s concluiu %d jobs, esperado %d", key, len(seqs), perKey)
		}
		for i, seq := range seqs {
			if seq != i {
				t.Fatalf("INVERSÃO DE ORDEM na chave %s: posição %d concluiu o seq %d (ordem: %v)",
					key, i, seq, seqs)
			}
		}
	}

	// (b) nenhum job roda duas vezes com sucesso.
	for jobID, n := range recorder.successes {
		if n != 1 {
			t.Fatalf("job %s concluiu %d vezes com sucesso (esperado 1)", jobID, n)
		}
	}
}

// TestWorkerFIFOBackoffNotOvertaken e o sub-teste dedicado ao backoff (assertiva (d)).
//
// E a armadilha mais sutil da fase: um job que falha volta para `pending` com
// run_after no futuro. Se o predicado head-of-line checasse so `status = 'processing'`,
// o SUCESSOR passaria na frente do antecessor que esta esperando retry — inversao
// silenciosa, com o provider saudavel. Head-of-line blocking por chave e o
// comportamento CORRETO aqui.
func TestWorkerFIFOBackoffNotOvertaken(t *testing.T) {
	pool := requireTestPool(t)
	accountID := setupEphemeralOutbox(t, pool)
	ctx := context.Background()

	store, err := jobs.NewPostgresStore(pool, fifoTestTable)
	if err != nil {
		t.Fatalf("NewPostgresStore: %v", err)
	}

	const orderingKey = "conversation-backoff"
	for seq := 0; seq < 3; seq++ {
		if _, created, err := store.Enqueue(ctx, jobs.NewJob{
			AccountID:      accountID,
			OrderingKey:    orderingKey,
			IdempotencyKey: fmt.Sprintf("backoff-%d", seq),
			Kind:           "backoff_test",
			Payload:        []byte(fmt.Sprintf(`{"seq":%d}`, seq)),
			MaxAttempts:    5,
		}); err != nil || !created {
			t.Fatalf("Enqueue(%d): created=%v err=%v", seq, created, err)
		}
	}

	var mu sync.Mutex
	var order []int
	attemptsBySeq := map[int]int{}

	worker := jobs.New(store, jobs.Config{WorkerID: "backoff-worker", Batch: 5})
	worker.Register("backoff_test", jobs.HandlerFunc(func(ctx context.Context, job jobs.Job) error {
		var payload jobPayload
		if err := decodeJSON(job.Payload, &payload); err != nil {
			return err
		}
		mu.Lock()
		attemptsBySeq[payload.Seq]++
		attempt := attemptsBySeq[payload.Seq]
		mu.Unlock()

		// O seq 0 falha na 1a tentativa e entra em backoff. Enquanto ele espera,
		// NENHUM sucessor pode concluir.
		if payload.Seq == 0 && attempt == 1 {
			return &jobs.StatusError{StatusCode: 503}
		}
		mu.Lock()
		order = append(order, payload.Seq)
		mu.Unlock()
		return nil
	}))

	// Primeiro ciclo: reivindica e faz o seq 0 falhar -> vai para pending com
	// run_after ~5s no futuro.
	if _, err := worker.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	// Janela critica: com o seq 0 em backoff, varios ciclos NAO podem concluir nada.
	// Se o predicado estivesse errado, os seq 1 e 2 apareceriam aqui.
	for i := 0; i < 20; i++ {
		if _, err := worker.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	overtakers := append([]int(nil), order...)
	mu.Unlock()
	if len(overtakers) != 0 {
		t.Fatalf("ULTRAPASSAGEM: com o seq 0 em backoff, concluíram %v — o predicado head-of-line "+
			"está checando só 'processing' e ignorando os 'pending' em backoff", overtakers)
	}

	// Passado o backoff (5s ±20% => ate 6s), tudo conclui em ordem.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := worker.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		mu.Lock()
		done := len(order) == 3
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 {
		t.Fatalf("esperado 3 jobs concluídos, veio %v", order)
	}
	for i, seq := range order {
		if seq != i {
			t.Fatalf("ordem final errada: %v (o job em backoff foi ultrapassado)", order)
		}
	}
}

// TestReleaseStuckFiltersByAccount prova que o monitor de presas NAO varre a tabela
// inteira: a presa da conta A nao pode ser liberada no ciclo da conta B. O legado faz
// exatamente isso (varredura sem tenant) e o canonico §8 proibe portar.
func TestReleaseStuckFiltersByAccount(t *testing.T) {
	pool := requireTestPool(t)
	accountA := setupEphemeralOutbox(t, pool)
	ctx := context.Background()

	// Segunda conta, na MESMA tabela efêmera.
	var accountB string
	slug := fmt.Sprintf("jobs-fifo-test-b-%d", time.Now().UnixNano())
	if err := pool.QueryRow(ctx,
		`insert into core.accounts (name, slug, is_active) values ($1, $1, false) returning id`,
		slug).Scan(&accountB); err != nil {
		t.Fatalf("criar conta B: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `delete from core.accounts where id = $1`, accountB); err != nil {
			t.Logf("limpar conta B: %v", err)
		}
	})

	store, err := jobs.NewPostgresStore(pool, fifoTestTable)
	if err != nil {
		t.Fatalf("NewPostgresStore: %v", err)
	}

	// Uma presa em cada conta: processing, locked_at 30 min atras.
	for _, accountID := range []string{accountA, accountB} {
		// account_id ($1) e idempotency_key ($2) vao separados: reusar $1 nos dois faz
		// o Postgres deduzir uuid e text para o mesmo parametro (SQLSTATE 42P08).
		if _, err := pool.Exec(ctx,
			`insert into `+fifoTestTable+` (account_id, ordering_key, idempotency_key, kind, status, locked_at, locked_by)
			 values ($1, 'k', $2, 'stuck_test', 'processing', now() - interval '30 minutes', 'dead-worker')`,
			accountID, "stuck-"+accountID); err != nil {
			t.Fatalf("criar presa: %v", err)
		}
	}

	threshold := time.Now().Add(-10 * time.Minute)

	// A descoberta enxerga as duas contas...
	accounts, err := store.StuckAccounts(ctx, threshold, 50)
	if err != nil {
		t.Fatalf("StuckAccounts: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("esperado 2 contas com presas, veio %d", len(accounts))
	}

	// ...mas a liberacao e por conta: soltar a conta A NAO pode soltar a da conta B.
	released, err := store.ReleaseStuck(ctx, accountA, threshold, 20)
	if err != nil {
		t.Fatalf("ReleaseStuck: %v", err)
	}
	if released != 1 {
		t.Fatalf("esperado 1 presa liberada na conta A, veio %d", released)
	}

	countsB, err := statusCounts(ctx, pool, accountB)
	if err != nil {
		t.Fatalf("contar status da conta B: %v", err)
	}
	if countsB["processing"] != 1 {
		t.Fatalf("VAZAMENTO CROSS-TENANT: o ciclo da conta A mexeu na presa da conta B (%v)", countsB)
	}
	countsA, err := statusCounts(ctx, pool, accountA)
	if err != nil {
		t.Fatalf("contar status da conta A: %v", err)
	}
	if countsA["pending"] != 1 {
		t.Fatalf("a presa da conta A não voltou para pending: %v", countsA)
	}
}

// TestEnqueueIdempotencyIsPerAccount prova a decisao do dono (2026-07-17): o unique e
// (account_id, idempotency_key). Com UNIQUE global, a mesma chave vinda da conta B
// deduparia contra o job da conta A — suprimindo o envio alheio (vazamento
// cross-tenant, principio 2).
func TestEnqueueIdempotencyIsPerAccount(t *testing.T) {
	pool := requireTestPool(t)
	accountA := setupEphemeralOutbox(t, pool)
	ctx := context.Background()

	var accountB string
	slug := fmt.Sprintf("jobs-idem-test-b-%d", time.Now().UnixNano())
	if err := pool.QueryRow(ctx,
		`insert into core.accounts (name, slug, is_active) values ($1, $1, false) returning id`,
		slug).Scan(&accountB); err != nil {
		t.Fatalf("criar conta B: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `delete from core.accounts where id = $1`, accountB); err != nil {
			t.Logf("limpar conta B: %v", err)
		}
	})

	store, err := jobs.NewPostgresStore(pool, fifoTestTable)
	if err != nil {
		t.Fatalf("NewPostgresStore: %v", err)
	}

	newJob := func(accountID string) jobs.NewJob {
		return jobs.NewJob{
			AccountID:      accountID,
			OrderingKey:    "k",
			IdempotencyKey: "abc", // a MESMA chave nas duas contas
			Kind:           "idem_test",
		}
	}

	if _, created, err := store.Enqueue(ctx, newJob(accountA)); err != nil || !created {
		t.Fatalf("conta A: created=%v err=%v", created, err)
	}
	// A conta B tem de conseguir enfileirar a MESMA chave.
	if _, created, err := store.Enqueue(ctx, newJob(accountB)); err != nil || !created {
		t.Fatalf("VAZAMENTO CROSS-TENANT: a chave 'abc' da conta B foi suprimida pela da conta A "+
			"(unique global?): created=%v err=%v", created, err)
	}
	// Repetir na MESMA conta dedupa (sem erro).
	if _, created, err := store.Enqueue(ctx, newJob(accountA)); err != nil || created {
		t.Fatalf("repetir a chave na mesma conta devia dedupar: created=%v err=%v", created, err)
	}
}

// allSettled diz se nao ha mais nada pending/processing para a conta.
func allSettled(ctx context.Context, pool *pgxpool.Pool, accountID string) (bool, error) {
	var n int
	err := pool.QueryRow(ctx,
		`select count(*) from `+fifoTestTable+`
		 where account_id = $1 and status in ('pending', 'processing')`, accountID).Scan(&n)
	return n == 0, err
}

// statusCounts devolve a contagem por status DA CONTA.
func statusCounts(ctx context.Context, pool *pgxpool.Pool, accountID string) (map[string]int, error) {
	rows, err := pool.Query(ctx,
		`select status, count(*) from `+fifoTestTable+` where account_id = $1 group by status`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		counts[status] = n
	}
	return counts, rows.Err()
}
