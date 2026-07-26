package customerintelligence

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

type storedRetentionPolicy struct {
	ID                 string
	Key                string
	Version            int
	SnapshotTTLSeconds int
	OnExpiry           string
}

func effectiveSourceRetention(
	input SourceConfigInput,
	current SourceConfig,
) (string, int, string) {
	key := input.RetentionPolicyKey
	if key == "" {
		key = current.RetentionPolicyKey
	}
	if key == "" {
		key = defaultRetentionPolicyKey
	}
	ttlSeconds := input.SnapshotTTLSeconds
	if ttlSeconds == 0 {
		ttlSeconds = current.SnapshotTTLSeconds
	}
	if ttlSeconds == 0 {
		ttlSeconds = defaultSnapshotTTLSeconds
	}
	onExpiry := input.OnExpiry
	if onExpiry == "" {
		onExpiry = current.OnExpiry
	}
	if onExpiry == "" {
		onExpiry = retentionActionTombstone
	}
	return key, ttlSeconds, onExpiry
}

func findPublishedRetentionPolicy(
	ctx context.Context,
	tx pgx.Tx,
	accountID, key string,
	ttlSeconds int,
	onExpiry string,
) (storedRetentionPolicy, error) {
	var policy storedRetentionPolicy
	err := tx.QueryRow(ctx, `
		select id, policy_key, version, snapshot_ttl_seconds, on_expiry
		from intelligence.retention_policy_versions
		where account_id = $1
		  and policy_key = $2
		  and status = 'published'
		  and snapshot_ttl_seconds = $3
		  and on_expiry = $4
		order by version desc, id desc
		limit 1`,
		accountID, key, ttlSeconds, onExpiry,
	).Scan(
		&policy.ID, &policy.Key, &policy.Version,
		&policy.SnapshotTTLSeconds, &policy.OnExpiry,
	)
	if err == nil {
		return policy, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return storedRetentionPolicy{}, repositoryError(err)
	}
	return storedRetentionPolicy{}, ErrRetentionPolicyApprovalRequired
}

func (s *PostgresRepository) ListRetentionPolicyVersions(
	ctx context.Context,
	accountID string,
) ([]RetentionPolicyVersion, error) {
	rows, err := s.pool.Query(ctx, `
		select
		    id, account_id, policy_key, version, status,
		    snapshot_ttl_seconds, on_expiry, legal_hold_behavior,
		    block_reingestion, revision,
		    coalesce(created_by_user_id::text, ''),
		    coalesce(published_by_user_id::text, ''),
		    publication_reason_code, approval_reference,
		    created_at, published_at
		from intelligence.retention_policy_versions
		where account_id = $1
		order by policy_key, version desc, id desc`,
		accountID,
	)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()

	items := make([]RetentionPolicyVersion, 0)
	for rows.Next() {
		item, scanErr := scanRetentionPolicyVersion(rows)
		if scanErr != nil {
			return nil, repositoryError(scanErr)
		}
		items = append(items, item)
	}
	return items, repositoryError(rows.Err())
}

func (s *PostgresRepository) CreateRetentionPolicyDraft(
	ctx context.Context,
	accountID, actorID, policyKey string,
	input RetentionPolicyDraftInput,
) (RetentionPolicyVersion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RetentionPolicyVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(
		ctx,
		`select pg_advisory_xact_lock(hashtextextended($1, 0))`,
		accountID+":"+policyKey,
	); err != nil {
		return RetentionPolicyVersion{}, repositoryError(err)
	}

	categoryRules, _ := json.Marshal(map[string]any{
		"snapshotTtlSeconds": input.SnapshotTTLSeconds,
		"onExpiry":           input.OnExpiry,
	})
	item, err := scanRetentionPolicyVersion(tx.QueryRow(ctx, `
		insert into intelligence.retention_policy_versions (
		    account_id, policy_key, version, status, category_rules,
		    snapshot_ttl_seconds, on_expiry, legal_hold_behavior,
		    block_reingestion, created_by_user_id
		)
		select $1, $2, coalesce(max(version), 0) + 1, 'draft', $3,
		       $4, $5, 'preserve', true, $6::uuid
		from intelligence.retention_policy_versions
		where account_id = $1 and policy_key = $2
		returning
		    id, account_id, policy_key, version, status,
		    snapshot_ttl_seconds, on_expiry, legal_hold_behavior,
		    block_reingestion, revision,
		    coalesce(created_by_user_id::text, ''),
		    coalesce(published_by_user_id::text, ''),
		    publication_reason_code, approval_reference,
		    created_at, published_at`,
		accountID,
		policyKey,
		categoryRules,
		input.SnapshotTTLSeconds,
		input.OnExpiry,
		actorID,
	))
	if err != nil {
		return RetentionPolicyVersion{}, repositoryError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return RetentionPolicyVersion{}, err
	}
	return item, nil
}

