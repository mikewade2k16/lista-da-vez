package app

import (
	"context"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/calendar"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerintelligence"
)

type calendarBusinessContextReader interface {
	ReadCustomerIntelligenceBusinessContext(
		context.Context,
		calendar.CustomerIntelligenceBusinessContextRequest,
	) (calendar.CustomerIntelligenceBusinessContext, error)
}

type calendarClientProfileSourceAdapter struct {
	calendar func() calendarBusinessContextReader
}

func (adapter calendarClientProfileSourceAdapter) Fetch(
	ctx context.Context,
	config customerintelligence.SourceConfig,
	_ string,
) ([]customerintelligence.Observation, error) {
	if config.Mode != "on_demand" {
		return nil, newPermanentSourceFailure("source_mode_invalid")
	}
	if adapter.calendar == nil {
		return nil, newPermanentSourceFailure("source_owner_unavailable")
	}
	reader := adapter.calendar()
	if reader == nil {
		return nil, newPermanentSourceFailure("source_owner_unavailable")
	}
	var options struct {
		Sections []string `json:"sections"`
		MaxBytes int      `json:"maxBytes"`
	}
	if err := decodeSourceOptions(config.Config, &options); err != nil {
		return nil, err
	}
	businessContext, err := reader.ReadCustomerIntelligenceBusinessContext(
		ctx,
		calendar.CustomerIntelligenceBusinessContextRequest{
			AccountID:       config.AccountID,
			ClientAccountID: config.ClientAccountID,
			Sections:        options.Sections,
			MaxBytes:        options.MaxBytes,
		},
	)
	if err != nil {
		return nil, err
	}
	if !businessContext.Found || len(businessContext.Sections) == 0 {
		return []customerintelligence.Observation{}, nil
	}
	snapshot, err := sourceSnapshot(businessContext.Sections)
	if err != nil {
		return nil, err
	}
	version := "profile"
	var occurredAt *time.Time
	if businessContext.UpdatedAt != nil {
		resolved := businessContext.UpdatedAt.UTC()
		occurredAt = &resolved
		version = resolved.Format(time.RFC3339Nano)
	}
	return []customerintelligence.Observation{{
		IdempotencyKey: strings.Join(
			[]string{"calendar.client_profile", config.ClientAccountID, version},
			":",
		),
		EntityType:  "client_business_context",
		EntityID:    config.ClientAccountID,
		Version:     version,
		ScopeType:   customerintelligence.ObservationScopeBusiness,
		OccurredAt:  occurredAt,
		Snapshot:    snapshot,
		Sensitivity: "internal",
		PurposeKey:  config.PurposeKey,
	}}, nil
}

var _ customerintelligence.SourceAdapter = calendarClientProfileSourceAdapter{}
