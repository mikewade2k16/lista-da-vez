package customerdata

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

func offlineColumns(alias string) string {
	if alias != "" {
		alias += "."
	}
	return strings.Join([]string{
		alias + "id::text", alias + "relationship_id::text", alias + "interaction_type",
		alias + "occurred_at", alias + "timezone", alias + "duration_seconds",
		alias + "title", alias + "content_sanitized", alias + "content_ciphertext",
		alias + "cipher_key_version", alias + "sensitivity", alias + "purpose_key",
		alias + "source_external_ref", alias + "status", alias + "revision",
		alias + "created_at", alias + "updated_at",
	}, ", ")
}

func scanOffline(row rowScanner, reveal func(string, string) (string, error)) (OfflineInteraction, error) {
	var item OfflineInteraction
	var sanitized, ciphertext, keyVersion *string
	err := row.Scan(
		&item.ID, &item.RelationshipID, &item.InteractionType, &item.OccurredAt,
		&item.Timezone, &item.DurationSeconds, &item.Title, &sanitized, &ciphertext,
		&keyVersion, &item.Sensitivity, &item.PurposeKey, &item.SourceExternalRef,
		&item.Status, &item.Revision, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return OfflineInteraction{}, mapDBError(err)
	}
	if sanitized != nil {
		item.Content = sanitized
	} else if ciphertext != nil && keyVersion != nil && reveal != nil {
		value, err := reveal(*ciphertext, *keyVersion)
		if err != nil {
			return OfflineInteraction{}, err
		}
		item.Content = &value
	}
	return item, nil
}

