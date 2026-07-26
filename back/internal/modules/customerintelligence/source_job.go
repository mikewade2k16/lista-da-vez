package customerintelligence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

const (
	sourceJobKind  = "source.ingest"
	sourceJobTable = "intelligence.source_ingestion_jobs"
)

type sourceJobPayload struct {
	RunID           string `json:"runId"`
	ClientAccountID string `json:"clientAccountId"`
	RelationshipID  string `json:"relationshipId,omitempty"`
}

type SourceJobHandler struct {
	service *Service
}

type classifiedSourceFailure interface {
	SourceFailureCode() string
	SourceRetryable() bool
}

func NewSourceJobHandler(service *Service) *SourceJobHandler {
	return &SourceJobHandler{service: service}
}

func (h *SourceJobHandler) Handle(ctx context.Context, job jobs.Job) error {
	var payload sourceJobPayload
	if json.Unmarshal(job.Payload, &payload) != nil || !validUUID(payload.RunID) ||
		!validUUID(payload.ClientAccountID) ||
		(payload.RelationshipID != "" && !validUUID(payload.RelationshipID)) {
		return &jobs.StatusError{Unrecoverable: true, Err: ErrInvalidInput}
	}
	run, err := h.service.foundation.GetSourceRun(ctx, Scope{
		AccountID:       job.AccountID,
		ClientAccountID: payload.ClientAccountID,
	}, payload.RunID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return &jobs.StatusError{Unrecoverable: true, Err: ErrNotFound}
		}
		return err
	}
	if run.Status == "completed" || run.Status == "partial" {
		return nil
	}
	scope := Scope{AccountID: run.AccountID, ClientAccountID: run.ClientAccountID}
	_, enabled, err := h.service.capability(ctx, scope, CapabilitySourceSync)
	if err != nil {
		return err
	}
	if !enabled {
		if completeErr := h.service.foundation.CompleteSourceRun(
			ctx, run.AccountID, run.ID, "failed", 0, 0, 0, "capability_disabled",
		); completeErr != nil {
			return completeErr
		}
		return nil
	}
	config, err := h.service.foundation.GetSourceConfig(ctx, scope, run.SourceConfigID)
	if err != nil {
		return err
	}
	if config.Status != "enabled" {
		if completeErr := h.service.foundation.CompleteSourceRun(
			ctx, run.AccountID, run.ID, "failed", 0, 0, 0, "source_disabled",
		); completeErr != nil {
			return completeErr
		}
		return nil
	}
	adapter := h.service.sourceAdapters[config.SourceKey]
	if adapter == nil {
		if completeErr := h.service.foundation.CompleteSourceRun(
			ctx, run.AccountID, run.ID, "failed", 0, 0, 0, "adapter_unavailable",
		); completeErr != nil {
			return completeErr
		}
		return nil
	}
	observations, err := adapter.Fetch(ctx, config, payload.RelationshipID)
	if err != nil {
		errorCode := safeErrorCode(err)
		var classified classifiedSourceFailure
		if errors.As(err, &classified) {
			if candidate := classified.SourceFailureCode(); len(candidate) <= 120 &&
				safeKeyPattern.MatchString(candidate) {
				errorCode = candidate
			}
		}
		if completeErr := h.service.foundation.CompleteSourceRun(
			ctx, run.AccountID, run.ID, "failed", 0, 0, 0, errorCode,
		); completeErr != nil {
			return completeErr
		}
		if classified != nil && !classified.SourceRetryable() {
			return nil
		}
		return err
	}
	acceptedObservations := make([]Observation, 0, len(observations))
	rejected := 0
	for _, observation := range observations {
		if observation.ScopeType == "" {
			observation.ScopeType = ObservationScopeSubject
		}
		if observation.ScopeType == ObservationScopeSubject && observation.RelationshipID == "" {
			observation.RelationshipID = payload.RelationshipID
		}
		if observation.PurposeKey == "" {
			observation.PurposeKey = config.PurposeKey
		}
		filtered, filterErr := filterObservation(config, observation)
		if filterErr != nil {
			rejected++
			continue
		}
		if filtered.ScopeType == ObservationScopeSubject && filtered.RelationshipID != "" {
			if scopeErr := h.service.authorizeRelationship(
				ctx, scope, filtered.SubjectID, filtered.RelationshipID,
			); scopeErr != nil {
				rejected++
				continue
			}
		}
		if validSensitivity(filtered.Sensitivity) &&
			(filtered.Sensitivity == "personal" || filtered.Sensitivity == "sensitive" ||
				filtered.Sensitivity == "restricted") {
			if h.service.secrets == nil {
				rejected++
				continue
			}
			ciphertext, encryptErr := h.service.secrets.Encrypt(string(filtered.Snapshot))
			if encryptErr != nil {
				return encryptErr
			}
			filtered.SnapshotCiphertext = ciphertext
		}
		acceptedObservations = append(acceptedObservations, filtered)
	}
	accepted, err := h.service.foundation.InsertObservations(ctx, run, acceptedObservations)
	if err != nil {
		return err
	}
	refreshErrorCode := ""
	if h.service.headlessJobs != nil {
		relationships := make(map[string]string)
		for _, observation := range acceptedObservations {
			if observation.ScopeType == ObservationScopeSubject &&
				validUUID(observation.SubjectID) &&
				validUUID(observation.RelationshipID) {
				relationships[observation.RelationshipID] = observation.SubjectID
			}
		}
		for relationshipID, subjectID := range relationships {
			_, enqueueErr := h.service.EnqueueRelationshipRefresh(
				ctx,
				run.AccountID,
				"",
				RelationshipRefreshInput{
					ClientAccountID: run.ClientAccountID,
					SubjectID:       subjectID,
					RelationshipID:  relationshipID,
					PurposeKey:      config.PurposeKey,
					SourceKeys:      []string{config.SourceKey},
					IdempotencyKey:  "source-run." + run.ID,
					AsOf:            h.service.now().UTC(),
				},
			)
			if enqueueErr != nil && !errors.Is(enqueueErr, ErrCapabilityDisabled) {
				// Observations are already committed. Keep ingestion truthful
				// and expose a bounded warning instead of retrying the whole
				// source job as if the persisted data had failed.
				refreshErrorCode = "refresh_enqueue_failed"
			}
		}
	}
	status := "completed"
	if rejected > 0 {
		status = "partial"
	}
	return h.service.foundation.CompleteSourceRun(
		ctx,
		run.AccountID,
		run.ID,
		status,
		len(observations),
		accepted,
		rejected,
		refreshErrorCode,
	)
}

