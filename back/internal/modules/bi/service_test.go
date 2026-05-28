package bi

import (
	"encoding/json"
	"testing"
)

func TestNormalizeFindEndpointAllowlist(t *testing.T) {
	if endpoint, ok := normalizeFindEndpoint("item/find"); !ok || endpoint != "/item/find" {
		t.Fatalf("expected /item/find, got %q ok=%v", endpoint, ok)
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
