package observability

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

func baseMetrics() alertMetrics {
	return alertMetrics{Now: t0, LastAPIAt: ptr(t0.Add(-10 * time.Second))}
}

func findRule(conds []condition, rule, target string) *condition {
	for index := range conds {
		if conds[index].Rule == rule && conds[index].Target == target {
			return &conds[index]
		}
	}
	return nil
}

func sampleAt(offset time.Duration, mutate func(*apiMetrics)) apiSample {
	s := apiSample{At: t0.Add(-offset)}
	if mutate != nil {
		mutate(&s.Metrics)
	}
	return s
}

func TestRulesQuietWhenHealthy(t *testing.T) {
	m := baseMetrics()
	m.HTTPTotal5m, m.HTTP5xx5m = 100, 1
	m.APISamples = []apiSample{sampleAt(0, nil)}
	if got := evaluateRules(m); len(got) != 0 {
		t.Fatalf("esperava nenhum alerta, veio %+v", got)
	}
}

func TestAPI5xxRateSeverity(t *testing.T) {
	cases := []struct {
		total, errs int64
		want        string
	}{{19, 19, ""}, {100, 5, ""}, {100, 6, sevHigh}, {100, 21, sevCritical}}
	for _, tc := range cases {
		m := baseMetrics()
		m.HTTPTotal5m, m.HTTP5xx5m = tc.total, tc.errs
		c := findRule(evaluateRules(m), "api_5xx_rate", "")
		switch {
		case tc.want == "" && c != nil:
			t.Errorf("%d/%d: nao deveria alertar", tc.errs, tc.total)
		case tc.want != "" && (c == nil || c.Severity != tc.want):
			t.Errorf("%d/%d: esperava %s, veio %+v", tc.errs, tc.total, tc.want, c)
		}
	}
}

func TestRouteAndSlowRoute(t *testing.T) {
	m := baseMetrics()
	m.RouteErrors5m = []routeCount{{"GET /a", 5}, {"GET /b", 4}}
	m.SlowRoutes15m = []routeLatency{{"GET /c", 2500, 10}, {"GET /d", 2500, 9}, {"GET /e", 1999, 50}}
	conds := evaluateRules(m)
	if findRule(conds, "route_errors", "GET /a") == nil || findRule(conds, "route_errors", "GET /b") != nil {
		t.Error("route_errors deve exigir >= 5")
	}
	if findRule(conds, "slow_route", "GET /c") == nil || findRule(conds, "slow_route", "GET /d") != nil || findRule(conds, "slow_route", "GET /e") != nil {
		t.Error("slow_route exige p95 > 2000 e >= 10 req")
	}
}

func TestPoolSaturationRules(t *testing.T) {
	fire := func(samples ...apiSample) bool {
		m := baseMetrics()
		m.APISamples = samples
		return findRule(evaluateRules(m), "pool_saturation", "") != nil
	}
	calm := func(m *apiMetrics) {
		m.Pool.Utilization, m.Pool.EmptyAcquireDelta, m.Pool.AcquireCountDelta = 0.3, 5, 100
	}
	if fire(sampleAt(0, calm), sampleAt(15*time.Second, calm), sampleAt(30*time.Second, calm)) {
		t.Fatal("pool a 30% com emptyAcquire nao deve alertar")
	}
	hot := func(m *apiMetrics) { m.Pool.Utilization = 0.95 }
	if fire(sampleAt(0, hot), sampleAt(15*time.Second, hot)) {
		t.Fatal("duas amostras de utilizacao nao bastam")
	}
	if !fire(sampleAt(0, hot), sampleAt(15*time.Second, hot), sampleAt(30*time.Second, hot)) {
		t.Fatal("95% por 3 amostras deve alertar")
	}
	canceled := func(m *apiMetrics) { m.Pool.CanceledAcquireDelta = 1 }
	if fire(sampleAt(0, canceled), sampleAt(15*time.Second, calm)) {
		t.Fatal("cancelado em 1 amostra nao basta")
	}
	if !fire(sampleAt(0, canceled), sampleAt(15*time.Second, canceled)) {
		t.Fatal("cancelado em 2 amostras deve alertar")
	}
	slow := func(m *apiMetrics) { m.Pool.AcquireCountDelta, m.Pool.AcquireWaitMsDelta = 10, 600 }
	if fire(sampleAt(0, slow), sampleAt(15*time.Second, slow)) {
		t.Fatal("espera media precisa de 3 amostras")
	}
	if !fire(sampleAt(0, slow), sampleAt(15*time.Second, slow), sampleAt(30*time.Second, slow)) {
		t.Fatal("espera media 60 ms em 3 amostras deve alertar")
	}
}

