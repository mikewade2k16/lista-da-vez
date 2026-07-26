package customerdata

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func (s *Service) ListOfflineInteractions(
	ctx context.Context,
	principal auth.Principal,
	relationshipID string,
	limit int,
) ([]OfflineInteraction, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceRelationship, relationshipID, permOfflineView)
	if err != nil {
		return nil, err
	}
	var reveal func(string, string) (string, error)
	if s.protector != nil {
		reveal = func(ciphertext, keyVersion string) (string, error) {
			return s.protector.RevealContent(scope, ciphertext, keyVersion)
		}
	}
	return s.repo.ListOfflineInteractions(ctx, scope, relationshipID, boundedLimit(limit, 50, 100), reveal)
}

func (s *Service) CreateOfflineInteraction(
	ctx context.Context,
	principal auth.Principal,
	relationshipID string,
	input OfflineInteractionInput,
) (OfflineInteraction, bool, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceRelationship, relationshipID, permOfflineManage)
	if err != nil {
		return OfflineInteraction{}, false, err
	}
	input.RelationshipID = relationshipID
	input.ClientAccountID = scope.ClientAccountID
	return s.createOffline(ctx, scope, input)
}

func (s *Service) UpdateOfflineInteraction(
	ctx context.Context,
	principal auth.Principal,
	interactionID string,
	patch OfflineInteractionPatch,
) (OfflineInteraction, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceOffline, interactionID, permOfflineManage)
	if err != nil {
		return OfflineInteraction{}, err
	}
	if patch.ExpectedRevision <= 0 {
		return OfflineInteraction{}, invalid("expectedRevision", "must_be_positive")
	}
	if err := validateOfflinePatch(patch); err != nil {
		return OfflineInteraction{}, err
	}
	if err := s.requireCapability(ctx, scope, CapabilityOffline, false); err != nil {
		return OfflineInteraction{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterRelationship); err != nil {
		return OfflineInteraction{}, err
	}
	ciphertext, keyVersion, err := s.protectOfflineContent(scope, patch.Sensitivity, patch.Content)
	if err != nil {
		return OfflineInteraction{}, err
	}
	return s.repo.UpdateOfflineInteraction(ctx, scope, interactionID, patch, ciphertext, keyVersion)
}

func (s *Service) ArchiveOfflineInteraction(
	ctx context.Context,
	principal auth.Principal,
	interactionID string,
	expectedRevision int64,
) (OfflineInteraction, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceOffline, interactionID, permOfflineManage)
	if err != nil {
		return OfflineInteraction{}, err
	}
	if expectedRevision <= 0 {
		return OfflineInteraction{}, invalid("expectedRevision", "must_be_positive")
	}
	if err := s.requireCapability(ctx, scope, CapabilityOffline, false); err != nil {
		return OfflineInteraction{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterRelationship); err != nil {
		return OfflineInteraction{}, err
	}
	return s.repo.ArchiveOfflineInteraction(ctx, scope, interactionID, expectedRevision)
}

// IngestOfflineInteraction é a façade owner-scoped para adapters confiáveis.
func (s *Service) IngestOfflineInteraction(ctx context.Context, input OfflineInteractionInput) (OfflineInteraction, bool, error) {
	if s == nil || s.repo == nil {
		return OfflineInteraction{}, false, ErrNotFound
	}
	scope, err := s.repo.ResolveServiceScope(ctx, strings.TrimSpace(input.AccountID), strings.TrimSpace(input.ClientAccountID))
	if err != nil {
		return OfflineInteraction{}, false, concealScopeError(err)
	}
	return s.createOffline(ctx, scope, input)
}

// GetSourceEvidence is the owner-scoped read façade used by registered
// Customer Intelligence adapters. It confirms the relationship in Customer
// Data before returning source links or offline content and obeys the offline
// capability independently from writer state.
func (s *Service) GetSourceEvidence(
	ctx context.Context,
	request SourceEvidenceRequest,
) (SourceEvidenceBundle, error) {
	if s == nil || s.repo == nil {
		return SourceEvidenceBundle{}, ErrNotFound
	}
	scope, err := s.repo.ResolveServiceScope(
		ctx,
		strings.TrimSpace(request.AccountID),
		strings.TrimSpace(request.ClientAccountID),
	)
	if err != nil {
		return SourceEvidenceBundle{}, concealScopeError(err)
	}
	relationshipID := strings.TrimSpace(request.RelationshipID)
	profile, err := s.repo.GetProfile(
		ctx,
		scope,
		relationshipID,
		ProfileSections{},
	)
	if err != nil {
		return SourceEvidenceBundle{}, concealScopeError(err)
	}
	links, err := s.repo.ListSourceReferences(ctx, scope, relationshipID)
	if err != nil {
		return SourceEvidenceBundle{}, err
	}
	out := SourceEvidenceBundle{
		SubjectID:      profile.Subject.ID,
		RelationshipID: profile.Relationship.ID,
		SourceLinks:    links,
		Interactions:   []OfflineInteraction{},
	}
	if err := s.requireCapability(ctx, scope, CapabilityOffline, true); err != nil {
		if errors.Is(err, ErrCapabilityDisabled) {
			return out, nil
		}
		return SourceEvidenceBundle{}, err
	}
	var reveal func(string, string) (string, error)
	if s.protector != nil {
		reveal = func(ciphertext, keyVersion string) (string, error) {
			return s.protector.RevealContent(scope, ciphertext, keyVersion)
		}
	}
	out.Interactions, err = s.repo.ListOfflineInteractions(
		ctx,
		scope,
		relationshipID,
		boundedLimit(request.Limit, 50, 200),
		reveal,
	)
	if err != nil {
		return SourceEvidenceBundle{}, err
	}
	return out, nil
}

