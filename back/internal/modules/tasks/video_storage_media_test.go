package tasks

import (
	"context"
	"strings"
	"testing"
)

func TestDiskVideoStorageAcceptsSupportedImage(t *testing.T) {
	storage := NewDiskVideoStorage(t.TempDir())
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R'}

	stored, err := storage.Save(context.Background(), "account-1", "user-1", "task-1", "key-1", "capa.png", "image/png", png)
	if err != nil {
		t.Fatalf("imagem suportada deveria ser aceita: %v", err)
	}
	if stored.ContentType != "image/png" || !strings.HasSuffix(stored.Path, ".png") {
		t.Fatalf("metadata inesperada: %+v", stored)
	}
}

func TestTaskMetadataMediaSnapshotsKeepsImageType(t *testing.T) {
	snapshots := taskMetadataMediaSnapshots(map[string]any{
		"videos": []any{map[string]any{
			"id": "image-1", "url": "/uploads/tasks/account/image-1/capa.png",
			"name": "capa.png", "contentType": "image/png", "size": 16,
		}},
	})

	if len(snapshots) != 1 || snapshots[0].Type != "image" {
		t.Fatalf("snapshot deveria preservar tipo image: %+v", snapshots)
	}
}
