package customerdata

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func (s *Service) SegmentFields(
	ctx context.Context,
	principal auth.Principal,
	clientAccountID string,
) (SegmentFieldCatalog, error) {
	scope, err := s.authorizedScope(ctx, principal, clientAccountID, permSegmentsView)
	if err != nil {
		return SegmentFieldCatalog{}, err
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, true); err != nil {
		return SegmentFieldCatalog{}, err
	}
	return CurrentSegmentFieldCatalog(), nil
}

func (s *Service) ListSegments(
	ctx context.Context,
	principal auth.Principal,
	clientAccountID, status, cursor string,
	limit int,
) ([]Segment, string, error) {
	scope, err := s.authorizedScope(ctx, principal, clientAccountID, permSegmentsView)
	if err != nil {
		return nil, "", err
	}
	if status != "" && status != "active" && status != "archived" {
		return nil, "", invalid("status", "unsupported")
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, true); err != nil {
		return nil, "", err
	}
	return s.repo.ListSegments(ctx, scope, status, cursor, boundedLimit(limit, 50, 100))
}

func (s *Service) CreateSegment(
	ctx context.Context,
	principal auth.Principal,
	input CreateSegmentInput,
) (CreateSegmentResult, error) {
	scope, err := s.authorizedScope(ctx, principal, input.ClientAccountID, permSegmentsManage)
	if err != nil {
		return CreateSegmentResult{}, err
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, false); err != nil {
		return CreateSegmentResult{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterSegment); err != nil {
		return CreateSegmentResult{}, err
	}
	if err := validateSegmentMetadata(input.SegmentKey, input.Name, input.Description, input.IdempotencyKey); err != nil {
		return CreateSegmentResult{}, err
	}
	_, definitionHash, _, err := ValidateSegmentDraft(input.Draft)
	if err != nil {
		return CreateSegmentResult{}, err
	}
	input.ClientAccountID = scope.ClientAccountID
	return s.repo.CreateSegment(ctx, scope, input, definitionHash)
}

func (s *Service) GetSegment(ctx context.Context, principal auth.Principal, segmentID string) (Segment, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceSegment, segmentID, permSegmentsView)
	if err != nil {
		return Segment{}, err
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, true); err != nil {
		return Segment{}, err
	}
	return s.repo.GetSegment(ctx, scope, segmentID)
}

func (s *Service) UpdateSegment(
	ctx context.Context,
	principal auth.Principal,
	segmentID string,
	patch SegmentPatch,
) (Segment, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceSegment, segmentID, permSegmentsManage)
	if err != nil {
		return Segment{}, err
	}
	if patch.ExpectedRevision <= 0 {
		return Segment{}, invalid("expectedRevision", "must_be_positive")
	}
	if patch.Name != nil && (strings.TrimSpace(*patch.Name) == "" || len(*patch.Name) > 160) {
		return Segment{}, invalid("name", "invalid_length")
	}
	if patch.Description != nil && len(*patch.Description) > 2000 {
		return Segment{}, invalid("description", "too_long")
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, false); err != nil {
		return Segment{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterSegment); err != nil {
		return Segment{}, err
	}
	return s.repo.UpdateSegment(ctx, scope, segmentID, patch)
}

func (s *Service) ArchiveSegment(
	ctx context.Context,
	principal auth.Principal,
	segmentID string,
	expectedRevision int64,
) (Segment, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceSegment, segmentID, permSegmentsManage)
	if err != nil {
		return Segment{}, err
	}
	if expectedRevision <= 0 {
		return Segment{}, invalid("expectedRevision", "must_be_positive")
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, false); err != nil {
		return Segment{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterSegment); err != nil {
		return Segment{}, err
	}
	return s.repo.ArchiveSegment(ctx, scope, segmentID, expectedRevision)
}

func (s *Service) ListSegmentVersions(
	ctx context.Context,
	principal auth.Principal,
	segmentID string,
) ([]SegmentVersion, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceSegment, segmentID, permSegmentsView)
	if err != nil {
		return nil, err
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, true); err != nil {
		return nil, err
	}
	return s.repo.ListSegmentVersions(ctx, scope, segmentID)
}

func (s *Service) CreateSegmentVersion(
	ctx context.Context,
	principal auth.Principal,
	segmentID string,
	input CreateSegmentVersionInput,
) (SegmentVersion, bool, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceSegment, segmentID, permSegmentsManage)
	if err != nil {
		return SegmentVersion{}, false, err
	}
	if len(strings.TrimSpace(input.IdempotencyKey)) < 8 {
		return SegmentVersion{}, false, invalid("idempotencyKey", "invalid_length")
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, false); err != nil {
		return SegmentVersion{}, false, err
	}
	if err := s.requireWriter(ctx, scope, WriterSegment); err != nil {
		return SegmentVersion{}, false, err
	}
	draft, err := s.resolveNewDraft(ctx, scope, input)
	if err != nil {
		return SegmentVersion{}, false, err
	}
	_, definitionHash, _, err := ValidateSegmentDraft(draft)
	if err != nil {
		return SegmentVersion{}, false, err
	}
	return s.repo.CreateSegmentVersion(ctx, scope, segmentID, input, draft, definitionHash)
}

