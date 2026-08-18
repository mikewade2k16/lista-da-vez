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
	for _, key := range []string{"status", "statusDate", "completedDate"} {
		if _, exists := checklist[0][key]; exists {
			t.Fatalf("item legado nao deve ganhar %s sem valor valido: %+v", key, checklist[0])
		}
	}
}

func TestNormalizeTaskChecklistMetadataKeepsOptionalWorkflowFields(t *testing.T) {
	statuses := []string{"captured", "editing", "approval", "approved", "scheduled", "posted"}
	rawItems := make([]any, 0, len(statuses))
	for index, status := range statuses {
		rawItems = append(rawItems, map[string]any{
			"id":            status,
			"title":         "Conteudo " + status,
			"completed":     index == len(statuses)-1,
			"status":        "  " + status + "  ",
			"statusDate":    " 2026-08-13 ",
			"completedDate": "2024-02-29",
		})
	}

	items := normalizeTaskChecklistMetadata(rawItems)
	if len(items) != len(statuses) {
		t.Fatalf("todos os status validos deveriam ser preservados; got %d itens", len(items))
	}
	for index, status := range statuses {
		if items[index]["status"] != status {
			t.Errorf("status %q nao foi preservado: %+v", status, items[index])
		}
		if items[index]["statusDate"] != "2026-08-13" {
			t.Errorf("statusDate deveria ser date-only normalizada: %+v", items[index])
		}
		if index == len(statuses)-1 && items[index]["completedDate"] != "2024-02-29" {
			t.Errorf("completedDate bissexta valida deveria ser preservada no item concluido: %+v", items[index])
		}
		if index != len(statuses)-1 {
			if _, exists := items[index]["completedDate"]; exists {
				t.Errorf("completedDate nao deve sobreviver quando completed=false: %+v", items[index])
			}
		}
	}
}

func TestNormalizeTaskChecklistMetadataOmitsInvalidOptionalWorkflowFields(t *testing.T) {
	items := normalizeTaskChecklistMetadata([]any{
		map[string]any{
			"id":            "item-invalid",
			"title":         "Item ainda valido",
			"completed":     false,
			"status":        "done",
			"statusDate":    "2026-02-30",
			"completedDate": "2026-08-13T10:00:00Z",
			"unknown":       "discarded",
		},
		map[string]any{
			"id":            "item-empty",
			"title":         "Campos vazios",
			"completed":     true,
			"status":        "   ",
			"statusDate":    " ",
			"completedDate": 20260813,
		},
	})

	if len(items) != 2 {
		t.Fatalf("campos opcionais invalidos nao devem remover o item legado; got %d itens", len(items))
	}
	for _, item := range items {
		for _, key := range []string{"status", "statusDate", "completedDate", "unknown"} {
			if _, exists := item[key]; exists {
				t.Errorf("campo %s invalido/desconhecido deveria ser omitido: %+v", key, item)
			}
		}
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

func TestNormalizeTaskVideoMetadataOmitsMissingChecklistAssociation(t *testing.T) {
	metadata := normalizeTaskUIMetadata(map[string]any{
		"videos": []any{map[string]any{
			"id": "video-1", "name": "original.mp4",
			"url": "/uploads/tasks/account/video-1/original.mp4",
		}},
	})

	videos := metadata["videos"].([]map[string]any)
	if _, exists := videos[0]["checklistItemId"]; exists {
		t.Fatalf("associacao ausente nao pode virar texto: %+v", videos[0])
	}
}

func TestNormalizeTaskMediaOrderKeepsUnifiedSources(t *testing.T) {
	metadata := normalizeTaskUIMetadata(map[string]any{
		"mediaOrder": []any{"calendar:media-2", "task-media-1", "calendar:media-2", ""},
	})

	order, ok := metadata["mediaOrder"].([]string)
	if !ok {
		t.Fatalf("mediaOrder deveria ser uma lista normalizada; got %#v", metadata["mediaOrder"])
	}
	if len(order) != 2 || order[0] != "calendar:media-2" || order[1] != "task-media-1" {
		t.Fatalf("mediaOrder deveria preservar fontes e remover duplicados; got %#v", order)
	}
}
