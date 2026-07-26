package customerintelligence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

func (s *Service) Processes(ctx context.Context) ([]ProcessDefinition, error) {
	if s.prompts == nil {
		return nil, ErrNotFound
	}
	return s.prompts.ListProcesses(ctx)
}

func (s *Service) PromptVersions(
	ctx context.Context,
	scope Scope,
	processKey string,
) ([]PromptVersion, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return nil, err
	}
	if processKey != "" && !validProcessKey(processKey) {
		return nil, ErrInvalidInput
	}
	return s.prompts.ListPromptVersions(
		ctx, scope.AccountID, optionalClient(scope), processKey,
	)
}

func (s *Service) PromptBindings(
	ctx context.Context,
	scope Scope,
	processKey string,
) ([]PromptBinding, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return nil, err
	}
	if processKey != "" && !validProcessKey(processKey) {
		return nil, ErrInvalidInput
	}
	return s.prompts.ListPromptBindings(
		ctx, scope.AccountID, optionalClient(scope), processKey,
	)
}

func (s *Service) CreatePromptDraft(
	ctx context.Context,
	accountID, actorID string,
	input PromptDraftInput,
) (PromptVersion, error) {
	scope := Scope{AccountID: accountID, ClientAccountID: effectiveClient(accountID, input.ClientAccountID)}
	if err := s.authorizeScope(ctx, scope); err != nil {
		return PromptVersion{}, err
	}
	input.Layer = normalizeLayer(input.Layer)
	if !validProcessKey(input.ProcessKey) ||
		!validMode(input.Layer, "agency_policy", "client_policy", "process_prompt") ||
		len(strings.TrimSpace(input.Content)) == 0 || len(input.Content) > 200000 ||
		(input.BasedOnVersionID != "" && !validUUID(input.BasedOnVersionID)) {
		return PromptVersion{}, ErrInvalidInput
	}
	if input.Layer == "agency_policy" && input.ClientAccountID != "" {
		return PromptVersion{}, ErrInvalidInput
	}
	if input.Layer == "client_policy" && input.ClientAccountID == "" {
		return PromptVersion{}, ErrInvalidInput
	}
	process, err := s.findProcess(ctx, input.ProcessKey)
	if err != nil {
		return PromptVersion{}, err
	}
	if len(input.OutputSchema) > 0 &&
		!jsonEquivalent(input.OutputSchema, process.OutputSchema) {
		return PromptVersion{}, fmt.Errorf("%w: output schema pertence ao processo e deve ser versionado separadamente", ErrInvalidInput)
	}
	validation := validatePrompt(input.Content, process.AllowedVariables, process.OutputSchema)
	if !validation.Valid {
		return PromptVersion{}, fmt.Errorf("%w: %s", ErrInvalidInput, strings.Join(validation.ReasonCodes, ","))
	}
	return s.prompts.CreatePromptDraft(ctx, accountID, actorID, input, validation.Variables)
}

func (s *Service) ValidatePromptVersion(
	ctx context.Context,
	accountID, actorID, id string,
) (PromptVersion, PromptValidation, error) {
	if !validUUID(accountID) || !validUUID(id) {
		return PromptVersion{}, PromptValidation{}, ErrInvalidInput
	}
	prompt, err := s.prompts.GetPromptVersion(ctx, accountID, id)
	if err != nil {
		return PromptVersion{}, PromptValidation{}, err
	}
	scope := Scope{AccountID: accountID, ClientAccountID: effectiveClient(accountID, prompt.ClientAccountID)}
	if err := s.authorizeScope(ctx, scope); err != nil {
		return PromptVersion{}, PromptValidation{}, err
	}
	process, err := s.findProcess(ctx, prompt.ProcessKey)
	if err != nil {
		return PromptVersion{}, PromptValidation{}, err
	}
	validation := validatePrompt(prompt.Content, process.AllowedVariables, process.OutputSchema)
	if !validation.Valid {
		return prompt, validation, nil
	}
	prompt, err = s.prompts.MarkPromptValidated(
		ctx, accountID, actorID, id, validation.Variables,
	)
	return prompt, validation, err
}

