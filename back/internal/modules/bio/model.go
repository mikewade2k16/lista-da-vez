package bio

import (
	"encoding/json"
	"time"
)

// Bio e uma pagina de link-in-bio de uma account. O conteudo vive em dois
// jsonb: data_draft (edicao no painel) e data_published (o que o publico ve).
// O merge com bio.defaults acontece na hora de servir (service/merge.go).
type Bio struct {
	ID            string
	AccountID     string
	AccountName   string // join core.accounts (so na projecao de leitura)
	Slug          string
	Name          string
	Status        string
	DataDraft     json.RawMessage
	DataPublished json.RawMessage // nil enquanto nunca publicada
	PublishedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// BioSummary e a projecao lean para a listagem do painel. NUNCA carrega jsonb
// (data_draft/data_published) — esses so vem no GET por id.
type BioSummary struct {
	ID          string     `json:"id"`
	AccountID   string     `json:"accountId"`
	AccountName string     `json:"accountName"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
}

// BioView e o detalhe completo de uma bio (GET por id) — inclui os dois jsonb.
type BioView struct {
	ID            string          `json:"id"`
	AccountID     string          `json:"accountId"`
	AccountName   string          `json:"accountName"`
	Slug          string          `json:"slug"`
	Name          string          `json:"name"`
	Status        string          `json:"status"`
	DataDraft     json.RawMessage `json:"dataDraft"`
	DataPublished json.RawMessage `json:"dataPublished,omitempty"`
	PublishedAt   *time.Time      `json:"publishedAt,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

func (b Bio) summary() BioSummary {
	return BioSummary{
		ID:          b.ID,
		AccountID:   b.AccountID,
		AccountName: b.AccountName,
		Slug:        b.Slug,
		Name:        b.Name,
		Status:      b.Status,
		UpdatedAt:   b.UpdatedAt,
		PublishedAt: b.PublishedAt,
	}
}

func (b Bio) view() BioView {
	return BioView{
		ID:            b.ID,
		AccountID:     b.AccountID,
		AccountName:   b.AccountName,
		Slug:          b.Slug,
		Name:          b.Name,
		Status:        b.Status,
		DataDraft:     normalizeRaw(b.DataDraft),
		DataPublished: b.DataPublished,
		PublishedAt:   b.PublishedAt,
		CreatedAt:     b.CreatedAt,
		UpdatedAt:     b.UpdatedAt,
	}
}

// normalizeRaw garante "{}" no lugar de nil para o draft (sempre um objeto).
func normalizeRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	return raw
}

// BioDefaults e a linha global bio.defaults (equivalente ao _default.json do
// front bio). O merge usa esse data como base.
type BioDefaults struct {
	Data      json.RawMessage `json:"data"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// Media e o metadado de um arquivo enviado (bio.media).
type Media struct {
	ID        string
	AccountID string
	BioID     *string
	Kind      string
	Path      string
	Mime      *string
	SizeBytes *int64
	CreatedAt time.Time
}

// ListFilter sao os filtros aceitos na listagem (todos opcionais; accountID e
// validado contra o Principal no service).
type ListFilter struct {
	AccountID string
	Status    string
	Q         string
}

// CreateRequest e o body de POST /v1/bio/bios.
type CreateRequest struct {
	AccountID string `json:"accountId"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
}

// PatchRequest e o body de PATCH /v1/bio/bios/{id}. Campos nil = nao alterar.
// DataDraft, quando presente, substitui o draft inteiro. AccountID move a bio
// para outra account e e honrado APENAS para platform_admin (nao-admin ignorado).
type PatchRequest struct {
	Name      *string          `json:"name"`
	Slug      *string          `json:"slug"`
	DataDraft *json.RawMessage `json:"dataDraft"`
	AccountID *string          `json:"accountId"`
}

// DuplicateRequest e o body de POST /v1/bio/bios/{id}/duplicate. AccountID e
// opcional e honrado apenas para platform_admin (cria a copia em outra account);
// vazio = mesma account da origem.
type DuplicateRequest struct {
	AccountID string `json:"accountId"`
}

// MediaView e a resposta do upload de midia.
type MediaView struct {
	URL string `json:"url"`
}

// ============================================================================
// B7 — Fonte de produtos plugavel
// ============================================================================

// Opcoes de tipo de fonte do slideTop.
const (
	SourceTypeManual       = "manual"        // slides a mao (default/retrocompat)
	SourceTypeSiteProducts = "site_products" // 1a fonte real: schema site.products
)

// Opcoes de link de cada slide-produto (D6: configuravel no editor).
const (
	ProductLinkProduct  = "product"  // link do produto no site do cliente
	ProductLinkWhatsApp = "whatsapp" // abre o WhatsApp da bio/lightbox
	ProductLinkNone     = "none"     // sem link
)

// SourceInfo descreve uma fonte de produtos disponivel para a account
// (GET /v1/bio/sources). MVP devolve apenas site_products.
type SourceInfo struct {
	Type      string `json:"type"`
	Label     string `json:"label"`
	Available bool   `json:"available"`
}

// SourceFacets sao os valores distintos de uma fonte usados para popular os
// selects do editor (GET /v1/bio/sources/{type}/facets). Sempre arrays nao-nil.
type SourceFacets struct {
	Categories []string `json:"categories"`
	Campaigns  []string `json:"campaigns"`
	Tipos      []string `json:"tipos"`
}

// SourceFilter sao os criterios de resolucao de uma fonte (subconjunto do
// slideTop.source). Campos vazios = sem filtro. Limit 0 = todos; senao N.
type SourceFilter struct {
	Category  string
	Campaigns []string
	Tipo      string
	Limit     int
}

// ResolvedSlide e um slide ja pronto para injetar em slideTop.slides. O href e
// resolvido conforme a opcao de link do source (produto/whatsapp/nenhum) — pode
// vir vazio (sem link). Desc/Price vem do produto (descricao e preco formatado)
// para o Lightbox do front exibir, espelhando o BioSlide manual.
type ResolvedSlide struct {
	Src   string `json:"src"`
	Title string `json:"title,omitempty"`
	Desc  string `json:"desc,omitempty"`
	Price string `json:"price,omitempty"`
	Href  string `json:"href,omitempty"`
}

// slideTopSource e o shape do slideTop.source dentro do BioData (jsonb). O painel
// preenche; o back so precisa entender o suficiente para resolver a fonte.
//
//	{ type, category?, campaigns?[], tipo?, limit?, link? }
type slideTopSource struct {
	Type      string   `json:"type"`
	Category  string   `json:"category,omitempty"`
	Campaigns []string `json:"campaigns,omitempty"`
	Tipo      string   `json:"tipo,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	Link      string   `json:"link,omitempty"`
}
