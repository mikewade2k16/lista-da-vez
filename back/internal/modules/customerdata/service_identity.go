package customerdata

import (
	"context"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func (s *Service) ListIdentities(
	ctx context.Context,
	principal auth.Principal,
	relationshipID string,
) ([]IdentityView, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceRelationship, relationshipID, permIdentitiesView)
	if err != nil {
		return nil, err
	}
	return s.repo.ListIdentities(ctx, scope, relationshipID)
}

func (s *Service) AddIdentity(
	ctx context.Context,
	principal auth.Principal,
	relationshipID string,
	input IdentityInput,
) (IdentityView, bool, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceRelationship, relationshipID, permIdentitiesManage)
	if err != nil {
		return IdentityView{}, false, err
	}
	if err := s.requireCapability(ctx, scope, CapabilityIdentity, false); err != nil {
		return IdentityView{}, false, err
	}
	if err := s.requireWriter(ctx, scope, WriterIdentity); err != nil {
		return IdentityView{}, false, err
	}
	if s.protector == nil {
		return IdentityView{}, false, ErrIdentityProtectionUnavailable
	}
	if len(strings.TrimSpace(input.IdempotencyKey)) < 8 {
		return IdentityView{}, false, invalid("idempotencyKey", "invalid_length")
	}
	protected, err := s.protector.Protect(scope, input)
	if err != nil {
		return IdentityView{}, false, err
	}
	return s.repo.AddIdentity(ctx, scope, relationshipID, protected)
}

func (s *Service) SetIdentityState(
	ctx context.Context,
	principal auth.Principal,
	identityID, state string,
	input IdentityStateInput,
) (IdentityView, bool, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceIdentity, identityID, permIdentitiesManage)
	if err != nil {
		return IdentityView{}, false, err
	}
	if state != "verified" && state != "revoked" {
		return IdentityView{}, false, invalid("state", "unsupported")
	}
	if input.ExpectedRevision <= 0 || len(strings.TrimSpace(input.IdempotencyKey)) < 8 {
		return IdentityView{}, false, invalid("revisionOrIdempotency", "invalid")
	}
	if state == "verified" && strings.TrimSpace(input.VerificationMethod) == "" {
		return IdentityView{}, false, invalid("verificationMethod", "required")
	}
	if err := s.requireCapability(ctx, scope, CapabilityIdentity, false); err != nil {
		return IdentityView{}, false, err
	}
	if err := s.requireWriter(ctx, scope, WriterIdentity); err != nil {
		return IdentityView{}, false, err
	}
	return s.repo.SetIdentityState(ctx, scope, identityID, state, input)
}

// ResolveSubject é a façade owner-scoped para adapters de canal/ERP/Site.
// Match cross-client nunca devolve IDs do outro client.
func (s *Service) ResolveSubject(ctx context.Context, request ResolveSubjectRequest) (ResolveSubjectResult, error) {
	if s == nil || s.repo == nil {
		return ResolveSubjectResult{}, ErrNotFound
	}
	scope, err := s.repo.ResolveServiceScope(ctx, strings.TrimSpace(request.AccountID), strings.TrimSpace(request.ClientAccountID))
	if err != nil {
		return ResolveSubjectResult{}, concealScopeError(err)
	}
	if err := s.requireCapability(ctx, scope, CapabilityIdentity, false); err != nil {
		return ResolveSubjectResult{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterIdentity); err != nil {
		return ResolveSubjectResult{}, err
	}
	if request.AllowCreate {
		if err := s.requireWriter(ctx, scope, WriterRelationship); err != nil {
			return ResolveSubjectResult{}, err
		}
	}
	if len(strings.TrimSpace(request.RequestID)) < 8 {
		return ResolveSubjectResult{}, invalid("requestId", "invalid_length")
	}
	if strings.TrimSpace(request.Source.SourceModule) == "" ||
		strings.TrimSpace(request.Source.SourceKey) == "" ||
		strings.TrimSpace(request.Source.SourceEntityType) == "" ||
		strings.TrimSpace(request.Source.SourceEntityID) == "" {
		return ResolveSubjectResult{}, invalid("source", "incomplete")
	}
	if len(request.Identities) > 20 {
		return ResolveSubjectResult{}, invalid("identities", "invalid_count")
	}
	if len(request.Identities) > 0 && s.protector == nil {
		return ResolveSubjectResult{}, ErrIdentityProtectionUnavailable
	}
	protected := make([]ProtectedIdentity, 0, len(request.Identities))
	for i, identity := range request.Identities {
		identity.IdempotencyKey = request.RequestID + ":identity:" + string(rune('a'+i))
		item, err := s.protector.Protect(scope, identity)
		if err != nil {
			return ResolveSubjectResult{}, err
		}
		protected = append(protected, item)
	}
	return s.repo.ResolveSubject(ctx, scope, request, protected)
}

// ResolveRelationship é o nome explícito da façade usada por adapters que
// precisam obter/criar o relacionamento a partir de source link + identidades.
func (s *Service) ResolveRelationship(
	ctx context.Context,
	request ResolveRelationshipRequest,
) (ResolveRelationshipResult, error) {
	return s.ResolveSubject(ctx, request)
}