func (s *Service) UpdateSegmentVersion(
	ctx context.Context,
	principal auth.Principal,
	versionID string,
	patch SegmentVersionPatch,
) (SegmentVersion, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceSegmentVersion, versionID, permSegmentsManage)
	if err != nil {
		return SegmentVersion{}, err
	}
	if patch.ExpectedRevision <= 0 {
		return SegmentVersion{}, invalid("expectedRevision", "must_be_positive")
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, false); err != nil {
		return SegmentVersion{}, err
	}
	if err := s.requireWriter(ctx, scope, WriterSegment); err != nil {
		return SegmentVersion{}, err
	}
	version, err := s.repo.GetSegmentVersion(ctx, scope, versionID)
	if err != nil {
		return SegmentVersion{}, err
	}
	if version.Status != "draft" && version.Status != "validated" {
		return SegmentVersion{}, ErrConflict
	}
	draft := SegmentDraftInput{
		FilterSchemaVersion: version.FilterSchemaVersion,
		FieldCatalogVersion: version.FieldCatalogVersion,
		FilterAST:           patch.FilterAST,
		EvaluationPolicy:    patch.EvaluationPolicy,
	}
	_, definitionHash, _, err := ValidateSegmentDraft(draft)
	if err != nil {
		return SegmentVersion{}, err
	}
	return s.repo.UpdateSegmentVersion(ctx, scope, versionID, patch, definitionHash)
}

func (s *Service) ValidateSegmentVersion(
	ctx context.Context,
	principal auth.Principal,
	versionID string,
) (SegmentValidationResult, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceSegmentVersion, versionID, permSegmentsManage)
	if err != nil {
		return SegmentValidationResult{}, err
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, true); err != nil {
		return SegmentValidationResult{}, err
	}
	version, err := s.repo.GetSegmentVersion(ctx, scope, versionID)
	if err != nil {
		return SegmentValidationResult{}, err
	}
	_, hash, cost, err := ValidateSegmentDraft(SegmentDraftInput{
		FilterSchemaVersion: version.FilterSchemaVersion,
		FieldCatalogVersion: version.FieldCatalogVersion,
		FilterAST:           version.FilterAST,
		EvaluationPolicy:    version.EvaluationPolicy,
	})
	if err != nil {
		return SegmentValidationResult{}, err
	}
	updated, err := s.repo.ValidateSegmentVersion(ctx, scope, versionID, hash, cost)
	if err != nil {
		return SegmentValidationResult{}, err
	}
	return SegmentValidationResult{
		VersionID: updated.ID, Status: updated.Status, ValidationHash: hash,
		ReasonCodes: updated.ValidationReasonCodes, EstimatedCost: cost, Revision: updated.Revision,
	}, nil
}

func (s *Service) PublishSegmentVersion(
	ctx context.Context,
	principal auth.Principal,
	versionID string,
	input PublishSegmentVersionInput,
) (SegmentVersion, bool, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceSegmentVersion, versionID, permSegmentsManage, permSegmentsPublish)
	if err != nil {
		return SegmentVersion{}, false, err
	}
	if input.ExpectedRevision <= 0 || len(strings.TrimSpace(input.ValidationHash)) != 64 ||
		strings.TrimSpace(input.Reason) == "" || len(strings.TrimSpace(input.IdempotencyKey)) < 8 {
		return SegmentVersion{}, false, invalid("publish", "invalid_precondition")
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, false); err != nil {
		return SegmentVersion{}, false, err
	}
	if err := s.requireWriter(ctx, scope, WriterSegment); err != nil {
		return SegmentVersion{}, false, err
	}
	return s.repo.PublishSegmentVersion(ctx, scope, versionID, input)
}

func (s *Service) RollbackSegment(
	ctx context.Context,
	principal auth.Principal,
	segmentID string,
	input RollbackSegmentInput,
) (Segment, bool, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceSegment, segmentID, permSegmentsPublish)
	if err != nil {
		return Segment{}, false, err
	}
	if input.ExpectedSegmentRevision <= 0 || strings.TrimSpace(input.TargetVersionID) == "" ||
		strings.TrimSpace(input.Reason) == "" || len(strings.TrimSpace(input.IdempotencyKey)) < 8 {
		return Segment{}, false, invalid("rollback", "invalid_precondition")
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, false); err != nil {
		return Segment{}, false, err
	}
	if err := s.requireWriter(ctx, scope, WriterSegment); err != nil {
		return Segment{}, false, err
	}
	return s.repo.RollbackSegment(ctx, scope, segmentID, input)
}

