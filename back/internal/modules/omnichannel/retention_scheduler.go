package omnichannel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

// ============================================================================
// F13 — Enfileiradores + ticker do purge (so ENFILEIRAM; o trabalho e do worker)
// ============================================================================
//
// Padrao da casa (app.go:116/133, cardapio): um ticker 24h/7d que so ENFILEIRA um job por
// conta no platform/jobs (F3). O primeiro disparo fica ~5 min apos o boot, fora do caminho
// critico de subida. A lista de contas vem do catalogo (AccountsWithModule), nunca hardcoded.

const (
	// intervalos do enfileirador (C4/C5).
	retentionEnqueueInterval   = 24 * time.Hour
	mediaOrphanEnqueueInterval = 7 * 24 * time.Hour
	// retentionBootDelay adia o primeiro disparo para fora do caminho critico de subida.
	retentionBootDelay = 5 * time.Minute
)

// EnqueueDailyPurge enfileira UM job de purge por conta com o modulo habilitado. A idempotencia
// diaria (unique (account_id, idempotency_key) da F3) torna re-boot/duplo tick no-op de graca.
func EnqueueDailyPurge(ctx context.Context, store *RetentionStore, enq purgeEnqueuer, dryRun bool) (int, error) {
	accounts, err := store.AccountsWithModule(ctx)
	if err != nil {
		return 0, err
	}
	date := time.Now().UTC().Format("2006-01-02")
	enqueued := 0
	for _, acct := range accounts {
		payload, mErr := json.Marshal(purgePayload{AccountID: acct, Date: date, DryRun: dryRun})
		if mErr != nil {
			return enqueued, mErr
		}
		if _, _, eErr := enq.Enqueue(ctx, jobs.NewJob{
			AccountID:      acct,
			OrderingKey:    purgeOrderingKey(acct),
			IdempotencyKey: dailyPurgeKey(acct, date),
			Kind:           PurgeAccountJobKind,
			Payload:        payload,
			MaxAttempts:    purgeMaxAttempts,
		}); eErr != nil {
			return enqueued, eErr
		}
		enqueued++
	}
	return enqueued, nil
}

// EnqueueMediaOrphanScan enfileira a varredura semanal de orfaos de midia por conta.
func EnqueueMediaOrphanScan(ctx context.Context, store *RetentionStore, enq purgeEnqueuer, dryRun bool) (int, error) {
	accounts, err := store.AccountsWithModule(ctx)
	if err != nil {
		return 0, err
	}
	year, week := time.Now().UTC().ISOWeek()
	stamp := fmt.Sprintf("%04d-W%02d", year, week)
	enqueued := 0
	for _, acct := range accounts {
		payload, mErr := json.Marshal(purgePayload{AccountID: acct, Date: stamp, DryRun: dryRun})
		if mErr != nil {
			return enqueued, mErr
		}
		if _, _, eErr := enq.Enqueue(ctx, jobs.NewJob{
			AccountID:      acct,
			OrderingKey:    purgeOrderingKey(acct),
			IdempotencyKey: fmt.Sprintf("purge-media:%s:%s", acct, stamp),
			Kind:           PurgeMediaOrphanJobKind,
			Payload:        payload,
			MaxAttempts:    purgeMaxAttempts,
		}); eErr != nil {
			return enqueued, eErr
		}
		enqueued++
	}
	return enqueued, nil
}

// StartRetentionScheduler sobe os tickers (padrao app.go:116/133). Para quando ctx morre.
// Primeiro disparo ~5 min apos o boot (fora do caminho critico de subida).
func StartRetentionScheduler(ctx context.Context, store *RetentionStore, enq purgeEnqueuer, logger *slog.Logger) {
	go retentionTicker(ctx, retentionEnqueueInterval, func() {
		if n, err := EnqueueDailyPurge(ctx, store, enq, false); err != nil && logger != nil {
			logger.Warn("omnichannel_purge_enqueue_failed", "error", jobs.MaskError(err))
		} else if logger != nil {
			logger.Info("omnichannel_purge_enqueued", "accounts", n)
		}
	})
	go retentionTicker(ctx, mediaOrphanEnqueueInterval, func() {
		if n, err := EnqueueMediaOrphanScan(ctx, store, enq, false); err != nil && logger != nil {
			logger.Warn("omnichannel_media_orphan_enqueue_failed", "error", jobs.MaskError(err))
		} else if logger != nil {
			logger.Info("omnichannel_media_orphan_enqueued", "accounts", n)
		}
	})
}

// retentionTicker adia o primeiro disparo por retentionBootDelay, dispara, e repete a cada
// interval ate o ctx morrer. Respeita o cancelamento em cada espera.
func retentionTicker(ctx context.Context, interval time.Duration, fn func()) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(retentionBootDelay):
	}
	fn()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fn()
		}
	}
}
