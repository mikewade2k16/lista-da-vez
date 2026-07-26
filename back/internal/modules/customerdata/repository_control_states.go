package customerdata

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) ListCapabilityStates(ctx context.Context, scope Scope) ([]CapabilityState, error) {
	rows, err := r.pool.Query(ctx, `
		select capability_key, mode, revision, updated_at
		from customer_data.capability_states
		where account_id = $1::uuid and client_account_id = $2::uuid
		order by capability_key
	`, scope.AccountID, scope.ClientAccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CapabilityState, 0)
	for rows.Next() {
		var item CapabilityState
		if err := rows.Scan(&item.CapabilityKey, &item.Mode, &item.Revision, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) GetCapabilityState(
	ctx context.Context,
	scope Scope,
	capability string,
) (CapabilityState, error) {
	var item CapabilityState
	err := r.pool.QueryRow(ctx, `
		select capability_key, mode, revision, updated_at
		from customer_data.capability_states
		where account_id = $1::uuid and client_account_id = $2::uuid and capability_key = $3
	`, scope.AccountID, scope.ClientAccountID, capability).Scan(
		&item.CapabilityKey, &item.Mode, &item.Revision, &item.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return CapabilityState{CapabilityKey: capability, Mode: CapabilityOff}, nil
	}
	return item, err
}

func (r *PostgresRepository) SetCapabilityState(
	ctx context.Context,
	scope Scope,
	capability string,
	input CapabilityStateInput,
) (CapabilityState, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return CapabilityState{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var current CapabilityState
	var lastKey *string
	err = tx.QueryRow(ctx, `
		select capability_key, mode, revision, updated_at, last_idempotency_key
		from customer_data.capability_states
		where account_id = $1::uuid and client_account_id = $2::uuid and capability_key = $3
		for update
	`, scope.AccountID, scope.ClientAccountID, capability).Scan(
		&current.CapabilityKey, &current.Mode, &current.Revision, &current.UpdatedAt, &lastKey,
	)
	switch err {
	case pgx.ErrNoRows:
		if input.ExpectedRevision != 0 {
			return CapabilityState{}, false, ErrConflict
		}
		err = tx.QueryRow(ctx, `
			insert into customer_data.capability_states (
				account_id, client_account_id, capability_key, mode, revision,
				last_idempotency_key, updated_by_user_id
			) values ($1::uuid, $2::uuid, $3, $4, 1, $5, nullif($6, '')::uuid)
			returning capability_key, mode, revision, updated_at
		`, scope.AccountID, scope.ClientAccountID, capability, input.Mode,
			input.IdempotencyKey, scope.ActorUserID).Scan(
			&current.CapabilityKey, &current.Mode, &current.Revision, &current.UpdatedAt,
		)
		if err != nil {
			return CapabilityState{}, false, mapDBError(err)
		}
	case nil:
		if lastKey != nil && *lastKey == input.IdempotencyKey {
			return current, true, nil
		}
		if current.Revision != input.ExpectedRevision {
			return CapabilityState{}, false, ErrConflict
		}
		err = tx.QueryRow(ctx, `
			update customer_data.capability_states
			set mode = $4, revision = revision + 1,
			    last_idempotency_key = $5,
			    updated_by_user_id = nullif($6, '')::uuid,
			    updated_at = now()
			where account_id = $1::uuid and client_account_id = $2::uuid
			  and capability_key = $3 and revision = $7
			returning capability_key, mode, revision, updated_at
		`, scope.AccountID, scope.ClientAccountID, capability, input.Mode,
			input.IdempotencyKey, scope.ActorUserID, input.ExpectedRevision).Scan(
			&current.CapabilityKey, &current.Mode, &current.Revision, &current.UpdatedAt,
		)
		if err != nil {
			return CapabilityState{}, false, mapMutationError(err)
		}
	default:
		return CapabilityState{}, false, err
	}
	if err := insertAudit(
		ctx, tx, scope, "", "", "control_state_changed",
		"capability_state", capability, input.Reason,
	); err != nil {
		return CapabilityState{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CapabilityState{}, false, err
	}
	return current, false, nil
}

func (r *PostgresRepository) ListWriterStates(ctx context.Context, scope Scope) ([]WriterState, error) {
	rows, err := r.pool.Query(ctx, `
		select entity_key, mode, watermark, source_checksum, target_checksum,
		       approved_by_user_id::text, approved_at, revision, updated_at
		from customer_data.writer_states
		where account_id = $1::uuid and client_account_id = $2::uuid
		order by entity_key
	`, scope.AccountID, scope.ClientAccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]WriterState, 0)
	for rows.Next() {
		item, err := scanWriterState(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) GetWriterState(
	ctx context.Context,
	scope Scope,
	entity string,
) (WriterState, error) {
	item, err := scanWriterState(r.pool.QueryRow(ctx, `
		select entity_key, mode, watermark, source_checksum, target_checksum,
		       approved_by_user_id::text, approved_at, revision, updated_at
		from customer_data.writer_states
		where account_id = $1::uuid and client_account_id = $2::uuid and entity_key = $3
	`, scope.AccountID, scope.ClientAccountID, entity))
	if err == ErrNotFound {
		return WriterState{EntityKey: entity, Mode: WriterLegacy}, nil
	}
	return item, err
}

func scanWriterState(row rowScanner) (WriterState, error) {
	var item WriterState
	err := row.Scan(
		&item.EntityKey, &item.Mode, &item.Watermark, &item.SourceChecksum,
		&item.TargetChecksum, &item.ApprovedBy, &item.ApprovedAt,
		&item.Revision, &item.UpdatedAt,
	)
	return item, mapDBError(err)
}

func (r *PostgresRepository) SetWriterState(
	ctx context.Context,
	scope Scope,
	entity string,
	input WriterStateInput,
) (WriterState, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return WriterState{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var current WriterState
	var lastKey *string
	err = tx.QueryRow(ctx, `
		select entity_key, mode, watermark, source_checksum, target_checksum,
		       approved_by_user_id::text, approved_at, revision, updated_at,
		       last_idempotency_key
		from customer_data.writer_states
		where account_id = $1::uuid and client_account_id = $2::uuid and entity_key = $3
		for update
	`, scope.AccountID, scope.ClientAccountID, entity).Scan(
		&current.EntityKey, &current.Mode, &current.Watermark,
		&current.SourceChecksum, &current.TargetChecksum, &current.ApprovedBy,
		&current.ApprovedAt, &current.Revision, &current.UpdatedAt, &lastKey,
	)
	switch err {
	case pgx.ErrNoRows:
		if input.ExpectedRevision != 0 {
			return WriterState{}, false, ErrConflict
		}
		err = tx.QueryRow(ctx, `
			insert into customer_data.writer_states (
				account_id, client_account_id, entity_key, mode, watermark,
				source_checksum, target_checksum, approved_by_user_id, approved_at,
				revision, last_idempotency_key
			) values (
				$1::uuid, $2::uuid, $3, $4, $5, $6, $7,
				case when $4 = 'new' then nullif($8, '')::uuid else null end,
				case when $4 = 'new' then now() else null end,
				1, $9
			)
			returning entity_key, mode, watermark, source_checksum, target_checksum,
			          approved_by_user_id::text, approved_at, revision, updated_at
		`, scope.AccountID, scope.ClientAccountID, entity, input.Mode,
			input.Watermark, input.SourceChecksum, input.TargetChecksum,
			scope.ActorUserID, input.IdempotencyKey).Scan(
			&current.EntityKey, &current.Mode, &current.Watermark,
			&current.SourceChecksum, &current.TargetChecksum, &current.ApprovedBy,
			&current.ApprovedAt, &current.Revision, &current.UpdatedAt,
		)
		if err != nil {
			return WriterState{}, false, mapDBError(err)
		}
	case nil:
		if lastKey != nil && *lastKey == input.IdempotencyKey {
			return current, true, nil
		}
		if current.Revision != input.ExpectedRevision {
			return WriterState{}, false, ErrConflict
		}
		err = tx.QueryRow(ctx, `
			update customer_data.writer_states
			set mode = $4,
			    watermark = $5,
			    source_checksum = $6,
			    target_checksum = $7,
			    approved_by_user_id = case when $4 = 'new' then nullif($8, '')::uuid else approved_by_user_id end,
			    approved_at = case when $4 = 'new' then now() else approved_at end,
			    revision = revision + 1,
			    last_idempotency_key = $9,
			    updated_at = now()
			where account_id = $1::uuid and client_account_id = $2::uuid
			  and entity_key = $3 and revision = $10
			returning entity_key, mode, watermark, source_checksum, target_checksum,
			          approved_by_user_id::text, approved_at, revision, updated_at
		`, scope.AccountID, scope.ClientAccountID, entity, input.Mode,
			input.Watermark, input.SourceChecksum, input.TargetChecksum,
			scope.ActorUserID, input.IdempotencyKey, input.ExpectedRevision).Scan(
			&current.EntityKey, &current.Mode, &current.Watermark,
			&current.SourceChecksum, &current.TargetChecksum, &current.ApprovedBy,
			&current.ApprovedAt, &current.Revision, &current.UpdatedAt,
		)
		if err != nil {
			return WriterState{}, false, mapMutationError(err)
		}
	default:
		return WriterState{}, false, err
	}
	if err := insertAudit(
		ctx, tx, scope, "", "", "control_state_changed",
		"writer_state", entity, input.Reason,
	); err != nil {
		return WriterState{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return WriterState{}, false, err
	}
	return current, false, nil
}
