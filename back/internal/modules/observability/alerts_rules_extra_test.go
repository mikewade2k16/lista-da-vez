package observability

import (
	"encoding/json"
	"testing"
	"time"
)

func tlsExpiringIn(host string, days float64) tlsMetric {
	at := t0.Add(time.Duration(days*24) * time.Hour)
	return tlsMetric{Host: host, ExpiresAt: &at}
}

func TestParseTLSTolerant(t *testing.T) {
	items := parseTLS([]byte(`[{"host":"omni.example","expiresAt":"2026-12-01T00:00:00Z","daysLeft":7},{"host":""},{"host":"x","daysLeft":"abc","expiresAt":12}]`))
	if len(items) != 2 || items[0].ExpiresAt == nil || items[0].DaysLeft == nil || *items[0].DaysLeft != 7 {
		t.Fatalf("%+v", items)
	}
	if items[1].Host != "x" || items[1].ExpiresAt != nil || items[1].DaysLeft != nil {
		t.Errorf("campos de tipo trocado devem ser ignorados: %+v", items[1])
	}
	for _, raw := range []string{"", "null", "{}", `"x"`, "[1,2]", `[{"host":1}]`} {
		if got := parseTLS([]byte(raw)); len(got) != 0 {
			t.Errorf("%q: %+v", raw, got)
		}
	}
}

func TestRuleTLSSeverity(t *testing.T) {
	m := baseMetrics()
	m.LastAgentAt = ptr(t0)
	m.TLS = []tlsMetric{tlsExpiringIn("ok.example", 40), tlsExpiringIn("soon.example", 10),
		tlsExpiringIn("crit.example", 2), tlsExpiringIn("dead.example", -3)}
	got := ruleTLS(m)
	if findRule(got, "tls_expiring", "ok.example") != nil {
		t.Error("40 dias nao alerta")
	}
	cases := map[string]string{"soon.example": sevHigh, "crit.example": sevCritical, "dead.example": sevCritical}
	for host, sev := range cases {
		c := findRule(got, "tls_expiring", host)
		if c == nil || c.Severity != sev {
			t.Errorf("%s: %+v", host, c)
		}
	}
}

func TestRuleTLSUsesDaysLeftWithoutExpiresAt(t *testing.T) {
	m := baseMetrics()
	m.TLS = []tlsMetric{{Host: "a", DaysLeft: ptr(5.0)}, {Host: "b", DaysLeft: ptr(30.0)}, {Host: "c"}}
	got := ruleTLS(m)
	if len(got) != 1 || got[0].Target != "a" || got[0].Severity != sevHigh {
		t.Fatalf("%+v", got)
	}
}

func TestRuleTLSErrorNeedsTwoChecks(t *testing.T) {
	m := baseMetrics()
	m.TLS = []tlsMetric{{Host: "bad.example", Error: "connection refused"}}
	if got := ruleTLS(m); len(got) != 0 {
		t.Fatalf("sem historico nao alerta: %+v", got)
	}
	m.TLSErrorSince = map[string]time.Time{"bad.example": t0.Add(-time.Hour)}
	if got := ruleTLS(m); len(got) != 0 {
		t.Fatalf("1 h de erro e uma checagem so: %+v", got)
	}
	m.TLSErrorSince["bad.example"] = t0.Add(-7 * time.Hour)
	got := ruleTLS(m)
	if len(got) != 1 || got[0].Severity != sevMedium || got[0].Fingerprint() != "tls_expiring:bad.example" {
		t.Fatalf("%+v", got)
	}
}

func TestRuleTLSAbsent(t *testing.T) {
	if got := ruleTLS(baseMetrics()); len(got) != 0 {
		t.Fatalf("coletor sem tls nao gera condicao: %+v", got)
	}
}

func TestRulePanics(t *testing.T) {
	m := baseMetrics()
	m.APISamples = []apiSample{sampleAt(0, nil)}
	if got := rulePanics(m); len(got) != 0 {
		t.Fatalf("sem panic: %+v", got)
	}
	m.APISamples = []apiSample{
		sampleAt(0, func(a *apiMetrics) { a.Panics.Total = 3 }),
		sampleAt(2*time.Minute, func(a *apiMetrics) { a.Panics.Total, a.Panics.Delta = 3, 2 }),
		sampleAt(5*time.Minute, func(a *apiMetrics) { a.Panics.Total, a.Panics.Delta = 1, 1 }),
	}
	got := rulePanics(m)
	if len(got) != 1 || got[0].Rule != "panic_detected" || got[0].Severity != sevHigh {
		t.Fatalf("%+v", got)
	}
	if got[0].Detail["panicsRecent"] != int64(3) || got[0].Detail["panicsTotal"] != int64(3) {
		t.Errorf("detail: %+v", got[0].Detail)
	}
	// panic antigo (fora da janela de 15 min) nao mantém o alerta aberto
	m.APISamples = []apiSample{sampleAt(0, nil), sampleAt(20*time.Minute, func(a *apiMetrics) { a.Panics.Delta = 4 })}
	if got := rulePanics(m); len(got) != 0 {
		t.Fatalf("fora da janela: %+v", got)
	}
}

func TestPanicsFieldDecodesFromSampleJSON(t *testing.T) {
	// amostras antigas (sem "panics") decodificam em zero
	var metrics apiMetrics
	if err := json.Unmarshal([]byte(`{"go":{"goroutines":5}}`), &metrics); err != nil || metrics.Panics.Delta != 0 {
		t.Fatalf("%v %+v", err, metrics)
	}
	if err := json.Unmarshal([]byte(`{"panics":{"total":4,"delta":1}}`), &metrics); err != nil || metrics.Panics.Total != 4 || metrics.Panics.Delta != 1 {
		t.Fatalf("%v %+v", err, metrics)
	}
}
