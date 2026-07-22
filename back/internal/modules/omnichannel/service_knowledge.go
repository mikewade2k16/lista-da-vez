package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func normalizeKnowledgeBaseInput(in *KnowledgeBaseInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 200 {
		return ErrValidation
	}
	if len(in.SearchConfig) == 0 {
		in.SearchConfig = json.RawMessage(`{}`)
	}
	return validateSafeJSONObject(in.SearchConfig, 16000)
}

func normalizeKnowledgeBasePatch(patch *KnowledgeBasePatch) error {
	if patch.Name != nil {
		value := strings.TrimSpace(*patch.Name)
		if value == "" || len([]rune(value)) > 200 {
			return ErrValidation
		}
		patch.Name = &value
	}
	if patch.SearchConfig != nil {
		if err := validateSafeJSONObject(*patch.SearchConfig, 16000); err != nil {
			return err
		}
	}
	return nil
}

func (s *AIService) ListKnowledgeBases(ctx context.Context, accountID string, p auth.Principal) ([]KnowledgeBaseView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return nil, err
	}
	return s.store.ListKnowledgeBases(ctx, accountID)
}

func (s *AIService) CreateKnowledgeBase(ctx context.Context, accountID string, p auth.Principal, in KnowledgeBaseInput) (KnowledgeBaseView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return KnowledgeBaseView{}, err
	}
	if err := normalizeKnowledgeBaseInput(&in); err != nil {
		return KnowledgeBaseView{}, err
	}
	enabled := false
	if in.IsEnabled != nil {
		enabled = *in.IsEnabled
	}
	out, err := s.store.CreateKnowledgeBase(ctx, accountID, in.Name, enabled, in.SearchConfig)
	if isUniqueViolation(err) {
		return KnowledgeBaseView{}, ErrConflict
	}
	return out, err
}

func (s *AIService) UpdateKnowledgeBase(ctx context.Context, accountID string, p auth.Principal, id string, patch KnowledgeBasePatch) (KnowledgeBaseView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return KnowledgeBaseView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(id)) {
		return KnowledgeBaseView{}, ErrValidation
	}
	if err := normalizeKnowledgeBasePatch(&patch); err != nil {
		return KnowledgeBaseView{}, err
	}
	out, err := s.store.UpdateKnowledgeBase(ctx, accountID, id, patch)
	if errorsIsNoRows(err) {
		return KnowledgeBaseView{}, ErrNotFound
	}
	if isUniqueViolation(err) {
		return KnowledgeBaseView{}, ErrConflict
	}
	return out, err
}

func (s *AIService) ListKnowledgeDocuments(ctx context.Context, accountID string, p auth.Principal, baseID string) ([]KnowledgeDocumentView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return nil, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(baseID)) {
		return nil, ErrValidation
	}
	if _, err := s.store.GetKnowledgeBase(ctx, accountID, baseID); err != nil {
		return nil, err
	}
	return s.store.ListKnowledgeDocuments(ctx, accountID, baseID)
}

func normalizeKnowledgeDocumentInput(in *KnowledgeDocumentInput) error {
	in.SourceRef = strings.TrimSpace(in.SourceRef)
	in.Title = strings.TrimSpace(in.Title)
	in.Checksum = strings.TrimSpace(in.Checksum)
	if in.SourceRef == "" || len([]rune(in.SourceRef)) > 1000 || in.Checksum == "" || len([]rune(in.Checksum)) > 128 || len([]rune(in.Title)) > 500 {
		return ErrValidation
	}
	if parsed, err := url.Parse(in.SourceRef); err == nil && parsed.Scheme != "" {
		if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "manual" {
			return ErrValidation
		}
		if parsed.User != nil {
			return ErrValidation
		}
	} else if err != nil {
		return ErrValidation
	}
	if in.Version == 0 {
		in.Version = 1
	}
	if in.Version < 1 || in.Version > 1000000 {
		return ErrValidation
	}
	if len(in.Metadata) == 0 {
		in.Metadata = json.RawMessage(`{}`)
	}
	return validateSafeJSONObject(in.Metadata, 32000)
}

