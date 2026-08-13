package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const providerName = "cloudflare_r2"

type Repository interface {
	ProviderState(ctx context.Context) (ProviderState, error)
	InitializeProvider(ctx context.Context, accountID, bucket string) (ProviderState, error)
	TouchProvider(ctx context.Context) error
	Settings(ctx context.Context) (Settings, error)
	UpdateSettings(ctx context.Context, input UpdateSettingsInput, actorID string) (Settings, error)
	ReserveRequest(ctx context.Context, requestClass string, billingMonth time.Time) error
	ReserveUpload(ctx context.Context, object Object, billingMonth time.Time) (Object, bool, error)
	MarkAvailable(ctx context.Context, accountID, objectID, etag string) (Object, error)
	MarkFailed(ctx context.Context, accountID, objectID string) error
	Object(ctx context.Context, accountID, objectID string) (Object, error)
	Usage(ctx context.Context, billingMonth time.Time) (Usage, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

type PendingObjectRepository interface {
	PendingObjects(ctx context.Context, limit int) ([]Object, error)
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) ProviderState(ctx context.Context) (ProviderState, error) {
	var state ProviderState
	err := repository.pool.QueryRow(ctx, `
		select account_identifier, bucket_name, initialized_at, checked_at
		from storage.provider_state
		where provider = $1
	`, providerName).Scan(&state.AccountID, &state.Bucket, &state.InitializedAt, &state.CheckedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProviderState{}, ErrNotInitialized
	}
	return state, err
}

func (repository *PostgresRepository) InitializeProvider(ctx context.Context, accountID, bucket string) (ProviderState, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return ProviderState{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('storage.r2.provider'))`); err != nil {
		return ProviderState{}, err
	}

	var state ProviderState
	err = tx.QueryRow(ctx, `
		insert into storage.provider_state (provider, account_identifier, bucket_name)
		values ($1, $2, $3)
		on conflict (provider) do nothing
		returning account_identifier, bucket_name, initialized_at, checked_at
	`, providerName, accountID, bucket).Scan(
		&state.AccountID,
		&state.Bucket,
		&state.InitializedAt,
		&state.CheckedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			select account_identifier, bucket_name, initialized_at, checked_at
			from storage.provider_state
			where provider = $1
		`, providerName).Scan(&state.AccountID, &state.Bucket, &state.InitializedAt, &state.CheckedAt)
	}
	if err != nil {
		return ProviderState{}, err
	}
	if state.AccountID != accountID || state.Bucket != bucket {
		return ProviderState{}, ErrProviderMismatch
	}
	if err := tx.Commit(ctx); err != nil {
		return ProviderState{}, err
	}
	return state, nil
}

func (repository *PostgresRepository) TouchProvider(ctx context.Context) error {
	command, err := repository.pool.Exec(ctx, `
		update storage.provider_state
		set checked_at = now()
		where provider = $1
	`, providerName)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrNotInitialized
	}
	return nil
}

func (repository *PostgresRepository) Settings(ctx context.Context) (Settings, error) {
	var settings Settings
	err := repository.pool.QueryRow(ctx, `
		select uploads_enabled, billing_cycle_day, storage_limit_bytes, class_a_limit, class_b_limit, max_object_bytes,
			image_max_bytes, video_max_bytes, audio_max_bytes, document_max_bytes,
			coalesce(updated_by::text, ''), updated_at
		from storage.settings
		where id = 1
	`).Scan(settingsScanTargets(&settings)...)
	return settings, err
}

