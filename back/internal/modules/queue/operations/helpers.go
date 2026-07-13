package operations

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

func cloneSessions(sessions []ConsultantSession) []ConsultantSession {
	cloned := make([]ConsultantSession, 0, len(sessions))
	cloned = append(cloned, sessions...)
	return cloned
}

func cloneCurrentStatus(currentStatus map[string]ConsultantStatus) map[string]ConsultantStatus {
	cloned := make(map[string]ConsultantStatus, len(currentStatus))
	for key, value := range currentStatus {
		cloned[key] = value
	}
	return cloned
}

func cloneHistory(history []ServiceHistoryEntry) []ServiceHistoryEntry {
	cloned := make([]ServiceHistoryEntry, 0, len(history))
	for _, item := range history {
		cloned = append(cloned, normalizeHistoryEntry(item))
	}
	return cloned
}

func cloneProducts(products []ProductEntry) []ProductEntry {
	cloned := make([]ProductEntry, 0, len(products))
	for _, item := range products {
		cloned = append(cloned, ProductEntry{
			ID:       strings.TrimSpace(item.ID),
			Name:     strings.TrimSpace(item.Name),
			Code:     strings.ToUpper(strings.TrimSpace(item.Code)),
			Price:    maxFloat(item.Price, 0),
			IsCustom: item.IsCustom,
		})
	}
	return cloned
}

func cloneSkippedPeople(items []SkippedPerson) []SkippedPerson {
	cloned := make([]SkippedPerson, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, SkippedPerson{
			ID:   strings.TrimSpace(item.ID),
			Name: strings.TrimSpace(item.Name),
		})
	}
	return cloned
}

func cloneStringSlice(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	cloned := make([]string, 0, len(items))
	cloned = append(cloned, items...)
	return cloned
}

func normalizeStringSlice(values []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func normalizeStringMap(values map[string]string) map[string]string {
	normalized := map[string]string{}
	for key, value := range values {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		normalized[trimmedKey] = strings.TrimSpace(value)
	}
	return normalized
}

func normalizeCampaignMatches(matches []CampaignMatch) []CampaignMatch {
	normalized := make([]CampaignMatch, 0, len(matches))
	for _, item := range matches {
		id := strings.TrimSpace(item.ID)
		name := strings.TrimSpace(item.Name)
		if id == "" && name == "" {
			continue
		}
		normalized = append(normalized, CampaignMatch{
			ID:          id,
			Name:        name,
			BonusAmount: maxFloat(item.BonusAmount, 0),
		})
	}
	return normalized
}

func normalizeOutcome(value string) string {
	trimmed := strings.TrimSpace(value)
	// 'auto' e o sentinela de um atendimento auto-encerrado (2h) aguardando o gerente
	// gravar o desfecho real; nao entra em finishOutcomes (validacao estrita do input
	// manual), mas precisa sobreviver ao normalizar o historico.
	if trimmed == outcomeAuto {
		return outcomeAuto
	}
	if _, ok := finishOutcomes[trimmed]; ok {
		return trimmed
	}
	return "nao-compra"
}

func normalizeStartMode(value string) string {
	switch strings.TrimSpace(value) {
	case startModeJump:
		return startModeJump
	case startModeParallel:
		return startModeParallel
	}
	return startModeQueue
}

func normalizeStatus(value string) string {
	switch strings.TrimSpace(value) {
	case statusQueue, statusService, statusPaused:
		return strings.TrimSpace(value)
	default:
		return statusAvailable
	}
}

func normalizePauseKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case pauseKindTask:
		return pauseKindTask
	default:
		return pauseKindPause
	}
}

func createServiceID(personID string, timestamp int64) string {
	buffer := make([]byte, 3)
	if _, err := rand.Read(buffer); err != nil {
		return personID + "-" + time.Now().UTC().Format("20060102150405")
	}
	return personID + "-" + strings.TrimSpace(time.UnixMilli(timestamp).UTC().Format("20060102150405")) + "-" + hex.EncodeToString(buffer)
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

func maxInt(value int, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}

func nowUnixMilli() int64 {
	return time.Now().UTC().UnixMilli()
}

func intPtr(v int) *int {
	return &v
}
