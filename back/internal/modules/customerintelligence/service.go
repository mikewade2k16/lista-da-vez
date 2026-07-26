package customerintelligence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

const (
	defaultContextItems  = 50
	maxContextItems      = 200
	defaultContextTokens = 6000
	maxContextTokens     = 20000
	contextTTL           = 15 * time.Minute
	minPortfolioCohort   = 10
)

type ClientScopeAuthorizer interface {
	AuthorizeClientScope(ctx context.Context, accountID, clientAccountID string) error
}

type ClientScopeAuthorizerFunc func(context.Context, string, string) error

func (f ClientScopeAuthorizerFunc) AuthorizeClientScope(
	ctx context.Context,
	accountID, clientAccountID string,
) error {
	return f(ctx, accountID, clientAccountID)
}

type PortfolioScopeAuthorizer interface {
	AuthorizePortfolioScope(ctx context.Context, accountID, targetClientAccountID string) error
}

type RelationshipScopeAuthorizer interface {
	AuthorizeRelationshipScope(
		ctx context.Context,
		accountID, clientAccountID, subjectID, relationshipID string,
	) error
}

type RelationshipScopeAuthorizerFunc func(context.Context, string, string, string, string) error

func (f RelationshipScopeAuthorizerFunc) AuthorizeRelationshipScope(
	ctx context.Context,
	accountID, clientAccountID, subjectID, relationshipID string,
) error {
	return f(ctx, accountID, clientAccountID, subjectID, relationshipID)
}

type PortfolioScopeAuthorizerFunc func(context.Context, string, string) error

func (f PortfolioScopeAuthorizerFunc) AuthorizePortfolioScope(
	ctx context.Context,
	accountID, targetClientAccountID string,
) error {
	return f(ctx, accountID, targetClientAccountID)
}

type ServiceOption func(*Service)

func WithClientScopeAuthorizer(authorizer ClientScopeAuthorizer) ServiceOption {
	return func(service *Service) {
		if authorizer != nil {
			service.clientAuthorizer = authorizer
		}
	}
}

func WithPortfolioScopeAuthorizer(authorizer PortfolioScopeAuthorizer) ServiceOption {
	return func(service *Service) {
		if authorizer != nil {
			service.portfolioAuthorizer = authorizer
		}
	}
}

func WithRelationshipScopeAuthorizer(authorizer RelationshipScopeAuthorizer) ServiceOption {
	return func(service *Service) {
		service.relationshipAuthorizer = authorizer
	}
}

func WithSourceAdapter(key string, adapter SourceAdapter) ServiceOption {
	return func(service *Service) {
		if validSourceKey(key) && adapter != nil {
			service.sourceAdapters[key] = adapter
		}
	}
}

func withClock(clock func() time.Time) ServiceOption {
	return func(service *Service) {
		if clock != nil {
			service.now = clock
		}
	}
}

type Service struct {
	foundation FoundationRepository
	prompts    PromptRepository
	runs       RuntimeRepository
	secrets    *secretbox.Box
	llm        llm.Client

	clientAuthorizer       ClientScopeAuthorizer
	relationshipAuthorizer RelationshipScopeAuthorizer
	portfolioAuthorizer    PortfolioScopeAuthorizer
	sourceAdapters         map[string]SourceAdapter
	headlessJobs           headlessJobEnqueuer
	now                    func() time.Time
}

func (s *Service) authorizeRelationship(
	ctx context.Context,
	scope Scope,
	subjectID, relationshipID string,
) error {
	if !validUUID(relationshipID) || (subjectID != "" && !validUUID(subjectID)) {
		return ErrInvalidInput
	}
	if s.relationshipAuthorizer == nil {
		return ErrForbidden
	}
	return s.relationshipAuthorizer.AuthorizeRelationshipScope(
		ctx, scope.AccountID, scope.ClientAccountID, subjectID, relationshipID,
	)
}

func NewService(
	repository Repository,
	secrets *secretbox.Box,
	client llm.Client,
	options ...ServiceOption,
) *Service {
	return NewServiceWithRepositories(repository, repository, repository, secrets, client, options...)
}

