package customerintelligence

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type projectionFoundationFake struct {
	FoundationRepository
	facts             []Fact
	summary           Summary
	summaryCiphertext string
	recommendations   []Recommendation
	suggestions       []SourceSuggestion
	reviewCalled      bool
}

func (f *projectionFoundationFake) GetCapability(
	_ context.Context,
	scope Scope,
	key, _ string,
) (Capability, error) {
	return Capability{
		ID:              headlessTestCapability,
		AccountID:       scope.AccountID,
		ClientAccountID: scope.ClientAccountID,
		Key:             key,
		Mode:            "on",
		Config:          json.RawMessage(`{}`),
	}, nil
}

func (f *projectionFoundationFake) ListFacts(
	_ context.Context,
	_ Scope,
	_ string,
	_ int,
) ([]Fact, error) {
	return append([]Fact(nil), f.facts...), nil
}

func (f *projectionFoundationFake) LatestSummary(
	_ context.Context,
	_ Scope,
	_ string,
) (string, Summary, error) {
	if f.summaryCiphertext == "" {
		return "", Summary{}, ErrNotFound
	}
	return f.summaryCiphertext, f.summary, nil
}

func (f *projectionFoundationFake) ListRecommendations(
	_ context.Context,
	_ Scope,
	_ string,
	_ int,
) ([]Recommendation, error) {
	return append([]Recommendation(nil), f.recommendations...), nil
}

func (f *projectionFoundationFake) ListSourceSuggestions(
	_ context.Context,
	_ Scope,
	_ string,
	_ int,
) ([]SourceSuggestion, error) {
	return append([]SourceSuggestion(nil), f.suggestions...), nil
}

func (f *projectionFoundationFake) ReviewSourceSuggestion(
	_ context.Context,
	_ Scope,
	_, _ string,
	_ SourceSuggestionFeedback,
) (SourceSuggestion, error) {
	f.reviewCalled = true
	return SourceSuggestion{}, nil
}

func projectionService(
	t *testing.T,
	foundation FoundationRepository,
) (*Service, *secretbox.Box) {
	t.Helper()
	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithRepositories(
		foundation,
		nil,
		nil,
		box,
		nil,
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(allowEveryClient)),
		WithRelationshipScopeAuthorizer(
			RelationshipScopeAuthorizerFunc(allowEveryRelationship),
		),
	)
	return service, box
}

func TestProfileReturnsSafeProjectionWithoutLLMContextEnvelope(t *testing.T) {
	t.Parallel()
	foundation := &projectionFoundationFake{
		facts: []Fact{{
			ID: headlessTestSubject, SubjectID: headlessTestSubject,
			RelationshipID: headlessTestRelationship, Key: "preferred_name",
			Version: 1, Value: json.RawMessage(`"Ana"`),
		}},
		summary: Summary{
			ID: headlessTestCapability, RelationshipID: headlessTestRelationship,
			SummaryType: "relationship_profile",
		},
	}
	service, box := projectionService(t, foundation)
	ciphertext, err := box.Encrypt(`{"summary":"Perfil seguro"}`)
	if err != nil {
		t.Fatal(err)
	}
	foundation.summaryCiphertext = ciphertext
	view, err := service.Profile(
		context.Background(),
		Scope{AccountID: headlessTestAccount, ClientAccountID: headlessTestClient},
		headlessTestRelationship,
	)
	if err != nil || view.Summary == nil || len(view.Facts) != 1 {
		t.Fatalf("projection=%#v err=%v", view, err)
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"observations", "snapshot", "provenance"} {
		if strings.Contains(strings.ToLower(string(raw)), forbidden) {
			t.Fatalf("projection vazou %s: %s", forbidden, raw)
		}
	}
}

func TestRecommendationsDecryptServerSideWithoutReturningCiphertext(t *testing.T) {
	t.Parallel()
	foundation := &projectionFoundationFake{}
	service, box := projectionService(t, foundation)
	output := validNonConversationalProcessOutputs()["recommendation.follow_up"]
	ciphertext, err := box.Encrypt(string(output))
	if err != nil {
		t.Fatal(err)
	}
	foundation.recommendations = []Recommendation{{
		ID: headlessTestCapability, ClientAccountID: headlessTestClient,
		RelationshipID: headlessTestRelationship, Type: "follow_up",
		Status: "proposed", PayloadCiphertext: ciphertext,
	}}
	items, err := service.Recommendations(
		context.Background(),
		Scope{AccountID: headlessTestAccount, ClientAccountID: headlessTestClient},
		headlessTestRelationship,
		20,
	)
	if err != nil || len(items) != 1 || len(items[0].Payload) == 0 ||
		items[0].PayloadCiphertext != "" {
		t.Fatalf("items=%#v err=%v", items, err)
	}
}

func TestSourceSuggestionMaterializesOnlyItsRegisteredRationale(t *testing.T) {
	t.Parallel()
	foundation := &projectionFoundationFake{}
	service, box := projectionService(t, foundation)
	output := validNonConversationalProcessOutputs()["source.suggest"]
	ciphertext, err := box.Encrypt(string(output))
	if err != nil {
		t.Fatal(err)
	}
	foundation.suggestions = []SourceSuggestion{{
		ID: headlessTestCapability, ClientAccountID: headlessTestClient,
		RelationshipID: headlessTestRelationship, SourceKey: "erp",
		RationaleCiphertext: ciphertext,
	}}
	items, err := service.SourceSuggestions(
		context.Background(),
		Scope{AccountID: headlessTestAccount, ClientAccountID: headlessTestClient},
		headlessTestRelationship,
		20,
	)
	if err != nil || len(items) != 1 || items[0].Rationale == "" ||
		items[0].RationaleCiphertext != "" {
		t.Fatalf("items=%#v err=%v", items, err)
	}
}

func TestSourceSuggestionReviewRejectsFreeTextReasonBeforeRepository(t *testing.T) {
	t.Parallel()
	foundation := &projectionFoundationFake{}
	service, _ := projectionService(t, foundation)
	_, err := service.ReviewSourceSuggestion(
		context.Background(),
		Scope{AccountID: headlessTestAccount, ClientAccountID: headlessTestClient},
		headlessTestActor,
		headlessTestCapability,
		SourceSuggestionFeedback{
			Status: "accepted",
			Reason: "texto livre com dados pessoais",
		},
	)
	if err == nil || foundation.reviewCalled {
		t.Fatalf("reason livre chegou ao repositorio: err=%v", err)
	}
}
