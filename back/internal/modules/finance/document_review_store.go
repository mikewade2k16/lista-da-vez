package finance

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *DocumentStore) ReviewApproval(ctx context.Context, account, user, id, hash string) (string, string, error) {
	return readDocumentReviewApproval(ctx, s.pool, account, user, id, hash)
}
func readDocumentReviewApproval(ctx context.Context, q fiscalQuerier, account, user, id, hash string) (string, string, error) {
	var approvalID string
	var approvedAt time.Time
	err := q.QueryRow(ctx, `select id::text,approved_at from finance.document_reviews where account_id=$1::uuid and user_id=$2::uuid and draft_id=$3::uuid and review_hash=$4 and revoked_at is null`, account, user, id, hash).Scan(&approvalID, &approvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	return approvalID, approvedAt.UTC().Format(time.RFC3339Nano), nil
}
func (s *DocumentStore) ApproveReview(ctx context.Context, account, user, id string, review DocumentReview) (string, string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockFiscalScope(ctx, tx, account, user); err != nil {
		return "", "", err
	}
	var updated time.Time
	err = tx.QueryRow(ctx, `select updated_at from finance.document_drafts where account_id=$1::uuid and user_id=$2::uuid and id=$3::uuid for update`, account, user, id).Scan(&updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrFiscalNotFound
	}
	if err != nil {
		return "", "", err
	}
	profile, err := readFiscalProfile(ctx, tx, account, user)
	if err != nil {
		return "", "", err
	}
	if updated.UTC().Format(time.RFC3339Nano) != review.DraftVersion || profile.UpdatedAt != review.ProfileVersion {
		return "", "", ErrFiscalConflict
	}
	approvalID, approvedAt, err := readDocumentReviewApproval(ctx, tx, account, user, id, review.Hash)
	if err != nil {
		return "", "", err
	}
	if approvalID != "" {
		return approvalID, approvedAt, tx.Commit(ctx)
	}
	snapshot, err := json.Marshal(review)
	if err != nil {
		return "", "", err
	}
	var date time.Time
	err = tx.QueryRow(ctx, `insert into finance.document_reviews(account_id,user_id,draft_id,review_hash,draft_version,profile_version,snapshot) values($1::uuid,$2::uuid,$3::uuid,$4,$5::timestamptz,$6::timestamptz,$7::jsonb) returning id::text,approved_at`, account, user, id, review.Hash, review.DraftVersion, review.ProfileVersion, snapshot).Scan(&approvalID, &date)
	if err != nil {
		return "", "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return approvalID, date.UTC().Format(time.RFC3339Nano), nil
}
func (s *DocumentStore) RevokeReview(ctx context.Context, account, user, id, hash string) error {
	_, err := s.pool.Exec(ctx, `update finance.document_reviews set revoked_at=clock_timestamp() where account_id=$1::uuid and user_id=$2::uuid and draft_id=$3::uuid and review_hash=$4 and revoked_at is null`, account, user, id, hash)
	return err
}
