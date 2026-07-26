package customerintelligence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

const (
	observationRetentionJobKind   = "source.observation.retention"
	observationRetentionMaxDrains = 20
)

var errObservationRetentionContinuation = errors.New(
	"customer intelligence: observation retention continuation required",
)

type observationRetentionJobPayload struct {
	ClientAccountID string `json:"clientAccountId"`
	ScheduledFor    string `json:"scheduledFor"`
}

type ObservationRetentionJobHandler struct {
	repository observationRetentionRepository
}

func NewObservationRetentionJobHandler(
	repository observationRetentionRepository,
) *ObservationRetentionJobHandler {
	return &ObservationRetentionJobHandler{repository: repository}
}

func (h *ObservationRetentionJobHandler) Handle(
	ctx context.Context,
	job jobs.Job,
) error {
	var payload observationRetentionJobPayload
	if h == nil || h.repository == nil ||
		json.Unmarshal(job.Payload, &payload) != nil ||
		!validUUID(job.AccountID) ||
		!validUUID(payload.ClientAccountID) {
		return &jobs.StatusError{Unrecoverable: true, Err: ErrInvalidInput}
	}
	if _, err := time.Parse("2006-01-02", payload.ScheduledFor); err != nil {
		return &jobs.StatusError{Unrecoverable: true, Err: ErrInvalidInput}
	}
	scope := Scope{
		AccountID:       job.AccountID,
		ClientAccountID: payload.ClientAccountID,
	}
	for drain := 0; drain < observationRetentionMaxDrains; drain++ {
		observationsApplied, err := h.repository.ApplyExpiredObservationRetention(
			ctx,
			scope,
			job.ID,
			observationRetentionBatchSize,
		)
		if err != nil {
			return err
		}
		snapshotsApplied, err := h.repository.ApplyExpiredContextSnapshotRetention(
			ctx,
			scope,
			job.ID,
			observationRetentionBatchSize,
		)
		if err != nil {
			return err
		}
		if observationsApplied < observationRetentionBatchSize &&
			snapshotsApplied < observationRetentionBatchSize {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	// Returning a safe retryable error continues the same idempotent job.
	// Observation and context rows already tombstoned are excluded by their
	// retention_state on the next attempt.
	return errObservationRetentionContinuation
}

var _ jobs.Handler = (*ObservationRetentionJobHandler)(nil)
