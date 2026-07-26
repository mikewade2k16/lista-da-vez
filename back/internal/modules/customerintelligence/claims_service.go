package customerintelligence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	maxAcceptedClaims       = 100
	maxClaimEvidenceRefs    = 100
	maxCandidateValueBytes  = 16 << 10
	maxClaimOutputSchemaLen = 160
)

type runtimeExtractedClaim struct {
	FactKey                string          `json:"factKey"`
	ValueType              string          `json:"valueType"`
	Value                  json.RawMessage `json:"value"`
	Confidence             float64         `json:"confidence"`
	EvidenceObservationIDs []string        `json:"evidenceObservationIds"`
	ValidFrom              *string         `json:"validFrom"`
	ValidUntil             *string         `json:"validUntil"`
}

type loadedRuntimeClaims struct {
	source runtimeClaimSource
	claims []runtimeExtractedClaim
}

func (s *Service) CandidateClaims(
	ctx context.Context,
	scope Scope,
	relationshipID, status string,
	limit int,
) ([]CandidateClaim, error) {
	_, enabled, err := s.capability(ctx, scope, CapabilityProfile)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return []CandidateClaim{}, nil
	}
	if err := s.authorizeRelationship(ctx, scope, "", relationshipID); err != nil {
		return nil, err
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = "candidate"
	}
	if !validMode(status, "candidate", "accepted", "rejected") {
		return nil, ErrInvalidInput
	}
	repository, ok := s.foundation.(candidateClaimRepository)
	if !ok {
		return nil, ErrNotFound
	}
	items, err := repository.ListCandidateClaims(
		ctx, scope, relationshipID, status, bounded(limit, 50, 1, 200),
	)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index], err = s.decryptCandidateClaim(items[index])
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Service) ReviewCandidateClaim(
	ctx context.Context,
	scope Scope,
	actorID, claimID string,
	input ClaimReviewInput,
) (CandidateClaim, error) {
	_, enabled, err := s.capability(ctx, scope, CapabilityProfile)
	if err != nil {
		return CandidateClaim{}, err
	}
	if !enabled {
		return CandidateClaim{}, ErrCapabilityDisabled
	}
	if !validUUID(claimID) ||
		!validUUID(actorID) ||
		!validMode(input.Status, "accepted", "rejected") ||
		!safeKeyPattern.MatchString(strings.TrimSpace(input.ReasonCode)) ||
		len(input.ReasonCode) > 160 ||
		input.ExpectedRevision < 1 {
		return CandidateClaim{}, ErrInvalidInput
	}
	repository, ok := s.foundation.(candidateClaimRepository)
	if !ok {
		return CandidateClaim{}, ErrNotFound
	}
	current, err := repository.GetCandidateClaim(ctx, scope, claimID)
	if err != nil {
		return CandidateClaim{}, err
	}
	if err := s.authorizeRelationship(
		ctx, scope, current.SubjectID, current.RelationshipID,
	); err != nil {
		return CandidateClaim{}, err
	}
	item, err := repository.ReviewCandidateClaim(
		ctx, scope, actorID, claimID, input,
	)
	if err != nil {
		return CandidateClaim{}, err
	}
	return s.decryptCandidateClaim(item)
}

func (s *Service) recordOutcome(
	ctx context.Context,
	outcome AcceptedOutcome,
) (bool, error) {
	if len(outcome.Claims) == 0 {
		return s.foundation.RecordOutcome(ctx, outcome)
	}
	if !outcome.Accepted || !validUUID(outcome.SubjectID) ||
		len(outcome.Claims) > maxAcceptedClaims {
		return false, ErrInvalidInput
	}
	repository, ok := s.foundation.(candidateClaimRepository)
	if !ok {
		return false, ErrNotFound
	}
	prepared, err := s.prepareAcceptedClaims(ctx, repository, outcome)
	if err != nil {
		return false, err
	}
	return repository.RecordOutcomeWithClaims(ctx, outcome, prepared)
}

