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
  group?: string;
}

export interface RoadmapGroup {
  id: string;
  label: string;
  description?: string;
}

export type ModuleStatus = "pending" | "in_progress" | "beta" | "done";
export type ModulePriority = "P0" | "P1" | "P2" | "P3";

export interface RoadmapModule {
  id: string;
  label: string;
  route: string;
  status: ModuleStatus;
  priority: ModulePriority;
  description: string;
  scope?: string[];
  dependsOn?: string[];
  category?: "atendimento" | "tools" | "operacao-comercial" | "indicadores" | "manage";
}

export type RuleCategory =
  | "frontend"
  | "backend"
  | "banco"
  | "linguagens"
  | "deploy"
  | "padroes-gerais";

export interface RoadmapRule {
  id: string;
  category: RuleCategory;
  title: string;
  body: string;
  why?: string;
  appliesWhen?: string;
}

export const ROADMAP_GROUPS: RoadmapGroup[] = [
  {
    id: "multi-tenant",
    label: "Reestruturação Multi-Tenant",
    description: "Branch refactor/multi-tenant-core — schema core, RBAC, Module Registry, layers e módulos satélites."
  },
  {
    id: "tasks-backend",
    label: "Tasks Orquestrador — Backend",
    description: "Transformar o protótipo localStorage em produto multi-tenant real: schema tasks.*, API Go, realtime, RBAC, notificações e sistema de views."
  },
  {
    id: "crm-360",
    label: "CRM 360 — Fila + ERP",
    description: "Indicadores por consultor e loja cruzando dados de atendimento da fila com vendas do ERP: conversão, faturamento, PA, ticket médio, produto não encontrado."
  },
  {
    id: "automation",
    label: "Automação WhatsApp/IA",
    description: "Assistente proativa de WhatsApp (n8n + WAHA, persona Tony) trazida para dentro do Omni como módulo automation/. Integração por fases com CRM/catálogo/ERP via API Go."
  }
];

export const ROADMAP_TITLE = "Reestruturação Multi-Tenant";
export const ROADMAP_SUBTITLE =
  "Acompanhamento das fases da branch refactor/multi-tenant-core. Cada fase é um deploy reversível; produção atual segue intocada em main/migracao/nuxt.";

