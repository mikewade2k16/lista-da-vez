package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// IdentityResolver extrai um identificador estavel do request para o bucket de rate limit.
// Quando retorna string vazia, o middleware cai para o IP de origem confiavel (ClientIP). Caller injeta a funcao para evitar import cycle com o pacote `auth` (httpapi nao
// pode importar auth, pois auth ja importa httpapi para escrever erros).
type IdentityResolver func(r *http.Request) string

// RateLimitOptions configura o middleware RateLimit. Limit e' o numero maximo de requisicoes
// permitidas dentro da janela (Window). Resolver opcional para usar identidade do usuario
// (ex: principal.UserID) em vez do IP — fallback automatico quando vazio.
//
// AccountResolver e AccountLimit adicionam uma segunda cota por tenant (account_id). Quando
// configurados, o middleware verifica AMBAS as cotas; qualquer estouro retorna 429. Isso impede
// que um tenant com muitos usuarios degrade vizinhos (noisy-neighbor). Buckets vivem em memoria
// e sao limpos periodicamente. Nao serve para deploy multi-instancia sem broker.
type RateLimitOptions struct {
	Limit    int
	Window   time.Duration
	Resolver IdentityResolver

	// AccountResolver extrai o account_id do request (ex: r.Header.Get("X-Account-Id")).
	// Quando vazio, a cota por tenant nao e' aplicada.
	AccountResolver IdentityResolver
	// AccountLimit e' o maximo de requisicoes por account dentro da Window.
	// Quando zero e AccountResolver estiver definido, usa 5x o Limit como padrao.
	AccountLimit int
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
		if !ok && len(store.buckets) >= maxRateLimitBuckets {
			store.evictLocked(now)
		}
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

// maxRateLimitBuckets limita a memoria do store. Ao atingir o teto, buckets expirados saem; se
// ainda assim estiver cheio, o mais antigo e' descartado.
const maxRateLimitBuckets = 100000

func (store *rateLimiterStore) evictLocked(now time.Time) {
	var oldestKey string
	var oldest time.Time
	for key, bucket := range store.buckets {
		if now.After(bucket.resetAt) {
			delete(store.buckets, key)
			continue
		}
		if oldestKey == "" || bucket.lastSeenAt.Before(oldest) {
			oldestKey, oldest = key, bucket.lastSeenAt
		}
	}
	if len(store.buckets) >= maxRateLimitBuckets && oldestKey != "" {
		delete(store.buckets, oldestKey)
	}
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

// RateLimit retorna um middleware que limita requisicoes por identidade. Requisicao autenticada
// e' chaveada pelo usuario (Resolver, que deve validar a assinatura do token: este middleware
// roda antes do RequireAuth); sem identidade valida, pelo IP de origem confiavel (ClientIP).
// X-Forwarded-For so conta quando a conexao vem de proxy confiavel (TRUSTED_PROXIES).
//
// Quando AccountResolver estiver configurado, aplica uma segunda cota por tenant. O resolver
// deve devolver o tenant do TOKEN VALIDADO, nunca de header do client: requisicao anonima nao
// consome a cota de nenhum tenant. Ambas as cotas precisam passar; estouro retorna 429.
//
// Quando o limite e' excedido, responde 429 com header `Retry-After` em segundos.
func RateLimit(options RateLimitOptions) Middleware {
	if options.Limit <= 0 {
		options = DefaultRESTRateLimit
	}
	if options.Window <= 0 {
		options.Window = time.Minute
	}

	accountLimit := options.AccountLimit
	if options.AccountResolver != nil && accountLimit <= 0 {
		accountLimit = options.Limit * 5
	}

	store := newRateLimiterStore()
	accountStore := newRateLimiterStore()
	resolver := options.Resolver
	accountResolver := options.AccountResolver

	accountOptions := RateLimitOptions{Limit: accountLimit, Window: options.Window}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()

			identity := resolveRateLimitIdentity(r, resolver)
			allowed, retryAfter := store.allow(identity, options, now)
			if !allowed {
				writeRateLimitError(w, r, retryAfter)
				return
			}

			if accountResolver != nil {
				if accountID := strings.TrimSpace(accountResolver(r)); accountID != "" {
					accountAllowed, accountRetryAfter := accountStore.allow("account:"+accountID, accountOptions, now)
					if !accountAllowed {
						writeRateLimitError(w, r, accountRetryAfter)
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeRateLimitError(w http.ResponseWriter, r *http.Request, retryAfter time.Duration) {
	seconds := int(retryAfter.Round(time.Second).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	WriteError(w, r, http.StatusTooManyRequests, "rate_limited", "Muitas requisicoes. Tente novamente em instantes.")
}

func resolveRateLimitIdentity(r *http.Request, resolver IdentityResolver) string {
	// Identidade autenticada: o Resolver deve devolver o usuario somente de token com assinatura
	// valida. Token invalido/ausente NUNCA vira chave (o atacante trocaria o token a cada request
	// para abrir buckets novos): cai para o IP de origem confiavel.
	if resolver != nil {
		if identity := strings.TrimSpace(resolver(r)); identity != "" {
			return "user:" + identity
		}
	}

	return "ip:" + ClientIP(r)
}