func NewServiceWithRepositories(
	foundation FoundationRepository,
	prompts PromptRepository,
	runs RuntimeRepository,
	secrets *secretbox.Box,
	client llm.Client,
	options ...ServiceOption,
) *Service {
	service := &Service{
		foundation: foundation,
		prompts:    prompts,
		runs:       runs,
		secrets:    secrets,
		llm:        client,
		clientAuthorizer: ClientScopeAuthorizerFunc(
			func(_ context.Context, accountID, clientAccountID string) error {
				if accountID != clientAccountID {
					return ErrForbidden
				}
				return nil
			},
		),
		portfolioAuthorizer: PortfolioScopeAuthorizerFunc(
			func(context.Context, string, string) error { return ErrForbidden },
		),
		sourceAdapters: make(map[string]SourceAdapter),
		now:            time.Now,
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func (s *Service) RegisterSourceAdapter(key string, adapter SourceAdapter) error {
	if !validSourceKey(key) || adapter == nil {
		return ErrInvalidInput
	}
	s.sourceAdapters[key] = adapter
	return nil
}

func (s *Service) authorizeScope(
	ctx context.Context,
	scope Scope,
) error {
	if !validUUID(scope.AccountID) || !validUUID(scope.ClientAccountID) {
		return ErrInvalidInput
	}
	if s.clientAuthorizer == nil {
		return ErrForbidden
	}
	return s.clientAuthorizer.AuthorizeClientScope(ctx, scope.AccountID, scope.ClientAccountID)
}

func (s *Service) capability(
	ctx context.Context,
	scope Scope,
	key string,
) (Capability, bool, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return Capability{}, false, err
	}
	item, err := s.foundation.GetCapability(ctx, scope, key, "")
	if errors.Is(err, ErrNotFound) {
		return Capability{
			AccountID: scope.AccountID, ClientAccountID: scope.ClientAccountID,
			Key: key, ScopeKey: "", Mode: "off", Config: json.RawMessage(`{}`),
		}, false, nil
	}
	if err != nil {
		return Capability{}, false, err
	}
	return item, item.Mode == "on" || item.Mode == "canary", nil
}

func (s *Service) Capability(
	ctx context.Context,
	scope Scope,
	key string,
	scopeKeys ...string,
) (Capability, error) {
	if _, ok := capabilityCatalog[key]; !ok {
		return Capability{}, ErrInvalidInput
	}
	scopeKey := ""
	if len(scopeKeys) > 0 {
		scopeKey = strings.TrimSpace(scopeKeys[0])
	}
	if !validCapabilityScope(key, scopeKey) {
		return Capability{}, ErrInvalidInput
	}
	if err := s.authorizeScope(ctx, scope); err != nil {
		return Capability{}, err
	}
	item, err := s.foundation.GetCapability(ctx, scope, key, scopeKey)
	if errors.Is(err, ErrNotFound) {
		return Capability{
			AccountID: scope.AccountID, ClientAccountID: scope.ClientAccountID,
			Key: key, ScopeKey: scopeKey, Mode: "off", Config: json.RawMessage(`{}`),
		}, nil
	}
	return item, err
}

func (s *Service) Capabilities(
	ctx context.Context,
	scope Scope,
) ([]Capability, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return nil, err
	}
	persisted, err := s.foundation.ListCapabilities(ctx, scope)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]Capability, len(persisted))
	for _, item := range persisted {
		byKey[item.Key+"\x00"+item.ScopeKey] = item
	}
	items := make([]Capability, 0, len(persisted)+len(capabilityCatalog))
	for key := range capabilityCatalog {
		lookup := key + "\x00"
		item, ok := byKey[lookup]
		if !ok {
			item = Capability{
				AccountID: scope.AccountID, ClientAccountID: scope.ClientAccountID,
				Key: key, Mode: "off", Config: json.RawMessage(`{}`),
			}
		}
		items = append(items, item)
		delete(byKey, lookup)
	}
	for _, item := range byKey {
		items = append(items, item)
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].Key == items[right].Key {
			return items[left].ScopeKey < items[right].ScopeKey
		}
		return items[left].Key < items[right].Key
	})
	return items, nil
}

func (s *Service) SetCapability(
	ctx context.Context,
	accountID, actorID string,
	input CapabilityInput,
) (Capability, error) {
	scope := Scope{AccountID: accountID, ClientAccountID: input.ClientAccountID}
	if err := s.authorizeScope(ctx, scope); err != nil {
		return Capability{}, err
	}
	if !validCapability(input.Key, input.Mode) ||
		!validCapabilityScope(input.Key, input.ScopeKey) ||
		!validJSONObject(input.Config) {
		return Capability{}, ErrInvalidInput
	}
	if input.Key == CapabilityRuntime {
		if _, err := runtimeCapabilityConfigFrom(
			input.Config, input.Mode == "canary",
		); err != nil {
			return Capability{}, err
		}
	}
	return s.foundation.UpsertCapability(ctx, accountID, actorID, input)
}

