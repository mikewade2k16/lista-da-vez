package tasks

import (
	"context"
	"errors"
	"testing"
	"time"
)

// trackingAccessFor monta um AccessContext com PermTrackingUse (membro padrao). Usado para os
// fluxos de start/pause/resume/stop.
func trackingAccessFor(accountID, userID string) AccessContext {
	return AccessContext{
		UserID:    userID,
		AccountID: accountID,
		Perspective: PerspectiveAgency,
		Permissions: map[string]struct{}{
			PermTrackingUse: {},
			PermTasksView:   {},
		},
	}
}

func TestStartTracking_NoPermReturnsForbidden(t *testing.T) {
	service := NewService(&repositoryMock{}, nil, nil, nil)
	access := AccessContext{UserID: "u", AccountID: "acc", Permissions: map[string]struct{}{}}
	_, err := service.StartTracking(context.Background(), access, "task-1")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("StartTracking sem PermTrackingUse deve ser 403; got %v", err)
	}
}

func TestStartTracking_TaskNotFoundReturns404(t *testing.T) {
	repository := &repositoryMock{
		onGetTask: func(_ context.Context, _ AccessContext, _ string) (Task, error) {
			return Task{}, ErrTaskNotFound
		},
	}
	service := NewService(repository, nil, nil, nil)
	_, err := service.StartTracking(context.Background(), trackingAccessFor("acc", "u"), "task-fora-da-account")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("task de outra account passa por GetTask -> 404; got %v", err)
	}
}

func TestStartTracking_HappyPathPublishesAndAudits(t *testing.T) {
	now := time.Now().UTC()
	repository := &repositoryMock{
		onGetTask: func(_ context.Context, _ AccessContext, _ string) (Task, error) {
			return Task{ID: "task-1", BoardID: "board-1", Version: 5, CreatedAt: now, UpdatedAt: now}, nil
		},
		onStartTracking: func(_ context.Context, accountID, taskID, userID string) (TimeEntry, error) {
			if accountID != "acc" || taskID != "task-1" || userID != "u" {
				t.Errorf("StartTracking recebeu args errados: account=%q task=%q user=%q", accountID, taskID, userID)
			}
			return TimeEntry{ID: "te-1", TaskID: "task-1", UserID: "u", StartedAt: now}, nil
		},
	}

	publisher := &capturingPublisher{}
	service := NewService(repository, publisher, nil, nil)

	entry, err := service.StartTracking(context.Background(), trackingAccessFor("acc", "u"), "task-1")
	if err != nil {
		t.Fatalf("StartTracking: %v", err)
	}
	if entry.ID != "te-1" {
		t.Errorf("retorno deve refletir TimeEntry; got %+v", entry)
	}
	if len(publisher.events) != 1 || publisher.events[0].Type != "task.time_started" {
		t.Errorf("StartTracking deve publicar 'task.time_started'; got %+v", publisher.events)
	}
	if len(repository.auditEntries) != 1 || repository.auditEntries[0].Action != "task.time_started" {
		t.Errorf("StartTracking deve auditar 'task.time_started'; got %+v", repository.auditEntries)
	}
}

func TestPauseTracking_VersionConflictBubblesUp(t *testing.T) {
	repository := &repositoryMock{
		onGetTask: func(_ context.Context, _ AccessContext, _ string) (Task, error) {
			return Task{ID: "task-1", BoardID: "b", Version: 3, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
		},
		onPauseTracking: func(_ context.Context, _, _, _ string, expectedVersion *int) (TimeEntry, error) {
			if expectedVersion == nil || *expectedVersion != 3 {
				t.Errorf("expectedVersion deve chegar intacto no repository; got %v", expectedVersion)
			}
			return TimeEntry{}, ErrVersionConflict
		},
	}
	service := NewService(repository, nil, nil, nil)

	expected := 3
	_, err := service.PauseTracking(context.Background(), trackingAccessFor("acc", "u"), "task-1", &expected)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("conflict no repository deve subir para PauseTracking -> 409 no HTTP; got %v", err)
	}
}

func TestResumeTracking_PassesExpectedVersionToRepo(t *testing.T) {
	var captured *int
	repository := &repositoryMock{
		onGetTask: func(_ context.Context, _ AccessContext, _ string) (Task, error) {
			return Task{ID: "task-1", BoardID: "b", Version: 7, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
		},
		onResumeTracking: func(_ context.Context, _, _, _ string, expectedVersion *int) (TimeEntry, error) {
			captured = expectedVersion
			return TimeEntry{ID: "te-r"}, nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	expected := 7
	_, err := service.ResumeTracking(context.Background(), trackingAccessFor("acc", "u"), "task-1", &expected)
	if err != nil {
		t.Fatalf("ResumeTracking: %v", err)
	}
	if captured == nil || *captured != 7 {
		t.Errorf("expectedVersion=7 deve chegar no repository; got %v", captured)
	}
}

func TestStopTracking_TaskNotFoundDoesNotPublish(t *testing.T) {
	publisher := &capturingPublisher{}
	repository := &repositoryMock{
		onGetTask: func(_ context.Context, _ AccessContext, _ string) (Task, error) {
			return Task{}, ErrTaskNotFound
		},
	}
	service := NewService(repository, publisher, nil, nil)

	_, err := service.StopTracking(context.Background(), trackingAccessFor("acc", "u"), "task-?", nil)
	if !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("got %v", err)
	}
	if len(publisher.events) != 0 {
		t.Errorf("nao deve publicar evento se task nao existe; got %d", len(publisher.events))
	}
	if len(repository.auditEntries) != 0 {
		t.Errorf("nao deve auditar se task nao existe; got %d", len(repository.auditEntries))
	}
}

func TestListActiveTimeEntries_AcceptsViewAll(t *testing.T) {
	called := false
	repository := &repositoryMock{
		onListActiveTimeEntries: func(_ context.Context, _ AccessContext) ([]TimeEntry, error) {
			called = true
			return []TimeEntry{}, nil
		},
	}
	service := NewService(repository, nil, nil, nil)
	access := AccessContext{
		UserID: "u", AccountID: "acc",
		Permissions: map[string]struct{}{PermTrackingViewAll: {}},
	}
	if _, err := service.ListActiveTimeEntries(context.Background(), access); err != nil {
		t.Fatalf("ListActiveTimeEntries: %v", err)
	}
	if !called {
		t.Error("repository.ListActiveTimeEntries deveria ter sido chamado")
	}
}

func TestListActiveTimeEntries_RejectsNoPerm(t *testing.T) {
	service := NewService(&repositoryMock{}, nil, nil, nil)
	access := AccessContext{UserID: "u", AccountID: "acc", Permissions: map[string]struct{}{}}
	_, err := service.ListActiveTimeEntries(context.Background(), access)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("sem PermTrackingUse e nem ViewAll -> 403; got %v", err)
	}
}

// capturingPublisher e' um Publisher que armazena os eventos publicados. Usado nos testes de
// tracking/relations para asserts sem precisar de WebSocket real.
type capturingPublisher struct {
	events    []TaskEvent
	boards    []BoardEvent
	presences []PresenceEvent
}

func (p *capturingPublisher) PublishTaskEvent(_ context.Context, event TaskEvent) {
	p.events = append(p.events, event)
}

func (p *capturingPublisher) PublishBoardEvent(_ context.Context, event BoardEvent) {
	p.boards = append(p.boards, event)
}

func (p *capturingPublisher) PublishPresenceEvent(_ context.Context, event PresenceEvent) {
	p.presences = append(p.presences, event)
}
