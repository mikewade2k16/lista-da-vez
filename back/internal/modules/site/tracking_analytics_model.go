package site

import "time"

// TrackingAnalyticsFilter parametriza GET /v1/admin/tracking-analytics.
type TrackingAnalyticsFilter struct {
	AccountID string
	Source    string
	Days      int
}

// TrackingAnalyticsView agrega os eventos de tracking para o dashboard do site.
type TrackingAnalyticsView struct {
	RangeDays    int                   `json:"rangeDays"`
	Totals       TrackingTotals        `json:"totals"`
	Devices      []TrackingCountItem   `json:"devices"`
	EventsByType []TrackingCountItem   `json:"eventsByType"`
	Conversions  []TrackingConversion  `json:"conversions"`
	AccessByDay  []TrackingDailyCount  `json:"accessByDay"`
	TopReferrers []TrackingCountItem   `json:"topReferrers"`
	RecentVisits []TrackingRecentVisit `json:"recentVisits"`
}

// TrackingTotals reune os KPIs de topo do dashboard.
type TrackingTotals struct {
	TotalEvents   int `json:"totalEvents"`
	TotalSessions int `json:"totalSessions"`
	TotalVisitors int `json:"totalVisitors"`
	PageViews     int `json:"pageViews"`
	Today         int `json:"today"`
	Last7Days     int `json:"last7Days"`
}

// TrackingCountItem e um par rotulo/contagem usado em dispositivos, eventos e referrers.
type TrackingCountItem struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// TrackingConversion e um evento de conversao com rotulo amigavel e % de visitantes.
type TrackingConversion struct {
	Key               string  `json:"key"`
	Label             string  `json:"label"`
	Count             int     `json:"count"`
	PercentOfVisitors float64 `json:"percentOfVisitors"`
}

// TrackingDailyCount e a contagem de eventos de um dia (serie continua).
type TrackingDailyCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// TrackingRecentVisit e uma linha da lista de ultimas visitas.
type TrackingRecentVisit struct {
	ReceivedAt time.Time `json:"receivedAt"`
	DeviceType string    `json:"deviceType"`
	IP         string    `json:"ip"`
	Referrer   string    `json:"referrer"`
	PagePath   string    `json:"pagePath"`
}
