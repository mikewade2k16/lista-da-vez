package commission

import (
	"sort"
	"strconv"
	"strings"
)

// Defaults v3 (espelham o contrato e o front em crm-performance-policy.ts).
// Faixas do consultor: threshold = % de atingimento da PROPRIA meta -> % sobre a
// propria venda (valem quando a loja >= StoreFullPercent).
var (
	defaultConsultant = []Rule{
		{Threshold: 80, Value: 1, Mode: ModePercent},
		{Threshold: 90, Value: 2, Mode: ModePercent},
		{Threshold: 100, Value: 3, Mode: ModePercent},
		{Threshold: 120, Value: 3.2, Mode: ModePercent},
	}

	defaultManagerShopping = []Rule{
		{Threshold: 80, Value: 0.8, Mode: ModePercent},
		{Threshold: 90, Value: 0.9, Mode: ModePercent},
		{Threshold: 100, Value: 1, Mode: ModePercent},
		{Threshold: 120, Value: 1.2, Mode: ModePercent},
	}

	defaultManagerBairro = []Rule{
		{Threshold: 80, Value: 1, Mode: ModePercent},
		{Threshold: 100, Value: 1.7, Mode: ModePercent},
		{Threshold: 120, Value: 2, Mode: ModePercent},
	}

	defaultSupport = []Rule{
		{Threshold: 80, Value: 80, Mode: ModeAmount},
		{Threshold: 90, Value: 90, Mode: ModeAmount},
		{Threshold: 100, Value: 100, Mode: ModeAmount},
		{Threshold: 120, Value: 120, Mode: ModeAmount},
	}

	defaultConsultantRules = ConsultantRules{
		Base:                      BaseSelf,
		QualityPenaltyPercent:     0.1,
		StoreFloorPercent:         50,
		StoreFullPercent:          80,
		ReducedRate:               1.5,
		ReducedRequiresOwnPercent: 100,
	}
)

// DefaultPolicy devolve a politica v2 padrao.
func DefaultPolicy() Policy {
	return Policy{
		Consultant:      cloneRules(defaultConsultant),
		ManagerShopping: cloneRules(defaultManagerShopping),
		ManagerBairro:   cloneRules(defaultManagerBairro),
		Support:         cloneRules(defaultSupport),
		ConsultantRules: defaultConsultantRules,
	}
}

var managerRoleTokens = []string{"manager", "gerente", "gerencia", "subgerente", "lider", "leader"}

var supportRoleTokens = []string{
	"support",
	"caixa",
	"cashier",
	"auxiliar",
	"assistant",
	"estoquista",
	"estoque",
	"financeiro",
	"recepcao",
}

// MapRoleToGroup mapeia o papel bruto para o grupo (consultant/manager/support).
// Normaliza minusculas + remove acentos antes de casar por token.
func MapRoleToGroup(role string) RoleGroup {
	normalized := removeAccents(strings.ToLower(strings.TrimSpace(role)))
	if normalized == "" {
		return GroupConsultant
	}
	for _, token := range managerRoleTokens {
		if strings.Contains(normalized, token) {
			return GroupManager
		}
	}
	for _, token := range supportRoleTokens {
		if strings.Contains(normalized, token) {
			return GroupSupport
		}
	}
	return GroupConsultant
}

// NormalizePolicy aplica defaults v2 + retrocompat. Faixas explicitamente
// vazias sao preservadas (usuario removeu tudo); campos AUSENTES caem no
// default. managerShopping/managerBairro ausentes sao semeados a partir do
// campo legado Manager quando ele existe; senao usam o default.
func NormalizePolicy(policy Policy) Policy {
	normalized := Policy{
		Consultant:      normalizeRules(policy.Consultant, defaultConsultant),
		Support:         normalizeRules(policy.Support, defaultSupport),
		ConsultantRules: normalizeConsultantRules(policy.ConsultantRules),
	}

	legacyManager := policy.Manager

	switch {
	case policy.ManagerShopping != nil:
		normalized.ManagerShopping = normalizeRules(policy.ManagerShopping, defaultManagerShopping)
	case legacyManager != nil:
		normalized.ManagerShopping = normalizeRules(legacyManager, defaultManagerShopping)
	default:
		normalized.ManagerShopping = cloneRules(defaultManagerShopping)
	}

	switch {
	case policy.ManagerBairro != nil:
		normalized.ManagerBairro = normalizeRules(policy.ManagerBairro, defaultManagerBairro)
	case legacyManager != nil:
		normalized.ManagerBairro = normalizeRules(legacyManager, defaultManagerBairro)
	default:
		normalized.ManagerBairro = cloneRules(defaultManagerBairro)
	}

	return normalized
}

