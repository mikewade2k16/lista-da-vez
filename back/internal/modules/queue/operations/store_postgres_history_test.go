package operations

import (
	"strings"
	"testing"
)

func TestBuildServiceHistoryQueryWithoutWindow(t *testing.T) {
	query, args := buildServiceHistoryQuery("store-1", 0)

	if strings.Contains(query, "finished_at >=") {
		t.Fatal("expected no finished_at window predicate when sinceMillis=0")
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 arg (storeID) without window, got %d", len(args))
	}
	if !strings.Contains(query, "order by started_at asc") {
		t.Fatal("expected order by started_at asc to be preserved")
	}
}

func TestBuildServiceHistoryQueryWithWindow(t *testing.T) {
	query, args := buildServiceHistoryQuery("store-1", 1234567890)

	if !strings.Contains(query, "and finished_at >= $2") {
		t.Fatal("expected finished_at >= $2 window predicate when sinceMillis>0")
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args (storeID, sinceMillis) with window, got %d", len(args))
	}
	if !strings.Contains(query, "order by started_at asc") {
		t.Fatal("expected order by started_at asc to be preserved")
	}
}