func (s *Service) UpdatePromptVersion(
	ctx context.Context,
	accountID, actorID, id string,
	input PromptPatchInput,
) (PromptVersion, error) {
	if !validUUID(accountID) || !validUUID(id) || input.ExpectedRevision <= 0 ||
		len(strings.TrimSpace(input.Content)) == 0 || len(input.Content) > 200000 {
		return PromptVersion{}, ErrInvalidInput
	}
	prompt, err := s.prompts.GetPromptVersion(ctx, accountID, id)
	if err != nil {
		return PromptVersion{}, err
	}
	scope := Scope{AccountID: accountID, ClientAccountID: effectiveClient(accountID, prompt.ClientAccountID)}
	if err := s.authorizeScope(ctx, scope); err != nil {
		return PromptVersion{}, err
	}
	process, err := s.findProcess(ctx, prompt.ProcessKey)
	if err != nil {
		return PromptVersion{}, err
	}
	if len(input.OutputSchema) > 0 && !jsonEquivalent(input.OutputSchema, process.OutputSchema) {
		return PromptVersion{}, ErrInvalidInput
	}
	validation := validatePrompt(input.Content, process.AllowedVariables, process.OutputSchema)
	if !validation.Valid {
		return PromptVersion{}, fmt.Errorf("%w: %s", ErrInvalidInput, strings.Join(validation.ReasonCodes, ","))
	}
	return s.prompts.UpdatePromptDraft(
		ctx, accountID, actorID, id, input.Content,
		validation.Variables, input.ExpectedRevision,
	)
}

func (s *Service) TestPromptVersion(
	ctx context.Context,
	accountID, actorID, id string,
	fixture json.RawMessage,
) (PromptEvaluation, error) {
	if !validUUID(accountID) || !validUUID(id) ||
		(len(fixture) > 0 && !validJSONObject(fixture)) {
		return PromptEvaluation{}, ErrInvalidInput
	}
	prompt, err := s.prompts.GetPromptVersion(ctx, accountID, id)
	if err != nil {
		return PromptEvaluation{}, err
	}
	scope := Scope{AccountID: accountID, ClientAccountID: effectiveClient(accountID, prompt.ClientAccountID)}
	if err := s.authorizeScope(ctx, scope); err != nil {
		return PromptEvaluation{}, err
	}
	process, err := s.findProcess(ctx, prompt.ProcessKey)
	if err != nil {
		return PromptEvaluation{}, err
	}
	validation := validatePrompt(prompt.Content, process.AllowedVariables, process.OutputSchema)
	status := "passed"
	score := 1.0
	if !validation.Valid {
		status = "failed"
		score = 0
	}
	scores, _ := json.Marshal(map[string]float64{"structural": score})
	return s.prompts.CreatePromptEvaluation(
		ctx, accountID, actorID, id, status, validation.ReasonCodes, scores,
	)
}

func (s *Service) PromptEvaluations(
	ctx context.Context,
	accountID, promptVersionID string,
) ([]PromptEvaluation, error) {
	if !validUUID(accountID) || !validUUID(promptVersionID) {
		return nil, ErrInvalidInput
	}
	prompt, err := s.prompts.GetPromptVersion(ctx, accountID, promptVersionID)
	if err != nil {
		return nil, err
	}
	scope := Scope{
		AccountID:       accountID,
		ClientAccountID: effectiveClient(accountID, prompt.ClientAccountID),
	}
	if err := s.authorizeScope(ctx, scope); err != nil {
		return nil, err
	}
	return s.prompts.ListPromptEvaluations(
		ctx, accountID, promptVersionID, 100,
	)
}

