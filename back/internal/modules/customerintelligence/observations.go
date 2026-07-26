package customerintelligence

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

const (
	maxObservationFieldDisplayBytes = 4096
	observationProvenanceNamespace  = "customer-intelligence.observation-provenance.v1"
	observationProvenancePrefix     = "obsref:v1:"
)

// ObservationRepository is deliberately separate from FoundationRepository.
// Besides keeping source ingestion and read projections independent, this lets
// tests that do not exercise observations continue to use small repository
// fakes.
type ObservationRepository interface {
	ListRelationshipObservations(
		ctx context.Context,
		scope Scope,
		relationshipID string,
		sourceKeys []string,
		purposeKeys []string,
		limit int,
	) ([]StoredObservation, error)
	GetObservation(
		ctx context.Context,
		scope Scope,
		observationID string,
	) (StoredObservation, error)
}

type ObservationAccessRecorder interface {
	RecordObservationAccess(
		ctx context.Context,
		scope Scope,
		actorUserID string,
		record StoredObservation,
		reasonCode string,
		revealed bool,
		fieldCount int,
	) error
}

// StoredObservation is an internal persistence projection. Ciphertext and the
// current allowlist never leave the service boundary.
type StoredObservation struct {
	ID                 string
	SubjectID          string
	RelationshipID     string
	SourceKey          string
	SourceEntityType   string
	SourceEntityID     string
	Snapshot           json.RawMessage
	SnapshotCiphertext string
	FieldAllowlist     []string
	Sensitivity        string
	PurposeKey         string
	SourceOccurredAt   *time.Time
	ObservedAt         time.Time
	ExpiresAt          *time.Time
}

// ContextObservation is the allowlisted, decrypted projection available to an
// LLM as untrusted JSON context. It intentionally contains no provider secret,
// ciphertext, idempotency key or unrestricted upstream payload.
type ContextObservation struct {
	ID            string          `json:"id"`
	SourceKey     string          `json:"sourceKey"`
	EntityType    string          `json:"entityType"`
	ProvenanceRef string          `json:"provenanceRef"`
	Sensitivity   string          `json:"sensitivity"`
	PurposeKey    string          `json:"purposeKey"`
	OccurredAt    *time.Time      `json:"occurredAt,omitempty"`
	ObservedAt    time.Time       `json:"observedAt"`
	ExpiresAt     *time.Time      `json:"expiresAt,omitempty"`
	Snapshot      json.RawMessage `json:"snapshot"`
}

type ObservationFieldView struct {
	Label        string `json:"label"`
	DisplayValue string `json:"displayValue"`
	Masked       bool   `json:"masked"`
}

// ObservationView is the audited UI projection. SnapshotFields is derived
// server-side from the source's current allowlist.
type ObservationView struct {
	ID             string                 `json:"id"`
	SourceKey      string                 `json:"sourceKey"`
	ProvenanceRef  string                 `json:"provenanceRef,omitempty"`
	Sensitivity    string                 `json:"sensitivity"`
	PurposeKey     string                 `json:"purposeKey"`
	RetentionState string                 `json:"retentionState"`
	ObservedAt     time.Time              `json:"observedAt"`
	ExpiresAt      *time.Time             `json:"expiresAt,omitempty"`
	Revealed       bool                   `json:"revealed,omitempty"`
	SnapshotFields []ObservationFieldView `json:"snapshotFields"`
}

type ObservationAccessInput struct {
	ActorUserID string
	ReasonCode  string
}

type ObservationRevealInput struct {
	ReasonCode string `json:"reasonCode"`
}

