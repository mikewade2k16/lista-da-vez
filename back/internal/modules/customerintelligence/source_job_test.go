package customerintelligence

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type sourceJobFoundationFake struct {
	FoundationRepository
	run        SourceRun
	config     SourceConfig
	completion struct {
		status    string
		errorCode string
	}
	inserted []Observation
}

func (f *sourceJobFoundationFake) GetCapability(
	context.Context,
	Scope,
	string,
	string,
) (Capability, error) {
	return Capability{Mode: "on", Config: json.RawMessage(`{}`)}, nil
}

func (f *sourceJobFoundationFake) GetSourceRun(
	context.Context,
	Scope,
	string,
) (SourceRun, error) {
	return f.run, nil
}

func (f *sourceJobFoundationFake) GetSourceConfig(
	context.Context,
	Scope,
	string,
) (SourceConfig, error) {
	return f.config, nil
}

func (f *sourceJobFoundationFake) CompleteSourceRun(
	_ context.Context,
	_, _, status string,
	_, _, _ int,
	errorCode string,
) error {
	f.completion.status = status
	f.completion.errorCode = errorCode
	return nil
}

func (f *sourceJobFoundationFake) InsertObservations(
	_ context.Context,
	_ SourceRun,
	observations []Observation,
) (int, error) {
	f.inserted = append([]Observation(nil), observations...)
	return len(observations), nil
}

type sourceAdapterPanic struct{}

func (sourceAdapterPanic) Fetch(
	context.Context,
	SourceConfig,
	string,
) ([]Observation, error) {
	panic("adapter nao pode executar depois que a fonte foi desabilitada")
}

type sourceAdapterFunc func(
	context.Context,
	SourceConfig,
	string,
) ([]Observation, error)

func (f sourceAdapterFunc) Fetch(
	ctx context.Context,
	config SourceConfig,
	relationshipID string,
) ([]Observation, error) {
	return f(ctx, config, relationshipID)
}

type permanentSourceTestError struct {
	code string
}

func (e permanentSourceTestError) Error() string {
	return "source unavailable"
}

func (e permanentSourceTestError) SourceFailureCode() string {
	return e.code
}

func (permanentSourceTestError) SourceRetryable() bool {
	return false
}

