package omnichannel

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

// AIDispatchJobKind is the durable worker job for the Omnichannel AI brain.
// The payload contains identifiers only; prompt, credentials and provider data stay in PostgreSQL.
const AIDispatchJobKind = "omnichannel.ai_dispatch"

type aiDispatchJobPayload struct {
	DispatchID string `json:"dispatchId"`
	Generation int64  `json:"generation"`
}

type aiDispatchHandler struct {
	store        *Store
	ai           *AIService
	domain       *Service
	send         *SendService
	intelligence CustomerIntelligenceBridge
	logger       *slog.Logger
}

func newAIDispatchHandler(
	store *Store,
	ai *AIService,
	domain *Service,
	send *SendService,
	intelligence CustomerIntelligenceBridge,
	logger *slog.Logger,
) jobs.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return aiDispatchHandler{
		store: store, ai: ai, domain: domain, send: send,
		intelligence: intelligence, logger: logger,
	}
}

func (h aiDispatchHandler) Handle(ctx context.Context, job jobs.Job) error {
	var p aiDispatchJobPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil || strings.TrimSpace(p.DispatchID) == "" || p.Generation < 0 {
		return &jobs.StatusError{StatusCode: 422, Unrecoverable: true, Err: errors.New("omnichannel: invalid ai dispatch payload")}
	}

	dispatch, err := h.store.GetAIDispatch(ctx, job.AccountID, p.DispatchID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if dispatch.Status == AIDispatchCompleted {
		return nil
	}
	if dispatch.Status == AIDispatchCancelled || dispatch.Status == AIDispatchFailed {
		if dispatch.Status == AIDispatchCancelled && dispatch.LastError == "history_reset" {
			return &jobs.StatusError{Unrecoverable: true, Err: ErrHistoryResetInvalidated}
		}
		if dispatch.Status == AIDispatchCancelled &&
			dispatch.LastError == "superseded_by_inbound" {
			return nil
		}
		return h.failOpenToHuman(ctx, job.AccountID, dispatch.ConversationID)
	}
	if dispatch.Generation != p.Generation {
		// A newer inbound message superseded this generation. The newer job owns execution.
		return nil
	}
	conv, err := h.store.ConvTriageContext(ctx, job.AccountID, dispatch.ConversationID)
	if err != nil {
		return err
	}
	if !conv.Found || isWhatsAppGroupExternalID(conv.ExternalID) {
		_, _ = h.store.CancelAIDispatch(ctx, job.AccountID, p.DispatchID, "group_or_missing_conversation")
		return nil
	}
	if conv.AIGeneration != dispatch.Generation {
		_, _ = h.store.CancelAIDispatch(
			ctx, job.AccountID, p.DispatchID, "superseded_by_inbound",
		)
		return nil
	}
	rollout, err := h.store.EvaluateAIRollout(ctx, job.AccountID, dispatch.ConversationID)
	if err != nil {
		return err
	}
	if !rollout.RunAI {
		_, _ = h.store.CancelAIDispatch(ctx, job.AccountID, p.DispatchID, "rollout_"+rollout.Mode)
		return h.failOpenToHuman(ctx, job.AccountID, dispatch.ConversationID)
	}
	agent, enabled, err := h.store.ActiveAgentForInstance(ctx, job.AccountID, deref(conv.InstanceID))
	if err != nil {
		return err
	}
	if !enabled || agent.ActiveVersionID == nil || *agent.ActiveVersionID != dispatch.AgentVersionID {
		_, _ = h.store.CancelAIDispatch(ctx, job.AccountID, p.DispatchID, "automation_disabled")
		return h.failOpenToHuman(ctx, job.AccountID, dispatch.ConversationID)
	}
	if h.domain == nil {
		return &jobs.StatusError{
			StatusCode: 503,
			Err:        errors.New("omnichannel: ai domain service unavailable"),
		}
	}
	if len(dispatch.MessageIDs) == 0 || h.ai == nil {
		_, _ = h.store.FailAIDispatch(ctx, job.AccountID, p.DispatchID, "executor_unavailable")
		return h.failOpenToHuman(ctx, job.AccountID, dispatch.ConversationID)
	}
	started, err := h.store.StartAIDispatch(ctx, job.AccountID, p.DispatchID, p.Generation)
	if err != nil {
		return err
	}
	if !started && job.Attempts > 1 {
		// The generic outbox monitor only retries a job that was left
		// processing after a worker crash. Recover the paired dispatch row too;
		// normal handler failures already requeue it before returning.
		recovered, recoverErr := h.store.RequeueAIDispatch(
			ctx, job.AccountID, p.DispatchID, "worker_retry_recovery",
		)
		if recoverErr != nil {
			return recoverErr
		}
		if recovered {
			started, err = h.store.StartAIDispatch(
				ctx, job.AccountID, p.DispatchID, p.Generation,
			)
			if err != nil {
				return err
			}
		}
	}
	if !started {
		return nil
	}
	allowed, err := h.store.AIDispatchExternalEffectAllowed(ctx, job.AccountID, p.DispatchID, p.Generation)
	if err != nil {
		return err
	}
	if !allowed {
		return &jobs.StatusError{Unrecoverable: true, Err: ErrHistoryResetInvalidated}
	}

	operatorForcedReply := isOperatorReplyDispatch(dispatch)
	messageID := dispatch.MessageIDs[len(dispatch.MessageIDs)-1]
	result, acceptedDecision, err := h.executeIntelligenceOrLegacy(
		ctx, job, dispatch, p, conv, messageID, operatorForcedReply,
	)
	if err != nil {
		if errors.Is(err, ErrHistoryResetInvalidated) {
			return &jobs.StatusError{Unrecoverable: true, Err: ErrHistoryResetInvalidated}
		}
		if isTerminalJobError(err, job) {
			_, _ = h.store.FailAIDispatch(ctx, job.AccountID, p.DispatchID, "dispatch_terminal_error")
			return h.failOpenToHuman(ctx, job.AccountID, dispatch.ConversationID)
		}
		_, _ = h.store.RequeueAIDispatch(ctx, job.AccountID, p.DispatchID, "dispatch_infra_error")
		return err
	}
	latest, err := h.store.GetAIDispatch(ctx, job.AccountID, p.DispatchID)
	if err != nil {
		return err
	}
	if latest.Status == AIDispatchCancelled && latest.LastError == "history_reset" {
		return &jobs.StatusError{Unrecoverable: true, Err: ErrHistoryResetInvalidated}
	}
	if latest.Generation != p.Generation || latest.Status == AIDispatchCancelled {
		// A message arrived while the model was running. Release the row so the newer
		// outbox generation can execute after this FIFO job settles.
		_, _ = h.store.RequeueAIDispatch(ctx, job.AccountID, p.DispatchID, "superseded_generation")
		return nil
	}
	rollout, err = h.store.EvaluateAIRollout(ctx, job.AccountID, dispatch.ConversationID)
	if err != nil {
		return err
	}
	if !rollout.RunAI {
		_, _ = h.store.CancelAIDispatch(ctx, job.AccountID, p.DispatchID, "rollout_"+rollout.Mode)
		return h.failOpenToHuman(ctx, job.AccountID, dispatch.ConversationID)
	}
	if !rollout.AutoSend {
		if rollout.Mode == RolloutModeAssist && result.Outcome == dispatchTriaged &&
			strings.TrimSpace(result.Output.ReplyDraft) != "" {
			_, saved, saveErr := h.store.CompleteAIDispatchWithReplyDraft(
				ctx, job.AccountID, p.DispatchID, p.Generation,
				result.RunID, result.Output.ReplyDraft,
			)
			if saveErr != nil {
				return saveErr
			}
			if saved {
				if h.send != nil {
					h.send.publishAIReplyDraftChanged(ctx, job.AccountID)
				}
				return nil
			}
		}
		return h.complete(ctx, job.AccountID, p.DispatchID, p.Generation, result.RunID)
	}

	if result.Outcome == dispatchBlocked {
		_, _ = h.store.CancelAIDispatch(ctx, job.AccountID, p.DispatchID, "conversation_not_ai_active")
		return nil
	}
	if result.Outcome == dispatchNoReply {
		if acceptedDecision != nil {
			_, err = h.store.CompleteAIDispatchWithIntelligence(
				ctx,
				h.acceptedOutcomeEvent(
					job.AccountID, dispatch, p, conv, messageID,
					acceptedDecision, "no_reply",
				),
			)
			return err
		}
		return h.complete(ctx, job.AccountID, p.DispatchID, p.Generation, result.RunID)
	}
	// The operator command is a one-reply policy override. Technical gates were
	// already enforced by Dispatch and the output still needs a valid reply.
	if operatorForcedReply {
		result = applyOperatorForcedReply(result)
	}
	if result.Outcome == dispatchTriaged && result.Output.NeedsHuman {
		err = h.handoff(ctx, job.AccountID, dispatch, p, result,
			normalizeAIHandoffReason(result.Output.HandoffReason, result.Output.HumanRequested),
			conv, messageID, acceptedDecision)
		return err
	}
	if result.Outcome == dispatchTriaged && result.Output.CloseRequested {
		closeRequest := AutoCloseRequest{
			Proposal: AutoCloseProposal{
				Requested: true, Confidence: result.Output.Confidence,
				HumanRequested: result.Output.HumanRequested, SensitiveTopic: result.Output.SensitiveTopic,
				Reason: result.Output.CloseReason,
			},
			AIRunID: result.RunID, IdempotencyKey: "ai-close:" + p.DispatchID + ":" + fmt.Sprint(p.Generation),
			CapturedGeneration: p.Generation, FinalReply: result.Output.ReplyDraft,
			ReplyIdempotencyKey: "ai-reply:" + firstNonEmpty(result.RunID, messageID),
		}
		if acceptedDecision != nil {
			event := h.acceptedOutcomeEvent(
				job.AccountID, dispatch, p, conv, messageID,
				acceptedDecision, "no_reply",
			)
			closeRequest.IntelligenceAcceptance = &event
		}
		decision, closeErr := h.domain.SystemTryAutoClose(
			ctx, job.AccountID, dispatch.ConversationID, closeRequest,
		)
		if closeErr != nil {
			if errors.Is(closeErr, ErrAILeaseInvalid) || errors.Is(closeErr, ErrConflict) || errors.Is(closeErr, ErrNotFound) {
				_, _ = h.store.CancelAIDispatch(ctx, job.AccountID, p.DispatchID, "conversation_state_changed")
				return nil
			}
			_, _ = h.store.RequeueAIDispatch(ctx, job.AccountID, p.DispatchID, "auto_close_error")
			return closeErr
		}
		if decision.Accepted {
			if h.send != nil {
				h.send.PublishAIAutoCloseResult(ctx, job.AccountID, dispatch.ConversationID, decision)
			}
			return nil
		}
		// During the pilot a blocked close fails safe to a human. The persisted
		// evaluation explains which configurable gate blocked it.
		err = h.handoff(ctx, job.AccountID, dispatch, p, result,
			normalizeAIHandoffReason(firstNonEmpty(result.Output.HandoffReason, result.Output.CloseReason), result.Output.HumanRequested),
			conv, messageID, acceptedDecision)
		return err
	}
	if result.Outcome == dispatchTriaged && strings.TrimSpace(result.Output.ReplyDraft) != "" && h.send != nil {
		moderated, moderationErr := h.store.IsInstagramModeratedMessage(ctx, job.AccountID, messageID)
		if moderationErr != nil && !errors.Is(moderationErr, ErrNotFound) {
			_, _ = h.store.RequeueAIDispatch(ctx, job.AccountID, p.DispatchID, "moderation_lookup_error")
			return moderationErr
		}
		if moderated {
			if err := h.store.SaveInstagramAIDraft(ctx, job.AccountID, messageID, result.Output.ReplyDraft); err != nil {
				_, _ = h.store.RequeueAIDispatch(ctx, job.AccountID, p.DispatchID, "moderation_draft_error")
				return err
			}
			if acceptedDecision != nil {
				_, err = h.store.CompleteAIDispatchWithIntelligence(
					ctx,
					h.acceptedOutcomeEvent(
						job.AccountID, dispatch, p, conv, messageID,
						acceptedDecision, "no_reply",
					),
				)
				return err
			}
		} else if acceptedDecision != nil {
			_, err := h.send.SendAIMessageWithIntelligence(
				ctx, job.AccountID, dispatch.ConversationID,
				result.Output.ReplyDraft, result.RunID, messageID,
				result.AIGeneration,
				h.acceptedOutcomeEvent(
					job.AccountID, dispatch, p, conv, messageID,
					acceptedDecision, "reply",
				),
			)
			if err != nil {
				if errors.Is(err, ErrAILeaseInvalid) {
					_, _ = h.store.CancelAIDispatch(ctx, job.AccountID, p.DispatchID, "conversation_takeover")
					return nil
				}
				_, _ = h.store.RequeueAIDispatch(ctx, job.AccountID, p.DispatchID, "reply_enqueue_error")
				return err
			}
			return nil
		} else if err := h.send.SendAIMessage(ctx, job.AccountID, dispatch.ConversationID, result.Output.ReplyDraft, result.RunID, messageID, result.AIGeneration); err != nil {
			if errors.Is(err, ErrAILeaseInvalid) {
				_, _ = h.store.CancelAIDispatch(ctx, job.AccountID, p.DispatchID, "conversation_takeover")
				return nil
			}
			_, _ = h.store.RequeueAIDispatch(ctx, job.AccountID, p.DispatchID, "reply_enqueue_error")
			return err
		}
		if !result.Output.NeedsHuman {
			return h.complete(ctx, job.AccountID, p.DispatchID, p.Generation, result.RunID)
		}
	}
	if result.Outcome != dispatchTriaged {
		err = h.handoff(ctx, job.AccountID, dispatch, p, result, handoffReasonForResult(result),
			conv, messageID, acceptedDecision)
		return err
	}

	event := triageEventFor(result.Outcome)
	state, err := h.domain.SystemTransition(ctx, job.AccountID, dispatch.ConversationID, event, TransitionPayload{})
	if err != nil {
		if errors.Is(err, ErrAILeaseInvalid) || errors.Is(err, ErrConflict) || errors.Is(err, ErrNotFound) {
			_, _ = h.store.CancelAIDispatch(ctx, job.AccountID, p.DispatchID, "conversation_state_changed")
			return nil
		}
		_, _ = h.store.RequeueAIDispatch(ctx, job.AccountID, p.DispatchID, "transition_error")
		return err
	}
	if state == StateRouting {
		if _, err := h.domain.SystemRoute(ctx, job.AccountID, dispatch.ConversationID); err != nil {
			_, _ = h.store.RequeueAIDispatch(ctx, job.AccountID, p.DispatchID, "routing_error")
			return err
		}
	}
	return h.complete(ctx, job.AccountID, p.DispatchID, p.Generation, result.RunID)
}

func (h aiDispatchHandler) failOpenToHuman(
	ctx context.Context,
	accountID string,
	conversationID string,
) error {
	if h.domain == nil {
		return &jobs.StatusError{
			StatusCode: 503,
			Err:        errors.New("omnichannel: ai domain service unavailable"),
		}
	}
	state, err := h.domain.SystemTransition(
		ctx,
		accountID,
		conversationID,
		EventAITriageFailed,
		TransitionPayload{},
	)
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidTransition) {
			return nil
		}
		return err
	}
	if state != StateRouting {
		return nil
	}
	_, err = h.domain.SystemRoute(ctx, accountID, conversationID)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidTransition) {
		return nil
	}
	return err
}

