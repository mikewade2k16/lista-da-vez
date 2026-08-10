package tasks

import "testing"

func TestNormalizeTaskUIMetadataChecklist(t *testing.T) {
	metadata := normalizeTaskUIMetadata(map[string]any{
		"checklist": []any{
			map[string]any{"id": "movie-1", "title": "  Filme um  ", "completed": true},
			map[string]any{"id": "movie-1", "title": "Duplicado", "completed": false},
			map[string]any{"id": "movie-2", "title": "Filme dois", "completed": "true"},
			map[string]any{"id": "empty", "title": "   ", "completed": true},
		},
		"unknown": "discarded",
	})

	if _, exists := metadata["unknown"]; exists {
		t.Fatal("normalizeTaskUIMetadata deve descartar chaves fora da whitelist")
	}
	checklist, ok := metadata["checklist"].([]map[string]any)
	if !ok {
		t.Fatalf("checklist deveria ser []map[string]any; got %T", metadata["checklist"])
	}
	if len(checklist) != 2 {
		t.Fatalf("checklist deveria remover duplicados e vazios; got %d itens", len(checklist))
	}
	if checklist[0]["title"] != "Filme um" || checklist[0]["completed"] != true {
		t.Fatalf("primeiro item nao foi normalizado: %+v", checklist[0])
	}
	if checklist[1]["completed"] != false {
		t.Fatalf("completed aceita somente booleano true; got %+v", checklist[1])
	}
}

func TestNormalizeTaskVideoMetadataKeepsChecklistAssociation(t *testing.T) {
	metadata := normalizeTaskUIMetadata(map[string]any{
		"videos": []any{map[string]any{
			"id":              "video-1",
			"name":            "original.mp4",
			"url":             "/uploads/tasks/account/video-1/original.mp4",
			"size":            125,
			"contentType":     "video/mp4",
			"checklistItemId": "item-1",
		}},
	})

	videos, ok := metadata["videos"].([]map[string]any)
	if !ok || len(videos) != 1 {
		t.Fatalf("videos deveria conter um item normalizado; got %#v", metadata["videos"])
	}
	if videos[0]["checklistItemId"] != "item-1" {
		t.Fatalf("associacao com checklist foi perdida: %+v", videos[0])
	}
}
