package customerintelligence

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type contextSourceFoundationFake struct {
	foundationFake
	facts                []Fact
	records              []StoredObservation
	requestedSourceKeys  []string
	requestedPurposeKeys []string
	saved                ContextEnvelope
}

func (fake *contextSourceFoundationFake) ListFacts(
	context.Context,
	Scope,
	string,
	int,
) ([]Fact, error) {
	return append([]Fact(nil), fake.facts...), nil
}

func (fake *contextSourceFoundationFake) ListRelationshipObservations(
	_ context.Context,
	_ Scope,
	_ string,
	sourceKeys []string,
	purposeKeys []string,
	limit int,
) ([]StoredObservation, error) {
	fake.requestedSourceKeys = append([]string(nil), sourceKeys...)
	fake.requestedPurposeKeys = append([]string(nil), purposeKeys...)
	allowed := make(map[string]bool, len(sourceKeys))
	for _, sourceKey := range sourceKeys {
		allowed[sourceKey] = true
	}
	allowedPurposes := make(map[string]bool, len(purposeKeys))
	for _, purposeKey := range purposeKeys {
		allowedPurposes[purposeKey] = true
	}
	items := make([]StoredObservation, 0, len(fake.records))
	for _, record := range fake.records {
		if len(allowed) > 0 && !allowed[record.SourceKey] {
			continue
		}
		if !allowedPurposes[record.PurposeKey] || record.Sensitivity == "restricted" {
			continue
		}
		items = append(items, record)
		if len(items) == limit {
			break
		}
	}
	return items, nil
}

func (fake *contextSourceFoundationFake) GetObservation(
	context.Context,
	Scope,
	string,
) (StoredObservation, error) {
	return StoredObservation{}, ErrNotFound
}

func (fake *contextSourceFoundationFake) SaveContextSnapshot(
	_ context.Context,
	envelope ContextEnvelope,
	_, _ string,
) (string, error) {
	fake.saved = envelope
	return "66666666-6666-4666-8666-666666666666", nil
}

func newContextSourceService(
	t *testing.T,
	foundation *contextSourceFoundationFake,
) *Service {
	t.Helper()
	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	return NewServiceWithRepositories(
		foundation,
		nil,
		nil,
		box,
		nil,
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(allowEveryClient)),
		WithRelationshipScopeAuthorizer(
			RelationshipScopeAuthorizerFunc(allowEveryRelationship),
		),
		withClock(func() time.Time {
			return time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
		}),
	)
}