func (s *Service) PublishPromptVersion(
	ctx context.Context,
	accountID, actorID, id string,
	input PublishPromptInput,
) (PromptBinding, error) {
	if !validUUID(accountID) || !validUUID(id) ||
		!validUUID(input.AgentVersionID) ||
		!validJSONArray(input.SourcePolicy) ||
		!validJSONArray(input.ToolPolicy) ||
		!validJSONArray(input.KnowledgePolicy) ||
		!validJSONObject(input.RuntimePolicy) {
		return PromptBinding{}, ErrInvalidInput
	}
	prompt, err := s.prompts.GetPromptVersion(ctx, accountID, id)
	if err != nil {
		return PromptBinding{}, err
	}
	scope := Scope{AccountID: accountID, ClientAccountID: effectiveClient(accountID, prompt.ClientAccountID)}
	if err := s.authorizeScope(ctx, scope); err != nil {
		return PromptBinding{}, err
	}
	if prompt.Status != "validated" && prompt.Status != "published" {
		return PromptBinding{}, ErrPromptNotValidated
	}
	evaluations, err := s.prompts.ListPromptEvaluations(ctx, accountID, id, 1)
	if err != nil {
		return PromptBinding{}, err
	}
	if len(evaluations) == 0 ||
		evaluations[0].Status != "passed" ||
		prompt.ValidatedAt == nil ||
		evaluations[0].CreatedAt.Before(*prompt.ValidatedAt) {
		return PromptBinding{}, ErrPromptEvaluationRequired
	}
	agentClientAccountID, err := s.prompts.AgentVersionClientScope(
		ctx, accountID, input.AgentVersionID,
	)
	if err != nil {
		return PromptBinding{}, err
	}
	if agentClientAccountID != prompt.ClientAccountID {
		return PromptBinding{}, ErrForbidden
	}
	input.ClientAccountID = prompt.ClientAccountID
	return s.prompts.PublishPrompt(ctx, accountID, actorID, id, input)
}

func (s *Service) RollbackPromptBinding(
	ctx context.Context,
	accountID, actorID, bindingID string,
	input RollbackPromptInput,
) (PromptBinding, error) {
	if !validUUID(accountID) || !validUUID(bindingID) ||
		!validUUID(input.TargetPromptVersionID) ||
		!safeKeyPattern.MatchString(strings.TrimSpace(input.ReasonCode)) {
		return PromptBinding{}, ErrInvalidInput
	}
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	target, err := s.prompts.GetPromptVersion(
		ctx, accountID, input.TargetPromptVersionID,
	)
	if err != nil {
		return PromptBinding{}, err
	}
	scope := Scope{AccountID: accountID, ClientAccountID: effectiveClient(accountID, target.ClientAccountID)}
	if err := s.authorizeScope(ctx, scope); err != nil {
		return PromptBinding{}, err
	}
	if target.Status != "published" {
		return PromptBinding{}, ErrInvalidInput
	}
	return s.prompts.RollbackPrompt(ctx, accountID, actorID, bindingID, input)
}

func (s *Service) Models(ctx context.Context, accountID string) ([]AIModel, error) {
	if !validUUID(accountID) {
		return nil, ErrInvalidInput
	}
	return s.prompts.ListModels(ctx, accountID)
}

func (s *Service) ConfigureModel(
	ctx context.Context,
	accountID, actorID string,
	model AIModel,
) (AIModel, error) {
	model.Provider = canonicalString(model.Provider)
	model.Model = strings.TrimSpace(model.Model)
	if !validUUID(accountID) || !validProvider(model.Provider) ||
		model.Model == "" || len(model.Model) > 200 ||
		!validMode(model.Status, "enabled", "disabled") ||
		!validJSONObject(model.Config) {
		return AIModel{}, ErrInvalidInput
	}
	effective, err := llm.ResolveBaseURL(model.Provider, model.BaseURL)
	if err != nil {
		return AIModel{}, err
	}
	model.BaseURL = effective
	return s.prompts.UpsertModel(ctx, accountID, actorID, model)
}

func (s *Service) Credentials(
	ctx context.Context,
	accountID string,
) ([]AICredential, error) {
	if !validUUID(accountID) {
		return nil, ErrInvalidInput
	}
	records, err := s.prompts.ListCredentials(ctx, accountID)
	if err != nil {
		return nil, err
	}
	items := make([]AICredential, 0, len(records))
	for _, record := range records {
		items = append(items, credentialDTO(record))
	}
	return items, nil
}

