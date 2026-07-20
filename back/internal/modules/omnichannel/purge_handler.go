package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

// ============================================================================
// F13 — Job de PURGE sobre o platform/jobs (F3). SEM scheduler novo (C4)
// ============================================================================
//
// O TRABALHO roda no worker do platform/jobs (o mesmo que despacha o envio, F6). Este arquivo
// tem tres pecas:
//   1. PurgeHandler   — o jobs.Handler que executa a poda de UMA conta (por classe, batelado).
//   2. Enfileiradores — EnqueueDailyPurge / EnqueueMediaOrphanScan: so ENFILEIRAM (a lista de
//                       contas vem do catalogo, nunca hardcoded).
//   3. StartRetentionScheduler — o ticker que chama os enfileiradores (padrao app.go:116/133 e
//                       cardapio; primeiro disparo ~5 min apos o boot, fora do caminho critico).
//
// Wiring (registrar o handler + subir o scheduler) mora no module.go/app.go — ver AGENT.md
// "Wiring pendente". Este arquivo nao edita nenhum dos dois.

// Kinds despachados pelo worker (jobs.Worker.Register).
const (
	// PurgeAccountJobKind poda UMA conta (todas as classes com prazo).
	PurgeAccountJobKind = "omnichannel.purge.account"
	// PurgeMediaOrphanJobKind varre orfaos de midia de UMA conta (semanal).
	PurgeMediaOrphanJobKind = "omnichannel.purge.media_orphan"
)

const (
	// purgeSoftDeadline e o teto de tempo de UM job. Fica ABAIXO do JobTimeout do worker (60s)
	// para sobrar margem para o re-enfileiramento da continuacao. Atingido no meio da classe
	// conversation => re-enfileira purge:{acct}:{date}:cont:{seq+1} e sai (C4).
	purgeSoftDeadline = 45 * time.Second
	// purgeMaxAttempts: erro de banco e transitorio; da folga antes do dead-letter.
	purgeMaxAttempts = 10
)

// purgeEnqueuer e o produtor de jobs (satisfeito por jobs.PostgresStore). O handler o usa para
// enfileirar a continuacao quando bate o teto de tempo.
type purgeEnqueuer interface {
	Enqueue(ctx context.Context, job jobs.NewJob) (string, bool, error)
}

// purgePayload e o corpo do job — SEM PII (F3.2). accountId aqui e do PRODUTOR (enfileirador
// server-side), nunca de uma requisicao. seq = numero da continuacao (0 = primeiro).
type purgePayload struct {
	AccountID string `json:"accountId"`
	Date      string `json:"date"`
	DryRun    bool   `json:"dryRun"`
	Seq       int    `json:"seq,omitempty"`
}

// PurgeHandler executa a poda. Implementa jobs.Handler para os dois kinds acima.
type PurgeHandler struct {
	resolver *RetentionResolver
	store    *RetentionStore
	media    *mediaPurger
	enqueuer purgeEnqueuer
	logger   *slog.Logger
}

// NewPurgeHandler monta o handler. mediaDir = MediaDirFromEnv() (a raiz da F6).
func NewPurgeHandler(resolver *RetentionResolver, store *RetentionStore, enqueuer purgeEnqueuer, mediaDir string, logger *slog.Logger) *PurgeHandler {
	return &PurgeHandler{
		resolver: resolver,
		store:    store,
		media:    newMediaPurger(mediaDir),
		enqueuer: enqueuer,
		logger:   logger,
	}
}

// Handle despacha por kind. account_id do JOB (produtor) tem de bater com o do payload —
// defesa contra um payload forjado apontar outra conta (embora o produtor seja server-side).
func (h *PurgeHandler) Handle(ctx context.Context, job jobs.Job) error {
	p, err := parsePurgePayload(job.Payload)
	if err != nil {
		return err // payload ilegivel: unrecoverable (no_status) — nao adianta repetir
	}
	if p.AccountID == "" || p.AccountID != job.AccountID {
		return errPurgeAccountMismatch
	}
	switch job.Kind {
	case PurgeMediaOrphanJobKind:
		return h.runMediaOrphan(ctx, p)
	default:
		return h.runAccountPurge(ctx, job, p)
	}
}