// ResolveRule devolve a faixa de MAIOR threshold <= progress. Sem faixa: (Rule{}, false).
// As faixas devem vir ordenadas asc por threshold (NormalizePolicy garante isso).
func ResolveRule(rules []Rule, progress float64) (Rule, bool) {
	var selected Rule
	found := false
	for _, rule := range rules {
		if progress >= rule.Threshold {
			selected = rule
			found = true
		}
	}
	return selected, found
}

// Calculate executa o calculo unitario de comissao. Paridade exata com o
// contrato (e com o front crm-performance-policy.ts).
func Calculate(input Input) Result {
	policy := NormalizePolicy(input.Policy)
	group := MapRoleToGroup(input.Role)

	switch group {
	case GroupManager:
		return calculateManager(input, policy, group)
	case GroupSupport:
		return calculateSupport(input, policy, group)
	default:
		return calculateConsultant(input, policy, GroupConsultant)
	}
}

func calculateManager(input Input, policy Policy, group RoleGroup) Result {
	rules := policy.ManagerBairro
	if normalizeStoreType(input.StoreType) == "shopping" {
		rules = policy.ManagerShopping
	}

	rule, ok := ResolveRule(rules, input.StoreProgress)
	if !ok {
		return Result{Group: group, RuleLabel: labelNoRule}
	}

	if rule.Mode == ModeAmount {
		return Result{
			Amount:      rule.Value,
			Base:        input.StoreSold,
			Group:       group,
			RuleLabel:   amountLabel(rule.Value),
			RuleMatched: true,
		}
	}

	return Result{
		Amount:      input.StoreSold * rule.Value / 100,
		RatePercent: rule.Value,
		Base:        input.StoreSold,
		Group:       group,
		RuleLabel:   percentLabel(rule.Value, "da loja"),
		RuleMatched: true,
	}
}

func calculateSupport(input Input, policy Policy, group RoleGroup) Result {
	rule, ok := ResolveRule(policy.Support, input.StoreProgress)
	if !ok {
		return Result{Group: group, RuleLabel: labelNoRule}
	}

	if rule.Mode == ModeAmount {
		return Result{
			Amount:      rule.Value,
			Base:        input.StoreSold,
			Group:       group,
			RuleLabel:   amountLabel(rule.Value),
			RuleMatched: true,
		}
	}

	return Result{
		Amount:      input.StoreSold * rule.Value / 100,
		RatePercent: rule.Value,
		Base:        input.StoreSold,
		Group:       group,
		RuleLabel:   percentLabel(rule.Value, "da loja"),
		RuleMatched: true,
	}
}

func calculateConsultant(input Input, policy Policy, group RoleGroup) Result {
	rules := policy.ConsultantRules

	base := input.ConsultantSold
	baseLabel := "da propria venda"
	if rules.Base == BaseStore {
		base = input.StoreSold
		baseLabel = "da loja"
	}

	// Gate da loja sobre o recebimento do consultor.
	if input.StoreProgress < rules.StoreFloorPercent {
		return Result{Group: group, Base: base, RuleLabel: labelStoreBelowFloor}
	}

	var rate float64
	if input.StoreProgress < rules.StoreFullPercent {
		// Faixa REDUZIDA (loja entre floor e full): so recebe quem bateu a propria
		// meta minima exigida, e recebe a taxa reduzida fixa.
		if input.ConsultantProgress < rules.ReducedRequiresOwnPercent {
			return Result{Group: group, Base: base, RuleLabel: labelStoreReduced}
		}
		rate = rules.ReducedRate
	} else {
		// Loja saudavel: faixa propria pelo % de atingimento da PROPRIA meta.
		rule, ok := ResolveRule(policy.Consultant, input.ConsultantProgress)
		if !ok {
			return Result{Group: group, Base: base, RuleLabel: labelBelowOwnGoal}
		}
		rate = rule.Value
	}

	missedMetrics := 0.0
	if !input.HitPa {
		missedMetrics++
	}
	if !input.HitTicket {
		missedMetrics++
	}
	penaltyApplied := rules.QualityPenaltyPercent * missedMetrics

	effectiveRate := rate - penaltyApplied
	if effectiveRate < 0 {
		effectiveRate = 0
	}

	return Result{
		Amount:         base * effectiveRate / 100,
		RatePercent:    effectiveRate,
		Base:           base,
		Group:          group,
		RuleLabel:      percentLabel(effectiveRate, baseLabel),
		PenaltyApplied: penaltyApplied,
		RuleMatched:    true,
	}
}

