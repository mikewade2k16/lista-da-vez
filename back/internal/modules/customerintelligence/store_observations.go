package customerintelligence

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresRepository) ListRelationshipObservations(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	sourceKeys []string,
	purposeKeys []string,
	limit int,
) ([]StoredObservation, error) {
	if len(purposeKeys) == 0 {
		return []StoredObservation{}, nil
	}
	if sourceKeys == nil {
		sourceKeys = []string{}
	}
	rows, err := s.pool.Query(ctx, `
		select o.id, coalesce(o.subject_id::text, ''),
		       coalesce(o.relationship_id::text, ''), o.source_key,
		       o.source_entity_type, o.source_entity_id,
		       coalesce(o.snapshot_json, '{}'::jsonb),
		       coalesce(o.snapshot_ciphertext, ''),
		       coalesce(c.field_allowlist, '[]'::jsonb),
		       o.sensitivity, o.purpose_key, o.source_occurred_at,
		       o.observed_at, o.expires_at
		from intelligence.source_observations o
		join intelligence.source_configs c
		  on c.account_id = o.account_id
		 and c.client_account_id = o.client_account_id
		 and c.id = o.source_config_id
		where o.account_id = $1
		  and o.client_account_id = $2
		  and (
		      (
		          o.relationship_id = $3
		          and o.subject_id is not null
		          and o.classification = 'customer_relationship'
		      )
		      or (
		          o.relationship_id is null
		          and o.subject_id is null
		          and o.classification = 'client_business_context'
		      )
		  )
		  and c.status = 'enabled'
		  and (o.expires_at is null or o.expires_at > now())
		  and (cardinality($4::text[]) = 0 or o.source_key = any($4::text[]))
		  and o.purpose_key = any($5::text[])
		  and o.sensitivity <> 'restricted'
		order by coalesce(o.source_occurred_at, o.observed_at) desc, o.id desc
		limit $6`,
		scope.AccountID,
		scope.ClientAccountID,
		relationshipID,
		sourceKeys,
		purposeKeys,
		limit,
	)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	items := make([]StoredObservation, 0)
	for rows.Next() {
		item, scanErr := scanStoredObservation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) GetObservation(
	ctx context.Context,
	scope Scope,
	observationID string,
) (StoredObservation, error) {
	row := s.pool.QueryRow(ctx, `
		select o.id, coalesce(o.subject_id::text, ''),
		       coalesce(o.relationship_id::text, ''), o.source_key,
		       o.source_entity_type, o.source_entity_id,
		       coalesce(o.snapshot_json, '{}'::jsonb),
		       coalesce(o.snapshot_ciphertext, ''),
		       coalesce(c.field_allowlist, '[]'::jsonb),
		       o.sensitivity, o.purpose_key, o.source_occurred_at,
		       o.observed_at, o.expires_at
		from intelligence.source_observations o
		join intelligence.source_configs c
		  on c.account_id = o.account_id
		 and c.client_account_id = o.client_account_id
		 and c.id = o.source_config_id
		where o.account_id = $1
		  and o.client_account_id = $2
		  and o.id = $3
		  and (
		      (
		          o.relationship_id is not null
		          and o.subject_id is not null
		          and o.classification = 'customer_relationship'
		      )
		      or (
		          o.relationship_id is null
		          and o.subject_id is null
		          and o.classification = 'client_business_context'
		      )
		  )
		  and (o.expires_at is null or o.expires_at > now())`,
		scope.AccountID,
		scope.ClientAccountID,
		observationID,
	)
	item, err := scanStoredObservation(row)
	if err != nil {
		return StoredObservation{}, repositoryError(err)
	}
	return item, nil
}

func (s *PostgresRepository) RecordObservationAccess(
	ctx context.Context,
	scope Scope,
	actorUserID string,
	record StoredObservation,
	reasonCode string,
	revealed bool,
	fieldCount int,
) error {
	tag, err := s.pool.Exec(ctx, `
		insert into intelligence.audit_events (
		    account_id, client_account_id, actor_user_id, event_type,
		    aggregate_type, aggregate_id, reason_code, metadata
		)
		select
		    o.account_id, o.client_account_id, $4,
		    'source.observation_accessed', 'source_observation', o.id::text,
		    $5,
		    jsonb_build_object(
		        'sourceKey', o.source_key,
		        'sensitivity', o.sensitivity,
		        'purposeKey', o.purpose_key,
		        'revealed', $6::boolean,
		        'fieldCount', $7::integer
		    )
		from intelligence.source_observations o
		where o.account_id = $1
		  and o.client_account_id = $2
		  and o.id = $3`,
		scope.AccountID,
		scope.ClientAccountID,
		record.ID,
		actorUserID,
		reasonCode,
		revealed,
		fieldCount,
	)
	if err != nil {
		return repositoryError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

type observationScanner interface {
	Scan(dest ...any) error
}

func scanStoredObservation(row observationScanner) (StoredObservation, error) {
	var item StoredObservation
	var snapshot, allowlist []byte
	err := row.Scan(
		&item.ID,
		&item.SubjectID,
		&item.RelationshipID,
		&item.SourceKey,
		&item.SourceEntityType,
		&item.SourceEntityID,
		&snapshot,
		&item.SnapshotCiphertext,
		&allowlist,
		&item.Sensitivity,
		&item.PurposeKey,
		&item.SourceOccurredAt,
		&item.ObservedAt,
		&item.ExpiresAt,
	)
	if err != nil {
		return StoredObservation{}, err
	}
	item.Snapshot = json.RawMessage(snapshot)
	if err := json.Unmarshal(allowlist, &item.FieldAllowlist); err != nil {
		return StoredObservation{}, err
	}
	if item.FieldAllowlist == nil {
		item.FieldAllowlist = []string{}
	}
	return item, nil
}

var _ ObservationRepository = (*PostgresRepository)(nil)
var _ ObservationAccessRecorder = (*PostgresRepository)(nil)
var _ observationScanner = (pgx.Row)(nil)
