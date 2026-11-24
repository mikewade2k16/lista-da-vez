package alerts

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPathUUIDRejectsInvalidID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/alerts/{id}", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := pathUUID(w, r, "id"); ok {
			w.WriteHeader(http.StatusOK)
		}
	})

	cases := map[string]int{
		"/v1/alerts/0b8f4a8e-3c1d-4d6a-9f0e-5a1b2c3d4e5f": http.StatusOK,
		"/v1/alerts/a":     http.StatusBadRequest,
		"/v1/alerts/%27--": http.StatusBadRequest,
	}
	for path, want := range cases {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != want {
			t.Fatalf("%s: status %d, esperado %d", path, recorder.Code, want)
		}
	}
}
