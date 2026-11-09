package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/knowledge"
)

func bridgeKnowledgeError(e error) error {
	if errors.Is(e, knowledge.ErrNotFound) {
		return ErrNotFound
	}
	if errors.Is(e, knowledge.ErrConflict) {
		return ErrConflict
	}
	return e
}
func (s *Store) ListKnowledgeBases(ctx context.Context, accountID string) ([]KnowledgeBaseView, error) {
	v, e := knowledge.NewStore(s.pool).ListKnowledgeBases(ctx, accountID)
	return v, bridgeKnowledgeError(e)
}
func (s *Store) CreateKnowledgeBase(ctx context.Context, accountID, name string, enabled bool, config json.RawMessage) (KnowledgeBaseView, error) {
	v, e := knowledge.NewStore(s.pool).CreateKnowledgeBase(ctx, accountID, name, enabled, config)
	return v, bridgeKnowledgeError(e)
}
func (s *Store) UpdateKnowledgeBase(ctx context.Context, accountID, id string, patch KnowledgeBasePatch) (KnowledgeBaseView, error) {
	v, e := knowledge.NewStore(s.pool).UpdateKnowledgeBase(ctx, accountID, id, patch)
	return v, bridgeKnowledgeError(e)
}
func (s *Store) GetKnowledgeBase(ctx context.Context, accountID, id string) (KnowledgeBaseView, error) {
	v, e := knowledge.NewStore(s.pool).GetKnowledgeBase(ctx, accountID, id)
	return v, bridgeKnowledgeError(e)
}
func (s *Store) ListKnowledgeDocuments(ctx context.Context, accountID, baseID string) ([]KnowledgeDocumentView, error) {
	v, e := knowledge.NewStore(s.pool).ListKnowledgeDocuments(ctx, accountID, baseID)
	return v, bridgeKnowledgeError(e)
}
func (s *Store) GetKnowledgeDocument(ctx context.Context, accountID, baseID, documentID string) (KnowledgeDocumentView, error) {
	v, e := knowledge.NewStore(s.pool).GetKnowledgeDocument(ctx, accountID, baseID, documentID)
	return v, bridgeKnowledgeError(e)
}
func (s *Store) CreateKnowledgeDocument(ctx context.Context, accountID, baseID string, in KnowledgeDocumentInput) (KnowledgeDocumentView, error) {
	v, e := knowledge.NewStore(s.pool).CreateKnowledgeDocument(ctx, accountID, baseID, in)
	return v, bridgeKnowledgeError(e)
}
func (s *Store) UpdateKnowledgeDocument(ctx context.Context, accountID, baseID, id string, patch KnowledgeDocumentPatch) (KnowledgeDocumentView, error) {
	v, e := knowledge.NewStore(s.pool).UpdateKnowledgeDocument(ctx, accountID, baseID, id, patch)
	return v, bridgeKnowledgeError(e)
}
func (s *Store) HasKnowledgeChunks(ctx context.Context, accountID, documentID string) (bool, error) {
	v, e := knowledge.NewStore(s.pool).HasKnowledgeChunks(ctx, accountID, documentID)
	return v, bridgeKnowledgeError(e)
}
func (s *Store) SearchKnowledge(ctx context.Context, accountID, agentID, query string, topK int, minScore float64) ([]KnowledgeSearchResult, error) {
	v, e := knowledge.NewStore(s.pool).SearchOmnichannel(ctx, accountID, agentID, query, topK, minScore)
	return v, bridgeKnowledgeError(e)
}
func (s *Store) ReplaceKnowledgeChunks(ctx context.Context, accountID, baseID, documentID string, chunks []KnowledgeChunkInput) error {
	return bridgeKnowledgeError(knowledge.NewStore(s.pool).ReplaceKnowledgeChunks(ctx, accountID, baseID, documentID, chunks))
}
