package calendar

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClientScopeForcesEventFilterAndInput(t *testing.T) {
	t.Parallel()

	scope := CalendarScope{StorageAccountID: "agency", LockedClientID: "client-a"}
	filter := scopeEventFilter(scope, EventFilter{ClientID: "client-b", From: "2026-07-01"})
	if filter.ClientID != "client-a" {
		t.Fatalf("client filter must be locked to active client, got %q", filter.ClientID)
	}
	input := scopeEventInput(scope, EventInput{ClientID: "client-b", Title: "Post"})
	if input.ClientID != "client-a" {
		t.Fatalf("event input must be locked to active client, got %q", input.ClientID)
	}
}

func TestClientScopeRejectsAnotherClientsEvent(t *testing.T) {
	t.Parallel()

	scope := CalendarScope{LockedClientID: "client-a"}
	if !scopeAllowsClient(scope, "client-a") {
		t.Fatal("active client must see its own event")
	}
	if scopeAllowsClient(scope, "client-b") {
		t.Fatal("active client must not see another client's event")
	}
	if scopeAllowsClient(scope, "") {
		t.Fatal("active client must not see unassigned agency event")
	}
}

func TestAgencyScopePreservesSelectedClient(t *testing.T) {
	t.Parallel()

	scope := CalendarScope{StorageAccountID: "agency", CanSelect: true}
	filter := scopeEventFilter(scope, EventFilter{ClientID: "client-b"})
	if filter.ClientID != "client-b" {
		t.Fatalf("agency filter should be preserved, got %q", filter.ClientID)
	}
	input := scopeEventInput(scope, EventInput{ClientID: "client-b"})
	if input.ClientID != "client-b" {
		t.Fatalf("agency input should be preserved, got %q", input.ClientID)
	}
}

func TestCalendarScopeDoesNotExposeStorageAccount(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(CalendarScope{
		StorageAccountID: "agency-secret-scope",
		LockedClientID:   "client-a",
		Clients:          []CalendarScopeClient{{ID: "client-a", Name: "Cliente A"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "agency-secret-scope") || strings.Contains(string(body), "StorageAccount") {
		t.Fatalf("storage account must remain server-side, got %s", body)
	}
}
