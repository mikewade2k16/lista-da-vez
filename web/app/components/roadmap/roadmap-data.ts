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
    status: "pending",
    estimateWeeks: "2–3 semanas",
    tasks: [
      { id: "migration", label: "Migration 0100_core_schema.sql cria seção A.2 completa", done: false },
      { id: "seed", label: "Job de seed: public.tenants → core.accounts (mesmo id) + account_users", done: false },
      { id: "endpoint-context", label: "GET /v2/me/context sob feature-flag", done: false },
      { id: "endpoint-switch", label: "POST /v2/accounts/:id/switch sob feature-flag", done: false },
      { id: "legacy", label: "GET /v1/me/context legado intacto", done: false }
    ],
    verifiable: "Login antigo funciona; com flag ligada, retorna accounts[] e activeAccountId."
  },
  {
    id: "fase-2",
    code: "Fase 2",
    title: "Module Registry e refactor do bootstrap",
    goal: "Trocar o wiring manual por um Registry de módulos plugáveis sem mudar comportamento das rotas atuais.",
    status: "pending",
    estimateWeeks: "2 semanas",
    tasks: [
      { id: "registry-pkg", label: "Pacote back/internal/platform/modules/ (Registry, Module, Dependencies, EventBus)", done: false },
      { id: "app-rewrite", label: "back/internal/platform/app/app.go reescrito sobre registry.Build(deps)", done: false },
      { id: "adapters", label: "Módulos atuais embrulhados em adapters Module finos (zero reescrita interna)", done: false },
      { id: "sync-catalog", label: "core.modules / core.permissions / core.role_templates populados pelo SyncCatalog no boot", done: false },
      { id: "guard", label: "Middleware accountModulesGuard ativo (todos os módulos atuais habilitados para todos os accounts existentes)", done: false }
    ],
    verifiable: "Rotas atuais respondem igual, agora gated pelo guard."
  },
  {
    id: "fase-3",
    code: "Fase 3",
    title: "RBAC dinâmico",
    goal: "Permitir que cada Account clone cargos-template e edite suas próprias permissões.",
    status: "pending",
    estimateWeeks: "2 semanas",
    tasks: [
      { id: "rbac-service", label: "Service core.rbac (CloneTemplateToAccount, CreateRole, UpdateRolePermissions, AssignRoleToUser)", done: false },
      { id: "rbac-endpoint", label: "Endpoint /v1/accounts/:id/roles CRUD", done: false },
      { id: "data-migration", label: "Migração de dados: roles atuais (Owner, Manager, Director, etc.) viram core.roles por account", done: false },
      { id: "principal-resolution", label: "Principal.Permissions resolvido pelo path novo; antigo continua como fallback", done: false }
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
    status: "pending",
    estimateWeeks: "1–2 semanas (paralelo à Fase 4)",
    tasks: [
      { id: "plugin-registry", label: "app/plugins/module-registry.client.ts lendo nav.config.ts via import.meta.glob", done: false },
      { id: "core-layer", label: "layers/core/ com AccountSwitcher, PermissionGate, usePermission, useNav", done: false },
      { id: "delete-static", label: "Deletar web/app/utils/sidebar-nav.ts", done: false },
      { id: "sidebar-rewrite", label: "DashboardSidebarNav.vue reescrito para consumir useNavStore", done: false }
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
