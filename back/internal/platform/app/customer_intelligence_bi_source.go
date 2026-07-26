package app

import (
	"context"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/bi"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/customerintelligence"
)

type biCustomerIntelligenceHealthReader interface {
	CustomerIntelligenceQueryHealth(
		context.Context,
		bi.CustomerIntelligenceQueryRequest,
	) (bi.CustomerIntelligenceQueryAvailability, error)
}

type biPerolaSourceAdapter struct {
	bi func() biCustomerIntelligenceHealthReader
}

func (adapter biPerolaSourceAdapter) Fetch(
	ctx context.Context,
	config customerintelligence.SourceConfig,
	_ string,
) ([]customerintelligence.Observation, error) {
	if config.Mode != "on_demand" {
		return nil, newPermanentSourceFailure("source_mode_invalid")
	}
	if adapter.bi == nil {
		return nil, newPermanentSourceFailure("source_owner_unavailable")
	}
	reader := adapter.bi()
	if reader == nil {
		return nil, newPermanentSourceFailure("source_owner_unavailable")
	}
	var options struct {
		DatasetID string                        `json:"datasetId"`
		Filters   []bi.PerolaDatasetFilterInput `json:"filters"`
		Limit     int                           `json:"limit"`
		OrderBy   bi.PerolaDatasetOrderInput    `json:"orderBy"`
	}
	if err := decodeSourceOptions(config.Config, &options); err != nil {
		return nil, err
	}
	health, err := reader.CustomerIntelligenceQueryHealth(
		ctx,
		bi.CustomerIntelligenceQueryRequest{
			DatasetID: options.DatasetID,
			Query: bi.PerolaDatasetQueryInput{
				PageNumber: 1,
				Limit:      options.Limit,
				OrderBy:    options.OrderBy,
				Filters:    options.Filters,
			},
		},
	)
	if err != nil {
		return nil, newPermanentSourceFailure("source_config_invalid")
	}
	if health.Status != bi.CustomerIntelligenceAvailabilityUnavailable {
		return nil, newPermanentSourceFailure("source_contract_invalid")
	}
	return nil, newPermanentSourceFailure(health.ReasonCode)
}

var _ customerintelligence.SourceAdapter = biPerolaSourceAdapter{}
