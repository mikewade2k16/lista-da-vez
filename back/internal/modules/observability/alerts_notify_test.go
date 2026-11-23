package observability

import (
	"context"
	"strings"
	"testing"
	"time"
)

func rowNotified(severity string, notifiedAgo time.Duration) alertRow {
	at := t0.Add(-notifiedAgo)
	return alertRow{Severity: severity, NotifiedAt: &at, Detail: map[string]any{}}
}

func TestNotifyReason(t *testing.T) {
	hour := time.Hour
	cases := []struct {
		name string
		row  alertRow
		sev  string
		want string
	}{
		{"nunca notificado", alertRow{Severity: sevHigh, Detail: map[string]any{}}, sevHigh, reasonOpened},
		{"notificado agora, mesma severidade", rowNotified(sevHigh, time.Minute), sevHigh, ""},
		{"sobe severidade", rowNotified(sevMedium, time.Minute), sevHigh, reasonEscalated},
		{"desce severidade nao notifica", rowNotified(sevHigh, time.Minute), sevMedium, ""},
		{"renotifica depois do intervalo", rowNotified(sevHigh, 61*time.Minute), sevHigh, reasonRenotify},
		{"nao renotifica antes", rowNotified(sevHigh, 59*time.Minute), sevHigh, ""},
	}
	for _, tc := range cases {
		if got := notifyReason(tc.row, tc.sev, t0, hour); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestNotifyReasonUsesNotifiedSeverityFromState(t *testing.T) {
	row := rowNotified(sevHigh, time.Minute) // linha já atualizada para high...
	row.Detail["_notify"] = map[string]any{"severity": sevMedium}
	if got := notifyReason(row, sevHigh, t0, time.Hour); got != reasonEscalated {
		t.Fatalf("a severidade notificada era medium: %q", got)
	}
}

func TestNotifyReasonBackoffAfterFailure(t *testing.T) {
	attempt := t0.Add(-time.Minute).Format(time.RFC3339Nano)
	row := alertRow{Severity: sevHigh, Detail: map[string]any{"_notify": map[string]any{"attemptAt": attempt}}}
	if got := notifyReason(row, sevHigh, t0, time.Hour); got != "" {
		t.Fatalf("deveria esperar o backoff: %q", got)
	}
	row.Detail["_notify"] = map[string]any{"attemptAt": t0.Add(-6 * time.Minute).Format(time.RFC3339Nano)}
	if got := notifyReason(row, sevHigh, t0, time.Hour); got != reasonOpened {
		t.Fatalf("passado o backoff deve tentar de novo: %q", got)
	}
}

func TestPlanSync(t *testing.T) {
	open := []alertRow{{ID: 1, Fingerprint: "a"}, {ID: 2, Fingerprint: "b"}}
	conds := []condition{{Rule: "a"}, {Rule: "c"}, {Rule: "c"}}
	plan := planSync(conds, open)
	if len(plan.Update) != 1 || plan.Update[0].Row.ID != 1 {
		t.Errorf("update: %+v", plan.Update)
	}
	if len(plan.Insert) != 1 || plan.Insert[0].Rule != "c" {
		t.Errorf("insert deduplicado: %+v", plan.Insert)
	}
	if len(plan.Resolve) != 1 || plan.Resolve[0].ID != 2 {
		t.Errorf("resolve: %+v", plan.Resolve)
	}
}

func TestBuildDigest(t *testing.T) {
	resolvedAt := t0
	items := []notifyItem{
		{Alert: alertRow{Severity: sevMedium, Area: "api", Title: "Rota lenta: GET /x", FirstSeen: t0, Detail: map[string]any{"measured": "p95 3000 ms"}}, Reason: reasonOpened},
		{Alert: alertRow{Severity: sevCritical, Area: "containers", Title: "Container fora do ar: web", FirstSeen: t0, Detail: map[string]any{}}, Reason: reasonEscalated},
		{Alert: alertRow{Severity: sevHigh, Area: "database", Title: "Deadlock", FirstSeen: t0, ResolvedAt: &resolvedAt, Detail: map[string]any{}}, Reason: reasonResolved},
	}
	subject, body := buildDigest(items, "https://app.example/manage/observabilidade")
	if !strings.Contains(subject, "2 alerta(s) ativo(s)") || !strings.Contains(subject, "CRITICO") {
		t.Errorf("assunto: %q", subject)
	}
	for _, want := range []string{"NOVOS", "AGRAVADOS", "RESOLVIDOS", "p95 3000 ms", "Painel: https://app.example/manage/observabilidade", "Onde: containers"} {
		if !strings.Contains(body, want) {
			t.Errorf("corpo sem %q:\n%s", want, body)
		}
	}
	subject, _ = buildDigest(items[2:], "")
	if subject != "[Omni] 1 alerta(s) resolvido(s)" {
		t.Errorf("assunto resolvido: %q", subject)
	}
}

type fakeSender struct {
	configured bool
	sent       int
}

func (f *fakeSender) Configured() bool { return f.configured }
func (f *fakeSender) Send(_ context.Context, _ []string, _, _ string) error {
	f.sent++
	return nil
}

func TestAlertsConfigured(t *testing.T) {
	cases := []struct {
		name       string
		alerts     *Alerts
		wantConfig bool
	}{
		{"nil", nil, false},
		{"smtp sem destinatario", &Alerts{sender: &fakeSender{configured: true}}, false},
		{"destinatario sem smtp", &Alerts{sender: &fakeSender{}, cfg: AlertConfig{Recipients: []string{"a@b.com"}}}, false},
		{"completo", &Alerts{sender: &fakeSender{configured: true}, cfg: AlertConfig{Recipients: []string{"a@b.com"}}}, true},
	}
	for _, tc := range cases {
		if tc.alerts.Configured() != tc.wantConfig {
			t.Errorf("%s", tc.name)
		}
	}
	var none *Alerts
	if err := none.SendTest(context.Background()); err != ErrAlertsNotConfigured {
		t.Errorf("SendTest sem config: %v", err)
	}
}

func TestAlertConfigFromEnv(t *testing.T) {
	t.Setenv("OBS_ALERT_EMAILS", " a@x.com, ,b@x.com ")
	t.Setenv("OBS_ALERT_RENOTIFY_MINUTES", "15")
	t.Setenv("OBS_ALERT_INTERVAL", "30s")
	t.Setenv("OBS_ALERTS_ENABLED", "false")
	cfg := AlertConfigFromEnv("https://app.example/")
	if len(cfg.Recipients) != 2 || cfg.Renotify != 15*time.Minute || cfg.Interval != 30*time.Second || cfg.Enabled {
		t.Errorf("cfg: %+v", cfg)
	}
	if cfg.panelLink() != "https://app.example/manage/observabilidade" {
		t.Errorf("link: %q", cfg.panelLink())
	}
}
