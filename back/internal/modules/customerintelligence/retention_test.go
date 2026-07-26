package customerintelligence

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

func TestEffectiveSourceRetentionUsesClosedDefaultsAndPreservesBinding(t *testing.T) {
	t.Parallel()

	key, ttl, action := effectiveSourceRetention(SourceConfigInput{}, SourceConfig{})
	if key != defaultRetentionPolicyKey ||
		ttl != defaultSnapshotTTLSeconds ||
		action != retentionActionTombstone {
		t.Fatalf("defaults = %q %d %q", key, ttl, action)
	}

	current := SourceConfig{
		RetentionPolicyKey: "customer_profile.short",
		SnapshotTTLSeconds: 7 * 24 * 60 * 60,
		OnExpiry:           retentionActionCryptoShred,
	}
	key, ttl, action = effectiveSourceRetention(SourceConfigInput{}, current)
	if key != current.RetentionPolicyKey ||
		ttl != current.SnapshotTTLSeconds ||
		action != current.OnExpiry {
		t.Fatalf("preserved = %q %d %q", key, ttl, action)
	}
}

func TestRetentionPolicyInputIsClosedAndBounded(t *testing.T) {
	t.Parallel()

	valid := SourceConfigInput{
		RetentionPolicyKey: "customer_profile.short",
		SnapshotTTLSeconds: minSnapshotTTLSeconds,
		OnExpiry:           retentionActionCryptoShred,
	}
	if !validRetentionPolicyInput(valid) {
		t.Fatal("valid policy rejected")
	}
	for _, invalid := range []SourceConfigInput{
		{RetentionPolicyKey: "INVALID POLICY"},
		{SnapshotTTLSeconds: minSnapshotTTLSeconds - 1},
		{SnapshotTTLSeconds: maxSnapshotTTLSeconds + 1},
		{OnExpiry: "delete"},
	} {
		if validRetentionPolicyInput(invalid) {
			t.Fatalf("invalid policy accepted: %#v", invalid)
		}
	}
}

type retentionPolicyFoundationFake struct {
	FoundationRepository
	drafts    []RetentionPolicyDraftInput
	publishes []PublishRetentionPolicyInput
}

func (f *retentionPolicyFoundationFake) ListRetentionPolicyVersions(
	context.Context,
	string,
) ([]RetentionPolicyVersion, error) {
	return []RetentionPolicyVersion{{ID: "policy-version"}}, nil
}

func (f *retentionPolicyFoundationFake) CreateRetentionPolicyDraft(
	_ context.Context,
	accountID, actorID, policyKey string,
	input RetentionPolicyDraftInput,
) (RetentionPolicyVersion, error) {
	f.drafts = append(f.drafts, input)
	return RetentionPolicyVersion{
		ID:              "55555555-5555-4555-8555-555555555555",
		AccountID:       accountID,
		PolicyKey:       policyKey,
		Status:          "draft",
		Revision:        1,
		CreatedByUserID: actorID,
	}, nil
}

func (f *retentionPolicyFoundationFake) PublishRetentionPolicyVersion(
	_ context.Context,
	accountID, actorID, id string,
	input PublishRetentionPolicyInput,
) (RetentionPolicyVersion, error) {
	f.publishes = append(f.publishes, input)
	return RetentionPolicyVersion{
		ID:                    id,
		AccountID:             accountID,
		Status:                "published",
		Revision:              input.ExpectedRevision + 1,
		PublishedByUserID:     actorID,
		PublicationReasonCode: input.ReasonCode,
		ApprovalReference:     input.ApprovalReference,
	}, nil
}

func TestRetentionPolicyServiceRequiresExplicitApprovalMetadata(t *testing.T) {
	t.Parallel()

	accountID := "11111111-1111-4111-8111-111111111111"
	actorID := "22222222-2222-4222-8222-222222222222"
	versionID := "33333333-3333-4333-8333-333333333333"
	repository := &retentionPolicyFoundationFake{}
	service := NewServiceWithRepositories(
		repository,
		nil,
		nil,
		nil,
		nil,
	)

	draft, err := service.CreateRetentionPolicyDraft(
		context.Background(),
		accountID,
		actorID,
		"customer_profile.short",
		RetentionPolicyDraftInput{
			SnapshotTTLSeconds: minSnapshotTTLSeconds,
			OnExpiry:           retentionActionCryptoShred,
		},
	)
	if err != nil || draft.Status != "draft" || len(repository.drafts) != 1 {
		t.Fatalf("draft=%#v calls=%d err=%v", draft, len(repository.drafts), err)
	}

	for _, input := range []PublishRetentionPolicyInput{
		{},
		{
			ExpectedRevision: 1,
			ReasonCode:       "legal_review_approved",
		},
		{
			ExpectedRevision:  1,
			ReasonCode:        "INVALID REASON",
			ApprovalReference: "LEGAL-RETENTION-1",
		},
	} {
		if _, err := service.PublishRetentionPolicy(
			context.Background(),
			accountID,
			actorID,
			versionID,
			input,
		); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid publish %#v error = %v", input, err)
		}
	}
	if len(repository.publishes) != 0 {
		t.Fatalf("invalid publication reached repository: %d calls", len(repository.publishes))
	}

	published, err := service.PublishRetentionPolicy(
		context.Background(),
		accountID,
		actorID,
		versionID,
		PublishRetentionPolicyInput{
			ExpectedRevision:  1,
			ReasonCode:        "legal_review_approved",
			ApprovalReference: "LEGAL-RETENTION-1",
		},
	)
	if err != nil ||
		published.Status != "published" ||
		len(repository.publishes) != 1 {
		t.Fatalf(
			"published=%#v calls=%d err=%v",
			published,
			len(repository.publishes),
			err,
		)
	}
}

