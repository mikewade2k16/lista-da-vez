package observability

import (
	"strings"
	"testing"
	"time"
)

func TestCounterDelta(t *testing.T) {
	if counterDelta(10, 4, true) != 6 || counterDelta(10, 4, false) != 0 || counterDelta(3, 9, true) != 0 {
		t.Fatal("delta incorreto")
	}
}

func TestPoolMetricsUtilizationAndDelta(t *testing.T) {
	prev := poolCounters{acquireCount: 10, emptyAcquire: 1, acquireDuration: time.Second}
	cur := poolCounters{max: 10, acquired: 5, acquireCount: 25, emptyAcquire: 3, acquireDuration: 3 * time.Second}
	m := buildPoolMetrics(cur, prev, true)
	if m.Utilization != 0.5 || m.AcquireCountDelta != 15 || m.EmptyAcquireDelta != 2 || m.AcquireWaitMsDelta != 2000 {
		t.Fatalf("metricas inesperadas: %+v", m)
	}
	if first := buildPoolMetrics(cur, poolCounters{}, false); first.AcquireCountDelta != 0 {
		t.Fatal("primeira amostra deve ter delta 0")
	}
	if utilization(1, 0) != 0 {
		t.Fatal("max 0 nao pode dividir")
	}
}

func TestGCPauseP99(t *testing.T) {
	pauses := make([]uint64, 256)
	for i := range 100 {
		pauses[i] = uint64(i+1) * uint64(time.Millisecond)
	}
	if got := gcPauseP99(pauses, 100); got != 100 {
		t.Fatalf("p99 = %v", got)
	}
	if gcPauseP99(pauses, 0) != 0 {
		t.Fatal("sem GC deve ser 0")
	}
}

func TestBuildJobSQL(t *testing.T) {
	sql, err := buildJobSQL(jobTable{schema: "messaging", name: "outbox", hasLockedAt: true, hasUpdatedAt: true})
	if err != nil || !strings.Contains(sql, `"messaging"."outbox"`) || !strings.Contains(sql, "locked_at") {
		t.Fatalf("sql: %v %s", err, sql)
	}
	plain, _ := buildJobSQL(jobTable{schema: "a", name: "b"})
	if strings.Contains(plain, "locked_at") || strings.Contains(plain, "updated_at") {
		t.Fatal("colunas ausentes nao devem aparecer")
	}
	for _, bad := range []jobTable{{schema: `a"b`, name: "x"}, {schema: "a", name: "x; drop"}, {schema: "A", name: "x"}} {
		if _, err := buildJobSQL(bad); err == nil {
			t.Fatalf("deveria rejeitar %+v", bad)
		}
	}
}

func TestParseHeartbeat(t *testing.T) {
	now := time.Now()
	event, ok := parseHeartbeat("pv-12345678", "/home", FrontendEvent{Kind: "heartbeat", Name: "/clientes/123456/editar?x=1"}, "u1", "UA", now)
	if !ok || event.Page != "/clientes/:id/editar" || event.UserID != "u1" {
		t.Fatalf("evento: %+v ok=%v", event, ok)
	}
	fallback, _ := parseHeartbeat("pv-12345678", "/home", FrontendEvent{Name: "sem barra"}, "u1", "", now)
	if fallback.Page != "/home" {
		t.Fatalf("fallback: %s", fallback.Page)
	}
	if _, ok := parseHeartbeat("pv-12345678", "/home", FrontendEvent{}, "", "", now); ok {
		t.Fatal("sem usuario nao grava presenca")
	}
}

func TestHeartbeatNotStoredAsFrontendEvent(t *testing.T) {
	rt := New(Config{Enabled: true, BufferSize: 100}, nil)
	n, err := rt.RecordFrontend(FrontendBatch{PageViewID: "pv-12345678", Page: "/home", Events: []FrontendEvent{{Kind: "heartbeat", Name: "/home"}}}, "u1", "UA")
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	batch := newEventBatch()
	batch.add(<-rt.recorder.events)
	if len(batch.frontend) != 0 || len(batch.presence) != 1 {
		t.Fatalf("batch: %+v", batch)
	}
}
