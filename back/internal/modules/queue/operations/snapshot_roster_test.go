package operations

import "testing"

// O snapshot precisa expor o roster ENXUTO para que a faixa de consultores
// funcione para papeis que podem ler a operacao mas nao tem a permissao de
// gestao de consultores (`/v1/consultants`). A projecao NAO pode vazar meta,
// comissao ou e-mail de acesso.
func TestBuildSnapshotViewExposesLeanRoster(t *testing.T) {
	roster := []ConsultantProfile{
		{
			ID:             "person-1",
			StoreID:        "store-1",
			Name:           "Daiane C.",
			Role:           "Atendimento",
			Initials:       "DC",
			Color:          "#168aad",
			MonthlyGoal:    50000,
			CommissionRate: 3.5,
		},
	}

	view := buildSnapshotView("store-1", "Loja 1", roster, SnapshotState{}, nil)

	if len(view.Roster) != 1 {
		t.Fatalf("expected 1 roster member, got %d", len(view.Roster))
	}

	member := view.Roster[0]
	if member.ID != "person-1" || member.StoreID != "store-1" || member.Name != "Daiane C." {
		t.Fatalf("unexpected roster identity fields: %+v", member)
	}
	if member.Initials != "DC" || member.Color != "#168aad" || member.Role != "Atendimento" {
		t.Fatalf("unexpected roster display fields: %+v", member)
	}
}

// buildSnapshotView deve embutir GoalStats por consultor na waitingList a partir
// do map de ponte com o CRM/ERP. Consultor sem entrada no map fica com
// GoalStats=nil (degradacao graciosa, "todos veem de todos" so onde ha dado).
func TestBuildSnapshotViewFillsGoalStatsOnWaitingList(t *testing.T) {
	roster := []ConsultantProfile{
		{ID: "person-1", StoreID: "store-1", Name: "Daiane C.", Role: "Atendimento", Initials: "DC", Color: "#168aad"},
		{ID: "person-2", StoreID: "store-1", Name: "Bruno A.", Role: "Atendimento", Initials: "BA", Color: "#222222"},
	}

	state := SnapshotState{
		WaitingList: []QueueStateItem{
			{ConsultantID: "person-1", QueueJoinedAt: 1000},
			{ConsultantID: "person-2", QueueJoinedAt: 2000},
		},
	}

	goalStats := map[string]GoalStats{
		"person-1": {
			MonthlyGoal:     50000,
			SoldValue:       30000,
			RemainingToGoal: 20000,
			Progress:        60,
			HasGoal:         true,
		},
	}

	view := buildSnapshotView("store-1", "Loja 1", roster, state, goalStats)

	if len(view.WaitingList) != 2 {
		t.Fatalf("expected 2 waiting entries, got %d", len(view.WaitingList))
	}

	first := view.WaitingList[0]
	if first.ID != "person-1" {
		t.Fatalf("unexpected first waiting entry: %+v", first)
	}
	if first.GoalStats == nil {
		t.Fatalf("expected GoalStats on person-1, got nil")
	}
	if !first.GoalStats.HasGoal || first.GoalStats.Progress != 60 || first.GoalStats.MonthlyGoal != 50000 {
		t.Fatalf("unexpected GoalStats on person-1: %+v", first.GoalStats)
	}
	if first.GoalStats.SoldValue != 30000 || first.GoalStats.RemainingToGoal != 20000 {
		t.Fatalf("unexpected GoalStats values on person-1: %+v", first.GoalStats)
	}

	second := view.WaitingList[1]
	if second.ID != "person-2" {
		t.Fatalf("unexpected second waiting entry: %+v", second)
	}
	if second.GoalStats != nil {
		t.Fatalf("expected nil GoalStats on person-2 (no map entry), got %+v", second.GoalStats)
	}
}
