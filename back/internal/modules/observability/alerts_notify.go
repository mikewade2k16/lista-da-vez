package observability

import (
	"time"
)

const (
	reasonOpened    = "opened"
	reasonEscalated = "escalated"
	reasonRenotify  = "renotify"
	reasonResolved  = "resolved"

	// retryAfterFailure evita reenviar a cada rodada quando o SMTP falhou.
	retryAfterFailure = 5 * time.Minute
)

// notifyState é o estado de notificação guardado em detail._notify.
type notifyState struct {
	Severity  string     `json:"severity,omitempty"`
	AttemptAt *time.Time `json:"attemptAt,omitempty"`
}

func notifyStateFrom(detail map[string]any) notifyState {
	var state notifyState
	raw, ok := detail["_notify"].(map[string]any)
	if !ok {
		return state
	}
	state.Severity, _ = raw["severity"].(string)
	if text, ok := raw["attemptAt"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
			state.AttemptAt = &parsed
		}
	}
	return state
}

// silencedAt: o alerta está silenciado em `now` (nenhum e-mail até silenced_until).
func silencedAt(row alertRow, now time.Time) bool {
	return row.SilencedUntil != nil && row.SilencedUntil.After(now)
}

// notifyReason decide se um alerta aberto deve ser notificado agora (anti-spam):
// ao abrir, ao subir de severidade e, no máximo, a cada `renotify` enquanto aberto.
// `row` é o estado anterior; `severity` é a severidade atual da condição.
// Silenciado: nada até silenced_until. Reconhecido: não reavisa nem avisa a abertura
// pendente, mas uma subida de severidade ainda avisa.
func notifyReason(row alertRow, severity string, now time.Time, renotify time.Duration) string {
	if silencedAt(row, now) {
		return ""
	}
	state := notifyStateFrom(row.Detail)
	if row.NotifiedAt == nil {
		if row.AcknowledgedAt != nil {
			return ""
		}
		if state.AttemptAt != nil && now.Sub(*state.AttemptAt) < retryAfterFailure {
			return ""
		}
		return reasonOpened
	}
	notified := state.Severity
	if notified == "" {
		notified = row.Severity
	}
	if severityRank(severity) > severityRank(notified) {
		return reasonEscalated
	}
	if row.AcknowledgedAt != nil {
		return ""
	}
	if now.Sub(*row.NotifiedAt) >= renotify {
		return reasonRenotify
	}
	return ""
}

// notifyResolved: avisa a resolução só se o alerta chegou a ser notificado e não está
// silenciado. Reconhecer não suprime a resolução.
func notifyResolved(row alertRow, now time.Time) bool {
	return row.NotifiedAt != nil && !silencedAt(row, now)
}

type notifyItem struct {
	Alert  alertRow
	Reason string
}

var brt = time.FixedZone("BRT", -3*60*60)

func severityLabel(severity string) string {
	switch severity {
	case sevCritical:
		return "CRITICO"
	case sevHigh:
		return "ALTO"
	case sevMedium:
		return "MEDIO"
	}
	return "BAIXO"
}

func filterReason(items []notifyItem, reason string) []notifyItem {
	var out []notifyItem
	for _, item := range items {
		if item.Reason == reason {
			out = append(out, item)
		}
	}
	return out
}

func testDigest(link string, now time.Time) (subject, body string) {
	body = "Este é um e-mail de teste dos alertas do Omni, enviado em " + now.In(brt).Format("02/01/2006 15:04") + ".\n" +
		"Se você o recebeu, o canal de e-mail está funcionando.\n"
	if link != "" {
		body += "\nPainel: " + link + "\n"
	}
	return "[Omni] Teste de alertas", body
}
