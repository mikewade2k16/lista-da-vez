package observability

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

const (
	verdictOK     = "ok"
	verdictSlow   = "slow"
	verdictFailed = "failed"

	deployReferenceSize = 10  // últimos sucessos do mesmo comando usados como referência
	deployMinReference  = 3   // mínimo de sucessos para julgar "lento"
	deploySlowFactor    = 1.5 // lento = duração > 1,5 × mediana
	deployFetchMin      = 50  // janela mínima lida do banco para calcular referências
)

type deployStats struct {
	Count           int        `json:"count"`
	SuccessRate     float64    `json:"successRate"`
	MedianMs        float64    `json:"medianMs"`
	P90Ms           float64    `json:"p90Ms"`
	SlowThresholdMs float64    `json:"slowThresholdMs"`
	LastSuccessAt   *time.Time `json:"lastSuccessAt"`
}

// Deploys lista os últimos deploys com veredito e as estatísticas de referência.
func (s *Service) Deploys(ctx context.Context, limit int) (map[string]any, error) {
	rows, err := s.store.RecentDeploys(ctx, max(limit, deployFetchMin))
	if err != nil {
		return nil, err
	}
	applyVerdicts(rows)
	stats := computeDeployStats(rows)
	if len(rows) > limit {
		rows = rows[:limit]
	}
	if rows == nil {
		rows = []deployRow{}
	}
	return map[string]any{"items": rows, "stats": stats}, nil
}

// successDurations devolve as durações dos últimos deploys com sucesso do comando
// (rows em ordem do mais recente ao mais antigo), no máximo deployReferenceSize.
func successDurations(rows []deployRow, command string) []float64 {
	var out []float64
	for _, row := range rows {
		if len(out) == deployReferenceSize {
			break
		}
		if row.Command == command && row.Status == "success" && row.DurationMs != nil {
			out = append(out, *row.DurationMs)
		}
	}
	return out
}

// applyVerdicts preenche o veredito de cada deploy: falhou (qualquer status que não
// seja sucesso), lento (> 1,5 × mediana com ao menos 3 referências) ou ok.
func applyVerdicts(rows []deployRow) {
	medians := map[string]float64{}
	enough := map[string]bool{}
	for _, row := range rows {
		if _, seen := medians[row.Command]; seen {
			continue
		}
		ref := successDurations(rows, row.Command)
		medians[row.Command] = percentile(ref, 0.5)
		enough[row.Command] = len(ref) >= deployMinReference
	}
	for index := range rows {
		row := &rows[index]
		switch {
		case row.Status != "success":
			row.Verdict = verdictFailed
		case enough[row.Command] && row.DurationMs != nil && *row.DurationMs > deploySlowFactor*medians[row.Command]:
			row.Verdict = verdictSlow
		default:
			row.Verdict = verdictOK
		}
	}
}

// computeDeployStats: taxa de sucesso sobre todos os deploys lidos; mediana e p90
// sobre os últimos sucessos do mesmo comando do deploy mais recente.
func computeDeployStats(rows []deployRow) deployStats {
	stats := deployStats{Count: len(rows)}
	if len(rows) == 0 {
		return stats
	}
	success := 0
	for _, row := range rows {
		if row.Status == "success" {
			success++
			if stats.LastSuccessAt == nil {
				at := row.StartedAt
				if row.FinishedAt != nil {
					at = *row.FinishedAt
				}
				stats.LastSuccessAt = &at
			}
		}
	}
	stats.SuccessRate = round1(float64(success)/float64(len(rows))*1000) / 1000
	ref := successDurations(rows, rows[0].Command)
	stats.MedianMs = math.Round(percentile(ref, 0.5))
	stats.P90Ms = math.Round(percentile(ref, 0.9))
	if len(ref) >= deployMinReference {
		stats.SlowThresholdMs = math.Round(deploySlowFactor * percentile(ref, 0.5))
	}
	return stats
}

// percentile usa interpolação linear; lista vazia devolve 0.
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	pos := p * float64(len(sorted)-1)
	low := int(math.Floor(pos))
	high := int(math.Ceil(pos))
	return sorted[low] + (sorted[high]-sorted[low])*(pos-float64(low))
}

func formatDeployDuration(ms float64) string {
	seconds := int(math.Round(ms / 1000))
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	return fmt.Sprintf("%dmin%02ds", seconds/60, seconds%60)
}

func formatAge(age time.Duration) string {
	switch {
	case age < time.Minute:
		return "agora há pouco"
	case age < time.Hour:
		return fmt.Sprintf("há %d min", int(age.Minutes()))
	case age < 48*time.Hour:
		return fmt.Sprintf("há %d h", int(age.Hours()))
	}
	return fmt.Sprintf("há %d d", int(age.Hours()/24))
}

// deployArea resume o último deploy: falhou (crítico se recente, aviso depois de
// 24 h), lento (aviso) ou ok. Sem deploys registrados é "unknown", nunca "ok".
func deployArea(in healthInputs) healthArea {
	area := healthArea{Key: "deploy", Label: "Deploy", Status: statusUnknown, Summary: "nenhum deploy registrado"}
	if len(in.Deploys) == 0 {
		return area
	}
	rows := append([]deployRow(nil), in.Deploys...)
	applyVerdicts(rows)
	last := rows[0]
	duration := "sem duração"
	if last.DurationMs != nil {
		duration = formatDeployDuration(*last.DurationMs)
	}
	label := map[string]string{verdictOK: "ok", verdictSlow: "lento", verdictFailed: "falhou"}[last.Verdict]
	area.Summary = fmt.Sprintf("último: %s, %s, %s", duration, label, formatAge(in.Now.Sub(last.StartedAt)))
	switch last.Verdict {
	case verdictOK:
		area.Status = statusOK
	case verdictSlow:
		area.Status = statusWarn
	default:
		area.Status = statusWarn
		if in.Now.Sub(last.StartedAt) <= 24*time.Hour {
			area.Status = statusCritical
		}
	}
	return area
}

// deployIssue devolve o problema do último deploy (falhou: high; lento: medium).
func deployIssue(in healthInputs) *healthIssue {
	if len(in.Deploys) == 0 {
		return nil
	}
	rows := append([]deployRow(nil), in.Deploys...)
	applyVerdicts(rows)
	last := rows[0]
	if last.Verdict == verdictOK {
		return nil
	}
	issue := healthIssue{Area: "deploy", Since: last.StartedAt, Link: issueLink{Tab: "deploys", Query: map[string]any{"id": last.ID}}}
	if last.Verdict == verdictFailed {
		issue.Severity = sevHigh
		issue.Title = fmt.Sprintf("Último deploy (%s) não terminou com sucesso", last.Command)
		issue.Detail = fmt.Sprintf("status %s", last.Status)
		return &issue
	}
	issue.Severity = sevMedium
	issue.Title = fmt.Sprintf("Último deploy (%s) foi lento", last.Command)
	if last.DurationMs != nil {
		issue.Detail = fmt.Sprintf("%s, mediana %s", formatDeployDuration(*last.DurationMs), formatDeployDuration(percentile(successDurations(rows, last.Command), 0.5)))
	}
	return &issue
}
