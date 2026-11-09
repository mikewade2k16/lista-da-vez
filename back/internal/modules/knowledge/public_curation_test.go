package knowledge

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestHTMLToTextDropsScriptsAndDecodesWindows1252(t *testing.T) {
	raw := []byte("<html><head><title>x</title><script>alert('x')</script></head><body><nav>Menu Ir para</nav><main><h2>Art. 18 \xe9 aplic\xe1vel</h2><p>O MEI recolhe o DAS &amp; declara\xe7\xe3o.</p><style>p{}</style><table><tr><td>A</td><td>B</td></tr></table></main><footer>rodap\xe9</footer></body></html>")
	text := htmlToText(decodeHTMLBytes(raw, ""))
	for _, banned := range []string{"alert", "Menu", "rodapé", "p{}", "<"} {
		if strings.Contains(text, banned) {
			t.Fatalf("leaked %q in %q", banned, text)
		}
	}
	for _, want := range []string{"## Art. 18 é aplicável", "O MEI recolhe o DAS & declaração.", "A | B"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	if decodeHTMLBytes([]byte("ação"), "") != "ação" {
		t.Fatal("valid utf-8 changed")
	}
}

func TestChunkPublicPagesKeepsAllTextWithinLimits(t *testing.T) {
	long := strings.Repeat("palavra ", 600)
	pages := []PublicPage{{Page: 1, Text: "## Capítulo 1\n\nPrimeiro parágrafo.\n\n" + long}, {Page: 2, Text: "Segunda página."}}
	chunks := chunkPublicPages(pages)
	if len(chunks) < 4 {
		t.Fatal("expected several chunks", len(chunks))
	}
	words := 0
	for i, c := range chunks {
		if c.Index != i || utf8.RuneCountInString(c.Text) > publicChunkMax || c.Text == "" {
			t.Fatal("bad chunk", i, utf8.RuneCountInString(c.Text))
		}
		words += strings.Count(c.Text, "palavra")
	}
	if words != 600 || chunks[0].Heading != "Capítulo 1" || chunks[len(chunks)-1].Page != 2 {
		t.Fatal("text lost or context wrong", words, chunks[0].Heading)
	}
}

func TestPublicConceptsKeepAcronymsAndExpandVocabulary(t *testing.T) {
	got := strings.Join(publicConcepts([]string{"das", "declaracoes", "imposto", "mei"}), " & ")
	for _, want := range []string{"das", "boleto", "declar:*", "dasn", "tributo:*", "microempreended:*"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
	if strings.Contains(got, "da:*") || publicMinCoverage(1) != 1 || publicMinCoverage(4) != 2 {
		t.Fatal("acronym stemmed or coverage", got)
	}
}

func publicCurationService(t *testing.T) (*Service, *pgxpool.Pool, string) {
	t.Helper()
	url := os.Getenv("PUBLIC_KNOWLEDGE_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires migrated disposable accounting_test database")
	}
	ctx := context.Background()
	p, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	if !strings.HasPrefix(p.Config().ConnConfig.Database, "accounting_test_") {
		t.Fatal("refusing non-disposable database")
	}
	var actor string
	if err = p.QueryRow(ctx, `insert into core.users(email,display_name) values('curadoria-'||gen_random_uuid()||'@example.invalid','Curadoria teste') returning id::text`).Scan(&actor); err != nil {
		t.Fatal(err)
	}
	return NewService(NewStore(p), Ports{}), p, actor
}

// CT-15 fase C: coleta → rascunho invisível → revisões → publicação pelo dono →
// nova versão substitui a anterior; hash igual não duplica; falha registrada.
func TestPublicCurationLifecycle(t *testing.T) {
	s, p, actor := publicCurationService(t)
	ctx := context.Background()
	// Conteúdo único por execução: o banco descartável pode guardar coletas anteriores.
	token := "zf" + strings.ReplaceAll(actor, "-", "")[:12]
	page := "<main><h2>Prazo da DASN-SIMEI</h2><p>O microempreendedor declara a receita bruta anual até o último dia de maio. " + token + " prazo.</p><h2>Empregado</h2><p>Outro trecho sobre empregado contratado " + token + ".</p></main>"
	fetchErr := error(nil)
	s.ports.FetchPublic = func(context.Context, PublicCatalogEntry) ([]byte, string, error) {
		return []byte(page), "utf-8", fetchErr
	}
	fetchErr = fail("source_http_rejected", 422)
	if a, err := s.CollectPublic(ctx, actor, "mei-dasn-servico"); err != nil || a.Outcome != "failed" || a.ErrorCode != "source_http_rejected" {
		t.Fatal("failure not recorded", a, err)
	}
	fetchErr = nil
	if _, err := s.CollectPublic(ctx, actor, "https://evil.example"); err == nil {
		t.Fatal("non-catalog source accepted")
	}
	first, err := s.CollectPublic(ctx, actor, "mei-dasn-servico")
	if err != nil || first.Outcome != "collected" {
		t.Fatal(first, err)
	}
	if again, _ := s.CollectPublic(ctx, actor, "mei-dasn-servico"); again.Outcome != "unchanged" || again.CollectionID != first.CollectionID {
		t.Fatal("same content created a version", again)
	}
	search := func() Result {
		out, e := s.SearchPublic(ctx, []string{token, "prazo"})
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	if len(search().Evidence) != 0 {
		t.Fatal("draft searchable before publication")
	}
	if _, err = s.ReviewPublicCollection(ctx, actor, first.CollectionID, PublicReviewInput{ReviewerKind: "ai", ReviewerName: "Modelo externo A", Verdict: "approved"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PublishPublicCollection(ctx, actor, first.CollectionID); errorCode(err, "") != "owner_approval_required" {
		t.Fatal("published without owner approval", err)
	}
	if _, err = s.ReviewPublicCollection(ctx, actor, first.CollectionID, PublicReviewInput{ReviewerKind: "owner", ReviewerName: "Dono", Verdict: "approved"}); err != nil {
		t.Fatal(err)
	}
	published, err := s.PublishPublicCollection(ctx, actor, first.CollectionID)
	if err != nil || published.Status != "published" || published.Review.Owner != "approved" || published.Review.AI != 1 || published.Review.Accountant != "pending" {
		t.Fatal("publish", published, err)
	}
	hits := search().Evidence
	if len(hits) == 0 || hits[0].Review == nil || hits[0].Review.Accountant != "pending" || !strings.Contains(hits[0].Title, "Prazo da DASN-SIMEI") {
		t.Fatal("published chunk", hits)
	}
	// Contador revisa depois: a evidência mostra a revisão sem alterar o trecho publicado.
	if _, err = s.ReviewPublicCollection(ctx, actor, first.CollectionID, PublicReviewInput{ReviewerKind: "accountant", ReviewerName: "Contador", Verdict: "approved", Notes: "ok"}); err != nil {
		t.Fatal(err)
	}
	if hits = search().Evidence; hits[0].Review.Accountant != "approved" {
		t.Fatal("accountant review not shown", hits[0].Review)
	}
	if _, err = p.Exec(ctx, `update knowledge.public_reviews set verdict='rejected' where collection_id=$1::uuid`, first.CollectionID); err == nil {
		t.Fatal("reviews are mutable")
	}
	// Conteúdo mudou: nova versão em rascunho; trecho excluído não é publicado.
	page = strings.Replace(page, "Outro trecho", "Trecho novo", 1)
	second, err := s.CollectPublic(ctx, actor, "mei-dasn-servico")
	if err != nil || second.Outcome != "collected" || second.CollectionID == first.CollectionID {
		t.Fatal("second version", second, err)
	}
	chunks, err := s.PublicChunks(ctx, second.CollectionID, 1, 50)
	if err != nil || len(chunks) == 0 {
		t.Fatal(chunks, err)
	}
	if err = s.ExcludePublicChunk(ctx, second.CollectionID, chunks[0].ID); err != nil {
		t.Fatal(err)
	}
	export, err := s.ExportPublicCollection(ctx, second.CollectionID)
	if err != nil || !strings.Contains(export, "SHA-256") || strings.Contains(export, chunks[0].Text) {
		t.Fatal("export", err)
	}
	if _, err = s.ReviewPublicCollection(ctx, actor, second.CollectionID, PublicReviewInput{ReviewerKind: "owner", ReviewerName: "Dono", Verdict: "approved"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PublishPublicCollection(ctx, actor, second.CollectionID); err != nil {
		t.Fatal(err)
	}
	var oldPublished int
	_ = p.QueryRow(ctx, `select count(*) from knowledge.public_sources where collection_id=$1::uuid and status='published'`, first.CollectionID).Scan(&oldPublished)
	if oldPublished != 0 {
		t.Fatal("old version still published")
	}
	status, err := s.PublicCatalogStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range status {
		if item.Entry.Key == "mei-dasn-servico" && (item.Published == nil || item.Published.ID != second.CollectionID || item.Published.Excluded != 1) {
			t.Fatal("catalog status", item.Published)
		}
	}
	if err = s.DiscardPublicCollection(ctx, second.CollectionID); !errors.Is(err, err) || errorCode(err, "") != "collection_not_discardable" {
		t.Fatal("published collection discarded", err)
	}
}

func TestPublicNoiseFilter(t *testing.T) {
	for _, noise := range []string{"3.1. Que tributos? ........ 10 3.2. Como pagar? ......... 11", "12", "Página 3 de 120", "Compartilhe :", "Todo o conteúdo deste site está publicado sob a licença Creative Commons"} {
		if !publicNoise(noise) {
			t.Fatal("noise kept", noise)
		}
	}
	for _, text := range []string{"O MEI paga o DAS até o dia 20.", "Art. 18. O valor devido mensalmente...", "2027: R$ 110 mil por ano;"} {
		if publicNoise(text) {
			t.Fatal("content dropped", text)
		}
	}
}
