package omnichannel

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// ============================================================================
// F9 — Management do agente de IA: CRUD + publish/rollback + campos + runs + simulate
// ============================================================================
//
// A F10 CONSOME estas rotas (nao recria). Permissao gateia FEATURE (agents.manage /
// audit.view => 403 se falta); escopo/recurso de outra conta => 404 (nunca 403,
// enumeration). account_id vem SEMPRE do Principal. A chave do provider so entra/sai
// mascarada ({set,last4}); a chave crua nunca volta ao front nem vai a log.

// ============================================================================
// Autorizacao (permissao gateia feature)
// ============================================================================

// requireAgentPerm exige uma permissao efetiva NA CONTA (403 se faltar). platform_admin passa;
// tambem cai na lista global do Principal quando resolvida. Mesmo padrao de service_transition.
func (s *AIService) requireAgentPerm(ctx context.Context, accountID string, p auth.Principal, key string) error {
	if strings.TrimSpace(accountID) == "" {
		return ErrForbidden
	}
	if p.Role == auth.RolePlatformAdmin {
		return nil
	}
	ok, err := s.store.hasEffectivePermission(ctx, accountID, p.UserID, key)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	if p.PermissionsResolved && containsPermission(p.Permissions, key) {
		return nil
	}
	return ErrForbidden
}

// assertAgentScope garante que o agente existe e e da conta (senao 404). Devolve a linha crua
// para os fluxos que ja precisam dela (evita um segundo fetch).
func (s *AIService) assertAgentScope(ctx context.Context, accountID, agentID string) (agentRow, error) {
	agent, err := s.store.GetAgent(ctx, accountID, agentID)
	if err != nil {
		return agentRow{}, translate(err)
	}
	return agent, nil
}

// ============================================================================
// Views
// ============================================================================

// agentView projeta a linha crua, MASCARANDO a chave do provider ({set,last4}).
func agentView(a agentRow) AIAgentView {
	return AIAgentView{
		ID:              a.ID,
		Slug:            a.Slug,
		Name:            a.Name,
		Enabled:         a.Enabled,
		ActiveVersionID: a.ActiveVersionID,
		ProviderKey:     secretbox.Status{Set: a.ProviderKeyCipher != "", Last4: a.ProviderKeyLast4},
		CreatedAt:       a.CreatedAt,
		UpdatedAt:       a.UpdatedAt,
	}
}

func versionView(v versionRow) AIAgentVersionView {
	return AIAgentVersionView{
		ID: v.ID, AgentID: v.AgentID, Version: v.Version, Status: v.Status,
		Provider: v.Provider, Model: v.Model, Temperature: v.Temperature,
		Layers: jsonOrEmpty(v.Layers), OutputSchema: jsonOrEmpty(v.OutputSchema),
		MediaConfig:   jsonOrEmpty(v.MediaConfig),
		SchemaVersion: v.SchemaVersion, DebounceMS: v.DebounceMS,
		MaxContextMessages: v.MaxContextMessages, MaxAITurns: v.MaxAITurns,
		MinConfidence: v.MinConfidence, HandoffOnError: v.HandoffOnError,
		HandoffOnLimit: v.HandoffOnLimit, WorkflowContract: v.WorkflowContract,
		PublishedAt: v.PublishedAt, CreatedAt: v.CreatedAt,
	}
}

