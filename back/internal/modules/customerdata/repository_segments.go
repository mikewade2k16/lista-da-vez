package customerdata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func segmentColumns(alias string) string {
	if alias != "" {
		alias += "."
	}
	return strings.Join([]string{
		alias + "id::text", alias + "client_account_id::text", alias + "segment_key",
		alias + "name", alias + "description", alias + "status",
		alias + "active_version_id::text", alias + "current_materialization_id::text",
		alias + "revision", alias + "archived_at", alias + "created_at", alias + "updated_at",
	}, ", ")
}

func scanSegment(row rowScanner) (Segment, error) {
	var item Segment
	err := row.Scan(
		&item.ID, &item.ClientAccountID, &item.SegmentKey, &item.Name,
		&item.Description, &item.Status, &item.ActiveVersionID,
		&item.CurrentMaterializationID, &item.Revision, &item.ArchivedAt,
		&item.CreatedAt, &item.UpdatedAt,
	)
	return item, mapDBError(err)
}

func versionColumns(alias string) string {
	if alias != "" {
		alias += "."
	}
	return strings.Join([]string{
		alias + "id::text", alias + "segment_id::text", alias + "version_number",
		alias + "status", alias + "filter_schema_version", alias + "field_catalog_version",
		alias + "filter_ast", alias + "evaluation_policy", alias + "definition_hash",
		alias + "validation_hash", alias + "validation_reason_codes", alias + "revision",
		alias + "created_at", alias + "updated_at", alias + "validated_at", alias + "published_at",
	}, ", ")
}

func scanSegmentVersion(row rowScanner) (SegmentVersion, error) {
	var item SegmentVersion
	var filter, policy, reasons []byte
	err := row.Scan(
		&item.ID, &item.SegmentID, &item.VersionNumber, &item.Status,
		&item.FilterSchemaVersion, &item.FieldCatalogVersion, &filter, &policy,
		&item.DefinitionHash, &item.ValidationHash, &reasons, &item.Revision,
		&item.CreatedAt, &item.UpdatedAt, &item.ValidatedAt, &item.PublishedAt,
	)
	if err != nil {
		return SegmentVersion{}, mapDBError(err)
	}
	item.FilterAST = filter
	item.EvaluationPolicy = policy
	_ = json.Unmarshal(reasons, &item.ValidationReasonCodes)
	return item, nil
}

func (r *PostgresRepository) ListSegments(
	ctx context.Context,
	scope Scope,
	status, cursorValue string,
	limit int,
) ([]Segment, string, error) {
	cursor, err := decodeStableCursor(cursorValue)
	if err != nil {
		return nil, "", err
	}
	args := []any{scope.AccountID, scope.ClientAccountID}
	var query strings.Builder
	query.WriteString(`
		select ` + segmentColumns("") + `
		from customer_data.segments
		where account_id = $1::uuid and client_account_id = $2::uuid
	`)
	if status != "" {
		appendClause(&query, &args, " and status = $%d", status)
	}
	if !cursor.At.IsZero() {
		args = append(args, cursor.At, cursor.ID)
		_, _ = fmt.Fprintf(
			&query,
			" and (updated_at, id) < ($%d, $%d::uuid)",
			len(args)-1,
			len(args),
		)
	}
	args = append(args, limit+1)
	_, _ = fmt.Fprintf(&query, " order by updated_at desc, id desc limit $%d", len(args))
	rows, err := r.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := make([]Segment, 0, limit)
	var next string
	for rows.Next() {
		item, err := scanSegment(rows)
		if err != nil {
			return nil, "", err
		}
		if len(items) == limit {
			next = encodeStableCursor(items[len(items)-1].UpdatedAt, items[len(items)-1].ID)
			break
		}
		items = append(items, item)
	}
	return items, next, rows.Err()
}

