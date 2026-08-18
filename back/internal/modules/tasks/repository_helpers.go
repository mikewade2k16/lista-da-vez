package tasks

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func decodeMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return map[string]any{}
	}
	return value
}

func mustJSON(value map[string]any) []byte {
	raw, err := json.Marshal(normalizeMap(value))
	if err != nil {
		return []byte(`{}`)
	}
	return raw
}

func normalizeMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	normalized := make(map[string]any, len(value))
	for key, item := range value {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey != "" {
			normalized[normalizedKey] = item
		}
	}
	return normalized
}

func mergeMetadata(current map[string]any, patch map[string]any) map[string]any {
	next := normalizeMap(current)
	for key, value := range normalizeTaskUIMetadata(patch) {
		next[key] = value
	}
	return next
}

func normalizeTaskUIMetadata(value map[string]any) map[string]any {
	raw := normalizeMap(value)
	if len(raw) == 0 {
		return map[string]any{}
	}

	normalized := map[string]any{}
	if item, ok := raw["responsible"]; ok {
		normalized["responsible"] = strings.TrimSpace(fmt.Sprint(item))
	}
	if item, ok := raw["involved"]; ok {
		normalized["involved"] = normalizeStringList(item)
	}
	if item, ok := raw["clientId"]; ok {
		normalized["clientId"] = item
	}
	if item, ok := raw["clientName"]; ok {
		normalized["clientName"] = strings.TrimSpace(fmt.Sprint(item))
	}
	if item, ok := raw["type"]; ok {
		normalized["type"] = strings.TrimSpace(fmt.Sprint(item))
	}
	if item, ok := raw["dueEndDate"]; ok {
		normalized["dueEndDate"] = strings.TrimSpace(fmt.Sprint(item))
	}
	if item, ok := raw["prioritySet"]; ok {
		normalized["prioritySet"] = item
	}
	if item, ok := raw["createdBy"]; ok {
		normalized["createdBy"] = strings.TrimSpace(fmt.Sprint(item))
	}
	// source identifica a origem da task quando criada por outro modulo (ex.: "calendar"
	// no contrato C10). String curta na whitelist para o front distinguir a procedencia.
	if item, ok := raw["source"]; ok {
		normalized["source"] = strings.TrimSpace(fmt.Sprint(item))
	}
	if item, ok := raw["videos"]; ok {
		normalized["videos"] = normalizeTaskVideoMetadata(item)
	}
	if item, ok := raw["mediaOrder"]; ok {
		normalized["mediaOrder"] = normalizeTaskMediaOrder(item)
	}
	if item, ok := raw["checklist"]; ok {
		normalized["checklist"] = normalizeTaskChecklistMetadata(item)
	}
	// calendarMedia (WAVE 6, cruzamento A): midia ESPELHADA do evento vinculado, read-only. O
	// sync do calendario popula; aqui e' defesa (so /uploads/calendar/, dedup) contra body forjado.
	if item, ok := raw["calendarMedia"]; ok {
		normalized["calendarMedia"] = normalizeCalendarMediaMetadata(item)
	}
	return normalized
}

func normalizeTaskMediaOrder(value any) []string {
	rawList := normalizeStringList(value)
	seen := make(map[string]struct{}, len(rawList))
	result := make([]string, 0, min(len(rawList), 100))
	for _, rawID := range rawList {
		id := truncateMetadataText(rawID, 300)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
		if len(result) == 100 {
			break
		}
	}
	return result
}

