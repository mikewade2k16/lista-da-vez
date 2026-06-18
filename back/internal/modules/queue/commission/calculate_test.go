package commission

import (
	"math"
	"testing"
)

const floatTolerance = 1e-9

func approxEqual(left, right float64) bool {
	return math.Abs(left-right) <= floatTolerance
}

func TestMapRoleToGroup(t *testing.T) {
	tests := []struct {
		role string
		want RoleGroup
	}{
		{"Gerente", GroupManager},
		{"GERENCIA", GroupManager},
		{"subgerente", GroupManager},
		{"Líder de loja", GroupManager},
		{"Caixa", GroupSupport},
		{"Auxiliar de estoque", GroupSupport},
		{"Recepção", GroupSupport},
		{"Consultor", GroupConsultant},
		{"Vendedora", GroupConsultant},
		{"", GroupConsultant},
	}

	for _, test := range tests {
		if got := MapRoleToGroup(test.role); got != test.want {
			t.Errorf("MapRoleToGroup(%q) = %q, want %q", test.role, got, test.want)
		}
	}
}

func TestCalculateConsultantSelfWithPenalty(t *testing.T) {
	// Consultor bateu a propria meta (>=100%), loja em 60% (faixa 50->1,5%),
	// base = propria venda (self). Ticket abaixo da meta => -0,1%.
	input := Input{
		Role:               "Consultor",
		StoreSold:          100000,
		StoreProgress:      60,
		ConsultantSold:     20000,
		ConsultantProgress: 100,
		HitPa:              true,
		HitTicket:          false,
		Policy:             DefaultPolicy(),
	}

	result := Calculate(input)

	if result.Group != GroupConsultant {
		t.Fatalf("group = %q, want consultant", result.Group)
	}
	if !result.RuleMatched {
		t.Fatalf("expected a matched rule")
	}
	if !approxEqual(result.PenaltyApplied, 0.1) {
		t.Errorf("penaltyApplied = %v, want 0.1", result.PenaltyApplied)
	}
	if !approxEqual(result.RatePercent, 1.4) {
		t.Errorf("ratePercent = %v, want 1.4", result.RatePercent)
	}
	if !approxEqual(result.Base, 20000) {
		t.Errorf("base = %v, want 20000 (self)", result.Base)
	}
	// 20000 * 1.4 / 100 = 280
	if !approxEqual(result.Amount, 280) {
		t.Errorf("amount = %v, want 280", result.Amount)
	}
}

func TestCalculateConsultantBothMetricsMissed(t *testing.T) {
	input := Input{
		Role:               "Consultor",
		StoreSold:          100000,
		StoreProgress:      60,
		ConsultantSold:     20000,
		ConsultantProgress: 120,
		HitPa:              false,
		HitTicket:          false,
		Policy:             DefaultPolicy(),
	}

	result := Calculate(input)

	// 1,5 - 0,1*2 = 1,3
	if !approxEqual(result.PenaltyApplied, 0.2) {
		t.Errorf("penaltyApplied = %v, want 0.2", result.PenaltyApplied)
	}
	if !approxEqual(result.RatePercent, 1.3) {
		t.Errorf("ratePercent = %v, want 1.3", result.RatePercent)
	}
	if !approxEqual(result.Amount, 260) {
		t.Errorf("amount = %v, want 260", result.Amount)
	}
}

func TestCalculateConsultantPenaltyClampsAtZero(t *testing.T) {
	policy := DefaultPolicy()
	policy.ConsultantRules.QualityPenaltyPercent = 1 // -2% no total derruba a faixa 1,5%

	input := Input{
		Role:               "Consultor",
		StoreProgress:      60,
		ConsultantSold:     20000,
		ConsultantProgress: 100,
		HitPa:              false,
		HitTicket:          false,
		Policy:             policy,
	}

	result := Calculate(input)

	if !approxEqual(result.RatePercent, 0) {
		t.Errorf("ratePercent = %v, want 0 (clamped)", result.RatePercent)
	}
	if !approxEqual(result.Amount, 0) {
		t.Errorf("amount = %v, want 0 (clamped)", result.Amount)
	}
}

func TestCalculateConsultantReducedBandNeedsFullOwnGoal(t *testing.T) {
	// Loja 60% (faixa reduzida 50-79): consultor a 99% (< 100) nao recebe.
	input := Input{
		Role:               "Consultor",
		StoreProgress:      60,
		ConsultantSold:     20000,
		ConsultantProgress: 99,
		HitPa:              true,
		HitTicket:          true,
		Policy:             DefaultPolicy(),
	}

	result := Calculate(input)

	if result.RuleMatched {
		t.Fatalf("expected no rule matched in reduced band below 100%% own goal")
	}
	if !approxEqual(result.Amount, 0) {
		t.Errorf("amount = %v, want 0", result.Amount)
	}
	if result.RuleLabel != labelStoreReduced {
		t.Errorf("ruleLabel = %q, want %q", result.RuleLabel, labelStoreReduced)
	}
}