func (s *Service) SetCredential(
	ctx context.Context,
	accountID, actorID string,
	input CredentialInput,
) (AICredential, error) {
	input.Provider = canonicalString(input.Provider)
	input.Label = strings.TrimSpace(input.Label)
	if !validUUID(accountID) || !validProvider(input.Provider) ||
		input.Label == "" || len(input.Label) > 160 ||
		len(strings.TrimSpace(input.APIKey)) < 8 || len(input.APIKey) > 20000 {
		return AICredential{}, ErrInvalidInput
	}
	if s.secrets == nil {
		return AICredential{}, ErrSecretsUnavailable
	}
	ciphertext, err := s.secrets.Encrypt(input.APIKey)
	if err != nil {
		return AICredential{}, err
	}
	mask := secretbox.Mask(input.APIKey)
	record, err := s.prompts.UpsertCredential(
		ctx, accountID, actorID, input, ciphertext, mask.Last4,
	)
	input.APIKey = ""
	if err != nil {
		return AICredential{}, err
	}
	return credentialDTO(record), nil
}

func (s *Service) RevokeCredential(
	ctx context.Context,
	accountID, actorID, id string,
) error {
	if !validUUID(accountID) || !validUUID(id) {
		return ErrInvalidInput
	}
	return s.prompts.RevokeCredential(ctx, accountID, actorID, id)
}

func (s *Service) Agents(
	ctx context.Context,
	scope Scope,
) ([]AIAgent, error) {
	if err := s.authorizeScope(ctx, scope); err != nil {
		return nil, err
	}
	return s.prompts.ListAgents(ctx, scope.AccountID, optionalClient(scope))
}

func (s *Service) CreateAgent(
	ctx context.Context,
	accountID, actorID, clientAccountID, slug, name string,
) (AIAgent, error) {
	scope := Scope{AccountID: accountID, ClientAccountID: effectiveClient(accountID, clientAccountID)}
	if err := s.authorizeScope(ctx, scope); err != nil {
		return AIAgent{}, err
	}
	slug = canonicalString(slug)
	name = strings.TrimSpace(name)
	if !safeKeyPattern.MatchString(slug) || name == "" || len(name) > 200 {
		return AIAgent{}, ErrInvalidInput
	}
	return s.prompts.CreateAgent(ctx, accountID, actorID, clientAccountID, slug, name)
}

func (s *Service) CreateAgentVersion(
	ctx context.Context,
	accountID, actorID, agentID string,
	input AIAgentVersionInput,
) (AIAgentVersion, error) {
	if !validUUID(accountID) || !validUUID(agentID) || !validUUID(input.ModelID) ||
		(input.CredentialID != "" && !validUUID(input.CredentialID)) ||
		input.Temperature < 0 || input.Temperature > 2 ||
		input.MaxOutputTokens < 16 || input.MaxOutputTokens > 100000 ||
		input.TimeoutMS < 1000 || input.TimeoutMS > 300000 ||
		len(input.PromptOverride) > 200000 || !validJSONObject(input.Config) {
		return AIAgentVersion{}, ErrInvalidInput
	}
	clientAccountID, err := s.prompts.AgentClientScope(ctx, accountID, agentID)
	if err != nil {
		return AIAgentVersion{}, err
	}
	if err := s.authorizeScope(ctx, Scope{
		AccountID: accountID, ClientAccountID: effectiveClient(accountID, clientAccountID),
	}); err != nil {
		return AIAgentVersion{}, err
	}
	return s.prompts.CreateAgentVersion(ctx, accountID, actorID, agentID, input)
}

func (s *Service) UpdateAgent(
	ctx context.Context,
	accountID, actorID, id string,
	input AgentPatchInput,
) (AIAgent, error) {
	input.Name = strings.TrimSpace(input.Name)
	if !validUUID(accountID) || !validUUID(id) || input.ExpectedRevision <= 0 ||
		(input.Name == "" && input.Enabled == nil) || len(input.Name) > 200 {
		return AIAgent{}, ErrInvalidInput
	}
	clientAccountID, err := s.prompts.AgentClientScope(ctx, accountID, id)
	if err != nil {
		return AIAgent{}, err
	}
	if err := s.authorizeScope(ctx, Scope{
		AccountID: accountID, ClientAccountID: effectiveClient(accountID, clientAccountID),
	}); err != nil {
		return AIAgent{}, err
	}
	return s.prompts.UpdateAgent(ctx, accountID, actorID, id, input)
}

