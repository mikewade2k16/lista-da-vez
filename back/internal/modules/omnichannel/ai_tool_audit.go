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

// AIToolRunView is the operator-safe projection of a tool call. The payloads
// are already masked by the gateway; no credential, connection detail or
// unbounded provider output is exposed here.
type AIToolRunView struct {
	ID             string          `json:"id"`
	ConversationID *string         `json:"conversationId"`
	DispatchID     *string         `json:"dispatchId"`
	AgentID        string          `json:"agentId"`
	BindingID      string          `json:"bindingId"`
	ToolID         string          `json:"toolId"`
	CallID         string          `json:"callId"`
	Status         string          `json:"status"`
	Operation      string          `json:"operation"`
	InputMasked    json.RawMessage `json:"inputMasked"`
	OutputMasked   json.RawMessage `json:"outputMasked"`
	LatencyMS      int             `json:"latencyMs"`
	Error          string          `json:"error"`
	CreatedAt      time.Time       `json:"createdAt"`
	CompletedAt    *time.Time      `json:"completedAt"`
}

const aiToolRunViewCols = `r.id::text, r.conversation_id::text, r.dispatch_id::text,
	b.agent_id::text, r.binding_id::text, b.tool_id, r.call_id, r.status, r.operation,
	r.input_masked, r.output_masked, r.latency_ms, r.error, r.created_at, r.completed_at`

func scanAIToolRunView(row rowScanner) (AIToolRunView, error) {
	var out AIToolRunView
	err := row.Scan(&out.ID, &out.ConversationID, &out.DispatchID, &out.AgentID,
		&out.BindingID, &out.ToolID, &out.CallID, &out.Status, &out.Operation,
		&out.InputMasked, &out.OutputMasked, &out.LatencyMS, &out.Error,
		&out.CreatedAt, &out.CompletedAt)
	return out, err
}

func validAIToolRunStatus(status string) bool {
	switch status {
	case "requested", "approved", "denied", "running", "completed", "failed", "timeout":
		return true
	default:
		return false
	}
}

// ListAIToolRuns is account+agent scoped at every query. An unknown cursor
// starts a page, matching the existing AI-run pagination contract.
func (s *Store) ListAIToolRuns(ctx context.Context, accountID, agentID, status, beforeID string, limit int) ([]AIToolRunView, error) {
	query := `select ` + aiToolRunViewCols + `
		from messaging.ai_tool_runs r
		join messaging.ai_tool_bindings b
		  on b.account_id = r.account_id and b.id = r.binding_id
		where r.account_id = $1::uuid and b.agent_id = $2::uuid`
	args := []any{accountID, agentID}
	if strings.TrimSpace(status) != "" {
		query += " and r.status = $" + strconv.Itoa(len(args)+1)
		args = append(args, status)
	}
	if strings.TrimSpace(beforeID) != "" {
		var before time.Time
		err := s.pool.QueryRow(ctx, `select r.created_at
			from messaging.ai_tool_runs r
			join messaging.ai_tool_bindings b
			  on b.account_id = r.account_id and b.id = r.binding_id
			where r.account_id=$1::uuid and b.agent_id=$2::uuid and r.id=$3::uuid`,
			accountID, agentID, beforeID).Scan(&before)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			query += " and r.created_at < $" + strconv.Itoa(len(args)+1) + "::timestamptz"
			args = append(args, before)
		}
	}
	args = append(args, limit)
	query += " order by r.created_at desc, r.id desc limit $" + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AIToolRunView, 0, limit)
	for rows.Next() {
		item, err := scanAIToolRunView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
