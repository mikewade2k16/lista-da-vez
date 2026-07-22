package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) UpsertInstagramAccount(ctx context.Context, accountID string, in InstagramAccountInput, config map[string]string, ciphertext string) (InstagramAccountView, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return InstagramAccountView{}, err
	}
	var id string
	err = s.pool.QueryRow(ctx, `insert into messaging.instagram_accounts
		(account_id,ig_user_id,username,display_name,page_id,provider_config,credentials_ciphertext)
		values ($1::uuid,$2,nullif($3,''),nullif($4,''),nullif($5,''),$6::jsonb,nullif($7,''))
		on conflict (account_id,ig_user_id) do update set username=excluded.username,display_name=excluded.display_name,page_id=excluded.page_id,
			provider_config=excluded.provider_config,credentials_ciphertext=coalesce(excluded.credentials_ciphertext,instagram_accounts.credentials_ciphertext),updated_at=now()
		returning id::text`, accountID, strings.TrimSpace(in.IGUserID), strings.TrimSpace(in.Username), strings.TrimSpace(in.DisplayName), strings.TrimSpace(in.PageID), raw, ciphertext).Scan(&id)
	if err != nil {
		return InstagramAccountView{}, err
	}
	return s.GetInstagramAccount(ctx, accountID, id)
}

func (s *Store) EnqueueInstagramAction(ctx context.Context, accountID, commentID, actionID string) error {
	payload, _ := json.Marshal(instagramActionJobPayload{CommentID: commentID, ActionID: actionID})
	_, err := s.pool.Exec(ctx, `insert into messaging.outbox(account_id,ordering_key,idempotency_key,kind,payload,max_attempts)
		values($1::uuid,$2,$3,$4,$5::jsonb,5) on conflict(account_id,idempotency_key) do nothing`, accountID, commentID, "instagram-action:"+actionID, InstagramActionJobKind, payload)
	return err
}

