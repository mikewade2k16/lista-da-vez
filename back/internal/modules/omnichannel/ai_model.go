package omnichannel

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// ============================================================================
// F9 — Triagem IA no Go: tipos do dominio (spec OMNI-F9.md, Contratos C9.1-C9.4)
// ============================================================================
//
// Views em camelCase: a F10 CONSOME estas rotas (nao recria). account_id NUNCA aparece na
// view nem no input — o escopo vem do Principal (principio 2). A chave do provider so sai
// MASCARADA ({set,last4} via secretbox.Status); a chave crua nunca deixa o server.
//
// Erros genericos (ErrNotFound, ErrValidation, ErrConflict, ErrForbidden) vivem em
// model.go/domain_model.go. Aqui so os especificos de IA.

var (
	// ErrAIProviderNotConfigured: provider/modelo/chave ausentes na version ativa. O
	// dispatch grava ai_runs.status=provider_error e a conversa segue pelo fallback da F8.
	ErrAIProviderNotConfigured = errors.New("omnichannel: ai provider/model/key not configured")
	// ErrVersionImmutable: tentativa de editar uma version ja publicada. Publicada e
	// imutavel: editar = criar version nova; rollback = repontar active_version_id.
	ErrVersionImmutable = errors.New("omnichannel: published version is immutable")
	ErrAILeaseInvalid   = errors.New("omnichannel: ai dispatch invalidated by conversation takeover")
)

// Status de run gravado em ai_runs.status (CHECK textual da migration 0206). Uma linha por
// TENTATIVA, inclusive as que nao chamaram o modelo (o silencio da IA precisa de trilha).
const (
	runOK            = "ok"
	runSchemaInvalid = "schema_invalid"
	runProviderError = "provider_error"
	runBlocked       = "blocked"
	runLimitExceeded = "limit_exceeded"
)

// versionPublished e o status que habilita rollback/uso (ai_agent_versions.status). Os
// demais valores possiveis (draft|archived) sao escritos direto no SQL (CreateVersion).
const versionPublished = "published"

// aiRunModuleID e o module_id em core.account_modules para o leitor de limites (F3). O
// limite mensal de triagens mora em config->monthly_ai_runs (canonico §5.3).
const aiRunModuleID = "omnichannel"

// monthlyAIRunsKey e a chave do limite mensal de execucoes de IA por conta.
const monthlyAIRunsKey = "monthly_ai_runs"

// ============================================================================
// Registros crus (colunas do banco, antes da projecao para view)
// ============================================================================

