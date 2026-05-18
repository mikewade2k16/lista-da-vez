package tasks

import (
	"context"
)

// repositoryMock satisfaz a interface `Repository` com hooks opcionais por metodo. Cada teste
// preenche so os hooks que precisa; o resto devolve zero values sem erro. Reduz boilerplate
// versus mockar todos os 30+ metodos individualmente em cada arquivo de teste.
//
// Os testes T9 sao unit puros — nao tocam Postgres. Para integration test com DB real, ver
// docs/TASKS_ORCHESTRATOR_PHASE12.md (smoke E2E manual).
type repositoryMock struct {
	onAccountExists                func(ctx context.Context, accountID string) (bool, error)
	onIsAccountMember              func(ctx context.Context, accountID, userID string) (bool, error)
	onListPermissionsForUser       func(ctx context.Context, accountID, userID string) ([]string, error)
	onFindOrganizationIDForAccount func(ctx context.Context, accountID string) (*string, error)

	onListBoards  func(ctx context.Context, access AccessContext) ([]Board, error)
	onGetBoard    func(ctx context.Context, access AccessContext, boardID string) (Board, error)
	onCreateBoard func(ctx context.Context, accountID string, input CreateBoardInput, createdByUserID string, organizationID *string) (Board, error)
	onUpdateBoard func(ctx context.Context, accountID string, input UpdateBoardInput) (Board, error)

	onCreateColumn func(ctx context.Context, accountID string, input CreateColumnInput) (Column, error)
	onUpdateColumn func(ctx context.Context, accountID string, input UpdateColumnInput) (Column, error)
	onDeleteColumn func(ctx context.Context, accountID string, input DeleteColumnInput) (string, error)
	onCreateField  func(ctx context.Context, accountID string, input CreateFieldInput) (Field, error)

	onListTasks   func(ctx context.Context, access AccessContext, input ListTasksInput) ([]Task, string, error)
	onGetTask     func(ctx context.Context, access AccessContext, taskID string) (Task, error)
	onCreateTask  func(ctx context.Context, accountID string, input CreateTaskInput, createdByUserID string) (Task, error)
	onUpdateTask  func(ctx context.Context, accountID string, input UpdateTaskInput) (Task, error)
	onMoveTask    func(ctx context.Context, accountID string, input MoveTaskInput) (Task, error)
	onArchiveTask func(ctx context.Context, accountID, taskID string) (Task, error)

	onAddComment             func(ctx context.Context, accountID string, input AddCommentInput, authorUserID string) (Comment, error)
	onAddCommentMentions     func(ctx context.Context, accountID, taskID, commentID string, mentionedUserIDs []string) ([]string, error)
	onListComments           func(ctx context.Context, access AccessContext, taskID string) ([]Comment, error)
	onUpsertSubscribers      func(ctx context.Context, accountID, taskID string, userIDs []string) error
	onListSubscriberUserIDs  func(ctx context.Context, accountID, taskID string) ([]string, error)
	onAddShare               func(ctx context.Context, accountID string, input AddShareInput, sharedByUserID string) (Share, error)
	onListRelations          func(ctx context.Context, access AccessContext, taskID string) ([]Relation, error)
	onAddRelation            func(ctx context.Context, accountID string, input AddRelationInput) (Relation, error)
	onListAudit              func(ctx context.Context, accountID, taskID string) ([]AuditEntry, error)
	auditEntries             []AuditEntry // captura passive das audit entries para verificacao
	onInsertAuditEntry       func(ctx context.Context, entry AuditEntry) error
	onListActiveTimeEntries  func(ctx context.Context, access AccessContext) ([]TimeEntry, error)
	onStartTracking          func(ctx context.Context, accountID, taskID, userID string) (TimeEntry, error)
	onPauseTracking          func(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error)
	onResumeTracking         func(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error)
	onStopTracking           func(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error)
	onTrackingMetrics        func(ctx context.Context, accountID string, input TrackingMetricsInput) (TrackingMetrics, error)
}

func (m *repositoryMock) AccountExists(ctx context.Context, accountID string) (bool, error) {
	if m.onAccountExists != nil {
		return m.onAccountExists(ctx, accountID)
	}
	return true, nil
}

func (m *repositoryMock) IsAccountMember(ctx context.Context, accountID, userID string) (bool, error) {
	if m.onIsAccountMember != nil {
		return m.onIsAccountMember(ctx, accountID, userID)
	}
	return true, nil
}

