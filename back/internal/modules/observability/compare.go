package observability

import (
	"context"
	"math"
	"sort"
)

// Métricas comparadas entre execuções; em todas, valor maior é pior.
// minDelta evita apontar regressão em ruído (ex.: 3 ms → 5 ms).
var comparedMetrics = map[string]float64{
	"p50Ms": 20, "p95Ms": 20, "p99Ms": 30, "avgMs": 20, "loadMs": 200,
	"lcp": 200, "inp": 50, "ttfb": 100, "cls": 0.02,
	"queries": 2, "consoleErrors": 1, "failedRequests": 1, "errorRate": 0.01,
}

const regressionRatio = 0.2

type Change struct {
	Kind     string  `json:"kind"`
	Target   string  `json:"target"`
	Module   string  `json:"module"`
	Metric   string  `json:"metric"`
	Before   any     `json:"before"`
	After    any     `json:"after"`
	DeltaPct float64 `json:"deltaPct"`
	Verdict  string  `json:"verdict"`
}

func (s *Service) Compare(ctx context.Context, a, b string) (map[string]any, error) {
	runA, resultsA, err := s.store.Run(ctx, a)
	if err != nil {
		return nil, err
	}
	runB, resultsB, err := s.store.Run(ctx, b)
	if err != nil {
		return nil, err
	}
	return map[string]any{"a": runA, "b": runB, "changes": diffResults(resultsA, resultsB)}, nil
}

func resultKey(item map[string]any) string {
	kind, _ := item["kind"].(string)
	target, _ := item["target"].(string)
	return kind + "|" + target
}

func diffResults(before, after []map[string]any) []Change {
	index := make(map[string]map[string]any, len(before))
	for _, item := range before {
		index[resultKey(item)] = item
	}
	changes := make([]Change, 0)
	for _, current := range after {
		previous, ok := index[resultKey(current)]
		if !ok {
			continue
		}
		kind, _ := current["kind"].(string)
		target, _ := current["target"].(string)
		module, _ := current["module"].(string)
		base := Change{Kind: kind, Target: target, Module: module}

		statusBefore, _ := previous["status"].(string)
		statusAfter, _ := current["status"].(string)
		if statusBefore != statusAfter && statusBefore != "skip" && statusAfter != "skip" {
			change := base
			change.Metric, change.Before, change.After = "status", statusBefore, statusAfter
			change.Verdict = statusVerdict(statusBefore, statusAfter)
			changes = append(changes, change)
		}
		metricsBefore, _ := previous["metrics"].(map[string]any)
		metricsAfter, _ := current["metrics"].(map[string]any)
		for metric, minDelta := range comparedMetrics {
			old, okOld := toFloat(metricsBefore[metric])
			now, okNow := toFloat(metricsAfter[metric])
			if !okOld || !okNow {
				continue
			}
			verdict, pct := metricVerdict(old, now, minDelta)
			if verdict == "same" {
				continue
			}
			change := base
			change.Metric, change.Before, change.After, change.DeltaPct, change.Verdict = metric, old, now, pct, verdict
			changes = append(changes, change)
		}
	}
	sort.SliceStable(changes, func(i, j int) bool {
		if changes[i].Verdict != changes[j].Verdict {
			return changes[i].Verdict == "regression"
		}
		return math.Abs(changes[i].DeltaPct) > math.Abs(changes[j].DeltaPct)
	})
	return changes
}

func statusVerdict(before, after string) string {
	rank := map[string]int{"pass": 0, "warn": 1, "fail": 2}
	if rank[after] > rank[before] {
		return "regression"
	}
	return "improvement"
}

func metricVerdict(before, after, minDelta float64) (string, float64) {
	delta := after - before
	pct := 0.0
	if before != 0 {
		pct = math.Round(delta/before*1000) / 10
	}
	if math.Abs(delta) < minDelta || (before != 0 && math.Abs(delta/before) < regressionRatio) {
		return "same", pct
	}
	if delta > 0 {
		return "regression", pct
	}
	return "improvement", pct
}

func toFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int64:
		return float64(typed), true
	case int:
		return float64(typed), true
	default:
		return 0, false
	}
}
