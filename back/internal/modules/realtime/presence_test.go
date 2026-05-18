package realtime

import (
	"testing"
	"time"
)

// drainEvents le todos os eventos pendentes de uma subscription dentro de `timeout`. Devolve
// quantos eventos chegaram por tipo — util para asserts de "publicou X, nao publicou Y".
func drainEvents(t *testing.T, sub *Subscription, timeout time.Duration) map[string]int {
	t.Helper()
	counts := map[string]int{}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				return counts
			}
			counts[event.Type]++
		case <-deadline.C:
			return counts
		}
	}
}

func TestPresenceJoin_PublishesUserJoinedOnce(t *testing.T) {
	hub := NewHub()
	store := NewPresenceStore(hub, 30*time.Second)
	topic := "presence:task:t1"
	sub := hub.Subscribe(topic, 8)
	defer sub.Close()

	store.Join(topic, PresenceUser{UserID: "u1", DisplayName: "Alice"})
	store.Join(topic, PresenceUser{UserID: "u1", DisplayName: "Alice"}) // segunda conexao do MESMO user

	events := drainEvents(t, sub, 100*time.Millisecond)
	if events[EventTypePresenceUserJoined] != 1 {
		t.Errorf("user_joined deveria publicar apenas na PRIMEIRA conexao do user; got %d eventos", events[EventTypePresenceUserJoined])
	}
}

func TestPresenceLockField_ExclusiveByFieldKey(t *testing.T) {
	hub := NewHub()
	store := NewPresenceStore(hub, 30*time.Second)
	topic := "presence:task:t1"
	sub := hub.Subscribe(topic, 8)
	defer sub.Close()

	// User A entra e trava o campo "title".
	store.Join(topic, PresenceUser{UserID: "userA", DisplayName: "Alice"})
	store.LockField(topic, PresenceUser{UserID: "userA", DisplayName: "Alice"}, "title", "lock-A")

	// User B tenta travar o MESMO campo. O estado de B nao muda, mas o servidor
	// republica o lock de A para recuperar clientes com snapshot/evento defasado.
	store.Join(topic, PresenceUser{UserID: "userB", DisplayName: "Bob"})
	store.LockField(topic, PresenceUser{UserID: "userB", DisplayName: "Bob"}, "title", "lock-B")

	events := drainEvents(t, sub, 100*time.Millisecond)
	if events[EventTypePresenceFieldLocked] != 2 {
		t.Errorf("LockField deve publicar o lock inicial e republicar o lock atual quando outro user tenta tomar; got %d", events[EventTypePresenceFieldLocked])
	}

	// O snapshot ainda deve ter ambos os users, mas so userA com FieldKey preenchido.
	snapshot := store.Snapshot(topic)
	if len(snapshot) != 2 {
		t.Fatalf("snapshot deve ter ambos users (A e B), pois Join foi aceito; got %d", len(snapshot))
	}
	for _, user := range snapshot {
		if user.UserID == "userA" && user.FieldKey != "title" {
			t.Errorf("userA deve manter FieldKey=title; got %q", user.FieldKey)
		}
		if user.UserID == "userB" && user.FieldKey != "" {
			t.Errorf("userB nao pode ter FieldKey, lock foi recusado; got %q", user.FieldKey)
		}
	}
}

func TestPresenceLockField_OwnerCanReclaim(t *testing.T) {
	hub := NewHub()
	store := NewPresenceStore(hub, 30*time.Second)
	topic := "presence:task:t1"

	store.Join(topic, PresenceUser{UserID: "userA"})
	store.LockField(topic, PresenceUser{UserID: "userA"}, "title", "lock-1")
	// Mesma user reentrando no mesmo campo (ex: heartbeat refresh) — deve continuar valido.
	store.LockField(topic, PresenceUser{UserID: "userA"}, "title", "lock-1-renewed")

	snapshot := store.Snapshot(topic)
	if len(snapshot) != 1 || snapshot[0].FieldKey != "title" {
		t.Fatalf("userA deve manter lock no field title apos reaffirmar; snapshot=%+v", snapshot)
	}
	if snapshot[0].LockID != "lock-1-renewed" {
		t.Errorf("LockID deveria atualizar para o ultimo valor; got %q", snapshot[0].LockID)
	}
}

func TestPresenceUnlockField_ReleasesAndPublishes(t *testing.T) {
	hub := NewHub()
	store := NewPresenceStore(hub, 30*time.Second)
	topic := "presence:task:t1"
	sub := hub.Subscribe(topic, 8)
	defer sub.Close()

	store.Join(topic, PresenceUser{UserID: "userA"})
	store.LockField(topic, PresenceUser{UserID: "userA"}, "title", "lock-1")
	store.UnlockField(topic, "userA", "title")

	events := drainEvents(t, sub, 100*time.Millisecond)
	if events[EventTypePresenceFieldUnlocked] != 1 {
		t.Errorf("UnlockField deve publicar field_unlocked; got %d", events[EventTypePresenceFieldUnlocked])
	}

	snapshot := store.Snapshot(topic)
	if len(snapshot) != 1 || snapshot[0].FieldKey != "" {
		t.Fatalf("apos unlock, user permanece mas FieldKey vazio; snapshot=%+v", snapshot)
	}
}

func TestPresenceLeave_DecrementsConnections(t *testing.T) {
	hub := NewHub()
	store := NewPresenceStore(hub, 30*time.Second)
	topic := "presence:task:t1"
	sub := hub.Subscribe(topic, 8)
	defer sub.Close()

	store.Join(topic, PresenceUser{UserID: "userA"})
	store.Join(topic, PresenceUser{UserID: "userA"}) // 2 conexoes
	store.Leave(topic, "userA")

	// Ainda ha 1 conexao, user_left nao deve ter sido publicado.
	events := drainEvents(t, sub, 50*time.Millisecond)
	if events[EventTypePresenceUserLeft] != 0 {
		t.Errorf("Leave com conexoes residuais nao deve publicar user_left; got %d", events[EventTypePresenceUserLeft])
	}

	if got := len(store.Snapshot(topic)); got != 1 {
		t.Errorf("snapshot ainda deve conter userA com 1 conexao; got %d entries", got)
	}

	store.Leave(topic, "userA")
	events = drainEvents(t, sub, 50*time.Millisecond)
	if events[EventTypePresenceUserLeft] != 1 {
		t.Errorf("Leave ate zerar deve publicar user_left; got %d", events[EventTypePresenceUserLeft])
	}
	if got := len(store.Snapshot(topic)); got != 0 {
		t.Errorf("apos ultimo Leave, topico fica vazio; got %d entries", got)
	}
}

func TestPresenceTTL_ExpiresStaleEntries(t *testing.T) {
	hub := NewHub()
	// TTL curto para o teste rodar rapido sem time.Sleep absurdo.
	ttl := 60 * time.Millisecond
	store := NewPresenceStore(hub, ttl)
	topic := "presence:task:t1"

	store.Join(topic, PresenceUser{UserID: "userA"})
	if got := len(store.Snapshot(topic)); got != 1 {
		t.Fatalf("preparacao: userA deve estar presente; got %d", got)
	}

	// Avanca o relogio simulando expiracao via call direto a removeExpired.
	store.removeExpired(time.Now().UTC().Add(2 * ttl))

	if got := len(store.Snapshot(topic)); got != 0 {
		t.Errorf("apos removeExpired, entrada deveria ter sido descartada; got %d", got)
	}
}