func (s *AIService) CreateKnowledgeDocument(ctx context.Context, accountID string, p auth.Principal, baseID string, in KnowledgeDocumentInput) (KnowledgeDocumentView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return KnowledgeDocumentView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(baseID)) {
		return KnowledgeDocumentView{}, ErrValidation
	}
	if _, err := s.store.GetKnowledgeBase(ctx, accountID, baseID); err != nil {
		return KnowledgeDocumentView{}, err
	}
	if err := normalizeKnowledgeDocumentInput(&in); err != nil {
		return KnowledgeDocumentView{}, err
	}
	out, err := s.store.CreateKnowledgeDocument(ctx, accountID, baseID, in)
	if isUniqueViolation(err) {
		return KnowledgeDocumentView{}, ErrConflict
	}
	return out, err
}

func normalizeKnowledgeDocumentPatch(patch *KnowledgeDocumentPatch) error {
	if patch.Title != nil {
		value := strings.TrimSpace(*patch.Title)
		if len([]rune(value)) > 500 {
			return ErrValidation
		}
		patch.Title = &value
	}
	if patch.Status != nil {
		value := strings.ToLower(strings.TrimSpace(*patch.Status))
		if value != "draft" && value != "processing" && value != "published" && value != "failed" && value != "archived" {
			return ErrValidation
		}
		patch.Status = &value
	}
	if patch.Error != nil && len([]rune(*patch.Error)) > 2000 {
		return ErrValidation
	}
	if patch.Metadata != nil {
		if err := validateSafeJSONObject(*patch.Metadata, 32000); err != nil {
			return err
		}
	}
	return nil
}

func (s *AIService) UpdateKnowledgeDocument(ctx context.Context, accountID string, p auth.Principal, baseID, id string, patch KnowledgeDocumentPatch) (KnowledgeDocumentView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return KnowledgeDocumentView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(baseID)) || !omnichannelUUIDPattern.MatchString(strings.TrimSpace(id)) {
		return KnowledgeDocumentView{}, ErrValidation
	}
	if _, err := s.store.GetKnowledgeDocument(ctx, accountID, baseID, id); err != nil {
		return KnowledgeDocumentView{}, err
	}
	if err := normalizeKnowledgeDocumentPatch(&patch); err != nil {
		return KnowledgeDocumentView{}, err
	}
	if patch.Status != nil && *patch.Status == "published" {
		has, err := s.store.HasKnowledgeChunks(ctx, accountID, id)
		if err != nil {
			return KnowledgeDocumentView{}, err
		}
		if !has {
			return KnowledgeDocumentView{}, ErrConflict
		}
	}
	out, err := s.store.UpdateKnowledgeDocument(ctx, accountID, baseID, id, patch)
	if errorsIsNoRows(err) {
		return KnowledgeDocumentView{}, ErrNotFound
	}
	return out, err
}

func normalizeKnowledgeChunksInput(in *KnowledgeChunksInput) error {
	if len(in.Chunks) == 0 || len(in.Chunks) > 500 {
		return ErrValidation
	}
	seen := make(map[int]struct{}, len(in.Chunks))
	for index := range in.Chunks {
		chunk := &in.Chunks[index]
		chunk.BodyText = strings.TrimSpace(chunk.BodyText)
		if chunk.Ordinal < 0 || chunk.Ordinal > 1000000 || chunk.BodyText == "" || len([]rune(chunk.BodyText)) > 20000 || chunk.TokenCount < 0 || chunk.TokenCount > 10000 {
			return ErrValidation
		}
		if _, exists := seen[chunk.Ordinal]; exists {
			return ErrConflict
		}
		seen[chunk.Ordinal] = struct{}{}
		if chunk.TokenCount == 0 {
			chunk.TokenCount = len(strings.Fields(chunk.BodyText))
		}
	}
	return nil
}

func (s *AIService) ReplaceKnowledgeChunks(ctx context.Context, accountID string, p auth.Principal, baseID, documentID string, in KnowledgeChunksInput) error {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(baseID)) || !omnichannelUUIDPattern.MatchString(strings.TrimSpace(documentID)) {
		return ErrValidation
	}
	if err := normalizeKnowledgeChunksInput(&in); err != nil {
		return err
	}
	return s.store.ReplaceKnowledgeChunks(ctx, accountID, baseID, documentID, in.Chunks)
}

