package analytics

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
)

var analyticsLocation = func() *time.Location {
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.UTC
	}

	return location
}()

func isCompleteEntry(entry operations.ServiceHistoryEntry) bool {
	hasCustomer := hasText(entry.CustomerName) || hasText(entry.CustomerPhone)
	hasProduct := len(entry.ProductsSeen) > 0 || hasText(entry.ProductSeen) || hasText(entry.ProductDetails) || entry.ProductsSeenNone
	hasReason := len(entry.VisitReasons) > 0 || entry.VisitReasonsNotInformed
	hasSource := len(entry.CustomerSources) > 0 || entry.CustomerSourcesNotInformed
	return hasCustomer && hasProduct && hasReason && hasSource
}

func resolveLiveStatusSnapshot(snapshot operations.SnapshotState, consultantID string, now int64) (string, int64) {
	for _, item := range snapshot.ActiveServices {
		if strings.TrimSpace(item.ConsultantID) == strings.TrimSpace(consultantID) {
			return "service", maxInt64(item.ServiceStartedAt, now)
		}
	}

	for _, item := range snapshot.WaitingList {
		if strings.TrimSpace(item.ConsultantID) == strings.TrimSpace(consultantID) {
			return "queue", maxInt64(item.QueueJoinedAt, now)
		}
	}

	for _, item := range snapshot.PausedEmployees {
		if strings.TrimSpace(item.ConsultantID) == strings.TrimSpace(consultantID) {
			return "paused", maxInt64(item.StartedAt, now)
		}
	}

	if currentStatus, ok := snapshot.ConsultantCurrentStatus[consultantID]; ok {
		return strings.TrimSpace(currentStatus.Status), maxInt64(currentStatus.StartedAt, now)
	}

	return "available", now
}

func addStatusDuration(totals *StatusTotals, status string, duration int64) {
	switch strings.TrimSpace(status) {
	case "queue":
		totals.Queue += maxInt64(duration, 0)
	case "service":
		totals.Service += maxInt64(duration, 0)
	case "paused":
		totals.Paused += maxInt64(duration, 0)
	default:
		totals.Available += maxInt64(duration, 0)
	}
}

func groupLabels(values []string, limit int) []CountRow {
	counter := map[string]*CountRow{}
	for _, rawValue := range values {
		value := strings.TrimSpace(rawValue)
		if value == "" {
			continue
		}

		key := strings.ToLower(value)
		row, ok := counter[key]
		if !ok {
			row = &CountRow{Label: value}
			counter[key] = row
		}
		row.Count++
	}

	rows := make([]CountRow, 0, len(counter))
	for _, row := range counter {
		rows = append(rows, *row)
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

func closedProductLabels(entry operations.ServiceHistoryEntry) []string {
	labels := make([]string, 0, len(entry.ProductsClosed))
	for _, product := range entry.ProductsClosed {
		label := firstNonEmpty(product.Name, product.Code)
		if label != "" {
			labels = append(labels, label)
		}
	}

	if len(labels) > 0 {
		return labels
	}

	fallback := firstNonEmpty(entry.ProductClosed, entry.ProductSeen, entry.ProductDetails)
	if fallback == "" {
		return []string{}
	}

	return []string{fallback}
}

func monthStamp(moment time.Time) string {
	return moment.Format("2006-01")
}

func dayStamp(moment time.Time) string {
	return moment.Format("2006-01-02")
}

func formatPercent(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

func formatDurationMinutes(valueMs int64) string {
	minutes := int64(0)
	if valueMs > 0 {
		minutes = valueMs / 60000
		if valueMs%60000 >= 30000 {
			minutes++
		}
	}

	return fmt.Sprintf("%d min", minutes)
}

func isSaleOutcome(outcome string) bool {
	switch strings.TrimSpace(outcome) {
	case "compra", "reserva":
		return true
	default:
		return false
	}
}

func hasText(value string) bool {
	return strings.TrimSpace(value) != ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func maxFloat(value float64, fallback float64) float64 {
	if value <= 0 {
		return fallback
	}

	return value
}

func maxInt(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}

	return value
}

func maxInt64(value int64, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}

	return value
}
