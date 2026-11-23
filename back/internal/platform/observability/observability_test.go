package observability

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSanitizeTextMasksSensitiveData(t *testing.T) {
	input := `login maria.silva@example.com cpf 123.456.789-09 cnpj 12.345.678/0001-90 card 4111 1111 1111 1111 Authorization: Bearer abcdefghijklmnop password=hunter22 eyJhbGciOiJI.eyJzdWIiOiIx.c2lnbmF0dXJl`
	out := SanitizeText(input, 0)
	for _, leaked := range []string{"maria.silva", "123.456.789-09", "12.345.678/0001-90", "4111 1111", "abcdefghijklmnop", "hunter22", "eyJhbGciOiJI"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("sanitized text still contains %q: %s", leaked, out)
		}
	}
}

func TestSanitizeValueMasksSensitiveKeys(t *testing.T) {
	out := SanitizeValue(map[string]any{"password": "x", "nested": map[string]any{"token": "y", "ok": "fine"}}, 0).(map[string]any)
	if out["password"] != masked {
		t.Fatalf("password not masked: %v", out["password"])
	}
	nested := out["nested"].(map[string]any)
	if nested["token"] != masked || nested["ok"] != "fine" {
		t.Fatalf("unexpected nested: %v", nested)
	}
}

func TestNormalizeSQLStripsLiteralsAndGroupsLists(t *testing.T) {
	a := normalizeSQL("SELECT * FROM finance.x WHERE id = $1 AND name = 'joao' AND n IN ($2, $3, $4) LIMIT 50", 2000)
	b := normalizeSQL("select *  from finance.x where id = $1 and name = 'ana' and n in ($2) limit 10", 2000)
	if a.Fingerprint != b.Fingerprint {
		t.Fatalf("fingerprints differ:\n%s\n%s", a.Statement, b.Statement)
	}
	if strings.Contains(a.Statement, "joao") || strings.Contains(a.Statement, "50") {
		t.Fatalf("literal leaked: %s", a.Statement)
	}
	if a.Verb != "select" {
		t.Fatalf("verb = %q", a.Verb)
	}
}

func TestPercentileFromBuckets(t *testing.T) {
	buckets := make([]int64, len(LatencyBucketsMS)+1)
	buckets[bucketIndex(8)] = 90   // 5–10 ms
	buckets[bucketIndex(400)] = 10 // 300–500 ms
	if p50 := PercentileFromBuckets(buckets, 0.5); p50 < 5 || p50 > 10 {
		t.Fatalf("p50 = %v", p50)
	}
	if p95 := PercentileFromBuckets(buckets, 0.95); p95 < 300 || p95 > 500 {
		t.Fatalf("p95 = %v", p95)
	}
	if PercentileFromBuckets(make([]int64, 3), 0.5) != 0 {
		t.Fatal("empty buckets must be 0")
	}
}

func TestNormalizePathReplacesIDs(t *testing.T) {
	got := NormalizePath("/v1/finance/receipts/123/files/550e8400-e29b-41d4-a716-446655440000?token=abc")
	if got != "/v1/finance/receipts/:id/files/:id" {
		t.Fatalf("got %q", got)
	}
}