func normalizeKnowledgeBindingInput(in *AIKnowledgeBindingInput) error {
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(in.KnowledgeBaseID)) {
		return ErrValidation
	}
	if in.TopK == 0 {
		in.TopK = 5
	}
	if in.TopK < 1 || in.TopK > 20 || in.MinScore < 0 || in.MinScore > 1 {
		return ErrValidation
	}
	return nil
}

func (s *AIService) ListAIKnowledgeBindings(ctx context.Context, accountID string, p auth.Principal, agentID string) ([]AIKnowledgeBindingView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return nil, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(agentID)) {
		return nil, ErrValidation
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return nil, err
	}
	return s.store.ListAIKnowledgeBindings(ctx, accountID, agentID)
}

func (s *AIService) CreateAIKnowledgeBinding(ctx context.Context, accountID string, p auth.Principal, agentID string, in AIKnowledgeBindingInput) (AIKnowledgeBindingView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AIKnowledgeBindingView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(agentID)) {
		return AIKnowledgeBindingView{}, ErrValidation
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return AIKnowledgeBindingView{}, err
	}
	if err := normalizeKnowledgeBindingInput(&in); err != nil {
		return AIKnowledgeBindingView{}, err
	}
	if _, err := s.store.GetKnowledgeBase(ctx, accountID, in.KnowledgeBaseID); err != nil {
		return AIKnowledgeBindingView{}, err
	}
	enabled := false
	if in.IsEnabled != nil {
		enabled = *in.IsEnabled
	}
	out, err := s.store.CreateAIKnowledgeBinding(ctx, accountID, agentID, in, enabled)
	if isUniqueViolation(err) {
		return AIKnowledgeBindingView{}, ErrConflict
	}
	return out, err
}

func (s *AIService) UpdateAIKnowledgeBinding(ctx context.Context, accountID string, p auth.Principal, agentID, id string, patch AIKnowledgeBindingPatch) (AIKnowledgeBindingView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AIKnowledgeBindingView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(agentID)) || !omnichannelUUIDPattern.MatchString(strings.TrimSpace(id)) {
		return AIKnowledgeBindingView{}, ErrValidation
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return AIKnowledgeBindingView{}, err
	}
	if patch.TopK != nil && (*patch.TopK < 1 || *patch.TopK > 20) || patch.MinScore != nil && (*patch.MinScore < 0 || *patch.MinScore > 1) {
		return AIKnowledgeBindingView{}, ErrValidation
	}
	out, err := s.store.UpdateAIKnowledgeBinding(ctx, accountID, agentID, id, patch)
	if errorsIsNoRows(err) {
		return AIKnowledgeBindingView{}, ErrNotFound
	}
	return out, err
}

func (s *AIService) DeleteAIKnowledgeBinding(ctx context.Context, accountID string, p auth.Principal, agentID, id string) error {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(agentID)) || !omnichannelUUIDPattern.MatchString(strings.TrimSpace(id)) {
		return ErrValidation
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return err
	}
	return s.store.DisableAIKnowledgeBinding(ctx, accountID, agentID, id)
}

func (s *AIService) SearchKnowledge(ctx context.Context, accountID string, p auth.Principal, agentID string, in KnowledgeSearchInput) ([]KnowledgeSearchResult, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return nil, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(agentID)) {
		return nil, ErrValidation
	}
	if _, err := s.assertAgentScope(ctx, accountID, agentID); err != nil {
		return nil, err
	}
	in.Query = strings.TrimSpace(in.Query)
	if in.Query == "" || len([]rune(in.Query)) > 500 {
		return nil, ErrValidation
	}
	if in.TopK == 0 {
		in.TopK = 5
	}
	if in.TopK < 1 || in.TopK > 20 || in.MinScore < 0 || in.MinScore > 1 {
		return nil, ErrValidation
	}
	return s.store.SearchKnowledge(ctx, accountID, agentID, in.Query, in.TopK, in.MinScore)
}

func errorsIsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