func (s *Service) Observations(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	sourceKeys []string,
	limit int,
) ([]ObservationView, error) {
	_, enabled, err := s.capability(ctx, scope, CapabilityProfile)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return []ObservationView{}, nil
	}
	if !validUUID(relationshipID) {
		return nil, ErrInvalidInput
	}
	if err := s.authorizeRelationship(ctx, scope, "", relationshipID); err != nil {
		return nil, err
	}
	keys, err := validatedSourceKeys(sourceKeys)
	if err != nil {
		return nil, err
	}
	purposeKeys, err := observationPurposeKeys("profile_view")
	if err != nil {
		return nil, err
	}
	if s.secrets == nil {
		return nil, ErrSecretsUnavailable
	}
	repository, ok := s.foundation.(ObservationRepository)
	if !ok {
		return nil, ErrNotFound
	}
	records, err := repository.ListRelationshipObservations(
		ctx,
		scope,
		relationshipID,
		keys,
		purposeKeys,
		bounded(limit, 50, 1, 100),
	)
	if err != nil {
		return nil, err
	}
	items := make([]ObservationView, 0, len(records))
	for _, record := range records {
		// The relationship profile route requires profile.view, not audit.view.
		// Restricted metadata is therefore omitted from this collection. An
		// auditor can inspect its masked projection through Observation().
		if record.Sensitivity == "restricted" {
			continue
		}
		item, materializeErr := s.observationView(scope, record, false)
		if materializeErr != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) Observation(
	ctx context.Context,
	scope Scope,
	observationID string,
	access ...ObservationAccessInput,
) (ObservationView, error) {
	record, err := s.authorizedObservation(ctx, scope, observationID)
	if err != nil {
		return ObservationView{}, err
	}
	item, err := s.observationView(scope, record, false)
	if err != nil {
		return ObservationView{}, err
	}
	if len(access) > 0 {
		if err := s.recordObservationAccess(
			ctx, scope, record, access[0], false, len(item.SnapshotFields),
		); err != nil {
			return ObservationView{}, err
		}
	}
	return item, nil
}

func (s *Service) RevealObservation(
	ctx context.Context,
	scope Scope,
	actorUserID, observationID string,
	input ObservationRevealInput,
) (ObservationView, error) {
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	if len(input.ReasonCode) > 80 || !safeKeyPattern.MatchString(input.ReasonCode) {
		return ObservationView{}, ErrInvalidInput
	}
	record, err := s.authorizedObservation(ctx, scope, observationID)
	if err != nil {
		return ObservationView{}, err
	}
	if !validSensitivity(record.Sensitivity) {
		return ObservationView{}, ErrForbidden
	}
	item, err := s.observationView(scope, record, true)
	if err != nil {
		return ObservationView{}, err
	}
	if err := s.recordObservationAccess(
		ctx,
		scope,
		record,
		ObservationAccessInput{
			ActorUserID: actorUserID,
			ReasonCode:  input.ReasonCode,
		},
		true,
		len(item.SnapshotFields),
	); err != nil {
		return ObservationView{}, err
	}
	return item, nil
}

func (s *Service) authorizedObservation(
	ctx context.Context,
	scope Scope,
	observationID string,
) (StoredObservation, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return StoredObservation{}, err
	}
	if !validUUID(observationID) {
		return StoredObservation{}, ErrInvalidInput
	}
	if s.secrets == nil {
		return StoredObservation{}, ErrSecretsUnavailable
	}
	repository, ok := s.foundation.(ObservationRepository)
	if !ok {
		return StoredObservation{}, ErrNotFound
	}
	record, err := repository.GetObservation(ctx, scope, observationID)
	if err != nil {
		return StoredObservation{}, err
	}
	if record.RelationshipID != "" {
		if err := s.authorizeRelationship(
			ctx,
			scope,
			record.SubjectID,
			record.RelationshipID,
		); err != nil {
			return StoredObservation{}, err
		}
	}
	return record, nil
}

func (s *Service) recordObservationAccess(
	ctx context.Context,
	scope Scope,
	record StoredObservation,
	access ObservationAccessInput,
	revealed bool,
	fieldCount int,
) error {
	access.ActorUserID = strings.TrimSpace(access.ActorUserID)
	access.ReasonCode = strings.TrimSpace(access.ReasonCode)
	if !validUUID(access.ActorUserID) ||
		len(access.ReasonCode) > 80 ||
		!safeKeyPattern.MatchString(access.ReasonCode) {
		return ErrInvalidInput
	}
	recorder, ok := s.foundation.(ObservationAccessRecorder)
	if !ok {
		return ErrNotFound
	}
	return recorder.RecordObservationAccess(
		ctx,
		scope,
		access.ActorUserID,
		record,
		access.ReasonCode,
		revealed,
		fieldCount,
	)
}

