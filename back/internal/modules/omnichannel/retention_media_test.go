package omnichannel

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeFile grava um arquivo de teste e devolve o path.
func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestPurgeConversationDirDeletesFilesAndBytes(t *testing.T) {
	root := t.TempDir()
	m := newMediaPurger(root)
	convDir := filepath.Join(root, "acct1", "conv1")
	writeFile(t, filepath.Join(convDir, "a.jpg"), 100)
	writeFile(t, filepath.Join(convDir, "b.ogg"), 50)

	// Dry-run: conta mas NAO apaga.
	files, bytes, err := m.purgeConversationDir("acct1", "conv1", true)
	if err != nil {
		t.Fatalf("dry-run erro: %v", err)
	}
	if files != 2 || bytes != 150 {
		t.Fatalf("dry-run contagem = {%d, %d}, quer {2, 150}", files, bytes)
	}
	if _, statErr := os.Stat(convDir); statErr != nil {
		t.Fatal("dry-run NAO podia apagar o diretorio")
	}

	// Delete real: apaga e contabiliza.
	files, bytes, err = m.purgeConversationDir("acct1", "conv1", false)
	if err != nil {
		t.Fatalf("delete erro: %v", err)
	}
	if files != 2 || bytes != 150 {
		t.Fatalf("delete contagem = {%d, %d}, quer {2, 150}", files, bytes)
	}
	if _, statErr := os.Stat(convDir); !os.IsNotExist(statErr) {
		t.Fatal("delete devia ter removido o diretorio da conversa")
	}
}

func TestPurgeConversationDirMissingIsNoop(t *testing.T) {
	m := newMediaPurger(t.TempDir())
	files, bytes, err := m.purgeConversationDir("acct1", "sem-midia", false)
	if err != nil || files != 0 || bytes != 0 {
		t.Fatalf("conversa sem diretorio devia ser no-op, veio {%d,%d,%v}", files, bytes, err)
	}
}

func TestScanOrphansGraceWindowAndKnownKeys(t *testing.T) {
	root := t.TempDir()
	m := newMediaPurger(root)

	known := filepath.Join(root, "acct1", "convK", "keep.jpg")    // referenciado por mensagem
	orphanOld := filepath.Join(root, "acct1", "convO", "old.jpg") // orfao antigo -> apaga
	orphanNew := filepath.Join(root, "acct1", "convN", "new.jpg") // orfao recente -> carencia
	writeFile(t, known, 10)
	writeFile(t, orphanOld, 20)
	writeFile(t, orphanNew, 30)

	now := time.Now()
	old := now.Add(-48 * time.Hour)
	if err := os.Chtimes(orphanOld, old, old); err != nil {
		t.Fatalf("chtimes old: %v", err)
	}
	if err := os.Chtimes(known, old, old); err != nil { // velho mas conhecido: nao apaga
		t.Fatalf("chtimes known: %v", err)
	}
	// orphanNew fica com mtime = agora (dentro da carencia).

	knownSet := map[string]bool{"acct1/convK/keep.jpg": true}
	files, bytes, err := m.scanOrphans("acct1", knownSet, now, false)
	if err != nil {
		t.Fatalf("scan erro: %v", err)
	}
	if files != 1 || bytes != 20 {
		t.Fatalf("scan = {%d,%d}, quer {1,20} (so o orfao antigo)", files, bytes)
	}
	if _, statErr := os.Stat(orphanOld); !os.IsNotExist(statErr) {
		t.Error("orfao antigo devia ter sido apagado")
	}
	if _, statErr := os.Stat(known); statErr != nil {
		t.Error("arquivo referenciado NAO podia ser apagado")
	}
	if _, statErr := os.Stat(orphanNew); statErr != nil {
		t.Error("orfao dentro da carencia (24h) NAO podia ser apagado")
	}
}

func TestScanOrphansDryRunCountsWithoutDeleting(t *testing.T) {
	root := t.TempDir()
	m := newMediaPurger(root)
	orphan := filepath.Join(root, "acct1", "conv", "o.jpg")
	writeFile(t, orphan, 15)
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(orphan, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	files, bytes, err := m.scanOrphans("acct1", map[string]bool{}, time.Now(), true)
	if err != nil || files != 1 || bytes != 15 {
		t.Fatalf("dry-run = {%d,%d,%v}, quer {1,15,nil}", files, bytes, err)
	}
	if _, statErr := os.Stat(orphan); statErr != nil {
		t.Error("dry-run NAO podia apagar o orfao")
	}
}

func TestStorageKeyForOutsideRootIsEmpty(t *testing.T) {
	root := t.TempDir()
	rootAbs, _ := filepath.Abs(root)
	if key := storageKeyFor(rootAbs, filepath.Join(rootAbs, "acct1", "conv", "f.jpg")); key != "acct1/conv/f.jpg" {
		t.Errorf("key dentro da raiz = %q, quer acct1/conv/f.jpg", key)
	}
	outside := filepath.Join(filepath.Dir(rootAbs), "fora.jpg")
	if key := storageKeyFor(rootAbs, outside); key != "" {
		t.Errorf("path fora da raiz devia dar key vazia, veio %q", key)
	}
}
