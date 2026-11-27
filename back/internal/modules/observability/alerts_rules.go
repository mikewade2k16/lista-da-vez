package observability

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	sevCritical = "critical"
	sevHigh     = "high"
	sevMedium   = "medium"
	sevLow      = "low"
)

func severityRank(severity string) int {
	switch severity {
	case sevCritical:
		return 4
	case sevHigh:
		return 3
	case sevMedium:
		return 2
	case sevLow:
		return 1
	}
	return 0
}

// apiSample é uma linha source='api' de obs.system_samples (formato do SPEC).
type apiSample struct {
	At      time.Time
	Metrics apiMetrics
}

type apiMetrics struct {
	Go struct {
		Goroutines int64 `json:"goroutines"`
		HeapAlloc  int64 `json:"heapAllocBytes"`
	} `json:"go"`
	Pool struct {
		Max                  int64   `json:"max"`
		AcquireCountDelta    int64   `json:"acquireCountDelta"`
		EmptyAcquireDelta    int64   `json:"emptyAcquireDelta"`
		CanceledAcquireDelta int64   `json:"canceledAcquireDelta"`
		AcquireWaitMsDelta   float64 `json:"acquireWaitMsDelta"`
		Utilization          float64 `json:"utilization"`
	} `json:"pool"`
	PG struct {
		Connections              int64   `json:"connections"`
		MaxConnections           int64   `json:"maxConnections"`
		IdleInTransactionOver60s int64   `json:"idleInTransactionOver60s"`
		WaitingOnLock            int64   `json:"waitingOnLock"`
		DeadlocksDelta           int64   `json:"deadlocksDelta"`
		LongestQuerySeconds      float64 `json:"longestQuerySeconds"`
	} `json:"pg"`
	Jobs   []jobMetrics `json:"jobs"`
	Panics struct {
		Total int64 `json:"total"`
		Delta int64 `json:"delta"`
	} `json:"panics"`
}

type jobMetrics struct {
	Table               string `json:"table"`
	Pending             int64  `json:"pending"`
	Due                 int64  `json:"due"`
	OldestDueLagSeconds int64  `json:"oldestDueLagSeconds"`
	StuckProcessing     int64  `json:"stuckProcessing"`
	Dead                int64  `json:"dead"`
	DeadLastHour        int64  `json:"deadLastHour"`
}

type diskMetric struct {
	Path       string  `json:"path"`
	TotalBytes int64   `json:"totalBytes"`
	FreeBytes  int64   `json:"freeBytes"`
	UsedPct    float64 `json:"usedPct"`
}

type containerState struct {
	Container    string
	Service      string
	State        string
	Health       string
	RestartCount int
	RestartDelta int // aumento do restart_count na última hora
	At           time.Time
	CPUPct       *float64
	MemBytes     *int64
	MemLimit     *int64

	RestartPolicy string // no|always|unless-stopped|on-failure ('' = desconhecida)
	ExitCode      *int
}

// ShouldBeUp: o container deveria estar no ar (parar é anomalia). Só vale para
// restart policy always|unless-stopped|on-failure, ou para quem está unhealthy.
// ShouldBeUp: o Docker mantém no ar. Com always/unless-stopped uma queda vira
// "restarting"; ficar "exited"/"created" significa parada manual (docker stop,
// stack desligada de propósito) e não conta como esperado no ar.
func (c containerState) ShouldBeUp() bool {
	if c.Health == "unhealthy" {
		return true
	}
	switch c.RestartPolicy {
	case "always", "unless-stopped":
		return c.State == "running" || c.State == "restarting" || c.State == "dead"
	case "on-failure":
		return c.State == "running" || c.State == "restarting" || c.State == "dead" ||
			(c.State == "exited" && c.ExitCode != nil && *c.ExitCode != 0)
	}
	return false
}

// IsDown: container esperado no ar que está em loop de restart, morto, saiu com
// erro (on-failure) ou unhealthy. Parada manual não alerta.
func (c containerState) IsDown() bool {
	return c.ShouldBeUp() && (c.Health == "unhealthy" || c.State != "running")
}

type routeCount struct {
	Route string
	Count int64
}

type routeLatency struct {
	Route string
	P95Ms float64
	Calls int64
}