func (r *PostgresRepository) CreateSegment(
	ctx context.Context,
	scope Scope,
	input CreateSegmentInput,
	definitionHash string,
) (CreateSegmentResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CreateSegmentResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	segment, err := scanSegment(tx.QueryRow(ctx, `
		select `+segmentColumns("")+`
		from customer_data.segments
		where account_id = $1::uuid and client_account_id = $2::uuid and idempotency_key = $3
	`, scope.AccountID, scope.ClientAccountID, input.IdempotencyKey))
	if err == nil {
		version, versionErr := scanSegmentVersion(tx.QueryRow(ctx, `
			select `+versionColumns("")+`
			from customer_data.segment_versions
			where account_id = $1::uuid and client_account_id = $2::uuid and segment_id = $3::uuid
			order by version_number limit 1
		`, scope.AccountID, scope.ClientAccountID, segment.ID))
		return CreateSegmentResult{Segment: segment, Version: version, Replayed: true}, versionErr
	}
	if err != ErrNotFound {
		return CreateSegmentResult{}, err
	}
	segment, err = scanSegment(tx.QueryRow(ctx, `
		insert into customer_data.segments (
			account_id, client_account_id, segment_key, name, description,
			idempotency_key, owner_user_id, created_by_user_id, updated_by_user_id
		) values (
			$1::uuid, $2::uuid, $3, $4, $5, $6,
			nullif($7, '')::uuid, nullif($7, '')::uuid, nullif($7, '')::uuid
		)
		returning `+segmentColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, input.SegmentKey, strings.TrimSpace(input.Name),
		input.Description, input.IdempotencyKey, scope.ActorUserID))
	if err != nil {
		return CreateSegmentResult{}, err
	}
	version, err := scanSegmentVersion(tx.QueryRow(ctx, `
		insert into customer_data.segment_versions (
			account_id, client_account_id, segment_id, version_number, status,
			filter_schema_version, field_catalog_version, filter_ast,
			evaluation_policy, definition_hash, idempotency_key, created_by_user_id
		) values (
			$1::uuid, $2::uuid, $3::uuid, 1, 'draft',
			$4, $5, $6::jsonb, $7::jsonb, $8, $9, nullif($10, '')::uuid
		)
		returning `+versionColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, segment.ID,
		input.Draft.FilterSchemaVersion, input.Draft.FieldCatalogVersion,
		input.Draft.FilterAST, nullableJSON(input.Draft.EvaluationPolicy, "{}"),
		definitionHash, input.IdempotencyKey+":version:1", scope.ActorUserID))
	if err != nil {
		return CreateSegmentResult{}, err
	}
	if err := insertAudit(ctx, tx, scope, "", "", "create", "segment", segment.ID, "segment_draft_created"); err != nil {
		return CreateSegmentResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CreateSegmentResult{}, err
	}
	return CreateSegmentResult{Segment: segment, Version: version}, nil
}

func (r *PostgresRepository) GetSegment(
	ctx context.Context,
	scope Scope,
	segmentID string,
) (Segment, error) {
	return scanSegment(r.pool.QueryRow(ctx, `
		select `+segmentColumns("")+`
		from customer_data.segments
		where account_id = $1::uuid and client_account_id = $2::uuid and id = $3::uuid
	`, scope.AccountID, scope.ClientAccountID, segmentID))
}

func (r *PostgresRepository) UpdateSegment(
	ctx context.Context,
	scope Scope,
	segmentID string,
	patch SegmentPatch,
) (Segment, error) {
	item, err := scanSegment(r.pool.QueryRow(ctx, `
		update customer_data.segments
		set name = case when $4::boolean then $5 else name end,
		    description = case when $6::boolean then $7 else description end,
		    revision = revision + 1,
		    updated_by_user_id = nullif($8, '')::uuid,
		    updated_at = now()
		where account_id = $1::uuid and client_account_id = $2::uuid
		  and id = $3::uuid and revision = $9 and status = 'active'
		returning `+segmentColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, segmentID,
		patch.Name != nil, pointerString(patch.Name),
		patch.Description != nil, pointerString(patch.Description),
		scope.ActorUserID, patch.ExpectedRevision))
	if err != nil {
		return Segment{}, mapMutationError(err)
	}
	return item, nil
}

func (r *PostgresRepository) ArchiveSegment(
	ctx context.Context,
	scope Scope,
	segmentID string,
	expectedRevision int64,
) (Segment, error) {
	item, err := scanSegment(r.pool.QueryRow(ctx, `
		update customer_data.segments
		set status = 'archived', archived_at = now(), revision = revision + 1,
		    updated_by_user_id = nullif($5, '')::uuid, updated_at = now()
		where account_id = $1::uuid and client_account_id = $2::uuid
		  and id = $3::uuid and revision = $4 and status = 'active'
		returning `+segmentColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, segmentID, expectedRevision, scope.ActorUserID))
	if err != nil {
		return Segment{}, mapMutationError(err)
	}
	return item, nil
}