func (m *repositoryMock) ListPermissionsForUser(ctx context.Context, accountID, userID string) ([]string, error) {
	if m.onListPermissionsForUser != nil {
		return m.onListPermissionsForUser(ctx, accountID, userID)
	}
	return []string{}, nil
}

func (m *repositoryMock) FindOrganizationIDForAccount(ctx context.Context, accountID string) (*string, error) {
	if m.onFindOrganizationIDForAccount != nil {
		return m.onFindOrganizationIDForAccount(ctx, accountID)
	}
	return nil, nil
}

func (m *repositoryMock) ListBoards(ctx context.Context, access AccessContext) ([]Board, error) {
	if m.onListBoards != nil {
		return m.onListBoards(ctx, access)
	}
	return []Board{}, nil
}

func (m *repositoryMock) GetBoard(ctx context.Context, access AccessContext, boardID string) (Board, error) {
	if m.onGetBoard != nil {
		return m.onGetBoard(ctx, access, boardID)
	}
	return Board{}, ErrBoardNotFound
}

func (m *repositoryMock) CreateBoard(ctx context.Context, accountID string, input CreateBoardInput, createdByUserID string, organizationID *string) (Board, error) {
	if m.onCreateBoard != nil {
		return m.onCreateBoard(ctx, accountID, input, createdByUserID, organizationID)
	}
	return Board{}, nil
}

func (m *repositoryMock) UpdateBoard(ctx context.Context, accountID string, input UpdateBoardInput) (Board, error) {
	if m.onUpdateBoard != nil {
		return m.onUpdateBoard(ctx, accountID, input)
	}
	return Board{}, nil
}

func (m *repositoryMock) CreateColumn(ctx context.Context, accountID string, input CreateColumnInput) (Column, error) {
	if m.onCreateColumn != nil {
		return m.onCreateColumn(ctx, accountID, input)
	}
	return Column{}, nil
}

func (m *repositoryMock) UpdateColumn(ctx context.Context, accountID string, input UpdateColumnInput) (Column, error) {
	if m.onUpdateColumn != nil {
		return m.onUpdateColumn(ctx, accountID, input)
	}
	return Column{}, nil
}

func (m *repositoryMock) DeleteColumn(ctx context.Context, accountID string, input DeleteColumnInput) (string, error) {
	if m.onDeleteColumn != nil {
		return m.onDeleteColumn(ctx, accountID, input)
	}
	return "", nil
}

func (m *repositoryMock) CreateField(ctx context.Context, accountID string, input CreateFieldInput) (Field, error) {
	if m.onCreateField != nil {
		return m.onCreateField(ctx, accountID, input)
	}
	return Field{}, nil
}

func (m *repositoryMock) ListTasks(ctx context.Context, access AccessContext, input ListTasksInput) ([]Task, string, error) {
	if m.onListTasks != nil {
		return m.onListTasks(ctx, access, input)
	}
	return []Task{}, "", nil
}

func (m *repositoryMock) GetTask(ctx context.Context, access AccessContext, taskID string) (Task, error) {
	if m.onGetTask != nil {
		return m.onGetTask(ctx, access, taskID)
	}
	return Task{}, ErrTaskNotFound
}

func (m *repositoryMock) CreateTask(ctx context.Context, accountID string, input CreateTaskInput, createdByUserID string) (Task, error) {
	if m.onCreateTask != nil {
		return m.onCreateTask(ctx, accountID, input, createdByUserID)
	}
	return Task{}, nil
}

func (m *repositoryMock) UpdateTask(ctx context.Context, accountID string, input UpdateTaskInput) (Task, error) {
	if m.onUpdateTask != nil {
		return m.onUpdateTask(ctx, accountID, input)
	}
	return Task{}, nil
}

func (m *repositoryMock) MoveTask(ctx context.Context, accountID string, input MoveTaskInput) (Task, error) {
	if m.onMoveTask != nil {
		return m.onMoveTask(ctx, accountID, input)
	}
	return Task{}, nil
}

func (m *repositoryMock) ArchiveTask(ctx context.Context, accountID, taskID string) (Task, error) {
	if m.onArchiveTask != nil {
		return m.onArchiveTask(ctx, accountID, taskID)
	}
	return Task{}, nil
}

