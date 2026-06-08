// Package queue agrupa os submodulos da plataforma de fila de atendimento:
// operations, alerts, analytics, reports, feedback, consultants, settings.
//
// O modulo declara seu catalogo (permissoes e role templates) no Module Registry
// para que core.modules contenha uma entrada "queue" e as accounts possam
// contratar/descontratar o modulo via core.account_modules.
//
// Roteamento HTTP: os endpoints /v1/operations/*, /v1/alerts/*, etc. ainda
// sao registrados diretamente no app.go (wiring legado com RequireAuth).
// A migracao para RequireAuthWithAccount + RequireModule("queue") acontece
// quando o frontend suportar X-Account-Id em todas as rotas de fila.
package queue

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// Module implementa modules.Module para o dominio queue.
type Module struct{}

// New cria um Module pronto para registrar no Registry.
func New() *Module { return &Module{} }

// ID identifica o modulo no Registry e em core.modules.
func (m *Module) ID() string { return "queue" }

// Metadata descreve o modulo no catalogo.
func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:      "queue",
		Label:           "Fila de Atendimento",
		Description:     "Gerenciamento de fila, operacoes, alertas, analytics, relatorios, feedback e configuracoes de consultores.",
		IsCore:          false,
		OptionalModules: []string{"crm"},
		SortOrder:       10,
	}
}

// Permissions declara o catalogo de permissoes do modulo queue.
func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{
			Key:         "queue.dashboard.read",
			Label:       "Ver dashboard da fila",
			Description: "Acesso ao painel de operacoes em tempo real.",
			Scope:       "store",
		},
		{
			Key:         "queue.operations.manage",
			Label:       "Gerenciar operacoes",
			Description: "Abrir, pausar, finalizar atendimentos e movimentar fila.",
			Scope:       "store",
		},
		{
			Key:         "queue.alerts.manage",
			Label:       "Gerenciar alertas",
			Description: "Criar, editar e visualizar regras de alerta da fila.",
			Scope:       "account",
		},
		{
			Key:         "queue.analytics.read",
			Label:       "Ver analytics",
			Description: "Acesso a metricas e rankings de desempenho.",
			Scope:       "account",
		},
		{
			Key:         "queue.reports.read",
			Label:       "Ver relatorios",
			Description: "Acesso a relatorios de atendimento e exportacoes.",
			Scope:       "account",
		},
		{
			Key:         "queue.feedback.read",
			Label:       "Ver feedbacks",
			Description: "Visualizar feedbacks dos atendimentos.",
			Scope:       "account",
		},
		{
			Key:         "queue.settings.manage",
			Label:       "Gerenciar configuracoes da fila",
			Description: "Alterar parametros de operacao, templates e pesos de pontuacao.",
			Scope:       "account",
		},
		{
			Key:         "queue.consultants.manage",
			Label:       "Gerenciar consultores",
			Description: "Criar, editar e desativar consultores. Vincular ao ERP.",
			Scope:       "account",
		},
	}
}

// RoleTemplates declara os cargos-template do modulo queue.
func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "queue.supervisor",
			Label:       "Supervisor de Fila",
			Description: "Acesso completo as operacoes, alertas, analytics e configuracoes da fila.",
			IsSystem:    true,
			SortOrder:   20,
			Permissions: []string{
				"queue.dashboard.read",
				"queue.operations.manage",
				"queue.alerts.manage",
				"queue.analytics.read",
				"queue.reports.read",
				"queue.feedback.read",
				"queue.settings.manage",
				"queue.consultants.manage",
			},
		},
		{
			ID:          "queue.consultant",
			Label:       "Consultor",
			Description: "Acesso ao dashboard e operacoes da propria fila. Somente leitura em analytics.",
			IsSystem:    true,
			SortOrder:   30,
			Permissions: []string{
				"queue.dashboard.read",
				"queue.operations.manage",
				"queue.analytics.read",
			},
		},
	}
}

// Build constroe o handle do modulo. Rotas sao montadas pelo wiring legado em
// app.go ate que todos os endpoints migrem para RequireAuthWithAccount.
func (m *Module) Build(_ modules.Dependencies) (modules.Handle, error) {
	return &handle{}, nil
}

// ============================================================================
// Handle interno (sem rotas por enquanto — wiring legado em app.go)
// ============================================================================

type handle struct{}

func (h *handle) ID() string { return "queue" }

func (h *handle) RegisterRoutes(_ *http.ServeMux) {}

func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error { return nil }
