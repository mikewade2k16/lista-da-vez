package socialpublishing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type stubWorkerRepository struct {
	target         publishTarget
	ready          bool
	protected      bool
	protectErr     error
	protectCalls   int
	prepareCalls   int
	prepareErr     error
	failedCode     string
	failedErr      error
	attempted      bool
	publishedMedia string
}

type stubAnalyticsRepository struct {
	target      analyticsTarget
	savedJobKey string
}

func (s *stubAnalyticsRepository) AnalyticsTarget(
	context.Context,
	string,
	string,
) (analyticsTarget, error) {
	return s.target, nil
}

func (s *stubAnalyticsRepository) SaveAnalytics(
	_ context.Context,
	_, _, jobKey string,
	_ Analytics,
) error {
	s.savedJobKey = jobKey
	return nil
}

func (s *stubWorkerRepository) PreparePublish(
	context.Context,
	string,
	string,
	int,
) (publishTarget, bool, error) {
	s.prepareCalls++
	return s.target, s.ready, s.prepareErr
}

func (s *stubWorkerRepository) ProtectPublishOutcome(
	context.Context,
	string,
	string,
	int,
) (bool, error) {
	s.protectCalls++
	return s.protected, s.protectErr
}

func (s *stubWorkerRepository) SaveCreationID(
	context.Context,
	string,
	string,
	int,
	string,
) error {
	return nil
}

func (s *stubWorkerRepository) MarkPublishAttempted(
	context.Context,
	string,
	string,
	int,
) (bool, error) {
	s.attempted = true
	return true, nil
}

func (s *stubWorkerRepository) MarkPublished(
	_ context.Context,
	_, _ string,
	_ int,
	mediaID string,
	_ time.Time,
) error {
	s.publishedMedia = mediaID
	return nil
}

func (s *stubWorkerRepository) SavePermalink(
	context.Context,
	string,
	string,
	string,
	string,
) error {
	return nil
}

func (s *stubWorkerRepository) MarkPublishFailed(
	_ context.Context,
	_, _ string,
	_ int,
	code, _ string,
) error {
	s.failedCode = code
	return s.failedErr
}