func (m *repositoryMock) AddComment(ctx context.Context, accountID string, input AddCommentInput, authorUserID string) (Comment, error) {
	if m.onAddComment != nil {
		return m.onAddComment(ctx, accountID, input, authorUserID)
	}
	return Comment{}, nil
}

func (m *repositoryMock) AddCommentMentions(ctx context.Context, accountID, taskID, commentID string, mentionedUserIDs []string) ([]string, error) {
	if m.onAddCommentMentions != nil {
		return m.onAddCommentMentions(ctx, accountID, taskID, commentID, mentionedUserIDs)
	}
	return []string{}, nil
}

func (m *repositoryMock) ListComments(ctx context.Context, access AccessContext, taskID string) ([]Comment, error) {
	if m.onListComments != nil {
		return m.onListComments(ctx, access, taskID)
	}
	return []Comment{}, nil
}

func (m *repositoryMock) UpsertSubscribers(ctx context.Context, accountID, taskID string, userIDs []string) error {
	if m.onUpsertSubscribers != nil {
		return m.onUpsertSubscribers(ctx, accountID, taskID, userIDs)
	}
	return nil
}

func (m *repositoryMock) ListSubscriberUserIDs(ctx context.Context, accountID, taskID string) ([]string, error) {
	if m.onListSubscriberUserIDs != nil {
		return m.onListSubscriberUserIDs(ctx, accountID, taskID)
	}
	return []string{}, nil
}

func (m *repositoryMock) AddShare(ctx context.Context, accountID string, input AddShareInput, sharedByUserID string) (Share, error) {
	if m.onAddShare != nil {
		return m.onAddShare(ctx, accountID, input, sharedByUserID)
	}
	return Share{}, nil
}

func (m *repositoryMock) ListRelations(ctx context.Context, access AccessContext, taskID string) ([]Relation, error) {
	if m.onListRelations != nil {
		return m.onListRelations(ctx, access, taskID)
	}
	return []Relation{}, nil
}

func (m *repositoryMock) AddRelation(ctx context.Context, accountID string, input AddRelationInput) (Relation, error) {
	if m.onAddRelation != nil {
		return m.onAddRelation(ctx, accountID, input)
	}
	return Relation{}, nil
}

func (m *repositoryMock) ListAudit(ctx context.Context, accountID, taskID string) ([]AuditEntry, error) {
	if m.onListAudit != nil {
		return m.onListAudit(ctx, accountID, taskID)
	}
	return []AuditEntry{}, nil
}

func (m *repositoryMock) InsertAuditEntry(ctx context.Context, entry AuditEntry) error {
	m.auditEntries = append(m.auditEntries, entry)
	if m.onInsertAuditEntry != nil {
		return m.onInsertAuditEntry(ctx, entry)
	}
	return nil
}

func (m *repositoryMock) ListActiveTimeEntries(ctx context.Context, access AccessContext) ([]TimeEntry, error) {
	if m.onListActiveTimeEntries != nil {
		return m.onListActiveTimeEntries(ctx, access)
	}
	return []TimeEntry{}, nil
}

func (m *repositoryMock) StartTracking(ctx context.Context, accountID, taskID, userID string) (TimeEntry, error) {
	if m.onStartTracking != nil {
		return m.onStartTracking(ctx, accountID, taskID, userID)
	}
	return TimeEntry{}, nil
}

func (m *repositoryMock) PauseTracking(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error) {
	if m.onPauseTracking != nil {
		return m.onPauseTracking(ctx, accountID, taskID, userID, expectedVersion)
	}
	return TimeEntry{}, nil
}

func (m *repositoryMock) ResumeTracking(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error) {
	if m.onResumeTracking != nil {
		return m.onResumeTracking(ctx, accountID, taskID, userID, expectedVersion)
	}
	return TimeEntry{}, nil
}

func (m *repositoryMock) StopTracking(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error) {
	if m.onStopTracking != nil {
		return m.onStopTracking(ctx, accountID, taskID, userID, expectedVersion)
	}
	return TimeEntry{}, nil
}

func (m *repositoryMock) TrackingMetrics(ctx context.Context, accountID string, input TrackingMetricsInput) (TrackingMetrics, error) {
	if m.onTrackingMetrics != nil {
		return m.onTrackingMetrics(ctx, accountID, input)
	}
	return TrackingMetrics{}, nil
}