func (s *Service) Sources(
	ctx context.Context,
	scope Scope,
) ([]SourceConfig, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return nil, err
	}
	return s.foundation.ListSourceConfigs(ctx, scope)
}

func (s *Service) ConfigureSource(
	ctx context.Context,
	accountID, actorID string,
	input SourceConfigInput,
) (SourceConfig, error) {
	scope := Scope{AccountID: accountID, ClientAccountID: input.ClientAccountID}
	if err := s.authorizeScope(ctx, scope); err != nil {
		return SourceConfig{}, err
	}
	if err := validateSourceConfig(input); err != nil {
		return SourceConfig{}, err
	}
	return s.foundation.UpsertSourceConfig(ctx, accountID, actorID, input)
}

func (s *Service) TriggerSourceSync(
	ctx context.Context,
	request SourceSyncRequest,
) (SourceRun, bool, error) {
	scope := Scope{AccountID: request.AccountID, ClientAccountID: request.ClientAccountID}
	_, enabled, err := s.capability(ctx, scope, CapabilitySourceSync)
	if err != nil {
		return SourceRun{}, false, err
	}
	if !enabled {
		return SourceRun{}, false, ErrCapabilityDisabled
	}
	if !validUUID(request.SourceConfigID) ||
		!requestKeyPattern.MatchString(request.IdempotencyKey) ||
		!validMode(request.Trigger, "event", "schedule", "manual", "replay", "backfill", "on_demand") ||
		(request.RelationshipID != "" && !validUUID(request.RelationshipID)) {
		return SourceRun{}, false, ErrInvalidInput
	}
	if request.RelationshipID != "" {
		if err := s.authorizeRelationship(ctx, scope, "", request.RelationshipID); err != nil {
			return SourceRun{}, false, err
		}
	}
	return s.foundation.CreateSourceRun(ctx, request)
}

func (s *Service) SourceRuns(
	ctx context.Context,
	scope Scope,
	sourceConfigID string,
	limit int,
) ([]SourceRun, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return nil, err
	}
	if !validUUID(sourceConfigID) {
		return nil, ErrInvalidInput
	}
	return s.foundation.ListSourceRuns(
		ctx, scope, sourceConfigID, bounded(limit, 50, 1, 200),
	)
}

func (s *Service) CreateManualFact(
	ctx context.Context,
	accountID, actorID string,
	input ManualFactInput,
) (Fact, error) {
	scope := Scope{AccountID: accountID, ClientAccountID: input.ClientAccountID}
	_, enabled, err := s.capability(ctx, scope, CapabilityMemoryWrite)
	if err != nil {
		return Fact{}, err
	}
	if !enabled {
		return Fact{}, ErrCapabilityDisabled
	}
	if !validUUID(input.SubjectID) || !validUUID(input.RelationshipID) ||
		!safeKeyPattern.MatchString(input.FactKey) ||
		!validFactValueType(input.ValueType) ||
		!validSensitivity(input.Sensitivity) ||
		!validJSONValue(input.Value) ||
		!requestKeyPattern.MatchString(input.IdempotencyKey) ||
		len(input.EvidenceNote) > 4000 {
		return Fact{}, ErrInvalidInput
	}
	if err := s.authorizeRelationship(
		ctx, scope, input.SubjectID, input.RelationshipID,
	); err != nil {
		return Fact{}, err
	}
	if input.ValidFrom != nil && input.ValidUntil != nil &&
		!input.ValidUntil.After(*input.ValidFrom) {
		return Fact{}, ErrInvalidInput
	}
	if input.Sensitivity == "personal" || input.Sensitivity == "sensitive" ||
		input.Sensitivity == "restricted" {
		if s.secrets == nil {
			return Fact{}, ErrSecretsUnavailable
		}
		value, err := s.secrets.Encrypt(string(input.Value))
		if err != nil {
			return Fact{}, err
		}
		evidence, _ := json.Marshal(map[string]any{
			"factKey": input.FactKey,
			"value":   input.Value,
			"note":    input.EvidenceNote,
		})
		evidenceCiphertext, err := s.secrets.Encrypt(string(evidence))
		if err != nil {
			return Fact{}, err
		}
		input.ValueCiphertext = value
		input.EvidenceCiphertext = evidenceCiphertext
	}
	fact, err := s.foundation.InsertManualFact(ctx, accountID, actorID, input)
	if err != nil {
		return Fact{}, err
	}
	return s.decryptFact(scope, fact)
}

