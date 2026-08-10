package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type MultipartRepository interface {
	ReserveMultipart(ctx context.Context, upload MultipartUpload, billingMonth time.Time, plannedClassA int64) (MultipartUpload, bool, error)
	ActivateMultipart(ctx context.Context, uploadID, providerUploadID string) error
	BeginMultipartProviderAttempt(ctx context.Context, uploadID string, billingMonth time.Time) error
	Multipart(ctx context.Context, accountID, sourceModule, uploadID string) (MultipartUpload, error)
	BeginPartAttempt(ctx context.Context, uploadID string, partNumber int, billingMonth time.Time) (*MultipartPart, error)
	SaveMultipartPart(ctx context.Context, uploadID string, part MultipartPart) error
	BeginMultipartCompletion(ctx context.Context, uploadID string, billingMonth time.Time) ([]MultipartPart, error)
	CompleteMultipart(ctx context.Context, uploadID, etag string) (Object, error)
	FailMultipart(ctx context.Context, uploadID string) error
	EnqueueMultipartDelivery(ctx context.Context, delivery MultipartDelivery) error
	ClaimMultipartDelivery(ctx context.Context) (MultipartDelivery, error)
	RetryMultipartDelivery(ctx context.Context, uploadID, message string, nextAttempt time.Time) error
	CompleteMultipartDelivery(ctx context.Context, uploadID string) error
	MultipartDeliveryPath(ctx context.Context, accountID, sourceModule, objectID string) (string, error)
}

