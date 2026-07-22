package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type AIToolBindingView struct {
	ID                  string          `json:"id"`
	AgentID             string          `json:"agentId"`
	ToolID              string          `json:"toolId"`
	IsEnabled           bool            `json:"isEnabled"`
	Mode                string          `json:"mode"`
	AllowedOperations   []string        `json:"allowedOperations"`
	InputSchema         json.RawMessage `json:"inputSchema"`
	OutputSchema        json.RawMessage `json:"outputSchema"`
	TimeoutMS           int             `json:"timeoutMs"`
	MaxCallsPerDispatch int             `json:"maxCallsPerDispatch"`
	Config              json.RawMessage `json:"config"`
	CreatedAt           string          `json:"createdAt"`
	UpdatedAt           string          `json:"updatedAt"`
}

type AIToolBindingInput struct {
	ToolID              string          `json:"toolId"`
	IsEnabled           *bool           `json:"isEnabled"`
	Mode                string          `json:"mode"`
	AllowedOperations   []string        `json:"allowedOperations"`
	InputSchema         json.RawMessage `json:"inputSchema"`
	OutputSchema        json.RawMessage `json:"outputSchema"`
	TimeoutMS           int             `json:"timeoutMs"`
	MaxCallsPerDispatch int             `json:"maxCallsPerDispatch"`
	Config              json.RawMessage `json:"config"`
}

type AIToolBindingPatch struct {
	IsEnabled           *bool            `json:"isEnabled"`
	Mode                *string          `json:"mode"`
	AllowedOperations   *[]string        `json:"allowedOperations"`
	InputSchema         *json.RawMessage `json:"inputSchema"`
	OutputSchema        *json.RawMessage `json:"outputSchema"`
	TimeoutMS           *int             `json:"timeoutMs"`
	MaxCallsPerDispatch *int             `json:"maxCallsPerDispatch"`
	Config              *json.RawMessage `json:"config"`
}

func validAIToolMode(mode string) bool {
	switch mode {
	case "read", "propose_write", "approved_write":
		return true
	default:
		return false
	}
}

func normalizeAIToolBindingInput(in *AIToolBindingInput) error {
	in.ToolID = strings.TrimSpace(in.ToolID)
	in.Mode = strings.ToLower(strings.TrimSpace(in.Mode))
	if in.Mode == "" {
		in.Mode = "read"
	}
	if in.TimeoutMS == 0 {
		in.TimeoutMS = 5000
	}
	if in.MaxCallsPerDispatch == 0 {
		in.MaxCallsPerDispatch = 4
	}
	if in.TimeoutMS == 0 {
		in.TimeoutMS = 5000
	}
	if in.MaxCallsPerDispatch == 0 {
		in.MaxCallsPerDispatch = 4
	}
	if in.ToolID == "" || len([]rune(in.ToolID)) > 160 || !validAIToolMode(in.Mode) || in.TimeoutMS < 100 || in.TimeoutMS > 30000 || in.MaxCallsPerDispatch < 1 || in.MaxCallsPerDispatch > 20 {
		return ErrValidation
	}
	if len(in.AllowedOperations) > 32 {
		return ErrValidation
	}
	if in.AllowedOperations == nil {
		in.AllowedOperations = []string{}
	}
	for index, op := range in.AllowedOperations {
		op = strings.TrimSpace(op)
		if op == "" || len([]rune(op)) > 160 {
			return ErrValidation
		}
		in.AllowedOperations[index] = op
	}
	if len(in.InputSchema) == 0 {
		in.InputSchema = json.RawMessage(`{}`)
	}
	if len(in.OutputSchema) == 0 {
		in.OutputSchema = json.RawMessage(`{}`)
	}
	if len(in.Config) == 0 {
		in.Config = json.RawMessage(`{}`)
	}
	if err := validateSafeJSONObject(in.InputSchema, 64000); err != nil {
		return err
	}
	if err := validateSafeJSONObject(in.OutputSchema, 64000); err != nil {
		return err
	}
	return validateSafeJSONObject(in.Config, 16000)
}

