package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AccountModulesGuard valida que o modulo da rota esta habilitado para o
// account informado em X-Account-Id.
//
// Uso (Fase 6 em diante, modulos satelites):
//
//	guard := httpapi.NewAccountModulesGuard(pool)
//	mux.Handle("GET /v1/finance/invoices", guard.RequireModule("finance")(handler))
//
// Modulos legados (auth, tenants, stores, ... ate Fase 4) NAO usam este guard
// — eles continuam acessiveis sem header X-Account-Id por compatibilidade.
//
// O guard mantem cache em memoria com TTL curto (60s) para evitar SELECT por
// request. Cache invalidado por evento "account.modules.changed" via Invalidate.
type AccountModulesGuard struct {
	pool *pgxpool.Pool
	ttl  time.Duration

	// bypass, quando definido e retorna true para o request, libera a rota sem
	// checar modulo (ex.: platform_admin nao esta vinculado aos modulos de uma
	// account especifica e nunca deve ser barrado pelo gating). Definido no
	// app.go via SetBypass (httpapi nao pode importar auth — ciclo).
	bypass func(*http.Request) bool

	mu    sync.RWMutex
	cache map[string]cachedModules // key: accountID
}

type cachedModules struct {
	modules   map[string]struct{}
	expiresAt time.Time
}

// NewAccountModulesGuard cria o guard com TTL padrao de 60s.
func NewAccountModulesGuard(pool *pgxpool.Pool) *AccountModulesGuard {
	return &AccountModulesGuard{
		pool:  pool,
		ttl:   60 * time.Second,
		cache: make(map[string]cachedModules),
	}
}

// SetTTL ajusta o TTL do cache. Util em testes.
func (g *AccountModulesGuard) SetTTL(ttl time.Duration) {
	g.ttl = ttl
}

// SetBypass define um predicado que, quando retorna true para o request, libera
// a rota sem validar modulo. Usado para isentar platform_admin do gating.
func (g *AccountModulesGuard) SetBypass(fn func(*http.Request) bool) {
	g.bypass = fn
}

