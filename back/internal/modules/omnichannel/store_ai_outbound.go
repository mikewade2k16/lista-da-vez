package omnichannel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// CreateAIOutboundMessage grava mensagem e outbox na mesma transacao protegida pela
// ai_generation da conversa. Assim um takeover nunca pode acontecer entre o check da
// lease, a mensagem e o enqueue.
func (s *Store) CreateAIOutboundMessage(ctx context.Context, accountID, conversationID,
	content, runID, idempotencyKey string, generation int64) (MessageView, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return MessageView{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var instanceID *string
	var instanceScopeKey, state string
	var currentGeneration int64
	err = tx.QueryRow(ctx, `select instance_id::text, instance_scope_key, state, ai_generation
		from messaging.conversations
		where account_id = $1::uuid and id = $2::uuid
		for update`, accountID, conversationID).Scan(&instanceID, &instanceScopeKey, &state, &currentGeneration)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MessageView{}, false, ErrNotFound
		}
		return MessageView{}, false, err
	}
	if state != string(StateAIActive) || currentGeneration != generation {
		return MessageView{}, false, ErrAILeaseInvalid
	}
	allowed, err := aiOutboundAllowedTx(ctx, tx, accountID, conversationID)
	if err != nil {
		return MessageView{}, false, err
	}
	if !allowed {
		return MessageView{}, false, ErrAILeaseInvalid
	}
	view, created, err := createAIOutboundMessageLockedTx(ctx, tx, accountID, conversationID,
		instanceID, instanceScopeKey, content, runID, idempotencyKey, generation)
	if err != nil {
		return MessageView{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MessageView{}, false, err
	}
	return view, created, nil
}

func createAIOutboundMessageLockedTx(ctx context.Context, tx pgx.Tx, accountID, conversationID string,
	instanceID *string, instanceScopeKey, content, runID, idempotencyKey string, generation int64) (MessageView, bool, error) {
	var existingMessageID string
	err := tx.QueryRow(ctx, `select payload->>'messageId' from messaging.outbox
		where account_id = $1::uuid and idempotency_key = $2`, accountID, idempotencyKey).Scan(&existingMessageID)
	if err == nil && existingMessageID != "" {
		view, scanErr := scanMessage(tx.QueryRow(ctx, `select `+messageCols+` from messaging.messages m
			where m.account_id = $1::uuid and m.id = $2::uuid`, accountID, existingMessageID))
		if scanErr != nil {
			return MessageView{}, false, scanErr
		}
		return view, false, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return MessageView{}, false, err
	}

	metadata, _ := json.Marshal(map[string]any{
		"source": "ai", "aiRunId": runID, "aiGeneration": generation,
	})
	var messageID string
	err = tx.QueryRow(ctx, `insert into messaging.messages
		(account_id, conversation_id, instance_id, instance_scope_key, direction,
		 message_type, content, metadata_json, status, origin)
		values ($1::uuid, $2::uuid, nullif($3, '')::uuid, $4, 'OUTBOUND',
		 'TEXT', $5, $6::jsonb, 'PENDING', 'ai')
		returning id::text`, accountID, conversationID, deref(instanceID), instanceScopeKey,
		content, metadata).Scan(&messageID)
	if err != nil {
		return MessageView{}, false, err
	}
	payload, _ := json.Marshal(outboundJobPayload{MessageID: messageID, ConversationID: conversationID})
	if _, err := tx.Exec(ctx, `insert into messaging.outbox
		(account_id, ordering_key, idempotency_key, kind, payload, max_attempts)
		values ($1::uuid, $2::text, $3, $4, $5::jsonb, 5)`, accountID,
		conversationID, idempotencyKey, OutboundJobKind, payload); err != nil {
		return MessageView{}, false, err
	}
	if _, err := tx.Exec(ctx, `update messaging.conversations
		set last_message_at = now(), updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`, accountID, conversationID); err != nil {
		return MessageView{}, false, err
	}
	view, err := scanMessage(tx.QueryRow(ctx, `select `+messageCols+` from messaging.messages m
		where m.account_id = $1::uuid and m.id = $2::uuid`, accountID, messageID))
	if err != nil {
		return MessageView{}, false, err
	}
	return view, true, nil
}
