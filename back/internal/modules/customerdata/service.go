package customerdata

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

const (
	permSubjectsView        = "customer_data.subjects.view"
	permSubjectsManage      = "customer_data.subjects.manage"
	permRelationshipsView   = "customer_data.relationships.view"
	permRelationshipsManage = "customer_data.relationships.manage"
	permIdentitiesView      = "customer_data.identities.view"
	permIdentitiesManage    = "customer_data.identities.manage"
	permNotesView           = "customer_data.notes.view"
	permNotesManage         = "customer_data.notes.manage"
	permConsentsView        = "customer_data.consents.view"
	permConsentsManage      = "customer_data.consents.manage"
	permOfflineView         = "customer_data.offline_interactions.view"
	permOfflineManage       = "customer_data.offline_interactions.manage"
	permMergeManage         = "customer_data.merge.manage"
	permSegmentsView        = "customer_data.segments.view"
	permSegmentsManage      = "customer_data.segments.manage"
	permSegmentsEvaluate    = "customer_data.segments.evaluate"
	permSegmentsPublish     = "customer_data.segments.publish"
)

type Service struct {
	repo        Repository
	permissions PermissionChecker
	protector   IdentityProtector
	now         func() time.Time
}

func NewService(repo Repository, permissions PermissionChecker, protector IdentityProtector) *Service {
	return &Service{
		repo: repo, permissions: permissions, protector: protector,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) authorizedScope(
	ctx context.Context,
	principal auth.Principal,
	requestedClientID string,
	permissions ...string,
) (Scope, error) {
	accountID := strings.TrimSpace(principal.AccountID)
	if accountID == "" || strings.TrimSpace(principal.UserID) == "" {
		return Scope{}, ErrForbidden
	}
	if s == nil || s.repo == nil {
		return Scope{}, ErrForbidden
	}
	scope, err := s.repo.ResolveClientScope(
		ctx, accountID, strings.TrimSpace(requestedClientID), principal.UserID,
		principal.Role == auth.RolePlatformAdmin,
	)
	if err != nil {
		return Scope{}, concealScopeError(err)
	}
	scope.ActorUserID = principal.UserID
	for _, permission := range permissions {
		if err := s.authorize(ctx, principal, accountID, permission); err != nil {
			return Scope{}, err
		}
	}
	return scope, nil
}

func (s *Service) resourceScope(
	ctx context.Context,
	principal auth.Principal,
	resourceKind, resourceID string,
	permissions ...string,
) (Scope, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(principal.AccountID) == "" {
		return Scope{}, ErrNotFound
	}
	clientID, err := s.repo.FindResourceClient(ctx, principal.AccountID, resourceKind, strings.TrimSpace(resourceID))
	if err != nil {
		return Scope{}, concealScopeError(err)
	}
	return s.authorizedScope(ctx, principal, clientID, permissions...)
}

func (s *Service) authorize(ctx context.Context, principal auth.Principal, accountID, permission string) error {
	if s.permissions == nil {
		return ErrForbidden
	}
	ok, err := s.permissions.HasAccountPermission(ctx, accountID, principal.UserID, permission)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (s *Service) hasPermission(ctx context.Context, principal auth.Principal, permission string) bool {
	if s.permissions == nil || principal.AccountID == "" || principal.UserID == "" {
		return false
	}
	ok, err := s.permissions.HasAccountPermission(ctx, principal.AccountID, principal.UserID, permission)
	return err == nil && ok
}

func (s *Service) requireCapability(ctx context.Context, scope Scope, capability string, allowShadow bool) error {
	mode, err := s.repo.CapabilityMode(ctx, scope, capability)
	if err != nil {
		return err
	}
	if mode == CapabilityOn || (allowShadow && mode == CapabilityShadow) {
		return nil
	}
	return ErrCapabilityDisabled
}

func (s *Service) requireWriter(ctx context.Context, scope Scope, entity string) error {
	mode, err := s.repo.WriterMode(ctx, scope, entity)
	if err != nil {
		return err
	}
	if mode != WriterNew {
		return ErrWriterInactive
	}
	return nil
}

func concealScopeError(err error) error {
	if errors.Is(err, ErrValidation) {
		return err
	}
	return ErrNotFound
}

func (s *Service) ListSubjects(ctx context.Context, principal auth.Principal, filter SubjectFilter) (SubjectPage, error) {
	scope, err := s.authorizedScope(ctx, principal, filter.ClientAccountID, permSubjectsView, permRelationshipsView)
	if err != nil {
		return SubjectPage{}, err
	}
	filter.ClientAccountID = scope.ClientAccountID
	filter.Limit = boundedLimit(filter.Limit, 50, 100)
	if len(strings.TrimSpace(filter.Query)) > 160 {
		return SubjectPage{}, invalid("q", "too_long")
	}
	return s.repo.ListSubjects(ctx, scope, filter, s.hasPermission(ctx, principal, permIdentitiesView))
}

func (s *Service) CreateSubject(ctx context.Context, principal auth.Principal, input CreateSubjectInput) (CreateSubjectResult, error) {
	required := []string{permSubjectsManage, permRelationshipsManage}
	if len(input.Identities) > 0 {
		required = append(required, permIdentitiesManage)
	}
	scope, err := s.authorizedScope(ctx, principal, input.ClientAccountID, required...)
	if err != nil {
		return CreateSubjectResult{}, err
	}
	if err := s.requireCapability(ctx, scope, CapabilityCore, false); err != nil {
		return CreateSubjectResult{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterRelationship); err != nil {
		return CreateSubjectResult{}, err
	}
	if err := validateCreateSubject(input); err != nil {
		return CreateSubjectResult{}, err
	}
	protected := make([]ProtectedIdentity, 0, len(input.Identities))
	if len(input.Identities) > 0 {
		if err := s.requireCapability(ctx, scope, CapabilityIdentity, false); err != nil {
			return CreateSubjectResult{}, err
		}
		if err := s.requireWriter(ctx, scope, WriterIdentity); err != nil {
			return CreateSubjectResult{}, err
		}
		if s.protector == nil {
			return CreateSubjectResult{}, ErrIdentityProtectionUnavailable
		}
		for i, identity := range input.Identities {
			if identity.IdempotencyKey == "" {
				identity.IdempotencyKey = input.IdempotencyKey + ":identity:" + string(rune('a'+i))
			}
			item, err := s.protector.Protect(scope, identity)
			if err != nil {
				return CreateSubjectResult{}, err
			}
			protected = append(protected, item)
		}
	}
	input.ClientAccountID = scope.ClientAccountID
	return s.repo.CreateSubject(ctx, scope, input, protected)
}

func (s *Service) GetProfile(ctx context.Context, principal auth.Principal, relationshipID string) (DeterministicProfile, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceRelationship, relationshipID, permSubjectsView, permRelationshipsView)
	if err != nil {
		return DeterministicProfile{}, err
	}
	sections := ProfileSections{
		Identities:   s.hasPermission(ctx, principal, permIdentitiesView),
		Notes:        s.hasPermission(ctx, principal, permNotesView),
		Interactions: s.hasPermission(ctx, principal, permOfflineView),
		Consents:     s.hasPermission(ctx, principal, permConsentsView),
	}
	var reveal func(string, string) (string, error)
	if sections.Interactions && s.protector != nil {
		reveal = func(ciphertext, keyVersion string) (string, error) {
			return s.protector.RevealContent(scope, ciphertext, keyVersion)
		}
	}
	_ = reveal // repository profile never reveals restricted content in this first surface.
	return s.repo.GetProfile(ctx, scope, relationshipID, sections)
}

func (s *Service) UpdateSubject(
	ctx context.Context,
	principal auth.Principal,
	subjectID, clientAccountID string,
	patch SubjectPatch,
) (Subject, error) {
	scope, err := s.authorizedScope(ctx, principal, clientAccountID, permSubjectsManage)
	if err != nil {
		return Subject{}, err
	}
	if patch.ExpectedRevision <= 0 {
		return Subject{}, invalid("expectedRevision", "must_be_positive")
	}
	if err := s.requireCapability(ctx, scope, CapabilityCore, false); err != nil {
		return Subject{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterRelationship); err != nil {
		return Subject{}, err
	}
	return s.repo.UpdateSubject(ctx, scope, strings.TrimSpace(subjectID), patch)
}

func (s *Service) UpdateRelationship(
	ctx context.Context,
	principal auth.Principal,
	relationshipID string,
	patch RelationshipPatch,
) (Relationship, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceRelationship, relationshipID, permRelationshipsManage)
	if err != nil {
		return Relationship{}, err
	}
	if patch.ExpectedRevision <= 0 {
		return Relationship{}, invalid("expectedRevision", "must_be_positive")
	}
	if err := validateRelationshipPatch(patch); err != nil {
		return Relationship{}, err
	}
	if err := s.requireCapability(ctx, scope, CapabilityCore, false); err != nil {
		return Relationship{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterRelationship); err != nil {
		return Relationship{}, err
	}
	return s.repo.UpdateRelationship(ctx, scope, relationshipID, patch)
}

// GetDeterministicProfile é a façade owner-scoped para adapters confiáveis.
// Ela não expõe notas nem conteúdo offline; somente dados determinísticos mínimos.
func (s *Service) GetDeterministicProfile(ctx context.Context, request DeterministicProfileRequest) (DeterministicProfile, error) {
	if s == nil || s.repo == nil {
		return DeterministicProfile{}, ErrNotFound
	}
	scope, err := s.repo.ResolveServiceScope(ctx, strings.TrimSpace(request.AccountID), strings.TrimSpace(request.ClientAccountID))
	if err != nil {
		return DeterministicProfile{}, concealScopeError(err)
	}
	return s.repo.GetProfile(ctx, scope, strings.TrimSpace(request.RelationshipID), ProfileSections{
		Identities: true,
		Consents:   true,
	})
}

func validateCreateSubject(input CreateSubjectInput) error {
	if input.SubjectType != "person" && input.SubjectType != "organization" {
		return invalid("subjectType", "unsupported")
	}
	if err := validateRelationshipInput(input.Relationship); err != nil {
		return err
	}
	if len(strings.TrimSpace(input.IdempotencyKey)) < 8 || len(input.IdempotencyKey) > 200 {
		return invalid("idempotencyKey", "invalid_length")
	}
	if len(input.Identities) > 20 {
		return invalid("identities", "too_many")
	}
	return nil
}

func validateRelationshipInput(input RelationshipInput) error {
	name := strings.TrimSpace(input.DisplayName)
	if name == "" || len(name) > 200 {
		return invalid("relationship.displayName", "invalid_length")
	}
	if input.LifecycleStatus == "" {
		input.LifecycleStatus = "lead"
	}
	switch input.LifecycleStatus {
	case "lead", "prospect", "customer", "inactive":
	default:
		return invalid("relationship.lifecycleStatus", "unsupported")
	}
	if err := validateTags(input.Tags); err != nil {
		return err
	}
	if len(input.CustomFields) > 0 && !jsonObject(input.CustomFields) {
		return invalid("relationship.customFields", "must_be_object")
	}
	return nil
}

func validateRelationshipPatch(patch RelationshipPatch) error {
	if patch.DisplayName != nil {
		name := strings.TrimSpace(*patch.DisplayName)
		if name == "" || len(name) > 200 {
			return invalid("displayName", "invalid_length")
		}
	}
	if patch.LifecycleStatus != nil {
		switch *patch.LifecycleStatus {
		case "lead", "prospect", "customer", "inactive":
		default:
			return invalid("lifecycleStatus", "unsupported")
		}
	}
	if patch.Tags != nil {
		if err := validateTags(*patch.Tags); err != nil {
			return err
		}
	}
	if patch.CustomFields != nil && !jsonObject(*patch.CustomFields) {
		return invalid("customFields", "must_be_object")
	}
	return nil
}

func validateTags(tags []string) error {
	if len(tags) > 50 {
		return invalid("tags", "too_many")
	}
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized == "" || len(normalized) > 64 {
			return invalid("tags", "invalid_item")
		}
		if _, ok := seen[normalized]; ok {
			return invalid("tags", "duplicate")
		}
		seen[normalized] = struct{}{}
	}
	return nil
}

func jsonObject(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var value map[string]json.RawMessage
	return json.Unmarshal(raw, &value) == nil && value != nil
}

func boundedLimit(value, defaultValue, maxValue int) int {
	if value <= 0 {
		return defaultValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