func (s *Service) RequestSegmentEvaluation(
	ctx context.Context,
	principal auth.Principal,
	segmentID string,
	request SegmentEvaluationRequest,
) (SegmentEvaluationRun, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceSegment, segmentID, permSegmentsView, permSegmentsEvaluate)
	if err != nil {
		return SegmentEvaluationRun{}, err
	}
	allowShadow := request.Mode == "preview"
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, allowShadow); err != nil {
		return SegmentEvaluationRun{}, err
	}
	if request.Mode != "preview" && request.Mode != "materialize" && request.Mode != "recompute" {
		return SegmentEvaluationRun{}, invalid("mode", "unsupported")
	}
	if len(strings.TrimSpace(request.IdempotencyKey)) < 8 {
		return SegmentEvaluationRun{}, invalid("idempotencyKey", "invalid_length")
	}
	segment, err := s.repo.GetSegment(ctx, scope, segmentID)
	if err != nil {
		return SegmentEvaluationRun{}, err
	}
	versionID := strings.TrimSpace(request.VersionID)
	if versionID == "" && segment.ActiveVersionID != nil {
		versionID = *segment.ActiveVersionID
	}
	if versionID == "" {
		return SegmentEvaluationRun{}, invalid("versionId", "required")
	}
	version, err := s.repo.GetSegmentVersion(ctx, scope, versionID)
	if err != nil {
		return SegmentEvaluationRun{}, err
	}
	if version.SegmentID != segmentID {
		return SegmentEvaluationRun{}, ErrNotFound
	}
	if request.Mode != "preview" && version.Status != "published" {
		return SegmentEvaluationRun{}, ErrConflict
	}
	if request.Mode == "preview" && version.Status != "draft" && version.Status != "validated" && version.Status != "published" {
		return SegmentEvaluationRun{}, ErrConflict
	}
	filter, hash, _, err := ValidateSegmentDraft(SegmentDraftInput{
		FilterSchemaVersion: version.FilterSchemaVersion,
		FieldCatalogVersion: version.FieldCatalogVersion,
		FilterAST:           version.FilterAST,
		EvaluationPolicy:    version.EvaluationPolicy,
	})
	if err != nil {
		return SegmentEvaluationRun{}, err
	}
	asOf := s.now()
	if request.AsOf != nil {
		asOf = request.AsOf.UTC()
		if asOf.After(s.now().Add(5*time.Minute)) || asOf.Before(s.now().AddDate(-2, 0, 0)) {
			return SegmentEvaluationRun{}, invalid("asOf", "out_of_range")
		}
	}
	if _, err := CompileSegmentFilter(scope, filter, asOf); err != nil {
		return SegmentEvaluationRun{}, err
	}
	if hash != version.DefinitionHash {
		return SegmentEvaluationRun{}, ErrConflict
	}
	request.VersionID = versionID
	return s.repo.CreateEvaluationRun(ctx, scope, segmentID, request, version, asOf)
}

func (s *Service) RequestSegmentPreview(
	ctx context.Context,
	principal auth.Principal,
	versionID string,
	request SegmentEvaluationRequest,
) (SegmentEvaluationRun, error) {
	scope, err := s.resourceScope(
		ctx, principal, ResourceSegmentVersion, versionID,
		permSegmentsView, permSegmentsEvaluate,
	)
	if err != nil {
		return SegmentEvaluationRun{}, err
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, true); err != nil {
		return SegmentEvaluationRun{}, err
	}
	if len(strings.TrimSpace(request.IdempotencyKey)) < 8 {
		return SegmentEvaluationRun{}, invalid("idempotencyKey", "invalid_length")
	}
	version, err := s.repo.GetSegmentVersion(ctx, scope, versionID)
	if err != nil {
		return SegmentEvaluationRun{}, err
	}
	if version.Status != "draft" && version.Status != "validated" && version.Status != "published" {
		return SegmentEvaluationRun{}, ErrConflict
	}
	filter, definitionHash, _, err := ValidateSegmentDraft(SegmentDraftInput{
		FilterSchemaVersion: version.FilterSchemaVersion,
		FieldCatalogVersion: version.FieldCatalogVersion,
		FilterAST:           version.FilterAST,
		EvaluationPolicy:    version.EvaluationPolicy,
	})
	if err != nil {
		return SegmentEvaluationRun{}, err
	}
	if definitionHash != version.DefinitionHash {
		return SegmentEvaluationRun{}, ErrConflict
	}
	asOf := s.now()
	if request.AsOf != nil {
		asOf = request.AsOf.UTC()
	}
	if _, err := CompileSegmentFilter(scope, filter, asOf); err != nil {
		return SegmentEvaluationRun{}, err
	}
	request.VersionID = versionID
	request.Mode = "preview"
	return s.repo.CreateEvaluationRun(ctx, scope, version.SegmentID, request, version, asOf)
}

