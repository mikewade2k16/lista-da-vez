package customerintelligence

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"time"
)

const (
	maxProcessKeyBytes        = 160
	maxProcessEvidenceRefs    = 200
	maxProcessReasonCodes     = 20
	maxProcessCandidateClaims = 100
)

var processSafetyFlagCatalog = map[string]struct{}{
	"adult":             {},
	"financial_data":    {},
	"hate":              {},
	"identity_document": {},
	"malware":           {},
	"medical_data":      {},
	"personal_data":     {},
	"self_harm":         {},
	"unknown":           {},
	"violence":          {},
}

func validateTriageProcessOutput(output triageResult) error {
	if !validProcessConfidence(output.Confidence) ||
		!validProcessRawJSONArray(output.ExtractedClaims, 64<<10) {
		return ErrInvalidInput
	}
	return validateClosure(output.Closure)
}

func validateReplyProcessOutput(output replyResult) error {
	if !validProcessConfidence(output.Confidence) {
		return ErrInvalidInput
	}
	return validateClosure(output.Closure)
}

func validateHandoffSummaryOutput(output handoffSummaryResult) error {
	if !validRequiredProcessText(output.Summary, 4000) ||
		!validProcessSafeKey(output.ReasonCode, maxProcessKeyBytes) ||
		!validProcessSafeKeys(output.CollectedFieldKeys, 0, 50, maxProcessKeyBytes) ||
		!validProcessSafeKeys(output.PendingFieldKeys, 0, 50, maxProcessKeyBytes) ||
		processStringSetsOverlap(output.CollectedFieldKeys, output.PendingFieldKeys) ||
		!validProcessSafeKeys(output.RedactionCodes, 0, 20, maxProcessKeyBytes) ||
		!validProcessUUIDs(output.MessageIDs, 0, 100) ||
		validateProcessEvidenceRefs(output.EvidenceRefs, 0, 100) != nil ||
		(len(output.MessageIDs) == 0 && len(output.EvidenceRefs) == 0) ||
		!validProcessConfidence(output.Confidence) {
		return ErrInvalidInput
	}
	return nil
}

func validateMemoryExtractOutput(output memoryExtractResult) error {
	return validateProcessCandidateClaims(output.Claims, 0, maxProcessCandidateClaims)
}

func validateProfileSummaryOutput(output profileSummaryResult) error {
	if !validRequiredProcessText(output.Summary, 8000) ||
		len(output.Sections) == 0 || len(output.Sections) > 12 ||
		validateProcessEvidenceRefs(output.EvidenceRefs, 0, maxProcessEvidenceRefs) != nil ||
		validateProcessFactRefs(output.FactRefs, 0, 100) != nil ||
		(len(output.EvidenceRefs) == 0 && len(output.FactRefs) == 0) ||
		!validProcessConfidence(output.Confidence) {
		return ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(output.Sections))
	for _, section := range output.Sections {
		if !validProcessSafeKey(section.Key, 80) ||
			!validRequiredProcessText(section.Content, 4000) ||
			validateProcessEvidenceRefs(section.EvidenceRefs, 0, 100) != nil ||
			validateProcessFactRefs(section.FactRefs, 0, 50) != nil ||
			(len(section.EvidenceRefs) == 0 && len(section.FactRefs) == 0) ||
			!validProcessConfidence(section.Confidence) {
			return ErrInvalidInput
		}
		if _, duplicate := seen[section.Key]; duplicate {
			return ErrInvalidInput
		}
		seen[section.Key] = struct{}{}
	}
	return nil
}

func validateMediaImageAnalysisOutput(output mediaImageAnalysisResult) error {
	if !validProcessConfidence(output.Confidence) ||
		!validProcessCatalogKeys(output.SafetyFlags, 0, 20, processSafetyFlagCatalog) ||
		validateProcessEvidenceRefs(output.EvidenceRefs, 0, 100) != nil ||
		validateProcessCandidateClaims(output.CandidateClaims, 0, 50) != nil ||
		!validProcessSafeKeys(output.BlockReasonCodes, 0, 20, maxProcessKeyBytes) {
		return ErrInvalidInput
	}
	if output.Blocked {
		if strings.TrimSpace(output.Description) != "" ||
			len(output.CandidateClaims) != 0 ||
			len(output.BlockReasonCodes) == 0 {
			return ErrInvalidInput
		}
		return nil
	}
	if !validRequiredProcessText(output.Description, 8000) ||
		len(output.EvidenceRefs) == 0 ||
		len(output.BlockReasonCodes) != 0 {
		return ErrInvalidInput
	}
	return nil
}

