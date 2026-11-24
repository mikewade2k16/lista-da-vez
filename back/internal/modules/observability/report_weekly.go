package observability

import (
	"fmt"
	"sort"
	"strings"
	"time"

	platformobs "github.com/mikewade2k16/lista-da-vez/back/internal/platform/observability"
)

const (
	weeklyCatchUp     = 24 * time.Hour   // depois disso sem envio, espera a próxima semana
	weeklyRetryAfter  = 15 * time.Minute // intervalo entre tentativas que falharam
	weeklyMaxListed   = 5
	weeklyMaxImproved = 3
)

// compareRoutes devolve as rotas que pioraram (maior aumento de p95 primeiro) e as que
// melhoraram, entre a semana anterior e a atual. Só rotas com volume nas duas semanas e
// variação relevante (>= 20% e >= 50 ms).
func compareRoutes(cur, prev map[string]routeWeek) (worse, better []routeDelta) {
	for route, now := range cur {
		before, ok := prev[route]
		if !ok || now.Calls < routeMinCalls || before.Calls < routeMinCalls {
			continue
		}
		delta := routeDelta{
			Route:   route,
			CurP95:  platformobs.PercentileFromBuckets(now.Buckets, 0.95),
			PrevP95: platformobs.PercentileFromBuckets(before.Buckets, 0.95),
			Calls:   now.Calls,
		}
		diff := delta.CurP95 - delta.PrevP95
		switch {
		case diff >= routeMinDeltaMs && delta.CurP95 >= delta.PrevP95*(1+routeMinChange):
			worse = append(worse, delta)
		case -diff >= routeMinDeltaMs && delta.CurP95 <= delta.PrevP95*(1-routeMinChange):
			better = append(better, delta)
		}
	}
	sort.Slice(worse, func(i, j int) bool {
		a, b := worse[i], worse[j]
		if a.CurP95-a.PrevP95 != b.CurP95-b.PrevP95 {
			return a.CurP95-a.PrevP95 > b.CurP95-b.PrevP95
		}
		return a.Route < b.Route
	})
	sort.Slice(better, func(i, j int) bool {
		a, b := better[i], better[j]
		if a.PrevP95-a.CurP95 != b.PrevP95-b.CurP95 {
			return a.PrevP95-a.CurP95 > b.PrevP95-b.CurP95
		}
		return a.Route < b.Route
	})
	return truncateDeltas(worse, weeklyMaxListed), truncateDeltas(better, weeklyMaxImproved)
}

func truncateDeltas(list []routeDelta, limit int) []routeDelta {
	if len(list) > limit {
		return list[:limit]
	}
	return list
}

// weeklyDue decide se o relatório semanal deve sair agora: ligado, depois do dia/hora
// da semana corrente (no fuso configurado), dentro de 24 h desse horário (recuperação
// de uma API fora do ar), ainda não enviado nesta semana e fora do intervalo entre
// tentativas que falharam.
func weeklyDue(cfg WeeklySettings, state weeklyState, now time.Time) bool {
	if !cfg.Enabled {
		return false
	}
	loc, err := loadZone(cfg.Timezone)
	if err != nil {
		return false
	}
	scheduled := lastScheduled(cfg, now.In(loc))
	if now.Before(scheduled) || now.Sub(scheduled) >= weeklyCatchUp {
		return false
	}
	if state.LastSentAt != nil && !state.LastSentAt.Before(scheduled) {
		return false
	}
	if state.LastAttemptAt != nil && now.Sub(*state.LastAttemptAt) < weeklyRetryAfter {
		return false
	}
	return true
}

// lastScheduled é o último "dia da semana + hora cheia" que já passou (ou é agora).
func lastScheduled(cfg WeeklySettings, local time.Time) time.Time {
	back := (int(local.Weekday()) - cfg.Weekday + 7) % 7
	at := time.Date(local.Year(), local.Month(), local.Day()-back, cfg.Hour, 0, 0, 0, local.Location())
	if at.After(local) {
		at = time.Date(local.Year(), local.Month(), local.Day()-back-7, cfg.Hour, 0, 0, 0, local.Location())
	}
	return at
}

func pctChange(current, previous float64) string {
	if previous <= 0 {
		return "sem base de comparação"
	}
	change := (current - previous) / previous * 100
	return fmt.Sprintf("%+.0f%%", change)
}

func formatRate(errors, total int64) string {
	if total == 0 {
		return "sem requisições"
	}
	return fmt.Sprintf("%.2f%% (%d de %d)", float64(errors)/float64(total)*100, errors, total)
}

var weeklySeverities = []string{sevCritical, sevHigh, sevMedium, sevLow}

func severityCounts(counts map[string]int64) string {
	var parts []string
	for _, severity := range weeklySeverities {
		if counts[severity] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[severity], strings.ToLower(severityLabel(severity))))
		}
	}
	if len(parts) == 0 {
		return "nenhum"
	}
	return strings.Join(parts, ", ")
}

func sumCounts(counts map[string]int64) int64 {
	var total int64
	for _, value := range counts {
		total += value
	}
	return total
}

func ruleLabel(rule string) string {
	if item, ok := catalogItem(rule); ok {
		return item.Label
	}
	return rule
}

