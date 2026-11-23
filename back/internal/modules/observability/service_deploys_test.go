package observability

import (
	"strings"
	"testing"
	"time"
)

func deployAt(id, command, status string, ms float64, ago time.Duration) deployRow {
	duration := ms
	started := t0.Add(-ago)
	return deployRow{ID: id, Command: command, Status: status, DurationMs: &duration, StartedAt: started, FinishedAt: &started}
}

func TestPercentile(t *testing.T) {
	if got := percentile(nil, 0.5); got != 0 {
		t.Errorf("vazio: %v", got)
	}
	values := []float64{40, 10, 30, 20}
	if got := percentile(values, 0.5); got != 25 {
		t.Errorf("mediana: %v", got)
	}
	if got := percentile(values, 0.9); got != 37 {
		t.Errorf("p90: %v", got)
	}
	if values[0] != 40 {
		t.Error("não deve reordenar a entrada")
	}
}

func TestVerdicts(t *testing.T) {
	rows := []deployRow{
		deployAt("a", "fast-prod", "success", 400000, time.Hour),
		deployAt("b", "fast-prod", "failed", 20000, 2*time.Hour),
		deployAt("c", "fast-prod", "success", 200000, 3*time.Hour),
		deployAt("d", "fast-prod", "success", 220000, 4*time.Hour),
		deployAt("e", "fast-prod", "success", 210000, 5*time.Hour),
		deployAt("f", "staging", "success", 900000, 6*time.Hour),
	}
	applyVerdicts(rows)
	want := map[string]string{"a": verdictSlow, "b": verdictFailed, "c": verdictOK, "d": verdictOK, "e": verdictOK, "f": verdictOK}
	for _, row := range rows {
		if row.Verdict != want[row.ID] {
			t.Errorf("%s: %s, esperado %s", row.ID, row.Verdict, want[row.ID])
		}
	}
}

func TestVerdictNeedsThreeReferences(t *testing.T) {
	rows := []deployRow{
		deployAt("a", "fast-prod", "success", 900000, time.Hour),
		deployAt("b", "fast-prod", "success", 100000, 2*time.Hour),
	}
	applyVerdicts(rows)
	if rows[0].Verdict != verdictOK {
		t.Errorf("com menos de 3 referências não há veredito lento: %s", rows[0].Verdict)
	}
	rows[1].Status = "interrupted"
	applyVerdicts(rows)
	if rows[1].Verdict != verdictFailed {
		t.Errorf("interrompido conta como falha: %s", rows[1].Verdict)
	}
}

func TestDeployStats(t *testing.T) {
	if got := computeDeployStats(nil); got.Count != 0 || got.LastSuccessAt != nil {
		t.Errorf("%+v", got)
	}
	rows := []deployRow{
		deployAt("a", "fast-prod", "failed", 1000, time.Hour),
		deployAt("b", "fast-prod", "success", 100000, 2*time.Hour),
		deployAt("c", "fast-prod", "success", 200000, 3*time.Hour),
		deployAt("d", "fast-prod", "success", 300000, 4*time.Hour),
	}
	got := computeDeployStats(rows)
	if got.Count != 4 || got.SuccessRate != 0.75 {
		t.Errorf("%+v", got)
	}
	if got.MedianMs != 200000 || got.P90Ms != 280000 || got.SlowThresholdMs != 300000 {
		t.Errorf("%+v", got)
	}
	if got.LastSuccessAt == nil || !got.LastSuccessAt.Equal(rows[1].StartedAt) {
		t.Errorf("lastSuccessAt: %v", got.LastSuccessAt)
	}
}

func TestDeployAreaAndIssue(t *testing.T) {
	area := deployArea(healthInputs{Now: t0})
	if area.Status != statusUnknown || deployIssue(healthInputs{Now: t0}) != nil {
		t.Errorf("sem deploys: %+v", area)
	}
	ok := []deployRow{
		deployAt("a", "fast-prod", "success", 222000, 2*time.Hour),
		deployAt("b", "fast-prod", "success", 220000, 3*time.Hour),
		deployAt("c", "fast-prod", "success", 230000, 4*time.Hour),
		deployAt("d", "fast-prod", "success", 210000, 5*time.Hour),
	}
	area = deployArea(healthInputs{Now: t0, Deploys: ok})
	if area.Status != statusOK || area.Summary != "último: 3min42s, ok, há 2 h" {
		t.Errorf("%+v", area)
	}
	if deployIssue(healthInputs{Now: t0, Deploys: ok}) != nil {
		t.Error("deploy ok não gera issue")
	}

	slow := append([]deployRow{deployAt("s", "fast-prod", "success", 600000, time.Hour)}, ok...)
	area = deployArea(healthInputs{Now: t0, Deploys: slow})
	issue := deployIssue(healthInputs{Now: t0, Deploys: slow})
	if area.Status != statusWarn || issue == nil || issue.Severity != sevMedium || issue.Link.Tab != "deploys" {
		t.Errorf("lento: %+v %+v", area, issue)
	}

	failed := append([]deployRow{deployAt("f", "fast-prod", "failed", 17000, 30*time.Minute)}, ok...)
	area = deployArea(healthInputs{Now: t0, Deploys: failed})
	issue = deployIssue(healthInputs{Now: t0, Deploys: failed})
	if area.Status != statusCritical || !strings.Contains(area.Summary, "falhou") || issue == nil || issue.Severity != sevHigh {
		t.Errorf("falhou: %+v %+v", area, issue)
	}
	old := append([]deployRow{deployAt("f", "fast-prod", "failed", 17000, 30*time.Hour)}, ok...)
	if got := deployArea(healthInputs{Now: t0, Deploys: old}); got.Status != statusWarn {
		t.Errorf("falha antiga vira aviso: %+v", got)
	}
}

func TestParseDeployLimit(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{{"", 30}, {"1", 1}, {"100", 100}, {" 50 ", 50}}
	for _, tc := range cases {
		if got, ok := parseDeployLimit(tc.raw); !ok || got != tc.want {
			t.Errorf("%q: %d %v", tc.raw, got, ok)
		}
	}
	for _, raw := range []string{"0", "101", "-1", "x"} {
		if _, ok := parseDeployLimit(raw); ok {
			t.Errorf("%q deveria ser inválido", raw)
		}
	}
}
