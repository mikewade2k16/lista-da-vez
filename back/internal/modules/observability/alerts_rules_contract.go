package observability

import (
	"fmt"
	"time"
)

// Regras de regressão funcional: um recurso que para de funcionar sem derrubar a API
// (ex.: 2026-11-27, o save de roteiros devolvia 400 "Dados invalidos" para todo
// "+ Linha" porque o back passou a recusar um campo que o front ainda enviava).
// 5xx e latência não pegam esse caso; o cliente percebia antes de nós.

// routeContractErrors: 400/422 de corpo que não decodifica (invalid_body/invalid_json).
// O front do próprio Omni nunca deveria mandar isso: é contrato quebrado entre front e back.
type routeContractErrors struct {
	Route   string
	Count   int64
	Code    string
	Message string
}

// routeClientErrors: respostas 400/422 e total de chamadas da rota na janela.
type routeClientErrors struct {
	Route  string
	Errors int64
	Calls  int64
}

// deployWindow: último deploy com sucesso e os erros (5xx + 400/422) de cada rota
// nos 30 min antes do deploy (Before) e nos últimos 15 min depois dele (After).
type deployWindow struct {
	ID         string
	GitSHA     string
	StartedAt  time.Time
	FinishedAt time.Time
	Routes     []deployRouteErrors
}

type deployRouteErrors struct {
	Route  string
	Before int64 // 30 min antes do início do deploy
	After  int64 // últimos 15 min (a partir do fim do deploy)
}

// route4xxMinCalls evita alarme em rota com pouquíssimo uso.
const route4xxMinCalls = 10

func ruleContract(m alertMetrics) []condition {
	var out []condition
	contract := map[string]bool{}
	for _, item := range m.ContractErrors15m {
		if item.Count < int64(m.threshold("contract_errors")) {
			continue
		}
		contract[item.Route] = true
		out = append(out, condition{Rule: "contract_errors", Target: item.Route, Severity: sevHigh, Area: "api",
			Title:    "Recurso quebrado (front e back fora de contrato): " + item.Route,
			Measured: fmt.Sprintf("%d respostas %s em 15 min (%s)", item.Count, item.Code, item.Message),
			Detail:   map[string]any{"route": item.Route, "count": item.Count, "code": item.Code, "message": item.Message}})
	}
	limit := m.threshold("route_4xx_spike")
	for _, item := range m.Route4xx15m {
		if contract[item.Route] || item.Calls < route4xxMinCalls || item.Errors == 0 {
			continue
		}
		rate := float64(item.Errors) / float64(item.Calls)
		if rate*100 < limit {
			continue
		}
		out = append(out, condition{Rule: "route_4xx_spike", Target: item.Route, Severity: sevMedium, Area: "api",
			Title:    "Rota recusando a maioria das requisições: " + item.Route,
			Measured: fmt.Sprintf("%.0f%% de 400/422 (%d de %d em 15 min)", rate*100, item.Errors, item.Calls),
			Detail:   map[string]any{"route": item.Route, "errors": item.Errors, "calls": item.Calls, "rate": rate}})
	}
	return out
}

// ruleDeployRegression: rota que, depois do último deploy, erra bem mais do que antes.
// After (15 min) é comparado com Before (30 min) normalizado para 15 min. A janela de
// After desliza, então o alerta resolve sozinho quando os erros param.
func ruleDeployRegression(m alertMetrics) []condition {
	d := m.LastDeploy
	if d == nil {
		return nil
	}
	sha := d.GitSHA
	if len(sha) > 7 {
		sha = sha[:7]
	}
	if sha == "" {
		sha = d.ID // deploys antigos não gravavam o commit; o ID começa pela data
	}
	var out []condition
	for _, item := range d.Routes {
		before15 := float64(item.Before) / 2
		if item.After < int64(m.threshold("deploy_regression")) || float64(item.After) <= 2*before15 {
			continue
		}
		out = append(out, condition{Rule: "deploy_regression", Target: item.Route, Severity: sevHigh, Area: "api",
			Title: "Erros novos depois do deploy " + sha + ": " + item.Route,
			Measured: fmt.Sprintf("%d erros em 15 min após o deploy (antes: %.1f em 15 min)",
				item.After, before15),
			Detail: map[string]any{"route": item.Route, "after15m": item.After, "before30m": item.Before,
				"deployId": d.ID, "gitSha": d.GitSHA, "deployFinishedAt": d.FinishedAt}})
	}
	return out
}
