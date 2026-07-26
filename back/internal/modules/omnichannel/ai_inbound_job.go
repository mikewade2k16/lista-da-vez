package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

// AIInboundJobKind is the durable hand-off between webhook persistence and the
// AI dispatcher. The intent is written in the same transaction as the inbound
// message, so a committed message can never depend on an in-memory goroutine.
const AIInboundJobKind = "omnichannel.ai.inbound"

const defaultAIInboundDebounceMS = 2500

type aiInboundJobPayload struct {
	ConversationID string `json:"conversationId"`
	MessageID      string `json:"messageId"`
}

type aiInboundStore interface {
	ConvTriageContext(context.Context, string, string) (convTriage, error)
	ActiveAgentForInstance(context.Context, string, string) (agentRow, bool, error)
	AIDispatchConfig(context.Context, string, string) (aiDispatchConfig, error)
	UpsertAIDispatch(context.Context, string, string, string, string, time.Time) (AIDispatchRecord, error)
	AIDispatchV2Enabled() bool
}

type aiInboundDomain interface {
	SystemTransition(context.Context, string, string, Event, TransitionPayload) (State, error)
	SystemRoute(context.Context, string, string) (State, error)
}

type aiInboundHandler struct {
	store  aiInboundStore
	domain aiInboundDomain
	logger *slog.Logger
}

func newAIInboundHandler(store aiInboundStore, domain aiInboundDomain, logger *slog.Logger) jobs.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return aiInboundHandler{store: store, domain: domain, logger: logger}
}

func (h aiInboundHandler) Handle(ctx context.Context, job jobs.Job) error {
	var payload aiInboundJobPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil ||
		strings.TrimSpace(job.AccountID) == "" ||
		strings.TrimSpace(payload.ConversationID) == "" ||
		strings.TrimSpace(payload.MessageID) == "" {
		return &jobs.StatusError{
			StatusCode:    422,
			Unrecoverable: true,
			Err:           errors.New("omnichannel: invalid ai inbound payload"),
		}
	}
	if h.store == nil || h.domain == nil {
		return &jobs.StatusError{
			StatusCode: 503,
			Err:        errors.New("omnichannel: ai inbound coordinator unavailable"),
		}
	}

	accountID := strings.TrimSpace(job.AccountID)
	conversationID := strings.TrimSpace(payload.ConversationID)
	messageID := strings.TrimSpace(payload.MessageID)

	conv, err := h.store.ConvTriageContext(ctx, accountID, conversationID)
	if err != nil {
		return h.retryOrRoute(ctx, job, conversationID, "conversation_lookup_failed", err)
	}
	if !conv.Found {
		// A tenant-scoped delete won the race. There is no conversation left to
		// automate and retrying cannot recreate it.
		return nil
	}

	// msg.inbound was applied in the same transaction that created this job.
	// Reading the resulting state preserves causal ordering: a later human
	// close/takeover must never be undone by delayed automation work.
	state := State(conv.State)
	if state == StateRouting {
		return h.route(ctx, accountID, conversationID)
	}
	if state != StateAIActive {
		// queued/human_active/pending are hard blocks for the AI. The durable
		// intent is complete without calling a model.
		return nil
	}

	if !h.store.AIDispatchV2Enabled() {
		h.logger.Error(
			"omnichannel_ai_inbound_dispatch_schema_unavailable",
			"account_id", accountID,
			"conversation_id", conversationID,
		)
		return h.routeToHuman(ctx, accountID, conversationID)
	}

	agent, enabled, err := h.store.ActiveAgentForInstance(ctx, accountID, deref(conv.InstanceID))
	if err != nil {
		return h.retryOrRoute(ctx, job, conversationID, "active_agent_lookup_failed", err)
	}
	if !enabled || agent.ActiveVersionID == nil || strings.TrimSpace(*agent.ActiveVersionID) == "" {
		return h.routeToHuman(ctx, accountID, conversationID)
	}

	versionID := strings.TrimSpace(*agent.ActiveVersionID)
	config, err := h.store.AIDispatchConfig(ctx, accountID, versionID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return h.routeToHuman(ctx, accountID, conversationID)
		}
		return h.retryOrRoute(ctx, job, conversationID, "dispatch_config_lookup_failed", err)
	}
	debounceMS := config.DebounceMS
	if debounceMS <= 0 {
		debounceMS = defaultAIInboundDebounceMS
	}
	runAfter := time.Now().UTC().Add(time.Duration(debounceMS) * time.Millisecond)
	if _, err := h.store.UpsertAIDispatch(
		ctx, accountID, conversationID, versionID, messageID, runAfter,
	); err != nil {
		if errors.Is(err, ErrAILeaseInvalid) || errors.Is(err, ErrNotFound) ||
			errors.Is(err, ErrAIDispatchInvalidInput) {
			return h.routeToHuman(ctx, accountID, conversationID)
		}
		return h.retryOrRoute(ctx, job, conversationID, "dispatch_enqueue_failed", err)
	}
	return nil
}

func (h aiInboundHandler) retryOrRoute(
	ctx context.Context,
	job jobs.Job,
	conversationID string,
	code string,
	cause error,
) error {
	if !isTerminalJobError(cause, job) {
		return cause
	}
	h.logger.Error(
		"omnichannel_ai_inbound_terminal_failure",
		"account_id", job.AccountID,
		"conversation_id", conversationID,
		"code", code,
	)
	if err := h.routeToHuman(ctx, job.AccountID, conversationID); err != nil {
		return err
	}
	return nil
}

func (h aiInboundHandler) routeToHuman(ctx context.Context, accountID, conversationID string) error {
	state, err := h.domain.SystemTransition(
		ctx, accountID, conversationID, EventAITriageFailed, TransitionPayload{},
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
	return h.route(ctx, accountID, conversationID)
}

func (h aiInboundHandler) route(ctx context.Context, accountID, conversationID string) error {
	_, err := h.domain.SystemRoute(ctx, accountID, conversationID)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidTransition) {
		// A human takeover/close/another worker may have won after the state
		// read. Those states are already safe and must not retry the AI.
		return nil
	}
	return err
}

func enqueueAIInboundJobTx(
	ctx context.Context,
	tx pgx.Tx,
	accountID string,
	conversationID string,
	messageID string,
) error {
	payload, err := json.Marshal(aiInboundJobPayload{
		ConversationID: conversationID,
		MessageID:      messageID,
	})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `insert into messaging.outbox
		(account_id, ordering_key, idempotency_key, kind, payload, max_attempts)
		values ($1::uuid, $2, $3, $4, $5::jsonb, 5)
		on conflict (account_id, idempotency_key) do nothing`,
		accountID,
		conversationID,
		fmt.Sprintf("ai-inbound:%s", messageID),
		AIInboundJobKind,
		payload,
	)
	return err
}

var _ jobs.Handler = aiInboundHandler{}
