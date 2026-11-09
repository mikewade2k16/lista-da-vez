package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestPortugueseRetrievalHeldOutEvaluation(t *testing.T) {
	s, sc, p, _ := fixture(t)
	ctx := context.Background()
	var dataset struct {
		Documents []struct {
			ID, Body, From string
			To             *string
			Source         string
		}
		DevelopmentQueries []string
		Holdout            []struct {
			ID, Query, CompetenceDate, Jurisdiction, Regime string
			Expected                                        []string
		}
	}
	rawDataset, e := os.ReadFile("../../../../docs/accounting/evaluations/CT-12-dataset.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(rawDataset, &dataset); e != nil {
		t.Fatal(e)
	}
	base, e := s.CreateBase(ctx, sc, "eval-"+sc.ResourceID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SetBase(ctx, sc, base.ID, base.Version, true); e != nil {
		t.Fatal(e)
	}
	ids := map[string]string{}
	for _, doc := range dataset.Documents {
		in := material(base.ID)
		in.Title = "Synthetic " + doc.ID
		in.Content = []byte(doc.Body)
		in.SourceRef = "manual://synthetic/" + doc.Source
		in.Metadata.ValidFrom = ptr(doc.From)
		in.Metadata.ValidTo = doc.To
		d := published(t, s, sc, in)
		ids[d.ID] = doc.ID
	}
	// Portuguese is evaluated offline against precisely the same authorization and
	// validity joins. No production index/configuration or query expansion changes.
	candidate := strings.ReplaceAll(strings.ReplaceAll(fiscalSearchSQL, "plainto_tsquery('simple',$3)", "plainto_tsquery('portuguese',$3)"), "c.search_vector", "to_tsvector('portuguese',c.body_text)")
	type sample struct {
		ID           string   `json:"id"`
		Expected     []string `json:"expected"`
		Found        []string `json:"found"`
		Milliseconds float64  `json:"milliseconds"`
	}
	type metrics struct {
		Mode      string   `json:"mode"`
		RecallAt5 float64  `json:"recallAt5"`
		P50MS     float64  `json:"p50Ms"`
		P95MS     float64  `json:"p95Ms"`
		Samples   []sample `json:"samples"`
	}
	reports := []metrics{}
	for _, mode := range []struct{ name, sql string }{{"fts_simple", fiscalSearchSQL}, {"portuguese_offline_candidate", candidate}} {
		m := metrics{Mode: mode.name, Samples: []sample{}}
		hits, total := 0, 0
		latencies := []float64{}
		for _, q := range dataset.Holdout {
			found := []string{}
			start := time.Now()
			rows, e := p.Query(ctx, mode.sql, sc.AccountID, sc.ResourceID, q.Query, q.CompetenceDate, q.Jurisdiction, q.Regime, 5, 0)
			if e != nil {
				t.Fatal(e)
			}
			for rows.Next() {
				v, e := rows.Values()
				if e != nil {
					rows.Close()
					t.Fatal(e)
				}
				id, ok := v[0].(string)
				if !ok {
					rows.Close()
					t.Fatal("unexpected document ID")
				}
				found = append(found, ids[id])
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				t.Fatal(e)
			}
			ms := float64(time.Since(start).Microseconds()) / 1000
			latencies = append(latencies, ms)
			m.Samples = append(m.Samples, sample{ID: q.ID, Expected: q.Expected, Found: found, Milliseconds: ms})
			if len(q.Expected) == 0 && len(found) > 0 {
				t.Fatal("filter/negative case leaked", q.ID, found)
			}
			for _, expected := range q.Expected {
				total++
				for _, got := range found {
					if got == expected {
						hits++
						break
					}
				}
			}
		}
		if total > 0 {
			m.RecallAt5 = float64(hits) / float64(total)
		}
		sort.Float64s(latencies)
		m.P50MS = latencies[int(math.Ceil(float64(len(latencies))*.5))-1]
		m.P95MS = latencies[int(math.Ceil(float64(len(latencies))*.95))-1]
		reports = append(reports, m)
	}
	hash := sha256.Sum256(rawDataset)
	report := map[string]any{"datasetSHA256": hex.EncodeToString(hash[:]), "documents": len(dataset.Documents), "developmentQuestions": len(dataset.DevelopmentQueries), "holdoutQuestions": len(dataset.Holdout), "topK": 5, "results": reports, "realLLMCalls": 0, "embeddingCalls": 0, "embeddingCost": nil, "decision": "retain_fts_simple; Portuguese candidate is offline only, no production-scale/index or embeddings benchmark"}
	if output := os.Getenv("KNOWLEDGE_EVAL_OUTPUT"); output != "" {
		b, e := json.MarshalIndent(report, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(output, append(b, '\n'), 0o600); e != nil {
			t.Fatal(e)
		}
	}
	t.Logf("simple recall %.3f; Portuguese candidate %.3f", reports[0].RecallAt5, reports[1].RecallAt5)
}