// alertMetrics reúne tudo que as regras consomem; é a fronteira entre o SQL e a
// lógica pura (testável sem banco).
type alertMetrics struct {
	Now time.Time

	HTTPTotal5m   int64
	HTTP5xx5m     int64
	RouteErrors5m []routeCount
	SlowRoutes15m []routeLatency

	APISamples      []apiSample // mais recente primeiro, janela de 30 min
	LastAPIAt       *time.Time
	LastAgentAt     *time.Time
	Containers      []containerState
	Disks           []diskMetric
	FrontendJSErr15 int64

	ContractErrors15m []routeContractErrors // 400/422 com invalid_body/invalid_json, por rota
	Route4xx15m       []routeClientErrors   // 400/422 e total de chamadas, por rota
	LastDeploy        *deployWindow         // último deploy com sucesso nas últimas 2 h

	TLS           []tlsMetric          // última amostra do agente (vazio = coletor sem TLS)
	TLSErrorSince map[string]time.Time // host -> primeira amostra com erro na janela de 13 h

	// Thresholds são os limites personalizados no painel (regra -> valor); ausente = padrão do código.
	Thresholds map[string]float64
}

// threshold devolve o limite da regra: o do painel ou, na falta, o padrão do código.
func (m alertMetrics) threshold(rule string) float64 {
	if value, ok := m.Thresholds[rule]; ok {
		return value
	}
	return defaultThreshold(rule)
}

// condition é uma condição de alerta ativa. Fingerprint = regra + alvo.
type condition struct {
	Rule     string
	Target   string
	Severity string
	Area     string
	Title    string
	Detail   map[string]any
	Measured string
}

func (c condition) Fingerprint() string {
	if c.Target == "" {
		return c.Rule
	}
	return c.Rule + ":" + c.Target
}

// evaluateRules aplica a tabela do SPEC às métricas. Função pura.
func evaluateRules(m alertMetrics) []condition {
	var out []condition
	out = append(out, ruleHTTP(m)...)
	out = append(out, rulePoolAndPG(m)...)
	out = append(out, ruleGoroutines(m)...)
	out = append(out, ruleJobs(m)...)
	out = append(out, ruleSilence(m)...)
	out = append(out, ruleContainers(m)...)
	out = append(out, ruleTLS(m)...)
	out = append(out, rulePanics(m)...)
	out = append(out, ruleContract(m)...)
	out = append(out, ruleDeployRegression(m)...)
	if m.FrontendJSErr15 >= int64(m.threshold("frontend_errors")) {
		out = append(out, condition{Rule: "frontend_errors", Severity: sevMedium, Area: "pages",
			Title:    "Erros de JavaScript no front",
			Measured: fmt.Sprintf("%d erros em 15 min", m.FrontendJSErr15),
			Detail:   map[string]any{"errors15m": m.FrontendJSErr15}})
	}
	sort.SliceStable(out, func(i, j int) bool { return severityRank(out[i].Severity) > severityRank(out[j].Severity) })
	return out
}

// evaluateRulesWith aplica os limites (m.Thresholds) e, depois, liga/desliga e severidade
// fixa de cada regra (configurações do painel).
func evaluateRulesWith(m alertMetrics, rules map[string]RuleSetting) []condition {
	m.Thresholds = ruleThresholds(rules)
	out := applyRuleSettings(evaluateRules(m), rules)
	sort.SliceStable(out, func(i, j int) bool { return severityRank(out[i].Severity) > severityRank(out[j].Severity) })
	return out
}

