package customerintelligence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

const (
	headlessTestAccount      = "11111111-1111-4111-8111-111111111111"
	headlessTestClient       = "22222222-2222-4222-8222-222222222222"
	headlessTestSubject      = "33333333-3333-4333-8333-333333333333"
	headlessTestRelationship = "44444444-4444-4444-8444-444444444444"
	headlessTestActor        = "55555555-5555-4555-8555-555555555555"
	headlessTestCapability   = "66666666-6666-4666-8666-666666666666"
)

type headlessFoundationFake struct {
	FoundationRepository
	profileMode string
	runtimeMode string
}

func (f *headlessFoundationFake) GetCapability(
	_ context.Context,
	scope Scope,
	key, _ string,
) (Capability, error) {
	if scope.AccountID != headlessTestAccount ||
		scope.ClientAccountID != headlessTestClient {
		return Capability{}, ErrNotFound
	}
	mode := f.profileMode
	if key == CapabilityRuntime {
		mode = f.runtimeMode
	}
	if mode == "" {
		mode = "on"
	}
	return Capability{
		ID:              headlessTestCapability,
		AccountID:       scope.AccountID,
		ClientAccountID: scope.ClientAccountID,
		Key:             key,
		Mode:            mode,
		Config:          json.RawMessage(`{}`),
	}, nil
}

type headlessEnqueuerFake struct {
	jobs    []jobs.NewJob
	id      string
	created bool
	err     error
}

func (f *headlessEnqueuerFake) Enqueue(
	_ context.Context,
	job jobs.NewJob,
) (string, bool, error) {
	f.jobs = append(f.jobs, job)
	return f.id, f.created, f.err
}

func newHeadlessEnqueueService(
	t *testing.T,
	foundation FoundationRepository,
	enqueuer headlessJobEnqueuer,
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
		WithClientScopeAuthorizer(ClientScopeAuthorizerFunc(
			func(_ context.Context, accountID, clientAccountID string) error {
				if accountID != headlessTestAccount ||
					clientAccountID != headlessTestClient {
					return ErrForbidden
				}
				return nil
			},
		)),
		WithRelationshipScopeAuthorizer(RelationshipScopeAuthorizerFunc(
			func(
				_ context.Context,
				accountID, clientAccountID, subjectID, relationshipID string,
			) error {
				if accountID != headlessTestAccount ||
					clientAccountID != headlessTestClient ||
					subjectID != headlessTestSubject ||
					relationshipID != headlessTestRelationship {
					return ErrForbidden
				}
				return nil
			},
		)),
		WithHeadlessJobEnqueuer(enqueuer),
	)
}

