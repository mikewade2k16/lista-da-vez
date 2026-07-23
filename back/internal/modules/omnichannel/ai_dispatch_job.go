package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

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
	store  *Store
	ai     *AIService
	domain *Service
	send   *SendService
	logger *slog.Logger
}

func newAIDispatchHandler(store *Store, ai *AIService, domain *Service, send *SendService, logger *slog.Logger) jobs.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return aiDispatchHandler{store: store, ai: ai, domain: domain, send: send, logger: logger}
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
	if dispatch.Status == AIDispatchCompleted || dispatch.Status == AIDispatchCancelled || dispatch.Status == AIDispatchFailed {
		return nil
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
	agent, enabled, err := h.store.ActiveAgentForInstance(ctx, job.AccountID, deref(conv.InstanceID))
	if err != nil {
		return err
	}
	if !enabled || agent.ActiveVersionID == nil || *agent.ActiveVersionID != dispatch.AgentVersionID {
		_, _ = h.store.CancelAIDispatch(ctx, job.AccountID, p.DispatchID, "automation_disabled")
		return nil
	}
	started, err := h.store.StartAIDispatch(ctx, job.AccountID, p.DispatchID, p.Generation)
	if err != nil {
		return err
	}
	if !started {
		return nil
	}
	if len(dispatch.MessageIDs) == 0 || h.ai == nil || h.domain == nil {
		_, _ = h.store.FailAIDispatch(ctx, job.AccountID, p.DispatchID, "executor_unavailable")
		return &jobs.StatusError{StatusCode: 503, Err: errors.New("omnichannel: ai executor unavailable")}
	}

	operatorForcedReply := isOperatorReplyDispatch(dispatch)
	result, err := h.ai.Dispatch(ctx, TriageInput{
		AccountID: job.AccountID, ConversationID: dispatch.ConversationID,
		MessageID: dispatch.MessageIDs[len(dispatch.MessageIDs)-1], DispatchID: p.DispatchID,
		ForceReply: operatorForcedReply,
	})
	if err != nil {
		_, _ = h.store.RequeueAIDispatch(ctx, job.AccountID, p.DispatchID, "dispatch_infra_error")
		return err
	}
	latest, err := h.store.GetAIDispatch(ctx, job.AccountID, p.DispatchID)
	if err != nil {
		return err
	}
	if latest.Generation != p.Generation || latest.Status == AIDispatchCancelled {
		// A message arrived while the model was running. Release the row so the newer
		// outbox generation can execute after this FIFO job settles.
		_, _ = h.store.RequeueAIDispatch(ctx, job.AccountID, p.DispatchID, "superseded_generation")
		return nil
	}

	if result.Outcome == dispatchBlocked {
		_, _ = h.store.CancelAIDispatch(ctx, job.AccountID, p.DispatchID, "conversation_not_ai_active")
		return nil
	}
	if result.Outcome == dispatchNoReply {
		return h.complete(ctx, job.AccountID, p.DispatchID, p.Generation, result.RunID)
	}
	// The operator command is a one-reply policy override. Technical gates were
	// already enforced by Dispatch and the output still needs a valid reply.
	if operatorForcedReply {
		result = applyOperatorForcedReply(result)
	}
	messageID := dispatch.MessageIDs[len(dispatch.MessageIDs)-1]
	if result.Outcome == dispatchTriaged && result.Output.NeedsHuman {
		return h.handoff(ctx, job.AccountID, dispatch, p, result,
			normalizeAIHandoffReason(result.Output.HandoffReason, result.Output.HumanRequested))
	}
	if result.Outcome == dispatchTriaged && result.Output.CloseRequested {
		decision, closeErr := h.domain.SystemTryAutoClose(ctx, job.AccountID, dispatch.ConversationID, AutoCloseRequest{
			Proposal: AutoCloseProposal{
				Requested: true, Confidence: result.Output.Confidence,
				HumanRequested: result.Output.HumanRequested, SensitiveTopic: result.Output.SensitiveTopic,
				Reason: result.Output.CloseReason,
			},
			AIRunID: result.RunID, IdempotencyKey: "ai-close:" + p.DispatchID + ":" + fmt.Sprint(p.Generation),
			CapturedGeneration: p.Generation, FinalReply: result.Output.ReplyDraft,
			ReplyIdempotencyKey: "ai-reply:" + firstNonEmpty(result.RunID, messageID),
		})
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
		return h.handoff(ctx, job.AccountID, dispatch, p, result,
			normalizeAIHandoffReason(firstNonEmpty(result.Output.HandoffReason, result.Output.CloseReason), result.Output.HumanRequested))
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
		return h.handoff(ctx, job.AccountID, dispatch, p, result, handoffReasonForResult(result))
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

func (h aiDispatchHandler) handoff(ctx context.Context, accountID string, dispatch AIDispatchRecord,
	p aiDispatchJobPayload, result DispatchResult, reason string) error {
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
	handoff, err := h.domain.SystemRequestHandoff(ctx, accountID, dispatch.ConversationID, HandoffRequest{
		ReasonCode: reason, Summary: summary, CollectedFields: fields,
		IdempotencyKey: "ai-handoff:" + p.DispatchID + ":" + fmt.Sprint(p.Generation),
		CustomerNotice: notice, AIRunID: result.RunID, CapturedGeneration: p.Generation,
		NoticeIdempotencyKey: "ai-handoff-notice:" + p.DispatchID + ":" + fmt.Sprint(p.Generation),
	})
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
