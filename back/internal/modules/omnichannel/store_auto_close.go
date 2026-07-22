package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

const autoCloseDecisionColumns = `id::text, conversation_id::text, requested, accepted,
	reason_codes, confidence::float8, minimum_confidence::float8, missing_fields,
	captured_generation, current_generation, created_at`

func scanAutoCloseDecision(row rowScanner) (AutoCloseDecisionView, error) {
	var out AutoCloseDecisionView
	err := row.Scan(&out.ID, &out.ConversationID, &out.Requested, &out.Accepted,
		&out.ReasonCodes, &out.Confidence, &out.MinimumConfidence, &out.MissingFields,
		&out.CapturedGeneration, &out.CurrentGeneration, &out.CreatedAt)
	return out, err
}

// ApplyAIAutoClose serializes the final policy check, state transition and audit
// row. The callback belongs to the domain service and is the only code that can
// authorize the close; this repository only supplies locked, tenant-scoped data.
func (s *Store) ApplyAIAutoClose(ctx context.Context, accountID, conversationID string, in AutoCloseRequest,
	decide func(autoCloseLockedContext) (autoClosePersistence, error)) (AutoCloseDecisionView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AutoCloseDecisionView{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	idempotencyKey := strings.TrimSpace(in.IdempotencyKey)
	if idempotencyKey == "" || len([]rune(idempotencyKey)) > 128 {
		return AutoCloseDecisionView{}, ErrInvalidBody
	}
	view, err := scanAutoCloseDecision(tx.QueryRow(ctx, `select `+autoCloseDecisionColumns+`
		from messaging.ai_close_evaluations
		where account_id=$1::uuid and idempotency_key=$2`, accountID, idempotencyKey))
	if err == nil {
		if view.ConversationID != conversationID {
			return AutoCloseDecisionView{}, ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return AutoCloseDecisionView{}, err
		}
		return view, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return AutoCloseDecisionView{}, err
	}

	snap, err := lockConversationSnapshotTx(ctx, tx, accountID, conversationID)
	if err != nil {
		return AutoCloseDecisionView{}, err
	}
	policy, err := automationClosePolicyTx(ctx, tx, accountID, snap.InstanceID)
	if err != nil {
		return AutoCloseDecisionView{}, err
	}
	collected := map[string]any{}
	if len(snap.ExtractedFields) > 0 && string(snap.ExtractedFields) != "null" {
		_ = json.Unmarshal(snap.ExtractedFields, &collected)
	}
	persist, err := decide(autoCloseLockedContext{Snapshot: snap, Policy: policy, Collected: collected})
	if err != nil {
		return AutoCloseDecisionView{}, err
	}
	var finalMessage *MessageView
	finalMessageCreated := false
	if persist.Evaluation.Accepted {
		if reply := strings.TrimSpace(in.FinalReply); reply != "" {
			message, created, createErr := createAIOutboundMessageLockedTx(ctx, tx, accountID, conversationID,
				snap.InstanceID, snap.InstanceScopeKey, reply, in.AIRunID,
				strings.TrimSpace(in.ReplyIdempotencyKey), in.CapturedGeneration)
			if createErr != nil {
				return AutoCloseDecisionView{}, createErr
			}
			persist.Update.PreserveAIMessageID = message.ID
			finalMessage = &message
			finalMessageCreated = created
		}
		if err := applyStateUpdateTx(ctx, tx, accountID, conversationID, persist.Update, s.AIDispatchV2Enabled()); err != nil {
			return AutoCloseDecisionView{}, err
		}
	}
	if len(persist.PolicyJSON) == 0 {
		persist.PolicyJSON = json.RawMessage(`{}`)
	}
	view, err = scanAutoCloseDecision(tx.QueryRow(ctx, `insert into messaging.ai_close_evaluations
		(account_id,conversation_id,automation_profile_id,ai_run_id,idempotency_key,
		 requested,accepted,reason_codes,confidence,minimum_confidence,required_fields,
		 missing_fields,human_requested,sensitive_topic,source_state,captured_generation,
		 current_generation,policy_snapshot)
		values ($1::uuid,$2::uuid,$3::uuid,nullif($4,'')::uuid,$5,$6,$7,$8,$9,$10,$11,
		 $12,$13,$14,$15,$16,$17,$18::jsonb)
		returning `+autoCloseDecisionColumns,
		accountID, conversationID, policy.ProfileID, in.AIRunID, idempotencyKey,
		in.Proposal.Requested, persist.Evaluation.Accepted, persist.Evaluation.ReasonCodes,
		in.Proposal.Confidence, policy.MinimumConfidence, policy.RequiredFields,
		persist.Evaluation.MissingFields, in.Proposal.HumanRequested, in.Proposal.SensitiveTopic,
		string(snap.State), in.CapturedGeneration, snap.AIGeneration, persist.PolicyJSON))
	if err != nil {
		return AutoCloseDecisionView{}, err
	}
	if persist.Evaluation.Accepted && strings.TrimSpace(in.FinalReply) != "" {
		// scanAutoCloseDecision replaces the public view; restore the transient
		// message metadata used by SendService after the transaction commits.
		messageID := persist.Update.PreserveAIMessageID
		view.FinalMessageID = &messageID
		view.finalMessage = finalMessage
		view.finalMessageCreated = finalMessageCreated
	}
	if err := tx.Commit(ctx); err != nil {
		return AutoCloseDecisionView{}, err
	}
	return view, nil
}

func automationClosePolicyTx(ctx context.Context, tx pgx.Tx, accountID string, instanceID *string) (AutoCloseRuntimePolicy, error) {
	if instanceID == nil || strings.TrimSpace(*instanceID) == "" {
		return AutoCloseRuntimePolicy{}, nil
	}
	var out AutoCloseRuntimePolicy
	err := tx.QueryRow(ctx, `select id::text, enabled, auto_close_enabled,
		auto_close_min_confidence::float8, auto_close_require_all_fields,
		auto_close_block_human_request, auto_close_block_sensitive
		from messaging.automation_profiles
		where account_id=$1::uuid and whatsapp_instance_id=$2::uuid`, accountID, *instanceID).
		Scan(&out.ProfileID, &out.ProfileEnabled, &out.AutoCloseEnabled,
			&out.MinimumConfidence, &out.RequireAllRequiredFields,
			&out.BlockOnHumanRequest, &out.BlockSensitiveTopics)
	if errors.Is(err, pgx.ErrNoRows) {
		return AutoCloseRuntimePolicy{}, nil
	}
	if err != nil {
		return AutoCloseRuntimePolicy{}, err
	}
	out.Found = true
	rows, err := tx.Query(ctx, `select c.key from messaging.collect_field_defs c
		join messaging.automation_profiles p
		  on p.account_id=c.account_id and p.ai_agent_id=c.agent_id
		where p.account_id=$1::uuid and p.whatsapp_instance_id=$2::uuid and c.required
		order by c.sort_order,c.key`, accountID, *instanceID)
	if err != nil {
		return AutoCloseRuntimePolicy{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return AutoCloseRuntimePolicy{}, err
		}
		out.RequiredFields = append(out.RequiredFields, key)
	}
	return out, rows.Err()
}
