package omnichannel

import (
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/knowledge"
	"time"
)

type KnowledgeBaseView = knowledge.KnowledgeBaseView
type KnowledgeBaseInput = knowledge.KnowledgeBaseInput
type KnowledgeBasePatch = knowledge.KnowledgeBasePatch
type KnowledgeDocumentView = knowledge.KnowledgeDocumentView
type KnowledgeDocumentInput = knowledge.KnowledgeDocumentInput
type KnowledgeDocumentPatch = knowledge.KnowledgeDocumentPatch
type KnowledgeChunkInput = knowledge.KnowledgeChunkInput
type KnowledgeChunksInput = knowledge.KnowledgeChunksInput
type AIKnowledgeBindingView struct {
	ID              string    `json:"id"`
	AgentID         string    `json:"agentId"`
	KnowledgeBaseID string    `json:"knowledgeBaseId"`
	BaseName        string    `json:"baseName"`
	IsEnabled       bool      `json:"isEnabled"`
	TopK            int       `json:"topK"`
	MinScore        float64   `json:"minScore"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type AIKnowledgeBindingInput struct {
	KnowledgeBaseID string  `json:"knowledgeBaseId"`
	IsEnabled       *bool   `json:"isEnabled"`
	TopK            int     `json:"topK"`
	MinScore        float64 `json:"minScore"`
}

type AIKnowledgeBindingPatch struct {
	IsEnabled *bool    `json:"isEnabled"`
	TopK      *int     `json:"topK"`
	MinScore  *float64 `json:"minScore"`
}

type KnowledgeSearchInput struct {
	Query    string  `json:"query"`
	TopK     int     `json:"topK"`
	MinScore float64 `json:"minScore"`
}

type KnowledgeSearchResult = knowledge.KnowledgeSearchResult
