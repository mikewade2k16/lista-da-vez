package observability

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// outgoingEmail é um e-mail pronto (assunto + texto simples).
type outgoingEmail struct {
	Subject string
	Body    string
}

// buildDigest monta o e-mail agrupado com o layout padrão.
func buildDigest(items []notifyItem, link string) (subject, body string) {
	mail := buildEmails(items, link, defaultEmailSettings())
	if len(mail) == 0 {
		return "", ""
	}
	return mail[0].Subject, mail[0].Body
}

// buildEmails aplica o layout (chave `email`): agrupa a rodada em um e-mail ou
// separa um por alerta, filtra resolvidos e escolhe as seções. Só ids, rotas e
// contagens: nenhum dado de negócio.
func buildEmails(items []notifyItem, link string, layout EmailSettings) []outgoingEmail {
	if !layout.IncludeResolved {
		kept := make([]notifyItem, 0, len(items))
		for _, item := range items {
			if item.Reason != reasonResolved {
				kept = append(kept, item)
			}
		}
		items = kept
	}
	if len(items) == 0 {
		return nil
	}
	if layout.Group {
		return []outgoingEmail{buildOneEmail(items, link, layout, false)}
	}
	out := make([]outgoingEmail, 0, len(items))
	for _, item := range items {
		out = append(out, buildOneEmail([]notifyItem{item}, link, layout, true))
	}
	return out
}

func subjectWithPrefix(prefix, text string) string {
	if prefix = strings.TrimSpace(prefix); prefix == "" {
		return text
	}
	return prefix + " " + text
}

// buildOneEmail: single = e-mail de um só alerta (layout sem agrupamento), com o título no assunto.
func buildOneEmail(items []notifyItem, link string, layout EmailSettings, single bool) outgoingEmail {
	var active, resolved []notifyItem
	for _, item := range items {
		if item.Reason == reasonResolved {
			resolved = append(resolved, item)
		} else {
			active = append(active, item)
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		return severityRank(active[i].Alert.Severity) > severityRank(active[j].Alert.Severity)
	})

	var subject string
	switch {
	case single && len(active) == 1:
		subject = fmt.Sprintf("%s - %s", severityLabel(active[0].Alert.Severity), active[0].Alert.Title)
	case single:
		subject = "Resolvido - " + resolved[0].Alert.Title
	case len(active) > 0:
		subject = fmt.Sprintf("%d alerta(s) ativo(s) - %s", len(active), severityLabel(active[0].Alert.Severity))
	default:
		subject = fmt.Sprintf("%d alerta(s) resolvido(s)", len(resolved))
	}

	sections := map[string]bool{}
	for _, section := range layout.Sections {
		sections[section] = true
	}
	var b strings.Builder
	if sections[sectionSummary] {
		b.WriteString("Resumo do monitoramento do Omni.\n")
	}
	writeSection := func(title string, list []notifyItem) {
		if len(list) == 0 {
			return
		}
		fmt.Fprintf(&b, "\n%s\n", title)
		for _, item := range list {
			writeAlert(&b, item, sections[sectionDetails])
		}
	}
	writeSection("NOVOS", filterReason(active, reasonOpened))
	writeSection("AGRAVADOS", filterReason(active, reasonEscalated))
	writeSection("CONTINUAM ABERTOS", filterReason(active, reasonRenotify))
	writeSection("RESOLVIDOS", resolved)
	if link != "" && sections[sectionLink] {
		fmt.Fprintf(&b, "\nPainel: %s\n", link)
	}
	if footer := strings.TrimSpace(layout.Footer); footer != "" {
		fmt.Fprintf(&b, "\n%s\n", footer)
	}
	return outgoingEmail{Subject: subjectWithPrefix(layout.SubjectPrefix, subject), Body: b.String()}
}

// writeAlert escreve o título; com `details`, também onde, desde quando e a medição.
func writeAlert(b *strings.Builder, item notifyItem, details bool) {
	alert := item.Alert
	fmt.Fprintf(b, "\n- [%s] %s\n", severityLabel(alert.Severity), alert.Title)
	if !details {
		return
	}
	fmt.Fprintf(b, "  Onde: %s\n", alert.Area)
	fmt.Fprintf(b, "  Desde: %s\n", alert.FirstSeen.In(brt).Format("02/01/2006 15:04"))
	if measured, _ := alert.Detail["measured"].(string); measured != "" {
		fmt.Fprintf(b, "  Medido: %s\n", measured)
	}
	if item.Reason == reasonResolved && alert.ResolvedAt != nil {
		fmt.Fprintf(b, "  Resolvido em: %s\n", alert.ResolvedAt.In(brt).Format("02/01/2006 15:04"))
	}
}

// sampleItems são os alertas de exemplo da pré-visualização do layout.
func sampleItems(now time.Time) []notifyItem {
	resolvedAt := now
	mk := func(id int64, rule, severity, area, title, measured string, ago time.Duration) alertRow {
		return alertRow{ID: id, Rule: rule, Severity: severity, Area: area, Title: title,
			Detail: map[string]any{"measured": measured}, Status: "open", FirstSeen: now.Add(-ago), LastSeen: now}
	}
	resolved := mk(4, "slow_route", sevMedium, "api", "Rota lenta: GET /v1/exemplo", "p95 de 2400 ms em 35 requisições (15 min)", 3*time.Hour)
	resolved.Status, resolved.ResolvedAt = "resolved", &resolvedAt
	return []notifyItem{
		{Alert: mk(1, "api_5xx_rate", sevCritical, "api", "Taxa de erros 5xx da API alta", "24.0% de 5xx (12 de 50 requisições em 5 min)", 10*time.Minute), Reason: reasonOpened},
		{Alert: mk(2, "disk_low", sevHigh, "containers", "Pouco espaço em disco: /", "91% usado", 2*time.Hour), Reason: reasonEscalated},
		{Alert: mk(3, "job_backlog", sevMedium, "workers", "Fila com atraso: exemplo_jobs", "job mais antigo atrasado 420 s (8 vencidos)", 5*time.Hour), Reason: reasonRenotify},
		{Alert: resolved, Reason: reasonResolved},
	}
}

// previewEmail devolve o primeiro e-mail do layout proposto, com alertas de exemplo.
// No modo "um e-mail por alerta" mostra o primeiro e informa quantos seriam enviados.
func previewEmail(layout EmailSettings, link string, now time.Time) outgoingEmail {
	mails := buildEmails(sampleItems(now), link, layout)
	if len(mails) == 0 {
		return outgoingEmail{Subject: subjectWithPrefix(layout.SubjectPrefix, "(nenhum e-mail com estas opções)"),
			Body: "Nenhum e-mail seria enviado para os alertas de exemplo.\n"}
	}
	first := mails[0]
	if len(mails) > 1 {
		first.Body += fmt.Sprintf("\n-- Pré-visualização: com este layout seriam enviados %d e-mails (um por alerta); este é o primeiro. --\n", len(mails))
	}
	return first
}
