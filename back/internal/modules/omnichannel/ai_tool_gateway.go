package omnichannel

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type aiToolCallRequest struct {
	Timestamp  int64           `json:"timestamp"`
	Signature  string          `json:"signature"`
	DispatchID string          `json:"dispatchId"`
	Generation int64           `json:"generation"`
	BindingID  string          `json:"bindingId"`
	CallID     string          `json:"callId"`
	Operation  string          `json:"operation"`
	Arguments  json.RawMessage `json:"arguments"`
}

type aiToolCallResponse struct {
	OK               bool            `json:"ok"`
	CallID           string          `json:"callId"`
	ToolID           string          `json:"toolId"`
	Operation        string          `json:"operation"`
	Status           string          `json:"status"`
	ApprovalRequired bool            `json:"approvalRequired"`
	Output           json.RawMessage `json:"output"`
	ErrorCode        string          `json:"errorCode,omitempty"`
	LatencyMS        int             `json:"latencyMs"`
}

type aiToolCallGateway struct {
	box      *secretbox.Box
	store    *Store
	registry *AIToolRegistry
}

func newAIToolCallGateway(box *secretbox.Box, store *Store, registry *AIToolRegistry) *aiToolCallGateway {
	return &aiToolCallGateway{box: box, store: store, registry: registry}
}

func registerAIToolCallRoutes(mux *http.ServeMux, gateway *aiToolCallGateway) {
	if gateway == nil {
		return
	}
	mux.HandleFunc("POST /v1/internal/omnichannel/ai/tool-calls", gateway.handle)
	mux.HandleFunc("POST /v1/internal/omnichannel/ai/tool-call-signatures", gateway.handleSign)
}

// handleSign is the only component allowed to mint a tool-call signature for
// the n8n orchestrator. The short-lived gateway token authenticates the
// dispatch, while the binding lookup keeps the logical operation tenant-scoped
// and enabled. No provider credential is returned.
func (g *aiToolCallGateway) handleSign(w http.ResponseWriter, r *http.Request) {
	if g == nil || g.box == nil || g.store == nil {
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "tool_gateway_unavailable", "Gateway de tools indisponivel.")
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "tool_gateway_unauthorized", "Token interno ausente.")
		return
	}
	claims, err := decryptBrainGatewayClaims(g.box, token)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "tool_gateway_unauthorized", "Token interno invalido.")
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var in aiToolCallRequest
	if err := decoder.Decode(&in); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body invalido.")
		return
	}
	if !validAIToolTimestamp(in.Timestamp) || in.DispatchID != claims.DispatchID || in.Generation != claims.Generation ||
		!omnichannelUUIDPattern.MatchString(in.DispatchID) || !omnichannelUUIDPattern.MatchString(in.BindingID) ||
		strings.TrimSpace(in.CallID) == "" || len(in.CallID) > 160 || strings.TrimSpace(in.Operation) == "" || len(in.Operation) > 160 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "tool_call_invalid", "Identidade da tool invalida.")
		return
	}
	binding, err := g.store.GetAIToolBindingForCall(r.Context(), claims.AccountID, in.DispatchID, in.Generation, in.BindingID)
	if errors.Is(err, ErrNotFound) {
		httpapi.WriteError(w, r, http.StatusNotFound, "tool_not_found", "Tool nao encontrada para este dispatch.")
		return
	}
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao carregar a tool.")
		return
	}
	if !binding.IsEnabled || !containsAIToolOperation(binding.AllowedOperations, in.Operation) {
		_ = g.store.AuditAIToolEvent(r.Context(), binding, "AI_TOOL_DENIED", in.CallID, in.Operation, "binding_disabled_or_operation_denied")
		httpapi.WriteError(w, r, http.StatusForbidden, "tool_call_denied", "A chamada da tool foi negada.")
		return
	}
	if err := validateAIToolArguments(binding.InputSchema, in.Arguments); err != nil {
		httpapi.WriteError(w, r, http.StatusUnprocessableEntity, "tool_call_invalid", "Argumentos da tool invalidos.")
		return
	}
	in.Signature = aiToolSignature(token, in)
	httpapi.WriteJSON(w, http.StatusOK, in)
}

