package customerintelligence

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const maxProcessOutputBytes = 128 << 10

type processEvidenceRef struct {
	ObservationID string `json:"observationId"`
	SourceKey     string `json:"sourceKey"`
}

type processFactRef struct {
	FactID  string `json:"factId"`
	FactKey string `json:"factKey"`
	Version int    `json:"version"`
}

type processCandidateClaimResult struct {
	FactKey                string          `json:"factKey"`
	ValueType              string          `json:"valueType"`
	Value                  json.RawMessage `json:"value"`
	Confidence             float64         `json:"confidence"`
	Sensitivity            string          `json:"sensitivity,omitempty"`
	EvidenceObservationIDs []string        `json:"evidenceObservationIds"`
	ValidFrom              *string         `json:"validFrom"`
	ValidUntil             *string         `json:"validUntil"`
}

type handoffSummaryResult struct {
	Summary            string               `json:"summary"`
	ReasonCode         string               `json:"reasonCode"`
	CollectedFieldKeys []string             `json:"collectedFieldKeys"`
	PendingFieldKeys   []string             `json:"pendingFieldKeys"`
	RedactionCodes     []string             `json:"redactionCodes"`
	MessageIDs         []string             `json:"messageIds"`
	EvidenceRefs       []processEvidenceRef `json:"evidenceRefs"`
	Confidence         float64              `json:"confidence"`
}

type memoryExtractResult struct {
	Claims []processCandidateClaimResult `json:"claims"`
}

type profileSummarySectionResult struct {
	Key          string               `json:"key"`
	Content      string               `json:"content"`
	EvidenceRefs []processEvidenceRef `json:"evidenceRefs"`
	FactRefs     []processFactRef     `json:"factRefs"`
	Confidence   float64              `json:"confidence"`
}

type profileSummaryResult struct {
	Summary      string                        `json:"summary"`
	Sections     []profileSummarySectionResult `json:"sections"`
	EvidenceRefs []processEvidenceRef          `json:"evidenceRefs"`
	FactRefs     []processFactRef              `json:"factRefs"`
	Confidence   float64                       `json:"confidence"`
}

type followUpConstraintsResult struct {
	ConsentEligible       bool     `json:"consentEligible"`
	ChannelEligible       bool     `json:"channelEligible"`
	QuietHoursSatisfied   bool     `json:"quietHoursSatisfied"`
	FrequencyCapSatisfied bool     `json:"frequencyCapSatisfied"`
	ReasonCodes           []string `json:"reasonCodes"`
}

type followUpRecommendationResult struct {
	RecommendedAt       string                    `json:"recommendedAt"`
	WindowStart         string                    `json:"windowStart"`
	WindowEnd           string                    `json:"windowEnd"`
	SuggestedChannel    string                    `json:"suggestedChannel"`
	CadencePolicyRef    string                    `json:"cadencePolicyRef"`
	ReasonCodes         []string                  `json:"reasonCodes"`
	ConversationBrief   string                    `json:"conversationBrief"`
	EvidenceRefs        []processEvidenceRef      `json:"evidenceRefs"`
	ConstraintsSnapshot followUpConstraintsResult `json:"constraintsSnapshot"`
	Confidence          float64                   `json:"confidence"`
	ExpiresAt           string                    `json:"expiresAt"`
}

type offerCatalogItemResult struct {
	ItemType   string `json:"itemType"`
	ItemID     string `json:"itemId"`
	VersionRef string `json:"versionRef"`
}

type offerRecommendationResult struct {
	CatalogOwnerModule      string                   `json:"catalogOwnerModule"`
	CatalogItems            []offerCatalogItemResult `json:"catalogItems"`
	FitReasonCodes          []string                 `json:"fitReasonCodes"`
	FitNarrative            string                   `json:"fitNarrative"`
	ExcludedItemReasonCodes []string                 `json:"excludedItemReasonCodes"`
	PriceContextRef         *string                  `json:"priceContextRef"`
	ValidityCheckedAt       string                   `json:"validityCheckedAt"`
	EvidenceRefs            []processEvidenceRef     `json:"evidenceRefs"`
	FactRefs                []processFactRef         `json:"factRefs"`
	Confidence              float64                  `json:"confidence"`
	ExpiresAt               string                   `json:"expiresAt"`
}

type recommendationDateWindowResult struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type importantDateRecommendationResult struct {
	DateFactID        string                         `json:"dateFactId"`
	DateFactVersion   int                            `json:"dateFactVersion"`
	DateValue         string                         `json:"dateValue"`
	DateKind          string                         `json:"dateKind"`
	Recurrence        string                         `json:"recurrence"`
	VerificationState string                         `json:"verificationState"`
	SuggestedWindow   recommendationDateWindowResult `json:"suggestedWindow"`
	ReasonCodes       []string                       `json:"reasonCodes"`
	EvidenceRefs      []processEvidenceRef           `json:"evidenceRefs"`
	RequiresReview    bool                           `json:"requiresReview"`
	Confidence        float64                        `json:"confidence"`
	ExpiresAt         string                         `json:"expiresAt"`
}

type sourceSuggestionItemResult struct {
	SourceKey     string               `json:"sourceKey"`
	GapCodes      []string             `json:"gapCodes"`
	RationaleCode string               `json:"rationaleCode"`
	Rationale     string               `json:"rationale"`
	EvidenceRefs  []processEvidenceRef `json:"evidenceRefs"`
	Confidence    float64              `json:"confidence"`
	ExpiresAt     string               `json:"expiresAt"`
}

type sourceSuggestionResult struct {
	Suggestions []sourceSuggestionItemResult `json:"suggestions"`
}

