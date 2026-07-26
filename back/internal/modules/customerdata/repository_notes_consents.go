package customerdata

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

func noteColumns(alias string) string {
	if alias != "" {
		alias += "."
	}
	return strings.Join([]string{
		alias + "id::text", alias + "relationship_id::text", alias + "content",
		alias + "revision", alias + "archived_at", alias + "created_at", alias + "updated_at",
	}, ", ")
}

func scanNote(row rowScanner) (Note, error) {
	var item Note
	err := row.Scan(
		&item.ID, &item.RelationshipID, &item.Content, &item.Revision,
		&item.ArchivedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, mapDBError(err)
}

func (r *PostgresRepository) ListNotes(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	limit int,
) ([]Note, error) {
	rows, err := r.pool.Query(ctx, `
		select `+noteColumns("")+`
		from customer_data.relationship_notes
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and relationship_id = $3::uuid
		  and archived_at is null
		order by created_at desc, id desc
		limit $4
	`, scope.AccountID, scope.ClientAccountID, relationshipID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Note, 0)
	for rows.Next() {
		item, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) CreateNote(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	input NoteInput,
) (Note, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Note{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	item, err := scanNote(tx.QueryRow(ctx, `
		select `+noteColumns("")+`
		from customer_data.relationship_notes
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and idempotency_key = $3
	`, scope.AccountID, scope.ClientAccountID, input.IdempotencyKey))
	if err == nil {
		return item, true, nil
	}
	if err != ErrNotFound {
		return Note{}, false, err
	}
	var sourceModule, entityType, entityID string
	if input.ContextSource != nil {
		sourceModule = input.ContextSource.SourceModule
		entityType = input.ContextSource.SourceEntityType
		entityID = input.ContextSource.SourceEntityID
	}
	item, err = scanNote(tx.QueryRow(ctx, `
		insert into customer_data.relationship_notes (
			account_id, client_account_id, relationship_id, content,
			context_source_module, context_entity_type, context_entity_id,
			author_user_id, idempotency_key
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4,
			nullif($5, ''), nullif($6, ''), nullif($7, ''),
			nullif($8, '')::uuid, $9
		)
		returning `+noteColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, relationshipID, strings.TrimSpace(input.Content),
		sourceModule, entityType, entityID, scope.ActorUserID, input.IdempotencyKey))
	if err != nil {
		return Note{}, false, err
	}
	if err := insertOutbox(ctx, tx, scope, "note", item.ID, "customer_data.relationship.changed", "note.create:"+input.IdempotencyKey); err != nil {
		return Note{}, false, err
	}
	if err := insertAudit(ctx, tx, scope, "", relationshipID, "create", "note", item.ID, "manual_note"); err != nil {
		return Note{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Note{}, false, err
	}
	return item, false, nil
}

func (r *PostgresRepository) UpdateNote(
	ctx context.Context,
	scope Scope,
	noteID string,
	patch NotePatch,
) (Note, error) {
	item, err := scanNote(r.pool.QueryRow(ctx, `
		update customer_data.relationship_notes
		set content = $4,
		    revision = revision + 1,
		    updated_at = now()
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and id = $3::uuid
		  and revision = $5
		  and archived_at is null
		returning `+noteColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, noteID, strings.TrimSpace(patch.Content), patch.ExpectedRevision))
	if err != nil {
		return Note{}, mapMutationError(err)
	}
	return item, nil
}

func (r *PostgresRepository) ArchiveNote(
	ctx context.Context,
	scope Scope,
	noteID string,
	expectedRevision int64,
) (Note, error) {
	item, err := scanNote(r.pool.QueryRow(ctx, `
		update customer_data.relationship_notes
		set archived_at = now(), revision = revision + 1, updated_at = now()
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and id = $3::uuid
		  and revision = $4
		  and archived_at is null
		returning `+noteColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, noteID, expectedRevision))
	if err != nil {
		return Note{}, mapMutationError(err)
	}
	return item, nil
}

func consentColumns(alias string) string {
	if alias != "" {
		alias += "."
	}
	return strings.Join([]string{
		alias + "id::text", alias + "relationship_id::text", alias + "purpose",
		alias + "channel", alias + "status", alias + "source_module",
		alias + "source_ref", alias + "evidence_hash", alias + "effective_at",
		alias + "expires_at", alias + "created_at",
	}, ", ")
}

func scanConsent(row rowScanner) (Consent, error) {
	var item Consent
	err := row.Scan(
		&item.ID, &item.RelationshipID, &item.Purpose, &item.Channel,
		&item.Status, &item.SourceModule, &item.SourceRef, &item.EvidenceHash,
		&item.EffectiveAt, &item.ExpiresAt, &item.CreatedAt,
	)
	return item, mapDBError(err)
}

func (r *PostgresRepository) ListConsents(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	limit int,
) ([]Consent, error) {
	rows, err := r.pool.Query(ctx, `
		select `+consentColumns("")+`
		from customer_data.relationship_consents
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and relationship_id = $3::uuid
		order by effective_at desc, id desc
		limit $4
	`, scope.AccountID, scope.ClientAccountID, relationshipID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Consent, 0)
	for rows.Next() {
		item, err := scanConsent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) RecordConsent(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	input ConsentInput,
) (Consent, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Consent{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	item, err := scanConsent(tx.QueryRow(ctx, `
		select `+consentColumns("")+`
		from customer_data.relationship_consents
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and idempotency_key = $3
	`, scope.AccountID, scope.ClientAccountID, input.IdempotencyKey))
	if err == nil {
		return item, true, nil
	}
	if err != ErrNotFound {
		return Consent{}, false, err
	}
	item, err = scanConsent(tx.QueryRow(ctx, `
		insert into customer_data.relationship_consents (
			account_id, client_account_id, relationship_id, purpose, channel,
			status, source_module, source_ref, evidence_hash, effective_at,
			expires_at, actor_user_id, idempotency_key
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7,
			$8, $9, $10, $11, nullif($12, '')::uuid, $13
		)
		returning `+consentColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, relationshipID,
		input.Purpose, input.Channel, input.Status, input.SourceModule,
		input.SourceRef, input.EvidenceHash, input.EffectiveAt.UTC(), input.ExpiresAt,
		scope.ActorUserID, input.IdempotencyKey))
	if err != nil {
		return Consent{}, false, err
	}
	if err := insertOutbox(ctx, tx, scope, "consent", item.ID, "customer_data.consent.changed", "consent.create:"+input.IdempotencyKey); err != nil {
		return Consent{}, false, err
	}
	if err := insertAudit(ctx, tx, scope, "", relationshipID, "append", "consent", item.ID, "consent_event"); err != nil {
		return Consent{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Consent{}, false, err
	}
	return item, false, nil
}
