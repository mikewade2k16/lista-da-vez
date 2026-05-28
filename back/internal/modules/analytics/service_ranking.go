package analytics

import (
	"sort"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
)

func buildRankingRowsAcrossBundles(bundles []bundle, scope string) []RankingRow {
	rows := make([]RankingRow, 0)
	for _, item := range bundles {
		storeRows := buildRankingRows(item.history, item.roster, scope)
		for index := range storeRows {
			storeRows[index].StoreID = item.storeID
			storeRows[index].StoreName = item.storeView.Name
		}

		rows = append(rows, storeRows...)
	}

	sortRankingRows(rows)
	return rows
}

func buildConsultantAlertsAcrossBundles(bundles []bundle) []ConsultantAlert {
	alerts := make([]ConsultantAlert, 0)
	for _, item := range bundles {
		alerts = append(alerts, buildConsultantAlerts(item.history, item.roster, item.settings)...)
	}

	return alerts
}

func buildRankingRows(history []operations.ServiceHistoryEntry, roster []operations.ConsultantProfile, scope string) []RankingRow {
	now := time.Now().In(analyticsLocation)
	currentMonth := monthStamp(now)
	currentDay := dayStamp(now)

	rows := make([]RankingRow, 0, len(roster))
	for _, consultant := range roster {
		entries := make([]operations.ServiceHistoryEntry, 0)
		for _, entry := range history {
			if strings.TrimSpace(entry.PersonID) != strings.TrimSpace(consultant.ID) {
				continue
			}

			finishedAt := time.UnixMilli(entry.FinishedAt).In(analyticsLocation)
			if scope == "today" {
				if dayStamp(finishedAt) != currentDay {
					continue
				}
			} else if monthStamp(finishedAt) != currentMonth {
				continue
			}

			entries = append(entries, entry)
		}

		converted := make([]operations.ServiceHistoryEntry, 0)
		soldValue := 0.0
		totalPieces := 0
		completeEntries := 0
		totalDuration := int64(0)
		queueJumpCount := 0
		nonClientConversions := 0

		for _, entry := range entries {
			if isSaleOutcome(entry.FinishOutcome) {
				converted = append(converted, entry)
				soldValue += maxFloat(entry.SaleAmount, 0)
				if !entry.IsExistingCustomer {
					nonClientConversions++
				}
			}

			totalPieces += len(entry.ProductsClosed)
			if isCompleteEntry(entry) {
				completeEntries++
			}

			totalDuration += maxInt64(entry.DurationMs, 0)
			if strings.TrimSpace(entry.StartMode) == "queue-jump" {
				queueJumpCount++
			}
		}

		attendances := len(entries)
		conversions := len(converted)
		ticketAverage := 0.0
		paScore := 0.0
		qualityScore := 0.0
		avgDurationMs := 0.0
		conversionRate := 0.0
		queueJumpRate := 0.0

		if conversions > 0 {
			ticketAverage = soldValue / float64(conversions)
			piecesForPA := totalPieces
			if piecesForPA < conversions {
				piecesForPA = conversions
			}
			paScore = float64(piecesForPA) / float64(conversions)
		}
		if attendances > 0 {
			qualityScore = (float64(completeEntries) / float64(attendances)) * 100
			avgDurationMs = float64(totalDuration) / float64(attendances)
			conversionRate = (float64(conversions) / float64(attendances)) * 100
			queueJumpRate = (float64(queueJumpCount) / float64(attendances)) * 100
		}

		rows = append(rows, RankingRow{
			ConsultantID:         consultant.ID,
			ConsultantName:       consultant.Name,
			SoldValue:            soldValue,
			Attendances:          attendances,
			Conversions:          conversions,
			NonConversions:       attendances - conversions,
			ConversionRate:       conversionRate,
			TicketAverage:        ticketAverage,
			PAScore:              paScore,
			QualityScore:         qualityScore,
			AvgDurationMs:        avgDurationMs,
			NonClientConversions: nonClientConversions,
			QueueJumpServices:    queueJumpCount,
			QueueJumpRate:        queueJumpRate,
		})
	}

	sortRankingRows(rows)

	return rows
}

func sortRankingRows(rows []RankingRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].SoldValue == rows[j].SoldValue {
			if rows[i].Conversions == rows[j].Conversions {
				return rows[i].ConversionRate > rows[j].ConversionRate
			}

			return rows[i].Conversions > rows[j].Conversions
		}

		return rows[i].SoldValue > rows[j].SoldValue
	})
}

func buildConsultantAlerts(history []operations.ServiceHistoryEntry, roster []operations.ConsultantProfile, settings StoreSettings) []ConsultantAlert {
	alerts := make([]ConsultantAlert, 0)
	now := time.Now().In(analyticsLocation)
	currentMonth := monthStamp(now)

	for _, consultant := range roster {
		entries := make([]operations.ServiceHistoryEntry, 0)
		for _, entry := range history {
			if strings.TrimSpace(entry.PersonID) != strings.TrimSpace(consultant.ID) {
				continue
			}

			if monthStamp(time.UnixMilli(entry.FinishedAt).In(analyticsLocation)) != currentMonth {
				continue
			}

			entries = append(entries, entry)
		}

		if len(entries) == 0 {
			continue
		}

		conversions := 0
		queueJumpCount := 0
		soldValue := 0.0
		totalPieces := 0
		for _, entry := range entries {
			if isSaleOutcome(entry.FinishOutcome) {
				conversions++
				soldValue += maxFloat(entry.SaleAmount, 0)
			}

			if strings.TrimSpace(entry.StartMode) == "queue-jump" {
				queueJumpCount++
			}

			totalPieces += len(entry.ProductsClosed)
		}

		conversionRate := (float64(conversions) / float64(len(entries))) * 100
		queueJumpRate := (float64(queueJumpCount) / float64(len(entries))) * 100
		ticketAverage := 0.0
		paScore := 0.0
		if conversions > 0 {
			ticketAverage = soldValue / float64(conversions)
			piecesForPA := totalPieces
			if piecesForPA < conversions {
				piecesForPA = conversions
			}
			paScore = float64(piecesForPA) / float64(conversions)
		}

		if settings.AlertMinConversionRate > 0 && conversionRate < settings.AlertMinConversionRate {
			alerts = append(alerts, ConsultantAlert{ConsultantID: consultant.ID, ConsultantName: consultant.Name, Type: "conversion", Value: conversionRate, Threshold: settings.AlertMinConversionRate})
		}
		if settings.AlertMaxQueueJumpRate > 0 && queueJumpRate > settings.AlertMaxQueueJumpRate {
			alerts = append(alerts, ConsultantAlert{ConsultantID: consultant.ID, ConsultantName: consultant.Name, Type: "queueJump", Value: queueJumpRate, Threshold: settings.AlertMaxQueueJumpRate})
		}
		if settings.AlertMinPAScore > 0 && paScore < settings.AlertMinPAScore {
			alerts = append(alerts, ConsultantAlert{ConsultantID: consultant.ID, ConsultantName: consultant.Name, Type: "pa", Value: paScore, Threshold: settings.AlertMinPAScore})
		}
		if settings.AlertMinTicketAverage > 0 && ticketAverage < settings.AlertMinTicketAverage {
			alerts = append(alerts, ConsultantAlert{ConsultantID: consultant.ID, ConsultantName: consultant.Name, Type: "ticket", Value: ticketAverage, Threshold: settings.AlertMinTicketAverage})
		}
	}

	return alerts
}
