package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerdata"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerintelligence"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/site"
)

const maxSiteSourceLinksPerRelationship = 50

type siteCustomerEvidenceReader interface {
	ReadCustomerIntelligenceEvidence(
		context.Context,
		site.CustomerIntelligenceEvidenceRequest,
	) (site.CustomerIntelligenceEvidence, error)
}

type siteSourceAdapter struct {
	customerData func() customerDataSourceEvidenceReader
	site         func() siteCustomerEvidenceReader
}

func (adapter siteSourceAdapter) Fetch(
	ctx context.Context,
	config customerintelligence.SourceConfig,
	relationshipID string,
) ([]customerintelligence.Observation, error) {
	if adapter.customerData == nil || adapter.site == nil {
		return nil, newPermanentSourceFailure("source_owner_unavailable")
	}
	dataReader := adapter.customerData()
	siteReader := adapter.site()
	if dataReader == nil || siteReader == nil || strings.TrimSpace(relationshipID) == "" {
		return nil, newPermanentSourceFailure("source_owner_unavailable")
	}
	var options struct {
		SiteID      string   `json:"siteId"`
		EntityTypes []string `json:"entityTypes"`
	}
	if err := decodeSourceOptions(config.Config, &options); err != nil {
		return nil, err
	}
	if len(options.SiteID) > 120 {
		return nil, newPermanentSourceFailure("source_config_invalid")
	}
	allowedEntityTypes, err := normalizedSiteEntityTypes(options.EntityTypes)
	if err != nil {
		return nil, err
	}
	bundle, err := dataReader.GetSourceEvidence(ctx, customerdata.SourceEvidenceRequest{
		AccountID:       config.AccountID,
		ClientAccountID: config.ClientAccountID,
		RelationshipID:  relationshipID,
		Limit:           1,
	})
	if err != nil {
		return nil, err
	}
	siteID := strings.TrimSpace(options.SiteID)
	items := make([]customerintelligence.Observation, 0)
	eligibleLinks := 0
	for _, link := range bundle.SourceLinks {
		entityType := strings.ToLower(strings.TrimSpace(link.SourceEntityType))
		if strings.TrimSpace(link.SourceModule) != "site" ||
			!sourceEntityTypeAllowed(entityType, allowedEntityTypes) ||
			strings.TrimSpace(link.SourceEntityID) == "" ||
			(siteID != "" && strings.TrimSpace(link.SourceKey) != siteID) {
			continue
		}
		eligibleLinks++
		if eligibleLinks > maxSiteSourceLinksPerRelationship {
			break
		}
		evidence, readErr := siteReader.ReadCustomerIntelligenceEvidence(
			ctx,
			site.CustomerIntelligenceEvidenceRequest{
				AccountID:  config.ClientAccountID,
				EntityType: entityType,
				EntityID:   link.SourceEntityID,
				Fields:     config.FieldAllowlist,
			},
		)
		if readErr != nil {
			return nil, readErr
		}
		snapshot, marshalErr := sourceSnapshot(evidence.Fields)
		if marshalErr != nil {
			return nil, marshalErr
		}
		occurredAt := evidence.OccurredAt
		items = append(items, customerintelligence.Observation{
			IdempotencyKey: fmt.Sprintf(
				"site:%s:%s:%s",
				evidence.EntityType,
				evidence.EntityID,
				evidence.Version,
			),
			EntityType:     "site_" + evidence.EntityType,
			EntityID:       evidence.EntityID,
			Version:        evidence.Version,
			ScopeType:      customerintelligence.ObservationScopeSubject,
			SubjectID:      bundle.SubjectID,
			RelationshipID: bundle.RelationshipID,
			OccurredAt:     &occurredAt,
			Snapshot:       snapshot,
			Sensitivity:    "personal",
			PurposeKey:     config.PurposeKey,
		})
	}
	return items, nil
}

func normalizedSiteEntityTypes(input []string) (map[string]bool, error) {
	if len(input) == 0 {
		input = []string{site.CustomerIntelligenceEntityLead}
	}
	allowed := map[string]bool{site.CustomerIntelligenceEntityLead: false}
	for _, candidate := range input {
		entityType := strings.ToLower(strings.TrimSpace(candidate))
		if _, exists := allowed[entityType]; !exists {
			return nil, newPermanentSourceFailure("source_config_invalid")
		}
		allowed[entityType] = true
	}
	return allowed, nil
}

var _ customerintelligence.SourceAdapter = siteSourceAdapter{}