func normalizeTaskChecklistMetadata(value any) []map[string]any {
	rawList, ok := value.([]any)
	if !ok {
		typed, typedOK := value.([]map[string]any)
		if !typedOK {
			return []map[string]any{}
		}
		rawList = make([]any, 0, len(typed))
		for _, item := range typed {
			rawList = append(rawList, item)
		}
	}

	const maxItems = 200
	normalized := make([]map[string]any, 0, min(len(rawList), maxItems))
	seen := map[string]struct{}{}
	for index, item := range rawList {
		if len(normalized) >= maxItems {
			break
		}
		raw, itemOK := item.(map[string]any)
		if !itemOK {
			continue
		}
		title := truncateMetadataText(raw["title"], 220)
		if title == "" {
			continue
		}
		id := truncateMetadataText(raw["id"], 120)
		if id == "" {
			id = fmt.Sprintf("item-%d", index+1)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		completed, _ := raw["completed"].(bool)
		normalizedItem := map[string]any{
			"id":        id,
			"title":     title,
			"completed": completed,
		}
		if status := normalizeTaskChecklistStatus(raw["status"]); status != "" {
			normalizedItem["status"] = status
			if statusDate := normalizeTaskChecklistDate(raw["statusDate"]); statusDate != "" {
				normalizedItem["statusDate"] = statusDate
			}
		}
		if completed {
			if completedDate := normalizeTaskChecklistDate(raw["completedDate"]); completedDate != "" {
				normalizedItem["completedDate"] = completedDate
			}
		}
		normalized = append(normalized, normalizedItem)
	}
	return normalized
}

func normalizeTaskChecklistStatus(value any) string {
	status, ok := value.(string)
	if !ok {
		return ""
	}
	status = strings.TrimSpace(status)
	switch status {
	case "captured", "editing", "approval", "approved", "scheduled", "posted":
		return status
	default:
		return ""
	}
}

func normalizeTaskChecklistDate(value any) string {
	raw, ok := value.(string)
	if !ok {
		return ""
	}
	date := strings.TrimSpace(raw)
	parsed, err := time.Parse(time.DateOnly, date)
	if err != nil || parsed.Format(time.DateOnly) != date {
		return ""
	}
	return date
}

func truncateMetadataText(value any, maxRunes int) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	runes := []rune(text)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return text
}

// normalizeCalendarMediaMetadata sanitiza a midia espelhada do calendario na task (cruzamento A,
// read-only, exibicao). Aceita so url sob /uploads/calendar/ (bloqueia externo e /uploads/tasks/);
// como e' display, imagem e video passam. Dedup por id. Mesma postura de prefixo do video da task.
func normalizeCalendarMediaMetadata(value any) []map[string]any {
	rawList, ok := value.([]any)
	if !ok {
		typed, tok := value.([]map[string]any)
		if !tok {
			return []map[string]any{}
		}
		rawList = make([]any, 0, len(typed))
		for _, item := range typed {
			rawList = append(rawList, item)
		}
	}
	out := make([]map[string]any, 0, len(rawList))
	seen := map[string]struct{}{}
	for _, item := range rawList {
		raw, rok := item.(map[string]any)
		if !rok {
			continue
		}
		url := strings.TrimSpace(fmt.Sprint(raw["url"]))
		if !strings.HasPrefix(url, "/uploads/calendar/") {
			continue
		}
		id := strings.TrimSpace(fmt.Sprint(raw["id"]))
		if id == "" {
			id = url
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		mediaType := strings.TrimSpace(fmt.Sprint(raw["type"]))
		if mediaType != "video" {
			mediaType = "image"
		}
		poster := strings.TrimSpace(fmt.Sprint(raw["posterUrl"]))
		if !strings.HasPrefix(poster, "/uploads/calendar/") {
			poster = ""
		}
		out = append(out, map[string]any{
			"id":          id,
			"url":         url,
			"name":        strings.TrimSpace(fmt.Sprint(raw["name"])),
			"type":        mediaType,
			"contentType": strings.TrimSpace(fmt.Sprint(raw["contentType"])),
			"sizeBytes":   max(metadataInt(raw["sizeBytes"]), 0),
			"posterUrl":   poster,
		})
	}
	return out
}

// metadataInt extrai um int de um valor generico de jsonb (int/float/json.Number/string), 0 se nao der.
func metadataInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
	default:
		if parsed, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(value))); err == nil {
			return parsed
		}
	}
	return 0
}

