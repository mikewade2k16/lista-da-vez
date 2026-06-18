package core

// platform_settings_model.go — tipos da config GLOBAL da plataforma
// (core.platform_settings). A primeira chave suportada é 'menu_layout': a
// organização do menu (header vs sidebar), definida por platform_admin e lida
// por todos os usuários autenticados. Config de NÍVEL PLATAFORMA, não per-account
// nem per-user.

// menuLayoutKey é a chave singleton em core.platform_settings que guarda o
// layout do menu.
const menuLayoutKey = "menu_layout"

// Placements válidos para um item de navegação. O service rejeita qualquer
// valor fora deste conjunto (400/validation).
const (
	PlacementHeader  = "header"
	PlacementSidebar = "sidebar"
	PlacementBoth    = "both"
	PlacementHidden  = "hidden"
)

// validPlacements é o set dos placements aceitos. Usado na validação do service.
var validPlacements = map[string]struct{}{
	PlacementHeader:  {},
	PlacementSidebar: {},
	PlacementBoth:    {},
	PlacementHidden:  {},
}

// isValidPlacement informa se p é um placement reconhecido.
func isValidPlacement(p string) bool {
	_, ok := validPlacements[p]
	return ok
}

// MenuLayout é o documento de layout do menu persistido como jsonb sob a chave
// 'menu_layout'. version permite evoluir o schema sem quebrar leitores antigos.
type MenuLayout struct {
	Version  int                       `json:"version"`
	Sections []MenuLayoutSection       `json:"sections"`
	Items    map[string]MenuLayoutItem `json:"items"`
}

// MenuLayoutSection descreve a ordem de uma seção do menu.
type MenuLayoutSection struct {
	ID    string `json:"id"`
	Order int    `json:"order"`
}

// MenuLayoutItem descreve onde e em que ordem um item de navegação aparece.
// Placement ∈ {header, sidebar, both, hidden}.
type MenuLayoutItem struct {
	Placement string `json:"placement"`
	Order     int    `json:"order"`
}

// MenuLayoutResponse é o shape de resposta do GET e do PATCH de
// /v1/platform/menu-layout. UpdatedAt (RFC3339) e UpdatedBy (userID) são nil
// quando a config ainda não foi escrita (default vazio).
type MenuLayoutResponse struct {
	Layout    MenuLayout `json:"layout"`
	UpdatedAt *string    `json:"updatedAt"`
	UpdatedBy *string    `json:"updatedBy"`
}

// defaultMenuLayout é o layout retornado quando não há linha persistida ainda.
func defaultMenuLayout() MenuLayout {
	return MenuLayout{
		Version:  1,
		Sections: []MenuLayoutSection{},
		Items:    map[string]MenuLayoutItem{},
	}
}