func newTestRuntime() *Runtime {
	cfg := ConfigFromEnv("test")
	cfg.Enabled = true
	cfg.NPlusOneThreshold = 3
	cfg.SampleRate = 1
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func drain(rt *Runtime) []any {
	var events []any
	for {
		select {
		case event := <-rt.recorder.events:
			events = append(events, event)
		default:
			return events
		}
	}
}

func TestMiddlewareRecordsRouteUserErrorAndNPlusOne(t *testing.T) {
	rt := newTestRuntime()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		SetPrincipal(r.Context(), "user-1", "acc-1")
		state := stateFrom(r.Context())
		for range 4 {
			state.addQuery("fp1", "select ? from x", time.Millisecond)
		}
		NoteError(r.Context(), "boom", "falhou para joao@example.com")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("x"))
	})
	handler := rt.Middleware(mux)(mux)
	request := httptest.NewRequest(http.MethodGet, "/v1/items/42", nil)
	recorder := httptest.NewRecorder()
	recorder.Header().Set("X-Request-ID", "abc12345-1")
	handler.ServeHTTP(recorder, request)

	var request1 *httpEvent
	var nPlusOne *nPlusOneEvent
	for _, event := range drain(rt) {
		switch typed := event.(type) {
		case httpEvent:
			request1 = &typed
		case nPlusOneEvent:
			nPlusOne = &typed
		}
	}
	if request1 == nil || request1.Route != "GET /v1/items/{id}" || request1.Status != 500 || request1.UserID != "user-1" || request1.QueryCount != 4 || request1.CorrelationID != "abc12345-1" {
		t.Fatalf("unexpected request event: %+v", request1)
	}
	if strings.Contains(request1.ErrorMessage, "joao@") {
		t.Fatalf("error message not sanitized: %s", request1.ErrorMessage)
	}
	if nPlusOne == nil || nPlusOne.Repetitions != 4 {
		t.Fatalf("expected N+1 event, got %+v", nPlusOne)
	}
	routes, _ := rt.recorder.takeAggregates()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route aggregate, got %d", len(routes))
	}
}

func TestDisabledRuntimeIsNoop(t *testing.T) {
	var rt *Runtime
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := rt.Middleware(nil)(next)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusNoContent || rt.QueryTracer() != nil || rt.Enabled() {
		t.Fatal("nil runtime must be a no-op")
	}
	rt.Start(nil)
	rt.Stop(context.Background())
	if _, err := rt.RecordFrontend(FrontendBatch{}, "", ""); err != nil {
		t.Fatal(err)
	}
}

func TestRecordFrontendValidatesAndSanitizes(t *testing.T) {
	rt := newTestRuntime()
	value := 1234.0
	batch := FrontendBatch{PageViewID: "0123456789abcdef", Page: "/manage/users?email=a@b.com", Events: []FrontendEvent{
		{Kind: "vital", Name: "LCP", Value: &value, At: time.Now().UTC().Format(time.RFC3339Nano)},
		{Kind: "js_error", Name: "TypeError at maria@example.com", Detail: map[string]any{"token": "secret"}},
		{Kind: "unknown", Name: "x"},
	}}
	accepted, err := rt.RecordFrontend(batch, "u1", "agent")
	if err != nil || accepted != 2 {
		t.Fatalf("accepted=%d err=%v", accepted, err)
	}
	for _, event := range drain(rt) {
		fe := event.(frontendEvent)
		if fe.Page != "/manage/users" || strings.Contains(fe.Name, "maria@") {
			t.Fatalf("not sanitized: %+v", fe)
		}
		if fe.Detail != nil && fe.Detail["token"] != masked {
			t.Fatalf("detail token not masked: %v", fe.Detail)
		}
	}
	if _, err := rt.RecordFrontend(FrontendBatch{PageViewID: "bad id!", Page: "/"}, "", ""); err == nil {
		t.Fatal("expected invalid batch error")
	}
}

func TestQueueFullDropsWithoutBlocking(t *testing.T) {
	rt := newTestRuntime()
	for range rt.cfg.BufferSize + 10 {
		rt.recorder.enqueue(slowQueryEvent{})
	}
	if rt.recorder.dropped.Load() != 10 {
		t.Fatalf("dropped = %d", rt.recorder.dropped.Load())
	}
}

func TestSanitizeKeepsNonCardNumbersAndIDs(t *testing.T) {
	if out := SanitizeText("run obsrun-20261123-050458-1 order 1234567890123", 0); !strings.Contains(out, "20261123-050458") || !strings.Contains(out, "1234567890123") {
		t.Fatalf("non-card numbers masked: %s", out)
	}
	attr := sanitizeAttr(slog.String("request_id", "obsrun-20261123-050458-1"))
	if attr.Value.String() != "obsrun-20261123-050458-1" {
		t.Fatalf("request_id altered: %s", attr.Value.String())
	}
	if sanitizeAttr(slog.String("access_token", "abc")).Value.String() != masked {
		t.Fatal("token key must be masked")
	}
}