func TestRetentionPolicyApprovalRequiredHasStableHTTPError(t *testing.T) {
	t.Parallel()

	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		context.Background(),
		"PUT",
		"/v1/customer-intelligence/sources",
		nil,
	)
	writeError(response, request, ErrRetentionPolicyApprovalRequired)
	if response.Code != 409 {
		t.Fatalf("status = %d", response.Code)
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "retention_policy_approval_required" {
		t.Fatalf("error code = %q body=%s", payload.Error.Code, response.Body.String())
	}
}

func TestEffectiveObservationExpiryUsesEarlierBound(t *testing.T) {
	t.Parallel()

	observedAt := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	policyExpiry := observedAt.Add(24 * time.Hour)
	if got := effectiveObservationExpiry(
		observedAt,
		nil,
		minSnapshotTTLSeconds,
	); !got.Equal(policyExpiry) {
		t.Fatalf("default expiry = %s, want %s", got, policyExpiry)
	}
	earlier := observedAt.Add(2 * time.Hour)
	if got := effectiveObservationExpiry(
		observedAt,
		&earlier,
		minSnapshotTTLSeconds,
	); !got.Equal(earlier) {
		t.Fatalf("adapter earlier expiry = %s, want %s", got, earlier)
	}
	later := observedAt.Add(7 * 24 * time.Hour)
	if got := effectiveObservationExpiry(
		observedAt,
		&later,
		minSnapshotTTLSeconds,
	); !got.Equal(policyExpiry) {
		t.Fatalf("adapter extended expiry = %s, want policy %s", got, policyExpiry)
	}
}

type observationRetentionRepositoryFake struct {
	scopes                    []Scope
	observationBatches        []int
	contextSnapshotBatches    []int
	observationApplyCalls     []Scope
	contextSnapshotApplyCalls []Scope
	observationCorrelations   []string
	contextCorrelations       []string
}

func (f *observationRetentionRepositoryFake) ListExpiredRetentionScopes(
	context.Context,
) ([]Scope, error) {
	return append([]Scope(nil), f.scopes...), nil
}

func (f *observationRetentionRepositoryFake) ApplyExpiredObservationRetention(
	_ context.Context,
	scope Scope,
	correlationID string,
	_ int,
) (int, error) {
	f.observationApplyCalls = append(f.observationApplyCalls, scope)
	f.observationCorrelations = append(
		f.observationCorrelations,
		correlationID,
	)
	if len(f.observationBatches) == 0 {
		return 0, nil
	}
	next := f.observationBatches[0]
	f.observationBatches = f.observationBatches[1:]
	return next, nil
}

func (f *observationRetentionRepositoryFake) ApplyExpiredContextSnapshotRetention(
	_ context.Context,
	scope Scope,
	correlationID string,
	_ int,
) (int, error) {
	f.contextSnapshotApplyCalls = append(f.contextSnapshotApplyCalls, scope)
	f.contextCorrelations = append(f.contextCorrelations, correlationID)
	if len(f.contextSnapshotBatches) == 0 {
		return 0, nil
	}
	next := f.contextSnapshotBatches[0]
	f.contextSnapshotBatches = f.contextSnapshotBatches[1:]
	return next, nil
}

