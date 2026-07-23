package omnichannel

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const operatorReplyDispatchPrefix = "operator-reply:"

// ReplayAutomationWithAI atomically closes the handoff, returns the conversation
// to the AI lease and schedules the already persisted unanswered inbound messages.
// It never calls n8n or a channel directly.
func (s *Store) ReplayAutomationWithAI(ctx context.Context, accountID, conversationID, actorID, actionKey string) (AIDispatchRecord, error) {
	accountID = strings.TrimSpace(accountID)
	conversationID = strings.TrimSpace(conversationID)
	actionKey = strings.TrimSpace(actionKey)
	if accountID == "" || conversationID == "" || actionKey == "" {
		return AIDispatchRecord{}, ErrAIDispatchInvalidInput
	}
	idempotencyKey := operatorReplyDispatchPrefix + actionKey
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AIDispatchRecord{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := scanAIDispatch(tx.QueryRow(ctx, `select `+aiDispatchColumns+`
		from messaging.ai_dispatches where account_id=$1::uuid and idempotency_key=$2`,
		accountID, idempotencyKey))
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return AIDispatchRecord{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return AIDispatchRecord{}, err
	}

	snap, err := lockConversationSnapshotTx(ctx, tx, accountID, conversationID)
	if err != nil {
		return AIDispatchRecord{}, err
	}
	if !automationReplyStateAllowed(snap.State) {
		return AIDispatchRecord{}, ErrConflict
	}

	var agentVersionID string
	err = tx.QueryRow(ctx, `select aa.active_version_id::text
		from messaging.automation_profiles p
		join messaging.whatsapp_instances wi
		  on wi.account_id=p.account_id and wi.id=p.whatsapp_instance_id
		join messaging.ai_agents aa
		  on aa.account_id=p.account_id and aa.id=p.ai_agent_id
		join messaging.ai_agent_versions av
		  on av.account_id=aa.account_id and av.agent_id=aa.id and av.id=aa.active_version_id
		where p.account_id=$1::uuid and p.whatsapp_instance_id=$2::uuid
		  and p.enabled and wi.is_active and aa.enabled
		  and aa.provider_key_ciphertext <> '' and av.provider <> '' and av.model <> ''`,
		accountID, snap.InstanceID).Scan(&agentVersionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return AIDispatchRecord{}, ErrAutomationNotReady
	}
	if err != nil {
		return AIDispatchRecord{}, err
	}

	var messageIDs []string
	err = tx.QueryRow(ctx, `select coalesce(array_agg(m.id order by m.created_at,m.id),'{}'::uuid[])::text[]
		from messaging.messages m
		where m.account_id=$1::uuid and m.conversation_id=$2::uuid and m.direction='INBOUND'
		  and m.created_at > coalesce((select suppression.history_cleared_at
		      from messaging.conversations conversation
		      join messaging.contact_suppressions suppression
		        on suppression.account_id=conversation.account_id and suppression.contact_id=conversation.contact_id
		      where conversation.account_id=m.account_id and conversation.id=m.conversation_id),
		      '-infinity'::timestamptz)
		  and m.created_at > coalesce((
			select max(answer.created_at) from messaging.messages answer
			where answer.account_id=$1::uuid and answer.conversation_id=$2::uuid
			  and answer.direction='OUTBOUND' and answer.status <> 'FAILED'
		  ),'-infinity'::timestamptz)`, accountID, conversationID).Scan(&messageIDs)
	if err != nil {
		return AIDispatchRecord{}, err
	}
	if len(messageIDs) == 0 {
		return AIDispatchRecord{}, ErrAutomationNoUnansweredInput
	}

	newGeneration := snap.AIGeneration + 1
	if err := applyStateUpdateTx(ctx, tx, accountID, conversationID, stateUpdate{
		State: StateAIActive, QueueID: nil, DepartmentID: nil, AssignedUserID: nil,
		InvalidateAI: true, CloseHandoffs: true,
	}, s.AIDispatchV2Enabled()); err != nil {
		return AIDispatchRecord{}, err
	}
	runAfter := time.Now().UTC()
	created, err := scanAIDispatch(tx.QueryRow(ctx, `insert into messaging.ai_dispatches
		(account_id,conversation_id,agent_version_id,generation,status,message_ids,run_after,idempotency_key)
		values ($1::uuid,$2::uuid,$3::uuid,$4,'buffering',$5::uuid[],$6,$7)
		returning `+aiDispatchColumns, accountID, conversationID, agentVersionID,
		newGeneration, messageIDs, runAfter, idempotencyKey))
	if err != nil {
		return AIDispatchRecord{}, err
	}
	if err := enqueueAIDispatchJobTx(ctx, tx, accountID, created.ID, conversationID, newGeneration, runAfter); err != nil {
		return AIDispatchRecord{}, err
	}
	if _, err := tx.Exec(ctx, `insert into messaging.audit_events
		(account_id,actor_user_id,conversation_id,event_type,payload_json)
		values ($1::uuid,nullif($2,'')::uuid,$3::uuid,'CONVERSATION_STATUS_CHANGED',
		jsonb_build_object('action','operator_reply_with_ai','to','ai_active','dispatchId',$4::text))`,
		accountID, actorID, conversationID, created.ID); err != nil {
		return AIDispatchRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AIDispatchRecord{}, err
	}
	return created, nil
}

func automationReplyStateAllowed(state ConversationState) bool {
	return state != StateAIActive && state != StateClosed
}

func isOperatorReplyDispatch(dispatch AIDispatchRecord) bool {
	return strings.HasPrefix(dispatch.IdempotencyKey, operatorReplyDispatchPrefix)
}
