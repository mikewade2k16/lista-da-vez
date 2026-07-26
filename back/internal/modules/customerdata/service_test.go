package customerdata

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type fakePermissionChecker struct {
	allow map[string]bool
}

func (f fakePermissionChecker) HasAccountPermission(
	_ context.Context,
	_, _, permission string,
) (bool, error) {
	return f.allow[permission], nil
}

type fakeProtector struct{}

func (fakeProtector) Protect(_ Scope, input IdentityInput) (ProtectedIdentity, error) {
	return ProtectedIdentity{
		Kind: input.Kind, Issuer: input.Issuer, Ciphertext: "cipher",
		Fingerprint: "fingerprint", KeyVersion: "test", MaskedValue: "***1234",
		VerificationStatus: input.VerificationStatus, OccurredAt: time.Now(),
		IdempotencyKey: input.IdempotencyKey,
	}, nil
}
func (fakeProtector) ProtectContent(_ Scope, plaintext string) (string, string, error) {
	return "cipher:" + plaintext, "test", nil
}
func (fakeProtector) RevealContent(_ Scope, ciphertext, _ string) (string, error) {
	return ciphertext, nil
}

type fakeRepository struct {
	Repository
	scope              Scope
	scopeErr           error
	resourceClient     string
	capabilities       map[string]CapabilityMode
	writers            map[string]WriterMode
	capabilityStates   map[string]CapabilityState
	writerStates       map[string]WriterState
	capabilityKeys     map[string]string
	writerKeys         map[string]string
	createSubjectCalls int
	consentCalls       int
	offlineCalls       int
	mergeCalls         int
	resolveCalls       int
	setCapabilityCalls int
	setWriterCalls     int
	updateVersionCalls int
	resolveResult      ResolveSubjectResult
	segmentVersion     SegmentVersion
	sourceProfile      DeterministicProfile
	sourceReferences   []SourceReference
	sourceInteractions []OfflineInteraction
}

func (f *fakeRepository) ResolveClientScope(
	_ context.Context,
	accountID, requestedClientID, userID string,
	_ bool,
) (Scope, error) {
	if f.scopeErr != nil {
		return Scope{}, f.scopeErr
	}
	scope := f.scope
	if scope.AccountID == "" {
		scope.AccountID = accountID
	}
	if scope.ClientAccountID == "" {
		scope.ClientAccountID = requestedClientID
	}
	scope.ActorUserID = userID
	return scope, nil
}

func (f *fakeRepository) ResolveServiceScope(
	_ context.Context,
	accountID, clientAccountID string,
) (Scope, error) {
	if f.scopeErr != nil {
		return Scope{}, f.scopeErr
	}
	return Scope{AccountID: accountID, ClientAccountID: clientAccountID}, nil
}

func (f *fakeRepository) FindResourceClient(_ context.Context, _, _, _ string) (string, error) {
	if f.scopeErr != nil {
		return "", f.scopeErr
	}
	return f.resourceClient, nil
}

func (f *fakeRepository) CapabilityMode(_ context.Context, _ Scope, capability string) (CapabilityMode, error) {
	return f.capabilities[capability], nil
}

func (f *fakeRepository) WriterMode(_ context.Context, _ Scope, entity string) (WriterMode, error) {
	return f.writers[entity], nil
}

func (f *fakeRepository) GetProfile(
	_ context.Context,
	_ Scope,
	_ string,
	_ ProfileSections,
) (DeterministicProfile, error) {
	if f.sourceProfile.Relationship.ID == "" {
		return DeterministicProfile{}, ErrNotFound
	}
	return f.sourceProfile, nil
}

func (f *fakeRepository) ListSourceReferences(
	_ context.Context,
	_ Scope,
	_ string,
) ([]SourceReference, error) {
	return append([]SourceReference(nil), f.sourceReferences...), nil
}

func (f *fakeRepository) ListOfflineInteractions(
	_ context.Context,
	_ Scope,
	_ string,
	_ int,
	_ func(string, string) (string, error),
) ([]OfflineInteraction, error) {
	return append([]OfflineInteraction(nil), f.sourceInteractions...), nil
}