func (h aiDispatchHandler) executeIntelligenceOrLegacy(
	ctx context.Context,
	job jobs.Job,
	dispatch AIDispatchRecord,
	p aiDispatchJobPayload,
	conv convTriage,
	messageID string,
	operatorForcedReply bool,
) (DispatchResult, *CustomerIntelligenceDecision, error) {
	accountID := job.AccountID
	legacy := func() (DispatchResult, *CustomerIntelligenceDecision, error) {
		result, err := h.ai.Dispatch(ctx, TriageInput{
			AccountID: accountID, ConversationID: dispatch.ConversationID,
			MessageID: messageID, DispatchID: p.DispatchID,
			ForceReply: operatorForcedReply,
		})
		return result, nil, err
	}

	if h.intelligence == nil {
		return legacy()
	}
	policy, err := h.store.CustomerIntelligencePolicy(ctx, accountID)
	if err != nil {
		h.logger.Warn(
			"omnichannel_customer_intelligence_policy_lookup_failed",
			"account_id", accountID,
			"dispatch_id", p.DispatchID,
		)
		return legacy()
	}
	if policy.Mode == "off" ||
		conv.ClientAccountID == nil ||
		strings.TrimSpace(*conv.ClientAccountID) == "" ||
		conv.ClientBindingState != "resolved" {
		return legacy()
	}

	contactSourceID := ""
	if conv.ContactID != nil {
		contactSourceID = strings.TrimSpace(*conv.ContactID)
	}
	message, operationalState, routingCatalog, channelCapabilities, snapshotErr :=
		h.customerIntelligenceSnapshot(ctx, accountID, dispatch, conv, messageID, operatorForcedReply)
	if snapshotErr != nil {
		h.logger.Warn(
			"omnichannel_customer_intelligence_snapshot_failed",
			"account_id", accountID,
			"dispatch_id", p.DispatchID,
		)
		return h.resolveCustomerIntelligenceFailure(
			job,
			p.Generation,
			policy.FailurePolicy,
			NewCustomerIntelligenceFailure(
				"temporarily_unavailable",
				"snapshot_unavailable",
				true,
				snapshotErr,
			),
			legacy,
		)
	}
	request := CustomerIntelligenceInteractionRequest{
		AccountID:               accountID,
		ClientAccountID:         strings.TrimSpace(*conv.ClientAccountID),
		ContactSourceID:         contactSourceID,
		ContactExternalID:       deref(conv.ContactExternalID),
		ContactPhone:            deref(conv.ContactPhone),
		ContactName:             deref(conv.ContactName),
		ConversationID:          dispatch.ConversationID,
		MessageID:               messageID,
		DispatchID:              p.DispatchID,
		Generation:              p.Generation,
		Channel:                 conv.Channel,
		ProcessKey:              "conversation.respond",
		OperatorForced:          operatorForcedReply,
		Message:                 message,
		OperationalState:        operationalState,
		RoutingCatalog:          routingCatalog,
		ChannelCapabilities:     channelCapabilities,
		OccurredAt:              time.Now().UTC(),
		DerivedMemorySuppressed: conv.DerivedMemorySuppressed,
	}
	var decision CustomerIntelligenceDecision
	var decisionErr error
	allowed, err := h.store.WithAIDispatchExternalEffectLease(ctx, accountID, p.DispatchID, p.Generation, func() error {
		decision, decisionErr = h.intelligence.ExecuteInteraction(ctx, request)
		return nil
	})
	if err != nil {
		return DispatchResult{}, nil, err
	}
	if !allowed {
		return DispatchResult{}, nil, ErrHistoryResetInvalidated
	}
	if policy.Mode == "shadow" {
		if decisionErr != nil {
			h.logger.Warn(
				"omnichannel_customer_intelligence_shadow_failed",
				"account_id", accountID,
				"dispatch_id", p.DispatchID,
			)
		} else {
			h.logger.Info(
				"omnichannel_customer_intelligence_shadow_completed",
				"account_id", accountID,
				"dispatch_id", p.DispatchID,
				"decision_id", decision.DecisionID,
				"outcome", decision.Outcome,
			)
		}
		return legacy()
	}
	if decisionErr != nil {
		kind, code, retryable, typed := CustomerIntelligenceFailureDetails(decisionErr)
		if !typed {
			kind, code, retryable = "temporarily_unavailable", "runtime_unavailable", true
		}
		h.logger.Warn(
			"omnichannel_customer_intelligence_primary_failed",
			"account_id", accountID,
			"dispatch_id", p.DispatchID,
			"failure_kind", kind,
			"failure_code", code,
			"retryable", retryable,
			"failure_policy", policy.FailurePolicy,
		)
		return h.resolveCustomerIntelligenceFailure(
			job,
			p.Generation,
			policy.FailurePolicy,
			decisionErr,
			legacy,
		)
	}
	if decision.ReasonCode == "customer_intelligence_disabled" {
		return legacy()
	}
	if !customerIntelligenceDecisionAllowsOperationalEffect(decision) {
		// Runtime shadow/canary is a comparison result, never a technical
		// failure. It must not be routed through a failure policy because an
		// immediate_handoff policy would itself be an operational effect.
		h.logger.Info(
			"omnichannel_customer_intelligence_non_operational_completed",
			"account_id", accountID,
			"dispatch_id", p.DispatchID,
			"decision_id", decision.DecisionID,
			"outcome", decision.Outcome,
		)
		return legacy()
	}
	result, valid := dispatchResultFromIntelligence(decision, p.Generation)
	if !valid {
		h.logger.Warn(
			"omnichannel_customer_intelligence_invalid_decision",
			"account_id", accountID,
			"dispatch_id", p.DispatchID,
			"decision_id", decision.DecisionID,
		)
		return h.resolveCustomerIntelligenceFailure(
			job,
			p.Generation,
			policy.FailurePolicy,
			NewCustomerIntelligenceFailure(
				"invalid_result",
				"decision_contract_invalid",
				false,
				ErrValidation,
			),
			legacy,
		)
	}
	return result, &decision, nil
}

