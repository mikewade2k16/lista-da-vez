package metaads

import "testing"

func TestFilterAdAccountsForSharedClientRequiresExplicitMapping(t *testing.T) {
	t.Parallel()

	clientA := "11111111-1111-4111-8111-111111111111"
	clientB := "22222222-2222-4222-8222-222222222222"
	rows := []AdAccount{
		{ID: "mapped-a", ClientAccountID: &clientA},
		{ID: "mapped-b", ClientAccountID: &clientB},
		{ID: "unmapped"},
	}

	got := filterAdAccountsForViewer(rows, clientA, "agency-account")
	if len(got) != 1 || got[0].ID != "mapped-a" {
		t.Fatalf("filtered rows = %#v, want only mapped-a", got)
	}
}

func TestFilterAdAccountsForConnectionOwnerKeepsAllRows(t *testing.T) {
	t.Parallel()

	client := "11111111-1111-4111-8111-111111111111"
	rows := []AdAccount{{ID: "mapped", ClientAccountID: &client}, {ID: "unmapped"}}
	got := filterAdAccountsForViewer(rows, "agency-account", "agency-account")
	if len(got) != len(rows) {
		t.Fatalf("filtered rows = %d, want %d", len(got), len(rows))
	}
}
