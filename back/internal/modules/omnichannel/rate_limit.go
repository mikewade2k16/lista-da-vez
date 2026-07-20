package omnichannel

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter e um limitador por (escopo, ip) em memoria, janela deslizante. Padrao da casa
// para rota PUBLICA sem JWT (precedente cardapio/rate_limit.go) — o RateLimit global do
// httpapi e por identidade/JWT e nao cobre o webhook. Em memoria: nao serve multi-instancia
// sem broker (registrado em docs/LEGADO.md); hoje a api e um container so.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string][]time.Time
	now     func() time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{buckets: make(map[string][]time.Time), now: time.Now}
}

// allow consome um slot para (scope, ip): no maximo limit eventos por window. false = 429.
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
