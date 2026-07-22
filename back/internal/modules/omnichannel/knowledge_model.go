package omnichannel

import (
	"encoding/json"
	"time"
)

type KnowledgeBaseView struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	IsEnabled    bool            `json:"isEnabled"`
	SearchConfig json.RawMessage `json:"searchConfig"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

type KnowledgeBaseInput struct {
	Name         string          `json:"name"`
	IsEnabled    *bool           `json:"isEnabled"`
	SearchConfig json.RawMessage `json:"searchConfig"`
}

type KnowledgeBasePatch struct {
	Name         *string          `json:"name"`
	IsEnabled    *bool            `json:"isEnabled"`
	SearchConfig *json.RawMessage `json:"searchConfig"`
}

type KnowledgeDocumentView struct {
	ID              string          `json:"id"`
	KnowledgeBaseID string          `json:"knowledgeBaseId"`
	SourceRef       string          `json:"sourceRef"`
	Title           string          `json:"title"`
	Checksum        string          `json:"checksum"`
	Status          string          `json:"status"`
	Version         int             `json:"version"`
	ChunkCount      int             `json:"chunkCount"`
	Metadata        json.RawMessage `json:"metadata"`
	Error           string          `json:"error"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

type KnowledgeDocumentInput struct {
	SourceRef string          `json:"sourceRef"`
	Title     string          `json:"title"`
	Checksum  string          `json:"checksum"`
	Version   int             `json:"version"`
	Metadata  json.RawMessage `json:"metadata"`
}

type KnowledgeDocumentPatch struct {
	Title    *string          `json:"title"`
	Status   *string          `json:"status"`
	Metadata *json.RawMessage `json:"metadata"`
	Error    *string          `json:"error"`
}

type KnowledgeChunkInput struct {
	Ordinal    int    `json:"ordinal"`
	BodyText   string `json:"bodyText"`
	TokenCount int    `json:"tokenCount"`
}

type KnowledgeChunksInput struct {
	Chunks []KnowledgeChunkInput `json:"chunks"`
}

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

type KnowledgeSearchResult struct {
	DocumentID string  `json:"documentId"`
	Title      string  `json:"title"`
	ChunkID    string  `json:"chunkId"`
	Excerpt    string  `json:"excerpt"`
	Score      float64 `json:"score"`
	SourceRef  string  `json:"sourceRef"`
	Version    int     `json:"version"`
}