func (repository *PostgresRepository) UpdateSettings(
	ctx context.Context,
	input UpdateSettingsInput,
	actorID string,
) (Settings, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Settings{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('storage.r2.global-budget'))`); err != nil {
		return Settings{}, err
	}

	var settings Settings
	err = tx.QueryRow(ctx, `
		update storage.settings
		set uploads_enabled = $1,
			billing_cycle_day = $2,
			storage_limit_bytes = $3,
			class_a_limit = $4,
			class_b_limit = $5,
			max_object_bytes = greatest($6::bigint, $7::bigint, $8::bigint, $9::bigint),
			image_max_bytes = $6,
			video_max_bytes = $7,
			audio_max_bytes = $8,
			document_max_bytes = $9,
			updated_by = $10::uuid,
			updated_at = now()
		where id = 1
		returning uploads_enabled, billing_cycle_day, storage_limit_bytes, class_a_limit, class_b_limit, max_object_bytes,
			image_max_bytes, video_max_bytes, audio_max_bytes, document_max_bytes,
			coalesce(updated_by::text, ''), updated_at
	`, input.UploadsEnabled, input.BillingCycleDay, input.StorageLimitBytes, input.ClassALimit, input.ClassBLimit, input.ImageMaxBytes,
		input.VideoMaxBytes, input.AudioMaxBytes, input.DocumentMaxBytes, actorID).
		Scan(settingsScanTargets(&settings)...)
	if err != nil {
		return Settings{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func (repository *PostgresRepository) ReserveRequest(
	ctx context.Context,
	requestClass string,
	billingMonth time.Time,
) error {
	column := ""
	limitColumn := ""
	var quotaErr error
	switch requestClass {
	case "A":
		column = "class_a_requests"
		limitColumn = "class_a_limit"
		quotaErr = ErrClassAQuotaExceeded
	case "B":
		column = "class_b_requests"
		limitColumn = "class_b_limit"
		quotaErr = ErrClassBQuotaExceeded
	default:
		return fmt.Errorf("%w: unknown request class", ErrInvalidUpload)
	}

	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('storage.r2.global-budget'))`); err != nil {
		return err
	}
	var limit int64
	limitQuery := fmt.Sprintf(`select %s from storage.settings where id = 1`, limitColumn)
	if err = tx.QueryRow(ctx, limitQuery).Scan(&limit); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		insert into storage.monthly_usage (billing_month)
		values ($1)
		on conflict (billing_month) do nothing
	`, billingMonth); err != nil {
		return err
	}

	query := fmt.Sprintf(`
		update storage.monthly_usage
		set %s = %s + 1, updated_at = now()
		where billing_month = $1 and %s < $2
	`, column, column, column)
	command, err := tx.Exec(ctx, query, billingMonth, limit)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return quotaErr
	}
	return tx.Commit(ctx)
}

func (repository *PostgresRepository) ReserveUpload(
	ctx context.Context,
	object Object,
	billingMonth time.Time,
) (Object, bool, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Object{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('storage.r2.global-budget'))`); err != nil {
		return Object{}, false, err
	}
	var settings Settings
	if err = tx.QueryRow(ctx, `
		select uploads_enabled, billing_cycle_day, storage_limit_bytes, class_a_limit, class_b_limit, max_object_bytes,
			image_max_bytes, video_max_bytes, audio_max_bytes, document_max_bytes,
			coalesce(updated_by::text, ''), updated_at
		from storage.settings
		where id = 1
	`).Scan(settingsScanTargets(&settings)...); err != nil {
		return Object{}, false, err
	}
	if object.SizeBytes > settings.MaxObjectBytes {
		return Object{}, false, ErrInvalidUpload
	}

	existing, err := findByIdempotency(ctx, tx, object.AccountID, object.SourceModule, object.IdempotencyKey)
	if err == nil {
		return existing, true, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Object{}, false, err
	}

	var reservedBytes int64
	if err := tx.QueryRow(ctx, `
		select coalesce(sum(size_bytes), 0)
		from storage.objects
		where status in ('pending', 'available')
	`).Scan(&reservedBytes); err != nil {
		return Object{}, false, err
	}
	if object.SizeBytes > settings.StorageLimitBytes-reservedBytes {
		return Object{}, false, ErrStorageQuotaExceeded
	}

	if _, err = tx.Exec(ctx, `
		insert into storage.monthly_usage (billing_month)
		values ($1)
		on conflict (billing_month) do nothing
	`, billingMonth); err != nil {
		return Object{}, false, err
	}
	command, err := tx.Exec(ctx, `
		update storage.monthly_usage
		set class_a_requests = class_a_requests + 1, updated_at = now()
		where billing_month = $1 and class_a_requests < $2
	`, billingMonth, settings.ClassALimit)
	if err != nil {
		return Object{}, false, err
	}
	if command.RowsAffected() != 1 {
		return Object{}, false, ErrClassAQuotaExceeded
	}

	err = tx.QueryRow(ctx, `
		insert into storage.objects (
			id, account_id, source_module, idempotency_key, object_key,
			file_name, content_type, size_bytes, created_by
		)
		values ($1, $2::uuid, $3, $4, $5, $6, $7, $8, $9::uuid)
		returning id, account_id::text, source_module, idempotency_key, object_key,
			file_name, content_type, size_bytes, etag, status, created_by::text,
			created_at, available_at
	`, object.ID, object.AccountID, object.SourceModule, object.IdempotencyKey, object.ObjectKey,
		object.FileName, object.ContentType, object.SizeBytes, object.CreatedBy).Scan(objectScanTargets(&object)...)
	if err != nil {
		return Object{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Object{}, false, err
	}
	return object, false, nil
}

