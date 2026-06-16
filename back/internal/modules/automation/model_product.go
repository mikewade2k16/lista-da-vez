package automation

// SourcesView e a config de fontes de produto por automacao (M5), persistida no
// settings jsonb da automacao default da account (sem migration nova). Contrato do
// painel: GET/PUT /v1/automation/sources.
type SourcesView struct {
	CatalogEnabled bool     `json:"catalogEnabled"`
	SiteURLs       []string `json:"siteUrls"`
}

// ProductHit e a projecao lean de um produto retornado pela tool de catalogo (M5)
// para o runtime (n8n). So os campos que o bot precisa para responder ao cliente.
type ProductHit struct {
	Name  string  `json:"name"`
	Code  string  `json:"code"`
	Price float64 `json:"price"`
}

// sourcesSettingsKey e a chave dentro de automation.automations.settings jsonb onde
// a config de fontes vive. Mantida estavel para nao colidir com outras chaves.
const sourcesSettingsKey = "sources"