func validateMediaDocumentAnalysisOutput(output mediaDocumentAnalysisResult) error {
	if !validProcessConfidence(output.Confidence) ||
		!validProcessCatalogKeys(output.SafetyFlags, 0, 20, processSafetyFlagCatalog) ||
		validateProcessEvidenceRefs(output.EvidenceRefs, 0, 100) != nil ||
		validateProcessCandidateClaims(output.CandidateClaims, 0, 50) != nil ||
		!validProcessSafeKeys(output.BlockReasonCodes, 0, 20, maxProcessKeyBytes) {
		return ErrInvalidInput
	}
	if output.Blocked {
		if strings.TrimSpace(output.Summary) != "" ||
			len(output.CandidateClaims) != 0 ||
			len(output.Chunks) != 0 ||
			len(output.BlockReasonCodes) == 0 ||
			output.PageCount < 0 || output.PageCount > 10000 {
			return ErrInvalidInput
		}
		return nil
	}
	if !validRequiredProcessText(output.Summary, 8000) ||
		output.PageCount < 1 || output.PageCount > 10000 ||
		len(output.Chunks) == 0 || len(output.Chunks) > 20 ||
		len(output.EvidenceRefs) == 0 ||
		len(output.BlockReasonCodes) != 0 {
		return ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(output.Chunks))
	totalTextBytes := 0
	for _, chunk := range output.Chunks {
		if !validProcessSafeKey(chunk.ChunkKey, 80) ||
			chunk.PageStart < 1 ||
			chunk.PageEnd < chunk.PageStart ||
			chunk.PageEnd > output.PageCount ||
			!validRequiredProcessText(chunk.Text, 4000) ||
			validateProcessEvidenceRefs(chunk.EvidenceRefs, 1, 100) != nil {
			return ErrInvalidInput
		}
		if _, duplicate := seen[chunk.ChunkKey]; duplicate {
			return ErrInvalidInput
		}
		seen[chunk.ChunkKey] = struct{}{}
		totalTextBytes += len(chunk.Text)
		if totalTextBytes > 32000 {
			return ErrInvalidInput
		}
	}
	return nil
}

func validateQualityReviewOutput(output qualityReviewResult) error {
	if !validProcessConfidence(output.OverallScore) ||
		!validProcessConfidence(output.Confidence) ||
		len(output.Scores) == 0 || len(output.Scores) > 20 ||
		len(output.Issues) > 30 ||
		len(output.Coaching) > 20 ||
		validateProcessEvidenceRefs(output.EvidenceRefs, 1, maxProcessEvidenceRefs) != nil ||
		!validProcessSafeKeys(output.ReasonCodes, 0, maxProcessReasonCodes, maxProcessKeyBytes) {
		return ErrInvalidInput
	}
	scoreKeys := make(map[string]struct{}, len(output.Scores))
	for _, score := range output.Scores {
		if !validProcessSafeKey(score.RubricKey, 80) ||
			!validProcessConfidence(score.Score) ||
			validateProcessEvidenceRefs(score.EvidenceRefs, 1, 100) != nil {
			return ErrInvalidInput
		}
		if _, duplicate := scoreKeys[score.RubricKey]; duplicate {
			return ErrInvalidInput
		}
		scoreKeys[score.RubricKey] = struct{}{}
	}
	for _, issue := range output.Issues {
		if !validProcessSafeKey(issue.Code, maxProcessKeyBytes) ||
			!validMode(issue.Severity, "info", "low", "medium", "high", "critical") ||
			!validRequiredProcessText(issue.Description, 1000) ||
			validateProcessEvidenceRefs(issue.EvidenceRefs, 1, 100) != nil {
			return ErrInvalidInput
		}
	}
	coachingKeys := make(map[string]struct{}, len(output.Coaching))
	for _, coaching := range output.Coaching {
		if !validProcessSafeKey(coaching.TopicKey, 80) ||
			!validRequiredProcessText(coaching.Guidance, 2000) ||
			validateProcessEvidenceRefs(coaching.EvidenceRefs, 1, 100) != nil {
			return ErrInvalidInput
		}
		if _, duplicate := coachingKeys[coaching.TopicKey]; duplicate {
			return ErrInvalidInput
		}
		coachingKeys[coaching.TopicKey] = struct{}{}
	}
	return nil
}

