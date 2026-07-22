package omnichannel

import "testing"

func TestDueSLAEventsAreMonotonic(t *testing.T) {
	if got := dueSLAEvents(180, 100, 100); len(got) != 1 || got[0] != "warning" {
		t.Fatalf("warning threshold=%v", got)
	}
	got := dueSLAEvents(200, 100, 100)
	if len(got) != 2 || got[0] != "warning" || got[1] != "breached" {
		t.Fatalf("breach threshold=%v", got)
	}
	if got := dueSLAEvents(99, 100, 100); len(got) != 0 {
		t.Fatalf("future event=%v", got)
	}
}
