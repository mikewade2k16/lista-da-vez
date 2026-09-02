package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// ============================================================================
// F9 — Triagem IA no Go: dispatch (gates C9.6) + chamada ao LLM + persistencia (C9.4)
// ============================================================================
//
// "A IA SUGERE; o motor da F8 DECIDE." Este service monta o prompt, chama o client LLM
// NATIVO (platform/llm — F3), valida a saida contra o schema versionado, grava ai_runs e
// (no inbound) funde os extracted_fields na conversa. NENHUM caminho escreve queue_id a
// partir do output do modelo — quem roteia e o routing_engine lendo routing_rules.
//
// O client LLM vem da F3: consumir, NAO redefinir. Provider/modelo/prompt/chave vem do
// PAINEL/BANCO (config da conta), NUNCA de env, NUNCA supostos (feedback_ai_config_from_panel).

// historyWindow e o tamanho da janela da camada 7 (historico) do prompt.
const historyWindow = 20

// AIService concentra as regras de IA (triagem + management). Dependencias por construtor
// (sem globais). box cifra/decifra a chave do provider; limits le monthly_ai_runs (F3).
type AIService struct {
	store               *Store
	llm                 llm.Client
	brain               brainExecutor
	box                 *secretbox.Box
	limits              *modules.LimitReader
	logger              *slog.Logger
	businessContext     AutomationBusinessContextProvider
	externalEffectLease func(context.Context, string, string, int64, func() error) (bool, error)
}

type AIServiceOption func(*AIService)

func WithBrainExecutor(executor brainExecutor) AIServiceOption {
	return func(s *AIService) { s.brain = executor }
}

func WithAIBusinessContext(provider AutomationBusinessContextProvider) AIServiceOption {
	return func(s *AIService) { s.businessContext = provider }
}

