package omnichannel

import (
	"testing"
	"time"
)

func TestAssembleUsageTotalsAndPricing(t *testing.T) {
	groups := []modelAgg{
		{Provider: "openai", Model: "gpt-4o-mini", Runs: 2, PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150, CostUsd: 0.01},
		{Provider: "anthropic", Model: "claude", Runs: 1, PromptTokens: 120, CompletionTokens: 80, TotalTokens: 200, CostUsd: 0}, // sem preco
		{Provider: "", Model: "", Runs: 1, TotalTokens: 0, CostUsd: 0},                                                           // gate (sem modelo)
	}
	priced := map[string]bool{"openai/gpt-4o-mini": true}
	lim := &UsageLimit{MonthlyAiRuns: 5000, Used: 4, Source: "account"}

	r := assembleUsage(groups, priced, lim)

	if r.Totals.Runs != 4 || r.Totals.TotalTokens != 350 || r.Totals.PromptTokens != 220 {
		t.Fatalf("totais errados: %+v", r.Totals)
	}
	if r.Totals.CostUsd != 0.01 {
		t.Fatalf("custo total = %v, quer 0.01", r.Totals.CostUsd)
	}
	if len(r.ByModel) != 3 || !r.ByModel[0].Priced {
		t.Fatalf("byModel[0] devia estar priced: %+v", r.ByModel)
	}
	// So o modelo que CONSUMIU e nao tem preco entra em unpricedModels. O gate (0 tokens) fica fora.
	if len(r.UnpricedModels) != 1 || r.UnpricedModels[0] != "anthropic/claude" {
		t.Fatalf("unpricedModels = %v, quer [anthropic/claude]", r.UnpricedModels)
	}
	if r.Limit != lim {
		t.Error("limit devia passar direto para o report")
	}
}

func TestAssembleUsageNoLimitStaysNil(t *testing.T) {
	r := assembleUsage(nil, map[string]bool{}, nil)
	if r.Limit != nil {
		t.Error("sem teto cadastrado, Limit deve ser nil (tela mostra 'Sem limite'), nunca 0")
	}
	if r.ByModel == nil || r.UnpricedModels == nil {
		t.Error("slices devem sair inicializadas (nunca null no JSON)")
	}
}

func TestParseUsageWindowDefaultsToCurrentMonth(t *testing.T) {
	now := time.Date(2026, 7, 17, 15, 30, 0, 0, time.UTC)
	from, toExcl := parseUsageWindow("", "", now)
	if from != time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC) {
		t.Errorf("from default = %v, quer 2026-07-01", from)
	}
	if !toExcl.Equal(now) {
		t.Errorf("to default = %v, quer now", toExcl)
	}
}

func TestParseUsageWindowInclusiveTo(t *testing.T) {
	now := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	from, toExcl := parseUsageWindow("2026-07-05", "2026-07-10", now)
	if from != time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC) {
		t.Errorf("from = %v", from)
	}
	// `to` e inclusivo do dia => exclusivo em 2026-07-11.
	if toExcl != time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC) {
		t.Errorf("toExclusive = %v, quer 2026-07-11", toExcl)
	}
}

func TestParseUsageWindowInvertedFallsToMinimum(t *testing.T) {
	now := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	from, toExcl := parseUsageWindow("2026-07-20", "2026-07-01", now)
	if !toExcl.After(from) {
		t.Errorf("janela invertida devia virar minima de 1 dia: from=%v toExcl=%v", from, toExcl)
	}
}
