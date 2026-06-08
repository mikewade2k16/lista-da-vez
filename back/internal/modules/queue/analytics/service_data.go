package analytics

import (
	"sort"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/operations"
)

func buildCombinedTimeIntelligence(bundles []bundle) TimeIntelligence {
	combined := TimeIntelligence{}
	totalAttendances := 0
	totalQueueWaitMs := 0.0
	totalQueueJumpAttendances := 0.0

	for _, item := range bundles {
		timeIntelligence := buildTimeIntelligence(item)
		attendanceCount := len(item.history)

		combined.QuickHighPotentialCount += timeIntelligence.QuickHighPotentialCount
		combined.LongLowSaleCount += timeIntelligence.LongLowSaleCount
		combined.LongNoSaleCount += timeIntelligence.LongNoSaleCount
		combined.QuickNoSaleCount += timeIntelligence.QuickNoSaleCount
		combined.TotalsByStatus.Available += timeIntelligence.TotalsByStatus.Available
		combined.TotalsByStatus.Queue += timeIntelligence.TotalsByStatus.Queue
		combined.TotalsByStatus.Service += timeIntelligence.TotalsByStatus.Service
		combined.TotalsByStatus.Paused += timeIntelligence.TotalsByStatus.Paused
		combined.ConsultantsInQueueMs += timeIntelligence.ConsultantsInQueueMs
		combined.ConsultantsPausedMs += timeIntelligence.ConsultantsPausedMs
		combined.ConsultantsInServiceMs += timeIntelligence.ConsultantsInServiceMs

		totalAttendances += attendanceCount
		totalQueueWaitMs += timeIntelligence.AvgQueueWaitMs * float64(attendanceCount)
		totalQueueJumpAttendances += (timeIntelligence.NotUsingQueueRate / 100) * float64(attendanceCount)
	}

	if totalAttendances > 0 {
		combined.AvgQueueWaitMs = totalQueueWaitMs / float64(totalAttendances)
		combined.NotUsingQueueRate = (totalQueueJumpAttendances / float64(totalAttendances)) * 100
	}

	return combined
}

func buildSoldProducts(history []operations.ServiceHistoryEntry) []CountRow {
	labels := make([]string, 0)
	for _, entry := range history {
		if !isSaleOutcome(entry.FinishOutcome) {
			continue
		}

		labels = append(labels, closedProductLabels(entry)...)
	}

	return groupLabels(labels, 8)
}

func buildRequestedProducts(history []operations.ServiceHistoryEntry) []CountRow {
	labels := make([]string, 0)
	for _, entry := range history {
		if len(entry.ProductsSeen) > 0 {
			for _, product := range entry.ProductsSeen {
				label := firstNonEmpty(product.Name, product.Code)
				if label != "" {
					labels = append(labels, label)
				}
			}
			continue
		}

		label := firstNonEmpty(entry.ProductSeen, entry.ProductDetails)
		if label != "" {
			labels = append(labels, label)
		}
	}

	return groupLabels(labels, 8)
}

func buildVisitReasons(history []operations.ServiceHistoryEntry, labels map[string]string) []CountRow {
	values := make([]string, 0)
	for _, entry := range history {
		for _, value := range entry.VisitReasons {
			values = append(values, firstNonEmpty(labels[strings.TrimSpace(value)], value))
		}
	}

	return groupLabels(values, 8)
}

func buildCustomerSources(history []operations.ServiceHistoryEntry, labels map[string]string) []CountRow {
	values := make([]string, 0)
	for _, entry := range history {
		for _, value := range entry.CustomerSources {
			values = append(values, firstNonEmpty(labels[strings.TrimSpace(value)], value))
		}
	}

	return groupLabels(values, 8)
}

func buildProfessions(history []operations.ServiceHistoryEntry) []CountRow {
	values := make([]string, 0)
	for _, entry := range history {
		values = append(values, strings.TrimSpace(entry.CustomerProfession))
	}

	return groupLabels(values, 6)
}

func buildOutcomeSummary(history []operations.ServiceHistoryEntry) []CountRow {
	values := make([]string, 0, len(history))
	for _, entry := range history {
		switch strings.TrimSpace(entry.FinishOutcome) {
		case "compra":
			values = append(values, "Compra")
		case "reserva":
			values = append(values, "Reserva")
		default:
			values = append(values, "Nao compra")
		}
	}

	return groupLabels(values, 0)
}

