package consultants

import "testing"

func TestConsultantViewExposesNickname(t *testing.T) {
	view := (Consultant{ID: "consultant-1", Name: "Daiane Caroline", Nick: "Daiane C."}).View()

	if view.Nick != "Daiane C." {
		t.Fatalf("expected nickname in consultant view, got %q", view.Nick)
	}
}