func (g *aiToolCallGateway) handle(w http.ResponseWriter, r *http.Request) {
	if g == nil || g.box == nil || g.store == nil {
		httpapi.WriteError(w, r, http.StatusServiceUnavailable, "tool_gateway_unavailable", "Gateway de tools indisponível.")
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "tool_gateway_unauthorized", "Token interno ausente.")
		return
	}
	claims, err := decryptBrainGatewayClaims(g.box, token)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "tool_gateway_unauthorized", "Token interno inválido.")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil || len(body) == 0 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body inválido.")
		return
	}
	var in aiToolCallRequest
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid_body", "Body inválido.")
		return
	}
	if !validAIToolTimestamp(in.Timestamp) || !validAIToolSignature(token, in) {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "tool_gateway_unauthorized", "Assinatura da chamada inválida.")
		return
	}
	if in.DispatchID != claims.DispatchID || in.Generation != claims.Generation || strings.TrimSpace(in.DispatchID) == "" {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "tool_gateway_unauthorized", "Chamada não corresponde ao dispatch.")
		return
	}
	if !omnichannelUUIDPattern.MatchString(in.DispatchID) || !omnichannelUUIDPattern.MatchString(in.BindingID) ||
		strings.TrimSpace(in.CallID) == "" || len(in.CallID) > 160 || strings.TrimSpace(in.Operation) == "" || len(in.Operation) > 160 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "tool_call_invalid", "Identidade da tool inválida.")
		return
	}
	binding, err := g.store.GetAIToolBindingForCall(r.Context(), claims.AccountID, in.DispatchID, in.Generation, in.BindingID)
	if errors.Is(err, ErrNotFound) {
		_ = g.store.InsertAudit(r.Context(), claims.AccountID, "", "", "", "AI_TOOL_DENIED", toolAuditPayload(in, "binding_not_found"))
		httpapi.WriteError(w, r, http.StatusNotFound, "tool_not_found", "Tool não encontrada para este dispatch.")
		return
	}
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao carregar a tool.")
		return
	}
	if !binding.IsEnabled || !containsAIToolOperation(binding.AllowedOperations, in.Operation) {
		g.finishDenied(w, r, binding, in, "binding_disabled_or_operation_denied", http.StatusForbidden)
		return
	}
	if err := validateAIToolArguments(binding.InputSchema, in.Arguments); err != nil {
		g.finishDenied(w, r, binding, in, "arguments_invalid", http.StatusUnprocessableEntity)
		return
	}
	inputMasked := maskAIToolJSON(in.Arguments, maxAIToolArgumentsBytes)
	run, existing, err := g.store.ClaimAIToolRun(r.Context(), binding, in.CallID, in.Operation, inputMasked)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			httpapi.WriteError(w, r, http.StatusConflict, "tool_call_conflict", "Call ID já foi usado com outro payload.")
			return
		}
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao reservar a tool.")
		return
	}
	if existing {
		// A pending proposal is intentionally visible to the caller until an
		// operator decides it. Once approved, the same signed call_id may
		// continue through the normal executor; completed/failed calls remain
		// idempotent and never invoke the handler again.
		if run.Status != "approved" {
			g.writeExistingRun(w, r, binding, in, run)
			return
		}
	}
	if run.Status == "denied" {
		_ = g.store.AuditAIToolEvent(r.Context(), binding, "AI_TOOL_DENIED", in.CallID, in.Operation, run.Error)
		httpapi.WriteJSON(w, http.StatusOK, aiToolCallResponse{OK: false, CallID: in.CallID, ToolID: binding.ToolID,
			Operation: in.Operation, Status: run.Status, ErrorCode: run.Error, Output: run.OutputMasked})
		return
	}
	if binding.Mode != "read" && run.Status != "approved" {
		if !existing {
			argumentsCiphertext, encryptErr := g.box.Encrypt(string(in.Arguments))
			if encryptErr != nil || g.store.CreateAIToolApproval(r.Context(), binding, run.ID, in.CallID, in.Operation, argumentsCiphertext) != nil {
				g.finishRunFailure(r.Context(), binding, run.ID, in, "approval_persist_failed", false)
				httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao registrar a aprovacao da tool.")
				return
			}
			_ = g.store.AuditAIToolEvent(r.Context(), binding, "AI_TOOL_APPROVAL_REQUESTED", in.CallID, in.Operation, "approval_required")
		}
		_ = g.store.AuditAIToolEvent(r.Context(), binding, "AI_TOOL_REQUESTED", in.CallID, in.Operation, "approval_required")
		httpapi.WriteJSON(w, http.StatusOK, aiToolCallResponse{OK: false, CallID: in.CallID, ToolID: binding.ToolID,
			Operation: in.Operation, Status: "requested", ApprovalRequired: true, ErrorCode: "approval_required",
			Output: json.RawMessage(`{"approvalRequired":true}`)})
		return
	}
	if err := g.store.StartAIToolRun(r.Context(), claims.AccountID, run.ID); err != nil {
		httpapi.WriteError(w, r, http.StatusConflict, "tool_call_in_progress", "A chamada já está em execução.")
		return
	}
	_ = g.store.AuditAIToolEvent(r.Context(), binding, "AI_TOOL_REQUESTED", in.CallID, in.Operation, "")
	var handler AIToolHandler
	var ok bool
	if g.registry != nil {
		handler, ok = g.registry.resolve(binding.ToolID)
	}
	if !ok {
		g.finishRunFailure(r.Context(), binding, run.ID, in, "tool_not_registered", false)
		httpapi.WriteJSON(w, http.StatusOK, aiToolCallResponse{OK: false, CallID: in.CallID, ToolID: binding.ToolID,
			Operation: in.Operation, Status: "failed", ErrorCode: "tool_not_registered", Output: json.RawMessage(`{}`)})
		return
	}
	start := time.Now()
	callCtx, cancel := context.WithTimeout(r.Context(), time.Duration(binding.TimeoutMS)*time.Millisecond)
	defer cancel()
	output, execErr := handler(callCtx, AIToolInvocation{AccountID: binding.AccountID, ConversationID: binding.ConversationID,
		DispatchID: binding.DispatchID, Generation: binding.Generation, AgentID: binding.AgentID, ToolID: binding.ToolID,
		Operation: in.Operation, Arguments: in.Arguments})
	latency := toolCallLatencyMS(start)
	if execErr != nil {
		isTimeout := errors.Is(execErr, context.DeadlineExceeded) || errors.Is(callCtx.Err(), context.DeadlineExceeded)
		code := "tool_failed"
		if isTimeout {
			code = "tool_timeout"
		}
		g.finishRunFailure(r.Context(), binding, run.ID, in, code, isTimeout)
		httpapi.WriteJSON(w, http.StatusOK, aiToolCallResponse{OK: false, CallID: in.CallID, ToolID: binding.ToolID,
			Operation: in.Operation, Status: map[bool]string{true: "timeout", false: "failed"}[isTimeout], ErrorCode: code,
			LatencyMS: latency, Output: json.RawMessage(`{}`)})
		return
	}
	if err := validateAIToolArguments(binding.OutputSchema, output); err != nil {
		g.finishRunFailure(r.Context(), binding, run.ID, in, "output_schema_invalid", false)
		httpapi.WriteJSON(w, http.StatusOK, aiToolCallResponse{OK: false, CallID: in.CallID, ToolID: binding.ToolID,
			Operation: in.Operation, Status: "failed", ErrorCode: "output_schema_invalid", LatencyMS: latency,
			Output: json.RawMessage(`{}`)})
		return
	}
	outputMasked := maskAIToolJSON(output, maxAIToolOutputBytes)
	if err := g.store.FinishAIToolRun(r.Context(), claims.AccountID, run.ID, "completed", outputMasked, latency, ""); err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Falha ao auditar o resultado da tool.")
		return
	}
	_ = g.store.AuditAIToolEvent(r.Context(), binding, "AI_TOOL_COMPLETED", in.CallID, in.Operation, "")
	httpapi.WriteJSON(w, http.StatusOK, aiToolCallResponse{OK: true, CallID: in.CallID, ToolID: binding.ToolID,
		Operation: in.Operation, Status: "completed", Output: outputMasked, LatencyMS: latency})
}

