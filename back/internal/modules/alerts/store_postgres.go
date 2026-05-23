package alerts

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func appendStoreFilter(builder *strings.Builder, args *[]any, argIndex *int, storeID string, storeIDs []string) {
	if normalizedStoreID := strings.TrimSpace(storeID); normalizedStoreID != "" {
		builder.WriteString(fmt.Sprintf(" and store_id = $%d::uuid", *argIndex))
		*args = append(*args, normalizedStoreID)
		*argIndex++
		return
	}

	normalizedStoreIDs := normalizeStringSlice(storeIDs)
	if len(normalizedStoreIDs) == 0 {
		return
	}

	builder.WriteString(fmt.Sprintf(" and store_id::text = any($%d::text[])", *argIndex))
	*args = append(*args, normalizedStoreIDs)
	*argIndex++
}

func normalizeStringSlice(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
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

func normalizeMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		cloned[trimmedKey] = value
	}

	return cloned
}

func metadataInt(metadata map[string]any, key string) int {
	value := metadataInt64(metadata, key)
	if value < 0 {
		return 0
	}
	return int(value)
}

func metadataInt64(metadata map[string]any, key string) int64 {
	if len(metadata) == 0 {
		return 0
	}

	switch value := metadata[key].(type) {
	case int:
		return int64(value)
	case int32:
		return int64(value)
	case int64:
		return value
	case float32:
		return int64(value)
	case float64:
		return int64(value)
	case json.Number:
		parsed, err := value.Int64()
		if err == nil {
			return parsed
		}
	case string:
		parsed := json.Number(strings.TrimSpace(value))
		parsedValue, err := parsed.Int64()
		if err == nil {
			return parsedValue
		}
	}

	return 0
}

func metadataString(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}

	value, exists := metadata[key]
	if !exists || value == nil {
		return ""
	}

	switch value := value.(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func marshalJSONB(metadata map[string]any) []byte {
	normalized := normalizeMetadata(metadata)
	encoded, err := json.Marshal(normalized)
	if err != nil || len(encoded) == 0 {
		return []byte("{}")
	}

	return encoded
}