func TestCalculateConsultantStoreBelowFloorIsZero(t *testing.T) {
	// Loja 40% (< 50): consultor nao recebe nada, mesmo batendo a propria meta.
	input := Input{
		Role:               "Consultor",
		StoreProgress:      40,
		ConsultantSold:     20000,
		ConsultantProgress: 130,
		HitPa:              true,
		HitTicket:          true,
		Policy:             DefaultPolicy(),
	}

	result := Calculate(input)

	if result.RuleMatched {
		t.Fatalf("expected no rule matched when store below floor")
	}
	if !approxEqual(result.Amount, 0) {
		t.Errorf("amount = %v, want 0", result.Amount)
	}
	if result.RuleLabel != labelStoreBelowFloor {
		t.Errorf("ruleLabel = %q, want %q", result.RuleLabel, labelStoreBelowFloor)
	}
}

func TestCalculateConsultantStoreHealthyOwnBands(t *testing.T) {
	// Loja 88% (>= 80, saudavel): faixa pela PROPRIA meta do consultor.
	cases := []struct {
		ownProgress float64
		wantRate    float64
	}{
		{82, 1},    // >= 80 -> 1%
		{94, 2},    // >= 90 -> 2%
		{105, 3},   // >= 100 -> 3%
		{125, 3.2}, // >= 120 -> 3,2%
	}

	for _, test := range cases {
		input := Input{
			Role:               "Consultor",
			StoreSold:          100000,
			StoreProgress:      88,
			ConsultantSold:     20000,
			ConsultantProgress: test.ownProgress,
			HitPa:              true,
			HitTicket:          true,
			Policy:             DefaultPolicy(),
		}

		result := Calculate(input)
		if !result.RuleMatched {
			t.Fatalf("ownProgress=%v: expected matched rule", test.ownProgress)
		}
		if !approxEqual(result.RatePercent, test.wantRate) {
			t.Errorf("ownProgress=%v: ratePercent = %v, want %v", test.ownProgress, result.RatePercent, test.wantRate)
		}
		if !approxEqual(result.Base, 20000) {
			t.Errorf("ownProgress=%v: base = %v, want 20000 (self)", test.ownProgress, result.Base)
		}
	}
}

func TestCalculateConsultantStoreHealthyBelowLowestOwnBandIsZero(t *testing.T) {
	// Loja 88% (saudavel) mas consultor a 70% (< menor faixa propria 80) => 0.
	input := Input{
		Role:               "Consultor",
		StoreSold:          100000,
		StoreProgress:      88,
		ConsultantSold:     20000,
		ConsultantProgress: 70,
		HitPa:              true,
		HitTicket:          true,
		Policy:             DefaultPolicy(),
	}

	result := Calculate(input)
	if result.RuleMatched {
		t.Fatalf("expected no rule matched below lowest own band")
	}
	if result.RuleLabel != labelBelowOwnGoal {
		t.Errorf("ruleLabel = %q, want %q", result.RuleLabel, labelBelowOwnGoal)
	}
}

func TestCalculateConsultantBaseStore(t *testing.T) {
	policy := DefaultPolicy()
	policy.ConsultantRules.Base = BaseStore

	input := Input{
		Role:               "Consultor",
		StoreSold:          100000,
		StoreProgress:      60,
		ConsultantSold:     20000,
		ConsultantProgress: 100,
		HitPa:              true,
		HitTicket:          true,
		Policy:             policy,
	}

	result := Calculate(input)

	if !approxEqual(result.Base, 100000) {
		t.Errorf("base = %v, want 100000 (store)", result.Base)
	}
	// 100000 * 1.5 / 100 = 1500
	if !approxEqual(result.Amount, 1500) {
		t.Errorf("amount = %v, want 1500", result.Amount)
	}
}

func TestCalculateManagerShoppingVsBairro(t *testing.T) {
	policy := DefaultPolicy()
	base := Input{
		Role:          "Gerente",
		StoreSold:     200000,
		StoreProgress: 100, // shopping: faixa 100->1%; bairro: faixa 100->1,7%
		Policy:        policy,
	}

	shopping := base
	shopping.StoreType = "shopping"
	shoppingResult := Calculate(shopping)
	if shoppingResult.Group != GroupManager {
		t.Fatalf("group = %q, want manager", shoppingResult.Group)
	}
	if !approxEqual(shoppingResult.RatePercent, 1) {
		t.Errorf("shopping ratePercent = %v, want 1", shoppingResult.RatePercent)
	}
	if !approxEqual(shoppingResult.Amount, 2000) {
		t.Errorf("shopping amount = %v, want 2000", shoppingResult.Amount)
	}

	bairro := base
	bairro.StoreType = "bairro"
	bairroResult := Calculate(bairro)
	if !approxEqual(bairroResult.RatePercent, 1.7) {
		t.Errorf("bairro ratePercent = %v, want 1.7", bairroResult.RatePercent)
	}
	if !approxEqual(bairroResult.Amount, 3400) {
		t.Errorf("bairro amount = %v, want 3400", bairroResult.Amount)
	}

	// store_type ausente => bairro (default).
	defaulted := base
	defaulted.StoreType = ""
	if got := Calculate(defaulted).RatePercent; !approxEqual(got, 1.7) {
		t.Errorf("default storeType ratePercent = %v, want 1.7 (bairro)", got)
	}
}

