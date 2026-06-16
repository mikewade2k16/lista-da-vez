package cardapio

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter e um limitador por IP em memoria, com janela deslizante simples
// por chave logica (ex.: "orders", "events"). Independente do RateLimit global
// por usuario do httpapi (que e por user e nao cobre rotas publicas sem JWT).
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string][]time.Time
	now     func() time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string][]time.Time),
		now:     time.Now,
	}
}

// allow consome um slot para (scope, ip). Permite no maximo limit eventos por
// window. Retorna false quando estoura (429).
func (l *rateLimiter) allow(scope, ip string, limit int, window time.Duration) bool {
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

// clientIP extrai o IP do request, preferindo X-Forwarded-For (primeiro host) e
// caindo para RemoteAddr.
func clientIP(r *http.Request) string {
	if fwd := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); fwd != "" {
		parts := strings.Split(fwd, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
