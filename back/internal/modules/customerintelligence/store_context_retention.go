package customerintelligence

import "context"

const contextSnapshotRetentionStateCryptoShredded = "crypto_shredded"

func (s *PostgresRepository) ApplyExpiredContextSnapshotRetention(
	ctx context.Context,
	scope Scope,
	correlationID string,
	limit int,
) (int, error) {
	limit = bounded(limit, observationRetentionBatchSize, 1, observationRetentionBatchSize)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Legal-hold mutations acquire the same transaction lock. Taking it in a
	// separate statement ensures the retention query gets a fresh snapshot
	// after any hold transaction that won the lock has committed.
	if _, err = tx.Exec(
		ctx,
		`select pg_advisory_xact_lock(hashtextextended($1, 0))`,
		contextSnapshotRetentionLockKey(scope),
	); err != nil {
		return 0, repositoryError(err)
	}

	var applied int
	err = tx.QueryRow(ctx, `
		with candidates as (
		    select snapshot.id
		    from intelligence.context_snapshots snapshot
		    where snapshot.account_id = $1
		      and snapshot.client_account_id = $2
		      and snapshot.retention_state = 'active'
		      and snapshot.expires_at <= now()
		      and not intelligence.context_snapshot_has_active_legal_hold(
		          snapshot.account_id,
		          snapshot.client_account_id,
		          snapshot.id,
		          snapshot.subject_id,
		          snapshot.relationship_id
		      )
		    order by snapshot.expires_at, snapshot.id
		    limit $4
		    for update of snapshot skip locked
		),
		updated as (
		    update intelligence.context_snapshots snapshot
		    set
		        payload_ciphertext = null,
		        cipher_key_version = '',
		        payload_hash = '',
		        retention_state = 'crypto_shredded',
		        tombstoned_at = now(),
		        retention_reason_code = 'context_snapshot_expired'
		    from candidates
		    where snapshot.account_id = $1
		      and snapshot.client_account_id = $2
		      and snapshot.id = candidates.id
		      and snapshot.retention_state = 'active'
		      and not intelligence.context_snapshot_has_active_legal_hold(
		          snapshot.account_id,
		          snapshot.client_account_id,
		          snapshot.id,
		          snapshot.subject_id,
		          snapshot.relationship_id
		      )
		    returning
		        snapshot.id,
		        snapshot.client_account_id,
		        snapshot.process_keys,
		        snapshot.purpose_key,
		        snapshot.as_of,
		        snapshot.expires_at,
		        snapshot.item_count,
		        snapshot.token_estimate,
		        snapshot.retention_state
		),
		audited as (
		    insert into intelligence.audit_events (
		        account_id,
		        client_account_id,
		        event_type,
		        aggregate_type,
		        aggregate_id,
		        correlation_id,
		        reason_code,
		        metadata
		    )
		    select
		        $1,
		        updated.client_account_id,
		        'context.snapshot_retention_applied',
		        'context_snapshot',
		        updated.id::text,
		        $3,
		        'context_snapshot_expired',
		        jsonb_build_object(
		            'processKeys', updated.process_keys,
		            'purposeKey', updated.purpose_key,
		            'asOf', updated.as_of,
		            'expiresAt', updated.expires_at,
		            'itemCount', updated.item_count,
		            'tokenEstimate', updated.token_estimate,
		            'appliedState', updated.retention_state
		        )
		    from updated
		    returning 1
		)
		select count(*)::integer from audited`,
		scope.AccountID,
		scope.ClientAccountID,
		correlationID,
		limit,
	).Scan(&applied)
	if err != nil {
		return 0, repositoryError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return applied, nil
}

func contextSnapshotRetentionLockKey(scope Scope) string {
	return "customer_intelligence:context_retention:" +
		scope.AccountID + ":" + scope.ClientAccountID
}
