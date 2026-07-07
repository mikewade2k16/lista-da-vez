package calendar

import (
	"testing"
	"time"
)

func TestSelectContextEventsOnlyReturnsAuthoritativeItems(t *testing.T) {
	events := []AIContextEvent{
		{ID: "event-1", Title: "Primeiro"},
		{ID: "event-2", Title: "Segundo"},
	}
	got := selectContextEvents(events, []string{"event-2", "inventado", "event-2", "event-1"})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != "event-2" || got[1].ID != "event-1" {
		t.Fatalf("unexpected order/items: %#v", got)
	}
}

func TestInferChatMonth(t *testing.T) {
	now := time.Date(2026, time.July, 7, 0, 0, 0, 0, time.UTC)
	tests := map[string]string{
		"o que temos em 18/08?":         "2026-08",
		"mostre 18/08/2027":             "2027-08",
		"agenda de março de 2028":       "2028-03",
		"postagens para 2027-11":        "2027-11",
		"sem mes explicito nesta frase": "2026-07",
	}
	for question, want := range tests {
		if got := inferChatMonth(question, "2026-07", now); got != want {
			t.Errorf("inferChatMonth(%q) = %q, want %q", question, got, want)
		}
	}
}

func TestAppendContextMediaDeduplicatesByIDOrURL(t *testing.T) {
	items := appendContextMedia(nil, MediaItem{ID: "media-1", URL: "/uploads/one.jpg"})
	items = appendContextMedia(items, MediaItem{ID: "media-1", URL: "/uploads/copy.jpg"})
	items = appendContextMedia(items, MediaItem{ID: "media-2", URL: "/uploads/one.jpg"})
	if len(items) != 1 {
		t.Fatalf("len = %d, want 1", len(items))
	}
}
