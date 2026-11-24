package observability

import (
	"context"
	"fmt"
	"sort"
	"time"

	platformobs "github.com/mikewade2k16/lista-da-vez/back/internal/platform/observability"
)

const (
	statusOK       = "ok"
	statusWarn     = "warn"
	statusCritical = "critical"
	statusUnknown  = "unknown"
	maxIssues      = 15
)

type healthArea struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

type issueLink struct {
	Tab   string         `json:"tab"`
	Query map[string]any `json:"query"`
}

type healthIssue struct {
	Severity string    `json:"severity"`
	Area     string    `json:"area"`
	Title    string    `json:"title"`
	Detail   string    `json:"detail"`
	Since    time.Time `json:"since"`
	Link     issueLink `json:"link"`
}

// healthInputs é tudo que o painel consulta; buildHealth é pura sobre isso.
type healthInputs struct {
	Now        time.Time
	HTTP       httpWindow
	Errors     errorCounts
	Top        []errorSignature
	Online     []onlineUser
	API        *apiSample
	Agent      *agentSample
	Containers []containerState
	Pages      pageStats
	Checks     []securityCheck
	Findings   []securityFinding
	Alerts     []alertRow  // abertos
	Deploys    []deployRow // mais recente primeiro
	Configured bool
	Recipients int
}

