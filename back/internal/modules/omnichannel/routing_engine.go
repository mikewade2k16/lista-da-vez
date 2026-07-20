package omnichannel

import (
	"context"
	"fmt"
	"strings"
)

// ============================================================================
// F8 — Motor de roteamento deterministico (Contrato 4 da spec OMNI-F8.md)
// ============================================================================
//
// "IA sugere; o motor decide." Decide recebe extracted_fields JA preenchido e NAO chama
// modelo — e testavel sem LLM (routing_engine_test.go). Avalia routing_rules ativas em
// `priority asc, id asc`, first-match-wins; sem match cai na fila default do setor default;
// sem default => unrouted (queue_id NULL). TODA chamada gera uma linha em routing_decisions
// (quem persiste e o service_transition.go; aqui so a decisao pura).

// RoutingContext e a entrada avaliavel de uma conversa: os campos canonicos + o mapa
// extracted_fields. Nao e a Conversation inteira — o motor so precisa do que as condicoes
// leem, e isso o torna trivial de testar.
type RoutingContext struct {
	ExtractedFields map[string]any
	MessageText     string
	ContactPhone    string
	InstanceName    string
}

// compiledRule e a regra pronta para avaliar (regra + destino ja resolvido no store).
type compiledRule struct {
	RuleID       string
	Name         string
	QueueID      string
	DepartmentID string
	Conditions   []Condition
}

// routingTarget e um par fila/setor destino (a fila default do setor default).
type routingTarget struct {
	QueueID      string
	DepartmentID string
}

// routingRuleSource e a fonte de regras/default do motor. A abstracao existe para o teste
// injetar um fake sem banco (Contrato 4: "testavel sem LLM"). O Store a implementa em
// store_postgres_routing.go.
type routingRuleSource interface {
	ActiveRoutingRules(ctx context.Context, accountID string) ([]compiledRule, error)
	DefaultTarget(ctx context.Context, accountID string) (routingTarget, bool, error)
}

// routingEngine e a implementacao de RoutingEngine (Contrato 4).
type routingEngine struct {
	source routingRuleSource
}

func newRoutingEngine(source routingRuleSource) *routingEngine {
	return &routingEngine{source: source}
}

// Decide roda a matriz de regras e devolve a decisao explicavel. Determinista: mesma
// entrada, mesma saida (a ordem `priority asc, id asc` vem do store). Nunca "some" — sem
// regra e sem default, devolve `unrouted` (estado honesto, nao default que minta).
func (e *routingEngine) Decide(ctx context.Context, accountID string, rc RoutingContext) (Decision, error) {
	input := snapshotInput(rc)

	rules, err := e.source.ActiveRoutingRules(ctx, accountID)
	if err != nil {
		return Decision{}, err
	}
	for _, r := range rules {
		if matchesAll(r.Conditions, rc) {
			ruleID, queueID, deptID := r.RuleID, r.QueueID, r.DepartmentID
			return Decision{
				RuleID:       &ruleID,
				QueueID:      &queueID,
				DepartmentID: &deptID,
				Outcome:      outcomeMatched,
				Reason:       fmt.Sprintf("regra %q casou", r.Name),
				Input:        input,
			}, nil
		}
	}

	def, ok, err := e.source.DefaultTarget(ctx, accountID)
	if err != nil {
		return Decision{}, err
	}
	if ok {
		queueID, deptID := def.QueueID, def.DepartmentID
		return Decision{
			QueueID:      &queueID,
			DepartmentID: &deptID,
			Outcome:      outcomeDefaultQueue,
			Reason:       "nenhuma regra casou; fila default do setor default",
			Input:        input,
		}, nil
	}

	return Decision{
		Outcome: outcomeUnrouted,
		Reason:  "nenhuma regra casou e nao ha fila default configurada",
		Input:   input,
	}, nil
}

// matchesAll aplica AND entre as condicoes. Array vazio casa sempre (Contrato 4).
func matchesAll(conditions []Condition, rc RoutingContext) bool {
	for _, c := range conditions {
		if !evalCondition(c, rc) {
			return false
		}
	}
	return true
}

// evalCondition avalia uma clausula. Op invalido nunca casa (defensivo — a criacao da regra
// ja rejeita op fora do conjunto, mas dado legado no jsonb nao pode derrubar o roteamento).
func evalCondition(c Condition, rc RoutingContext) bool {
	val, present := resolveField(c.Field, rc)
	switch c.Op {
	case opExists:
		return present
	case opEq:
		return present && strings.EqualFold(toStr(val), toStr(c.Value))
	case opNeq:
		return !strings.EqualFold(toStr(val), toStr(c.Value))
	case opContains:
		return present && strings.Contains(strings.ToLower(toStr(val)), strings.ToLower(toStr(c.Value)))
	case opIn:
		return present && matchIn(toStr(val), c.Value)
	default:
		return false
	}
}

// resolveField resolve o valor de um field. Campos canonicos vem dos atributos da conversa;
// o resto e chave de extracted_fields. present distingue "ausente" de "vazio" (op exists).
func resolveField(field string, rc RoutingContext) (any, bool) {
	switch field {
	case "message.text":
		return rc.MessageText, rc.MessageText != ""
	case "contact.phone":
		return rc.ContactPhone, rc.ContactPhone != ""
	case "instance.name":
		return rc.InstanceName, rc.InstanceName != ""
	default:
		v, ok := rc.ExtractedFields[field]
		return v, ok
	}
}

// matchIn responde se `val` esta na lista `raw` (esperada como array). Comparacao
// case-insensitive por string — o jsonb pode trazer numero/string misturados.
func matchIn(val string, raw any) bool {
	list, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, item := range list {
		if strings.EqualFold(val, toStr(item)) {
			return true
		}
	}
	return false
}

// toStr normaliza um valor jsonb (string/number/bool) em texto para comparacao estavel.
// Numeros do json chegam como float64: %v ja imprime inteiros sem casa decimal.
func toStr(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}

// snapshotInput monta o `input` gravado em routing_decisions: o que foi avaliado. Sem isso
// a decisao vira caixa preta (Contrato 1). Campos canonicos so entram quando presentes.
func snapshotInput(rc RoutingContext) map[string]any {
	input := map[string]any{}
	if rc.MessageText != "" {
		input["message.text"] = rc.MessageText
	}
	if rc.ContactPhone != "" {
		input["contact.phone"] = rc.ContactPhone
	}
	if rc.InstanceName != "" {
		input["instance.name"] = rc.InstanceName
	}
	for k, v := range rc.ExtractedFields {
		input[k] = v
	}
	return input
}