func normalizeTaskVideoMetadata(value any) []map[string]any {
	rawList, ok := value.([]any)
	if !ok {
		switch typed := value.(type) {
		case []map[string]any:
			rawList = make([]any, 0, len(typed))
			for _, item := range typed {
				rawList = append(rawList, item)
			}
		case []TaskVideo:
			rawList = make([]any, 0, len(typed))
			for _, item := range typed {
				rawList = append(rawList, map[string]any{
					"id":              item.ID,
					"name":            item.Name,
					"url":             item.URL,
					"size":            item.Size,
					"contentType":     item.ContentType,
					"checklistItemId": item.ChecklistItemID,
					"uploadedAt":      item.UploadedAt.Format(time.RFC3339Nano),
				})
			}
		default:
			return []map[string]any{}
		}
	}

	normalized := make([]map[string]any, 0, len(rawList))
	seen := map[string]struct{}{}
	for _, item := range rawList {
		raw, ok := item.(map[string]any)
		if !ok {
			if typed, ok := item.(TaskVideo); ok {
				raw = map[string]any{
					"id":              typed.ID,
					"name":            typed.Name,
					"url":             typed.URL,
					"size":            typed.Size,
					"contentType":     typed.ContentType,
					"checklistItemId": typed.ChecklistItemID,
					"uploadedAt":      typed.UploadedAt.Format(time.RFC3339Nano),
				}
			} else {
				continue
			}
		}

		url := strings.TrimSpace(fmt.Sprint(raw["url"]))
		if url == "" {
			url = strings.TrimSpace(fmt.Sprint(raw["path"]))
		}
		if !strings.HasPrefix(url, "/uploads/tasks/") {
			continue
		}

		id := strings.TrimSpace(fmt.Sprint(raw["id"]))
		if id == "" {
			id = url
		}
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}

		name := strings.TrimSpace(fmt.Sprint(raw["name"]))
		if name == "" {
			name = strings.TrimPrefix(id, "/uploads/tasks/")
		}

		sizeValue := 0
		switch typed := raw["size"].(type) {
		case int:
			sizeValue = typed
		case int32:
			sizeValue = int(typed)
		case int64:
			sizeValue = int(typed)
		case float32:
			sizeValue = int(typed)
		case float64:
			sizeValue = int(typed)
		case json.Number:
			if parsed, err := typed.Int64(); err == nil {
				sizeValue = int(parsed)
			}
		default:
			if parsed, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(raw["size"]))); err == nil {
				sizeValue = parsed
			}
		}

		contentType := strings.TrimSpace(fmt.Sprint(raw["contentType"]))
		uploadedAt := strings.TrimSpace(fmt.Sprint(raw["uploadedAt"]))
		if uploadedAt != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, uploadedAt); err == nil {
				uploadedAt = parsed.UTC().Format(time.RFC3339Nano)
			} else {
				uploadedAt = ""
			}
		}

		normalizedItem := map[string]any{
			"id":          id,
			"name":        name,
			"url":         url,
			"size":        max(sizeValue, 0),
			"contentType": contentType,
		}
		if uploadedAt != "" {
			normalizedItem["uploadedAt"] = uploadedAt
		}
		if checklistValue, exists := raw["checklistItemId"]; exists && checklistValue != nil {
			if checklistItemID := truncateMetadataText(checklistValue, 120); checklistItemID != "" {
				normalizedItem["checklistItemId"] = checklistItemID
			}
		}
		normalized = append(normalized, normalizedItem)
	}
	return normalized
}

func normalizeStringList(value any) []string {
	rawList, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]string); ok {
			result := make([]string, 0, len(typed))
			for _, item := range typed {
				trimmed := strings.TrimSpace(item)
				if trimmed != "" {
					result = append(result, trimmed)
				}
			}
			return result
		}
		return []string{}
	}

	result := make([]string, 0, len(rawList))
	for _, item := range rawList {
		trimmed := strings.TrimSpace(fmt.Sprint(item))
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseOptionalTime(raw string) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
