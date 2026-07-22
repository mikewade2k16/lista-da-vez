package omnichannel

import (
	"context"
	"log/slog"
	"time"
)

const slaEvaluationInterval = time.Minute

// StartSLAScheduler só escreve eventos idempotentes na tabela do Omnichannel.
// Ele não envia mensagens nem toca workflows n8n; restart é seguro porque as
// chaves `sla:{handoff}:{event}` suprimem duplicatas.
func StartSLAScheduler(ctx context.Context, store *Store, logger *slog.Logger) {
	if store == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	go func() {
		ticker := time.NewTicker(slaEvaluationInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				created, err := store.EvaluateSLAs(ctx, now.UTC().Unix())
				if err != nil {
					logger.Error("omnichannel_sla_evaluate_failed", "error", err.Error())
					continue
				}
				if created > 0 {
					logger.Info("omnichannel_sla_events_created", "count", created)
				}
			}
		}
	}()
}
