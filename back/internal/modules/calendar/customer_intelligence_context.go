package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	customerIntelligenceContextSchema  = "calendar.client_business_context.v1"
	defaultBusinessContextMaxBytes     = 16 * 1024
	maxBusinessContextMaxBytes         = 64 * 1024
	maxBusinessContextFieldRunes       = 2_000
	businessContextSectionStrategy     = "strategy"
	businessContextSectionPresence     = "presence"
	businessContextSectionVoice        = "voice"
	businessContextSectionBrief        = "brief"
	businessContextWarningByteLimit    = "context_byte_limit_reached"
	businessContextWarningFieldTrimmed = "context_field_trimmed"
)

var ErrInvalidBusinessContext = errors.New("calendar: invalid business context request")

// CustomerIntelligenceBusinessContextRequest is the trusted, owner-scoped read
// contract used by the composition root. AccountID is the calendar owner and
// ClientAccountID is the client whose strategic profile is requested.
type CustomerIntelligenceBusinessContextRequest struct {
	AccountID       string
	ClientAccountID string
	Sections        []string
	MaxBytes        int
}

// CustomerIntelligenceBusinessContext deliberately has no subject or
// relationship identifier. Strategic positioning and brand voice describe the
// client business, not an individual customer.
type CustomerIntelligenceBusinessContext struct {
	SchemaVersion   string
	ClientAccountID string
	Sections        map[string]json.RawMessage
	UpdatedAt       *time.Time
	Found           bool
	Warnings        []string
}

// ReadCustomerIntelligenceBusinessContext reads only calendar-owned data,
// applies a closed section allowlist, and returns a byte-bounded projection.
func (s *Service) ReadCustomerIntelligenceBusinessContext(
	ctx context.Context,
	request CustomerIntelligenceBusinessContextRequest,
) (CustomerIntelligenceBusinessContext, error) {
	if s == nil || s.store == nil {
		return CustomerIntelligenceBusinessContext{}, ErrNotFound
	}
	accountID := normalizeUUID(request.AccountID)
	clientAccountID := normalizeUUID(request.ClientAccountID)
	if accountID == "" || clientAccountID == "" {
		return CustomerIntelligenceBusinessContext{}, ErrInvalidBusinessContext
	}
	sections, err := normalizeBusinessContextSections(request.Sections)
	if err != nil {
		return CustomerIntelligenceBusinessContext{}, err
	}
	profile, found, err := s.store.GetClientProfile(ctx, accountID, clientAccountID)
	if err != nil {
		return CustomerIntelligenceBusinessContext{}, err
	}
	out := CustomerIntelligenceBusinessContext{
		SchemaVersion:   customerIntelligenceContextSchema,
		ClientAccountID: clientAccountID,
		Sections:        map[string]json.RawMessage{},
		Found:           found,
		Warnings:        []string{},
	}
	if !found {
		return out, nil
	}
	updatedAt := profile.UpdatedAt.UTC()
	out.UpdatedAt = &updatedAt
	out.Sections, out.Warnings, err = projectBusinessContext(
		profile,
		sections,
		boundedBusinessContextBytes(request.MaxBytes),
	)
	if err != nil {
		return CustomerIntelligenceBusinessContext{}, err
	}
	return out, nil
}

func normalizeBusinessContextSections(input []string) ([]string, error) {
	if len(input) == 0 {
		return []string{
			businessContextSectionStrategy,
			businessContextSectionPresence,
			businessContextSectionVoice,
			businessContextSectionBrief,
		}, nil
	}
	allowed := map[string]bool{
		businessContextSectionStrategy: true,
		businessContextSectionPresence: true,
		businessContextSectionVoice:    true,
		businessContextSectionBrief:    true,
	}
	out := make([]string, 0, len(input))
	seen := make(map[string]bool, len(input))
	for _, candidate := range input {
		section := strings.ToLower(strings.TrimSpace(candidate))
		if !allowed[section] {
			return nil, ErrInvalidBusinessContext
		}
		if !seen[section] {
			seen[section] = true
			out = append(out, section)
		}
	}
	if len(out) == 0 {
		return nil, ErrInvalidBusinessContext
	}
	return out, nil
}

func boundedBusinessContextBytes(value int) int {
	if value <= 0 {
		return defaultBusinessContextMaxBytes
	}
	if value > maxBusinessContextMaxBytes {
		return maxBusinessContextMaxBytes
	}
	return value
}

func projectBusinessContext(
	profile ClientProfile,
	sections []string,
	maxBytes int,
) (map[string]json.RawMessage, []string, error) {
	candidates := map[string]map[string]string{
		businessContextSectionStrategy: {
			"segment":     profile.Segment,
			"positioning": profile.Positioning,
			"description": profile.Description,
			"history":     profile.History,
			"objectives":  profile.Objectives,
		},
		businessContextSectionPresence: {
			"site_url":  profile.SiteURL,
			"instagram": profile.Instagram,
			"address":   profile.Address,
		},
		businessContextSectionVoice: {
			"brand_voice": profile.BrandVoice,
		},
		businessContextSectionBrief: {
			"audience":     profile.Extra.Audience,
			"offer":        profile.Extra.Offer,
			"pillars":      profile.Extra.Pillars,
			"cadence":      profile.Extra.Cadence,
			"restrictions": profile.Extra.Restrictions,
			"performance":  profile.Extra.Performance,
			"assets":       profile.Extra.Assets,
		},
	}
	out := make(map[string]json.RawMessage, len(sections))
	warnings := make([]string, 0, 2)
	usedBytes := 2
	for _, section := range sections {
		values := make(map[string]string)
		for key, raw := range candidates[section] {
			value, trimmed := boundedBusinessContextString(raw)
			if value == "" {
				continue
			}
			if trimmed && !containsString(warnings, businessContextWarningFieldTrimmed) {
				warnings = append(warnings, businessContextWarningFieldTrimmed)
			}
			values[key] = value
		}
		if len(values) == 0 {
			continue
		}
		encoded, err := json.Marshal(values)
		if err != nil {
			return nil, nil, err
		}
		projectedBytes := usedBytes + len(section) + len(encoded) + 4
		if projectedBytes > maxBytes {
			if !containsString(warnings, businessContextWarningByteLimit) {
				warnings = append(warnings, businessContextWarningByteLimit)
			}
			continue
		}
		out[section] = encoded
		usedBytes = projectedBytes
	}
	return out, warnings, nil
}

func boundedBusinessContextString(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if utf8.RuneCountInString(trimmed) <= maxBusinessContextFieldRunes {
		return trimmed, false
	}
	runes := []rune(trimmed)
	return string(runes[:maxBusinessContextFieldRunes]), true
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