func (r *PostgresRepository) ListSegmentVersions(
	ctx context.Context,
	scope Scope,
	segmentID string,
) ([]SegmentVersion, error) {
	rows, err := r.pool.Query(ctx, `
		select `+versionColumns("")+`
		from customer_data.segment_versions
		where account_id = $1::uuid and client_account_id = $2::uuid and segment_id = $3::uuid
		order by version_number desc, id
	`, scope.AccountID, scope.ClientAccountID, segmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SegmentVersion, 0)
	for rows.Next() {
		item, err := scanSegmentVersion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateSegmentVersion(
	ctx context.Context,
	scope Scope,
	segmentID string,
	input CreateSegmentVersionInput,
	draft SegmentDraftInput,
	definitionHash string,
) (SegmentVersion, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return SegmentVersion{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	existing, err := scanSegmentVersion(tx.QueryRow(ctx, `
		select `+versionColumns("")+`
		from customer_data.segment_versions
		where account_id = $1::uuid and client_account_id = $2::uuid and idempotency_key = $3
	`, scope.AccountID, scope.ClientAccountID, input.IdempotencyKey))
	if err == nil {
		return existing, true, nil
	}
	if err != ErrNotFound {
		return SegmentVersion{}, false, err
	}
	var segmentExists bool
	err = tx.QueryRow(ctx, `
		select true
		from customer_data.segments
		where account_id = $1::uuid and client_account_id = $2::uuid
		  and id = $3::uuid and status = 'active'
		for update
	`, scope.AccountID, scope.ClientAccountID, segmentID).Scan(&segmentExists)
	if err != nil || !segmentExists {
		return SegmentVersion{}, false, mapDBError(err)
	}
	var next int
	err = tx.QueryRow(ctx, `
		select coalesce(max(version_number), 0) + 1
		from customer_data.segment_versions
		where account_id = $1::uuid and client_account_id = $2::uuid and segment_id = $3::uuid
	`, scope.AccountID, scope.ClientAccountID, segmentID).Scan(&next)
	if err != nil {
		return SegmentVersion{}, false, err
	}
	item, err := scanSegmentVersion(tx.QueryRow(ctx, `
		insert into customer_data.segment_versions (
			account_id, client_account_id, segment_id, version_number, status,
			filter_schema_version, field_catalog_version, filter_ast,
			evaluation_policy, definition_hash, idempotency_key, change_summary,
			created_by_user_id
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4, 'draft',
			$5, $6, $7::jsonb, $8::jsonb, $9, $10, nullif($11, ''),
			nullif($12, '')::uuid
		)
		returning `+versionColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, segmentID, next,
		draft.FilterSchemaVersion, draft.FieldCatalogVersion, draft.FilterAST,
		nullableJSON(draft.EvaluationPolicy, "{}"), definitionHash,
		input.IdempotencyKey, input.ChangeSummary, scope.ActorUserID))
	if err != nil {
		return SegmentVersion{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SegmentVersion{}, false, err
	}
	return item, false, nil
}

func (r *PostgresRepository) GetSegmentVersion(
	ctx context.Context,
	scope Scope,
	versionID string,
) (SegmentVersion, error) {
	return scanSegmentVersion(r.pool.QueryRow(ctx, `
		select `+versionColumns("")+`
		from customer_data.segment_versions
		where account_id = $1::uuid and client_account_id = $2::uuid and id = $3::uuid
	`, scope.AccountID, scope.ClientAccountID, versionID))
}

func (r *PostgresRepository) UpdateSegmentVersion(
	ctx context.Context,
	scope Scope,
	versionID string,
	patch SegmentVersionPatch,
	definitionHash string,
) (SegmentVersion, error) {
	item, err := scanSegmentVersion(r.pool.QueryRow(ctx, `
		update customer_data.segment_versions
		set filter_ast = $4::jsonb,
		    evaluation_policy = $5::jsonb,
		    definition_hash = $6,
		    change_summary = nullif($7, ''),
		    status = 'draft',
		    validation_hash = null,
		    validation_reason_codes = '[]'::jsonb,
		    validated_by_user_id = null,
		    validated_at = null,
		    revision = revision + 1,
		    updated_at = now()
		where account_id = $1::uuid and client_account_id = $2::uuid
		  and id = $3::uuid and revision = $8
		  and status in ('draft', 'validated')
		returning `+versionColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, versionID,
		patch.FilterAST, nullableJSON(patch.EvaluationPolicy, "{}"), definitionHash,
		patch.ChangeSummary, patch.ExpectedRevision))
	if err != nil {
		return SegmentVersion{}, mapMutationError(err)
	}
	return item, nil
}

func (r *PostgresRepository) ValidateSegmentVersion(
	ctx context.Context,
	scope Scope,
	versionID, validationHash string,
	cost int,
) (SegmentVersion, error) {
	reasons, _ := json.Marshal([]string{})
	item, err := scanSegmentVersion(r.pool.QueryRow(ctx, `
		update customer_data.segment_versions
		set status = 'validated',
		    validation_hash = $4,
		    validation_reason_codes = $5::jsonb,
		    validated_by_user_id = nullif($6, '')::uuid,
		    validated_at = now(),
		    revision = revision + 1,
		    updated_at = now()
		where account_id = $1::uuid and client_account_id = $2::uuid
		  and id = $3::uuid and status in ('draft', 'validated')
		returning `+versionColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, versionID, validationHash, reasons, scope.ActorUserID))
	if err != nil {
		return SegmentVersion{}, mapMutationError(err)
	}
	_ = cost
	return item, nil
}

func (r *PostgresRepository) PublishSegmentVersion(
	ctx context.Context,
	scope Scope,
	versionID string,
	input PublishSegmentVersionInput,
) (SegmentVersion, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return SegmentVersion{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var replay bool
	err = tx.QueryRow(ctx, `
		select exists (
			select 1 from customer_data.outbox_events
			where account_id = $1::uuid
			  and client_account_id = $2::uuid
			  and idempotency_key = $3
			)
	`, scope.AccountID, scope.ClientAccountID, "segment.publish:"+input.IdempotencyKey).Scan(&replay)
	if err != nil {
		return SegmentVersion{}, false, err
	}
	if replay {
		item, err := scanSegmentVersion(tx.QueryRow(ctx, `
			select `+versionColumns("")+` from customer_data.segment_versions
			where account_id = $1::uuid and client_account_id = $2::uuid and id = $3::uuid
		`, scope.AccountID, scope.ClientAccountID, versionID))
		return item, true, err
	}
	item, err := scanSegmentVersion(tx.QueryRow(ctx, `
		update customer_data.segment_versions
		set status = 'published',
		    published_by_user_id = nullif($7, '')::uuid,
		    published_at = now(),
		    revision = revision + 1,
		    updated_at = now()
		where account_id = $1::uuid and client_account_id = $2::uuid
		  and id = $3::uuid and status = 'validated'
		  and revision = $4 and validation_hash = $5
		  and definition_hash = $6
		returning `+versionColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, versionID, input.ExpectedRevision,
		input.ValidationHash, input.ValidationHash, scope.ActorUserID))
	if err != nil {
		return SegmentVersion{}, false, mapMutationError(err)
	}
	tag, err := tx.Exec(ctx, `
		update customer_data.segments
		set active_version_id = $4::uuid, revision = revision + 1,
		    updated_by_user_id = nullif($5, '')::uuid, updated_at = now()
		where account_id = $1::uuid and client_account_id = $2::uuid and id = $3::uuid
		  and status = 'active'
	`, scope.AccountID, scope.ClientAccountID, item.SegmentID, versionID, scope.ActorUserID)
	if err != nil {
		return SegmentVersion{}, false, err
	}
	if err := requireRowsAffected(tag); err != nil {
		return SegmentVersion{}, false, err
	}
	if err := insertOutbox(ctx, tx, scope, "segment", item.SegmentID, "customer_data.segment.version_published", "segment.publish:"+input.IdempotencyKey); err != nil {
		return SegmentVersion{}, false, err
	}
	if err := insertAudit(ctx, tx, scope, "", "", "publish", "segment_version", item.ID, input.Reason); err != nil {
		return SegmentVersion{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SegmentVersion{}, false, err
	}
	return item, false, nil
}

func (r *PostgresRepository) RollbackSegment(
	ctx context.Context,
	scope Scope,
	segmentID string,
	input RollbackSegmentInput,
) (Segment, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Segment{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var replay bool
	err = tx.QueryRow(ctx, `
		select exists (
			select 1 from customer_data.outbox_events
			where account_id = $1::uuid
			  and client_account_id = $2::uuid
			  and idempotency_key = $3
			)
	`, scope.AccountID, scope.ClientAccountID, "segment.rollback:"+input.IdempotencyKey).Scan(&replay)
	if err != nil {
		return Segment{}, false, err
	}
	if replay {
		item, err := scanSegment(tx.QueryRow(ctx, `
			select `+segmentColumns("")+` from customer_data.segments
			where account_id = $1::uuid and client_account_id = $2::uuid and id = $3::uuid
		`, scope.AccountID, scope.ClientAccountID, segmentID))
		return item, true, err
	}
	var published bool
	err = tx.QueryRow(ctx, `
		select exists (
			select 1 from customer_data.segment_versions
			where account_id = $1::uuid and client_account_id = $2::uuid
			  and segment_id = $3::uuid and id = $4::uuid and status = 'published'
		)
	`, scope.AccountID, scope.ClientAccountID, segmentID, input.TargetVersionID).Scan(&published)
	if err != nil {
		return Segment{}, false, err
	}
	if !published {
		return Segment{}, false, ErrNotFound
	}
	item, err := scanSegment(tx.QueryRow(ctx, `
		update customer_data.segments
		set active_version_id = $4::uuid, current_materialization_id = null,
		    revision = revision + 1, updated_by_user_id = nullif($5, '')::uuid,
		    updated_at = now()
		where account_id = $1::uuid and client_account_id = $2::uuid
		  and id = $3::uuid and revision = $6 and status = 'active'
		returning `+segmentColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, segmentID, input.TargetVersionID,
		scope.ActorUserID, input.ExpectedSegmentRevision))
	if err != nil {
		return Segment{}, false, mapMutationError(err)
	}
	if err := insertOutbox(ctx, tx, scope, "segment", segmentID, "customer_data.segment.version_published", "segment.rollback:"+input.IdempotencyKey); err != nil {
		return Segment{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Segment{}, false, err
	}
	return item, false, nil
}

func runColumns(alias string) string {
	if alias != "" {
		alias += "."
	}
	return strings.Join([]string{
		alias + "id::text", alias + "segment_id::text", alias + "version_id::text",
		alias + "mode", alias + "status", alias + "as_of", alias + "definition_hash",
		alias + "field_catalog_version", alias + "matched_count", alias + "excluded_count",
		alias + "error_count", alias + "reason_codes", alias + "requested_at",
		alias + "started_at", alias + "finished_at",
	}, ", ")
}

func scanRun(row rowScanner) (SegmentEvaluationRun, error) {
	var item SegmentEvaluationRun
	var reasons []byte
	err := row.Scan(
		&item.ID, &item.SegmentID, &item.VersionID, &item.Mode, &item.Status,
		&item.AsOf, &item.DefinitionHash, &item.FieldCatalogVersion,
		&item.MatchedCount, &item.ExcludedCount, &item.ErrorCount, &reasons,
		&item.RequestedAt, &item.StartedAt, &item.FinishedAt,
	)
	if err != nil {
		return SegmentEvaluationRun{}, mapDBError(err)
	}
	item.ReasonCodes = reasons
	return item, nil
}

func (r *PostgresRepository) CreateEvaluationRun(
	ctx context.Context,
	scope Scope,
	segmentID string,
	request SegmentEvaluationRequest,
	version SegmentVersion,
	asOf time.Time,
) (SegmentEvaluationRun, error) {
	existing, err := scanRun(r.pool.QueryRow(ctx, `
		select `+runColumns("")+`
		from customer_data.segment_evaluation_runs
		where account_id = $1::uuid and client_account_id = $2::uuid and idempotency_key = $3
	`, scope.AccountID, scope.ClientAccountID, request.IdempotencyKey))
	if err == nil {
		existing.Replayed = true
		return existing, nil
	}
	if err != ErrNotFound {
		return SegmentEvaluationRun{}, err
	}
	fingerprintSource := version.DefinitionHash + "\x00" + asOf.UTC().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(fingerprintSource))
	item, err := scanRun(r.pool.QueryRow(ctx, `
		insert into customer_data.segment_evaluation_runs (
			account_id, client_account_id, segment_id, version_id, mode,
			trigger_kind, status, as_of, definition_hash, input_fingerprint,
			field_catalog_version, idempotency_key, requested_by_user_id
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4::uuid, $5,
			'manual', 'queued', $6, $7, $8, $9, $10, nullif($11, '')::uuid
		)
		returning `+runColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, segmentID, version.ID, request.Mode,
		asOf.UTC(), version.DefinitionHash, hex.EncodeToString(sum[:]),
		version.FieldCatalogVersion, request.IdempotencyKey, scope.ActorUserID))
	return item, err
}

func (r *PostgresRepository) GetEvaluationRun(
	ctx context.Context,
	scope Scope,
	runID string,
) (SegmentEvaluationRun, error) {
	return scanRun(r.pool.QueryRow(ctx, `
		select `+runColumns("")+`
		from customer_data.segment_evaluation_runs
		where account_id = $1::uuid and client_account_id = $2::uuid and id = $3::uuid
	`, scope.AccountID, scope.ClientAccountID, runID))
}

func materializationColumns(alias string) string {
	if alias != "" {
		alias += "."
	}
	return strings.Join([]string{
		alias + "id::text", alias + "segment_id::text", alias + "version_id::text",
		alias + "evaluation_run_id::text", alias + "as_of", alias + "status",
		alias + "member_count", alias + "fresh_until", alias + "created_at", alias + "completed_at",
	}, ", ")
}

func scanMaterialization(row rowScanner) (SegmentMaterialization, error) {
	var item SegmentMaterialization
	err := row.Scan(
		&item.ID, &item.SegmentID, &item.VersionID, &item.EvaluationRunID,
		&item.AsOf, &item.Status, &item.MemberCount, &item.FreshUntil,
		&item.CreatedAt, &item.CompletedAt,
	)
	return item, mapDBError(err)
}

func (r *PostgresRepository) ListMaterializations(
	ctx context.Context,
	scope Scope,
	segmentID string,
	limit int,
) ([]SegmentMaterialization, error) {
	rows, err := r.pool.Query(ctx, `
		select `+materializationColumns("")+`
		from customer_data.segment_materializations
		where account_id = $1::uuid and client_account_id = $2::uuid and segment_id = $3::uuid
		order by created_at desc, id desc
		limit $4
	`, scope.AccountID, scope.ClientAccountID, segmentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SegmentMaterialization, 0)
	for rows.Next() {
		item, err := scanMaterialization(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) ListMaterializationMembers(
	ctx context.Context,
	scope Scope,
	materializationID, cursor string,
	limit int,
) ([]SegmentMember, string, error) {
	args := []any{scope.AccountID, scope.ClientAccountID, materializationID}
	var query strings.Builder
	query.WriteString(`
		select relationship_id::text, subject_id::text, matched_at
		from customer_data.segment_memberships
		where account_id = $1::uuid and client_account_id = $2::uuid
		  and materialization_id = $3::uuid
	`)
	if cursor != "" {
		args = append(args, cursor)
		_, _ = fmt.Fprintf(&query, " and relationship_id > $%d::uuid", len(args))
	}
	args = append(args, limit+1)
	_, _ = fmt.Fprintf(&query, " order by relationship_id limit $%d", len(args))
	rows, err := r.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := make([]SegmentMember, 0, limit)
	var next string
	for rows.Next() {
		var item SegmentMember
		if err := rows.Scan(&item.RelationshipID, &item.SubjectID, &item.MatchedAt); err != nil {
			return nil, "", err
		}
		if len(items) == limit {
			next = items[len(items)-1].RelationshipID
			break
		}
		items = append(items, item)
	}
	return items, next, rows.Err()
}

func (r *PostgresRepository) GetSegmentContext(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	asOf time.Time,
) (SegmentContext, error) {
	rows, err := r.pool.Query(ctx, `
		select s.id::text, s.segment_key, m.version_id::text, m.completed_at
		from customer_data.segment_memberships membership
		join customer_data.segment_materializations m
		  on m.account_id = membership.account_id
		 and m.client_account_id = membership.client_account_id
		 and m.id = membership.materialization_id
		join customer_data.segments s
		  on s.account_id = m.account_id
		 and s.client_account_id = m.client_account_id
		 and s.id = m.segment_id
		 and s.current_materialization_id = m.id
		where membership.account_id = $1::uuid
		  and membership.client_account_id = $2::uuid
		  and membership.relationship_id = $3::uuid
		  and s.status = 'active'
		  and m.status = 'current'
		  and m.as_of <= $4
		  and (m.expires_at is null or m.expires_at > $4)
		order by s.segment_key, s.id
	`, scope.AccountID, scope.ClientAccountID, relationshipID, asOf.UTC())
	if err != nil {
		return SegmentContext{}, err
	}
	defer rows.Close()
	out := SegmentContext{RelationshipID: relationshipID, AsOf: asOf.UTC(), Segments: []SegmentContextItem{}}
	for rows.Next() {
		var item SegmentContextItem
		if err := rows.Scan(&item.SegmentID, &item.SegmentKey, &item.VersionID, &item.MaterializedAt); err != nil {
			return SegmentContext{}, err
		}
		out.Segments = append(out.Segments, item)
	}
	return out, rows.Err()
}
