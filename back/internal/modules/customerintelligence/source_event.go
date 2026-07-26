package customerintelligence

import (
	"context"
	"fmt"
	"strings"
)

// SourceEventRequest is the headless trigger used by trusted module adapters
// after a durable domain event has already been accepted by its owner.
type SourceEventRequest struct {
	AccountID       string
	ClientAccountID string
	SourceKey       string
	RelationshipID  string
	EventID         string
}

type SourceEventResult struct {
	MatchedConfigs int `json:"matchedConfigs"`
	CreatedRuns    int `json:"createdRuns"`
	ReplayedRuns   int `json:"replayedRuns"`
}

// TriggerSourceEvent schedules every enabled event-mode config for one source.
// It does not fetch synchronously and therefore cannot block the producer's
// transactional path.
func (s *Service) TriggerSourceEvent(
	ctx context.Context,
	request SourceEventRequest,
) (SourceEventResult, error) {
	scope := Scope{
		AccountID:       strings.TrimSpace(request.AccountID),
		ClientAccountID: strings.TrimSpace(request.ClientAccountID),
	}
	if !validSourceKey(request.SourceKey) ||
		!validUUID(request.RelationshipID) ||
		!requestKeyPattern.MatchString(request.EventID) {
		return SourceEventResult{}, ErrInvalidInput
	}
	_, enabled, err := s.capability(ctx, scope, CapabilitySourceSync)
	if err != nil {
		return SourceEventResult{}, err
	}
	if !enabled {
		return SourceEventResult{}, nil
	}
	if err := s.authorizeRelationship(
		ctx,
		scope,
		"",
		request.RelationshipID,
	); err != nil {
		return SourceEventResult{}, err
	}
	configs, err := s.foundation.ListSourceConfigs(ctx, scope)
	if err != nil {
		return SourceEventResult{}, err
	}
	result := SourceEventResult{}
	for _, config := range configs {
		if config.SourceKey != request.SourceKey ||
			config.Status != "enabled" ||
			config.Mode != "event" {
			continue
		}
		result.MatchedConfigs++
		_, created, createErr := s.foundation.CreateSourceRun(ctx, SourceSyncRequest{
			AccountID:       scope.AccountID,
			ClientAccountID: scope.ClientAccountID,
			SourceConfigID:  config.ID,
			IdempotencyKey: fmt.Sprintf(
				"source.event:%s:%s",
				config.ID,
				request.EventID,
			),
			Trigger:        "event",
			RelationshipID: request.RelationshipID,
		})
		if createErr != nil {
			return result, createErr
		}
		if created {
			result.CreatedRuns++
		} else {
			result.ReplayedRuns++
		}
	}
	return result, nil
}
