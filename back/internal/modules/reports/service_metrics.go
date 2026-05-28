package reports

import (
	"sort"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
)

type completionInfo struct {
	CoreFillRate float64
	HasNotes     bool
	Level        string
}

func buildMetrics(entries []operations.ServiceHistoryEntry) Metrics {
	conversions := 0
	soldValue := 0.0
	totalDuration := int64(0)
	totalQueueWait := int64(0)
	queueJumpCount := 0
	campaignBonusTotal := 0.0

	for _, entry := range entries {
		if isSaleOutcome(entry.FinishOutcome) {
			conversions++
			soldValue += maxFloat(entry.SaleAmount, 0)
		}

		totalDuration += maxInt64(entry.DurationMs, 0)
		totalQueueWait += maxInt64(entry.QueueWaitMs, 0)
		campaignBonusTotal += maxFloat(entry.CampaignBonusTotal, 0)

		if strings.TrimSpace(entry.StartMode) == "queue-jump" {
			queueJumpCount++
		}
	}

	totalAttendances := len(entries)
	nonConversions := totalAttendances - conversions
	averageTicket := 0.0
	if conversions > 0 {
		averageTicket = soldValue / float64(conversions)
	}

	averageDuration := 0.0
	averageQueueWait := 0.0
	conversionRate := 0.0
	queueJumpRate := 0.0

	if totalAttendances > 0 {
		averageDuration = float64(totalDuration) / float64(totalAttendances)
		averageQueueWait = float64(totalQueueWait) / float64(totalAttendances)
		conversionRate = (float64(conversions) / float64(totalAttendances)) * 100
		queueJumpRate = (float64(queueJumpCount) / float64(totalAttendances)) * 100
	}

	return Metrics{
		TotalAttendances:   totalAttendances,
		Conversions:        conversions,
		NonConversions:     nonConversions,
		ConversionRate:     conversionRate,
		SoldValue:          soldValue,
		AverageTicket:      averageTicket,
		AverageDurationMs:  averageDuration,
		AverageQueueWaitMs: averageQueueWait,
		QueueJumpRate:      queueJumpRate,
		CampaignBonusTotal: campaignBonusTotal,
	}
}

