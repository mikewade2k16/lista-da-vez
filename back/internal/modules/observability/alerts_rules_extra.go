package observability

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	tlsCriticalDays = 3
	tlsErrorMinSpan = 6 * time.Hour // duas checagens (o agente checa a cada 6 h)
	panicRecentSpan = 15 * time.Minute
	tlsMaxHostsKept = 50
)

// tlsMetric é um item de metrics.tls da amostra do agente. Todos os campos são
// opcionais: o coletor evolui em paralelo e campo ausente nunca vira condição.
type tlsMetric struct {
	Host      string
	ExpiresAt *time.Time
	DaysLeft  *float64
	Error     string
}

// parseTLS lê o JSON de metrics.tls de forma tolerante: qualquer formato inesperado
// (não é array, item que não é objeto, tipo trocado) é ignorado, sem erro.
func parseTLS(raw []byte) []tlsMetric {
	var items []map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &items) != nil {
		return nil
	}
	var out []tlsMetric
	for _, item := range items {
		host, _ := item["host"].(string)
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		metric := tlsMetric{Host: host}
		if text, ok := item["expiresAt"].(string); ok {
			if parsed, err := time.Parse(time.RFC3339, text); err == nil {
				metric.ExpiresAt = &parsed
			}
		}
		if days, ok := item["daysLeft"].(float64); ok {
			metric.DaysLeft = &days
		}
		if text, ok := item["error"].(string); ok {
			metric.Error = strings.TrimSpace(text)
		}
		out = append(out, metric)
		if len(out) == tlsMaxHostsKept {
			break
		}
	}
	return out
}

// daysLeftAt prefere expiresAt (o cache do agente envelhece entre checagens) e cai
// para daysLeft. ok=false quando nenhum dos dois veio.
func (t tlsMetric) daysLeftAt(now time.Time) (float64, bool) {
	if t.ExpiresAt != nil {
		return math.Floor(t.ExpiresAt.Sub(now).Hours() / 24), true
	}
	if t.DaysLeft != nil {
		return *t.DaysLeft, true
	}
	return 0, false
}

// ruleTLS: vencimento em <= 14 dias é high, <= 3 (ou vencido) é critical; erro de
// checagem persistente (>= 6 h, duas checagens) é medium.
func ruleTLS(m alertMetrics) []condition {
	var out []condition
	warnDays := m.threshold("tls_expiring")
	for _, item := range m.TLS {
		days, known := item.daysLeftAt(m.Now)
		if known && days <= warnDays {
			severity, title := sevHigh, "Certificado TLS perto de vencer: "+item.Host
			measured := fmt.Sprintf("vence em %.0f dia(s)", days)
			if days <= min(float64(tlsCriticalDays), warnDays) {
				severity = sevCritical
			}
			if days < 0 {
				title = "Certificado TLS vencido: " + item.Host
				measured = fmt.Sprintf("vencido há %.0f dia(s)", -days)
			}
			out = append(out, condition{Rule: "tls_expiring", Target: item.Host, Severity: severity, Area: "containers",
				Title: title, Measured: measured, Detail: map[string]any{"host": item.Host, "daysLeft": days}})
			continue
		}
		since, tracked := m.TLSErrorSince[item.Host]
		if item.Error != "" && tracked && m.Now.Sub(since) >= tlsErrorMinSpan {
			out = append(out, condition{Rule: "tls_expiring", Target: item.Host, Severity: sevMedium, Area: "containers",
				Title:    "Não foi possível checar o certificado TLS: " + item.Host,
				Measured: fmt.Sprintf("checagem com erro há %.0f h", m.Now.Sub(since).Hours()),
				Detail:   map[string]any{"host": item.Host, "error": truncateText(item.Error, 160)}})
		}
	}
	return out
}

func truncateText(text string, limit int) string {
	if len(text) > limit {
		return text[:limit]
	}
	return text
}

// rulePanics: soma os panics recuperados (middleware HTTP, background e jobs) nas
// amostras dos últimos 15 min; qualquer panic abre o alerta e ele fecha sozinho
// depois da janela (evita abrir e resolver a cada amostra).
func rulePanics(m alertMetrics) []condition {
	var recent, total int64
	for index, sample := range m.APISamples {
		if index == 0 {
			total = sample.Metrics.Panics.Total
		}
		if m.Now.Sub(sample.At) <= panicRecentSpan && sample.Metrics.Panics.Delta > 0 {
			recent += sample.Metrics.Panics.Delta
		}
	}
	if recent <= 0 {
		return nil
	}
	return []condition{{Rule: "panic_detected", Severity: sevHigh, Area: "api",
		Title:    "Panic recuperado na API",
		Measured: fmt.Sprintf("%d panic(s) em 15 min (%d desde o início do processo)", recent, total),
		Detail:   map[string]any{"panicsRecent": recent, "panicsTotal": total}}}
}
