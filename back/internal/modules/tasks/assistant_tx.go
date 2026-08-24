package tasks

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrAssistantSnapshotMissing = errors.New("tasks: assistant snapshot missing")
	ErrAssistantSnapshotStale   = errors.New("tasks: assistant snapshot stale")
)

type AssistantTaskItemInput struct {
	ID            string
	Title         string
	Status        string
	StatusDate    string
	Completed     *bool
	CompletedDate string
}

type AssistantTaskMutationInput struct {
	AccountID, ActorUserID, Action, Kind, TargetID, BoardID string
	Title, ContentHTML, Status, Priority                    string
	DueDate, StartDate, DueEndDate, Time, ColumnID          string
	ResponsibleUserID, ClientAccountID, ClientName, Type    string
	InvolvedIDs                                             []string
	Archived                                                *bool
	Item                                                    *AssistantTaskItemInput
	ExpectedVersion                                         *int
	BeforeSnapshot                                          json.RawMessage
	BeforeHash                                              []byte
	DeterministicItemID                                     string
}

type AssistantTaskMutationResult struct {
	Before  *Task
	After   Task
	Deleted bool
}

func LoadAssistantTaskSnapshotTx(ctx context.Context, tx pgx.Tx, accountID, taskID string) (Task, error) {
	return scanTask(tx.QueryRow(ctx, assistantTaskSelect+` where t.account_id=$1::uuid and t.id=$2::uuid`, accountID, taskID).Scan)
}

const assistantTaskSelect = `select t.id::text, t.account_id::text, t.board_id::text, t.column_id::text,
	t.title, t.content_html, t.status, t.priority, t.due_date, t.start_date,
	t.archived, t.sort_order::float8, t.created_by_user_id::text,
	t.responsible_user_id::text, t.client_account_id::text, t.ui_metadata,
	t.roadmap_module_id::text, t.pinned_to_roadmap, t.version, t.created_at, t.updated_at
	from tasks.tasks t`

func ExecuteAssistantTaskMutationTx(ctx context.Context, tx pgx.Tx, in AssistantTaskMutationInput) (AssistantTaskMutationResult, error) {
	if strings.TrimSpace(in.AccountID) == "" || strings.TrimSpace(in.ActorUserID) == "" {
		return AssistantTaskMutationResult{}, ErrValidation
	}
	if in.Kind == "task" && in.Action == "create" {
		return createAssistantTaskTx(ctx, tx, in)
	}
	current, err := scanTask(tx.QueryRow(ctx, assistantTaskSelect+` where t.account_id=$1::uuid and t.id=$2::uuid for update of t`, in.AccountID, in.TargetID).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return AssistantTaskMutationResult{}, ErrTaskNotFound
	}
	if err != nil {
		return AssistantTaskMutationResult{}, err
	}
	if err := validateAssistantTaskSnapshot(current, in); err != nil {
		return AssistantTaskMutationResult{}, err
	}
	before := current
	if in.Kind == "taskItem" {
		if err := applyAssistantTaskItem(&current, in); err != nil {
			return AssistantTaskMutationResult{}, err
		}
	} else if in.Action == "delete" {
		current.Archived = true
	} else if err := applyAssistantTaskFields(&current, in); err != nil {
		return AssistantTaskMutationResult{}, err
	}
	after, err := updateAssistantTaskTx(ctx, tx, current)
	if err != nil {
		return AssistantTaskMutationResult{}, err
	}
	return AssistantTaskMutationResult{Before: &before, After: after, Deleted: in.Action == "delete"}, nil
}

func validateAssistantTaskSnapshot(current Task, in AssistantTaskMutationInput) error {
	// tasks.tasks nasce em version=0; nil (nao zero) representa ausencia de snapshot.
	if in.ExpectedVersion == nil || *in.ExpectedVersion < 0 || current.Version != *in.ExpectedVersion || len(in.BeforeHash) != sha256.Size {
		return ErrAssistantSnapshotMissing
	}
	raw, err := json.Marshal(current)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	if subtle.ConstantTimeCompare(sum[:], in.BeforeHash) != 1 {
		return ErrAssistantSnapshotStale
	}
	return nil
}

