package omnichannel

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// ============================================================================
// F8 — Tipos do dominio de atendimento (setores/filas/roteamento)
// ============================================================================
//
// Views em camelCase: a F10 CONSOME estas rotas (nao recria). account_id NUNCA aparece na
// view nem no input — o escopo vem do Principal (principio 2). Erros de dominio novos abaixo;
// os genericos (ErrNotFound, ErrInvalidBody, ErrForbidden) e ErrInvalidTransition ja existem.

var (
	// ErrConflict = violacao de unicidade (slug repetido na conta/setor). Mapeado em 409.
	ErrConflict = errors.New("omnichannel: conflict")
	// ErrValidation = input malformado que nao e nem escopo (404) nem conflito (409).
	ErrValidation = errors.New("omnichannel: validation")
)

// ============================================================================
// Setores / filas / membros / regras — registros e views
// ============================================================================

// DepartmentView e o setor servido a F10.
type DepartmentView struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	IsDefault bool      `json:"isDefault"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// QueueView e a fila servida a F10. departmentId identifica o setor dono.
type QueueView struct {
	ID           string    `json:"id"`
	DepartmentID string    `json:"departmentId"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	IsDefault    bool      `json:"isDefault"`
	IsActive     bool      `json:"isActive"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// QueueMemberView e o vinculo atendente<->fila (o gate de dado). Traz nome/email do usuario
// para a tela nao precisar de um segundo fetch.
type QueueMemberView struct {
	ID        string    `json:"id"`
	QueueID   string    `json:"queueId"`
	UserID    string    `json:"userId"`
	UserName  string    `json:"userName"`
	UserEmail string    `json:"userEmail"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
}

// RoutingRuleView e a regra de roteamento servida a F10. conditions e o array cru (o front
// edita op a op). targetQueueId e a fila destino quando a regra casa.
type RoutingRuleView struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Priority      int         `json:"priority"`
	IsActive      bool        `json:"isActive"`
	Conditions    []Condition `json:"conditions"`
	TargetQueueID string      `json:"targetQueueId"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
}

// RoutingDecisionView e a auditoria de uma decisao (GET /conversations/{id}/routing-decisions).
type RoutingDecisionView struct {
	ID                 string    `json:"id"`
	ConversationID     string    `json:"conversationId"`
	RuleID             *string   `json:"ruleId"`
	Outcome            string    `json:"outcome"`
	Reason             string    `json:"reason"`
	TargetDepartmentID *string   `json:"targetDepartmentId"`
	TargetQueueID      *string   `json:"targetQueueId"`
	DecidedAt          time.Time `json:"decidedAt"`
}

// ============================================================================
// Roteamento — condicoes e decisao (Contrato 4)
// ============================================================================

// Condition e uma clausula de regra: {field, op, value}. AND entre as clausulas de uma
// regra. field = chave de extracted_fields ou campo canonico (message.text, contact.phone,
// instance.name). Op fechado: eq/neq/contains/in/exists.
type Condition struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

const (
	opEq       = "eq"
	opNeq      = "neq"
	opContains = "contains"
	opIn       = "in"
	opExists   = "exists"
)

// Outcomes de routing_decisions (CHECK da migration 0205).
const (
	outcomeMatched        = "matched"
	outcomeDefaultQueue   = "default_queue"
	outcomeUnrouted       = "unrouted"
	outcomeManualTransfer = "manual_transfer"
	outcomeAIFailed       = "ai_failed"
)

// Decision e o resultado do motor (Contrato 4). Ponteiros nil quando `unrouted` (sem fila).
// Input = snapshot do que foi avaliado (torna a decisao explicavel meses depois, sem re-rodar).
type Decision struct {
	RuleID       *string
	QueueID      *string
	DepartmentID *string
	Outcome      string
	Reason       string
	Input        map[string]any
}

// ============================================================================
// Inputs (bodies das rotas /settings/*) — sem account_id (vem do Principal)
// ============================================================================

// DepartmentInput e o POST /settings/departments. Slug vazio => derivado do nome (slugify).
type DepartmentInput struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

// DepartmentPatch e o PATCH /settings/departments/{id}. Ponteiro = campo ausente nao muda.
type DepartmentPatch struct {
	Name      *string `json:"name"`
	IsDefault *bool   `json:"isDefault"`
	IsActive  *bool   `json:"isActive"`
}

// QueueInput e o POST /settings/queues. departmentId obrigatorio (validado contra a conta).
type QueueInput struct {
	DepartmentID string `json:"departmentId"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	IsDefault    bool   `json:"isDefault"`
}

// QueuePatch e o PATCH /settings/queues/{id}.
type QueuePatch struct {
	Name      *string `json:"name"`
	IsDefault *bool   `json:"isDefault"`
	IsActive  *bool   `json:"isActive"`
}

// QueueMemberInput e o POST /settings/queues/{id}/members. So o userId — a fila vem do path
// e a conta do Principal. (re)ativa o membro; idempotente.
type QueueMemberInput struct {
	UserID string `json:"userId"`
}

// RoutingRuleInput e o POST /settings/routing-rules.
type RoutingRuleInput struct {
	Name          string      `json:"name"`
	Priority      int         `json:"priority"`
	IsActive      *bool       `json:"isActive"`
	Conditions    []Condition `json:"conditions"`
	TargetQueueID string      `json:"targetQueueId"`
}

// RoutingRulePatch e o PATCH /settings/routing-rules/{id}.
type RoutingRulePatch struct {
	Name          *string      `json:"name"`
	Priority      *int         `json:"priority"`
	IsActive      *bool        `json:"isActive"`
	Conditions    *[]Condition `json:"conditions"`
	TargetQueueID *string      `json:"targetQueueId"`
}

// RoutingRuleOrder e o PUT /settings/routing-rules/order: reordena priority em UMA transacao
// (tudo ou nada). Id ausente/de outra conta => 404 e a ordem NAO muda.
type RoutingRuleOrder struct {
	RuleIDs []string `json:"ruleIds"`
}

// ============================================================================
// Transicao — payload e escopo de visibilidade
// ============================================================================

// TransitionPayload carrega os alvos que o efeito colateral de cada evento consome
// (Contrato 2, notas 2/5/8/9/10): quem age, quem recebe a atribuicao, para qual fila.
// Todos opcionais; cada evento usa o que precisa.
type TransitionPayload struct {
	ActorUserID   string // msg.outbound.human: vira assigned_user_id (tomou a conversa)
	TargetUserID  string // human.assign: destino (validado por guarda; fora => 404 do usuario)
	TargetQueueID string // queue.transfer: fila destino (validada contra a conta => 404)
}

// VisibilityScope combina as instancias autorizadas com o gate de fila/atribuicao. Permissoes
// administrativas nunca transformam este escopo em "ver tudo".
type VisibilityScope struct {
	UserID                  string
	InstanceScopeKeys       []string
	ManageInstanceScopeKeys []string
}

// ============================================================================
// Helpers de dominio
// ============================================================================

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// slugify normaliza um nome em slug estavel (minusculo, hifen). Usado quando o input nao
// traz slug — a fonte de verdade e a coluna slug, este helper so gera o default inicial.
func slugify(s string) string {
	out := slugPattern.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	return strings.Trim(out, "-")
}

// validOp responde se o operador da condicao esta no conjunto fechado do Contrato 4.
func validOp(op string) bool {
	switch op {
	case opEq, opNeq, opContains, opIn, opExists:
		return true
	default:
		return false
	}
}