// errPurgeAccountMismatch: o accountId do payload nao bate com o do job (isolamento).
var errPurgeAccountMismatch = errors.New("omnichannel: purge account mismatch")

// runAccountPurge roda as quatro classes na ordem ephemeral -> ai_io -> conversation -> audit
// (C4). Uma linha em purge_runs por classe. Erro de banco em qualquer classe => devolve o erro
// e o engine reagenda (transitorio); a poda e idempotente (opera no que AINDA passou do prazo).
// Bateu o teto de tempo no meio de uma classe => re-enfileira a continuacao e sai (C4).
func (h *PurgeHandler) runAccountPurge(ctx context.Context, _ jobs.Job, p purgePayload) error {
	policy, err := h.resolver.Resolve(ctx, p.AccountID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	deadline := time.Now().Add(purgeSoftDeadline)

	classes := []struct {
		cutoff time.Time
		run    func(context.Context, purgePayload, time.Time, time.Time) (bool, error)
	}{
		{cutoffFor(now, policy.Ephemeral.Days), h.runEphemeral},
		{cutoffFor(now, policy.AIIO.Days), h.runAIIO},
		{cutoffFor(now, policy.Conversation.Days), h.runConversationClass},
		{cutoffFor(now, policy.Audit.Days), h.runAudit},
	}
	for _, c := range classes {
		complete, err := c.run(ctx, p, c.cutoff, deadline)
		if err != nil {
			return err
		}
		if !complete {
			return h.reenqueue(ctx, p) // bateu o teto de tempo: continua no proximo job
		}
	}
	return nil
}

// runEphemeral apaga webhook_events + outbox terminal (classe ephemeral). Dry-run => so conta.
func (h *PurgeHandler) runEphemeral(ctx context.Context, p purgePayload, cutoff, deadline time.Time) (bool, error) {
	started := time.Now()
	rec := purgeRunRecord{Class: classEphemeral, CutoffAt: cutoff}
	if p.DryRun {
		n, err := h.store.CountEphemeral(ctx, p.AccountID, cutoff)
		rec.RowsDeleted = n
		h.finishRun(ctx, p, rec, started, err)
		return true, err
	}
	n, complete, err := drainAll(ctx, deadline, []batchOp{
		func() (int64, error) { return h.store.DeleteWebhookBatch(ctx, p.AccountID, cutoff) },
		func() (int64, error) { return h.store.DeleteOutboxBatch(ctx, p.AccountID, cutoff) },
	})
	rec.RowsDeleted = n
	h.finishRun(ctx, p, rec, started, err)
	return complete, err
}

// runAIIO faz o SCRUB de ai_runs.input/output e routing_decisions.input (classe ai_io). A linha
// sobrevive (C1.2). Dry-run => so conta.
func (h *PurgeHandler) runAIIO(ctx context.Context, p purgePayload, cutoff, deadline time.Time) (bool, error) {
	started := time.Now()
	rec := purgeRunRecord{Class: classAIIO, CutoffAt: cutoff}
	if p.DryRun {
		n, err := h.store.CountAIIO(ctx, p.AccountID, cutoff)
		rec.RowsScrubbed = n
		h.finishRun(ctx, p, rec, started, err)
		return true, err
	}
	n, complete, err := drainAll(ctx, deadline, []batchOp{
		func() (int64, error) { return h.store.ScrubAIRunsBatch(ctx, p.AccountID, cutoff) },
		func() (int64, error) { return h.store.ScrubRoutingDecisionsBatch(ctx, p.AccountID, cutoff) },
	})
	rec.RowsScrubbed = n
	h.finishRun(ctx, p, rec, started, err)
	return complete, err
}

// runAudit apaga audit_events + purge_runs (classe audit; purge_runs poda a si mesma). Dry-run
// => so conta.
func (h *PurgeHandler) runAudit(ctx context.Context, p purgePayload, cutoff, deadline time.Time) (bool, error) {
	started := time.Now()
	rec := purgeRunRecord{Class: classAudit, CutoffAt: cutoff}
	if p.DryRun {
		n, err := h.store.CountAudit(ctx, p.AccountID, cutoff)
		rec.RowsDeleted = n
		h.finishRun(ctx, p, rec, started, err)
		return true, err
	}
	n, complete, err := drainAll(ctx, deadline, []batchOp{
		func() (int64, error) { return h.store.DeleteAuditEventsBatch(ctx, p.AccountID, cutoff) },
		func() (int64, error) { return h.store.DeletePurgeRunsBatch(ctx, p.AccountID, cutoff) },
	})
	rec.RowsDeleted = n
	h.finishRun(ctx, p, rec, started, err)
	return complete, err
}

// runConversationClass poda a classe conversation: apaga o ARQUIVO de midia antes da LINHA
// (C5), faz o scrub explicito de ai_runs (C1.3) e apaga a conversa (cascade), depois os
// contatos orfaos. Batelado, com teto de tempo. complete=false quando bate o teto (re-enfileira).
func (h *PurgeHandler) runConversationClass(ctx context.Context, p purgePayload, cutoff, deadline time.Time) (bool, error) {
	started := time.Now()
	rec := purgeRunRecord{Class: classConversation, CutoffAt: cutoff}
	if p.DryRun {
		convs, err := h.store.CountConversationsToPurge(ctx, p.AccountID, cutoff)
		rec.RowsDeleted = convs
		if err == nil {
			files, ferr := h.store.CountMediaToPurge(ctx, p.AccountID, cutoff)
			rec.FilesDeleted, err = files, ferr
		}
		h.finishRun(ctx, p, rec, started, err)
		return true, err
	}

	for {
		if time.Now().After(deadline) {
			h.finishRun(ctx, p, rec, started, nil)
			return false, nil // incompleto: o caller re-enfileira a continuacao
		}
		ids, err := h.store.ConversationsToPurge(ctx, p.AccountID, cutoff)
		if err != nil {
			h.finishRun(ctx, p, rec, started, err)
			return false, err
		}
		if len(ids) == 0 {
			break
		}
		if err := h.purgeConversationBatch(ctx, p.AccountID, ids, &rec); err != nil {
			h.finishRun(ctx, p, rec, started, err)
			return false, err
		}
	}
	contacts, complete, err := drainBatches(ctx, deadline, func() (int64, error) {
		return h.store.DeleteOrphanContactsBatch(ctx, p.AccountID, cutoff)
	})
	rec.RowsDeleted += contacts
	h.finishRun(ctx, p, rec, started, err)
	return complete, err
}

// batchOp apaga/limpa UM batch e devolve as linhas afetadas.
type batchOp func() (int64, error)

// drainBatches roda op ate um batch vir < purgeBatchSize (drenado) ou o teto de tempo/ctx.
func drainBatches(ctx context.Context, deadline time.Time, op batchOp) (int64, bool, error) {
	var total int64
	for {
		if ctx.Err() != nil {
			return total, false, ctx.Err()
		}
		if time.Now().After(deadline) {
			return total, false, nil
		}
		n, err := op()
		total += n
		if err != nil {
			return total, false, err
		}
		if n < purgeBatchSize {
			return total, true, nil
		}
	}
}

// drainAll drena varios ops em sequencia (uma classe que toca varias tabelas). Para no primeiro
// que estourar o teto de tempo ou der erro.
func drainAll(ctx context.Context, deadline time.Time, ops []batchOp) (int64, bool, error) {
	var total int64
	for _, op := range ops {
		n, complete, err := drainBatches(ctx, deadline, op)
		total += n
		if err != nil {
			return total, false, err
		}
		if !complete {
			return total, false, nil
		}
	}
	return total, true, nil
}

// purgeConversationBatch apaga a midia (arquivo primeiro), faz o scrub de ai_runs e apaga as
// conversas do batch. Ordem C5: arquivo -> scrub -> linha. Falha de containment de midia
// (ErrMediaInvalid) e nao-fatal (pula a midia daquela conversa); erro de disco/banco e fatal.
func (h *PurgeHandler) purgeConversationBatch(ctx context.Context, accountID string, ids []string, rec *purgeRunRecord) error {
	for _, id := range ids {
		files, bytes, err := h.media.purgeConversationDir(accountID, id, false)
		rec.FilesDeleted += files
		rec.BytesFreed += bytes
		if err != nil && !errors.Is(err, ErrMediaInvalid) {
			return err
		}
	}
	scrubbed, err := h.store.ScrubRunsForConversations(ctx, accountID, ids)
	if err != nil {
		return err
	}
	rec.RowsScrubbed += scrubbed
	deleted, err := h.store.DeleteConversations(ctx, accountID, ids)
	if err != nil {
		return err
	}
	rec.RowsDeleted += deleted
	return nil
}

// runMediaOrphan varre os orfaos de midia da conta (arquivo sem media_storage_key vivo, com
// carencia de 24h). Uma linha em purge_runs (classe media_orphan).
func (h *PurgeHandler) runMediaOrphan(ctx context.Context, p purgePayload) error {
	started := time.Now()
	known, err := h.store.KnownMediaKeys(ctx, p.AccountID)
	if err != nil {
		return err
	}
	files, bytes, err := h.media.scanOrphans(p.AccountID, known, time.Now(), p.DryRun)
	h.finishRun(ctx, p, purgeRunRecord{Class: classMediaOrphan, CutoffAt: started.UTC(),
		FilesDeleted: files, BytesFreed: bytes}, started, err)
	return err
}

// finishRun completa e grava a linha de purge_runs. O erro e MASCARADO (jobs.MaskError):
// nunca payload/PII, so a classe do erro (C6). Falha ao gravar a trilha e apenas logada (a
// poda ja ocorreu; a evidencia e best-effort — nunca id/telefone/texto no log, so contagens).
func (h *PurgeHandler) finishRun(ctx context.Context, p purgePayload, rec purgeRunRecord, started time.Time, cause error) {
	rec.AccountID = p.AccountID
	rec.StartedAt = started
	rec.FinishedAt = time.Now()
	rec.Mode = "delete"
	if p.DryRun {
		rec.Mode = "dry_run"
	}
	if cause != nil {
		rec.Error = jobs.MaskError(cause)
	}
	if err := h.store.RecordPurgeRun(ctx, rec); err != nil && h.logger != nil {
		h.logger.Error("omnichannel_purge_run_record_failed",
			"account_id", p.AccountID, "class", rec.Class, "error", jobs.MaskError(err))
	}
	if h.logger != nil {
		h.logger.Info("omnichannel_purge_run", "account_id", p.AccountID, "class", rec.Class,
			"mode", rec.Mode, "rows_deleted", rec.RowsDeleted, "rows_scrubbed", rec.RowsScrubbed,
			"files_deleted", rec.FilesDeleted, "bytes_freed", rec.BytesFreed)
	}
}

// reenqueue enfileira a continuacao com uma NOVA idempotency_key (cont:{seq+1}) — a chave
// diaria ja foi consumida, entao a continuacao precisa de chave propria. ordering_key
// purge:{acct} serializa com o job atual (nao disputa a chave da conversa: nao atrasa envio).
func (h *PurgeHandler) reenqueue(ctx context.Context, p purgePayload) error {
	next := p
	next.Seq++
	payload, err := json.Marshal(next)
	if err != nil {
		return err
	}
	_, _, err = h.enqueuer.Enqueue(ctx, jobs.NewJob{
		AccountID:      p.AccountID,
		OrderingKey:    purgeOrderingKey(p.AccountID),
		IdempotencyKey: continuationKey(p.AccountID, p.Date, next.Seq),
		Kind:           PurgeAccountJobKind,
		Payload:        payload,
		MaxAttempts:    purgeMaxAttempts,
	})
	return err
}

// ============================================================================
// Helpers puros (testaveis sem banco)
// ============================================================================

// parsePurgePayload desserializa o corpo do job.
func parsePurgePayload(raw json.RawMessage) (purgePayload, error) {
	var p purgePayload
	if len(raw) == 0 {
		return p, errors.New("omnichannel: purge payload vazio")
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, fmt.Errorf("omnichannel: purge payload ilegivel: %w", err)
	}
	return p, nil
}

// cutoffFor devolve o instante de corte de uma classe: now - days. days sempre >= 1 (o
// resolver garante), entao o cutoff nunca cai no futuro nem em "agora".
func cutoffFor(now time.Time, days int) time.Time {
	if days < 1 {
		days = 1
	}
	return now.AddDate(0, 0, -days)
}

func purgeOrderingKey(accountID string) string { return "purge:" + accountID }

func dailyPurgeKey(accountID, date string) string { return "purge:" + accountID + ":" + date }

func continuationKey(accountID, date string, seq int) string {
	return fmt.Sprintf("purge:%s:%s:cont:%d", accountID, date, seq)
}