func TestGetSourceEvidenceIsScopedAndObeysOfflineCapability(t *testing.T) {
	repo := &fakeRepository{
		capabilities: map[string]CapabilityMode{
			CapabilityOffline: CapabilityShadow,
		},
		sourceProfile: DeterministicProfile{
			Subject: Subject{ID: "subject-1"},
			Relationship: Relationship{
				ID:              "relationship-1",
				ClientAccountID: "client-1",
			},
		},
		sourceReferences: []SourceReference{{
			SourceModule:     "omnichannel",
			SourceKey:        "whatsapp",
			SourceEntityType: "contact",
			SourceEntityID:   "contact-1",
		}},
		sourceInteractions: []OfflineInteraction{{
			ID:              "offline-1",
			RelationshipID:  "relationship-1",
			InteractionType: "meeting",
			Title:           "Reunião",
		}},
	}
	service := NewService(repo, fakePermissionChecker{}, fakeProtector{})

	bundle, err := service.GetSourceEvidence(context.Background(), SourceEvidenceRequest{
		AccountID:       "account-1",
		ClientAccountID: "client-1",
		RelationshipID:  "relationship-1",
		Limit:           20,
	})
	if err != nil {
		t.Fatalf("GetSourceEvidence: %v", err)
	}
	if bundle.SubjectID != "subject-1" ||
		len(bundle.SourceLinks) != 1 ||
		len(bundle.Interactions) != 1 {
		t.Fatalf("bundle inesperado: %#v", bundle)
	}

	repo.capabilities[CapabilityOffline] = CapabilityOff
	bundle, err = service.GetSourceEvidence(context.Background(), SourceEvidenceRequest{
		AccountID:       "account-1",
		ClientAccountID: "client-1",
		RelationshipID:  "relationship-1",
	})
	if err != nil {
		t.Fatalf("capability off deve degradar leitura: %v", err)
	}
	if len(bundle.SourceLinks) != 1 || len(bundle.Interactions) != 0 {
		t.Fatalf("capability off vazou interações: %#v", bundle)
	}
}

func (f *fakeRepository) ListCapabilityStates(_ context.Context, _ Scope) ([]CapabilityState, error) {
	out := make([]CapabilityState, 0, len(f.capabilityStates))
	for _, state := range f.capabilityStates {
		out = append(out, state)
	}
	return out, nil
}

func (f *fakeRepository) GetCapabilityState(
	_ context.Context,
	_ Scope,
	capability string,
) (CapabilityState, error) {
	if state, ok := f.capabilityStates[capability]; ok {
		return state, nil
	}
	return CapabilityState{CapabilityKey: capability, Mode: CapabilityOff}, nil
}

func (f *fakeRepository) SetCapabilityState(
	_ context.Context,
	_ Scope,
	capability string,
	input CapabilityStateInput,
) (CapabilityState, bool, error) {
	if f.capabilityKeys[capability] == input.IdempotencyKey {
		return f.capabilityStates[capability], true, nil
	}
	if current := f.capabilityStates[capability]; current.Revision != input.ExpectedRevision {
		return CapabilityState{}, false, ErrConflict
	}
	f.setCapabilityCalls++
	state := CapabilityState{
		CapabilityKey: capability,
		Mode:          input.Mode,
		Revision:      input.ExpectedRevision + 1,
	}
	f.capabilityStates[capability] = state
	f.capabilityKeys[capability] = input.IdempotencyKey
	f.capabilities[capability] = input.Mode
	return state, false, nil
}

func (f *fakeRepository) ListWriterStates(_ context.Context, _ Scope) ([]WriterState, error) {
	out := make([]WriterState, 0, len(f.writerStates))
	for _, state := range f.writerStates {
		out = append(out, state)
	}
	return out, nil
}

func (f *fakeRepository) GetWriterState(
	_ context.Context,
	_ Scope,
	entity string,
) (WriterState, error) {
	if state, ok := f.writerStates[entity]; ok {
		return state, nil
	}
	return WriterState{EntityKey: entity, Mode: WriterLegacy}, nil
}

