package customerintelligence

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const (
	processObservationOne = "11111111-1111-4111-8111-111111111111"
	processObservationTwo = "22222222-2222-4222-8222-222222222222"
	processFactOne        = "33333333-3333-4333-8333-333333333333"
	processPolicyOne      = "44444444-4444-4444-8444-444444444444"
	processCatalogItem    = "55555555-5555-4555-8555-555555555555"
	processCatalogVersion = "66666666-6666-4666-8666-666666666666"
	processTargetClient   = "77777777-7777-4777-8777-777777777777"
	processSnapshot       = "88888888-8888-4888-8888-888888888888"
	processPolicyTwo      = "99999999-9999-4999-8999-999999999999"
	processMessageOne     = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

func TestValidateTypedProcessOutputAcceptsNonConversationalContracts(t *testing.T) {
	t.Parallel()
	for processKey, raw := range validNonConversationalProcessOutputs() {
		processKey, raw := processKey, raw
		t.Run(processKey, func(t *testing.T) {
			t.Parallel()
			if err := validateTypedProcessOutput(processKey, raw); err != nil {
				t.Fatalf("output valido rejeitado: %v\n%s", err, raw)
			}
		})
	}
}

func TestValidateTypedProcessOutputRejectsUnknownFieldsForEveryContract(t *testing.T) {
	t.Parallel()
	for processKey, raw := range validNonConversationalProcessOutputs() {
		processKey, raw := processKey, raw
		t.Run(processKey, func(t *testing.T) {
			t.Parallel()
			mutated := mutateProcessOutput(t, raw, func(output map[string]any) {
				output["unexpected"] = "must_fail_closed"
			})
			if err := validateTypedProcessOutput(processKey, mutated); err == nil {
				t.Fatalf("campo desconhecido aceito: %s", mutated)
			}
		})
	}
}

func TestValidateTypedProcessOutputRejectsNestedUnknownField(t *testing.T) {
	t.Parallel()
	raw := validNonConversationalProcessOutputs()["profile.summary"]
	mutated := mutateProcessOutput(t, raw, func(output map[string]any) {
		sections := output["sections"].([]any)
		sections[0].(map[string]any)["unexpected"] = true
	})
	if err := validateTypedProcessOutput("profile.summary", mutated); err == nil {
		t.Fatalf("campo desconhecido aninhado aceito: %s", mutated)
	}
}

func TestValidateTypedProcessOutputFailsClosedForUnknownProcessAndTrailingJSON(t *testing.T) {
	t.Parallel()
	fixtures := validNonConversationalProcessOutputs()
	if err := validateTypedProcessOutput("unknown.process", fixtures["memory.extract"]); err == nil {
		t.Fatal("processo desconhecido aceito")
	}
	trailing := append(
		append(json.RawMessage(nil), fixtures["memory.extract"]...),
		[]byte(`{"claims":[]}`)...,
	)
	if err := validateTypedProcessOutput("memory.extract", trailing); err == nil {
		t.Fatal("JSON concatenado aceito")
	}
}

func TestValidateTypedProcessOutputAcceptsAuditablePortfolioSuppression(t *testing.T) {
	t.Parallel()
	raw := mutateProcessOutput(
		t,
		validNonConversationalProcessOutputs()["portfolio.opportunity"],
		func(output map[string]any) {
			output["suppressionApplied"] = true
			output["suppressionReasonCodes"] = []any{"rare_dimension_bucketed"}
		},
	)
	if err := validateTypedProcessOutput("portfolio.opportunity", raw); err != nil {
		t.Fatalf("supressao auditavel rejeitada: %v\n%s", err, raw)
	}
}

func TestValidateTypedProcessOutputRejectsInvalidCatalogsAndReferences(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		processKey string
		mutate     func(map[string]any)
	}{
		{
			name: "evidence source outside catalog", processKey: "profile.summary",
			mutate: func(output map[string]any) {
				output["evidenceRefs"].([]any)[0].(map[string]any)["sourceKey"] = "external.sql"
			},
		},
		{
			name: "follow up channel outside catalog", processKey: "recommendation.follow_up",
			mutate: func(output map[string]any) {
				output["suggestedChannel"] = "telegram"
			},
		},
		{
			name: "source suggestion outside catalog", processKey: "source.suggest",
			mutate: func(output map[string]any) {
				output["suggestions"].([]any)[0].(map[string]any)["sourceKey"] = "external.sql"
			},
		},
		{
			name: "candidate value type outside catalog", processKey: "memory.extract",
			mutate: func(output map[string]any) {
				output["claims"].([]any)[0].(map[string]any)["valueType"] = "currency"
			},
		},
		{
			name:       "important date recurrence outside catalog",
			processKey: "recommendation.important_dates",
			mutate: func(output map[string]any) {
				output["recurrence"] = "weekly"
			},
		},
		{
			name: "media safety flag outside catalog", processKey: "media.image_analysis",
			mutate: func(output map[string]any) {
				output["safetyFlags"] = []any{"biometric_data"}
			},
		},
		{
			name: "quality severity outside catalog", processKey: "quality.review",
			mutate: func(output map[string]any) {
				output["issues"].([]any)[0].(map[string]any)["severity"] = "urgent"
			},
		},
		{
			name: "invalid observation uuid", processKey: "conversation.handoff_summary",
			mutate: func(output map[string]any) {
				output["evidenceRefs"].([]any)[0].(map[string]any)["observationId"] = "not-a-uuid"
			},
		},
		{
			name: "invalid catalog item uuid", processKey: "recommendation.offer",
			mutate: func(output map[string]any) {
				output["catalogItems"].([]any)[0].(map[string]any)["itemId"] = "sku-123"
			},
		},
		{
			name: "invalid reason code", processKey: "recommendation.offer",
			mutate: func(output map[string]any) {
				output["fitReasonCodes"] = []any{"not valid"}
			},
		},
	}
	fixtures := validNonConversationalProcessOutputs()
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			raw := mutateProcessOutput(t, fixtures[test.processKey], test.mutate)
			if err := validateTypedProcessOutput(test.processKey, raw); err == nil {
				t.Fatalf("catalogo/referencia invalida aceita: %s", raw)
			}
		})
	}
}

