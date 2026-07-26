package customerintelligence

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const maxSourceConnectionKeyLength = 120

func validateSourceConfig(input SourceConfigInput) error {
	descriptor, ok := sourceCatalog[input.SourceKey]
	if !ok ||
		!validUUID(input.ClientAccountID) ||
		!validSourceConnectionKey(input.ConnectionKey) ||
		!validMode(input.Status, "draft", "enabled", "disabled", "error") ||
		!descriptorAllowsValue(descriptor.Modes, input.Mode) ||
		!descriptorAllowsValue(descriptor.PurposeKeys, input.PurposeKey) ||
		input.FreshnessSeconds < 0 ||
		input.FreshnessSeconds > 31536000 ||
		!validRetentionPolicyInput(input) ||
		!validJSONObject(input.Config) {
		return ErrInvalidInput
	}
	if err := validateSourceFields(descriptor, input.FieldAllowlist); err != nil {
		return err
	}
	return validateTypedSourceConfig(descriptor, input.Config)
}

func validSourceConnectionKey(value string) bool {
	return len(value) >= 1 &&
		len(value) <= maxSourceConnectionKeyLength &&
		safeKeyPattern.MatchString(value)
}

func descriptorAllowsValue(allowed []string, value string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func validateSourceFields(descriptor SourceDescriptor, fields []string) error {
	allowed := make(map[string]bool, len(descriptor.AllowedFields))
	for _, field := range descriptor.AllowedFields {
		allowed[field] = true
	}
	seen := make(map[string]bool, len(fields))
	for _, field := range fields {
		if !safeKeyPattern.MatchString(field) || !allowed[field] || seen[field] {
			return fmt.Errorf("%w: source field %q nao permitido", ErrInvalidInput, field)
		}
		seen[field] = true
	}
	return nil
}

func validateTypedSourceConfig(
	descriptor SourceDescriptor,
	raw json.RawMessage,
) error {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(normalizedJSON(raw, `{}`), &values); err != nil {
		return ErrInvalidInput
	}
	schema := make(map[string]SourceConfigFieldDescriptor, len(descriptor.ConfigSchema))
	for _, field := range descriptor.ConfigSchema {
		if field.Key == "" || schema[field.Key].Key != "" {
			return ErrInvalidInput
		}
		schema[field.Key] = field
	}
	for key, value := range values {
		field, ok := schema[key]
		if !ok {
			return fmt.Errorf("%w: config key %q nao permitida", ErrInvalidInput, key)
		}
		if err := validateSourceConfigValue(field, value); err != nil {
			return fmt.Errorf("%w: config key %q invalida", ErrInvalidInput, key)
		}
	}
	for _, field := range descriptor.ConfigSchema {
		if field.Required {
			if _, ok := values[field.Key]; !ok {
				return fmt.Errorf(
					"%w: config key obrigatoria %q ausente",
					ErrInvalidInput,
					field.Key,
				)
			}
		}
	}
	return nil
}

func validateSourceConfigValue(
	field SourceConfigFieldDescriptor,
	raw json.RawMessage,
) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ErrInvalidInput
	}
	switch field.Type {
	case "integer":
		var value int
		if err := json.Unmarshal(raw, &value); err != nil ||
			(field.Min != 0 && value < field.Min) ||
			(field.Max != 0 && value > field.Max) {
			return ErrInvalidInput
		}
	case "boolean":
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return ErrInvalidInput
		}
	case "select":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil ||
			!descriptorAllowsValue(field.Options, value) {
			return ErrInvalidInput
		}
	case "safe_key":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil ||
			!validSourceConnectionKey(value) {
			return ErrInvalidInput
		}
	case "uuid":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil ||
			len(value) != 36 ||
			!validUUID(value) {
			return ErrInvalidInput
		}
	case "string_list":
		var values []string
		if err := json.Unmarshal(raw, &values); err != nil ||
			len(values) > len(field.ElementKeys) {
			return ErrInvalidInput
		}
		seen := make(map[string]bool, len(values))
		for _, value := range values {
			if !descriptorAllowsValue(field.ElementKeys, value) || seen[value] {
				return ErrInvalidInput
			}
			seen[value] = true
		}
	default:
		return ErrInvalidInput
	}
	return nil
}
