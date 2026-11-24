package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const validUUID = "0b8f4a8e-3c1d-4d6a-9f0e-5a1b2c3d4e5f"

func newQueryRequest(rawQuery string) *http.Request {
	return httptest.NewRequest(http.MethodGet, "/x?"+rawQuery, nil)
}

func TestQueryUUID(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		want    string
		wantErr bool
	}{
		{"vazio", "", "", false},
		{"valido", "storeId=" + validUUID, validUUID, false},
		{"valido com espacos", "storeId=%20" + validUUID + "%20", validUUID, false},
		{"letra", "storeId=a", "", true},
		{"aspas", "storeId=%27%22", "", true},
		{"injecao", "storeId=" + validUUID + "%27%3B--", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := QueryUUID(newQueryRequest(tc.query), "storeId")
			if (err != nil) != tc.wantErr {
				t.Fatalf("erro = %v, esperado erro = %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("valor = %q, esperado %q", got, tc.want)
			}
		})
	}
}

func TestQueryInt(t *testing.T) {
	if got, err := QueryInt(newQueryRequest("limit=15"), "limit", 0, 200); err != nil || got != 15 {
		t.Fatalf("valido: got=%d err=%v", got, err)
	}
	if got, err := QueryInt(newQueryRequest(""), "limit", 0, 200); err != nil || got != 0 {
		t.Fatalf("vazio: got=%d err=%v", got, err)
	}
	for _, q := range []string{"limit=a", "limit=-1", "limit=999", "limit=1.5"} {
		if _, err := QueryInt(newQueryRequest(q), "limit", 0, 200); err == nil {
			t.Fatalf("%s deveria falhar", q)
		}
	}
}

func TestWriteInvalidQueryResponds400(t *testing.T) {
	r := newQueryRequest("tenantId=a")
	_, err := QueryUUIDs(r, "tenantId", "storeId")
	recorder := httptest.NewRecorder()
	if !WriteInvalidQuery(recorder, r, err) {
		t.Fatal("deveria ter respondido")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "validation_error") || !strings.Contains(body, "tenantId") {
		t.Fatalf("corpo inesperado: %s", body)
	}

	ok := httptest.NewRecorder()
	if WriteInvalidQuery(ok, r, nil) || ok.Body.Len() != 0 {
		t.Fatal("nao deveria responder sem erro")
	}
}
