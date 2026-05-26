package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// agencyAccess monta um AccessContext com todas as permissoes de agencia (admin). Util para
// casos em que so importa o perspective, nao a matriz de permissoes.
func agencyAccess() AccessContext {
	perms := make(map[string]struct{}, len(adminPermissions))
	for _, key := range adminPermissions {
		perms[key] = struct{}{}
	}
	return AccessContext{
		UserID:      "user-agency",
		AccountID:   "acc-agency",
		Perspective: PerspectiveAgency,
		Permissions: perms,
	}
}

// clientViewerAccess monta um AccessContext de cliente externo (tasks.client_view + comment).
// Sem PermBoardsManage para o perspective virar client_viewer no service.
func clientViewerAccess() AccessContext {
	perms := make(map[string]struct{}, len(clientViewerPermissions))
	for _, key := range clientViewerPermissions {
		perms[key] = struct{}{}
	}
	return AccessContext{
		UserID:      "user-client",
		AccountID:   "acc-client",
		Perspective: PerspectiveClientViewer,
		Permissions: perms,
	}
}

// noPermAccess monta um AccessContext na mesma account, mas SEM nenhuma permissao de tasks.
// Cobre o caso "user esta na account certa, mas falta perm" -> 403 (ErrForbidden), nao 404.
func noPermAccess() AccessContext {
	return AccessContext{
		UserID:      "user-zero",
		AccountID:   "acc-agency",
		Perspective: PerspectiveAgency,
		Permissions: map[string]struct{}{},
	}
}