func (repository *PostgresRepository) ReserveMultipart(ctx context.Context, upload MultipartUpload, month time.Time, plannedClassA int64) (MultipartUpload, bool, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return MultipartUpload{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('storage.r2.global-budget'))`); err != nil {
		return MultipartUpload{}, false, err
	}

	var existingObject Object
	err = tx.QueryRow(ctx, `select id, account_id::text, source_module, idempotency_key, object_key, file_name, content_type,
		size_bytes, etag, status, created_by::text, created_at, available_at from storage.objects
		where account_id=$1::uuid and source_module=$2 and idempotency_key=$3`, upload.Object.AccountID, upload.Object.SourceModule, upload.Object.IdempotencyKey).
		Scan(objectScanTargets(&existingObject)...)
	if err == nil {
		existing, loadErr := multipartByObject(ctx, tx, existingObject)
		if loadErr != nil {
			return MultipartUpload{}, false, loadErr
		}
		return existing, true, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return MultipartUpload{}, false, err
	}

	var settings Settings
	if err = tx.QueryRow(ctx, `select uploads_enabled, storage_limit_bytes, class_a_limit, class_b_limit, max_object_bytes,
		image_max_bytes, video_max_bytes, audio_max_bytes, document_max_bytes, coalesce(updated_by::text,''), updated_at
		from storage.settings where id=1`).Scan(settingsScanTargets(&settings)...); err != nil {
		return MultipartUpload{}, false, err
	}
	if !settings.UploadsEnabled {
		return MultipartUpload{}, false, ErrUploadsDisabled
	}
	var reservedBytes int64
	if err = tx.QueryRow(ctx, `select coalesce(sum(size_bytes),0) from storage.objects where status in ('pending','available')`).Scan(&reservedBytes); err != nil {
		return MultipartUpload{}, false, err
	}
	if upload.Object.SizeBytes > settings.StorageLimitBytes-reservedBytes {
		return MultipartUpload{}, false, ErrStorageQuotaExceeded
	}
	if _, err = tx.Exec(ctx, `insert into storage.monthly_usage (billing_month) values ($1) on conflict do nothing`, month); err != nil {
		return MultipartUpload{}, false, err
	}
	command, err := tx.Exec(ctx, `update storage.monthly_usage set class_a_requests=class_a_requests+$2, updated_at=now()
		where billing_month=$1 and class_a_requests+$2 <= $3`, month, plannedClassA, settings.ClassALimit)
	if err != nil {
		return MultipartUpload{}, false, err
	}
	if command.RowsAffected() != 1 {
		return MultipartUpload{}, false, ErrClassAQuotaExceeded
	}
	if err = tx.QueryRow(ctx, `insert into storage.objects (id,account_id,source_module,idempotency_key,object_key,file_name,content_type,size_bytes,created_by)
		values ($1,$2::uuid,$3,$4,$5,$6,$7,$8,$9::uuid)
		returning id,account_id::text,source_module,idempotency_key,object_key,file_name,content_type,size_bytes,etag,status,created_by::text,created_at,available_at`,
		upload.Object.ID, upload.Object.AccountID, upload.Object.SourceModule, upload.Object.IdempotencyKey, upload.Object.ObjectKey,
		upload.Object.FileName, upload.Object.ContentType, upload.Object.SizeBytes, upload.Object.CreatedBy).Scan(objectScanTargets(&upload.Object)...); err != nil {
		return MultipartUpload{}, false, err
	}
	_, err = tx.Exec(ctx, `insert into storage.multipart_uploads (id,object_id,account_id,source_module,part_size_bytes,part_count,created_by)
		values ($1,$2,$3::uuid,$4,$5,$6,$7::uuid)`, upload.ID, upload.Object.ID, upload.Object.AccountID, upload.Object.SourceModule,
		upload.PartSizeBytes, upload.PartCount, upload.CreatedBy)
	if err != nil {
		return MultipartUpload{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MultipartUpload{}, false, err
	}
	return upload, false, nil
}

func multipartByObject(ctx context.Context, tx pgx.Tx, object Object) (MultipartUpload, error) {
	var upload MultipartUpload
	upload.Object = object
	err := tx.QueryRow(ctx, `select id,provider_upload_id,part_size_bytes,part_count,status,created_by::text,created_at
		from storage.multipart_uploads where object_id=$1`, object.ID).
		Scan(&upload.ID, &upload.ProviderUploadID, &upload.PartSizeBytes, &upload.PartCount, &upload.Status, &upload.CreatedBy, &upload.CreatedAt)
	if err != nil {
		return MultipartUpload{}, err
	}
	parts, err := multipartParts(ctx, tx, upload.ID)
	upload.UploadedParts = parts
	return upload, err
}

func multipartParts(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, uploadID string) ([]MultipartPart, error) {
	rows, err := q.Query(ctx, `select part_number,etag,size_bytes from storage.multipart_parts where upload_id=$1 and etag<>'' order by part_number`, uploadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	parts := make([]MultipartPart, 0)
	for rows.Next() {
		var p MultipartPart
		if err := rows.Scan(&p.PartNumber, &p.ETag, &p.SizeBytes); err != nil {
			return nil, err
		}
		parts = append(parts, p)
	}
	return parts, rows.Err()
}

func (repository *PostgresRepository) ActivateMultipart(ctx context.Context, uploadID, providerID string) error {
	command, err := repository.pool.Exec(ctx, `update storage.multipart_uploads set provider_upload_id=$2,status='uploading',updated_at=now() where id=$1 and status='creating'`, uploadID, providerID)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrInvalidUpload
	}
	return nil
}

func (repository *PostgresRepository) BeginMultipartProviderAttempt(ctx context.Context, uploadID string, month time.Time) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('storage.r2.global-budget'))`); err != nil {
		return err
	}
	var attempts int
	if err = tx.QueryRow(ctx, `select creation_attempts from storage.multipart_uploads where id=$1 and status='creating' for update`, uploadID).Scan(&attempts); err != nil {
		return ErrInvalidUpload
	}
	if attempts > 0 {
		var limit int64
		if err = tx.QueryRow(ctx, `select class_a_limit from storage.settings where id=1`).Scan(&limit); err != nil {
			return err
		}
		command, e := tx.Exec(ctx, `update storage.monthly_usage set class_a_requests=class_a_requests+1,updated_at=now() where billing_month=$1 and class_a_requests<$2`, month, limit)
		if e != nil {
			return e
		}
		if command.RowsAffected() != 1 {
			return ErrClassAQuotaExceeded
		}
	}
	if _, err = tx.Exec(ctx, `update storage.multipart_uploads set creation_attempts=creation_attempts+1,updated_at=now() where id=$1`, uploadID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repository *PostgresRepository) Multipart(ctx context.Context, accountID, source, uploadID string) (MultipartUpload, error) {
	var object Object
	err := repository.pool.QueryRow(ctx, `select o.id,o.account_id::text,o.source_module,o.idempotency_key,o.object_key,o.file_name,o.content_type,o.size_bytes,o.etag,o.status,o.created_by::text,o.created_at,o.available_at
		from storage.objects o join storage.multipart_uploads m on m.object_id=o.id where m.id=$1 and m.account_id=$2::uuid and m.source_module=$3`, uploadID, accountID, source).Scan(objectScanTargets(&object)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return MultipartUpload{}, ErrObjectNotFound
	}
	if err != nil {
		return MultipartUpload{}, err
	}
	var upload MultipartUpload
	upload.Object = object
	err = repository.pool.QueryRow(ctx, `select id,provider_upload_id,part_size_bytes,part_count,status,created_by::text,created_at from storage.multipart_uploads where id=$1`, uploadID).Scan(&upload.ID, &upload.ProviderUploadID, &upload.PartSizeBytes, &upload.PartCount, &upload.Status, &upload.CreatedBy, &upload.CreatedAt)
	if err != nil {
		return MultipartUpload{}, err
	}
	upload.UploadedParts, err = multipartParts(ctx, repository.pool, uploadID)
	return upload, err
}