func (s *Store) ClaimInstagramAction(ctx context.Context, accountID, commentID, actionID string) (claimedInstagramAction, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return claimedInstagramAction{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var out claimedInstagramAction
	err = tx.QueryRow(ctx, `select a.status,a.action_kind,coalesce(c.external_comment_id,''),coalesce(a.approved_text,a.proposed_text,'') from messaging.instagram_comment_actions a join messaging.instagram_comments c on c.account_id=a.account_id and c.id=a.comment_id where a.account_id=$1::uuid and a.comment_id=$2::uuid and a.id=$3::uuid for update`, accountID, commentID, actionID).Scan(&out.Status, &out.ActionKind, &out.ExternalCommentID, &out.Text)
	if errors.Is(err, pgx.ErrNoRows) {
		return claimedInstagramAction{}, ErrNotFound
	}
	if err != nil {
		return claimedInstagramAction{}, err
	}
	if out.Status == "sent" || out.Status == "ignored" {
		if err := tx.Commit(ctx); err != nil {
			return claimedInstagramAction{}, err
		}
		return out, nil
	}
	if out.Status != "approved" {
		return claimedInstagramAction{}, ErrConflict
	}
	if _, err = tx.Exec(ctx, `update messaging.instagram_comment_actions set status='processing' where account_id=$1::uuid and id=$2::uuid`, accountID, actionID); err != nil {
		return claimedInstagramAction{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return claimedInstagramAction{}, err
	}
	out.Status = "processing"
	return out, nil
}

func (s *Store) MarkInstagramActionSent(ctx context.Context, accountID, actionID, externalID string) error {
	_, err := s.pool.Exec(ctx, `update messaging.instagram_comment_actions set status='sent',external_message_id=nullif($3,''),last_error='',executed_at=now() where account_id=$1::uuid and id=$2::uuid and status='processing'`, accountID, actionID, externalID)
	return err
}

func (s *Store) GetInstagramAccount(ctx context.Context, accountID, id string) (InstagramAccountView, error) {
	var out InstagramAccountView
	var cipher *string
	err := s.pool.QueryRow(ctx, `select id::text,ig_user_id,username,display_name,page_id,is_active,webhook_status,credentials_ciphertext,updated_at
		from messaging.instagram_accounts where account_id=$1::uuid and id=$2::uuid`, accountID, id).Scan(&out.ID, &out.IGUserID, &out.Username, &out.DisplayName, &out.PageID, &out.IsActive, &out.WebhookStatus, &cipher, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return InstagramAccountView{}, ErrNotFound
	}
	if err != nil {
		return InstagramAccountView{}, err
	}
	out.CredentialsSet = cipher != nil && strings.TrimSpace(*cipher) != ""
	return out, nil
}

func (s *Store) ListInstagramAccounts(ctx context.Context, accountID string) ([]InstagramAccountView, error) {
	rows, err := s.pool.Query(ctx, `select id::text,ig_user_id,username,display_name,page_id,is_active,webhook_status,credentials_ciphertext,updated_at from messaging.instagram_accounts where account_id=$1::uuid order by username,ig_user_id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]InstagramAccountView, 0)
	for rows.Next() {
		var item InstagramAccountView
		var cipher *string
		if err := rows.Scan(&item.ID, &item.IGUserID, &item.Username, &item.DisplayName, &item.PageID, &item.IsActive, &item.WebhookStatus, &cipher, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.CredentialsSet = cipher != nil && strings.TrimSpace(*cipher) != ""
		out = append(out, item)
	}
	return out, rows.Err()
}

const instagramCommentCols = `c.id::text,c.instagram_account_id::text,c.external_comment_id,c.external_media_id,c.parent_comment_id,c.contact_id::text,c.author_scoped_id,c.username,c.text,c.event_kind,c.status,c.is_live,c.occurred_at,c.metadata,c.created_at,c.updated_at`

func scanInstagramComment(row rowScanner) (InstagramCommentView, error) {
	var out InstagramCommentView
	err := row.Scan(&out.ID, &out.AccountID, &out.ExternalID, &out.MediaID, &out.ParentID, &out.ContactID, &out.AuthorScopedID, &out.Username, &out.Text, &out.EventKind, &out.Status, &out.IsLive, &out.OccurredAt, &out.Metadata, &out.CreatedAt, &out.UpdatedAt)
	if len(out.Metadata) == 0 {
		out.Metadata = json.RawMessage(`{}`)
	}
	return out, err
}

func (s *Store) ListInstagramComments(ctx context.Context, accountID, accountRef string, limit int) ([]InstagramCommentView, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `select `+instagramCommentCols+` from messaging.instagram_comments c where c.account_id=$1::uuid and ($2='' or c.instagram_account_id=$2::uuid) order by c.occurred_at desc,c.id desc limit $3`, accountID, strings.TrimSpace(accountRef), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]InstagramCommentView, 0)
	for rows.Next() {
		item, e := scanInstagramComment(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ListInstagramActions(ctx context.Context, accountID, commentID string) ([]InstagramCommentActionView, error) {
	rows, err := s.pool.Query(ctx, `select id::text,comment_id::text,action_kind,status,proposed_text,approved_text,approved_by_user_id::text,external_message_id,idempotency_key,private_reply_expires_at,last_error,created_at,executed_at from messaging.instagram_comment_actions where account_id=$1::uuid and comment_id=$2::uuid order by created_at desc`, accountID, commentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]InstagramCommentActionView, 0)
	for rows.Next() {
		var item InstagramCommentActionView
		if err := rows.Scan(&item.ID, &item.CommentID, &item.ActionKind, &item.Status, &item.ProposedText, &item.ApprovedText, &item.ApprovedByUserID, &item.ExternalMessageID, &item.IdempotencyKey, &item.PrivateReplyExpiresAt, &item.LastError, &item.CreatedAt, &item.ExecutedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) DecideInstagramAction(ctx context.Context, accountID, commentID, actionID, actorID string, in InstagramActionDecisionInput) (InstagramCommentActionView, error) {
	if in.ActionKind != "public_reply" && in.ActionKind != "private_reply" && in.ActionKind != "hide" && in.ActionKind != "ignore" {
		return InstagramCommentActionView{}, ErrInvalidBody
	}
	if len([]rune(in.ApprovedText)) > 4000 {
		return InstagramCommentActionView{}, ErrInvalidBody
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return InstagramCommentActionView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var expires *time.Time
	var status string
	var currentKind string
	err = tx.QueryRow(ctx, `select a.private_reply_expires_at,a.status,a.action_kind from messaging.instagram_comment_actions a join messaging.instagram_comments c on c.account_id=a.account_id and c.id=a.comment_id where a.account_id=$1::uuid and a.comment_id=$2::uuid and a.id=$3::uuid for update`, accountID, commentID, actionID).Scan(&expires, &status, &currentKind)
	if errors.Is(err, pgx.ErrNoRows) {
		return InstagramCommentActionView{}, ErrNotFound
	}
	if err != nil {
		return InstagramCommentActionView{}, err
	}
	if status != "pending_review" {
		return InstagramCommentActionView{}, ErrConflict
	}
	if in.ActionKind == "private_reply" && expires != nil && expires.Before(time.Now()) {
		return InstagramCommentActionView{}, ErrConflict
	}
	approvedText := strings.TrimSpace(in.ApprovedText)
	next := "approved"
	if in.ActionKind == "ignore" {
		next = "ignored"
	}
	if approvedText == "" && in.ActionKind != "hide" && in.ActionKind != "ignore" {
		return InstagramCommentActionView{}, ErrInvalidBody
	}
	_, err = tx.Exec(ctx, `update messaging.instagram_comment_actions set action_kind=$4,status=$5,approved_text=nullif($6,''),approved_by_user_id=$7::uuid,approved_at=now() where account_id=$1::uuid and comment_id=$2::uuid and id=$3::uuid`, accountID, commentID, actionID, in.ActionKind, next, approvedText, actorID)
	if err != nil {
		return InstagramCommentActionView{}, err
	}
	if _, err = tx.Exec(ctx, `insert into messaging.audit_events(account_id,actor_user_id,event_type,payload_json) values($1::uuid,$2::uuid,'INSTAGRAM_COMMENT_ACTION_DECIDED',jsonb_build_object('commentId',$3::text,'actionId',$4::text,'actionKind',$5::text,'status',$6::text))`, accountID, actorID, commentID, actionID, in.ActionKind, next); err != nil {
		return InstagramCommentActionView{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return InstagramCommentActionView{}, err
	}
	var out InstagramCommentActionView
	err = s.pool.QueryRow(ctx, `select id::text,comment_id::text,action_kind,status,proposed_text,approved_text,approved_by_user_id::text,external_message_id,idempotency_key,private_reply_expires_at,last_error,created_at,executed_at from messaging.instagram_comment_actions where account_id=$1::uuid and comment_id=$2::uuid and id=$3::uuid`, accountID, commentID, actionID).Scan(&out.ID, &out.CommentID, &out.ActionKind, &out.Status, &out.ProposedText, &out.ApprovedText, &out.ApprovedByUserID, &out.ExternalMessageID, &out.IdempotencyKey, &out.PrivateReplyExpiresAt, &out.LastError, &out.CreatedAt, &out.ExecutedAt)
	return out, err
}
