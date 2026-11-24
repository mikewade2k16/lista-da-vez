package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type loginTestRepo struct {
	UserRepository
	user User
}

func (r *loginTestRepo) FindByEmail(_ context.Context, email string) (User, error) {
	if r.user.Email != "" && strings.EqualFold(email, r.user.Email) {
		return r.user, nil
	}
	return User{}, ErrInvalidCredentials
}

// countingHasher considera valida somente a senha "certa" e conta as verificacoes.
type countingHasher struct {
	verifyCalls atomic.Int64
}

func (h *countingHasher) Hash(password string) (string, error) { return "hash:" + password, nil }
func (h *countingHasher) Verify(hash, password string) error {
	h.verifyCalls.Add(1)
	if hash == "hash:certa-123" && password == "certa-123" {
		return nil
	}
	return ErrInvalidCredentials
}

func newLoginTestMux(t *testing.T) (*http.ServeMux, *countingHasher) {
	t.Helper()
	hasher := &countingHasher{}
	repo := &loginTestRepo{user: User{
		ID: "u1", Email: "ana@omni.test", DisplayName: "Ana", Active: true,
		PasswordHash: "hash:certa-123", Role: RoleDirector, TenantID: "t1",
	}}
	service := NewService(repo, hasher, NewHMACTokenManager("segredo", time.Hour), nil, nil, nil, nil)
	mux := http.NewServeMux()
	RegisterRoutes(mux, service, nil, nil, NewMiddleware(service), GatewayConfig{})
	return mux, hasher
}

func doLogin(mux *http.ServeMux, remote, email, password string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/login", strings.NewReader(string(body)))
	req.RemoteAddr = remote
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func TestLogin_SixthFailureOfSameEmailIs429WithRetryAfter(t *testing.T) {
	mux, _ := newLoginTestMux(t)
	// IPs publicos distintos por tentativa: so o limite por e-mail pode disparar.
	for index := 0; index < 5; index++ {
		rr := doLogin(mux, "203.0.113."+string(rune('1'+index))+":1", "ana@omni.test", "errada")
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("falha %d: esperado 401; got %d", index+1, rr.Code)
		}
	}
	rr := doLogin(mux, "203.0.113.99:1", "Ana@Omni.test ", "certa-123") // inclusive com a senha certa
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("6a tentativa: esperado 429; got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Error("429 deve trazer Retry-After")
	}
}

func TestLogin_UnknownEmailBlocksIdenticallyAndMessageIsGeneric(t *testing.T) {
	mux, _ := newLoginTestMux(t)
	for index := 0; index < 5; index++ {
		doLogin(mux, "198.51.100."+string(rune('1'+index))+":1", "fantasma@omni.test", "x")
	}
	unknown := doLogin(mux, "198.51.100.90:1", "fantasma@omni.test", "x")

	for index := 0; index < 5; index++ {
		doLogin(mux, "192.0.2."+string(rune('1'+index))+":1", "ana@omni.test", "errada")
	}
	known := doLogin(mux, "192.0.2.90:1", "ana@omni.test", "errada")

	if unknown.Code != http.StatusTooManyRequests || known.Code != http.StatusTooManyRequests {
		t.Fatalf("ambos devem bloquear: %d / %d", unknown.Code, known.Code)
	}
	if unknown.Body.String() == "" || stripRequestID(unknown.Body.String()) != stripRequestID(known.Body.String()) {
		t.Errorf("corpo do 429 deve ser identico:\n%s\n%s", unknown.Body.String(), known.Body.String())
	}
}

func stripRequestID(body string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return body
	}
	delete(payload, "requestId")
	delete(payload, "request_id")
	out, _ := json.Marshal(payload)
	return string(out)
}

