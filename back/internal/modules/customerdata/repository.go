package customerdata

import (
	"context"
	"time"
)

const (
	ResourceRelationship    = "relationship"
	ResourceSubject         = "subject"
	ResourceIdentity        = "identity"
	ResourceNote            = "note"
	ResourceOffline         = "offline_interaction"
	ResourceCandidate       = "match_candidate"
	ResourceMerge           = "merge"
	ResourceSegment         = "segment"
	ResourceSegmentVersion  = "segment_version"
	ResourceEvaluationRun   = "segment_evaluation_run"
	ResourceMaterialization = "segment_materialization"
)

type PermissionChecker interface {
	HasAccountPermission(ctx context.Context, accountID, userID, permission string) (bool, error)
}

type IdentityProtector interface {
	Protect(scope Scope, input IdentityInput) (ProtectedIdentity, error)
	ProtectContent(scope Scope, plaintext string) (ciphertext, keyVersion string, err error)
	RevealContent(scope Scope, ciphertext, keyVersion string) (string, error)
}

type ProfileSections struct {
	Identities   bool
	Notes        bool
	Interactions bool
	Consents     bool
}

type Repository interface {
	ResolveClientScope(ctx context.Context, accountID, requestedClientID, userID string, platformAdmin bool) (Scope, error)
	ResolveServiceScope(ctx context.Context, accountID, clientAccountID string) (Scope, error)
	FindResourceClient(ctx context.Context, accountID, resourceKind, resourceID string) (string, error)
	CapabilityMode(ctx context.Context, scope Scope, capability string) (CapabilityMode, error)
	WriterMode(ctx context.Context, scope Scope, entity string) (WriterMode, error)
	ListCapabilityStates(ctx context.Context, scope Scope) ([]CapabilityState, error)
	GetCapabilityState(ctx context.Context, scope Scope, capability string) (CapabilityState, error)
	SetCapabilityState(ctx context.Context, scope Scope, capability string, input CapabilityStateInput) (CapabilityState, bool, error)
	ListWriterStates(ctx context.Context, scope Scope) ([]WriterState, error)
	GetWriterState(ctx context.Context, scope Scope, entity string) (WriterState, error)
	SetWriterState(ctx context.Context, scope Scope, entity string, input WriterStateInput) (WriterState, bool, error)

	ListSubjects(ctx context.Context, scope Scope, filter SubjectFilter, includeIdentities bool) (SubjectPage, error)
	CreateSubject(ctx context.Context, scope Scope, input CreateSubjectInput, identities []ProtectedIdentity) (CreateSubjectResult, error)
	GetProfile(ctx context.Context, scope Scope, relationshipID string, sections ProfileSections) (DeterministicProfile, error)
	UpdateSubject(ctx context.Context, scope Scope, subjectID string, patch SubjectPatch) (Subject, error)
	UpdateRelationship(ctx context.Context, scope Scope, relationshipID string, patch RelationshipPatch) (Relationship, error)

	ListIdentities(ctx context.Context, scope Scope, relationshipID string) ([]IdentityView, error)
	AddIdentity(ctx context.Context, scope Scope, relationshipID string, identity ProtectedIdentity) (IdentityView, bool, error)
	SetIdentityState(ctx context.Context, scope Scope, identityID, state string, input IdentityStateInput) (IdentityView, bool, error)
	ResolveSubject(ctx context.Context, scope Scope, request ResolveSubjectRequest, identities []ProtectedIdentity) (ResolveSubjectResult, error)
	ListSourceReferences(ctx context.Context, scope Scope, relationshipID string) ([]SourceReference, error)

	ListNotes(ctx context.Context, scope Scope, relationshipID string, limit int) ([]Note, error)
	CreateNote(ctx context.Context, scope Scope, relationshipID string, input NoteInput) (Note, bool, error)
	UpdateNote(ctx context.Context, scope Scope, noteID string, patch NotePatch) (Note, error)
	ArchiveNote(ctx context.Context, scope Scope, noteID string, expectedRevision int64) (Note, error)

	ListOfflineInteractions(ctx context.Context, scope Scope, relationshipID string, limit int, reveal func(string, string) (string, error)) ([]OfflineInteraction, error)
	CreateOfflineInteraction(ctx context.Context, scope Scope, input OfflineInteractionInput, ciphertext, keyVersion string) (OfflineInteraction, bool, error)
	UpdateOfflineInteraction(ctx context.Context, scope Scope, interactionID string, patch OfflineInteractionPatch, ciphertext, keyVersion string) (OfflineInteraction, error)
	ArchiveOfflineInteraction(ctx context.Context, scope Scope, interactionID string, expectedRevision int64) (OfflineInteraction, error)

	ListConsents(ctx context.Context, scope Scope, relationshipID string, limit int) ([]Consent, error)
	RecordConsent(ctx context.Context, scope Scope, relationshipID string, input ConsentInput) (Consent, bool, error)

	ListMatchCandidates(ctx context.Context, scope Scope, filter MatchCandidateFilter) ([]MatchCandidate, string, error)
	GetMatchCandidate(ctx context.Context, scope Scope, candidateID string) (MatchCandidate, error)
	DecideMatchCandidate(ctx context.Context, scope Scope, candidateID string, input MatchDecisionInput) (MatchCandidate, bool, error)
	MergeSubjects(ctx context.Context, scope Scope, sourceSubjectID string, input MergeInput) (MergeEvent, error)
	UndoMerge(ctx context.Context, scope Scope, mergeEventID string, input UndoMergeInput) (MergeEvent, error)

	ListSegments(ctx context.Context, scope Scope, status, cursor string, limit int) ([]Segment, string, error)
	CreateSegment(ctx context.Context, scope Scope, input CreateSegmentInput, definitionHash string) (CreateSegmentResult, error)
	GetSegment(ctx context.Context, scope Scope, segmentID string) (Segment, error)
	UpdateSegment(ctx context.Context, scope Scope, segmentID string, patch SegmentPatch) (Segment, error)
	ArchiveSegment(ctx context.Context, scope Scope, segmentID string, expectedRevision int64) (Segment, error)
	ListSegmentVersions(ctx context.Context, scope Scope, segmentID string) ([]SegmentVersion, error)
	CreateSegmentVersion(ctx context.Context, scope Scope, segmentID string, input CreateSegmentVersionInput, draft SegmentDraftInput, definitionHash string) (SegmentVersion, bool, error)
	GetSegmentVersion(ctx context.Context, scope Scope, versionID string) (SegmentVersion, error)
	UpdateSegmentVersion(ctx context.Context, scope Scope, versionID string, patch SegmentVersionPatch, definitionHash string) (SegmentVersion, error)
	ValidateSegmentVersion(ctx context.Context, scope Scope, versionID, validationHash string, cost int) (SegmentVersion, error)
	PublishSegmentVersion(ctx context.Context, scope Scope, versionID string, input PublishSegmentVersionInput) (SegmentVersion, bool, error)
	RollbackSegment(ctx context.Context, scope Scope, segmentID string, input RollbackSegmentInput) (Segment, bool, error)
	CreateEvaluationRun(ctx context.Context, scope Scope, segmentID string, request SegmentEvaluationRequest, version SegmentVersion, asOf time.Time) (SegmentEvaluationRun, error)
	GetEvaluationRun(ctx context.Context, scope Scope, runID string) (SegmentEvaluationRun, error)
	ListMaterializations(ctx context.Context, scope Scope, segmentID string, limit int) ([]SegmentMaterialization, error)
	ListMaterializationMembers(ctx context.Context, scope Scope, materializationID, cursor string, limit int) ([]SegmentMember, string, error)
	GetSegmentContext(ctx context.Context, scope Scope, relationshipID string, asOf time.Time) (SegmentContext, error)
}
