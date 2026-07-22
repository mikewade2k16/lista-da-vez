package omnichannel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
)

const (
	maxAIToolArgumentsBytes = 64 << 10
	maxAIToolOutputBytes    = 128 << 10
)

func validateAIToolArguments(schema, arguments json.RawMessage) error {
	if len(arguments) == 0 || len(arguments) > maxAIToolArgumentsBytes {
		return ErrAIToolArguments
	}
	var input map[string]json.RawMessage
	if err := decodeJSONObject(arguments, &input); err != nil {
		return ErrAIToolArguments
	}
	var definition map[string]json.RawMessage
	if len(schema) == 0 {
		schema = json.RawMessage(`{}`)
	}
	if err := decodeJSONObject(schema, &definition); err != nil {
		return ErrAIToolArguments
	}
	var properties map[string]json.RawMessage
	if raw := definition["properties"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &properties); err != nil {
			return ErrAIToolArguments
		}
	}
	var required []string
	if raw := definition["required"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &required); err != nil || len(required) > 64 {
			return ErrAIToolArguments
		}
	}
	for _, key := range required {
		if _, ok := input[key]; !ok {
			return ErrAIToolArguments
		}
	}
	additional := true
	if raw := definition["additionalProperties"]; len(raw) > 0 {
		var flag bool
		if json.Unmarshal(raw, &flag) == nil {
			additional = flag
		}
	}
	for key, value := range input {
		property, exists := properties[key]
		if !exists {
			if !additional {
				return ErrAIToolArguments
			}
			continue
		}
		if err := validateAIToolValue(property, value, 0); err != nil {
			return err
		}
	}
	return nil
}

func decodeJSONObject(raw []byte, out *map[string]json.RawMessage) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil || *out == nil {
		return ErrAIToolArguments
	}
	var extra any
	if decoder.Decode(&extra) == nil {
		return ErrAIToolArguments
	}
	return nil
}

func validateAIToolValue(schema, raw json.RawMessage, depth int) error {
	if depth > 5 {
		return ErrAIToolArguments
	}
	var definition map[string]json.RawMessage
	if len(schema) == 0 || json.Unmarshal(schema, &definition) != nil || definition == nil {
		return ErrAIToolArguments
	}
	var kind string
	_ = json.Unmarshal(definition["type"], &kind)
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return ErrAIToolArguments
	}
	if !matchesAIToolType(kind, value) {
		return ErrAIToolArguments
	}
	if rawEnum := definition["enum"]; len(rawEnum) > 0 {
		var options []json.RawMessage
		if json.Unmarshal(rawEnum, &options) != nil || !rawJSONEqualAny(options, raw) {
			return ErrAIToolArguments
		}
	}
	if kind == "string" {
		var text string
		_ = json.Unmarshal(raw, &text)
		if limit := integerSchemaValue(definition["maxLength"]); limit >= 0 && len([]rune(text)) > limit {
			return ErrAIToolArguments
		}
	}
	if kind == "object" {
		if err := validateAIToolArguments(schema, raw); err != nil {
			return err
		}
	}
	if kind == "array" {
		var values []json.RawMessage
		if json.Unmarshal(raw, &values) != nil || len(values) > 64 {
			return ErrAIToolArguments
		}
		for _, item := range values {
			if err := validateAIToolValue(definition["items"], item, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func matchesAIToolType(kind string, value any) bool {
	switch kind {
	case "", "object":
		_, ok := value.(map[string]any)
		return ok || kind == ""
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "integer":
		number, ok := value.(json.Number)
		if !ok {
			return false
		}
		_, err := number.Int64()
		return err == nil
	case "number":
		number, ok := value.(json.Number)
		if !ok {
			return false
		}
		parsed, err := number.Float64()
		return err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
	default:
		return false
	}
}

func integerSchemaValue(raw json.RawMessage) int {
	if len(raw) == 0 {
		return -1
	}
	var value int
	if json.Unmarshal(raw, &value) != nil || value < 0 {
		return -1
	}
	return value
}

func rawJSONEqualAny(options []json.RawMessage, raw json.RawMessage) bool {
	var want any
	if json.Unmarshal(raw, &want) != nil {
		return false
	}
	for _, option := range options {
		var candidate any
		if json.Unmarshal(option, &candidate) == nil && reflect.DeepEqual(candidate, want) {
			return true
		}
	}
	return false
}

func maskAIToolJSON(raw json.RawMessage, maxBytes int) json.RawMessage {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return json.RawMessage(`{"redacted":true}`)
	}
	value = maskAIToolValue(value, 0)
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) > maxBytes {
		return json.RawMessage(`{"truncated":true}`)
	}
	if _, ok := value.(map[string]any); !ok {
		return json.RawMessage(fmt.Sprintf(`{"value":%s}`, encoded))
	}
	return encoded
}

func maskAIToolValue(value any, depth int) any {
	if depth > 4 {
		return "[truncated]"
	}
	switch item := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(item))
		for key, nested := range item {
			lower := strings.ToLower(strings.TrimSpace(key))
			if containsAIToolSecretMarker(lower) {
				out[key] = "[redacted]"
				continue
			}
			out[key] = maskAIToolValue(nested, depth+1)
		}
		return out
	case []any:
		limit := len(item)
		if limit > 32 {
			limit = 32
		}
		out := make([]any, limit)
		for i := 0; i < limit; i++ {
			out[i] = maskAIToolValue(item[i], depth+1)
		}
		return out
	case string:
		if len([]rune(item)) > 512 {
			return string([]rune(item)[:512]) + "…"
		}
		return item
	default:
		return value
	}
}

func containsAIToolSecretMarker(key string) bool {
	for _, marker := range []string{"apikey", "api_key", "token", "password", "secret", "credential", "authorization"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}
