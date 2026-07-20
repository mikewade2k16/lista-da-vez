package jobs

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Defaults do worker (canonico §8 / OMNI-F3.2).
const (
	defaultBatch         = 10
	defaultInterval      = 1 * time.Second
	defaultJobTimeout    = 60 * time.Second
	defaultStuckAfter    = 10 * time.Minute // presa = processing com locked_at > 10min
	defaultStuckInterval = 5 * time.Minute  // ciclo do monitor
	defaultStuckLimit    = 20               // ate 20 presas liberadas por ciclo
	defaultStuckAccounts = 50               // teto de contas inspecionadas por ciclo
)

// Config parametriza o Worker. Campo zero => default.
type Config struct {
	// WorkerID identifica a instancia em locked_by (ex.: hostname+pid). Vazio => "worker".
	WorkerID string

	// Batch e o teto de jobs por claim.
	Batch int

	// Interval e o intervalo do ticker de claim.
	Interval time.Duration

	// JobTimeout limita a execucao de UM job. Handler travado nao segura o worker.
	JobTimeout time.Duration

	// StuckAfter: processing com locked_at mais velho que isso vira presa.
	StuckAfter time.Duration

	// StuckInterval e o intervalo do monitor de presas.
	StuckInterval time.Duration

	// StuckLimit e o teto de presas liberadas POR CONTA por ciclo.
	StuckLimit int

	// StuckAccounts e o teto de contas inspecionadas por ciclo do monitor.
	StuckAccounts int

	// Logger recebe log estruturado (campos explicitos). nil => sem log.
	Logger *slog.Logger
}

// withDefaults preenche os zeros.
func (c Config) withDefaults() Config {
	if c.WorkerID == "" {
		c.WorkerID = "worker"
	}
	if c.Batch <= 0 {
		c.Batch = defaultBatch
	}
	if c.Interval <= 0 {
		c.Interval = defaultInterval
	}
	if c.JobTimeout <= 0 {
		c.JobTimeout = defaultJobTimeout
	}
	if c.StuckAfter <= 0 {
		c.StuckAfter = defaultStuckAfter
	}
	if c.StuckInterval <= 0 {
		c.StuckInterval = defaultStuckInterval
	}
	if c.StuckLimit <= 0 {
		c.StuckLimit = defaultStuckLimit
	}
	if c.StuckAccounts <= 0 {
		c.StuckAccounts = defaultStuckAccounts
	}
	return c
}

// Worker consome o outbox: claim -> handler -> done/backoff/dead-letter, mais o
// monitor de presas. Uma goroutine de claim + uma de monitor por instancia (padrao da
// casa: realtime/service.go, httpapi/rate_limit.go). Parada por context.CancelFunc em
// Close(), como Module.Close() (platform/modules/module.go:70).
//
// Concorrencia entre INSTANCIAS e segura por construcao: o claim head-of-line deixa no
// maximo um job em voo por (account_id, ordering_key), entao dois workers nunca
// processam a mesma chave ao mesmo tempo. Dentro de uma instancia o lote e processado
// em serie — o paralelismo vem de rodar N instancias.
type Worker struct {
	store Store
	cfg   Config

	mu       sync.RWMutex
	handlers map[string]Handler

	cancel context.CancelFunc
	wg     sync.WaitGroup
	closed bool
}

// New cria o Worker. Registre os handlers antes de Start.
func New(store Store, cfg Config) *Worker {
	return &Worker{store: store, cfg: cfg.withDefaults(), handlers: map[string]Handler{}}
}

// Register liga um kind a um handler. Kind repetido sobrescreve.
func (w *Worker) Register(kind string, h Handler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[kind] = h
}

// handlerFor busca o handler do kind.
func (w *Worker) handlerFor(kind string) (Handler, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	h, ok := w.handlers[kind]
	return h, ok
}

// Start sobe as goroutines de claim e do monitor de presas. Nao bloqueia.
func (w *Worker) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	w.mu.Lock()
	w.cancel = cancel
	w.mu.Unlock()

	w.wg.Add(2)
	go w.loop(ctx, w.cfg.Interval, func(c context.Context) {
		if _, err := w.RunOnce(c); err != nil {
			w.log("error", "ciclo de claim falhou", "error", err.Error())
		}
	})
	go w.loop(ctx, w.cfg.StuckInterval, func(c context.Context) {
		if err := w.ReleaseStuck(c); err != nil {
			w.log("error", "monitor de presas falhou", "error", err.Error())
		}
	})
}

// loop roda fn a cada interval ate o contexto morrer.
func (w *Worker) loop(ctx context.Context, interval time.Duration, fn func(context.Context)) {
	defer w.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fn(ctx)
		}
	}
}

// Close para as goroutines e espera o job em voo terminar. Idempotente.
func (w *Worker) Close() error {
	w.mu.Lock()
	if w.closed || w.cancel == nil {
		w.closed = true
		w.mu.Unlock()
		return nil
	}
	cancel := w.cancel
	w.closed = true
	w.mu.Unlock()

	cancel()
	w.wg.Wait()
	return nil
}