func (s *Service) prepareAcceptedClaims(
	ctx context.Context,
	repository candidateClaimRepository,
	outcome AcceptedOutcome,
) ([]preparedCandidateClaim, error) {
	if s.secrets == nil {
		return nil, ErrSecretsUnavailable
	}
	scope := Scope{
		AccountID:       outcome.AccountID,
		ClientAccountID: outcome.ClientAccountID,
	}
	runRefs := make(map[string]ProcessRunRef, len(outcome.ProcessRuns))
	for _, reference := range outcome.ProcessRuns {
		if err := validateAcceptedRunReference(reference); err != nil {
			return nil, err
		}
		if _, exists := runRefs[reference.RunID]; exists {
			return nil, ErrInvalidInput
		}
		runRefs[reference.RunID] = reference
	}
	seenOrdinals := make(map[string]bool, len(outcome.Claims))
	loaded := make(map[string]loadedRuntimeClaims)
	prepared := make([]preparedCandidateClaim, 0, len(outcome.Claims))
	for _, reference := range outcome.Claims {
		runReference, exists := runRefs[reference.RuntimeRunID]
		if !exists ||
			reference.Ordinal < 0 || reference.Ordinal >= maxAcceptedClaims ||
			reference.ProcessKey != runReference.ProcessKey ||
			reference.PromptBindingID != runReference.PromptBindingID ||
			reference.OutputSchemaVersion != runReference.OutputSchemaVersion {
			return nil, ErrInvalidInput
		}
		ordinalKey := reference.RuntimeRunID + ":" + strconv.Itoa(reference.Ordinal)
		if seenOrdinals[ordinalKey] {
			return nil, ErrInvalidInput
		}
		seenOrdinals[ordinalKey] = true

		runtimeClaims, exists := loaded[reference.RuntimeRunID]
		if !exists {
			source, err := repository.GetRuntimeClaimSource(
				ctx, scope, outcome.SubjectID, outcome.RelationshipID,
				reference.RuntimeRunID,
			)
			if err != nil {
				return nil, err
			}
			if !sameAcceptedRunReference(runReference, source.RunRef) {
				return nil, ErrInvalidInput
			}
			claims, err := s.decryptRuntimeClaims(source.OutputCiphertext)
			if err != nil {
				return nil, err
			}
			runtimeClaims = loadedRuntimeClaims{source: source, claims: claims}
			loaded[reference.RuntimeRunID] = runtimeClaims
		}
		if reference.Ordinal >= len(runtimeClaims.claims) {
			return nil, ErrInvalidInput
		}
		extracted := runtimeClaims.claims[reference.Ordinal]
		if strings.TrimSpace(reference.FactKey) != strings.TrimSpace(extracted.FactKey) ||
			strings.TrimSpace(reference.ValueType) != strings.TrimSpace(extracted.ValueType) {
			return nil, ErrInvalidInput
		}
		validFrom, err := parseOptionalClaimTime(extracted.ValidFrom)
		if err != nil {
			return nil, ErrInvalidInput
		}
		validUntil, err := parseOptionalClaimTime(extracted.ValidUntil)
		if err != nil || (validFrom != nil && validUntil != nil && !validUntil.After(*validFrom)) {
			return nil, ErrInvalidInput
		}
		extracted.FactKey = strings.TrimSpace(extracted.FactKey)
		extracted.ValueType = strings.TrimSpace(extracted.ValueType)
		if !safeKeyPattern.MatchString(extracted.FactKey) ||
			!validFactValueType(extracted.ValueType) ||
			extracted.Confidence < 0 || extracted.Confidence > 1 ||
			!validTypedClaimValue(extracted.ValueType, extracted.Value) {
			return nil, ErrInvalidInput
		}
		value := compactJSON(extracted.Value)
		ciphertext, err := s.secrets.Encrypt(string(value))
		if err != nil {
			return nil, err
		}
		cleanReference := AcceptedClaimRef{
			Ordinal:                reference.Ordinal,
			FactKey:                extracted.FactKey,
			ValueType:              extracted.ValueType,
			Confidence:             extracted.Confidence,
			EvidenceObservationIDs: validObservationIDs(extracted.EvidenceObservationIDs),
			ProcessKey:             runtimeClaims.source.RunRef.ProcessKey,
			RuntimeRunID:           runtimeClaims.source.RunRef.RunID,
			PromptBindingID:        runtimeClaims.source.RunRef.PromptBindingID,
			OutputSchemaVersion:    runtimeClaims.source.RunRef.OutputSchemaVersion,
		}
		if validFrom != nil {
			cleanReference.ValidFrom = validFrom.Format(time.RFC3339)
		}
		if validUntil != nil {
			cleanReference.ValidUntil = validUntil.Format(time.RFC3339)
		}
		prepared = append(prepared, preparedCandidateClaim{
			Reference:                  cleanReference,
			ValueCiphertext:            ciphertext,
			ValueCiphertextFingerprint: hashBytes([]byte(ciphertext)),
			ValidFrom:                  validFrom,
			ValidUntil:                 validUntil,
		})
	}
	return prepared, nil
}