// agentRow e a linha crua de messaging.ai_agents. providerKeyCipher NUNCA vai a view/log.
type agentRow struct {
	ID                string
	Slug              string
	Name              string
	Enabled           bool
	ActiveVersionID   *string
	ProviderKeyCipher string
	ProviderKeyLast4  string
	CreatedBy         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// versionRow e a linha crua de messaging.ai_agent_versions.
type versionRow struct {
	ID                   string
	AgentID              string
	Version              int
	Status               string
	Provider             string
	Model                string
	ResponseCredentialID *string
	Temperature          float64
	Layers               json.RawMessage
	OutputSchema         json.RawMessage
	MediaConfig          json.RawMessage
	SchemaVersion        string
	DebounceMS           int
	MaxContextMessages   int
	MaxAITurns           int
	MinConfidence        float64
	HandoffOnError       bool
	HandoffOnLimit       bool
	WorkflowContract     string
	PublishedAt          *time.Time
	PublishedBy          string
	CreatedAt            time.Time
}

// ============================================================================
// Views servidas a F10 (camelCase)
// ============================================================================

// AIAgentView e o agente servido a F10. providerKey e o status MASCARADO (nunca a chave).
type AIAgentView struct {
	ID              string           `json:"id"`
	Slug            string           `json:"slug"`
	Name            string           `json:"name"`
	Enabled         bool             `json:"enabled"`
	ActiveVersionID *string          `json:"activeVersionId"`
	ProviderKey     secretbox.Status `json:"providerKey"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
}

// AIAgentVersionView e a version servida a F10. layers/outputSchema saem como jsonb cru (o
// editor da F10 os manipula). A chave do provider NAO esta aqui (vive no agente, mascarada).
type AIAgentVersionView struct {
	ID                   string          `json:"id"`
	AgentID              string          `json:"agentId"`
	Version              int             `json:"version"`
	Status               string          `json:"status"`
	Provider             string          `json:"provider"`
	Model                string          `json:"model"`
	ResponseCredentialID *string         `json:"responseCredentialId"`
	Temperature          float64         `json:"temperature"`
	Layers               json.RawMessage `json:"layers"`
	OutputSchema         json.RawMessage `json:"outputSchema"`
	MediaConfig          json.RawMessage `json:"mediaConfig"`
	SchemaVersion        string          `json:"schemaVersion"`
	DebounceMS           int             `json:"debounceMs"`
	MaxContextMessages   int             `json:"maxContextMessages"`
	MaxAITurns           int             `json:"maxAiTurns"`
	MinConfidence        float64         `json:"minConfidence"`
	HandoffOnError       bool            `json:"handoffOnError"`
	HandoffOnLimit       bool            `json:"handoffOnLimit"`
	WorkflowContract     string          `json:"workflowContractVersion"`
	PublishedAt          *time.Time      `json:"publishedAt"`
	CreatedAt            time.Time       `json:"createdAt"`
}

// CollectFieldView e o campo-a-coletar servido a F10.
type CollectFieldView struct {
	ID          string          `json:"id"`
	AgentID     string          `json:"agentId"`
	Key         string          `json:"key"`
	Label       string          `json:"label"`
	FieldType   string          `json:"fieldType"`
	EnumOptions json.RawMessage `json:"enumOptions"`
	Required    bool            `json:"required"`
	SortOrder   int             `json:"sortOrder"`
}

// AIRunView e a linha de auditoria (GET /agents/{id}/runs). input JA esta mascarado no banco.
type AIRunView struct {
	ID               string          `json:"id"`
	ConversationID   *string         `json:"conversationId"`
	AgentID          *string         `json:"agentId"`
	AgentVersionID   *string         `json:"agentVersionId"`
	MessageID        *string         `json:"messageId"`
	Status           string          `json:"status"`
	Provider         string          `json:"provider"`
	Model            string          `json:"model"`
	SchemaVersion    string          `json:"schemaVersion"`
	Input            json.RawMessage `json:"input"`
	Output           json.RawMessage `json:"output"`
	PromptTokens     int             `json:"promptTokens"`
	CompletionTokens int             `json:"completionTokens"`
	TotalTokens      int             `json:"totalTokens"`
	CostUSD          float64         `json:"costUsd"`
	LatencyMs        int             `json:"latencyMs"`
	Error            string          `json:"error"`
	CreatedAt        time.Time       `json:"createdAt"`
}

// ============================================================================
// Inputs (bodies das rotas /agents/*) — sem account_id (vem do Principal)
// ============================================================================

// AIAgentInput e o POST /agents. slug vazio => derivado do nome (slugify).
type AIAgentInput struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// AIAgentPatch e o PATCH /agents/{id}. Ponteiro = campo ausente nao muda. providerKey nao-nil
// grava (ou limpa, se vazio) a chave CIFRADA do provider; a resposta so devolve {set,last4}.
type AIAgentPatch struct {
	Name        *string `json:"name"`
	Enabled     *bool   `json:"enabled"`
	ProviderKey *string `json:"providerKey"`
}

// AIVersionInput e o POST /agents/{id}/versions: cria sempre um DRAFT. provider/model vem do
// painel (NUNCA supostos). outputSchema vazio => default C9.3 aplicado no service.
type AIVersionInput struct {
	Provider             string          `json:"provider"`
	Model                string          `json:"model"`
	ResponseCredentialID *string         `json:"responseCredentialId"`
	Temperature          float64         `json:"temperature"`
	Layers               json.RawMessage `json:"layers"`
	OutputSchema         json.RawMessage `json:"outputSchema"`
	MediaConfig          json.RawMessage `json:"mediaConfig"`
	SchemaVersion        string          `json:"schemaVersion"`
	DebounceMS           int             `json:"debounceMs"`
	MaxContextMessages   int             `json:"maxContextMessages"`
	MaxAITurns           int             `json:"maxAiTurns"`
	MinConfidence        *float64        `json:"minConfidence"`
	HandoffOnError       *bool           `json:"handoffOnError"`
	HandoffOnLimit       *bool           `json:"handoffOnLimit"`
	WorkflowContract     string          `json:"workflowContractVersion"`
}

// CollectFieldInput e o POST /agents/{id}/collect-fields.
type CollectFieldInput struct {
	Key         string          `json:"key"`
	Label       string          `json:"label"`
	FieldType   string          `json:"fieldType"`
	EnumOptions json.RawMessage `json:"enumOptions"`
	Required    bool            `json:"required"`
	SortOrder   int             `json:"sortOrder"`
}

// CollectFieldPatch e o PATCH /agents/{id}/collect-fields/{fieldId}.
type CollectFieldPatch struct {
	Label       *string          `json:"label"`
	FieldType   *string          `json:"fieldType"`
	EnumOptions *json.RawMessage `json:"enumOptions"`
	Required    *bool            `json:"required"`
	SortOrder   *int             `json:"sortOrder"`
}

// RollbackInput e o POST /agents/{id}/rollback: repointa active_version_id (nao reescreve).
type RollbackInput struct {
	VersionID string `json:"versionId"`
}

// SimMessage e uma mensagem do historico simulado (C9.7). role: contact|agent.
type SimMessage struct {
	ID   string `json:"id,omitempty"`
	Role string `json:"role"`
	Text string `json:"text"`
}

// SimulateInput e o POST /agents/{id}/simulate. versionId ausente = active_version_id.
type SimulateInput struct {
	VersionID string       `json:"versionId"`
	Messages  []SimMessage `json:"messages"`
	Contact   *struct {
		Name string `json:"name"`
	} `json:"contact"`
}

// SimulateView e a resposta do dry-run (C9.7): output + traco (valido, matchedRule, wouldRoute,
// usage). Prova "IA sugere, motor decide" sem mandar mensagem de verdade.
type SimulateView struct {
	Output           json.RawMessage `json:"output"`
	SchemaVersion    string          `json:"schemaVersion"`
	Valid            bool            `json:"valid"`
	ValidationErrors []string        `json:"validationErrors"`
	ExtractedFields  map[string]any  `json:"extractedFields"`
	MatchedRule      *SimMatchedRule `json:"matchedRule"`
	WouldRoute       *SimWouldRoute  `json:"wouldRoute"`
	Usage            SimUsage        `json:"usage"`
}

// SimMatchedRule e a regra que o motor da F8 casou (nil = nenhuma; caiu no default/unrouted).
type SimMatchedRule struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Priority int    `json:"priority"`
}

// SimWouldRoute e o destino que o motor escolheria (nil = unrouted).
type SimWouldRoute struct {
	DepartmentID *string `json:"departmentId"`
	QueueID      *string `json:"queueId"`
}

// SimUsage e o custo/tokens do run do simulador (real: chama o modelo).
type SimUsage struct {
	PromptTokens     int     `json:"promptTokens"`
	CompletionTokens int     `json:"completionTokens"`
	TotalTokens      int     `json:"totalTokens"`
	CostUSD          float64 `json:"costUsd"`
}

// ============================================================================
// Contrato Go da triagem (C9.4) — a IA SUGERE; o motor da F8 DECIDE
// ============================================================================

// TriageInput e a entrada da triagem. AccountID vem do Principal (nunca do body).
type TriageInput struct {
	AccountID      string
	ConversationID string
	MessageID      string
	// DispatchID is populated only by the durable E2 worker. Legacy/manual
	// triage keeps it empty and therefore remains on the native rollback path.
	DispatchID string
	// ForceReply is set only by the authenticated operator action. It bypasses
	// configurable conversational limits for one reply, never technical gates.
	ForceReply bool
}

// TriageOutput e a SUGESTAO da IA (entrada do motor deterministico da F8). Nenhum caminho
// escreve conversation.queue_id a partir daqui — quem roteia e a regra (routing_engine).
type TriageOutput struct {
	Intent              string
	Sentiment           string
	Confidence          float64
	ExtractedFields     map[string]any
	ContactMemory       ContactMemorySuggestion
	SuggestedDepartment string
	SuggestedQueue      string
	NeedsHuman          bool
	HumanRequested      bool
	SensitiveTopic      bool
	CloseRequested      bool
	CloseReason         string
	HandoffReason       string
	HandoffSummary      string
	ReplyDraft          string
}

// triageOutputJSON e o shape cru da resposta do modelo (snake_case, C9.3). Ponteiros nos
// campos anulaveis (suggested_*/reply_draft): o modelo pode devolver null.
type triageOutputJSON struct {
	Intent              string                  `json:"intent"`
	Sentiment           string                  `json:"sentiment"`
	Confidence          float64                 `json:"confidence"`
	ExtractedFields     map[string]any          `json:"extracted_fields"`
	ContactMemory       ContactMemorySuggestion `json:"contact_memory"`
	SuggestedDepartment *string                 `json:"suggested_department"`
	SuggestedQueue      *string                 `json:"suggested_queue"`
	NeedsHuman          bool                    `json:"needs_human"`
	HumanRequested      bool                    `json:"human_requested"`
	SensitiveTopic      bool                    `json:"sensitive_topic"`
	CloseRequested      bool                    `json:"close_requested"`
	CloseReason         *string                 `json:"close_reason"`
	ReplyDraft          *string                 `json:"reply_draft"`
}

// toTriageOutput normaliza o shape cru (ponteiros -> strings vazias) para o contrato Go.
func (j triageOutputJSON) toTriageOutput() TriageOutput {
	out := TriageOutput{
		Intent:          j.Intent,
		Sentiment:       normalizeContactSentiment(j.Sentiment),
		Confidence:      j.Confidence,
		ExtractedFields: j.ExtractedFields,
		ContactMemory:   normalizeContactMemory(j.ContactMemory),
		NeedsHuman:      j.NeedsHuman,
		HumanRequested:  j.HumanRequested,
		SensitiveTopic:  j.SensitiveTopic,
		CloseRequested:  j.CloseRequested,
	}
	if out.ExtractedFields == nil {
		out.ExtractedFields = map[string]any{}
	}
	if len(out.ContactMemory.Facts) == 0 && len(out.ExtractedFields) > 0 {
		out.ContactMemory.Facts = normalizeContactMemoryMap(out.ExtractedFields)
	}
	if j.SuggestedDepartment != nil {
		out.SuggestedDepartment = *j.SuggestedDepartment
	}
	if j.SuggestedQueue != nil {
		out.SuggestedQueue = *j.SuggestedQueue
	}
	if j.ReplyDraft != nil {
		out.ReplyDraft = *j.ReplyDraft
	}
	if j.CloseReason != nil {
		out.CloseReason = *j.CloseReason
	}
	return out
}

// DispatchOutcome classifica o resultado do dispatch da triagem (C9.6). O inbound degrada
// para o fallback da F8 em qualquer outcome != triaged; o simulate trata cada um.
type DispatchOutcome string

const (
	dispatchTriaged        DispatchOutcome = "triaged"         // LLM rodou, saida valida
	dispatchBlocked        DispatchOutcome = "blocked"         // state nao permite IA (human_active)
	dispatchContactBlocked DispatchOutcome = "contact_blocked" // contato bloqueado para atendimento por IA
	dispatchNoAgent        DispatchOutcome = "no_agent"        // desabilitado / sem version ativa
	dispatchLimitExceeded  DispatchOutcome = "limit_exceeded"  // teto mensal estourado
	dispatchProviderError  DispatchOutcome = "provider_error"  // provider/modelo/chave ausente/falho
	dispatchSchemaInvalid  DispatchOutcome = "schema_invalid"  // saida nao validou (apos 1 retry)
	dispatchNoReply        DispatchOutcome = "no_reply"        // policy silencia e aguarda novo inbound
)

// DispatchResult e o resultado do dispatch. Output so e valido quando Outcome==triaged. RunID
// vazio quando nenhum run foi gravado (no_agent). Nunca carrega a chave nem prompt bruto.
type DispatchResult struct {
	Outcome      DispatchOutcome
	Output       TriageOutput
	RunID        string
	AIGeneration int64
	ReasonCode   string
}

// defaultOutputSchema e o JSON Schema canonico da saida (C9.3), aplicado quando a version e
// criada sem output_schema proprio. additionalProperties:false barra campo alucinado.
func defaultOutputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "intent": {"type": "string"},
    "sentiment": {"type": "string", "enum": ["positive", "neutral", "negative", "unknown"]},
    "confidence": {"type": "number", "minimum": 0, "maximum": 1},
    "extracted_fields": {"type": "object"},
    "contact_memory": {
      "type": "object",
      "properties": {
        "summary": {"type": ["string", "null"], "maxLength": 1000},
        "facts": {"type": "object"},
        "preferences": {"type": "object"}
      },
      "required": ["facts", "preferences"],
      "additionalProperties": false
    },
    "suggested_department": {"type": ["string", "null"]},
    "suggested_queue": {"type": ["string", "null"]},
    "needs_human": {"type": "boolean"},
	"human_requested": {"type": "boolean"},
	"sensitive_topic": {"type": "boolean"},
	"close_requested": {"type": "boolean"},
	"close_reason": {"type": ["string", "null"]},
    "reply_draft": {"type": ["string", "null"]}
  },
	"required": ["intent", "confidence", "extracted_fields", "needs_human", "human_requested", "sensitive_topic", "close_requested"],
  "additionalProperties": false
}`)
}
