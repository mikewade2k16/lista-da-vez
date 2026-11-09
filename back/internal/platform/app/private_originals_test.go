package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/storage"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/config"
)

func TestPrivateOriginalsDiskFallback(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	p := privateOriginals{storage: storage.New(storage.Config{}), dir: dir}
	account := "11111111-2222-4333-8444-555555555555"
	ref, err := p.Put(ctx, account, "actor", "finance", "receipt:1:abc", "a.pdf", "application/pdf", []byte("%PDF-1"))
	if err != nil || !strings.HasPrefix(ref, "disk:") {
		t.Fatal(ref, err)
	}
	again, _ := p.Put(ctx, account, "actor", "finance", "receipt:1:abc", "a.pdf", "application/pdf", []byte("%PDF-1"))
	if again != ref {
		t.Fatal("same key created another file")
	}
	info, err := os.Stat(filepath.Join(dir, "finance", account, strings.TrimPrefix(ref, "disk:")))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("file mode", err)
	}
	body, err := p.Open(ctx, account, "finance", ref)
	if err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(body)
	_ = body.Close()
	if string(content) != "%PDF-1" {
		t.Fatal("bytes changed")
	}
	// Another account or source cannot reach the file; traversal refs are rejected.
	if _, err = p.Open(ctx, "99999999-2222-4333-8444-555555555555", "finance", ref); !errors.Is(err, errPrivateOriginalNotFound) {
		t.Fatal("cross-account open", err)
	}
	for _, bad := range []string{"disk:../../etc/passwd", "s3:" + strings.TrimPrefix(ref, "disk:"), "disk"} {
		if _, err = p.Open(ctx, account, "finance", bad); !errors.Is(err, errPrivateOriginalNotFound) {
			t.Fatal("bad ref accepted", bad, err)
		}
	}
	if err = p.Delete(ctx, account, "finance", ref); err != nil {
		t.Fatal(err)
	}
	if err = p.Delete(ctx, account, "finance", ref); err != nil {
		t.Fatal("second delete is not idempotent", err)
	}
	if _, err = p.Open(ctx, account, "finance", ref); !errors.Is(err, errPrivateOriginalNotFound) {
		t.Fatal("deleted file still readable", err)
	}
	// Without R2 and without a private directory nothing is stored.
	if _, err = (privateOriginals{storage: storage.New(storage.Config{})}).Put(ctx, account, "actor", "finance", "k", "a", "text/plain", []byte("x")); !errors.Is(err, errPrivateOriginalsUnavailable) {
		t.Fatal(err)
	}
	if financeOriginalFiles(storage.New(storage.Config{}), config.Config{}) != nil {
		t.Fatal("port enabled without destination")
	}
	if uuidFromHex("0123456789abcdef0123456789abcdef") != "01234567-89ab-cdef-0123-456789abcdef" {
		t.Fatal("uuid conversion")
	}
}
