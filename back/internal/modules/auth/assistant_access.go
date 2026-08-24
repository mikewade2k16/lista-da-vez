package auth

import "strings"

// CanConfigureAssistant centraliza a politica transversal de configuracao do
// Assistente 360. RequireAuthWithAccount deve ter validado membership e
// hidratado Permissions antes desta checagem.
func CanConfigureAssistant(principal Principal) bool {
	if principal.Role == RolePlatformAdmin || principal.Role == RoleOwner {
		return true
	}
	if !principal.PermissionsResolved {
		return false
	}
	for _, permission := range principal.Permissions {
		switch strings.TrimSpace(permission) {
		case "automation.manage",
			"calendar.manage",
			"meta_ads.manage",
			"omnichannel.agents.manage",
			"core.account.manage",
			"workspace.configuracoes.edit":
			return true
		}
	}
	return false
}

// CanManageAssistantConfiguration protege os campos transversais do Assistente
// 360 (persona, modelo, credencial e matriz de todas as surfaces). Uma permissao
// restrita a Calendar, Meta Ads ou Omnichannel nao pode reconfigurar os demais
// dominios da conta.
func CanManageAssistantConfiguration(principal Principal) bool {
	if principal.Role == RolePlatformAdmin || principal.Role == RoleOwner {
		return true
	}
	if !principal.PermissionsResolved {
		return false
	}
	for _, permission := range principal.Permissions {
		switch strings.TrimSpace(permission) {
		case "automation.manage", "core.account.manage", "workspace.configuracoes.edit":
			return true
		}
	}
	return false
}

// CanManageAssistantCredentials restringe mutacoes do cofre compartilhado a
// administradores transversais. Gestores de um modulo podem selecionar e usar
// entradas mascaradas, mas nao criar, rotacionar ou excluir segredos globais.
func CanManageAssistantCredentials(principal Principal) bool {
	return CanManageAssistantConfiguration(principal)
}
