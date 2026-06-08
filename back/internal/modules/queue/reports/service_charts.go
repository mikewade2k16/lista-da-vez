package reports

import (
	"sort"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/operations"
)

func buildChartData(entries []operations.ServiceHistoryEntry) ChartData {
	hourlyMap := map[string]HourlyDataPoint{}
	consultantMap := map[string]ConsultantAggRow{}
	visitReasons := map[string]*CountRow{}
	customerSources := map[string]*CountRow{}
	productsClosed := map[string]*CountRow{}
	outcomes := OutcomeCounts{}

	for _, entry := range entries {
		switch strings.TrimSpace(entry.FinishOutcome) {
		case "compra":
			outcomes.Compra++
		case "reserva":
			outcomes.Reserva++
		default:
			outcomes.NaoCompra++
		}

		hour := time.UnixMilli(entry.FinishedAt).UTC().Format("15")
		hourBucket := hourlyMap[hour]
		hourBucket.Hour = hour
		hourBucket.Attendances++
		if isSaleOutcome(entry.FinishOutcome) {
			hourBucket.Conversions++
			hourBucket.SaleAmount += maxFloat(entry.SaleAmount, 0)
		}
		hourlyMap[hour] = hourBucket

		consultantID := strings.TrimSpace(entry.PersonID)
		consultantBucket := consultantMap[consultantID]
		consultantBucket.ConsultantID = consultantID
		consultantBucket.ConsultantName = strings.TrimSpace(entry.PersonName)
		consultantBucket.Attendances++
		if isSaleOutcome(entry.FinishOutcome) {
			consultantBucket.Conversions++
			consultantBucket.SaleAmount += maxFloat(entry.SaleAmount, 0)
		}
		consultantMap[consultantID] = consultantBucket

		for _, value := range entry.VisitReasons {
			incrementCountMap(visitReasons, value)
		}

		for _, value := range entry.CustomerSources {
			incrementCountMap(customerSources, value)
		}

		for _, label := range closedProductLabels(entry) {
			incrementCountMap(productsClosed, label)
		}
	}

	hourlyRows := make([]HourlyDataPoint, 0, len(hourlyMap))
	for _, item := range hourlyMap {
		hourlyRows = append(hourlyRows, item)
	}
	sort.SliceStable(hourlyRows, func(i, j int) bool {
		return hourlyRows[i].Hour < hourlyRows[j].Hour
	})

	consultantRows := make([]ConsultantAggRow, 0, len(consultantMap))
	for _, item := range consultantMap {
		consultantRows = append(consultantRows, item)
	}
	sort.SliceStable(consultantRows, func(i, j int) bool {
		if consultantRows[i].SaleAmount == consultantRows[j].SaleAmount {
			return consultantRows[i].ConsultantName < consultantRows[j].ConsultantName
		}

		return consultantRows[i].SaleAmount > consultantRows[j].SaleAmount
	})

	return ChartData{
		OutcomeCounts:      outcomes,
		HourlyData:         hourlyRows,
		ConsultantAgg:      consultantRows,
		TopProductsClosed:  topCountRows(productsClosed, 8),
		TopVisitReasons:    topCountRows(visitReasons, 8),
		TopCustomerSources: topCountRows(customerSources, 8),
	}
}

func buildResultRows(entries []operations.ServiceHistoryEntry) []ResultRow {
	rows := make([]ResultRow, 0, len(entries))
	for _, entry := range entries {
		completion := evaluateCompletion(entry)
		rows = append(rows, ResultRow{
			ServiceID:          strings.TrimSpace(entry.ServiceID),
			StoreID:            strings.TrimSpace(entry.StoreID),
			StoreName:          strings.TrimSpace(entry.StoreName),
			ConsultantID:       strings.TrimSpace(entry.PersonID),
			ConsultantName:     strings.TrimSpace(entry.PersonName),
			StartedAt:          entry.StartedAt,
			FinishedAt:         entry.FinishedAt,
			DurationMs:         maxInt64(entry.DurationMs, 0),
			QueueWaitMs:        maxInt64(entry.QueueWaitMs, 0),
			Outcome:            strings.TrimSpace(entry.FinishOutcome),
			StartMode:          strings.TrimSpace(entry.StartMode),
			SaleAmount:         maxFloat(entry.SaleAmount, 0),
			IsWindowService:    entry.IsWindowService,
			IsGift:             entry.IsGift,
			IsExistingCustomer: entry.IsExistingCustomer,
			CustomerName:       strings.TrimSpace(entry.CustomerName),
			CustomerPhone:      strings.TrimSpace(entry.CustomerPhone),
			CustomerEmail:      strings.TrimSpace(entry.CustomerEmail),
			CustomerProfession: strings.TrimSpace(entry.CustomerProfession),
			ProductSeen:        strings.TrimSpace(entry.ProductSeen),
			ProductClosed:      primaryClosedProductLabel(entry),
			ProductDetails:     strings.TrimSpace(entry.ProductDetails),
			VisitReasons:       cloneStringSlice(entry.VisitReasons),
			CustomerSources:    cloneStringSlice(entry.CustomerSources),
			CampaignNames:      cloneCampaignNames(entry.CampaignMatches),
			QueueJumpReason:    strings.TrimSpace(entry.QueueJumpReason),
			Notes:              strings.TrimSpace(entry.Notes),
			HasNotes:           completion.HasNotes,
			CompletionLevel:    completion.Level,
			CompletionRate:     completion.CoreFillRate,
			CampaignBonusTotal: maxFloat(entry.CampaignBonusTotal, 0),
		})
	}

	return rows
}

func paginateRows(rows []ResultRow, page int, pageSize int) []ResultRow {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	start := (page - 1) * pageSize
	if start >= len(rows) {
		return []ResultRow{}
	}

	end := start + pageSize
	if end > len(rows) {
		end = len(rows)
	}

	return rows[start:end]
}

func closedProductLabels(entry operations.ServiceHistoryEntry) []string {
	labels := make([]string, 0, len(entry.ProductsClosed)+1)
	for _, product := range entry.ProductsClosed {
		label := firstNonEmpty(strings.TrimSpace(product.Name), strings.TrimSpace(product.Code))
		if label != "" {
			labels = append(labels, label)
		}
	}

	if len(labels) > 0 {
		return labels
	}

	fallback := strings.TrimSpace(entry.ProductClosed)
	if fallback != "" {
		return []string{fallback}
	}

	return []string{}
}

func primaryClosedProductLabel(entry operations.ServiceHistoryEntry) string {
	labels := closedProductLabels(entry)
	if len(labels) > 0 {
		return labels[0]
	}

	return strings.TrimSpace(entry.ProductClosed)
}

func incrementCountMap(counter map[string]*CountRow, label string) {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return
	}

	key := strings.ToLower(trimmed)
	row, ok := counter[key]
	if !ok {
		row = &CountRow{Label: trimmed}
		counter[key] = row
	}

	row.Count++
}

func topCountRows(counter map[string]*CountRow, limit int) []CountRow {
	rows := make([]CountRow, 0, len(counter))
	for _, item := range counter {
		rows = append(rows, *item)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Count == rows[j].Count {
			return rows[i].Label < rows[j].Label
		}

		return rows[i].Count > rows[j].Count
	})

	if limit > 0 && len(rows) > limit {
		return rows[:limit]
	}

	return rows
}
