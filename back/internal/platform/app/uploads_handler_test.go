package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUploadsHandler(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "avatars"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "avatars", "u-abc.png"), []byte("png-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".secret"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(root), "outside-secret.txt")
	if err := os.WriteFile(outside, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	handler := newUploadsHandler(root)
	get := func(target string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}

	for _, target := range []string{"/uploads/", "/uploads/avatars/", "/uploads/avatars", "/uploads/feedback/"} {
		if res := get(target); res.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404 (sem listagem)", target, res.Code)
		}
	}
	if res := get("/uploads/avatars/u-abc.png"); res.Code != http.StatusOK || res.Body.String() != "png-bytes" {
		t.Errorf("arquivo: status %d body %q", res.Code, res.Body.String())
	}
	for _, target := range []string{
		"/uploads/../outside-secret.txt",
		"/uploads/avatars/../../outside-secret.txt",
		"/uploads/%2e%2e/outside-secret.txt",
		"/uploads/avatars/%2e%2e/%2e%2e/outside-secret.txt",
		"/uploads/.secret",
	} {
		if res := get(target); res.Code == http.StatusOK {
			t.Errorf("%s: serviu conteudo fora do escopo", target)
		}
	}
}