func (s *Service) PublishAgentVersion(
	ctx context.Context,
	accountID, actorID, versionID string,
) (AIAgentVersion, error) {
	if !validUUID(accountID) || !validUUID(versionID) {
		return AIAgentVersion{}, ErrInvalidInput
	}
	clientAccountID, err := s.prompts.AgentVersionClientScope(ctx, accountID, versionID)
	if err != nil {
		return AIAgentVersion{}, err
	}
	if err := s.authorizeScope(ctx, Scope{
		AccountID: accountID, ClientAccountID: effectiveClient(accountID, clientAccountID),
	}); err != nil {
		return AIAgentVersion{}, err
	}
	return s.prompts.PublishAgentVersion(ctx, accountID, actorID, versionID)
}

func (s *Service) findProcess(
	ctx context.Context,
	key string,
) (ProcessDefinition, error) {
	items, err := s.prompts.ListProcesses(ctx)
	if err != nil {
		return ProcessDefinition{}, err
	}
	for _, item := range items {
		if item.Key == key && item.Status == "registered" {
			return item, nil
		}
	}
	return ProcessDefinition{}, ErrNotFound
}

func validatePrompt(
	content string,
	allowedVariables []string,
	outputSchema json.RawMessage,
) PromptValidation {
	reasons := make([]string, 0)
	content = strings.TrimSpace(content)
	if content == "" {
		reasons = append(reasons, "prompt_empty")
	}
	if len(content) > 200000 {
		reasons = append(reasons, "prompt_too_large")
	}
	if strings.Count(content, "{{") != strings.Count(content, "}}") {
		reasons = append(reasons, "template_unbalanced")
	}
	variables := make([]string, 0)
	for _, match := range templateVarRE.FindAllStringSubmatch(content, -1) {
		variables = append(variables, match[1])
	}
	variables = uniqueSorted(variables)
	allowed := make(map[string]bool, len(allowedVariables))
	for _, key := range allowedVariables {
		allowed[key] = true
	}
	for _, key := range variables {
		root := strings.SplitN(key, ".", 2)[0]
		if !allowed[key] && !allowed[root] {
			reasons = append(reasons, "variable_not_allowed:"+key)
		}
	}
	if !validJSONObject(outputSchema) {
		reasons = append(reasons, "output_schema_invalid")
	} else {
		var schema map[string]any
		_ = json.Unmarshal(outputSchema, &schema)
		if schema["type"] != "object" {
			reasons = append(reasons, "output_schema_not_object")
		}
	}
	sort.Strings(reasons)
	return PromptValidation{
		Valid:       len(reasons) == 0,
		Variables:   variables,
		ReasonCodes: reasons,
	}
}

func credentialDTO(record credentialRecord) AICredential {
	return AICredential{
		ID: record.ID, Provider: record.Provider, Label: record.Label,
		Status: secretbox.Status{
			Set:   record.Status == "active" && record.Ciphertext != "",
			Last4: record.Last4,
		},
		UpdatedAt: record.UpdatedAt,
	}
}

func validJSONArray(raw json.RawMessage) bool {
	raw = normalizedJSON(raw, `[]`)
	var value []json.RawMessage
	return json.Unmarshal(raw, &value) == nil && value != nil
}

func jsonEquivalent(left, right json.RawMessage) bool {
	var l, r any
	if json.Unmarshal(left, &l) != nil || json.Unmarshal(right, &r) != nil {
		return false
	}
	a, _ := json.Marshal(l)
	b, _ := json.Marshal(r)
	return bytes.Equal(a, b)
}

func effectiveClient(accountID, clientAccountID string) string {
	if clientAccountID == "" {
		return accountID
	}
	return clientAccountID
}

func optionalClient(scope Scope) string {
	if scope.ClientAccountID == scope.AccountID {
		return ""
	}
	return scope.ClientAccountID
}