type portfolioPeriodResult struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type portfolioOpportunityResult struct {
	OpportunityType        string                `json:"opportunityType"`
	TargetClientAccountIDs []string              `json:"targetClientAccountIds"`
	PurposeKey             string                `json:"purposeKey"`
	AggregateSnapshotID    string                `json:"aggregateSnapshotId"`
	DatasetKeys            []string              `json:"datasetKeys"`
	SourceKeys             []string              `json:"sourceKeys"`
	DimensionKeys          []string              `json:"dimensionKeys"`
	MetricKeys             []string              `json:"metricKeys"`
	Period                 portfolioPeriodResult `json:"period"`
	CohortClass            string                `json:"cohortClass"`
	CohortSize             int                   `json:"cohortSize"`
	SuppressionThreshold   int                   `json:"suppressionThreshold"`
	SuppressionApplied     bool                  `json:"suppressionApplied"`
	SuppressionReasonCodes []string              `json:"suppressionReasonCodes"`
	Rationale              string                `json:"rationale"`
	ReasonCodes            []string              `json:"reasonCodes"`
	CampaignBrief          string                `json:"campaignBrief"`
	PolicyVersionRefs      []string              `json:"policyVersionRefs"`
	Confidence             float64               `json:"confidence"`
	ValidFrom              string                `json:"validFrom"`
	ExpiresAt              string                `json:"expiresAt"`
}

type mediaImageAnalysisResult struct {
	Description      string                        `json:"description"`
	CandidateClaims  []processCandidateClaimResult `json:"candidateClaims"`
	EvidenceRefs     []processEvidenceRef          `json:"evidenceRefs"`
	SafetyFlags      []string                      `json:"safetyFlags"`
	Blocked          bool                          `json:"blocked"`
	BlockReasonCodes []string                      `json:"blockReasonCodes"`
	Confidence       float64                       `json:"confidence"`
}

type mediaDocumentChunkResult struct {
	ChunkKey     string               `json:"chunkKey"`
	PageStart    int                  `json:"pageStart"`
	PageEnd      int                  `json:"pageEnd"`
	Text         string               `json:"text"`
	EvidenceRefs []processEvidenceRef `json:"evidenceRefs"`
}

type mediaDocumentAnalysisResult struct {
	Summary          string                        `json:"summary"`
	PageCount        int                           `json:"pageCount"`
	CandidateClaims  []processCandidateClaimResult `json:"candidateClaims"`
	Chunks           []mediaDocumentChunkResult    `json:"chunks"`
	EvidenceRefs     []processEvidenceRef          `json:"evidenceRefs"`
	SafetyFlags      []string                      `json:"safetyFlags"`
	Blocked          bool                          `json:"blocked"`
	BlockReasonCodes []string                      `json:"blockReasonCodes"`
	Confidence       float64                       `json:"confidence"`
}

type qualityScoreResult struct {
	RubricKey    string               `json:"rubricKey"`
	Score        float64              `json:"score"`
	EvidenceRefs []processEvidenceRef `json:"evidenceRefs"`
}

type qualityIssueResult struct {
	Code         string               `json:"code"`
	Severity     string               `json:"severity"`
	Description  string               `json:"description"`
	EvidenceRefs []processEvidenceRef `json:"evidenceRefs"`
}

type qualityCoachingResult struct {
	TopicKey     string               `json:"topicKey"`
	Guidance     string               `json:"guidance"`
	EvidenceRefs []processEvidenceRef `json:"evidenceRefs"`
}

type qualityReviewResult struct {
	OverallScore float64                 `json:"overallScore"`
	Scores       []qualityScoreResult    `json:"scores"`
	Issues       []qualityIssueResult    `json:"issues"`
	Coaching     []qualityCoachingResult `json:"coaching"`
	EvidenceRefs []processEvidenceRef    `json:"evidenceRefs"`
	ReasonCodes  []string                `json:"reasonCodes"`
	Confidence   float64                 `json:"confidence"`
}

func validateTypedProcessOutput(processKey string, raw json.RawMessage) error {
	if len(raw) == 0 || len(raw) > maxProcessOutputBytes {
		return ErrInvalidInput
	}
	switch processKey {
	case "conversation.triage":
		var output triageResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		return validateTriageProcessOutput(output)
	case "conversation.reply":
		var output replyResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		return validateReplyProcessOutput(output)
	case "conversation.handoff_summary":
		var output handoffSummaryResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		return validateHandoffSummaryOutput(output)
	case "memory.extract":
		var output memoryExtractResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		return validateMemoryExtractOutput(output)
	case "profile.summary":
		var output profileSummaryResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		return validateProfileSummaryOutput(output)
	case "recommendation.follow_up":
		var output followUpRecommendationResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		return validateFollowUpRecommendationOutput(output)
	case "recommendation.offer":
		var output offerRecommendationResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		return validateOfferRecommendationOutput(output)
	case "recommendation.important_dates":
		var output importantDateRecommendationResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		return validateImportantDateRecommendationOutput(output)
	case "source.suggest":
		var output sourceSuggestionResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		return validateSourceSuggestionOutput(output)
	case "portfolio.opportunity":
		var output portfolioOpportunityResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		return validatePortfolioOpportunityOutput(output)
	case "media.image_analysis":
		var output mediaImageAnalysisResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		return validateMediaImageAnalysisOutput(output)
	case "media.document_analysis":
		var output mediaDocumentAnalysisResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		return validateMediaDocumentAnalysisOutput(output)
	case "quality.review":
		var output qualityReviewResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		return validateQualityReviewOutput(output)
	default:
		return ErrInvalidInput
	}
}

func decodeStrictProcessOutput(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return ErrInvalidInput
		}
		return err
	}
	return nil
}