func (s *Service) Facts(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	limit int,
) ([]Fact, error) {
	_, enabled, err := s.capability(ctx, scope, CapabilityProfile)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return []Fact{}, nil
	}
	if err := s.authorizeRelationship(ctx, scope, "", relationshipID); err != nil {
		return nil, err
	}
	items, err := s.foundation.ListFacts(
		ctx, scope, relationshipID, bounded(limit, 50, 1, 200),
	)
	if err != nil {
		return nil, err
	}
	out := make([]Fact, 0, len(items))
	for _, item := range items {
		decrypted, decryptErr := s.decryptFact(scope, item)
		if decryptErr != nil {
			continue
		}
		out = append(out, decrypted)
	}
	return out, nil
}

func (s *Service) Profile(
	ctx context.Context,
	scope Scope,
	relationshipID string,
) (RelationshipProfileView, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return RelationshipProfileView{}, err
	}
	if !validUUID(relationshipID) {
		return RelationshipProfileView{}, ErrInvalidInput
	}
	if err := s.authorizeRelationship(ctx, scope, "", relationshipID); err != nil {
		return RelationshipProfileView{}, err
	}
	_, enabled, err := s.capability(ctx, scope, CapabilityProfile)
	if err != nil {
		return RelationshipProfileView{}, err
	}
	view := RelationshipProfileView{
		ClientAccountID: scope.ClientAccountID,
		RelationshipID:  relationshipID,
		Facts:           []Fact{},
		Warnings:        []string{},
	}
	if !enabled {
		return view, nil
	}
	facts, err := s.foundation.ListFacts(ctx, scope, relationshipID, 100)
	if err != nil {
		return RelationshipProfileView{}, err
	}
	for _, fact := range facts {
		decrypted, decryptErr := s.decryptFact(scope, fact)
		if decryptErr != nil {
			view.Warnings = append(view.Warnings, "fact_decryption_failed")
			continue
		}
		view.Facts = append(view.Facts, decrypted)
	}
	summaryCiphertext, summary, err := s.foundation.LatestSummary(
		ctx,
		scope,
		relationshipID,
	)
	switch {
	case err == nil && s.secrets == nil:
		view.Warnings = append(view.Warnings, "summary_decryption_unavailable")
	case err == nil:
		plaintext, decryptErr := s.secrets.Decrypt(summaryCiphertext)
		if decryptErr != nil {
			view.Warnings = append(view.Warnings, "summary_decryption_failed")
		} else {
			summary.Body = json.RawMessage(plaintext)
			view.Summary = &summary
		}
	case !errors.Is(err, ErrNotFound):
		return RelationshipProfileView{}, err
	}
	view.Warnings = uniqueSorted(view.Warnings)
	return view, nil
}