func normalizeAIToolBindingPatch(patch *AIToolBindingPatch) error {
	if patch.Mode != nil {
		value := strings.ToLower(strings.TrimSpace(*patch.Mode))
		if !validAIToolMode(value) {
			return ErrValidation
		}
		patch.Mode = &value
	}
	if patch.TimeoutMS != nil && (*patch.TimeoutMS < 100 || *patch.TimeoutMS > 30000) {
		return ErrValidation
	}
	if patch.MaxCallsPerDispatch != nil && (*patch.MaxCallsPerDispatch < 1 || *patch.MaxCallsPerDispatch > 20) {
		return ErrValidation
	}
	if patch.AllowedOperations != nil {
		if len(*patch.AllowedOperations) > 32 {
			return ErrValidation
		}
		for index, op := range *patch.AllowedOperations {
			value := strings.TrimSpace(op)
			if value == "" || len([]rune(value)) > 160 {
				return ErrValidation
			}
			(*patch.AllowedOperations)[index] = value
		}
	}
	if patch.InputSchema != nil {
		if err := validateSafeJSONObject(*patch.InputSchema, 64000); err != nil {
			return err
		}
	}
	if patch.OutputSchema != nil {
		if err := validateSafeJSONObject(*patch.OutputSchema, 64000); err != nil {
			return err
		}
	}
	if patch.Config != nil {
		if err := validateSafeJSONObject(*patch.Config, 16000); err != nil {
			return err
		}
	}
	return nil
}

