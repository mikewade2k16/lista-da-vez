package omnichannel

import (
	"context"
	"testing"
)

// fakeRuleSource injeta regras/default em memoria — o motor roda SEM banco e SEM LLM
// (Contrato 4, entrega 8: "motor sem chamar modelo").
type fakeRuleSource struct {
	rules      []compiledRule
	defTarget  routingTarget
	hasDefault bool
}

func (f *fakeRuleSource) ActiveRoutingRules(_ context.Context, _ string) ([]compiledRule, error) {
	return f.rules, nil
}

func (f *fakeRuleSource) DefaultTarget(_ context.Context, _ string) (routingTarget, bool, error) {
	return f.defTarget, f.hasDefault, nil
}

func TestDecideFirstMatchWins(t *testing.T) {
	src := &fakeRuleSource{
		rules: []compiledRule{
			{RuleID: "r1", Name: "preco", QueueID: "q-vendas", DepartmentID: "d-vendas",
				Conditions: []Condition{{Field: "message.text", Op: opContains, Value: "preço"}}},
			{RuleID: "r2", Name: "catch-all", QueueID: "q-geral", DepartmentID: "d-geral",
				Conditions: []Condition{}},
		},
	}
	eng := newRoutingEngine(src)

	dec, err := eng.Decide(context.Background(), "acc", RoutingContext{MessageText: "quanto é o preço?"})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if dec.Outcome != outcomeMatched || dec.QueueID == nil || *dec.QueueID != "q-vendas" {
		t.Fatalf("esperava matched q-vendas, veio %+v", dec)
	}
	if dec.RuleID == nil || *dec.RuleID != "r1" {
		t.Fatalf("esperava rule r1, veio %+v", dec.RuleID)
	}
	if dec.Input["message.text"] != "quanto é o preço?" {
		t.Fatalf("input nao capturou message.text: %+v", dec.Input)
	}
}

func TestDecideEmptyConditionsMatchesAlways(t *testing.T) {
	src := &fakeRuleSource{rules: []compiledRule{
		{RuleID: "r1", Name: "all", QueueID: "q1", DepartmentID: "d1", Conditions: []Condition{}},
	}}
	dec, err := newRoutingEngine(src).Decide(context.Background(), "acc", RoutingContext{})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if dec.Outcome != outcomeMatched || *dec.QueueID != "q1" {
		t.Fatalf("regra sem condicao deveria casar, veio %+v", dec)
	}
}

func TestDecideDefaultQueueWhenNoMatch(t *testing.T) {
	src := &fakeRuleSource{
		rules: []compiledRule{
			{RuleID: "r1", Name: "preco", QueueID: "q1", DepartmentID: "d1",
				Conditions: []Condition{{Field: "message.text", Op: opContains, Value: "preço"}}},
		},
		defTarget:  routingTarget{QueueID: "q-default", DepartmentID: "d-default"},
		hasDefault: true,
	}
	dec, err := newRoutingEngine(src).Decide(context.Background(), "acc", RoutingContext{MessageText: "bom dia"})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if dec.Outcome != outcomeDefaultQueue || *dec.QueueID != "q-default" {
		t.Fatalf("esperava default_queue, veio %+v", dec)
	}
	if dec.RuleID != nil {
		t.Fatalf("default_queue nao tem regra, veio %+v", dec.RuleID)
	}
}

func TestDecideUnroutedWhenNoDefault(t *testing.T) {
	src := &fakeRuleSource{hasDefault: false}
	dec, err := newRoutingEngine(src).Decide(context.Background(), "acc", RoutingContext{MessageText: "oi"})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if dec.Outcome != outcomeUnrouted || dec.QueueID != nil {
		t.Fatalf("esperava unrouted sem fila, veio %+v", dec)
	}
}

// TestEvalConditionOperators cobre o conjunto fechado de operadores (Contrato 4).
func TestEvalConditionOperators(t *testing.T) {
	rc := RoutingContext{
		MessageText:     "Quero um Orçamento",
		ContactPhone:    "5511999",
		ExtractedFields: map[string]any{"intent": "sales", "vip": true, "score": float64(7)},
	}
	cases := []struct {
		name string
		c    Condition
		want bool
	}{
		{"eq hit", Condition{"intent", opEq, "sales"}, true},
		{"eq miss", Condition{"intent", opEq, "support"}, false},
		{"neq hit", Condition{"intent", opNeq, "support"}, true},
		{"neq on absent", Condition{"nope", opNeq, "x"}, true},
		{"contains ci", Condition{"message.text", opContains, "orçamento"}, true},
		{"contains miss", Condition{"message.text", opContains, "boleto"}, false},
		{"exists hit", Condition{"contact.phone", opExists, nil}, true},
		{"exists miss", Condition{"email", opExists, nil}, false},
		{"in hit", Condition{"intent", opIn, []any{"sales", "billing"}}, true},
		{"in miss", Condition{"intent", opIn, []any{"support"}}, false},
		{"in number", Condition{"score", opIn, []any{float64(7), float64(8)}}, true},
		{"invalid op never matches", Condition{"intent", "regex", ".*"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evalCondition(tc.c, rc); got != tc.want {
				t.Errorf("evalCondition(%+v) = %v, esperava %v", tc.c, got, tc.want)
			}
		})
	}
}

// TestMatchesAllAnd garante o AND entre condicoes: uma falha derruba a regra.
func TestMatchesAllAnd(t *testing.T) {
	rc := RoutingContext{MessageText: "preço do plano", ExtractedFields: map[string]any{"intent": "sales"}}
	all := []Condition{
		{"message.text", opContains, "preço"},
		{"intent", opEq, "sales"},
	}
	if !matchesAll(all, rc) {
		t.Fatal("todas as condicoes casam, esperava true")
	}
	all = append(all, Condition{"intent", opEq, "support"})
	if matchesAll(all, rc) {
		t.Fatal("uma condicao falha deveria derrubar o AND")
	}
}