// Aviso de estado real — 2026-05-28
//
// Auditoria de 2026-05-28 mostrou que várias Fases marcadas como `done` neste
// arquivo estão na verdade `in_progress` ou parciais quando confrontadas com o
// runtime real (ver docs/ROADMAP.md → "Estado real em 2026-05-28"):
//
//  - Fase 2: AccountModulesGuard é instanciado e descartado (`_ =` em app.go).
//  - Fase 3: core.user_tenant_roles / core.account_modules estão com 0 linhas.
//  - Fase 5: menu não consulta core.account_modules — filtra por role hardcoded.
//  - Fases 13/15/16: páginas mock (clientes-web/leads-web/produtos-web,
//    site/*Workspace) consomem um BFF Nitro em web/server/ com seed in-memory,
//    sem tocar Postgres.
//
// As notas das tarefas afetadas foram atualizadas. A fase nova
// "multitenant-completion" abaixo é a fonte de verdade da próxima branch
// (refactor/multi-tenant-complete) — só depois dela o lote 13/14/15/16+ retoma.

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
    finishedAt: "2026-06-01",
    tasks: [
      { id: "registry-pkg", label: "Pacote back/internal/platform/modules/ com Module, Handle, Dependencies, Registry, CatalogRepository", done: true },
      { id: "events-pkg", label: "Pacote back/internal/platform/events/ com Bus + InMemoryBus (causationId, correlationId, MaxDepth=10)", done: true },
      { id: "guard", label: "Middleware AccountModulesGuard em platform/httpapi/ (cache 60s, X-Account-Id)", done: true, note: "Concluído em multitenant-completion/C2: guard plugado via Dependencies.ModulesGuard em app.go. RequireModule em rotas satélites deferido para fase dedicada (requer X-Account-Id no frontend das rotas de fila)." },
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
    finishedAt: "2026-06-01",
    tasks: [
      { id: "rbac-service", label: "Service core.rbac (CloneTemplateToAccount, CreateRole, UpdateRolePermissions, AssignRoleToUser)", done: true },
      { id: "rbac-endpoint", label: "Endpoint /v1/accounts/:id/roles CRUD + AssignRoleToUser", done: true },
      { id: "data-migration", label: "Migração de dados: roles atuais (Owner, Manager, Director, etc.) viram core.roles por account", done: true, note: "Concluído em multitenant-completion/C1 via migration 0125_core_roles_backfill.sql (clona role_templates para cada account ativa + mapeia user_tenant_roles → core.user_role_assignments)." },
      { id: "principal-resolution", label: "MeContext resolve Roles[] e Permissions[] reais de core.role_permissions (legado continua como fallback no auth)", done: true, note: "Concluído em multitenant-completion/C2: RequireAuthWithAccount injeta AccountID no Principal; tabelas core.user_role_assignments populadas pelo backfill 0125." }
    ],
    verifiable: "UI de roles permite clonar template e ajustar permissões; mudança reflete no login do user."
  },
  {
    id: "fase-4a",
    code: "Fase 4A",
    title: "Schema queue — fundação",
    goal: "Mover tabelas estáveis (stores, consultants, settings, catalog) para schema queue sem quebrar leitores atuais.",
    status: "done",
    estimateWeeks: "1 semana",
    startedAt: "2026-05-11",
    finishedAt: "2026-05-11",
    tasks: [
      { id: "schema-create", label: "Criar schema queue + migrations base", done: true },
      { id: "move-stable", label: "Migrar stores, consultants, settings, catalog → queue.* com FK para core.accounts(id)", done: true, note: "Migration 0104. FKs internas para queue.*; compat views em public.*." },
      { id: "compat-views", label: "Views de compatibilidade public.* → queue.* durante transição", done: true, note: "Transição encerrada (2026-06-06): todo o Go migrou para queue.*/core.* e as views public.users/stores/consultants + a tabela public.tenants foram DROPADAS na migration 0136 (27 FKs repontadas para core.accounts)." }
    ],
    verifiable: "Produto roda igual; queries SELECT batem nas views novas; testes existentes passam."
  },
  {
    id: "fase-4b",
    code: "Fase 4B",
    title: "Domínio operacional principal",
    goal: "Migrar operations e feedback (core do dia-a-dia) para o módulo queue.",
    status: "done",
    estimateWeeks: "1–2 semanas",
    startedAt: "2026-05-11",
    finishedAt: "2026-05-11",
    tasks: [
      { id: "move-ops", label: "Migrar operations + feedback para queue.*", done: true, note: "Migration 0105: operation_*, user_feedback, feedback_messages, feedback_read_states, tenant settings." },
      { id: "module-rewrite", label: "Reescrever back/internal/modules/operations/ como subpacote queue/operations/", done: true, note: "Concluído: operations virou subpacote queue/operations/ e lê queue.* direto (sem views compat). Itens 2&3 do LEGADO removeram public.* na 0136." },
      { id: "shape-compat", label: "Endpoints /v1/operations/* mantêm shape (front não muda)", done: true, note: "Go agora lê queue.* direto; shape dos endpoints mantido. Views compat dropadas na 0136." }
    ],
    verifiable: "Fluxo golden de operação (entrada → pausa → atendimento → fim) idêntico em staging."
  },
  {
    id: "fase-4c",
    code: "Fase 4C",
    title: "Analytics, alertas, ERP",
    goal: "Migrar módulos auxiliares (alerts, analytics, reports, erp) para o schema queue.",
    status: "done",
    estimateWeeks: "1 semana",
    startedAt: "2026-05-11",
    finishedAt: "2026-06-01",
    tasks: [
      { id: "move-aux", label: "Migrar alerts, analytics, reports, erp → queue.*", done: true, note: "Migration 0106: tenant_alert_settings, alert_instances, alert_actions, erp_sync_runs, erp_item_raw, erp_item_current." },
      { id: "subpackages", label: "Cada um vira subpacote queue/<nome>/", done: true, note: "Concluído em multitenant-completion/C4: alerts, analytics, reports, feedback, consultants, settings movidos para back/internal/modules/queue/*; queue/module.go consolida o módulo." },
      { id: "erp-rebuild", label: "ERP: testar rebuild de projeções com base no schema novo", done: true, note: "ERP movido para crm/erp em C5; rebuild validado via go build. Testes de projeções em staging pendentes como validação de runtime." }
    ],
    verifiable: "Dashboards de relatório, alertas e sincronização ERP funcionam idênticos."
  },
  {
    id: "fase-4d",
    code: "Fase 4D",
    title: "Frontend layer queue",
    goal: "Mover páginas e stores da fila-atendimento para web/layers/queue/.",
    status: "done",
    estimateWeeks: "1 semana (paralelo a 4C)",
    startedAt: "2026-05-11",
    finishedAt: "2026-06-01",
    tasks: [
      { id: "layer-create", label: "Criar web/layers/queue/ com nav.config.ts", done: true, note: "nav.config.ts com todas as seções existentes; sobrescreve legado via module-registry plugin." },
      { id: "pages-move", label: "Mover pages/stores listadas em E.4 para o layer", done: false, note: "Deferido para tasks-refactor-v2: pages ainda em web/app/pages/. Não bloqueia o menu dinâmico (layer queue já é carregado)." },
      { id: "shell-minimal", label: "Shell app/ fica minimal", done: false, note: "Deferido junto com pages-move." }
    ],
    verifiable: "Trocar account no AccountSwitcher recarrega menu; rota /operacao continua funcionando dentro do layer."
  },
  {
    id: "fase-5",
    code: "Fase 5",
    title: "Frontend layers + menu dinâmico",
    goal: "Substituir sidebar estática por menu montado a partir dos nav.config.ts dos layers.",
    status: "done",
    estimateWeeks: "1–2 semanas (paralelo à Fase 4)",
    startedAt: "2026-05-10",
    finishedAt: "2026-06-01",
    tasks: [
      { id: "plugin-registry", label: "app/plugins/module-registry.client.ts lendo nav.config.ts via import.meta.glob", done: true, note: "Injeta layers dinamicamente + fallback legado via sidebar-nav.ts enquanto layer queue não chega." },
      { id: "core-layer", label: "layers/core/ com AccountSwitcher, PermissionGate, usePermission, useNav", done: true, note: "stores/account.ts (multi-account v2), composables/usePermission, composables/useNav, CoreAccountSwitcher.vue, CorePermissionGate.vue." },
      { id: "delete-static", label: "Deletar web/app/utils/sidebar-nav.ts", done: false, note: "Deferido: sidebar-nav.ts ainda é fallback legado. Remover quando pages-move (Fase 4D) for concluído." },
      { id: "sidebar-rewrite", label: "DashboardSidebarNav.vue reescrito para consumir useNavStore", done: true },
      { id: "menu-account-modules", label: "useNav consome core.account_modules para gating dinâmico (esconde itens de módulo desabilitado)", done: true, note: "Concluído em multitenant-completion/C11: useDashboardNav filtra por useCoreAccountStore().enabledModules; middleware module-enabled.global.ts bloqueia rota direta; auth.global.ts dispara fetchAccounts no boot." }
    ],
    verifiable: "Trocar account no AccountSwitcher recarrega menu; desabilitar módulo no banco esconde itens."
  },
  {
    id: "fase-6",
    code: "Fase 6",
    title: "Orquestração dos módulos satélites",
    goal: "Transformar o inventário da Fase 10 em fases executáveis, com 1 trilha por módulo e PRs pequenos.",
    status: "in_progress",
    estimateWeeks: "Coordenação contínua",
    startedAt: "2026-05-12",
    tasks: [
      { id: "module-order", label: "Definir ordem de entrada dos módulos e confirmar o primeiro módulo", done: true, note: "Ordem atual detalhada nas Fases 11-20: Theme Studio; depois Tasks; em seguida o lote simples (Profile, Team, Site e a frente nova de Users/Clientes em paralelo ao legado); por fim os módulos mais pesados como Omni, Finance, Indicators, Tools e Bio." },
      { id: "module-components", label: "Migrar componentes específicos junto com cada módulo importado", done: false, note: "Cada fase de módulo documenta componentes vindos do web-reference e mantém Core* só para reuso real." },
      { id: "module-contract", label: "Para cada módulo: backend Module + schema + permissões + layer + nav.config.ts + aceite de habilitar/desabilitar", done: false },
      { id: "docs-loop", label: "Atualizar docs/COMPONENT_INVENTORY.md e /roadmap a cada fase concluída", done: false }
    ],
    verifiable: "Cada módulo tem fase própria, critérios de aceite claros e validação de account_modules: habilitar mostra menu/rotas; desabilitar esconde e bloqueia acesso."
  },
  {
    id: "fase-7",
    code: "Fase 7",
    title: "Otimização de performance",
    goal: "Reduzir queries por request, parar com login/navegação lentos e consertar o logout que trava.",
    status: "done",
    estimateWeeks: "1 semana",
    startedAt: "2026-05-11",
    finishedAt: "2026-06-01",
    tasks: [
      { id: "indices-stats", label: "Migration 0107 — ANALYZE pós-Fase 4 + índice cobertura sessions(id) WHERE revoked_at IS NULL", done: true },
      { id: "load-user-fast", label: "auth.LoadUserForAuth consolida findRecord + findStoreIDs em 1 query única (LATERAL)", done: true, note: "2 round-trips → 1 no lookup de user no hot-path." },
      { id: "resolve-perms-fast", label: "access.ResolveEffectivePermissions combina role_permissions + overrides em 1 query (UNION/EXCEPT)", done: true, note: "2 round-trips → 1 na resolução de permissões. Fallback DefaultRolePermissions preservado." },
      { id: "auth-token-consolidated", label: "AuthenticateToken usa LoadUserForAuth + ResolveEffectivePermissions (4 → 2 queries por request autenticado)", done: true },
      { id: "logout-endpoint", label: "POST /v1/auth/logout idempotente + revogação real", done: true, note: "Revogação ligada em P0·2 (2026-06-07): logout revoga o sid em core.user_sessions; token revogado → 401." },
      { id: "logout-frontend", label: "Frontend auth.logout() chama backend antes de clearSession; falha de rede tratada como sucesso", done: true },
      { id: "middleware-auth-skip", label: "auth.global.ts pula ensureSession em rotas /auth/*", done: true, note: "Mata cascata de ensureSession na tela de login pós-logout." },
      { id: "bootstrap-parallel", label: "core/account.ts paraleliza /v2/me/accounts + /v2/me/context speculativo via Promise.all", done: true, note: "Quando o accountId do cookie bate com a lista, salva 1 round-trip no bootstrap." },
      { id: "principal-cache", label: "TTL em memória para Principal (sync.Map/ristretto) com invalidação por eventos — sessão, role, permission, account_modules", done: true, note: "Concluído em multitenant-completion/C6: PrincipalCache[auth.Principal] em platform/httpapi/principal_cache.go (TTL 2 min, invalidação por 4 eventos). principalCacheAdapter quebra ciclo de importação." },
      { id: "session-revogation", label: "JWT carrega sessionId + middleware checa core.user_sessions.revoked_at", done: true, note: "C6 criou o SessionRepository + claim `sid`, MAS o setter não era chamado no boot (sessão nunca revogava — logout era no-op). Ligado de verdade em P0·2 (2026-06-07): SetSessionRepository no app.go + logout revoga + Authenticate checa IsRevoked por request. PrincipalCache (perf) fica para quando escalar." },
      { id: "redis-cache", label: "Trocar PrincipalCache para Redis (pré-produção full)", done: false, note: "Deferido para quando subir produção com múltiplas instâncias." }
    ],
    verifiable: "Login < 500ms; navegação entre páginas sem latência perceptível; logout < 200ms sem bugs."
  },
  {
    id: "fase-8",
    code: "Fase 8",
    title: "Split CRM + Queue",
    goal: "Separar fila-atendimento (queue) de dados/dashboards CRM (ERP + catalog) em módulos independentes.",
    status: "done",
    estimateWeeks: "2–3 semanas (bloqueada até Fase 7 entregar)",
    startedAt: "2026-05-29",
    finishedAt: "2026-06-01",
    tasks: [
      { id: "migration-crm", label: "Migration 0108 — schema crm + mover erp_* de queue.* → crm.* (views compat em public.*)", done: false, note: "Deferido: dados ainda em queue.*, Go packages já separados. Migration de schema crm fica para fase de isolamento de dados." },
      { id: "module-crm", label: "back/internal/modules/crm/ com erp/ + catalog/ implementando interface Module", done: true, note: "Concluído em multitenant-completion/C5: crm/module.go + crm/erp/ + crm/catalog/." },
      { id: "resolver-crm", label: "crm.Resolver registrado em Dependencies para queue consumir opcionalmente", done: true, note: "Concluído em multitenant-completion/C5: CatalogAdapter + ErrCRMNotEnabled; registry.MustRegister(crm.New()) em app.go." },
      { id: "module-queue", label: "Consolidar back/internal/modules/queue/ — operations + alerts + analytics + reports + feedback + consultants + settings", done: true, note: "Concluído em multitenant-completion/C4: queue/module.go com 8 permissões + 2 role templates." },
      { id: "catalog-adapter", label: "queue/catalog_adapter.go — usa crm.Resolver se habilitado, senão entidade local", done: true, note: "Concluído em multitenant-completion/C5: CatalogResolver interface + CatalogAdapter com fallback local." },
      { id: "layer-crm", label: "web/layers/crm/ com nav.config.ts + pages /crm, /erp", done: false, note: "Deferido: pages /crm, /erp ainda em web/layers/queue/. Mover quando CRM ganhar layer próprio." },
      { id: "nav-queue-cleanup", label: "Remover /crm, /erp do web/layers/queue/nav.config.ts", done: false, note: "Deferido: depende de layer-crm." },
      { id: "docs", label: "AGENT.md de crm, queue + CONTRACT_FREEZE.md", done: true, note: "Concluído em multitenant-completion/C5: AGENT.md para crm e queue criados; CONTRACT_FREEZE.md atualizado." }
    ],
    verifiable: "CRM autônomo + queue consome CRM opcionalmente. Desabilitar CRM em core.account_modules: nav /crm e /erp somem; queue.catalog faz fallback local."
  },
  {
    id: "fase-9",
    code: "Fase 9",
    title: "UX de loading / feedback visual",
    goal: "Nunca deixar o usuário olhando para nada. Loading sempre presente (overlay global, skeleton da página, spinner local) mesmo na primeira carga.",
    status: "in_progress",
    estimateWeeks: "3–5 dias (paralela à Fase 7)",
    startedAt: "2026-05-11",
    tasks: [
      { id: "loading-overlay", label: "CoreLoadingOverlay.vue — barra de progresso no topo + leve fade durante navegação e bootstrap", done: true, note: "Montado em app.vue; hooks page:start/page:finish ativam em mudança de rota." },
      { id: "skeleton", label: "CoreSkeleton.vue com variantes (card / table-row / text / avatar / block) e shimmer animation", done: true },
      { id: "use-loading", label: "useCoreLoading() — contador global push/pop; api-client.ts dispara em requests > 200ms", done: true, note: "Plugin loading-bridge.client.ts conecta store ao api-client (evita dependência circular)." },
      { id: "apply-login", label: "Aplicar overlay no fluxo de login/bootstrap (sumiu quando context carregou)", done: true, note: "Coberto automaticamente: api-client dispara overlay > 200ms; hook page:start/finish cobre a navegação pós-login." },
      { id: "apply-dashboard", label: "Skeleton dos cards no dashboard inicial (/)", done: false },
      { id: "apply-operacao", label: "Skeleton da grid de stores + fila em /operacao enquanto realtime conecta", done: false },
      { id: "apply-tables", label: "Skeleton rows em tabelas grandes (clientes, usuários, relatórios) + loading inline na paginação", done: true, note: "AppEntityGrid.vue usa CoreSkeleton variant=table-row count=6; propaga para todas as workspaces que usam o grid (clientes, usuários, ERP, relatórios, etc.)." },
      { id: "apply-switch", label: "Overlay durante AccountSwitcher trocar account (/v2/me/context da nova account)", done: true, note: "CoreAccountSwitcher.select() chama useCoreLoadingStore.push('Trocando de account...') antes do switchAccount." },
      { id: "empty-state", label: "CoreEmptyState.vue padronizado (ícone + título + descrição + ação opcional)", done: true },
      { id: "error-state", label: "CoreErrorState.vue padronizado com botão de retry (mensagem amigável, sem stack)", done: true },
      { id: "replace-hardcoded", label: "Substituir mensagens hardcoded de 'Sem dados' / 'Erro ao carregar' pelos componentes novos", done: false }
    ],
    verifiable: "Nenhuma página fica em branco em qualquer transição. Tempo até primeiro pixel renderizado < 300ms mesmo na primeira carga. AccountSwitcher mostra overlay até o novo context chegar."
  },
  {
    id: "fase-10",
    code: "Fase 10",
    title: "Inventário do front de referência + design system",
    goal: "Mapear o front em web-reference/, usar o design system/temas trazido de lá como referência e preservar o visual atual até os módulos novos entrarem.",
    status: "done",
    estimateWeeks: "Concluída",
    startedAt: "2026-05-12",
    finishedAt: "2026-05-12",
    tasks: [
      { id: "reference-folder", label: "web-reference/ presente e fora do build do Nuxt via .gitignore", done: true, note: "Pasta de leitura/análise; não entra no bundle do app atual." },
      { id: "inventory", label: "docs/COMPONENT_INVENTORY.md — inventário de componentes, páginas, props/eventos, dependências e destino provável por módulo", done: true, note: "Inventário funcional concluído: 63 componentes, 35 páginas, dependências, candidatos Core e páginas por módulo." },
      { id: "design-system-map", label: "Mapear design system do web-reference: tokens.css, useOmniTheme, useThemeStudio, /admin/themes e app/components/theme/**", done: true, note: "Documentado com Theme Studio, token defaults, page header visibility e dependências Nuxt UI/Tailwind." },
      { id: "tokens-css", label: "Definir adaptação de tokens/variantes usando o design system do front de referência, não o design antigo do projeto atual", done: true, note: "Decisão: não trocar tokens globais atuais agora; páginas novas usam tokens do web-reference e pontes CSS só entram quando houver necessidade real." },
      { id: "preserve-current", label: "Preservar visual e componentes das páginas atuais por enquanto; não substituir selects/tabelas/modais existentes nesta fase", done: true, note: "Atualização de design das páginas atuais fica para depois da migração dos módulos." },
      { id: "new-pages-visual", label: "Novas páginas vindas do outro projeto entram com o visual delas dentro do layer do módulo correspondente", done: true, note: "Finance, tasks e omni foram mapeados como primeiras entradas; manager/clientes e users ficam como overlap futuro." },
      { id: "page-decisions", label: "Criar lista de decisão por página: permanece atual, será removida/deprecada, ou receberá update de design depois", done: true, note: "Tabela de decisão por página atual adicionada em docs/COMPONENT_INVENTORY.md." },
      { id: "core-candidates", label: "Portar para web/layers/core/components/ com prefixo Core apenas componentes realmente compartilhados ou necessários ao shell/design system", done: true, note: "Candidatos listados; decisão é não promover para Core antes de reuso real em mais de um módulo." },
      { id: "module-components", label: "Componentes específicos migram junto com cada módulo na Fase 6 (finance, tasks, omni, site, bio, ...)", done: true, note: "Escopo transferido para a Fase 6 como regra de execução; Finance não será o primeiro módulo." }
    ],
    verifiable: "Inventário revisado; seção de design system/temas documentada; front atual preservado; páginas novas migram com o visual do web-reference; decisões por página registradas antes de qualquer troca visual ampla."
  },
  {
    id: "fase-11",
    code: "Fase 11",
    title: "Design System / Theme Studio",
    goal: "Portar a página de temas antes dos módulos para que qualquer tela nova responda corretamente a light/dark/apple/custom, tokens e overrides.",
    status: "done",
    estimateWeeks: "3-5 dias",
    startedAt: "2026-05-12",
    finishedAt: "2026-05-12",
    tasks: [
      { id: "theme-core", label: "Trazer useOmniTheme.ts para o layer core/design-system com inicialização global no app", done: true, note: "Plugin client inicializa tema/overrides a partir do localStorage." },
      { id: "theme-studio", label: "Trazer useThemeStudio.ts, /admin/themes e components/theme/** para uma rota dev-only /themes", done: true, note: "Rota /themes no layout dashboard e menu dev/admin." },
      { id: "tokens", label: "Unificar tokens do web-reference com omni-design-system.css sem quebrar shell atual", done: true, note: "Tokens light, dark, apple e custom conectados a aliases legados." },
      { id: "page-header", label: "Restaurar AdminPageHeader com visibilidade controlada por tema", done: true },
      { id: "shell-bridge", label: "Fazer dashboard/sidebar/header atuais consumirem os tokens ou terem ponte visual compatível", done: true, note: "Header e sidebar usam variaveis admin-header/theme." },
      { id: "module-proof", label: "Validar /themes e /tasks nos temas light, dark, apple e custom sem contraste quebrado", done: true, note: "Validado via Docker 3003: /themes aplica light/dark/apple/custom; /tasks legivel em dark; sem warnings de console apos restart." }
    ],
    verifiable: "Theme Studio aplica e persiste tema; trocar tema altera tokens globais; /tasks fica visualmente consistente e legível em todos os temas; rota/menu ficam dev-only."
  },
  {
    id: "fase-12",
    code: "Fase 12",
    title: "Tasks Orchestrator / Notion-like",
    goal: "Evoluir /tasks de uma tela de tarefas para um orquestrador front-first de paginas, views e itens configuraveis, usando o template visual do web-reference antes de criar o backend.",
    status: "in_progress",
    estimateWeeks: "1-2 semanas",
    startedAt: "2026-05-12",
    tasks: [
      { id: "phase12-brief", label: "Documentar conceito Tasks Orchestrator: paginas notion-like, views, campos, cards, tabela, modal e colunas configuraveis", done: true, note: "Criado docs/TASKS_ORCHESTRATOR_PHASE12.md para continuidade por agentes." },
      { id: "frontend-layer", label: "Criar web/layers/tasks/ com pagina, composable, types, store local e componentes importados do web-reference", done: true, note: "Layer /tasks existe e esta no Nuxt extends." },
      { id: "reference-page", label: "Portar base visual de web-reference/app/pages/admin/tasks.vue preservando Nuxt UI e tokens do Theme Studio", done: true, note: "Port inicial validado em /tasks no Docker 3003." },
      { id: "shared-components", label: "Trazer OmniDataTable e OmniSelectMenuInput localmente, sem promover cedo demais para Core", done: true },
      { id: "dev-access", label: "Habilitar /tasks no menu/rota apenas para acesso dev/admin inicial", done: true },
      { id: "workspace-model", label: "Trocar modelo mental de projeto/tarefa para page/template/view/field/item, mantendo o nome inicial Tasks", done: true, note: "Base front-first adicionou columns, fields e views mantendo compatibilidade com TaskProject/TaskItem enquanto migra." },
      { id: "page-switcher", label: "Permitir criar mais de uma pagina/base usando o mesmo template, com selecao e configuracao por pagina", done: true, note: "Seletor/criador de pagina usa o antigo seletor de projeto, ja renomeado na UI." },
      { id: "view-config", label: "Configurar views board/tabela: nome, tipo, agrupamento, ordenacao, filtros e campos visiveis", done: true, note: "Configuracao front-first permite agrupar board por status/responsavel/cliente/tipo/prioridade, ocultar grupos/contagem e escolher campos visiveis." },
      { id: "field-schema", label: "Definir campos editaveis por pagina: texto, select, pessoa, cliente, data, prioridade, status, numero e checkbox", done: true, note: "Schema padrao de campos esta no estado da pagina; criacao de tipos custom fica para a API/modelo final." },
      { id: "board-columns", label: "Colunas configuraveis: renomear, colorir, reordenar por drag, adicionar/remover e mapear itens ao excluir", done: true, note: "Colunas agora sao objetos com id/label/color/order; rename propaga status dos cards." },
      { id: "inline-board", label: "Editar dados direto no card usando OmniSelectMenuInput e inputs inline; abrir modal somente no clique neutro do card", done: true, note: "Titulo, status, responsavel, cliente, tipo, prioridade e data editam no card com click.stop." },
      { id: "column-actions", label: "Adicionar botao de criar item na coluna, menu de edicao da coluna e movimentacao de cards/colunas", done: true, note: "Edicao de coluna agora fica em popover; drag de coluna usa handle separado do drag de cards." },
      { id: "table-inline", label: "Tabela com edicao inline, colunas configuraveis, ordenacao e exibicao alinhadas com a view ativa", done: true, note: "Tabela usa a view para colunas visiveis, cria nova linha com foco e edita titulo/descricao/status/responsavel/cliente/tipo/prioridade/data/arquivada." },
      { id: "card-layout", label: "Configurar quais campos aparecem no card, ordem, labels, badges, cores e densidade", done: true, note: "Card respeita campos visiveis, esconde campos vazios fora do foco e abre modal apenas em clique neutro." },
      { id: "modal-layout", label: "Configurar quais campos aparecem no modal e em quais secoes; implementar modal depois do board/tabela", done: true, note: "Modal respeita campos visiveis, tem modos lado a lado/central/pagina inteira, resize lateral e editor rico TipTap para textos longos, imagens, HTML, emojis, links e mencoes." },
      { id: "external-layout", label: "Mover /tasks e novos modulos fora de fila-atendimento para layout externo full-width igual ao front de referencia", done: false, note: "Essas paginas nao devem ficar presas na sidebar/layout operacional da fila." },
      { id: "full-editor", label: "Evoluir editor para componente completo reutilizavel: scroll interno, toolbar fixa, drag por bloco, botao +, slash menu, mention menu e bubble menu", done: true, note: "Criado OmniEditor em app/components/omni com UEditor/Nuxt UI, toolbar fixa, bubble toolbar, drag handle, botao +, slash menu, @ pessoas, # clientes/tasks, emoji menu, upload/URL de imagem, HTML e pagina /editor; modal de tasks passou a usar o componente." },
      { id: "front-persistence", label: "Persistir pages/views/fields/items em localStorage estruturado para fechar UX antes do backend", done: true, note: "Persistencia local migrada com columns/fields/views e fallback para dados antigos." },
      { id: "split-components", label: "Quebrar tasks.vue (2955 linhas) em 5 sub-componentes + useTasksPageContext via provide/inject", done: true, note: "TasksFilterBar, TasksBoardView, TasksTableView, TasksProjectSettings, TasksTaskModal. tasks.vue ficou com 832 linhas totais no estado atual, com template fino e CSS global prefixado tasks-page__ / tasks-toolbar__." },
      { id: "agent-docs", label: "Criar AGENT.md para tasks, notifications, realtime e web/layers/tasks antes de qualquer backend", done: true, note: "back/internal/modules/tasks/AGENT.md (scopedQuery, BuildTaskDTO, 13 perms, 3 roles, 30+ endpoints); notifications/AGENT.md (adapter pattern, MVP in-app, stubs email/wpp/push); realtime/AGENT.md atualizado (6 topicos WS, PresenceStore); web/layers/tasks/AGENT.md (migracao localStorage, composables T2-T7)." },
      { id: "backend-deferred", label: "Criar back/internal/modules/tasks/ com migration 0108, API Go, RBAC e realtime (Fases T1-T9)", done: true, note: "T1 e T2 entregues: migration 0108, módulo Go tasks no Registry, RBAC declarativo, endpoints REST/tracking básicos e realtime WS/presence/notifications. Próxima ação: T3 notifications." }
    ],
    verifiable: "Em /tasks, criar paginas, views e itens; configurar board/tabela/card/modal; editar inline; mover cards e colunas; trocar agrupamento/ordenacao; tudo persistindo no front antes da API Go."
  },
  {
    id: "fase-13",
    code: "Fase 13",
    title: "Módulo Omni",
    goal: "Trazer omnichannel/inbox com suas páginas, realtime e auditoria em um layer próprio.",
    status: "pending",
    estimateWeeks: "2-4 semanas",
    tasks: [
      { id: "backend", label: "Criar back/internal/modules/omni/ com schema omni.*, canais, conversas, mensagens, contatos vinculados e auditoria", done: false },
      { id: "dependencies", label: "Adicionar dependências necessárias do módulo, como socket.io-client e emoji-mart, somente nesta fase", done: false },
      { id: "pages", label: "Portar páginas admin/omnichannel: index, inbox, operacao, auditoria e docs conforme decisão de produto", done: false },
      { id: "components", label: "Portar OmnichannelInboxModule.vue e componentes de inbox/chat/composer/anexos/sessão", done: false },
      { id: "realtime", label: "Integrar realtime de conversas ao backend Go, sem depender do BFF mock do web-reference", done: false },
      { id: "acceptance", label: "Enviar/receber mensagem em conta piloto; auditoria registra ação; módulo desabilitado bloqueia rotas", done: false }
    ],
    verifiable: "Inbox abre, lista conversas, envia mensagem de teste, recebe atualização realtime e respeita permissões do módulo."
  },
  {
    id: "fase-13a",
    code: "Fase 13A",
    title: "Lote simples do front",
    goal: "Executar primeiro as páginas mais simples vindas do web-reference para ganhar gestão rápida no front novo sem substituir o legado operacional da fila.",
    status: "blocked",
    estimateWeeks: "1-2 semanas",
    startedAt: "2026-05-25",
    blockers: ["multitenant-completion"],
    tasks: [
      { id: "theme-baseline", label: "Usar Theme Studio já concluído como base visual do lote simples, sem reabrir o escopo de temas", done: true, note: "Fase 11 concluída; o foco agora é aplicar o visual base nas páginas simples novas." },
      { id: "profile", label: "Trazer primeiro o ajuste de profile, reaproveitando /perfil atual e aproximando layout/fluxo do admin/profile.vue", done: false },
      { id: "team", label: "Trazer Team antes de finance, começando por treinamento e candidatos como recorte inicial", done: false },
      { id: "site", label: "Trazer Site antes de finance, começando por produtos e leads com escopo front-first", done: false, note: "AUDITORIA 2026-05-28: páginas SiteLeadsAdminWorkspace.vue e SiteProductsAdminWorkspace.vue criadas. P0·5 (2026-06-07): backend site REGISTRADO no boot — /v1/admin/leads, /products, /tracking-analytics, webhook-sources e ingest /v1/webhooks/* agora servem de verdade (antes 404). Falta confirmar que o front consome o real e não o BFF Nitro." },
      { id: "users-parallel", label: "Abrir frente nova de usuários no front novo reaproveitando UsersWorkspace, sem remover /usuarios legado da fila", done: false },
      { id: "clients-parallel", label: "Abrir frente nova de clientes/tenants no front novo mantendo /clientes legado intacto até fechar a estratégia tenant", done: false, note: "AUDITORIA 2026-05-28: tela manage/clientes-web.vue + composable useClientsManager.ts já existem, mas batem no BFF mock /api/admin/clients (web/server/utils/clients-repository.ts in-memory). Não é fonte de verdade. Será reescrito contra API Go real na multitenant-completion." },
      { id: "sequencing", label: "Deixar finance e demais módulos pesados para depois do lote simples validado no painel", done: false }
    ],
    verifiable: "Roadmap deixa explícito o lote simples como próxima onda; /usuarios e /clientes atuais permanecem operacionais; finance só começa depois desse recorte ser validado."
  },
  {
    id: "fase-14",
    code: "Fase 14",
    title: "Módulo Finance",
    goal: "Trazer financeiro só depois do lote simples do front, mantendo /finance como placeholder até a fase começar.",
    status: "pending",
    estimateWeeks: "2-3 semanas",
    tasks: [
      { id: "sequencing", label: "Iniciar somente após validar profile, team, site e a frente nova de users/clientes no roadmap", done: false },
      { id: "backend", label: "Criar back/internal/modules/finance/ com schema finance.*, lançamentos, categorias, recorrências e ajustes", done: false },
      { id: "frontend-layer", label: "Criar web/layers/finance/ com página admin/finance.vue portada para o path /finance", done: false },
      { id: "components", label: "Portar FinanceLineCard.vue, FinanceRecurringGroupCard.vue e OmniMoneyInput.vue no layer finance", done: false },
      { id: "contacts-integration", label: "Integrar com contacts quando habilitado; usar entidade local quando contacts estiver desligado", done: false },
      { id: "permissions", label: "Declarar permissões finance.read, finance.write, finance.recurring.manage e role templates", done: false },
      { id: "acceptance", label: "Criar lançamento, efetivar recorrência, ajustar valor e consultar histórico via API Go", done: false }
    ],
    verifiable: "/finance deixa de ser placeholder, operações principais persistem no backend e o módulo respeita account_modules."
  },
  {
    id: "fase-15",
    code: "Fase 15",
    title: "Módulo Contacts/Admin",
    goal: "Trazer a nova frente de admin/users/clientes em paralelo ao legado da fila, definindo a fonte de verdade tenant sem quebrar o operacional atual.",
    status: "pending",
    estimateWeeks: "2-3 semanas",
    tasks: [
      { id: "decision", label: "Fixar a regra do rollout: /usuarios e /clientes atuais seguem operacionais; a nova frente admin entra primeiro em paralelo", done: false },
      { id: "backend", label: "Criar back/internal/modules/contacts/ com Resolver consumível por finance, omni, site e queue", done: false },
      { id: "pages", label: "Portar admin/manage/clientes.vue, users.vue e modulos.vue como frente nova de gestão, sem remover os CRUDs atuais da fila", done: false },
      { id: "components", label: "Reaproveitar UsersWorkspace/TenantsWorkspace onde já houver base pronta e portar manager/clients só no que faltar", done: false },
      { id: "account-modules", label: "Mapear admin/manage/modulos.vue para gestão de core.account_modules no futuro", done: false },
      { id: "acceptance", label: "Nova frente admin funciona em paralelo; consumers fazem fallback e o legado da fila segue intacto", done: false }
    ],
    verifiable: "Contacts pode ser habilitado por account, expõe Resolver estável e não substitui /usuarios ou /clientes atuais antes da transição explícita."
  },
  {
    id: "fase-16",
    code: "Fase 16",
    title: "Módulo Site",
    goal: "Trazer páginas de produtos e leads do site/e-commerce no lote simples, antes do financeiro e com acoplamento mínimo ao restante.",
    status: "pending",
    estimateWeeks: "1-2 semanas",
    tasks: [
      { id: "backend", label: "Criar back/internal/modules/site/ com produtos publicados, leads, configurações e permissões", done: false },
      { id: "pages", label: "Portar admin/site/produtos.vue e admin/site/leads.vue para web/layers/site/pages/ como primeira entrega simples do front novo", done: false },
      { id: "contacts-integration", label: "Decidir se leads entram em contacts quando contacts estiver habilitado", done: false },
      { id: "nav", label: "Adicionar nav.config.ts do site e proteger rotas com module-enabled", done: false },
      { id: "acceptance", label: "Cadastrar produto, alternar visibilidade no site e consultar lead via API Go", done: false },
      { id: "tracking-analytics", label: "Dashboard de analytics do tracking: GET /v1/admin/tracking-analytics agrega totais, conversões dinâmicas, dispositivos, eventos por tipo, acessos/dia, origem de tráfego e últimas visitas (C17.1)", done: true, note: "Concluído em multitenant-completion/C17.1 (2026-06-02): backend agrega site.tracking_events (sem migration, índices já existem); SiteTrackingDashboard.vue com toggle Resumo|Eventos no SiteTrackingAdminWorkspace. KPIs genéricos/dinâmicos derivados dos event_names presentes. go build + vue-tsc limpos." }
    ],
    verifiable: "Produtos e leads funcionam no layer site, com fallback claro para contacts e sem afetar /crm ou /erp. Tela Site→Tracking mostra dashboard agregado (não só grid crua)."
  },
  {
    id: "fase-17",
    code: "Fase 17",
    title: "Módulo Indicators",
    goal: "Separar indicadores como módulo próprio ou acoplar conscientemente a analytics/crm, com decisão antes de portar telas.",
    status: "pending",
    estimateWeeks: "2-3 semanas",
    tasks: [
      { id: "domain-decision", label: "Decidir destino: indicators próprio, analytics ou CRM", done: false },
      { id: "backend", label: "Criar schema e APIs para templates, avaliações, governança, evidências e exportações", done: false },
      { id: "pages", label: "Portar admin/indicadores/index.vue e configuracoes.vue", done: false },
      { id: "components", label: "Portar components/indicators/* e composables useIndicators* necessários", done: false },
      { id: "live", label: "Trocar mocks/live do web-reference por dados reais do backend", done: false },
      { id: "acceptance", label: "Criar avaliação, configurar template, filtrar período e exportar sem dados mockados", done: false }
    ],
    verifiable: "Indicadores rodam com dados persistidos e destino de domínio documentado antes de entrar no menu."
  },
  {
    id: "fase-18",
    code: "Fase 18",
    title: "Módulo Tools",
    goal: "Trazer ferramentas utilitárias como módulos pequenos e independentes.",
    status: "pending",
    estimateWeeks: "1-2 semanas",
    tasks: [
      { id: "scope", label: "Separar tools em qrcodes, short-links e scripts ou manter como um módulo tools", done: false },
      { id: "backend", label: "Criar APIs Go para QR Code, encurtador de link e scripts, evitando BFF duplicado", done: false },
      { id: "pages", label: "Portar admin/tools/qr-code.vue, encurtador-link.vue e scripts.vue conforme escopo aprovado", done: false },
      { id: "permissions", label: "Declarar permissões tools.qrcode.*, tools.short_links.* e tools.scripts.*", done: false },
      { id: "acceptance", label: "Gerar QR, criar link curto e listar scripts com persistência real", done: false }
    ],
    verifiable: "Ferramentas funcionam isoladas, com dependências adicionadas só quando cada ferramenta entrar."
  },
  {
    id: "fase-19",
    code: "Fase 19",
    title: "Módulo Team",
    goal: "Trazer Team como parte do lote simples inicial, começando por treinamento/candidatos e deixando escopos mais pesados para depois.",
    status: "pending",
    estimateWeeks: "1-2 semanas",
    tasks: [
      { id: "product-decision", label: "Confirmar o recorte inicial de team: treinamento e candidatos primeiro; escalas detalhadas podem vir depois", done: false },
      { id: "backend", label: "Modelar candidatos, treinamentos, anexos e estados de processo no recorte inicial aprovado", done: false },
      { id: "pages", label: "Portar admin/team/treinamento.vue e candidatos.vue como uma das primeiras entregas simples do front", done: false },
      { id: "files", label: "Definir estratégia para anexos/CVs antes de subir a tela de candidatos", done: false },
      { id: "acceptance", label: "Criar candidato/treinamento e validar permissões por account", done: false }
    ],
    verifiable: "Team entra com recorte claro (treinamento/candidatos), respeita permissões por account e não bloqueia o avanço do lote simples."
  },
  {
    id: "fase-20",
    code: "Fase 20",
    title: "Módulo Bio",
    goal: "Reservar a fase do módulo Bio do plano original, começando por descoberta porque não há página concreta mapeada no web-reference atual.",
    status: "pending",
    estimateWeeks: "A definir",
    tasks: [
      { id: "discovery", label: "Localizar fonte real do módulo Bio ou confirmar que será criado do zero", done: false },
      { id: "scope", label: "Definir escopo: links, perfil público, temas, analytics e integrações com site/contacts", done: false },
      { id: "backend", label: "Criar back/internal/modules/bio/ e schema bio.* quando o escopo estiver fechado", done: false },
      { id: "frontend-layer", label: "Criar web/layers/bio/ somente após existir fonte visual ou especificação", done: false },
      { id: "acceptance", label: "Página pública bio renderiza, salva links e respeita módulo habilitado por account", done: false }
    ],
    verifiable: "Bio não começa no escuro: a fase só vira implementação após descoberta ou especificação validada."
  },

  // ─── Tasks Orquestrador — Backend ──────────────────────────────────────────

  {
    id: "tasks-t0",
    code: "Tasks T0",
    title: "Documentação prévia",
    goal: "AGENT.md para tasks/notifications/realtime/web-layer + lane no roadmap antes de qualquer código de backend.",
    status: "done",
    estimateWeeks: "< 1 dia",
    startedAt: "2026-05-14",
    finishedAt: "2026-05-14",
    group: "tasks-backend",
    tasks: [
      { id: "roadmap-lane", label: "Adicionar lane 'Tasks Orquestrador' em roadmap-data.ts com 11 cards", done: true },
      { id: "agent-tasks", label: "Criar back/internal/modules/tasks/AGENT.md (escopo, HTTP, regras de scope, WS)", done: true },
      { id: "agent-notifications", label: "Criar back/internal/modules/notifications/AGENT.md (MVP in-app, adapters futuros)", done: true },
      { id: "agent-realtime", label: "Atualizar back/internal/modules/realtime/AGENT.md com canais Tasks/Presence/Notifications", done: true },
      { id: "agent-web", label: "Criar web/layers/tasks/AGENT.md (migração localStorage → backend, composables novos)", done: true }
    ],
    verifiable: "/roadmap renderiza a nova lane com 11 cards; T0/T0.5/T1/T2 ficam concluídas e T3+ seguem pendentes; AGENT.md de cada módulo afetado existe."
  },
  {
    id: "tasks-t05",
    code: "Tasks T0.5",
    title: "Quebrar tasks.vue",
    goal: "Extrair os 2955 linhas de tasks.vue em 6 sub-componentes + useTasksPageContext antes de plugar o backend.",
    status: "done",
    estimateWeeks: "1 dia",
    startedAt: "2026-05-14",
    finishedAt: "2026-05-14",
    group: "tasks-backend",
    tasks: [
      { id: "context", label: "Criar useTasksPageContext.ts com todo o estado/lógica (provide/inject)", done: true },
      { id: "filter-bar", label: "Extrair TasksFilterBar.vue (toolbar, filtros, troca de view)", done: true },
      { id: "board-view", label: "Extrair TasksBoardView.vue (colunas, cards, drag/drop)", done: true },
      { id: "table-view", label: "Extrair TasksTableView.vue (wrapper OmniDataTable)", done: true },
      { id: "settings", label: "Extrair TasksProjectSettings.vue (slideover de configuração)", done: true },
      { id: "modal", label: "Extrair TasksTaskModal.vue (slideover de detalhe da task)", done: true },
      { id: "tasks-vue", label: "Reescrever tasks.vue como wrapper fino (832 linhas totais no estado atual; template enxuto)", done: true }
    ],
    verifiable: "/tasks renderiza identicamente ao antes; tasks.vue saiu de ~2955 para 832 linhas totais, com estado/lógica extraídos para useTasksPageContext e sub-componentes."
  },
  {
    id: "tasks-t1",
    code: "Tasks T1",
    title: "Schema multi-tenant + módulo Go",
    goal: "Migration 0108_tasks_schema_foundation.sql (17 tabelas) + módulo Go com scopedQuery, BuildTaskDTO, RBAC e endpoints REST.",
    status: "done",
    estimateWeeks: "6–8 dias",
    startedAt: "2026-05-14",
    finishedAt: "2026-05-14",
    group: "tasks-backend",
    tasks: [
      { id: "migration", label: "Migration 0108: schema tasks.* com 17 tabelas, índices, constraints", done: true },
      { id: "model", label: "tasks/model.go: Board, Column, Field, Task, TimeEntry, Comment, Relation, Share, Perspective", done: true },
      { id: "repository", label: "tasks/repository_postgres.go: scopedQuery (panic sem accountID) + CRUD base", done: true },
      { id: "service-dto", label: "tasks/service_dto.go: BuildTaskDTO(task, perspective) — client_viewer omite campos de agência", done: true },
      { id: "service", label: "tasks/service.go + service_tracking.go: CRUD boards/tasks/comments/shares/relations/tracking + audit log helper", done: true },
      { id: "http", label: "tasks/http.go + http_tracking.go: endpoints REST básicos com RequireAuth + withPermission", done: true },
      { id: "module", label: "tasks/module.go: registrar no Module Registry (13 permissões, 3 role templates)", done: true },
      { id: "permissions", label: "SyncCatalog popula core.permissions e core.role_templates com keys tasks.* quando CORE_V2_ENABLED=true", done: true }
    ],
    verifiable: "go test ./... em back/ passa. Em runtime com CORE_V2_ENABLED=true, SyncCatalog registra tasks; aplicar migration fresh e smoke curl ficam para validação de ambiente/staging."
  },
  {
    id: "tasks-t2",
    code: "Tasks T2",
    title: "Realtime para tasks",
    goal: "Estender back/internal/modules/realtime/ com canais tasks:account, tasks:board, tasks:task e presence sem quebrar operations/context.",
    status: "done",
    estimateWeeks: "2–3 dias",
    startedAt: "2026-05-14",
    finishedAt: "2026-05-14",
    group: "tasks-backend",
    tasks: [
      { id: "service-tasks", label: "realtime/service_tasks.go: HandleTasksSocket, HandlePresenceSocket, HandleNotificationsSocket", done: true },
      { id: "presence", label: "realtime/presence.go: PresenceStore em memória (TTL 30s, heartbeat 15s)", done: true },
      { id: "publisher", label: "realtime/service_tasks.go: implementa tasks.Publisher (PublishTaskEvent, PublishBoardEvent, PublishPresenceEvent)", done: true },
      { id: "events", label: "realtime/model.go: 25+ EventType* novos (task.created, presence.snapshot, notification.created…)", done: true },
      { id: "auth-ws", label: "Autorização do canal: validate accountID + tasks.tasks.view/tasks.client_view antes do upgrade WS", done: true }
    ],
    verifiable: "go test ./... em back/ passa. Runtime esperado: mutation REST publica em tasks:account/board/task; WS rejeita cross-account antes do upgrade; presence envia snapshot/join/left/field lock e expira em 30s sem heartbeat."
  },
  {
    id: "tasks-t3",
    code: "Tasks T3",
    title: "Módulo notifications",
    goal: "Migration 0109 + módulo Go com InAppAdapter funcional e stubs email/WhatsApp/push; triggers internos em tasks (assign, mention, move).",
    status: "done",
    estimateWeeks: "2–3 dias",
    startedAt: "2026-05-14",
    finishedAt: "2026-05-14",
    group: "tasks-backend",
    tasks: [
      { id: "migration", label: "Migration 0109: schema notifications.* (user_notifications, channels, delivery_log, mutes)", done: true },
      { id: "adapter", label: "InAppAdapter: persiste user_notifications, publica notification.created/read e usa o canal notifications:user:{userId}", done: true },
      { id: "stubs", label: "Stubs EmailAdapter, WhatsAppAdapter, PushAdapter retornam ErrNotConfigured", done: true },
      { id: "triggers", label: "tasks/service.go: assign/status-change/comment mention|subscriber/move disparam notifications sem bloquear a mutation", done: true },
      { id: "endpoints", label: "GET /v1/notifications, POST read, mark-all-read, preferences e mute", done: true }
    ],
    verifiable: "go test ./... em back/ passa. Runtime esperado: assign/comment/move gravam notifications.user_notifications, InAppAdapter publica notification.created/read em notifications:user:{userId}, mute TTL silencia resourceType/resourceId e stubs externos seguem retornando ErrNotConfigured."
  },
  {
    id: "tasks-t4",
    code: "Tasks T4",
    title: "Registry de resolvers cross-module",
    goal: "Interface RelationResolver em platform/modules/ + implementações em crm, erp, operations; endpoint /relations:expand com cache 60s.",
    status: "done",
    estimateWeeks: "2 dias",
    startedAt: "2026-05-14",
    finishedAt: "2026-05-14",
    group: "tasks-backend",
    tasks: [
      { id: "interface", label: "platform/modules/relations.go: RelationRegistry + RelationResolver + RelationRef/Result", done: true },
      { id: "crm-resolver", label: "erp/relations_resolver.go: alias crm resolve contact e lead sobre ERP raw", done: true },
      { id: "erp-resolver", label: "erp/relations_resolver.go: resolver bulk para customer, employee, order e record", done: true },
      { id: "ops-resolver", label: "operations/relations_resolver.go: resolver para service_history com fallback active", done: true },
      { id: "endpoint", label: "GET /v1/tasks/:id/relations:expand — resolve por modulo, atualiza label_cache/metadata_cache/refreshed_at (TTL 60s)", done: true }
    ],
    verifiable: "go test ./... em back/ passa. GET /v1/tasks/:id/relations:expand resolve por modulo, reaproveita cache fresco por 60s e grava label/url/status em metadata_cache; recurso fora da account retorna status='unknown'."
  },
  {
    id: "tasks-t5",
    code: "Tasks T5",
    title: "Front: localStorage → backend",
    goal: "Substituir useTasksWorkspace (localStorage) pelo Pinia store + API Go; wipe do storage legado com aviso single-shot.",
    status: "done",
    estimateWeeks: "7–10 dias",
    startedAt: "2026-05-14",
    finishedAt: "2026-05-14",
    group: "tasks-backend",
    tasks: [
      { id: "store", label: "web/layers/tasks/stores/tasks.ts: Pinia store com fetchBoards, fetchBoard, createTask, moveTask, applyRealtimeEvent", done: true },
      { id: "realtime", label: "useTasksRealtime.ts: clone de useOperationsRealtime, tópicos tasks:account + tasks:board, reconexão exponencial", done: true },
      { id: "tracking", label: "useTaskTracking.ts: server-backed, clockOffset, tick local 1s", done: true },
      { id: "relations", label: "useTaskRelations.ts: lazy load + cache + re-fetch em task.relation_added", done: true },
      { id: "can", label: "useCan.ts: computed contra useMeContext().permissions", done: true },
      { id: "wipe", label: "Boot detecta localStorage legado (omni.admin.tasks.workspace.v1) e descarta com aviso", done: true },
      { id: "pagination", label: "Paginação cursor-based (backend keyset + front loop ate esgotar); infinite scroll na table view fica para T5.1", done: true },
      { id: "client-view", label: "Perspective derivada de permissões reais; servidor filtra; front não esconde dados", done: true }
    ],
    verifiable: "/tasks carrega via REST, zero localStorage; drag → REST+WS < 300ms; F5 mantém estado; client_viewer vê só boards com share."
  },
  {
    id: "tasks-t6",
    code: "Tasks T6",
    title: "Tracking server-side autoritativo",
    goal: "StartTracking/PauseTracking/ResumeTracking/StopTracking persistidos no banco; timer sincronizado por WS com clockOffset.",
    status: "done",
    estimateWeeks: "2–3 dias",
    startedAt: "2026-05-14",
    finishedAt: "2026-05-14",
    group: "tasks-backend",
    tasks: [
      { id: "service", label: "tasks/service_tracking.go: 6 métodos + partial unique (user_id, task_id) WHERE stopped_at IS NULL", done: true },
      { id: "optimistic-lock", label: "version check em transação com FOR UPDATE; ErrVersionConflict → 409", done: true },
      { id: "ws-events", label: "task.time_started/paused/resumed/stopped publicados no WS após cada mutation", done: true },
      { id: "frontend", label: "useTaskTracking.ts: tick local com serverOffset; reconcilia via WS; modal + card mostram timer", done: true }
    ],
    verifiable: "User A inicia → User B vê timer correndo; servidor reinicia → timer correto; máquina travada 5min → valor real do server."
  },
  {
    id: "tasks-t7",
    code: "Tasks T7",
    title: "Presence (avatares + field locking)",
    goal: "PresenceStore em memória com TTL 30s; protocolo heartbeat/field_focus/field_blur; front exibe avatares e badge 'editando campo X'; lock exclusivo por campo e identidade via nick.",
    status: "in_progress",
    estimateWeeks: "2–3 dias",
    group: "tasks-backend",
    tasks: [
      { id: "presence-store", label: "realtime/presence.go: PresenceStore TTL 30s + ticker de limpeza + publish presence.user_left", done: true },
      { id: "protocol", label: "Protocolo cliente→server: presence.heartbeat, field_focus, field_blur", done: true },
      { id: "frontend", label: "useTaskPresence.ts: abre presence:task:{id} ao abrir modal; heartbeat 15s", done: true },
      { id: "badge", label: "Front exibe badge 'Fulano editando título' (trava input com :disabled enquanto outro user está nele)", done: true },
      { id: "future-yjs", label: "tasks.task_doc_snapshots vazia criada para futuro cursor Y.js/Tiptap", done: false },
      { id: "lock-exclusivo", label: "presence.go: LockField recusa lock se outro user já está no fieldKey e republica o lock atual para recuperar clients defasados", done: true },
      { id: "nick-infra", label: "Migration 0111 + Nick em User/Principal/UserView/tokens; users module e presence priorizam nick (fallback display_name)", done: true },
      { id: "input-espaco", label: "Front: clampText (slice puro) vs normalizeText (trim+colapso) — permite espaço no final ao renomear card inline", done: true },
      { id: "presence-sem-guard-local", label: "useTaskPresence: sem guard local por usersForField; envia field_focus, preserva lock em reconnect e libera em blur/aba oculta", done: true },
      { id: "board-realtime", label: "useTasksRealtime: assinar account + board ativo; canal board garante sync entre usuarios em escopos/contas diferentes", done: true },
      { id: "patch-local-realtime", label: "useTasksRealtime: manter refresh full debounced; patch local/hydrateTask revertido", done: false, note: "REVERTIDO em 2026-05-15. O patch local quebrava sincronizacao entre abas quando a task remota nao existia no store local (caso comum). Voltamos ao padrao do useOperationsRealtime: SEMPRE refresh full debounced (200ms) para qualquer evento task.*/board.*/field.*. Funciona em todos os casos; flicker do board e' aceitavel — operations roda assim ha meses sem queixas." }
    ],
    verifiable: "Abrir modal em 2 abas → avatares mútuos visíveis com nick; focar campo → badge na outra aba; segundo user vê input :disabled e não consegue 'roubar' o lock; renomear card inline com 'palavra ' (espaço no final) funciona; editar campo em uma aba reflete na outra via refresh full debounced."
  },
  {
    id: "tasks-t8",
    code: "Tasks T8",
    title: "Segurança, audit, hardening",
    goal: "Audit log com retention, rate limit WS+REST, validação rigorosa de IDs, defense in depth em 3 camadas confirmada por testes.",
    status: "done",
    estimateWeeks: "2 dias",
    startedAt: "2026-05-15",
    finishedAt: "2026-05-15",
    group: "tasks-backend",
    tasks: [
      { id: "audit-endpoint", label: "GET /v1/tasks/:id/audit (perm tasks.boards.manage); retention 180d para não-premium", done: true, note: "Endpoint pronto na T1; retention 180d fica para job futuro." },
      { id: "rate-limit", label: "WS: 30 events/s por conexão (close 1008); REST: 60 req/min; metrics: 1 req/3s", done: true, note: "WS pronto na T2; REST 60/min via httpapi.RateLimit (token bucket por user+IP); metrics dedicada fica para T9." },
      { id: "validation", label: "Nunca aceitar account_id do body — sempre derivar do Principal; client_account_id via share OU manage", done: true, note: "withPermission resolve accountID do header X-Account-Id → query → principal.TenantID; service ignora campos AccountID dos inputs." },
      { id: "404-not-403", label: "Cross-account → 404 (nunca 403); integration test confirma em todos os endpoints", done: true, note: "ResolveAccessContext devolve ErrAccountNotFound; scopedQuery retorna pgx.ErrNoRows. 403 reservado para perm faltando na mesma account. Integration test em T9." },
      { id: "logs", label: "slog estruturado em cada mutation: accountId, taskId, userId; erros sem IDs de outras accounts", done: true, note: "service.audit() agora dispara slog.LogAttrs com action/account_id/user_id/resource em todas as 13 mutations." }
    ],
    verifiable: "Fuzz 100 IDs de outros tenants → 100% 404 (T9); 70 requisicoes em 60s contra /v1/tasks → a 61a recebe 429 com Retry-After; tail -f no log do app mostra tasks.mutation com account_id/user_id/resource em cada CRUD."
  },
  // ─── CRM 360 — Fila + ERP ─────────────────────────────────────────────────

  {
    id: "crm-c1",
    code: "CRM C1",
    title: "Indicadores por consultor — backend",
    goal: "Novo endpoint que funde dados de atendimento da fila (conversão, cancelamento) com dados de vendas do ERP (faturamento, PA, ticket médio, % meta).",
    status: "in_progress",
    estimateWeeks: "3–5 dias",
    startedAt: "2026-05-21",
    group: "crm-360",
    tasks: [
      { id: "model-queue-stats", label: "Adicionar QueueStats (atendimentos, conversões, taxa conversão, taxa cancelamento fila) ao CRMConsultantMetric e CRMStoreMetric", done: false },
      { id: "query-fusion", label: "Query SQL em repository_crm_aggregates.go que agrega operation_service_history por consultor/loja no período", done: false },
      { id: "service-fusion", label: "service.CRMOverview inclui dados de fila; taxa cancelamento ERP (ordercanceled/order) calculada separadamente", done: false },
      { id: "agent-md", label: "Atualizar back/internal/modules/erp/AGENT.md com novos campos e query de fusão", done: false }
    ],
    verifiable: "GET /v1/erp/crm retorna atendimentos, taxaConversao e taxaCancelamentoFila por consultor; go test ./... passa."
  },
  {
    id: "crm-c2",
    code: "CRM C2",
    title: "Produto não encontrado — modelo e modal",
    goal: "Separar campo 'produto que o cliente queria mas a loja não tinha' de 'produto visto' no modelo da fila. Adicionar distinção de motivo de perda: preço vs falta de estoque.",
    status: "in_progress",
    estimateWeeks: "2–3 dias",
    startedAt: "2026-05-21",
    group: "crm-360",
    tasks: [
      { id: "migration", label: "Migration SQL: adicionar products_not_found_json em operation_service_history", done: false },
      { id: "model-go", label: "Adicionar ProductsNotFound []ProductEntry ao FinishCommandInput e ServiceHistoryEntry em operations/model.go", done: false },
      { id: "store-postgres", label: "Persistir e ler products_not_found_json em operations/store_postgres.go", done: false },
      { id: "frontend-modal", label: "Adicionar seção 'Produto procurado / não encontrado' no OperationFinishModal.vue separada de produtos vistos", done: false },
      { id: "agent-md", label: "Atualizar back/internal/modules/operations/CONCURRENT_SERVICES.md e AGENT.md com novo campo", done: false }
    ],
    verifiable: "Finalizar atendimento com produto não encontrado persiste no banco; histórico retorna o campo; modal mostra seção distinta."
  },
  {
    id: "crm-c3",
    code: "CRM C3",
    title: "Painel CRM — gráficos e 360",
    goal: "ErpCrmWorkspace com gráficos de faturamento, % meta, conversão e cancelamento por consultor e por loja. Cards de indicadores com comparativo.",
    status: "pending",
    estimateWeeks: "4–6 dias",
    group: "crm-360",
    tasks: [
      { id: "ts-types", label: "Atualizar tipos TypeScript no CRM store com novos campos de QueueStats", done: false },
      { id: "chart-consultant", label: "Gráficos por consultor: faturamento, % meta, PA, ticket médio, taxa conversão, taxa cancelamento", done: false },
      { id: "chart-store", label: "Comparativo por loja: mesmos indicadores + ranking", done: false },
      { id: "products-not-found", label: "Seção de produto não encontrado no painel CRM com agrupamento por SKU/motivo", done: false },
      { id: "360-checklist", label: "Aba 360 com indicadores + atendimento + metas + não compra (estrutura inicial)", done: false },
      { id: "agent-md", label: "Atualizar web/app/components/erp/AGENT.md com novos componentes de gráficos", done: false }
    ],
    verifiable: "Painel CRM exibe gráficos com dados reais do novo endpoint; filtro de período funciona; sem erros de console."
  },
  {
    id: "crm-c4",
    code: "CRM C4",
    title: "Consultor gamificado — player card + drawer",
    goal: "Substituir o painel de cards planos da workspace Consultor por player card (gauge dominante + 4 KPIs core + badges) e mover métricas secundárias + simulador para drawer lateral. Visão all-stores vira grid de mini-cards.",
    status: "in_progress",
    estimateWeeks: "4–6 dias",
    startedAt: "2026-05-22",
    group: "crm-360",
    tasks: [
      { id: "agent-md", label: "Atualizar web/app/components/consultant/AGENT.md com contrato dos novos componentes (player card, drawer, grid)", done: false },
      { id: "gamification-config-composable", label: "useGamificationConfig() composable com defaults hardcoded para badges + Score 360 weights (preparado para C6 plugar fonte real)", done: false },
      { id: "player-card-component", label: "ConsultantPlayerCard.vue (modos full e mini) — gauge SVG, 4 KPIs core (Vendido, Ticket, PA, Conversão), slot de badges", done: false },
      { id: "badges-component", label: "ConsultantBadges.vue puro recebe stats + badgesConfig — defaults: Meta batida, Top N, Conversão > média loja, Ticket > meta, PA > meta", done: false },
      { id: "drawer-shell", label: "ConsultantDetailsDrawer.vue com USlideover (modos center/fullscreen/side igual TasksTaskModal) + composable useConsultantDetailsDrawer()", done: false },
      { id: "drawer-tabs", label: "Drawer com 3 tabs: Visão geral (todos KPIs incluindo cancelamento/fora-da-vez/tempo médio), Histórico (sparkline 7d), Simulador (move ConsultantSimulator atual)", done: false },
      { id: "single-store-wire", label: "ConsultantWorkspace.vue (single-store) usa ConsultantPlayerCard full + drawer", done: false },
      { id: "multi-store-grid", label: "ConsultantPlayerGrid.vue substitui tabela 'Comparativo completo' por grid de mini-cards filtráveis", done: false },
      { id: "cancellation-wire", label: "Garantir cancellationRate no DTO consumido por consultants/analytics stores (já calculado em repository_crm_queue.go)", done: false },
      { id: "delete-old-metrics", label: "Remover ConsultantMetrics.vue após migração completa", done: false }
    ],
    verifiable: "/consultor em loja única mostra player card + drawer abrindo via 'Ver detalhes'. Visão all-stores mostra grid de mini-cards; click no card abre drawer. Sem erros de console; npm test passa."
  },
  {
    id: "crm-c5",
    code: "CRM C5",
    title: "Ranking gamificado — pódio + leaderboard + drawer",
    goal: "Substituir as duas tabelas de 11 colunas por pódio dos 3 primeiros + leaderboard de cards horizontais para o resto. Tabs Lojas/Consultores/Por-loja. Score 360 vira sort default. Detalhes (breakdown 360 + alertas) em drawer lateral.",
    status: "in_progress",
    estimateWeeks: "5–7 dias",
    startedAt: "2026-05-22",
    group: "crm-360",
    tasks: [
      { id: "agent-md", label: "Criar web/app/components/ranking/AGENT.md com contrato do novo workspace", done: false },
      { id: "tabs-header", label: "RankingTabsHeader.vue (3 tabs Lojas/Consultores/Por-loja + chips de sort, Score 360 default)", done: false },
      { id: "podium-component", label: "RankingPodium.vue — top-3 visual (2º-1º-3º) com avatar, nome/loja, número grande da métrica ativa", done: false },
      { id: "leaderboard-card", label: "RankingLeaderboardCard.vue — card horizontal (4º+) com posição, métrica grande, barra meta, badge variação ↑/↓", done: false },
      { id: "variation-derivation", label: "Derivar variação vs período anterior client-side (comparar monthlyRows com snapshot mês anterior já disponível)", done: false },
      { id: "stores-tab", label: "Agregação por loja para tab Lojas: totalSoldValue, Score 360 ponderado por attendances (decisão fechada), consultantsAtGoal", done: false },
      { id: "per-store-tab", label: "Tab 'Por loja' com combobox de loja + pódio + leaderboard filtrados", done: false },
      { id: "drawer-ranking", label: "RankingDetailsDrawer.vue com USlideover (center/fullscreen/side) + tabs Visão geral / Breakdown 360 / Alertas. Mover alertas do topo do RankingWorkspace para drawer (manter contador)", done: false },
      { id: "score-breakdown", label: "Componente de barra stackeada para breakdown do Score 360 — pesos vêm de useGamificationConfig() (defaults 35/25/20/15/5 até C6 plugar fonte real)", done: false },
      { id: "legacy-table", label: "Manter RankingTable.vue acessível como 'Ver como tabela' dentro do drawer para usuários que preferem formato denso", done: false }
    ],
    verifiable: "/ranking mostra pódio + leaderboard cards; tabs trocam agrupamento; click no card abre drawer com breakdown 360. ESC/overlay/X fecham. Alertas continuam acessíveis via drawer. npm test passa; sem regressões."
  },
  {
    id: "crm-c6",
    code: "CRM C6",
    title: "Backend de gamificação — config de badges + Score 360 weights",
    goal: "Permitir que cada tenant configure regras de badges (Meta batida, Top N, Conversão > média loja, etc.) e os pesos do Score 360 (Conversão/Valor/Qualidade/PA/Queue-jump). Plugar no composable useGamificationConfig() do front (criado em C4) substituindo defaults hardcoded.",
    status: "pending",
    estimateWeeks: "3–5 dias",
    group: "crm-360",
    tasks: [
      { id: "model-go", label: "Adicionar GamificationConfig (BadgeRules []BadgeRule + ScoreWeights ScoreWeights) ao settings.Bundle e Record", done: false },
      { id: "migration", label: "Migration SQL: tabela settings_gamification (tenant_id, badge_rules_json, score_weights_json)", done: false },
      { id: "store-postgres", label: "Persistir e ler GamificationConfig em settings store_postgres.go", done: false },
      { id: "defaults", label: "settings/defaults.go com defaults de GamificationConfig (mesmos hardcoded usados em C4/C5)", done: false },
      { id: "http-endpoint", label: "PATCH /v1/settings/gamification com perm settings.write; GET expõe junto com o bundle existente", done: false },
      { id: "frontend-settings-ui", label: "Seção 'Gamificação' na página de configurações para editar badges (CRUD lista) e weights (5 sliders que somam 100%)", done: false },
      { id: "wire-composable", label: "useGamificationConfig() passa a ler do settings store (com fallback para defaults se config não existir)", done: false },
      { id: "agent-md", label: "Atualizar back/internal/modules/settings/AGENT.md com novo bundle field e endpoint", done: false }
    ],
    verifiable: "PATCH /v1/settings/gamification persiste; GET /v1/settings retorna gamificationConfig no bundle; UI permite editar badges e weights; após salvar, player cards e ranking refletem mudanças sem recarregar. go test ./... passa."
  },

  {
    id: "crm-c7",
    code: "CRM C7",
    title: "CRM 360 — atribuicao multi-loja e uso da lista",
    goal: "Corrigir atribuicao de vendas ERP para consultores multi-loja e substituir o uso da lista por uma metrica de cobertura que nao passa de 100%.",
    status: "done",
    estimateWeeks: "1 dia",
    startedAt: "2026-06-08",
    finishedAt: "2026-06-08",
    group: "crm-360",
    tasks: [
      { id: "store-attribution", label: "Priorizar loja explicita do ERP e historico dominante antes do cadastro atual do vendedor", done: true },
      { id: "list-usage-contract", label: "Definir cobertura da lista como consultores com atendimentos >= pedidos ERP no periodo", done: true },
      { id: "summary-cards", label: "Remover cards confusos e exibir atendimentos, conversao, uso da lista e cancelamento ERP", done: true },
      { id: "consultant-grid", label: "Trocar coluna percentual de uso por status de cobertura da lista por consultor", done: true },
      { id: "docs-tests", label: "Atualizar AGENT/docs e cobrir calculos com testes", done: true }
    ],
    verifiable: "Maio/2026 separa vendas por loja de consultores multi-loja; card Uso da lista mostra cobertura 0-100%; tabela destaca Coberto/Parcial/Sem uso; go test do pacote CRM e testes web de util passam."
  },

  {
    id: "crm-c8",
    code: "CRM C8",
    title: "CRM 360 — politica comercial de lista e recebimento",
    goal: "Evitar falsos destaques quando uso da lista esta ruim, configurar faixas de cobertura e mostrar recebimento por atingimento de meta na grade de consultores.",
    status: "done",
    estimateWeeks: "1 dia",
    startedAt: "2026-06-08",
    finishedAt: "2026-06-08",
    group: "crm-360",
    tasks: [
      { id: "list-rankings", label: "Trocar melhor loja/consultor por diagnostico quando todos estao abaixo da faixa Normal", done: true },
      { id: "config-tab", label: "Criar aba Metas CRM para faixas de uso da lista e recebimento por meta", done: true },
      { id: "settings-contract", label: "Persistir politica comercial nas settings de operacao com edicao para platform_admin e director", done: true },
      { id: "consultant-payout", label: "Adicionar recebimento na grade de consultores calculado pela meta da loja", done: true },
      { id: "docs-tests", label: "Atualizar AGENT/docs e cobrir calculos com testes", done: true }
    ],
    verifiable: "Cards nao exibem melhor loja/consultor como premio quando tudo esta ruim; Configuracoes > Metas CRM edita faixas e recebimentos; coluna Recebimento aparece na grade; testes crm-list-usage/crm-performance-policy e settings passam."
  },

  {
    id: "roadmap-b1",
    code: "Roadmap B1",
    title: "Backend de Modulos & Regras editaveis",
    goal: "Persistir RoadmapModule e RoadmapRule via API Go (schema novo roadmap.*). UI passa de read-only para edicao inline com workspace de prioridade, status e descricao. Regenera AGENT_RULES.md a cada PUT para que agentes leiam sempre a versao canonica.",
    status: "done",
    estimateWeeks: "3-5 dias",
    startedAt: "2026-05-23",
    finishedAt: "2026-05-23",
    group: "multi-tenant",
    tasks: [
      { id: "migration", label: "Migration 0115_roadmap_schema.sql: schema roadmap + tabelas modules e rules (account-scoped + global)", done: true, note: "Constraints check em status/priority/category; index parcial para registros globais." },
      { id: "module-go", label: "back/internal/modules/roadmap/ (model/store_postgres/service/http/AGENT.md) seguindo padrao", done: true, note: "Tipo de dominio nomeado ModuleRecord para nao colidir com modules.Module do registry." },
      { id: "endpoints", label: "GET /v1/roadmap/modules, PUT /v1/roadmap/modules/:id, GET /v1/roadmap/rules, PUT /v1/roadmap/rules/:id, POST /v1/roadmap/rules", done: true, note: "8 endpoints CRUD + GET /v1/roadmap/rules.md." },
      { id: "seed", label: "Seed inicial a partir de ROADMAP_MODULES e ROADMAP_RULES de web/app/components/roadmap/roadmap-data.ts", done: true, note: "12 modulos + 21 regras embutidas como seed global (account_id IS NULL) na propria migration; ON CONFLICT DO NOTHING." },
      { id: "front-store", label: "Pinia store useRoadmapStore() com fetch/update; substitui ROADMAP_MODULES/ROADMAP_RULES estaticos", done: true, note: "Store em app/stores/roadmap.ts; fallback para seeds estaticos quando backend retorna 404." },
      { id: "front-edit", label: "Edicao inline em RoadmapModulesBoard.vue e RoadmapRulesBoard.vue (prioridade, status, descricao)", done: true, note: "Cards ganham botao Editar; abrem form com select status/priority + textarea descricao." },
      { id: "export-md", label: "Endpoint GET /v1/roadmap/rules.md serve AGENT_RULES.md regenerado a partir do banco", done: true, note: "service.BuildMarkdown gera mesmo formato do AGENT_RULES.md raiz." },
      { id: "agent-md", label: "Adicionar back/internal/modules/roadmap/AGENT.md", done: true }
    ],
    verifiable: "Login + PUT em uma regra reflete em GET /v1/roadmap/rules.md instantaneamente; UI permite editar prioridade do modulo Tracking de P1 para P0 e o valor persiste apos refresh."
  },

  {
    id: "tasks-t9",
    code: "Tasks T9",
    title: "Testes E2E + observabilidade",
    goal: "Cobertura > 70% no service Go (scope, DTO, tracking, version conflict); testes Vitest no front (store, realtime, useCan); smoke E2E 12 passos.",
    status: "done",
    estimateWeeks: "2–3 dias",
    startedAt: "2026-05-15",
    finishedAt: "2026-05-15",
    group: "tasks-backend",
    tasks: [
      { id: "dto-test", label: "tasks/dto_test.go: snapshot JSON agency vs client_viewer (campos ausentes, não escondidos)", done: true, note: "4 testes: agency mantem clientAccountId, client_viewer omite, uiMetadata sempre nao-nil, ISO dates." },
      { id: "cursor-test", label: "tasks/cursor_test.go: round-trip + base64url-safe + decode invalido nao panica", done: true, note: "4 testes cobrindo encode/decode opaco do listTasksCursor (paginacao T5)." },
      { id: "presence-test", label: "realtime/presence_test.go: lock exclusivo (T7.2), TTL, snapshot, leave decrementa", done: true, note: "6 testes: user_joined unico, LockField exclusivo por fieldKey, owner reclaim, UnlockField publica, Leave decrementa, TTL expira." },
      { id: "rate-limit-test", label: "httpapi/rate_limit_test.go: bucket, reset, identity resolver, X-Forwarded-For", done: true, note: "6 testes cobrindo o middleware da T8." },
      { id: "service-test", label: "tasks/service_test.go: CRUD com 3 perspectives (agency, client_viewer, outro tenant)", done: true, note: "10 testes com repository_mock_test.go (Repository mock leve com hooks): CreateTask happy/no-perm/validation, GetTask perspective, ListTasks default-limit/clamp/no-perm/perspective/nextCursor." },
      { id: "scope-test", label: "tasks/scope_test.go: fuzz 100 IDs de outros accounts → 100% 404", done: true, note: "8 testes: accountID vazio, account inexistente, cross-account = 404 (nunca 403), platform_admin bypass, client_viewer perspective, manage override, fuzz 100 IDs cross-account 100% 404, scopedQuery panica sem accountID." },
      { id: "tracking-test", label: "tasks/tracking_test.go: version conflict, 1 entry ativa por (user, task)", done: true, note: "8 testes: no-perm/task-not-found/happy-path (publica WS + audita), PauseTracking propaga ErrVersionConflict, ResumeTracking passa expectedVersion, StopTracking 404 nao publica nem audita." },
      { id: "front-tests", label: "Vitest: clampText/normalizeText + setup para composables completos (futuro)", done: true, note: "Vitest 2.1 configurado em web/. 9 testes em utils/text.test.ts cobrindo o caso T7.2 (espaco no final preservado). Composables Vue completos ficam para quando @nuxt/test-utils for adicionado." },
      { id: "smoke-e2e", label: "Smoke E2E 12 passos: migrate fresh → seed → login agência → criar task → WS → presence → tracking → share → curl 404 → inspect payload", done: true, note: "Roteiro documentado em docs/TASKS_ORCHESTRATOR_PHASE12.md (secao 'Smoke E2E 12 passos') para o usuario rodar manualmente em staging." }
    ],
    verifiable: "go test ./... passa (50 testes T9 no total); npm test passa (9 Vitest); smoke E2E 12 passos roteiro pronto para staging."
  },

  // ─── Multi-Tenant Completion (branch refactor/multi-tenant-complete) ──────
  //
  // Fase crítica criada em 2026-05-28 após auditoria descobrir que o
  // esqueleto multi-tenant (Fases 0-5) está codificado mas inerte em runtime,
  // e que páginas de admin (clientes/leads/produtos) foram criadas como BFF
  // mock em web/server/ em vez de bater na API Go. Plano canônico:
  // docs/MULTITENANT_COMPLETION_PLAN.md.

  {
    id: "multitenant-completion",
    code: "MT",
    title: "Multi-Tenant Completion (do início ao fim)",
    goal: "Tornar o multi-tenant a única fonte de verdade do painel: AccountModulesGuard ativo, core.account_modules editável pelo front, web/server/ removido, BFF mock extintos, painel admin real de clientes substituindo o mock anterior.",
    status: "in_progress",
    startedAt: "2026-05-28",
    estimateWeeks: "3-5 semanas",
    group: "multi-tenant",
    tasks: [
      // C1 — Schema completo do que falta (✅ migrations escritas; aplicar com `go run ./cmd/migrate up`)
      { id: "mig-billing", label: "Migration 0123_core_accounts_billing.sql — colunas billing/contact/webhook em core.accounts (substitui campos inventados do mock)", done: true },
      { id: "mig-seed-modules", label: "Migration 0124_core_account_modules_seed.sql — popula core.account_modules para Pérola/Duby com módulos default", done: true },
      { id: "mig-seed-roles", label: "Migration 0125_core_roles_backfill.sql — clona role_templates em core.roles + mapeia user_tenant_roles → core.user_role_assignments", done: true },
      // C2 — Ativar fundação inerte (✅ concluído 2026-05-28)
      { id: "guard-wire", label: "app.go: plugar AccountModulesGuard no chain dos módulos satélites (parar de descartar com `_ =`)", done: true, note: "Fechado de verdade 2026-06-04 (C20): guard instanciado + Dependencies.ModulesGuard + RequireModuleByPath no Chain + Invalidate por evento. Estava `done:true` falso desde 2026-05-28 (era descartado com `_ =`)." },
      { id: "principal-header", label: "Middleware de auth resolve Principal.AccountID a partir de X-Account-Id em toda rota autenticada (valida membership em core.account_users)", done: true },
      // C11 — Menu dinâmico real (2026-05-29)
      { id: "c11-nav-modules", label: "C11.1: NavItem ganha moduleId; useDashboardNav filtra por useCoreAccountStore().enabledModules; tasks/crm/erp/site marcados", done: true },
      { id: "c11-middleware", label: "C11.2: web/app/middleware/module-enabled.global.ts bloqueia rotas /tasks, /crm, /erp, /manage/leads-web, /manage/produtos-web se módulo não habilitado para a account ativa", done: true },
      { id: "c11-switcher-real", label: "C11.3: useCoreAccountStore.fetchAccounts já era real (/v2/me/accounts); faltava ser disparada no boot — adicionado em auth.global.ts pós-ensureSession", done: true },
      { id: "contract-freeze", label: "Reforçar CONTRACT_FREEZE.md: nenhum handler aceita account_id do body — só via Principal", done: true },
      // C3 — Endpoints admin (substituem BFF Nitro) ✅ 2026-05-29
      { id: "api-accounts-crud", label: "GET/POST/GET/{id}/PATCH/DELETE /v1/admin/accounts — CRUD real de account (platform admin)", done: true },
      { id: "api-account-modules", label: "GET/PUT /v1/admin/accounts/{id}/modules — habilita/desabilita módulos + invalida guard cache + evento account.modules.changed", done: true },
      { id: "api-account-stores", label: "GET/PUT /v1/admin/accounts/{id}/stores — billing_amount por loja (migration 0126)", done: true },
      { id: "api-account-webhook", label: "POST /v1/admin/accounts/{id}/webhook/rotate — rotaciona chave do webhook (32 bytes hex)", done: true },
      { id: "api-me-accounts", label: "GET /v1/me/accounts (lean) + GET /v1/me/context?accountId= (full) — aliases v1 adicionados", done: true },
      // C4 — Finalizar Fase 4 (reorganização do queue em Go)
      { id: "queue-subpackages", label: "Mover back/internal/modules/{operations,alerts,analytics,reports,feedback,consultants,settings} para queue/* com queue/module.go consolidando", done: true },
      // C5 — Finalizar Fase 8 (split CRM)
      { id: "crm-subpackages", label: "Mover back/internal/modules/{erp,catalog} para crm/* + crm/module.go implementando Module interface", done: true },
      { id: "crm-resolver", label: "crm.Resolver registrado em Dependencies; queue/catalog_adapter.go usa Resolver com fallback local", done: true },
      // C6 — Performance Fase 7 (terminar)
      { id: "principal-cache", label: "PrincipalCache em memória com invalidação por eventos (sessão, role, permission, account_modules) — fecha Fase 7D", done: true },
      { id: "session-revogation", label: "JWT.sessionId + middleware checa core.user_sessions.revoked_at no mesmo lookup do Principal (fecha 7B)", done: true },
      // C7 — Remoção do BFF Nitro
      { id: "rm-bff-clients", label: "Apagar web/server/api/admin/clients/ inteiro + web/server/utils/clients-repository.ts", done: true },
      { id: "rm-bff-products", label: "Apagar web/server/api/admin/products/ inteiro + web/server/utils/products-repository.ts", done: true },
      { id: "rm-bff-leads", label: "Apagar web/server/api/admin/leads/ inteiro + web/server/utils/leads-repository.ts", done: true },
      { id: "rm-bff-shared", label: "Apagar web/server/utils/reference-admin-access.ts + web/app/composables/useBffFetch.ts", done: true },
      { id: "rm-server-dir", label: "Apagar web/server/ inteiro (decisão fechada 2026-05-28: sem BFF Nitro no produto)", done: true },
      // C8 — Remover fontes paralelas de cliente
      { id: "rm-session-sim", label: "Remover store de simulacao de sessao (lista hardcoded de 6 clientes 101-106): migrado para tasks-client.ts no tasks-layer; task de client-CRUD real em tasks-refactor-v2", done: true },
      { id: "rm-admin-session", label: "Apagar web/app/composables/useAdminSession.ts e useTenantRealtime.ts (stubs mock); manager composables substituídos por stubs para C9", done: true },
      { id: "rm-fake-headers", label: "Trocar todos os usos de x-client-id / x-tenant-id falsos por X-Account-Id real vindo de useAccountStore()", done: true },
      { id: "tasks-real-context", label: "useTasksPageContext: userType derivado do auth real; isAdmin substituído por viewerUserType no onMounted", done: true },
      // C9 — Reescrever páginas mock contra a API Go real
      { id: "rewrite-clients-manager", label: "useClientsManager.ts chamando /v1/admin/accounts; ALL 13 campos do UI persistem em core.accounts; agregados (userCount, userNicks, projectCount, projectSegments, modules, stores) computados no backend via 4 loaders batch", done: true },
      { id: "admin-backend-aggregates", label: "Backend: AccountAdminView ganha agregados; admin_repository_aggregates.go (215 lin) com 4 loaders batch; PATCH active toggle suportado; admin_repository.go 489→384 lin", done: true },
      { id: "rewrite-leads-manager", label: "useLeadsManager.ts reescrito contra /v1/admin/leads (C17); X-Account-Id enviado em todos os requests (fix 2026-06-01)", done: true },
      { id: "rewrite-products-manager", label: "useProductsManager.ts reescrito contra /v1/admin/products (C17); X-Account-Id enviado em todos os requests (fix 2026-06-01)", done: true },
      { id: "consolidate-page", label: "Decisão final 2026-05-29: /clientes (queue) vira /operacao/clientes (módulo Fila); /manage/clientes-web fica como admin global de accounts. Análogo ao /usuarios → /operacao/usuarios. Alias /clientes preservado.", done: true },
      // C10 — UI real de CRUD de cliente
      { id: "ui-list", label: "Tela lista de accounts (filtros, busca, status, paginação)", done: true },
      { id: "ui-modal-card-mirror", label: "Modal + board card de account espelhados (regra do usuário: mudanças em um aplicam no outro)", done: true, note: "Feito 2026-06-04: account-fields.ts (fonte única) consumido por AccountDetailModal + AccountBoardCard — espelho por construção." },
      { id: "ui-create", label: "Form de criar account: módulos contratados, billing, admin inicial", done: true, note: "Feito 2026-06-04: AccountCreateModal → POST /v1/admin/accounts (createClient real, era stub)." },
      { id: "ui-modules", label: "Painel de habilitar/desabilitar módulos por account (com confirmação)", done: true },
      { id: "ui-billing", label: "Painel de billing por account (single + per_store com lista de lojas)", done: true },
      { id: "ui-roles", label: "Painel de cargos por account (clonar template, editar permissões)", done: false, note: "Deferido p/ C18 (RBAC Admin UI), decisão de produto 2026-06-01. Não bloqueia os 8 critérios de saída." },
      // C20 — Ativação real do AccountModulesGuard (full wiring queue/crm/tasks) — 2026-06-04 (go build+test e npm build verdes)
      { id: "mt-guard-activate", label: "app.go: instanciar 1 AccountModulesGuard, passar via Dependencies.ModulesGuard (parar de descartar) + assinar account.modules.changed → guard.Invalidate(accountID)", done: true },
      { id: "mt-account-checker", label: "app.go: authMiddleware.SetAccountChecker(PostgresAccountMemberChecker) para habilitar validação de membership via X-Account-Id", done: true },
      { id: "mt-require-module-paths", label: "httpapi: RequireModuleByPath (gate-list por prefixo, espelha MODULE_PATH_GUARDS do front) gateando queue/crm/tasks; 403 module_disabled < 1s; teste unitário verde", done: true },
      { id: "mt-register-queue-crm", label: "app.go: registrar queue.New() + crm.New() no Registry (core.modules ganha queue/crm; seed 0124 + guard funcionam). Corrige imports legados de app.go (C4/C5 estavam quebrados).", done: true },
      { id: "mt-front-account-id", label: "front: injetar X-Account-Id = auth.activeTenantId no wrapper central createApiRequest (setApiAccountIdProvider) + plugin account-id-bridge.client.ts", done: true },
      { id: "mt-tasks-roadmap-fix", label: "FIX pré-existente (merge): restaurar campos roadmap-pin do tasks (model + service applyCreate/UpdateRoadmapLink + UnmarshalJSON null≠ausente). 6 testes verdes. Era bloqueio do go build.", done: true },
      { id: "mt-session-sim-kill", label: "Critério 5: useTasksPageContext usa useTasksClientStore; deletar shim session-simulation.ts; limpar tasks/AGENT.md → grep (código runtime) = 0", done: true },
      { id: "c10-detail-modal", label: "C10: AccountDetailModal.vue sobre account-fields.ts (fonte única de campos billing/contact/webhook/módulos)", done: true },
      { id: "c10-board-card", label: "C10: AccountBoardCard.vue espelhando os mesmos campos do detail modal (regra modal↔card)", done: true },
      { id: "c10-create-modal", label: "C10: AccountCreateModal.vue (slug, name, plan, admin inicial) → POST /v1/admin/accounts", done: true },
      // C11 — Menu dinâmico real
      { id: "use-nav-real", label: "useNav consome useAccountStore().enabledModules em vez de nav.config.ts estático filtrado por role", done: true },
      { id: "middleware-module", label: "Middleware Nuxt module-enabled.global.ts: bloqueia rota se módulo não está habilitado", done: true },
      { id: "account-switcher-real", label: "AccountSwitcher consome GET /v1/me/accounts real + dispara fetchAccounts no boot via auth.global.ts", done: true },
      // C12 — Smoke tests + migração de dados
      { id: "smoke-tenants", label: "Pérola e Duby aparecem na lista nova com módulos contratados corretos via API Go", done: false },
      { id: "smoke-billing", label: "Backfill manual dos campos novos de billing para Pérola (Duby fica com defaults)", done: false },
      { id: "smoke-switch", label: "Trocar account no AccountSwitcher → menu recarrega; desabilitar módulo no banco → item some da UI sem reload", done: false, note: "Manual no navegador — depende do user testar. Estrutura pronta (useDashboardNav filtra por enabledModules, middleware module-enabled.global.ts bloqueia rota direta)." },
      // C13 — Documentação
      { id: "doc-adr", label: "docs/adr/0002-remove-bff-nitro-mock.md formalizando a remoção do BFF Nitro (cronologia C7→C17 + cenários onde BFF voltaria a fazer sentido)", done: true },
      { id: "doc-contract", label: "docs/CONTRACT_FREEZE.md 2.1 (X-Account-Id ativado) + 2.8 (AccountAdminView) + 2.9 (AdminUserView/OrganizationAdminView) já documentados", done: true },
      { id: "doc-agents", label: "AGENT.md atualizado em core (admin_users/orgs + nick.go), site (novo), omni/ (novo)", done: true },
      { id: "doc-roadmap-flip", label: "Reverter para `done` no roadmap as fases 0-5, 7, 8 — executado 2026-06-01", done: true },
      { id: "tasks-client-rename", label: "Renomear stores/session-simulation.ts → tasks-client.ts na tasks-layer; grep session-simulation = 0 ocorrências", done: true, note: "Fechado de verdade 2026-06-04 (ver mt-session-sim-kill): shim deletado, useTasksPageContext usa useTasksClientStore. Código runtime = 0 refs (sobram só labels/AGENT que descrevem a remoção)." },
      { id: "tasks-client-mock-badge", label: "Sinalizar clientes mock de Tasks com badge MOCK (só platform_admin)", done: true, note: "2026-06-10: DEFAULT_CLIENT_OPTIONS marcados isMock + tasksClient.isMockClient(); dropdown mostra description 'MOCK' e label da modal mostra UBadge MOCK, gateados por platform_admin. Pipeline integer mantido funcionando de propósito até o link real. Ver docs/LEGADO.md §4." },
      { id: "tasks-client-real-link", label: "Ligar cliente de Tasks ao real: trocar clientId integer mock por clientAccountId (UUID de core.accounts via /v1/tenants), linkar os 4 mocks aos accounts reais e religar tracking.vue ao GET /v1/tasks/tracking/metrics", done: false, note: "2026-06-10: os 4 clientes (crow/Pérola/Dr Antonio Tavares/UNO) criados em core.accounts. Front passa a puxar TODOS os tenants ativos de /v1/tenants (sem filtro por ora). Fonte de verdade = core.accounts (public.tenants foi dropada). Destrava a inteligência de tempo por cliente da página de tracking. Ver docs/LEGADO.md §4 + memória project_tasks_client_source." },
      { id: "tasks-client-visibility-flag", label: "Página de clientes: toggle por cliente 'aparece em tasks' (account não marcado some do seletor de Tasks), para não despejar contas de teste/internas no seletor", done: false, note: "Pedido 2026-06-10. Por ora o seletor de tasks puxa todos os tenants ativos; este flag substitui o 'puxa todos' por filtro por visibilidade." },
      { id: "agency-tenant-architecture", label: "Arquitetura Agência→Clientes (tenants): org Crow Visuals dona das contas-cliente; conta-agência dona do board Tasks; acesso por nível; switcher ligado ao contexto do Tasks. Ver docs/AGENCY_TENANT_ARCHITECTURE.md", done: false, note: "2026-06-10: descoberto ao corrigir o 'dono' do board. core.organizations existe mas vazia; contas soltas (organization_id null). Plano em 5 estágios (doc-first). ORDEM CRÍTICA: Estágio 1 = ligar troca de conta ao auth.activeTenantId que o Tasks lê, ANTES de mover board (mover antes quebrou e foi revertido do backup). Multitenant-completion." },
      { id: "tasks-loading-optimization", label: "Otimizar carregamento da página de Tasks: skeleton imediato (<100ms), carregar só o board ativo above-the-fold + lazy-load dos demais, parar de puxar arquivadas no boot", done: true, note: "2026-06-10 (agente paralelo): refresh() carrega só as tasks do board ativo; ensureBoardTasksLoaded/ensureArchivedTasksLoaded sob demanda; archived=false no boot; AbortController ao trocar board; realtime preserva boards de fundo. + render progressivo no TasksBoardView (15 cards/coluna no 1o paint, resto em lotes via requestIdleCallback, reseta ao trocar board). ESLint 0 errors." },
      { id: "tasks-board-render-improve", label: "Melhorar render do board no futuro (já USÁVEL): montar os selects pesados do card só ao clicar (hoje cada card monta vários OmniSelectMenuInput de uma vez) e/ou windowing real por viewport", done: false, note: "2026-06-10: o render progressivo deixou usável; o próximo nível de perf é não montar os editores pesados em todos os cards. Não-bloqueante." },
      { id: "tracking-board-redesign", label: "Página Tracking com layout de board igual ao Tasks: só tasks em play/pause, card focado em nome/tempo/cliente/responsável (configurável), clique abre o modal da task", done: true, note: "2026-06-10: tracking.vue reescrita provendo o contexto do Tasks (TASKS_PAGE_CONTEXT_KEY) + novo TrackingBoardView.vue (board com mesmas colunas, filtrado a isTracking=play/pause; card enxuto nome/tempo/cliente/responsável; clique -> openTaskEditor abre o modal; play/pause/stop no card). Config de campos (Tempo/Cliente/Responsável) via popover, persistida em localStorage (pref de visão). Seletor de projeto. ESLint 0 errors. A inteligência (useTrackingMetrics.ts + GROUP BY) fica PARADA/pronta para virar uma visão/aba complementar depois (não está mais na página)." },
      { id: "tasks-tracking-metrics", label: "Tracking: religar tracking.vue ao GET /v1/tasks/tracking/metrics (tempo por cliente/usuário/período)", done: true, note: "2026-06-10: useTrackingMetrics.ts + tracking.vue com abas Inteligência/Em andamento, período via AppDatePicker, gate por tasks.tracking.view_all. Após o GROUP BY (tasks-tracking-metrics-groupby), o front faz UMA chamada e lê byClient/byUser/byType prontos (N+1 eliminado); 'Por tipo' agora é real (por período), não mais só timers ativos. ESLint 0 errors." },
      { id: "tasks-tracking-metrics-groupby", label: "BACKEND: GROUP BY no GET /v1/tasks/tracking/metrics (breakdown por cliente/usuário/tipo em 1 query)", done: true, note: "2026-06-10: TrackingMetrics ganhou ByClient/ByUser/ByType (TrackingMetricsBucket key/label/total/count); repository faz 3 group-by server-side com label resolvido por join (core.accounts/core.users); total escalar respeita todos os filtros, breakdowns respeitam período. go build+test+vet+gosec OK (0 novos). Elimina o N+1 do front (ENGINEERING_PRINCIPLES §10.3). EXIGE rebuild da api: docker compose up -d --build api. NOTA: trackingTotalMs por task NÃO foi populado no ListTasks de propósito (somar duração por task no hot path do board contraria a otimização de carregamento); fica sob demanda se necessário." },
      { id: "site-account-id-fix", label: "Fix accountIDFromContext em site/http_admin.go: lê X-Account-Id header primeiro (fallback TenantID); useLeads/ProductsManager passam X-Account-Id em todos os requests", done: true },
      // C14 — Users admin global (/manage/users)
      { id: "c14-backend-users", label: "Backend: admin_users_model/repo/service/http no módulo core; 6 endpoints /v1/admin/users + GET /{id}/memberships; safeguard último platform_admin; hasher via Dependencies", done: true },
      { id: "c14-frontend-users", label: "Frontend: useAdminUsersManager + AdminUsersWorkspace + page /manage/users com modal de criação; cross-account view via accountCount/accountSlugs agregados", done: true },
      { id: "c14-workspace-wire", label: "Wire do workspaceId: usuarios_admin registrado em workspaces.ts (WORKSPACES), permissions.ts (WORKSPACE_ACCESS_DEFINITIONS + ROLE_WORKSPACES.platform_admin), nav.config.ts", done: true },
      { id: "c14-nick-backfill", label: "Backfill de core.users.nick via migration 0127 + helper Go core.BuildNickname + auto-geração em AdminUserService.CreateUser (regra espelha buildNickname do front)", done: true },
      { id: "c14-route-canonical", label: "/usuarios passou a ser /operacao/usuarios (módulo Fila); alias /usuarios mantido para URLs antigas", done: true },
      // C15 — Organizations admin (/manage/organizations)
      { id: "c15-backend-orgs", label: "Backend: admin_organizations_model/repo/service/http no módulo core; 5 endpoints /v1/admin/organizations; agregados accountCount/accountSlugs; PATCH account aceita organizationId (''=NULL)", done: true },
      { id: "c15-frontend-orgs", label: "Frontend: useAdminOrganizationsManager + AdminOrganizationsWorkspace + page /manage/organizations + modal de criação; usa nova API C16 (locked + drag-n-drop) já de cara", done: true },
      { id: "c15-clients-org-column", label: "ClientsAdminWorkspace ganha coluna Organization (select editável inline com options de /v1/admin/organizations); FIELD_TO_PATCH no useClientsManager mapeia organizationId", done: true },
      { id: "c15-wire-3-places", label: "Wire de organizations_admin em workspaces.ts + permissions.ts (WORKSPACE_ACCESS_DEFINITIONS + ROLE_WORKSPACES.platform_admin) + nav.config.ts", done: true },
      // C16 — Tabela admin canônica: travar colunas + drag-n-drop
      { id: "c16-types", label: "OmniTableColumn ganha `locked?` e `defaultOrder?`; OmniTableColumnsConfig mostra cadeado + drag handle só para admin; useOmniVisibleColumns persiste lockedKeys + columnOrder em localStorage", done: true },
      { id: "c16-migrate-existing", label: "ClientsAdminWorkspace + AdminUsersWorkspace migrados: defaultOrder nas colunas, name/displayName com locked:true por padrão, v-model:locked-columns + v-model:column-order + reset-columns wired", done: true },
      { id: "c16-followup-usuarios", label: "Fase de consolidação UI: migrar /usuarios (UsersAccessTable) para OmniDataTable para unificar tabela admin do Fila com admin global", done: false, note: "Pendente, não bloqueia C15" },
      // C17 — Módulo site (leads + products via webhook/API + admin CRUD)
      { id: "c17-migrations", label: "Migration 0128_site_schema.sql: schema site + tabelas site.leads, site.products, site.webhook_sources com FKs para core.accounts", done: true },
      { id: "c17-backend-leads", label: "Backend site: model + leads_repository (List/Find/Create/CreateFromWebhook/Update/SoftDelete) + 5 endpoints /v1/admin/leads*", done: true },
      { id: "c17-backend-products", label: "Backend site: products_repository com serialização jsonb de categories/campaigns + 5 endpoints /v1/admin/products*", done: true },
      { id: "c17-backend-webhooks", label: "Backend site: webhook_sources CRUD com secret em claro (necessário para HMAC) + endpoints ingest POST /v1/webhooks/{leads,products}/{slug} validando X-Signature HMAC SHA-256", done: true },
      { id: "c17-frontend", label: "Frontend: useLeadsManager + useProductsManager reescritos contra API real (UUID string IDs); SiteLeads/Products workspaces ganham filtros, modal de criação, colunas C16 (locked + drag-n-drop), tooltips, popover controlado", done: true },
      { id: "c17-webhook-ui", label: "Frontend: useWebhookSourcesManager + WebhookSourcesDrawer (USlideover) com cadastro/rotate/delete; secret revelado uma vez com botão copiar; URL completa de ingest exibida por source", done: true },
      { id: "c17-wire", label: "Wire dos workspaceIds (site_leads_web + site_produtos_web) já estavam em workspaces.ts + permissions.ts; nav só renomeado de 'Leads/Produtos Web' para 'Leads/Produtos do site'", done: true },
      { id: "c17-agent-md", label: "AGENT.md do módulo site documentando: arquitetura, endpoints, exemplo de curl com HMAC, drift cross-camada", done: true },
      // C18 — Site tracking analytics (webhook Perola)
      { id: "c18-tracking-contract", label: "Contrato assinado do painel Perola: POST /v1/webhooks/tracking/{sourceSlug} com X-Omni-Timestamp + X-Omni-Signature HMAC SHA-256", done: true },
      { id: "c18-tracking-schema", label: "Migration 0129_site_tracking_events.sql: entity_type tracking + tabela site.tracking_events com idempotencia por source_event_id", done: true },
      { id: "c18-tracking-ingest", label: "Backend site: receptor tracking valida source/assinatura/timestamp, persiste lote events[] e responde received/inserted/skipped", done: true },
      { id: "c18-tracking-smoke", label: "Smoke local ponta a ponta: painel Perola salva evento, assina webhook, Omni recebe 201 e nao deixa lote na outbox", done: true },
      // C19 — Site admin canônico no menu principal
      { id: "c19-site-menu-wire", label: "Mover o menu Site para as telas canônicas /site/leads + /site/produtos, remover atalhos equivalentes do bloco Manage e adicionar /site/tracking no dropdown principal", done: true },
      { id: "c19-site-tracking-backend", label: "Backend site: GET /v1/admin/tracking-events com filtros de busca/origem/evento/página, leitura paginada de site.tracking_events e DTOs próprios", done: true },
      { id: "c19-site-tracking-frontend", label: "Frontend: SiteTrackingAdminWorkspace com OmniCollectionFilters + OmniDataTable + modal de detalhes completos (eventData/rawPayload) e ação de refresh", done: true },
      { id: "c19-site-workspace-wire", label: "Wire do workspaceId site_tracking_web em workspaces.ts + permissions.ts + nav.config.ts/sidebar-nav.ts; /site/leads e /site/produtos passam a usar as workspaces admin reais", done: true },
      { id: "c19-site-docs", label: "Atualizar AGENTs do frontend/site com a navegação canônica e o novo endpoint admin de tracking", done: true }
    ],
    blockers: [],
    verifiable: "1) GET /v1/admin/accounts retorna Pérola e Duby do banco real (não do mock). 2) Habilitar módulo via UI grava em core.account_modules e o item aparece no menu sem reload. 3) Desabilitar módulo retorna 403 module_disabled na rota dele. 4) Diretório web/server/ não existe. 5) Apenas 1 fonte de cliente no painel inteiro. 6) Roadmap volta a refletir verdade: Fases 0-5/7/8 efetivamente `done`. 7) /manage/users e /manage/organizations operando contra API real."
  },

  {
    id: "user-model-unification",
    code: "UMU",
    title: "Unificação do modelo de usuário (remover legado de papéis)",
    goal: "Zero legado: 1 usuário = 1 linha em core.users, papéis 100% em core.* (account_users + user_role_assignments), config por módulo em core.user_module_settings, e DROP das tabelas legadas user_tenant_roles/user_store_roles/user_platform_roles. Auth e /operacao/usuarios passam a ler de core. Plano: docs/USER_MODEL_UNIFICATION_PLAN.md. Concluído 2026-06-06 (U1-U4): paralelismo Claude+Codex, auth/operacao/scope 100% core, tabelas legadas dropadas.",
    status: "done",
    startedAt: "2026-06-05",
    estimateWeeks: "2-3 semanas",
    group: "multi-tenant",
    tasks: [
      { id: "umu-module-settings", label: "U1: migration 0132 core.user_module_settings (user_id, module_id, config jsonb) — destino da config por módulo", done: true },
      { id: "umu-legacy-marker", label: "U1: componente LegacyMarker.vue (badge LEGADO/MOCK só para platform_admin) + plugado em /operacao/usuarios", done: true },
      { id: "umu-auth-core", label: "U2 (keystone): auth (LoadUserForAuth) resolve role/permissões de core.account_users + core.user_role_assignments (+ is_platform_admin); fallback legado atrás de flag; user core-only loga. Testes Go.", done: true, note: "Concluido em 2026-06-05: AUTH_ROLES_SOURCE=core|legacy|core_with_fallback (default core_with_fallback), migration 0133 backfill legado->core, testes Go de mapeamento/login." },
      { id: "umu-operacao-core", label: "U3: /operacao/usuarios (users module) lista de core.* + projeção da Fila (employee_code/store/consultor → core.user_module_settings)", done: true, note: "Concluido em 2026-06-05: GET /v1/users le core.account_users/users/user_role_assignments/roles + storeIdsByAccount; create/update tambem gravam core e mantem dual-write legado ate U4." },
      { id: "umu-u4a-claude", label: "U4a (Claude, paralelo): crm/erp (scope) + queue/settings leem core.* em vez de user_*_roles", done: true, note: "2026-06-05: crm/erp (CanAccessTenant + 2 predicados + ResolveDefaultTenantID + fallback de loja do funcionario) e queue/settings (acesso + ResolveDefaultTenantID) migrados p/ core.account_users + core.user_module_settings. Zero leitura legada." },
      { id: "umu-u4a-codex", label: "U4a (Codex, paralelo): stores + tenants leem core.* em vez de user_*_roles; dual-write na escrita", done: true, note: "2026-06-05: tenants/scope_queries.go + stores/scope_queries.go+core_scope.go leem core.*; delete de loja faz dual-write (user_store_roles legado + core.user_module_settings). Testes Go novos. Revisado pelo engenheiro." },
      { id: "umu-u4b-dual-write", label: "U4b: remover dual-write legado (users, core/admin band-aid, consultants, stores, bootstrap_owner) — escrita só em core", done: true, note: "2026-06-05 (Claude, apos Codex nao concluir): users->upsertCoreAssignmentsTx; stores->deleteCoreStoreScopeTx; consultants novo core_scope.go (membership+role+storeIds); admin->user_role_assignments; bootstrap->core. Runtime: manage cria com zero user_tenant_roles + login OK. Restam apenas seeds historicos (0002/0012/0015/0036) que rodam antes do drop." },
      { id: "umu-u4c-drop", label: "U4c (Claude, destrutivo): backup + grep zero-usos + DROP user_tenant_roles/user_store_roles/user_platform_roles; AUTH_ROLES_SOURCE=core", done: true, note: "2026-06-06: backup (legacy + full dump 249M), migration 0135 dropa as 3 tabelas (idempotente, roda no boot), .env AUTH_ROLES_SOURCE=core. Fix: findRecord nao referencia legado em core mode (era 500 pos-drop). Validado: login/manage/modulos OK sem as tabelas; testes auth verdes. LEGADO de papeis eliminado." }
    ],
    blockers: [],
    verifiable: "1) Usuário criado só no core (sem user_tenant_roles) loga e tem o acesso certo. 2) /operacao/usuarios lista de core.*, sem ler tabela legada. 3) Tabelas user_*_roles não existem mais. 4) Nenhum write em user_*_roles no código. 5) docs/LEGADO.md item 1 marcado como removido."
  },

  // ─── Automação WhatsApp/IA (módulo automation/) ──────────────────────────
  //
  // Assistente proativa de WhatsApp construída em n8n + WAHA (persona Tony),
  // migrada para dentro do Omni em 2026-06-04 como módulo automation/ (containers
  // no profile "automation" do docker-compose). As fases que viram módulo Go/banco
  // (Etapa 2 em diante) aguardam o fechamento da multitenant-completion.
  // Doc do módulo: automation/AGENT.md. Detalhe do bot: docs/automation/*.

  {
    id: "automation-whatsapp",
    code: "AUT",
    title: "Automação WhatsApp/IA (n8n) dentro do Omni",
    goal: "Evoluir a automação de WhatsApp/IA de '1 bot atendimento no n8n' para uma PLATAFORMA multi-tenant: cada cliente cria N automações (robôs), cada uma com número, comportamento (persona/instruções/knowledge RAG), modelos de IA por etapa, tools e BYOK (chave/créditos do próprio cliente). Multi-tenant desde o dia 1 (automation_id central). Visão: docs/automation/PLATAFORMA_AUTOMACAO.md.",
    status: "in_progress",
    startedAt: "2026-06-04",
    estimateWeeks: "2-4 semanas (após multitenant-completion)",
    group: "automation",
    tasks: [
      // Infra/migração (feita em 2026-06-04)
      { id: "aut-merge-infra", label: "Containers n8n + WAHA + Redis mesclados no docker-compose.yml da raiz sob profiles:[automation] (sem postgres dedicado; WAHA fala com n8n via rede interna; portas 5680/3010/6380)", done: true, note: "Concluído 2026-06-04. Validado com docker compose --profile automation config." },
      { id: "aut-move-folder", label: "Pasta 'n8n Whatsapp' migrada para automation/ (export + .mcp.json + .gitignore + docker-compose.reference.yml) e docs para docs/automation/", done: true, note: "Concluído 2026-06-04." },
      { id: "aut-docs", label: "automation/AGENT.md criado; SETUP.md adaptado (profile, nomes de serviço, caminhos); .gitignore raiz protege segredos; .env.docker.example com AUTOMATION_*", done: true, note: "Concluído 2026-06-04." },
      { id: "aut-runbook-validate", label: "Subir profile automation, instalar community node, importar credenciais+workflow, escanear QR e validar 1 mensagem real (depende do usuário; ativar responde no WhatsApp real)", done: false, note: "2026-06-08: corrigida a tag da WAHA (manifest 2026.5.1 não existe puro) → devlikeapro/waha:gows-2026.5.1 (engine GOWS) no dev e prod. `up -d` volta a funcionar. Falta os passos interativos do Mike." },
      // Fases de produto (bloqueadas pela multitenant-completion). Design: docs/automation/PLANO_INTEGRACAO_OMNI.md
      { id: "aut-a1-schema", label: "A1 — Migration schema automation.* (tenant-aware): settings, personas, guardrails, model_catalog, waha_sessions, service_tokens, contacts, messages, lead_state, long_memory, follow_ups, purchases + seeds", done: false },
      { id: "aut-a2-modulo-go", label: "A2 — Módulo Go automation (Module Registry): settings, personas, model_catalog, endpoint runtime-config (persona+guardrails+contexto+modelos) e service_tokens; auth por token de serviço resolve account_id", done: false },
      { id: "aut-a3-n8n-config", label: "A3 — n8n consome runtime-config: systemMessage/modelos/contexto/enabled dinâmicos via HTTP (para de cravar persona/modelo nos nós). Valida troca de modelo por expression (responsesApiEnabled)", done: false },
      { id: "aut-a4-painel-status", label: "A4 — Painel /automation: Status (WhatsApp connect/QR via proxy WAHA) + liga/desliga + contexto temporário com expiração", done: false },
      { id: "aut-a5-painel-personas", label: "A5 — Painel: Personas/Prompts CRUD + escolher ATIVA; guardrails anexados automaticamente; modal e board card espelhados", done: false },
      { id: "aut-a6-painel-modelos", label: "A6 — Painel: Modelos (catálogo + regras do MODELOS.md aplicadas sozinhas: Responses API/temperature)", done: false },
      { id: "aut-a7-crm", label: "A7 — CRM persistente no Postgres do Omni (contacts/messages/lead_state/long_memory); n8n grava cada mensagem e o resumo via API (substitui staticData lite)", done: false },
      { id: "aut-a8-tools", label: "A8 — Tools do agente via API Go (catalog/stock/price, registrar lead/pedido) com escopo por account; sem SQL cru nas tabelas", done: false },
      { id: "aut-a9-proativo", label: "A9 — Motor proativo (Etapa 3): follow-up sem resposta (cadência), pós-venda, nurture/upsell — depende do estado persistente (A7)", done: false },
      { id: "aut-a10-deploy-vps", label: "A10 — Deploy VPS: n8n/waha/redis no docker-compose.prod.yml + Caddy (auth no editor, webhook interno) + .env.production + backups dos volumes", done: false, note: "Infra preparada 2026-06-08 (independe de A1+; bot standalone): serviços no docker-compose.prod.yml (profile automation, mesmos nomes do dev), Caddy+basic auth nos subdomínios n8n./waha., Redis só na rede app (disponível p/ a API depois), vars AUTOMATION_* no .env.production.example, runbook em SETUP.md §8. Pendente do Mike: snippet Caddy+DNS, subir na VPS, QR, ativar, backups." },
      // ─── Plataforma multi-tenant (P) — generaliza A1-A10. Visão: docs/automation/PLATAFORMA_AUTOMACAO.md ───
      // A entidade central passa a ser "automation" (o robô): N por account, cada uma com número,
      // comportamento, modelos por etapa, tools e BYOK. Multi-tenant desde o dia 1. Também bloqueado pela multitenant-completion.
      { id: "aut-p1-schema-automation-centric", label: "P1 — Migration automation-centric: tabela automations (N por account) + entitlements (gating/quotas) + channels (sessão WAHA→automação); persona/CRM/modelos passam a ter automation_id. Generaliza A1", done: false },
      { id: "aut-p2-byok", label: "P2 — BYOK: provider_credentials por account (chave do cliente criptografada at-rest AES-GCM, master key AUTOMATION_CRED_ENC_KEY; só últimos 4 no painel). Cada automação escolhe provider+credencial; créditos são do cliente", done: false },
      { id: "aut-p6-modelos-por-no", label: "P6 — Modelos de IA por etapa/nó por automação (chat/visão/áudio/classificador/triagem) configurados no painel; catálogo provider-agnostico (OpenAI + Anthropic/Claude)", done: false },
      { id: "aut-p8-rag", label: "P8 — Knowledge/RAG por automação (comportamento estilo GPT): pgvector no Postgres do Omni (extensão vector), upload→chunk→embed→retrieval por automation_id. Alternativa avaliada: Vector Store do provedor", done: false },
      { id: "aut-p9-tools-cross-data", label: "P9 — Tools cruzando dados: agente consulta CRM/ERP/métricas de outros módulos via API Go, escopo por account (base para decisões com dados do banco)", done: false },
      { id: "aut-p11-multi-numero", label: "P11 — Multi-número: N channels por account (WAHA Plus multi-sessão; decisão de custo da licença). Schema já modela channel por sessão desde o P1", done: false },
      { id: "aut-p12-super-robo", label: "P12 — Super-robô interno do time (automation_type 'super'): cruza dados cross-account (CRM/ERP/métricas), admin-gated (automation.platform.admin). Pode ser surface no painel, não WhatsApp", done: false },
      { id: "aut-p13-metering", label: "P13 — Metering/cotas: usage_log (tokens/custo estimado por automação) visível no painel; cota opcional por entitlement", done: false },
      // ─── Painel /automation (M1, entregue 2026-06-09 após multitenant-completion) ───
      { id: "aut-m1-painel-status", label: "M1 — Módulo Go automation (Module Registry) + migration 0140 (automations/channels) + painel /automation: Status + Conectar WhatsApp (QR via proxy WAHA) + liga/desliga. Gated platform_admin (você+irmão). Endpoints /v1/automation*; gating moduleGatingRules + bypass admin", done: true, note: "2026-06-09: back (model/waha_client/store/service/http/module) build+gofmt+vet+golangci-lint 0 issues; migration validada em rollback no banco real; front (página + useAutomation + AutomationWorkspace BEM) eslint 0 erros + vue-tsc limpo. liga/desliga só persiste status (enforcement no M2)." },
      { id: "aut-m2-runtime-config", label: "M2 — runtime-config: n8n consome persona/enabled do banco via HTTP (para de cravar no nó); on/off passa a valer de verdade", done: true, note: "2026-06-09: migration 0141 (automation.personas) + GET /v1/runtime/automation/config (auth AUTOMATION_RUNTIME_TOKEN, fora do gating) monta persona ativa + guardrails; seed Tony/Crow via go:embed (persona-tony-crowvisuals.md verbatim). n8n: nó Get runtime config (off Webhook) + AI Agent systemMessage por expression + Bot ligado? (gate enabled). build+lint 0 issues; migrations validadas em rollback. Ativação: rebuild api + token + re-import workflow." },
      { id: "aut-m3-personas", label: "M3 — Comportamento da IA: editor de persona (instruções) no painel /automation", done: true, note: "2026-06-09: GET/PUT /v1/automation/persona + card Comportamento no AutomationWorkspace (nome + system_prompt, textarea). Salvar altera o bot sem tocar no n8n (runtime-config lê do banco). Seed Tony/Crow verbatim. build+lint+vue-tsc verdes." },
      { id: "aut-m3plus-knowledge-docs", label: "M3+ — Knowledge por documento: documentos editáveis no painel (título + corpo); runtime-config concatena os habilitados após as instruções da persona. RAG pgvector (P8) quando o volume for grande", done: true, note: "2026-06-10: migration 0142 (automation.knowledge_documents); CRUD completo pelo painel (6 cards independentes); runtime-config devolve docs[] separados (Opção B) + systemMessage montado (Opção A fallback); AutomationContextPreview.vue mostra estrutura completa; workflow n8n com nó 'Montar systemMessage' para injeção dinâmica por keywords. 6 docs Tony/Crow Visuals injetados no banco." },
    ],
    blockers: [],
    verifiable: "Infra: `docker compose --profile automation up -d` sobe n8n/waha/redis na mesma rede do Omni e o workflow importado responde uma mensagem real de teste. Produto (futuro): bot lê produto/estoque via API Go e persiste contato/lead no schema automation.* do Postgres do Omni."
  }
];

