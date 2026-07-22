package realtime

import (
	"reflect"
	"testing"

	calendarmodule "github.com/mikewade2k16/lista-da-vez/back/internal/modules/calendar"
)

func TestCalendarPublishAccountIDsIncludesAgencyAndAffectedClients(t *testing.T) {
	t.Parallel()

	got := calendarPublishAccountIDs(calendarmodule.RealtimeEvent{
		AccountID: "agency",
		ClientIDs: []string{"client-a", "client-a", "", "client-b"},
	})
	want := []string{"agency", "client-a", "client-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected publish scopes: got %v want %v", got, want)
	}
}