func TestPostgresRules(t *testing.T) {
	m := baseMetrics()
	bad := func(x *apiMetrics) {
		x.PG.IdleInTransactionOver60s = 1
		x.PG.WaitingOnLock = 1
		x.PG.DeadlocksDelta = 1
		x.PG.Connections, x.PG.MaxConnections = 90, 100
	}
	m.APISamples = []apiSample{sampleAt(0, bad), sampleAt(15*time.Second, bad)}
	conds := evaluateRules(m)
	for _, rule := range []string{"pg_idle_in_tx", "pg_lock_wait", "pg_deadlock", "pg_connections"} {
		if findRule(conds, rule, "") == nil {
			t.Errorf("faltou %s", rule)
		}
	}
	m.APISamples = []apiSample{sampleAt(0, bad), sampleAt(15*time.Second, nil)}
	if findRule(evaluateRules(m), "pg_lock_wait", "") != nil {
		t.Error("pg_lock_wait exige 2 amostras seguidas")
	}
}

func TestGoroutineGrowth(t *testing.T) {
	g := func(n int64) func(*apiMetrics) { return func(m *apiMetrics) { m.Go.Goroutines = n } }
	m := baseMetrics()
	m.APISamples = []apiSample{sampleAt(0, g(10001))}
	if findRule(evaluateRules(m), "goroutine_growth", "") == nil {
		t.Fatal("> 10000 deve alertar")
	}
	m.APISamples = []apiSample{sampleAt(0, g(900)), sampleAt(15*time.Minute, g(700)), sampleAt(30*time.Minute, g(500))}
	if findRule(evaluateRules(m), "goroutine_growth", "") == nil {
		t.Fatal("crescimento > 50% sem queda deve alertar")
	}
	m.APISamples = []apiSample{sampleAt(0, g(900)), sampleAt(10*time.Minute, g(400)), sampleAt(20*time.Minute, g(800)), sampleAt(30*time.Minute, g(500))}
	if findRule(evaluateRules(m), "goroutine_growth", "") != nil {
		t.Fatal("queda no meio da janela descarta o vazamento")
	}
	m.APISamples = []apiSample{sampleAt(0, g(900)), sampleAt(5*time.Minute, g(500))}
	if findRule(evaluateRules(m), "goroutine_growth", "") != nil {
		t.Fatal("janela curta nao conclui")
	}
}

func TestJobRules(t *testing.T) {
	m := baseMetrics()
	m.APISamples = []apiSample{sampleAt(0, func(x *apiMetrics) {
		x.Jobs = []jobMetrics{
			{Table: "a.jobs", OldestDueLagSeconds: 400},
			{Table: "b.jobs", OldestDueLagSeconds: 2000, DeadLastHour: 2},
			{Table: "c.jobs", OldestDueLagSeconds: 300},
		}
	})}
	conds := evaluateRules(m)
	if c := findRule(conds, "job_backlog", "a.jobs"); c == nil || c.Severity != sevMedium {
		t.Error("a.jobs: medium")
	}
	if c := findRule(conds, "job_backlog", "b.jobs"); c == nil || c.Severity != sevHigh {
		t.Error("b.jobs: high")
	}
	if findRule(conds, "job_backlog", "c.jobs") != nil || findRule(conds, "job_dead", "b.jobs") == nil {
		t.Error("limite e job_dead")
	}
}