// RunOnce faz UM ciclo de claim e processa o lote em serie. Devolve quantos jobs
// rodaram. Exportado para teste e para quem quiser drenar a fila sem ticker.
func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	claimed, err := w.store.Claim(ctx, w.cfg.WorkerID, w.cfg.Batch)
	if err != nil {
		return 0, err
	}
	for _, job := range claimed {
		if ctx.Err() != nil {
			// Shutdown no meio do lote: o que sobrou fica em processing e o monitor
			// de presas devolve para pending. Nao marcamos nada com contexto morto.
			return len(claimed), nil
		}
		w.process(ctx, job)
	}
	return len(claimed), nil
}

// process executa um job e resolve o desfecho: done, backoff ou dead-letter.
func (w *Worker) process(ctx context.Context, job Job) {
	err := w.execute(ctx, job)
	if err == nil {
		if markErr := w.store.MarkDone(ctx, job.AccountID, job.ID); markErr != nil {
			w.log("error", "marcar job como done falhou",
				"account_id", job.AccountID, "job_id", job.ID, "error", markErr.Error())
		}
		return
	}
	w.settleFailure(ctx, job, err)
}

// execute roda o handler do kind com timeout proprio. Kind sem handler => ErrNoHandler
// (classificado como unrecoverable: so codigo novo resolve).
func (w *Worker) execute(ctx context.Context, job Job) error {
	handler, ok := w.handlerFor(job.Kind)
	if !ok {
		return ErrNoHandler
	}
	jobCtx, cancel := context.WithTimeout(ctx, w.cfg.JobTimeout)
	defer cancel()
	return handler.Handle(jobCtx, job)
}

// settleFailure aplica a classificacao: unrecoverable ou tentativas esgotadas =>
// dead-letter; senao backoff exponencial com jitter.
//
// O teto de tentativas vem da CLASSE, nao so da coluna max_attempts: um 429 merece 5
// tentativas e um 401 nao merece nenhuma repeticao. Respeitamos o menor dos dois
// quando a linha pediu um teto menor de propria vontade.
func (w *Worker) settleFailure(ctx context.Context, job Job, jobErr error) {
	class := Classify(jobErr)
	limit := class.MaxAttempts
	if job.MaxAttempts > 0 && job.MaxAttempts < limit {
		limit = job.MaxAttempts
	}
	lastError := MaskError(jobErr)

	if class.Unrecoverable || job.Attempts >= limit {
		if err := w.store.MarkDead(ctx, job.AccountID, job.ID, lastError); err != nil {
			w.log("error", "dead-letter do job falhou",
				"account_id", job.AccountID, "job_id", job.ID, "error", err.Error())
			return
		}
		w.log("warn", "job para a dead-letter",
			"account_id", job.AccountID, "job_id", job.ID, "kind", job.Kind,
			"class", class.Name, "attempts", job.Attempts, "error", lastError)
		return
	}

	runAfter := time.Now().Add(Backoff(job.Attempts))
	if err := w.store.Reschedule(ctx, job.AccountID, job.ID, runAfter, lastError); err != nil {
		w.log("error", "reagendar job falhou",
			"account_id", job.AccountID, "job_id", job.ID, "error", err.Error())
	}
}

// ReleaseStuck e um ciclo do monitor de presas: job em processing com locked_at mais
// velho que StuckAfter volta para pending (o worker que o segurava morreu).
//
// SEMPRE COM FILTRO DE CONTA: descobre as contas afetadas e libera conta a conta, ate
// StuckLimit por conta. O legado varre a tabela inteira sem tenant — esse
// comportamento NAO e portado (canonico §8).
func (w *Worker) ReleaseStuck(ctx context.Context) error {
	threshold := time.Now().Add(-w.cfg.StuckAfter)
	accounts, err := w.store.StuckAccounts(ctx, threshold, w.cfg.StuckAccounts)
	if err != nil {
		return err
	}
	for _, accountID := range accounts {
		released, err := w.store.ReleaseStuck(ctx, accountID, threshold, w.cfg.StuckLimit)
		if err != nil {
			w.log("error", "liberar presas da conta falhou", "account_id", accountID, "error", err.Error())
			continue
		}
		if released > 0 {
			w.log("warn", "presas devolvidas para pending", "account_id", accountID, "released", released)
		}
	}
	return nil
}

// log emite log estruturado com campos EXPLICITOS — nunca a struct do job
// interpolada, nunca payload, nunca segredo (canonico §10).
func (w *Worker) log(level, msg string, attrs ...any) {
	if w.cfg.Logger == nil {
		return
	}
	attrs = append([]any{"module", "jobs", "worker_id", w.cfg.WorkerID}, attrs...)
	switch level {
	case "error":
		w.cfg.Logger.Error(msg, attrs...)
	case "warn":
		w.cfg.Logger.Warn(msg, attrs...)
	default:
		w.cfg.Logger.Info(msg, attrs...)
	}
}