func TestBuildContextIncludesAllowedBusinessObservationAndOmitsWrongPurpose(
	t *testing.T,
) {
	t.Parallel()
	foundation := &contextSourceFoundationFake{
		foundationFake: foundationFake{modes: map[string]string{
			CapabilityContext: "on",
		}},
		records: []StoredObservation{
			{
				ID:               "77777777-7777-4777-8777-777777777777",
				SourceKey:        "calendar.client_profile",
				SourceEntityType: "client_business_context",
				SourceEntityID:   testClient,
				Snapshot:         json.RawMessage(`{"voice":{"brand_voice":"consultivo"}}`),
				FieldAllowlist:   []string{"voice"},
				Sensitivity:      "internal",
				PurposeKey:       "customer_profile",
				ObservedAt:       time.Date(2026, time.July, 23, 11, 0, 0, 0, time.UTC),
			},
			{
				ID:               "88888888-8888-4888-8888-888888888888",
				SourceKey:        "site",
				SourceEntityType: "site_lead",
				SourceEntityID:   "99999999-9999-4999-8999-999999999999",
				Snapshot:         json.RawMessage(`{"page":"/campanha"}`),
				FieldAllowlist:   []string{"page"},
				Sensitivity:      "internal",
				PurposeKey:       "marketing",
				ObservedAt:       time.Date(2026, time.July, 23, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	service := newContextSourceService(t, foundation)
	envelope, err := service.BuildContext(context.Background(), ContextRequest{
		AccountID:       testAccount,
		ClientAccountID: testClient,
		SubjectID:       testSubject,
		RelationshipID:  testRelationship,
		ProcessKeys:     []string{"conversation.reply"},
		Purpose:         "customer_service",
		MaxItems:        10,
		MaxTokens:       2_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(envelope.Observations) != 1 ||
		envelope.Observations[0].SourceKey != "calendar.client_profile" {
		t.Fatalf("observations = %#v", envelope.Observations)
	}
	if !strings.HasPrefix(
		envelope.Observations[0].ProvenanceRef,
		observationProvenancePrefix,
	) || strings.Contains(envelope.Observations[0].ProvenanceRef, testClient) {
		t.Fatalf("proveniencia nao opaca: %q", envelope.Observations[0].ProvenanceRef)
	}
	if contextWarningsContain(envelope.Warnings, "purpose_mismatch_observation_omitted") {
		t.Fatalf("mismatch deveria ser filtrado no repository: %#v", envelope.Warnings)
	}
	if len(foundation.requestedPurposeKeys) != 3 ||
		foundation.requestedPurposeKeys[0] != "customer_profile" ||
		foundation.requestedPurposeKeys[1] != "customer_relationship" ||
		foundation.requestedPurposeKeys[2] != "customer_service" {
		t.Fatalf("purpose policy nao propagada: %#v", foundation.requestedPurposeKeys)
	}
	if len(envelope.Provenance) != 1 ||
		envelope.Provenance[0].ObservationID != envelope.Observations[0].ID {
		t.Fatalf("provenance = %#v", envelope.Provenance)
	}
	if envelope.Budget.IncludedItems != 1 ||
		len(foundation.saved.Observations) != 1 {
		t.Fatalf("budget/snapshot incoerentes: %#v saved=%#v",
			envelope.Budget, foundation.saved.Observations)
	}
}

func TestBuildContextTokenTruncationRemovesObservationProvenance(t *testing.T) {
	t.Parallel()
	factEvidenceID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	largeSnapshot, err := json.Marshal(map[string]string{
		"strategy": strings.Repeat("x", 5_000),
	})
	if err != nil {
		t.Fatal(err)
	}
	foundation := &contextSourceFoundationFake{
		foundationFake: foundationFake{modes: map[string]string{
			CapabilityContext: "on",
		}},
		facts: []Fact{{
			ID:             "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
			SubjectID:      testSubject,
			RelationshipID: testRelationship,
			Key:            "profile.preferred_name",
			Value:          json.RawMessage(`"Ana"`),
			ValueType:      "string",
			Sensitivity:    "public",
			Evidence: []EvidenceRef{{
				ObservationID: factEvidenceID,
				SourceKey:     "manual.offline",
				Locator:       "manual",
			}},
		}},
		records: []StoredObservation{{
			ID:               "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
			SourceKey:        "calendar.client_profile",
			SourceEntityType: "client_business_context",
			SourceEntityID:   testClient,
			Snapshot:         largeSnapshot,
			FieldAllowlist:   []string{"strategy"},
			Sensitivity:      "internal",
			PurposeKey:       "customer_profile",
			ObservedAt:       time.Date(2026, time.July, 23, 11, 0, 0, 0, time.UTC),
		}},
	}
	service := newContextSourceService(t, foundation)
	envelope, err := service.BuildContext(context.Background(), ContextRequest{
		AccountID:       testAccount,
		ClientAccountID: testClient,
		SubjectID:       testSubject,
		RelationshipID:  testRelationship,
		ProcessKeys:     []string{"conversation.reply"},
		Purpose:         "customer_service",
		SourceKeys:      []string{"calendar.client_profile"},
		MaxItems:        10,
		MaxTokens:       512,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(envelope.Observations) != 0 || len(envelope.Facts) != 1 {
		t.Fatalf("token truncation order incorreta: facts=%d observations=%d",
			len(envelope.Facts), len(envelope.Observations))
	}
	if !contextWarningsContain(envelope.Warnings, "context_token_budget") {
		t.Fatalf("token warning ausente: %#v", envelope.Warnings)
	}
	if len(envelope.Provenance) != 1 ||
		envelope.Provenance[0].ObservationID != factEvidenceID {
		t.Fatalf("provenance stale apos truncation: %#v", envelope.Provenance)
	}
	if !strings.HasPrefix(envelope.Provenance[0].Locator, observationProvenancePrefix) ||
		envelope.Provenance[0].Locator == "manual" {
		t.Fatalf("locator de fato nao foi protegido: %#v", envelope.Provenance)
	}
	if len(foundation.requestedSourceKeys) != 1 ||
		foundation.requestedSourceKeys[0] != "calendar.client_profile" {
		t.Fatalf("source policy nao propagada: %#v", foundation.requestedSourceKeys)
	}
	if envelope.Budget.IncludedItems != 1 ||
		envelope.Budget.EstimatedTokens > envelope.Budget.MaxTokens {
		t.Fatalf("budget = %#v", envelope.Budget)
	}
}

func contextWarningsContain(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
