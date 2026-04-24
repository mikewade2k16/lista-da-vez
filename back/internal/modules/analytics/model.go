package analytics

import (
	"context"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

type StoreSettings struct {
	TimingFastCloseMinutes   int
	TimingLongServiceMinutes int
	TimingLowSaleAmount      float64
	AlertMinConversionRate   float64
	AlertMaxQueueJumpRate    float64
	AlertMinPAScore          float64
	AlertMinTicketAverage    float64
	VisitReasonLabels        map[string]string
	CustomerSourceLabels     map[string]string
}

type Repository interface {
	LoadSnapshot(ctx context.Context, storeID string) (operations.SnapshotState, error)
	ListRoster(ctx context.Context, storeID string) ([]operations.ConsultantProfile, error)
	LoadSettings(ctx context.Context, storeID string) (StoreSettings, error)
}

type StoreFinder interface {
	FindAccessible(ctx context.Context, principal auth.Principal, storeID string) (stores.StoreView, error)
}

type RankingResponse struct {
	StoreID     string            `json:"storeId"`
	MonthlyRows []RankingRow      `json:"monthlyRows"`
	DailyRows   []RankingRow      `json:"dailyRows"`
	Alerts      []ConsultantAlert `json:"alerts"`
}

type RankingRow struct {
	ConsultantID         string  `json:"consultantId"`
	ConsultantName       string  `json:"consultantName"`
	SoldValue            float64 `json:"soldValue"`
	Attendances          int     `json:"attendances"`
	Conversions          int     `json:"conversions"`
	NonConversions       int     `json:"nonConversions"`
	ConversionRate       float64 `json:"conversionRate"`
	TicketAverage        float64 `json:"ticketAverage"`
	PAScore              float64 `json:"paScore"`
	QualityScore         float64 `json:"qualityScore"`
	AvgDurationMs        float64 `json:"avgDurationMs"`
	NonClientConversions int     `json:"nonClientConversions"`
	QueueJumpServices    int     `json:"queueJumpServices"`
	QueueJumpRate        float64 `json:"queueJumpRate"`
}

type ConsultantAlert struct {
	ConsultantID   string  `json:"consultantId"`
	ConsultantName string  `json:"consultantName"`
	Type           string  `json:"type"`
	Value          float64 `json:"value"`
	Threshold      float64 `json:"threshold"`
}

type DataResponse struct {
	StoreID           string           `json:"storeId"`
	TimeIntelligence  TimeIntelligence `json:"timeIntelligence"`
	SoldProducts      []CountRow       `json:"soldProducts"`
	RequestedProducts []CountRow       `json:"requestedProducts"`
	VisitReasons      []CountRow       `json:"visitReasons"`
	CustomerSources   []CountRow       `json:"customerSources"`
	Professions       []CountRow       `json:"professions"`
	OutcomeSummary    []CountRow       `json:"outcomeSummary"`
	HourlySales       []HourlySalesRow `json:"hourlySales"`
}

type IntelligenceResponse struct {
	StoreID            string                  `json:"storeId"`
	TotalAttendances   int                     `json:"totalAttendances"`
	ConversionRate     float64                 `json:"conversionRate"`
	TicketAverage      float64                 `json:"ticketAverage"`
	HealthScore        float64                 `json:"healthScore"`
	SeverityCounts     SeverityCounts          `json:"severityCounts"`
	Diagnosis          []IntelligenceDiagnosis `json:"diagnosis"`
	RecommendedActions []string                `json:"recommendedActions"`
	Time               TimeIntelligence        `json:"time"`
}

type TimeIntelligence struct {
	QuickHighPotentialCount int          `json:"quickHighPotentialCount"`
	LongLowSaleCount        int          `json:"longLowSaleCount"`
	LongNoSaleCount         int          `json:"longNoSaleCount"`
	QuickNoSaleCount        int          `json:"quickNoSaleCount"`
	AvgQueueWaitMs          float64      `json:"avgQueueWaitMs"`
	TotalsByStatus          StatusTotals `json:"totalsByStatus"`
	ConsultantsInQueueMs    int64        `json:"consultantsInQueueMs"`
	ConsultantsPausedMs     int64        `json:"consultantsPausedMs"`
	ConsultantsInServiceMs  int64        `json:"consultantsInServiceMs"`
	NotUsingQueueRate       float64      `json:"notUsingQueueRate"`
}

type StatusTotals struct {
	Available int64 `json:"available"`
	Queue     int64 `json:"queue"`
	Service   int64 `json:"service"`
	Paused    int64 `json:"paused"`
}

type CountRow struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type HourlySalesRow struct {
	Label string  `json:"label"`
	Count int     `json:"count"`
	Value float64 `json:"value"`
}

type SeverityCounts struct {
	Critical  int `json:"critical"`
	Attention int `json:"attention"`
	Healthy   int `json:"healthy"`
}

type IntelligenceDiagnosis struct {
	ID         string `json:"id"`
	Level      string `json:"level"`
	Title      string `json:"title"`
	Reading    string `json:"reading"`
	Hypothesis string `json:"hypothesis"`
	Action     string `json:"action"`
}
