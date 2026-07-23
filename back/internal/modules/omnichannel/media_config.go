package omnichannel

import (
	"encoding/json"
	"strings"
)

// normalizeMediaConfig validates the versioned multimodal policy. Secrets are
// intentionally not part of this shape; provider credentials remain on the agent.
func normalizeMediaConfig(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return json.RawMessage(`{}`), nil
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil || root == nil {
		return nil, ErrValidation
	}
	for key, value := range root {
		if forbiddenMediaConfigKey(key) {
			return nil, ErrValidation
		}
		switch key {
		case "audio":
			if err := validateMediaSection(value, "audio"); err != nil {
				return nil, err
			}
		case "image":
			if err := validateMediaSection(value, "image"); err != nil {
				return nil, err
			}
		case "video":
			if err := validateMediaSection(value, "video"); err != nil {
				return nil, err
			}
		case "document":
			if err := validateMediaSection(value, "document"); err != nil {
				return nil, err
			}
		case "retentionDays":
			var n int
			if json.Unmarshal(value, &n) != nil || n < 1 || n > 3650 {
				return nil, ErrValidation
			}
		case "includeInReply":
			var enabled bool
			if json.Unmarshal(value, &enabled) != nil {
				return nil, ErrValidation
			}
		default:
			return nil, ErrValidation
		}
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		return nil, ErrValidation
	}
	return encoded, nil
}

func validateMediaSection(raw json.RawMessage, kind string) error {
	var section map[string]json.RawMessage
	if json.Unmarshal(raw, &section) != nil || section == nil {
		return ErrValidation
	}
	enabled := false
	provider := ""
	model := ""
	for key, value := range section {
		if forbiddenMediaConfigKey(key) {
			return ErrValidation
		}
		switch key {
		case "enabled":
			if json.Unmarshal(value, &enabled) != nil {
				return ErrValidation
			}
		case "provider", "model":
			var text string
			if json.Unmarshal(value, &text) != nil || strings.TrimSpace(text) == "" || len([]rune(text)) > 120 {
				return ErrValidation
			}
			if key == "provider" {
				provider = strings.TrimSpace(text)
			} else {
				model = strings.TrimSpace(text)
			}
		case "credentialId":
			var credentialID string
			if json.Unmarshal(value, &credentialID) != nil || !omnichannelUUIDPattern.MatchString(strings.TrimSpace(credentialID)) {
				return ErrValidation
			}
		case "maxSeconds":
			if kind != "audio" || !validMediaInt(value, 1, 600) {
				return ErrValidation
			}
		case "maxBytes":
			maxBytes := 60 << 20
			if kind == "video" {
				maxBytes = 100 << 20
			}
			if (kind != "image" && kind != "video" && kind != "document") || !validMediaInt(value, 1, maxBytes) {
				return ErrValidation
			}
		case "allowedMime":
			if kind != "document" || !validMediaMimeList(value) {
				return ErrValidation
			}
		case "maxPages":
			if kind != "document" || !validMediaInt(value, 1, 100) {
				return ErrValidation
			}
		default:
			return ErrValidation
		}
	}
	if enabled && (provider == "" || model == "") {
		return ErrValidation
	}
	return nil
}

func validMediaInt(raw json.RawMessage, min, max int) bool {
	var n int
	return json.Unmarshal(raw, &n) == nil && n >= min && n <= max
}

func validMediaMimeList(raw json.RawMessage) bool {
	var values []string
	if json.Unmarshal(raw, &values) != nil || len(values) > 20 {
		return false
	}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || len([]rune(value)) > 120 || !strings.Contains(value, "/") {
			return false
		}
	}
	return true
}

func forbiddenMediaConfigKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "credentialid" {
		return false
	}
	for _, marker := range []string{"key", "token", "secret", "password", "credential"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

type mediaCredentialBinding struct {
	Role         string
	Provider     string
	CredentialID string
}

func mediaCredentialBindings(raw json.RawMessage) ([]mediaCredentialBinding, error) {
	normalized, err := normalizeMediaConfig(raw)
	if err != nil {
		return nil, err
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(normalized, &root) != nil {
		return nil, ErrValidation
	}
	out := make([]mediaCredentialBinding, 0, 4)
	for _, role := range []string{"audio", "image", "video", "document"} {
		sectionRaw, ok := root[role]
		if !ok {
			continue
		}
		var section struct {
			Enabled      bool   `json:"enabled"`
			Provider     string `json:"provider"`
			CredentialID string `json:"credentialId"`
		}
		if json.Unmarshal(sectionRaw, &section) != nil {
			return nil, ErrValidation
		}
		if section.Enabled {
			if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(section.CredentialID)) {
				return nil, ErrValidation
			}
			out = append(out, mediaCredentialBinding{Role: role, Provider: strings.ToLower(strings.TrimSpace(section.Provider)), CredentialID: strings.TrimSpace(section.CredentialID)})
		}
	}
	return out, nil
}
