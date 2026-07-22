package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// AIToolApprovalView é a projeção administrativa de uma proposta. O payload original
// permanece cifrado no Go; somente argumentos mascarados chegam ao painel.
type AIToolApprovalView struct {
	ID             string          `json:"id"`
	RunID          string          `json:"runId"`
	AgentID        string          `json:"agentId"`
	ToolID         string          `json:"toolId"`
	BindingID      string          `json:"bindingId"`
	ConversationID *string         `json:"conversationId"`
	DispatchID     *string         `json:"dispatchId"`
	CallID         string          `json:"callId"`
	Operation      string          `json:"operation"`
	Status         string          `json:"status"`
	InputMasked    json.RawMessage `json:"inputMasked"`
	OutputMasked   json.RawMessage `json:"outputMasked"`
	Reason         string          `json:"reason"`
	Error          string          `json:"error"`
	LatencyMS      int             `json:"latencyMs"`
	RequestedAt    time.Time       `json:"requestedAt"`
	DecidedAt      *time.Time      `json:"decidedAt"`
	DecidedBy      *string         `json:"decidedBy"`
	ExpiresAt      time.Time       `json:"expiresAt"`
}

// CreateAIToolApproval persists the encrypted proposal exactly once. The browser and
// n8n never receive arguments_ciphertext.
func (s *Store) CreateAIToolApproval(ctx context.Context, binding aiToolBindingExecution, runID, callID, operation, ciphertext string) error {
	if strings.TrimSpace(ciphertext) == "" || len(ciphertext) > 262144 {
		return ErrValidation
	}
	_, err := s.pool.Exec(ctx, `insert into messaging.ai_tool_approvals
		(account_id,tool_run_id,binding_id,agent_id,conversation_id,dispatch_id,call_id,operation,arguments_ciphertext)
		values ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,$6::uuid,$7,$8,$9)
		on conflict (account_id,tool_run_id) do nothing`, binding.AccountID, runID, binding.BindingID,
		binding.AgentID, binding.ConversationID, binding.DispatchID, callID, operation, ciphertext)
	return err
}

const aiToolApprovalViewCols = `a.id::text, a.tool_run_id::text, a.agent_id::text, b.tool_id,
	a.binding_id::text, a.conversation_id::text, a.dispatch_id::text, a.call_id, a.operation,
	case when a.status='pending' and a.expires_at <= now() then 'expired' else a.status end,
	r.input_masked, r.output_masked, a.reason, r.error, r.latency_ms, a.requested_at, a.decided_at,
	a.decided_by::text, a.expires_at`

func scanAIToolApprovalView(row rowScanner) (AIToolApprovalView, error) {
	var out AIToolApprovalView
	err := row.Scan(&out.ID, &out.RunID, &out.AgentID, &out.ToolID, &out.BindingID,
		&out.ConversationID, &out.DispatchID, &out.CallID, &out.Operation, &out.Status,
		&out.InputMasked, &out.OutputMasked, &out.Reason, &out.Error, &out.LatencyMS, &out.RequestedAt,
		&out.DecidedAt, &out.DecidedBy, &out.ExpiresAt)
	return out, err
}

