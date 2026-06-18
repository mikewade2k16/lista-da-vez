package stores

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) ListAccessible(ctx context.Context, principal auth.Principal, input ListInput) ([]Store, error) {
	query, args := buildListAccessibleQuery(principal, input)
	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stores := make([]Store, 0)
	for rows.Next() {
		store, err := scanStore(rows)
		if err != nil {
			return nil, err
		}

		stores = append(stores, store)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stores, nil
}

func (repository *PostgresRepository) FindAccessibleByID(ctx context.Context, principal auth.Principal, storeID string) (Store, error) {
	query, args := buildFindAccessibleQuery(principal, storeID)
	store, err := scanStore(repository.pool.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Store{}, ErrStoreNotFound
		}

		return Store{}, err
	}

	return store, nil
}

func (repository *PostgresRepository) Create(ctx context.Context, store Store) (Store, error) {
	query := `
		insert into queue.stores (
			tenant_id,
			code,
			name,
			city,
			default_template_id,
			store_type,
			monthly_goal,
			weekly_goal,
			avg_ticket_goal,
			conversion_goal,
			pa_goal,
			is_active
		)
		values (
			$1::uuid,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12
		)
		returning
			id::text,
			tenant_id::text,
			code,
			name,
			city,
			default_template_id,
			store_type,
			monthly_goal,
			weekly_goal,
			avg_ticket_goal,
			conversion_goal,
			pa_goal,
			is_active,
			created_at,
			updated_at;
	`

	created, err := scanStore(repository.pool.QueryRow(
		ctx,
		query,
		store.TenantID,
		store.Code,
		store.Name,
		store.City,
		store.DefaultTemplateID,
		store.StoreType,
		store.MonthlyGoal,
		store.WeeklyGoal,
		store.AvgTicketGoal,
		store.ConversionGoal,
		store.PAGoal,
		store.Active,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return Store{}, ErrStoreConflict
		}

		return Store{}, err
	}

	return created, nil
}

func (repository *PostgresRepository) Update(ctx context.Context, store Store) (Store, error) {
	query := `
		update queue.stores
		set
			code = $2,
			name = $3,
			city = $4,
			default_template_id = $5,
			store_type = $6,
			monthly_goal = $7,
			weekly_goal = $8,
			avg_ticket_goal = $9,
			conversion_goal = $10,
			pa_goal = $11,
			is_active = $12,
			updated_at = now()
		where id = $1::uuid
		returning
			id::text,
			tenant_id::text,
			code,
			name,
			city,
			default_template_id,
			store_type,
			monthly_goal,
			weekly_goal,
			avg_ticket_goal,
			conversion_goal,
			pa_goal,
			is_active,
			created_at,
			updated_at;
	`

	updated, err := scanStore(repository.pool.QueryRow(
		ctx,
		query,
		store.ID,
		store.Code,
		store.Name,
		store.City,
		store.DefaultTemplateID,
		store.StoreType,
		store.MonthlyGoal,
		store.WeeklyGoal,
		store.AvgTicketGoal,
		store.ConversionGoal,
		store.PAGoal,
		store.Active,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Store{}, ErrStoreNotFound
		}

		if isUniqueViolation(err) {
			return Store{}, ErrStoreConflict
		}

		return Store{}, err
	}

	return updated, nil
}

func (repository *PostgresRepository) Delete(ctx context.Context, storeID string) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
		update queue.consultants
		set store_id = null
		where store_id = $1::uuid;
	`, storeID); err != nil {
		return err
	}

	// U4b: o legado `delete from user_store_roles where store_id` foi removido;
	// a limpeza do escopo da loja agora e so em core (deleteCoreStoreScopeTx).
	if err := deleteCoreStoreScopeTx(ctx, tx, storeID); err != nil {
		return err
	}

	commandTag, err := tx.Exec(ctx, `
		delete from queue.stores
		where id = $1::uuid;
	`, storeID)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return ErrStoreNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func scanStore(row pgx.Row) (Store, error) {
	var store Store
	err := row.Scan(
		&store.ID,
		&store.TenantID,
		&store.Code,
		&store.Name,
		&store.City,
		&store.DefaultTemplateID,
		&store.StoreType,
		&store.MonthlyGoal,
		&store.WeeklyGoal,
		&store.AvgTicketGoal,
		&store.ConversionGoal,
		&store.PAGoal,
		&store.Active,
		&store.CreatedAt,
		&store.UpdatedAt,
	)
	if err != nil {
		return Store{}, err
	}

	store.Code = strings.ToUpper(strings.TrimSpace(store.Code))
	store.Name = strings.TrimSpace(store.Name)
	store.City = strings.TrimSpace(store.City)
	store.DefaultTemplateID = strings.TrimSpace(store.DefaultTemplateID)
	store.StoreType = strings.ToLower(strings.TrimSpace(store.StoreType))
	if store.StoreType == "" {
		store.StoreType = StoreTypeBairro
	}

	return store, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	return false
}
