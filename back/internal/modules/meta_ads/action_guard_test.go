package metaads

import (
	"testing"
	"time"
)

func TestActionGuardHashCoversConnectionMappingPolicyAndCampaignState(t *testing.T) {
	t.Parallel()

	base := actionGuardTestSnapshot()
	if err := base.calculateHashes(actionTestAccount, actionTestResource, "request-hash"); err != nil {
		t.Fatal(err)
	}
	if len(base.Hash) != 64 || len(base.AdAccountHash) != 64 ||
		len(base.PolicyHash) != 64 || len(base.CampaignHash) != 64 {
		t.Fatalf("hashes incompletos: %#v", base)
	}

	tests := []struct {
		name   string
		mutate func(*actionGuardSnapshot)
	}{
		{name: "connection revision", mutate: func(snapshot *actionGuardSnapshot) {
			snapshot.ConnectionRevision = "revision-2"
		}},
		{name: "client mapping", mutate: func(snapshot *actionGuardSnapshot) {
			clientID := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
			snapshot.AdAccount.ClientAccountID = &clientID
		}},
		{name: "ad account timestamp", mutate: func(snapshot *actionGuardSnapshot) {
			snapshot.AdAccount.UpdatedAt = snapshot.AdAccount.UpdatedAt.Add(time.Second)
		}},
		{name: "policy cap", mutate: func(snapshot *actionGuardSnapshot) {
			capValue := 51.0
			snapshot.Policy.MaxDailyBudget = &capValue
		}},
		{name: "policy timestamp", mutate: func(snapshot *actionGuardSnapshot) {
			snapshot.Policy.UpdatedAt = snapshot.Policy.UpdatedAt.Add(time.Second)
		}},
		{name: "campaign status", mutate: func(snapshot *actionGuardSnapshot) {
			snapshot.Campaign.Status = "PAUSED"
		}},
		{name: "campaign sync timestamp", mutate: func(snapshot *actionGuardSnapshot) {
			snapshot.Campaign.SyncedAt = snapshot.Campaign.SyncedAt.Add(time.Second)
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			changed := actionGuardTestSnapshot()
			test.mutate(&changed)
			if err := changed.calculateHashes(actionTestAccount, actionTestResource, "request-hash"); err != nil {
				t.Fatal(err)
			}
			if changed.Hash == base.Hash {
				t.Fatalf("guard hash nao mudou para %s", test.name)
			}
		})
	}
}

func TestActionGuardHashNormalizesBudgetToMinorUnits(t *testing.T) {
	t.Parallel()
	first := actionGuardTestSnapshot()
	second := actionGuardTestSnapshot()
	firstBudget := 10.001
	secondBudget := 10.004
	first.Campaign.DailyBudget = &firstBudget
	second.Campaign.DailyBudget = &secondBudget
	if err := first.calculateHashes(actionTestAccount, actionTestResource, "request-hash"); err != nil {
		t.Fatal(err)
	}
	if err := second.calculateHashes(actionTestAccount, actionTestResource, "request-hash"); err != nil {
		t.Fatal(err)
	}
	if first.Hash != second.Hash {
		t.Fatal("valores equivalentes em minor units devem produzir o mesmo guard hash")
	}
}

func actionGuardTestSnapshot() actionGuardSnapshot {
	clientID := actionTestAccount
	dailyBudget := 10.0
	maxDaily := 50.0
	now := time.Date(2026, time.August, 18, 12, 0, 0, 123000000, time.UTC)
	return actionGuardSnapshot{
		ConnectionID:       actionTestConnection,
		ConnectionRevision: actionTestRevision,
		AdAccount: AdAccount{
			ID:        "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
			AccountID: actionTestResource, ConnectionID: actionTestConnection,
			MetaAdAccountID: "act_123", ClientAccountID: &clientID,
			Name: "Conta", Currency: "BRL", Status: "ACTIVE", IsCurrent: true,
			CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
		},
		Policy: &ActionPolicy{
			ID:          "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
			AccountID:   actionTestResource,
			AdAccountID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
			Currency:    "BRL", MaxDailyBudget: &maxDaily,
			AllowCreate: false, AllowDuplicate: false, AllowResume: false,
			CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
		},
		Campaign: &Campaign{
			ID:             "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee",
			AccountID:      actionTestResource,
			AdAccountID:    "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
			MetaCampaignID: actionTestMetaCampaign, Name: "Campanha",
			Objective: "OUTCOME_TRAFFIC", Status: "ACTIVE",
			DailyBudget: &dailyBudget, IsCurrent: true, SyncedAt: now,
		},
	}
}