func (s *PostgresRepository) PublishRetentionPolicyVersion(
	ctx context.Context,
	accountID, actorID, id string,
	input PublishRetentionPolicyInput,
) (RetentionPolicyVersion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RetentionPolicyVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	var revision int64
	err = tx.QueryRow(ctx, `
		select status, revision
		from intelligence.retention_policy_versions
		where account_id = $1 and id = $2
		for update`,
		accountID,
		id,
	).Scan(&status, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return RetentionPolicyVersion{}, ErrNotFound
	}
	if err != nil {
		return RetentionPolicyVersion{}, repositoryError(err)
	}
	if status != "draft" || revision != input.ExpectedRevision {
		return RetentionPolicyVersion{}, ErrConflict
	}

	item, err := scanRetentionPolicyVersion(tx.QueryRow(ctx, `
		update intelligence.retention_policy_versions
		set
		    status = 'published',
		    revision = revision + 1,
		    published_by_user_id = $3::uuid,
		    published_at = now(),
		    publication_reason_code = $4,
		    approval_reference = $5
		where account_id = $1
		  and id = $2
		  and status = 'draft'
		  and revision = $6
		returning
		    id, account_id, policy_key, version, status,
		    snapshot_ttl_seconds, on_expiry, legal_hold_behavior,
		    block_reingestion, revision,
		    coalesce(created_by_user_id::text, ''),
		    coalesce(published_by_user_id::text, ''),
		    publication_reason_code, approval_reference,
		    created_at, published_at`,
		accountID,
		id,
		actorID,
		input.ReasonCode,
		input.ApprovalReference,
		input.ExpectedRevision,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return RetentionPolicyVersion{}, ErrConflict
	}
	if err != nil {
		return RetentionPolicyVersion{}, repositoryError(err)
	}
	if _, err = tx.Exec(ctx, `
		insert into intelligence.audit_events (
		    account_id,
		    actor_user_id,
		    event_type,
		    aggregate_type,
		    aggregate_id,
		    reason_code,
		    metadata
		)
		values (
		    $1,
		    $2::uuid,
		    'retention_policy.published',
		    'retention_policy_version',
		    $3,
		    $4,
		    jsonb_build_object(
		        'policyKey', $5::text,
		        'version', $6::integer,
		        'approvalReference', $7::text,
		        'expectedRevision', $8::bigint,
		        'publishedRevision', $9::bigint
		    )
		)`,
		accountID,
		actorID,
		item.ID,
		input.ReasonCode,
		item.PolicyKey,
		item.Version,
		input.ApprovalReference,
		input.ExpectedRevision,
		item.Revision,
	); err != nil {
		return RetentionPolicyVersion{}, repositoryError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return RetentionPolicyVersion{}, err
	}
	return item, nil
}

type retentionPolicyVersionScanner interface {
	Scan(dest ...any) error
}

func scanRetentionPolicyVersion(
	row retentionPolicyVersionScanner,
) (RetentionPolicyVersion, error) {
	var item RetentionPolicyVersion
	err := row.Scan(
		&item.ID,
		&item.AccountID,
		&item.PolicyKey,
		&item.Version,
		&item.Status,
		&item.SnapshotTTLSeconds,
		&item.OnExpiry,
		&item.LegalHoldBehavior,
		&item.BlockReingestion,
		&item.Revision,
		&item.CreatedByUserID,
		&item.PublishedByUserID,
		&item.PublicationReasonCode,
		&item.ApprovalReference,
		&item.CreatedAt,
		&item.PublishedAt,
	)
	return item, err
}
