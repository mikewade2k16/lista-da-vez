package customerintelligence

import (
	"encoding/json"
	"strconv"
)

// validateRelationshipProcessProvenance binds model-supplied references to the
// exact server-built context. UUID shape alone is insufficient: an LLM must
// never be able to cite a real object from another context/tenant by guessing
// its identifier.
func validateRelationshipProcessProvenance(
	processKey string,
	raw json.RawMessage,
	envelope ContextEnvelope,
) error {
	observations := make(map[string]struct{})
	facts := make(map[string]struct{})
	factVersions := make(map[string]struct{})
	for _, observation := range envelope.Observations {
		observations[observationRefKey(observation.ID, observation.SourceKey)] = struct{}{}
	}
	for _, fact := range envelope.Facts {
		facts[factRefKey(fact.ID, fact.Key, fact.Version)] = struct{}{}
		factVersions[factVersionRefKey(fact.ID, fact.Version)] = struct{}{}
		for _, evidence := range fact.Evidence {
			observations[observationRefKey(evidence.ObservationID, evidence.SourceKey)] = struct{}{}
		}
	}

	validateEvidence := func(refs []processEvidenceRef) error {
		for _, ref := range refs {
			if _, ok := observations[observationRefKey(ref.ObservationID, ref.SourceKey)]; !ok {
				return ErrInvalidInput
			}
		}
		return nil
	}
	validateFacts := func(refs []processFactRef) error {
		for _, ref := range refs {
			if _, ok := facts[factRefKey(ref.FactID, ref.FactKey, ref.Version)]; !ok {
				return ErrInvalidInput
			}
		}
		return nil
	}

	switch processKey {
	case "profile.summary":
		var output profileSummaryResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		if err := validateEvidence(output.EvidenceRefs); err != nil {
			return err
		}
		if err := validateFacts(output.FactRefs); err != nil {
			return err
		}
		for _, section := range output.Sections {
			if err := validateEvidence(section.EvidenceRefs); err != nil {
				return err
			}
			if err := validateFacts(section.FactRefs); err != nil {
				return err
			}
		}
	case "recommendation.follow_up":
		var output followUpRecommendationResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		return validateEvidence(output.EvidenceRefs)
	case "recommendation.offer":
		var output offerRecommendationResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		if err := validateEvidence(output.EvidenceRefs); err != nil {
			return err
		}
		return validateFacts(output.FactRefs)
	case "recommendation.important_dates":
		var output importantDateRecommendationResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		key := factVersionRefKey(output.DateFactID, output.DateFactVersion)
		if _, ok := factVersions[key]; !ok {
			return ErrInvalidInput
		}
		return validateEvidence(output.EvidenceRefs)
	case "source.suggest":
		var output sourceSuggestionResult
		if err := decodeStrictProcessOutput(raw, &output); err != nil {
			return err
		}
		for _, suggestion := range output.Suggestions {
			if !validSourceKey(suggestion.SourceKey) {
				return ErrInvalidInput
			}
			if err := validateEvidence(suggestion.EvidenceRefs); err != nil {
				return err
			}
		}
	}
	return nil
}

func observationRefKey(id, sourceKey string) string {
	return id + "\x00" + sourceKey
}

func factRefKey(id, factKey string, version int) string {
	return id + "\x00" + factKey + "\x00" + strconv.Itoa(version)
}

func factVersionRefKey(id string, version int) string {
	return id + "\x00" + strconv.Itoa(version)
}