func (s *Service) GetEvaluationRun(
	ctx context.Context,
	principal auth.Principal,
	runID string,
) (SegmentEvaluationRun, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceEvaluationRun, runID, permSegmentsView)
	if err != nil {
		return SegmentEvaluationRun{}, err
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, true); err != nil {
		return SegmentEvaluationRun{}, err
	}
	return s.repo.GetEvaluationRun(ctx, scope, runID)
}

func (s *Service) ListMaterializations(
	ctx context.Context,
	principal auth.Principal,
	segmentID string,
	limit int,
) ([]SegmentMaterialization, error) {
	scope, err := s.resourceScope(ctx, principal, ResourceSegment, segmentID, permSegmentsView)
	if err != nil {
		return nil, err
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, false); err != nil {
		return nil, err
	}
	return s.repo.ListMaterializations(ctx, scope, segmentID, boundedLimit(limit, 50, 100))
}

func (s *Service) ListMaterializationMembers(
	ctx context.Context,
	principal auth.Principal,
	materializationID, cursor string,
	limit int,
) ([]SegmentMember, string, error) {
	scope, err := s.resourceScope(
		ctx, principal, ResourceMaterialization, materializationID,
		permSegmentsView, permSubjectsView, permRelationshipsView,
	)
	if err != nil {
		return nil, "", err
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, false); err != nil {
		return nil, "", err
	}
	return s.repo.ListMaterializationMembers(ctx, scope, materializationID, cursor, boundedLimit(limit, 50, 100))
}

// GetSegmentContext é a façade owner-scoped para adapters confiáveis.
func (s *Service) GetSegmentContext(ctx context.Context, request SegmentContextRequest) (SegmentContext, error) {
	if s == nil || s.repo == nil {
		return SegmentContext{}, ErrNotFound
	}
	scope, err := s.repo.ResolveServiceScope(ctx, strings.TrimSpace(request.AccountID), strings.TrimSpace(request.ClientAccountID))
	if err != nil {
		return SegmentContext{}, concealScopeError(err)
	}
	if err := s.requireCapability(ctx, scope, CapabilitySegmentation, false); err != nil {
		return SegmentContext{}, err
	}
	asOf := request.AsOf.UTC()
	if asOf.IsZero() {
		asOf = s.now()
	}
	return s.repo.GetSegmentContext(ctx, scope, strings.TrimSpace(request.RelationshipID), asOf)
}

func (s *Service) resolveNewDraft(
	ctx context.Context,
	scope Scope,
	input CreateSegmentVersionInput,
) (SegmentDraftInput, error) {
	if input.Draft != nil {
		return *input.Draft, nil
	}
	if input.BaseVersionID == nil || strings.TrimSpace(*input.BaseVersionID) == "" {
		return SegmentDraftInput{}, invalid("draft", "required")
	}
	base, err := s.repo.GetSegmentVersion(ctx, scope, *input.BaseVersionID)
	if err != nil {
		return SegmentDraftInput{}, err
	}
	if base.Status != "published" {
		return SegmentDraftInput{}, ErrConflict
	}
	return SegmentDraftInput{
		FilterSchemaVersion: base.FilterSchemaVersion,
		FieldCatalogVersion: base.FieldCatalogVersion,
		FilterAST:           append(json.RawMessage(nil), base.FilterAST...),
		EvaluationPolicy:    append(json.RawMessage(nil), base.EvaluationPolicy...),
	}, nil
}

func validateSegmentMetadata(key, name string, description *string, idempotency string) error {
	key = strings.TrimSpace(key)
	if len(key) < 2 || len(key) > 80 {
		return invalid("segmentKey", "invalid_length")
	}
	for i, r := range key {
		isLowercaseLetter := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		isInnerSeparator := i > 0 && (r == '-' || r == '_')
		if !isLowercaseLetter && !isDigit && !isInnerSeparator {
			return invalid("segmentKey", "invalid_format")
		}
	}
	if strings.TrimSpace(name) == "" || len(name) > 160 {
		return invalid("name", "invalid_length")
	}
	if description != nil && len(*description) > 2000 {
		return invalid("description", "too_long")
	}
	if len(strings.TrimSpace(idempotency)) < 8 {
		return invalid("idempotencyKey", "invalid_length")
	}
	return nil
}