// buildWeeklyReport monta assunto e texto simples do relatório. Função pura.
func buildWeeklyReport(data weeklyData, prefix, link string, loc *time.Location) (subject, text string) {
	day := func(at time.Time) string { return at.In(loc).Format("02/01") }
	from, to := data.From, data.To
	subject = subjectWithPrefix(prefix, fmt.Sprintf("Relatório semanal - %s a %s", day(from), day(to.Add(-time.Second))))

	var b strings.Builder
	fmt.Fprintf(&b, "Relatório semanal do Omni.\nPeríodo: %s a %s (comparado com %s a %s).\n",
		day(from), day(to.Add(-time.Second)), day(from.AddDate(0, 0, -7)), day(from.Add(-time.Second)))

	b.WriteString("\nDESEMPENHO DA API\n")
	if data.Cur.Calls == 0 && data.Cur.LiveTotal == 0 {
		b.WriteString("- Sem dados de requisições na semana.\n")
	} else {
		fmt.Fprintf(&b, "- p95: %.0f ms (semana anterior: %.0f ms, %s)\n", data.Cur.P95Ms, data.Prev.P95Ms, pctChange(data.Cur.P95Ms, data.Prev.P95Ms))
		fmt.Fprintf(&b, "- Erros 5xx: %s; semana anterior: %s\n", formatRate(data.Cur.Live5xx, data.Cur.LiveTotal), formatRate(data.Prev.Live5xx, data.Prev.LiveTotal))
		fmt.Fprintf(&b, "- Chamadas registradas: %d (semana anterior: %d)\n", data.Cur.Calls, data.Prev.Calls)
	}

	b.WriteString("\nROTAS QUE PIORARAM (p95)\n")
	writeDeltas(&b, data.Worse, "Nenhuma rota piorou de forma relevante.")
	if len(data.Better) > 0 {
		b.WriteString("\nROTAS QUE MELHORARAM (p95)\n")
		writeDeltas(&b, data.Better, "")
	}

	b.WriteString("\nALERTAS\n")
	fmt.Fprintf(&b, "- Abertos na semana: %d (%s)\n", sumCounts(data.AlertsOpened), severityCounts(data.AlertsOpened))
	fmt.Fprintf(&b, "- Resolvidos na semana: %d; abertos agora: %d\n", data.AlertsResolved, data.AlertsOpenNow)
	for _, item := range data.AlertsTopRules {
		fmt.Fprintf(&b, "  - %s: %d vez(es)\n", ruleLabel(item.Name), item.Count)
	}

	b.WriteString("\nACHADOS DE SEGURANÇA\n")
	fmt.Fprintf(&b, "- Abertos: %d (%s)\n", sumCounts(data.FindingsOpen), severityCounts(data.FindingsOpen))
	fmt.Fprintf(&b, "- Com prazo vencido: %d\n", data.FindingsOverdue)

	b.WriteString("\nDEPLOYS\n")
	if data.Deploys.Count == 0 {
		b.WriteString("- Nenhum deploy na semana.\n")
	} else {
		fmt.Fprintf(&b, "- %d deploy(s), %d com falha; duração mediana %s\n", data.Deploys.Count, data.Deploys.Failed, formatDeployDuration(data.Deploys.MedianMs))
	}

	b.WriteString("\nÚLTIMA EXECUÇÃO COMPLETA DO RUNNER\n")
	writeRunNotes(&b, data.Run, loc)

	if link != "" {
		fmt.Fprintf(&b, "\nPainel: %s\n", link)
	}
	return subject, b.String()
}

func writeDeltas(b *strings.Builder, list []routeDelta, empty string) {
	if len(list) == 0 {
		if empty != "" {
			fmt.Fprintf(b, "- %s\n", empty)
		}
		return
	}
	for index, item := range list {
		fmt.Fprintf(b, "%d. %s: %.0f ms -> %.0f ms (%s, %d chamadas)\n", index+1, item.Route, item.PrevP95, item.CurP95, pctChange(item.CurP95, item.PrevP95), item.Calls)
	}
}

func writeRunNotes(b *strings.Builder, run *runNotes, loc *time.Location) {
	if run == nil {
		b.WriteString("- Nenhuma execução completa registrada.\n")
		return
	}
	fmt.Fprintf(b, "- %s (%s)\n", run.ID, run.At.In(loc).Format("02/01 15:04"))
	if len(run.Scores) == 0 {
		b.WriteString("- Sem notas medidas nesta execução.\n")
		return
	}
	var sum float64
	for _, item := range run.Scores {
		sum += item.Score
	}
	fmt.Fprintf(b, "- Nota média: %.0f em %d módulo(s). Menores notas:\n", sum/float64(len(run.Scores)), len(run.Scores))
	scores := append([]moduleScore{}, run.Scores...)
	sort.SliceStable(scores, func(i, j int) bool { return scores[i].Score < scores[j].Score })
	for _, item := range truncateScores(scores, weeklyMaxListed) {
		fmt.Fprintf(b, "  - %s: %.0f\n", item.Module, item.Score)
	}
}

func truncateScores(list []moduleScore, limit int) []moduleScore {
	if len(list) > limit {
		return list[:limit]
	}
	return list
}