func TestLogin_SuccessResetsEmailCounter(t *testing.T) {
	mux, _ := newLoginTestMux(t)
	for index := 0; index < 4; index++ {
		doLogin(mux, "203.0.113."+string(rune('1'+index))+":1", "ana@omni.test", "errada")
	}
	if rr := doLogin(mux, "203.0.113.50:1", "ana@omni.test", "certa-123"); rr.Code != http.StatusOK {
		t.Fatalf("login correto deveria passar; got %d", rr.Code)
	}
	// Contador zerado: mais 4 falhas nao bloqueiam.
	for index := 0; index < 4; index++ {
		if rr := doLogin(mux, "203.0.113."+string(rune('6'+index))+":1", "ana@omni.test", "errada"); rr.Code != http.StatusUnauthorized {
			t.Fatalf("apos sucesso a falha %d deveria ser 401; got %d", index+1, rr.Code)
		}
	}
}

func TestLogin_DifferentIPsDoNotAffectEachOther(t *testing.T) {
	mux, _ := newLoginTestMux(t)
	// Um IP esgota seu limite (20 falhas) com e-mails distintos.
	for index := 0; index < 20; index++ {
		doLogin(mux, "203.0.113.7:1", "alvo"+string(rune('a'+index))+"@omni.test", "x")
	}
	if rr := doLogin(mux, "203.0.113.7:1", "novo@omni.test", "x"); rr.Code != http.StatusTooManyRequests {
		t.Fatalf("IP com 20 falhas deve ser bloqueado; got %d", rr.Code)
	}
	if rr := doLogin(mux, "203.0.113.8:1", "novo@omni.test", "x"); rr.Code != http.StatusUnauthorized {
		t.Fatalf("outro IP nao pode ser afetado; got %d", rr.Code)
	}
}

func TestLogin_ForgedXForwardedForFromPublicOriginDoesNotEvadeIPLimit(t *testing.T) {
	mux, _ := newLoginTestMux(t)
	for index := 0; index < 20; index++ {
		body, _ := json.Marshal(map[string]string{"email": "v" + string(rune('a'+index)) + "@omni.test", "password": "x"})
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/login", strings.NewReader(string(body)))
		req.RemoteAddr = "203.0.113.200:1"
		req.Header.Set("X-Forwarded-For", "10.9.9."+string(rune('a'+index)))
		mux.ServeHTTP(httptest.NewRecorder(), req)
	}
	body, _ := json.Marshal(map[string]string{"email": "outro@omni.test", "password": "x"})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/login", strings.NewReader(string(body)))
	req.RemoteAddr = "203.0.113.200:1"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("XFF forjado nao pode escapar do limite por IP; got %d", rr.Code)
	}
}

func TestLogin_UnknownEmailStillRunsPasswordComparator(t *testing.T) {
	mux, hasher := newLoginTestMux(t)
	rr := doLogin(mux, "203.0.113.1:1", "inexistente@omni.test", "qualquer")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401; got %d", rr.Code)
	}
	if hasher.verifyCalls.Load() != 1 {
		t.Errorf("usuario inexistente deve chamar o comparador de hash (igualar tempo); chamadas=%d", hasher.verifyCalls.Load())
	}
}

func TestRateLimitIdentity_OnlyValidSignedTokens(t *testing.T) {
	tokens := NewHMACTokenManager("segredo", time.Hour)
	service := NewService(nil, nil, tokens, nil, nil, nil, nil)
	session, err := tokens.Issue("", User{ID: "u1", Email: "a@b.c", DisplayName: "A", Role: RoleDirector, TenantID: "t1", Active: true})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+session.AccessToken)
	if user, tenant := service.RateLimitIdentity(req); user != "u1" || tenant != "t1" {
		t.Errorf("token valido: user=%q tenant=%q", user, tenant)
	}

	forged := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	forged.Header.Set("Authorization", "Bearer lixo-aleatorio")
	forged.Header.Set("X-Account-Id", "vitima")
	if user, tenant := service.RateLimitIdentity(forged); user != "" || tenant != "" {
		t.Errorf("token invalido nao pode gerar identidade: %q %q", user, tenant)
	}
}
