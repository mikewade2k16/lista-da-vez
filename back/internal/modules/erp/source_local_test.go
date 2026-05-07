package erp

import (
	"context"
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestLocalSourceListAndOpen(t *testing.T) {
	source, err := NewLocalSource(SourceOptions{
		LocalDir: "fixtures",
		LocalFS: fstest.MapFS{
			"processed/184/order/20240517042655_184-12583959000186-order-20240510010212.csv": &fstest.MapFile{
				Data:    []byte("first"),
				Mode:    0o644,
				ModTime: time.Date(2024, 5, 17, 4, 26, 55, 0, time.UTC),
			},
			"processed/184/item/20260413010001_184-12583959000186-item-20260413010001.csv": &fstest.MapFile{
				Data:    []byte("second"),
				Mode:    0o644,
				ModTime: time.Date(2026, 4, 13, 1, 0, 1, 0, time.UTC),
			},
			"processed/907/order/20240517042655_907-12583959000186-order-20240510010212.csv": &fstest.MapFile{
				Data: []byte("other-store"),
				Mode: 0o644,
			},
			"processed/184/order/not-a-csv.txt": &fstest.MapFile{Data: []byte("ignore"), Mode: 0o644},
		},
	})
	if err != nil {
		t.Fatalf("NewLocalSource() error = %v", err)
	}

	files, err := source.List(context.Background(), "184")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Name != "processed/184/item/20260413010001_184-12583959000186-item-20260413010001.csv" {
		t.Fatalf("unexpected first file %q", files[0].Name)
	}

	reader, err := source.Open(context.Background(), files[1].Name)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(body) != "first" {
		t.Fatalf("unexpected opened content %q", string(body))
	}
}

func TestLocalSourceRejectsParentTraversal(t *testing.T) {
	source, err := NewLocalSource(SourceOptions{LocalDir: "fixtures", LocalFS: fstest.MapFS{"a.csv": &fstest.MapFile{Data: []byte("ok")}}})
	if err != nil {
		t.Fatalf("NewLocalSource() error = %v", err)
	}

	_, err = source.Open(context.Background(), "../secret.csv")
	if err == nil || !strings.Contains(err.Error(), ErrSourcePathOutsideRoot.Error()) {
		t.Fatalf("expected path outside root error, got %v", err)
	}
}

var _ fs.FS = fstest.MapFS{}
