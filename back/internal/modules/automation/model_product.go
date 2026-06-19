package automation

// SourcesView e a config de fontes de produto por automacao (M5), persistida no
// settings jsonb da automacao default da account (sem migration nova). Contrato do
// painel: GET/PUT /v1/automation/sources.
type SourcesView struct {
	CatalogEnabled bool     `json:"catalogEnabled"`
	SiteURLs       []string `json:"siteUrls"`
}

// ProductHit e a projecao lean de um produto retornado pela tool de catalogo (M5)
// para o runtime (n8n). Base = site.products (lista + imagem), ENRIQUECIDO pelo ERP
// via o codigo (nome real, preco e marca): o site.products costuma vir com nome
// generico e preco 0; o ERP (erp_item_current, por sku == code) tem o dado bom.
type ProductHit struct {
	Name  string  `json:"name"`
	Code  string  `json:"code"`
	Price float64 `json:"price"`
	Brand string  `json:"brand,omitempty"`
	Image string  `json:"image,omitempty"`
}

// sourcesSettingsKey e a chave dentro de automation.automations.settings jsonb onde
// a config de fontes vive. Mantida estavel para nao colidir com outras chaves.
const sourcesSettingsKey = "sources"