func ruleHTTP(m alertMetrics) []condition {
	var out []condition
	if m.HTTPTotal5m >= 20 {
		rate := float64(m.HTTP5xx5m) / float64(m.HTTPTotal5m)
		limit := m.threshold("api_5xx_rate")
		if rate > limit/100 {
			severity := sevHigh
			if rate > limit*4/100 {
				severity = sevCritical
			}
			out = append(out, condition{Rule: "api_5xx_rate", Severity: severity, Area: "api",
				Title:    "Taxa de erros 5xx da API alta",
				Measured: fmt.Sprintf("%.1f%% de 5xx (%d de %d requisições em 5 min)", rate*100, m.HTTP5xx5m, m.HTTPTotal5m),
				Detail:   map[string]any{"rate": rate, "errors5xx": m.HTTP5xx5m, "requests": m.HTTPTotal5m}})
		}
	}
	for _, item := range m.RouteErrors5m {
		if item.Count >= int64(m.threshold("route_errors")) {
			out = append(out, condition{Rule: "route_errors", Target: item.Route, Severity: sevMedium, Area: "api",
				Title:    "Rota com erros 5xx: " + item.Route,
				Measured: fmt.Sprintf("%d respostas 5xx em 5 min", item.Count),
				Detail:   map[string]any{"route": item.Route, "errors5xx": item.Count}})
		}
	}
	for _, item := range m.SlowRoutes15m {
		if item.Calls >= 10 && item.P95Ms > m.threshold("slow_route") {
			out = append(out, condition{Rule: "slow_route", Target: item.Route, Severity: sevMedium, Area: "api",
				Title:    "Rota lenta: " + item.Route,
				Measured: fmt.Sprintf("p95 de %.0f ms em %d requisições (15 min)", item.P95Ms, item.Calls),
				Detail:   map[string]any{"route": item.Route, "p95Ms": item.P95Ms, "calls": item.Calls}})
		}
	}
	return out
}

// firstN devolve as n amostras mais recentes (ou nil se houver menos que n).
func firstN(samples []apiSample, n int) []apiSample {
	if len(samples) < n {
		return nil
	}
	return samples[:n]
}

func rulePoolAndPG(m alertMetrics) []condition {
	var out []condition
	if c := poolSaturation(m.APISamples); c != nil {
		out = append(out, *c)
	}
	if len(m.APISamples) == 0 {
		return out
	}
	pg := m.APISamples[0].Metrics.PG
	if pg.IdleInTransactionOver60s > 0 {
		out = append(out, condition{Rule: "pg_idle_in_tx", Severity: sevMedium, Area: "database",
			Title:    "Transações paradas há mais de 60 s",
			Measured: fmt.Sprintf("%d conexões idle in transaction", pg.IdleInTransactionOver60s),
			Detail:   map[string]any{"idleInTransactionOver60s": pg.IdleInTransactionOver60s}})
	}
	if recent := firstN(m.APISamples, 2); recent != nil && recent[0].Metrics.PG.WaitingOnLock > 0 && recent[1].Metrics.PG.WaitingOnLock > 0 {
		out = append(out, condition{Rule: "pg_lock_wait", Severity: sevMedium, Area: "database",
			Title:    "Consultas esperando lock",
			Measured: fmt.Sprintf("%d consultas aguardando lock", pg.WaitingOnLock),
			Detail:   map[string]any{"waitingOnLock": pg.WaitingOnLock}})
	}
	if pg.DeadlocksDelta > 0 {
		out = append(out, condition{Rule: "pg_deadlock", Severity: sevHigh, Area: "database",
			Title:    "Deadlock no Postgres",
			Measured: fmt.Sprintf("%d deadlocks desde a última amostra", pg.DeadlocksDelta),
			Detail:   map[string]any{"deadlocksDelta": pg.DeadlocksDelta}})
	}
	if pg.MaxConnections > 0 && float64(pg.Connections)/float64(pg.MaxConnections) >= m.threshold("pg_connections")/100 {
		out = append(out, condition{Rule: "pg_connections", Severity: sevHigh, Area: "database",
			Title:    "Conexões do Postgres perto do limite",
			Measured: fmt.Sprintf("%d de %d conexões", pg.Connections, pg.MaxConnections),
			Detail:   map[string]any{"connections": pg.Connections, "maxConnections": pg.MaxConnections}})
	}
	return out
}

// goroutineGrowthMinDelta é o crescimento absoluto mínimo para suspeitar de vazamento.
const goroutineGrowthMinDelta = 300

