package realtime

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type presenceEntry struct {
	user        PresenceUser
	connections int
}

type PresenceStore struct {
	mu      sync.Mutex
	hub     *Hub
	ttl     time.Duration
	entries map[string]map[string]presenceEntry
}

func NewPresenceStore(hub *Hub, ttl time.Duration) *PresenceStore {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}

	store := &PresenceStore{
		hub:     hub,
		ttl:     ttl,
		entries: map[string]map[string]presenceEntry{},
	}

	go store.cleanupLoop()
	return store
}

func (store *PresenceStore) Join(topic string, user PresenceUser) []PresenceUser {
	topic = strings.TrimSpace(topic)
	user = normalizePresenceUser(user, time.Now().UTC())
	if topic == "" || user.UserID == "" {
		return nil
	}

	shouldPublishJoined := false

	store.mu.Lock()
	topicEntries := store.ensureTopicLocked(topic)
	entry, exists := topicEntries[user.UserID]
	if exists {
		entry.user.DisplayName = user.DisplayName
		entry.user.AvatarPath = user.AvatarPath
		entry.user.UpdatedAt = user.UpdatedAt
		entry.connections++
	} else {
		entry = presenceEntry{user: user, connections: 1}
		shouldPublishJoined = true
	}
	topicEntries[user.UserID] = entry
	snapshot := snapshotPresenceLocked(topicEntries)
	store.mu.Unlock()

	if shouldPublishJoined {
		store.publishPresenceEvent(topic, EventTypePresenceUserJoined, user)
	}

	return snapshot
}

func (store *PresenceStore) Heartbeat(topic string, user PresenceUser) {
	topic = strings.TrimSpace(topic)
	user = normalizePresenceUser(user, time.Now().UTC())
	if topic == "" || user.UserID == "" {
		return
	}

	shouldPublishJoined := false

	store.mu.Lock()
	topicEntries := store.ensureTopicLocked(topic)
	entry, exists := topicEntries[user.UserID]
	if exists {
		entry.user.DisplayName = user.DisplayName
		entry.user.AvatarPath = user.AvatarPath
		entry.user.UpdatedAt = user.UpdatedAt
	} else {
		entry = presenceEntry{user: user, connections: 1}
		shouldPublishJoined = true
	}
	topicEntries[user.UserID] = entry
	store.mu.Unlock()

	if shouldPublishJoined {
		store.publishPresenceEvent(topic, EventTypePresenceUserJoined, user)
	}
}

// LockField marca o usuario como "editando" `fieldKey` no `topic` informado.
//
// Regra de exclusividade: se outro usuario ja esta no mesmo `fieldKey` dentro
// do TTL, a chamada nao atualiza o estado de quem tentou tomar o lock. O store
// republica o lock atual para recuperar clientes que tenham perdido snapshot ou
// evento anterior.
func (store *PresenceStore) LockField(topic string, user PresenceUser, fieldKey string, lockID string) {
	topic = strings.TrimSpace(topic)
	fieldKey = strings.TrimSpace(fieldKey)
	user = normalizePresenceUser(user, time.Now().UTC())
	if topic == "" || user.UserID == "" || fieldKey == "" {
		return
	}
	if strings.TrimSpace(lockID) == "" {
		lockID = fmt.Sprintf("%s:%s:%d", user.UserID, fieldKey, user.UpdatedAt.UnixNano())
	}

	shouldPublishJoined := false
	denied := false
	var currentLocker PresenceUser

	store.mu.Lock()
	topicEntries := store.ensureTopicLocked(topic)
	for otherUserID, other := range topicEntries {
		if otherUserID == user.UserID {
			continue
		}
		if other.user.FieldKey != fieldKey {
			continue
		}
		if user.UpdatedAt.Sub(other.user.UpdatedAt) > store.ttl {
			// lock alheio ja expirou pelo TTL; cleanup vai remover, podemos
			// assumir como livre nessa chamada.
			continue
		}
		denied = true
		currentLocker = other.user
		break
	}

	if denied {
		store.mu.Unlock()
		store.publishPresenceEvent(topic, EventTypePresenceFieldLocked, currentLocker)
		return
	}

	entry, exists := topicEntries[user.UserID]
	if exists {
		entry.user.DisplayName = user.DisplayName
		entry.user.AvatarPath = user.AvatarPath
		entry.user.UpdatedAt = user.UpdatedAt
	} else {
		entry = presenceEntry{user: user, connections: 1}
		shouldPublishJoined = true
	}
	entry.user.FieldKey = fieldKey
	entry.user.LockID = strings.TrimSpace(lockID)
	topicEntries[user.UserID] = entry
	lockedUser := entry.user
	store.mu.Unlock()

	if shouldPublishJoined {
		store.publishPresenceEvent(topic, EventTypePresenceUserJoined, user)
	}
	store.publishPresenceEvent(topic, EventTypePresenceFieldLocked, lockedUser)
}

