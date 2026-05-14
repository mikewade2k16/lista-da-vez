package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) scopedQuery(accountID string, baseSQL string, args ...any) (string, []any) {
	if strings.TrimSpace(accountID) == "" {
		panic("tasks: scopedQuery called without accountID")
	}
	return baseSQL, append([]any{accountID}, args...)
}

func (repository *PostgresRepository) AccountExists(ctx context.Context, accountID string) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists (
			select 1 from core.accounts where id = $1::uuid and is_active = true
		)
	`, accountID).Scan(&exists)
	return exists, err
}

func (repository *PostgresRepository) IsAccountMember(ctx context.Context, accountID, userID string) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists (
			select 1
			from core.account_users
			where account_id = $1::uuid and user_id = $2::uuid and is_active = true
		)
	`, accountID, userID).Scan(&exists)
	return exists, err
}

func (repository *PostgresRepository) ListPermissionsForUser(ctx context.Context, accountID, userID string) ([]string, error) {
	rows, err := repository.pool.Query(ctx, `
		select rp.permission_key
		from core.user_role_assignments ura
		join core.role_permissions rp on rp.role_id = ura.role_id
		join core.permissions p on p.key = rp.permission_key and p.deprecated_at is null
		where ura.account_id = $1::uuid and ura.user_id = $2::uuid

		union

		select permission_key
		from core.user_permission_overrides
		where account_id = $1::uuid and user_id = $2::uuid
		  and effect = 'allow' and is_active = true

		except

		select permission_key
		from core.user_permission_overrides
		where account_id = $1::uuid and user_id = $2::uuid
		  and effect = 'deny' and is_active = true

		order by 1 asc
	`, accountID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		permissions = append(permissions, key)
	}
	return permissions, rows.Err()
}

