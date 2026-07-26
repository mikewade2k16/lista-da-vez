package customerintelligence

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type candidateClaimRepositoryFake struct {
	FoundationRepository
	source        runtimeClaimSource
	recorded      []preparedCandidateClaim
	current       CandidateClaim
	reviewed      ClaimReviewInput
	reviewedActor string
}

func (f *candidateClaimRepositoryFake) GetRuntimeClaimSource(
	context.Context,
	Scope,
	string,
	string,
	string,
) (runtimeClaimSource, error) {
	return f.source, nil
}

func (f *candidateClaimRepositoryFake) RecordOutcomeWithClaims(
	_ context.Context,
	_ AcceptedOutcome,
	claims []preparedCandidateClaim,
) (bool, error) {
	f.recorded = append([]preparedCandidateClaim(nil), claims...)
	return true, nil
}

func (f *candidateClaimRepositoryFake) ListCandidateClaims(
	context.Context,
	Scope,
	string,
	string,
	int,
) ([]CandidateClaim, error) {
	return []CandidateClaim{f.current}, nil
}

func (f *candidateClaimRepositoryFake) GetCandidateClaim(
	context.Context,
	Scope,
	string,
) (CandidateClaim, error) {
	return f.current, nil
}

func (f *candidateClaimRepositoryFake) ReviewCandidateClaim(
	_ context.Context,
	_ Scope,
	actorID, _ string,
	input ClaimReviewInput,
) (CandidateClaim, error) {
	f.reviewed = input
	f.reviewedActor = actorID
	item := f.current
	item.Status = input.Status
	item.ReviewReasonCode = input.ReasonCode
	item.Revision++
	return item, nil
}

func (f *candidateClaimRepositoryFake) GetCapability(
	_ context.Context,
	scope Scope,
	key, scopeKey string,
) (Capability, error) {
	return Capability{
		AccountID: scope.AccountID, ClientAccountID: scope.ClientAccountID,
		Key: key, ScopeKey: scopeKey, Mode: "on", Config: json.RawMessage(`{}`),
	}, nil
}

func TestRecordOutcomeRehydratesEncryptedRuntimeClaims(t *testing.T) {
	t.Parallel()
	box := testClaimsSecretBox(t)
	run := completeClaimRunRef()
	output, err := box.Encrypt(`{
		"extractedClaims":[{
			"factKey":"profile.preferred_name",
			"valueType":"string",
			"value":"Ana",
			"confidence":0.91,
			"evidenceObservationIds":[
				"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
				"not-an-id"
			],
			"validFrom":null,
			"validUntil":null
		}]
	}`)
	if err != nil {
		t.Fatal(err)
	}
	repository := &candidateClaimRepositoryFake{
		source: runtimeClaimSource{
			RunRef: run, SubjectID: testSubject,
			RelationshipID: testRelationship, OutputCiphertext: output,
		},
	}
	service := NewServiceWithRepositories(
		repository, nil, nil, box, nil,
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(allowEveryClient)),
		WithRelationshipScopeAuthorizer(RelationshipScopeAuthorizerFunc(allowEveryRelationship)),
	)
	created, err := service.RecordOutcome(context.Background(), AcceptedOutcome{
		AccountID: testAccount, ClientAccountID: testClient,
		EventID:       "aaaaaaaa-1111-4111-8111-111111111111",
		InteractionID: "dispatch-1", DecisionID: "decision-1",
		SubjectID: testSubject, RelationshipID: testRelationship,
		ConversationID: testConversation, OutcomeType: "reply",
		Accepted: true, ActorType: "system",
		ProcessRuns: []ProcessRunRef{run},
		Claims: []AcceptedClaimRef{{
			Ordinal: 0, FactKey: "profile.preferred_name", ValueType: "string",
			Confidence: 0.10, EvidenceObservationIDs: []string{"not-trusted"},
			ProcessKey: run.ProcessKey, RuntimeRunID: run.RunID,
			PromptBindingID:     run.PromptBindingID,
			OutputSchemaVersion: run.OutputSchemaVersion,
		}},
		Payload: json.RawMessage(`{"reasonCode":"omnichannel_effect_accepted"}`),
	})
	if err != nil || !created {
		t.Fatalf("RecordOutcome: created=%v err=%v", created, err)
	}
	if len(repository.recorded) != 1 {
		t.Fatalf("claims gravadas = %d", len(repository.recorded))
	}
	recorded := repository.recorded[0]
	plaintext, err := box.Decrypt(recorded.ValueCiphertext)
	if err != nil {
		t.Fatal(err)
	}
	if plaintext != `"Ana"` {
		t.Fatalf("valor reidratado = %s", plaintext)
	}
	if recorded.Reference.Confidence != 0.91 {
		t.Fatalf("confidence veio da outbox, nao do runtime: %f", recorded.Reference.Confidence)
	}
	if len(recorded.Reference.EvidenceObservationIDs) != 1 ||
		recorded.Reference.EvidenceObservationIDs[0] != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("evidencias nao sanitizadas: %#v", recorded.Reference.EvidenceObservationIDs)
	}
}