func (s *Service) BuildContext(
	ctx context.Context,
	request ContextRequest,
) (ContextEnvelope, error) {
	scope := Scope{AccountID: request.AccountID, ClientAccountID: request.ClientAccountID}
	_, enabled, err := s.capability(ctx, scope, CapabilityContext)
	if err != nil {
		return ContextEnvelope{}, err
	}
	now := s.now().UTC()
	envelope := ContextEnvelope{
		SchemaVersion: "customer-context.v1",
		AccountID:     request.AccountID, ClientAccountID: request.ClientAccountID,
		SubjectID: request.SubjectID, RelationshipID: request.RelationshipID,
		ProcessKeys: append([]string(nil), request.ProcessKeys...),
		Purpose:     strings.TrimSpace(request.Purpose), AsOf: now,
		ExpiresAt: now.Add(contextTTL), Facts: []Fact{},
		Observations: []ContextObservation{},
		Provenance:   []EvidenceRef{}, Warnings: []string{},
		Metadata: json.RawMessage(`{}`),
	}
	if !enabled {
		envelope.Warnings = append(envelope.Warnings, "context_capability_disabled")
		return envelope, nil
	}
	if !validUUID(request.RelationshipID) ||
		(request.SubjectID != "" && !validUUID(request.SubjectID)) ||
		!safeKeyPattern.MatchString(envelope.Purpose) ||
		len(request.ProcessKeys) == 0 {
		return ContextEnvelope{}, ErrInvalidInput
	}
	if err := s.authorizeRelationship(
		ctx, scope, request.SubjectID, request.RelationshipID,
	); err != nil {
		return ContextEnvelope{}, err
	}
	for _, key := range request.ProcessKeys {
		if !validProcessKey(key) {
			return ContextEnvelope{}, ErrInvalidInput
		}
	}
	for _, key := range request.SourceKeys {
		if !validSourceKey(key) {
			return ContextEnvelope{}, ErrInvalidInput
		}
	}
	envelope.ProcessKeys = uniqueSorted(request.ProcessKeys)
	maxItems := bounded(request.MaxItems, defaultContextItems, 1, maxContextItems)
	maxTokens := bounded(request.MaxTokens, defaultContextTokens, 128, maxContextTokens)
	envelope.Budget.MaxItems = maxItems
	envelope.Budget.MaxTokens = maxTokens

	facts, err := s.foundation.ListFacts(ctx, scope, request.RelationshipID, maxItems)
	if err != nil {
		return ContextEnvelope{}, err
	}
	for _, fact := range facts {
		decrypted, err := s.decryptFact(scope, fact)
		if err != nil {
			envelope.Warnings = append(envelope.Warnings, "fact_decryption_failed")
			continue
		}
		envelope.Facts = append(envelope.Facts, decrypted)
	}
	summaryCiphertext, summary, err := s.foundation.LatestSummary(
		ctx,
		scope,
		request.RelationshipID,
	)
	if err == nil {
		if s.secrets == nil {
			envelope.Warnings = append(envelope.Warnings, "summary_decryption_unavailable")
		} else if plaintext, decryptErr := s.secrets.Decrypt(summaryCiphertext); decryptErr != nil {
			envelope.Warnings = append(envelope.Warnings, "summary_decryption_failed")
		} else if len(envelope.Facts) >= maxItems {
			envelope.Warnings = append(envelope.Warnings, "context_item_budget")
		} else {
			summary.Body = json.RawMessage(plaintext)
			envelope.Summary = &summary
		}
	} else if !errors.Is(err, ErrNotFound) {
		return ContextEnvelope{}, err
	}
	remainingItems := maxItems - len(envelope.Facts)
	if envelope.Summary != nil {
		remainingItems--
	}
	if remainingItems > 0 {
		observations, warnings, observationErr := s.contextObservations(
			ctx,
			scope,
			request.RelationshipID,
			request.SourceKeys,
			envelope.Purpose,
			remainingItems,
		)
		envelope.Warnings = append(envelope.Warnings, warnings...)
		if observationErr != nil {
			envelope.Warnings = append(
				envelope.Warnings,
				"source_observations_unavailable",
			)
		} else {
			allowed, purposeWarnings := filterContextObservationsByPurpose(
				envelope.Purpose,
				observations,
			)
			envelope.Warnings = append(envelope.Warnings, purposeWarnings...)
			envelope.Observations = append(envelope.Observations, allowed...)
		}
	} else {
		envelope.Warnings = append(envelope.Warnings, "context_item_budget")
	}

	raw, err := refreshContextEnvelope(&envelope)
	if err != nil {
		return ContextEnvelope{}, err
	}
	for envelope.Budget.EstimatedTokens > maxTokens {
		trimmed := false
		switch {
		case len(envelope.Observations) > 0:
			envelope.Observations = envelope.Observations[:len(envelope.Observations)-1]
			trimmed = true
		case envelope.Summary != nil:
			envelope.Summary = nil
			trimmed = true
		case len(envelope.Facts) > 0:
			envelope.Facts = envelope.Facts[:len(envelope.Facts)-1]
			trimmed = true
		}
		if !trimmed {
			break
		}
		envelope.Warnings = append(envelope.Warnings, "context_token_budget")
		raw, err = refreshContextEnvelope(&envelope)
		if err != nil {
			return ContextEnvelope{}, err
		}
	}
	if s.secrets == nil {
		return ContextEnvelope{}, ErrSecretsUnavailable
	}
	ciphertext, err := s.secrets.Encrypt(string(raw))
	if err != nil {
		return ContextEnvelope{}, err
	}
	envelope.SnapshotID, err = s.foundation.SaveContextSnapshot(
		ctx, envelope, ciphertext, hashBytes(raw),
	)
	if err != nil {
		return ContextEnvelope{}, err
	}
	return envelope, nil
}

