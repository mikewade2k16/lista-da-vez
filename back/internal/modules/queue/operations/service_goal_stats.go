package operations

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"
)

// effectiveGoalsCacheTTL controla por quanto tempo a meta mensal canonica por
// consultor (operation_goal_targets, via Repository.EffectiveMonthlyGoalByConsultant)
// fica cacheada por (storeIDs ordenados, mes). Snapshot/overview da Operacao sao hot
// path (realtime: o front refaz a leitura apos cada mutacao e a cada evento), entao
// rodar essa query no Postgres a cada chamada e desperdicio. A meta muda raramente;
// 60s e curto o bastante para refletir edicao de meta sem o operador esperar.
const effectiveGoalsCacheTTL = 60 * time.Second

// storeTenantCacheTTL controla o cache de store -> tenant_id (GetStoreTenantID),
// usado so quando o principal nao traz escopo (ex.: platform_admin em rota
// RequireAuth sem X-Account-Id). store -> tenant e praticamente imutavel, entao o
// TTL e generoso para nao bater no banco a cada snapshot.
const storeTenantCacheTTL = 300 * time.Second

// effectiveGoalsCacheEntry guarda o resultado da meta canonica por consultor com a
// hora de expiracao. Erro NUNCA e cacheado (mantem a degradacao graciosa: na proxima
// chamada tenta de novo).
type effectiveGoalsCacheEntry struct {
	goals     map[string]float64
	expiresAt time.Time
}

// storeTenantCacheEntry guarda o tenant_id de uma loja com a hora de expiracao.
type storeTenantCacheEntry struct {
	tenantID  string
	expiresAt time.Time
}

// goalStatsForTenant busca o atingimento de meta do MES CORRENTE por consultor.
// Nil-safe: provider ausente, tenant vazio ou erro => map nil (snapshot/overview
// degradam para GoalStats=nil). Erro nao propaga: e enriquecimento, nao hot path
// critico, e o numero canonico continua disponivel em /v1/erp/crm. O cache de 120s
// por (tenant, mes) vive no adapter do ERP (operations_goal_progress_adapter.go).
func (service *Service) goalStatsForTenant(ctx context.Context, tenantID string) map[string]GoalStats {
	if service.goalProgressProvider == nil {
		return nil
	}

	trimmedTenantID := strings.TrimSpace(tenantID)
	if trimmedTenantID == "" {
		// Sem account/tenant resolvido o ERP nao tem escopo: o vendido fica 0 e o anel
		// so mostra a meta. Logamos para nao esconder de novo o caso "platform_admin com
		// TenantID vazio" (o escopo correto vem do AccountID via ScopeTenantID()).
		slog.Warn("operations_goal_stats_no_scope")
		return nil
	}

	month := time.Now().UTC().Format("2006-01")
	stats, err := service.goalProgressProvider.GoalStatsByConsultant(ctx, trimmedTenantID, month)
	if err != nil {
		slog.Warn("operations_goal_stats_unavailable",
			slog.String("tenant_id", trimmedTenantID),
			slog.String("month", month),
			slog.Any("error", err),
		)
		return nil
	}
	return stats
}

// effectiveGoalsByConsultant busca a meta mensal CANONICA por consultor
// (operation_goal_targets) do mes corrente. Nil-safe: erro vira map nil (o anel
// degrada para "sem meta"), pois e enriquecimento e nao deve quebrar a operacao.
//
// Cacheado em memoria (TTL effectiveGoalsCacheTTL) por chave =
// (storeIDs unicos+ordenados juntados) + "|" + mes "YYYY-MM". O mes na chave garante
// que a virada de mes nunca devolve dado stale do mes anterior. Erro NAO e cacheado.
func (service *Service) effectiveGoalsByConsultant(ctx context.Context, storeIDs []string) map[string]float64 {
	if len(storeIDs) == 0 {
		return nil
	}

	now := time.Now().UTC()
	month := now.Format("2006-01")
	cacheKey := effectiveGoalsCacheKey(storeIDs, month)

	if cached, ok := service.lookupEffectiveGoalsCache(cacheKey); ok {
		return cached
	}

	goals, err := service.repository.EffectiveMonthlyGoalByConsultant(ctx, storeIDs, now)
	if err != nil {
		slog.Warn("operations_effective_goals_unavailable", slog.Any("error", err))
		return nil
	}

	service.storeEffectiveGoalsCache(cacheKey, goals)
	return goals
}