func (h aiDispatchHandler) resolveCustomerIntelligenceFailure(
	job jobs.Job,
	generation int64,
	failurePolicy string,
	failure error,
	legacy func() (DispatchResult, *CustomerIntelligenceDecision, error),
) (DispatchResult, *CustomerIntelligenceDecision, error) {
	switch failurePolicy {
	case "legacy_fallback":
		return legacy()
	case "immediate_handoff":
		return customerIntelligenceFailureHandoff(generation), nil, nil
	default:
		_, _, retryable, typed := CustomerIntelligenceFailureDetails(failure)
		if !typed {
			retryable = true
		}
		if retryable && !isTerminalJobError(failure, job) {
			return DispatchResult{}, nil, failure
		}
		return customerIntelligenceFailureHandoff(generation), nil, nil
	}
}

func customerIntelligenceFailureHandoff(generation int64) DispatchResult {
	return DispatchResult{
		Outcome: dispatchTriaged,
		Output: TriageOutput{
			NeedsHuman:      true,
			HandoffReason:   HandoffReasonError,
			HandoffSummary:  defaultAIHandoffSummary(HandoffReasonError),
			ExtractedFields: map[string]any{},
		},
		AIGeneration: generation,
		ReasonCode:   HandoffReasonError,
	}
}

func (h aiDispatchHandler) customerIntelligenceSnapshot(
	ctx context.Context,
	accountID string,
	dispatch AIDispatchRecord,
	conv convTriage,
	messageID string,
	operatorForcedReply bool,
) (json.RawMessage, json.RawMessage, json.RawMessage, json.RawMessage, error) {
	messages, err := h.store.RecentMessages(ctx, accountID, dispatch.ConversationID, 20)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	targets, err := h.store.RoutingCatalog(ctx, accountID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	message, err := customerIntelligenceMessageSnapshot(
		messageID,
		messages,
		deref(conv.ContactName),
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	operationalState, err := json.Marshal(map[string]any{
		"conversationId":  dispatch.ConversationID,
		"state":           conv.State,
		"aiGeneration":    conv.AIGeneration,
		"operatorForced":  operatorForcedReply,
		"extractedFields": normalizedObject(conv.ExtractedFields),
	})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	routingCatalog, err := json.Marshal(map[string]any{"targets": targets})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	channelCapabilities, err := json.Marshal(map[string]any{
		"channel": conv.Channel,
		"reply":   true,
		"handoff": true,
		"close":   true,
		"send":    false,
	})
	return message, operationalState, routingCatalog, channelCapabilities, err
}

func customerIntelligenceMessageSnapshot(
	messageID string,
	messages []SimMessage,
	contactName string,
) (json.RawMessage, error) {
	// Provider-supplied contact name is untrusted customer data. It travels in
	// the user JSON beside the conversation and is never interpolated into a
	// system prompt. Customer Data remains the durable profile source.
	return json.Marshal(map[string]any{
		"currentMessageId": messageID,
		"contact": map[string]any{
			"displayName": strings.TrimSpace(contactName),
		},
		"messages": messages,
	})
}

func normalizedObject(raw json.RawMessage) json.RawMessage {
	var object map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &object) != nil || object == nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

func dispatchResultFromIntelligence(
	decision CustomerIntelligenceDecision,
	generation int64,
) (DispatchResult, bool) {
	if !customerIntelligenceDecisionAllowsOperationalEffect(decision) {
		return DispatchResult{}, false
	}
	out := TriageOutput{
		Confidence:      decision.Confidence,
		ExtractedFields: decision.ExtractedFields,
		NeedsHuman:      decision.NeedsHuman,
		HumanRequested:  decision.HumanRequested,
		SensitiveTopic:  decision.SensitiveTopic,
		CloseRequested:  decision.CloseRequested,
		CloseReason:     strings.TrimSpace(decision.CloseReason),
		HandoffReason:   strings.TrimSpace(decision.HandoffReason),
		HandoffSummary:  strings.TrimSpace(decision.HandoffSummary),
		ReplyDraft:      strings.TrimSpace(decision.ReplyDraft),
	}
	if out.ExtractedFields == nil {
		out.ExtractedFields = map[string]any{}
	}
	result := DispatchResult{
		Output: out,
		// intelligence.run IDs do not belong to messaging.ai_runs. Passing one
		// through the legacy FK would couple schemas and fail the accept path.
		RunID:        "",
		AIGeneration: generation,
		ReasonCode:   strings.TrimSpace(decision.ReasonCode),
	}
	switch strings.ToLower(strings.TrimSpace(decision.Outcome)) {
	case "no_reply":
		result.Outcome = dispatchNoReply
		return result, true
	case "reply":
		if out.ReplyDraft == "" {
			return DispatchResult{}, false
		}
		result.Outcome = dispatchTriaged
		return result, true
	case "handoff":
		result.Outcome = dispatchTriaged
		result.Output.NeedsHuman = true
		return result, true
	case "close":
		result.Outcome = dispatchTriaged
		result.Output.CloseRequested = true
		return result, true
	case "blocked":
		result.Outcome = dispatchBlocked
		return result, true
	default:
		return DispatchResult{}, false
	}
}

func customerIntelligenceDecisionAllowsOperationalEffect(
	decision CustomerIntelligenceDecision,
) bool {
	if !decision.OperationalEffectAllowed || len(decision.ProcessRuns) == 0 {
		return false
	}
	for _, run := range decision.ProcessRuns {
		if strings.TrimSpace(run.RunID) == "" ||
			run.Status != "succeeded" ||
			run.ExecutionMode != "active" {
			return false
		}
	}
	return true
}

func (h aiDispatchHandler) acceptedOutcomeEvent(
	accountID string,
	dispatch AIDispatchRecord,
	p aiDispatchJobPayload,
	conv convTriage,
	messageID string,
	decision *CustomerIntelligenceDecision,
	outcome string,
) CustomerIntelligenceAcceptedOutcome {
	if decision == nil {
		return CustomerIntelligenceAcceptedOutcome{}
	}
	clientAccountID, contactSourceID := "", ""
	if conv.ClientAccountID != nil {
		clientAccountID = strings.TrimSpace(*conv.ClientAccountID)
	}
	if conv.ContactID != nil {
		contactSourceID = strings.TrimSpace(*conv.ContactID)
	}
	return CustomerIntelligenceAcceptedOutcome{
		EventID: deterministicUUID(fmt.Sprintf(
			"omnichannel:%s:%d:%s",
			p.DispatchID, p.Generation, strings.TrimSpace(outcome),
		)),
		AccountID:         accountID,
		ClientAccountID:   clientAccountID,
		ContactSourceID:   contactSourceID,
		ConversationID:    dispatch.ConversationID,
		MessageID:         messageID,
		DispatchID:        p.DispatchID,
		DecisionID:        decision.DecisionID,
		PipelineVersionID: decision.PipelineVersionID,
		RunID:             decision.RunID,
		ProcessRuns:       append([]CustomerIntelligenceProcessRunRef(nil), decision.ProcessRuns...),
		Claims:            append([]CustomerIntelligenceAcceptedClaimRef(nil), decision.CandidateClaims...),
		SubjectID:         decision.SubjectID,
		RelationshipID:    decision.RelationshipID,
		Generation:        p.Generation,
		Outcome:           strings.TrimSpace(outcome),
	}
}

func deterministicUUID(value string) string {
	sum := sha256.Sum256([]byte(value))
	raw := sum[:16]
	raw[6] = (raw[6] & 0x0f) | 0x50
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16],
	)
}