func TestReviewAcceptedClaimDoesNotVerifyIt(t *testing.T) {
	t.Parallel()
	box := testClaimsSecretBox(t)
	ciphertext, err := box.Encrypt(`"Ana"`)
	if err != nil {
		t.Fatal(err)
	}
	actorID := "99999999-9999-4999-8999-999999999999"
	repository := &candidateClaimRepositoryFake{
		current: CandidateClaim{
			ID:        "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
			AccountID: testAccount, ClientAccountID: testClient,
			SubjectID: testSubject, RelationshipID: testRelationship,
			Status: "candidate", VerificationState: "unverified",
			Revision: 1, valueCiphertext: ciphertext,
		},
	}
	service := NewServiceWithRepositories(
		repository, nil, nil, box, nil,
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(allowEveryClient)),
		WithRelationshipScopeAuthorizer(RelationshipScopeAuthorizerFunc(allowEveryRelationship)),
	)
	item, err := service.ReviewCandidateClaim(
		context.Background(),
		Scope{AccountID: testAccount, ClientAccountID: testClient},
		actorID,
		repository.current.ID,
		ClaimReviewInput{
			Status: "accepted", ReasonCode: "reviewed_by_operator",
			ExpectedRevision: 1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != "accepted" || item.VerificationState != "unverified" {
		t.Fatalf("review promoveu verificacao indevidamente: %#v", item)
	}
	if string(item.Value) != `"Ana"` || repository.reviewedActor != actorID {
		t.Fatalf("review inesperado: item=%#v actor=%q", item, repository.reviewedActor)
	}
}

func testClaimsSecretBox(t *testing.T) *secretbox.Box {
	t.Helper()
	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func completeClaimRunRef() ProcessRunRef {
	return ProcessRunRef{
		ProcessKey:              "conversation.triage",
		RunID:                   "10101010-1010-4010-8010-101010101010",
		Status:                  "succeeded",
		ExecutionMode:           "active",
		ProcessDefinitionID:     "20202020-2020-4020-8020-202020202020",
		ProcessConfigVersionID:  "30303030-3030-4030-8030-303030303030",
		PromptBindingID:         "40404040-4040-4040-8040-404040404040",
		PlatformPromptVersionID: "50505050-5050-4050-8050-505050505050",
		AgencyPromptVersionID:   "60606060-6060-4060-8060-606060606060",
		ClientPromptVersionID:   "70707070-7070-4070-8070-707070707070",
		ProcessPromptVersionID:  "80808080-8080-4080-8080-808080808080",
		AgentVersionID:          "90909090-9090-4090-8090-909090909090",
		ModelID:                 "abababab-abab-4bab-8bab-abababababab",
		ContextSnapshotID:       "bcbcbcbc-bcbc-4cbc-8cbc-bcbcbcbcbcbc",
		OutputSchemaVersion:     "conversation.triage.result.v2",
	}
}