func (f *fakeRepository) SetWriterState(
	_ context.Context,
	_ Scope,
	entity string,
	input WriterStateInput,
) (WriterState, bool, error) {
	if f.writerKeys[entity] == input.IdempotencyKey {
		return f.writerStates[entity], true, nil
	}
	if current := f.writerStates[entity]; current.Revision != input.ExpectedRevision {
		return WriterState{}, false, ErrConflict
	}
	f.setWriterCalls++
	state := WriterState{
		EntityKey:      entity,
		Mode:           input.Mode,
		Watermark:      input.Watermark,
		SourceChecksum: input.SourceChecksum,
		TargetChecksum: input.TargetChecksum,
		Revision:       input.ExpectedRevision + 1,
	}
	f.writerStates[entity] = state
	f.writerKeys[entity] = input.IdempotencyKey
	f.writers[entity] = input.Mode
	return state, false, nil
}

func (f *fakeRepository) CreateSubject(
	_ context.Context,
	_ Scope,
	_ CreateSubjectInput,
	_ []ProtectedIdentity,
) (CreateSubjectResult, error) {
	f.createSubjectCalls++
	return CreateSubjectResult{}, nil
}

func (f *fakeRepository) RecordConsent(
	_ context.Context,
	_ Scope,
	relationshipID string,
	input ConsentInput,
) (Consent, bool, error) {
	f.consentCalls++
	return Consent{ID: input.IdempotencyKey, RelationshipID: relationshipID}, f.consentCalls > 1, nil
}

func (f *fakeRepository) CreateOfflineInteraction(
	_ context.Context,
	_ Scope,
	input OfflineInteractionInput,
	ciphertext, keyVersion string,
) (OfflineInteraction, bool, error) {
	f.offlineCalls++
	if input.Sensitivity == "sensitive" && (ciphertext == "" || keyVersion == "") {
		return OfflineInteraction{}, false, errors.New("unprotected content")
	}
	return OfflineInteraction{ID: "offline-1", RelationshipID: input.RelationshipID}, false, nil
}

func (f *fakeRepository) MergeSubjects(
	_ context.Context,
	scope Scope,
	sourceSubjectID string,
	input MergeInput,
) (MergeEvent, error) {
	f.mergeCalls++
	return MergeEvent{
		ID: "merge-1", ClientAccountID: scope.ClientAccountID,
		SourceSubjectID: sourceSubjectID, TargetSubjectID: input.TargetSubjectID,
	}, nil
}

func (f *fakeRepository) ResolveSubject(
	_ context.Context,
	_ Scope,
	_ ResolveSubjectRequest,
	_ []ProtectedIdentity,
) (ResolveSubjectResult, error) {
	f.resolveCalls++
	return f.resolveResult, nil
}

func (f *fakeRepository) GetSegmentVersion(_ context.Context, _ Scope, _ string) (SegmentVersion, error) {
	return f.segmentVersion, nil
}

func (f *fakeRepository) UpdateSegmentVersion(
	_ context.Context,
	_ Scope,
	_ string,
	_ SegmentVersionPatch,
	_ string,
) (SegmentVersion, error) {
	f.updateVersionCalls++
	return f.segmentVersion, nil
}

func allPermissions() fakePermissionChecker {
	keys := map[string]bool{}
	for _, permission := range New().Permissions() {
		keys[permission.Key] = true
	}
	return fakePermissionChecker{allow: keys}
}

func principal() auth.Principal {
	return auth.Principal{
		UserID: "user-a", AccountID: "owner-a", Role: auth.RoleManager,
	}
}

func readyFakeRepo() *fakeRepository {
	return &fakeRepository{
		scope:          Scope{AccountID: "owner-a", ClientAccountID: "client-a"},
		resourceClient: "client-a",
		capabilities: map[string]CapabilityMode{
			CapabilityCore: CapabilityOn, CapabilityIdentity: CapabilityOn,
			CapabilityMatchingMerge: CapabilityOn, CapabilityOffline: CapabilityOn,
			CapabilitySegmentation: CapabilityOn,
		},
		writers: map[string]WriterMode{
			WriterRelationship: WriterNew, WriterIdentity: WriterNew,
			WriterNote: WriterNew, WriterConsent: WriterNew,
			WriterMerge: WriterNew, WriterSegment: WriterNew,
		},
		capabilityStates: map[string]CapabilityState{},
		writerStates:     map[string]WriterState{},
		capabilityKeys:   map[string]string{},
		writerKeys:       map[string]string{},
	}
}

