package contentoperations

import (
	"testing"
	"time"
)

func TestBuildBriefCreatesProactiveAlertsFromTasksAndCalendar(t *testing.T) {
	location := saoPaulo()
	now := time.Date(2026, time.August, 16, 10, 0, 0, 0, location) // domingo: fechamento
	clients := []ScopeClient{{ID: "client-1", Name: "Cliente X"}}
	tasks := []TaskSnapshot{{
		ID: "task-1", ClientID: "client-1", Title: "Conteúdos agosto", Updated: now.AddDate(0, 0, -5),
		Items: []ChecklistItem{{ID: "item-1", Title: "Reel A", Status: "approved", StatusDate: "2026-08-15"}},
	}}
	events := []EventSnapshot{{
		ID: "event-1", ClientID: "client-1", Date: now.AddDate(0, 0, -2), Type: "reels", Title: "Reel passado", Status: "planejado",
	}}

	brief := buildBrief(now, clients, tasks, events)
	if brief.Mode != "closing" {
		t.Fatalf("mode = %q, want closing", brief.Mode)
	}
	if brief.Counts.Critical < 3 {
		t.Fatalf("critical = %d, want calendar confirmation, ready item and no recent post", brief.Counts.Critical)
	}
	assertAlertType(t, brief.Alerts, "calendar_unconfirmed")
	assertAlertType(t, brief.Alerts, "task_item_approved")
	assertAlertType(t, brief.Alerts, "client_without_post")
}

func TestBuildBriefMondayUsesPlanningAndDoesNotRequestCaptureWithPipeline(t *testing.T) {
	now := time.Date(2026, time.August, 17, 9, 0, 0, 0, saoPaulo())
	brief := buildBrief(now,
		[]ScopeClient{{ID: "client-1", Name: "Cliente X"}},
		[]TaskSnapshot{{ID: "task-1", ClientID: "client-1", Updated: now, Items: []ChecklistItem{{ID: "i", Title: "A", Status: "editing"}}}},
		nil,
	)
	if brief.Mode != "planning" {
		t.Fatalf("mode = %q, want planning", brief.Mode)
	}
	for _, alert := range brief.Alerts {
		if alert.Type == "schedule_capture" {
			t.Fatal("pipeline content must prevent schedule_capture")
		}
	}
}

func TestFilterBriefKeepsCrowInsideVisibleClientScope(t *testing.T) {
	brief := Brief{
		Clients: []ClientHealth{{ClientID: "a"}, {ClientID: "b"}},
		Alerts: []Alert{
			{ID: "a", ClientID: "a", Severity: SeverityCritical},
			{ID: "b", ClientID: "b", Severity: SeverityAttention},
		},
	}
	filtered := FilterBrief(brief, []string{"b"})
	if len(filtered.Clients) != 1 || filtered.Clients[0].ClientID != "b" {
		t.Fatalf("clients = %#v", filtered.Clients)
	}
	if len(filtered.Alerts) != 1 || filtered.Alerts[0].ClientID != "b" {
		t.Fatalf("alerts = %#v", filtered.Alerts)
	}
	if filtered.Counts.Total != 1 || filtered.Counts.Attention != 1 {
		t.Fatalf("counts = %#v", filtered.Counts)
	}
}

func assertAlertType(t *testing.T, alerts []Alert, wanted string) {
	t.Helper()
	for _, alert := range alerts {
		if alert.Type == wanted {
			return
		}
	}
	t.Fatalf("alert type %q not found in %#v", wanted, alerts)
}
