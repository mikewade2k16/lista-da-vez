package omnichannel

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxContactMemoryEntries = 40
	maxContactMemoryValue   = 500
)

// ContactMemorySuggestion is untrusted model output. Only short scalar facts
// explicitly requested by the prompt survive normalization.
type ContactMemorySuggestion struct {
	Summary     *string        `json:"summary"`
	Facts       map[string]any `json:"facts"`
	Preferences map[string]any `json:"preferences"`
}

func normalizeContactMemory(in ContactMemorySuggestion) ContactMemorySuggestion {
	out := ContactMemorySuggestion{
		Facts:       normalizeContactMemoryMap(in.Facts),
		Preferences: normalizeContactMemoryMap(in.Preferences),
	}
	if in.Summary != nil {
		value := strings.TrimSpace(*in.Summary)
		if value != "" {
			if utf8.RuneCountInString(value) > 1000 {
				value = string([]rune(value)[:1000])
			}
			out.Summary = &value
		}
	}
	return out
}

func normalizeContactMemoryMap(in map[string]any) map[string]any {
	out := make(map[string]any)
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if len(out) >= maxContactMemoryEntries {
			break
		}
		value := in[key]
		key = strings.TrimSpace(key)
		if key == "" || len(key) > 80 || contactMemorySensitiveKey(key) {
			continue
		}
		switch typed := value.(type) {
		case string:
			typed = strings.TrimSpace(typed)
			if typed == "" {
				continue
			}
			if utf8.RuneCountInString(typed) > maxContactMemoryValue {
				typed = string([]rune(typed)[:maxContactMemoryValue])
			}
			out[key] = typed
		case bool, float64:
			out[key] = typed
		}
	}
	return out
}

func contactMemorySensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, fragment := range []string{
		"password", "senha", "token", "secret", "segredo", "api_key", "apikey",
		"cartao", "card_number", "cvv", "documento", "cpf", "cnpj", "rg",
	} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

func normalizeContactSentiment(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "positive", "neutral", "negative":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}

func contactMemoryJSON(value map[string]any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

// withContactMemoryOutputSchema upgrades already-published legacy schemas at
// execution time without mutating their audited version. This keeps old agents
// compatible while making the new fields available to both native and n8n runs.
func withContactMemoryOutputSchema(raw json.RawMessage) json.RawMessage {
	var schema map[string]any
	if json.Unmarshal(raw, &schema) != nil {
		return raw
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return raw
	}
	properties["sentiment"] = map[string]any{
		"type": "string", "enum": []string{"positive", "neutral", "negative", "unknown"},
	}
	properties["contact_memory"] = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary":     map[string]any{"type": []any{"string", "null"}, "maxLength": 1000},
			"facts":       map[string]any{"type": "object"},
			"preferences": map[string]any{"type": "object"},
		},
		"required":             []string{"facts", "preferences"},
		"additionalProperties": false,
	}
	required, _ := schema["required"].([]any)
	seen := map[string]bool{}
	for _, value := range required {
		if key, ok := value.(string); ok {
			seen[key] = true
		}
	}
	for _, key := range []string{"sentiment", "contact_memory"} {
		if !seen[key] {
			required = append(required, key)
		}
	}
	schema["required"] = required
	updated, err := json.Marshal(schema)
	if err != nil {
		return raw
	}
	return updated
}
