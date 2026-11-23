package observability

import (
	"errors"
	"math"
	"regexp"
	"strings"
	"time"
)

// FrontendBatch é o corpo de POST /v1/obs/collect.
type FrontendBatch struct {
	PageViewID string          `json:"pageViewId"`
	Page       string          `json:"page"`
	Events     []FrontendEvent `json:"events"`
}

type FrontendEvent struct {
	Kind          string         `json:"kind"`
	Name          string         `json:"name"`
	Value         *float64       `json:"value,omitempty"`
	Status        *int           `json:"status,omitempty"`
	CorrelationID string         `json:"correlationId,omitempty"`
	Detail        map[string]any `json:"detail,omitempty"`
	At            any            `json:"at,omitempty"`
}

const MaxFrontendEvents = 100

var (
	ErrInvalidFrontendBatch = errors.New("invalid frontend batch")
	pageViewIDPattern       = regexp.MustCompile(`^[A-Za-z0-9_-]{8,40}$`)
	correlationPattern      = regexp.MustCompile(`^[A-Za-z0-9._-]{8,80}$`)
	frontendKinds           = map[string]bool{"vital": true, "pageload": true, "js_error": true, "api_call": true, "api_failure": true}
)

// ValidCorrelationID informa se um X-Request-ID recebido pode ser reaproveitado.
func ValidCorrelationID(value string) bool { return correlationPattern.MatchString(value) }

// RecordFrontend valida, sanitiza e enfileira eventos do navegador. Eventos
// inválidos são ignorados um a um; o lote só falha se o envelope for inválido.
func (rt *Runtime) RecordFrontend(batch FrontendBatch, userID, userAgent string) (int, error) {
	if rt == nil {
		return 0, nil
	}
	if !pageViewIDPattern.MatchString(batch.PageViewID) || len(batch.Events) > MaxFrontendEvents {
		return 0, ErrInvalidFrontendBatch
	}
	page := NormalizePath(SanitizeURLPath(strings.TrimSpace(batch.Page), 300))
	if page == "" || !strings.HasPrefix(page, "/") {
		return 0, ErrInvalidFrontendBatch
	}
	now := time.Now().UTC()
	accepted := 0
	for _, item := range batch.Events {
		event, ok := sanitizeFrontendEvent(item, now)
		if !ok {
			continue
		}
		event.PageViewID = batch.PageViewID
		event.Page = page
		event.UserID = userID
		event.UserAgent = truncate(userAgent, 300)
		rt.recorder.enqueue(event)
		accepted++
	}
	return accepted, nil
}

func sanitizeFrontendEvent(item FrontendEvent, now time.Time) (frontendEvent, bool) {
	kind := strings.TrimSpace(item.Kind)
	if !frontendKinds[kind] {
		return frontendEvent{}, false
	}
	if item.Value != nil && (math.IsNaN(*item.Value) || math.IsInf(*item.Value, 0)) {
		return frontendEvent{}, false
	}
	occurredAt := now
	if at, ok := parseEventTime(item.At); ok {
		if at.After(now.Add(-24*time.Hour)) && at.Before(now.Add(time.Minute)) {
			occurredAt = at
		}
	}
	event := frontendEvent{
		OccurredAt: occurredAt,
		Kind:       kind,
		Name:       SanitizeText(strings.TrimSpace(item.Name), 300),
		Value:      item.Value,
		Status:     item.Status,
	}
	if kind == "api_call" || kind == "api_failure" {
		event.Name = NormalizePath(event.Name)
	}
	if ValidCorrelationID(item.CorrelationID) {
		event.CorrelationID = item.CorrelationID
	}
	if len(item.Detail) > 0 {
		if sanitized, ok := SanitizeValue(item.Detail, 0).(map[string]any); ok {
			event.Detail = sanitized
		}
	}
	return event, true
}

// parseEventTime aceita epoch em ms (número) ou ISO-8601 (string).
func parseEventTime(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case float64:
		return time.UnixMilli(int64(typed)).UTC(), true
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, typed)
		return parsed.UTC(), err == nil
	default:
		return time.Time{}, false
	}
}
