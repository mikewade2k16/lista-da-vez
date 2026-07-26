package customerdata

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func (s *Service) ListNotes(ctx context.Context, principal auth.Principal, relationshipID string, limit int) ([]Note, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceRelationship, relationshipID, permNotesView)
	if err != nil {
		return nil, err
	}
	return s.repo.ListNotes(ctx, scope, relationshipID, boundedLimit(limit, 50, 100))
}

func (s *Service) CreateNote(ctx context.Context, principal auth.Principal, relationshipID string, input NoteInput) (Note, bool, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceRelationship, relationshipID, permNotesManage)
	if err != nil {
		return Note{}, false, err
	}
	if err := s.requireCapability(ctx, scope, CapabilityCore, false); err != nil {
		return Note{}, false, err
	}
	if err := s.requireWriter(ctx, scope, WriterNote); err != nil {
		return Note{}, false, err
	}
	if strings.TrimSpace(input.Content) == "" || len(input.Content) > 10000 {
		return Note{}, false, invalid("content", "invalid_length")
	}
	if len(strings.TrimSpace(input.IdempotencyKey)) < 8 {
		return Note{}, false, invalid("idempotencyKey", "invalid_length")
	}
	return s.repo.CreateNote(ctx, scope, relationshipID, input)
}

func (s *Service) UpdateNote(ctx context.Context, principal auth.Principal, noteID string, patch NotePatch) (Note, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceNote, noteID, permNotesManage)
	if err != nil {
		return Note{}, err
	}
	if patch.ExpectedRevision <= 0 || strings.TrimSpace(patch.Content) == "" || len(patch.Content) > 10000 {
		return Note{}, invalid("note", "invalid_patch")
	}
	if err := s.requireCapability(ctx, scope, CapabilityCore, false); err != nil {
		return Note{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterNote); err != nil {
		return Note{}, err
	}
	return s.repo.UpdateNote(ctx, scope, noteID, patch)
}

func (s *Service) ArchiveNote(ctx context.Context, principal auth.Principal, noteID string, expectedRevision int64) (Note, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceNote, noteID, permNotesManage)
	if err != nil {
		return Note{}, err
	}
	if expectedRevision <= 0 {
		return Note{}, invalid("expectedRevision", "must_be_positive")
	}
	if err := s.requireCapability(ctx, scope, CapabilityCore, false); err != nil {
		return Note{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterNote); err != nil {
		return Note{}, err
	}
	return s.repo.ArchiveNote(ctx, scope, noteID, expectedRevision)
}

func (s *Service) ListConsents(ctx context.Context, principal auth.Principal, relationshipID string, limit int) ([]Consent, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceRelationship, relationshipID, permConsentsView)
	if err != nil {
		return nil, err
	}
	return s.repo.ListConsents(ctx, scope, relationshipID, boundedLimit(limit, 50, 100))
}

func (s *Service) RecordConsent(
	ctx context.Context,
	principal auth.Principal,
	relationshipID string,
	input ConsentInput,
) (Consent, bool, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceRelationship, relationshipID, permConsentsManage)
	if err != nil {
		return Consent{}, false, err
	}
	if err := validateConsent(input); err != nil {
		return Consent{}, false, err
	}
	if err := s.requireCapability(ctx, scope, CapabilityCore, false); err != nil {
		return Consent{}, false, err
	}
	if err := s.requireWriter(ctx, scope, WriterConsent); err != nil {
		return Consent{}, false, err
	}
	return s.repo.RecordConsent(ctx, scope, relationshipID, input)
}

func validateConsent(input ConsentInput) error {
	if strings.TrimSpace(input.Purpose) == "" || len(input.Purpose) > 120 {
		return invalid("purpose", "invalid_length")
	}
	if strings.TrimSpace(input.Channel) == "" || len(input.Channel) > 80 {
		return invalid("channel", "invalid_length")
	}
	switch input.Status {
	case "granted", "revoked", "unknown":
	default:
		return invalid("status", "unsupported")
	}
	if strings.TrimSpace(input.SourceModule) == "" || len(input.SourceModule) > 120 {
		return invalid("sourceModule", "invalid_length")
	}
	if input.EffectiveAt.IsZero() {
		return invalid("effectiveAt", "required")
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(input.EffectiveAt) {
		return invalid("expiresAt", "must_be_after_effective")
	}
	if len(strings.TrimSpace(input.IdempotencyKey)) < 8 {
		return invalid("idempotencyKey", "invalid_length")
	}
	return nil
}
