package app

import (
	"context"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/analytics"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/performancefeedback"
)

type performanceFeedbackMetricsAdapter struct {
	analytics *analytics.Service
}

func newPerformanceFeedbackMetricsAdapter(service *analytics.Service) *performanceFeedbackMetricsAdapter {
	return &performanceFeedbackMetricsAdapter{analytics: service}
}

func (adapter *performanceFeedbackMetricsAdapter) LoadConsultantMetrics(
	ctx context.Context,
	principal auth.Principal,
	storeID string,
	consultantID string,
	dateFrom string,
	dateTo string,
) (performancefeedback.Metrics, error) {
	row, found, err := adapter.analytics.ConsultantMetricsWithinAccessibleStore(
		ctx,
		principal,
		storeID,
		consultantID,
		dateFrom,
		dateTo,
	)
	if err != nil {
		return performancefeedback.Metrics{}, err
	}
	if !found {
		return performancefeedback.Metrics{}, performancefeedback.ErrConsultantNotFound
	}

	return performancefeedback.Metrics{
		SoldValue:            row.SoldValue,
		Attendances:          row.Attendances,
		Conversions:          row.Conversions,
		NonConversions:       row.NonConversions,
		ConversionRate:       row.ConversionRate,
		TicketAverage:        row.TicketAverage,
		PAScore:              row.PAScore,
		QualityScore:         row.QualityScore,
		AvgDurationMs:        row.AvgDurationMs,
		NonClientConversions: row.NonClientConversions,
		QueueJumpRate:        row.QueueJumpRate,
	}, nil
}
