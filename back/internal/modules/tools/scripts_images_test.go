package tools

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScriptImagePathIsScoped(t *testing.T) {
	root := t.TempDir()
	account := "00000000-0000-4000-8000-000000000001"
	id := "00000000-0000-4000-8000-000000000002"
	path, err := scriptImagePath(root, account, id)
	if err != nil || path != filepath.Join(root, "scripts", account, id) {
		t.Fatalf("unexpected private path: %s %v", path, err)
	}
	for _, bad := range []string{"", "../other", "../../public"} {
		if _, err = scriptImagePath(root, account, bad); !errors.Is(err, errScriptNotFound) {
			t.Fatalf("accepted unsafe image id %q", bad)
		}
	}
	if _, err = scriptImagePath("", account, id); !errors.Is(err, errScriptStorageUnavailable) {
		t.Fatal("storage must fail closed")
	}
}
func TestScriptUploadRejectsActiveContentAndOversize(t *testing.T) {
	repo := &scriptRepositoryFake{}
	svc := newScriptService(repo)
	svc.imageRoot = t.TempDir()
	for _, body := range []string{"<svg onload='alert(1)'></svg>", "<html>not an image</html>", strings.Repeat("a", maxScriptImageBytes+1)} {
		if _, err := svc.uploadImage(context.Background(), "00000000-0000-4000-8000-000000000001", strings.NewReader(body)); !errors.Is(err, errInvalidScript) {
			t.Fatalf("accepted invalid image: %v", err)
		}
	}
	if repo.calls != 0 {
		t.Fatal("invalid content reached repository")
	}
}
func TestScriptUploadPreservesBytesAndCleansFailedMetadata(t *testing.T) {
	repo := &scriptRepositoryFake{}
	svc := newScriptService(repo)
	svc.imageRoot = t.TempDir()
	// PNG signature plus opaque payload: no transcoding, original bytes are preserved.
	data := append([]byte{137, 80, 78, 71, 13, 10, 26, 10}, bytes.Repeat([]byte{0}, 512)...)
	account := "00000000-0000-4000-8000-000000000001"
	image, err := svc.uploadImage(context.Background(), account, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	path, _ := scriptImagePath(svc.imageRoot, account, image.ID)
	stored, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, stored) {
		t.Fatal("original image was changed")
	}
	if repo.account != account || image.URL != "/v1/tools/scripts/images/"+image.ID {
		t.Fatal("image escaped account context")
	}
	repo.err = errors.New("metadata failed")
	if _, err = svc.uploadImage(context.Background(), account, bytes.NewReader(data)); err == nil {
		t.Fatal("metadata failure ignored")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatal("failed upload left a file behind")
	}
}
func TestScriptReorderRejectsInvalidOrDuplicateIDs(t *testing.T) {
	repo := &scriptRepositoryFake{}
	svc := newScriptService(repo)
	id := "00000000-0000-4000-8000-000000000001"
	for _, ids := range [][]string{nil, {id, id}, {"../other"}} {
		if err := svc.reorder(context.Background(), id, id, ids); !errors.Is(err, errInvalidScript) {
			t.Fatalf("accepted invalid order: %v", err)
		}
	}
	if repo.calls != 0 {
		t.Fatal("invalid order reached repository")
	}
	if err := svc.reorder(context.Background(), id, id, []string{id}); err != nil || repo.account != id {
		t.Fatal("valid order did not preserve scope")
	}
}