func TestServiceFailsClosedWithoutPermissionChecker(t *testing.T) {
	t.Parallel()
	repo := readyFakeRepo()
	service := NewService(repo, nil, fakeProtector{})
	_, err := service.ListSubjects(context.Background(), principal(), SubjectFilter{ClientAccountID: "client-a"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestServiceConcealsClientScope(t *testing.T) {
	t.Parallel()
	repo := readyFakeRepo()
	repo.scopeErr = errors.New("client belongs to another organization")
	service := NewService(repo, allPermissions(), fakeProtector{})
	_, err := service.ListSubjects(context.Background(), principal(), SubjectFilter{ClientAccountID: "client-b"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected concealed not found, got %v", err)
	}
}

func TestCapabilitiesDefaultOffBlockWrites(t *testing.T) {
	t.Parallel()
	repo := readyFakeRepo()
	repo.capabilities[CapabilityCore] = CapabilityOff
	service := NewService(repo, allPermissions(), fakeProtector{})
	_, err := service.CreateSubject(context.Background(), principal(), CreateSubjectInput{
		ClientAccountID: "client-a", SubjectType: "person", IdempotencyKey: "request-123",
		Relationship: RelationshipInput{
			DisplayName: "Contato", LifecycleStatus: "lead", Tags: []string{},
			CustomFields: json.RawMessage(`{}`),
		},
	})
	if !errors.Is(err, ErrCapabilityDisabled) {
		t.Fatalf("expected capability disabled, got %v", err)
	}
	if repo.createSubjectCalls != 0 {
		t.Fatal("repository write must not run while capability is off")
	}
}

func TestConsentIsAppendOnlyAndRelationshipScoped(t *testing.T) {
	t.Parallel()
	repo := readyFakeRepo()
	service := NewService(repo, allPermissions(), fakeProtector{})
	input := ConsentInput{
		Purpose: "marketing", Channel: "whatsapp", Status: "granted",
		SourceModule: "manual", EffectiveAt: time.Now(), IdempotencyKey: "consent-123",
	}
	first, replayed, err := service.RecordConsent(context.Background(), principal(), "relationship-a", input)
	if err != nil || replayed {
		t.Fatalf("first append: replay=%v err=%v", replayed, err)
	}
	second, replayed, err := service.RecordConsent(context.Background(), principal(), "relationship-a", input)
	if err != nil || !replayed || first.ID != second.ID {
		t.Fatalf("idempotent append: first=%+v second=%+v replay=%v err=%v", first, second, replayed, err)
	}
	if repo.consentCalls != 2 {
		t.Fatalf("expected two repository invocations, got %d", repo.consentCalls)
	}
}

func TestSensitiveOfflineInteractionIsProtectedBeforeRepository(t *testing.T) {
	t.Parallel()
	repo := readyFakeRepo()
	service := NewService(repo, allPermissions(), fakeProtector{})
	_, _, err := service.CreateOfflineInteraction(context.Background(), principal(), "relationship-a", OfflineInteractionInput{
		InteractionType: "meeting", OccurredAt: time.Now().Add(-time.Hour),
		Timezone: "America/Sao_Paulo", Title: "Reunião", Content: "conteúdo sensível",
		Sensitivity: "sensitive", PurposeKey: "service", IdempotencyKey: "offline-123",
	})
	if err != nil {
		t.Fatalf("create offline: %v", err)
	}
	if repo.offlineCalls != 1 {
		t.Fatalf("expected one write, got %d", repo.offlineCalls)
	}
}

func TestMergeRequiresDistinctSubjectsAndKeepsClientScope(t *testing.T) {
	t.Parallel()
	repo := readyFakeRepo()
	service := NewService(repo, allPermissions(), fakeProtector{})
	input := MergeInput{
		TargetSubjectID: "subject-b", Reason: "revisão manual",
		ExpectedSourceRevision: 1, ExpectedTargetRevision: 1,
		IdempotencyKey: "merge-123",
	}
	event, err := service.MergeSubjects(context.Background(), principal(), "subject-a", "client-a", input)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if event.ClientAccountID != "client-a" || repo.mergeCalls != 1 {
		t.Fatalf("scope not preserved: %+v calls=%d", event, repo.mergeCalls)
	}
	input.TargetSubjectID = "subject-a"
	if _, err := service.MergeSubjects(context.Background(), principal(), "subject-a", "client-a", input); !errors.Is(err, ErrValidation) {
		t.Fatalf("self merge must fail validation, got %v", err)
	}
	if repo.mergeCalls != 1 {
		t.Fatal("self merge reached repository")
	}
}

func TestResolveSubjectCrossClientCandidateDoesNotExposeSubject(t *testing.T) {
	t.Parallel()
	repo := readyFakeRepo()
	repo.resolveResult = ResolveSubjectResult{
		Status: "candidate", CandidateID: "candidate-a", ReasonCodes: []string{"cross_client"},
	}
	service := NewService(repo, allPermissions(), fakeProtector{})
	result, err := service.ResolveSubject(context.Background(), ResolveSubjectRequest{
		AccountID: "owner-a", ClientAccountID: "client-a", RequestID: "resolve-123",
		Source: SourceReference{
			SourceModule: "omnichannel", SourceKey: "contact",
			SourceEntityType: "messaging.contact", SourceEntityID: "contact-a",
		},
		Identities: []IdentityInput{{
			Kind: "whatsapp", Issuer: "whatsapp-cloud", Value: "+5511999999999",
			VerificationStatus: "verified",
		}},
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Status != "candidate" || result.CandidateID == "" ||
		result.SubjectID != "" || result.RelationshipID != "" {
		t.Fatalf("cross-client result leaked IDs: %+v", result)
	}
}

func TestResolveRelationshipCanUseExistingSourceLinkWithoutIdentityPayload(t *testing.T) {
	t.Parallel()
	repo := readyFakeRepo()
	repo.resolveResult = ResolveSubjectResult{
		Status: "resolved", SubjectID: "subject-a", RelationshipID: "relationship-a",
		MatchMethod: "source_link", MatchConfidence: 1, Replayed: true,
	}
	service := NewService(repo, allPermissions(), nil)
	result, err := service.ResolveRelationship(context.Background(), ResolveRelationshipRequest{
		AccountID: "owner-a", ClientAccountID: "client-a", RequestID: "resolve-source-123",
		Source: SourceReference{
			SourceModule: "omnichannel", SourceKey: "contact",
			SourceEntityType: "messaging.contact", SourceEntityID: "contact-a",
		},
	})
	if err != nil {
		t.Fatalf("resolve relationship by source link: %v", err)
	}
	if result.Status != "resolved" || result.RelationshipID != "relationship-a" ||
		result.MatchMethod != "source_link" || !result.Replayed {
		t.Fatalf("unexpected source link result: %+v", result)
	}
	if repo.resolveCalls != 1 {
		t.Fatalf("expected repository resolution, got %d calls", repo.resolveCalls)
	}
}

func TestPublishedSegmentVersionIsImmutableAtServiceBoundary(t *testing.T) {
	t.Parallel()
	repo := readyFakeRepo()
	repo.segmentVersion = SegmentVersion{
		ID: "version-a", Status: "published",
		FilterSchemaVersion: segmentFilterSchema,
		FieldCatalogVersion: SegmentFieldCatalogVersion,
		FilterAST:           validSegmentDraft().FilterAST, EvaluationPolicy: json.RawMessage(`{}`),
	}
	service := NewService(repo, allPermissions(), fakeProtector{})
	_, err := service.UpdateSegmentVersion(context.Background(), principal(), "version-a", SegmentVersionPatch{
		FilterAST: validSegmentDraft().FilterAST, EvaluationPolicy: json.RawMessage(`{}`),
		ExpectedRevision: 1,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected immutable published conflict, got %v", err)
	}
	if repo.updateVersionCalls != 0 {
		t.Fatal("published version reached repository update")
	}
}

func TestControlStateReturnsSafeDefaultsForEntireCatalog(t *testing.T) {
	t.Parallel()
	repo := readyFakeRepo()
	service := NewService(repo, allPermissions(), fakeProtector{})

	state, err := service.GetControlState(context.Background(), principal(), "client-a")
	if err != nil {
		t.Fatalf("get control state: %v", err)
	}
	if len(state.Capabilities) != len(capabilityCatalog) || len(state.Writers) != len(writerCatalog) {
		t.Fatalf("incomplete catalog: %+v", state)
	}
	for _, capability := range state.Capabilities {
		if capability.Mode != CapabilityOff || capability.Revision != 0 {
			t.Fatalf("unsafe capability default: %+v", capability)
		}
	}
	for _, writer := range state.Writers {
		if writer.Mode != WriterLegacy || writer.Revision != 0 {
			t.Fatalf("unsafe writer default: %+v", writer)
		}
	}
}

func TestCapabilityActivationRequiresShadowExceptCore(t *testing.T) {
	t.Parallel()
	repo := readyFakeRepo()
	service := NewService(repo, allPermissions(), fakeProtector{})

	input := CapabilityStateInput{
		Mode: CapabilityOn, ExpectedRevision: 0,
		IdempotencyKey: "capability-123", Reason: "validated rollout",
	}
	if _, _, err := service.SetCapabilityState(
		context.Background(), principal(), "client-a", CapabilitySegmentation, input,
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("off to on must fail for non-core capability, got %v", err)
	}
	if repo.setCapabilityCalls != 0 {
		t.Fatal("invalid transition reached repository")
	}

	input.Mode = CapabilityShadow
	state, _, err := service.SetCapabilityState(
		context.Background(), principal(), "client-a", CapabilitySegmentation, input,
	)
	if err != nil || state.Mode != CapabilityShadow || state.Revision != 1 {
		t.Fatalf("off to shadow failed: state=%+v err=%v", state, err)
	}
	replayedState, replayed, err := service.SetCapabilityState(
		context.Background(), principal(), "client-a", CapabilitySegmentation, input,
	)
	if err != nil || !replayed || replayedState.Revision != state.Revision ||
		repo.setCapabilityCalls != 1 {
		t.Fatalf("capability replay failed: state=%+v replayed=%v err=%v calls=%d",
			replayedState, replayed, err, repo.setCapabilityCalls)
	}
}

func TestWriterCutoverRequiresShadowChecksumsAndCapability(t *testing.T) {
	t.Parallel()
	repo := readyFakeRepo()
	service := NewService(repo, allPermissions(), fakeProtector{})
	checksum := "sha256:equal"

	direct := WriterStateInput{
		Mode: WriterNew, SourceChecksum: &checksum, TargetChecksum: &checksum,
		ExpectedRevision: 0, IdempotencyKey: "writer-direct-123", Reason: "direct cutover",
	}
	if _, _, err := service.SetWriterState(
		context.Background(), principal(), "client-a", WriterSegment, direct,
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("legacy to new must fail, got %v", err)
	}

	shadow := direct
	shadow.Mode = WriterShadow
	shadow.SourceChecksum = nil
	shadow.TargetChecksum = nil
	shadow.IdempotencyKey = "writer-shadow-123"
	state, _, err := service.SetWriterState(
		context.Background(), principal(), "client-a", WriterSegment, shadow,
	)
	if err != nil || state.Mode != WriterShadow || state.Revision != 1 {
		t.Fatalf("legacy to shadow failed: state=%+v err=%v", state, err)
	}

	cutover := direct
	cutover.ExpectedRevision = 1
	cutover.IdempotencyKey = "writer-cutover-123"
	state, _, err = service.SetWriterState(
		context.Background(), principal(), "client-a", WriterSegment, cutover,
	)
	if err != nil || state.Mode != WriterNew || state.Revision != 2 {
		t.Fatalf("shadow to new failed: state=%+v err=%v", state, err)
	}
	replayedState, replayed, err := service.SetWriterState(
		context.Background(), principal(), "client-a", WriterSegment, cutover,
	)
	if err != nil || !replayed || replayedState.Revision != state.Revision ||
		repo.setWriterCalls != 2 {
		t.Fatalf("writer replay failed: state=%+v replayed=%v err=%v calls=%d",
			replayedState, replayed, err, repo.setWriterCalls)
	}
}
