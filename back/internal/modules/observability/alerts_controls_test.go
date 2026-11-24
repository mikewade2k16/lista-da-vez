package observability

import (
	"context"
	"testing"
	"time"
)

func TestNotifyReasonAcknowledged(t *testing.T) {
	ack := t0.Add(-time.Hour)
	cases := []struct {
		name string
		row  alertRow
		sev  string
		want string
	}{
		{"reconhecido nao reavisa", func() alertRow { r := rowNotified(sevHigh, 90*time.Minute); r.AcknowledgedAt = &ack; return r }(), sevHigh, ""},
		{"reconhecido ainda avisa escalada", func() alertRow { r := rowNotified(sevMedium, 90*time.Minute); r.AcknowledgedAt = &ack; return r }(), sevHigh, reasonEscalated},
		{"reconhecido sem aviso previo nao abre", alertRow{Severity: sevHigh, AcknowledgedAt: &ack, Detail: map[string]any{}}, sevHigh, ""},
	}
	for _, tc := range cases {
		if got := notifyReason(tc.row, tc.sev, t0, time.Hour); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestNotifyReasonSilenced(t *testing.T) {
	future, past := t0.Add(time.Hour), t0.Add(-time.Minute)
	silenced := rowNotified(sevMedium, 3*time.Hour)
	silenced.SilencedUntil = &future
	if got := notifyReason(silenced, sevCritical, t0, time.Hour); got != "" {
		t.Errorf("silenciado nao avisa nem escalada: %q", got)
	}
	never := alertRow{Severity: sevHigh, SilencedUntil: &future, Detail: map[string]any{}}
	if got := notifyReason(never, sevHigh, t0, time.Hour); got != "" {
		t.Errorf("silenciado nao avisa abertura: %q", got)
	}
	expired := rowNotified(sevHigh, 3*time.Hour)
	expired.SilencedUntil = &past
	if got := notifyReason(expired, sevHigh, t0, time.Hour); got != reasonRenotify {
		t.Errorf("silencio vencido volta a avisar: %q", got)
	}
}

func TestNotifyResolved(t *testing.T) {
	future, past := t0.Add(time.Hour), t0.Add(-time.Hour)
	ack := t0.Add(-time.Hour)
	notified := rowNotified(sevHigh, time.Hour)
	if !notifyResolved(notified, t0) {
		t.Error("notificado e nao silenciado deve avisar a resolucao")
	}
	notified.AcknowledgedAt = &ack
	if !notifyResolved(notified, t0) {
		t.Error("reconhecido ainda avisa a resolucao")
	}
	notified.SilencedUntil = &future
	if notifyResolved(notified, t0) {
		t.Error("silenciado nao avisa a resolucao")
	}
	notified.SilencedUntil = &past
	if !notifyResolved(notified, t0) {
		t.Error("silencio vencido avisa a resolucao")
	}
	if notifyResolved(alertRow{Detail: map[string]any{}}, t0) {
		t.Error("nunca notificado nao avisa resolucao")
	}
}

func TestParseAlertID(t *testing.T) {
	for _, ok := range []string{"1", "42", "123456789012345678"} {
		if _, valid := parseAlertID(ok); !valid {
			t.Errorf("%q deveria ser valido", ok)
		}
	}
	for _, bad := range []string{"", "0", "-1", "abc", "1;drop", "1.5", " 1", "1234567890123456789", "01"} {
		if _, valid := parseAlertID(bad); valid {
			t.Errorf("%q deveria ser invalido", bad)
		}
	}
}

func TestSilenceHoursValidation(t *testing.T) {
	s := &Service{}
	for _, hours := range []int{0, -1, 169, 1000} {
		if err := s.SilenceAlert(context.Background(), 1, hours, alertActor{}); err != ErrInvalidSilenceHours {
			t.Errorf("hours=%d: %v", hours, err)
		}
	}
}

func TestToIntegrationStat(t *testing.T) {
	stat := toIntegrationStat(integrationRow{Name: "waha", Calls: 200, Errors: 5, AvgMs: 123.456, P95Ms: 400.04})
	if stat.ErrorRate != 0.025 || stat.AvgMs != 123.5 || stat.P95Ms != 400 {
		t.Errorf("%+v", stat)
	}
	if zero := toIntegrationStat(integrationRow{Name: "x"}); zero.ErrorRate != 0 {
		t.Errorf("sem chamadas: %+v", zero)
	}
}
