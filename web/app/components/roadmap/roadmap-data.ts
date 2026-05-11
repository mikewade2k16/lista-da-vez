export type PhaseStatus = "pending" | "in_progress" | "done" | "blocked";

export interface RoadmapTask {
  id: string;
  label: string;
  done: boolean;
  note?: string;
}

export interface RoadmapPhase {
  id: string;
  code: string;
  title: string;
  goal: string;
  status: PhaseStatus;
  estimateWeeks: string;
  startedAt?: string;
  finishedAt?: string;
  tasks: RoadmapTask[];
  verifiable: string;
  blockers?: string[];
}

export const ROADMAP_TITLE = "Reestruturação Multi-Tenant";
export const ROADMAP_SUBTITLE =
  "Acompanhamento das fases da branch refactor/multi-tenant-core. Cada fase é um deploy reversível; produção atual segue intocada em main/migracao/nuxt.";

export const ROADMAP_PHASES: RoadmapPhase[] = [
  {
    id: "fase-0",
    code: "Fase 0",
    title: "Fundação",
    goal: "Preparar terreno para o trabalho da reestruturação sem quebrar nada do que já existe.",
    status: "done",
    estimateWeeks: "1–2 semanas",
    startedAt: "2026-05-10",
    finishedAt: "2026-05-10",
    tasks: [
      { id: "branch", label: "Criar branch refactor/multi-tenant-core a partir de migracao/nuxt", done: true },
      { id: "contract-freeze", label: "docs/CONTRACT_FREEZE.md com interfaces que não podem quebrar até Fase 4", done: true },
      { id: "schema-target", label: "docs/SCHEMA_TARGET.md com diagrama dos schemas Postgres alvo", done: true },
      { id: "feature-flag", label: "Feature-flag CORE_V2_ENABLED no backend", done: true, note: "Exposta em GET /healthz e logada no boot quando ativa." }
    ],
    verifiable: "Projeto compila e roda igual ao main."
  },
  {
    id: "fase-1",
    code: "Fase 1",
    title: "Schema core novo",
    goal: "Criar tabelas core (organizations, accounts, users globais, sessions, roles, permissions) sem desligar o produto atual.",
    status: "done",
    estimateWeeks: "2–3 semanas",
    startedAt: "2026-05-10",
    finishedAt: "2026-05-10",
    tasks: [
      { id: "migration", label: "Migration 0100_core_schema.sql cria seção A.2 completa", done: true, note: "15 tabelas em schema core; idempotente." },
      { id: "seed", label: "Job de seed: public.tenants → core.accounts (mesmo id) + account_users", done: true, note: "Migration 0101 com ON CONFLICT DO NOTHING." },
      { id: "endpoint-accounts", label: "GET /v2/me/accounts (lean) sob feature-flag", done: true, note: "Lista accounts do user autenticado." },
      { id: "endpoint-context", label: "GET /v2/me/context?accountId=... (full) sob feature-flag", done: true, note: "Roles/permissions vazios até Fase 3." },
      { id: "module-go", label: "Módulo Go back/internal/modules/core/ (model/store/service/http/AGENT.md)", done: true },
      { id: "legacy", label: "GET /v1/me/context legado intacto", done: true, note: "Endpoints v1 não foram modificados." }
    ],
    verifiable: "Login antigo funciona; com flag CORE_V2_ENABLED=true, /v2/me/accounts retorna lista de accounts e /v2/me/context retorna o contexto completo."
  },
  {
    id: "fase-2",
    code: "Fase 2",
    title: "Module Registry e refactor do bootstrap",
    goal: "Introduzir Registry de módulos plugáveis e event bus in-process sem mudar comportamento das rotas legadas.",
    status: "done",
    estimateWeeks: "2 semanas",
    startedAt: "2026-05-10",
    finishedAt: "2026-05-10",
    tasks: [
      { id: "registry-pkg", label: "Pacote back/internal/platform/modules/ com Module, Handle, Dependencies, Registry, CatalogRepository", done: true },
      { id: "events-pkg", label: "Pacote back/internal/platform/events/ com Bus + InMemoryBus (causationId, correlationId, MaxDepth=10)", done: true },
      { id: "guard", label: "Middleware AccountModulesGuard em platform/httpapi/ (cache 60s, X-Account-Id)", done: true, note: "Disponível para módulos satélites; não aplicado a rotas v2/me/* do core (são ponto de descoberta)." },
      { id: "sync-catalog", label: "SyncCatalog no boot popula core.modules, core.permissions, core.role_templates declarativamente", done: true, note: "deprecated_at marca removidas; nunca DELETE auto." },
      { id: "core-module", label: "Módulo core implementa interface Module (8 permissões, 3 role templates: owner/admin/member)", done: true },
      { id: "app-integration", label: "app.go usa Registry.Build/SyncCatalog quando CORE_V2_ENABLED=true; legado intacto quando off", done: true },
      { id: "adapters", label: "Módulos legados (auth, tenants, stores, etc.) NÃO foram embrulhados em adapters", done: true, note: "Decisão pragmática: continuam pelo wiring legado até serem reescritos na Fase 4 (queue) e Fase 6 (satélites). Infra do Registry está pronta para receber satélites quando chegarem." }
    ],
    verifiable: "go build ./... passa; com flag on, SyncCatalog roda no boot e popula core.modules/permissions/role_templates a partir do módulo core declarativo. Endpoints /v2/me/* continuam funcionando via handle do Registry."
  },
  {
    id: "fase-3",
    code: "Fase 3",
    title: "RBAC dinâmico",
    goal: "Permitir que cada Account clone cargos-template e edite suas próprias permissões.",
    status: "done",
    estimateWeeks: "2 semanas",
    startedAt: "2026-05-10",
    finishedAt: "2026-05-10",
    tasks: [
      { id: "rbac-service", label: "Service core.rbac (CloneTemplateToAccount, CreateRole, UpdateRolePermissions, AssignRoleToUser)", done: true },
      { id: "rbac-endpoint", label: "Endpoint /v1/accounts/:id/roles CRUD + AssignRoleToUser", done: true },
      { id: "data-migration", label: "Migração de dados: roles atuais (Owner, Manager, Director, etc.) viram core.roles por account", done: true, note: "Migration 0103. Requer boot com CORE_V2_ENABLED=true antes de executar." },
      { id: "principal-resolution", label: "MeContext resolve Roles[] e Permissions[] reais de core.role_permissions (legado continua como fallback no auth)", done: true }
    ],
    verifiable: "UI de roles permite clonar template e ajustar permissões; mudança reflete no login do user."
  },
  {
    id: "fase-4a",
    code: "Fase 4A",
    title: "Schema queue — fundação",
    goal: "Mover tabelas estáveis (stores, consultants, settings, catalog) para schema queue sem quebrar leitores atuais.",
    status: "pending",
    estimateWeeks: "1 semana",
    tasks: [
      { id: "schema-create", label: "Criar schema queue + migrations base", done: false },
      { id: "move-stable", label: "Migrar stores, consultants, settings, catalog → queue.* com FK para core.accounts(id)", done: false },
      { id: "compat-views", label: "Views de compatibilidade public.* → queue.* durante transição", done: false }
    ],
    verifiable: "Produto roda igual; queries SELECT batem nas views novas; testes existentes passam."
  },
  {
    id: "fase-4b",
    code: "Fase 4B",
    title: "Domínio operacional principal",
    goal: "Migrar operations e feedback (core do dia-a-dia) para o módulo queue.",
    status: "pending",
    estimateWeeks: "1–2 semanas",
    tasks: [
      { id: "move-ops", label: "Migrar operations + feedback para queue.*", done: false },
      { id: "module-rewrite", label: "Reescrever back/internal/modules/operations/ como subpacote queue/operations/", done: false },
      { id: "shape-compat", label: "Endpoints /v1/operations/* mantêm shape (front não muda)", done: false }
    ],
    verifiable: "Fluxo golden de operação (entrada → pausa → atendimento → fim) idêntico em staging."
  },
  {
    id: "fase-4c",
    code: "Fase 4C",
    title: "Analytics, alertas, ERP",
    goal: "Migrar módulos auxiliares (alerts, analytics, reports, erp) para o schema queue.",
    status: "pending",
    estimateWeeks: "1 semana",
    tasks: [
      { id: "move-aux", label: "Migrar alerts, analytics, reports, erp → queue.*", done: false },
      { id: "subpackages", label: "Cada um vira subpacote queue/<nome>/", done: false },
      { id: "erp-rebuild", label: "ERP: testar rebuild de projeções com base no schema novo", done: false }
    ],
    verifiable: "Dashboards de relatório, alertas e sincronização ERP funcionam idênticos."
  },
  {
    id: "fase-4d",
    code: "Fase 4D",
    title: "Frontend layer queue",
    goal: "Mover páginas e stores da fila-atendimento para web/layers/queue/.",
    status: "pending",
    estimateWeeks: "1 semana (paralelo a 4C)",
    tasks: [
      { id: "layer-create", label: "Criar web/layers/queue/ com nav.config.ts", done: false },
      { id: "pages-move", label: "Mover pages/stores listadas em E.4 para o layer", done: false },
      { id: "shell-minimal", label: "Shell app/ fica minimal", done: false }
    ],
    verifiable: "Trocar account no AccountSwitcher recarrega menu; rota /operacao continua funcionando dentro do layer."
  },
  {
    id: "fase-5",
    code: "Fase 5",
    title: "Frontend layers + menu dinâmico",
    goal: "Substituir sidebar estática por menu montado a partir dos nav.config.ts dos layers.",
    status: "in_progress",
    estimateWeeks: "1–2 semanas (paralelo à Fase 4)",
    startedAt: "2026-05-10",
    tasks: [
      { id: "plugin-registry", label: "app/plugins/module-registry.client.ts lendo nav.config.ts via import.meta.glob", done: true, note: "Injeta layers dinamicamente + fallback legado via sidebar-nav.ts enquanto layer queue não chega." },
      { id: "core-layer", label: "layers/core/ com AccountSwitcher, PermissionGate, usePermission, useNav", done: true, note: "stores/account.ts (multi-account v2), composables/usePermission, composables/useNav, CoreAccountSwitcher.vue, CorePermissionGate.vue." },
      { id: "delete-static", label: "Deletar web/app/utils/sidebar-nav.ts", done: false, note: "Bloqueado: requer layer queue com nav.config.ts próprio para substituir o fallback legado." },
      { id: "sidebar-rewrite", label: "DashboardSidebarNav.vue reescrito para consumir useNavStore", done: true }
    ],
    verifiable: "Trocar account no AccountSwitcher recarrega menu; desabilitar módulo no banco esconde itens."
  },
  {
    id: "fase-6",
    code: "Fase 6",
    title: "Trazer módulos satélites",
    goal: "Incorporar finance, tasks, omni, site, bio, contacts (1 PR por módulo) ao monorepo.",
    status: "pending",
    estimateWeeks: "Incremental — 1 a 2 semanas por módulo",
    tasks: [
      { id: "finance", label: "Finance — backend module + layer + permissões + role templates", done: false },
      { id: "tasks", label: "Tasks (notion-like) — backend module + layer + permissões", done: false },
      { id: "omni", label: "Omnichannel (WhatsApp/Instagram) — backend module + layer + permissões", done: false },
      { id: "contacts", label: "Contacts (core opcional) — fonte de verdade quando habilitado", done: false },
      { id: "site", label: "Site — backend module + layer + permissões", done: false },
      { id: "bio", label: "Bio — backend module + layer + permissões", done: false }
    ],
    verifiable: "Por módulo: habilitar no account-piloto → menu mostra → criar registro → consultar via API → desabilitar → menu/rota somem."
  }
];
