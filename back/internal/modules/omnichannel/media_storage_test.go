package omnichannel

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pngDataURL(payload string) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte(payload))
}

func TestDiskMediaStorageSaveAndOpen(t *testing.T) {
	store := NewDiskMediaStorage(t.TempDir())
	body := "hello-omni"
	stored, err := store.Save(context.Background(), "acc-1", "conv-1", "IMAGE", "image/png", "foto.png", pngDataURL(body), 1<<20)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if stored.MimeType != "image/png" {
		t.Errorf("mime = %q, want image/png", stored.MimeType)
	}
	if stored.SizeBytes != int64(len(body)) {
		t.Errorf("size = %d, want %d", stored.SizeBytes, len(body))
	}
	if stored.StorageKey == "" {
		t.Fatal("StorageKey vazio")
	}
	wantDigest := sha256.Sum256([]byte(body))
	if stored.SHA256 != hex.EncodeToString(wantDigest[:]) {
		t.Errorf("sha256 = %q, want %q", stored.SHA256, hex.EncodeToString(wantDigest[:]))
	}

	file, info, err := store.Open(stored.StorageKey)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = file.Close() }()
	if info.Size() != int64(len(body)) {
		t.Errorf("file size = %d, want %d", info.Size(), len(body))
	}
	got, _ := io.ReadAll(file)
	if string(got) != body {
		t.Errorf("conteudo = %q, want %q", got, body)
	}
}

func TestDiskMediaStorageSaveInboundIsDeterministicAndAtomic(t *testing.T) {
	root := t.TempDir()
	store := NewDiskMediaStorage(root)
	first, err := store.SaveInboundReader("acc-1", "conv-1", "msg-1", "image/png", "foto.png", strings.NewReader("first"), 1<<20)
	if err != nil {
		t.Fatalf("SaveInboundReader(first): %v", err)
	}
	second, err := store.SaveInboundReader("acc-1", "conv-1", "msg-1", "image/png", "foto.png", strings.NewReader("second"), 1<<20)
	if err != nil {
		t.Fatalf("SaveInboundReader(second): %v", err)
	}
	if first.StorageKey != second.StorageKey {
		t.Fatalf("storage key mudou no retry: %q != %q", first.StorageKey, second.StorageKey)
	}
	file, _, err := store.Open(second.StorageKey)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, _ := io.ReadAll(file)
	_ = file.Close()
	if string(got) != "second" {
		t.Fatalf("conteudo final = %q, want second", got)
	}
	entries, err := os.ReadDir(filepath.Join(root, "acc-1", "conv-1"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || strings.HasPrefix(entries[0].Name(), ".media-") {
		t.Fatalf("arquivos residuais: %#v", entries)
	}
}

func TestDiskMediaStorageTooLarge(t *testing.T) {
	store := NewDiskMediaStorage(t.TempDir())
	// 10 bytes decodificados, teto 3 => 413.
	_, err := store.Save(context.Background(), "acc-1", "conv-1", "IMAGE", "image/png", "x.png", pngDataURL("0123456789"), 3)
	if !errors.Is(err, ErrMediaTooLarge) {
		t.Fatalf("Save = %v, want ErrMediaTooLarge", err)
	}
}

func TestDiskMediaStorageUnsupported(t *testing.T) {
	store := NewDiskMediaStorage(t.TempDir())
	// mime fora do allowlist => 415.
	bad := "data:application/x-msdownload;base64," + base64.StdEncoding.EncodeToString([]byte("MZ"))
	if _, err := store.Save(context.Background(), "acc-1", "conv-1", "DOCUMENT", "", "x.exe", bad, 1<<20); !errors.Is(err, ErrMediaUnsupported) {
		t.Fatalf("Save(mime invalido) = %v, want ErrMediaUnsupported", err)
	}
	// mime valido mas categoria incompativel (imagem declarada como AUDIO) => 415.
	if _, err := store.Save(context.Background(), "acc-1", "conv-1", "AUDIO", "", "x.png", pngDataURL("abc"), 1<<20); !errors.Is(err, ErrMediaUnsupported) {
		t.Fatalf("Save(categoria incompativel) = %v, want ErrMediaUnsupported", err)
	}
}

func TestDiskMediaStorageOpenTraversal(t *testing.T) {
	store := NewDiskMediaStorage(t.TempDir())
	for _, key := range []string{"../secret", "../../etc/passwd", "/etc/passwd", ""} {
		if _, _, err := store.Open(key); !errors.Is(err, ErrMediaInvalid) && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("Open(%q) = %v, want ErrMediaInvalid", key, err)
		}
	}
}

func TestParseDataURL(t *testing.T) {
	mime, payload, ok := parseDataURL("data:image/png;base64,QUJD")
	if !ok || mime != "image/png" || payload != "QUJD" {
		t.Fatalf("parseDataURL = (%q,%q,%v)", mime, payload, ok)
	}
	if _, _, ok := parseDataURL("data:image/png,rawnotbase64"); ok {
		t.Error("parseDataURL deveria rejeitar data URL nao-base64")
	}
	if _, _, ok := parseDataURL("not-a-data-url"); ok {
		t.Error("parseDataURL deveria rejeitar string sem virgula")
	}
}