func TestPublishWorkerHonorsModuleKillSwitch(t *testing.T) {
	repository := &stubWorkerRepository{}
	provider := &stubProvider{}
	handler := NewPublishHandler(
		repository,
		provider,
		nil,
		stubModules{enabled: false},
		nil,
	)
	payload, _ := marshalJobPayload(publishJobPayload{PostID: "post-1", Revision: 2})

	err := handler.Handle(context.Background(), jobs.Job{
		AccountID: "account-1",
		Payload:   payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if repository.failedCode != "module_disabled" {
		t.Fatalf("failedCode = %q", repository.failedCode)
	}
	if provider.createCalls != 0 || provider.publishCalls != 0 {
		t.Fatal("provider called while module was disabled")
	}
}

func TestPublishWorkerDoesNotIgnoreKillSwitchPersistenceFailure(t *testing.T) {
	repository := &stubWorkerRepository{failedErr: errStub}
	handler := NewPublishHandler(
		repository,
		&stubProvider{},
		nil,
		stubModules{enabled: false},
		nil,
	)
	payload, _ := marshalJobPayload(publishJobPayload{PostID: "post-1", Revision: 2})

	err := handler.Handle(context.Background(), jobs.Job{
		AccountID: "account-1",
		Payload:   payload,
	})

	if !errors.Is(err, errStub) {
		t.Fatalf("Handle() error = %v, want persistence failure", err)
	}
}

func TestAmbiguousOutcomeWinsBeforeKillSwitchOrMissingConnection(t *testing.T) {
	tests := []struct {
		name       string
		modules    stubModules
		prepareErr error
	}{
		{
			name:    "job recovered after mark-done failure and module disable",
			modules: stubModules{enabled: false},
		},
		{
			name:       "job recovered after connection removal",
			modules:    stubModules{enabled: true},
			prepareErr: ErrNotConnected,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &stubWorkerRepository{
				protected:  true,
				prepareErr: test.prepareErr,
			}
			provider := &stubProvider{}
			handler := NewPublishHandler(
				repository,
				provider,
				nil,
				test.modules,
				nil,
			)
			payload, _ := marshalJobPayload(
				publishJobPayload{PostID: "post-1", Revision: 2},
			)

			err := handler.Handle(context.Background(), jobs.Job{
				AccountID: "account-1",
				Payload:   payload,
			})

			if err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if repository.protectCalls != 1 || repository.prepareCalls != 0 {
				t.Fatalf(
					"protect calls = %d, prepare calls = %d",
					repository.protectCalls,
					repository.prepareCalls,
				)
			}
			if repository.failedCode != "" {
				t.Fatalf("ambiguous state was downgraded to %q", repository.failedCode)
			}
			if provider.createCalls != 0 || provider.publishCalls != 0 {
				t.Fatal("provider called for an already attempted publish")
			}
		})
	}
}

func TestPublishWorkerNeverRepeatsAttemptWithUnknownOutcome(t *testing.T) {
	attemptedAt := time.Now().UTC()
	repository := &stubWorkerRepository{
		ready: true,
		target: publishTarget{
			AccountID:          "account-1",
			PostID:             "post-1",
			Revision:           2,
			PublishAttemptedAt: &attemptedAt,
		},
	}
	provider := &stubProvider{}
	handler := NewPublishHandler(
		repository,
		provider,
		nil,
		stubModules{enabled: true},
		nil,
	)
	payload, _ := marshalJobPayload(publishJobPayload{PostID: "post-1", Revision: 2})

	err := handler.Handle(context.Background(), jobs.Job{
		AccountID: "account-1",
		Payload:   payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if repository.failedCode != "publish_outcome_unknown" {
		t.Fatalf("failedCode = %q", repository.failedCode)
	}
	if provider.createCalls != 0 || provider.publishCalls != 0 {
		t.Fatal("provider called after an external attempt was already recorded")
	}
}

func TestPublishWorkerPersistsAttemptBeforeMediaPublish(t *testing.T) {
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt("token")
	if err != nil {
		t.Fatal(err)
	}
	repository := &stubWorkerRepository{
		ready: true,
		target: publishTarget{
			AccountID:       "account-1",
			PostID:          "post-1",
			Revision:        2,
			IGUserID:        "ig-1",
			TokenCiphertext: ciphertext,
			MediaURL:        "https://cdn.example.com/post.jpg",
		},
	}
	provider := &stubProvider{}
	handler := NewPublishHandler(
		repository,
		provider,
		box,
		stubModules{enabled: true},
		nil,
	)
	payload, _ := marshalJobPayload(publishJobPayload{PostID: "post-1", Revision: 2})

	err = handler.Handle(context.Background(), jobs.Job{
		AccountID: "account-1",
		Payload:   payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !repository.attempted || provider.publishCalls != 1 ||
		repository.publishedMedia != "media-1" {
		t.Fatalf(
			"attempted=%v publishCalls=%d media=%q",
			repository.attempted,
			provider.publishCalls,
			repository.publishedMedia,
		)
	}
}

func TestPublishWorkerTreatsServerErrorAfterMediaPublishAsUnknown(t *testing.T) {
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt("token")
	if err != nil {
		t.Fatal(err)
	}
	repository := &stubWorkerRepository{
		ready: true,
		target: publishTarget{
			AccountID:       "account-1",
			PostID:          "post-1",
			Revision:        2,
			IGUserID:        "ig-1",
			TokenCiphertext: ciphertext,
			MediaURL:        "https://cdn.example.com/post.jpg",
		},
	}
	provider := &stubProvider{
		publishErr: &ProviderError{StatusCode: 500, Code: 2},
	}
	handler := NewPublishHandler(
		repository,
		provider,
		box,
		stubModules{enabled: true},
		nil,
	)
	payload, _ := marshalJobPayload(publishJobPayload{PostID: "post-1", Revision: 2})

	err = handler.Handle(context.Background(), jobs.Job{
		AccountID: "account-1",
		Payload:   payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if provider.publishCalls != 1 || repository.failedCode != "publish_outcome_unknown" {
		t.Fatalf(
			"publishCalls=%d failedCode=%q",
			provider.publishCalls,
			repository.failedCode,
		)
	}
	if repository.publishedMedia != "" {
		t.Fatalf("published media = %q after unknown outcome", repository.publishedMedia)
	}
}

func TestPublishJobKeyChangesWithRevision(t *testing.T) {
	first := publishJobKey("post-1", 1)
	second := publishJobKey("post-1", 2)
	if first == second {
		t.Fatalf("publish keys must differ across revisions: %q", first)
	}
	if first != "publish:post-1:v1" || second != "publish:post-1:v2" {
		t.Fatalf("unexpected keys: %q %q", first, second)
	}
}

func TestAnalyticsWorkerUsesOutboxIDAsSnapshotJobKey(t *testing.T) {
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt("token")
	if err != nil {
		t.Fatal(err)
	}
	repository := &stubAnalyticsRepository{target: analyticsTarget{
		AccountID:       "account-1",
		PostID:          "post-1",
		ExternalMediaID: "media-1",
		TokenCiphertext: ciphertext,
	}}
	handler := NewAnalyticsHandler(
		repository,
		&stubProvider{},
		box,
		stubModules{enabled: true},
	)
	payload, _ := marshalJobPayload(analyticsJobPayload{PostID: "post-1"})

	err = handler.Handle(context.Background(), jobs.Job{
		ID:        "outbox-job-1",
		AccountID: "account-1",
		Payload:   payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if repository.savedJobKey != "outbox-job-1" {
		t.Fatalf("job key = %q, want outbox-job-1", repository.savedJobKey)
	}
}

func TestLegacyAnalyticsJobIsForwardedToAnalyticsLane(t *testing.T) {
	queue := &recordingJobEnqueuer{}
	handler := NewAnalyticsForwardHandler(queue)
	payload, _ := marshalJobPayload(analyticsJobPayload{PostID: "post-1"})

	err := handler.Handle(context.Background(), jobs.Job{
		ID:          "legacy-job-1",
		AccountID:   "account-1",
		OrderingKey: "legacy-ordering-key",
		Kind:        AnalyticsJobKind,
		Payload:     payload,
	})

	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("forwarded jobs = %d, want 1", len(queue.enqueued))
	}
	forwarded := queue.enqueued[0]
	if forwarded.AccountID != "account-1" ||
		forwarded.Kind != AnalyticsJobKind ||
		forwarded.IdempotencyKey != "analytics:legacy:legacy-job-1" {
		t.Fatalf("forwarded job = %#v", forwarded)
	}
}
