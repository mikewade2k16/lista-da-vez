package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newOkHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRateLimit_AllowsWithinLimit(t *testing.T) {
	middleware := RateLimit(RateLimitOptions{Limit: 3, Window: time.Minute})
	handler := middleware(newOkHandler())

	for index := 0; index < 3; index++ {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		req.RemoteAddr = "127.0.0.1:0"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("requisicao %d dentro do limite deveria passar (200); got %d", index+1, rr.Code)
		}
	}
}

func TestRateLimit_BlocksAfterLimit(t *testing.T) {
	middleware := RateLimit(RateLimitOptions{Limit: 2, Window: time.Minute})
	handler := middleware(newOkHandler())

	// Esgota o limite (2 reqs).
	for index := 0; index < 2; index++ {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		req.RemoteAddr = "127.0.0.1:0"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("preparacao: req %d deveria passar; got %d", index+1, rr.Code)
		}
	}

	// 3a request deve bloquear.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.RemoteAddr = "127.0.0.1:0"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("requisicao acima do limite deveria voltar 429; got %d", rr.Code)
	}
	if retryAfter := rr.Header().Get("Retry-After"); retryAfter == "" {
		t.Errorf("Retry-After deveria ser preenchido com segundos restantes; got vazio")
	}
}

func TestRateLimit_ResolverPrecedesIP(t *testing.T) {
	// Identidades diferentes (mesma IP, users diferentes) nao podem ser limitadas juntas.
	middleware := RateLimit(RateLimitOptions{
		Limit:  1,
		Window: time.Minute,
		Resolver: func(r *http.Request) string {
			return r.Header.Get("X-User")
		},
	})
	handler := middleware(newOkHandler())

	for _, user := range []string{"u1", "u2", "u3"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		req.RemoteAddr = "127.0.0.1:0"
		req.Header.Set("X-User", user)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("user %q tem bucket proprio; primeira req deveria passar; got %d", user, rr.Code)
		}
	}

	// Segunda request do u1 deve bloquear (mesma identidade).
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.RemoteAddr = "127.0.0.1:0"
	req.Header.Set("X-User", "u1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("segunda req de u1 deveria bater no limite (429); got %d", rr.Code)
	}
}

func TestRateLimit_ResolverEmptyFallsBackToIP(t *testing.T) {
	middleware := RateLimit(RateLimitOptions{
		Limit:    1,
		Window:   time.Minute,
		Resolver: func(_ *http.Request) string { return "" }, // sem identidade
	})
	handler := middleware(newOkHandler())

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req1.RemoteAddr = "10.0.0.1:0"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("primeira req IP1 deveria passar; got %d", rr1.Code)
	}

	// Mesma IP, segunda req — bloqueia.
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req2.RemoteAddr = "10.0.0.1:0"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("segunda req mesma IP deveria voltar 429; got %d", rr2.Code)
	}

	// IP diferente — bucket proprio, passa.
	req3 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req3.RemoteAddr = "10.0.0.2:0"
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Errorf("IP2 tem bucket proprio; primeira req deveria passar; got %d", rr3.Code)
	}
}

func TestRateLimit_UnvalidatedTokenDoesNotOpenNewBuckets(t *testing.T) {
	// Sem Resolver (token nao validado), trocar o bearer a cada request NAO escapa do limite:
	// a chave e' o IP de origem.
	middleware := RateLimit(RateLimitOptions{Limit: 1, Window: time.Minute})
	handler := middleware(newOkHandler())

	codes := []int{}
	for _, token := range []string{"token-a", "token-b", "token-c"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x?access_token="+token, nil)
		req.RemoteAddr = "203.0.113.5:1234"
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		codes = append(codes, rr.Code)
	}
	if codes[0] != http.StatusOK || codes[1] != http.StatusTooManyRequests || codes[2] != http.StatusTooManyRequests {
		t.Errorf("trocar token nao deve abrir bucket novo; got %v", codes)
	}
}

func TestRateLimit_XForwardedForIgnoredFromUntrustedOrigin(t *testing.T) {
	middleware := RateLimit(RateLimitOptions{Limit: 1, Window: time.Minute})
	handler := middleware(newOkHandler())

	codes := []int{}
	for _, forged := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		req.RemoteAddr = "203.0.113.5:1234" // origem publica: nao e' proxy confiavel
		req.Header.Set("X-Forwarded-For", forged)
		req.Header.Set("X-Real-IP", forged)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		codes = append(codes, rr.Code)
	}
	if codes[0] != http.StatusOK || codes[1] != http.StatusTooManyRequests || codes[2] != http.StatusTooManyRequests {
		t.Errorf("XFF forjado de origem nao confiavel deve ser ignorado; got %v", codes)
	}
}

