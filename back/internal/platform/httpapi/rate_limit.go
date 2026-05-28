package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// IdentityResolver extrai um identificador estavel do request para o bucket de rate limit.
// Quando retorna string vazia, o middleware tenta identificar o bearer token antes de cair
// para IP. Caller injeta a funcao para evitar import cycle com o pacote `auth` (httpapi nao
// pode importar auth, pois auth ja importa httpapi para escrever erros).
type IdentityResolver func(r *http.Request) string

// RateLimitOptions configura o middleware RateLimit. Limit e' o numero maximo de requisicoes
// permitidas dentro da janela (Window). Resolver opcional para usar identidade do usuario
// (ex: principal.UserID) em vez do IP — fallback automatico quando vazio.
//
// Buckets vivem em memoria e sao limpos periodicamente. Nao serve para deploy multi-instancia
// sem broker — aceitavel ate T9 trazer observabilidade real.
type RateLimitOptions struct {
	Limit    int
	Window   time.Duration
	Resolver IdentityResolver
}

// DefaultRESTRateLimit aplica 60 req/min por identidade, alinhado com a T8 do roadmap.
var DefaultRESTRateLimit = RateLimitOptions{
	Limit:  60,
	Window: time.Minute,
}

type rateLimitBucket struct {
	count      int
	resetAt    time.Time
	lastSeenAt time.Time
}

type rateLimiterStore struct {
	mu      sync.Mutex
	buckets map[string]*rateLimitBucket
}

func newRateLimiterStore() *rateLimiterStore {
	store := &rateLimiterStore{buckets: map[string]*rateLimitBucket{}}
	go store.cleanupLoop()
	return store
}

// allow incrementa o contador da identidade e devolve se a requisicao passa. Quando bloqueia,
// retorna tambem o tempo restante ate o reset (para o header Retry-After).
func (store *rateLimiterStore) allow(identity string, options RateLimitOptions, now time.Time) (bool, time.Duration) {
	if options.Limit <= 0 || options.Window <= 0 {
		return true, 0
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	bucket, ok := store.buckets[identity]
	if !ok || now.After(bucket.resetAt) {
		bucket = &rateLimitBucket{resetAt: now.Add(options.Window)}
		store.buckets[identity] = bucket
	}
	bucket.count++
	bucket.lastSeenAt = now

	if bucket.count > options.Limit {
		return false, time.Until(bucket.resetAt)
	}
	return true, 0
}

func (store *rateLimiterStore) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for now := range ticker.C {
		store.mu.Lock()
		for identity, bucket := range store.buckets {
			// Bucket inativo por mais de 10 minutos pode sair — janela maxima atual e' 1 min.
			if now.Sub(bucket.lastSeenAt) > 10*time.Minute {
				delete(store.buckets, identity)
			}
		}
		store.mu.Unlock()
	}
}

// RateLimit retorna um middleware que limita requisicoes por identidade. A identidade prefere
// `principal.UserID` (quando autenticado via RequireAuth), depois bearer/access_token, e por
// ultimo cai para o IP do client. Isso importa porque este middleware roda antes dos handlers
// autenticados injetarem o principal no contexto.
//
// Quando o limite e' excedido, responde 429 com header `Retry-After` em segundos.
func RateLimit(options RateLimitOptions) Middleware {
	if options.Limit <= 0 {
		options = DefaultRESTRateLimit
	}
	if options.Window <= 0 {
		options.Window = time.Minute
	}

	store := newRateLimiterStore()
	resolver := options.Resolver

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity := resolveRateLimitIdentity(r, resolver)
			allowed, retryAfter := store.allow(identity, options, time.Now())
			if !allowed {
				seconds := int(retryAfter.Round(time.Second).Seconds())
				if seconds < 1 {
					seconds = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
				WriteError(w, r, http.StatusTooManyRequests, "rate_limited", "Muitas requisicoes. Tente novamente em instantes.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func resolveRateLimitIdentity(r *http.Request, resolver IdentityResolver) string {
	if resolver != nil {
		if identity := strings.TrimSpace(resolver(r)); identity != "" {
			return "user:" + identity
		}
	}

	if token := bearerTokenFromRequest(r); token != "" {
		return "token:" + shortTokenHash(token)
	}

	// Fallback: IP do client. Honra X-Forwarded-For quando presente (esperado em proxy/CDN);
	// senao usa o RemoteAddr direto.
	forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwarded != "" {
		// X-Forwarded-For: "client, proxy1, proxy2" — primeiro IP e' o cliente original.
		if commaIndex := strings.Index(forwarded, ","); commaIndex > 0 {
			forwarded = forwarded[:commaIndex]
		}
		return "ip:" + strings.TrimSpace(forwarded)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "ip:" + r.RemoteAddr
	}
	return "ip:" + host
}

func bearerTokenFromRequest(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[len("Bearer "):])
	}

	return strings.TrimSpace(r.URL.Query().Get("access_token"))
}

func shortTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:8])
}