export const ROADMAP_MODULES: RoadmapModule[] = [
  {
    id: "tasks",
    label: "Tasks",
    route: "/tasks",
    status: "beta",
    priority: "P0",
    category: "atendimento",
    description:
      "Orquestrador de tarefas multi-tenant (boards + tabela). Backend pronto (Fases T1-T9). UI em refino: rolagem vertical, performance, checklist no editor, expand/restore. Em uso interno antes de liberar para tenants.",
    scope: [
      "Refinar performance do board para >500 cards",
      "Melhorar feedback de drag-and-drop entre colunas",
      "Adicionar filtros salvos por usuario",
      "Notificacoes in-app quando @mention"
    ],
    dependsOn: []
  },
  {
    id: "editor",
    label: "Editor",
    route: "/editor",
    status: "beta",
    priority: "P1",
    category: "tools",
    description:
      "Editor rich-text Omni baseado em Tiptap (StarterKit + TaskList + Emoji + Mention + TextAlign). Usado em descricao de tasks. Falta: salvar/abrir documentos avulsos, versionamento, sharing.",
    scope: [
      "Persistir documentos avulsos (nao apenas dentro de Tasks)",
      "Adicionar /slash commands",
      "Suporte a colaboracao em tempo real (avaliar @tiptap/y-tiptap)"
    ],
    dependsOn: ["tasks"]
  },
  {
    id: "tracking",
    label: "Tracking",
    route: "/tracking",
    status: "pending",
    priority: "P1",
    category: "atendimento",
    description:
      "Visao de time-tracking por consultor: tempos por task, relatorios diarios/semanais. Depende de Tasks 100% (presence + tracking ja existem no backend T7).",
    scope: [
      "Agregacao por consultor/dia/semana",
      "Export CSV",
      "Comparativo Pessoa A vs B no periodo"
    ],
    dependsOn: ["tasks"]
  },
  {
    id: "omnichannel",
    label: "Omnichannel",
    route: "/omnichannel",
    status: "pending",
    priority: "P2",
    category: "atendimento",
    description:
      "Conversas unificadas WhatsApp/Instagram/Email/Webchat com handoff humano e bot. Page existe mas vazia. Escopo grande: webhook providers + threads + roteamento.",
    scope: [
      "Conectores WhatsApp Cloud API + Instagram Direct",
      "Schema messaging.* com threads",
      "Roteamento por fila + handoff",
      "Bot simples por palavra-chave"
    ],
    dependsOn: []
  },
  {
    id: "assistente-ia",
    label: "Assistente IA (WhatsApp)",
    route: "/automation",
    status: "in_progress",
    priority: "P1",
    category: "atendimento",
    description:
      "Assistente proativa de WhatsApp (n8n + WAHA, persona Tony): multimodal (texto/audio/imagem via Whisper+visao), debounce, memoria por segmento + memoria longa, naturalidade (digitando/baloes). Migrada para dentro do Omni como modulo automation/ (containers no profile docker 'automation'). Distinta do Omnichannel (conversas unificadas): aqui o foco e o cerebro de IA proativo.",
    scope: [
      "Mini-CRM no Postgres do Omni (schema automation.*, tenant-aware)",
      "Tools do agente via API Go (catalogo/estoque/preco, registrar lead/pedido)",
      "Motor proativo (follow-up/pos-venda/nurture)",
      "Painel de config (modelos, personas, liga/desliga, contexto temporario)"
    ],
    dependsOn: []
  },
  {
    id: "team",
    label: "Team (Equipe + Escalas)",
    route: "/team/equipe",
    status: "pending",
    priority: "P2",
    category: "operacao-comercial",
    description:
      "Gestao de equipe e escalas. Pagina existe mas sem CRUD real. Compartilha schema core.users + roles ja existentes.",
    scope: [
      "CRUD de equipe com avatar e cargo",
      "Calendario de escalas (turnos)",
      "Aprovacao de troca de turno"
    ],
    dependsOn: []
  },
  {
    id: "site",
    label: "Site (Campanhas + Paginas + Forms)",
    route: "/campanhas",
    status: "pending",
    priority: "P3",
    category: "operacao-comercial",
    description:
      "Builder visual de paginas/forms + campanhas atreladas a pagina. Pagina /campanhas existe mas sem builder. Forms ainda nao implementado.",
    scope: [
      "Builder drag-drop simples para pagina",
      "Geracao de form com webhook",
      "Campanha = pagina + canal de divulgacao + meta",
      "Tracking de conversao"
    ],
    dependsOn: ["site"]
  },
  {
    id: "inteligencia",
    label: "Inteligencia",
    route: "/inteligencia",
    status: "pending",
    priority: "P2",
    category: "indicadores",
    description:
      "Insights gerados por LLM sobre dados de vendas e atendimento. Cards 'Por que conversao caiu?' / 'Quais produtos faltam mais?'. Sera consumidor pesado do backend de BI.",
    scope: [
      "Prompts canonicos para 5 perguntas frequentes",
      "Cache de resposta por janela (dia/semana)",
      "Exportar insight como PDF"
    ],
    dependsOn: ["bi"]
  },
  {
    id: "relatorios",
    label: "Relatorios",
    route: "/relatorios",
    status: "pending",
    priority: "P2",
    category: "indicadores",
    description:
      "Reports estaticos exportaveis (PDF/CSV). Backend reports/ existe parcial. Faltam templates e UI de configuracao.",
    scope: [
      "Template Ranking Mensal Consultor (PDF)",
      "Template Vendas por Loja (CSV + PDF)",
      "Agendamento de envio recorrente por email"
    ],
    dependsOn: []
  },
  {
    id: "bi",
    label: "BI",
    route: "/bi",
    status: "pending",
    priority: "P2",
    category: "indicadores",
    description:
      "Dashboards customizaveis. Modulo backend bi/ ja criado mas sem UI. Definir entre dashboard hardcoded vs builder.",
    scope: [
      "Decidir entre Metabase embedded vs builder proprio",
      "MVP com 3 dashboards fixos (vendas, atendimento, estoque)",
      "Filtros por loja/consultor/periodo"
    ],
    dependsOn: []
  },
  {
    id: "finance",
    label: "Finance",
    route: "/finance",
    status: "pending",
    priority: "P3",
    category: "indicadores",
    description:
      "Comissoes, metas financeiras, fechamento mensal. Hoje nao existe. Depende de Vendas (ERP) ja integrada.",
    scope: [
      "Calculo de comissao por consultor com regras configuraveis",
      "Fechamento mensal exportavel",
      "Integracao com folha (fora do escopo inicial)"
    ],
    dependsOn: []
  },
  {
    id: "monitoramento",
    label: "Monitoramento",
    route: "/monitoramento",
    status: "pending",
    priority: "P2",
    category: "indicadores",
    description:
      "Pagina interna de health: uptime API, jobs ERP, sync FTP, fila de atendimento em tempo real. Pega de healthz + module registry + alerts.",
    scope: [
      "Painel de modulos ativos (do module registry)",
      "Historico de jobs ERP",
      "Latencia /healthz dos ultimos 7 dias"
    ],
    dependsOn: []
  },
  {
    id: "qr-tools",
    label: "Tools secundarias (QR / Encurtador / Scripts)",
    route: "/tools/qr-code",
    status: "pending",
    priority: "P3",
    category: "tools",
    description:
      "Ferramentas auxiliares hoje meio implementadas. Atualmente ocultas do menu. Reativar so quando tiver demanda real.",
    scope: [
      "QR Code com tracking de cliques",
      "Encurtador integrado com tracking",
      "Scripts: snippets reutilizaveis de mensagens"
    ],
    dependsOn: []
  }
];