const (
	labelNoRule          = "Sem faixa"
	labelBelowOwnGoal    = "Abaixo da meta propria"
	labelStoreBelowFloor = "Loja abaixo do minimo"
	labelStoreReduced    = "Loja parcial: exige 100% da meta propria"
)

func normalizeRules(rules []Rule, fallback []Rule) []Rule {
	if rules == nil {
		return cloneRules(fallback)
	}

	normalized := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		normalized = append(normalized, Rule{
			Threshold: clampRate(rule.Threshold),
			Value:     clampRate(rule.Value),
			Mode:      normalizeMode(rule.Mode),
		})
	}

	sort.SliceStable(normalized, func(left, right int) bool {
		return normalized[left].Threshold < normalized[right].Threshold
	})

	return normalized
}

func normalizeMode(mode PayoutMode) PayoutMode {
	if mode == ModeAmount {
		return ModeAmount
	}
	return ModePercent
}

func normalizeConsultantRules(rules ConsultantRules) ConsultantRules {
	base := BaseSelf
	if rules.Base == BaseStore {
		base = BaseStore
	}

	// 0 cai no default (mantem o padrao do projeto; o editor sempre envia valores).
	pick := func(value, fallback float64) float64 {
		if value != 0 {
			return clampRate(value)
		}
		return fallback
	}

	return ConsultantRules{
		Base:                      base,
		QualityPenaltyPercent:     pick(rules.QualityPenaltyPercent, defaultConsultantRules.QualityPenaltyPercent),
		StoreFloorPercent:         pick(rules.StoreFloorPercent, defaultConsultantRules.StoreFloorPercent),
		StoreFullPercent:          pick(rules.StoreFullPercent, defaultConsultantRules.StoreFullPercent),
		ReducedRate:               pick(rules.ReducedRate, defaultConsultantRules.ReducedRate),
		ReducedRequiresOwnPercent: pick(rules.ReducedRequiresOwnPercent, defaultConsultantRules.ReducedRequiresOwnPercent),
	}
}

func normalizeStoreType(storeType string) string {
	if strings.EqualFold(strings.TrimSpace(storeType), "shopping") {
		return "shopping"
	}
	return "bairro"
}

func clampRate(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1000 {
		return 1000
	}
	return value
}

func cloneRules(rules []Rule) []Rule {
	cloned := make([]Rule, len(rules))
	copy(cloned, rules)
	return cloned
}

func percentLabel(rate float64, suffix string) string {
	return formatNumber(rate) + "% " + suffix
}

func amountLabel(amount float64) string {
	return "R$ " + formatNumber(amount) + " fixo"
}

// formatNumber formata com virgula decimal (pt-BR) e sem zeros a direita.
func formatNumber(value float64) string {
	formatted := strconv.FormatFloat(value, 'f', -1, 64)
	return strings.Replace(formatted, ".", ",", 1)
}

func removeAccents(value string) string {
	var builder strings.Builder
	for _, char := range value {
		builder.WriteRune(foldAccentRune(char))
	}
	return builder.String()
}

func foldAccentRune(char rune) rune {
	switch char {
	case 'á', 'à', 'â', 'ã', 'ä':
		return 'a'
	case 'é', 'è', 'ê', 'ë':
		return 'e'
	case 'í', 'ì', 'î', 'ï':
		return 'i'
	case 'ó', 'ò', 'ô', 'õ', 'ö':
		return 'o'
	case 'ú', 'ù', 'û', 'ü':
		return 'u'
	case 'ç':
		return 'c'
	case 'ñ':
		return 'n'
	default:
		return char
	}
}