func TestObservationRetentionJobDrainsBoundedBatchesWithClientScope(t *testing.T) {
	t.Parallel()

	accountID := "11111111-1111-4111-8111-111111111111"
	clientAccountID := "22222222-2222-4222-8222-222222222222"
	repository := &observationRetentionRepositoryFake{
		observationBatches: []int{observationRetentionBatchSize, 3},
	}
	handler := NewObservationRetentionJobHandler(repository)
	payload, _ := json.Marshal(observationRetentionJobPayload{
		ClientAccountID: clientAccountID,
		ScheduledFor:    "2026-07-23",
	})
	err := handler.Handle(context.Background(), jobs.Job{
		ID:        "retention-job-1",
		AccountID: accountID,
		Payload:   payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repository.observationApplyCalls) != 2 {
		t.Fatalf(
			"observation apply calls = %d, want 2",
			len(repository.observationApplyCalls),
		)
	}
	if len(repository.contextSnapshotApplyCalls) != 2 {
		t.Fatalf(
			"context apply calls = %d, want 2",
			len(repository.contextSnapshotApplyCalls),
		)
	}
	for _, scope := range append(
		append([]Scope(nil), repository.observationApplyCalls...),
		repository.contextSnapshotApplyCalls...,
	) {
		if scope.AccountID != accountID || scope.ClientAccountID != clientAccountID {
			t.Fatalf("cross-scope apply: %#v", scope)
		}
	}
	for _, correlation := range append(
		append([]string(nil), repository.observationCorrelations...),
		repository.contextCorrelations...,
	) {
		if correlation != "retention-job-1" {
			t.Fatalf("correlation = %q", correlation)
		}
	}
}

func TestObservationRetentionJobDrainsContextSnapshotsIndependently(t *testing.T) {
	t.Parallel()

	repository := &observationRetentionRepositoryFake{
		contextSnapshotBatches: []int{observationRetentionBatchSize, 2},
	}
	handler := NewObservationRetentionJobHandler(repository)
	payload, _ := json.Marshal(observationRetentionJobPayload{
		ClientAccountID: "22222222-2222-4222-8222-222222222222",
		ScheduledFor:    "2026-07-23",
	})
	err := handler.Handle(context.Background(), jobs.Job{
		ID:        "context-retention-job-1",
		AccountID: "11111111-1111-4111-8111-111111111111",
		Payload:   payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repository.contextSnapshotApplyCalls) != 2 ||
		len(repository.observationApplyCalls) != 2 {
		t.Fatalf(
			"observation calls=%d context calls=%d",
			len(repository.observationApplyCalls),
			len(repository.contextSnapshotApplyCalls),
		)
	}
}

func TestObservationRetentionJobRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	handler := NewObservationRetentionJobHandler(&observationRetentionRepositoryFake{})
	err := handler.Handle(context.Background(), jobs.Job{
		AccountID: "11111111-1111-4111-8111-111111111111",
		Payload:   json.RawMessage(`{"clientAccountId":"other","scheduledFor":"2026-07-23"}`),
	})
	var statusErr *jobs.StatusError
	if !errors.As(err, &statusErr) || !statusErr.Unrecoverable {
		t.Fatalf("error = %#v, want unrecoverable", err)
	}
}

type observationRetentionEnqueuerFake struct {
	seen map[string]bool
	jobs []jobs.NewJob
}

func (f *observationRetentionEnqueuerFake) Enqueue(
	_ context.Context,
	job jobs.NewJob,
) (string, bool, error) {
	if f.seen == nil {
		f.seen = make(map[string]bool)
	}
	key := job.AccountID + "\x00" + job.IdempotencyKey
	if f.seen[key] {
		return "", false, nil
	}
	f.seen[key] = true
	f.jobs = append(f.jobs, job)
	return "queued", true, nil
}

func TestRetentionSchedulerEnqueuesPerClientAndDateIdempotently(t *testing.T) {
	t.Parallel()

	repository := &observationRetentionRepositoryFake{scopes: []Scope{
		{
			AccountID:       "11111111-1111-4111-8111-111111111111",
			ClientAccountID: "22222222-2222-4222-8222-222222222222",
		},
		{
			AccountID:       "11111111-1111-4111-8111-111111111111",
			ClientAccountID: "33333333-3333-4333-8333-333333333333",
		},
	}}
	enqueuer := &observationRetentionEnqueuerFake{}
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)

	count, err := EnqueueExpiredObservationRetention(
		context.Background(), repository, enqueuer, now,
	)
	if err != nil || count != 2 {
		t.Fatalf("first enqueue count=%d err=%v", count, err)
	}
	count, err = EnqueueExpiredObservationRetention(
		context.Background(), repository, enqueuer, now,
	)
	if err != nil || count != 0 {
		t.Fatalf("replay enqueue count=%d err=%v", count, err)
	}
	if len(enqueuer.jobs) != 2 {
		t.Fatalf("jobs = %d", len(enqueuer.jobs))
	}
	for _, job := range enqueuer.jobs {
		var payload observationRetentionJobPayload
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if job.Kind != observationRetentionJobKind ||
			job.OrderingKey != "observation-retention:"+payload.ClientAccountID {
			t.Fatalf(
				"job kind=%q ordering=%q payload=%s",
				job.Kind,
				job.OrderingKey,
				job.Payload,
			)
		}
	}
}