func (g *aiToolCallGateway) finishDenied(w http.ResponseWriter, r *http.Request, binding aiToolBindingExecution, in aiToolCallRequest, code string, status int) {
	_ = g.store.InsertAudit(r.Context(), binding.AccountID, "", binding.ConversationID, "", "AI_TOOL_DENIED", toolAuditPayload(in, code))
	httpapi.WriteError(w, r, status, "tool_call_denied", "A chamada da tool foi negada.")
}

func (g *aiToolCallGateway) finishRunFailure(ctx context.Context, binding aiToolBindingExecution, runID string, in aiToolCallRequest, code string, timeout bool) {
	status := "failed"
	event := "AI_TOOL_FAILED"
	if timeout {
		status = "timeout"
		event = "AI_TOOL_TIMEOUT"
	}
	_ = g.store.FinishAIToolRun(ctx, binding.AccountID, runID, status, json.RawMessage(`{}`), 0, code)
	_ = g.store.AuditAIToolEvent(ctx, binding, event, in.CallID, in.Operation, code)
}

func (g *aiToolCallGateway) writeExistingRun(w http.ResponseWriter, r *http.Request, binding aiToolBindingExecution, in aiToolCallRequest, run aiToolRunRecord) {
	if run.Status == "requested" && binding.Mode != "read" {
		httpapi.WriteJSON(w, http.StatusOK, aiToolCallResponse{OK: false, CallID: in.CallID,
			ToolID: binding.ToolID, Operation: run.Operation, Status: "requested",
			ApprovalRequired: true, ErrorCode: "approval_required",
			Output: json.RawMessage(`{"approvalRequired":true}`)})
		return
	}
	if run.Status == "requested" || run.Status == "running" {
		httpapi.WriteError(w, r, http.StatusConflict, "tool_call_in_progress", "A chamada já está em execução.")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, aiToolCallResponse{OK: run.Status == "completed", CallID: in.CallID,
		ToolID: binding.ToolID, Operation: run.Operation, Status: run.Status, ErrorCode: run.Error,
		Output: run.OutputMasked, LatencyMS: run.LatencyMS})
}