func TestCalculateSupportAmount(t *testing.T) {
	input := Input{
		Role:          "Caixa",
		StoreSold:     100000,
		StoreProgress: 95, // faixa 90->R$ 90 fixo
		Policy:        DefaultPolicy(),
	}

	result := Calculate(input)

	if result.Group != GroupSupport {
		t.Fatalf("group = %q, want support", result.Group)
	}
	if !approxEqual(result.Amount, 90) {
		t.Errorf("amount = %v, want 90 (fixed)", result.Amount)
	}
	if !approxEqual(result.RatePercent, 0) {
		t.Errorf("ratePercent = %v, want 0 for amount mode", result.RatePercent)
	}
}

func TestCalculateBelowLowestThresholdIsZero(t *testing.T) {
	// Loja em 40%: abaixo do menor threshold de qualquer grupo => sem faixa.
	manager := Input{
		Role:          "Gerente",
		StoreType:     "shopping",
		StoreSold:     100000,
		StoreProgress: 40,
		Policy:        DefaultPolicy(),
	}
	result := Calculate(manager)
	if result.RuleMatched {
		t.Fatalf("expected no rule matched below lowest threshold")
	}
	if !approxEqual(result.Amount, 0) {
		t.Errorf("amount = %v, want 0", result.Amount)
	}
	if result.RuleLabel != labelNoRule {
		t.Errorf("ruleLabel = %q, want %q", result.RuleLabel, labelNoRule)
	}
}

func TestNormalizePolicyLegacyManagerSeedsBothGroups(t *testing.T) {
	legacy := Policy{
		Manager: []Rule{{Threshold: 80, Value: 0.8, Mode: ModePercent}},
	}

	normalized := NormalizePolicy(legacy)

	if len(normalized.ManagerShopping) != 1 || !approxEqual(normalized.ManagerShopping[0].Value, 0.8) {
		t.Errorf("managerShopping = %#v, want seeded from legacy", normalized.ManagerShopping)
	}
	if len(normalized.ManagerBairro) != 1 || !approxEqual(normalized.ManagerBairro[0].Value, 0.8) {
		t.Errorf("managerBairro = %#v, want seeded from legacy", normalized.ManagerBairro)
	}
}

func TestNormalizePolicyExplicitGroupsBeatLegacy(t *testing.T) {
	policy := Policy{
		Manager:         []Rule{{Threshold: 80, Value: 0.8, Mode: ModePercent}},
		ManagerShopping: []Rule{{Threshold: 100, Value: 1, Mode: ModePercent}},
		ManagerBairro:   []Rule{}, // vazio explicito preservado
	}

	normalized := NormalizePolicy(policy)

	if len(normalized.ManagerShopping) != 1 || !approxEqual(normalized.ManagerShopping[0].Value, 1) {
		t.Errorf("managerShopping = %#v, want explicit", normalized.ManagerShopping)
	}
	if len(normalized.ManagerBairro) != 0 {
		t.Errorf("managerBairro = %#v, want empty preserved", normalized.ManagerBairro)
	}
}

func TestNormalizePolicyDefaultsWhenEmpty(t *testing.T) {
	normalized := NormalizePolicy(Policy{})

	if len(normalized.Consultant) != len(defaultConsultant) {
		t.Errorf("consultant = %#v, want defaults", normalized.Consultant)
	}
	if len(normalized.ManagerShopping) != len(defaultManagerShopping) {
		t.Errorf("managerShopping = %#v, want defaults", normalized.ManagerShopping)
	}
	if len(normalized.ManagerBairro) != len(defaultManagerBairro) {
		t.Errorf("managerBairro = %#v, want defaults", normalized.ManagerBairro)
	}
	if normalized.ConsultantRules != defaultConsultantRules {
		t.Errorf("consultantRules = %#v, want defaults", normalized.ConsultantRules)
	}
}

func TestNormalizePolicyPreservesExplicitEmptyConsultant(t *testing.T) {
	normalized := NormalizePolicy(Policy{Consultant: []Rule{}})
	if normalized.Consultant == nil || len(normalized.Consultant) != 0 {
		t.Errorf("consultant = %#v, want empty preserved (not defaults)", normalized.Consultant)
	}
}

func TestResolveRulePicksHighestThreshold(t *testing.T) {
	rules := []Rule{
		{Threshold: 80, Value: 1},
		{Threshold: 100, Value: 2},
		{Threshold: 120, Value: 3},
	}

	rule, ok := ResolveRule(rules, 110)
	if !ok || !approxEqual(rule.Value, 2) {
		t.Errorf("ResolveRule(110) = %#v ok=%v, want value 2", rule, ok)
	}

	if _, ok := ResolveRule(rules, 79); ok {
		t.Errorf("ResolveRule(79) matched, want no match below lowest threshold")
	}
}