func TestRateLimit_XForwardedForHonoredFromTrustedProxy(t *testing.T) {
	middleware := RateLimit(RateLimitOptions{Limit: 1, Window: time.Minute})
	handler := middleware(newOkHandler())

	send := func(xff string) int {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		req.RemoteAddr = "172.18.0.5:1234" // Caddy na rede Docker
		req.Header.Set("X-Forwarded-For", xff)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	if code := send("198.51.100.7"); code != http.StatusOK {
		t.Fatalf("primeira req deveria passar; got %d", code)
	}
	// Cliente tenta forjar o primeiro hop; o ultimo hop nao confiavel (anexado pelo proxy) vale.
	if code := send("9.9.9.9, 198.51.100.7"); code != http.StatusTooManyRequests {
		t.Errorf("mesmo IP real deve compartilhar bucket mesmo com hop forjado; got %d", code)
	}
	if code := send("198.51.100.8"); code != http.StatusOK {
		t.Errorf("IP real diferente tem bucket proprio; got %d", code)
	}
}

func TestRateLimit_AnonymousAccountHeaderDoesNotConsumeTenantQuota(t *testing.T) {
	// AccountResolver que so devolve tenant de token validado: anonimo (resolver vazio) nao conta.
	middleware := RateLimit(RateLimitOptions{
		Limit:           100,
		Window:          time.Minute,
		AccountLimit:    1,
		AccountResolver: func(*http.Request) string { return "" },
	})
	handler := middleware(newOkHandler())

	for index := 0; index < 5; index++ {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		req.RemoteAddr = "203.0.113.5:1"
		req.Header.Set("X-Account-Id", "vitima")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("req %d anonima nao deve gastar cota de tenant; got %d", index+1, rr.Code)
		}
	}
}

func TestRateLimit_AccountQuotaBlocksNoisyTenant(t *testing.T) {
	// Account "acc-a" tem cota de 2 (AccountLimit=2); usuarios distintos compartilham o bucket.
	middleware := RateLimit(RateLimitOptions{
		Limit:        10, // cota individual alta para nao interferir
		Window:       time.Minute,
		AccountLimit: 2,
		AccountResolver: func(r *http.Request) string {
			return r.Header.Get("X-Account-Id")
		},
	})
	handler := middleware(newOkHandler())

	do := func(userIP, accountID string) int {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		req.RemoteAddr = userIP + ":0"
		req.Header.Set("X-Account-Id", accountID)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	// Dois usuarios distintos da mesma account passam (dentro da cota de 2).
	if code := do("1.1.1.1", "acc-a"); code != http.StatusOK {
		t.Fatalf("1a req acc-a: want 200, got %d", code)
	}
	if code := do("2.2.2.2", "acc-a"); code != http.StatusOK {
		t.Fatalf("2a req acc-a: want 200, got %d", code)
	}
	// 3o usuario da mesma account deve ser bloqueado (noisy-neighbor).
	if code := do("3.3.3.3", "acc-a"); code != http.StatusTooManyRequests {
		t.Errorf("3a req acc-a deveria bater na cota do tenant (429); got %d", code)
	}
}

func TestRateLimit_AccountQuotaDoesNotAffectOtherTenants(t *testing.T) {
	middleware := RateLimit(RateLimitOptions{
		Limit:        10,
		Window:       time.Minute,
		AccountLimit: 1, // cota minima: 1 req por tenant
		AccountResolver: func(r *http.Request) string {
			return r.Header.Get("X-Account-Id")
		},
	})
	handler := middleware(newOkHandler())

	do := func(ip, accountID string) int {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		req.RemoteAddr = ip + ":0"
		req.Header.Set("X-Account-Id", accountID)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	// acc-a esgota sua cota.
	if code := do("1.1.1.1", "acc-a"); code != http.StatusOK {
		t.Fatalf("acc-a 1a req: want 200, got %d", code)
	}
	if code := do("2.2.2.2", "acc-a"); code != http.StatusTooManyRequests {
		t.Fatalf("acc-a 2a req deveria bloquear; got %d", code)
	}

	// acc-b tem bucket proprio — nao e' afetado pelo estouro de acc-a.
	if code := do("3.3.3.3", "acc-b"); code != http.StatusOK {
		t.Errorf("acc-b deve ter bucket independente; got %d", code)
	}
}

func TestRateLimit_NoAccountResolverSkipsAccountCheck(t *testing.T) {
	// Sem AccountResolver: comportamento legado inalterado.
	middleware := RateLimit(RateLimitOptions{Limit: 2, Window: time.Minute})
	handler := middleware(newOkHandler())

	for i := 0; i < 2; i++ {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		req.RemoteAddr = "10.0.0.1:0"
		req.Header.Set("X-Account-Id", "acc-z") // header presente mas ignorado sem resolver
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("req %d dentro do limite: want 200, got %d", i+1, rr.Code)
		}
	}
}

func TestRateLimiterStore_ResetsAfterWindow(t *testing.T) {
	store := newRateLimiterStore()
	options := RateLimitOptions{Limit: 1, Window: 50 * time.Millisecond}
	now := time.Now()

	if ok, _ := store.allow("u1", options, now); !ok {
		t.Fatal("primeira chamada deveria passar")
	}
	if ok, _ := store.allow("u1", options, now); ok {
		t.Fatal("segunda chamada dentro da janela deveria bloquear")
	}
	// Janela expira -> bucket reseta.
	if ok, _ := store.allow("u1", options, now.Add(60*time.Millisecond)); !ok {
		t.Error("apos a janela, contador reseta e proxima chamada passa")
	}
}
