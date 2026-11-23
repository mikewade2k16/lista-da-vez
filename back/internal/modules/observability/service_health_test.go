package observability

import (
	"strings"
	"testing"
	"time"
)

func areaByKey(areas []healthArea, key string) healthArea {
	for _, a := range areas {
		if a.Key == key {
			return a
		}
	}
	return healthArea{}
}

func areasOf(in healthInputs) []healthArea { return buildHealth(in)["areas"].([]healthArea) }

func TestHealthEmptyIsUnknown(t *testing.T) {
	out := buildHealth(healthInputs{Now: t0})
	areas := out["areas"].([]healthArea)
	for _, key := range []string{"api", "database", "containers", "pages", "workers", "security"} {
		if areaByKey(areas, key).Status != statusUnknown {
			t.Errorf("%s deveria ser unknown", key)
		}
	}
	if got := areaByKey(areas, "containers").Summary; got != "agente de containers não está rodando" {
		t.Errorf("summary: %q", got)
	}
	if out["overall"] != statusOK {
		t.Errorf("overall sem dados: %v", out["overall"])
	}
}

func TestSecurityNeverOKWithNotTested(t *testing.T) {
	in := healthInputs{Now: t0, Checks: []securityCheck{{Status: "pass"}, {Status: "pass"}, {Status: "not_tested"}}}
	a := areaByKey(areasOf(in), "security")
	if a.Status != statusWarn || a.Summary != "2 de 3 ataques testados" {
		t.Errorf("%+v", a)
	}
	in.Checks[2].Status = "pass"
	if areaByKey(areasOf(in), "security").Status != statusOK {
		t.Error("tudo testado e passando deve ser ok")
	}
	in.Checks[2].Status = "fail"
	if areaByKey(areasOf(in), "security").Status != statusCritical {
		t.Error("fail deve ser critical")
	}
	in.Checks[2].Status = "pass"
	in.Findings = []securityFinding{{Severity: sevHigh, Status: "open"}}
	if areaByKey(areasOf(in), "security").Status != statusWarn {
		t.Error("achado alto aberto deve ser warn")
	}
	in.Findings = []securityFinding{{Severity: sevCritical, Status: "open"}}
	if areaByKey(areasOf(in), "security").Status != statusCritical {
		t.Error("achado critico aberto deve ser critical")
	}
}

func TestContainersArea(t *testing.T) {
	in := healthInputs{Now: t0, Agent: &agentSample{At: t0.Add(-10 * time.Second), Disk: []diskMetric{{UsedPct: 50}}},
		Containers: []containerState{{Container: "a", State: "running", RestartPolicy: "always"}, {Container: "b", State: "running", RestartPolicy: "always"},
			{Container: "x", State: "exited", RestartPolicy: "no"}, {Container: "y", State: "exited"}}}
	if a := areaByKey(areasOf(in), "containers"); a.Status != statusOK || !strings.HasPrefix(a.Summary, "2 de 2 containers esperados rodando, 2 parados de propósito") {
		t.Errorf("%+v", a)
	}
	in.Containers[1].Health = "unhealthy"
	if a := areaByKey(areasOf(in), "containers"); a.Status != statusCritical {
		t.Errorf("%+v", a)
	}
	in.Agent.At = t0.Add(-10 * time.Minute)
	if a := areaByKey(areasOf(in), "containers"); a.Status != statusWarn {
		t.Errorf("agente atrasado: %+v", a)
	}
}

func TestDatabaseAndAPIAreas(t *testing.T) {
	sample := &apiSample{At: t0}
	sample.Metrics.PG.Connections, sample.Metrics.PG.MaxConnections = 95, 100
	in := healthInputs{Now: t0, API: sample}
	if a := areaByKey(areasOf(in), "database"); a.Status != statusCritical {
		t.Errorf("%+v", a)
	}
	in = healthInputs{Now: t0, HTTP: httpWindow{Total: 100, Total5xx: 30, HourCalls: 100}}
	if a := areaByKey(areasOf(in), "api"); a.Status != statusCritical {
		t.Errorf("%+v", a)
	}
	in.HTTP.Total5xx = 1
	if a := areaByKey(areasOf(in), "api"); a.Status != statusOK {
		t.Errorf("%+v", a)
	}
}

func TestOpenAlertRaisesAreaStatus(t *testing.T) {
	in := healthInputs{Now: t0, HTTP: httpWindow{Total: 100, HourCalls: 100},
		Alerts: []alertRow{{Area: "api", Severity: sevMedium}}}
	if a := areaByKey(areasOf(in), "api"); a.Status != statusWarn {
		t.Errorf("%+v", a)
	}
	in.Alerts[0].Severity = sevCritical
	if a := areaByKey(areasOf(in), "api"); a.Status != statusCritical {
		t.Errorf("%+v", a)
	}
}

func TestIssuesOrderedAndCapped(t *testing.T) {
	in := healthInputs{Now: t0}
	for i := 0; i < 20; i++ {
		in.Alerts = append(in.Alerts, alertRow{Area: "api", Severity: sevMedium, Title: "m", Detail: map[string]any{}})
	}
	in.Findings = []securityFinding{{ID: "F1", Severity: sevCritical, Status: "open", Title: "c"}, {ID: "F2", Severity: sevLow, Status: "open"}, {ID: "F3", Severity: sevHigh, Status: "fixed"}}
	in.Checks = []securityCheck{{Key: "k", Status: "fail", Title: "t"}}
	issues := buildIssues(in)
	if len(issues) != maxIssues {
		t.Fatalf("cap: %d", len(issues))
	}
	if issues[0].Severity != sevCritical || issues[1].Severity != sevHigh {
		t.Errorf("ordem: %+v %+v", issues[0], issues[1])
	}
	for _, issue := range issues {
		if issue.Title == "" && issue.Severity == sevLow {
			t.Error("achado low nao entra")
		}
	}
}

func TestBuildErrorsTop5(t *testing.T) {
	var top []errorSignature
	for i := 0; i < 8; i++ {
		top = append(top, errorSignature{Source: "api", Signature: "s", Count: int64(i)})
	}
	got := buildErrors(healthInputs{Top: top})["top"].([]errorSignature)
	if len(got) != 5 || got[0].Count != 7 {
		t.Errorf("%+v", got)
	}
}

func TestQueryValidation(t *testing.T) {
	for raw, want := range map[string]struct {
		v  int
		ok bool
	}{"": {6, true}, "1": {1, true}, "72": {72, true}, "0": {0, false}, "73": {0, false}, "x": {0, false}, "-1": {0, false}} {
		v, ok := parseHours(raw)
		if v != want.v || ok != want.ok {
			t.Errorf("hours %q: %d %v", raw, v, ok)
		}
	}
	for raw, ok := range map[string]bool{"": true, "open": true, "resolved": true, "all": true, "closed": false, "OPEN": false} {
		if _, got := parseAlertStatus(raw); got != ok {
			t.Errorf("status %q", raw)
		}
	}
}