func (s *Service) createOffline(ctx context.Context, scope Scope, input OfflineInteractionInput) (OfflineInteraction, bool, error) {
	if err := validateOffline(input); err != nil {
		return OfflineInteraction{}, false, err
	}
	if err := s.requireCapability(ctx, scope, CapabilityOffline, false); err != nil {
		return OfflineInteraction{}, false, err
	}
	if err := s.requireWriter(ctx, scope, WriterRelationship); err != nil {
		return OfflineInteraction{}, false, err
	}
	var content *string
	if input.Content != "" {
		content = &input.Content
	}
	ciphertext, keyVersion, err := s.protectOfflineContent(scope, &input.Sensitivity, content)
	if err != nil {
		return OfflineInteraction{}, false, err
	}
	input.ClientAccountID = scope.ClientAccountID
	return s.repo.CreateOfflineInteraction(ctx, scope, input, ciphertext, keyVersion)
}

func (s *Service) protectOfflineContent(scope Scope, sensitivity *string, content *string) (string, string, error) {
	if content == nil || strings.TrimSpace(*content) == "" {
		return "", "", nil
	}
	level := "internal"
	if sensitivity != nil {
		level = strings.TrimSpace(*sensitivity)
	}
	switch level {
	case "public", "internal":
		return "", "", nil
	case "personal", "sensitive", "restricted":
		if s.protector == nil {
			return "", "", ErrIdentityProtectionUnavailable
		}
		return s.protector.ProtectContent(scope, *content)
	default:
		return "", "", invalid("sensitivity", "unsupported")
	}
}

func validateOffline(input OfflineInteractionInput) error {
	switch input.InteractionType {
	case "meeting", "call", "offline_chat", "visit", "note", "other":
	default:
		return invalid("interactionType", "unsupported")
	}
	if input.OccurredAt.IsZero() || input.OccurredAt.After(time.Now().Add(24*time.Hour)) {
		return invalid("occurredAt", "invalid")
	}
	if strings.TrimSpace(input.Timezone) == "" || len(input.Timezone) > 80 {
		return invalid("timezone", "invalid_length")
	}
	if input.DurationSeconds != nil && (*input.DurationSeconds < 0 || *input.DurationSeconds > 86400) {
		return invalid("durationSeconds", "out_of_range")
	}
	if strings.TrimSpace(input.Title) == "" || len(input.Title) > 240 {
		return invalid("title", "invalid_length")
	}
	if len(input.Content) > 20000 {
		return invalid("content", "too_long")
	}
	switch input.Sensitivity {
	case "public", "internal", "personal", "sensitive", "restricted":
	default:
		return invalid("sensitivity", "unsupported")
	}
	if (input.Sensitivity == "personal" || input.Sensitivity == "sensitive" || input.Sensitivity == "restricted") &&
		strings.TrimSpace(input.Content) == "" {
		return invalid("content", "required_for_sensitive")
	}
	if strings.TrimSpace(input.PurposeKey) == "" || len(input.PurposeKey) > 120 {
		return invalid("purposeKey", "invalid_length")
	}
	if len(strings.TrimSpace(input.IdempotencyKey)) < 8 {
		return invalid("idempotencyKey", "invalid_length")
	}
	return nil
}

func validateOfflinePatch(patch OfflineInteractionPatch) error {
	if patch.InteractionType != nil {
		switch *patch.InteractionType {
		case "meeting", "call", "offline_chat", "visit", "note", "other":
		default:
			return invalid("interactionType", "unsupported")
		}
	}
	if patch.Title != nil && (strings.TrimSpace(*patch.Title) == "" || len(*patch.Title) > 240) {
		return invalid("title", "invalid_length")
	}
	if patch.Content != nil && len(*patch.Content) > 20000 {
		return invalid("content", "too_long")
	}
	if (patch.Content == nil) != (patch.Sensitivity == nil) {
		return invalid("content", "content_and_sensitivity_required_together")
	}
	if patch.Sensitivity != nil {
		switch *patch.Sensitivity {
		case "public", "internal", "personal", "sensitive", "restricted":
		default:
			return invalid("sensitivity", "unsupported")
		}
	}
	return nil
}
