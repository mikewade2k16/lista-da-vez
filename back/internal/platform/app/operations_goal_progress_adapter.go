package app

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/crm/erp"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/operations"
)

// goalProgressCacheTTL controla por quanto tempo o atingimento de meta por
// consultor fica cacheado por (tenant, mes). O snapshot da Operacao e hot path
// (realtime), entao rodar o overview inteiro do CRM a cada chamada e proibitivo.
const goalProgressCacheTTL = 120 * time.Second

// operationsGoalProgressAdapter cumpre operations.GoalProgressProvider chamando o
// service do erp na composition root (onde tanto operations quanto crm/erp ja
// estao importados), evitando que operations importe erp (sem ciclo de import).
//
// E uma PONTE server-side: monta um principal privilegiado escopado ao tenant
// pedido para obter o numero canonico do CRM sem passar pelo gate canViewERP
// (decisao de produto "todos os operadores veem a meta de todos"). O escopo
// continua restrito ao tenant informado.
type operationsGoalProgressAdapter struct {
	erpService *erp.Service

	mu    sync.Mutex
	cache map[string]goalProgressCacheEntry
}

type goalProgressCacheEntry struct {
	stats     map[string]operations.GoalStats
	expiresAt time.Time
}

func newOperationsGoalProgressAdapter(erpService *erp.Service) *operationsGoalProgressAdapter {
	return &operationsGoalProgressAdapter{
		erpService: erpService,
		cache:      make(map[string]goalProgressCacheEntry),
	}
}

func (adapter *operationsGoalProgressAdapter) GoalStatsByConsultant(
	ctx context.Context,
	tenantID string,
	month string,
) (map[string]operations.GoalStats, error) {
	trimmedTenantID := strings.TrimSpace(tenantID)
	if trimmedTenantID == "" {
		return nil, nil
	}
	trimmedMonth := strings.TrimSpace(month)
	cacheKey := trimmedTenantID + "\x00" + trimmedMonth

	if cached, ok := adapter.lookupCache(cacheKey); ok {
		return cached, nil
	}

	// Principal privilegiado escopado ao tenant: platform_admin com TenantID
	// preenchido passa CanAccessTenant e nao exige filtro por loja
	// (requiresStoreScopedFilter=false), garantindo o escopo tenant-wide que
	// espelha a pagina de consultores.
	principal := auth.Principal{
		Role:     auth.RolePlatformAdmin,
		TenantID: trimmedTenantID,
	}

	erpStats, err := adapter.erpService.GoalStatsByConsultant(ctx, principal, trimmedTenantID, trimmedMonth)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]operations.GoalStats, len(erpStats))
	for consultantID, stat := range erpStats {
		stats[consultantID] = operations.GoalStats{
			MonthlyGoal:     stat.MonthlyGoal,
			SoldValue:       stat.SoldValue,
			RemainingToGoal: stat.RemainingToGoal,
			Progress:        stat.Progress,
			HasGoal:         stat.HasGoal,
		}
	}

	adapter.storeCache(cacheKey, stats)
	return stats, nil
}

func (adapter *operationsGoalProgressAdapter) lookupCache(cacheKey string) (map[string]operations.GoalStats, bool) {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()

	entry, ok := adapter.cache[cacheKey]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.stats, true
}

func (adapter *operationsGoalProgressAdapter) storeCache(cacheKey string, stats map[string]operations.GoalStats) {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()

	adapter.cache[cacheKey] = goalProgressCacheEntry{
		stats:     stats,
		expiresAt: time.Now().Add(goalProgressCacheTTL),
	}
}
