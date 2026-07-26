package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerdata"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerintelligence"
)

func TestTranslateIntelligenceDecisionPreservesAllProcessRunReferences(t *testing.T) {
	translated := translateIntelligenceDecision(
		customerintelligence.InteractionDecision{
			DecisionID:        "decision-1",
			PipelineVersionID: "pipeline-v1",
			Outcome:           "reply",
			ProcessRuns: []customerintelligence.ProcessRunRef{
				{
					ProcessKey: "conversation.triage", RunID: "run-triage",
					Status: "succeeded", ExecutionMode: "active",
					ProcessDefinitionID:     "definition-triage",
					ProcessConfigVersionID:  "config-triage",
					PromptBindingID:         "binding-triage",
					PlatformPromptVersionID: "platform-triage",
					AgencyPromptVersionID:   "agency-triage",
					ClientPromptVersionID:   "client-triage",
					ProcessPromptVersionID:  "process-triage",
					AgentVersionID:          "agent-triage",
					ModelID:                 "model-triage",
					ContextSnapshotID:       "context-triage",
					OutputSchemaVersion:     "triage-v2",
				},
				{
					ProcessKey: "conversation.reply", RunID: "run-reply",
					Status: "succeeded", ExecutionMode: "active",
					ProcessDefinitionID:     "definition-reply",
					ProcessConfigVersionID:  "config-reply",
					PromptBindingID:         "binding-reply",
					PlatformPromptVersionID: "platform-reply",
					ProcessPromptVersionID:  "process-reply",
					AgentVersionID:          "agent-reply",
					ModelID:                 "model-reply",
					ContextSnapshotID:       "context-reply",
					OutputSchemaVersion:     "reply-v2",
				},
			},
		},
		customerdata.ResolveRelationshipResult{
			SubjectID:      "subject-1",
			RelationshipID: "relationship-1",
		},
	)

	if translated.RunID != "run-reply" {
		t.Fatalf("run final = %q", translated.RunID)
	}
	if len(translated.ProcessRuns) != 2 {
		t.Fatalf("process runs = %d", len(translated.ProcessRuns))
	}
	if translated.ProcessRuns[0].ProcessKey != "conversation.triage" ||
		translated.ProcessRuns[1].ProcessKey != "conversation.reply" {
		t.Fatalf("referencias inesperadas: %#v", translated.ProcessRuns)
	}
	if translated.PipelineVersionID != "pipeline-v1" ||
		!translated.OperationalEffectAllowed ||
		translated.ProcessRuns[0].PromptBindingID != "binding-triage" ||
		translated.ProcessRuns[0].PlatformPromptVersionID != "platform-triage" ||
		translated.ProcessRuns[0].ContextSnapshotID != "context-triage" ||
		translated.ProcessRuns[1].OutputSchemaVersion != "reply-v2" {
		t.Fatalf("referencias completas foram perdidas: %#v", translated)
	}
}

func TestTranslateIntelligenceDecisionNeverAuthorizesShadowOrCanaryEffect(t *testing.T) {
	for _, mode := range []string{"shadow", "canary", ""} {
		t.Run(mode, func(t *testing.T) {
			translated := translateIntelligenceDecision(
				customerintelligence.InteractionDecision{
					DecisionID: "decision-1", Outcome: "reply",
					ProcessRuns: []customerintelligence.ProcessRunRef{{
						ProcessKey: "conversation.reply", RunID: "run-1",
						Status: "succeeded", ExecutionMode: mode,
					}},
				},
				customerdata.ResolveRelationshipResult{
					SubjectID: "subject-1", RelationshipID: "relationship-1",
				},
			)
			if translated.OperationalEffectAllowed {
				t.Fatalf("modo %q autorizou efeito operacional", mode)
			}
		})
	}
}

func TestTranslateIntelligenceDecisionCarriesClaimReferenceWithoutValue(t *testing.T) {
	run := customerintelligence.ProcessRunRef{
		ProcessKey:              "conversation.triage",
		RunID:                   "10101010-1010-4010-8010-101010101010",
		Status:                  "succeeded",
		ExecutionMode:           "active",
		ProcessDefinitionID:     "20202020-2020-4020-8020-202020202020",
		ProcessConfigVersionID:  "30303030-3030-4030-8030-303030303030",
		PromptBindingID:         "40404040-4040-4040-8040-404040404040",
		PlatformPromptVersionID: "50505050-5050-4050-8050-505050505050",
		ProcessPromptVersionID:  "60606060-6060-4060-8060-606060606060",
		AgentVersionID:          "70707070-7070-4070-8070-707070707070",
		ModelID:                 "80808080-8080-4080-8080-808080808080",
		ContextSnapshotID:       "90909090-9090-4090-8090-909090909090",
		OutputSchemaVersion:     "conversation.triage.result.v2",
	}
	translated := translateIntelligenceDecision(
		customerintelligence.InteractionDecision{
			DecisionID: "decision-1", Outcome: "reply",
			ProcessRuns: []customerintelligence.ProcessRunRef{run},
			ExtractedClaims: json.RawMessage(`[{
				"factKey":"profile.preferred_name",
				"valueType":"string",
				"value":"Ana",
				"confidence":0.9,
				"evidenceObservationIds":["aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"],
				"validFrom":null,
				"validUntil":null
			}]`),
		},
		customerdata.ResolveRelationshipResult{
			SubjectID: "subject-1", RelationshipID: "relationship-1",
		},
	)
	if len(translated.CandidateClaims) != 1 {
		t.Fatalf("claim refs = %#v", translated.CandidateClaims)
	}
	encoded, err := json.Marshal(translated.CandidateClaims[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "Ana") ||
		strings.Contains(string(encoded), `"value"`) {
		t.Fatalf("PII da claim vazou para referencia: %s", encoded)
	}
	if _, exists := translated.ExtractedFields["customer_intelligence_claims"]; exists {
		t.Fatal("claim foi duplicada em extracted fields operacional")
	}
}
