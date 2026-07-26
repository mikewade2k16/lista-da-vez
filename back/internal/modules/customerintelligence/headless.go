package customerintelligence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

const (
	headlessPipelineKey         = "intelligence.headless"
	relationshipRefreshJobKind  = "intelligence.relationship.refresh"
	runtimeJobTable             = "intelligence.runtime_jobs"
	maxHeadlessRelationshipRuns = 6
)

var defaultRelationshipRefreshProcesses = []string{
	"profile.summary",
	"recommendation.follow_up",
	"recommendation.offer",
	"recommendation.important_dates",
	"source.suggest",
}

var relationshipHeadlessProcessCatalog = map[string]bool{
	"profile.summary":                true,
	"recommendation.follow_up":       true,
	"recommendation.offer":           true,
	"recommendation.important_dates": true,
	"source.suggest":                 true,
}

type headlessJobEnqueuer interface {
	Enqueue(ctx context.Context, job jobs.NewJob) (id string, created bool, err error)
}

type headlessResultRepository interface {
	PersistHeadlessRefresh(
		ctx context.Context,
		input HeadlessRefreshPersistence,
	) (int, error)
}

type RelationshipRefreshInput struct {
	ClientAccountID string    `json:"clientAccountId"`
	SubjectID       string    `json:"subjectId"`
	RelationshipID  string    `json:"relationshipId"`
	PurposeKey      string    `json:"purposeKey"`
	SourceKeys      []string  `json:"sourceKeys,omitempty"`
	ProcessKeys     []string  `json:"processKeys,omitempty"`
	IdempotencyKey  string    `json:"idempotencyKey"`
	AsOf            time.Time `json:"asOf,omitempty"`
}

type RelationshipRefreshJob struct {
	ID      string `json:"id,omitempty"`
	Status  string `json:"status"`
	Created bool   `json:"created"`
}

type RelationshipRefreshResult struct {
	RequestID        string          `json:"requestId"`
	ExecutionMode    string          `json:"executionMode"`
	Persisted        bool            `json:"persisted"`
	PersistedRows    int             `json:"persistedRows"`
	ProcessRuns      []ProcessRunRef `json:"processRunRefs"`
	Warnings         []string        `json:"warnings"`
	RetryProcessKeys []string        `json:"-"`
}

type HeadlessPersistedExecution struct {
	RunRef           ProcessRunRef
	Output           json.RawMessage
	OutputCiphertext string
	OutputHash       string
}

type HeadlessRefreshPersistence struct {
	Scope          Scope
	SubjectID      string
	RelationshipID string
	ContextID      string
	Context        ContextEnvelope
	AsOf           time.Time
	Executions     []HeadlessPersistedExecution
}

func WithHeadlessJobEnqueuer(enqueuer headlessJobEnqueuer) ServiceOption {
	return func(service *Service) {
		service.headlessJobs = enqueuer
	}
}

func (s *Service) EnqueueRelationshipRefresh(
	ctx context.Context,
	accountID, actorUserID string,
	input RelationshipRefreshInput,
) (RelationshipRefreshJob, error) {
	scope := Scope{
		AccountID:       strings.TrimSpace(accountID),
		ClientAccountID: strings.TrimSpace(input.ClientAccountID),
	}
	if err := s.authorizeScope(ctx, scope); err != nil {
		return RelationshipRefreshJob{}, err
	}
	if actorUserID != "" && !validUUID(actorUserID) {
		return RelationshipRefreshJob{}, ErrInvalidInput
	}
	normalized, err := normalizedRelationshipRefreshInput(input)
	if err != nil {
		return RelationshipRefreshJob{}, err
	}
	if err := s.authorizeRelationship(
		ctx,
		scope,
		normalized.SubjectID,
		normalized.RelationshipID,
	); err != nil {
		return RelationshipRefreshJob{}, err
	}
	_, profileEnabled, err := s.capability(ctx, scope, CapabilityProfile)
	if err != nil {
		return RelationshipRefreshJob{}, err
	}
	if !profileEnabled {
		return RelationshipRefreshJob{}, ErrCapabilityDisabled
	}
	runtimeCapability, err := s.runtimeCapability(ctx, scope, "")
	if err != nil {
		return RelationshipRefreshJob{}, err
	}
	if runtimeCapability.Mode == "off" {
		return RelationshipRefreshJob{}, ErrCapabilityDisabled
	}
	if s.headlessJobs == nil || s.secrets == nil {
		return RelationshipRefreshJob{}, ErrNotFound
	}
	payload, err := json.Marshal(relationshipRefreshJobPayload{
		ClientAccountID: normalized.ClientAccountID,
		SubjectID:       normalized.SubjectID,
		RelationshipID:  normalized.RelationshipID,
		PurposeKey:      normalized.PurposeKey,
		SourceKeys:      normalized.SourceKeys,
		ProcessKeys:     normalized.ProcessKeys,
		ActorUserID:     actorUserID,
		AsOf:            normalized.AsOf,
	})
	if err != nil {
		return RelationshipRefreshJob{}, err
	}
	idempotencyFingerprint := s.secrets.OpaqueFingerprint(
		"customer-intelligence.relationship-refresh-idempotency.v1",
		scope.AccountID,
		scope.ClientAccountID,
		normalized.RelationshipID,
		normalized.IdempotencyKey,
	)
	if idempotencyFingerprint == "" {
		return RelationshipRefreshJob{}, ErrSecretsUnavailable
	}
	id, created, err := s.headlessJobs.Enqueue(ctx, jobs.NewJob{
		AccountID:      scope.AccountID,
		OrderingKey:    "relationship-refresh:" + normalized.RelationshipID,
		IdempotencyKey: "relationship-refresh:" + idempotencyFingerprint,
		Kind:           relationshipRefreshJobKind,
		Payload:        payload,
		MaxAttempts:    5,
	})
	if err != nil {
		return RelationshipRefreshJob{}, err
	}
	status := "existing"
	if created {
		status = "pending"
	}
	return RelationshipRefreshJob{ID: id, Status: status, Created: created}, nil
}

