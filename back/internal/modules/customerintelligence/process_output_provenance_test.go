package customerintelligence

import (
	"encoding/json"
	"errors"
	"testing"
)

func relationshipProcessEnvelope() ContextEnvelope {
	return ContextEnvelope{
		Observations: []ContextObservation{{
			ID: processObservationOne, SourceKey: "omnichannel",
		}},
		Facts: []Fact{{
			ID: processFactOne, Key: "preferred_name", Version: 1,
			Evidence: []EvidenceRef{{
				ObservationID: processObservationOne,
				SourceKey:     "omnichannel",
			}},
		}},
	}
}

func TestRelationshipProcessProvenanceAcceptsExactContextReferences(t *testing.T) {
	t.Parallel()
	envelope := relationshipProcessEnvelope()
	for _, processKey := range defaultRelationshipRefreshProcesses {
		processKey := processKey
		t.Run(processKey, func(t *testing.T) {
			t.Parallel()
			raw := validNonConversationalProcessOutputs()[processKey]
			if err := validateRelationshipProcessProvenance(
				processKey,
				raw,
				envelope,
			); err != nil {
				t.Fatalf("referencia valida rejeitada: %v", err)
			}
		})
	}
}

func TestRelationshipProcessProvenanceRejectsHallucinatedObservation(t *testing.T) {
	t.Parallel()
	raw := mutateProcessOutput(
		t,
		validNonConversationalProcessOutputs()["recommendation.follow_up"],
		func(output map[string]any) {
			output["evidenceRefs"].([]any)[0].(map[string]any)["observationId"] =
				"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		},
	)
	err := validateRelationshipProcessProvenance(
		"recommendation.follow_up",
		raw,
		relationshipProcessEnvelope(),
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("referencia externa aceita: %v", err)
	}
}

func TestRelationshipProcessProvenanceRejectsWrongSourceForRealObservation(t *testing.T) {
	t.Parallel()
	raw := mutateProcessOutput(
		t,
		validNonConversationalProcessOutputs()["profile.summary"],
		func(output map[string]any) {
			output["evidenceRefs"].([]any)[0].(map[string]any)["sourceKey"] = "erp"
		},
	)
	err := validateRelationshipProcessProvenance(
		"profile.summary",
		raw,
		relationshipProcessEnvelope(),
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("source falsa aceita: %v", err)
	}
}

func TestRelationshipProcessProvenanceRejectsWrongFactVersion(t *testing.T) {
	t.Parallel()
	raw := mutateProcessOutput(
		t,
		validNonConversationalProcessOutputs()["recommendation.offer"],
		func(output map[string]any) {
			output["factRefs"].([]any)[0].(map[string]any)["version"] = float64(2)
		},
	)
	err := validateRelationshipProcessProvenance(
		"recommendation.offer",
		raw,
		relationshipProcessEnvelope(),
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("versao de fato externa aceita: %v", err)
	}
}

func TestRelationshipProcessProvenanceRejectsUnregisteredSuggestedSource(t *testing.T) {
	t.Parallel()
	var output map[string]any
	raw := validNonConversationalProcessOutputs()["source.suggest"]
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatal(err)
	}
	output["suggestions"].([]any)[0].(map[string]any)["sourceKey"] = "external.scraper"
	mutated, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	err = validateRelationshipProcessProvenance(
		"source.suggest",
		mutated,
		relationshipProcessEnvelope(),
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("fonte nao registrada aceita: %v", err)
	}
}
