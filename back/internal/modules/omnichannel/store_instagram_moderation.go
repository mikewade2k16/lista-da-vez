package omnichannel

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// IsInstagramModeratedMessage identifies comment/mention messages from the
// canonical messages table. It is deliberately account-scoped so an AI job
// cannot use a message id from another tenant to bypass moderation.
func (s *Store) IsInstagramModeratedMessage(ctx context.Context, accountID, messageID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `select c.channel='INSTAGRAM'
			and coalesce(m.metadata_json->>'socialEventKind','') in ('comment','mention')
		from messaging.messages m
		join messaging.conversations c
		  on c.account_id=m.account_id and c.id=m.conversation_id
		where m.account_id=$1::uuid and m.id=$2::uuid`, accountID, messageID).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	return ok, err
}

// SaveInstagramAIDraft keeps the model's suggestion in the moderation action;
// it never creates an outbox message and therefore cannot publish a comment.
func (s *Store) SaveInstagramAIDraft(ctx context.Context, accountID, messageID, draft string) error {
	draft = strings.TrimSpace(draft)
	if len([]rune(draft)) > 4000 {
		draft = string([]rune(draft)[:4000])
	}
	_, err := s.pool.Exec(ctx, `update messaging.instagram_comment_actions a set proposed_text=nullif($3,''),updated_at=now()
		from messaging.instagram_comments c, messaging.messages m
		where a.account_id=$1::uuid and c.account_id=a.account_id and c.id=a.comment_id
		and m.account_id=c.account_id and m.id=$2::uuid
		and c.external_comment_id=m.external_message_id
		and a.status='pending_review'`, accountID, messageID, draft)
	return err
}