// ruleGoroutines: > 10000, ou crescimento > 50% (e >= +300) em 30 min sem nenhuma queda
// relevante (queda > 20% sobre o pico anterior zera a suspeita de vazamento).
func ruleGoroutines(m alertMetrics) []condition {
	if len(m.APISamples) == 0 {
		return nil
	}
	latest := m.APISamples[0]
	current := latest.Metrics.Go.Goroutines
	if current > int64(m.threshold("goroutine_growth")) {
		return []condition{{Rule: "goroutine_growth", Severity: sevMedium, Area: "api",
			Title:    "Goroutines acima do limite",
			Measured: fmt.Sprintf("%d goroutines", current),
			Detail:   map[string]any{"goroutines": current}}}
	}
	oldest := m.APISamples[len(m.APISamples)-1]
	if latest.At.Sub(oldest.At) < 20*time.Minute || oldest.Metrics.Go.Goroutines <= 0 {
		return nil
	}
	peak := int64(0)
	for index := len(m.APISamples) - 1; index >= 0; index-- {
		value := m.APISamples[index].Metrics.Go.Goroutines
		if peak > 0 && float64(value) < float64(peak)*0.8 {
			return nil
		}
		peak = max(peak, value)
	}
	growth := float64(current)/float64(oldest.Metrics.Go.Goroutines) - 1
	// Piso absoluto: com poucas goroutines (ex.: 46 -> 75, alguns websockets abertos) os
	// 50% não indicam vazamento; só alerta com +300 ou mais além do percentual.
	if growth <= 0.5 || current-oldest.Metrics.Go.Goroutines < goroutineGrowthMinDelta {
		return nil
	}
	return []condition{{Rule: "goroutine_growth", Severity: sevMedium, Area: "api",
		Title:    "Goroutines crescendo sem queda",
		Measured: fmt.Sprintf("%d goroutines (+%.0f%% em %.0f min)", current, growth*100, latest.At.Sub(oldest.At).Minutes()),
		Detail:   map[string]any{"goroutines": current, "growth": growth}}}
}

func ruleJobs(m alertMetrics) []condition {
	if len(m.APISamples) == 0 {
		return nil
	}
	var out []condition
	for _, job := range m.APISamples[0].Metrics.Jobs {
		limit := m.threshold("job_backlog")
		if float64(job.OldestDueLagSeconds) > limit {
			severity := sevMedium
			if float64(job.OldestDueLagSeconds) > limit*6 {
				severity = sevHigh
			}
			out = append(out, condition{Rule: "job_backlog", Target: job.Table, Severity: severity, Area: "workers",
				Title:    "Fila com atraso: " + job.Table,
				Measured: fmt.Sprintf("job mais antigo atrasado %d s (%d vencidos)", job.OldestDueLagSeconds, job.Due),
				Detail:   map[string]any{"table": job.Table, "oldestDueLagSeconds": job.OldestDueLagSeconds, "due": job.Due}})
		}
		if job.DeadLastHour > 0 {
			out = append(out, condition{Rule: "job_dead", Target: job.Table, Severity: sevMedium, Area: "workers",
				Title:    "Jobs mortos: " + job.Table,
				Measured: fmt.Sprintf("%d jobs mortos na última hora", job.DeadLastHour),
				Detail:   map[string]any{"table": job.Table, "deadLastHour": job.DeadLastHour}})
		}
	}
	return out
}

func ruleSilence(m alertMetrics) []condition {
	var out []condition
	if m.LastAPIAt == nil || m.Now.Sub(*m.LastAPIAt) > 2*time.Minute {
		measured := "nenhuma amostra registrada"
		if m.LastAPIAt != nil {
			measured = fmt.Sprintf("última amostra há %.0f min", m.Now.Sub(*m.LastAPIAt).Minutes())
		}
		out = append(out, condition{Rule: "collector_silent", Severity: sevHigh, Area: "api",
			Title: "Coletor de métricas da API parado", Measured: measured, Detail: map[string]any{}})
	}
	if m.LastAgentAt != nil && m.Now.Sub(*m.LastAgentAt) > 3*time.Minute {
		out = append(out, condition{Rule: "agent_silent", Severity: sevMedium, Area: "containers",
			Title:    "Agente de containers parado",
			Measured: fmt.Sprintf("última amostra há %.0f min", m.Now.Sub(*m.LastAgentAt).Minutes()),
			Detail:   map[string]any{}})
	}
	return out
}