func buildHourlySales(history []operations.ServiceHistoryEntry) []HourlySalesRow {
	counter := map[string]*HourlySalesRow{}
	for _, entry := range history {
		if !isSaleOutcome(entry.FinishOutcome) {
			continue
		}

		hour := time.UnixMilli(entry.FinishedAt).In(analyticsLocation).Format("15")
		key := hour + "h"
		row, ok := counter[key]
		if !ok {
			row = &HourlySalesRow{Label: key}
			counter[key] = row
		}

		row.Count++
		row.Value += maxFloat(entry.SaleAmount, 0)
	}

	rows := make([]HourlySalesRow, 0, len(counter))
	for _, row := range counter {
		rows = append(rows, *row)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Value == rows[j].Value {
			return rows[i].Label < rows[j].Label
		}
		return rows[i].Value > rows[j].Value
	})

	if len(rows) > 8 {
		return rows[:8]
	}

	return rows
}

func buildTimeIntelligence(data bundle) TimeIntelligence {
	now := time.Now().UnixMilli()
	fastThresholdMs := int64(maxInt(data.settings.TimingFastCloseMinutes, 5)) * 60000
	longThresholdMs := int64(maxInt(data.settings.TimingLongServiceMinutes, 25)) * 60000
	lowSaleThreshold := maxFloat(data.settings.TimingLowSaleAmount, 1200)

	quickHighPotentialCount := 0
	longLowSaleCount := 0
	longNoSaleCount := 0
	quickNoSaleCount := 0
	queueJumpCount := 0
	totalQueueWait := int64(0)

	for _, entry := range data.history {
		if strings.TrimSpace(entry.StartMode) == "queue-jump" {
			queueJumpCount++
		}
		totalQueueWait += maxInt64(entry.QueueWaitMs, 0)

		switch {
		case isSaleOutcome(entry.FinishOutcome) && maxInt64(entry.DurationMs, 0) <= fastThresholdMs:
			quickHighPotentialCount++
		case isSaleOutcome(entry.FinishOutcome) && maxInt64(entry.DurationMs, 0) >= longThresholdMs && maxFloat(entry.SaleAmount, 0) <= lowSaleThreshold:
			longLowSaleCount++
		case strings.TrimSpace(entry.FinishOutcome) == "nao-compra" && maxInt64(entry.DurationMs, 0) >= longThresholdMs:
			longNoSaleCount++
		case strings.TrimSpace(entry.FinishOutcome) == "nao-compra" && maxInt64(entry.DurationMs, 0) <= fastThresholdMs:
			quickNoSaleCount++
		}
	}

	avgQueueWaitMs := 0.0
	if len(data.history) > 0 {
		avgQueueWaitMs = float64(totalQueueWait) / float64(len(data.history))
	}

	totals := StatusTotals{}
	for _, session := range data.snapshot.ConsultantActivitySessions {
		addStatusDuration(&totals, session.Status, session.DurationMs)
	}

	for _, consultant := range data.roster {
		status, startedAt := resolveLiveStatusSnapshot(data.snapshot, consultant.ID, now)
		addStatusDuration(&totals, status, maxInt64(now-startedAt, 0))
	}

	consultantsInQueueMs := int64(0)
	for _, item := range data.snapshot.WaitingList {
		consultantsInQueueMs += maxInt64(now-item.QueueJoinedAt, 0)
	}

	consultantsPausedMs := int64(0)
	for _, item := range data.snapshot.PausedEmployees {
		consultantsPausedMs += maxInt64(now-item.StartedAt, 0)
	}

	consultantsInServiceMs := int64(0)
	for _, item := range data.snapshot.ActiveServices {
		consultantsInServiceMs += maxInt64(now-item.ServiceStartedAt, 0)
	}

	notUsingQueueRate := 0.0
	if len(data.history) > 0 {
		notUsingQueueRate = (float64(queueJumpCount) / float64(len(data.history))) * 100
	}

	return TimeIntelligence{
		QuickHighPotentialCount: quickHighPotentialCount,
		LongLowSaleCount:        longLowSaleCount,
		LongNoSaleCount:         longNoSaleCount,
		QuickNoSaleCount:        quickNoSaleCount,
		AvgQueueWaitMs:          avgQueueWaitMs,
		TotalsByStatus:          totals,
		ConsultantsInQueueMs:    consultantsInQueueMs,
		ConsultantsPausedMs:     consultantsPausedMs,
		ConsultantsInServiceMs:  consultantsInServiceMs,
		NotUsingQueueRate:       notUsingQueueRate,
	}
}
