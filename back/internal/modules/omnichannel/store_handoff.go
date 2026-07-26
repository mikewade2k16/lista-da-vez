package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

const handoffColumns = `id::text, conversation_id::text, ai_run_id::text, routing_decision_id::text,
	policy_id::text, policy_snapshot, reason_code, summary, collected_fields, source_state, target_queue_id::text, status,
	accepted_by_user_id::text, requested_at, queued_at, accepted_at, closed_at, created_at, updated_at`

func scanHandoff(row rowScanner) (HandoffView, error) {
	var out HandoffView
	err := row.Scan(&out.ID, &out.ConversationID, &out.AIRunID, &out.RoutingDecisionID,
		&out.PolicyID, &out.PolicySnapshot, &out.ReasonCode, &out.Summary, &out.CollectedFields, &out.SourceState, &out.TargetQueueID,
		&out.Status, &out.AcceptedByUserID, &out.RequestedAt, &out.QueuedAt, &out.AcceptedAt,
		&out.ClosedAt, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (s *Store) GetOpenHandoff(ctx context.Context, accountID, conversationID string) (HandoffView, error) {
	row, err := scanHandoff(s.pool.QueryRow(ctx, `select `+handoffColumns+` from messaging.handoffs
		where account_id=$1::uuid and conversation_id=$2::uuid
		  and status in ('requested','queued','accepted') order by created_at desc, id desc limit 1`, accountID, conversationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return HandoffView{}, ErrNotFound
	}
	return row, err
}

func (s *Store) ListHandoffs(ctx context.Context, accountID, conversationID string) ([]HandoffView, error) {
	rows, err := s.pool.Query(ctx, `select `+handoffColumns+` from messaging.handoffs
		where account_id=$1::uuid and conversation_id=$2::uuid order by created_at desc, id desc`, accountID, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]HandoffView, 0)
	for rows.Next() {
		row, err := scanHandoff(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// CreateHandoff enfileira o snapshot sob lock da conversa. O retorno de uma linha
// existente é um replay idempotente. A policy é avaliada no mesmo lock/transação;
// n8n não escolhe fila nem grava estado.
func (s *Store) CreateHandoff(ctx context.Context, accountID, conversationID, actorID string, in HandoffRequest) (HandoffView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return HandoffView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	snap, err := lockConversationSnapshotTx(ctx, tx, accountID, conversationID)
	if err != nil {
		return HandoffView{}, err
	}
	if snap.State == StateClosed || snap.State == StateHumanActive || snap.State == StatePending {
		return HandoffView{}, ErrConflict
	}
	fields := in.CollectedFields
	if len(fields) == 0 || string(fields) == "null" {
		fields = json.RawMessage(`{}`)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(fields, &object); err != nil || object == nil {
		return HandoffView{}, ErrInvalidBody
	}
	if strings.TrimSpace(in.IdempotencyKey) != "" {
		var existing HandoffView
		existing, err = scanHandoff(tx.QueryRow(ctx, `select `+handoffColumns+` from messaging.handoffs
			where account_id=$1::uuid and idempotency_key=$2`, accountID, strings.TrimSpace(in.IdempotencyKey)))
		if err == nil {
			if err := tx.Commit(ctx); err != nil {
				return HandoffView{}, err
			}
			return existing, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return HandoffView{}, err
		}
	}
	var open HandoffView
	open, err = scanHandoff(tx.QueryRow(ctx, `select `+handoffColumns+` from messaging.handoffs
		where account_id=$1::uuid and conversation_id=$2::uuid
		  and status in ('requested','queued','accepted') order by created_at desc, id desc limit 1`, accountID, conversationID))
	if err == nil {
		if in.IntelligenceAcceptance != nil {
			return HandoffView{}, ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return HandoffView{}, err
		}
		return open, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return HandoffView{}, err
	}
	var queueID, departmentID, policyID *string
	policySnapshot := json.RawMessage(`{}`)
	if selected, ok, err := s.selectHandoffPolicyTx(ctx, tx, accountID, snap, in); err != nil {
		return HandoffView{}, err
	} else if ok {
		policyID = &selected.ID
		policySnapshot, err = json.Marshal(selected.Snapshot())
		if err != nil {
			return HandoffView{}, err
		}
		queueID, departmentID, err = resolvePolicyQueueTx(ctx, tx, accountID, selected)
		if err != nil {
			return HandoffView{}, err
		}
	}
	if queueID == nil && in.TargetQueueID != nil {
		var q, d string
		if err := tx.QueryRow(ctx, `select q.id::text, q.department_id::text from messaging.queues q
			where q.account_id=$1::uuid and q.id=$2::uuid and q.is_active`, accountID, *in.TargetQueueID).Scan(&q, &d); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return HandoffView{}, ErrNotFound
			}
			return HandoffView{}, err
		}
		queueID, departmentID = &q, &d
	} else {
		var q, d string
		err := tx.QueryRow(ctx, `select q.id::text, d.id::text from messaging.departments d
			join messaging.queues q on q.account_id=d.account_id and q.department_id=d.id
			where d.account_id=$1::uuid and d.is_default and d.is_active and q.is_default and q.is_active
			limit 1`, accountID).Scan(&q, &d)
		if err == nil {
			queueID, departmentID = &q, &d
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return HandoffView{}, err
		}
	}
	update := stateUpdate{
		State: StateQueued, QueueID: queueID, DepartmentID: departmentID,
		AssignedUserID: nil, InvalidateAI: true,
	}
	var customerNoticeMessage *MessageView
	customerNoticeCreated := false
	if notice := strings.TrimSpace(in.CustomerNotice); notice != "" {
		if snap.State != StateAIActive || snap.AIGeneration != in.CapturedGeneration {
			return HandoffView{}, ErrAILeaseInvalid
		}
		message, created, createErr := createAIOutboundMessageLockedTx(ctx, tx, accountID, conversationID,
			snap.InstanceID, snap.InstanceScopeKey, notice, in.AIRunID,
			in.NoticeIdempotencyKey, in.CapturedGeneration)
		if createErr != nil {
			return HandoffView{}, createErr
		}
		update.PreserveAIMessageID = message.ID
		customerNoticeMessage = &message
		customerNoticeCreated = created
	}
	if in.IntelligenceAcceptance != nil {
		event := *in.IntelligenceAcceptance
		event.AccountID = accountID
		event.ConversationID = conversationID
		event.Generation = in.CapturedGeneration
		event.Outcome = "handoff"
		if customerNoticeMessage != nil {
			event.MessageID = customerNoticeMessage.ID
		}
		if err := insertIntelligenceAcceptanceTx(ctx, tx, event); err != nil {
			return HandoffView{}, err
		}
	}
	if err := applyStateUpdateTx(ctx, tx, accountID, conversationID, update, s.AIDispatchV2Enabled()); err != nil {
		return HandoffView{}, err
	}
	row, err := scanHandoff(tx.QueryRow(ctx, `insert into messaging.handoffs
		(account_id, conversation_id, reason_code, summary, collected_fields, source_state,
		 target_queue_id, policy_id, policy_snapshot, status, idempotency_key, queued_at)
		values ($1::uuid,$2::uuid,$3,$4,$5::jsonb,$6,$7::uuid,$8::uuid,$9::jsonb,'queued',$10,now())
		returning `+handoffColumns, accountID, conversationID, in.ReasonCode, in.Summary, fields,
		string(snap.State), queueID, policyID, policySnapshot, strings.TrimSpace(in.IdempotencyKey)))
	if err != nil {
		return HandoffView{}, err
	}
	if err := insertSLAEventTx(ctx, tx, accountID, conversationID, row.ID, "started", "sla:"+row.ID+":started", nil); err != nil {
		return HandoffView{}, err
	}
	if _, err := tx.Exec(ctx, `insert into messaging.audit_events
		(account_id, actor_user_id, conversation_id, event_type, payload_json)
		values ($1::uuid, nullif($2,'')::uuid, $3::uuid, 'HANDOFF_REQUESTED',
		jsonb_build_object('handoffId',$4::text,'reasonCode',$5::text,'policyId',$6::text))`, accountID, actorID, conversationID, row.ID, row.ReasonCode, policyID); err != nil {
		return HandoffView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return HandoffView{}, err
	}
	if customerNoticeMessage != nil {
		messageID := customerNoticeMessage.ID
		row.CustomerNoticeMessageID = &messageID
		row.customerNoticeMessage = customerNoticeMessage
		row.customerNoticeCreated = customerNoticeCreated
	}
	return row, nil
}

// TakeConversation serializa o primeiro atendente com o mesmo lock da FSM. Um
// segundo usuário recebe conflito; o mesmo usuário repetindo a ação recebe o
// estado já aceito, mantendo o endpoint idempotente mesmo sem confiar no body.
func (s *Store) TakeConversation(ctx context.Context, accountID, conversationID, userID string, allowUnscoped bool) (conversationRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return conversationRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	snap, err := lockConversationSnapshotTx(ctx, tx, accountID, conversationID)
	if err != nil {
		return conversationRow{}, err
	}
	if snap.AssignedUserID != nil && *snap.AssignedUserID != userID {
		return conversationRow{}, ErrConflict
	}
	if snap.QueueID != nil {
		var member bool
		if err := tx.QueryRow(ctx, `select exists(select 1 from messaging.queue_members
			where account_id=$1::uuid and queue_id=$2::uuid and user_id=$3::uuid and is_active)`, accountID, *snap.QueueID, userID).Scan(&member); err != nil {
			return conversationRow{}, err
		}
		if !member && !allowUnscoped {
			return conversationRow{}, ErrNotFound
		}
	}
	if snap.AssignedUserID == nil {
		out, err := Apply(snap.State, EventHumanAssign, TransitionContext{HasQueue: snap.QueueID != nil})
		if err != nil {
			return conversationRow{}, err
		}
		upd := stateUpdate{State: out.To, QueueID: snap.QueueID, DepartmentID: snap.DepartmentID,
			AssignedUserID: &userID, InvalidateAI: true}
		if err := applyStateUpdateTx(ctx, tx, accountID, conversationID, upd, s.AIDispatchV2Enabled()); err != nil {
			return conversationRow{}, err
		}
	}
	if _, err := tx.Exec(ctx, `update messaging.handoffs set status='accepted', accepted_by_user_id=$3::uuid,
		accepted_at=coalesce(accepted_at,now()), updated_at=now()
		where account_id=$1::uuid and conversation_id=$2::uuid and status in ('requested','queued')`, accountID, conversationID, userID); err != nil {
		return conversationRow{}, err
	}
	if _, err := tx.Exec(ctx, `insert into messaging.audit_events
		(account_id, actor_user_id, conversation_id, event_type, payload_json)
		values ($1::uuid,$2::uuid,$3::uuid,'HANDOFF_ACCEPTED',jsonb_build_object('userId',$2::text))`, accountID, userID, conversationID); err != nil {
		return conversationRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return conversationRow{}, err
	}
	return s.GetConversation(ctx, accountID, conversationID)
}

func (s *Store) ListSLAEvents(ctx context.Context, accountID, conversationID string) ([]SLAEventView, error) {
	rows, err := s.pool.Query(ctx, `select id::text, conversation_id::text, handoff_id::text,
		event_type, idempotency_key, occurred_at, metadata from messaging.sla_events
		where account_id=$1::uuid and conversation_id=$2::uuid order by occurred_at desc, id desc`, accountID, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SLAEventView, 0)
	for rows.Next() {
		var item SLAEventView
		if err := rows.Scan(&item.ID, &item.ConversationID, &item.HandoffID, &item.EventType, &item.IdempotencyKey, &item.OccurredAt, &item.Metadata); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