func TestEnqueueRelationshipRefreshUsesOpaqueClientScopedIdempotency(t *testing.T) {
	t.Parallel()
	foundation := &headlessFoundationFake{}
	enqueuer := &headlessEnqueuerFake{
		id:      "77777777-7777-4777-8777-777777777777",
		created: true,
	}
	service := newHeadlessEnqueueService(t, foundation, enqueuer)

	item, err := service.EnqueueRelationshipRefresh(
		context.Background(),
		headlessTestAccount,
		headlessTestActor,
		RelationshipRefreshInput{
			ClientAccountID: headlessTestClient,
			SubjectID:       headlessTestSubject,
			RelationshipID:  headlessTestRelationship,
			IdempotencyKey:  "panel.request-1",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !item.Created || item.Status != "pending" || item.ID != enqueuer.id {
		t.Fatalf("job inesperado: %#v", item)
	}
	if len(enqueuer.jobs) != 1 {
		t.Fatalf("jobs = %d, esperado 1", len(enqueuer.jobs))
	}
	job := enqueuer.jobs[0]
	if job.AccountID != headlessTestAccount ||
		job.Kind != relationshipRefreshJobKind ||
		job.OrderingKey != "relationship-refresh:"+headlessTestRelationship ||
		strings.Contains(job.IdempotencyKey, "panel.request-1") ||
		!strings.HasPrefix(job.IdempotencyKey, "relationship-refresh:") {
		t.Fatalf("envelope de job inseguro: %#v", job)
	}
	var payload relationshipRefreshJobPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ClientAccountID != headlessTestClient ||
		payload.SubjectID != headlessTestSubject ||
		payload.RelationshipID != headlessTestRelationship ||
		payload.PurposeKey != "customer_profile" ||
		len(payload.ProcessKeys) != len(defaultRelationshipRefreshProcesses) {
		t.Fatalf("payload inesperado: %#v", payload)
	}
}

func TestEnqueueRelationshipRefreshRejectsUnsupportedProcess(t *testing.T) {
	t.Parallel()
	enqueuer := &headlessEnqueuerFake{}
	service := newHeadlessEnqueueService(
		t,
		&headlessFoundationFake{},
		enqueuer,
	)

	_, err := service.EnqueueRelationshipRefresh(
		context.Background(),
		headlessTestAccount,
		headlessTestActor,
		RelationshipRefreshInput{
			ClientAccountID: headlessTestClient,
			SubjectID:       headlessTestSubject,
			RelationshipID:  headlessTestRelationship,
			ProcessKeys:     []string{"portfolio.opportunity"},
			IdempotencyKey:  "panel.request-2",
		},
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("erro = %v, esperado invalid input", err)
	}
	if len(enqueuer.jobs) != 0 {
		t.Fatalf("processo nao permitido foi enfileirado: %#v", enqueuer.jobs)
	}
}

func TestEnqueueRelationshipRefreshStopsBeforeQueueForCrossClient(t *testing.T) {
	t.Parallel()
	enqueuer := &headlessEnqueuerFake{}
	service := newHeadlessEnqueueService(
		t,
		&headlessFoundationFake{},
		enqueuer,
	)

	_, err := service.EnqueueRelationshipRefresh(
		context.Background(),
		headlessTestAccount,
		headlessTestActor,
		RelationshipRefreshInput{
			ClientAccountID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
			SubjectID:       headlessTestSubject,
			RelationshipID:  headlessTestRelationship,
			IdempotencyKey:  "panel.request-3",
		},
	)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("erro = %v, esperado forbidden", err)
	}
	if len(enqueuer.jobs) != 0 {
		t.Fatalf("cross-client chegou a fila: %#v", enqueuer.jobs)
	}
}

func TestEnqueueRelationshipRefreshRequiresRuntimeCapability(t *testing.T) {
	t.Parallel()
	enqueuer := &headlessEnqueuerFake{}
	service := newHeadlessEnqueueService(
		t,
		&headlessFoundationFake{runtimeMode: "off"},
		enqueuer,
	)

	_, err := service.EnqueueRelationshipRefresh(
		context.Background(),
		headlessTestAccount,
		headlessTestActor,
		RelationshipRefreshInput{
			ClientAccountID: headlessTestClient,
			SubjectID:       headlessTestSubject,
			RelationshipID:  headlessTestRelationship,
			IdempotencyKey:  "panel.request-4",
		},
	)
	if !errors.Is(err, ErrCapabilityDisabled) {
		t.Fatalf("erro = %v, esperado capability disabled", err)
	}
	if len(enqueuer.jobs) != 0 {
		t.Fatalf("runtime off chegou a fila: %#v", enqueuer.jobs)
	}
}

func TestRelationshipRefreshJobRejectsInvalidPayload(t *testing.T) {
	t.Parallel()
	handler := NewRelationshipRefreshJobHandler(nil)
	err := handler.Handle(context.Background(), jobs.Job{
		ID:        "invalid",
		AccountID: headlessTestAccount,
		Payload:   json.RawMessage(`{}`),
	})
	var statusError *jobs.StatusError
	if !errors.As(err, &statusError) || !statusError.Unrecoverable {
		t.Fatalf("erro = %#v, esperado unrecoverable", err)
	}
}

func validHeadlessPersistenceInput() HeadlessRefreshPersistence {
	envelope := relationshipProcessEnvelope()
	envelope.SnapshotID = headlessTestCapability
	envelope.AccountID = headlessTestAccount
	envelope.ClientAccountID = headlessTestClient
	envelope.SubjectID = headlessTestSubject
	envelope.RelationshipID = headlessTestRelationship
	raw := validNonConversationalProcessOutputs()["profile.summary"]
	return HeadlessRefreshPersistence{
		Scope: Scope{
			AccountID:       headlessTestAccount,
			ClientAccountID: headlessTestClient,
		},
		SubjectID:      headlessTestSubject,
		RelationshipID: headlessTestRelationship,
		ContextID:      headlessTestCapability,
		Context:        envelope,
		AsOf:           time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC),
		Executions: []HeadlessPersistedExecution{{
			RunRef: ProcessRunRef{
				ProcessKey:        "profile.summary",
				RunID:             "77777777-7777-4777-8777-777777777777",
				Status:            "succeeded",
				ExecutionMode:     "active",
				PromptBindingID:   "88888888-8888-4888-8888-888888888888",
				ContextSnapshotID: headlessTestCapability,
			},
			Output:           raw,
			OutputCiphertext: "v1:encrypted",
			OutputHash:       hashBytes(raw),
		}},
	}
}

func TestValidateHeadlessPersistenceRejectsShadowEffect(t *testing.T) {
	t.Parallel()
	input := validHeadlessPersistenceInput()
	input.Executions[0].RunRef.ExecutionMode = "shadow"
	if err := validateHeadlessPersistenceInput(input); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("shadow persistido: %v", err)
	}
}

func TestValidateHeadlessPersistenceRejectsOutputHashMismatch(t *testing.T) {
	t.Parallel()
	input := validHeadlessPersistenceInput()
	input.Executions[0].OutputHash = "tampered"
	if err := validateHeadlessPersistenceInput(input); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("hash divergente aceito: %v", err)
	}
}

func TestValidateHeadlessPersistenceRejectsDifferentContextSnapshot(t *testing.T) {
	t.Parallel()
	input := validHeadlessPersistenceInput()
	input.Context.SnapshotID = "99999999-9999-4999-8999-999999999999"
	if err := validateHeadlessPersistenceInput(input); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("contexto divergente aceito: %v", err)
	}
}