func (repository *PostgresRepository) MarkAvailable(ctx context.Context, accountID, objectID, etag string) (Object, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Object{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var object Object
	err = tx.QueryRow(ctx, `
		update storage.objects
		set status = 'available', etag = $3, available_at = now()
		where id = $1 and account_id = $2::uuid and status = 'pending'
		returning id, account_id::text, source_module, idempotency_key, object_key,
			file_name, content_type, size_bytes, etag, status, created_by::text,
			created_at, available_at
	`, objectID, accountID, etag).Scan(objectScanTargets(&object)...)
	if err != nil {
		return Object{}, err
	}
	if _, err = tx.Exec(ctx, `
		update storage.monthly_usage
		set uploaded_bytes = uploaded_bytes + $2, updated_at = now()
		where billing_month = date_trunc('month', $1::timestamptz)::date
	`, object.CreatedAt, object.SizeBytes); err != nil {
		return Object{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Object{}, err
	}
	return object, nil
}

func (repository *PostgresRepository) MarkFailed(ctx context.Context, accountID, objectID string) error {
	_, err := repository.pool.Exec(ctx, `
		update storage.objects
		set status = 'failed', failed_at = now()
		where id = $1 and account_id = $2::uuid and status = 'pending'
	`, objectID, accountID)
	return err
}

func (repository *PostgresRepository) Object(ctx context.Context, accountID, objectID string) (Object, error) {
	var object Object
	err := repository.pool.QueryRow(ctx, `
		select id, account_id::text, source_module, idempotency_key, object_key,
			file_name, content_type, size_bytes, etag, status, created_by::text,
			created_at, available_at
		from storage.objects
		where id = $1 and account_id = $2::uuid and status in ('pending','available')
	`, objectID, accountID).Scan(objectScanTargets(&object)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Object{}, ErrObjectNotFound
	}
	return object, err
}

func (repository *PostgresRepository) PendingObjects(ctx context.Context, limit int) ([]Object, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := repository.pool.Query(ctx, `
		select id, account_id::text, source_module, idempotency_key, object_key,
			file_name, content_type, size_bytes, etag, status, created_by::text,
			created_at, available_at
		from storage.objects
		where status = 'pending'
		order by created_at, id
		limit $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	objects := make([]Object, 0)
	for rows.Next() {
		var object Object
		if err := rows.Scan(objectScanTargets(&object)...); err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	return objects, rows.Err()
}

func (repository *PostgresRepository) Usage(ctx context.Context, billingMonth time.Time) (Usage, error) {
	var usage Usage
	err := repository.pool.QueryRow(ctx, `
		select
			to_char($1::date, 'YYYY-MM'),
			coalesce(sum(size_bytes) filter (where status = 'available'), 0),
			coalesce(sum(size_bytes) filter (where status = 'pending'), 0),
			count(*) filter (where status = 'available'),
			count(*) filter (where status = 'pending'),
			coalesce((select class_a_requests from storage.monthly_usage where billing_month = $1::date), 0),
			coalesce((select class_b_requests from storage.monthly_usage where billing_month = $1::date), 0),
			coalesce((select uploaded_bytes from storage.monthly_usage where billing_month = $1::date), 0)
		from storage.objects
	`, billingMonth).Scan(
		&usage.BillingMonth,
		&usage.StoredBytes,
		&usage.PendingBytes,
		&usage.AvailableObjects,
		&usage.PendingObjects,
		&usage.ClassARequests,
		&usage.ClassBRequests,
		&usage.UploadedBytes,
	)
	return usage, err
}

func findByIdempotency(ctx context.Context, tx pgx.Tx, accountID, sourceModule, key string) (Object, error) {
	var object Object
	err := tx.QueryRow(ctx, `
		select id, account_id::text, source_module, idempotency_key, object_key,
			file_name, content_type, size_bytes, etag, status, created_by::text,
			created_at, available_at
		from storage.objects
		where account_id = $1::uuid and source_module = $2 and idempotency_key = $3
	`, accountID, sourceModule, key).Scan(objectScanTargets(&object)...)
	return object, err
}

func objectScanTargets(object *Object) []any {
	return []any{
		&object.ID,
		&object.AccountID,
		&object.SourceModule,
		&object.IdempotencyKey,
		&object.ObjectKey,
		&object.FileName,
		&object.ContentType,
		&object.SizeBytes,
		&object.ETag,
		&object.Status,
		&object.CreatedBy,
		&object.CreatedAt,
		&object.AvailableAt,
	}
}

func settingsScanTargets(settings *Settings) []any {
	return []any{
		&settings.UploadsEnabled,
		&settings.BillingCycleDay,
		&settings.StorageLimitBytes,
		&settings.ClassALimit,
		&settings.ClassBLimit,
		&settings.MaxObjectBytes,
		&settings.ImageMaxBytes,
		&settings.VideoMaxBytes,
		&settings.AudioMaxBytes,
		&settings.DocumentMaxBytes,
		&settings.UpdatedBy,
		&settings.UpdatedAt,
	}
}