func buildQuality(entries []operations.ServiceHistoryEntry) QualityOverview {
	type consultantBucket struct {
		ConsultantID     string
		ConsultantName   string
		TotalAttendances int
		CompleteCount    int
		ExcellentCount   int
		IncompleteCount  int
		NotesCount       int
	}

	consultants := map[string]*consultantBucket{}
	completeCount := 0
	excellentCount := 0
	incompleteCount := 0
	notesCount := 0

	for _, entry := range entries {
		completion := evaluateCompletion(entry)

		switch completion.Level {
		case "excellent":
			excellentCount++
			completeCount++
		case "complete":
			completeCount++
		default:
			incompleteCount++
		}

		if completion.HasNotes {
			notesCount++
		}

		key := strings.TrimSpace(entry.PersonID)
		if key == "" {
			key = strings.TrimSpace(entry.PersonName)
		}

		bucket, ok := consultants[key]
		if !ok {
			bucket = &consultantBucket{
				ConsultantID:   strings.TrimSpace(entry.PersonID),
				ConsultantName: strings.TrimSpace(entry.PersonName),
			}
			consultants[key] = bucket
		}

		bucket.TotalAttendances++
		switch completion.Level {
		case "excellent":
			bucket.ExcellentCount++
			bucket.CompleteCount++
		case "complete":
			bucket.CompleteCount++
		default:
			bucket.IncompleteCount++
		}

		if completion.HasNotes {
			bucket.NotesCount++
		}
	}

	rows := make([]ConsultantQualityRow, 0, len(consultants))
	for _, bucket := range consultants {
		completeRate := 0.0
		excellentRate := 0.0
		incompleteRate := 0.0
		notesRate := 0.0
		if bucket.TotalAttendances > 0 {
			total := float64(bucket.TotalAttendances)
			completeRate = (float64(bucket.CompleteCount) / total) * 100
			excellentRate = (float64(bucket.ExcellentCount) / total) * 100
			incompleteRate = (float64(bucket.IncompleteCount) / total) * 100
			notesRate = (float64(bucket.NotesCount) / total) * 100
		}

		levelKey, levelLabel := resolveConsultantQualityLevel(completeRate, excellentRate)
		rows = append(rows, ConsultantQualityRow{
			ConsultantID:      bucket.ConsultantID,
			ConsultantName:    bucket.ConsultantName,
			TotalAttendances:  bucket.TotalAttendances,
			CompleteCount:     bucket.CompleteCount,
			ExcellentCount:    bucket.ExcellentCount,
			IncompleteCount:   bucket.IncompleteCount,
			NotesCount:        bucket.NotesCount,
			CompleteRate:      completeRate,
			ExcellentRate:     excellentRate,
			IncompleteRate:    incompleteRate,
			NotesRate:         notesRate,
			QualityLevelKey:   levelKey,
			QualityLevelLabel: levelLabel,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].ExcellentRate == rows[j].ExcellentRate {
			if rows[i].CompleteRate == rows[j].CompleteRate {
				return rows[i].TotalAttendances > rows[j].TotalAttendances
			}

			return rows[i].CompleteRate > rows[j].CompleteRate
		}

		return rows[i].ExcellentRate > rows[j].ExcellentRate
	})

	totalAttendances := len(entries)
	completeRate := 0.0
	excellentRate := 0.0
	incompleteRate := 0.0
	notesRate := 0.0

	if totalAttendances > 0 {
		total := float64(totalAttendances)
		completeRate = (float64(completeCount) / total) * 100
		excellentRate = (float64(excellentCount) / total) * 100
		incompleteRate = (float64(incompleteCount) / total) * 100
		notesRate = (float64(notesCount) / total) * 100
	}

	return QualityOverview{
		CompleteCount:   completeCount,
		ExcellentCount:  excellentCount,
		IncompleteCount: incompleteCount,
		NotesCount:      notesCount,
		CompleteRate:    completeRate,
		ExcellentRate:   excellentRate,
		IncompleteRate:  incompleteRate,
		NotesRate:       notesRate,
		ByConsultant:    rows,
	}
}

func evaluateCompletion(entry operations.ServiceHistoryEntry) completionInfo {
	checks := []bool{
		hasText(entry.CustomerName),
		hasText(entry.CustomerPhone),
		hasText(entry.ProductClosed) ||
			hasText(entry.ProductSeen) ||
			hasText(entry.ProductDetails) ||
			len(entry.ProductsSeen) > 0 ||
			entry.ProductsSeenNone,
		len(entry.VisitReasons) > 0 || entry.VisitReasonsNotInformed,
		len(entry.CustomerSources) > 0 || entry.CustomerSourcesNotInformed,
	}

	filled := 0
	for _, item := range checks {
		if item {
			filled++
		}
	}

	coreTotal := len(checks)
	coreFillRate := 0.0
	if coreTotal > 0 {
		coreFillRate = float64(filled) / float64(coreTotal)
	}

	hasNotes := hasText(entry.Notes)
	level := "incomplete"
	if filled == coreTotal {
		if hasNotes {
			level = "excellent"
		} else {
			level = "complete"
		}
	}

	return completionInfo{
		CoreFillRate: coreFillRate,
		HasNotes:     hasNotes,
		Level:        level,
	}
}

func resolveConsultantQualityLevel(completeRate float64, excellentRate float64) (string, string) {
	if completeRate >= 85 && excellentRate >= 35 {
		return "highlight", "Destaque"
	}
	if completeRate >= 70 {
		return "consistent", "Consistente"
	}
	return "attention", "Precisa melhorar"
}
