package app

import (
	"log/slog"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/access"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/users"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/config"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

// wirePrincipalCache liga o PrincipalCache (AC-01) quando AUTH_PRINCIPAL_CACHE_TTL > 0.
// Retorna nil quando desligado (0s) — callers fazem nil-check antes de injetar.
// Invalidacao e direta/sincrona (nao via bus): logout, access e users derrubam a
// entrada na hora; o TTL e so o teto de exposicao dos caminhos sem invalidacao.
func wirePrincipalCache(
	cfg config.Config,
	logger *slog.Logger,
	authService *auth.Service,
	accessService *access.Service,
	usersService *users.Service,
) *httpapi.PrincipalCache[auth.Principal] {
	if cfg.AuthPrincipalCacheTTL <= 0 {
		logger.Info("principal_cache_disabled", "ttl", cfg.AuthPrincipalCacheTTL.String())
		return nil
	}

	cache := httpapi.NewPrincipalCache[auth.Principal](cfg.AuthPrincipalCacheTTL)
	authService.SetPrincipalCache(cache)
	accessService.SetPrincipalCacheInvalidator(cache)
	usersService.SetPrincipalCacheInvalidator(cache)
	logger.Info("principal_cache_enabled", "ttl", cfg.AuthPrincipalCacheTTL.String())

	// Manutencao: Cleanup a cada 60s; hit rate logado a cada 5 min quando houve trafego.
	go func() {
		const statsEvery = 5
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		tick := 0
		for range ticker.C {
			cache.Cleanup()
			tick++
			if tick%statsEvery != 0 {
				continue
			}
			hits, misses := cache.Stats()
			if total := hits + misses; total > 0 {
				logger.Info("principal_cache_stats",
					"hits", hits,
					"misses", misses,
					"hit_rate_pct", float64(hits)*100/float64(total),
					"entries", cache.Len(),
				)
			}
		}
	}()

	return cache
}
