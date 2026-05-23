package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type txQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
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
		{BoardID: board.ID, Label: "Raw", Color: "slate", SortOrder: 100},
		{BoardID: board.ID, Label: "Standby", Color: "violet", SortOrder: 200},
		{BoardID: board.ID, Label: "Running", Color: "blue", SortOrder: 300},
		{BoardID: board.ID, Label: "Aguardando aprovacao", Color: "amber", SortOrder: 400},
		{BoardID: board.ID, Label: "Aprovada", Color: "green", SortOrder: 500},
		{BoardID: board.ID, Label: "Finalizada", Color: "indigo", SortOrder: 600},
		{BoardID: board.ID, Label: "Rotina", Color: "rose", SortOrder: 700},
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