func (h aiDispatchHandler) handoff(
	ctx context.Context,
	accountID string,
	dispatch AIDispatchRecord,
	p aiDispatchJobPayload,
	result DispatchResult,
	reason string,
	conv convTriage,
	messageID string,
	decision *CustomerIntelligenceDecision,
) error {
	fields, err := json.Marshal(result.Output.ExtractedFields)
	if err != nil || len(fields) == 0 || string(fields) == "null" {
		fields = json.RawMessage(`{}`)
	}
	summary := strings.TrimSpace(firstNonEmpty(result.Output.HandoffSummary, result.Output.CloseReason))
	if summary == "" {
		summary = defaultAIHandoffSummary(reason)
	}
	notice := strings.TrimSpace(result.Output.ReplyDraft)
	if notice == "" {
		notice = defaultAIHandoffCustomerNotice(reason)
	}
	request := HandoffRequest{
		ReasonCode: reason, Summary: summary, CollectedFields: fields,
		IdempotencyKey: "ai-handoff:" + p.DispatchID + ":" + fmt.Sprint(p.Generation),
		CustomerNotice: notice, AIRunID: result.RunID, CapturedGeneration: p.Generation,
		NoticeIdempotencyKey: "ai-handoff-notice:" + p.DispatchID + ":" + fmt.Sprint(p.Generation),
	}
	if decision != nil {
		event := h.acceptedOutcomeEvent(
			accountID, dispatch, p, conv, messageID, decision, "handoff",
		)
		request.IntelligenceAcceptance = &event
	}
	handoff, err := h.domain.SystemRequestHandoff(ctx, accountID, dispatch.ConversationID, request)
	if err == nil {
		if h.send != nil {
			h.send.PublishAIHandoffResult(ctx, accountID, dispatch.ConversationID, handoff)
		}
		return nil
	}
	if errors.Is(err, ErrConflict) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrAILeaseInvalid) {
		_, _ = h.store.CancelAIDispatch(ctx, accountID, p.DispatchID, "conversation_state_changed")
		return nil
	}
	_, _ = h.store.RequeueAIDispatch(ctx, accountID, p.DispatchID, "handoff_error")
	return err
}

