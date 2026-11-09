package finance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
	"io"
	"log/slog"
)

type PlusConfig struct {
	Explanation ExplanationConfig `json:"explanation"`

	Enabled           bool `json:"enabled"`
	ToolPolicyVersion int  `json:"toolPolicyVersion"`
}
type PlusScope struct{ AccountID, ActorID, IssuerID, RequestID string }
type PlusAction struct {
	ToolID string          `json:"toolId"`
	Input  json.RawMessage `json:"input"`
}
type PlusInvocation struct {
	CompetenceDate string      `json:"competenceDate,omitempty"`
	Question       string      `json:"question"`
	FinanceSheetID string      `json:"financeSheetId,omitempty"`
	FinanceDraftID string      `json:"financeDraftId,omitempty"`
	DiagnosisID    string      `json:"diagnosisId,omitempty"`
	DocumentID     string      `json:"documentId,omitempty"`
	Month          string      `json:"month,omitempty"`
	Action         *PlusAction `json:"action,omitempty"`
}
type PlusError struct {
	Code   string
	Status int
}

func (e *PlusError) Error() string            { return e.Code }
func plusError(code string, status int) error { return &PlusError{code, status} }

type PlusPorts struct {
	PublicKnowledge        func(context.Context, []string) (json.RawMessage, error)
	ValidatePublicEvidence func(context.Context, []string) error

	LLM              llm.Client
	ValidateEvidence func(context.Context, PlusScope, []string) error

	Authorize  func(context.Context, PlusScope, string) error
	Accounting func(context.Context, PlusScope, PlusAction) (json.RawMessage, error)
	Knowledge  func(context.Context, PlusScope, PlusAction) (json.RawMessage, error)
}
type PlusResult struct {
	Intent        string            `json:"intent"`
	Status        string            `json:"status"`
	Message       string            `json:"message"`
	Data          json.RawMessage   `json:"data"`
	Evidence      []json.RawMessage `json:"evidence"`
	Proposals     []json.RawMessage `json:"proposals"`
	Tools         []PlusToolRun     `json:"tools"`
	RoutingReason string            `json:"routingReason"`
}
type PlusToolRun struct {
	ID         string `json:"id"`
	Mode       string `json:"mode"`
	Status     string `json:"status"`
	DurationMS int64  `json:"durationMs"`
}
type PlusTool struct {
	ID             string          `json:"id"`
	Version        int             `json:"version"`
	Mode           string          `json:"mode"`
	Permission     string          `json:"permission"`
	InputSchema    json.RawMessage `json:"inputSchema"`
	OutputSchema   json.RawMessage `json:"outputSchema"`
	TimeoutMS      int             `json:"timeoutMs"`
	MaxOutputBytes int             `json:"maxOutputBytes"`
	Confirmation   string          `json:"confirmation"`
}
type PlusService struct {
	assistant *AssistantService
	ports     PlusPorts
	logger    *slog.Logger
}

func NewPlusService(assistant *AssistantService, ports PlusPorts, logger *slog.Logger) *PlusService {
	return &PlusService{assistant: assistant, ports: ports, logger: logger}
}
func (m *Module) WithPlusPorts(ports PlusPorts) *Module { m.plusPorts = ports; return m }
func (m *Module) Plus() *PlusService {
	if m.handle == nil {
		return nil
	}
	return m.handle.plus
}
func strictPlus(raw json.RawMessage, out any) error {
	if len(raw) == 0 || len(raw) > 12*1024 {
		return plusError("invalid_input", 400)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		return plusError("invalid_input", 400)
	}
	if e := d.Decode(new(any)); !errors.Is(e, io.EOF) {
		return plusError("invalid_input", 400)
	}
	return nil
}
func plusJSON(value any) json.RawMessage { raw, _ := json.Marshal(value); return raw }