func validateProcessCandidateClaims(
	claims []processCandidateClaimResult,
	minimum, maximum int,
) error {
	if len(claims) < minimum || len(claims) > maximum {
		return ErrInvalidInput
	}
	for _, claim := range claims {
		if !validProcessSafeKey(claim.FactKey, maxClaimOutputSchemaLen) ||
			!validFactValueType(claim.ValueType) ||
			(claim.Sensitivity != "" && !validSensitivity(claim.Sensitivity)) ||
			!validTypedClaimValue(claim.ValueType, claim.Value) ||
			!validProcessConfidence(claim.Confidence) ||
			!validProcessUUIDs(claim.EvidenceObservationIDs, 1, maxClaimEvidenceRefs) {
			return ErrInvalidInput
		}
		validFrom, err := parseOptionalProcessTime(claim.ValidFrom)
		if err != nil {
			return ErrInvalidInput
		}
		validUntil, err := parseOptionalProcessTime(claim.ValidUntil)
		if err != nil || (validFrom != nil && validUntil != nil && !validUntil.After(*validFrom)) {
			return ErrInvalidInput
		}
	}
	return nil
}

func validateProcessEvidenceRefs(
	refs []processEvidenceRef,
	minimum, maximum int,
) error {
	if len(refs) < minimum || len(refs) > maximum {
		return ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if !validExactProcessUUID(ref.ObservationID) ||
			ref.SourceKey != strings.TrimSpace(ref.SourceKey) ||
			!validSourceKey(ref.SourceKey) {
			return ErrInvalidInput
		}
		if _, duplicate := seen[ref.ObservationID]; duplicate {
			return ErrInvalidInput
		}
		seen[ref.ObservationID] = struct{}{}
	}
	return nil
}

func validateProcessFactRefs(refs []processFactRef, minimum, maximum int) error {
	if len(refs) < minimum || len(refs) > maximum {
		return ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if !validExactProcessUUID(ref.FactID) ||
			!validProcessSafeKey(ref.FactKey, maxProcessKeyBytes) ||
			ref.Version < 1 {
			return ErrInvalidInput
		}
		if _, duplicate := seen[ref.FactID]; duplicate {
			return ErrInvalidInput
		}
		seen[ref.FactID] = struct{}{}
	}
	return nil
}

func validRequiredProcessText(value string, maximum int) bool {
	return value == strings.TrimSpace(value) && value != "" && len(value) <= maximum
}

func validProcessSafeKey(value string, maximum int) bool {
	return value == strings.TrimSpace(value) &&
		value != "" &&
		len(value) <= maximum &&
		safeKeyPattern.MatchString(value)
}

func validProcessSafeKeys(values []string, minimum, maximum, itemMaximum int) bool {
	if len(values) < minimum || len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validProcessSafeKey(value, itemMaximum) {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validProcessCatalogKeys(
	values []string,
	minimum, maximum int,
	catalog map[string]struct{},
) bool {
	if !validProcessSafeKeys(values, minimum, maximum, maxProcessKeyBytes) {
		return false
	}
	for _, value := range values {
		if _, ok := catalog[value]; !ok {
			return false
		}
	}
	return true
}

func validProcessUUIDs(values []string, minimum, maximum int) bool {
	if len(values) < minimum || len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validExactProcessUUID(value) {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validExactProcessUUID(value string) bool {
	return value == strings.TrimSpace(value) && validUUID(value)
}

func validProcessConfidence(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func processStringSetsOverlap(left, right []string) bool {
	seen := make(map[string]struct{}, len(left))
	for _, value := range left {
		seen[value] = struct{}{}
	}
	for _, value := range right {
		if _, ok := seen[value]; ok {
			return true
		}
	}
	return false
}

func parseOptionalProcessTime(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	if *value != strings.TrimSpace(*value) || *value == "" || len(*value) > 40 {
		return nil, ErrInvalidInput
	}
	parsed, err := time.Parse(time.RFC3339, *value)
	if err != nil {
		parsed, err = time.Parse("2006-01-02", *value)
	}
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseRequiredProcessTimestamp(value string) (time.Time, error) {
	if value != strings.TrimSpace(value) || value == "" || len(value) > 40 {
		return time.Time{}, ErrInvalidInput
	}
	return time.Parse(time.RFC3339, value)
}

func parseRequiredProcessDate(value string) (time.Time, error) {
	if value != strings.TrimSpace(value) || len(value) != len("2006-01-02") {
		return time.Time{}, ErrInvalidInput
	}
	return time.Parse("2006-01-02", value)
}

func parseRequiredProcessDateOrTimestamp(value string) (time.Time, error) {
	parsed, err := parseRequiredProcessTimestamp(value)
	if err == nil {
		return parsed, nil
	}
	return parseRequiredProcessDate(value)
}

func validProcessRawJSONArray(raw json.RawMessage, maximum int) bool {
	return len(bytes.TrimSpace(raw)) > 0 && len(raw) <= maximum && validJSONArray(raw)
}
