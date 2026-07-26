package customerdata

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/core"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

type Module struct {
	service *Service
	handle  *handle
}

func New() *Module { return &Module{} }

func (m *Module) ID() string { return ModuleID }

func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:      SchemaName,
		Label:           "Customer Data",
		Description:     "Subjects, relacionamentos, identidades, consentimentos, interações offline, matching e segmentos determinísticos.",
		RequiresModules: []string{"core"},
		OptionalModules: []string{"omnichannel", "crm", "site"},
		SortOrder:       44,
	}
}

func (m *Module) Permissions() []modules.PermissionDef {
	keys := []struct {
		key, label string
	}{
		{"customer_data.subjects.view", "Ver subjects"},
		{"customer_data.subjects.manage", "Gerenciar subjects"},
		{"customer_data.relationships.view", "Ver relacionamentos"},
		{"customer_data.relationships.manage", "Gerenciar relacionamentos"},
		{"customer_data.identities.view", "Ver identidades mascaradas"},
		{"customer_data.identities.manage", "Gerenciar identidades"},
		{"customer_data.notes.view", "Ver notas"},
		{"customer_data.notes.manage", "Gerenciar notas"},
		{"customer_data.consents.view", "Ver consentimentos"},
		{"customer_data.consents.manage", "Registrar consentimentos"},
		{"customer_data.offline_interactions.view", "Ver interações offline"},
		{"customer_data.offline_interactions.manage", "Gerenciar interações offline"},
		{"customer_data.offline_interactions.import", "Importar interações offline"},
		{"customer_data.merge.manage", "Revisar matching e executar merge/undo"},
		{"customer_data.segments.view", "Ver segmentos"},
		{"customer_data.segments.manage", "Gerenciar segmentos"},
		{"customer_data.segments.evaluate", "Avaliar segmentos"},
		{"customer_data.segments.publish", "Publicar e reverter versões de segmentos"},
		{"customer_data.segments.export", "Solicitar exportação elegível"},
		{"customer_data.audit.view", "Ver auditoria do Customer Data"},
		{"customer_data.capabilities.manage", "Gerenciar capabilities e writer state"},
	}
	out := make([]modules.PermissionDef, 0, len(keys))
	for _, item := range keys {
		out = append(out, modules.PermissionDef{Key: item.key, Label: item.label, Scope: "account"})
	}
	return out
}

func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID: "customer_data.viewer", Label: "Leitor de Customer Data", SortOrder: 100,
			Permissions: []string{
				"customer_data.subjects.view", "customer_data.relationships.view",
				"customer_data.identities.view", "customer_data.notes.view",
				"customer_data.consents.view", "customer_data.offline_interactions.view",
				"customer_data.segments.view",
			},
		},
		{
			ID: "customer_data.operator", Label: "Operador de Customer Data", SortOrder: 110,
			Permissions: []string{
				"customer_data.subjects.view", "customer_data.subjects.manage",
				"customer_data.relationships.view", "customer_data.relationships.manage",
				"customer_data.identities.view", "customer_data.identities.manage",
				"customer_data.notes.view", "customer_data.notes.manage",
				"customer_data.consents.view", "customer_data.consents.manage",
				"customer_data.offline_interactions.view", "customer_data.offline_interactions.manage",
				"customer_data.segments.view", "customer_data.segments.evaluate",
			},
		},
		{
			ID: "customer_data.steward", Label: "Steward de Customer Data", SortOrder: 120,
			Permissions: []string{
				"customer_data.subjects.view", "customer_data.subjects.manage",
				"customer_data.relationships.view", "customer_data.relationships.manage",
				"customer_data.identities.view", "customer_data.identities.manage",
				"customer_data.notes.view", "customer_data.notes.manage",
				"customer_data.consents.view", "customer_data.consents.manage",
				"customer_data.offline_interactions.view", "customer_data.offline_interactions.manage",
				"customer_data.offline_interactions.import", "customer_data.merge.manage",
				"customer_data.segments.view", "customer_data.segments.manage",
				"customer_data.segments.evaluate", "customer_data.segments.publish",
				"customer_data.audit.view",
			},
		},
		{
			ID: "customer_data.admin", Label: "Administrador de Customer Data", SortOrder: 130,
			Permissions: []string{
				"customer_data.subjects.view", "customer_data.subjects.manage",
				"customer_data.relationships.view", "customer_data.relationships.manage",
				"customer_data.identities.view", "customer_data.identities.manage",
				"customer_data.notes.view", "customer_data.notes.manage",
				"customer_data.consents.view", "customer_data.consents.manage",
				"customer_data.offline_interactions.view", "customer_data.offline_interactions.manage",
				"customer_data.offline_interactions.import", "customer_data.merge.manage",
				"customer_data.segments.view", "customer_data.segments.manage",
				"customer_data.segments.evaluate", "customer_data.segments.publish",
				"customer_data.segments.export", "customer_data.audit.view",
				"customer_data.capabilities.manage",
			},
		},
	}
}

func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	repo := NewPostgresRepository(deps.Pool)
	authorizer := core.NewRBACService(core.NewPostgresRBACRepository(deps.Pool))
	protector, err := NewEnvironmentIdentityProtector()
	if err != nil && deps.Logger != nil {
		deps.Logger.Warn("customer data identity writes disabled", "reason", "identity_keys_unavailable")
	}
	m.service = NewService(repo, authorizer, protector)
	m.handle = &handle{
		service:        m.service,
		authMiddleware: deps.AuthMiddleware,
		modulesGuard:   deps.ModulesGuard,
	}
	return m.handle, nil
}

// Service expõe as façades owner-scoped para adapters no composition root.
func (m *Module) Service() *Service { return m.service }

type handle struct {
	service        *Service
	authMiddleware *auth.Middleware
	modulesGuard   *httpapi.AccountModulesGuard
}

func (h *handle) ID() string { return ModuleID }

func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.authMiddleware, h.modulesGuard)
}

func (h *handle) RegisterEventHandlers(_ events.Bus) {}
func (h *handle) Close() error                       { return nil }
