package site

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	CustomerIntelligenceEntityLead       = "lead"
	maxCustomerIntelligenceSiteTextRunes = 2_000
)

var customerIntelligenceUUIDPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`,
)

type CustomerIntelligenceEvidenceRequest struct {
	AccountID  string
	EntityType string
	EntityID   string
	Fields     []string
}

type CustomerIntelligenceEvidence struct {
	EntityType string
	EntityID   string
	Version    string
	OccurredAt time.Time
	Fields     map[string]any
}

// ReadCustomerIntelligenceEvidence exposes a minimized projection of one exact
// Site entity. The first rollout supports only a lead UUID that Customer Data
// has already linked deterministically; visitor/session IDs are never accepted.
func (s *Service) ReadCustomerIntelligenceEvidence(
	ctx context.Context,
	request CustomerIntelligenceEvidenceRequest,
) (CustomerIntelligenceEvidence, error) {
	if s == nil || s.leads == nil {
		return CustomerIntelligenceEvidence{}, ErrLeadNotFound
	}
	accountID := strings.TrimSpace(request.AccountID)
	entityType := strings.ToLower(strings.TrimSpace(request.EntityType))
	entityID := strings.TrimSpace(request.EntityID)
	if !customerIntelligenceUUIDPattern.MatchString(accountID) ||
		entityType != CustomerIntelligenceEntityLead ||
		!customerIntelligenceUUIDPattern.MatchString(entityID) ||
		len(request.Fields) == 0 {
		return CustomerIntelligenceEvidence{}, ErrInvalidEntityType
	}
	lead, err := s.leads.Find(ctx, accountID, entityID)
	if err != nil {
		return CustomerIntelligenceEvidence{}, err
	}
	fields := projectCustomerIntelligenceLead(lead, request.Fields)
	if len(fields) == 0 {
		return CustomerIntelligenceEvidence{}, ErrInvalidEntityType
	}
	return CustomerIntelligenceEvidence{
		EntityType: entityType,
		EntityID:   lead.ID,
		Version:    lead.UpdatedAt.UTC().Format(time.RFC3339Nano),
		OccurredAt: lead.CreatedAt.UTC(),
		Fields:     fields,
	}, nil
}

func projectCustomerIntelligenceLead(lead LeadView, requested []string) map[string]any {
	allowed := make(map[string]bool, len(requested))
	for _, field := range requested {
		allowed[strings.TrimSpace(field)] = true
	}
	out := make(map[string]any)
	put := func(key string, value any) {
		if !allowed[key] {
			return
		}
		if text, ok := value.(string); ok {
			text = boundedCustomerIntelligenceSiteText(text)
			if text == "" {
				return
			}
			value = text
		}
		out[key] = value
	}
	put("source_label", lead.SourceLabel)
	put("page", lead.Page)
	put("coupon", lead.Cupom)
	put("consent", lead.Consent)
	put("consent_label", lead.ConsentLabel)
	put("status", lead.Status)
	put("created_at", lead.CreatedAt.UTC().Format(time.RFC3339))
	return out
}

func boundedCustomerIntelligenceSiteText(value string) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= maxCustomerIntelligenceSiteTextRunes {
		return value
	}
	return string([]rune(value)[:maxCustomerIntelligenceSiteTextRunes])
}
