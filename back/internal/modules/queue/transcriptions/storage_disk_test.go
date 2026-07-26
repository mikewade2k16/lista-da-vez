package transcriptions

import (
	"io"
	"strings"
	"testing"
)

func TestDiskAudioStorageKeepsConflictingChunksSeparate(t *testing.T) {
	t.Parallel()
	storage := NewDiskAudioStorage(t.TempDir())

	first, err := storage.SaveChunk(
		"account-1",
		"recording-1",
		0,
		"audio/webm",
		strings.NewReader("first"),
		100,
	)
	if err != nil {
		t.Fatalf("SaveChunk first: %v", err)
	}
	second, err := storage.SaveChunk(
		"account-1",
		"recording-1",
		0,
		"audio/webm",
		strings.NewReader("second"),
		100,
	)
	if err != nil {
		t.Fatalf("SaveChunk second: %v", err)
	}
	if first.StorageKey == second.StorageKey {
		t.Fatalf("conflicting chunks share storage key %q", first.StorageKey)
	}

	opened, _, err := storage.openContained(first.StorageKey)
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	defer opened.Close()
	content, err := io.ReadAll(opened)
	if err != nil {
		t.Fatalf("read first: %v", err)
	}
	if string(content) != "first" {
		t.Fatalf("first content = %q, want first", content)
	}
}

func TestDiskAudioStorageConsolidatesOrderedChunks(t *testing.T) {
	t.Parallel()
	storage := NewDiskAudioStorage(t.TempDir())
	stored := make([]StoredChunk, 0, 2)
	for sequence, content := range []string{"first", "second"} {
		chunk, err := storage.SaveChunk(
			"account-1",
			"recording-1",
			sequence,
			"audio/webm",
			strings.NewReader(content),
			100,
		)
		if err != nil {
			t.Fatalf("SaveChunk %d: %v", sequence, err)
		}
		stored = append(stored, chunk)
	}

	audio, err := storage.Consolidate(
		"account-1",
		"recording-1",
		"audio/webm",
		[]Chunk{
			{Sequence: 0, StorageKey: stored[0].StorageKey},
			{Sequence: 1, StorageKey: stored[1].StorageKey},
		},
		100,
	)
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	opened, err := storage.Open(audio.StorageKey, "audio/webm", "audio.webm")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer opened.File.Close()
	content, err := io.ReadAll(opened.File)
	if err != nil {
		t.Fatalf("read audio: %v", err)
	}
	if string(content) != "firstsecond" {
		t.Fatalf("content = %q, want firstsecond", content)
	}
}
