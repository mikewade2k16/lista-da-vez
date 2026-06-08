package site

import (
	"context"
	"time"
)

// TrackingEventInput representa um evento normalizado de tracking do site.
type TrackingEventInput struct {
	SourceEventID  string
	VisitorID      string
	SessionID      string
	EventType      string
	EventName      string
	PageURL        string
	PagePath       string
	PageTitle      string
	PageGroup      string
	PageName       string
	Referrer       string
	ElementTag     string
	ElementText    string
	ElementHref    string
	ElementID      string
	ElementClasses string
	ElementRole    string
	ProductCode    string
	ActiveSeconds  *int
	ScrollDepth    *int
	ScreenWidth    *int
	ScreenHeight   *int
	ViewportWidth  *int
	ViewportHeight *int
	DeviceType     string
	BrowserLang    string
	Timezone       string
	UTMSource      string
	UTMMedium      string
	UTMCampaign    string
	UTMTerm        string
	UTMContent     string
	EventData      any
	RawPayload     map[string]any
	IP             string
	UserAgent      string
}

// TrackingBatchInput agrupa os metadados de um lote recebido por webhook.
type TrackingBatchInput struct {
	AccountID   string
	SourceID    string
	SourceLabel string
	Source      string
	BatchID     string
	SentAt      *time.Time
	Events      []TrackingEventInput
}

// TrackingIngestResponse e a resposta enxuta do ingest de tracking.
type TrackingIngestResponse struct {
	OK       bool   `json:"ok"`
	BatchID  string `json:"batchId"`
	Received int    `json:"received"`
	Inserted int    `json:"inserted"`
	Skipped  int    `json:"skipped"`
}

// TrackingEventView representa um evento bruto retornado pela API admin.
type TrackingEventView struct {
	ID             string     `json:"id"`
	AccountID      string     `json:"accountId"`
	SourceID       string     `json:"sourceId,omitempty"`
	SourceLabel    string     `json:"sourceLabel"`
	Source         string     `json:"source"`
	BatchID        string     `json:"batchId"`
	SourceEventID  string     `json:"sourceEventId"`
	VisitorID      string     `json:"visitorId"`
	SessionID      string     `json:"sessionId"`
	EventType      string     `json:"eventType"`
	EventName      string     `json:"eventName"`
	PageURL        string     `json:"pageUrl"`
	PagePath       string     `json:"pagePath"`
	PageTitle      string     `json:"pageTitle"`
	PageGroup      string     `json:"pageGroup"`
	PageName       string     `json:"pageName"`
	Referrer       string     `json:"referrer"`
	ElementTag     string     `json:"elementTag"`
	ElementText    string     `json:"elementText"`
	ElementHref    string     `json:"elementHref"`
	ElementID      string     `json:"elementId"`
	ElementClasses string     `json:"elementClasses"`
	ElementRole    string     `json:"elementRole"`
	ProductCode    string     `json:"productCode"`
	ActiveSeconds  *int       `json:"activeSeconds,omitempty"`
	ScrollDepth    *int       `json:"scrollDepth,omitempty"`
	ScreenWidth    *int       `json:"screenWidth,omitempty"`
	ScreenHeight   *int       `json:"screenHeight,omitempty"`
	ViewportWidth  *int       `json:"viewportWidth,omitempty"`
	ViewportHeight *int       `json:"viewportHeight,omitempty"`
	DeviceType     string     `json:"deviceType"`
	BrowserLang    string     `json:"browserLang"`
	Timezone       string     `json:"timezone"`
	UTMSource      string     `json:"utmSource"`
	UTMMedium      string     `json:"utmMedium"`
	UTMCampaign    string     `json:"utmCampaign"`
	UTMTerm        string     `json:"utmTerm"`
	UTMContent     string     `json:"utmContent"`
	EventData      string     `json:"eventData,omitempty"`
	RawPayload     string     `json:"rawPayload,omitempty"`
	IP             string     `json:"ip"`
	UserAgent      string     `json:"userAgent"`
	SentAt         *time.Time `json:"sentAt,omitempty"`
	ReceivedAt     time.Time  `json:"receivedAt"`
}

// TrackingEventListFilter parametriza GET /v1/admin/tracking-events.
type TrackingEventListFilter struct {
	AccountID string
	Q         string
	Source    string
	EventType string
	PagePath  string
	Page      int
	PerPage   int
}

// TrackingEventListResponse e o body de GET /v1/admin/tracking-events.
type TrackingEventListResponse struct {
	Events  []TrackingEventView `json:"events"`
	Total   int                 `json:"total"`
	Page    int                 `json:"page"`
	PerPage int                 `json:"perPage"`
}

// TrackingRepository abstrai a persistencia dos eventos de tracking.
type TrackingRepository interface {
	List(ctx context.Context, filter TrackingEventListFilter) ([]TrackingEventView, int, error)
	InsertBatch(ctx context.Context, input TrackingBatchInput) (inserted int, skipped int, err error)
	Analytics(ctx context.Context, filter TrackingAnalyticsFilter) (TrackingAnalyticsView, error)
}