func TestSilenceRules(t *testing.T) {
	m := baseMetrics()
	m.LastAPIAt = ptr(t0.Add(-3 * time.Minute))
	if findRule(evaluateRules(m), "collector_silent", "") == nil {
		t.Error("collector_silent")
	}
	m = baseMetrics()
	if findRule(evaluateRules(m), "agent_silent", "") != nil {
		t.Error("agente nunca visto nao alerta")
	}
	m.LastAgentAt = ptr(t0.Add(-4 * time.Minute))
	if findRule(evaluateRules(m), "agent_silent", "") == nil {
		t.Error("agent_silent")
	}
}

func TestContainerAndDiskRules(t *testing.T) {
	m := baseMetrics()
	m.LastAgentAt = ptr(t0)
	m.Containers = []containerState{
		{Container: "web", State: "running", Health: "healthy", RestartPolicy: "always"},
		{Container: "api", State: "running", Health: "unhealthy"},
		{Container: "db", State: "restarting", RestartPolicy: "unless-stopped"},
		{Container: "dead", State: "dead", RestartPolicy: "always"},
		{Container: "stopped", State: "exited", RestartPolicy: "unless-stopped", ExitCode: iptr(0)},
		{Container: "staging", State: "exited", RestartPolicy: "always", ExitCode: iptr(137)},
		{Container: "old", State: "exited", RestartPolicy: "no"},
		{Container: "old2", State: "exited"},
		{Container: "oc", State: "exited", RestartPolicy: "on-failure", ExitCode: iptr(1)},
		{Container: "ocok", State: "exited", RestartPolicy: "on-failure", ExitCode: iptr(0)},
		{Container: "w", State: "running", RestartDelta: 2},
	}
	m.Disks = []diskMetric{{Path: "/", UsedPct: 96}, {Path: "/data", UsedPct: 86}, {Path: "/x", UsedPct: 50}}
	conds := evaluateRules(m)
	if findRule(conds, "container_down", "web") != nil || findRule(conds, "container_down", "api") == nil || findRule(conds, "container_down", "db") == nil {
		t.Error("container_down")
	}
	if findRule(conds, "container_down", "dead") == nil {
		t.Error("container dead alerta")
	}
	// Parada manual (docker stop / stack desligada): always/unless-stopped exited
	// nao reinicia sozinho, entao foi de proposito.
	for _, name := range []string{"old", "old2", "stopped", "staging", "ocok"} {
		if findRule(conds, "container_down", name) != nil {
			t.Errorf("%s parado de proposito nao alerta", name)
		}
	}
	if findRule(conds, "container_down", "oc") == nil {
		t.Error("on-failure saiu com erro alerta")
	}
	if findRule(conds, "container_restarts", "w") == nil {
		t.Error("container_restarts")
	}
	if c := findRule(conds, "disk_low", "/"); c == nil || c.Severity != sevCritical {
		t.Error("disco >= 95 critical")
	}
	if c := findRule(conds, "disk_low", "/data"); c == nil || c.Severity != sevHigh {
		t.Error("disco >= 85 high")
	}
	if findRule(conds, "disk_low", "/x") != nil {
		t.Error("disco 50% nao alerta")
	}
	// Agente parado: containers antigos não geram container_down.
	m.LastAgentAt = ptr(t0.Add(-10 * time.Minute))
	if findRule(evaluateRules(m), "container_down", "api") != nil {
		t.Error("sem agente vivo nao avalia containers")
	}
}

func TestFrontendErrors(t *testing.T) {
	m := baseMetrics()
	m.FrontendJSErr15 = 19
	if findRule(evaluateRules(m), "frontend_errors", "") != nil {
		t.Error("19 nao alerta")
	}
	m.FrontendJSErr15 = 20
	if findRule(evaluateRules(m), "frontend_errors", "") == nil {
		t.Error("20 alerta")
	}
}

func TestOrderedBySeverityAndFingerprint(t *testing.T) {
	m := baseMetrics()
	m.FrontendJSErr15 = 30
	m.HTTPTotal5m, m.HTTP5xx5m = 100, 50
	conds := evaluateRules(m)
	if len(conds) != 2 || conds[0].Severity != sevCritical {
		t.Fatalf("ordem: %+v", conds)
	}
	if got := (condition{Rule: "disk_low", Target: "/"}).Fingerprint(); got != "disk_low:/" {
		t.Errorf("fingerprint %q", got)
	}
}

func iptr(v int) *int { return &v }