func (s *Store) ListAIToolApprovals(ctx context.Context, accountID, agentID string, limit int, beforeID string) ([]AIToolApprovalView, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	query := `select ` + aiToolApprovalViewCols + ` from messaging.ai_tool_approvals a
		join messaging.ai_tool_runs r on r.account_id=a.account_id and r.id=a.tool_run_id
		join messaging.ai_tool_bindings b on b.account_id=a.account_id and b.id=a.binding_id
		where a.account_id=$1::uuid and a.agent_id=$2::uuid`
	args := []any{accountID, agentID}
	if strings.TrimSpace(beforeID) != "" {
		var before time.Time
		err := s.pool.QueryRow(ctx, `select requested_at from messaging.ai_tool_approvals
			where account_id=$1::uuid and agent_id=$2::uuid and id=$3::uuid`, accountID, agentID, beforeID).Scan(&before)
		if errors.Is(err, pgx.ErrNoRows) {
			return []AIToolApprovalView{}, nil
		}
		if err != nil {
			return nil, err
		}
		args = append(args, before)
		query += " and a.requested_at < $" + strconv.Itoa(len(args)) + "::timestamptz"
	}
	args = append(args, limit)
	query += " order by a.requested_at desc, a.id desc limit $" + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AIToolApprovalView, 0, limit)
	for rows.Next() {
		item, scanErr := scanAIToolApprovalView(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// DecideAIToolApproval atomically records a human decision. Approval changes the
// run to approved; the same signed call can then pass the normal Go executor.
func (s *Store) DecideAIToolApproval(ctx context.Context, accountID, agentID, approvalID, actorID string, approved bool, reason string) error {
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) > 500 {
		return ErrValidation
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var runID, runStatus, approvalStatus string
	var expiresAt time.Time
	var conversationID *string
	var bindingID, toolID, callID, operation string
	err = tx.QueryRow(ctx, `select a.tool_run_id::text,r.status,a.status,a.expires_at,r.conversation_id::text,
		a.binding_id::text,b.tool_id,r.call_id,r.operation
		from messaging.ai_tool_approvals a
		join messaging.ai_tool_runs r on r.account_id=a.account_id and r.id=a.tool_run_id
		join messaging.ai_tool_bindings b on b.account_id=a.account_id and b.id=a.binding_id
		where a.account_id=$1::uuid and a.agent_id=$2::uuid and a.id=$3::uuid
		for update of a,r`, accountID, agentID, approvalID).Scan(&runID, &runStatus, &approvalStatus, &expiresAt,
		&conversationID, &bindingID, &toolID, &callID, &operation)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if approvalStatus != "pending" || runStatus != "requested" || !expiresAt.After(time.Now()) {
		return ErrConflict
	}
	nextApproval, nextRun, eventType, code := "rejected", "denied", "AI_TOOL_REJECTED", "approval_rejected"
	if approved {
		nextApproval, nextRun, eventType, code = "approved", "approved", "AI_TOOL_APPROVED", "approved_by_operator"
	}
	if _, err := tx.Exec(ctx, `update messaging.ai_tool_approvals
		set status=$4,reason=$5,decided_by=nullif($6,'')::uuid,decided_at=now()
		where account_id=$1::uuid and agent_id=$2::uuid and id=$3::uuid`, accountID, agentID, approvalID,
		nextApproval, reason, actorID); err != nil {
		return err
	}
	completed := "null"
	if !approved {
		completed = "now()"
	}
	if _, err := tx.Exec(ctx, `update messaging.ai_tool_runs
		set status=$3,error=$4,completed_at=`+completed+`
		where account_id=$1::uuid and id=$2::uuid and status='requested'`, accountID, runID, nextRun, code); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"runId": runID, "bindingId": bindingID, "toolId": toolID,
		"callId": callID, "operation": operation, "code": code})
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `insert into messaging.audit_events
		(account_id,actor_user_id,conversation_id,event_type,payload_json)
		values ($1::uuid,nullif($2,'')::uuid,nullif($3,'')::uuid,$4,$5::jsonb)`, accountID, actorID,
		nullableString(conversationID), eventType, payload); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Store) ApprovalView(ctx context.Context, accountID, agentID, approvalID string) (AIToolApprovalView, error) {
	return scanAIToolApprovalView(s.pool.QueryRow(ctx, `select `+aiToolApprovalViewCols+` from messaging.ai_tool_approvals a
		join messaging.ai_tool_runs r on r.account_id=a.account_id and r.id=a.tool_run_id
		join messaging.ai_tool_bindings b on b.account_id=a.account_id and b.id=a.binding_id
		where a.account_id=$1::uuid and a.agent_id=$2::uuid and a.id=$3::uuid`, accountID, agentID, approvalID))
}

func nullableString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
