package cardapio

import "encoding/json"

// DTOs do layout de secoes do site (Fase 3 / Opcao B). Espelham EXATAMENTE o
// contrato do front TAVOLA (app/types/layout.ts): o Omni guarda o SiteLayout como
// jsonb e o serve; o site renderiza. props fica como json.RawMessage (forma livre,
// validada estruturalmente no service — sanitizacao pesada e Fase 4).

// SiteLayout e o layout completo de um site (todas as paginas + tema + barra de aviso).
type SiteLayout struct {
	Pages        map[string]PageLayout `json:"pages"`
	Theme        *ThemeOverrides       `json:"theme,omitempty"`
	Announcement *SiteAnnouncement     `json:"announcement,omitempty"`
	UpdatedAt    string                `json:"updatedAt"`
}

// PageLayout e a lista ordenada de blocos de uma pagina.
type PageLayout struct {
	Page   string        `json:"page"`
	Blocks []LayoutBlock `json:"blocks"`
}

// LayoutBlock e uma instancia de secao no layout (referencia SectionDef.type).
type LayoutBlock struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Props   json.RawMessage `json:"props,omitempty"`
	Visible bool            `json:"visible"`
}

// ThemeOverrides sao os overrides de tema do site (tokens curados).
type ThemeOverrides struct {
	Base   string            `json:"base,omitempty"`
	Mode   string            `json:"mode,omitempty"`
	Accent string            `json:"accent,omitempty"`
	Font   string            `json:"font,omitempty"`
	Tokens map[string]string `json:"tokens,omitempty"`
}

// SiteAnnouncement e a barra de aviso do site (config de nivel de site, irma do
// tema). Espelha SiteAnnouncement do front (app/types/layout.ts): enabled liga a
// faixa; text e a mensagem; link/linkLabel sao opcionais. Round-trip preservado
// no struct (sem ele o json.Marshal de validateSiteLayout descartaria o campo).
type SiteAnnouncement struct {
	Enabled   bool   `json:"enabled"`
	Text      string `json:"text"`
	Link      string `json:"link,omitempty"`
	LinkLabel string `json:"linkLabel,omitempty"`
}
