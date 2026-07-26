package customerintelligence

import (
	"context"
)

const observationRetentionBatchSize = 250

type observationRetentionRepository interface {
	ListExpiredRetentionScopes(ctx context.Context) ([]Scope, error)
	ApplyExpiredObservationRetention(
		ctx context.Context,
		scope Scope,
		correlationID string,
		limit int,
	) (int, error)
	ApplyExpiredContextSnapshotRetention(
		ctx context.Context,
		scope Scope,
		correlationID string,
		limit int,
	) (int, error)
}

func (s *PostgresRepository) ListExpiredRetentionScopes(
	ctx context.Context,
) ([]Scope, error) {
	rows, err := s.pool.Query(ctx, `
		select distinct
		    expired.account_id::text,
		    expired.client_account_id::text
		from (
		    select
		        observation.account_id,
		        observation.client_account_id
		    from intelligence.source_observations observation
		    where observation.retention_state = 'active'
		      and observation.expires_at <= now()
		      and not exists (
		          select 1
		          from intelligence.observation_legal_holds legal_hold
		          where legal_hold.account_id = observation.account_id
		            and legal_hold.client_account_id = observation.client_account_id
		            and legal_hold.observation_id = observation.id
		            and legal_hold.status = 'active'
		      )

		    union all

		    select
		        snapshot.account_id,
		        snapshot.client_account_id
		    from intelligence.context_snapshots snapshot
		    where snapshot.retention_state = 'active'
		      and snapshot.expires_at <= now()
		      and not intelligence.context_snapshot_has_active_legal_hold(
		          snapshot.account_id,
		          snapshot.client_account_id,
		          snapshot.id,
		          snapshot.subject_id,
		          snapshot.relationship_id
		      )
		) expired
		order by
		    expired.account_id::text,
		    expired.client_account_id::text`)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()

	scopes := make([]Scope, 0)
	for rows.Next() {
		var scope Scope
		if err := rows.Scan(&scope.AccountID, &scope.ClientAccountID); err != nil {
			return nil, err
		}
		scopes = append(scopes, scope)
	}
	return scopes, rows.Err()
}

func (s *PostgresRepository) ApplyExpiredObservationRetention(
	ctx context.Context,
	scope Scope,
	correlationID string,
	limit int,
) (int, error) {
	limit = bounded(limit, observationRetentionBatchSize, 1, observationRetentionBatchSize)
	var applied int
	err := s.pool.QueryRow(ctx, `
		with candidates as (
		    select
		        observation.id,
		        policy.on_expiry as configured_action,
		        case
		            when policy.on_expiry = 'crypto_shred'
		             and observation.snapshot_ciphertext is not null
		                then 'crypto_shredded'
		            else 'tombstoned'
		        end as applied_state
		    from intelligence.source_observations observation
		    join intelligence.retention_policy_versions policy
		      on policy.account_id = observation.account_id
		     and policy.id = observation.retention_policy_version_id
		    where observation.account_id = $1
		      and observation.client_account_id = $2
		      and observation.retention_state = 'active'
		      and observation.expires_at <= now()
		      and not exists (
		          select 1
		          from intelligence.observation_legal_holds legal_hold
		          where legal_hold.account_id = observation.account_id
		            and legal_hold.client_account_id = observation.client_account_id
		            and legal_hold.observation_id = observation.id
		            and legal_hold.status = 'active'
		      )
		    order by observation.expires_at, observation.id
		    limit $4
		    for update of observation skip locked
		),
		updated as (
		    update intelligence.source_observations observation
		    set
		        snapshot_json = null,
		        snapshot_ciphertext = null,
		        cipher_key_version = '',
		        retention_state = candidates.applied_state,
		        retention_applied_at = now(),
		        retention_reason_code = 'retention_policy_expired'
		    from candidates
		    where observation.account_id = $1
		      and observation.client_account_id = $2
		      and observation.id = candidates.id
		      and observation.retention_state = 'active'
		      and not exists (
		          select 1
		          from intelligence.observation_legal_holds legal_hold
		          where legal_hold.account_id = observation.account_id
		            and legal_hold.client_account_id = observation.client_account_id
		            and legal_hold.observation_id = observation.id
		            and legal_hold.status = 'active'
		      )
		    returning
		        observation.id,
		        observation.client_account_id,
		        observation.source_key,
		        observation.retention_policy_version_id,
		        observation.expires_at,
		        candidates.configured_action,
		        candidates.applied_state
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
		        'source.observation_retention_applied',
		        'source_observation',
		        updated.id::text,
		        $3,
		        'retention_policy_expired',
		        jsonb_build_object(
		            'sourceKey', updated.source_key,
		            'retentionPolicyVersionId', updated.retention_policy_version_id,
		            'configuredAction', updated.configured_action,
		            'appliedState', updated.applied_state,
		            'expiresAt', updated.expires_at
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
	return applied, repositoryError(err)
}

var _ observationRetentionRepository = (*PostgresRepository)(nil)