func defaultAIHandoffCustomerNotice(reason string) string {
	if reason == HandoffReasonError || reason == HandoffReasonToolFailed {
		return "Tive uma dificuldade técnica e não consegui concluir isso agora. Vou chamar um atendente para continuar com você."
	}
	return "Não consegui concluir isso com segurança. Vou chamar um atendente para continuar com você."
}

func defaultAIHandoffSummary(reason string) string {
	switch reason {
	case HandoffReasonModel:
		return "A IA solicitou apoio humano antes de continuar."
	case HandoffReasonLowConfidence:
		return "A IA interrompeu o atendimento por baixa confianca."
	case HandoffReasonMaxTurns:
		return "A IA atingiu o maximo de respostas automaticas configurado para esta conversa."
	case HandoffReasonError:
		return "A IA interrompeu o atendimento por uma falha tecnica."
	default:
		return "Atendimento transferido pela automacao."
	}
}

func handoffReasonForResult(result DispatchResult) string {
	if validHandoffReason(result.ReasonCode) {
		return result.ReasonCode
	}
	switch result.Outcome {
	case dispatchLimitExceeded:
		return HandoffReasonPolicy
	case dispatchProviderError, dispatchSchemaInvalid:
		return HandoffReasonError
	default:
		return HandoffReasonPolicy
	}
}

func applyOperatorForcedReply(result DispatchResult) DispatchResult {
	if result.Outcome != dispatchTriaged || strings.TrimSpace(result.Output.ReplyDraft) == "" {
		return result
	}
	result.Output.NeedsHuman = false
	result.Output.HandoffReason = ""
	result.Output.HandoffSummary = ""
	result.Output.CloseRequested = false
	result.Output.CloseReason = ""
	return result
}

func (h aiDispatchHandler) complete(ctx context.Context, accountID, dispatchID string, generation int64, runID string) error {
	var run *string
	if strings.TrimSpace(runID) != "" {
		run = &runID
	}
	completed, err := h.store.CompleteAIDispatch(ctx, accountID, dispatchID, generation, run)
	if err != nil {
		return err
	}
	if !completed {
		h.logger.Info("omnichannel_ai_dispatch_stale_completion", "account_id", accountID, "dispatch_id", dispatchID)
	}
	return nil
}

var _ jobs.Handler = aiDispatchHandler{}
