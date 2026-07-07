package core

import "regexp"

// platform_appearance_model.go — aparência GLOBAL da plataforma (tema visual),
// persistida em core.platform_settings sob a chave 'appearance'. É config de
// NÍVEL PLATAFORMA (como 'menu_layout'), NÃO por account nem por módulo — saiu
// do acoplamento com queue/settings, que era o motivo do tema/Page Headers não
// persistir para platform_admin (tenant vazio). Escrita só por platform_admin;
// leitura por qualquer usuário autenticado (é o que pinta o painel).

// appearanceKey é a chave singleton em core.platform_settings que guarda a
// aparência global.
const appearanceKey = "appearance"

// defaultActiveTheme é o tema aplicado quando ainda não há aparência gravada.
const defaultActiveTheme = "dark"

// defaultCustomThemeName é o rótulo do tema custom quando não definido.
const defaultCustomThemeName = "Custom"

// themeSlugPattern valida o nome do tema ativo. NÃO há enum fechado de temas: o
// catálogo de temas vive no FRONT (fonte de verdade de QUAIS temas existem);
// aqui só garantimos um slug bem-formado, para adicionar um tema novo (ex.:
// liquidglass) não exigir deploy do back.
var themeSlugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)

// isValidThemeSlug informa se s é um slug de tema bem-formado.
func isValidThemeSlug(s string) bool {
	return themeSlugPattern.MatchString(s)
}

// Appearance é o documento de aparência persistido como jsonb sob 'appearance'.
// Version permite evoluir o schema sem quebrar leitores antigos. Overrides são
// por tema → (variável CSS → valor); espelha o snapshot que o front monta.
type Appearance struct {
	Version         int                          `json:"version"`
	ActiveTheme     string                       `json:"activeTheme"`
	CustomThemeName string                       `json:"customThemeName"`
	Overrides       map[string]map[string]string `json:"overrides"`
}

// AppearanceResponse é o shape de resposta do GET e do PUT de
// /v1/platform/appearance. UpdatedAt (RFC3339) e UpdatedBy (userID) são nil
// quando a aparência ainda não foi escrita (default).
type AppearanceResponse struct {
	Appearance Appearance `json:"appearance"`
	UpdatedAt  *string    `json:"updatedAt"`
	UpdatedBy  *string    `json:"updatedBy"`
}

// defaultAppearance é a aparência retornada quando não há linha persistida.
func defaultAppearance() Appearance {
	return Appearance{
		Version:         1,
		ActiveTheme:     defaultActiveTheme,
		CustomThemeName: defaultCustomThemeName,
		Overrides:       map[string]map[string]string{},
	}
}
