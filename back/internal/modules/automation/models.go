package automation

import "encoding/json"

// CatalogModel e uma opcao do catalogo global de modelos (provider-agnostico).
// As flags vem do MODELOS.md: modelos de raciocinio (gpt-5*/o-series) exigem a
// Responses API e nao aceitam temperature; visao so funciona em modelos vision_ok.
type CatalogModel struct {
	ID                   string
	Provider             string
	Kind                 string // chat | vision | audio | classifier | embedding
	Label                string
	RequiresResponsesAPI bool
	AcceptsTemperature   bool
	VisionOK             bool
	Enabled              bool
	SortOrder            int
}

// CatalogModelView e a projecao de uma opcao do catalogo para o painel/runtime.
type CatalogModelView struct {
	ID                   string `json:"id"`
	Provider             string `json:"provider"`
	Kind                 string `json:"kind"`
	Label                string `json:"label"`
	RequiresResponsesAPI bool   `json:"requiresResponsesApi"`
	AcceptsTemperature   bool   `json:"acceptsTemperature"`
	VisionOK             bool   `json:"visionOk"`
	SortOrder            int    `json:"sortOrder"`
}

func toCatalogModelView(m CatalogModel) CatalogModelView {
	return CatalogModelView{
		ID:                   m.ID,
		Provider:             m.Provider,
		Kind:                 m.Kind,
		Label:                m.Label,
		RequiresResponsesAPI: m.RequiresResponsesAPI,
		AcceptsTemperature:   m.AcceptsTemperature,
		VisionOK:             m.VisionOK,
		SortOrder:            m.SortOrder,
	}
}

// AutomationModel e o modelo escolhido por automacao + funcao (role).
type AutomationModel struct {
	AutomationID string
	AccountID    string
	Role         string // chat | vision | audio | classifier
	Provider     string
	ModelID      string
	Params       json.RawMessage // temperature, max_tokens, etc.
}

// AutomationModelView e a projecao da selecao por funcao para o painel/runtime.
// As flags do catalogo sao embutidas para o n8n consumir por expression sem um
// segundo lookup (requiresResponsesApi, acceptsTemperature, visionOk).
type AutomationModelView struct {
	Role                 string          `json:"role"`
	Provider             string          `json:"provider"`
	ModelID              string          `json:"modelId"`
	Label                string          `json:"label"`
	RequiresResponsesAPI bool            `json:"requiresResponsesApi"`
	AcceptsTemperature   bool            `json:"acceptsTemperature"`
	VisionOK             bool            `json:"visionOk"`
	Params               json.RawMessage `json:"params"`
}

// ModelsView e a resposta do painel: catalogo de opcoes + selecao atual por funcao.
type ModelsView struct {
	Catalog   []CatalogModelView    `json:"catalog"`
	Selection []AutomationModelView `json:"selection"`
}

// modelRoles sao as funcoes selecionaveis no painel (V1). Embedding/audio sao
// fixos ou ficam para fases posteriores (RAG/Whisper).
var modelRoles = []string{roleChat, roleVision, roleAudio, roleClassifier}

const (
	roleChat       = "chat"
	roleVision     = "vision"
	roleAudio      = "audio"
	roleClassifier = "classifier"
)

// defaultModelForRole devolve o modelo default usado quando a automacao ainda
// nao escolheu nada para aquela funcao (espelha o que o n8n usa hoje).
func defaultModelForRole(role string) (provider, modelID string) {
	if role == roleAudio {
		return "openai", "whisper-1"
	}
	// chat | vision | classifier: gpt-4o-mini (barato, rapido, visao ok).
	return "openai", "gpt-4o-mini"
}
