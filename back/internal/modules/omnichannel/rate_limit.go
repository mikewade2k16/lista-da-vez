package omnichannel

import (
	"context"
	"crypto/sha256"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// rateLimiter protege rotas publicas sem JWT por (escopo, ip). O construtor usado pelo modulo
// persiste um bucket de janela fixa no PostgreSQL, compartilhado entre replicas e fail-closed.
// newRateLimiter preserva a implementacao local de janela deslizante somente para testes/fallbacks
// explicitamente montados sem pool; o RateLimit global do httpapi e por identidade/JWT e nao cobre
// webhook, avatar ou captura publica.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string][]time.Time
	now     func() time.Time
	pool    *pgxpool.Pool
	ops     atomic.Uint64
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{buckets: make(map[string][]time.Time), now: time.Now}
}

func newSharedRateLimiter(pool *pgxpool.Pool) *rateLimiter {
	limiter := newRateLimiter()
	limiter.pool = pool
	return limiter
}

// allow consome um slot para (scope, ip): no maximo limit eventos por window. false = 429.
func (l *rateLimiter) allow(scope, ip string, limit int, window time.Duration) bool {
	if l.pool != nil {
		return l.allowShared(scope, ip, limit, window)
	}
	return l.allowLocal(scope, ip, limit, window)
}

func (l *rateLimiter) allowShared(scope, ip string, limit int, window time.Duration) bool {
	key := sha256.Sum256([]byte(scope + "|" + ip))
	now := l.now().UTC()
	expiresAt := now.Add(window)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var allowed bool
	err := l.pool.QueryRow(ctx, `insert into messaging.runtime_rate_limit_buckets
		(bucket_key,hits,window_started,expires_at) values ($1,1,$2,$3)
		on conflict(bucket_key) do update set
		 hits=case when messaging.runtime_rate_limit_buckets.expires_at <= $2
		           then 1 else messaging.runtime_rate_limit_buckets.hits+1 end,
		 window_started=case when messaging.runtime_rate_limit_buckets.expires_at <= $2
		                     then $2 else messaging.runtime_rate_limit_buckets.window_started end,
		 expires_at=case when messaging.runtime_rate_limit_buckets.expires_at <= $2
		                 then $3 else messaging.runtime_rate_limit_buckets.expires_at end
		returning hits <= $4`, key[:], now, expiresAt, limit).Scan(&allowed)
	if err != nil {
		return false
	}
	if l.ops.Add(1)%128 == 0 {
		_, _ = l.pool.Exec(ctx, `delete from messaging.runtime_rate_limit_buckets
			where bucket_key in (select bucket_key from messaging.runtime_rate_limit_buckets
			 where expires_at <= $1 limit 500)`, now)
	}
	return allowed
}

func (l *rateLimiter) allowLocal(scope, ip string, limit int, window time.Duration) bool {
	key := scope + "|" + ip
	now := l.now()
	cutoff := now.Add(-window)

	l.mu.Lock()
	defer l.mu.Unlock()

	hits := l.buckets[key]
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= limit {
		l.buckets[key] = kept
		return false
	}
	l.buckets[key] = append(kept, now)
	return true
}

// clientIP usa o ULTIMO hop de X-Forwarded-For (o IP que o proxy confiavel — Caddy — anexou),
// nao o primeiro (que o cliente controla e forjaria para escapar do limite). Sem XFF, cai
// para RemoteAddr. Premissa de prod: exatamente UM proxy confiavel na frente.
func clientIP(r *http.Request) string {
	if fwd := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); fwd != "" {
		parts := strings.Split(fwd, ",")
		if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