func (s *Service) contextObservations(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	sourceKeys []string,
	requestPurpose string,
	limit int,
) ([]ContextObservation, []string, error) {
	repository, ok := s.foundation.(ObservationRepository)
	if !ok {
		return []ContextObservation{}, []string{"source_observations_unavailable"}, nil
	}
	keys, err := validatedSourceKeys(sourceKeys)
	if err != nil {
		return nil, nil, err
	}
	purposeKeys, err := observationPurposeKeys(requestPurpose)
	if err != nil {
		return nil, nil, err
	}
	records, err := repository.ListRelationshipObservations(
		ctx,
		scope,
		relationshipID,
		keys,
		purposeKeys,
		bounded(limit, defaultContextItems, 1, maxContextItems),
	)
	if err != nil {
		return nil, nil, err
	}
	items := make([]ContextObservation, 0, len(records))
	warnings := make([]string, 0)
	for _, record := range records {
		if !validSensitivity(record.Sensitivity) {
			warnings = append(warnings, "invalid_observation_sensitivity_omitted")
			continue
		}
		// Restricted observations stay available as audited metadata but are
		// never placed in a generic LLM context without a dedicated policy.
		if record.Sensitivity == "restricted" {
			warnings = append(warnings, "restricted_observation_omitted")
			continue
		}
		snapshot, materializeErr := s.materializeObservationSnapshot(record)
		if materializeErr != nil {
			warnings = append(warnings, "observation_decryption_failed")
			continue
		}
		provenanceRef, provenanceErr := s.observationProvenanceRef(scope, record)
		if provenanceErr != nil {
			warnings = append(warnings, "observation_provenance_unavailable")
			continue
		}
		items = append(items, ContextObservation{
			ID:            record.ID,
			SourceKey:     record.SourceKey,
			EntityType:    record.SourceEntityType,
			ProvenanceRef: provenanceRef,
			Sensitivity:   record.Sensitivity,
			PurposeKey:    record.PurposeKey,
			OccurredAt:    record.SourceOccurredAt,
			ObservedAt:    record.ObservedAt,
			ExpiresAt:     record.ExpiresAt,
			Snapshot:      snapshot,
		})
	}
	return items, uniqueSorted(warnings), nil
}