func (store *PresenceStore) UnlockField(topic string, userID string, fieldKey string) {
	topic = strings.TrimSpace(topic)
	userID = strings.TrimSpace(userID)
	fieldKey = strings.TrimSpace(fieldKey)
	if topic == "" || userID == "" {
		return
	}

	var unlockedUser PresenceUser
	shouldPublish := false

	store.mu.Lock()
	if topicEntries, ok := store.entries[topic]; ok {
		if entry, ok := topicEntries[userID]; ok {
			if fieldKey == "" || entry.user.FieldKey == fieldKey {
				entry.user.FieldKey = ""
				entry.user.LockID = ""
				entry.user.UpdatedAt = time.Now().UTC()
				topicEntries[userID] = entry
				unlockedUser = entry.user
				if fieldKey != "" {
					unlockedUser.FieldKey = fieldKey
				}
				shouldPublish = true
			}
		}
	}
	store.mu.Unlock()

	if shouldPublish {
		store.publishPresenceEvent(topic, EventTypePresenceFieldUnlocked, unlockedUser)
	}
}

func (store *PresenceStore) Leave(topic string, userID string) {
	topic = strings.TrimSpace(topic)
	userID = strings.TrimSpace(userID)
	if topic == "" || userID == "" {
		return
	}

	var leftUser PresenceUser
	shouldPublish := false

	store.mu.Lock()
	if topicEntries, ok := store.entries[topic]; ok {
		if entry, ok := topicEntries[userID]; ok {
			entry.connections--
			if entry.connections <= 0 {
				leftUser = entry.user
				delete(topicEntries, userID)
				shouldPublish = true
			} else {
				entry.user.UpdatedAt = time.Now().UTC()
				topicEntries[userID] = entry
			}

			if len(topicEntries) == 0 {
				delete(store.entries, topic)
			}
		}
	}
	store.mu.Unlock()

	if shouldPublish {
		store.publishPresenceEvent(topic, EventTypePresenceUserLeft, leftUser)
	}
}

func (store *PresenceStore) Snapshot(topic string) []PresenceUser {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return nil
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	return snapshotPresenceLocked(store.entries[topic])
}

func (store *PresenceStore) ensureTopicLocked(topic string) map[string]presenceEntry {
	if _, ok := store.entries[topic]; !ok {
		store.entries[topic] = map[string]presenceEntry{}
	}
	return store.entries[topic]
}

func (store *PresenceStore) cleanupLoop() {
	interval := store.ttl / 2
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for now := range ticker.C {
		store.removeExpired(now.UTC())
	}
}

func (store *PresenceStore) removeExpired(now time.Time) {
	type expiredPresence struct {
		topic string
		user  PresenceUser
	}

	expired := make([]expiredPresence, 0)

	store.mu.Lock()
	for topic, topicEntries := range store.entries {
		for userID, entry := range topicEntries {
			if now.Sub(entry.user.UpdatedAt) > store.ttl {
				expired = append(expired, expiredPresence{topic: topic, user: entry.user})
				delete(topicEntries, userID)
			}
		}
		if len(topicEntries) == 0 {
			delete(store.entries, topic)
		}
	}
	store.mu.Unlock()

	for _, item := range expired {
		store.publishPresenceEvent(item.topic, EventTypePresenceUserLeft, item.user)
	}
}

func (store *PresenceStore) publishPresenceEvent(topic string, eventType string, user PresenceUser) {
	if store.hub == nil {
		return
	}

	event := presenceEventForTopic(topic, eventType, user)
	if event.Type == "" {
		return
	}

	store.hub.Publish(topic, event)
}

func normalizePresenceUser(user PresenceUser, now time.Time) PresenceUser {
	user.UserID = strings.TrimSpace(user.UserID)
	user.DisplayName = strings.TrimSpace(user.DisplayName)
	user.AvatarPath = strings.TrimSpace(user.AvatarPath)
	user.FieldKey = strings.TrimSpace(user.FieldKey)
	user.LockID = strings.TrimSpace(user.LockID)
	user.UpdatedAt = now
	return user
}

func snapshotPresenceLocked(entries map[string]presenceEntry) []PresenceUser {
	if len(entries) == 0 {
		return []PresenceUser{}
	}

	snapshot := make([]PresenceUser, 0, len(entries))
	for _, entry := range entries {
		snapshot = append(snapshot, entry.user)
	}

	sort.Slice(snapshot, func(i, j int) bool {
		return snapshot[i].DisplayName < snapshot[j].DisplayName
	})

	return snapshot
}

func presenceEventForTopic(topic string, eventType string, user PresenceUser) Event {
	topic = strings.TrimSpace(topic)
	eventType = strings.TrimSpace(eventType)
	if topic == "" || eventType == "" || user.UserID == "" {
		return Event{}
	}

	event := Event{
		Type:        eventType,
		UserID:      user.UserID,
		DisplayName: user.DisplayName,
		AvatarPath:  user.AvatarPath,
		FieldKey:    user.FieldKey,
		LockID:      user.LockID,
		SavedAt:     time.Now().UTC(),
	}

	switch {
	case strings.HasPrefix(topic, "presence:board:"):
		event.BoardID = strings.TrimPrefix(topic, "presence:board:")
	case strings.HasPrefix(topic, "presence:task:"):
		event.TaskID = strings.TrimPrefix(topic, "presence:task:")
	default:
		return Event{}
	}

	return event
}
