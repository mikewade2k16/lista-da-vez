package metaads

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInstagramIdentityMappingBodyIsStrict(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{name: "missing field", body: `{}`},
		{name: "unknown field", body: `{"clientAccountId":"","accountId":"impersonated"}`},
		{name: "trailing json", body: `{"clientAccountId":""}{}`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(
				context.Background(), "PATCH", "/instagram-identities/1/client", strings.NewReader(test.body),
			)
			response := httptest.NewRecorder()
			if _, ok := readInstagramIdentityClientBody(response, request); ok {
				t.Fatal("strict mapping body accepted invalid input")
			}
			if response.Code != 400 {
				t.Fatalf("status=%d want=400", response.Code)
			}
		})
	}
}
