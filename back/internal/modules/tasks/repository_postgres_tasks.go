package tasks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// listTasksCursor representa a posicao paginada de ListTasks. Encoding como base64url(JSON) e'
// opaco para o cliente — podemos mudar a estrategia (ex: adicionar um filtro) sem quebrar URLs
// salvas. Tuple (sort_order, created_at, id) e' estavel e total no SQL.
type listTasksCursor struct {
	SortOrder float64   `json:"s"`
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

func encodeListTasksCursor(cursor listTasksCursor) string {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeListTasksCursor(raw string) (listTasksCursor, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return listTasksCursor{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return listTasksCursor{}, false
	}
	var cursor listTasksCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return listTasksCursor{}, false
	}
	if strings.TrimSpace(cursor.ID) == "" {
		return listTasksCursor{}, false
	}
	return cursor, true
}

func listTasksCursorQueryArgs(cursor listTasksCursor, hasCursor bool) (any, any, any) {
	if !hasCursor {
		return nil, nil, nil
	}
	return cursor.SortOrder, cursor.CreatedAt, cursor.ID
}

func (repository *PostgresRepository) ListTasks(ctx context.Context, access AccessContext, input ListTasksInput) ([]Task, string, error) {
	cursor, hasCursor := decodeListTasksCursor(input.Cursor)
	cursorSortOrder, cursorCreatedAt, cursorID := listTasksCursorQueryArgs(cursor, hasCursor)

	fetchLimit := input.Limit + 1

	sql, args := repository.scopedQuery(access.AccountID, `
		select t.id::text, t.account_id::text, t.board_id::text, t.column_id::text,
		       t.title, t.content_html, t.status, t.priority, t.due_date, t.start_date,
		       t.archived, t.sort_order::float8, t.created_by_user_id::text,
		       t.responsible_user_id::text, t.client_account_id::text, t.ui_metadata,
		       t.roadmap_module_id::text, t.pinned_to_roadmap, t.version,
		       t.created_at, t.updated_at
		from tasks.tasks t
		where t.account_id = $1::uuid and t.board_id = $2::uuid
		  and ($3::boolean = true or t.archived = false)
		  and ($4::boolean = false or (t.sort_order, t.created_at, t.id) > ($5::float8, $6::timestamptz, $7::uuid))
		order by t.sort_order asc, t.created_at asc, t.id asc
		limit $8
	`, input.BoardID, input.IncludeArchived, hasCursor, cursorSortOrder, cursorCreatedAt, cursorID, fetchLimit)

	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	tasks := make([]Task, 0, input.Limit)
	for rows.Next() {
		task, err := scanTask(rows.Scan)
		if err != nil {
			return nil, "", err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(tasks) > input.Limit {
		last := tasks[input.Limit-1]
		tasks = tasks[:input.Limit]
		nextCursor = encodeListTasksCursor(listTasksCursor{
			SortOrder: last.SortOrder,
			CreatedAt: last.CreatedAt,
			ID:        last.ID,
		})
	}
	return tasks, nextCursor, nil
}

func (repository *PostgresRepository) GetTask(ctx context.Context, access AccessContext, taskID string) (Task, error) {
	sql, args := repository.scopedQuery(access.AccountID, `
		select t.id::text, t.account_id::text, t.board_id::text, t.column_id::text,
		       t.title, t.content_html, t.status, t.priority, t.due_date, t.start_date,
		       t.archived, t.sort_order::float8, t.created_by_user_id::text,
		       t.responsible_user_id::text, t.client_account_id::text, t.ui_metadata,
		       t.roadmap_module_id::text, t.pinned_to_roadmap, t.version,
		       t.created_at, t.updated_at
		from tasks.tasks t
		where t.account_id = $1::uuid and t.id = $2::uuid
	`, taskID)

	task, err := scanTask(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	return task, err
}

func (repository *PostgresRepository) CreateTask(ctx context.Context, accountID string, input CreateTaskInput, createdByUserID string) (Task, error) {
	sql, args := repository.scopedQuery(accountID, `
		insert into tasks.tasks (
			account_id, board_id, column_id, title, content_html, status, priority,
			due_date, start_date, sort_order, created_by_user_id, responsible_user_id,
			client_account_id, ui_metadata, roadmap_module_id, pinned_to_roadmap
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10,
			$11::uuid, $12::uuid, $13::uuid, $14::jsonb, $15::uuid, $16
		)
		returning id::text, account_id::text, board_id::text, column_id::text,
		          title, content_html, status, priority, due_date, start_date,
		          archived, sort_order::float8, created_by_user_id::text,
		          responsible_user_id::text, client_account_id::text, ui_metadata,
		          roadmap_module_id::text, pinned_to_roadmap, version,
		          created_at, updated_at
	`, input.BoardID, input.ColumnID, input.Title, input.ContentHTML, input.Status, input.Priority,
		input.DueDate, input.StartDate, input.SortOrder, createdByUserID, input.ResponsibleUserID, input.ClientAccountID,
		mustJSON(normalizeTaskUIMetadata(input.UIMetadata)), input.RoadmapModuleID, input.PinnedToRoadmap != nil && *input.PinnedToRoadmap)

	task, err := scanTask(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrBoardNotFound
	}
	return task, err
}

func (repository *PostgresRepository) UpdateTask(ctx context.Context, accountID string, input UpdateTaskInput) (Task, error) {
	access := AccessContext{AccountID: accountID, IsPlatformAdmin: true, Perspective: PerspectiveAgency}
	task, err := repository.GetTask(ctx, access, input.ID)
	if err != nil {
		return Task{}, err
	}
	if input.ExpectedVersion != nil && task.Version != *input.ExpectedVersion {
		return Task{}, ErrVersionConflict
	}

	if input.ColumnID != nil {
		task.ColumnID = *input.ColumnID
	}
	if input.Title != nil {
		task.Title = *input.Title
	}
	if input.ContentHTML != nil {
		task.ContentHTML = *input.ContentHTML
	}
	if input.Status != nil {
		task.Status = *input.Status
	}
	if input.Priority != nil {
		task.Priority = *input.Priority
	}
	if input.DueDate != nil {
		task.DueDate = *input.DueDate
	}
	if input.StartDate != nil {
		task.StartDate = *input.StartDate
	}
	if input.Archived != nil {
		task.Archived = *input.Archived
	}
	if input.SortOrder != nil {
		task.SortOrder = *input.SortOrder
	}
	if input.ResponsibleUserID != nil {
		task.ResponsibleUserID = *input.ResponsibleUserID
	}
	if input.ClientAccountID != nil {
		task.ClientAccountID = *input.ClientAccountID
	}
	if input.UIMetadata != nil {
		task.UIMetadata = mergeMetadata(task.UIMetadata, *input.UIMetadata)
	}
	if input.RoadmapModuleID != nil {
		task.RoadmapModuleID = *input.RoadmapModuleID
	}
	if input.PinnedToRoadmap != nil {
		task.PinnedToRoadmap = *input.PinnedToRoadmap
	}

	return repository.updateTaskRow(ctx, accountID, task)
}

func (repository *PostgresRepository) MoveTask(ctx context.Context, accountID string, input MoveTaskInput) (Task, error) {
	access := AccessContext{AccountID: accountID, IsPlatformAdmin: true, Perspective: PerspectiveAgency}
	task, err := repository.GetTask(ctx, access, input.ID)
	if err != nil {
		return Task{}, err
	}
	if input.ExpectedVersion != nil && task.Version != *input.ExpectedVersion {
		return Task{}, ErrVersionConflict
	}
	task.ColumnID = input.ColumnID
	if input.SortOrder != nil {
		task.SortOrder = *input.SortOrder
	}
	return repository.updateTaskRow(ctx, accountID, task)
}

func (repository *PostgresRepository) ArchiveTask(ctx context.Context, accountID, taskID string) (Task, error) {
	sql, args := repository.scopedQuery(accountID, `
		update tasks.tasks
		   set archived = true, version = version + 1, updated_at = now()
		 where account_id = $1::uuid and id = $2::uuid and archived = false
		returning id::text, account_id::text, board_id::text, column_id::text,
		          title, content_html, status, priority, due_date, start_date,
		          archived, sort_order::float8, created_by_user_id::text,
		          responsible_user_id::text, client_account_id::text, ui_metadata,
		          roadmap_module_id::text, pinned_to_roadmap, version,
		          created_at, updated_at
	`, taskID)
	task, err := scanTask(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	return task, err
}

func (repository *PostgresRepository) updateTaskRow(ctx context.Context, accountID string, task Task) (Task, error) {
	sql, args := repository.scopedQuery(accountID, `
		update tasks.tasks
		   set column_id = $3::uuid,
		       title = $4,
		       content_html = $5,
		       status = $6,
		       priority = $7,
		       due_date = $8,
		       start_date = $9,
		       archived = $10,
		       sort_order = $11,
		       responsible_user_id = $12::uuid,
		       client_account_id = $13::uuid,
		       ui_metadata = $14::jsonb,
		       roadmap_module_id = $15::uuid,
		       pinned_to_roadmap = $16,
		       version = version + 1,
		       updated_at = now()
		 where account_id = $1::uuid and id = $2::uuid
		returning id::text, account_id::text, board_id::text, column_id::text,
		          title, content_html, status, priority, due_date, start_date,
		          archived, sort_order::float8, created_by_user_id::text,
		          responsible_user_id::text, client_account_id::text, ui_metadata,
		          roadmap_module_id::text, pinned_to_roadmap, version,
		          created_at, updated_at
	`, task.ID, task.ColumnID, task.Title, task.ContentHTML, task.Status, task.Priority,
		task.DueDate, task.StartDate, task.Archived, task.SortOrder, task.ResponsibleUserID, task.ClientAccountID, mustJSON(normalizeMap(task.UIMetadata)),
		task.RoadmapModuleID, task.PinnedToRoadmap)
	updated, err := scanTask(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	return updated, err
}

func scanTask(scan func(...any) error) (Task, error) {
	var task Task
	var uiMetadataRaw []byte
	err := scan(
		&task.ID,
		&task.AccountID,
		&task.BoardID,
		&task.ColumnID,
		&task.Title,
		&task.ContentHTML,
		&task.Status,
		&task.Priority,
		&task.DueDate,
		&task.StartDate,
		&task.Archived,
		&task.SortOrder,
		&task.CreatedByUserID,
		&task.ResponsibleUserID,
		&task.ClientAccountID,
		&uiMetadataRaw,
		&task.RoadmapModuleID,
		&task.PinnedToRoadmap,
		&task.Version,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	task.UIMetadata = decodeMap(uiMetadataRaw)
	return task, err
}