// jsonOrEmpty garante que um jsonb nulo/vazio saia como {} (o front tipa objeto, nao null).
func jsonOrEmpty(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{}`)
	}
	return raw
}

// ============================================================================
// Agentes
// ============================================================================

func (s *AIService) ListAgents(ctx context.Context, accountID string, p auth.Principal) ([]AIAgentView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return nil, err
	}
	rows, err := s.store.ListAgents(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]AIAgentView, 0, len(rows))
	for _, a := range rows {
		out = append(out, agentView(a))
	}
	return out, nil
}

func (s *AIService) CreateAgent(ctx context.Context, accountID string, p auth.Principal, in AIAgentInput) (AIAgentView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AIAgentView{}, err
	}
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		slug = slugify(in.Name)
	}
	if slug == "" {
		return AIAgentView{}, ErrValidation
	}
	row, err := s.store.CreateAgent(ctx, accountID, slug, strings.TrimSpace(in.Name), in.Enabled, p.UserID)
	if err != nil {
		if isUniqueViolation(err) {
			return AIAgentView{}, ErrConflict
		}
		return AIAgentView{}, err
	}
	return agentView(row), nil
}

func (s *AIService) GetAgent(ctx context.Context, accountID string, p auth.Principal, id string) (AIAgentView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AIAgentView{}, err
	}
	agent, err := s.assertAgentScope(ctx, accountID, id)
	if err != nil {
		return AIAgentView{}, err
	}
	return agentView(agent), nil
}

// UpdateAgent aplica o patch. providerKey nao-nil grava/limpa a chave CIFRADA (secretbox);
// vazio limpa. A chave crua nunca e logada nem devolvida — so o {set,last4} da view.
func (s *AIService) UpdateAgent(ctx context.Context, accountID string, p auth.Principal, id string, patch AIAgentPatch) (AIAgentView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AIAgentView{}, err
	}
	agent, err := s.assertAgentScope(ctx, accountID, id)
	if err != nil {
		return AIAgentView{}, err
	}
	sp := agentPatch{Name: patch.Name, Enabled: patch.Enabled}
	if patch.ProviderKey != nil {
		activeProvider, err := s.store.AgentActiveProvider(ctx, accountID, id)
		if err != nil {
			return AIAgentView{}, err
		}
		keys, err := s.decodeProviderKeyring(agent.ProviderKeyCipher, activeProvider)
		if err != nil {
			return AIAgentView{}, err
		}
		value := strings.TrimSpace(*patch.ProviderKey)
		if provider := normalizeAIProviderKeyID(activeProvider); provider != "" {
			if value == "" {
				delete(keys, provider)
			} else {
				keys[provider] = value
			}
		} else if value != "" {
			return AIAgentView{}, ErrAIProviderUnsupported
		}
		cipher, last4, err := s.encryptProviderKeyring(keys)
		if err != nil {
			return AIAgentView{}, err
		}
		sp.ProviderKeyCiph = &cipher
		sp.ProviderKeyLast4 = &last4
	}
	row, err := s.store.UpdateAgent(ctx, accountID, id, sp)
	if err != nil {
		return AIAgentView{}, translate(err)
	}
	return agentView(row), nil
}

// ============================================================================
// Versoes (publish/rollback) — publicada e IMUTAVEL
// ============================================================================

func (s *AIService) ListVersions(ctx context.Context, accountID string, p auth.Principal, agentID string) ([]AIAgentVersionView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return nil, err
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return nil, err
	}
	rows, err := s.store.ListVersions(ctx, accountID, agentID)
	if err != nil {
		return nil, err
	}
	out := make([]AIAgentVersionView, 0, len(rows))
	for _, v := range rows {
		out = append(out, versionView(v))
	}
	return out, nil
}

// CreateVersion cria um DRAFT. outputSchema/layers vazios => defaults (schema C9.3, layers {}).
func (s *AIService) CreateVersion(ctx context.Context, accountID string, p auth.Principal, agentID string, in AIVersionInput) (AIAgentVersionView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AIAgentVersionView{}, err
	}
	normalized, schema, layers, err := normalizeVersionInput(in)
	if err != nil {
		return AIAgentVersionView{}, err
	}
	row, err := s.store.CreateVersion(ctx, accountID, agentID, normalized, schema, layers)
	if err != nil {
		return AIAgentVersionView{}, translate(err) // agente fora de escopo => ErrNoRows -> 404
	}
	return versionView(row), nil
}

// SaveConfiguration e o caminho simples do MVP: persiste e ativa a configuracao no mesmo
// commit. O versionamento continua interno para auditoria/rollback, sem etapa de rascunho na UI.
func (s *AIService) SaveConfiguration(ctx context.Context, accountID string, p auth.Principal, agentID string, in AIVersionInput) (AIAgentVersionView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AIAgentVersionView{}, err
	}
	normalized, schema, layers, err := normalizeVersionInput(in)
	if err != nil {
		return AIAgentVersionView{}, err
	}
	row, err := s.store.SavePublishedVersion(ctx, accountID, agentID, normalized, schema, layers, p.UserID)
	if err != nil {
		return AIAgentVersionView{}, translate(err)
	}
	return versionView(row), nil
}

func normalizeVersionInput(in AIVersionInput) (AIVersionInput, json.RawMessage, json.RawMessage, error) {
	if strings.TrimSpace(in.Provider) == "" || strings.TrimSpace(in.Model) == "" {
		return AIVersionInput{}, nil, nil, ErrValidation
	}
	schema := in.OutputSchema
	if len(schema) == 0 || string(schema) == "null" || string(schema) == "{}" {
		schema = defaultOutputSchema()
	}
	layers := jsonOrEmpty(in.Layers)
	mediaConfig, err := normalizeMediaConfig(in.MediaConfig)
	if err != nil {
		return AIVersionInput{}, nil, nil, err
	}
	in.MediaConfig = mediaConfig
	if strings.TrimSpace(in.SchemaVersion) == "" {
		in.SchemaVersion = "v1"
	}
	if in.DebounceMS == 0 {
		in.DebounceMS = 2500
	}
	if in.MaxContextMessages == 0 {
		in.MaxContextMessages = 30
	}
	if in.MaxAITurns == 0 {
		in.MaxAITurns = 6
	}
	if in.MinConfidence == nil {
		value := 0.650
		in.MinConfidence = &value
	}
	if in.WorkflowContract == "" {
		in.WorkflowContract = "brain.v2"
	}
	if in.DebounceMS < 500 || in.DebounceMS > 15000 || in.MaxContextMessages < 1 || in.MaxContextMessages > 100 ||
		in.MaxAITurns < 1 || in.MaxAITurns > 20 || *in.MinConfidence < 0 || *in.MinConfidence > 1 ||
		(in.WorkflowContract != "brain.v2" && in.WorkflowContract != "brain.v3") {
		return AIVersionInput{}, nil, nil, ErrValidation
	}
	return in, schema, layers, nil
}

func (s *AIService) PublishVersion(ctx context.Context, accountID string, p auth.Principal, agentID string, version int) (AIAgentView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AIAgentView{}, err
	}
	row, err := s.store.PublishVersion(ctx, accountID, agentID, version, p.UserID)
	if err != nil {
		return AIAgentView{}, translate(err)
	}
	return agentView(row), nil
}

func (s *AIService) Rollback(ctx context.Context, accountID string, p auth.Principal, agentID string, in RollbackInput) (AIAgentView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AIAgentView{}, err
	}
	if strings.TrimSpace(in.VersionID) == "" {
		return AIAgentView{}, ErrValidation
	}
	row, err := s.store.RollbackAgent(ctx, accountID, agentID, in.VersionID)
	if err != nil {
		return AIAgentView{}, translate(err)
	}
	return agentView(row), nil
}

// ============================================================================
// Campos a coletar
// ============================================================================

func (s *AIService) ListCollectFields(ctx context.Context, accountID string, p auth.Principal, agentID string) ([]CollectFieldView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return nil, err
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return nil, err
	}
	return s.store.ListCollectFields(ctx, accountID, agentID)
}

func (s *AIService) CreateCollectField(ctx context.Context, accountID string, p auth.Principal, agentID string, in CollectFieldInput) (CollectFieldView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return CollectFieldView{}, err
	}
	if strings.TrimSpace(in.Key) == "" {
		return CollectFieldView{}, ErrValidation
	}
	row, err := s.store.CreateCollectField(ctx, accountID, agentID, in, jsonOrEmptyArray(in.EnumOptions))
	if err != nil {
		if isUniqueViolation(err) {
			return CollectFieldView{}, ErrConflict
		}
		return CollectFieldView{}, translate(err)
	}
	return row, nil
}

func (s *AIService) UpdateCollectField(ctx context.Context, accountID string, p auth.Principal, agentID, fieldID string, patch CollectFieldPatch) (CollectFieldView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return CollectFieldView{}, err
	}
	row, err := s.store.UpdateCollectField(ctx, accountID, agentID, fieldID, patch)
	if err != nil {
		return CollectFieldView{}, translate(err)
	}
	return row, nil
}

func (s *AIService) DeleteCollectField(ctx context.Context, accountID string, p auth.Principal, agentID, fieldID string) error {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return err
	}
	return s.store.DeleteCollectField(ctx, accountID, agentID, fieldID)
}

// jsonOrEmptyArray garante que enum_options saia como [] quando vazio/nulo.
func jsonOrEmptyArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`[]`)
	}
	return raw
}

// ============================================================================
// Trilha de runs (audit.view)
// ============================================================================

func (s *AIService) ListRuns(ctx context.Context, accountID string, p auth.Principal, agentID string, limit int, beforeID string) ([]AIRunView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.audit.view"); err != nil {
		return nil, err
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return nil, err
	}
	return s.store.ListRuns(ctx, accountID, agentID, normalizeRunLimit(limit), beforeID)
}

// normalizeRunLimit aplica 1..200 default 50 (C9.5).
func normalizeRunLimit(limit int) int {
	switch {
	case limit <= 0:
		return 50
	case limit > 200:
		return 200
	default:
		return limit
	}
}