func TestSourceJobRechecksDisabledSourceBeforeAdapter(t *testing.T) {
	t.Parallel()
	foundation := &sourceJobFoundationFake{
		run: SourceRun{
			ID:        "66666666-6666-4666-8666-666666666666",
			AccountID: testAccount, ClientAccountID: testClient,
			SourceConfigID: "77777777-7777-4777-8777-777777777777",
			SourceKey:      "erp", Status: "queued",
		},
		config: SourceConfig{
			ID:        "77777777-7777-4777-8777-777777777777",
			AccountID: testAccount, ClientAccountID: testClient,
			SourceKey: "erp", Status: "disabled",
		},
	}
	service := NewServiceWithRepositories(
		foundation, nil, nil, nil, nil,
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(allowEveryClient)),
		WithSourceAdapter("erp", sourceAdapterPanic{}),
	)
	payload, err := json.Marshal(sourceJobPayload{
		RunID: foundation.run.ID, ClientAccountID: testClient,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = NewSourceJobHandler(service).Handle(context.Background(), jobs.Job{
		AccountID: testAccount, Payload: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if foundation.completion.status != "failed" ||
		foundation.completion.errorCode != "source_disabled" {
		t.Fatalf("completion inesperada: %#v", foundation.completion)
	}
}

func TestSourceJobKeepsBusinessContextOutsideRelationshipScope(t *testing.T) {
	t.Parallel()
	const relationshipID = "88888888-8888-4888-8888-888888888888"
	foundation := &sourceJobFoundationFake{
		run: SourceRun{
			ID:        "66666666-6666-4666-8666-666666666666",
			AccountID: testAccount, ClientAccountID: testClient,
			SourceConfigID: "77777777-7777-4777-8777-777777777777",
			SourceKey:      "calendar.client_profile", Status: "queued",
		},
		config: SourceConfig{
			ID:        "77777777-7777-4777-8777-777777777777",
			AccountID: testAccount, ClientAccountID: testClient,
			SourceKey: "calendar.client_profile", Status: "enabled",
			PurposeKey: "customer_profile", FieldAllowlist: []string{"strategy"},
		},
	}
	adapter := sourceAdapterFunc(func(
		context.Context,
		SourceConfig,
		string,
	) ([]Observation, error) {
		return []Observation{{
			EntityType:  "client_business_context",
			EntityID:    testClient,
			Version:     "1",
			ScopeType:   ObservationScopeBusiness,
			Snapshot:    json.RawMessage(`{"strategy":{"summary":"premium"}}`),
			Sensitivity: "internal",
		}}, nil
	})
	service := NewServiceWithRepositories(
		foundation, nil, nil, nil, nil,
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(allowEveryClient)),
		WithSourceAdapter("calendar.client_profile", adapter),
	)
	payload, err := json.Marshal(sourceJobPayload{
		RunID: foundation.run.ID, ClientAccountID: testClient,
		RelationshipID: relationshipID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSourceJobHandler(service).Handle(context.Background(), jobs.Job{
		AccountID: testAccount, Payload: payload,
	}); err != nil {
		t.Fatal(err)
	}
	if len(foundation.inserted) != 1 {
		t.Fatalf("observations inseridas = %d, want 1", len(foundation.inserted))
	}
	got := foundation.inserted[0]
	if got.ScopeType != ObservationScopeBusiness ||
		got.Classification != ObservationClassificationBusinessContext ||
		got.SubjectID != "" ||
		got.RelationshipID != "" {
		t.Fatalf("contexto empresarial herdou escopo pessoal: %#v", got)
	}
}

func TestSourceJobDoesNotRetryPermanentSourceFailure(t *testing.T) {
	t.Parallel()
	foundation := &sourceJobFoundationFake{
		run: SourceRun{
			ID:        "66666666-6666-4666-8666-666666666666",
			AccountID: testAccount, ClientAccountID: testClient,
			SourceConfigID: "77777777-7777-4777-8777-777777777777",
			SourceKey:      "bi.perola", Status: "queued",
		},
		config: SourceConfig{
			ID:        "77777777-7777-4777-8777-777777777777",
			AccountID: testAccount, ClientAccountID: testClient,
			SourceKey: "bi.perola", Status: "enabled",
		},
	}
	wantErr := permanentSourceTestError{code: "deterministic_subject_link_unavailable"}
	adapter := sourceAdapterFunc(func(
		context.Context,
		SourceConfig,
		string,
	) ([]Observation, error) {
		return nil, wantErr
	})
	service := NewServiceWithRepositories(
		foundation, nil, nil, nil, nil,
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(allowEveryClient)),
		WithSourceAdapter("bi.perola", adapter),
	)
	payload, err := json.Marshal(sourceJobPayload{
		RunID: foundation.run.ID, ClientAccountID: testClient,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = NewSourceJobHandler(service).Handle(context.Background(), jobs.Job{
		AccountID: testAccount, Payload: payload,
	})
	if err != nil {
		t.Fatalf("falha permanente deveria encerrar sem retry: %v", err)
	}
	if foundation.completion.status != "failed" ||
		foundation.completion.errorCode != wantErr.code {
		t.Fatalf("completion inesperada: %#v", foundation.completion)
	}
}

func TestFilterObservationRejectsPurposeDifferentFromSourceConfig(t *testing.T) {
	t.Parallel()
	config := SourceConfig{
		PurposeKey:     "customer_profile",
		FieldAllowlist: []string{"total_amount_cents"},
	}
	_, err := filterObservation(config, Observation{
		EntityType:     "order",
		EntityID:       "erp-order-1",
		ScopeType:      ObservationScopeSubject,
		SubjectID:      testSubject,
		RelationshipID: testRelationship,
		Snapshot:       json.RawMessage(`{"total_amount_cents":300}`),
		Sensitivity:    "internal",
		PurposeKey:     "marketing",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("purpose divergente retornou %v, esperado forbidden", err)
	}
}

func TestSourceJobRejectsAdapterPurposeEscalation(t *testing.T) {
	t.Parallel()
	foundation := &sourceJobFoundationFake{
		run: SourceRun{
			ID:        "66666666-6666-4666-8666-666666666666",
			AccountID: testAccount, ClientAccountID: testClient,
			SourceConfigID: "77777777-7777-4777-8777-777777777777",
			SourceKey:      "erp", Status: "queued",
		},
		config: SourceConfig{
			ID:        "77777777-7777-4777-8777-777777777777",
			AccountID: testAccount, ClientAccountID: testClient,
			SourceKey: "erp", Status: "enabled",
			PurposeKey: "customer_profile", FieldAllowlist: []string{"total_amount_cents"},
		},
	}
	adapter := sourceAdapterFunc(func(
		context.Context,
		SourceConfig,
		string,
	) ([]Observation, error) {
		return []Observation{{
			EntityType:     "order",
			EntityID:       "erp-order-purpose-escalation",
			ScopeType:      ObservationScopeSubject,
			SubjectID:      testSubject,
			RelationshipID: testRelationship,
			Snapshot:       json.RawMessage(`{"total_amount_cents":300}`),
			Sensitivity:    "internal",
			PurposeKey:     "marketing",
		}}, nil
	})
	service := NewServiceWithRepositories(
		foundation, nil, nil, nil, nil,
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(allowEveryClient)),
		WithRelationshipScopeAuthorizer(
			RelationshipScopeAuthorizerFunc(allowEveryRelationship),
		),
		WithSourceAdapter("erp", adapter),
	)
	payload, err := json.Marshal(sourceJobPayload{
		RunID: foundation.run.ID, ClientAccountID: testClient,
		RelationshipID: testRelationship,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSourceJobHandler(service).Handle(
		context.Background(),
		jobs.Job{AccountID: testAccount, Payload: payload},
	); err != nil {
		t.Fatal(err)
	}
	if foundation.completion.status != "partial" {
		t.Fatalf("status = %q, esperado partial", foundation.completion.status)
	}
	if len(foundation.inserted) != 0 {
		t.Fatalf("purpose escalation foi persistida: %#v", foundation.inserted)
	}
}

func TestSourceJobKeepsCommittedIngestionWhenRefreshEnqueueFails(t *testing.T) {
	t.Parallel()
	foundation := &sourceJobFoundationFake{
		run: SourceRun{
			ID:        "66666666-6666-4666-8666-666666666666",
			AccountID: testAccount, ClientAccountID: testClient,
			SourceConfigID: "77777777-7777-4777-8777-777777777777",
			SourceKey:      "erp", Status: "queued",
		},
		config: SourceConfig{
			ID:        "77777777-7777-4777-8777-777777777777",
			AccountID: testAccount, ClientAccountID: testClient,
			SourceKey: "erp", Status: "enabled",
			PurposeKey: "customer_profile", FieldAllowlist: []string{"total_amount_cents"},
		},
	}
	adapter := sourceAdapterFunc(func(
		context.Context,
		SourceConfig,
		string,
	) ([]Observation, error) {
		return []Observation{{
			EntityType:     "order",
			EntityID:       "erp-order-refresh-warning",
			ScopeType:      ObservationScopeSubject,
			SubjectID:      testSubject,
			RelationshipID: testRelationship,
			Snapshot:       json.RawMessage(`{"total_amount_cents":300}`),
			Sensitivity:    "internal",
			PurposeKey:     "customer_profile",
		}}, nil
	})
	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	enqueuer := &headlessEnqueuerFake{err: errors.New("queue unavailable")}
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
		WithSourceAdapter("erp", adapter),
		WithHeadlessJobEnqueuer(enqueuer),
	)
	payload, err := json.Marshal(sourceJobPayload{
		RunID: foundation.run.ID, ClientAccountID: testClient,
		RelationshipID: testRelationship,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSourceJobHandler(service).Handle(
		context.Background(),
		jobs.Job{AccountID: testAccount, Payload: payload},
	); err != nil {
		t.Fatalf("ingestao gravada virou falha: %v", err)
	}
	if len(foundation.inserted) != 1 ||
		foundation.completion.status != "completed" ||
		foundation.completion.errorCode != "refresh_enqueue_failed" {
		t.Fatalf(
			"completion nao preservou sucesso+warning: inserted=%d completion=%#v",
			len(foundation.inserted),
			foundation.completion,
		)
	}
}
