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
	return l.allowN(scope, ip, 1, limit, window)
}

// allowN consome n slots de uma vez para (scope, ip), tudo-ou-nada: se os n nao
// couberem dentro do limite na janela, nenhum e consumido e retorna false (429). Usado
// pela ingestao em lote (debita len(events) de uma vez). n<=0 e tratado como 1.
func (l *rateLimiter) allowN(scope, ip string, n, limit int, window time.Duration) bool {
	if n < 1 {
		n = 1
	}
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
	if len(kept)+n > limit {
		l.buckets[key] = kept
		return false
	}
	for i := 0; i < n; i++ {
		kept = append(kept, now)
	}
	l.buckets[key] = kept
	return true
}

// clientIP extrai o IP do request para a chave do rate limit. Usa o ULTIMO hop
// de X-Forwarded-For (o IP que o proxy CONFIAVEL mais proximo — Caddy em prod —
// observou e anexou), nao o primeiro: o primeiro elemento e totalmente
// controlado pelo cliente e seria trivial de forjar para escapar do limite
// (cada request com um XFF diferente caia num bucket novo). Sem XFF, cai para
// RemoteAddr (conexao direta).
//
// PREMISSA DE PROD (ver AGENT.md): assume-se EXATAMENTE UM proxy confiavel na
// frente (Caddy), que reescreve/anexa o IP real ao XFF. Se um dia houver mais de
// um proxy, o ultimo hop deixa de ser o cliente e este calculo precisa do
// numero de proxies confiaveis para indexar o hop correto.
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
