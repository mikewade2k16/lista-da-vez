package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/crm/erp"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerdata"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerintelligence"
)

const maxERPSourceLinksPerRelationship = 50

type erpCustomerEvidenceReader interface {
	ReadCustomerIntelligenceEvidence(
		context.Context,
		erp.CustomerIntelligenceEvidenceRequest,
	) (erp.CustomerIntelligenceEvidence, error)
}

type erpSourceAdapter struct {
	customerData func() customerDataSourceEvidenceReader
	erp          func() erpCustomerEvidenceReader
}

func (adapter erpSourceAdapter) Fetch(
	ctx context.Context,
	config customerintelligence.SourceConfig,
	relationshipID string,
) ([]customerintelligence.Observation, error) {
	if adapter.customerData == nil || adapter.erp == nil {
		return nil, newPermanentSourceFailure("source_owner_unavailable")
	}
	dataReader := adapter.customerData()
	erpReader := adapter.erp()
	if dataReader == nil || erpReader == nil || strings.TrimSpace(relationshipID) == "" {
		return nil, newPermanentSourceFailure("source_owner_unavailable")
	}
	var options struct {
		ConnectionID string   `json:"connectionId"`
		EntityTypes  []string `json:"entityTypes"`
		LookbackDays int      `json:"lookbackDays"`
	}
	if err := decodeSourceOptions(config.Config, &options); err != nil {
		return nil, err
	}
	if options.LookbackDays < 0 || options.LookbackDays > 3650 ||
		len(options.ConnectionID) > 120 {
		return nil, newPermanentSourceFailure("source_config_invalid")
	}
	allowedEntityTypes, err := normalizedERPEntityTypes(options.EntityTypes)
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
	connectionID := strings.TrimSpace(options.ConnectionID)
	items := make([]customerintelligence.Observation, 0)
	eligibleLinks := 0
	for _, link := range bundle.SourceLinks {
		entityType := strings.ToLower(strings.TrimSpace(link.SourceEntityType))
		if strings.TrimSpace(link.SourceModule) != "erp" ||
			!sourceEntityTypeAllowed(entityType, allowedEntityTypes) ||
			strings.TrimSpace(link.SourceEntityID) == "" ||
			(connectionID != "" && strings.TrimSpace(link.SourceKey) != connectionID) {
			continue
		}
		eligibleLinks++
		if eligibleLinks > maxERPSourceLinksPerRelationship {
			break
		}
		evidence, readErr := erpReader.ReadCustomerIntelligenceEvidence(
			ctx,
			erp.CustomerIntelligenceEvidenceRequest{
				ClientAccountID: config.ClientAccountID,
				EntityType:      entityType,
				EntityID:        link.SourceEntityID,
				Fields:          config.FieldAllowlist,
			},
		)
		if readErr != nil {
			return nil, readErr
		}
		if options.LookbackDays > 0 &&
			evidence.OccurredAt != nil &&
			evidence.OccurredAt.Before(time.Now().UTC().AddDate(0, 0, -options.LookbackDays)) {
			continue
		}
		snapshot, marshalErr := sourceSnapshot(evidence.Fields)
		if marshalErr != nil {
			return nil, marshalErr
		}
		items = append(items, customerintelligence.Observation{
			IdempotencyKey: fmt.Sprintf(
				"erp:%s:%s:%s",
				evidence.EntityType,
				evidence.EntityID,
				evidence.Version,
			),
			EntityType:     "erp_" + evidence.EntityType,
			EntityID:       evidence.EntityID,
			Version:        evidence.Version,
			ScopeType:      customerintelligence.ObservationScopeSubject,
			SubjectID:      bundle.SubjectID,
			RelationshipID: bundle.RelationshipID,
			OccurredAt:     evidence.OccurredAt,
			Snapshot:       snapshot,
			Sensitivity:    "personal",
			PurposeKey:     config.PurposeKey,
		})
	}
	return items, nil
}

func normalizedERPEntityTypes(input []string) (map[string]bool, error) {
	if len(input) == 0 {
		input = []string{erp.CustomerIntelligenceEntityCustomer}
	}
	allowed := map[string]bool{
		erp.CustomerIntelligenceEntityCustomer:      false,
		erp.CustomerIntelligenceEntityOrder:         false,
		erp.CustomerIntelligenceEntityOrderCanceled: false,
	}
	for _, candidate := range input {
		entityType := strings.ToLower(strings.TrimSpace(candidate))
		if _, exists := allowed[entityType]; !exists {
			return nil, newPermanentSourceFailure("source_config_invalid")
		}
		allowed[entityType] = true
	}
	return allowed, nil
}

var _ customerintelligence.SourceAdapter = erpSourceAdapter{}
