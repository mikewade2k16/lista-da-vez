package tasks

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/notifications"
	platformmodules "github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

type Service struct {
	repository Repository
	publisher  Publisher
	notifier   notifications.Notifier
	relations  *platformmodules.RelationRegistry
	logger     *slog.Logger
}

func NewService(repository Repository, publisher Publisher, notifier notifications.Notifier, relationRegistry *platformmodules.RelationRegistry) *Service {
	if publisher == nil {
		publisher = noopPublisher{}
	}
	if notifier == nil {
		notifier = notifications.NewNoopNotifier()
	}
	return &Service{
		repository: repository,
		publisher:  publisher,
		notifier:   notifier,
		relations:  relationRegistry,
		logger:     slog.Default(),
	}
}

// SetLogger injeta o slog do modulo (com `app_name` ja em atributos). Quando nao for chamado,
// o servico cai para `slog.Default()`. Usar `tasks.New(...).Build(deps)` no Module Registry
// passa o `deps.Logger` automaticamente.
func (service *Service) SetLogger(logger *slog.Logger) {
	if logger == nil {
		service.logger = slog.Default()
		return
	}
	service.logger = logger.With(slog.String("module", "tasks"))
}

// logMutation registra mutations criticas com atributos estruturados — accountId/userId/action
// e o par (resourceType, resourceId). T8 exige: nunca expor IDs de outras accounts (na pratica,
// `scopedQuery` ja garantiu o filtro antes do log). Nao logar payload completo — comentarios e
// titulos podem ter PII.
func (service *Service) logMutation(ctx context.Context, access AccessContext, action, resourceType, resourceID string, extra ...slog.Attr) {
	if service.logger == nil {
		return
	}
	attrs := make([]any, 0, 4+len(extra))
	attrs = append(attrs,
		slog.String("action", action),
		slog.String("account_id", access.AccountID),
		slog.String("user_id", access.UserID),
	)
	if resourceType != "" {
		attrs = append(attrs, slog.String("resource_type", resourceType))
	}
	if resourceID != "" {
		attrs = append(attrs, slog.String("resource_id", resourceID))
	}
	for _, attr := range extra {
		attrs = append(attrs, attr)
	}
	service.logger.LogAttrs(ctx, slog.LevelInfo, "tasks.mutation", convertAnyToAttrs(attrs)...)
}

// convertAnyToAttrs converte `[]any` (misturado com slog.Attr) em `[]slog.Attr` para
// `LogAttrs`. Helper interno — mantemos os call sites com varargs simples.
func convertAnyToAttrs(items []any) []slog.Attr {
	out := make([]slog.Attr, 0, len(items))
	for _, item := range items {
		if attr, ok := item.(slog.Attr); ok {
			out = append(out, attr)
		}
	}
	return out
}

func (service *Service) ResolveAccessContext(ctx context.Context, principal auth.Principal, accountID string) (AccessContext, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return AccessContext{}, ErrAccountRequired
	}

	exists, err := service.repository.AccountExists(ctx, accountID)
	if err != nil {
		return AccessContext{}, err
	}
	if !exists {
		return AccessContext{}, ErrAccountNotFound
	}

	isPlatformAdmin := principal.Role == auth.RolePlatformAdmin
	if !isPlatformAdmin {
		isMember, err := service.repository.IsAccountMember(ctx, accountID, principal.UserID)
		if err != nil {
			return AccessContext{}, err
		}
		if !isMember {
			return AccessContext{}, ErrAccountNotFound
		}
	}

	permissionKeys := []string{}
	if isPlatformAdmin {
		permissionKeys = adminPermissions
	} else {
		permissionKeys, err = service.repository.ListPermissionsForUser(ctx, accountID, principal.UserID)
		if err != nil {
			return AccessContext{}, err
		}
	}

	permissions := make(map[string]struct{}, len(permissionKeys))
	for _, key := range permissionKeys {
		normalized := strings.TrimSpace(key)
		if normalized != "" {
			permissions[normalized] = struct{}{}
		}
	}

	perspective := PerspectiveAgency
	if _, clientView := permissions[PermClientView]; clientView {
		if _, manageBoards := permissions[PermBoardsManage]; !manageBoards && !isPlatformAdmin {
			perspective = PerspectiveClientViewer
		}
	}

	return AccessContext{
		UserID:          strings.TrimSpace(principal.UserID),
		AccountID:       accountID,
		IsPlatformAdmin: isPlatformAdmin,
		Perspective:     perspective,
		Permissions:     permissions,
	}, nil
}