func (repository *PostgresRepository) FindOrganizationIDForAccount(ctx context.Context, accountID string) (*string, error) {
	var organizationID *string
	err := repository.pool.QueryRow(ctx, `
		select organization_id::text from core.accounts where id = $1::uuid
	`, accountID).Scan(&organizationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	return organizationID, err
}

func (repository *PostgresRepository) ListBoards(ctx context.Context, access AccessContext) ([]Board, error) {
	sql, args := repository.scopedQuery(access.AccountID, `
		select id::text, account_id::text, organization_id::text, slug, name, description,
		       icon, archived, created_by_user_id::text, created_at, updated_at
		from tasks.boards
		where account_id = $1::uuid and archived = false
		order by updated_at desc, name asc
	`)

	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	boards := make([]Board, 0)
	for rows.Next() {
		board, err := scanBoard(rows.Scan)
		if err != nil {
			return nil, err
		}
		boards = append(boards, board)
	}
	return boards, rows.Err()
}

func (repository *PostgresRepository) GetBoard(ctx context.Context, access AccessContext, boardID string) (Board, error) {
	sql, args := repository.scopedQuery(access.AccountID, `
		select id::text, account_id::text, organization_id::text, slug, name, description,
		       icon, archived, created_by_user_id::text, created_at, updated_at
		from tasks.boards
		where account_id = $1::uuid and id = $2::uuid and archived = false
	`, boardID)

	board, err := scanBoard(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Board{}, ErrBoardNotFound
	}
	if err != nil {
		return Board{}, err
	}

	if board.Columns, err = repository.listColumns(ctx, access.AccountID, board.ID); err != nil {
		return Board{}, err
	}
	if board.Fields, err = repository.listFields(ctx, access.AccountID, board.ID); err != nil {
		return Board{}, err
	}
	if board.Views, err = repository.listViews(ctx, access.AccountID, board.ID); err != nil {
		return Board{}, err
	}
	return board, nil
}

func (repository *PostgresRepository) CreateBoard(
	ctx context.Context,
	accountID string,
	input CreateBoardInput,
	createdByUserID string,
	organizationID *string,
) (Board, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Board{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sql, args := repository.scopedQuery(accountID, `
		insert into tasks.boards (
			account_id, organization_id, slug, name, description, icon, created_by_user_id
		) values ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::uuid)
		returning id::text, account_id::text, organization_id::text, slug, name, description,
		          icon, archived, created_by_user_id::text, created_at, updated_at
	`, organizationID, input.Slug, input.Name, input.Description, input.Icon, createdByUserID)

	board, err := scanBoard(tx.QueryRow(ctx, sql, args...).Scan)
	if err != nil {
		return Board{}, err
	}

	defaultColumns := []CreateColumnInput{
		{BoardID: board.ID, Label: "A fazer", Color: "slate", SortOrder: 100},
		{BoardID: board.ID, Label: "Em andamento", Color: "blue", SortOrder: 200},
		{BoardID: board.ID, Label: "Concluido", Color: "green", SortOrder: 300},
	}
	for _, column := range defaultColumns {
		if _, err := insertColumn(ctx, tx, accountID, column); err != nil {
			return Board{}, err
		}
	}

	defaultFields := []CreateFieldInput{
		{BoardID: board.ID, Key: "title", Label: "Titulo", Type: "title", Required: true, SortOrder: 10},
		{BoardID: board.ID, Key: "status", Label: "Status", Type: "status", SortOrder: 20},
		{BoardID: board.ID, Key: "responsible", Label: "Responsavel", Type: "person", SortOrder: 30},
		{BoardID: board.ID, Key: "priority", Label: "Prioridade", Type: "priority", SortOrder: 40},
		{BoardID: board.ID, Key: "due_date", Label: "Prazo", Type: "date", SortOrder: 50},
	}
	for _, field := range defaultFields {
		if _, err := insertField(ctx, tx, accountID, field); err != nil {
			return Board{}, err
		}
	}

	if _, err := tx.Exec(ctx, `
		insert into tasks.views (board_id, name, type, scope, config, sort_order)
		values
			($1::uuid, 'Board', 'board', 'board', '{"groupByFieldId":"status"}'::jsonb, 100),
			($1::uuid, 'Tabela', 'table', 'board', '{}'::jsonb, 200)
	`, board.ID); err != nil {
		return Board{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Board{}, err
	}

	return repository.GetBoard(ctx, AccessContext{AccountID: accountID, Perspective: PerspectiveAgency, IsPlatformAdmin: true}, board.ID)
}

func (repository *PostgresRepository) UpdateBoard(ctx context.Context, accountID string, input UpdateBoardInput) (Board, error) {
	sql, args := repository.scopedQuery(accountID, `
		update tasks.boards
		   set name = coalesce($3, name),
		       slug = coalesce($4, slug),
		       description = coalesce($5, description),
		       icon = coalesce($6, icon),
		       archived = coalesce($7, archived),
		       updated_at = now()
		 where account_id = $1::uuid and id = $2::uuid
		returning id::text, account_id::text, organization_id::text, slug, name, description,
		          icon, archived, created_by_user_id::text, created_at, updated_at
	`, input.ID, input.Name, input.Slug, input.Description, input.Icon, input.Archived)

	board, err := scanBoard(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Board{}, ErrBoardNotFound
	}
	if err != nil {
		return Board{}, err
	}
	return repository.GetBoard(ctx, AccessContext{AccountID: accountID, Perspective: PerspectiveAgency, IsPlatformAdmin: true}, board.ID)
}

func (repository *PostgresRepository) CreateColumn(ctx context.Context, accountID string, input CreateColumnInput) (Column, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Column{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	column, err := insertColumn(ctx, tx, accountID, input)
	if err != nil {
		return Column{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Column{}, err
	}
	return column, nil
}

func (repository *PostgresRepository) UpdateColumn(ctx context.Context, accountID string, input UpdateColumnInput) (Column, error) {
	sql, args := repository.scopedQuery(accountID, `
		update tasks.columns c
		   set label = coalesce($3, c.label),
		       color = coalesce($4, c.color),
		       sort_order = coalesce($5, c.sort_order)
		  from tasks.boards b
		 where c.board_id = b.id
		   and b.account_id = $1::uuid
		   and c.id = $2::uuid
		returning c.id::text, c.board_id::text, c.label, c.color, c.sort_order, c.created_at
	`, input.ID, input.Label, input.Color, input.SortOrder)

	column, err := scanColumn(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Column{}, ErrColumnNotFound
	}
	return column, err
}

func (repository *PostgresRepository) DeleteColumn(ctx context.Context, accountID string, input DeleteColumnInput) (string, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var boardID string
	sql, args := repository.scopedQuery(accountID, `
		select c.board_id::text
		from tasks.columns c
		join tasks.boards b on b.id = c.board_id
		where b.account_id = $1::uuid and c.id = $2::uuid
	`, input.ID)
	if err := tx.QueryRow(ctx, sql, args...).Scan(&boardID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrColumnNotFound
		}
		return "", err
	}

	if input.RemapToColumnID != "" {
		sql, args = repository.scopedQuery(accountID, `
			select 1
			from tasks.columns c
			join tasks.boards b on b.id = c.board_id
			where b.account_id = $1::uuid and c.id = $2::uuid and c.board_id = $3::uuid
		`, input.RemapToColumnID, boardID)
		var ok int
		if err := tx.QueryRow(ctx, sql, args...).Scan(&ok); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return "", ErrColumnNotFound
			}
			return "", err
		}
	}

	if input.RemapToColumnID == "" {
		if _, err := tx.Exec(ctx, `
			update tasks.tasks set column_id = null, updated_at = now(), version = version + 1
			where account_id = $1::uuid and column_id = $2::uuid
		`, accountID, input.ID); err != nil {
			return "", err
		}
	} else {
		if _, err := tx.Exec(ctx, `
			update tasks.tasks set column_id = $3::uuid, updated_at = now(), version = version + 1
			where account_id = $1::uuid and column_id = $2::uuid
		`, accountID, input.ID, input.RemapToColumnID); err != nil {
			return "", err
		}
	}

	tag, err := tx.Exec(ctx, `
		delete from tasks.columns
		where id = $1::uuid and board_id = $2::uuid
	`, input.ID, boardID)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		return "", ErrColumnNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return boardID, nil
}

func (repository *PostgresRepository) CreateField(ctx context.Context, accountID string, input CreateFieldInput) (Field, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Field{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	field, err := insertField(ctx, tx, accountID, input)
	if err != nil {
		return Field{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Field{}, err
	}
	return field, nil
}

func (repository *PostgresRepository) ListTasks(ctx context.Context, access AccessContext, input ListTasksInput) ([]Task, error) {
	sql, args := repository.scopedQuery(access.AccountID, `
		select t.id::text, t.account_id::text, t.board_id::text, t.column_id::text,
		       t.title, t.content_html, t.status, t.priority, t.due_date, t.start_date,
		       t.archived, t.sort_order::float8, t.created_by_user_id::text,
		       t.responsible_user_id::text, t.client_account_id::text, t.version,
		       t.created_at, t.updated_at
		from tasks.tasks t
		where t.account_id = $1::uuid and t.board_id = $2::uuid
		  and ($3::boolean = true or t.archived = false)
		order by t.sort_order asc, t.created_at asc
		limit $4
	`, input.BoardID, input.IncludeArchived, input.Limit)

	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]Task, 0)
	for rows.Next() {
		task, err := scanTask(rows.Scan)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (repository *PostgresRepository) GetTask(ctx context.Context, access AccessContext, taskID string) (Task, error) {
	sql, args := repository.scopedQuery(access.AccountID, `
		select t.id::text, t.account_id::text, t.board_id::text, t.column_id::text,
		       t.title, t.content_html, t.status, t.priority, t.due_date, t.start_date,
		       t.archived, t.sort_order::float8, t.created_by_user_id::text,
		       t.responsible_user_id::text, t.client_account_id::text, t.version,
		       t.created_at, t.updated_at
		from tasks.tasks t
		where t.account_id = $1::uuid and t.id = $2::uuid and t.archived = false
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
			client_account_id
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10,
			$11::uuid, $12::uuid, $13::uuid
		)
		returning id::text, account_id::text, board_id::text, column_id::text,
		          title, content_html, status, priority, due_date, start_date,
		          archived, sort_order::float8, created_by_user_id::text,
		          responsible_user_id::text, client_account_id::text, version,
		          created_at, updated_at
	`, input.BoardID, input.ColumnID, input.Title, input.ContentHTML, input.Status, input.Priority,
		input.DueDate, input.StartDate, input.SortOrder, createdByUserID, input.ResponsibleUserID, input.ClientAccountID)

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
		          responsible_user_id::text, client_account_id::text, version,
		          created_at, updated_at
	`, taskID)
	task, err := scanTask(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	return task, err
}

func (repository *PostgresRepository) AddComment(ctx context.Context, accountID string, input AddCommentInput, authorUserID string) (Comment, error) {
	sql, args := repository.scopedQuery(accountID, `
		insert into tasks.task_comments (task_id, author_user_id, body_html)
		select t.id, $3::uuid, $4
		from tasks.tasks t
		where t.account_id = $1::uuid and t.id = $2::uuid and t.archived = false
		returning id::text, task_id::text, author_user_id::text, body_html, created_at, updated_at, deleted_at
	`, input.TaskID, authorUserID, input.BodyHTML)
	comment, err := scanComment(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Comment{}, ErrTaskNotFound
	}
	return comment, err
}

func (repository *PostgresRepository) AddCommentMentions(ctx context.Context, accountID, taskID, commentID string, mentionedUserIDs []string) ([]string, error) {
	mentionedUserIDs = uniqueUserIDs(mentionedUserIDs...)
	if len(mentionedUserIDs) == 0 {
		return nil, nil
	}

	rows, err := repository.pool.Query(ctx, `
		with eligible_users as (
			select distinct au.user_id
			from core.account_users au
			join unnest($3::text[]) input(user_id_text) on au.user_id = input.user_id_text::uuid
			where au.account_id = $1::uuid and au.is_active = true
		)
		insert into tasks.task_mentions (task_id, comment_id, mentioned_user_id)
		select t.id, $2::uuid, eu.user_id
		from tasks.tasks t
		join eligible_users eu on true
		where t.account_id = $1::uuid and t.id = $4::uuid and t.archived = false
		returning mentioned_user_id::text
	`, accountID, commentID, mentionedUserIDs, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	inserted := make([]string, 0)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		inserted = append(inserted, userID)
	}
	return inserted, rows.Err()
}

func (repository *PostgresRepository) ListComments(ctx context.Context, access AccessContext, taskID string) ([]Comment, error) {
	sql, args := repository.scopedQuery(access.AccountID, `
		select c.id::text, c.task_id::text, c.author_user_id::text, c.body_html,
		       c.created_at, c.updated_at, c.deleted_at
		from tasks.task_comments c
		join tasks.tasks t on t.id = c.task_id
		where t.account_id = $1::uuid and t.id = $2::uuid and c.deleted_at is null
		order by c.created_at asc
	`, taskID)
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := make([]Comment, 0)
	for rows.Next() {
		comment, err := scanComment(rows.Scan)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func (repository *PostgresRepository) UpsertSubscribers(ctx context.Context, accountID, taskID string, userIDs []string) error {
	userIDs = uniqueUserIDs(userIDs...)
	if len(userIDs) == 0 {
		return nil
	}
	_, err := repository.pool.Exec(ctx, `
		with eligible_users as (
			select distinct au.user_id
			from core.account_users au
			join unnest($3::text[]) input(user_id_text) on au.user_id = input.user_id_text::uuid
			where au.account_id = $1::uuid and au.is_active = true
		)
		insert into tasks.task_subscribers (task_id, user_id)
		select t.id, eu.user_id
		from tasks.tasks t
		join eligible_users eu on true
		where t.account_id = $1::uuid and t.id = $2::uuid and t.archived = false
		on conflict (task_id, user_id) do nothing
	`, accountID, taskID, userIDs)
	return err
}

func (repository *PostgresRepository) ListSubscriberUserIDs(ctx context.Context, accountID, taskID string) ([]string, error) {
	rows, err := repository.pool.Query(ctx, `
		select s.user_id::text
		from tasks.task_subscribers s
		join tasks.tasks t on t.id = s.task_id
		where t.account_id = $1::uuid and t.id = $2::uuid and t.archived = false
		order by s.user_id::text asc
	`, accountID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	userIDs := make([]string, 0)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, rows.Err()
}

func (repository *PostgresRepository) AddShare(ctx context.Context, accountID string, input AddShareInput, sharedByUserID string) (Share, error) {
	sql, args := repository.scopedQuery(accountID, `
		insert into tasks.task_shares (task_id, client_account_id, permission, shared_by_user_id)
		select t.id, $3::uuid, $4, $5::uuid
		from tasks.tasks t
		where t.account_id = $1::uuid and t.id = $2::uuid and t.archived = false
		on conflict (task_id, client_account_id) where revoked_at is null
		do update set permission = excluded.permission
		returning id::text, task_id::text, client_account_id::text, permission,
		          shared_by_user_id::text, created_at, revoked_at
	`, input.TaskID, input.ClientAccountID, input.Permission, sharedByUserID)
	share, err := scanShare(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Share{}, ErrTaskNotFound
	}
	return share, err
}

func (repository *PostgresRepository) ListRelations(ctx context.Context, access AccessContext, taskID string) ([]Relation, error) {
	sql, args := repository.scopedQuery(access.AccountID, `
		select r.id::text, r.task_id::text, r.module, r.resource_type, r.resource_id,
		       r.label_cache, r.metadata_cache, r.refreshed_at
		from tasks.task_relations r
		join tasks.tasks t on t.id = r.task_id
		where t.account_id = $1::uuid and t.id = $2::uuid
		order by r.module asc, r.resource_type asc, r.label_cache asc
	`, taskID)
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	relations := make([]Relation, 0)
	for rows.Next() {
		relation, err := scanRelation(rows.Scan)
		if err != nil {
			return nil, err
		}
		relations = append(relations, relation)
	}
	return relations, rows.Err()
}

func (repository *PostgresRepository) AddRelation(ctx context.Context, accountID string, input AddRelationInput) (Relation, error) {
	metadataJSON, err := json.Marshal(normalizeMap(input.MetadataCache))
	if err != nil {
		return Relation{}, err
	}
	sql, args := repository.scopedQuery(accountID, `
		insert into tasks.task_relations (
			task_id, module, resource_type, resource_id, label_cache, metadata_cache
		)
		select t.id, $3, $4, $5, $6, $7::jsonb
		from tasks.tasks t
		where t.account_id = $1::uuid and t.id = $2::uuid and t.archived = false
		on conflict (task_id, module, resource_type, resource_id)
		do update set label_cache = excluded.label_cache,
		              metadata_cache = excluded.metadata_cache,
		              refreshed_at = now()
		returning id::text, task_id::text, module, resource_type, resource_id,
		          label_cache, metadata_cache, refreshed_at
	`, input.TaskID, input.Module, input.ResourceType, input.ResourceID, input.LabelCache, metadataJSON)
	relation, err := scanRelation(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Relation{}, ErrTaskNotFound
	}
	return relation, err
}

func (repository *PostgresRepository) ListAudit(ctx context.Context, accountID, taskID string) ([]AuditEntry, error) {
	sql, args := repository.scopedQuery(accountID, `
		select id, account_id::text, user_id::text, action, resource_type, resource_id,
		       coalesce(before, '{}'::jsonb), coalesce(after, '{}'::jsonb), at
		from tasks.audit_log
		where account_id = $1::uuid and resource_type = 'task' and resource_id = $2
		order by at desc
		limit 200
	`, taskID)
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]AuditEntry, 0)
	for rows.Next() {
		entry, err := scanAuditEntry(rows.Scan)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (repository *PostgresRepository) InsertAuditEntry(ctx context.Context, entry AuditEntry) error {
	beforeJSON, _ := json.Marshal(entry.Before)
	afterJSON, _ := json.Marshal(entry.After)
	_, err := repository.pool.Exec(ctx, `
		insert into tasks.audit_log (
			account_id, user_id, action, resource_type, resource_id, before, after
		) values ($1::uuid, $2::uuid, $3, $4, $5, $6::jsonb, $7::jsonb)
	`, entry.AccountID, entry.UserID, entry.Action, entry.ResourceType, entry.ResourceID, beforeJSON, afterJSON)
	return err
}

func (repository *PostgresRepository) ListActiveTimeEntries(ctx context.Context, access AccessContext) ([]TimeEntry, error) {
	sql, args := repository.scopedQuery(access.AccountID, `
		select id::text, task_id::text, user_id::text, account_id::text, started_at,
		       paused_at, resumed_at, stopped_at, duration_ms, notes, version,
		       created_at, updated_at
		from tasks.task_time_entries
		where account_id = $1::uuid and stopped_at is null
		  and ($2::boolean = true or user_id = $3::uuid)
		order by started_at desc
	`, access.Has(PermTrackingViewAll), access.UserID)
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]TimeEntry, 0)
	for rows.Next() {
		entry, err := scanTimeEntry(rows.Scan)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (repository *PostgresRepository) StartTracking(ctx context.Context, accountID, taskID, userID string) (TimeEntry, error) {
	sql, args := repository.scopedQuery(accountID, `
		insert into tasks.task_time_entries (task_id, user_id, account_id, started_at)
		select t.id, $3::uuid, t.account_id, now()
		from tasks.tasks t
		where t.account_id = $1::uuid and t.id = $2::uuid and t.archived = false
		returning id::text, task_id::text, user_id::text, account_id::text, started_at,
		          paused_at, resumed_at, stopped_at, duration_ms, notes, version,
		          created_at, updated_at
	`, taskID, userID)
	entry, err := scanTimeEntry(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return TimeEntry{}, ErrTaskNotFound
	}
	return entry, err
}

func (repository *PostgresRepository) PauseTracking(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error) {
	return repository.updateTracking(ctx, accountID, taskID, userID, expectedVersion, "pause")
}

func (repository *PostgresRepository) ResumeTracking(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error) {
	return repository.updateTracking(ctx, accountID, taskID, userID, expectedVersion, "resume")
}

func (repository *PostgresRepository) StopTracking(ctx context.Context, accountID, taskID, userID string, expectedVersion *int) (TimeEntry, error) {
	return repository.updateTracking(ctx, accountID, taskID, userID, expectedVersion, "stop")
}

func (repository *PostgresRepository) TrackingMetrics(ctx context.Context, accountID string, input TrackingMetricsInput) (TrackingMetrics, error) {
	query := strings.Builder{}
	query.WriteString(`
		select coalesce(sum(duration_ms), 0)::bigint, count(*)::bigint
		from tasks.task_time_entries e
		join tasks.tasks t on t.id = e.task_id
		where e.account_id = $1::uuid
	`)
	args := []any{accountID}
	position := 2
	if input.UserID != "" {
		query.WriteString(fmt.Sprintf(" and e.user_id = $%d::uuid", position))
		args = append(args, input.UserID)
		position++
	}
	if input.ClientAccountID != "" {
		query.WriteString(fmt.Sprintf(" and t.client_account_id = $%d::uuid", position))
		args = append(args, input.ClientAccountID)
		position++
	}
	if input.From != nil {
		query.WriteString(fmt.Sprintf(" and e.started_at >= $%d", position))
		args = append(args, *input.From)
		position++
	}
	if input.To != nil {
		query.WriteString(fmt.Sprintf(" and e.started_at <= $%d", position))
		args = append(args, *input.To)
	}

	var metrics TrackingMetrics
	err := repository.pool.QueryRow(ctx, query.String(), args...).Scan(&metrics.TotalDurationMs, &metrics.EntryCount)
	return metrics, err
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
		       version = version + 1,
		       updated_at = now()
		 where account_id = $1::uuid and id = $2::uuid
		returning id::text, account_id::text, board_id::text, column_id::text,
		          title, content_html, status, priority, due_date, start_date,
		          archived, sort_order::float8, created_by_user_id::text,
		          responsible_user_id::text, client_account_id::text, version,
		          created_at, updated_at
	`, task.ID, task.ColumnID, task.Title, task.ContentHTML, task.Status, task.Priority,
		task.DueDate, task.StartDate, task.Archived, task.SortOrder, task.ResponsibleUserID, task.ClientAccountID)
	updated, err := scanTask(repository.pool.QueryRow(ctx, sql, args...).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	return updated, err
}

func (repository *PostgresRepository) updateTracking(
	ctx context.Context,
	accountID string,
	taskID string,
	userID string,
	expectedVersion *int,
	action string,
) (TimeEntry, error) {
	var query string
	switch action {
	case "pause":
		query = `
			update tasks.task_time_entries
			   set paused_at = now(),
			       duration_ms = duration_ms + floor(extract(epoch from (now() - coalesce(resumed_at, started_at))) * 1000)::bigint,
			       version = version + 1,
			       updated_at = now()
			 where account_id = $1::uuid and task_id = $2::uuid and user_id = $3::uuid
			   and stopped_at is null and paused_at is null
			   and ($4::integer is null or version = $4)
			returning id::text, task_id::text, user_id::text, account_id::text, started_at,
			          paused_at, resumed_at, stopped_at, duration_ms, notes, version,
			          created_at, updated_at
		`
	case "resume":
		query = `
			update tasks.task_time_entries
			   set resumed_at = now(),
			       paused_at = null,
			       version = version + 1,
			       updated_at = now()
			 where account_id = $1::uuid and task_id = $2::uuid and user_id = $3::uuid
			   and stopped_at is null and paused_at is not null
			   and ($4::integer is null or version = $4)
			returning id::text, task_id::text, user_id::text, account_id::text, started_at,
			          paused_at, resumed_at, stopped_at, duration_ms, notes, version,
			          created_at, updated_at
		`
	case "stop":
		query = `
			update tasks.task_time_entries
			   set stopped_at = now(),
			       duration_ms = case
			           when paused_at is null then duration_ms + floor(extract(epoch from (now() - coalesce(resumed_at, started_at))) * 1000)::bigint
			           else duration_ms
			       end,
			       version = version + 1,
			       updated_at = now()
			 where account_id = $1::uuid and task_id = $2::uuid and user_id = $3::uuid
			   and stopped_at is null
			   and ($4::integer is null or version = $4)
			returning id::text, task_id::text, user_id::text, account_id::text, started_at,
			          paused_at, resumed_at, stopped_at, duration_ms, notes, version,
			          created_at, updated_at
		`
	default:
		return TimeEntry{}, ErrValidation
	}

	entry, err := scanTimeEntry(repository.pool.QueryRow(ctx, query, accountID, taskID, userID, expectedVersion).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return TimeEntry{}, ErrTimeEntryNotFound
	}
	return entry, err
}

func (repository *PostgresRepository) listColumns(ctx context.Context, accountID, boardID string) ([]Column, error) {
	sql, args := repository.scopedQuery(accountID, `
		select c.id::text, c.board_id::text, c.label, c.color, c.sort_order, c.created_at
		from tasks.columns c
		join tasks.boards b on b.id = c.board_id
		where b.account_id = $1::uuid and c.board_id = $2::uuid
		order by c.sort_order asc, c.created_at asc
	`, boardID)
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make([]Column, 0)
	for rows.Next() {
		column, err := scanColumn(rows.Scan)
		if err != nil {
			return nil, err
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func (repository *PostgresRepository) listFields(ctx context.Context, accountID, boardID string) ([]Field, error) {
	sql, args := repository.scopedQuery(accountID, `
		select f.id::text, f.board_id::text, f.key, f.label, f.type, f.required,
		       f.hidden, f.sort_order, f.config
		from tasks.fields f
		join tasks.boards b on b.id = f.board_id
		where b.account_id = $1::uuid and f.board_id = $2::uuid
		order by f.sort_order asc, f.label asc
	`, boardID)
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := make([]Field, 0)
	for rows.Next() {
		field, err := scanField(rows.Scan)
		if err != nil {
			return nil, err
		}
		field.Options, err = repository.listFieldOptions(ctx, field.ID)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, rows.Err()
}

func (repository *PostgresRepository) listFieldOptions(ctx context.Context, fieldID string) ([]FieldOption, error) {
	rows, err := repository.pool.Query(ctx, `
		select id::text, field_id::text, value, label, color, coalesce(sort_order, 100)
		from tasks.field_options
		where field_id = $1::uuid
		order by sort_order asc, label asc
	`, fieldID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	options := make([]FieldOption, 0)
	for rows.Next() {
		var option FieldOption
		if err := rows.Scan(&option.ID, &option.FieldID, &option.Value, &option.Label, &option.Color, &option.SortOrder); err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	return options, rows.Err()
}

func (repository *PostgresRepository) listViews(ctx context.Context, accountID, boardID string) ([]View, error) {
	sql, args := repository.scopedQuery(accountID, `
		select v.id::text, v.board_id::text, v.name, v.type, v.scope, v.owner_user_id::text,
		       v.config, v.sort_order, v.created_at, v.updated_at
		from tasks.views v
		join tasks.boards b on b.id = v.board_id
		where b.account_id = $1::uuid and v.board_id = $2::uuid
		order by v.sort_order asc, v.name asc
	`, boardID)
	rows, err := repository.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	views := make([]View, 0)
	for rows.Next() {
		view, err := scanView(rows.Scan)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, rows.Err()
}

type txQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func insertColumn(ctx context.Context, tx txQueryer, accountID string, input CreateColumnInput) (Column, error) {
	sql := `
		insert into tasks.columns (board_id, label, color, sort_order)
		select b.id, $3, $4, case when $5::integer = 0 then 100 else $5 end
		from tasks.boards b
		where b.account_id = $1::uuid and b.id = $2::uuid and b.archived = false
		returning id::text, board_id::text, label, color, sort_order, created_at
	`
	column, err := scanColumn(tx.QueryRow(ctx, sql, accountID, input.BoardID, input.Label, input.Color, input.SortOrder).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Column{}, ErrBoardNotFound
	}
	return column, err
}

func insertField(ctx context.Context, tx txQueryer, accountID string, input CreateFieldInput) (Field, error) {
	configJSON, err := json.Marshal(normalizeMap(input.Config))
	if err != nil {
		return Field{}, err
	}
	field, err := scanField(tx.QueryRow(ctx, `
		insert into tasks.fields (board_id, key, label, type, required, hidden, sort_order, config)
		select b.id, $3, $4, $5, $6, $7, case when $8::integer = 0 then 100 else $8 end, $9::jsonb
		from tasks.boards b
		where b.account_id = $1::uuid and b.id = $2::uuid and b.archived = false
		returning id::text, board_id::text, key, label, type, required, hidden, sort_order, config
	`, accountID, input.BoardID, input.Key, input.Label, input.Type, input.Required, input.Hidden, input.SortOrder, configJSON).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Field{}, ErrBoardNotFound
	}
	if err != nil {
		return Field{}, err
	}

	for _, option := range input.Options {
		value := strings.TrimSpace(option.Value)
		label := strings.TrimSpace(option.Label)
		if value == "" || label == "" {
			continue
		}
		color := defaultString(strings.TrimSpace(option.Color), "slate")
		sortOrder := option.SortOrder
		if sortOrder == 0 {
			sortOrder = 100
		}
		if _, err := tx.Exec(ctx, `
			insert into tasks.field_options (field_id, value, label, color, sort_order)
			values ($1::uuid, $2, $3, $4, $5)
			on conflict (field_id, value) do update set
				label = excluded.label,
				color = excluded.color,
				sort_order = excluded.sort_order
		`, field.ID, value, label, color, sortOrder); err != nil {
			return Field{}, err
		}
	}
	return field, nil
}

func scanBoard(scan func(...any) error) (Board, error) {
	var board Board
	err := scan(
		&board.ID,
		&board.AccountID,
		&board.OrganizationID,
		&board.Slug,
		&board.Name,
		&board.Description,
		&board.Icon,
		&board.Archived,
		&board.CreatedByUserID,
		&board.CreatedAt,
		&board.UpdatedAt,
	)
	return board, err
}

func scanColumn(scan func(...any) error) (Column, error) {
	var column Column
	err := scan(&column.ID, &column.BoardID, &column.Label, &column.Color, &column.SortOrder, &column.CreatedAt)
	return column, err
}

func scanField(scan func(...any) error) (Field, error) {
	var field Field
	var configRaw []byte
	err := scan(&field.ID, &field.BoardID, &field.Key, &field.Label, &field.Type, &field.Required, &field.Hidden, &field.SortOrder, &configRaw)
	if err != nil {
		return Field{}, err
	}
	field.Config = decodeMap(configRaw)
	return field, nil
}

func scanView(scan func(...any) error) (View, error) {
	var view View
	var configRaw []byte
	err := scan(&view.ID, &view.BoardID, &view.Name, &view.Type, &view.Scope, &view.OwnerUserID, &configRaw, &view.SortOrder, &view.CreatedAt, &view.UpdatedAt)
	if err != nil {
		return View{}, err
	}
	view.Config = decodeMap(configRaw)
	return view, nil
}

func scanTask(scan func(...any) error) (Task, error) {
	var task Task
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
		&task.Version,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	return task, err
}

func scanComment(scan func(...any) error) (Comment, error) {
	var comment Comment
	err := scan(&comment.ID, &comment.TaskID, &comment.AuthorUserID, &comment.BodyHTML, &comment.CreatedAt, &comment.UpdatedAt, &comment.DeletedAt)
	return comment, err
}

func scanShare(scan func(...any) error) (Share, error) {
	var share Share
	err := scan(&share.ID, &share.TaskID, &share.ClientAccountID, &share.Permission, &share.SharedByUserID, &share.CreatedAt, &share.RevokedAt)
	return share, err
}

func scanRelation(scan func(...any) error) (Relation, error) {
	var relation Relation
	var metadataRaw []byte
	err := scan(&relation.ID, &relation.TaskID, &relation.Module, &relation.ResourceType, &relation.ResourceID, &relation.LabelCache, &metadataRaw, &relation.RefreshedAt)
	if err != nil {
		return Relation{}, err
	}
	relation.MetadataCache = decodeMap(metadataRaw)
	return relation, nil
}

func scanTimeEntry(scan func(...any) error) (TimeEntry, error) {
	var entry TimeEntry
	err := scan(&entry.ID, &entry.TaskID, &entry.UserID, &entry.AccountID, &entry.StartedAt, &entry.PausedAt, &entry.ResumedAt, &entry.StoppedAt, &entry.DurationMs, &entry.Notes, &entry.Version, &entry.CreatedAt, &entry.UpdatedAt)
	return entry, err
}

func scanAuditEntry(scan func(...any) error) (AuditEntry, error) {
	var entry AuditEntry
	var beforeRaw, afterRaw []byte
	err := scan(&entry.ID, &entry.AccountID, &entry.UserID, &entry.Action, &entry.ResourceType, &entry.ResourceID, &beforeRaw, &afterRaw, &entry.At)
	if err != nil {
		return AuditEntry{}, err
	}
	entry.Before = decodeMap(beforeRaw)
	entry.After = decodeMap(afterRaw)
	return entry, nil
}

func decodeMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return map[string]any{}
	}
	return value
}

func normalizeMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	normalized := make(map[string]any, len(value))
	for key, item := range value {
		normalized[strings.TrimSpace(key)] = item
	}
	return normalized
}

func parseOptionalTime(raw string) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