// effectiveGoalsCacheKey monta a chave do cache normalizando os storeIDs (trim,
// dedup, ordenacao) para que a mesma combinacao de lojas em qualquer ordem caia no
// mesmo slot. O mes entra no fim para isolar a virada de mes.
func effectiveGoalsCacheKey(storeIDs []string, month string) string {
	unique := make(map[string]struct{}, len(storeIDs))
	ordered := make([]string, 0, len(storeIDs))
	for _, storeID := range storeIDs {
		trimmed := strings.TrimSpace(storeID)
		if trimmed == "" {
			continue
		}
		if _, seen := unique[trimmed]; seen {
			continue
		}
		unique[trimmed] = struct{}{}
		ordered = append(ordered, trimmed)
	}
	sort.Strings(ordered)
	return strings.Join(ordered, ",") + "|" + month
}

func (service *Service) lookupEffectiveGoalsCache(cacheKey string) (map[string]float64, bool) {
	service.effectiveGoalsMu.Lock()
	defer service.effectiveGoalsMu.Unlock()

	entry, ok := service.effectiveGoalsCache[cacheKey]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.goals, true
}

func (service *Service) storeEffectiveGoalsCache(cacheKey string, goals map[string]float64) {
	service.effectiveGoalsMu.Lock()
	defer service.effectiveGoalsMu.Unlock()

	if service.effectiveGoalsCache == nil {
		service.effectiveGoalsCache = make(map[string]effectiveGoalsCacheEntry)
	}
	service.effectiveGoalsCache[cacheKey] = effectiveGoalsCacheEntry{
		goals:     goals,
		expiresAt: time.Now().Add(effectiveGoalsCacheTTL),
	}
}

// storeTenantID resolve o tenant_id dono da loja, com cache em memoria (TTL
// storeTenantCacheTTL). Usado so no fallback de escopo do ERP quando o principal nao
// traz account/tenant. Em erro, devolve "" sem cachear (mantem a degradacao
// graciosa: o snapshot segue com goalStats=nil).
func (service *Service) storeTenantID(ctx context.Context, storeID string) string {
	trimmed := strings.TrimSpace(storeID)
	if trimmed == "" {
		return ""
	}

	if cached, ok := service.lookupStoreTenantCache(trimmed); ok {
		return cached
	}

	tenantID, err := service.repository.GetStoreTenantID(ctx, trimmed)
	if err != nil {
		return ""
	}

	service.storeStoreTenantCache(trimmed, tenantID)
	return tenantID
}

func (service *Service) lookupStoreTenantCache(storeID string) (string, bool) {
	service.storeTenantMu.Lock()
	defer service.storeTenantMu.Unlock()

	entry, ok := service.storeTenantCache[storeID]
	if !ok || time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.tenantID, true
}

func (service *Service) storeStoreTenantCache(storeID string, tenantID string) {
	service.storeTenantMu.Lock()
	defer service.storeTenantMu.Unlock()

	if service.storeTenantCache == nil {
		service.storeTenantCache = make(map[string]storeTenantCacheEntry)
	}
	service.storeTenantCache[storeID] = storeTenantCacheEntry{
		tenantID:  tenantID,
		expiresAt: time.Now().Add(storeTenantCacheTTL),
	}
}

// combineGoalStats monta o GoalStats final por consultor priorizando a meta
// CANONICA (operation_goal_targets). Quando o ERP ja cobre o consultor (meta>0),
// usa o stat do ERP inteiro, batendo exatamente com /v1/erp/crm. Para os demais
// com meta cadastrada (ex.: consultor sem venda/vinculo ERP), usa a meta canonica
// com o vendido do ERP quando houver (senao 0). Consultor sem meta fica de fora
// (map nil naquela chave => anel neutro no front).
func combineGoalStats(providerStats map[string]GoalStats, metaByConsultant map[string]float64) map[string]GoalStats {
	final := make(map[string]GoalStats)

	for consultantID, stat := range providerStats {
		if stat.HasGoal && stat.MonthlyGoal > 0 {
			final[consultantID] = stat
		}
	}

	for consultantID, meta := range metaByConsultant {
		if meta <= 0 {
			continue
		}
		if _, ok := final[consultantID]; ok {
			continue
		}

		sold := 0.0
		if stat, ok := providerStats[consultantID]; ok {
			sold = stat.SoldValue
		}

		remaining := meta - sold
		if remaining < 0 {
			remaining = 0
		}
		progress := (sold / meta) * 100

		final[consultantID] = GoalStats{
			MonthlyGoal:     meta,
			SoldValue:       sold,
			RemainingToGoal: remaining,
			Progress:        progress,
			HasGoal:         true,
		}
	}

	if len(final) == 0 {
		return nil
	}
	return final
}
