package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type aiToolBindingExecution struct {
	AccountID           string
	ConversationID      string
	DispatchID          string
	Generation          int64
	AgentID             string
	ToolID              string
	BindingID           string
	IsEnabled           bool
	Mode                string
	AllowedOperations   []string
	InputSchema         json.RawMessage
	OutputSchema        json.RawMessage
	TimeoutMS           int
	MaxCallsPerDispatch int
}

type aiToolRunRecord struct {
	ID           string
	BindingID    string
	Status       string
	Operation    string
	InputMasked  json.RawMessage
	OutputMasked json.RawMessage
	LatencyMS    int
	Error        string
}

func (s *Store) GetAIToolBindingForCall(ctx context.Context, accountID, dispatchID string, generation int64, bindingID string) (aiToolBindingExecution, error) {
	var out aiToolBindingExecution
	var operationsRaw json.RawMessage
	err := s.pool.QueryRow(ctx, `select d.conversation_id::text, d.id::text, v.agent_id::text,
		b.id::text, b.tool_id, b.is_enabled, b.mode, b.allowed_operations,
		b.input_schema, b.output_schema, b.timeout_ms, b.max_calls_per_dispatch
		from messaging.ai_dispatches d
		join messaging.ai_agent_versions v
		  on v.account_id = d.account_id and v.id = d.agent_version_id
		join messaging.ai_tool_bindings b
		  on b.account_id = d.account_id and b.agent_id = v.agent_id and b.id = $4::uuid
		where d.account_id = $1::uuid and d.id = $2::uuid and d.generation = $3
		  and d.status = 'processing'`, accountID, dispatchID, generation, bindingID).Scan(
		&out.ConversationID, &out.DispatchID, &out.AgentID, &out.BindingID, &out.ToolID,
		&out.IsEnabled, &out.Mode, &operationsRaw, &out.InputSchema, &out.OutputSchema,
		&out.TimeoutMS, &out.MaxCallsPerDispatch)
	if errors.Is(err, pgx.ErrNoRows) {
		return aiToolBindingExecution{}, ErrNotFound
	}
	if err != nil {
		return aiToolBindingExecution{}, err
	}
	var operations []string
	if len(operationsRaw) > 0 && json.Unmarshal(operationsRaw, &operations) != nil {
		return aiToolBindingExecution{}, ErrAIToolDenied
	}
	out.AccountID = accountID
	out.AllowedOperations = operations
	out.DispatchID = dispatchID
	out.Generation = generation
	return out, nil
}

func (s *Store) ClaimAIToolRun(ctx context.Context, binding aiToolBindingExecution, callID, operation string, inputMasked json.RawMessage) (aiToolRunRecord, bool, error) {
	callID = strings.TrimSpace(callID)
	operation = strings.TrimSpace(operation)
	if callID == "" || operation == "" || len(callID) > 160 || len(operation) > 160 {
		return aiToolRunRecord{}, false, ErrValidation
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return aiToolRunRecord{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var lockedDispatchID string
	if err := tx.QueryRow(ctx, `select id::text from messaging.ai_dispatches
		where account_id=$1::uuid and id=$2::uuid and generation=$3 and status='processing'
		for update`, binding.AccountID, binding.DispatchID, binding.Generation).Scan(&lockedDispatchID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return aiToolRunRecord{}, false, ErrNotFound
		}
		return aiToolRunRecord{}, false, err
	}
	var existing aiToolRunRecord
	err = scanAIToolRun(tx.QueryRow(ctx, `select id::text,binding_id::text,status,operation,input_masked,output_masked,latency_ms,error
		from messaging.ai_tool_runs where account_id=$1::uuid and dispatch_id=$2::uuid and call_id=$3`,
		binding.AccountID, binding.DispatchID, callID), &existing)
	if err == nil {
		if existing.BindingID != binding.BindingID || existing.Operation != operation || string(existing.InputMasked) != string(inputMasked) {
			return aiToolRunRecord{}, false, ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return aiToolRunRecord{}, false, err
		}
		return existing, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return aiToolRunRecord{}, false, err
	}
	var callCount int
	if err := tx.QueryRow(ctx, `select count(*) from messaging.ai_tool_runs
		where account_id=$1::uuid and dispatch_id=$2::uuid
		  and status in ('requested','approved','running','completed','failed','timeout')`,
		binding.AccountID, binding.DispatchID).Scan(&callCount); err != nil {
		return aiToolRunRecord{}, false, err
	}
	status := "requested"
	errorText := ""
	if callCount >= binding.MaxCallsPerDispatch {
		status = "denied"
		errorText = "max_calls_per_dispatch"
	}
	err = scanAIToolRun(tx.QueryRow(ctx, `insert into messaging.ai_tool_runs
		(account_id,conversation_id,dispatch_id,binding_id,call_id,status,operation,input_masked,error)
		values ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6,$7,$8::jsonb,$9)
		returning id::text,binding_id::text,status,operation,input_masked,output_masked,latency_ms,error`,
		binding.AccountID, binding.ConversationID, binding.DispatchID, binding.BindingID,
		callID, status, operation, inputMasked, errorText), &existing)
	if err != nil {
		return aiToolRunRecord{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return aiToolRunRecord{}, false, err
	}
	return existing, false, nil
}

func scanAIToolRun(row rowScanner, out *aiToolRunRecord) error {
	return row.Scan(&out.ID, &out.BindingID, &out.Status, &out.Operation, &out.InputMasked, &out.OutputMasked, &out.LatencyMS, &out.Error)
}

func (s *Store) StartAIToolRun(ctx context.Context, accountID, runID string) error {
	tag, err := s.pool.Exec(ctx, `update messaging.ai_tool_runs set status='running'
		where account_id=$1::uuid and id=$2::uuid and status in ('requested','approved')`, accountID, runID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAIToolInProgress
	}
	return nil
}

func (s *Store) FinishAIToolRun(ctx context.Context, accountID, runID, status string, outputMasked json.RawMessage, latencyMS int, errorText string) error {
	if status != "completed" && status != "failed" && status != "timeout" && status != "denied" {
		return ErrValidation
	}
	if latencyMS < 0 {
		latencyMS = 0
	}
	if len(errorText) > 2000 {
		errorText = errorText[:2000]
	}
	tag, err := s.pool.Exec(ctx, `update messaging.ai_tool_runs
		set status=$3, output_masked=$4::jsonb, latency_ms=$5, error=$6, completed_at=now()
		where account_id=$1::uuid and id=$2::uuid and status in ('requested','running')`,
		accountID, runID, status, outputMasked, latencyMS, errorText)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAIToolInProgress
	}
	return nil
}

func (s *Store) AuditAIToolEvent(ctx context.Context, binding aiToolBindingExecution, eventType, callID, operation, code string) error {
	payload, err := json.Marshal(map[string]any{
		"dispatchId": binding.DispatchID, "generation": binding.Generation, "bindingId": binding.BindingID,
		"toolId": binding.ToolID, "callId": callID, "operation": operation, "code": code,
	})
	if err != nil {
		return err
	}
	return s.InsertAudit(ctx, binding.AccountID, "", binding.ConversationID, "", eventType, payload)
}

func toolCallLatencyMS(start time.Time) int { return int(time.Since(start).Milliseconds()) }