func (repository *PostgresRepository) BeginPartAttempt(ctx context.Context, uploadID string, number int, month time.Time) (*MultipartPart, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('storage.r2.global-budget'))`); err != nil {
		return nil, err
	}
	var part MultipartPart
	var attempts int
	err = tx.QueryRow(ctx, `select part_number,etag,size_bytes,attempts from storage.multipart_parts where upload_id=$1 and part_number=$2 for update`, uploadID, number).Scan(&part.PartNumber, &part.ETag, &part.SizeBytes, &attempts)
	if err == nil && part.ETag != "" {
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &part, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) && err != nil {
		return nil, err
	}
	if attempts > 0 {
		var limit int64
		if err = tx.QueryRow(ctx, `select class_a_limit from storage.settings where id=1`).Scan(&limit); err != nil {
			return nil, err
		}
		command, e := tx.Exec(ctx, `update storage.monthly_usage set class_a_requests=class_a_requests+1,updated_at=now() where billing_month=$1 and class_a_requests<$2`, month, limit)
		if e != nil {
			return nil, e
		}
		if command.RowsAffected() != 1 {
			return nil, ErrClassAQuotaExceeded
		}
	}
	_, err = tx.Exec(ctx, `insert into storage.multipart_parts(upload_id,part_number,attempts) values($1,$2,1)
		on conflict(upload_id,part_number) do update set attempts=storage.multipart_parts.attempts+1`, uploadID, number)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return nil, nil
}

func (repository *PostgresRepository) SaveMultipartPart(ctx context.Context, uploadID string, part MultipartPart) error {
	_, err := repository.pool.Exec(ctx, `update storage.multipart_parts set etag=$3,size_bytes=$4,uploaded_at=now() where upload_id=$1 and part_number=$2`, uploadID, part.PartNumber, part.ETag, part.SizeBytes)
	return err
}
func (repository *PostgresRepository) BeginMultipartCompletion(ctx context.Context, uploadID string, month time.Time) ([]MultipartPart, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('storage.r2.global-budget'))`); err != nil {
		return nil, err
	}
	var attempts int
	if err = tx.QueryRow(ctx, `select completion_attempts from storage.multipart_uploads where id=$1 and status in ('uploading','completing') for update`, uploadID).Scan(&attempts); err != nil {
		return nil, ErrInvalidUpload
	}
	if attempts > 0 {
		var limit int64
		if err = tx.QueryRow(ctx, `select class_a_limit from storage.settings where id=1`).Scan(&limit); err != nil {
			return nil, err
		}
		command, e := tx.Exec(ctx, `update storage.monthly_usage set class_a_requests=class_a_requests+1,updated_at=now() where billing_month=$1 and class_a_requests<$2`, month, limit)
		if e != nil {
			return nil, e
		}
		if command.RowsAffected() != 1 {
			return nil, ErrClassAQuotaExceeded
		}
	}
	if _, err = tx.Exec(ctx, `update storage.multipart_uploads set status='completing',completion_attempts=completion_attempts+1,updated_at=now() where id=$1`, uploadID); err != nil {
		return nil, err
	}
	parts, err := multipartParts(ctx, tx, uploadID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return parts, nil
}
func (repository *PostgresRepository) CompleteMultipart(ctx context.Context, uploadID, etag string) (Object, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Object{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var object Object
	err = tx.QueryRow(ctx, `update storage.objects o set status='available',etag=$2,available_at=now() from storage.multipart_uploads m where m.id=$1 and m.object_id=o.id and o.status='pending'
		returning o.id,o.account_id::text,o.source_module,o.idempotency_key,o.object_key,o.file_name,o.content_type,o.size_bytes,o.etag,o.status,o.created_by::text,o.created_at,o.available_at`, uploadID, etag).Scan(objectScanTargets(&object)...)
	if err != nil {
		return Object{}, err
	}
	if _, err = tx.Exec(ctx, `update storage.multipart_uploads set status='completed',updated_at=now() where id=$1`, uploadID); err != nil {
		return Object{}, err
	}
	if _, err = tx.Exec(ctx, `update storage.monthly_usage set uploaded_bytes=uploaded_bytes+$2,updated_at=now() where billing_month=date_trunc('month',$1::timestamptz)::date`, object.CreatedAt, object.SizeBytes); err != nil {
		return Object{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Object{}, err
	}
	return object, nil
}
func (repository *PostgresRepository) FailMultipart(ctx context.Context, uploadID string) error {
	_, err := repository.pool.Exec(ctx, `update storage.multipart_uploads set status='failed',updated_at=now() where id=$1`, uploadID)
	return err
}

func (repository *PostgresRepository) EnqueueMultipartDelivery(ctx context.Context, delivery MultipartDelivery) error {
	_, err := repository.pool.Exec(ctx, `insert into storage.multipart_deliveries(upload_id,account_id,source_module,created_by,staging_path)
		values($1,$2::uuid,$3,$4::uuid,$5) on conflict(upload_id) do nothing`, delivery.UploadID, delivery.AccountID, delivery.SourceModule, delivery.CreatedBy, delivery.StagingPath)
	return err
}

func (repository *PostgresRepository) ClaimMultipartDelivery(ctx context.Context) (MultipartDelivery, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return MultipartDelivery{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var delivery MultipartDelivery
	err = tx.QueryRow(ctx, `select upload_id,account_id::text,source_module,created_by::text,staging_path,attempts
		from storage.multipart_deliveries where (status in ('queued','retry') and next_attempt_at<=now())
		or (status='uploading' and locked_at<now()-interval '20 minutes') order by next_attempt_at,created_at for update skip locked limit 1`).Scan(&delivery.UploadID, &delivery.AccountID, &delivery.SourceModule, &delivery.CreatedBy, &delivery.StagingPath, &delivery.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return MultipartDelivery{}, ErrObjectNotFound
	}
	if err != nil {
		return MultipartDelivery{}, err
	}
	if _, err = tx.Exec(ctx, `update storage.multipart_deliveries set status='uploading',attempts=attempts+1,locked_at=now(),updated_at=now() where upload_id=$1`, delivery.UploadID); err != nil {
		return MultipartDelivery{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MultipartDelivery{}, err
	}
	delivery.Attempts++
	return delivery, nil
}

func (repository *PostgresRepository) RetryMultipartDelivery(ctx context.Context, uploadID, message string, next time.Time) error {
	_, err := repository.pool.Exec(ctx, `update storage.multipart_deliveries set status='retry',next_attempt_at=$2,locked_at=null,last_error=$3,updated_at=now() where upload_id=$1`, uploadID, next, message)
	return err
}
func (repository *PostgresRepository) CompleteMultipartDelivery(ctx context.Context, uploadID string) error {
	_, err := repository.pool.Exec(ctx, `update storage.multipart_deliveries set status='completed',locked_at=null,last_error='',updated_at=now() where upload_id=$1`, uploadID)
	return err
}
func (repository *PostgresRepository) MultipartDeliveryPath(ctx context.Context, accountID, source, objectID string) (string, error) {
	var path string
	err := repository.pool.QueryRow(ctx, `select d.staging_path from storage.multipart_deliveries d join storage.multipart_uploads m on m.id=d.upload_id join storage.objects o on o.id=m.object_id
		where o.id=$1 and o.account_id=$2::uuid and o.source_module=$3 and d.status in ('queued','uploading','retry')`, objectID, accountID, source).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrObjectNotFound
	}
	return path, err
}
