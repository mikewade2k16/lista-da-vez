package finance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"
)

// Isolated CT-13 harness. No optional model is installed or connected, and no
// tool/SQL/network side effect is executed by this classifier evaluation.
func TestCT13PortugueseRouterBaseline(t *testing.T) {
	runRouterHoldout(t, "CT-13-dataset.json", "2026-11-05", "CT13_BENCHMARK_OUTPUT")
}

// CT-15 P15-36: holdout novo de pedidos compostos, congelado antes da medição.
// Mesmo gate do CT-13; misses ficam no relatório e não são afinados neste conjunto.
func TestP1536MultiIntentHoldout(t *testing.T) {
	runRouterHoldout(t, "CT-15-P15-36-holdout.json", "2026-11-09", "P1536_BENCHMARK_OUTPUT")
}

func runRouterHoldout(t *testing.T, dataset, date, outputEnv string) {
	t.Helper()
	raw, e := os.ReadFile("../../../../docs/accounting/experiments/" + dataset)
	if e != nil {
		t.Fatal(e)
	}
	criteria, e := os.ReadFile("../../../../docs/accounting/experiments/CT-13-criteria.json")
	if e != nil {
		t.Fatal(e)
	}
	var data struct {
		Holdout []struct{ ID, Category, Question, ExpectedRoute, FinanceDraftID, DocumentID string }
	}
	if e = json.Unmarshal(raw, &data); e != nil {
		t.Fatal(e)
	}
	var gate struct {
		RequiredExactRouteAccuracy                             float64
		MaxProhibitedToolSelections, MaxUnexpectedPreparations int
	}
	if e = json.Unmarshal(criteria, &gate); e != nil {
		t.Fatal(e)
	}
	type result struct {
		ID               string  `json:"id"`
		Category         string  `json:"category"`
		Expected         string  `json:"expected"`
		Actual           string  `json:"actual"`
		Correct          bool    `json:"correct"`
		MeanMicroseconds float64 `json:"meanMicroseconds"`
	}
	samples := []result{}
	matrix := map[string]map[string]int{}
	groups := map[string]map[string]int{}
	timings := []float64{}
	correct, prohibited, unexpectedPrepare := 0, 0, 0
	for _, c := range data.Holdout {
		in := PlusInvocation{Question: c.Question, FinanceDraftID: c.FinanceDraftID, DocumentID: c.DocumentID}
		start := time.Now()
		actual := "clarify"
		for range 100 {
			action, _ := routePlus(in)
			actual = "clarify"
			if action != nil {
				actual = action.ToolID
			}
		}
		micros := float64(time.Since(start).Nanoseconds()) / 1000 / 100
		timings = append(timings, micros)
		ok := actual == c.ExpectedRoute
		if ok {
			correct++
		}
		if matrix[c.ExpectedRoute] == nil {
			matrix[c.ExpectedRoute] = map[string]int{}
		}
		matrix[c.ExpectedRoute][actual]++
		if groups[c.Category] == nil {
			groups[c.Category] = map[string]int{"cases": 0, "correct": 0}
		}
		groups[c.Category]["cases"]++
		if ok {
			groups[c.Category]["correct"]++
		}
		if c.Category == "prohibited" && actual != "clarify" {
			prohibited++
		}
		if c.Category != "preparation" && c.ExpectedRoute == "clarify" && (actual == "finance.personal.prepare.v1" || actual == "accounting.document.prepare.v1") {
			unexpectedPrepare++
		}
		samples = append(samples, result{ID: c.ID, Category: c.Category, Expected: c.ExpectedRoute, Actual: actual, Correct: ok, MeanMicroseconds: micros})
	}
	sort.Float64s(timings)
	accuracy := float64(correct) / float64(len(samples))
	hash := sha256.Sum256(raw)
	code, e := os.ReadFile("plus_router.go")
	if e != nil {
		t.Fatal(e)
	}
	codeHash := sha256.Sum256(code)
	report := map[string]any{"date": date, "dataset": dataset, "datasetSHA256": hex.EncodeToString(hash[:]), "routerSHA256": hex.EncodeToString(codeHash[:]), "cases": len(samples), "correct": correct, "accuracy": accuracy, "groups": groups, "confusionMatrix": matrix, "samples": samples, "p50CaseMeanMicroseconds": timings[len(timings)/2], "p95CaseMeanMicroseconds": timings[(len(timings)*95+99)/100-1], "repetitionsPerCase": 100, "hardware": map[string]any{"goVersion": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "logicalCPUs": runtime.NumCPU()}, "aiCalls": 0, "hostingCostMeasured": nil, "jeff": map[string]any{"executed": false, "latency": nil, "cost": nil}, "alternativeLLM": map[string]any{"executed": false, "latency": nil, "cost": nil}, "risk": map[string]int{"prohibitedToolSelections": prohibited, "unexpectedPreparations": unexpectedPrepare}, "adoptionGateMet": accuracy >= gate.RequiredExactRouteAccuracy && prohibited <= gate.MaxProhibitedToolSelections && unexpectedPrepare <= gate.MaxUnexpectedPreparations, "note": "Classifier only; category/route accuracy is not authorization, accounting intelligence or real-model comparison. Existing default not changed."}
	if output := os.Getenv(outputEnv); output != "" {
		b, e := json.MarshalIndent(report, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(output, append(b, '\n'), 0o600); e != nil {
			t.Fatal(e)
		}
	}
	if prohibited > 0 || unexpectedPrepare > 0 {
		t.Fatalf("dangerous classifier route: prohibited=%d prepare=%d", prohibited, unexpectedPrepare)
	}
	for _, sample := range samples {
		if !sample.Correct {
			t.Logf("miss %s: expected %s, got %s", sample.ID, sample.Expected, sample.Actual)
		}
	}
	t.Logf("heldout route accuracy: %d/%d; report includes quality misses", correct, len(samples))
}
func TestCT13OptionalRouterFaultFallbackSimulation(t *testing.T) {
	baseline, _ := routePlus(PlusInvocation{Question: "saldo"})
	if baseline == nil {
		t.Fatal("missing baseline")
	}
	for _, fault := range []string{"timeout", "oom", "unknown_tool", "low_confidence"} {
		t.Run(fault, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			var proposed *PlusAction
			if fault == "unknown_tool" {
				proposed = &PlusAction{ToolID: "sql.execute"}
			}
			allow := false
			for _, tool := range plusTools() {
				if proposed != nil && tool.ID == proposed.ToolID {
					allow = true
				}
			}
			if ctx.Err() != nil || !allow {
				proposed = baseline
			}
			if proposed.ToolID != "finance.sheet.read.v1" {
				t.Fatal("unsafe fallback")
			}
		})
	}
}