func refreshContextEnvelope(envelope *ContextEnvelope) ([]byte, error) {
	provenance := make([]EvidenceRef, 0)
	for _, fact := range envelope.Facts {
		provenance = append(provenance, fact.Evidence...)
	}
	for _, observation := range envelope.Observations {
		provenance = append(provenance, EvidenceRef{
			ObservationID: observation.ID,
			SourceKey:     observation.SourceKey,
			Locator:       observation.ProvenanceRef,
		})
	}
	envelope.Provenance = dedupeEvidence(provenance)
	envelope.Warnings = uniqueSorted(envelope.Warnings)
	envelope.Budget.IncludedItems = len(envelope.Facts) + len(envelope.Observations)
	if envelope.Summary != nil {
		envelope.Budget.IncludedItems++
	}
	envelope.Budget.EstimatedTokens = 0
	raw, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	envelope.Budget.EstimatedTokens = (len(raw) + 3) / 4
	raw, err = json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	estimatedTokens := (len(raw) + 3) / 4
	if estimatedTokens != envelope.Budget.EstimatedTokens {
		envelope.Budget.EstimatedTokens = estimatedTokens
		return json.Marshal(envelope)
	}
	return raw, nil
}

func filterContextObservationsByPurpose(
	requestPurpose string,
	observations []ContextObservation,
) ([]ContextObservation, []string) {
	purposeKeys, err := observationPurposeKeys(requestPurpose)
	if err != nil {
		return []ContextObservation{}, []string{"invalid_context_purpose"}
	}
	purposeAllowed := make(map[string]bool, len(purposeKeys))
	for _, purposeKey := range purposeKeys {
		purposeAllowed[purposeKey] = true
	}
	allowed := make([]ContextObservation, 0, len(observations))
	omitted := false
	for _, observation := range observations {
		observationPurpose := strings.TrimSpace(observation.PurposeKey)
		if !purposeAllowed[observationPurpose] {
			omitted = true
			continue
		}
		allowed = append(allowed, observation)
	}
	if omitted {
		return allowed, []string{"purpose_mismatch_observation_omitted"}
	}
	return allowed, nil
}

func (s *Service) Recommendations(
	ctx context.Context,
	scope Scope,
	relationshipID string,
	limit int,
) ([]Recommendation, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return nil, err
	}
	if !validUUID(relationshipID) {
		return nil, ErrInvalidInput
	}
	if err := s.authorizeRelationship(ctx, scope, "", relationshipID); err != nil {
		return nil, err
	}
	items, err := s.foundation.ListRecommendations(
		ctx,
		scope,
		relationshipID,
		bounded(limit, 50, 1, 200),
	)
	if err != nil {
		return nil, err
	}
	for index := range items {
		if err := s.materializeRecommendation(&items[index]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Service) ReviewRecommendation(
	ctx context.Context,
	scope Scope,
	actorID, recommendationID string,
	input RecommendationFeedback,
) (Recommendation, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return Recommendation{}, err
	}
	if !validUUID(recommendationID) ||
		!validMode(input.Status, "accepted", "rejected") ||
		!safeKeyPattern.MatchString(strings.TrimSpace(input.Reason)) ||
		len(input.Reason) > 80 ||
		!validJSONObject(normalizedJSON(input.Metadata, `{}`)) {
		return Recommendation{}, ErrInvalidInput
	}
	input.Reason = strings.TrimSpace(input.Reason)
	input.Metadata = normalizedJSON(input.Metadata, `{}`)
	item, err := s.foundation.ReviewRecommendation(
		ctx,
		scope,
		actorID,
		recommendationID,
		input,
	)
	if err != nil {
		return Recommendation{}, err
	}
	if err := s.materializeRecommendation(&item); err != nil {
		return Recommendation{}, err
	}
	return item, nil
}

func (s *Service) materializeRecommendation(item *Recommendation) error {
	if item.PayloadCiphertext == "" {
		if len(item.Payload) == 0 || !json.Valid(item.Payload) {
			return ErrInvalidInput
		}
		return nil
	}
	if s.secrets == nil {
		return ErrSecretsUnavailable
	}
	plaintext, err := s.secrets.Decrypt(item.PayloadCiphertext)
	if err != nil {
		return err
	}
	raw := json.RawMessage(plaintext)
	if !json.Valid(raw) {
		return ErrInvalidInput
	}
	item.Payload = raw
	item.PayloadCiphertext = ""
	return nil
}

