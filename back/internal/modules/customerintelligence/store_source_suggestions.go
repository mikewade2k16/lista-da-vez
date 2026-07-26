package customerintelligence

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresRepository) ListSourceSuggestions(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	limit int,
) ([]SourceSuggestion, error) {
	rows, err := s.pool.Query(ctx, `
		select id, client_account_id, relationship_id, source_key, gap_codes,
		       rationale_code, confidence, status, expires_at, created_at,
		       rationale_ciphertext
		  from intelligence.source_suggestions
		 where account_id = $1
		   and client_account_id = $2
		   and relationship_id = $3
		 order by created_at desc, id desc
		 limit $4`,
		scope.AccountID,
		scope.ClientAccountID,
		relationshipID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SourceSuggestion, 0)
	for rows.Next() {
		var item SourceSuggestion
		var gapCodes []byte
		if err := rows.Scan(
			&item.ID,
			&item.ClientAccountID,
			&item.RelationshipID,
			&item.SourceKey,
			&gapCodes,
			&item.RationaleCode,
			&item.Confidence,
			&item.Status,
			&item.ExpiresAt,
			&item.CreatedAt,
			&item.RationaleCiphertext,
		); err != nil {
			return nil, err
		}
		item.GapCodes = decodeStrings(gapCodes)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresRepository) ReviewSourceSuggestion(
	ctx context.Context,
	scope Scope,
	actorID, suggestionID string,
	input SourceSuggestionFeedback,
) (SourceSuggestion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SourceSuggestion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var item SourceSuggestion
	var gapCodes []byte
	err = tx.QueryRow(ctx, `
		update intelligence.source_suggestions
		   set status = $4,
		       review_reason_code = $5,
		       reviewed_by_user_id = $6,
		       reviewed_at = now()
		 where account_id = $1
		   and client_account_id = $2
		   and id = $3
		   and status = 'proposed'
		   and (expires_at is null or expires_at > now())
		returning id, client_account_id, relationship_id, source_key, gap_codes,
		          rationale_code, confidence, status, expires_at, created_at,
		          rationale_ciphertext`,
		scope.AccountID,
		scope.ClientAccountID,
		suggestionID,
		input.Status,
		input.Reason,
		actorID,
	).Scan(
		&item.ID,
		&item.ClientAccountID,
		&item.RelationshipID,
		&item.SourceKey,
		&gapCodes,
		&item.RationaleCode,
		&item.Confidence,
		&item.Status,
		&item.ExpiresAt,
		&item.CreatedAt,
		&item.RationaleCiphertext,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return SourceSuggestion{}, ErrConflict
	}
	if err != nil {
		return SourceSuggestion{}, repositoryError(err)
	}
	item.GapCodes = decodeStrings(gapCodes)
	metadata, err := json.Marshal(map[string]any{
		"sourceKey": item.SourceKey,
		"status":    item.Status,
	})
	if err != nil {
		return SourceSuggestion{}, err
	}
	if _, err := tx.Exec(ctx, `
		insert into intelligence.audit_events (
		    account_id, client_account_id, actor_user_id, event_type,
		    aggregate_type, aggregate_id, reason_code, metadata
		)
		values (
		    $1, $2, $3, 'source_suggestion.reviewed',
		    'source_suggestion', $4, $5, $6
		)`,
		scope.AccountID,
		scope.ClientAccountID,
		actorID,
		item.ID,
		input.Reason,
		metadata,
	); err != nil {
		return SourceSuggestion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SourceSuggestion{}, err
	}
	return item, nil
}