func createAssistantTaskTx(ctx context.Context, tx pgx.Tx, in AssistantTaskMutationInput) (AssistantTaskMutationResult, error) {
	title := strings.TrimSpace(in.Title)
	boardID := strings.TrimSpace(in.BoardID)
	if title == "" || boardID == "" {
		return AssistantTaskMutationResult{}, ErrValidation
	}
	var boardExists bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from tasks.boards where account_id=$1::uuid and id=$2::uuid and archived=false)`, in.AccountID, boardID).Scan(&boardExists); err != nil || !boardExists {
		if err != nil {
			return AssistantTaskMutationResult{}, err
		}
		return AssistantTaskMutationResult{}, ErrBoardNotFound
	}
	dueDate, err := assistantTaskDate(in.DueDate, in.Time)
	if err != nil {
		return AssistantTaskMutationResult{}, err
	}
	startDate, err := assistantTaskDate(in.StartDate, "")
	if err != nil {
		return AssistantTaskMutationResult{}, err
	}
	metadata := assistantTaskMetadata(in, nil)
	priority := strings.TrimSpace(in.Priority)
	if priority == "" {
		priority = "media"
	}
	status := nullableText(in.Status)
	columnID := nullableText(in.ColumnID)
	responsibleID := nullableText(in.ResponsibleUserID)
	clientID := nullableText(in.ClientAccountID)
	const q = `insert into tasks.tasks (account_id,board_id,column_id,title,content_html,status,priority,
		due_date,start_date,sort_order,created_by_user_id,responsible_user_id,client_account_id,ui_metadata)
		values ($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7,$8,$9,0,$10::uuid,$11::uuid,$12::uuid,$13::jsonb)
		returning id::text,account_id::text,board_id::text,column_id::text,title,content_html,status,priority,
		due_date,start_date,archived,sort_order::float8,created_by_user_id::text,responsible_user_id::text,
		client_account_id::text,ui_metadata,roadmap_module_id::text,pinned_to_roadmap,version,created_at,updated_at`
	task, err := scanTask(tx.QueryRow(ctx, q, in.AccountID, boardID, columnID, title,
		strings.TrimSpace(in.ContentHTML), status, priority, dueDate, startDate, in.ActorUserID,
		responsibleID, clientID, mustJSON(normalizeTaskUIMetadata(metadata))).Scan)
	return AssistantTaskMutationResult{After: task}, err
}

func applyAssistantTaskFields(task *Task, in AssistantTaskMutationInput) error {
	if value := strings.TrimSpace(in.Title); value != "" {
		task.Title = value
	}
	if value := strings.TrimSpace(in.ContentHTML); value != "" {
		task.ContentHTML = value
	}
	if value := strings.TrimSpace(in.Status); value != "" {
		task.Status = &value
	}
	if value := strings.TrimSpace(in.Priority); value != "" {
		task.Priority = value
	}
	if in.DueDate != "" {
		value, err := assistantTaskDate(in.DueDate, in.Time)
		if err != nil {
			return err
		}
		task.DueDate = value
	}
	if in.StartDate != "" {
		value, err := assistantTaskDate(in.StartDate, "")
		if err != nil {
			return err
		}
		task.StartDate = value
	}
	if value := strings.TrimSpace(in.ColumnID); value != "" {
		task.ColumnID = &value
	}
	if value := strings.TrimSpace(in.ResponsibleUserID); value != "" {
		task.ResponsibleUserID = &value
	}
	if value := strings.TrimSpace(in.ClientAccountID); value != "" {
		task.ClientAccountID = &value
	}
	if in.Archived != nil {
		task.Archived = *in.Archived
	}
	task.UIMetadata = mergeMetadata(task.UIMetadata, assistantTaskMetadata(in, task.UIMetadata))
	return nil
}

func applyAssistantTaskItem(task *Task, in AssistantTaskMutationInput) error {
	if in.Item == nil {
		return ErrValidation
	}
	items := normalizeTaskChecklistMetadata(task.UIMetadata["checklist"])
	index := -1
	for i := range items {
		if strings.TrimSpace(stringValue(items[i]["id"])) == strings.TrimSpace(in.Item.ID) {
			index = i
			break
		}
	}
	switch in.Action {
	case "create":
		if strings.TrimSpace(in.Item.Title) == "" || strings.TrimSpace(in.DeterministicItemID) == "" {
			return ErrValidation
		}
		item := map[string]any{"id": in.DeterministicItemID, "title": strings.TrimSpace(in.Item.Title), "completed": false}
		applyAssistantItemFields(item, *in.Item)
		items = append(items, item)
	case "update":
		if index < 0 {
			return ErrValidation
		}
		applyAssistantItemFields(items[index], *in.Item)
	case "delete":
		if index < 0 {
			return ErrValidation
		}
		items = append(items[:index], items[index+1:]...)
	default:
		return ErrValidation
	}
	if task.UIMetadata == nil {
		task.UIMetadata = map[string]any{}
	}
	task.UIMetadata["checklist"] = items
	return nil
}

func applyAssistantItemFields(item map[string]any, in AssistantTaskItemInput) {
	if value := strings.TrimSpace(in.Title); value != "" {
		item["title"] = value
	}
	if value := normalizeTaskChecklistStatus(in.Status); value != "" {
		item["status"] = value
		if date := normalizeTaskChecklistDate(in.StatusDate); date != "" {
			item["statusDate"] = date
		}
	}
	if in.Completed != nil {
		item["completed"] = *in.Completed
		if !*in.Completed {
			delete(item, "completedDate")
		} else if date := normalizeTaskChecklistDate(in.CompletedDate); date != "" {
			item["completedDate"] = date
		}
	}
}

func assistantTaskMetadata(in AssistantTaskMutationInput, current map[string]any) map[string]any {
	metadata := map[string]any{}
	if current == nil {
		metadata["source"] = "assistant"
	}
	if value := strings.TrimSpace(in.Type); value != "" {
		metadata["type"] = value
	}
	if value := strings.TrimSpace(in.DueEndDate); value != "" {
		metadata["dueEndDate"] = value
	}
	if value := strings.TrimSpace(in.ClientName); value != "" {
		metadata["clientName"] = value
	}
	if len(in.InvolvedIDs) > 0 {
		metadata["involved"] = append([]string(nil), in.InvolvedIDs...)
	}
	return metadata
}

func updateAssistantTaskTx(ctx context.Context, tx pgx.Tx, task Task) (Task, error) {
	const q = `update tasks.tasks set column_id=$3::uuid,title=$4,content_html=$5,status=$6,priority=$7,
		due_date=$8,start_date=$9,archived=$10,sort_order=$11,responsible_user_id=$12::uuid,
		client_account_id=$13::uuid,ui_metadata=$14::jsonb,roadmap_module_id=$15::uuid,
		pinned_to_roadmap=$16,version=version+1,updated_at=now()
		where account_id=$1::uuid and id=$2::uuid and version=$17
		returning id::text,account_id::text,board_id::text,column_id::text,title,content_html,status,priority,
		due_date,start_date,archived,sort_order::float8,created_by_user_id::text,responsible_user_id::text,
		client_account_id::text,ui_metadata,roadmap_module_id::text,pinned_to_roadmap,version,created_at,updated_at`
	updated, err := scanTask(tx.QueryRow(ctx, q, task.AccountID, task.ID, task.ColumnID, task.Title,
		task.ContentHTML, task.Status, task.Priority, task.DueDate, task.StartDate, task.Archived,
		task.SortOrder, task.ResponsibleUserID, task.ClientAccountID, mustJSON(normalizeTaskUIMetadata(task.UIMetadata)),
		task.RoadmapModuleID, task.PinnedToRoadmap, task.Version).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrVersionConflict
	}
	return updated, err
}

func assistantTaskDate(value, clock string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if clock = strings.TrimSpace(clock); clock != "" {
		location, locationErr := time.LoadLocation("America/Sao_Paulo")
		if locationErr != nil {
			return nil, locationErr
		}
		parsed, err := time.ParseInLocation("2006-01-02 15:04", value+" "+clock, location)
		if err != nil {
			return nil, ErrValidation
		}
		return &parsed, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, ErrValidation
	}
	return &parsed, nil
}

func nullableText(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func (service *Service) FinalizeAssistantTaskMutation(ctx context.Context, access AccessContext, outcome AssistantTaskMutationResult) {
	after := outcome.After
	if outcome.Before == nil {
		service.ensureTaskSubscribers(ctx, access.AccountID, after.ID, access.UserID, optionalStringValue(after.ResponsibleUserID))
		service.audit(ctx, access, "task.created", "task", after.ID, nil, after)
		service.publisher.PublishTaskEvent(ctx, TaskEvent{Type: "task.created", AccountID: access.AccountID, BoardID: after.BoardID, TaskID: after.ID, Version: after.Version})
		service.notifyTaskAssigned(ctx, access, after, optionalStringValue(after.ResponsibleUserID))
		service.dispatchTaskSync(ctx, access, after, false)
		return
	}
	before := *outcome.Before
	eventType, auditAction := "task.updated", "task.updated"
	if outcome.Deleted {
		eventType, auditAction = "task.deleted", "task.deleted"
	}
	service.audit(ctx, access, auditAction, "task", after.ID, before, after)
	service.publisher.PublishTaskEvent(ctx, TaskEvent{Type: eventType, AccountID: access.AccountID, BoardID: after.BoardID, TaskID: after.ID, Version: after.Version})
	if !outcome.Deleted {
		service.ensureTaskSubscribers(ctx, access.AccountID, after.ID, access.UserID, optionalStringValue(after.ResponsibleUserID))
		if optionalStringValue(before.ResponsibleUserID) != optionalStringValue(after.ResponsibleUserID) {
			service.notifyTaskAssigned(ctx, access, after, optionalStringValue(after.ResponsibleUserID))
		}
		if optionalStringValue(before.Status) != optionalStringValue(after.Status) {
			service.notifyTaskSubscribers(ctx, access, after, "task.status_changed", "Task atualizada", taskStatusChangedBody(after), access.UserID)
		}
	}
	syncTask := after
	if outcome.Deleted {
		syncTask = before
	}
	service.dispatchTaskSync(ctx, access, syncTask, outcome.Deleted)
}
