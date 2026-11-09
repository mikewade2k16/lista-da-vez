package knowledge

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
)

func TestInitialPublicCorpusWithoutTenantOrLLM(t *testing.T) {
	url := os.Getenv("PUBLIC_KNOWLEDGE_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires migrated disposable accounting_test database")
	}
	ctx := context.Background()
	p, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if !strings.HasPrefix(p.Config().ConnConfig.Database, "accounting_test_") {
		t.Fatal("refusing non-disposable database")
	}
	s := NewService(NewStore(p), Ports{})
	for _, terms := range [][]string{{"limite", "mei", "receita", "anual"}, {"dasn"}, {"nfse", "servicos"}, {"simples", "nacional"}} {
		out, err := s.SearchPublic(ctx, terms)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Evidence) == 0 || len(out.Evidence) > 3 {
			t.Fatalf("query %v evidence %d", terms, len(out.Evidence))
		}
		for _, e := range out.Evidence {
			if e.Applicable || e.SourceRef == "" || e.Version < 1 {
				t.Fatal("invalid evidence")
			}
		}
	}
	out, err := s.SearchPublic(ctx, []string{"entropiazz"})
	if err != nil || len(out.Evidence) != 0 {
		t.Fatal("missing source masked", err)
	}
	if _, err = s.SearchPublic(ctx, []string{"mei | !private"}); err == nil {
		t.Fatal("tsquery operator accepted")
	}
}
