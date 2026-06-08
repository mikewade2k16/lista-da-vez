package reports

import (
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/operations"
)

func entryHasCampaign(entry operations.ServiceHistoryEntry, campaignIDs []string) bool {
	for _, match := range entry.CampaignMatches {
		if containsValue(campaignIDs, strings.TrimSpace(match.ID)) {
			return true
		}
	}
	return false
}

func matchesSearch(entry operations.ServiceHistoryEntry, query string) bool {
	searchable := []string{
		entry.StoreName,
		entry.ServiceID,
		entry.PersonName,
		entry.CustomerName,
		entry.CustomerPhone,
		entry.CustomerEmail,
		entry.CustomerProfession,
		entry.ProductSeen,
		entry.ProductClosed,
		entry.ProductDetails,
		entry.Notes,
		entry.QueueJumpReason,
	}

	for _, product := range entry.ProductsSeen {
		searchable = append(searchable, product.Name, product.Code)
	}
	for _, product := range entry.ProductsClosed {
		searchable = append(searchable, product.Name, product.Code)
	}
	searchable = append(searchable, entry.VisitReasons...)
	searchable = append(searchable, entry.CustomerSources...)

	for _, value := range searchable {
		if strings.Contains(comparableText(value), query) {
			return true
		}
	}

	return false
}

func normalizeList(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, rawValue := range values {
		for _, part := range strings.Split(rawValue, ",") {
			value := strings.TrimSpace(part)
			if value == "" {
				continue
			}

			if _, ok := seen[value]; ok {
				continue
			}

			seen[value] = struct{}{}
			result = append(result, value)
		}
	}

	return result
}

func normalizeAllowedList(values []string, allowed map[string]struct{}) []string {
	normalized := normalizeList(values)
	result := make([]string, 0, len(normalized))
	for _, value := range normalized {
		if _, ok := allowed[value]; ok {
			result = append(result, value)
		}
	}
	return result
}

func dayStartMillis(dateValue string) (int64, error) {
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		location = time.UTC
	}

	value, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(dateValue), location)
	if err != nil {
		return 0, err
	}

	return value.UnixMilli(), nil
}

func dayEndMillis(dateValue string) (int64, error) {
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		location = time.UTC
	}

	value, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(dateValue), location)
	if err != nil {
		return 0, err
	}

	return value.Add((24 * time.Hour) - time.Millisecond).UnixMilli(), nil
}

func intersectsAny(values []string, required []string) bool {
	if len(required) == 0 {
		return true
	}

	for _, value := range values {
		if containsValue(required, strings.TrimSpace(value)) {
			return true
		}
	}

	return false
}

func containsValue(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(target) {
			return true
		}
	}

	return false
}

func comparableText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func hasText(value string) bool {
	return strings.TrimSpace(value) != ""
}

func isSaleOutcome(outcome string) bool {
	switch strings.TrimSpace(outcome) {
	case "compra", "reserva":
		return true
	default:
		return false
	}
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

func cloneStringSlice(values []string) []string {
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			cloned = append(cloned, trimmed)
		}
	}
	return cloned
}

func cloneCampaignNames(matches []operations.CampaignMatch) []string {
	cloned := make([]string, 0, len(matches))
	for _, match := range matches {
		label := firstNonEmpty(match.Name, match.ID)
		if label != "" {
			cloned = append(cloned, label)
		}
	}

	return cloned
}

func maxFloat(value float64, minimum float64) float64 {
	if value < minimum {
		return minimum
	}
	return value
}

func maxInt64(value int64, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}