func worse(a, b string) string {
	rank := map[string]int{statusUnknown: 0, statusOK: 1, statusWarn: 2, statusCritical: 3}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

// Health monta o resumo do painel "Saúde do sistema".
func (s *Service) Health(ctx context.Context) (map[string]any, error) {
	in, err := s.gatherHealth(ctx)
	if err != nil {
		return nil, err
	}
	return buildHealth(in), nil
}

func (s *Service) gatherHealth(ctx context.Context) (healthInputs, error) {
	now := time.Now().UTC()
	in := healthInputs{Now: now, Configured: s.alerts.Configured(), Recipients: s.alerts.RecipientCount()}
	var err error
	steps := []func() error{
		func() (e error) { in.HTTP, e = s.store.RecentHTTP(ctx, now); return },
		func() (e error) { in.Errors, e = s.store.ErrorCounts(ctx, now); return },
		func() (e error) { in.Top, e = s.store.TopErrors(ctx, now); return },
		func() (e error) { in.Online, e = s.store.OnlineUsers(ctx, now); return },
		func() (e error) { in.API, e = s.store.latestAPISample(ctx); return },
		func() (e error) { in.Agent, e = s.store.latestAgentSample(ctx); return },
		func() (e error) { in.Containers, e = s.store.latestContainers(ctx, now.Add(-10*time.Minute)); return },
		func() (e error) { in.Pages, e = s.store.PageStats(ctx, now); return },
		func() (e error) { in.Checks, e = s.store.SecurityChecks(ctx); return },
		func() (e error) { in.Findings, e = s.store.SecurityFindings(ctx, todayLocal(time.Now())); return },
		func() (e error) { in.Alerts, e = s.store.ListAlerts(ctx, "open", 100); return },
		func() (e error) { in.Deploys, e = s.store.RecentDeploys(ctx, deployFetchMin); return },
	}
	for _, step := range steps {
		if err = step(); err != nil {
			return in, err
		}
	}
	return in, nil
}

func buildHealth(in healthInputs) map[string]any {
	areas := []healthArea{
		apiArea(in), databaseArea(in), containersArea(in), pagesArea(in), workersArea(in), securityArea(in), deployArea(in),
	}
	for index := range areas {
		areas[index].Status = worse(areas[index].Status, alertStatusFor(areas[index].Key, in.Alerts))
	}
	overall := statusOK
	for _, area := range areas {
		if area.Status != statusUnknown {
			overall = worse(overall, area.Status)
		}
	}
	return map[string]any{
		"generatedAt": in.Now,
		"overall":     overall,
		"areas":       areas,
		"issues":      buildIssues(in),
		"errors":      buildErrors(in),
		"online":      buildOnline(in),
		"system":      buildSystem(in),
		"alerts":      buildAlertsSummary(in),
		"security":    buildSecuritySummary(in),
	}
}

// alertStatusFor: alerta aberto crítico torna a área crítica; os demais, aviso.
func alertStatusFor(area string, alerts []alertRow) string {
	status := statusUnknown
	for _, alert := range alerts {
		if alert.Area != area {
			continue
		}
		if alert.Severity == sevCritical {
			return statusCritical
		}
		status = statusWarn
	}
	return status
}

func apiArea(in healthInputs) healthArea {
	area := healthArea{Key: "api", Label: "API", Status: statusUnknown, Summary: "sem requisições recentes"}
	w := in.HTTP
	if w.Total == 0 && w.HourCalls == 0 {
		return area
	}
	p95 := platformobs.PercentileFromBuckets(w.Buckets, 0.95)
	area.Status, area.Summary = statusOK, fmt.Sprintf("p95 %.0f ms, sem erros 5xx", p95)
	rate := 0.0
	if w.Total > 0 {
		rate = float64(w.Total5xx) / float64(w.Total)
	}
	switch {
	case w.Total >= 20 && rate > 0.20:
		area.Status = statusCritical
	case w.Total >= 20 && rate > 0.05, p95 > 2000:
		area.Status = statusWarn
	}
	if w.Total5xx > 0 {
		area.Summary = fmt.Sprintf("%d de %d requisições com 5xx em 15 min, p95 %.0f ms", w.Total5xx, w.Total, p95)
	} else if p95 > 2000 {
		area.Summary = fmt.Sprintf("p95 alto: %.0f ms", p95)
	}
	return area
}

func databaseArea(in healthInputs) healthArea {
	area := healthArea{Key: "database", Label: "Banco de dados", Status: statusUnknown, Summary: "coletor sem amostras"}
	if in.API == nil {
		return area
	}
	age := in.Now.Sub(in.API.At)
	if age > 2*time.Minute {
		area.Status, area.Summary = statusWarn, fmt.Sprintf("última amostra há %.0f min", age.Minutes())
		return area
	}
	pool, pg := in.API.Metrics.Pool, in.API.Metrics.PG
	ratio := 0.0
	if pg.MaxConnections > 0 {
		ratio = float64(pg.Connections) / float64(pg.MaxConnections)
	}
	area.Status = statusOK
	area.Summary = fmt.Sprintf("pool %.0f%%, %d de %d conexões", pool.Utilization*100, pg.Connections, pg.MaxConnections)
	switch {
	case ratio >= 0.9 || pool.Utilization >= 1:
		area.Status = statusCritical
	case ratio >= 0.7, pool.Utilization >= 0.7, pg.IdleInTransactionOver60s > 0, pg.WaitingOnLock > 0, pg.DeadlocksDelta > 0:
		area.Status = statusWarn
	}
	return area
}

func containersArea(in healthInputs) healthArea {
	area := healthArea{Key: "containers", Label: "Containers", Status: statusUnknown, Summary: "agente de containers não está rodando"}
	if in.Agent == nil {
		return area
	}
	if age := in.Now.Sub(in.Agent.At); age > 3*time.Minute {
		area.Status, area.Summary = statusWarn, fmt.Sprintf("agente sem enviar há %.0f min", age.Minutes())
		return area
	}
	down, restarting, expected, stopped := 0, 0, 0, 0
	for _, c := range in.Containers {
		if !c.ShouldBeUp() {
			if c.State != "running" {
				stopped++
			}
			continue
		}
		expected++
		if c.IsDown() {
			down++
		}
		if c.RestartDelta > 0 {
			restarting++
		}
	}
	area.Status = statusOK
	area.Summary = fmt.Sprintf("%d de %d containers esperados rodando", expected-down, expected)
	if stopped > 0 {
		area.Summary += fmt.Sprintf(", %d parados de propósito", stopped)
	}
	maxDisk := 0.0
	for _, disk := range in.Agent.Disk {
		maxDisk = max(maxDisk, disk.UsedPct)
	}
	switch {
	case down > 0 || maxDisk >= 95:
		area.Status = statusCritical
	case restarting > 0 || maxDisk >= 85:
		area.Status = statusWarn
	}
	if maxDisk > 0 {
		area.Summary += fmt.Sprintf(", disco %.0f%%", maxDisk)
	}
	return area
}

func pagesArea(in healthInputs) healthArea {
	p := in.Pages
	area := healthArea{Key: "pages", Label: "Páginas", Status: statusUnknown, Summary: "sem eventos do front em 24 h"}
	if p.Events24h == 0 {
		return area
	}
	area.Status, area.Summary = statusOK, "Web Vitals dentro do esperado"
	poor := p.LCP != nil && *p.LCP > 4000 || p.INP != nil && *p.INP > 500 || p.CLS != nil && *p.CLS > 0.25
	switch {
	case p.JSErr15m >= 20:
		area.Status, area.Summary = statusWarn, fmt.Sprintf("%d erros de JavaScript em 15 min", p.JSErr15m)
	case poor:
		area.Status, area.Summary = statusWarn, "Web Vitals p75 acima do limite (LCP, INP ou CLS)"
	case p.JSErr1h > 0:
		area.Summary = fmt.Sprintf("%d erros de JavaScript na última hora", p.JSErr1h)
	}
	return area
}

func workersArea(in healthInputs) healthArea {
	area := healthArea{Key: "workers", Label: "Workers", Status: statusUnknown, Summary: "coletor sem amostras"}
	if in.API == nil || in.Now.Sub(in.API.At) > 2*time.Minute {
		return area
	}
	jobs := in.API.Metrics.Jobs
	area.Status, area.Summary = statusOK, fmt.Sprintf("%d filas monitoradas, sem atraso", len(jobs))
	late, dead := 0, 0
	for _, job := range jobs {
		switch {
		case job.OldestDueLagSeconds > 1800:
			area.Status = statusCritical
			late++
		case job.OldestDueLagSeconds > 300, job.StuckProcessing > 0:
			area.Status = worse(area.Status, statusWarn)
			late++
		}
		if job.DeadLastHour > 0 {
			area.Status = worse(area.Status, statusWarn)
			dead++
		}
	}
	if late > 0 || dead > 0 {
		area.Summary = fmt.Sprintf("%d fila(s) atrasada(s), %d com jobs mortos", late, dead)
	}
	return area
}

func securityArea(in healthInputs) healthArea {
	area := healthArea{Key: "security", Label: "Segurança", Status: statusUnknown, Summary: "catálogo de ataques não sincronizado"}
	counts := checkCounts(in.Checks)
	total := len(in.Checks)
	if total == 0 && len(in.Findings) == 0 {
		return area
	}
	open := openFindingCounts(in.Findings)
	tested := total - counts["not_tested"]
	area.Status = statusOK
	area.Summary = fmt.Sprintf("%d de %d ataques testados", tested, total)
	switch {
	case counts["fail"] > 0 || open[sevCritical] > 0:
		area.Status = statusCritical
		area.Summary = fmt.Sprintf("%d teste(s) falhando, %d achado(s) crítico(s); %d de %d testados", counts["fail"], open[sevCritical], tested, total)
	case open[sevHigh] > 0 || counts["partial"] > 0:
		area.Status = statusWarn
		area.Summary = fmt.Sprintf("%d achado(s) alto(s) aberto(s); %d de %d ataques testados", open[sevHigh], tested, total)
	case counts["not_tested"] > 0:
		// Nunca "ok" com ataques sem teste: ausência de teste não é ausência de falha.
		area.Status = statusWarn
	}
	return area
}

func checkCounts(checks []securityCheck) map[string]int {
	counts := map[string]int{"pass": 0, "fail": 0, "partial": 0, "not_tested": 0}
	for _, check := range checks {
		counts[check.Status]++
	}
	return counts
}

func openFindingCounts(findings []securityFinding) map[string]int {
	counts := map[string]int{sevCritical: 0, sevHigh: 0, sevMedium: 0, sevLow: 0}
	for _, finding := range findings {
		if finding.Status == "open" {
			counts[finding.Severity]++
		}
	}
	return counts
}

var areaTab = map[string]string{
	"api": "endpoints", "database": "db", "containers": "system", "pages": "pages", "workers": "system", "security": "security", "deploy": "deploys",
}

func buildIssues(in healthInputs) []healthIssue {
	var issues []healthIssue
	for _, alert := range in.Alerts {
		detail, _ := alert.Detail["measured"].(string)
		tab := areaTab[alert.Area]
		if tab == "" {
			tab = "system"
		}
		issues = append(issues, healthIssue{Severity: alert.Severity, Area: alert.Area, Title: alert.Title,
			Detail: detail, Since: alert.FirstSeen, Link: issueLink{Tab: tab, Query: map[string]any{}}})
	}
	for _, finding := range in.Findings {
		if finding.Status == "open" && (finding.Severity == sevCritical || finding.Severity == sevHigh) {
			issues = append(issues, healthIssue{Severity: finding.Severity, Area: "security", Title: finding.Title,
				Detail: finding.Location, Since: finding.UpdatedAt, Link: issueLink{Tab: "security", Query: map[string]any{"finding": finding.ID}}})
		}
	}
	for _, check := range in.Checks {
		if check.Status == "fail" {
			since := check.UpdatedAt
			if check.CheckedAt != nil {
				since = *check.CheckedAt
			}
			issues = append(issues, healthIssue{Severity: sevHigh, Area: "security", Title: "Teste de segurança falhando: " + check.Title,
				Detail: check.Detail, Since: since, Link: issueLink{Tab: "security", Query: map[string]any{"check": check.Key}}})
		}
	}
	if issue := deployIssue(in); issue != nil {
		issues = append(issues, *issue)
	}
	sort.SliceStable(issues, func(i, j int) bool { return severityRank(issues[i].Severity) > severityRank(issues[j].Severity) })
	if len(issues) > maxIssues {
		issues = issues[:maxIssues]
	}
	if issues == nil {
		issues = []healthIssue{}
	}
	return issues
}

func buildErrors(in healthInputs) map[string]any {
	top := append([]errorSignature(nil), in.Top...)
	sort.SliceStable(top, func(i, j int) bool { return top[i].Count > top[j].Count })
	if len(top) > 5 {
		top = top[:5]
	}
	if top == nil {
		top = []errorSignature{}
	}
	return map[string]any{
		"lastHour": map[string]any{"api5xx": in.Errors.API1h, "frontend": in.Errors.Front1h},
		"last24h":  map[string]any{"api5xx": in.Errors.API24h, "frontend": in.Errors.Front24h},
		"top":      top,
	}
}

func buildOnline(in healthInputs) map[string]any {
	users := in.Online
	if users == nil {
		users = []onlineUser{}
	}
	return map[string]any{"count": len(users), "users": users}
}

func buildSystem(in healthInputs) map[string]any {
	var api, agent any
	if in.API != nil {
		m := in.API.Metrics
		api = map[string]any{"at": in.API.At, "goroutines": m.Go.Goroutines, "heapMB": round1(float64(m.Go.HeapAlloc) / 1048576),
			"poolUtilization": m.Pool.Utilization, "pgConnections": m.PG.Connections, "pgMaxConnections": m.PG.MaxConnections}
	}
	if in.Agent != nil {
		disk := in.Agent.Disk
		if disk == nil {
			disk = []diskMetric{}
		}
		agent = map[string]any{"at": in.Agent.At, "disk": disk, "containers": in.Agent.Docker.Containers, "running": in.Agent.Docker.Running}
	}
	return map[string]any{"api": api, "agent": agent}
}

func buildAlertsSummary(in healthInputs) map[string]any {
	open := make([]map[string]any, 0, 10)
	for _, alert := range in.Alerts {
		if len(open) == 10 {
			break
		}
		item := map[string]any{"id": alert.ID, "severity": alert.Severity, "title": alert.Title,
			"since": alert.FirstSeen, "notifiedAt": alert.NotifiedAt,
			"acknowledgedAt": alert.AcknowledgedAt, "acknowledgedBy": alert.AcknowledgedBy,
			"silencedUntil": nil, "silencedBy": nil}
		if silencedAt(alert, in.Now) {
			item["silencedUntil"], item["silencedBy"] = alert.SilencedUntil, alert.SilencedBy
		}
		open = append(open, item)
	}
	return map[string]any{"channel": "email", "configured": in.Configured, "recipients": in.Recipients, "whatsapp": "planned", "open": open}
}

func buildSecuritySummary(in healthInputs) map[string]any {
	fixed := 0
	for _, finding := range in.Findings {
		if finding.Status == "fixed" {
			fixed++
		}
	}
	return map[string]any{"checks": checkCounts(in.Checks), "findings": map[string]any{"open": openFindingCounts(in.Findings), "fixed": fixed}}
}