func normalizedRelationshipRefreshInput(
	input RelationshipRefreshInput,
) (RelationshipRefreshInput, error) {
	input.ClientAccountID = strings.TrimSpace(input.ClientAccountID)
	input.SubjectID = strings.TrimSpace(input.SubjectID)
	input.RelationshipID = strings.TrimSpace(input.RelationshipID)
	input.PurposeKey = strings.TrimSpace(input.PurposeKey)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.PurposeKey == "" {
		input.PurposeKey = "customer_profile"
	}
	if !validUUID(input.ClientAccountID) ||
		!validUUID(input.SubjectID) ||
		!validUUID(input.RelationshipID) ||
		!requestKeyPattern.MatchString(input.IdempotencyKey) ||
		!validMode(input.PurposeKey, "customer_profile", "customer_service", "marketing") {
		return RelationshipRefreshInput{}, ErrInvalidInput
	}
	sourceKeys, err := validatedSourceKeys(input.SourceKeys)
	if err != nil {
		return RelationshipRefreshInput{}, err
	}
	input.SourceKeys = sourceKeys
	if len(input.ProcessKeys) == 0 {
		input.ProcessKeys = append([]string(nil), defaultRelationshipRefreshProcesses...)
	} else {
		input.ProcessKeys = uniqueSorted(input.ProcessKeys)
	}
	if len(input.ProcessKeys) == 0 ||
		len(input.ProcessKeys) > maxHeadlessRelationshipRuns {
		return RelationshipRefreshInput{}, ErrInvalidInput
	}
	for _, processKey := range input.ProcessKeys {
		if !relationshipHeadlessProcessCatalog[processKey] {
			return RelationshipRefreshInput{}, ErrInvalidInput
		}
	}
	input.AsOf = nowOr(input.AsOf, time.Now().UTC()).UTC()
	return input, nil
}