func TestValidateTypedProcessOutputRejectsLimitsAndUnsafeRelationships(t *testing.T) {
	t.Parallel()
	fixtures := validNonConversationalProcessOutputs()
	tests := []struct {
		name       string
		processKey string
		mutate     func(map[string]any)
	}{
		{
			name: "handoff summary too long", processKey: "conversation.handoff_summary",
			mutate: func(output map[string]any) {
				output["summary"] = strings.Repeat("x", 4001)
			},
		},
		{
			name: "confidence above one", processKey: "media.image_analysis",
			mutate: func(output map[string]any) {
				output["confidence"] = 1.01
			},
		},
		{
			name: "memory claim list above limit", processKey: "memory.extract",
			mutate: func(output map[string]any) {
				claim := output["claims"].([]any)[0]
				claims := make([]any, maxProcessCandidateClaims+1)
				for index := range claims {
					claims[index] = claim
				}
				output["claims"] = claims
			},
		},
		{
			name: "profile section list above limit", processKey: "profile.summary",
			mutate: func(output map[string]any) {
				section := output["sections"].([]any)[0]
				sections := make([]any, 13)
				for index := range sections {
					sections[index] = section
				}
				output["sections"] = sections
			},
		},
		{
			name: "source suggestion list above limit", processKey: "source.suggest",
			mutate: func(output map[string]any) {
				suggestion := output["suggestions"].([]any)[0]
				suggestions := make([]any, 11)
				for index := range suggestions {
					suggestions[index] = suggestion
				}
				output["suggestions"] = suggestions
			},
		},
		{
			name: "follow up window inverted", processKey: "recommendation.follow_up",
			mutate: func(output map[string]any) {
				output["windowStart"] = "2026-08-10T15:00:00Z"
				output["windowEnd"] = "2026-08-10T13:00:00Z"
			},
		},
		{
			name:       "contested date without review",
			processKey: "recommendation.important_dates",
			mutate: func(output map[string]any) {
				output["verificationState"] = "contested"
				output["requiresReview"] = false
			},
		},
		{
			name: "portfolio below suppression threshold", processKey: "portfolio.opportunity",
			mutate: func(output map[string]any) {
				output["cohortSize"] = float64(9)
				output["cohortClass"] = "10_24"
			},
		},
		{
			name: "portfolio cohort class mismatch", processKey: "portfolio.opportunity",
			mutate: func(output map[string]any) {
				output["cohortClass"] = "100_plus"
			},
		},
		{
			name: "portfolio suppression without reason", processKey: "portfolio.opportunity",
			mutate: func(output map[string]any) {
				output["suppressionApplied"] = true
				output["suppressionReasonCodes"] = []any{}
			},
		},
		{
			name: "document chunk outside page range", processKey: "media.document_analysis",
			mutate: func(output map[string]any) {
				output["chunks"].([]any)[0].(map[string]any)["pageEnd"] = float64(3)
			},
		},
		{
			name: "duplicate quality rubric", processKey: "quality.review",
			mutate: func(output map[string]any) {
				first := output["scores"].([]any)[0]
				output["scores"] = append(output["scores"].([]any), first)
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			raw := mutateProcessOutput(t, fixtures[test.processKey], test.mutate)
			if err := validateTypedProcessOutput(test.processKey, raw); err == nil {
				t.Fatalf("limite/relacao insegura aceita: %s", raw)
			}
		})
	}

	oversized := bytes.Repeat([]byte("x"), maxProcessOutputBytes+1)
	if err := validateTypedProcessOutput("profile.summary", oversized); err == nil {
		t.Fatal("output acima de 128 KiB aceito")
	}
}