func ruleContainers(m alertMetrics) []condition {
	var out []condition
	agentAlive := m.LastAgentAt != nil && m.Now.Sub(*m.LastAgentAt) <= 3*time.Minute
	if agentAlive {
		for _, c := range m.Containers {
			if c.IsDown() {
				out = append(out, condition{Rule: "container_down", Target: c.Container, Severity: sevCritical, Area: "containers",
					Title:    "Container fora do ar: " + c.Container,
					Measured: fmt.Sprintf("estado %s, saúde %s", orDash(c.State), orDash(c.Health)),
					Detail:   map[string]any{"container": c.Container, "state": c.State, "health": c.Health, "restartPolicy": c.RestartPolicy}})
			}
			if c.RestartDelta > 0 {
				out = append(out, condition{Rule: "container_restarts", Target: c.Container, Severity: sevHigh, Area: "containers",
					Title:    "Container reiniciando: " + c.Container,
					Measured: fmt.Sprintf("%d reinício(s) na última hora", c.RestartDelta),
					Detail:   map[string]any{"container": c.Container, "restartDelta": c.RestartDelta}})
			}
		}
	}
	for _, disk := range m.Disks {
		limit := m.threshold("disk_low")
		if disk.UsedPct >= limit {
			severity := sevHigh
			if disk.UsedPct >= min(limit+10, 99) {
				severity = sevCritical
			}
			out = append(out, condition{Rule: "disk_low", Target: disk.Path, Severity: severity, Area: "containers",
				Title:    "Pouco espaço em disco: " + disk.Path,
				Measured: fmt.Sprintf("%.0f%% usado", disk.UsedPct),
				Detail:   map[string]any{"path": disk.Path, "usedPct": disk.UsedPct}})
		}
	}
	return out
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// poolWaitMs: espera média por conexão na amostra (ms); 0 sem acquires.
func poolWaitMs(p apiPool) float64 {
	if p.AcquireCountDelta <= 0 {
		return 0
	}
	return p.AcquireWaitMsDelta / float64(p.AcquireCountDelta)
}

type apiPool = struct {
	Max                  int64   `json:"max"`
	AcquireCountDelta    int64   `json:"acquireCountDelta"`
	EmptyAcquireDelta    int64   `json:"emptyAcquireDelta"`
	CanceledAcquireDelta int64   `json:"canceledAcquireDelta"`
	AcquireWaitMsDelta   float64 `json:"acquireWaitMsDelta"`
	Utilization          float64 `json:"utilization"`
}

// poolSaturation: utilização >= 0.9 em 3 amostras seguidas, OU acquires
// cancelados em 2 seguidas, OU espera média > 50 ms por conexão em 3 seguidas.
// emptyAcquireDelta não conta: só indica que um acquire criou conexão nova.
func poolSaturation(samples []apiSample) *condition {
	allOf := func(n int, pred func(apiPool) bool) bool {
		recent := firstN(samples, n)
		if recent == nil {
			return false
		}
		for _, s := range recent {
			if !pred(s.Metrics.Pool) {
				return false
			}
		}
		return true
	}
	util := allOf(3, func(p apiPool) bool { return p.Utilization >= 0.9 })
	canceled := allOf(2, func(p apiPool) bool { return p.CanceledAcquireDelta > 0 })
	wait := allOf(3, func(p apiPool) bool { return poolWaitMs(p) > 50 })
	if !util && !canceled && !wait {
		return nil
	}
	last := samples[0].Metrics.Pool
	var reasons []string
	if util {
		reasons = append(reasons, "utilização >= 90% em 3 amostras")
	}
	if canceled {
		reasons = append(reasons, "acquires cancelados em 2 amostras")
	}
	if wait {
		reasons = append(reasons, "espera média > 50 ms em 3 amostras")
	}
	return &condition{Rule: "pool_saturation", Severity: sevHigh, Area: "database",
		Title: "Pool de conexões saturado",
		Measured: fmt.Sprintf("utilização %.0f%% (%d conexões máx.), %d acquires cancelados, espera média %.0f ms; %s",
			last.Utilization*100, last.Max, last.CanceledAcquireDelta, poolWaitMs(last), strings.Join(reasons, "; ")),
		Detail: map[string]any{"utilization": last.Utilization, "canceledAcquireDelta": last.CanceledAcquireDelta,
			"acquireCountDelta": last.AcquireCountDelta, "avgWaitMs": poolWaitMs(last), "emptyAcquireDelta": last.EmptyAcquireDelta}}
}