func TestCreateTask_AgencyHappyPath(t *testing.T) {
	created := Task{
		ID:        "task-1",
		BoardID:   "board-1",
		AccountID: "acc-agency",
		Title:     "Renomear card",
		Priority:  "media",
		Version:   1,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	repository := &repositoryMock{
		onCreateTask: func(_ context.Context, accountID string, input CreateTaskInput, createdBy string) (Task, error) {
			if accountID != "acc-agency" {
				t.Errorf("CreateTask deve receber accountID da access; got %q", accountID)
			}
			if input.Title != "Renomear card" {
				t.Errorf("Title trimado deve chegar no repository; got %q", input.Title)
			}
			if createdBy != "user-agency" {
				t.Errorf("createdByUserID deve vir do access.UserID; got %q", createdBy)
			}
			return created, nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	dto, err := service.CreateTask(context.Background(), agencyAccess(), CreateTaskInput{
		BoardID: "board-1",
		Title:   "  Renomear card  ",
	})
	if err != nil {
		t.Fatalf("CreateTask deveria suceder; got err=%v", err)
	}
	if dto.ID != "task-1" {
		t.Errorf("DTO deve refletir a task criada; got %q", dto.ID)
	}
	if len(repository.auditEntries) != 1 || repository.auditEntries[0].Action != "task.created" {
		t.Errorf("CreateTask deve gerar audit 'task.created'; got %+v", repository.auditEntries)
	}
}

func TestCreateTask_ForwardsRoadmapLink(t *testing.T) {
	roadmapModuleID := "00000000-0000-0000-0000-000000000321"
	created := Task{
		ID:              "task-roadmap",
		BoardID:         "board-1",
		AccountID:       "acc-agency",
		Title:           "Publicar no roadmap",
		Priority:        "media",
		RoadmapModuleID: &roadmapModuleID,
		PinnedToRoadmap: true,
		Version:         1,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	repository := &repositoryMock{
		onCreateTask: func(_ context.Context, _ string, input CreateTaskInput, _ string) (Task, error) {
			if input.RoadmapModuleID == nil || *input.RoadmapModuleID != roadmapModuleID {
				t.Fatalf("CreateTask deve encaminhar roadmapModuleId normalizado; got %v", input.RoadmapModuleID)
			}
			if input.PinnedToRoadmap == nil || !*input.PinnedToRoadmap {
				t.Fatalf("CreateTask deve preservar pinnedToRoadmap quando ha modulo")
			}
			return created, nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	dto, err := service.CreateTask(context.Background(), agencyAccess(), CreateTaskInput{
		BoardID:         "board-1",
		Title:           "Publicar no roadmap",
		RoadmapModuleID: stringPtr("  " + roadmapModuleID + "  "),
	})
	if err != nil {
		t.Fatalf("CreateTask deveria suceder; got err=%v", err)
	}
	if dto.RoadmapModuleID == nil || *dto.RoadmapModuleID != roadmapModuleID || !dto.PinnedToRoadmap {
		t.Fatalf("DTO deve voltar com vinculo do roadmap; got module=%v pinned=%v", dto.RoadmapModuleID, dto.PinnedToRoadmap)
	}
}

func TestCreateTask_PinWithoutRoadmapModuleIsDisabled(t *testing.T) {
	repository := &repositoryMock{
		onCreateTask: func(_ context.Context, _ string, input CreateTaskInput, _ string) (Task, error) {
			if input.RoadmapModuleID != nil {
				t.Fatalf("roadmapModuleId vazio deve virar nil; got %v", input.RoadmapModuleID)
			}
			if input.PinnedToRoadmap == nil || *input.PinnedToRoadmap {
				t.Fatalf("pinnedToRoadmap nao deve ficar true sem modulo")
			}
			return Task{ID: "task-1", BoardID: "board-1", Title: "x", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	_, err := service.CreateTask(context.Background(), agencyAccess(), CreateTaskInput{
		BoardID:         "board-1",
		Title:           "x",
		RoadmapModuleID: stringPtr("   "),
		PinnedToRoadmap: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("CreateTask deveria suceder; got err=%v", err)
	}
}

func TestCreateTask_NoPermReturnsForbidden(t *testing.T) {
	repository := &repositoryMock{}
	service := NewService(repository, nil, nil, nil)

	_, err := service.CreateTask(context.Background(), noPermAccess(), CreateTaskInput{
		BoardID: "board-1",
		Title:   "x",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("user na account sem perm deve receber ErrForbidden (403); got %v", err)
	}
	if len(repository.auditEntries) != 0 {
		t.Errorf("nao deve gerar audit quando permissao falha; got %d entries", len(repository.auditEntries))
	}
}

func TestUpdateTask_ClearingRoadmapModuleDisablesPin(t *testing.T) {
	roadmapModuleID := "00000000-0000-0000-0000-000000000321"
	var clearedRoadmapModuleID *string
	repository := &repositoryMock{
		onGetTask: func(_ context.Context, _ AccessContext, _ string) (Task, error) {
			return Task{
				ID:              "task-1",
				BoardID:         "board-1",
				RoadmapModuleID: &roadmapModuleID,
				PinnedToRoadmap: true,
				CreatedAt:       time.Now().UTC(),
				UpdatedAt:       time.Now().UTC(),
			}, nil
		},
		onUpdateTask: func(_ context.Context, _ string, input UpdateTaskInput) (Task, error) {
			if input.RoadmapModuleID == nil || *input.RoadmapModuleID != nil {
				t.Fatalf("update deve solicitar limpeza do roadmapModuleId; got %v", input.RoadmapModuleID)
			}
			if input.PinnedToRoadmap == nil || *input.PinnedToRoadmap {
				t.Fatalf("limpar modulo deve forcar pinnedToRoadmap=false; got %v", input.PinnedToRoadmap)
			}
			return Task{ID: "task-1", BoardID: "board-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	_, err := service.UpdateTask(context.Background(), agencyAccess(), UpdateTaskInput{
		ID:              "task-1",
		RoadmapModuleID: &clearedRoadmapModuleID,
	})
	if err != nil {
		t.Fatalf("UpdateTask deveria suceder; got err=%v", err)
	}
}

func TestUpdateTask_SettingRoadmapModuleDefaultsPin(t *testing.T) {
	roadmapModuleID := "00000000-0000-0000-0000-000000000321"
	nextRoadmapModuleID := &roadmapModuleID
	repository := &repositoryMock{
		onGetTask: func(_ context.Context, _ AccessContext, _ string) (Task, error) {
			return Task{
				ID:        "task-1",
				BoardID:   "board-1",
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}, nil
		},
		onUpdateTask: func(_ context.Context, _ string, input UpdateTaskInput) (Task, error) {
			if input.RoadmapModuleID == nil || *input.RoadmapModuleID == nil || **input.RoadmapModuleID != roadmapModuleID {
				t.Fatalf("update deve encaminhar roadmapModuleId; got %v", input.RoadmapModuleID)
			}
			if input.PinnedToRoadmap == nil || !*input.PinnedToRoadmap {
				t.Fatalf("selecionar modulo deve fixar no roadmap por padrao; got %v", input.PinnedToRoadmap)
			}
			return Task{
				ID:              "task-1",
				BoardID:         "board-1",
				RoadmapModuleID: &roadmapModuleID,
				PinnedToRoadmap: true,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}, nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	dto, err := service.UpdateTask(context.Background(), agencyAccess(), UpdateTaskInput{
		ID:              "task-1",
		RoadmapModuleID: &nextRoadmapModuleID,
	})
	if err != nil {
		t.Fatalf("UpdateTask deveria suceder; got err=%v", err)
	}
	if dto.RoadmapModuleID == nil || *dto.RoadmapModuleID != roadmapModuleID || !dto.PinnedToRoadmap {
		t.Fatalf("DTO deve voltar fixada no roadmap; got module=%v pinned=%v", dto.RoadmapModuleID, dto.PinnedToRoadmap)
	}
}

func TestUpdateTask_CannotHideLinkedRoadmapTask(t *testing.T) {
	roadmapModuleID := "00000000-0000-0000-0000-000000000321"
	repository := &repositoryMock{
		onGetTask: func(_ context.Context, _ AccessContext, _ string) (Task, error) {
			return Task{
				ID:              "task-1",
				BoardID:         "board-1",
				RoadmapModuleID: &roadmapModuleID,
				PinnedToRoadmap: true,
				CreatedAt:       time.Now().UTC(),
				UpdatedAt:       time.Now().UTC(),
			}, nil
		},
		onUpdateTask: func(_ context.Context, _ string, input UpdateTaskInput) (Task, error) {
			if input.PinnedToRoadmap == nil || !*input.PinnedToRoadmap {
				t.Fatalf("task vinculada a modulo do roadmap deve continuar visivel; got %v", input.PinnedToRoadmap)
			}
			return Task{
				ID:              "task-1",
				BoardID:         "board-1",
				RoadmapModuleID: &roadmapModuleID,
				PinnedToRoadmap: true,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}, nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	_, err := service.UpdateTask(context.Background(), agencyAccess(), UpdateTaskInput{
		ID:              "task-1",
		PinnedToRoadmap: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("UpdateTask deveria suceder; got err=%v", err)
	}
}

func TestUpdateTaskInput_UnmarshalRoadmapNullMarksFieldPresent(t *testing.T) {
	var input UpdateTaskInput
	if err := json.Unmarshal([]byte(`{"roadmapModuleId":null,"pinnedToRoadmap":false}`), &input); err != nil {
		t.Fatalf("decode UpdateTaskInput: %v", err)
	}
	if input.RoadmapModuleID == nil {
		t.Fatalf("roadmapModuleId null deve ser distinguido de campo ausente")
	}
	if *input.RoadmapModuleID != nil {
		t.Fatalf("roadmapModuleId null deve virar ponteiro interno nil; got %v", *input.RoadmapModuleID)
	}
	if input.PinnedToRoadmap == nil || *input.PinnedToRoadmap {
		t.Fatalf("pinnedToRoadmap=false deve ser preservado; got %v", input.PinnedToRoadmap)
	}
}

func TestCreateTask_ValidationEmptyTitle(t *testing.T) {
	service := NewService(&repositoryMock{}, nil, nil, nil)
	_, err := service.CreateTask(context.Background(), agencyAccess(), CreateTaskInput{
		BoardID: "board-1",
		Title:   "   ",
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("title vazio deve retornar ErrValidation; got %v", err)
	}
}

func TestGetTask_PerspectiveControlsClientAccount(t *testing.T) {
	clientAccountID := "client-acc-1"
	taskAtBackend := Task{
		ID:              "task-1",
		BoardID:         "board-1",
		AccountID:       "acc-agency",
		ClientAccountID: &clientAccountID,
		Version:         3,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	repository := &repositoryMock{
		onGetTask: func(_ context.Context, _ AccessContext, _ string) (Task, error) {
			return taskAtBackend, nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	dtoAgency, err := service.GetTask(context.Background(), agencyAccess(), "task-1")
	if err != nil {
		t.Fatalf("agency GetTask: %v", err)
	}
	if dtoAgency.ClientAccountID == nil {
		t.Errorf("agency deve receber clientAccountId no DTO")
	}

	dtoClient, err := service.GetTask(context.Background(), clientViewerAccess(), "task-1")
	if err != nil {
		t.Fatalf("client_viewer GetTask: %v", err)
	}
	if dtoClient.ClientAccountID != nil {
		t.Errorf("client_viewer NAO deve receber clientAccountId; got %q", *dtoClient.ClientAccountID)
	}
}

func TestGetTask_NotFoundPassesThrough(t *testing.T) {
	repository := &repositoryMock{
		onGetTask: func(_ context.Context, _ AccessContext, _ string) (Task, error) {
			return Task{}, ErrTaskNotFound
		},
	}
	service := NewService(repository, nil, nil, nil)

	_, err := service.GetTask(context.Background(), agencyAccess(), "task-nao-existe")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("repository ErrTaskNotFound deve subir intacto -> 404 no HTTP; got %v", err)
	}
}

func TestListTasks_AppliesDefaultLimit(t *testing.T) {
	var capturedLimit int
	repository := &repositoryMock{
		onListTasks: func(_ context.Context, _ AccessContext, input ListTasksInput) ([]Task, string, error) {
			capturedLimit = input.Limit
			return []Task{}, "", nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	_, err := service.ListTasks(context.Background(), agencyAccess(), ListTasksInput{BoardID: "b"})
	if err != nil {
		t.Fatalf("ListTasks erro inesperado: %v", err)
	}
	if capturedLimit != 50 {
		t.Errorf("limit ausente deve virar 50 (default); got %d", capturedLimit)
	}
}

func TestListTasks_ClampsTooLargeLimit(t *testing.T) {
	var capturedLimit int
	repository := &repositoryMock{
		onListTasks: func(_ context.Context, _ AccessContext, input ListTasksInput) ([]Task, string, error) {
			capturedLimit = input.Limit
			return []Task{}, "", nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	_, _ = service.ListTasks(context.Background(), agencyAccess(), ListTasksInput{BoardID: "b", Limit: 9999})
	if capturedLimit != 50 {
		t.Errorf("limit > 200 deve virar 50 (default); got %d", capturedLimit)
	}
}

func TestListTasks_NoPermReturnsForbidden(t *testing.T) {
	service := NewService(&repositoryMock{}, nil, nil, nil)
	_, err := service.ListTasks(context.Background(), noPermAccess(), ListTasksInput{BoardID: "b"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListTasks sem perm tasks.view deve retornar ErrForbidden; got %v", err)
	}
}

func TestListTasks_PerspectivePropagates(t *testing.T) {
	clientAccountID := "client-1"
	repository := &repositoryMock{
		onListTasks: func(_ context.Context, _ AccessContext, _ ListTasksInput) ([]Task, string, error) {
			return []Task{
				{ID: "t1", BoardID: "b", ClientAccountID: &clientAccountID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			}, "", nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	resAgency, err := service.ListTasks(context.Background(), agencyAccess(), ListTasksInput{BoardID: "b"})
	if err != nil {
		t.Fatalf("agency ListTasks: %v", err)
	}
	if len(resAgency.Tasks) != 1 || resAgency.Tasks[0].ClientAccountID == nil {
		t.Errorf("agency deve manter clientAccountId em todos os itens")
	}

	resClient, err := service.ListTasks(context.Background(), clientViewerAccess(), ListTasksInput{BoardID: "b"})
	if err != nil {
		t.Fatalf("client_viewer ListTasks: %v", err)
	}
	if len(resClient.Tasks) != 1 || resClient.Tasks[0].ClientAccountID != nil {
		t.Errorf("client_viewer NAO pode receber clientAccountId; got %+v", resClient.Tasks[0])
	}
}

func TestListTasks_PropagatesNextCursor(t *testing.T) {
	repository := &repositoryMock{
		onListTasks: func(_ context.Context, _ AccessContext, _ ListTasksInput) ([]Task, string, error) {
			return []Task{}, "next-cursor-abc", nil
		},
	}
	service := NewService(repository, nil, nil, nil)

	res, err := service.ListTasks(context.Background(), agencyAccess(), ListTasksInput{BoardID: "b"})
	if err != nil {
		t.Fatalf("ListTasks erro: %v", err)
	}
	if res.NextCursor != "next-cursor-abc" {
		t.Errorf("NextCursor do repository deve aparecer no Result; got %q", res.NextCursor)
	}
}