export const ROADMAP_RULES: RoadmapRule[] = [
  {
    id: "fe-componentes-reutilizaveis",
    category: "frontend",
    title: "Componentes reutilizaveis acima de tudo",
    body: "Sempre que houver repeticao de markup ou logica visual, extrair em componente proprio em web/app/components/ ou na layer adequada. Workspaces nao podem virar arquivos gigantes; quebrar em cards/secoes/listas.",
    why: "Evita duplicacao e drift visual entre paginas. Facilita aplicar mudanca em um lugar so.",
    appliesWhen: "Qualquer feature nova ou refactor que adicione UI."
  },
  {
    id: "fe-classes-semanticas",
    category: "frontend",
    title: "Classes semanticas BEM-like",
    body: "Sempre usar nomes semanticos no estilo .nome-componente__elemento--modificador. Nao usar utility classes inline ou IDs para estilizacao.",
    why: "Permite leitura rapida do escopo de cada estilo e evita colisao global.",
    appliesWhen: "Estilizacao de qualquer componente novo."
  },
  {
    id: "fe-design-system-tokens",
    category: "frontend",
    title: "Seguir o design system — usar as variaveis de cor, nunca hex hardcoded",
    body: "O projeto TEM design system (tokens em web/app/assets/styles/omni-tokens.css + aliases em tokens.css). Toda cor/borda/sombra/raio usa as variaveis existentes: rgb(var(--primary)), rgb(var(--primary) / 0.16), var(--text-main), var(--text-muted), var(--line-soft), rgb(var(--surface) / 0.9), var(--shadow-card), var(--radius-card). NUNCA hex/rgb cravado nem inventar nome de variavel (ex.: --color-primary nao existe). Botao primario = linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600))).",
    why: "Aconteceu (AutomationWorkspace.vue): o CSS usava var(--color-primary, #16a34a) etc.; esses nomes --color-* nao existem, entao caia no fallback hex e o componente ignorava o tema/dark mode do resto do painel.",
    appliesWhen: "Qualquer <style> de componente novo ou refactor. Conferir o token em omni-tokens.css/tokens.css; se a cor nao existe como token, perguntar/adicionar token, nunca cravar hex."
  },
  {
    id: "fe-pagina-rolagem",
    category: "frontend",
    title: "Pagina nova precisa rolar como as outras",
    body: "O layout dashboard envolve a pagina em .module-workspace-full que e overflow:hidden. O componente-raiz da pagina precisa ser o container de rolagem (flex:1; min-height:0; overflow-y:auto) ou usar o wrapper .page-workspace. Sem isso o conteudo que passa da altura fica cortado, sem scroll.",
    why: "Aconteceu (AutomationWorkspace.vue): a raiz so tinha padding, sem flex/overflow, entao o editor de persona longo ficava cortado e a pagina nao rolava.",
    appliesWhen: "Criar componente-raiz de pagina/workspace nova no layout dashboard."
  },
  {
    id: "fe-sem-emojis",
    category: "frontend",
    title: "Sem emojis em UI nem em codigo",
    body: "Nao usar emojis em labels, mensagens de UI, codigo, comentarios ou commits, salvo se o usuario pedir explicitamente.",
    why: "Mantem consistencia visual e profissional do produto.",
    appliesWhen: "Sempre."
  },
  {
    id: "fe-feature-flag-hidden",
    category: "frontend",
    title: "Esconder pagina nao pronta via hidden no menu",
    body: "Quando um modulo/pagina nao esta pronto, usar hidden:true em web/app/utils/sidebar-nav.ts E em web/layers/queue/nav.config.ts. Para itens em beta, usar beta:true (renderiza badge).",
    why: "Evita que usuario navegue para pagina quebrada. Beta deixa explicito que a feature pode mudar.",
    appliesWhen: "Adicionar/remover modulo do menu lateral."
  },
  {
    id: "be-padrao-modulo-go",
    category: "backend",
    title: "Padrao de modulo Go",
    body: "Cada modulo em back/internal/modules/<nome>/ tem: model.go (tipos), store_postgres.go (persistencia), service.go (regras), http.go (handlers), AGENT.md (documentacao). Modulos plugaveis se registram via Module Registry quando CORE_V2_ENABLED.",
    why: "Consistencia entre modulos facilita onboarding e troca de agente.",
    appliesWhen: "Criar novo modulo backend."
  },
  {
    id: "be-ids-strings",
    category: "backend",
    title: "IDs como string, nunca uuid externo",
    body: "Usar string para IDs no Go; nao importar pacote uuid externo. Casts e geracao ficam centralizados em internal/platform/ids/.",
    why: "Reduz dependencia externa e facilita refatoracao do esquema de IDs.",
    appliesWhen: "Qualquer struct nova com ID."
  },
  {
    id: "be-scan-nullable-string",
    category: "backend",
    title: "Scan de campos NULL com *string",
    body: "Para colunas nullable, declarar *string (ou sql.NullString se preferir) no Scan; nunca string puro.",
    why: "Evita panic em scan de NULL.",
    appliesWhen: "Implementar store_postgres.go."
  },
  {
    id: "be-perms-no-banco",
    category: "backend",
    title: "Permissoes vivem no banco (RBAC dinamico)",
    body: "Nao hardcoded permission names em codigo Go. Permissoes vivem em core.permissions + core.role_permissions; service consulta via Module Registry.",
    why: "Permite agencia customizar role sem deploy.",
    appliesWhen: "Implementar checagem de permissao em handler ou service."
  },
  {
    id: "banco-migration-idempotente",
    category: "banco",
    title: "Migration idempotente (IF NOT EXISTS)",
    body: "Toda migration usa IF NOT EXISTS / CREATE OR REPLACE. Numerar sequencialmente em back/internal/platform/database/migrations/####_nome.sql. Nunca renumerar migration ja aplicada.",
    why: "Migrations falhas no meio precisam poder ser reaplicadas sem dropar dados.",
    appliesWhen: "Criar migration nova."
  },
  {
    id: "banco-schema-multitenant",
    category: "banco",
    title: "Schema-per-modulo + account_id em todas as tabelas tenant-scoped",
    body: "Schemas: core, queue, tasks, alerts, settings, roadmap. Toda tabela tenant-scoped tem account_id NOT NULL com FK para core.accounts. Public schema pode ter VIEWS sobre tabelas dos schemas.",
    why: "Multi-tenancy com isolamento logico e queries por schema mais previsiveis.",
    appliesWhen: "Criar tabela nova."
  },
  {
    id: "banco-view-publica",
    category: "banco",
    title: "Mover tabela para schema: criar view publica",
    body: "Quando mover tabela de public.* para schema.*, criar CREATE OR REPLACE VIEW public.<tabela> AS SELECT * FROM schema.<tabela> para manter compat com codigo legado.",
    why: "Evita quebrar queries antigas que ainda apontam para public.*.",
    appliesWhen: "Refactor de schema."
  },
  {
    id: "lang-go-version",
    category: "linguagens",
    title: "Go 1.26",
    body: "Backend usa Go 1.26. Aproveitar generics, max/min builtins, slices/maps stdlib.",
    why: "Versao alinhada com infra de CI e Docker.",
    appliesWhen: "Backend."
  },
  {
    id: "lang-vue-nuxt",
    category: "linguagens",
    title: "Vue 3 + Nuxt 4 + Pinia",
    body: "Frontend usa Vue 3 (Composition API + <script setup>), Nuxt 4 (com layers em web/layers/*), Pinia para state. Tipos TS sempre que possivel.",
    why: "Stack escolhida pelo time; layers permitem isolar dominios.",
    appliesWhen: "Frontend."
  },
  {
    id: "lang-typescript-strict",
    category: "linguagens",
    title: "TypeScript strict",
    body: "Codigo TS deve passar em vue-tsc --noEmit. Evitar any. Preferir tipos explicitos em props e composables.",
    why: "Pega bug em build time, nao em prod.",
    appliesWhen: "Qualquer codigo TS/Vue."
  },
  {
    id: "deploy-vps-caddy",
    category: "deploy",
    title: "VPS Hostinger com Caddy + Docker Compose",
    body: "Deploy em VPS 85.31.62.33, user deploy. Caddy reverse proxy em /opt/omnichannel/Caddyfile. Cada projeto roda em /home/deploy/<projeto> com docker-compose.prod.yml. Nginx-style aliases por projeto na network proxy.",
    why: "Isolamento por projeto + um Caddy gerencia todos os dominios.",
    appliesWhen: "Deploy ou troubleshooting de prod."
  },
  {
    id: "deploy-feature-flag",
    category: "deploy",
    title: "Feature flag CORE_V2_ENABLED em .env.production E docker-compose",
    body: "Variaveis novas precisam de duas adicoes: .env.production (na VPS) E docker-compose.prod.yml na secao environment. Sem a segunda, o container nao recebe a variavel.",
    why: "Compose nao propaga automaticamente .env file inteiro; precisa de declaracao explicita.",
    appliesWhen: "Adicionar variavel de ambiente nova."
  },
  {
    id: "deploy-caddy-restart",
    category: "deploy",
    title: "Apos mudar upstream, restart Caddy (nao reload)",
    body: "Caddy reload mantem cache do upstream antigo em alguns casos. Para garantir, fazer docker restart omnichannel-mvp-caddy-1.",
    why: "Sintoma classico: site continua mostrando versao antiga apos deploy.",
    appliesWhen: "Trocar upstream Caddy ou criar novo dominio."
  },
  {
    id: "geral-doc-first",
    category: "padroes-gerais",
    title: "Documentar antes de implementar",
    body: "Antes de codar feature nao trivial: criar fase pending no roadmap-data.ts (status:'pending', tasks done:false), apresentar plano ao usuario, so depois codar.",
    why: "Evita retrabalho e mantem roadmap como fonte de verdade para o agente.",
    appliesWhen: "Tarefa com 3+ passos ou impacto em multiplas camadas."
  },
  {
    id: "geral-agent-md",
    category: "padroes-gerais",
    title: "Atualizar AGENT.md ao alterar modulo",
    body: "Toda mudanca em modulo backend (ou layer/area significativa do front) reflete no AGENT.md correspondente: novos endpoints, novas tabelas, novos contratos.",
    why: "AGENT.md e a fonte que outros agentes leem para entender o modulo.",
    appliesWhen: "PR que mexe em modulo."
  },
  {
    id: "geral-sem-coauthor",
    category: "padroes-gerais",
    title: "Sem Co-Authored-By Claude em commits",
    body: "Commits nao devem ter Co-Authored-By: Claude. Atribuicao fica so com o desenvolvedor humano.",
    why: "Preferencia explicita do mantenedor.",
    appliesWhen: "Toda criacao de commit."
  },
  {
    id: "geral-local-first",
    category: "padroes-gerais",
    title: "Validar local antes de qualquer coisa",
    body: "Sempre rodar e testar local antes de propor commit ou deploy. UI changes precisam de browser test, nao so type-check.",
    why: "Type-check + test suite validam corretude de codigo, nao de feature.",
    appliesWhen: "Sempre."
  }
];

export const ROADMAP_MODULE_STATUS_LABEL: Record<ModuleStatus, string> = {
  pending: "Pendente",
  in_progress: "Em andamento",
  beta: "Beta",
  done: "Concluido"
};

export const ROADMAP_PRIORITY_LABEL: Record<ModulePriority, string> = {
  P0: "P0 - Critica",
  P1: "P1 - Alta",
  P2: "P2 - Media",
  P3: "P3 - Baixa"
};

export const ROADMAP_RULE_CATEGORY_LABEL: Record<RuleCategory, string> = {
  frontend: "Frontend",
  backend: "Backend",
  banco: "Banco",
  linguagens: "Linguagens",
  deploy: "Deploy",
  "padroes-gerais": "Padroes Gerais"
};