func (s *Service) observationView(
	scope Scope,
	record StoredObservation,
	reveal bool,
) (ObservationView, error) {
	protected := observationSensitivityProtected(record.Sensitivity) && !reveal
	values := make(map[string]json.RawMessage)
	keys := make([]string, 0, len(record.FieldAllowlist))
	if protected {
		for _, key := range record.FieldAllowlist {
			if safeKeyPattern.MatchString(key) {
				keys = append(keys, key)
			}
		}
		keys = uniqueSorted(keys)
		if len(keys) == 0 {
			return ObservationView{}, ErrForbidden
		}
	} else {
		snapshot, err := s.materializeObservationSnapshot(record)
		if err != nil {
			return ObservationView{}, err
		}
		if err := json.Unmarshal(snapshot, &values); err != nil {
			return ObservationView{}, ErrInvalidInput
		}
		for key := range values {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	fields := make([]ObservationFieldView, 0, len(keys))
	for _, key := range keys {
		display := "[conteudo protegido]"
		if !protected {
			display = compactObservationDisplay(values[key])
		}
		fields = append(fields, ObservationFieldView{
			Label:        key,
			DisplayValue: display,
			Masked:       protected,
		})
	}
	provenanceRef, err := s.observationProvenanceRef(scope, record)
	if err != nil {
		return ObservationView{}, err
	}
	return ObservationView{
		ID:             record.ID,
		SourceKey:      record.SourceKey,
		ProvenanceRef:  provenanceRef,
		Sensitivity:    record.Sensitivity,
		PurposeKey:     record.PurposeKey,
		RetentionState: observationRetentionState(record.ExpiresAt, s.now().UTC()),
		ObservedAt:     record.ObservedAt,
		ExpiresAt:      record.ExpiresAt,
		Revealed:       reveal,
		SnapshotFields: fields,
	}, nil
}

func (s *Service) materializeObservationSnapshot(
	record StoredObservation,
) (json.RawMessage, error) {
	raw := record.Snapshot
	if record.SnapshotCiphertext != "" {
		if s.secrets == nil {
			return nil, ErrSecretsUnavailable
		}
		plaintext, err := s.secrets.Decrypt(record.SnapshotCiphertext)
		if err != nil {
			return nil, err
		}
		raw = json.RawMessage(plaintext)
	}
	var snapshot map[string]json.RawMessage
	if json.Unmarshal(raw, &snapshot) != nil || snapshot == nil {
		return nil, ErrInvalidInput
	}
	if len(record.FieldAllowlist) == 0 {
		return nil, ErrForbidden
	}
	allowed := make(map[string]bool, len(record.FieldAllowlist))
	for _, key := range record.FieldAllowlist {
		if safeKeyPattern.MatchString(key) {
			allowed[key] = true
		}
	}
	filtered := make(map[string]json.RawMessage)
	for key, value := range snapshot {
		if allowed[key] {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return nil, ErrForbidden
	}
	result, err := json.Marshal(filtered)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func validatedSourceKeys(values []string) ([]string, error) {
	keys := uniqueSorted(values)
	for _, key := range keys {
		if !validSourceKey(key) {
			return nil, ErrInvalidInput
		}
	}
	return keys, nil
}

func (s *Service) observationProvenanceRef(
	scope Scope,
	record StoredObservation,
) (string, error) {
	return s.opaqueObservationRef(scope, record.SourceKey, record.ID)
}

func (s *Service) opaqueObservationRef(
	scope Scope,
	sourceKey string,
	observationID string,
) (string, error) {
	if s.secrets == nil {
		return "", ErrSecretsUnavailable
	}
	fingerprint := s.secrets.OpaqueFingerprint(
		observationProvenanceNamespace,
		scope.AccountID,
		scope.ClientAccountID,
		sourceKey,
		observationID,
	)
	if fingerprint == "" {
		return "", ErrSecretsUnavailable
	}
	return observationProvenancePrefix + fingerprint, nil
}

func observationSensitivityProtected(sensitivity string) bool {
	return !validMode(sensitivity, "public", "internal")
}

func observationPurposeKeys(requestPurpose string) ([]string, error) {
	requestPurpose = strings.TrimSpace(requestPurpose)
	if !safeKeyPattern.MatchString(requestPurpose) {
		return nil, ErrInvalidInput
	}
	switch requestPurpose {
	case "customer_service":
		return []string{
			"customer_profile",
			"customer_relationship",
			"customer_service",
		}, nil
	case "profile_view":
		return []string{"customer_profile", "customer_relationship"}, nil
	default:
		return []string{requestPurpose}, nil
	}
}

func observationRetentionState(expiresAt *time.Time, now time.Time) string {
	if expiresAt == nil {
		return "active"
	}
	if !expiresAt.After(now) {
		return "expired"
	}
	return "active"
}

func compactObservationDisplay(raw json.RawMessage) string {
	var compact any
	if err := json.Unmarshal(raw, &compact); err != nil {
		return ""
	}
	var display string
	if text, ok := compact.(string); ok {
		display = text
	} else {
		encoded, err := json.Marshal(compact)
		if err != nil {
			return ""
		}
		display = string(encoded)
	}
	runes := []rune(display)
	if len(runes) <= maxObservationFieldDisplayBytes {
		return display
	}
	return string(runes[:maxObservationFieldDisplayBytes]) + "..."
}
