package finance

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func (s *PlusService) Authorize(ctx context.Context, sc PlusScope) error {
	if s == nil || s.assistant == nil || s.ports.Authorize == nil {
		return plusError("capability_unavailable", 503)
	}
	if !isUUID(sc.AccountID) || !isUUID(sc.ActorID) {
		return plusError("unauthorized", 401)
	}
	if e := s.ports.Authorize(ctx, sc, "finance.sheets.view"); e != nil {
		return e
	}
	cfg, e := s.assistant.Config(ctx, sc.AccountID)
	if e != nil {
		return e
	}
	if !cfg.Plus.Enabled {
		return plusError("plus_disabled", 403)
	}
	if sc.IssuerID != "" {
		if !isUUID(sc.IssuerID) || s.ports.Accounting == nil {
			return plusError("not_found", 404)
		}
		if _, e = s.ports.Accounting(ctx, sc, PlusAction{ToolID: "accounting.issuer.read.v1", Input: json.RawMessage(`{}`)}); e != nil {
			return e
		}
	}
	return nil
}
func (s *PlusService) Catalog(ctx context.Context, sc PlusScope) ([]PlusTool, error) {
	if e := s.Authorize(ctx, sc); e != nil {
		return nil, e
	}
	out := []PlusTool{}
	for _, tool := range plusTools() {
		if sc.IssuerID != "" && strings.HasPrefix(tool.ID, "finance.personal.") {
			continue
		}
		if strings.HasPrefix(tool.Permission, "accounting.") && sc.IssuerID == "" {
			continue
		}
		if tool.ID == "knowledge.search.v1" && s.ports.Knowledge == nil {
			continue
		}
		if s.ports.Authorize(ctx, sc, tool.Permission) == nil {
			out = append(out, tool)
		}
	}
	return out, nil
}
func (s *PlusService) Execute(ctx context.Context, sc PlusScope, in PlusInvocation) (PlusResult, error) {
	out := PlusResult{Status: "missing_data", Message: "Escolha a operação e os dados necessários para continuar.", Data: json.RawMessage(`{}`), Evidence: []json.RawMessage{}, Proposals: []json.RawMessage{}, Tools: []PlusToolRun{}}
	if e := s.Authorize(ctx, sc); e != nil {
		return out, e
	}
	action, reason := routePlus(in)
	if action != nil && action.ToolID == "finance.due_dates.read.v1" && sc.IssuerID != "" {
		action = &PlusAction{ToolID: "accounting.obligations.read.v1", Input: plusJSON(map[string]string{"period": in.Month})}
		reason = "obligations_requested"
	}
	out.RoutingReason = reason
	out.Intent = reason
	if action == nil {
		return out, nil
	}
	switch action.ToolID {
	case "finance.due_dates.read.v1":
		out.Status = "capability_unavailable"
		out.Message = "As linhas da planilha não possuem vencimento individual. Data de efetivação não é vencimento."
		return out, nil
	case "plus.explanation.v1":
		return s.explain(ctx, sc, in)
	}
	var tool *PlusTool
	for _, definition := range plusTools() {
		if definition.ID == action.ToolID {
			value := definition
			tool = &value
			break
		}
	}
	if tool == nil {
		return out, plusError("tool_not_registered", 400)
	}
	if tool.Mode == "prepare" && reason != "structured_action" && action.ToolID != "finance.personal.prepare.v1" && action.ToolID != "finance.personal.guide.v1" {
		return out, plusError("explicit_intent_required", 400)
	}
	started := time.Now()
	auditStatus := "denied"
	defer func() {
		if s.logger != nil {
			s.logger.Info("finance_plus_tool", "account_id", sc.AccountID, "request_id", sc.RequestID, "tool", tool.ID, "mode", tool.Mode, "status", auditStatus, "duration_ms", time.Since(started).Milliseconds())
		}
	}()
	if e := s.ports.Authorize(ctx, sc, tool.Permission); e != nil {
		return out, e
	}
	if strings.HasPrefix(tool.Permission, "accounting.") && sc.IssuerID == "" {
		return out, nil
	}
	if len(action.Input) > 12*1024 {
		return out, plusError("invalid_input", 400)
	}
	auditStatus = "failed"
	call, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var data json.RawMessage
	var e error
	switch action.ToolID {
	case "finance.sheet.read.v1":
		data, e = s.readSheet(call, sc, action.Input)
	case "finance.personal.prepare.v1", "finance.personal.guide.v1":
		if sc.IssuerID != "" {
			return out, plusError("personal_scope_required", 409)
		}
		data, e = s.personal(call, sc, *action)
	case "knowledge.search.v1":
		if s.ports.Knowledge == nil {
			out.Status = "capability_unavailable"
			out.Message = "A busca de fontes revisadas ainda não foi configurada para este emitente."
			return out, nil
		}
		data, e = s.ports.Knowledge(call, sc, *action)
	default:
		if s.ports.Accounting == nil {
			out.Status = "capability_unavailable"
			return out, nil
		}
		data, e = s.ports.Accounting(call, sc, *action)
	}
	if e != nil {
		var domain *PlusError
		if errors.As(e, &domain) && domain.Status == 422 {
			out.Message = "Complete os dados necessários para esta operação."
			if domain.Code == "sheet_required" || domain.Code == "sheet_period_mismatch" {
				out.Message = "Selecione uma planilha do período solicitado no Finance."
			}
			out.Data = plusJSON(map[string]string{"reason": domain.Code})
			auditStatus = out.Status
			return out, nil
		}
		return out, e
	}
	out.Status = "completed"
	out.Message = "Consulta concluída. Confira os dados retornados."
	if tool.Mode == "prepare" {
		out.Message = "Preparação concluída. Confira os dados e pendências; nenhuma emissão foi autorizada."
	}
	if len(data) > tool.MaxOutputBytes {
		data = plusJSON(map[string]any{"truncated": true, "message": "Abra o recurso para consultar o resultado completo."})
	}
	var pending struct {
		Pending []json.RawMessage `json:"pending"`
	}
	if json.Unmarshal(data, &pending) == nil && len(pending.Pending) > 0 {
		out.Status = "missing_data"
		out.Message = "Confira os dados retornados e complete as pendências antes da revisão."
	}
	if action.ToolID == "knowledge.search.v1" {
		var retrieval struct {
			Status   string            `json:"status"`
			Message  string            `json:"message"`
			Evidence []json.RawMessage `json:"evidence"`
		}
		if json.Unmarshal(data, &retrieval) == nil && retrieval.Status != "" {
			out.Status = retrieval.Status
			out.Message = retrieval.Message
			out.Evidence = retrieval.Evidence
		}
	}
	auditStatus = out.Status
	out.Data = data
	out.Tools = append(out.Tools, PlusToolRun{ID: tool.ID, Mode: tool.Mode, Status: out.Status, DurationMS: time.Since(started).Milliseconds()})

	return out, nil
}
func (s *PlusService) personal(ctx context.Context, sc PlusScope, action PlusAction) (json.RawMessage, error) {
	if s.assistant.access == nil {
		return nil, plusError("capability_unavailable", 503)
	}
	var draft, text string
	if action.ToolID == "finance.personal.prepare.v1" {
		var in struct {
			DraftID string `json:"draftId"`
		}
		if e := strictPlus(action.Input, &in); e != nil {
			return nil, e
		}
		if !isUUID(in.DraftID) {
			return nil, plusError("missing_data", 422)
		}
		draft = in.DraftID
		text = "Preparar nota"
	} else {
		var in struct {
			Text string `json:"text"`
		}
		if e := strictPlus(action.Input, &in); e != nil {
			return nil, e
		}
		switch normalizedDocumentLabel(in.Text) {
		case "abrir perfil", "cadastro fiscal":
			text = "abrir perfil"
		case "cadastrar acesso":
			text = "cadastrar acesso"
		case "anexar comprovante":
			text = "anexar comprovante"
		default:
			return nil, plusError("invalid_input", 400)
		}
	}
	value, e := s.assistant.access.Send(ctx, sc.AccountID, sc.ActorID, AccessConversationInput{RequestID: sc.RequestID, DraftID: draft, Text: text})
	if e != nil {
		return nil, e
	}
	// Existing personal messages remain in their original history. Return only the current card.
	if len(value.Messages) == 0 {
		return plusJSON(map[string]any{"action": "document", "missingData": true}), nil
	}
	last := value.Messages[len(value.Messages)-1]
	return plusJSON(map[string]any{"action": last.Action, "message": last.Text, "draftId": last.DraftID}), nil
}
