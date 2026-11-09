package knowledge

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/finance"
)

// Coleta real do catálogo (download público das fontes oficiais) num banco
// descartável, seguida de medição da busca no conteúdo coletado. Opt-in:
// PUBLIC_COLLECT_LIVE=1 e poppler (pdftotext) no ambiente. A aprovação/publicação
// aqui é automática SÓ para medir no banco descartável; na conta real quem publica é o dono.
func TestPublicCatalogLiveCollectionAndRetrieval(t *testing.T) {
	if os.Getenv("PUBLIC_COLLECT_LIVE") != "1" {
		t.Skip("set PUBLIC_COLLECT_LIVE=1 to download the official catalog")
	}
	s, p, actor := publicCurationService(t)
	s.ports.ExtractPublicPDF = finance.ExtractPublicPDF
	ctx := context.Background()
	report := map[string]any{"date": time.Now().UTC().Format(time.RFC3339), "database": "disposable accounting_test_*", "aiCalls": 0,
		"note": "publicação automática apenas no banco descartável para medir a busca; na conta real exige aprovação do dono"}
	collections := map[string]any{}
	for _, entry := range PublicCatalog() {
		attempt, err := s.CollectPublic(ctx, actor, entry.Key)
		if err != nil {
			t.Fatal(entry.Key, err)
		}
		item := map[string]any{"outcome": attempt.Outcome, "errorCode": attempt.ErrorCode}
		if attempt.CollectionID != "" {
			c, err := s.store.publicCollection(ctx, attempt.CollectionID)
			if err != nil {
				t.Fatal(err)
			}
			item["version"], item["chunks"], item["pages"], item["bytes"], item["sha256"] = c.Version, c.ChunkCount, c.Pages, c.ByteSize, c.SHA256
			if c.Status == "collected" {
				if _, err = s.ReviewPublicCollection(ctx, actor, c.ID, PublicReviewInput{ReviewerKind: "owner", ReviewerName: "avaliação automática (banco descartável)", Verdict: "approved"}); err != nil {
					t.Fatal(err)
				}
				if _, err = s.PublishPublicCollection(ctx, actor, c.ID); err != nil {
					t.Fatal(err)
				}
			}
		}
		collections[entry.Key] = item
		t.Logf("%s: %v", entry.Key, item)
	}
	report["collections"] = collections

	raw, err := os.ReadFile("../../../../docs/accounting/experiments/CT-15-BASE-PUBLICA-holdout.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Cases []struct {
			ID     string   `json:"id"`
			Terms  []string `json:"terms"`
			Accept []string `json:"accept"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	curated := map[string]string{"mei-limite": "mei-teto-memp", "mei-limite-atualizacao": "mei-teto-memp", "mei-dasn": "mei-dasn-servico", "nfse-emissores": "nfse-guia-emissor-web", "simples-conceito": "simples-perguntao-sn"}
	keyOf := func(ids []string) []string {
		rows, err := p.Query(ctx, `select coalesce(c.catalog_key,p.source_key) from unnest($1::uuid[]) with ordinality u(id,n) join knowledge.public_sources p on p.id=u.id left join knowledge.public_collections c on c.id=p.collection_id order by u.n`, ids)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := []string{}
		for rows.Next() {
			var k string
			if err = rows.Scan(&k); err != nil {
				t.Fatal(err)
			}
			if alias := curated[k]; alias != "" {
				k = alias
			}
			out = append(out, k)
		}
		return out
	}
	baseline := func(terms []string) []string {
		rows, err := p.Query(ctx, `select p.id::text from knowledge.public_sources p where p.status='published' and p.reviewed_at is not null and p.search_vector@@to_tsquery('simple',$1)
 and not exists(select 1 from knowledge.public_sources n where n.source_key=p.source_key and n.status='published' and n.version>p.version)
 order by ts_rank_cd(p.search_vector,to_tsquery('simple',$1)) desc,p.source_key limit 3`, strings.Join(terms, " | "))
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		ids := []string{}
		for rows.Next() {
			var id string
			_ = rows.Scan(&id)
			ids = append(ids, id)
		}
		return ids
	}
	hit := func(keys, accept []string) bool {
		if len(accept) == 0 {
			return len(keys) == 0
		}
		for _, k := range keys {
			for _, a := range accept {
				if k == a {
					return true
				}
			}
		}
		return false
	}
	results := []map[string]any{}
	score := map[string]int{"baseline": 0, "concepts": 0}
	for _, c := range data.Cases {
		baseKeys := keyOf(baseline(c.Terms))
		out, err := s.SearchPublic(ctx, c.Terms)
		if err != nil {
			t.Fatal(c.ID, err)
		}
		ids := []string{}
		for _, e := range out.Evidence {
			ids = append(ids, e.ChunkID)
		}
		newKeys := keyOf(ids)
		b, n := hit(baseKeys, c.Accept), hit(newKeys, c.Accept)
		if b {
			score["baseline"]++
		}
		if n {
			score["concepts"]++
		}
		results = append(results, map[string]any{"id": c.ID, "baseline": baseKeys, "baselineHit": b, "concepts": newKeys, "conceptsHit": n})
		if !n {
			t.Logf("miss %s: got %v (baseline %v)", c.ID, newKeys, baseKeys)
		}
	}
	report["cases"], report["score"], report["total"] = results, score, len(data.Cases)
	t.Logf("acerto@3 baseline=%d conceitos=%d de %d", score["baseline"], score["concepts"], len(data.Cases))
	if out := os.Getenv("PUBLIC_COLLECT_OUTPUT"); out != "" {
		b, _ := json.MarshalIndent(report, "", "  ")
		if err = os.WriteFile(out, append(b, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
