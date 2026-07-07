package calendar

import (
	"context"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	platformmodules "github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

const (
	// relationModule/relationResourceEvent identificam o vinculo calendario<->task em
	// tasks.task_relations (contrato C10). Mesmos valores usados em task_link.go.
	relationModule        = "calendar"
	relationResourceEvent = "event"
)

// relationResolver resolve os vinculos do modulo calendar para o expand de relations do
// tasks (interface platformmodules.RelationResolver). Le SO calendar.events, escopado por
// account (nunca vaza evento de outra conta) e numa UNICA query (sem N+1).
type relationResolver struct {
	pool *pgxpool.Pool
}

// NewRelationResolver cria o resolver do calendar para registrar no RelationRegistry
// (app.go). Espelha o padrao dos resolvers de erp/operations.
func NewRelationResolver(pool *pgxpool.Pool) platformmodules.RelationResolver {
	return &relationResolver{pool: pool}
}

func (r *relationResolver) ModuleID() string { return relationModule }

// ResolveMany resolve os eventos referenciados (resource_type='event') numa UNICA query,
// escopada por account. Refs de outro tipo / eventos inexistentes / de outra conta viram
// status "unknown".
func (r *relationResolver) ResolveMany(ctx context.Context, accountID string, refs []platformmodules.RelationRef) ([]platformmodules.RelationResult, error) {
	results := make([]platformmodules.RelationResult, 0, len(refs))
	account := strings.TrimSpace(accountID)
	if r.pool == nil || account == "" || len(refs) == 0 {
		for _, ref := range refs {
			results = append(results, unknownCalendarRelation(ref))
		}
		return results, nil
	}

	seen := make(map[string]struct{})
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		if r.normalizeType(ref.ResourceType) != relationResourceEvent {
			continue
		}
		id := normalizeUUID(ref.ResourceID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	matches, err := r.resolveEvents(ctx, account, ids)
	if err != nil {
		return nil, err
	}

	for _, ref := range refs {
		id := normalizeUUID(ref.ResourceID)
		if id == "" || r.normalizeType(ref.ResourceType) != relationResourceEvent {
			results = append(results, unknownCalendarRelation(ref))
			continue
		}
		if match, ok := matches[id]; ok {
			results = append(results, resultForCalendar(ref, match))
			continue
		}
		results = append(results, unknownCalendarRelation(ref))
	}
	return results, nil
}

// calendarRelationMatch e a projecao minima do evento usada no rotulo/deep-link.
type calendarRelationMatch struct {
	id     string
	date   string
	title  string
	status string
}

func (r *relationResolver) resolveEvents(ctx context.Context, accountID string, ids []string) (map[string]calendarRelationMatch, error) {
	out := make(map[string]calendarRelationMatch)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		select id::text, event_date::text, title, status
		from calendar.events
		where account_id = $1::uuid and id = any($2::uuid[])
	`, accountID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m calendarRelationMatch
		if err := rows.Scan(&m.id, &m.date, &m.title, &m.status); err != nil {
			return nil, err
		}
		out[m.id] = m
	}
	return out, rows.Err()
}

func (r *relationResolver) normalizeType(resourceType string) string {
	switch strings.TrimSpace(strings.ToLower(resourceType)) {
	case "event", "events", "calendar_event":
		return relationResourceEvent
	default:
		return ""
	}
}

func resultForCalendar(ref platformmodules.RelationRef, match calendarRelationMatch) platformmodules.RelationResult {
	status := strings.TrimSpace(match.status)
	if status == "" {
		status = "planejado"
	}
	return platformmodules.RelationResult{
		ModuleID:     ref.ModuleID,
		ResourceType: ref.ResourceType,
		ResourceID:   ref.ResourceID,
		Label:        calendarRelationLabel(match.date, match.title),
		URL:          calendarRelationURL(match.date),
		Status:       status,
		Metadata: map[string]any{
			"date":   match.date,
			"title":  match.title,
			"status": status,
		},
	}
}

func unknownCalendarRelation(ref platformmodules.RelationRef) platformmodules.RelationResult {
	return platformmodules.RelationResult{
		ModuleID:     ref.ModuleID,
		ResourceType: ref.ResourceType,
		ResourceID:   ref.ResourceID,
		Status:       "unknown",
		Metadata:     map[string]any{"status": "unknown"},
	}
}

// calendarRelationLabel formata o rotulo cacheado da relation (mesmo formato que o
// resolver produz na atualizacao): "<date> - <title>". Usado tambem por task_link.go.
func calendarRelationLabel(date, title string) string {
	return strings.TrimSpace(date) + " - " + strings.TrimSpace(title)
}

// calendarRelationURL e o deep-link do evento no painel (contrato C10): /calendario?date=<date>.
func calendarRelationURL(date string) string {
	date = strings.TrimSpace(date)
	if date == "" {
		return "/calendario"
	}
	params := url.Values{}
	params.Set("date", date)
	return "/calendario?" + params.Encode()
}
