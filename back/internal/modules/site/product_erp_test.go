package site

import (
	"strings"
	"testing"
)

// TestErpUnmatchedWhere_NoQuery garante que sem filtro `q` o WHERE so usa $1
// (account/tenant) e nao adiciona o predicado de busca nem um segundo arg.
func TestErpUnmatchedWhere_NoQuery(t *testing.T) {
	where, args := erpUnmatchedWhere("")
	if len(args) != 1 {
		t.Fatalf("esperava 1 arg (placeholder $1), obteve %d: %v", len(args), args)
	}
	if !strings.Contains(where, "e.tenant_id = $1::uuid") {
		t.Errorf("WHERE deveria escopar por tenant_id em $1, obteve: %s", where)
	}
	if strings.Contains(where, "$2") {
		t.Errorf("sem q nao deveria referenciar $2, obteve: %s", where)
	}
	if !strings.Contains(where, "not exists") {
		t.Errorf("WHERE deveria conter o anti-join 'not exists', obteve: %s", where)
	}
}

// TestErpUnmatchedWhere_WithQuery garante que com `q` o predicado ilike entra
// em $2 e o arg normalizado (lower + %wrap) e adicionado.
func TestErpUnmatchedWhere_WithQuery(t *testing.T) {
	where, args := erpUnmatchedWhere("  Anel  ")
	if len(args) != 2 {
		t.Fatalf("esperava 2 args (placeholder + pattern), obteve %d: %v", len(args), args)
	}
	pattern, ok := args[1].(string)
	if !ok {
		t.Fatalf("arg[1] deveria ser string, obteve %T", args[1])
	}
	if pattern != "%anel%" {
		t.Errorf("pattern deveria ser '%%anel%%' (lower+trim+wrap), obteve %q", pattern)
	}
	if !strings.Contains(where, "$2") {
		t.Errorf("com q o WHERE deveria referenciar $2, obteve: %s", where)
	}
	if !strings.Contains(where, "lower(e.sku) like $2") || !strings.Contains(where, "lower(e.name) like $2") {
		t.Errorf("q deveria filtrar sku/name via ilike, obteve: %s", where)
	}
}

// TestProductCodeSegments documenta a semantica do split de code por '_' que o
// SQL (string_to_array(p.code,'_')) reproduz: cada segmento e um sku candidato.
func TestProductCodeSegments(t *testing.T) {
	cases := map[string][]string{
		"368252_360856": {"368252", "360856"},
		"368252":        {"368252"},
		"":              {""},
	}
	for code, want := range cases {
		got := strings.Split(code, "_")
		if len(got) != len(want) {
			t.Errorf("code %q: esperava %v, obteve %v", code, want, got)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("code %q seg %d: esperava %q, obteve %q", code, i, want[i], got[i])
			}
		}
	}
}
