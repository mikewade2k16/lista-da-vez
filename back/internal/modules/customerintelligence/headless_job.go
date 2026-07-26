package customerintelligence

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

type relationshipRefreshJobPayload struct {
	ClientAccountID string    `json:"clientAccountId"`
	SubjectID       string    `json:"subjectId"`
	RelationshipID  string    `json:"relationshipId"`
	PurposeKey      string    `json:"purposeKey"`
	SourceKeys      []string  `json:"sourceKeys"`
	ProcessKeys     []string  `json:"processKeys"`
	ActorUserID     string    `json:"actorUserId,omitempty"`
	AsOf            time.Time `json:"asOf"`
}

type RelationshipRefreshJobHandler struct {
	service *Service
}

func NewRelationshipRefreshJobHandler(
	service *Service,
) *RelationshipRefreshJobHandler {
	return &RelationshipRefreshJobHandler{service: service}
}

func (h *RelationshipRefreshJobHandler) Handle(
	ctx context.Context,
	job jobs.Job,
) error {
	if h == nil || h.service == nil || !validUUID(job.AccountID) ||
		!validUUID(job.ID) {
		return &jobs.StatusError{Unrecoverable: true, Err: ErrInvalidInput}
	}
	var payload relationshipRefreshJobPayload
	if json.Unmarshal(job.Payload, &payload) != nil ||
		(payload.ActorUserID != "" && !validUUID(payload.ActorUserID)) {
		return &jobs.StatusError{Unrecoverable: true, Err: ErrInvalidInput}
	}
	requestID := job.ID + ".attempt." + strconv.Itoa(job.Attempts)
	result, err := h.service.executeRelationshipRefresh(
		ctx,
		job.AccountID,
		requestID,
		RelationshipRefreshInput{
			ClientAccountID: payload.ClientAccountID,
			SubjectID:       payload.SubjectID,
			RelationshipID:  payload.RelationshipID,
			PurposeKey:      payload.PurposeKey,
			SourceKeys:      payload.SourceKeys,
			ProcessKeys:     payload.ProcessKeys,
			IdempotencyKey:  job.ID,
			AsOf:            payload.AsOf,
		},
	)
	if err != nil &&
		result.PersistedRows > 0 &&
		len(result.RetryProcessKeys) > 0 {
		_, enqueueErr := h.service.EnqueueRelationshipRefresh(
			ctx,
			job.AccountID,
			payload.ActorUserID,
			RelationshipRefreshInput{
				ClientAccountID: payload.ClientAccountID,
				SubjectID:       payload.SubjectID,
				RelationshipID:  payload.RelationshipID,
				PurposeKey:      payload.PurposeKey,
				SourceKeys:      payload.SourceKeys,
				ProcessKeys:     result.RetryProcessKeys,
				IdempotencyKey: "headless-retry." + job.ID + "." +
					strconv.Itoa(job.Attempts),
				AsOf: payload.AsOf,
			},
		)
		if enqueueErr == nil {
			return nil
		}
	}
	return headlessJobError(err)
}

var _ jobs.Handler = (*RelationshipRefreshJobHandler)(nil)
