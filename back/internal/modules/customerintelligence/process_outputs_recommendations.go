package customerintelligence

import (
	"strings"
)

var followUpChannelCatalog = map[string]struct{}{
	"email":     {},
	"instagram": {},
	"phone":     {},
	"sms":       {},
	"webchat":   {},
	"whatsapp":  {},
}

func validateFollowUpRecommendationOutput(output followUpRecommendationResult) error {
	recommendedAt, err := parseRequiredProcessTimestamp(output.RecommendedAt)
	if err != nil {
		return ErrInvalidInput
	}
	windowStart, err := parseRequiredProcessTimestamp(output.WindowStart)
	if err != nil {
		return ErrInvalidInput
	}
	windowEnd, err := parseRequiredProcessTimestamp(output.WindowEnd)
	if err != nil {
		return ErrInvalidInput
	}
	expiresAt, err := parseRequiredProcessTimestamp(output.ExpiresAt)
	if err != nil {
		return ErrInvalidInput
	}
	if windowEnd.Before(windowStart) ||
		recommendedAt.Before(windowStart) ||
		recommendedAt.After(windowEnd) ||
		!expiresAt.After(recommendedAt) ||
		!validProcessCatalogKeys(
			[]string{output.SuggestedChannel}, 1, 1, followUpChannelCatalog,
		) ||
		!validExactProcessUUID(output.CadencePolicyRef) ||
		!validProcessSafeKeys(
			output.ReasonCodes, 1, maxProcessReasonCodes, maxProcessKeyBytes,
		) ||
		!validRequiredProcessText(output.ConversationBrief, 1000) ||
		validateProcessEvidenceRefs(output.EvidenceRefs, 1, 100) != nil ||
		!validProcessConfidence(output.Confidence) ||
		!validProcessSafeKeys(
			output.ConstraintsSnapshot.ReasonCodes,
			0,
			maxProcessReasonCodes,
			maxProcessKeyBytes,
		) {
		return ErrInvalidInput
	}
	constraints := output.ConstraintsSnapshot
	if (!constraints.ConsentEligible ||
		!constraints.ChannelEligible ||
		!constraints.QuietHoursSatisfied ||
		!constraints.FrequencyCapSatisfied) &&
		len(constraints.ReasonCodes) == 0 {
		return ErrInvalidInput
	}
	return nil
}

