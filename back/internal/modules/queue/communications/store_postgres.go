package communications

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const communicationSelect = `
	select
		communication.id::text,
		communication.account_id::text,
		communication.title,
		communication.excerpt,
		communication.body,
		communication.starts_at,
		communication.ends_at,
		communication.is_published,
		communication.display_order,
		communication.targets_all_stores,
		coalesce(
			array_agg(target.store_id::text order by target.store_id)
				filter (where target.store_id is not null),
			'{}'::text[]
		),
		communication.created_by::text,
		communication.updated_by::text,
		communication.created_at,
		communication.updated_at
	from queue.communications communication
	left join queue.communication_stores target
	  on target.account_id = communication.account_id
	 and target.communication_id = communication.id
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCommunication(scanner rowScanner) (Communication, error) {
	var item Communication
	err := scanner.Scan(
		&item.ID,
		&item.AccountID,
		&item.Title,
		&item.Excerpt,
		&item.Body,
		&item.StartsAt,
		&item.EndsAt,
		&item.IsPublished,
		&item.DisplayOrder,
		&item.TargetsAllStores,
		&item.StoreIDs,
		&item.CreatedBy,
		&item.UpdatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}

func (repository *PostgresRepository) List(
	ctx context.Context,
	accountID string,
	filter ListFilter,
) ([]Communication, error) {
	rows, err := repository.pool.Query(ctx, communicationSelect+`
		where communication.account_id = $1::uuid
		  and communication.archived_at is null
		  and (
		    not $2::boolean
		    or (
		      communication.is_published
		      and (communication.starts_at is null or communication.starts_at <= now())
		      and (communication.ends_at is null or communication.ends_at > now())
		    )
		  )
		  and (
		    $3::text = ''
		    or communication.targets_all_stores
		    or exists (
		      select 1
		      from queue.communication_stores selected_target
		      where selected_target.account_id = communication.account_id
		        and selected_target.communication_id = communication.id
		        and selected_target.store_id = nullif($3::text, '')::uuid
		    )
		  )
		group by communication.id
		order by communication.display_order asc, communication.updated_at desc, communication.id;
	`, accountID, filter.PublishedOnly, filter.StoreID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Communication, 0)
	for rows.Next() {
		item, scanErr := scanCommunication(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *PostgresRepository) Get(
	ctx context.Context,
	accountID, communicationID string,
) (Communication, error) {
	item, err := scanCommunication(repository.pool.QueryRow(ctx, communicationSelect+`
		where communication.account_id = $1::uuid
		  and communication.id = $2::uuid
		  and communication.archived_at is null
		group by communication.id;
	`, accountID, communicationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Communication{}, ErrNotFound
	}
	return item, err
}

func (repository *PostgresRepository) StoresBelongToAccount(
	ctx context.Context,
	accountID string,
	storeIDs []string,
) (bool, error) {
	var count int
	err := repository.pool.QueryRow(ctx, `
		select count(*)
		from queue.stores
		where tenant_id = $1::uuid
		  and id::text = any($2::text[]);
	`, accountID, storeIDs).Scan(&count)
	return count == len(storeIDs), err
}

func replaceTargets(
	ctx context.Context,
	tx pgx.Tx,
	communication Communication,
) error {
	if _, err := tx.Exec(ctx, `
		delete from queue.communication_stores
		where account_id = $1::uuid
		  and communication_id = $2::uuid;
	`, communication.AccountID, communication.ID); err != nil {
		return err
	}
	if communication.TargetsAllStores {
		return nil
	}
	for _, storeID := range communication.StoreIDs {
		if _, err := tx.Exec(ctx, `
			insert into queue.communication_stores (
				account_id,
				communication_id,
				store_id
			)
			values ($1::uuid, $2::uuid, $3::uuid);
		`, communication.AccountID, communication.ID, storeID); err != nil {
			return err
		}
	}
	return nil
}

func (repository *PostgresRepository) Create(
	ctx context.Context,
	communication Communication,
) (Communication, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Communication{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx, `
		insert into queue.communications (
			account_id,
			title,
			excerpt,
			body,
			starts_at,
			ends_at,
			is_published,
			display_order,
			targets_all_stores,
			created_by,
			updated_by
		)
		values (
			$1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10::uuid, $10::uuid
		)
		returning id::text;
	`,
		communication.AccountID,
		communication.Title,
		communication.Excerpt,
		communication.Body,
		communication.StartsAt,
		communication.EndsAt,
		communication.IsPublished,
		communication.DisplayOrder,
		communication.TargetsAllStores,
		communication.CreatedBy,
	).Scan(&communication.ID)
	if err != nil {
		return Communication{}, err
	}
	if err = replaceTargets(ctx, tx, communication); err != nil {
		return Communication{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Communication{}, err
	}
	return repository.Get(ctx, communication.AccountID, communication.ID)
}

func (repository *PostgresRepository) Update(
	ctx context.Context,
	communication Communication,
) (Communication, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Communication{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	command, err := tx.Exec(ctx, `
		update queue.communications
		set title = $3,
		    excerpt = $4,
		    body = $5,
		    starts_at = $6,
		    ends_at = $7,
		    is_published = $8,
		    display_order = $9,
		    targets_all_stores = $10,
		    updated_by = $11::uuid,
		    updated_at = now()
		where account_id = $1::uuid
		  and id = $2::uuid
		  and archived_at is null;
	`,
		communication.AccountID,
		communication.ID,
		communication.Title,
		communication.Excerpt,
		communication.Body,
		communication.StartsAt,
		communication.EndsAt,
		communication.IsPublished,
		communication.DisplayOrder,
		communication.TargetsAllStores,
		communication.UpdatedBy,
	)
	if err != nil {
		return Communication{}, err
	}
	if command.RowsAffected() == 0 {
		return Communication{}, ErrNotFound
	}
	if err = replaceTargets(ctx, tx, communication); err != nil {
		return Communication{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Communication{}, err
	}
	return repository.Get(ctx, communication.AccountID, communication.ID)
}

func (repository *PostgresRepository) Archive(
	ctx context.Context,
	accountID, communicationID, updatedBy string,
) error {
	command, err := repository.pool.Exec(ctx, `
		update queue.communications
		set archived_at = now(),
		    updated_at = now(),
		    updated_by = $3::uuid
		where account_id = $1::uuid
		  and id = $2::uuid
		  and archived_at is null;
	`, accountID, communicationID, updatedBy)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