func validateSafeJSONObject(raw json.RawMessage, maxBytes int) error {
	if len(raw) == 0 || len(raw) > maxBytes {
		return ErrValidation
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil || obj == nil {
		return ErrValidation
	}
	encoded, _ := json.Marshal(obj)
	if len(encoded) > maxBytes {
		return ErrValidation
	}
	return rejectSensitiveJSON(obj, 0)
}

func rejectSensitiveJSON(value any, depth int) error {
	if depth > 5 {
		return ErrValidation
	}
	switch item := value.(type) {
	case map[string]any:
		for key, nested := range item {
			lower := strings.ToLower(strings.TrimSpace(key))
			for _, marker := range []string{"apikey", "api_key", "token", "password", "secret", "credential"} {
				if strings.Contains(lower, marker) {
					return ErrValidation
				}
			}
			if err := rejectSensitiveJSON(nested, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, nested := range item {
			if err := rejectSensitiveJSON(nested, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

const aiToolBindingColumns = `b.id::text, b.agent_id::text, b.tool_id, b.is_enabled, b.mode,
	b.allowed_operations, b.input_schema, b.output_schema, b.timeout_ms, b.max_calls_per_dispatch,
	b.config, b.created_at::text, b.updated_at::text`

// ListEnabledAIToolIDs devolve somente a allowlist de identificadores lógicos
// vinculados ao agente. Configuração, credencial e permissões não entram no
// envelope do modelo.
func (s *Store) ListEnabledAIToolIDs(ctx context.Context, accountID, agentID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `select tool_id from messaging.ai_tool_bindings
		where account_id=$1::uuid and agent_id=$2::uuid and is_enabled
		order by tool_id`, accountID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var toolID string
		if err := rows.Scan(&toolID); err != nil {
			return nil, err
		}
		out = append(out, toolID)
	}
	return out, rows.Err()
}

func (s *Store) ListEnabledAIToolBindings(ctx context.Context, accountID, agentID string) (map[string]string, error) {
	rows, err := s.pool.Query(ctx, `select tool_id, id::text from messaging.ai_tool_bindings
		where account_id=$1::uuid and agent_id=$2::uuid and is_enabled
		order by tool_id`, accountID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var toolID, bindingID string
		if err := rows.Scan(&toolID, &bindingID); err != nil {
			return nil, err
		}
		out[toolID] = bindingID
	}
	return out, rows.Err()
}

func scanAIToolBinding(row rowScanner) (AIToolBindingView, error) {
	var out AIToolBindingView
	var operations []string
	if err := row.Scan(&out.ID, &out.AgentID, &out.ToolID, &out.IsEnabled, &out.Mode, &operations,
		&out.InputSchema, &out.OutputSchema, &out.TimeoutMS, &out.MaxCallsPerDispatch, &out.Config,
		&out.CreatedAt, &out.UpdatedAt); err != nil {
		return AIToolBindingView{}, err
	}
	if operations == nil {
		operations = []string{}
	}
	out.AllowedOperations = operations
	return out, nil
}

func (s *Store) ListAIToolBindings(ctx context.Context, accountID, agentID string) ([]AIToolBindingView, error) {
	rows, err := s.pool.Query(ctx, `select `+aiToolBindingColumns+` from messaging.ai_tool_bindings b
		where b.account_id=$1::uuid and b.agent_id=$2::uuid order by b.tool_id, b.id`, accountID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AIToolBindingView, 0)
	for rows.Next() {
		item, err := scanAIToolBinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CreateAIToolBinding(ctx context.Context, accountID, agentID string, in AIToolBindingInput) (AIToolBindingView, error) {
	active := false
	if in.IsEnabled != nil {
		active = *in.IsEnabled
	}
	return scanAIToolBinding(s.pool.QueryRow(ctx, `insert into messaging.ai_tool_bindings
		(account_id,agent_id,tool_id,is_enabled,mode,allowed_operations,input_schema,output_schema,timeout_ms,max_calls_per_dispatch,config)
		values ($1::uuid,$2::uuid,$3,$4,$5,$6::jsonb,$7::jsonb,$8::jsonb,$9,$10,$11::jsonb)
		returning `+aiToolBindingColumns, accountID, agentID, in.ToolID, active, in.Mode,
		jsonArray(in.AllowedOperations), in.InputSchema, in.OutputSchema, in.TimeoutMS, in.MaxCallsPerDispatch, in.Config))
}

func (s *Store) UpdateAIToolBinding(ctx context.Context, accountID, agentID, id string, patch AIToolBindingPatch) (AIToolBindingView, error) {
	var operations any
	if patch.AllowedOperations != nil {
		operations = jsonArray(*patch.AllowedOperations)
	}
	return scanAIToolBinding(s.pool.QueryRow(ctx, `update messaging.ai_tool_bindings b set
		is_enabled=coalesce($4,is_enabled), mode=coalesce($5,mode), allowed_operations=coalesce($6::jsonb,allowed_operations),
		input_schema=coalesce($7::jsonb,input_schema), output_schema=coalesce($8::jsonb,output_schema),
		timeout_ms=coalesce($9,timeout_ms), max_calls_per_dispatch=coalesce($10,max_calls_per_dispatch),
		config=coalesce($11::jsonb,config), updated_at=now()
		where b.account_id=$1::uuid and b.agent_id=$2::uuid and b.id=$3::uuid returning `+aiToolBindingColumns,
		accountID, agentID, id, patch.IsEnabled, patch.Mode, operations, nullableRaw(patch.InputSchema), nullableRaw(patch.OutputSchema),
		patch.TimeoutMS, patch.MaxCallsPerDispatch, nullableRaw(patch.Config)))
}

func (s *Store) DisableAIToolBinding(ctx context.Context, accountID, agentID, id string) error {
	tag, err := s.pool.Exec(ctx, `update messaging.ai_tool_bindings set is_enabled=false, updated_at=now()
		where account_id=$1::uuid and agent_id=$2::uuid and id=$3::uuid`, accountID, agentID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func jsonArray(values []string) []byte {
	if values == nil {
		values = []string{}
	}
	raw, _ := json.Marshal(values)
	return raw
}
func nullableRaw(raw *json.RawMessage) any {
	if raw == nil {
		return nil
	}
	return *raw
}

func (s *AIService) ListAIToolBindings(ctx context.Context, accountID string, p auth.Principal, agentID string) ([]AIToolBindingView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return nil, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(agentID)) {
		return nil, ErrValidation
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return nil, err
	}
	return s.store.ListAIToolBindings(ctx, accountID, agentID)
}

func (s *AIService) CreateAIToolBinding(ctx context.Context, accountID string, p auth.Principal, agentID string, in AIToolBindingInput) (AIToolBindingView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AIToolBindingView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(agentID)) {
		return AIToolBindingView{}, ErrValidation
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return AIToolBindingView{}, err
	}
	if err := normalizeAIToolBindingInput(&in); err != nil {
		return AIToolBindingView{}, err
	}
	out, err := s.store.CreateAIToolBinding(ctx, accountID, agentID, in)
	if isUniqueViolation(err) {
		return AIToolBindingView{}, ErrConflict
	}
	return out, err
}

func (s *AIService) UpdateAIToolBinding(ctx context.Context, accountID string, p auth.Principal, agentID, bindingID string, patch AIToolBindingPatch) (AIToolBindingView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AIToolBindingView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(agentID)) || !omnichannelUUIDPattern.MatchString(strings.TrimSpace(bindingID)) {
		return AIToolBindingView{}, ErrValidation
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return AIToolBindingView{}, err
	}
	if err := normalizeAIToolBindingPatch(&patch); err != nil {
		return AIToolBindingView{}, err
	}
	out, err := s.store.UpdateAIToolBinding(ctx, accountID, agentID, bindingID, patch)
	if errors.Is(err, pgx.ErrNoRows) {
		return AIToolBindingView{}, ErrNotFound
	}
	return out, translate(err)
}

func (s *AIService) DeleteAIToolBinding(ctx context.Context, accountID string, p auth.Principal, agentID, bindingID string) error {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(agentID)) || !omnichannelUUIDPattern.MatchString(strings.TrimSpace(bindingID)) {
		return ErrValidation
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return err
	}
	return s.store.DisableAIToolBinding(ctx, accountID, agentID, bindingID)
}
