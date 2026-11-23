package observability

import (
	"strings"
	"time"
)

const heartbeatKind = "heartbeat"

// presenceEvent é um heartbeat do front: não vai para obs.frontend_events, só
// atualiza obs.presence (uma linha por aba).
type presenceEvent struct {
	PageViewID string
	UserID     string
	Page       string
	UserAgent  string
	SeenAt     time.Time
}

const upsertPresenceSQL = `
insert into obs.presence (page_view_id, user_id, page, user_agent, started_at, last_seen)
values ($1, $2, $3, $4, $5, $5)
on conflict (page_view_id) do update set
  user_id = excluded.user_id,
  page = excluded.page,
  user_agent = excluded.user_agent,
  last_seen = greatest(obs.presence.last_seen, excluded.last_seen)`

// parseHeartbeat monta o evento de presença. A rota atual vem em name; sem name
// válido, usa a página do lote. Sem usuário autenticado não há presença.
func parseHeartbeat(pageViewID, batchPage string, item FrontendEvent, userID, userAgent string, now time.Time) (presenceEvent, bool) {
	if strings.TrimSpace(userID) == "" {
		return presenceEvent{}, false
	}
	page := batchPage
	if name := strings.TrimSpace(item.Name); strings.HasPrefix(name, "/") {
		if normalized := NormalizePath(SanitizeURLPath(name, 300)); normalized != "" {
			page = normalized
		}
	}
	return presenceEvent{
		PageViewID: pageViewID,
		UserID:     truncate(userID, 120),
		Page:       page,
		UserAgent:  truncate(userAgent, 300),
		SeenAt:     now,
	}, true
}

func (rt *Runtime) recordHeartbeat(pageViewID, page string, item FrontendEvent, userID, userAgent string, now time.Time) bool {
	event, ok := parseHeartbeat(pageViewID, page, item, userID, userAgent, now)
	if !ok {
		return false
	}
	rt.recorder.enqueue(event)
	return true
}

// addPresence guarda só o último heartbeat de cada aba no lote.
func (b *eventBatch) addPresence(event presenceEvent) {
	if b.presence == nil {
		b.presence = make(map[string]presenceEvent)
	}
	b.presence[event.PageViewID] = event
}
