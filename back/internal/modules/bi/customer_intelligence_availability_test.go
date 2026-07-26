package bi

import (
	"context"
	"errors"
	"testing"
)

func TestCustomerIntelligenceQueryHealthValidatesWithoutExecutingQuery(t *testing.T) {
	service := NewService()
	health, err := service.CustomerIntelligenceQueryHealth(
		context.Background(),
		CustomerIntelligenceQueryRequest{
			DatasetID: perolaDatasetInventoryID,
			Query: PerolaDatasetQueryInput{
				Limit: 100,
				Filters: []PerolaDatasetFilterInput{
					{Field: "itemSaldoId", Operator: "eq", Value: 123},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("CustomerIntelligenceQueryHealth: %v", err)
	}
	if health.Status != CustomerIntelligenceAvailabilityUnavailable ||
		health.ReasonCode != CustomerIntelligenceUnavailableReason {
		t.Fatalf("unexpected health: %+v", health)
	}
	if health.ValidatedLimit != 25 || health.ValidatedFilters != 1 {
		t.Fatalf("registry bounds were not preserved: %+v", health)
	}
}

func TestCustomerIntelligenceQueryHealthRejectsOpenInventory(t *testing.T) {
	service := NewService()
	_, err := service.CustomerIntelligenceQueryHealth(
		context.Background(),
		CustomerIntelligenceQueryRequest{
			DatasetID: perolaDatasetInventoryID,
			Query:     PerolaDatasetQueryInput{Limit: 1},
		},
	)
	if !errors.Is(err, ErrFilterRequired) {
		t.Fatalf("expected selective filter error, got %v", err)
	}
}
