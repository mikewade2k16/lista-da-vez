// Package crm agrupa os submodulos de CRM/ERP da plataforma:
// erp (ingestao FTP/CSV, projecoes, dashboard CRM 360) e catalog (busca de produtos).
//
// O modulo declara seu catalogo (permissoes e role templates) no Module Registry
// para que core.modules contenha uma entrada "crm" e as accounts possam
// contratar/descontratar o modulo via core.account_modules.
//
// Roteamento HTTP: os endpoints /v1/erp/*, /v1/catalog/* ainda sao registrados
// diretamente no app.go (wiring legado). A migracao para RequireModule("crm")
// acontece quando o frontend suportar X-Account-Id em todas as rotas de CRM.
package crm

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// Module implementa modules.Module para o dominio CRM.
type Module struct{}

// New cria um Module pronto para registrar no Registry.
func New() *Module { return &Module{} }

// ID identifica o modulo no Registry e em core.modules.
func (m *Module) ID() string { return "crm" }

// Metadata descreve o modulo no catalogo.
func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:      "crm",
		Label:           "CRM / ERP",
		Description:     "Integracao ERP (ingestao FTP/CSV, projecoes de estoque), dashboard CRM 360 (vendas x fila), busca de produtos no catalogo.",
		IsCore:          false,
		OptionalModules: []string{},
		SortOrder:       20,
	}
}

// Permissions declara o catalogo de permissoes do modulo crm.
func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{
			Key:         "crm.erp.sync",
			Label:       "Disparar sync do ERP",
			Description: "Autoriza trigger manual de sincronizacao FTP/CSV do ERP.",
			Scope:       "account",
		},
		{
			Key:         "crm.erp.read",
			Label:       "Ver dados do ERP",
			Description: "Acesso a registros sincronizados do ERP: clientes, pedidos, funcionarios, itens.",
			Scope:       "account",
		},
		{
			Key:         "crm.dashboard.read",
			Label:       "Ver dashboard CRM",
			Description: "Acesso ao painel CRM 360 com metricas de vendas, conversao, faturamento e ranking.",
			Scope:       "account",
		},
		{
			Key:         "crm.catalog.read",
			Label:       "Buscar no catalogo",
			Description: "Buscar produtos no catalogo (ERP atual ou produtos internos).",
			Scope:       "store",
		},
		{
			Key:         "crm.analytics.read",
			Label:       "Ver analytics de vendas",
			Description: "Acesso a agregacoes de vendas por consultor, loja e periodo.",
			Scope:       "account",
		},
	}
}

// RoleTemplates declara os cargos-template do modulo crm.
func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "crm.manager",
			Label:       "Gerente CRM",
			Description: "Acesso completo ao CRM: sync ERP, dashboard, analytics e catalogo.",
			IsSystem:    true,
			SortOrder:   25,
			Permissions: []string{
				"crm.erp.sync",
				"crm.erp.read",
				"crm.dashboard.read",
				"crm.catalog.read",
				"crm.analytics.read",
			},
		},
		{
			ID:          "crm.analyst",
			Label:       "Analista CRM",
			Description: "Somente leitura: dashboard, analytics e catalogo. Sem permissao para disparar sync.",
			IsSystem:    true,
			SortOrder:   35,
			Permissions: []string{
				"crm.erp.read",
				"crm.dashboard.read",
				"crm.catalog.read",
				"crm.analytics.read",
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

func (h *handle) ID() string { return "crm" }

func (h *handle) RegisterRoutes(_ *http.ServeMux) {}

func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error { return nil }
