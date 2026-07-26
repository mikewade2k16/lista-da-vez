package analytics

import (
	"sort"
	"strings"
)

const maxRecentAutoCloseRows = 50

func collectAutoCloseValidatorIDs(bundles []bundle) []string {
	seen := map[string]struct{}{}
	ids := make([]string, 0)

	for _, item := range bundles {
		for _, entry := range item.snapshot.ServiceHistory {
			userID := strings.TrimSpace(entry.ValidatedBy)
			if strings.TrimSpace(entry.CloseReason) != "auto" || userID == "" {
				continue
			}
			if _, exists := seen[userID]; exists {
				continue
			}
			seen[userID] = struct{}{}
			ids = append(ids, userID)
		}
	}

	sort.Strings(ids)
	return ids
}

func buildAutoCloseData(bundles []bundle, validatorNames map[string]string) AutoCloseData {
	result := AutoCloseData{
		ByConsultant: []AutoCloseConsultantRow{},
		ByStore:      []AutoCloseStoreRow{},
		Recent:       []AutoCloseAuditRow{},
	}
	consultants := map[string]*AutoCloseConsultantRow{}
	stores := map[string]*AutoCloseStoreRow{}

	for _, item := range bundles {
		storeID := strings.TrimSpace(item.storeID)
		storeName := strings.TrimSpace(item.storeView.Name)

		for _, entry := range item.snapshot.ServiceHistory {
			if strings.TrimSpace(entry.CloseReason) != "auto" {
				continue
			}

			entryStoreID := firstNonEmpty(strings.TrimSpace(entry.StoreID), storeID)
			status := normalizeAutoCloseStatus(entry.ValidationStatus)
			snoozeCount := entry.SnoozeCount
			if snoozeCount < 0 {
				snoozeCount = 0
			}
			result.Summary.Total++
			result.Summary.SnoozeCount += snoozeCount
			incrementAutoCloseStatus(&result.Summary.Pending, &result.Summary.Validated, &result.Summary.Cancelled, status)

			consultantKey := entryStoreID + ":" + strings.TrimSpace(entry.PersonID)
			consultantRow := consultants[consultantKey]
			if consultantRow == nil {
				consultantRow = &AutoCloseConsultantRow{
					ConsultantID:   strings.TrimSpace(entry.PersonID),
					ConsultantName: strings.TrimSpace(entry.PersonName),
					StoreID:        entryStoreID,
					StoreName:      storeName,
				}
				consultants[consultantKey] = consultantRow
			}
			consultantRow.Total++
			consultantRow.SnoozeCount += snoozeCount
			incrementAutoCloseStatus(&consultantRow.Pending, &consultantRow.Validated, &consultantRow.Cancelled, status)

			storeRow := stores[entryStoreID]
			if storeRow == nil {
				storeRow = &AutoCloseStoreRow{StoreID: entryStoreID, StoreName: storeName}
				stores[entryStoreID] = storeRow
			}
			storeRow.Total++
			storeRow.SnoozeCount += snoozeCount
			incrementAutoCloseStatus(&storeRow.Pending, &storeRow.Validated, &storeRow.Cancelled, status)

			validatorID := strings.TrimSpace(entry.ValidatedBy)
			reason := strings.TrimSpace(entry.ValidationReason)
			if status == "cancelled" {
				reason = strings.TrimSpace(entry.CancelReason)
			}
			result.Recent = append(result.Recent, AutoCloseAuditRow{
				ServiceID:      strings.TrimSpace(entry.ServiceID),
				ConsultantID:   strings.TrimSpace(entry.PersonID),
				ConsultantName: strings.TrimSpace(entry.PersonName),
				StoreID:        entryStoreID,
				StoreName:      storeName,
				Status:         status,
				Reason:         reason,
				ClosedByUserID: validatorID,
				ClosedByName:   strings.TrimSpace(validatorNames[validatorID]),
				AutoClosedAt:   entry.FinishedAt,
				ValidatedAt:    entry.ValidatedAt,
				SnoozeCount:    snoozeCount,
			})
		}
	}

	for _, row := range consultants {
		result.ByConsultant = append(result.ByConsultant, *row)
	}
	for _, row := range stores {
		result.ByStore = append(result.ByStore, *row)
	}

	sort.SliceStable(result.ByConsultant, func(i, j int) bool {
		if result.ByConsultant[i].Total == result.ByConsultant[j].Total {
			if result.ByConsultant[i].StoreName == result.ByConsultant[j].StoreName {
				return result.ByConsultant[i].ConsultantName < result.ByConsultant[j].ConsultantName
			}
			return result.ByConsultant[i].StoreName < result.ByConsultant[j].StoreName
		}
		return result.ByConsultant[i].Total > result.ByConsultant[j].Total
	})
	sort.SliceStable(result.ByStore, func(i, j int) bool {
		if result.ByStore[i].Total == result.ByStore[j].Total {
			return result.ByStore[i].StoreName < result.ByStore[j].StoreName
		}
		return result.ByStore[i].Total > result.ByStore[j].Total
	})
	sort.SliceStable(result.Recent, func(i, j int) bool {
		return result.Recent[i].AutoClosedAt > result.Recent[j].AutoClosedAt
	})
	if len(result.Recent) > maxRecentAutoCloseRows {
		result.Recent = result.Recent[:maxRecentAutoCloseRows]
	}

	return result
}

func normalizeAutoCloseStatus(value string) string {
	switch strings.TrimSpace(value) {
	case "validated", "cancelled":
		return strings.TrimSpace(value)
	default:
		return "pending"
	}
}

func incrementAutoCloseStatus(pending *int, validated *int, cancelled *int, status string) {
	switch status {
	case "validated":
		(*validated)++
	case "cancelled":
		(*cancelled)++
	default:
		(*pending)++
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