func decryptBrainGatewayClaims(box *secretbox.Box, token string) (brainGatewayClaims, error) {
	var claims brainGatewayClaims
	raw, err := box.Decrypt(token)
	if err != nil || json.Unmarshal([]byte(raw), &claims) != nil || claims.Version != "brain-gateway.v1" ||
		claims.ExpiresAt < time.Now().Unix() || strings.TrimSpace(claims.AccountID) == "" ||
		strings.TrimSpace(claims.DispatchID) == "" || claims.Generation < 0 {
		return brainGatewayClaims{}, ErrAIToolDenied
	}
	return claims, nil
}

func validAIToolTimestamp(timestamp int64) bool {
	if timestamp <= 0 {
		return false
	}
	delta := time.Now().Unix() - timestamp
	return delta >= -300 && delta <= 300
}

func validAIToolSignature(token string, in aiToolCallRequest) bool {
	want := aiToolSignature(token, in)
	got, err := hex.DecodeString(strings.TrimSpace(in.Signature))
	if err != nil || len(got) != sha256.Size {
		return false
	}
	decodedWant, err := hex.DecodeString(want)
	if err != nil {
		return false
	}
	return hmac.Equal(got, decodedWant)
}

func aiToolSignature(token string, in aiToolCallRequest) string {
	canonical := strings.Join([]string{strconv.FormatInt(in.Timestamp, 10), in.DispatchID,
		strconv.FormatInt(in.Generation, 10), in.BindingID, in.CallID, in.Operation}, "\n")
	hash := hmac.New(sha256.New, []byte(token))
	_, _ = hash.Write([]byte(canonical))
	return hex.EncodeToString(hash.Sum(nil))
}

func containsAIToolOperation(operations []string, operation string) bool {
	for _, candidate := range operations {
		if strings.TrimSpace(candidate) == operation {
			return true
		}
	}
	return false
}

func toolAuditPayload(in aiToolCallRequest, code string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"dispatchId": in.DispatchID, "generation": in.Generation,
		"bindingId": in.BindingID, "callId": in.CallID, "operation": in.Operation, "code": code})
	return raw
}