func validateOfferRecommendationOutput(output offerRecommendationResult) error {
	checkedAt, err := parseRequiredProcessTimestamp(output.ValidityCheckedAt)
	if err != nil {
		return ErrInvalidInput
	}
	expiresAt, err := parseRequiredProcessTimestamp(output.ExpiresAt)
	if err != nil {
		return ErrInvalidInput
	}
	if !expiresAt.After(checkedAt) ||
		!validProcessSafeKey(output.CatalogOwnerModule, 80) ||
		len(output.CatalogItems) == 0 || len(output.CatalogItems) > 20 ||
		!validProcessSafeKeys(
			output.FitReasonCodes, 1, maxProcessReasonCodes, maxProcessKeyBytes,
		) ||
		!validRequiredProcessText(output.FitNarrative, 2000) ||
		!validProcessSafeKeys(
			output.ExcludedItemReasonCodes,
			0,
			maxProcessReasonCodes,
			maxProcessKeyBytes,
		) ||
		!validOptionalExactProcessUUID(output.PriceContextRef) ||
		validateProcessEvidenceRefs(output.EvidenceRefs, 1, 100) != nil ||
		validateProcessFactRefs(output.FactRefs, 1, 100) != nil ||
		!validProcessConfidence(output.Confidence) {
		return ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(output.CatalogItems))
	for _, item := range output.CatalogItems {
		if !validProcessSafeKey(item.ItemType, 80) ||
			!validExactProcessUUID(item.ItemID) ||
			!validExactProcessUUID(item.VersionRef) {
			return ErrInvalidInput
		}
		key := item.ItemType + ":" + item.ItemID + ":" + item.VersionRef
		if _, duplicate := seen[key]; duplicate {
			return ErrInvalidInput
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateImportantDateRecommendationOutput(
	output importantDateRecommendationResult,
) error {
	if _, err := parseRequiredProcessDate(output.DateValue); err != nil {
		return ErrInvalidInput
	}
	windowStart, err := parseRequiredProcessDate(output.SuggestedWindow.Start)
	if err != nil {
		return ErrInvalidInput
	}
	windowEnd, err := parseRequiredProcessDate(output.SuggestedWindow.End)
	if err != nil {
		return ErrInvalidInput
	}
	if _, err = parseRequiredProcessTimestamp(output.ExpiresAt); err != nil {
		return ErrInvalidInput
	}
	if !validExactProcessUUID(output.DateFactID) ||
		output.DateFactVersion < 1 ||
		!validProcessSafeKey(output.DateKind, 80) ||
		!validMode(output.Recurrence, "none", "monthly", "yearly") ||
		!validMode(output.VerificationState, "verified", "resolved", "contested") ||
		windowEnd.Before(windowStart) ||
		!validProcessSafeKeys(
			output.ReasonCodes, 1, maxProcessReasonCodes, maxProcessKeyBytes,
		) ||
		validateProcessEvidenceRefs(output.EvidenceRefs, 1, 100) != nil ||
		!validProcessConfidence(output.Confidence) ||
		(output.VerificationState == "contested" && !output.RequiresReview) {
		return ErrInvalidInput
	}
	return nil
}

func validateSourceSuggestionOutput(output sourceSuggestionResult) error {
	if len(output.Suggestions) > 10 {
		return ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(output.Suggestions))
	for _, suggestion := range output.Suggestions {
		if suggestion.SourceKey != strings.TrimSpace(suggestion.SourceKey) ||
			!validSourceKey(suggestion.SourceKey) ||
			!validProcessSafeKeys(
				suggestion.GapCodes, 1, maxProcessReasonCodes, maxProcessKeyBytes,
			) ||
			!validProcessSafeKey(suggestion.RationaleCode, maxProcessKeyBytes) ||
			!validRequiredProcessText(suggestion.Rationale, 1000) ||
			validateProcessEvidenceRefs(suggestion.EvidenceRefs, 0, 100) != nil ||
			!validProcessConfidence(suggestion.Confidence) {
			return ErrInvalidInput
		}
		if _, err := parseRequiredProcessTimestamp(suggestion.ExpiresAt); err != nil {
			return ErrInvalidInput
		}
		if _, duplicate := seen[suggestion.SourceKey]; duplicate {
			return ErrInvalidInput
		}
		seen[suggestion.SourceKey] = struct{}{}
	}
	return nil
}

func validatePortfolioOpportunityOutput(output portfolioOpportunityResult) error {
	periodStart, err := parseRequiredProcessDateOrTimestamp(output.Period.Start)
	if err != nil {
		return ErrInvalidInput
	}
	periodEnd, err := parseRequiredProcessDateOrTimestamp(output.Period.End)
	if err != nil {
		return ErrInvalidInput
	}
	validFrom, err := parseRequiredProcessTimestamp(output.ValidFrom)
	if err != nil {
		return ErrInvalidInput
	}
	expiresAt, err := parseRequiredProcessTimestamp(output.ExpiresAt)
	if err != nil {
		return ErrInvalidInput
	}
	if !validProcessSafeKey(output.OpportunityType, 80) ||
		!validProcessUUIDs(output.TargetClientAccountIDs, 1, 20) ||
		output.PurposeKey != "portfolio_analysis" ||
		!validExactProcessUUID(output.AggregateSnapshotID) ||
		!validProcessSafeKeys(output.DatasetKeys, 1, 20, maxProcessKeyBytes) ||
		!validPortfolioSourceKeys(output.SourceKeys) ||
		!validProcessSafeKeys(output.DimensionKeys, 1, 20, maxProcessKeyBytes) ||
		!validProcessSafeKeys(output.MetricKeys, 1, 20, maxProcessKeyBytes) ||
		periodEnd.Before(periodStart) ||
		output.SuppressionThreshold < minPortfolioCohort ||
		output.SuppressionThreshold > 1_000_000_000 ||
		output.CohortSize < output.SuppressionThreshold ||
		output.CohortSize > 1_000_000_000 ||
		output.CohortClass != portfolioCohortClass(output.CohortSize) ||
		!validProcessSafeKeys(
			output.SuppressionReasonCodes,
			0,
			maxProcessReasonCodes,
			maxProcessKeyBytes,
		) ||
		(output.SuppressionApplied && len(output.SuppressionReasonCodes) == 0) ||
		(!output.SuppressionApplied && len(output.SuppressionReasonCodes) != 0) ||
		!validRequiredProcessText(output.Rationale, 2000) ||
		!validProcessSafeKeys(
			output.ReasonCodes, 1, maxProcessReasonCodes, maxProcessKeyBytes,
		) ||
		!validRequiredProcessText(output.CampaignBrief, 2000) ||
		!validProcessUUIDs(output.PolicyVersionRefs, 1, 20) ||
		!validProcessConfidence(output.Confidence) ||
		!expiresAt.After(validFrom) {
		return ErrInvalidInput
	}
	return nil
}

func validPortfolioSourceKeys(values []string) bool {
	if len(values) == 0 || len(values) > 20 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != strings.TrimSpace(value) || !validSourceKey(value) {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func portfolioCohortClass(size int) string {
	switch {
	case size >= 100:
		return "100_plus"
	case size >= 50:
		return "50_99"
	case size >= 25:
		return "25_49"
	case size >= minPortfolioCohort:
		return "10_24"
	default:
		return ""
	}
}

func validOptionalExactProcessUUID(value *string) bool {
	return value == nil || validExactProcessUUID(*value)
}