func (r *PostgresRepository) ListOfflineInteractions(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	limit int,
	reveal func(string, string) (string, error),
) ([]OfflineInteraction, error) {
	rows, err := r.pool.Query(ctx, `
		select `+offlineColumns("")+`
		from customer_data.offline_interactions
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and relationship_id = $3::uuid
		  and status = 'active'
		order by occurred_at desc, id desc
		limit $4
	`, scope.AccountID, scope.ClientAccountID, relationshipID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]OfflineInteraction, 0)
	for rows.Next() {
		item, err := scanOffline(rows, reveal)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) CreateOfflineInteraction(
	ctx context.Context,
	scope Scope,
	input OfflineInteractionInput,
	ciphertext, keyVersion string,
) (OfflineInteraction, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return OfflineInteraction{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	item, err := scanOffline(tx.QueryRow(ctx, `
		select `+offlineColumns("")+`
		from customer_data.offline_interactions
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and idempotency_key = $3
	`, scope.AccountID, scope.ClientAccountID, input.IdempotencyKey), nil)
	if err == nil {
		return item, true, nil
	}
	if err != ErrNotFound {
		return OfflineInteraction{}, false, err
	}
	sanitized := any(nil)
	if input.Sensitivity == "public" || input.Sensitivity == "internal" {
		sanitized = nullIfEmpty(strings.TrimSpace(input.Content))
	}
	item, err = scanOffline(tx.QueryRow(ctx, `
		insert into customer_data.offline_interactions (
			account_id, client_account_id, relationship_id, interaction_type,
			occurred_at, timezone, duration_seconds, title, content_sanitized,
			content_ciphertext, cipher_key_version, sensitivity, purpose_key,
			source_external_ref, idempotency_key, created_by_user_id, updated_by_user_id
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9,
			nullif($10, ''), nullif($11, ''), $12, $13, $14, $15,
			nullif($16, '')::uuid, nullif($16, '')::uuid
		)
		returning `+offlineColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, input.RelationshipID, input.InteractionType,
		input.OccurredAt.UTC(), input.Timezone, input.DurationSeconds, strings.TrimSpace(input.Title),
		sanitized, ciphertext, keyVersion, input.Sensitivity, input.PurposeKey,
		input.SourceExternalRef, input.IdempotencyKey, scope.ActorUserID), nil)
	if err != nil {
		return OfflineInteraction{}, false, err
	}
	if err := insertOutbox(ctx, tx, scope, "offline_interaction", item.ID, "customer_data.offline_interaction.changed", "offline.create:"+input.IdempotencyKey); err != nil {
		return OfflineInteraction{}, false, err
	}
	if err := insertAudit(ctx, tx, scope, "", input.RelationshipID, "create", "offline_interaction", item.ID, "offline_ingest"); err != nil {
		return OfflineInteraction{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OfflineInteraction{}, false, err
	}
	if input.Sensitivity == "public" || input.Sensitivity == "internal" {
		content := input.Content
		item.Content = &content
	}
	return item, false, nil
}

func (r *PostgresRepository) UpdateOfflineInteraction(
	ctx context.Context,
	scope Scope,
	interactionID string,
	patch OfflineInteractionPatch,
	ciphertext, keyVersion string,
) (OfflineInteraction, error) {
	publicContent := any(nil)
	if patch.Content != nil && patch.Sensitivity != nil &&
		(*patch.Sensitivity == "public" || *patch.Sensitivity == "internal") {
		publicContent = nullIfEmpty(strings.TrimSpace(*patch.Content))
	}
	row := r.pool.QueryRow(ctx, `
		update customer_data.offline_interactions
		set interaction_type = case when $4::boolean then $5 else interaction_type end,
		    occurred_at = case when $6::boolean then $7 else occurred_at end,
		    timezone = case when $8::boolean then $9 else timezone end,
		    duration_seconds = case when $10::boolean then $11 else duration_seconds end,
		    title = case when $12::boolean then $13 else title end,
		    sensitivity = case when $14::boolean then $15 else sensitivity end,
		    purpose_key = case when $16::boolean then $17 else purpose_key end,
		    content_sanitized = case when $18::boolean then $19 else content_sanitized end,
		    content_ciphertext = case when $18::boolean then nullif($20, '') else content_ciphertext end,
		    cipher_key_version = case when $18::boolean then nullif($21, '') else cipher_key_version end,
		    revision = revision + 1,
		    updated_by_user_id = nullif($22, '')::uuid,
		    updated_at = now()
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and id = $3::uuid
		  and revision = $23
		  and status = 'active'
		returning `+offlineColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, interactionID,
		patch.InteractionType != nil, pointerString(patch.InteractionType),
		patch.OccurredAt != nil, patch.OccurredAt,
		patch.Timezone != nil, pointerString(patch.Timezone),
		patch.DurationSeconds != nil, patch.DurationSeconds,
		patch.Title != nil, pointerString(patch.Title),
		patch.Sensitivity != nil, pointerString(patch.Sensitivity),
		patch.PurposeKey != nil, pointerString(patch.PurposeKey),
		patch.Content != nil, publicContent, ciphertext, keyVersion,
		scope.ActorUserID, patch.ExpectedRevision)
	item, err := scanOffline(row, nil)
	if err != nil {
		return OfflineInteraction{}, mapMutationError(err)
	}
	if patch.Content != nil && patch.Sensitivity != nil &&
		(*patch.Sensitivity == "public" || *patch.Sensitivity == "internal") {
		content := *patch.Content
		item.Content = &content
	}
	return item, nil
}

func (r *PostgresRepository) ArchiveOfflineInteraction(
	ctx context.Context,
	scope Scope,
	interactionID string,
	expectedRevision int64,
) (OfflineInteraction, error) {
	item, err := scanOffline(r.pool.QueryRow(ctx, `
		update customer_data.offline_interactions
		set status = 'archived', revision = revision + 1,
		    updated_by_user_id = nullif($5, '')::uuid, updated_at = now()
		where account_id = $1::uuid
		  and client_account_id = $2::uuid
		  and id = $3::uuid
		  and revision = $4
		  and status = 'active'
		returning `+offlineColumns("")+`
	`, scope.AccountID, scope.ClientAccountID, interactionID, expectedRevision, scope.ActorUserID), nil)
	if err != nil {
		return OfflineInteraction{}, mapMutationError(err)
	}
	return item, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
