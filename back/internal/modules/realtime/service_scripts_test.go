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

func TestPresenceLockersRespectOwnerAndTTL(t *testing.T) {
	t.Parallel()
	store := &PresenceStore{hub: NewHub(), ttl: time.Minute, entries: map[string]map[string]presenceEntry{}}
	topic := presenceScriptTopic("s1")
	store.LockField(topic, PresenceUser{UserID: "ana", DisplayName: "Ana"}, "cell:r1:audio", "")
	if got := store.FieldLocker(topic, "cell:r1:audio", "bob"); got != "Ana" {
		t.Fatalf("bob should see Ana's lock, got %q", got)
	}
	if got := store.FieldLocker(topic, "cell:r1:audio", "ana"); got != "" {
		t.Fatalf("own lock must not block, got %q", got)
	}
	if got := store.FieldLocker(topic, "cell:r1:video", "bob"); got != "" {
		t.Fatalf("other cell is free, got %q", got)
	}
	if got := store.AnyLocker(topic, "bob"); got != "Ana" {
		t.Fatalf("any locker = %q", got)
	}
	store.UnlockField(topic, "ana", "cell:r1:audio")
	if got := store.AnyLocker(topic, "bob"); got != "" {
		t.Fatalf("unlocked must be free, got %q", got)
	}
}

func TestPublishScriptCellCarriesContent(t *testing.T) {
	t.Parallel()
	hub := NewHub()
	service := &Service{hub: hub}
	sub := hub.Subscribe(scriptsAccountTopic("agency"), 4)
	defer sub.Close()
	service.PublishScriptEvent(context.Background(), toolsmodule.ScriptEvent{
		Type: toolsmodule.ScriptEventCell, AccountID: "agency", ResourceID: "s1", UserID: "ana",
		Field: "audio", RowID: "r1", Value: "<p>oi</p>", UpdatedAt: "2026-11-12T10:00:00Z",
	})
	select {
	case event := <-sub.Events():
		if event.Payload["value"] != "<p>oi</p>" || event.Payload["rowId"] != "r1" || event.Payload["field"] != "audio" || event.UserID != "ana" {
			t.Fatalf("cell event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("cell event not delivered")
	}
}

func TestScriptPresenceBroadcastsLockChanges(t *testing.T) {
	t.Parallel()
	hub := NewHub()
	store := &PresenceStore{hub: hub, ttl: time.Minute, entries: map[string]map[string]presenceEntry{}}
	topic := presenceScriptTopic("s1")
	sub := hub.Subscribe(topic, 8)
	defer sub.Close()
	store.LockField(topic, PresenceUser{UserID: "ana", DisplayName: "Ana"}, "cell:r1:audio", "")
	deadline := time.After(time.Second)
	for {
		select {
		case event := <-sub.Events():
			if event.Type == EventTypePresenceFieldLocked {
				if event.FieldKey != "cell:r1:audio" || event.ResourceID != "s1" {
					t.Fatalf("lock event = %+v", event)
				}
				return
			}
		case <-deadline:
			t.Fatal("script presence lock was not broadcast")
		}
	}
}
