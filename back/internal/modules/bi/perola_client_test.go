package bi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPerolaClientLoginSendsRequiredHeadersAndCachesToken(t *testing.T) {
	var loginCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessoes" {
			t.Errorf("expected /sessoes, got %s", r.URL.Path)
		}
		if got := r.Header.Get("dsCompanyKey"); got != "company-key" {
			t.Errorf("expected company header, got %q", got)
		}
		if got := r.Header.Get("dsCnpjEmpresa"); got != "12345678000199" {
			t.Errorf("expected normalized CNPJ header, got %q", got)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode login body: %v", err)
		}
		if body["login"] != "user" || body["pass"] != "secret" {
			t.Errorf("unexpected login body")
		}

		loginCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"aaa.bbb.ccc"}`))
	}))
	t.Cleanup(server.Close)

	client := newTestPerolaClient(server.URL, 500*time.Millisecond)
	first, err := client.EnsureToken(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.EnsureToken(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}

	if first != "aaa.bbb.ccc" || second != first {
		t.Fatalf("expected cached token, got first=%q second=%q", first, second)
	}
	if calls := loginCalls.Load(); calls != 1 {
		t.Fatalf("expected one login, got %d", calls)
	}
}

func TestPerolaClientCoalescesConcurrentLogin(t *testing.T) {
	var loginCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		loginCalls.Add(1)
		time.Sleep(20 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"aaa.bbb.ccc"}`))
	}))
	t.Cleanup(server.Close)

	client := newTestPerolaClient(server.URL, time.Second)
	const callers = 24
	results := make(chan string, callers)
	errorsFound := make(chan error, callers)
	var waitGroup sync.WaitGroup
	waitGroup.Add(callers)

	for range callers {
		go func() {
			defer waitGroup.Done()
			token, err := client.EnsureToken(context.Background(), false)
			if err != nil {
				errorsFound <- err
				return
			}
			results <- token
		}()
	}

	waitGroup.Wait()
	close(results)
	close(errorsFound)

	for err := range errorsFound {
		t.Errorf("unexpected login error: %v", err)
	}
	for token := range results {
		if token != "aaa.bbb.ccc" {
			t.Errorf("unexpected token %q", token)
		}
	}
	if calls := loginCalls.Load(); calls != 1 {
		t.Fatalf("expected one coalesced login, got %d", calls)
	}
}

func TestPerolaClientRefreshesExpiredCachedToken(t *testing.T) {
	var loginCalls atomic.Int32
	currentTime := time.Date(2026, time.July, 23, 3, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := loginCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			_, _ = w.Write([]byte(`{"token":"old.token.value"}`))
			return
		}
		_, _ = w.Write([]byte(`{"token":"new.token.value"}`))
	}))
	t.Cleanup(server.Close)

	client := newTestPerolaClient(server.URL, time.Second)
	client.tokenTTL = time.Minute
	client.now = func() time.Time {
		return currentTime
	}

	first, err := client.EnsureToken(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	currentTime = currentTime.Add(2 * time.Minute)
	second, err := client.EnsureToken(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}

	if first != "old.token.value" || second != "new.token.value" {
		t.Fatalf("expected token refresh after expiry, got first=%q second=%q", first, second)
	}
	if calls := loginCalls.Load(); calls != 2 {
		t.Fatalf("expected two logins across expiry, got %d", calls)
	}
}

func TestPerolaClientRefreshesOnceAfterUnauthorized(t *testing.T) {
	var loginCalls atomic.Int32
	var findCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/sessoes":
			call := loginCalls.Add(1)
			if call == 1 {
				_, _ = w.Write([]byte(`{"token":"old.token.value"}`))
				return
			}
			_, _ = w.Write([]byte(`{"token":"new.token.value"}`))
		case "/item/find":
			findCalls.Add(1)
			if r.Header.Get("Authorization") == "Bearer old.token.value" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"expired"}`))
				return
			}
			_, _ = w.Write([]byte(`{"paginacao":{"totalRegistros":1},"registros":[{"id":1}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client := newTestPerolaClient(server.URL, time.Second)
	response, err := client.Find(context.Background(), "/item/find", []byte(`{"limit":1}`))
	if err != nil {
		t.Fatal(err)
	}

	if !response.OK || response.UpstreamStatus != http.StatusOK {
		t.Fatalf("expected successful retry, got status=%d ok=%v", response.UpstreamStatus, response.OK)
	}
	if calls := loginCalls.Load(); calls != 2 {
		t.Fatalf("expected initial login plus one refresh, got %d", calls)
	}
	if calls := findCalls.Load(); calls != 2 {
		t.Fatalf("expected one request plus one retry, got %d", calls)
	}
}

func TestPerolaClientRetriesUnauthorizedAtMostOnce(t *testing.T) {
	var loginCalls atomic.Int32
	var findCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/sessoes" {
			loginCalls.Add(1)
			_, _ = w.Write([]byte(`{"token":"aaa.bbb.ccc"}`))
			return
		}

		findCalls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	t.Cleanup(server.Close)

	client := newTestPerolaClient(server.URL, time.Second)
	response, err := client.Find(context.Background(), "/item/find", []byte(`{"limit":1}`))
	if err != nil {
		t.Fatal(err)
	}

	if response.UpstreamStatus != http.StatusUnauthorized {
		t.Fatalf("expected final 401, got %d", response.UpstreamStatus)
	}
	if calls := loginCalls.Load(); calls != 2 {
		t.Fatalf("expected exactly two logins, got %d", calls)
	}
	if calls := findCalls.Load(); calls != 2 {
		t.Fatalf("expected exactly two find attempts, got %d", calls)
	}
}

func TestPerolaClientHonorsRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(`{"token":"aaa.bbb.ccc"}`))
	}))
	t.Cleanup(server.Close)

	client := newTestPerolaClient(server.URL, 20*time.Millisecond)
	startedAt := time.Now()
	_, err := client.EnsureToken(context.Background(), false)

	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("expected upstream timeout, got %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 90*time.Millisecond {
		t.Fatalf("request timeout was not applied, elapsed=%s", elapsed)
	}
}

func TestPerolaClientHonorsCallerCancellation(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		select {
		case <-r.Context().Done():
		case <-releaseHandler:
		}
	}))
	t.Cleanup(server.Close)

	client := newTestPerolaClient(server.URL, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := client.EnsureToken(ctx, false)
		result <- err
	}()

	<-requestStarted
	cancel()

	select {
	case err := <-result:
		close(releaseHandler)
		if !errors.Is(err, ErrUpstream) {
			t.Fatalf("expected cancellation wrapped as upstream error, got %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		close(releaseHandler)
		t.Fatal("client did not honor caller cancellation")
	}
}

func newTestPerolaClient(baseURL string, requestTimeout time.Duration) *PerolaClient {
	return newPerolaClient(perolaClientOptions{
		BaseURL: baseURL,
		Credentials: perolaCredentials{
			CompanyKey:  "company-key",
			CNPJEmpresa: "12.345.678/0001-99",
			Login:       "user",
			Pass:        "secret",
		},
		TokenTTL:       time.Minute,
		RequestTimeout: requestTimeout,
	})
}
