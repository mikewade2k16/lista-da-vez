package customerintelligence

import (
	"context"
	"encoding/json"
)

func (s *Service) SourceSuggestions(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	limit int,
) ([]SourceSuggestion, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return nil, err
	}
	if !validUUID(relationshipID) {
		return nil, ErrInvalidInput
	}
	if err := s.authorizeRelationship(ctx, scope, "", relationshipID); err != nil {
		return nil, err
	}
	_, enabled, err := s.capability(ctx, scope, CapabilityProfile)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return []SourceSuggestion{}, nil
	}
	items, err := s.foundation.ListSourceSuggestions(
		ctx,
		scope,
		relationshipID,
		bounded(limit, 50, 1, 200),
	)
	if err != nil {
		return nil, err
	}
	for index := range items {
		if err := s.materializeSourceSuggestion(&items[index]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Service) ReviewSourceSuggestion(
	ctx context.Context,
	scope Scope,
	actorID, suggestionID string,
	input SourceSuggestionFeedback,
) (SourceSuggestion, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return SourceSuggestion{}, err
	}
	if !validUUID(actorID) ||
		!validUUID(suggestionID) ||
		!validMode(input.Status, "accepted", "rejected") ||
		!safeKeyPattern.MatchString(input.Reason) ||
		len(input.Reason) > 80 {
		return SourceSuggestion{}, ErrInvalidInput
	}
	item, err := s.foundation.ReviewSourceSuggestion(
		ctx,
		scope,
		actorID,
		suggestionID,
		input,
	)
	if err != nil {
		return SourceSuggestion{}, err
	}
	if err := s.materializeSourceSuggestion(&item); err != nil {
		return SourceSuggestion{}, err
	}
	return item, nil
}

func (s *Service) materializeSourceSuggestion(item *SourceSuggestion) error {
	if item.RationaleCiphertext == "" || s.secrets == nil {
		return ErrSecretsUnavailable
	}
	plaintext, err := s.secrets.Decrypt(item.RationaleCiphertext)
	if err != nil {
		return err
	}
	raw := json.RawMessage(plaintext)
	if err := validateTypedProcessOutput("source.suggest", raw); err != nil {
		return ErrInvalidInput
	}
	var output sourceSuggestionResult
	if err := decodeStrictProcessOutput(raw, &output); err != nil {
		return ErrInvalidInput
	}
	for _, suggestion := range output.Suggestions {
		if suggestion.SourceKey == item.SourceKey {
			item.Rationale = suggestion.Rationale
			item.RationaleCiphertext = ""
			return nil
		}
	}
	return ErrInvalidInput
}