func (s *Service) executeRelationshipRefresh(
	ctx context.Context,
	accountID, requestID string,
	input RelationshipRefreshInput,
) (RelationshipRefreshResult, error) {
	scope := Scope{AccountID: accountID, ClientAccountID: input.ClientAccountID}
	if err := s.authorizeScope(ctx, scope); err != nil {
		return RelationshipRefreshResult{}, err
	}
	normalized, err := normalizedRelationshipRefreshInput(input)
	if err != nil || !requestKeyPattern.MatchString(requestID) {
		if err != nil {
			return RelationshipRefreshResult{}, err
		}
		return RelationshipRefreshResult{}, ErrInvalidInput
	}
	if err := s.authorizeRelationship(
		ctx,
		scope,
		normalized.SubjectID,
		normalized.RelationshipID,
	); err != nil {
		return RelationshipRefreshResult{}, err
	}
	_, profileEnabled, err := s.capability(ctx, scope, CapabilityProfile)
	if err != nil {
		return RelationshipRefreshResult{}, err
	}
	if !profileEnabled {
		return RelationshipRefreshResult{}, ErrCapabilityDisabled
	}
	runtimeCapability, err := s.runtimeCapability(ctx, scope, "")
	if err != nil {
		return RelationshipRefreshResult{}, err
	}
	if runtimeCapability.Mode == "off" {
		return RelationshipRefreshResult{}, ErrCapabilityDisabled
	}
	if s.llm == nil || s.secrets == nil || s.runs == nil {
		return RelationshipRefreshResult{}, ErrSecretsUnavailable
	}
	request := InteractionRequest{
		SchemaVersion:       "interaction.request.v1",
		RequestID:           requestID,
		AccountID:           accountID,
		ClientAccountID:     normalized.ClientAccountID,
		SubjectID:           normalized.SubjectID,
		RelationshipID:      normalized.RelationshipID,
		PipelineKey:         headlessPipelineKey,
		Message:             json.RawMessage(`{"trigger":"relationship_refresh"}`),
		OperationalState:    json.RawMessage(`{}`),
		RoutingCatalog:      json.RawMessage(`{}`),
		ChannelCapabilities: json.RawMessage(`{}`),
		Purpose:             normalized.PurposeKey,
		AsOf:                normalized.AsOf,
		SourceKeys:          normalized.SourceKeys,
	}
	executionMode := "active"
	persist := true
	warnings := make([]string, 0)
	switch runtimeCapability.Mode {
	case "shadow":
		executionMode = "shadow"
		persist = false
		warnings = append(warnings, "shadow_no_effect")
	case "canary":
		selected, reasonCode := s.runtimeCanarySelected(runtimeCapability, request)
		warnings = append(warnings, reasonCode)
		if !selected {
			executionMode = "shadow"
			persist = false
			warnings = append(warnings, "shadow_no_effect")
		}
	case "on":
	default:
		return RelationshipRefreshResult{}, ErrInvalidInput
	}
	pipelineVersionID, err := s.runs.ResolvePipelineVersion(ctx, headlessPipelineKey)
	if err != nil {
		return RelationshipRefreshResult{}, err
	}
	envelope, err := s.BuildContext(ctx, ContextRequest{
		AccountID:       accountID,
		ClientAccountID: normalized.ClientAccountID,
		SubjectID:       normalized.SubjectID,
		RelationshipID:  normalized.RelationshipID,
		ProcessKeys:     normalized.ProcessKeys,
		Purpose:         normalized.PurposeKey,
		SourceKeys:      normalized.SourceKeys,
	})
	if err != nil {
		return RelationshipRefreshResult{}, err
	}
	warnings = append(warnings, envelope.Warnings...)
	executions := make([]HeadlessPersistedExecution, 0, len(normalized.ProcessKeys))
	runs := make([]ProcessRunRef, 0, len(normalized.ProcessKeys))
	var retryableProcessErr error
	retryProcessKeys := make([]string, 0)
	for _, processKey := range normalized.ProcessKeys {
		execution, executeErr := s.executeProcess(
			ctx,
			request,
			envelope,
			pipelineVersionID,
			executionMode,
			processKey,
		)
		if executeErr != nil {
			warnings = append(
				warnings,
				headlessProcessWarning(processKey, executeErr),
			)
			if _, _, retryable, ok := RuntimeFailureDetails(executeErr); ok &&
				retryable && retryableProcessErr == nil {
				retryableProcessErr = executeErr
			}
			if _, _, retryable, ok := RuntimeFailureDetails(executeErr); ok &&
				retryable {
				retryProcessKeys = append(retryProcessKeys, processKey)
			}
			continue
		}
		ciphertext, encryptErr := s.secrets.Encrypt(string(execution.Raw))
		if encryptErr != nil {
			return RelationshipRefreshResult{}, encryptErr
		}
		runs = append(runs, execution.RunRef)
		executions = append(executions, HeadlessPersistedExecution{
			RunRef:           execution.RunRef,
			Output:           append(json.RawMessage(nil), execution.Raw...),
			OutputCiphertext: ciphertext,
			OutputHash:       hashBytes(execution.Raw),
		})
	}
	result := RelationshipRefreshResult{
		RequestID:        requestID,
		ExecutionMode:    executionMode,
		Persisted:        false,
		ProcessRuns:      runs,
		Warnings:         uniqueSorted(warnings),
		RetryProcessKeys: uniqueSorted(retryProcessKeys),
	}
	if !persist {
		if retryableProcessErr != nil {
			return result, retryableProcessErr
		}
		return result, nil
	}
	if len(executions) == 0 {
		if retryableProcessErr != nil {
			return result, retryableProcessErr
		}
		return result, nil
	}
	repository, ok := s.foundation.(headlessResultRepository)
	if !ok {
		return RelationshipRefreshResult{}, ErrNotFound
	}
	persistedRows, err := repository.PersistHeadlessRefresh(
		ctx,
		HeadlessRefreshPersistence{
			Scope:          scope,
			SubjectID:      normalized.SubjectID,
			RelationshipID: normalized.RelationshipID,
			ContextID:      envelope.SnapshotID,
			Context:        envelope,
			AsOf:           normalized.AsOf,
			Executions:     executions,
		},
	)
	if err != nil {
		return RelationshipRefreshResult{}, err
	}
	result.Persisted = true
	result.PersistedRows = persistedRows
	if retryableProcessErr != nil {
		return result, retryableProcessErr
	}
	return result, nil
}

func headlessProcessWarning(processKey string, err error) string {
	code := "runtime_unavailable"
	if _, failureCode, _, ok := RuntimeFailureDetails(err); ok &&
		safeKeyPattern.MatchString(failureCode) {
		code = failureCode
	}
	processCode := strings.ReplaceAll(processKey, ".", "_")
	return "process." + processCode + "." + code
}

func headlessJobError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrInvalidInput) ||
		errors.Is(err, ErrForbidden) ||
		errors.Is(err, ErrCapabilityDisabled) ||
		errors.Is(err, ErrPromptNotPublished) ||
		errors.Is(err, ErrAgentNotPublished) ||
		errors.Is(err, ErrProviderNotConfigured) {
		return &jobs.StatusError{Unrecoverable: true, Err: err}
	}
	if _, _, retryable, ok := RuntimeFailureDetails(err); ok {
		return &jobs.StatusError{Unrecoverable: !retryable, Err: err}
	}
	return err
}