// RequireModule retorna middleware que so deixa o request prosseguir se o
// modulo informado estiver habilitado para o account em X-Account-Id.
//
// Erros retornados:
//   - 400 missing_account_id quando header ausente.
//   - 403 module_disabled quando modulo nao esta em core.account_modules
//     (ou esta com enabled = false).
//   - 500 quando consulta ao banco falha.
func (g *AccountModulesGuard) RequireModule(moduleID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			accountID := strings.TrimSpace(r.Header.Get("X-Account-Id"))
			if accountID == "" {
				WriteError(w, r, http.StatusBadRequest, "missing_account_id",
					"Header X-Account-Id e obrigatorio para esta rota.")
				return
			}

			enabled, err := g.IsEnabled(r.Context(), accountID, moduleID)
			if err != nil {
				WriteError(w, r, http.StatusInternalServerError, "internal_error",
					"Erro ao validar modulos da account.")
				return
			}
			if !enabled {
				WriteError(w, r, http.StatusForbidden, "module_disabled",
					"Modulo nao habilitado para esta account.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ModulePathRule mapeia um prefixo de path para o modulo que o protege.
// Ex: {Prefix: "/v1/erp", ModuleID: "crm"}.
type ModulePathRule struct {
	Prefix   string
	ModuleID string
}

// RequireModuleByPath retorna um middleware que gateia rotas cujo path casa com
// um dos prefixos em rules (gate-list). Para cada request escolhe a regra de
// prefixo mais longo que casa com r.URL.Path; se nenhuma casa, segue sem gatear
// (fail-open p/ rotas nao listadas). Quando casa, aplica a mesma logica de
// RequireModule (400 missing_account_id / 403 module_disabled / 500), com o
// moduleID da regra.
//
// Aplicado uma unica vez no Chain (espelha o MODULE_PATH_GUARDS do front), em
// vez de embrulhar cada handler dos modulos legados.
func (g *AccountModulesGuard) RequireModuleByPath(rules []ModulePathRule) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			moduleID := matchModuleRule(rules, r.URL.Path)
			if moduleID == "" {
				next.ServeHTTP(w, r)
				return
			}

			// platform_admin (ou outro caso definido pelo bypass) nao esta preso
			// aos modulos de uma account — passa direto.
			if g.bypass != nil && g.bypass(r) {
				next.ServeHTTP(w, r)
				return
			}

			accountID := strings.TrimSpace(r.Header.Get("X-Account-Id"))
			if accountID == "" {
				WriteError(w, r, http.StatusBadRequest, "missing_account_id",
					"Header X-Account-Id e obrigatorio para esta rota.")
				return
			}

			enabled, err := g.IsEnabled(r.Context(), accountID, moduleID)
			if err != nil {
				WriteError(w, r, http.StatusInternalServerError, "internal_error",
					"Erro ao validar modulos da account.")
				return
			}
			if !enabled {
				WriteError(w, r, http.StatusForbidden, "module_disabled",
					"Modulo nao habilitado para esta account.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// matchModuleRule devolve o moduleID da regra de prefixo mais longo que casa com
// path, ou "" quando nenhuma casa.
func matchModuleRule(rules []ModulePathRule, path string) string {
	best := ""
	bestLen := 0
	for _, rule := range rules {
		if len(rule.Prefix) > bestLen && pathHasSegmentPrefix(path, rule.Prefix) {
			best = rule.ModuleID
			bestLen = len(rule.Prefix)
		}
	}
	return best
}

// pathHasSegmentPrefix casa prefixo respeitando limite de segmento: "/v1/tasks"
// casa "/v1/tasks" e "/v1/tasks/123" mas NAO "/v1/task-boards" nem "/v1/tasksx".
func pathHasSegmentPrefix(path, prefix string) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	if len(path) == len(prefix) {
		return true
	}
	return path[len(prefix)] == '/'
}

// IsEnabled checa cache e (em miss) consulta core.account_modules.
func (g *AccountModulesGuard) IsEnabled(ctx context.Context, accountID, moduleID string) (bool, error) {
	if accountID == "" || moduleID == "" {
		return false, nil
	}

	if modules, ok := g.lookup(accountID); ok {
		_, enabled := modules[moduleID]
		return enabled, nil
	}

	modules, err := g.loadFromDB(ctx, accountID)
	if err != nil {
		return false, err
	}

	g.store(accountID, modules)
	_, enabled := modules[moduleID]
	return enabled, nil
}

// Invalidate remove o cache da account. Chamar quando publicar
// "account.modules.changed".
func (g *AccountModulesGuard) Invalidate(accountID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.cache, accountID)
}

// InvalidateAll limpa tudo. Util quando admin altera defaults globais.
func (g *AccountModulesGuard) InvalidateAll() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cache = make(map[string]cachedModules)
}

// ============================================================================
// Internals
// ============================================================================

func (g *AccountModulesGuard) lookup(accountID string) (map[string]struct{}, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	cached, ok := g.cache[accountID]
	if !ok {
		return nil, false
	}
	if time.Now().After(cached.expiresAt) {
		return nil, false
	}
	return cached.modules, true
}

func (g *AccountModulesGuard) store(accountID string, modules map[string]struct{}) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cache[accountID] = cachedModules{
		modules:   modules,
		expiresAt: time.Now().Add(g.ttl),
	}
}

func (g *AccountModulesGuard) loadFromDB(ctx context.Context, accountID string) (map[string]struct{}, error) {
	const query = `
		select module_id
		from core.account_modules
		where account_id = $1::uuid
		  and enabled = true
	`

	rows, err := g.pool.Query(ctx, query, accountID)
	if err != nil {
		// Caso o schema core ainda nao exista (CORE_V2_ENABLED ainda na Fase 0/1
		// em algum ambiente), interpretamos como "nenhum modulo habilitado".
		if errors.Is(err, pgx.ErrNoRows) {
			return map[string]struct{}{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	modules := make(map[string]struct{})
	for rows.Next() {
		var moduleID string
		if err := rows.Scan(&moduleID); err != nil {
			return nil, err
		}
		modules[moduleID] = struct{}{}
	}
	return modules, rows.Err()
}