func (s *Service) decryptRuntimeClaims(ciphertext string) ([]runtimeExtractedClaim, error) {
	plaintext, err := s.secrets.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}
	var output map[string]json.RawMessage
	if json.Unmarshal([]byte(plaintext), &output) != nil {
		return nil, ErrInvalidInput
	}
	raw, ok := output["extractedClaims"]
	if !ok {
		return nil, ErrInvalidInput
	}
	var claims []runtimeExtractedClaim
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&claims); err != nil || len(claims) > maxAcceptedClaims {
		return nil, ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidInput
	}
	if claims == nil {
		claims = []runtimeExtractedClaim{}
	}
	return claims, nil
}

func (s *Service) decryptCandidateClaim(item CandidateClaim) (CandidateClaim, error) {
	if item.valueCiphertext == "" || s.secrets == nil {
		return CandidateClaim{}, ErrSecretsUnavailable
	}
	plaintext, err := s.secrets.Decrypt(item.valueCiphertext)
	if err != nil {
		return CandidateClaim{}, err
	}
	item.Value = json.RawMessage(plaintext)
	item.valueCiphertext = ""
	return item, nil
}

func validateAcceptedRunReference(reference ProcessRunRef) error {
	requiredUUIDs := []string{
		reference.RunID,
		reference.ProcessDefinitionID,
		reference.ProcessConfigVersionID,
		reference.PromptBindingID,
		reference.PlatformPromptVersionID,
		reference.ProcessPromptVersionID,
		reference.AgentVersionID,
		reference.ModelID,
		reference.ContextSnapshotID,
	}
	for _, value := range requiredUUIDs {
		if !validUUID(value) {
			return ErrInvalidInput
		}
	}
	for _, value := range []string{
		reference.AgencyPromptVersionID,
		reference.ClientPromptVersionID,
	} {
		if value != "" && !validUUID(value) {
			return ErrInvalidInput
		}
	}
	if !validProcessKey(reference.ProcessKey) ||
		reference.Status != "succeeded" ||
		reference.ExecutionMode != "active" ||
		reference.OutputSchemaVersion == "" ||
		len(reference.OutputSchemaVersion) > maxClaimOutputSchemaLen {
		return ErrInvalidInput
	}
	return nil
}

func sameAcceptedRunReference(expected, actual ProcessRunRef) bool {
	return expected == actual
}

func parseOptionalClaimTime(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	raw := strings.TrimSpace(*value)
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		parsed, err = time.Parse("2006-01-02", raw)
	}
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func validTypedClaimValue(valueType string, raw json.RawMessage) bool {
	if len(raw) == 0 || len(raw) > maxCandidateValueBytes || !validJSONValue(raw) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return false
	}
	switch valueType {
	case "string", "enum":
		_, ok := value.(string)
		return ok
	case "date":
		text, ok := value.(string)
		if !ok {
			return false
		}
		_, err := time.Parse("2006-01-02", text)
		return err == nil
	case "timestamp":
		text, ok := value.(string)
		if !ok {
			return false
		}
		_, err := time.Parse(time.RFC3339, text)
		return err == nil
	case "integer":
		number, ok := value.(json.Number)
		if !ok {
			return false
		}
		_, err := strconv.ParseInt(number.String(), 10, 64)
		return err == nil
	case "decimal":
		number, ok := value.(json.Number)
		if !ok {
			return false
		}
		parsed, err := number.Float64()
		return err == nil && !math.IsInf(parsed, 0) && !math.IsNaN(parsed)
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "string_list":
		items, ok := value.([]any)
		if !ok || len(items) > 100 {
			return false
		}
		for _, item := range items {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	case "object_closed":
		_, ok := value.(map[string]any)
		return ok
	default:
		return false
	}
}

func compactJSON(raw json.RawMessage) json.RawMessage {
	var buffer bytes.Buffer
	if json.Compact(&buffer, raw) != nil {
		return raw
	}
	return buffer.Bytes()
}

func validObservationIDs(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if validUUID(value) && !seen[value] {
			seen[value] = true
			out = append(out, value)
			if len(out) == maxClaimEvidenceRefs {
				break
			}
		}
	}
	return out
}
