package realtime

import (
	"context"
	"reflect"
	"testing"
	"time"

	toolsmodule "github.com/mikewade2k16/lista-da-vez/back/internal/modules/tools"
)

func TestScriptPublishAccountIDs(t *testing.T) {
	t.Parallel()
	got := scriptPublishAccountIDs(toolsmodule.ScriptEvent{
		Type:             toolsmodule.ScriptEventSaved,
		AccountID:        "agency",
		ClientAccountIDs: []string{"client-a", "", "client-a", "agency", "client-b"},
	})
	if want := []string{"agency", "client-a", "client-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("saved scopes: got %v want %v", got, want)
	}
	got = scriptPublishAccountIDs(toolsmodule.ScriptEvent{
		Type:             toolsmodule.ScriptEventReordered,
		AccountID:        "agency",
		ClientAccountIDs: []string{"client-a"},
	})
	if want := []string{"agency"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reorder must stay in the owner account: got %v", got)
	}
}

func TestPublishScriptEventReachesOnlyScopedTopics(t *testing.T) {
	t.Parallel()
	hub := NewHub()
	service := &Service{hub: hub}
	agencySub := hub.Subscribe(scriptsAccountTopic("agency"), 4)
	defer agencySub.Close()
	clientSub := hub.Subscribe(scriptsAccountTopic("client-a"), 4)
	defer clientSub.Close()
	otherSub := hub.Subscribe(scriptsAccountTopic("client-b"), 4)
	defer otherSub.Close()

	service.PublishScriptEvent(context.Background(), toolsmodule.ScriptEvent{
		Type: toolsmodule.ScriptEventSaved, AccountID: "agency", ClientAccountIDs: []string{"client-a"},
		ResourceID: "script-1", UpdatedAt: "2026-11-12T10:00:00Z",
	})
	for name, ch := range map[string]<-chan Event{"agency": agencySub.Events(), "client-a": clientSub.Events()} {
		select {
		case event := <-ch:
			if event.ResourceID != "script-1" || event.Payload["updatedAt"] != "2026-11-12T10:00:00Z" {
				t.Fatalf("%s got %+v", name, event)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s did not receive the event", name)
		}
	}
	select {
	case event := <-otherSub.Events():
		t.Fatalf("unrelated client received %+v", event)
	case <-time.After(50 * time.Millisecond):
	}
}
