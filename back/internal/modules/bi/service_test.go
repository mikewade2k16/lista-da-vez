package bi

import (
	"encoding/json"
	"testing"
)

func TestNormalizeFindEndpointAllowlist(t *testing.T) {
	allowed := map[string]string{
		"item/find":                 "/item/find",
		"/imagemItem/find":          "/imagemItem/find",
		"itemSaldoPrecoCompra/find": "/itemSaldoPrecoCompra/find",
		"/nota/find":                "/nota/find",
		"notaItem/find":             "/notaItem/find",
		"/inventario/find":          "/inventario/find",
	}
	for input, expected := range allowed {
		if endpoint, ok := normalizeFindEndpoint(input); !ok || endpoint != expected {
			t.Fatalf("expected %s for %s, got %q ok=%v", expected, input, endpoint, ok)
		}
	}

	if _, ok := normalizeFindEndpoint("https://example.com/api"); ok {
		t.Fatal("expected arbitrary URL to be rejected")
	}
}

func TestExtractBearerToken(t *testing.T) {
	body := map[string]any{
		"data": map[string]any{
			"token": "Bearer aaa.bbb.ccc",
		},
	}

	if token := extractBearerToken(body); token != "aaa.bbb.ccc" {
		t.Fatalf("expected nested JWT, got %q", token)
	}
}

func TestParseUpstreamBody(t *testing.T) {
	body, raw := parseUpstreamBody("application/json; charset=utf-8", []byte(`{"ok":true}`))
	if raw == "" {
		t.Fatal("expected raw body")
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"ok":true}` {
		t.Fatalf("expected parsed JSON body, got %s", encoded)
	}
}

func TestNewServiceDefaultMaxPagesIsCapped(t *testing.T) {
	service := NewService()

	if service.options.MaxPages != 5 {
		t.Fatalf("expected default MaxPages=5, got %d", service.options.MaxPages)
	}

	if service.options.PageLimit != 100 {
		t.Fatalf("expected default PageLimit=100, got %d", service.options.PageLimit)
	}
}

func TestOverviewDatasetDefinitionsKeepSinglePage(t *testing.T) {
	for _, definition := range perolaDatasetDefinitions() {
		if !definition.IncludeInOverview {
			continue
		}

		if definition.MaxPages != 1 {
			t.Fatalf("dataset %q no overview deve fixar MaxPages=1 (fan-out de paginas fica fora do overview), got %d", definition.Key, definition.MaxPages)
		}
	}
}
