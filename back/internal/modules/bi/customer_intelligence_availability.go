package bi

import "context"

const (
	CustomerIntelligenceAvailabilityUnavailable = "unavailable"
	CustomerIntelligenceUnavailableReason       = "deterministic_subject_link_unavailable"
)

type CustomerIntelligenceQueryRequest struct {
	DatasetID string
	Query     PerolaDatasetQueryInput
}

type CustomerIntelligenceQueryAvailability struct {
	Status             string
	ReasonCode         string
	DatasetID          string
	ValidatedLimit     int
	ValidatedFilters   int
	RequiredFilterRule string
}

// CustomerIntelligenceQueryHealth validates a proposed on-demand query against
// the BI-owned closed registry without executing an external request. The
// current BI contract has no deterministic Customer Data relationship key, so
// it explicitly remains unavailable instead of exposing a broad query proxy.
func (service *Service) CustomerIntelligenceQueryHealth(
	_ context.Context,
	request CustomerIntelligenceQueryRequest,
) (CustomerIntelligenceQueryAvailability, error) {
	spec, ok := findPerolaDatasetSpec(request.DatasetID)
	if !ok {
		return CustomerIntelligenceQueryAvailability{}, ErrUnsupportedDataset
	}
	normalized, err := normalizePerolaDatasetQuery(spec, request.Query)
	if err != nil {
		return CustomerIntelligenceQueryAvailability{}, err
	}
	return CustomerIntelligenceQueryAvailability{
		Status:             CustomerIntelligenceAvailabilityUnavailable,
		ReasonCode:         CustomerIntelligenceUnavailableReason,
		DatasetID:          spec.ID,
		ValidatedLimit:     normalized.Limit,
		ValidatedFilters:   len(normalized.Filters),
		RequiredFilterRule: spec.RequiredFilterRule,
	}, nil
}