// NewAIService monta o service. Nenhuma dependencia e opcional em producao — o wiring
// (RegisterAIRoutes) injeta todas. logger nil => slog.Default().
func NewAIService(store *Store, client llm.Client, box *secretbox.Box, limits *modules.LimitReader, logger *slog.Logger, opts ...AIServiceOption) *AIService {
	if logger == nil {
		logger = slog.Default()
	}
	svc := &AIService{store: store, llm: client, box: box, limits: limits, logger: logger}
	if store != nil {
		svc.externalEffectLease = store.WithAIDispatchExternalEffectLease
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

// triageExec e o resultado interno de uma execucao de triagem (o run ja gravado). Carrega o
// suficiente para o inbound (Outcome/Output) E para o simulate (OutputJSON/Valid/erros/usage).
type triageExec struct {
	Outcome          DispatchOutcome
	Output           TriageOutput
	OutputJSON       json.RawMessage
	Valid            bool
	ValidationErrors []string
	Usage            llm.Usage
	CostUSD          float64
	RunID            string
}

// Run satisfaz o contrato C9.4 (TriageService). Devolve a SUGESTAO quando a triagem roda; em
// qualquer gate/erro devolve erro (o INBOUND deve preferir Dispatch, que degrada sem erro).
func (s *AIService) Run(ctx context.Context, in TriageInput) (TriageOutput, error) {
	res, err := s.Dispatch(ctx, in)
	if err != nil {
		return TriageOutput{}, err
	}
	if res.Outcome == dispatchTriaged {
		return res.Output, nil
	}
	return TriageOutput{}, fmt.Errorf("omnichannel: triagem nao executou (%s)", res.Outcome)
}

// Dispatch e a entrada do INBOUND: aplica os gates da C9.6 na ORDEM e, passando todos, roda a
// triagem. NUNCA devolve erro que trave a conversa — o resultado vem no Outcome, e o caller
// (service_inbound, via wiring) degrada para o fallback deterministico da F8 em qualquer
// outcome != triaged. Erro devolvido = falha de infraestrutura (banco), que o caller loga.
func (s *AIService) Dispatch(ctx context.Context, in TriageInput) (DispatchResult, error) {
	conv, err := s.store.ConvTriageContext(ctx, in.AccountID, in.ConversationID)
	if err != nil {
		return DispatchResult{}, err
	}
	if !conv.Found {
		// Conversa inexistente/fora de escopo: sem agente a que atribuir, sem run.
		return DispatchResult{Outcome: dispatchNoAgent}, nil
	}
	// Grupos WhatsApp nunca sao atendidos pela IA. Este gate e deterministico e precede
	// provider, prompt, ferramentas e ForceReply.
	if isWhatsAppGroupExternalID(conv.ExternalID) {
		return DispatchResult{Outcome: dispatchNoAgent, AIGeneration: conv.AIGeneration}, nil
	}
	contactBlocked, err := s.store.IsConversationAIBlocked(ctx, in.AccountID, in.ConversationID)
	if err != nil {
		return DispatchResult{}, err
	}
	if contactBlocked {
		runID := s.recordGateRun(ctx, in, gateRun{Status: runBlocked, Error: "contact_ai_blocked"})
		return DispatchResult{Outcome: dispatchContactBlocked, RunID: runID, AIGeneration: conv.AIGeneration,
			ReasonCode: HandoffReasonPolicy}, nil
	}

	// Resolve o agente ativo antes do gate 1 para que o run `blocked` referencie o agente
	// quando ele existe. O gate 1 (state) MANTEM a precedencia: human_active bloqueia mesmo
	// com agente ativo (reconciliacao da ordem C9.6; documentada).
	agent, hasAgent, err := s.store.ActiveAgentForInstance(ctx, in.AccountID, deref(conv.InstanceID))
	if err != nil {
		return DispatchResult{}, err
	}

	// Gate 1 — state nao permite IA (human_active/pending/queued/routing/closed): HARD-BLOCK.
	if !AIAllowed(State(conv.State)) {
		var agentID *string
		if hasAgent {
			agentID = &agent.ID
		}
		runID := s.recordGateRun(ctx, in, gateRun{AgentID: agentID, Status: runBlocked})
		return DispatchResult{Outcome: dispatchBlocked, RunID: runID}, nil
	}

	// Gate 2 — sem agente habilitado com version ativa: nao roda, sem ai_runs.
	if !hasAgent {
		return DispatchResult{Outcome: dispatchNoAgent}, nil
	}

	version, err := s.store.GetVersionByID(ctx, in.AccountID, agent.ID, deref(agent.ActiveVersionID))
	if err != nil {
		// active_version_id apontando para version inexistente e inconsistencia: trata como
		// sem agente (nao chama modelo, sem run) e degrada.
		return DispatchResult{Outcome: dispatchNoAgent}, translate(err)
	}

	// Gate 4 — provider/modelo/chave ausentes: nao roda; grava provider_error.
	providerKey, providerKeyErr := s.versionAPIKey(ctx, in.AccountID, agent, version)
	if version.Provider == "" || version.Model == "" || providerKeyErr != nil || providerKey == "" {
		runID := s.recordGateRun(ctx, in, gateRun{
			AgentID: &agent.ID, VersionID: &version.ID, Provider: version.Provider,
			Model: version.Model, SchemaVersion: version.SchemaVersion,
			Status: runProviderError, Error: "provider/model/key ausente",
		})
		if !version.HandoffOnError {
			return DispatchResult{Outcome: dispatchNoReply, RunID: runID}, nil
		}
		return DispatchResult{Outcome: dispatchProviderError, RunID: runID}, nil
	}

	// Gate 3 — limite mensal estourado: NAO chama o modelo; grava limit_exceeded. O INBOUND
	// degrada para o fallback (nao ha a quem devolver 409); so o simulate responde 409.
	count, err := s.store.CountRunsThisMonth(ctx, in.AccountID)
	if err != nil {
		return DispatchResult{}, err
	}
	if err := s.limits.Check(ctx, in.AccountID, aiRunModuleID, monthlyAIRunsKey, count); err != nil {
		if modules.IsLimitExceeded(err) {
			runID := s.recordGateRun(ctx, in, gateRun{
				AgentID: &agent.ID, VersionID: &version.ID, Provider: version.Provider,
				Model: version.Model, SchemaVersion: version.SchemaVersion, Status: runLimitExceeded,
			})
			if !version.HandoffOnLimit {
				return DispatchResult{Outcome: dispatchNoReply, RunID: runID, AIGeneration: conv.AIGeneration,
					ReasonCode: HandoffReasonPolicy}, nil
			}
			return DispatchResult{Outcome: dispatchLimitExceeded, RunID: runID, AIGeneration: conv.AIGeneration,
				ReasonCode: HandoffReasonPolicy}, nil
		}
		return DispatchResult{}, err
	}

	// Gate de turnos por conversa: respostas já persistidas/enfileiradas contam, mas uma
	// mensagem FAILED não consome o teto. A policy escolhe handoff ou silêncio; nunca se
	// cria uma segunda resposta local para “compensar” o limite.
	aiTurns, err := s.store.CountAIOutboundTurns(ctx, in.AccountID, in.ConversationID)
	if err != nil {
		return DispatchResult{}, err
	}
	if aiTurnLimitReached(in.ForceReply, version.MaxAITurns, aiTurns) {
		runID := s.recordGateRun(ctx, in, gateRun{
			AgentID: &agent.ID, VersionID: &version.ID, Provider: version.Provider,
			Model: version.Model, SchemaVersion: version.SchemaVersion,
			Status: runLimitExceeded, Error: "max_ai_turns",
		})
		if !version.HandoffOnLimit {
			return DispatchResult{Outcome: dispatchNoReply, RunID: runID, AIGeneration: conv.AIGeneration,
				ReasonCode: HandoffReasonMaxTurns}, nil
		}
		return DispatchResult{Outcome: dispatchLimitExceeded, RunID: runID, AIGeneration: conv.AIGeneration,
			ReasonCode: HandoffReasonMaxTurns}, nil
	}

	window := historyWindow
	if version.MaxContextMessages > 0 {
		window = version.MaxContextMessages
	}
	history, err := s.store.RecentMessages(ctx, in.AccountID, in.ConversationID, window)
	if err != nil {
		return DispatchResult{}, err
	}
	contactContext := map[string]any{}
	_ = json.Unmarshal(conv.ExtractedFields, &contactContext)
	var contactIntelligence *ContactIntelligenceView
	safeContactName := ""
	if conv.ContactID != nil {
		loaded, loadErr := s.store.GetOperationalContactIntelligence(
			ctx, in.AccountID, *conv.ContactID, in.ConversationID,
		)
		if loadErr != nil {
			return DispatchResult{}, loadErr
		}
		contactIntelligence = &loaded
		if loaded.PreferredName != nil {
			safeContactName = *loaded.PreferredName
		}
	}
	var businessContext *AutomationBusinessContext
	if s.businessContext != nil {
		clientID, configured, lookupErr := s.store.AutomationClientForInstance(ctx, in.AccountID, deref(conv.InstanceID))
		if lookupErr != nil {
			return DispatchResult{}, lookupErr
		}
		if configured {
			profile, available, loadErr := s.businessContext.Load(ctx, in.AccountID, clientID)
			if loadErr != nil {
				s.logger.Warn("omnichannel_business_context_unavailable", "account_id", in.AccountID)
			} else if available {
				businessContext = &profile
			}
		}
	}

	exec, err := s.runTriage(ctx, triageParams{
		AccountID:           in.AccountID,
		ConversationID:      &in.ConversationID,
		MessageID:           nilIfEmpty(in.MessageID),
		Agent:               agent,
		Version:             version,
		History:             history,
		ContactName:         safeContactName,
		ContactContext:      contactContext,
		ContactIntelligence: contactIntelligence,
		ContactID:           deref(conv.ContactID),
		Channel:             conv.Channel,
		ConversationState:   conv.State,
		MergeToConv:         true,
		AIGeneration:        conv.AIGeneration,
		DispatchID:          in.DispatchID,
		ForceReply:          in.ForceReply,
		BusinessContext:     businessContext,
	})
	if err != nil {
		return DispatchResult{}, err
	}
	if exec.Outcome != dispatchTriaged {
		if !version.HandoffOnError && (exec.Outcome == dispatchProviderError || exec.Outcome == dispatchSchemaInvalid) {
			return DispatchResult{Outcome: dispatchNoReply, RunID: exec.RunID, AIGeneration: conv.AIGeneration}, nil
		}
		return DispatchResult{Outcome: exec.Outcome, Output: exec.Output, RunID: exec.RunID, AIGeneration: conv.AIGeneration}, nil
	}
	if confidenceBelowReplyMinimum(in.ForceReply, exec.Output.Confidence, version.MinConfidence) {
		if !version.HandoffOnLimit {
			return DispatchResult{Outcome: dispatchNoReply, RunID: exec.RunID, AIGeneration: conv.AIGeneration,
				ReasonCode: HandoffReasonLowConfidence}, nil
		}
		return DispatchResult{Outcome: dispatchLimitExceeded, RunID: exec.RunID, AIGeneration: conv.AIGeneration,
			ReasonCode: HandoffReasonLowConfidence}, nil
	}
	return DispatchResult{Outcome: exec.Outcome, Output: exec.Output, RunID: exec.RunID, AIGeneration: conv.AIGeneration}, nil
}

func aiTurnLimitReached(forceReply bool, maximum, current int) bool {
	return !forceReply && maximum > 0 && current >= maximum
}

func confidenceBelowReplyMinimum(forceReply bool, actual, minimum float64) bool {
	return !forceReply && actual < minimum
}

// triageParams sao os parametros de uma execucao de triagem (inbound OU simulate).
type triageParams struct {
	AccountID           string
	ConversationID      *string // nil no simulate (NUNCA cria conversa)
	MessageID           *string
	Agent               agentRow
	Version             versionRow
	History             []SimMessage
	ContactName         string
	ContactContext      map[string]any
	ContactIntelligence *ContactIntelligenceView
	ContactID           string
	Channel             string
	ConversationState   string
	// MergeToConv=true funde os extracted_fields na conversa (inbound). Falso no simulate.
	MergeToConv bool
	// AIGeneration e a lease capturada antes da chamada ao modelo. Zero e valido.
	AIGeneration int64
	// DispatchID is present for the durable brain.v2 worker only.
	DispatchID      string
	ForceReply      bool
	BusinessContext *AutomationBusinessContext
}

// runTriage monta o prompt (8 camadas), chama o LLM com o schema versionado (1 retry no
// schema_invalid), grava o ai_runs e — se MergeToConv — funde os campos na conversa. Devolve
// o resultado rico. Erro devolvido = falha de infra (banco); o resto vira Outcome + run.
func (s *AIService) runTriage(ctx context.Context, p triageParams) (triageExec, error) {
	p.ContactName, _ = safePreferredPersonalName(p.ContactName)
	catalog, err := s.store.RoutingCatalog(ctx, p.AccountID)
	if err != nil {
		return triageExec{}, err
	}
	fields, err := s.store.ListCollectFields(ctx, p.AccountID, p.Agent.ID)
	if err != nil {
		return triageExec{}, err
	}
	// input MASCARADO (§10): estrutura, nunca o texto cru das mensagens (PII). Computado uma
	// vez porque todo run (inclusive os de falha) grava a mesma referencia de entrada.
	maskedInput := maskedTriageInput(fields, len(p.History))
	empty := json.RawMessage(`{}`)

	apiKey, err := s.versionAPIKey(ctx, p.AccountID, p.Agent, p.Version)
	if err != nil {
		// Chave adulterada/chave-mestra errada: nao vaza o ciphertext, so a classe do erro.
		run := s.persistRun(ctx, p, runProviderError, maskedInput, empty, llm.Usage{}, 0, 0, "decrypt falhou")
		return triageExec{Outcome: dispatchProviderError, RunID: run}, nil
	}

	schemaDef := p.Version.OutputSchema
	if len(schemaDef) == 0 || string(schemaDef) == "{}" || string(schemaDef) == "null" {
		schemaDef = defaultOutputSchema()
	}
	schemaDef = withContactMemoryOutputSchema(schemaDef)
	req := llm.Request{
		Provider:     p.Version.Provider,
		Model:        p.Version.Model,
		Temperature:  p.Version.Temperature,
		SystemPrompt: buildSystemPrompt(parseLayers(p.Version.Layers), catalog, fields, p.Version.SchemaVersion),
		UserPrompt:   buildUserPromptWithContactIntelligence(p.History, p.ContactName, p.ContactContext, p.BusinessContext, p.ContactIntelligence),
		Schema:       &llm.Schema{Name: "omnichannel_triage", Version: 1, Definition: schemaDef},
		APIKey:       apiKey,
		AccountID:    p.AccountID,
	}
	if p.ForceReply {
		req.SystemPrompt = appendOperatorForceReplyInstructions(req.SystemPrompt)
	}

	if s.brain != nil && strings.TrimSpace(p.DispatchID) != "" {
		brainRequest := buildBrainRequestV2(p, fields)
		brainRequest.Capabilities.Tools, err = s.store.ListEnabledAIToolIDs(ctx, p.AccountID, p.Agent.ID)
		if err != nil {
			return triageExec{}, err
		}
		toolBindings, err := s.store.ListEnabledAIToolBindings(ctx, p.AccountID, p.Agent.ID)
		if err != nil {
			return triageExec{}, err
		}
		executionSystemPrompt := appendAIToolInstructions(req.SystemPrompt, brainRequest.Capabilities.Tools)
		executionSchema := schemaDef
		if len(brainRequest.Capabilities.Tools) > 0 {
			executionSchema = toolAwareBrainOutputSchema()
		}
		execution := BrainExecutionV2{
			Provider: p.Version.Provider, Model: p.Version.Model, Temperature: p.Version.Temperature,
			SystemPrompt: executionSystemPrompt, UserPrompt: req.UserPrompt, OutputSchema: executionSchema,
			ToolBindings: toolBindings, APIKey: apiKey,
		}
		// O lease exterior impede que prompt/PII antigo seja entregue ao n8n depois de um
		// reset. Gateways chamados pelo workflow usam a variante NOWAIT: um reset que entrou
		// entre o POST e o callback interno falha fechado sem formar ciclo distribuido.
		result, usage, latency, brainErr := s.completeBrainWithLease(ctx, p, brainRequest, execution)
		if errors.Is(brainErr, ErrHistoryResetInvalidated) {
			return triageExec{}, brainErr
		}
		if errors.Is(brainErr, ErrBrainSchemaInvalid) {
			// The workflow/model gets one deterministic retry, matching the native path.
			result, usage, latency, brainErr = s.completeBrainWithLease(ctx, p, brainRequest, execution)
			if errors.Is(brainErr, ErrHistoryResetInvalidated) {
				return triageExec{}, brainErr
			}
		}
		if errors.Is(brainErr, ErrBrainSchemaInvalid) {
			run := s.persistRun(ctx, p, runSchemaInvalid, maskedInput, empty, usage, latency, 0, "brain.result.v2 invalido")
			return triageExec{Outcome: dispatchSchemaInvalid, RunID: run, Valid: false, ValidationErrors: []string{ErrBrainSchemaInvalid.Error()}, OutputJSON: empty}, nil
		}
		if brainErr != nil {
			run := s.persistRun(ctx, p, runProviderError, maskedInput, empty, usage, latency, 0, "executor brain indisponivel")
			return triageExec{Outcome: dispatchProviderError, RunID: run}, nil
		}
		out := triageOutputFromBrainResult(result)
		outputJSON, _ := json.Marshal(out)
		cost := s.cost(ctx, p.Version.Provider, p.Version.Model, usage)
		run := s.persistRun(ctx, p, runOK, maskedInput, outputJSON, usage, latency, cost, "")
		var committed bool
		run, out, committed, err = s.applyExtracted(ctx, p, run, out)
		if err != nil {
			return triageExec{}, err
		}
		if !committed {
			return triageExec{Outcome: dispatchBlocked, RunID: run}, nil
		}
		return triageExec{Outcome: dispatchTriaged, Output: out, OutputJSON: outputJSON, Valid: true,
			Usage: usage, CostUSD: cost, RunID: run}, nil
	}

	var resp llm.Response
	var callErr error
	if err := s.withAIDispatchExternalEffectLease(ctx, p, func() error {
		resp, callErr = s.llm.Complete(ctx, req)
		return nil
	}); err != nil {
		return triageExec{}, err
	}
	if errors.Is(callErr, llm.ErrSchemaViolation) {
		// 1 retry (C9.3): o modelo pode acertar o formato na segunda.
		if err := s.withAIDispatchExternalEffectLease(ctx, p, func() error {
			resp, callErr = s.llm.Complete(ctx, req)
			return nil
		}); err != nil {
			return triageExec{}, err
		}
	}

	switch {
	case errors.Is(callErr, llm.ErrSchemaViolation):
		run := s.persistRun(ctx, p, runSchemaInvalid, maskedInput, empty, llm.Usage{}, 0, 0, callErr.Error())
		return triageExec{
			Outcome: dispatchSchemaInvalid, RunID: run,
			Valid: false, ValidationErrors: []string{callErr.Error()}, OutputJSON: empty,
		}, nil
	case callErr != nil:
		// ErrKeyMissing/ErrInvalidProvider/ErrInvalidModel/ErrBaseURLNotAllowed/provider fora.
		run := s.persistRun(ctx, p, runProviderError, maskedInput, empty, llm.Usage{}, 0, 0, callErr.Error())
		return triageExec{Outcome: dispatchProviderError, RunID: run}, nil
	}

	var parsed triageOutputJSON
	if err := json.Unmarshal(resp.JSON, &parsed); err != nil {
		run := s.persistRun(ctx, p, runSchemaInvalid, maskedInput, resp.JSON, resp.Usage, resp.LatencyMs, 0, "parse output")
		return triageExec{Outcome: dispatchSchemaInvalid, RunID: run,
			Valid: false, ValidationErrors: []string{"saida ilegivel"}, OutputJSON: resp.JSON}, nil
	}

	out := parsed.toTriageOutput()
	cost := s.cost(ctx, p.Version.Provider, p.Version.Model, resp.Usage)
	run := s.persistRun(ctx, p, runOK, maskedInput, resp.JSON, resp.Usage, resp.LatencyMs, cost, "")
	var committed bool
	run, out, committed, err = s.applyExtracted(ctx, p, run, out)
	if err != nil {
		return triageExec{}, err
	}
	if !committed {
		return triageExec{Outcome: dispatchBlocked, RunID: run}, nil
	}
	return triageExec{
		Outcome: dispatchTriaged, Output: out, OutputJSON: resp.JSON, Valid: true,
		Usage: resp.Usage, CostUSD: cost, RunID: run,
	}, nil
}

func (s *AIService) withAIDispatchExternalEffectLease(ctx context.Context, p triageParams, effect func() error) error {
	if strings.TrimSpace(p.DispatchID) == "" {
		return effect()
	}
	if s.externalEffectLease == nil {
		return ErrHistoryResetInvalidated
	}
	allowed, err := s.externalEffectLease(ctx, p.AccountID, p.DispatchID, p.AIGeneration, effect)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrHistoryResetInvalidated
	}
	return nil
}

func (s *AIService) completeBrainWithLease(ctx context.Context, p triageParams, request BrainRequestV2,
	execution BrainExecutionV2) (BrainResultV2, llm.Usage, int, error) {
	var result BrainResultV2
	var usage llm.Usage
	var latency int
	var brainErr error
	err := s.withAIDispatchExternalEffectLease(ctx, p, func() error {
		result, usage, latency, brainErr = s.brain.CompleteBrain(ctx, request, execution)
		return nil
	})
	if err != nil {
		return BrainResultV2{}, llm.Usage{}, 0, err
	}
	return result, usage, latency, brainErr
}

// applyExtracted funde os campos na conversa quando o inbound pede (MergeToConv). O simulate
// NUNCA toca a conversa (C9.7). Falha de merge nao invalida o run ja gravado.
func (s *AIService) applyExtracted(ctx context.Context, p triageParams, runID string, out TriageOutput) (string, TriageOutput, bool, error) {
	if !p.MergeToConv || p.ConversationID == nil {
		return runID, out, true, nil
	}
	committed, err := s.store.CommitAITriageWithIntelligence(ctx, p.AccountID, *p.ConversationID, p.AIGeneration, runID, out)
	if err != nil {
		return runID, out, false, err
	}
	return runID, out, committed, nil
}

// gateRun descreve um run que NAO chamou o modelo (tokens 0): blocked/limit/provider_error.
type gateRun struct {
	AgentID       *string
	VersionID     *string
	Provider      string
	Model         string
	SchemaVersion string
	Status        string
	Error         string
}

// recordGateRun grava um run de gate (sem modelo) e devolve o id. Falha ao gravar apenas loga
// (a conversa nao pode travar por causa da trilha) e devolve "".
func (s *AIService) recordGateRun(ctx context.Context, in TriageInput, g gateRun) string {
	id, err := s.store.InsertRun(ctx, aiRunInsert{
		AccountID:      in.AccountID,
		ConversationID: nilIfEmpty(in.ConversationID),
		AgentID:        g.AgentID,
		AgentVersionID: g.VersionID,
		MessageID:      nilIfEmpty(in.MessageID),
		Status:         g.Status,
		Provider:       g.Provider,
		Model:          g.Model,
		SchemaVersion:  g.SchemaVersion,
		Error:          g.Error,
	})
	if err != nil {
		s.logger.Error("omnichannel_ai_run_insert_failed", "account_id", in.AccountID, "status", g.Status)
		return ""
	}
	return id
}

// persistRun grava um run que CHAMOU (ou tentou) o modelo. input JA vem mascarado (§10). Falha
// ao gravar apenas loga e devolve "" — o run e trilha, nao pode derrubar a triagem.
func (s *AIService) persistRun(ctx context.Context, p triageParams, status string, maskedInput, output json.RawMessage, usage llm.Usage, latencyMs int, cost float64, errMsg string) string {
	if len(output) == 0 {
		output = json.RawMessage(`{}`)
	}
	id, err := s.store.InsertRun(ctx, aiRunInsert{
		AccountID:        p.AccountID,
		ConversationID:   p.ConversationID,
		AgentID:          &p.Agent.ID,
		AgentVersionID:   &p.Version.ID,
		MessageID:        p.MessageID,
		Status:           status,
		Provider:         p.Version.Provider,
		Model:            p.Version.Model,
		SchemaVersion:    p.Version.SchemaVersion,
		Input:            maskedInput,
		Output:           output,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		CostUSD:          cost,
		LatencyMs:        latencyMs,
		Error:            errMsg,
	})
	if err != nil {
		s.logger.Error("omnichannel_ai_run_insert_failed", "account_id", p.AccountID, "status", status)
		return ""
	}
	return id
}

// cost calcula o custo do run a partir da precificacao do banco (platform_settings). Sem
// preco cadastrado => 0 (a F13 e a dona da precificacao). NUNCA hardcode de preco (principio 1).
func (s *AIService) cost(ctx context.Context, provider, model string, u llm.Usage) float64 {
	price, ok, err := s.store.ModelPricing(ctx, provider, model)
	if err != nil || !ok {
		return 0
	}
	return float64(u.PromptTokens)/1000*price.InputPer1kUSD +
		float64(u.CompletionTokens)/1000*price.OutputPer1kUSD
}

// maskedTriageInput monta o ai_runs.input MASCARADO (§10): estrutura, NUNCA o texto cru das
// mensagens (PII do cliente). So a contagem e as chaves dos campos a coletar.
func maskedTriageInput(fields []CollectFieldView, historyCount int) json.RawMessage {
	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		keys = append(keys, f.Key)
	}
	raw, err := json.Marshal(map[string]any{"messages": historyCount, "collectFields": keys})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

// nilIfEmpty devolve nil para string vazia (para colunas uuid nullable no InsertRun).
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