func filterObservation(config SourceConfig, item Observation) (Observation, error) {
	if item.ScopeType == "" {
		item.ScopeType = ObservationScopeSubject
	}
	configPurpose := strings.TrimSpace(config.PurposeKey)
	itemPurpose := strings.TrimSpace(item.PurposeKey)
	if configPurpose == "" || itemPurpose != configPurpose {
		return Observation{}, ErrForbidden
	}
	item.PurposeKey = configPurpose
	classification := observationClassification(item.ScopeType)
	if stringsEmpty(item.EntityType, item.EntityID, item.Sensitivity, item.PurposeKey) ||
		!safeKeyPattern.MatchString(item.EntityType) ||
		!validSensitivity(item.Sensitivity) ||
		!validJSONObject(item.Snapshot) ||
		!validMode(item.ScopeType, ObservationScopeSubject, ObservationScopeBusiness) ||
		classification == "" ||
		(item.Classification != "" && item.Classification != classification) ||
		(item.SubjectID != "" && !validUUID(item.SubjectID)) ||
		(item.RelationshipID != "" && !validUUID(item.RelationshipID)) {
		return Observation{}, ErrInvalidInput
	}
	item.Classification = classification
	if item.ScopeType == ObservationScopeBusiness &&
		(item.SubjectID != "" || item.RelationshipID != "") {
		return Observation{}, ErrInvalidInput
	}
	if item.ScopeType == ObservationScopeSubject &&
		(item.SubjectID == "" || item.RelationshipID == "") {
		return Observation{}, ErrInvalidInput
	}
	var snapshot map[string]json.RawMessage
	if json.Unmarshal(item.Snapshot, &snapshot) != nil {
		return Observation{}, ErrInvalidInput
	}
	if len(config.FieldAllowlist) == 0 {
		return Observation{}, ErrForbidden
	}
	filtered := make(map[string]json.RawMessage)
	for key, value := range snapshot {
		if sourceConfigAllows(config, key) {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return Observation{}, ErrForbidden
	}
	raw, err := json.Marshal(filtered)
	if err != nil {
		return Observation{}, err
	}
	item.Snapshot = raw
	return item, nil
}

func observationClassification(scopeType string) string {
	switch scopeType {
	case "", ObservationScopeSubject:
		return ObservationClassificationRelationship
	case ObservationScopeBusiness:
		return ObservationClassificationBusinessContext
	default:
		return ""
	}
}

func stringsEmpty(values ...string) bool {
	for _, value := range values {
		if value == "" {
			return true
		}
	}
	return false
}

var _ jobs.Handler = (*SourceJobHandler)(nil)