func (service *Service) ListBoards(ctx context.Context, access AccessContext) ([]Board, error) {
	if !access.Has(PermBoardsView) {
		return nil, ErrForbidden
	}
	return service.repository.ListBoards(ctx, access)
}

func (service *Service) GetBoard(ctx context.Context, access AccessContext, boardID string) (Board, error) {
	if !access.Has(PermBoardsView) {
		return Board{}, ErrForbidden
	}
	return service.repository.GetBoard(ctx, access, strings.TrimSpace(boardID))
}

func (service *Service) CreateBoard(ctx context.Context, access AccessContext, input CreateBoardInput) (Board, error) {
	if !access.Has(PermBoardsManage) {
		return Board{}, ErrForbidden
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = normalizeSlug(input.Slug, input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Icon = strings.TrimSpace(input.Icon)
	if input.Name == "" || input.Slug == "" {
		return Board{}, ErrValidation
	}

	organizationID, err := service.repository.FindOrganizationIDForAccount(ctx, access.AccountID)
	if err != nil {
		return Board{}, err
	}
	board, err := service.repository.CreateBoard(ctx, access.AccountID, input, access.UserID, organizationID)
	if err != nil {
		return Board{}, err
	}

	service.audit(ctx, access, "board.created", "board", board.ID, nil, board)
	service.publisher.PublishBoardEvent(ctx, BoardEvent{Type: "board.created", AccountID: access.AccountID, BoardID: board.ID})
	return board, nil
}

func (service *Service) UpdateBoard(ctx context.Context, access AccessContext, input UpdateBoardInput) (Board, error) {
	if !access.Has(PermBoardsManage) {
		return Board{}, ErrForbidden
	}
	input.ID = strings.TrimSpace(input.ID)
	if input.ID == "" {
		return Board{}, ErrValidation
	}
	if input.Name != nil {
		*input.Name = strings.TrimSpace(*input.Name)
		if *input.Name == "" {
			return Board{}, ErrValidation
		}
	}
	if input.Slug != nil {
		*input.Slug = normalizeSlug(*input.Slug, "")
		if *input.Slug == "" {
			return Board{}, ErrValidation
		}
	}

	before, err := service.repository.GetBoard(ctx, access, input.ID)
	if err != nil {
		return Board{}, err
	}
	after, err := service.repository.UpdateBoard(ctx, access.AccountID, input)
	if err != nil {
		return Board{}, err
	}

	service.audit(ctx, access, "board.updated", "board", after.ID, before, after)
	service.publisher.PublishBoardEvent(ctx, BoardEvent{Type: "board.updated", AccountID: access.AccountID, BoardID: after.ID})
	return after, nil
}

func (service *Service) CreateColumn(ctx context.Context, access AccessContext, input CreateColumnInput) (Column, error) {
	if !access.Has(PermBoardsManage) {
		return Column{}, ErrForbidden
	}
	input.BoardID = strings.TrimSpace(input.BoardID)
	input.Label = strings.TrimSpace(input.Label)
	input.Color = defaultString(strings.TrimSpace(input.Color), "slate")
	if input.BoardID == "" || input.Label == "" {
		return Column{}, ErrValidation
	}

	column, err := service.repository.CreateColumn(ctx, access.AccountID, input)
	if err != nil {
		return Column{}, err
	}
	service.audit(ctx, access, "board.column_added", "column", column.ID, nil, column)
	service.publisher.PublishBoardEvent(ctx, BoardEvent{Type: "board.column_added", AccountID: access.AccountID, BoardID: column.BoardID})
	return column, nil
}

func (service *Service) UpdateColumn(ctx context.Context, access AccessContext, input UpdateColumnInput) (Column, error) {
	if !access.Has(PermBoardsManage) {
		return Column{}, ErrForbidden
	}
	input.ID = strings.TrimSpace(input.ID)
	if input.ID == "" {
		return Column{}, ErrValidation
	}
	if input.Label != nil {
		*input.Label = strings.TrimSpace(*input.Label)
		if *input.Label == "" {
			return Column{}, ErrValidation
		}
	}
	if input.Color != nil {
		*input.Color = defaultString(strings.TrimSpace(*input.Color), "slate")
	}

	column, err := service.repository.UpdateColumn(ctx, access.AccountID, input)
	if err != nil {
		return Column{}, err
	}
	service.audit(ctx, access, "board.column_updated", "column", column.ID, nil, column)
	service.publisher.PublishBoardEvent(ctx, BoardEvent{Type: "board.column_updated", AccountID: access.AccountID, BoardID: column.BoardID})
	return column, nil
}

func (service *Service) DeleteColumn(ctx context.Context, access AccessContext, input DeleteColumnInput) error {
	if !access.Has(PermBoardsManage) {
		return ErrForbidden
	}
	input.ID = strings.TrimSpace(input.ID)
	input.RemapToColumnID = strings.TrimSpace(input.RemapToColumnID)
	if input.ID == "" {
		return ErrValidation
	}

	boardID, err := service.repository.DeleteColumn(ctx, access.AccountID, input)
	if err != nil {
		return err
	}
	service.audit(ctx, access, "board.column_deleted", "column", input.ID, nil, nil)
	service.publisher.PublishBoardEvent(ctx, BoardEvent{Type: "board.column_deleted", AccountID: access.AccountID, BoardID: boardID})
	return nil
}

func (service *Service) CreateField(ctx context.Context, access AccessContext, input CreateFieldInput) (Field, error) {
	if !access.Has(PermBoardsManage) {
		return Field{}, ErrForbidden
	}
	input.BoardID = strings.TrimSpace(input.BoardID)
	input.Key = normalizeFieldKey(input.Key)
	input.Label = strings.TrimSpace(input.Label)
	input.Type = strings.TrimSpace(input.Type)
	if input.BoardID == "" || input.Key == "" || input.Label == "" || input.Type == "" {
		return Field{}, ErrValidation
	}
	if input.Config == nil {
		input.Config = map[string]any{}
	}

	field, err := service.repository.CreateField(ctx, access.AccountID, input)
	if err != nil {
		return Field{}, err
	}
	service.audit(ctx, access, "field.created", "field", field.ID, nil, field)
	service.publisher.PublishBoardEvent(ctx, BoardEvent{Type: "field.created", AccountID: access.AccountID, BoardID: field.BoardID})
	return field, nil
}

// ListTasksResult devolve a pagina de tasks junto com o cursor para a proxima pagina. NextCursor
// vazio sinaliza fim da paginacao.
type ListTasksResult struct {
	Tasks      []TaskDTO
	NextCursor string
}

func (service *Service) ListTasks(ctx context.Context, access AccessContext, input ListTasksInput) (ListTasksResult, error) {
	if !access.Has(PermTasksView) {
		return ListTasksResult{}, ErrForbidden
	}
	input.BoardID = strings.TrimSpace(input.BoardID)
	if input.BoardID == "" {
		return ListTasksResult{}, ErrValidation
	}
	if input.Limit <= 0 || input.Limit > 200 {
		input.Limit = 50
	}

	tasks, nextCursor, err := service.repository.ListTasks(ctx, access, input)
	if err != nil {
		return ListTasksResult{}, err
	}
	dtos := make([]TaskDTO, 0, len(tasks))
	for _, task := range tasks {
		dtos = append(dtos, service.BuildTaskDTO(task, access.Perspective))
	}
	return ListTasksResult{Tasks: dtos, NextCursor: nextCursor}, nil
}

func (service *Service) GetTask(ctx context.Context, access AccessContext, taskID string) (TaskDTO, error) {
	if !access.Has(PermTasksView) {
		return TaskDTO{}, ErrForbidden
	}
	task, err := service.repository.GetTask(ctx, access, strings.TrimSpace(taskID))
	if err != nil {
		return TaskDTO{}, err
	}
	return service.BuildTaskDTO(task, access.Perspective), nil
}

func (service *Service) CreateTask(ctx context.Context, access AccessContext, input CreateTaskInput) (TaskDTO, error) {
	if !access.Has(PermTasksCreate) {
		return TaskDTO{}, ErrForbidden
	}
	input.BoardID = strings.TrimSpace(input.BoardID)
	input.Title = strings.TrimSpace(input.Title)
	input.ContentHTML = strings.TrimSpace(input.ContentHTML)
	input.Priority = defaultString(strings.TrimSpace(input.Priority), "media")
	input.UIMetadata = normalizeTaskUIMetadata(input.UIMetadata)
	if input.BoardID == "" || input.Title == "" {
		return TaskDTO{}, ErrValidation
	}

	task, err := service.repository.CreateTask(ctx, access.AccountID, input, access.UserID)
	if err != nil {
		return TaskDTO{}, err
	}
	service.ensureTaskSubscribers(ctx, access.AccountID, task.ID, access.UserID, optionalStringValue(task.ResponsibleUserID))
	service.audit(ctx, access, "task.created", "task", task.ID, nil, task)
	service.publisher.PublishTaskEvent(ctx, TaskEvent{Type: "task.created", AccountID: access.AccountID, BoardID: task.BoardID, TaskID: task.ID, Version: task.Version})
	service.notifyTaskAssigned(ctx, access, task, optionalStringValue(task.ResponsibleUserID))
	return service.BuildTaskDTO(task, access.Perspective), nil
}

func (service *Service) UpdateTask(ctx context.Context, access AccessContext, input UpdateTaskInput) (TaskDTO, error) {
	if !access.Has(PermTasksEdit) {
		return TaskDTO{}, ErrForbidden
	}
	input.ID = strings.TrimSpace(input.ID)
	if input.ID == "" {
		return TaskDTO{}, ErrValidation
	}
	if input.Title != nil {
		*input.Title = strings.TrimSpace(*input.Title)
		if *input.Title == "" {
			return TaskDTO{}, ErrValidation
		}
	}
	if input.Priority != nil {
		*input.Priority = defaultString(strings.TrimSpace(*input.Priority), "media")
	}
	if input.UIMetadata != nil {
		normalized := normalizeTaskUIMetadata(*input.UIMetadata)
		input.UIMetadata = &normalized
	}

	before, err := service.repository.GetTask(ctx, access, input.ID)
	if err != nil {
		return TaskDTO{}, err
	}
	after, err := service.repository.UpdateTask(ctx, access.AccountID, input)
	if err != nil {
		return TaskDTO{}, err
	}
	service.ensureTaskSubscribers(ctx, access.AccountID, after.ID, access.UserID, optionalStringValue(after.ResponsibleUserID))
	service.audit(ctx, access, "task.updated", "task", after.ID, before, after)
	service.publisher.PublishTaskEvent(ctx, TaskEvent{Type: "task.updated", AccountID: access.AccountID, BoardID: after.BoardID, TaskID: after.ID, Version: after.Version})
	if optionalStringValue(before.ResponsibleUserID) != optionalStringValue(after.ResponsibleUserID) {
		service.notifyTaskAssigned(ctx, access, after, optionalStringValue(after.ResponsibleUserID))
	}
	if optionalStringValue(before.Status) != optionalStringValue(after.Status) {
		service.notifyTaskSubscribers(ctx, access, after, "task.status_changed", "Task atualizada", taskStatusChangedBody(after), access.UserID)
	}
	return service.BuildTaskDTO(after, access.Perspective), nil
}

func (service *Service) MoveTask(ctx context.Context, access AccessContext, input MoveTaskInput) (TaskDTO, error) {
	if !access.Has(PermTasksEdit) {
		return TaskDTO{}, ErrForbidden
	}
	input.ID = strings.TrimSpace(input.ID)
	if input.ID == "" {
		return TaskDTO{}, ErrValidation
	}
	before, err := service.repository.GetTask(ctx, access, input.ID)
	if err != nil {
		return TaskDTO{}, err
	}
	after, err := service.repository.MoveTask(ctx, access.AccountID, input)
	if err != nil {
		return TaskDTO{}, err
	}
	service.ensureTaskSubscribers(ctx, access.AccountID, after.ID, access.UserID, optionalStringValue(after.ResponsibleUserID))
	service.audit(ctx, access, "task.moved", "task", after.ID, before, after)
	service.publisher.PublishTaskEvent(ctx, TaskEvent{Type: "task.moved", AccountID: access.AccountID, BoardID: after.BoardID, TaskID: after.ID, Version: after.Version})
	if optionalStringValue(before.ColumnID) != optionalStringValue(after.ColumnID) {
		service.notifyTaskSubscribers(ctx, access, after, "task.moved", "Task movida", taskMovedBody(after), access.UserID)
	}
	return service.BuildTaskDTO(after, access.Perspective), nil
}

func (service *Service) ArchiveTask(ctx context.Context, access AccessContext, taskID string) error {
	if !access.Has(PermTasksDelete) {
		return ErrForbidden
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return ErrValidation
	}
	before, err := service.repository.GetTask(ctx, access, taskID)
	if err != nil {
		return err
	}
	after, err := service.repository.ArchiveTask(ctx, access.AccountID, taskID)
	if err != nil {
		return err
	}
	service.audit(ctx, access, "task.deleted", "task", after.ID, before, after)
	service.publisher.PublishTaskEvent(ctx, TaskEvent{Type: "task.deleted", AccountID: access.AccountID, BoardID: after.BoardID, TaskID: after.ID, Version: after.Version})
	return nil
}

func (service *Service) AddComment(ctx context.Context, access AccessContext, input AddCommentInput) (Comment, error) {
	if !access.Has(PermTasksComment) {
		return Comment{}, ErrForbidden
	}
	input.TaskID = strings.TrimSpace(input.TaskID)
	input.BodyHTML = strings.TrimSpace(input.BodyHTML)
	if input.TaskID == "" || input.BodyHTML == "" {
		return Comment{}, ErrValidation
	}
	task, err := service.repository.GetTask(ctx, access, input.TaskID)
	if err != nil {
		return Comment{}, err
	}
	comment, err := service.repository.AddComment(ctx, access.AccountID, input, access.UserID)
	if err != nil {
		return Comment{}, err
	}
	mentionedUserIDs := service.persistCommentMentions(ctx, access.AccountID, task.ID, comment.ID, input.MentionedUserIDs)
	service.ensureTaskSubscribers(ctx, access.AccountID, task.ID, access.UserID)
	if len(mentionedUserIDs) > 0 {
		service.ensureTaskSubscribers(ctx, access.AccountID, task.ID, mentionedUserIDs...)
		service.notifyTaskMentions(ctx, access, task, comment, mentionedUserIDs)
	} else {
		service.notifyTaskSubscribers(ctx, access, task, "task.comment_added", "Novo comentario em task", taskCommentBody(task), access.UserID)
	}
	service.audit(ctx, access, "task.comment_added", "task", input.TaskID, nil, comment)
	service.publisher.PublishTaskEvent(ctx, TaskEvent{Type: "task.comment_added", AccountID: access.AccountID, BoardID: task.BoardID, TaskID: task.ID, Version: task.Version})
	return comment, nil
}

func (service *Service) ListComments(ctx context.Context, access AccessContext, taskID string) ([]Comment, error) {
	if !access.Has(PermTasksView) {
		return nil, ErrForbidden
	}
	return service.repository.ListComments(ctx, access, strings.TrimSpace(taskID))
}

func (service *Service) AddShare(ctx context.Context, access AccessContext, input AddShareInput) (Share, error) {
	if !access.Has(PermSharesManage) {
		return Share{}, ErrForbidden
	}
	input.TaskID = strings.TrimSpace(input.TaskID)
	input.ClientAccountID = strings.TrimSpace(input.ClientAccountID)
	input.Permission = strings.TrimSpace(input.Permission)
	if input.TaskID == "" || input.ClientAccountID == "" {
		return Share{}, ErrValidation
	}
	if input.Permission == "" {
		input.Permission = "view"
	}
	if input.Permission != "view" && input.Permission != "comment" && input.Permission != "edit" {
		return Share{}, ErrValidation
	}
	task, err := service.repository.GetTask(ctx, access, input.TaskID)
	if err != nil {
		return Share{}, err
	}
	share, err := service.repository.AddShare(ctx, access.AccountID, input, access.UserID)
	if err != nil {
		return Share{}, err
	}
	service.audit(ctx, access, "task.share_added", "task", input.TaskID, nil, share)
	service.publisher.PublishTaskEvent(ctx, TaskEvent{Type: "task.share_added", AccountID: access.AccountID, BoardID: task.BoardID, TaskID: task.ID, Version: task.Version})
	return share, nil
}

func (service *Service) ListRelations(ctx context.Context, access AccessContext, taskID string) ([]Relation, error) {
	if !access.Has(PermTasksView) {
		return nil, ErrForbidden
	}
	return service.repository.ListRelations(ctx, access, strings.TrimSpace(taskID))
}

func (service *Service) AddRelation(ctx context.Context, access AccessContext, input AddRelationInput) (Relation, error) {
	if !access.Has(PermRelationsManage) {
		return Relation{}, ErrForbidden
	}
	input.TaskID = strings.TrimSpace(input.TaskID)
	input.Module = strings.TrimSpace(input.Module)
	input.ResourceType = strings.TrimSpace(input.ResourceType)
	input.ResourceID = strings.TrimSpace(input.ResourceID)
	input.LabelCache = strings.TrimSpace(input.LabelCache)
	if input.MetadataCache == nil {
		input.MetadataCache = map[string]any{}
	}
	if input.TaskID == "" || input.Module == "" || input.ResourceType == "" || input.ResourceID == "" {
		return Relation{}, ErrValidation
	}
	task, err := service.repository.GetTask(ctx, access, input.TaskID)
	if err != nil {
		return Relation{}, err
	}
	relation, err := service.repository.AddRelation(ctx, access.AccountID, input)
	if err != nil {
		return Relation{}, err
	}
	service.audit(ctx, access, "task.relation_added", "task", input.TaskID, nil, relation)
	service.publisher.PublishTaskEvent(ctx, TaskEvent{Type: "task.relation_added", AccountID: access.AccountID, BoardID: task.BoardID, TaskID: task.ID, Version: task.Version})
	return relation, nil
}

func (service *Service) ListAudit(ctx context.Context, access AccessContext, taskID string) ([]AuditEntry, error) {
	if !access.Has(PermBoardsManage) {
		return nil, ErrForbidden
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, ErrValidation
	}
	if _, err := service.repository.GetTask(ctx, access, taskID); err != nil {
		return nil, err
	}
	return service.repository.ListAudit(ctx, access.AccountID, taskID)
}

func (service *Service) audit(ctx context.Context, access AccessContext, action, resourceType, resourceID string, before, after any) {
	_ = service.repository.InsertAuditEntry(ctx, AuditEntry{
		AccountID:    access.AccountID,
		UserID:       &access.UserID,
		Action:       strings.TrimSpace(action),
		ResourceType: strings.TrimSpace(resourceType),
		ResourceID:   strings.TrimSpace(resourceID),
		Before:       snapshotMap(before),
		After:        snapshotMap(after),
	})
	// T8: alem do registro persistente em tasks.tasks_audit, emite slog estruturado em todas as
	// mutations para observabilidade externa (ELK/Loki/etc). audit() ja e' o ponto unico onde
	// passa toda mutation; centralizar aqui evita ter que tocar 13+ call sites.
	service.logMutation(ctx, access, action, resourceType, resourceID)
}

func snapshotMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil
	}
	return decoded
}

func normalizeSlug(slug, fallback string) string {
	value := strings.ToLower(strings.TrimSpace(slug))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(fallback))
	}
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return strings.Trim(value, "-")
}

func normalizeFieldKey(key string) string {
	value := strings.ToLower(strings.TrimSpace(key))
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "-", "_")
	return strings.Trim(value, "_")
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