func (s *Service) RecordOutcome(
	ctx context.Context,
	outcome AcceptedOutcome,
) (bool, error) {
	scope := Scope{AccountID: outcome.AccountID, ClientAccountID: outcome.ClientAccountID}
	if err := s.authorizeScope(ctx, scope); err != nil {
		return false, err
	}
	if !validUUID(outcome.EventID) || outcome.DecisionID == "" ||
		!validMode(outcome.OutcomeType, "reply", "handoff", "no_reply") ||
		(outcome.SubjectID != "" && !validUUID(outcome.SubjectID)) ||
		(outcome.RelationshipID != "" && !validUUID(outcome.RelationshipID)) ||
		(outcome.ConversationID != "" && !validUUID(outcome.ConversationID)) ||
		!validJSONObject(outcome.Payload) {
		return false, ErrInvalidInput
	}
	if outcome.RelationshipID != "" {
		if err := s.authorizeRelationship(
			ctx, scope, outcome.SubjectID, outcome.RelationshipID,
		); err != nil {
			return false, err
		}
	}
	outcome.OccurredAt = nowOr(outcome.OccurredAt, s.now().UTC())
	created, err := s.recordOutcome(ctx, outcome)
	if err != nil {
		return false, err
	}
	if outcome.Accepted &&
		validUUID(outcome.SubjectID) &&
		validUUID(outcome.RelationshipID) &&
		s.headlessJobs != nil {
		_, enqueueErr := s.EnqueueRelationshipRefresh(
			ctx,
			outcome.AccountID,
			"",
			RelationshipRefreshInput{
				ClientAccountID: outcome.ClientAccountID,
				SubjectID:       outcome.SubjectID,
				RelationshipID:  outcome.RelationshipID,
				PurposeKey:      "customer_profile",
				IdempotencyKey:  "accepted-outcome." + outcome.EventID,
				AsOf:            outcome.OccurredAt,
			},
		)
		if enqueueErr != nil && !errors.Is(enqueueErr, ErrCapabilityDisabled) {
			return created, enqueueErr
		}
	}
	return created, nil
}

func (s *Service) PortfolioOpportunities(
	ctx context.Context,
	accountID, targetClientAccountID string,
	limit int,
) ([]PortfolioOpportunity, error) {
	if !validUUID(accountID) || !validUUID(targetClientAccountID) ||
		s.portfolioAuthorizer == nil {
		return nil, ErrForbidden
	}
	if err := s.portfolioAuthorizer.AuthorizePortfolioScope(
		ctx, accountID, targetClientAccountID,
	); err != nil {
		return nil, err
	}
	scope := Scope{AccountID: accountID, ClientAccountID: targetClientAccountID}
	_, enabled, err := s.capability(ctx, scope, CapabilityPortfolio)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return []PortfolioOpportunity{}, nil
	}
	items, err := s.foundation.ListPortfolioOpportunities(
		ctx, accountID, targetClientAccountID, bounded(limit, 50, 1, 200),
	)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].CohortClass = portfolioCohortClass(items[index].CohortSize)
		items[index].SuppressionPolicy = json.RawMessage(
			`{"aggregateOnly":true,"contributorsSuppressed":true,"piiSuppressed":true}`,
		)
		// Exact contributor counts and aggregate payloads never cross the API;
		// they remain server-side inputs protected against differencing.
		items[index].CohortSize = 0
		items[index].SuppressionThreshold = 0
		items[index].Aggregate = nil
	}
	return items, nil
}

func (s *Service) RuntimeRuns(
	ctx context.Context,
	scope Scope,
	limit int,
) ([]RuntimeRunView, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return nil, err
	}
	return s.runs.ListRuntimeRuns(ctx, scope, bounded(limit, 50, 1, 200))
}

func (s *Service) AuditEvents(
	ctx context.Context,
	scope Scope,
	limit int,
) ([]AuditEvent, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return nil, err
	}
	return s.foundation.ListAuditEvents(ctx, scope, bounded(limit, 50, 1, 200))
}

