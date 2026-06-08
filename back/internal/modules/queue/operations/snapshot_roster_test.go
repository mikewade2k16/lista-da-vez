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

	view := buildSnapshotView("store-1", "Loja 1", roster, SnapshotState{})

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
