package access

import (
	"sort"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

var permissionCatalog = []PermissionDefinition{
	{Key: PermissionOperationsView, Scope: ScopeTenant, Description: "Visualizar a workspace Operacao."},
	{Key: PermissionOperationsEdit, Scope: ScopeTenant, Description: "Executar comandos na workspace Operacao."},
	{Key: PermissionConsultantView, Scope: ScopeTenant, Description: "Visualizar a workspace Consultor."},
	{Key: PermissionRankingView, Scope: ScopeTenant, Description: "Visualizar a workspace Ranking."},
	{Key: PermissionDataView, Scope: ScopeTenant, Description: "Visualizar a workspace Dados."},
	{Key: PermissionIntelligenceView, Scope: ScopeTenant, Description: "Visualizar a workspace Inteligencia."},
	{Key: PermissionReportsView, Scope: ScopeTenant, Description: "Visualizar a workspace Relatorios."},
	{Key: PermissionCampaignsView, Scope: ScopeTenant, Description: "Visualizar a workspace Campanhas."},
	{Key: PermissionCampaignsEdit, Scope: ScopeTenant, Description: "Editar a workspace Campanhas."},
	{Key: PermissionClientsView, Scope: ScopeTenant, Description: "Visualizar a workspace Clientes."},
	{Key: PermissionClientsEdit, Scope: ScopeTenant, Description: "Editar clientes e grupos acessiveis."},
	{Key: PermissionMultiStoreView, Scope: ScopeTenant, Description: "Visualizar a workspace Multi-loja."},
	{Key: PermissionMultiStoreEdit, Scope: ScopeTenant, Description: "Editar lojas e configuracoes administrativas da workspace Multi-loja."},
	{Key: PermissionUsersView, Scope: ScopeTenant, Description: "Visualizar a workspace Usuarios."},
	{Key: PermissionUsersEdit, Scope: ScopeTenant, Description: "Editar usuarios e overrides de acesso pelo painel."},
	{Key: PermissionSettingsView, Scope: ScopeTenant, Description: "Visualizar a workspace Configuracoes."},
	{Key: PermissionSettingsEdit, Scope: ScopeTenant, Description: "Editar configuracoes operacionais."},
	{Key: PermissionAlertsView, Scope: ScopeTenant, Description: "Visualizar a workspace Alertas."},
	{Key: PermissionAlertsEdit, Scope: ScopeTenant, Description: "Gerenciar a workspace Alertas."},
	{Key: PermissionAlertsRulesManage, Scope: ScopeTenant, Description: "Editar regras tenant-wide do modulo de alertas."},
	{Key: PermissionAlertsActionsManage, Scope: ScopeTenant, Description: "Executar acknowledge e resolucao de alertas operacionais."},
	{Key: PermissionFeedbackView, Scope: ScopeTenant, Description: "Visualizar a workspace Feedback."},
	{Key: PermissionFeedbackEdit, Scope: ScopeTenant, Description: "Editar feedback e notas administrativas."},
	{Key: PermissionERPView, Scope: ScopeTenant, Description: "Visualizar a workspace ERP."},
	{Key: PermissionERPEdit, Scope: ScopeTenant, Description: "Executar sync manual e administrar a workspace ERP."},
	{Key: PermissionUsersPasswordEdit, Scope: ScopePlatform, Description: "Redefinir senha administrativa pelo painel."},
	{Key: PermissionRoleMatrixEdit, Scope: ScopePlatform, Description: "Editar o acesso padrao por papel."},
}

var defaultRolePermissionMap = map[auth.Role][]string{
	auth.RoleConsultant: {
		PermissionOperationsView,
		PermissionOperationsEdit,
	},
	auth.RoleStoreTerminal: {
		PermissionOperationsView,
		PermissionOperationsEdit,
		PermissionConsultantView,
		PermissionRankingView,
		PermissionDataView,
		PermissionIntelligenceView,
		PermissionReportsView,
		PermissionAlertsView,
		PermissionAlertsActionsManage,
	},
	auth.RoleManager: {
		PermissionOperationsView,
		PermissionOperationsEdit,
		PermissionAlertsView,
		PermissionAlertsActionsManage,
		PermissionERPView,
		PermissionFeedbackView,
		PermissionFeedbackEdit,
	},
	auth.RoleMarketing: {
		PermissionOperationsView,
		PermissionERPView,
		PermissionCampaignsView,
		PermissionCampaignsEdit,
	},
	auth.RoleDirector: {
		PermissionOperationsView,
		PermissionERPView,
	},
	auth.RoleOwner: {
		PermissionOperationsView,
		PermissionOperationsEdit,
		PermissionConsultantView,
		PermissionRankingView,
		PermissionDataView,
		PermissionIntelligenceView,
		PermissionReportsView,
		PermissionCampaignsView,
		PermissionCampaignsEdit,
		PermissionClientsView,
		PermissionClientsEdit,
		PermissionMultiStoreView,
		PermissionMultiStoreEdit,
		PermissionUsersView,
		PermissionUsersEdit,
		PermissionSettingsView,
		PermissionSettingsEdit,
		PermissionAlertsView,
		PermissionAlertsEdit,
		PermissionAlertsRulesManage,
		PermissionAlertsActionsManage,
		PermissionFeedbackView,
		PermissionFeedbackEdit,
		PermissionERPView,
		PermissionERPEdit,
	},
	auth.RolePlatformAdmin: {
		PermissionOperationsView,
		PermissionOperationsEdit,
		PermissionConsultantView,
		PermissionRankingView,
		PermissionDataView,
		PermissionIntelligenceView,
		PermissionReportsView,
		PermissionCampaignsView,
		PermissionCampaignsEdit,
		PermissionClientsView,
		PermissionClientsEdit,
		PermissionMultiStoreView,
		PermissionMultiStoreEdit,
		PermissionUsersView,
		PermissionUsersEdit,
		PermissionSettingsView,
		PermissionSettingsEdit,
		PermissionAlertsView,
		PermissionAlertsEdit,
		PermissionAlertsRulesManage,
		PermissionAlertsActionsManage,
		PermissionFeedbackView,
		PermissionFeedbackEdit,
		PermissionERPView,
		PermissionERPEdit,
		PermissionUsersPasswordEdit,
		PermissionRoleMatrixEdit,
	},
}

func PermissionCatalog() []PermissionDefinition {
	cloned := make([]PermissionDefinition, len(permissionCatalog))
	copy(cloned, permissionCatalog)
	return cloned
}

func DefaultRolePermissions(role auth.Role) []string {
	return normalizePermissionKeys(defaultRolePermissionMap[role])
}

func RecognizedPermissionKeys(keys []string) []string {
	recognized := make([]string, 0, len(keys))
	for _, key := range normalizePermissionKeys(keys) {
		if _, ok := PermissionDefinitionForKey(key); ok {
			recognized = append(recognized, key)
		}
	}

	return recognized
}

func PermissionCatalogKeys() []string {
	keys := make([]string, 0, len(permissionCatalog))
	for _, definition := range permissionCatalog {
		keys = append(keys, definition.Key)
	}

	return keys
}

func PermissionDefinitionForKey(key string) (PermissionDefinition, bool) {
	normalizedKey := strings.TrimSpace(key)
	for _, definition := range permissionCatalog {
		if definition.Key == normalizedKey {
			return definition, true
		}
	}

	return PermissionDefinition{}, false
}

func EffectivePermissionKeys(base []string, overrides []UserOverride) []string {
	grants := make(map[string]bool, len(base))
	for _, key := range normalizePermissionKeys(base) {
		grants[key] = true
	}

	for _, override := range overrides {
		key := strings.TrimSpace(override.PermissionKey)
		if key == "" || !override.IsActive {
			continue
		}

		switch strings.TrimSpace(override.Effect) {
		case EffectAllow:
			grants[key] = true
		case EffectDeny:
			delete(grants, key)
		}
	}

	keys := make([]string, 0, len(grants))
	for key := range grants {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys
}

func HasPermission(permissionKeys []string, permissionKey string) bool {
	normalizedNeedle := strings.TrimSpace(permissionKey)
	if normalizedNeedle == "" {
		return false
	}

	for _, candidate := range permissionKeys {
		if strings.TrimSpace(candidate) == normalizedNeedle {
			return true
		}
	}

	return false
}

func normalizePermissionKeys(keys []string) []string {
	seen := make(map[string]struct{}, len(keys))
	normalized := make([]string, 0, len(keys))
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	sort.Strings(normalized)
	return normalized
}
