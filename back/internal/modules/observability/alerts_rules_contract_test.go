package observability

import (
	"testing"
	"time"
)

// Cenário real de 2026-11-27: "+ Linha" nos roteiros devolvia 400 invalid_body.
func TestContractErrorsCatchesScriptsRegression(t *testing.T) {
	m := baseMetrics()
	m.ContractErrors15m = []routeContractErrors{{Route: "PUT /v1/tools/scripts/{id}", Count: 4, Code: "invalid_body", Message: "Dados invalidos."}}
	m.Route4xx15m = []routeClientErrors{{Route: "PUT /v1/tools/scripts/{id}", Errors: 4, Calls: 12}}
	conds := evaluateRules(m)
	c := findRule(conds, "contract_errors", "PUT /v1/tools/scripts/{id}")
	if c == nil || c.Severity != sevHigh {
		t.Fatalf("contrato quebrado deve alertar alto: %+v", conds)
	}
	if findRule(conds, "route_4xx_spike", "PUT /v1/tools/scripts/{id}") != nil {
		t.Error("rota já alertada como contrato não duplica no 4xx")
	}
	m.ContractErrors15m[0].Count = 1
	if findRule(evaluateRules(m), "contract_errors", "PUT /v1/tools/scripts/{id}") != nil {
		t.Error("uma requisição isolada não alerta (limite padrão 2)")
	}
}

func TestRoute4xxSpike(t *testing.T) {
	m := baseMetrics()
	m.Route4xx15m = []routeClientErrors{
		{Route: "POST /a", Errors: 6, Calls: 10}, // 60%: alerta
		{Route: "POST /b", Errors: 4, Calls: 10}, // 40%: não
		{Route: "POST /c", Errors: 5, Calls: 5},  // pouco uso: não
	}
	conds := evaluateRules(m)
	if c := findRule(conds, "route_4xx_spike", "POST /a"); c == nil || c.Severity != sevMedium {
		t.Fatalf("60%% de 400 deve alertar: %+v", conds)
	}
	for _, route := range []string{"POST /b", "POST /c"} {
		if findRule(conds, "route_4xx_spike", route) != nil {
			t.Errorf("%s não deveria alertar", route)
		}
	}
}

func TestDeployRegression(t *testing.T) {
	m := baseMetrics()
	m.LastDeploy = &deployWindow{ID: "d1", GitSHA: "272ebf1abcdef", StartedAt: t0.Add(-40 * time.Minute), FinishedAt: t0.Add(-35 * time.Minute),
		Routes: []deployRouteErrors{
			{Route: "PUT /novo", Before: 0, After: 3},     // erro novo: alerta
			{Route: "GET /cronico", Before: 20, After: 8}, // já errava antes (10/15 min): não
			{Route: "GET /pouco", Before: 0, After: 2},    // abaixo do limite: não
		}}
	conds := evaluateRules(m)
	c := findRule(conds, "deploy_regression", "PUT /novo")
	if c == nil || c.Severity != sevHigh || c.Title != "Erros novos depois do deploy 272ebf1: PUT /novo" {
		t.Fatalf("erro novo pós-deploy deve alertar: %+v", conds)
	}
	for _, route := range []string{"GET /cronico", "GET /pouco"} {
		if findRule(conds, "deploy_regression", route) != nil {
			t.Errorf("%s não deveria alertar", route)
		}
	}
	m.LastDeploy = nil
	if findRule(evaluateRules(m), "deploy_regression", "PUT /novo") != nil {
		t.Error("sem deploy recente não há comparação")
	}
}
