package customerintelligence

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

const (
	observationRetentionEnqueueInterval = 24 * time.Hour
	observationRetentionBootDelay       = 5 * time.Minute
)

type observationRetentionEnqueuer interface {
	Enqueue(ctx context.Context, job jobs.NewJob) (string, bool, error)
}

func EnqueueExpiredObservationRetention(
	ctx context.Context,
	repository observationRetentionRepository,
	enqueuer observationRetentionEnqueuer,
	now time.Time,
) (int, error) {
	scopes, err := repository.ListExpiredRetentionScopes(ctx)
	if err != nil {
		return 0, err
	}
	scheduledFor := now.UTC().Format("2006-01-02")
	enqueued := 0
	for _, scope := range scopes {
		payload, err := json.Marshal(observationRetentionJobPayload{
			ClientAccountID: scope.ClientAccountID,
			ScheduledFor:    scheduledFor,
		})
		if err != nil {
			return enqueued, err
		}
		_, created, err := enqueuer.Enqueue(ctx, jobs.NewJob{
			AccountID: scope.AccountID,
			OrderingKey: "observation-retention:" +
				scope.ClientAccountID,
			IdempotencyKey: "observation-retention:" +
				scope.ClientAccountID + ":" + scheduledFor,
			Kind:        observationRetentionJobKind,
			Payload:     payload,
			MaxAttempts: 5,
		})
		if err != nil {
			return enqueued, err
		}
		if created {
			enqueued++
		}
	}
	return enqueued, nil
}

func StartObservationRetentionScheduler(
	ctx context.Context,
	repository observationRetentionRepository,
	enqueuer observationRetentionEnqueuer,
	logger *slog.Logger,
) {
	go func() {
		timer := time.NewTimer(observationRetentionBootDelay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		enqueue := func() {
			count, err := EnqueueExpiredObservationRetention(
				ctx,
				repository,
				enqueuer,
				time.Now(),
			)
			if err != nil {
				if logger != nil {
					logger.Warn(
						"customer_intelligence_observation_retention_enqueue_failed",
						"error",
						jobs.MaskError(err),
					)
				}
				return
			}
			if logger != nil && count > 0 {
				logger.Info(
					"customer_intelligence_observation_retention_enqueued",
					"scopes",
					count,
				)
			}
		}
		enqueue()
		ticker := time.NewTicker(observationRetentionEnqueueInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				enqueue()
			}
		}
	}()
}
