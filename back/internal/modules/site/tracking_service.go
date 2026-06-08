package site

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// IngestTracking recebe um lote de eventos de tracking ja autenticado pelo
// handler HTTP e persiste os eventos validos com idempotencia por source_event_id.
func (s *Service) IngestTracking(
	ctx context.Context,
	source WebhookSourceView,
	payloadSource string,
	sentAt string,
	batchID string,
	rawEvents []map[string]any,
) (TrackingIngestResponse, error) {
	if source.EntityType != "tracking" {
		return TrackingIngestResponse{}, errors.New("source is not configured for tracking")
	}
	if s.tracking == nil {
		return TrackingIngestResponse{}, errors.New("tracking repository is not configured")
	}

	events := make([]TrackingEventInput, 0, len(rawEvents))
	skipped := 0
	for _, raw := range rawEvents {
		event, ok := normalizeTrackingWebhookEvent(raw)
		if !ok {
			skipped++
			continue
		}
		events = append(events, event)
	}

	parsedSentAt := parseTrackingSentAt(sentAt)
	inserted, duplicateSkipped, err := s.tracking.InsertBatch(ctx, TrackingBatchInput{
		AccountID:   source.AccountID,
		SourceID:    source.ID,
		SourceLabel: source.Name,
		Source:      strings.TrimSpace(payloadSource),
		BatchID:     strings.TrimSpace(batchID),
		SentAt:      parsedSentAt,
		Events:      events,
	})
	if err != nil {
		return TrackingIngestResponse{}, err
	}

	return TrackingIngestResponse{
		OK:       true,
		BatchID:  strings.TrimSpace(batchID),
		Received: len(rawEvents),
		Inserted: inserted,
		Skipped:  skipped + duplicateSkipped,
	}, nil
}

func normalizeTrackingWebhookEvent(raw map[string]any) (TrackingEventInput, bool) {
	page := mapValue(raw["page"])
	element := mapValue(raw["element"])
	device := mapValue(raw["device"])
	utm := mapValue(raw["utm"])
	metrics := mapValue(raw["metrics"])

	eventType := firstString(raw, "event_type")
	sessionID := firstString(raw, "session_id")
	if eventType == "" || sessionID == "" {
		return TrackingEventInput{}, false
	}

	eventName := firstString(raw, "event_name")
	if eventName == "" {
		eventName = eventType
	}

	sourceEventID := firstString(raw, "source_event_id")
	if sourceEventID == "" {
		sourceEventID = firstString(raw, "id")
	}
	if sourceEventID == "" {
		sourceEventID = firstString(raw, "event_id")
	}

	return TrackingEventInput{
		SourceEventID:  sourceEventID,
		VisitorID:      firstString(raw, "visitor_id"),
		SessionID:      sessionID,
		EventType:      eventType,
		EventName:      eventName,
		PageURL:        firstNonEmpty(firstString(raw, "page_url"), firstString(page, "url")),
		PagePath:       firstNonEmpty(firstString(raw, "page_path"), firstString(page, "path")),
		PageTitle:      firstNonEmpty(firstString(raw, "page_title"), firstString(page, "title")),
		PageGroup:      firstNonEmpty(firstString(raw, "page_group"), firstString(page, "group")),
		PageName:       firstNonEmpty(firstString(raw, "page_name"), firstString(page, "name")),
		Referrer:       firstNonEmpty(firstString(raw, "referrer"), firstString(page, "referrer")),
		ElementTag:     firstNonEmpty(firstString(raw, "element_tag"), firstString(element, "tag")),
		ElementText:    firstNonEmpty(firstString(raw, "element_text"), firstString(element, "text")),
		ElementHref:    firstNonEmpty(firstString(raw, "element_href"), firstString(element, "href")),
		ElementID:      firstNonEmpty(firstString(raw, "element_id"), firstString(element, "id")),
		ElementClasses: firstNonEmpty(firstString(raw, "element_classes"), firstString(element, "classes")),
		ElementRole:    firstNonEmpty(firstString(raw, "element_role"), firstString(element, "role")),
		ProductCode:    firstNonEmpty(firstString(raw, "product_code"), firstString(element, "product_code")),
		ActiveSeconds:  firstIntPointer(raw, metrics, "active_seconds"),
		ScrollDepth:    firstIntPointer(raw, metrics, "scroll_depth"),
		ScreenWidth:    firstIntPointer(raw, device, "screen_width"),
		ScreenHeight:   firstIntPointer(raw, device, "screen_height"),
		ViewportWidth:  firstIntPointer(raw, device, "viewport_width"),
		ViewportHeight: firstIntPointer(raw, device, "viewport_height"),
		DeviceType:     firstNonEmpty(firstString(raw, "device_type"), firstString(device, "type")),
		BrowserLang:    firstNonEmpty(firstString(raw, "browser_lang"), firstString(device, "language")),
		Timezone:       firstNonEmpty(firstString(raw, "timezone"), firstString(device, "timezone")),
		UTMSource:      firstNonEmpty(firstString(raw, "utm_source"), firstString(utm, "utm_source")),
		UTMMedium:      firstNonEmpty(firstString(raw, "utm_medium"), firstString(utm, "utm_medium")),
		UTMCampaign:    firstNonEmpty(firstString(raw, "utm_campaign"), firstString(utm, "utm_campaign")),
		UTMTerm:        firstNonEmpty(firstString(raw, "utm_term"), firstString(utm, "utm_term")),
		UTMContent:     firstNonEmpty(firstString(raw, "utm_content"), firstString(utm, "utm_content")),
		EventData:      firstAny(raw, "event_data", "data"),
		RawPayload:     raw,
		IP:             firstString(raw, "ip"),
		UserAgent:      firstString(raw, "user_agent"),
	}, true
}

func parseTrackingSentAt(value string) *time.Time {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return nil
	}
	return &parsed
}

func firstString(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	return stringifyTrackingValue(values[key])
}

func firstAny(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstIntPointer(flat map[string]any, nested map[string]any, key string) *int {
	if value, ok := intPointer(flat[key]); ok {
		return value
	}
	if value, ok := intPointer(nested[key]); ok {
		return value
	}
	return nil
}

func intPointer(value any) (*int, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, false
	case int:
		return &typed, true
	case int64:
		converted := int(typed)
		return &converted, true
	case float64:
		converted := int(typed)
		return &converted, true
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil, false
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil {
			return nil, false
		}
		return &parsed, true
	default:
		return nil, false
	}
}

func mapValue(value any) map[string]any {
	if mapped, ok := value.(map[string]any); ok {
		return mapped
	}
	return nil
}

func stringifyTrackingValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}
