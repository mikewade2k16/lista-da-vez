package omnichannel

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// ============================================================================
// Simulate (dry-run) — C9.7: roda o LLM + o motor da F8, NUNCA cria conversa/envia mensagem
// ============================================================================

// Simulate roda a triagem de verdade (com custo real, grava ai_runs e consome o limite) SEM
// tocar conversa/outbox/state, e devolve o traco: output, validacao, campos, matchedRule e
// wouldRoute (do motor da F8) — a prova de "IA sugere, motor decide" sem mandar mensagem.
func (s *AIService) Simulate(ctx context.Context, accountID string, p auth.Principal, agentID string, in SimulateInput) (SimulateView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return SimulateView{}, err
	}
	agent, err := s.assertAgentScope(ctx, accountID, agentID)
	if err != nil {
		return SimulateView{}, err
	}
	version, err := s.resolveSimVersion(ctx, accountID, agent, in.VersionID)
	if err != nil {
		return SimulateView{}, err
	}
	// Gate 4 (provider/model/key) — no simulate e erro ACIONAVEL (o operador esta testando).
	if version.Provider == "" || version.Model == "" || agent.ProviderKeyCipher == "" {
		return SimulateView{}, ErrAIProviderNotConfigured
	}
	// Gate 3 (limite) — simulate responde 409 acionavel (C9.6/C9.7).
	count, err := s.store.CountRunsThisMonth(ctx, accountID)
	if err != nil {
		return SimulateView{}, err
	}
	if err := s.limits.Check(ctx, accountID, aiRunModuleID, monthlyAIRunsKey, count); err != nil {
		return SimulateView{}, err // *modules.ErrLimitExceeded -> 409 no handler
	}

	contactName := ""
	if in.Contact != nil {
		contactName = in.Contact.Name
	}
	exec, err := s.runTriage(ctx, triageParams{
		AccountID: accountID, Agent: agent, Version: version,
		History: in.Messages, ContactName: contactName, MergeToConv: false,
	})
	if err != nil {
		return SimulateView{}, err
	}
	if exec.Outcome == dispatchProviderError {
		return SimulateView{}, ErrAIProviderNotConfigured
	}

	view := SimulateView{
		Output: jsonOrEmpty(exec.OutputJSON), SchemaVersion: version.SchemaVersion,
		Valid: exec.Valid, ValidationErrors: exec.ValidationErrors,
		ExtractedFields: exec.Output.ExtractedFields,
		Usage: SimUsage{
			PromptTokens: exec.Usage.PromptTokens, CompletionTokens: exec.Usage.CompletionTokens,
			TotalTokens: exec.Usage.TotalTokens, CostUSD: exec.CostUSD,
		},
	}
	if view.ExtractedFields == nil {
		view.ExtractedFields = map[string]any{}
	}
	if view.ValidationErrors == nil {
		view.ValidationErrors = []string{}
	}
	// So roda o motor se a saida validou (senao nao ha extracted_fields confiavel).
	if exec.Valid {
		if err := s.attachRoutingTrace(ctx, accountID, in.Messages, exec.Output, &view); err != nil {
			return SimulateView{}, err
		}
	}
	return view, nil
}

// resolveSimVersion escolhe a version a simular: versionId do body, ou a active_version_id.
// Sem nenhuma => ErrValidation (nada a testar). Version fora de escopo => 404.
func (s *AIService) resolveSimVersion(ctx context.Context, accountID string, agent agentRow, versionID string) (versionRow, error) {
	if strings.TrimSpace(versionID) != "" {
		v, err := s.store.GetVersionByID(ctx, accountID, agent.ID, versionID)
		return v, translate(err)
	}
	if agent.ActiveVersionID == nil {
		return versionRow{}, ErrValidation
	}
	v, err := s.store.GetVersionByID(ctx, accountID, agent.ID, *agent.ActiveVersionID)
	return v, translate(err)
}

// attachRoutingTrace roda o motor deterministico da F8 (Decide) com os campos extraidos e
// preenche matchedRule/wouldRoute — a prova de que a REGRA decide, nao a IA. Read-only.
func (s *AIService) attachRoutingTrace(ctx context.Context, accountID string, history []SimMessage, out TriageOutput, view *SimulateView) error {
	engine := newRoutingEngine(s.store)
	rc := RoutingContext{
		ExtractedFields: out.ExtractedFields,
		MessageText:     lastContactText(history),
	}
	decision, err := engine.Decide(ctx, accountID, rc)
	if err != nil {
		return err
	}
	if decision.RuleID != nil {
		name, priority, ok, err := s.store.RuleSummary(ctx, accountID, *decision.RuleID)
		if err != nil {
			return err
		}
		if ok {
			view.MatchedRule = &SimMatchedRule{ID: *decision.RuleID, Name: name, Priority: priority}
		}
	}
	if decision.QueueID != nil || decision.DepartmentID != nil {
		view.WouldRoute = &SimWouldRoute{DepartmentID: decision.DepartmentID, QueueID: decision.QueueID}
	}
	return nil
}

// lastContactText devolve o texto da ultima mensagem do CLIENTE (o `message.text` que o motor
// avalia). Sem mensagem do cliente => "".
func lastContactText(history []SimMessage) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "contact" {
			return history[i].Text
		}
	}
	return ""
}
