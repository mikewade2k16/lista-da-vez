package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

const intelligenceAcceptedJobKind = "omnichannel.intelligence_accepted"

func insertIntelligenceAcceptanceTx(
	ctx context.Context,
	tx pgx.Tx,
	event CustomerIntelligenceAcceptedOutcome,
) error {
	event.EventID = strings.TrimSpace(event.EventID)
	event.AccountID = strings.TrimSpace(event.AccountID)
	event.ClientAccountID = strings.TrimSpace(event.ClientAccountID)
	event.ConversationID = strings.TrimSpace(event.ConversationID)
	event.DispatchID = strings.TrimSpace(event.DispatchID)
	event.DecisionID = strings.TrimSpace(event.DecisionID)
	event.RunID = strings.TrimSpace(event.RunID)
	event.SubjectID = strings.TrimSpace(event.SubjectID)
	event.RelationshipID = strings.TrimSpace(event.RelationshipID)
	event.MessageID = strings.TrimSpace(event.MessageID)
	event.Outcome = strings.TrimSpace(event.Outcome)
	if !omnichannelUUIDPattern.MatchString(event.EventID) ||
		!omnichannelUUIDPattern.MatchString(event.AccountID) ||
		!omnichannelUUIDPattern.MatchString(event.ClientAccountID) ||
		!omnichannelUUIDPattern.MatchString(event.ConversationID) ||
		!omnichannelUUIDPattern.MatchString(event.DispatchID) ||
		!omnichannelUUIDPattern.MatchString(event.SubjectID) ||
		!omnichannelUUIDPattern.MatchString(event.RelationshipID) ||
		(event.MessageID != "" && !omnichannelUUIDPattern.MatchString(event.MessageID)) ||
		(event.RunID != "" && !omnichannelUUIDPattern.MatchString(event.RunID)) ||
		event.DecisionID == "" || len(event.DecisionID) > 256 ||
		event.Generation < 0 ||
		(event.Outcome != "reply" && event.Outcome != "handoff" && event.Outcome != "no_reply") ||
		!validCustomerIntelligenceClaimEnvelope(event.ProcessRuns, event.Claims) {
		return ErrInvalidBody
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `
		update messaging.ai_dispatches
		set status = 'completed',
		    completed_at = coalesce(completed_at, now()),
		    result_run_id = null,
		    intelligence_decision_id = $4,
		    intelligence_run_id = nullif($5, '')::uuid,
		    locked_at = null,
		    updated_at = now()
		where account_id = $1::uuid
		  and id = $2::uuid
		  and generation = $3
		  and status = 'processing'`,
		event.AccountID, event.DispatchID, event.Generation,
		event.DecisionID, event.RunID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrAILeaseInvalid
	}
	_, err = tx.Exec(ctx, `
		insert into messaging.intelligence_decision_acceptances (
			event_id, account_id, client_account_id, conversation_id,
			dispatch_id, message_id, subject_id, relationship_id,
			generation, decision_id, intelligence_run_id, outcome, reason_code
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4::uuid,
			$5::uuid, nullif($6, '')::uuid, $7::uuid, $8::uuid,
			$9, $10, nullif($11, '')::uuid, $12, 'omnichannel_effect_accepted'
		)
		on conflict (account_id, event_id) do nothing`,
		event.EventID, event.AccountID, event.ClientAccountID,
		event.ConversationID, event.DispatchID, event.MessageID,
		event.SubjectID, event.RelationshipID, event.Generation,
		event.DecisionID, event.RunID, event.Outcome,
	)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		insert into messaging.intelligence_outbox (
			event_id, account_id, client_account_id, ordering_key,
			idempotency_key, kind, aggregate_id, causation_id,
			correlation_id, payload, max_attempts
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4,
			$5, $6, $4::uuid, $7, $8, $9::jsonb, 8
		)
		on conflict (account_id, idempotency_key) do nothing`,
		event.EventID, event.AccountID, event.ClientAccountID,
		event.ConversationID, "intelligence-accepted:"+event.EventID,
		intelligenceAcceptedJobKind, event.DispatchID,
		event.DispatchID, payload,
	)
	return err
}

func (s *Store) CompleteAIDispatchWithIntelligence(
	ctx context.Context,
	event CustomerIntelligenceAcceptedOutcome,
) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := insertIntelligenceAcceptanceTx(ctx, tx, event); err != nil {
		if errors.Is(err, ErrAILeaseInvalid) {
			return false, nil
		}
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