func mutateProcessOutput(
	t *testing.T,
	raw json.RawMessage,
	mutate func(map[string]any),
) json.RawMessage {
	t.Helper()
	var output map[string]any
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatal(err)
	}
	mutate(output)
	mutated, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	return mutated
}

func validNonConversationalProcessOutputs() map[string]json.RawMessage {
	evidence := `{"observationId":"` + processObservationOne + `","sourceKey":"omnichannel"}`
	fact := `{"factId":"` + processFactOne + `","factKey":"preferred_name","version":1}`
	claim := `{"factKey":"preferred_name","valueType":"string","value":"Ana","confidence":0.83,` +
		`"sensitivity":"personal","evidenceObservationIds":["` + processObservationOne + `"],` +
		`"validFrom":null,"validUntil":null}`
	return map[string]json.RawMessage{
		"conversation.handoff_summary": json.RawMessage(
			`{"summary":"Cliente pediu atendimento humano.","reasonCode":"customer_requested",` +
				`"collectedFieldKeys":["preferred_name"],"pendingFieldKeys":["appointment_date"],` +
				`"redactionCodes":[],"messageIds":["` + processMessageOne + `"],` +
				`"evidenceRefs":[` + evidence + `],"confidence":0.91}`,
		),
		"memory.extract": json.RawMessage(`{"claims":[` + claim + `]}`),
		"profile.summary": json.RawMessage(
			`{"summary":"Ana prefere contato por WhatsApp.",` +
				`"sections":[{"key":"communication","content":"Prefere WhatsApp.",` +
				`"evidenceRefs":[` + evidence + `],"factRefs":[` + fact + `],"confidence":0.88}],` +
				`"evidenceRefs":[` + evidence + `],"factRefs":[` + fact + `],"confidence":0.87}`,
		),
		"recommendation.follow_up": json.RawMessage(
			`{"recommendedAt":"2026-08-10T14:00:00-03:00",` +
				`"windowStart":"2026-08-10T13:00:00-03:00",` +
				`"windowEnd":"2026-08-10T15:00:00-03:00","suggestedChannel":"whatsapp",` +
				`"cadencePolicyRef":"` + processPolicyOne + `","reasonCodes":["awaiting_decision"],` +
				`"conversationBrief":"Retomar a proposta solicitada.",` +
				`"evidenceRefs":[` + evidence + `],` +
				`"constraintsSnapshot":{"consentEligible":true,"channelEligible":true,` +
				`"quietHoursSatisfied":true,"frequencyCapSatisfied":true,"reasonCodes":[]},` +
				`"confidence":0.84,"expiresAt":"2026-08-11T14:00:00-03:00"}`,
		),
		"recommendation.offer": json.RawMessage(
			`{"catalogOwnerModule":"site","catalogItems":[{"itemType":"service",` +
				`"itemId":"` + processCatalogItem + `","versionRef":"` + processCatalogVersion + `"}],` +
				`"fitReasonCodes":["stated_interest"],"fitNarrative":"Servico alinhado ao interesse.",` +
				`"excludedItemReasonCodes":[],"priceContextRef":null,` +
				`"validityCheckedAt":"2026-08-01T12:00:00Z","evidenceRefs":[` + evidence + `],` +
				`"factRefs":[` + fact + `],"confidence":0.82,"expiresAt":"2026-08-08T12:00:00Z"}`,
		),
		"recommendation.important_dates": json.RawMessage(
			`{"dateFactId":"` + processFactOne + `","dateFactVersion":1,` +
				`"dateValue":"2026-08-20","dateKind":"birthday","recurrence":"yearly",` +
				`"verificationState":"verified","suggestedWindow":{"start":"2026-08-18","end":"2026-08-20"},` +
				`"reasonCodes":["relationship_milestone"],"evidenceRefs":[` + evidence + `],` +
				`"requiresReview":false,"confidence":0.9,"expiresAt":"2026-08-21T00:00:00Z"}`,
		),
		"source.suggest": json.RawMessage(
			`{"suggestions":[{"sourceKey":"erp","gapCodes":["purchase_history_missing"],` +
				`"rationaleCode":"profile_gap","rationale":"Historico ajudaria a qualificar ofertas.",` +
				`"evidenceRefs":[],"confidence":0.72,"expiresAt":"2026-09-01T00:00:00Z"}]}`,
		),
		"portfolio.opportunity": json.RawMessage(
			`{"opportunityType":"cross_sell_campaign","targetClientAccountIds":["` +
				processTargetClient + `"],"purposeKey":"portfolio_analysis",` +
				`"aggregateSnapshotId":"` + processSnapshot + `","datasetKeys":["sales.aggregate"],` +
				`"sourceKeys":["bi.perola"],"dimensionKeys":["service_category"],` +
				`"metricKeys":["conversion_rate"],"period":{"start":"2026-01-01","end":"2026-06-30"},` +
				`"cohortClass":"25_49","cohortSize":25,"suppressionThreshold":10,` +
				`"suppressionApplied":false,"suppressionReasonCodes":[],` +
				`"rationale":"Coorte agregada demonstra afinidade.","reasonCodes":["aggregate_affinity"],` +
				`"campaignBrief":"Validar campanha segmentada sem exportar contribuidores.",` +
				`"policyVersionRefs":["` + processPolicyTwo + `"],"confidence":0.78,` +
				`"validFrom":"2026-07-01T00:00:00Z","expiresAt":"2026-08-01T00:00:00Z"}`,
		),
		"media.image_analysis": json.RawMessage(
			`{"description":"Imagem mostra o produto solicitado.","candidateClaims":[],` +
				`"evidenceRefs":[` + evidence + `],"safetyFlags":[],"blocked":false,` +
				`"blockReasonCodes":[],"confidence":0.86}`,
		),
		"media.document_analysis": json.RawMessage(
			`{"summary":"Documento de uma pagina com preferencia declarada.","pageCount":1,` +
				`"candidateClaims":[` + claim + `],"chunks":[{"chunkKey":"page_1","pageStart":1,` +
				`"pageEnd":1,"text":"Nome preferido: Ana.","evidenceRefs":[` + evidence + `]}],` +
				`"evidenceRefs":[` + evidence + `],"safetyFlags":["personal_data"],` +
				`"blocked":false,"blockReasonCodes":[],"confidence":0.89}`,
		),
		"quality.review": json.RawMessage(
			`{"overallScore":0.82,"scores":[{"rubricKey":"clarity","score":0.9,` +
				`"evidenceRefs":[` + evidence + `]}],"issues":[{"code":"missing_confirmation",` +
				`"severity":"low","description":"Confirmacao final nao foi registrada.",` +
				`"evidenceRefs":[` + evidence + `]}],"coaching":[{"topicKey":"confirmation",` +
				`"guidance":"Confirmar o proximo passo antes de encerrar.",` +
				`"evidenceRefs":[` + evidence + `]}],"evidenceRefs":[` + evidence + `],` +
				`"reasonCodes":["review_complete"],"confidence":0.85}`,
		),
	}
}
