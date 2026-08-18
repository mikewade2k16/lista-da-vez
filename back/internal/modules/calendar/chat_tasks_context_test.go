package calendar

import (
	"fmt"
	"testing"
)

func TestTaskContextChecklistItemsDefensiveAndBounded(t *testing.T) {
	t.Parallel()
	checklist := make([]any, 0, maxContextTaskItems+8)
	checklist = append(checklist,
		map[string]any{"id": "item-1", "title": " Reel 01 ", "completed": true, "status": "posted", "statusDate": "2026-08-13", "completedDate": "2026-08-12"},
		map[string]any{"id": "item-1", "title": "duplicado", "completed": false},
		map[string]any{"id": "item-invalido", "title": "Invalido", "completed": "sim", "status": "waiting", "statusDate": "2026-02-30", "completedDate": 12},
		"shape invalido",
		map[string]any{"id": "", "title": "sem id"},
	)
	for i := 0; i < maxContextTaskItems+5; i++ {
		checklist = append(checklist, map[string]any{"id": fmt.Sprintf("bulk-%d", i), "title": fmt.Sprintf("Item %d", i), "completed": false})
	}
	got := taskContextChecklistItems(map[string]any{"checklist": checklist})
	if len(got) != maxContextTaskItems {
		t.Fatalf("contexto deveria limitar em %d, veio %d", maxContextTaskItems, len(got))
	}
	first := got[0]
	if first.ID != "item-1" || first.Title != "Reel 01" || !first.Completed || first.Status != "posted" || first.StatusDate != "2026-08-13" || first.CompletedDate != "2026-08-12" {
		t.Fatalf("item valido projetado incorretamente: %#v", first)
	}
	invalid := got[1]
	if invalid.ID != "item-invalido" || invalid.Completed || invalid.Status != "" || invalid.StatusDate != "" || invalid.CompletedDate != "" {
		t.Fatalf("campos dinamicos invalidos deveriam ser omitidos: %#v", invalid)
	}
	if got[len(got)-1].ID != fmt.Sprintf("bulk-%d", maxContextTaskItems-3) {
		t.Fatalf("item no fim do limite autoritativo deveria continuar acessivel: %#v", got[len(got)-1])
	}
}

func TestTaskContextChecklistItemsAcceptsTypedSlice(t *testing.T) {
	t.Parallel()
	got := taskContextChecklistItems(map[string]any{"checklist": []map[string]any{{"id": "a", "title": "A", "completed": false}}})
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("slice tipado deveria ser aceito: %#v", got)
	}
}
