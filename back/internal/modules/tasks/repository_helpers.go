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
	if item, ok := raw["videos"]; ok {
		normalized["videos"] = normalizeTaskVideoMetadata(item)
	}
	return normalized
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
					"id":          item.ID,
					"name":        item.Name,
					"url":         item.URL,
					"size":        item.Size,
					"contentType": item.ContentType,
					"uploadedAt":  item.UploadedAt.Format(time.RFC3339Nano),
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
					"id":          typed.ID,
					"name":        typed.Name,
					"url":         typed.URL,
					"size":        typed.Size,
					"contentType": typed.ContentType,
					"uploadedAt":  typed.UploadedAt.Format(time.RFC3339Nano),
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