func (s *Service) CreatePortfolioOpportunity(
	ctx context.Context,
	accountID, actorID string,
	item PortfolioOpportunity,
) (PortfolioOpportunity, error) {
	if !validUUID(accountID) || !validUUID(item.TargetClientAccountID) ||
		s.portfolioAuthorizer == nil {
		return PortfolioOpportunity{}, ErrForbidden
	}
	if err := s.portfolioAuthorizer.AuthorizePortfolioScope(
		ctx, accountID, item.TargetClientAccountID,
	); err != nil {
		return PortfolioOpportunity{}, err
	}
	scope := Scope{AccountID: accountID, ClientAccountID: item.TargetClientAccountID}
	_, enabled, err := s.capability(ctx, scope, CapabilityPortfolio)
	if err != nil {
		return PortfolioOpportunity{}, err
	}
	if !enabled {
		return PortfolioOpportunity{}, ErrCapabilityDisabled
	}
	if item.CohortSize < minPortfolioCohort ||
		item.SuppressionThreshold < minPortfolioCohort ||
		item.CohortSize < item.SuppressionThreshold ||
		!safeKeyPattern.MatchString(item.SegmentKey) ||
		!safeKeyPattern.MatchString(item.OpportunityType) ||
		!safeKeyPattern.MatchString(item.RationaleCode) ||
		item.Confidence < 0 || item.Confidence > 1 ||
		!validJSONObject(item.Aggregate) ||
		containsIndividualFields(item.Aggregate) {
		return PortfolioOpportunity{}, ErrInvalidInput
	}
	return s.foundation.CreatePortfolioOpportunity(ctx, accountID, actorID, item)
}

func (s *Service) decryptFact(scope Scope, fact Fact) (Fact, error) {
	if fact.ValueCiphertext != "" {
		if s.secrets == nil {
			return Fact{}, ErrSecretsUnavailable
		}
		plaintext, err := s.secrets.Decrypt(fact.ValueCiphertext)
		if err != nil {
			return Fact{}, err
		}
		fact.Value = json.RawMessage(plaintext)
		fact.ValueCiphertext = ""
	}
	evidence, err := s.protectEvidenceProvenance(scope, fact.Evidence)
	if err != nil {
		return Fact{}, err
	}
	fact.Evidence = evidence
	return fact, nil
}

func (s *Service) protectEvidenceProvenance(
	scope Scope,
	evidence []EvidenceRef,
) ([]EvidenceRef, error) {
	if len(evidence) == 0 {
		return []EvidenceRef{}, nil
	}
	if s.secrets == nil {
		return nil, ErrSecretsUnavailable
	}
	protected := make([]EvidenceRef, 0, len(evidence))
	for _, item := range evidence {
		if !validUUID(item.ObservationID) || !validSourceKey(item.SourceKey) {
			continue
		}
		locator, err := s.opaqueObservationRef(scope, item.SourceKey, item.ObservationID)
		if err != nil {
			return nil, err
		}
		item.Locator = locator
		protected = append(protected, item)
	}
	return protected, nil
}

func validJSONObject(raw json.RawMessage) bool {
	raw = normalizedJSON(raw, `{}`)
	var value map[string]json.RawMessage
	return json.Unmarshal(raw, &value) == nil && value != nil
}

func validJSONValue(raw json.RawMessage) bool {
	if len(bytes.TrimSpace(raw)) == 0 {
		return false
	}
	var value any
	return json.Unmarshal(raw, &value) == nil
}

func validFactValueType(value string) bool {
	return validMode(value, "string", "integer", "decimal", "boolean", "date",
		"timestamp", "enum", "string_list", "object_closed")
}

func validSensitivity(value string) bool {
	return validMode(value, "public", "internal", "personal", "sensitive", "restricted")
}

func bounded(value, fallback, min, max int) int {
	if value == 0 {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func dedupeEvidence(values []EvidenceRef) []EvidenceRef {
	seen := make(map[string]bool, len(values))
	out := make([]EvidenceRef, 0, len(values))
	for _, item := range values {
		key := item.ObservationID + "\x00" + item.Locator
		if item.ObservationID != "" && !seen[key] {
			seen[key] = true
			out = append(out, item)
		}
	}
	return out
}

func containsIndividualFields(raw json.RawMessage) bool {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return true
	}
	blocked := map[string]bool{
		"clientid": true, "customerid": true, "subjectid": true,
		"relationshipid": true, "conversationid": true, "userid": true,
		"name": true, "fullname": true, "email": true, "phone": true,
		"whatsapp": true, "cpf": true, "cnpj": true, "document": true,
		"individuals": true, "records": true, "rows": true,
	}
	var visit func(any) bool
	visit = func(node any) bool {
		switch typed := node.(type) {
		case map[string]any:
			for key, child := range typed {
				normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
				if blocked[normalized] || visit(child) {
					return true
				}
			}
		case []any:
			for _, child := range typed {
				if visit(child) {
					return true
				}
			}
		}
		return false
	}
	return visit(value)
}
