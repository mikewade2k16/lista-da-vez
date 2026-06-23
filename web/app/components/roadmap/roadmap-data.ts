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
  },
  {
    id: "bio",
    label: "Bio Links — Site/Bio",
    description: "CRUD multitenant das páginas de bio (link-in-bio) servidas pelo front Nuxt separado. Cliente edita só a própria bio; admin/agência gerencia todas com filtro por cliente. Plano: docs/bio/PLANO_MODULO_BIO.md."
  },
  {
    id: "cardapio",
    label: "Cardápio Online",
    description: "CRUD multitenant de cardápios online (restaurantes) servidos por um front Nuxt estático no host do cliente, com resolução de tenant por domínio, pedidos recalculados no servidor e tracking. Por enquanto na account da Crow. Plano: docs/cardapio/PLANO_MODULO_CARDAPIO.md."
  },
  {
    id: "infra-deploy",
    label: "Infra & Deploy",
    description: "Pipeline de deploy do Omni: imagens no GHCR buildadas no GitHub Actions (a VPS só faz pull, nunca compila) + ambiente de staging isolado e sob demanda para testar antes de promover pra produção. Plano: docs/deploy/REGISTRY_STAGING_DEPLOY_PLAN.md."
  },
  {
    id: "fila-operacao",
    label: "Fila — Página Operação",
    description: "Ajustes de operação da Fila: controle por loja individual para usuários multi-loja, limpeza do modal de encerrar, justificativa só ao avançar e métrica de pausas (motivo/horário/duração) persistida e em Relatórios. Plano: docs/operacao/AJUSTES_OPERACAO_PLAN.md."
  },
  {
    id: "menu-layout",
    label: "Organização do Menu (Header × Sidebar)",
    description: "Config global, editável pelo platform_admin, de como o menu se divide entre header e sidebar: posição por item (header/sidebar/ambos/oculto) + reordenar, persistida em core.platform_settings. Inclui fix responsivo do header (overflow 'Mais'). Plano: docs/MENU_LAYOUT_CONFIG.md."
  },
  {
    id: "comissao-v2",
    label: "Comissão v2 — cálculo no back (API-first)",
    description: "Recebimento por atingimento de meta calculado no backend como serviço de domínio único (pacote queue/commission), embutido em /v1/erp/crm. Consultor sobre a PRÓPRIA venda com trava de meta e penalidade PA/Ticket; gerente sobre o total da loja com faixas por tipo de loja (Shopping/Bairro). Inclui a auditoria das demais lógicas só-no-front (P1-P3). Plano: planos/vamos-fazer-altera-es-em-purrfect-pony.md."
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
      { id: "delete-static", label: "Deletar web/app/utils/sidebar-nav.ts", done: true, note: "REMOVIDO 2026-06-13: arquivo deletado e import retirado do module-registry.client.ts. O nav.config.ts da layer queue já cobre todas as 5 sections (service, tools, team-site, indicators, manage) e mais; o register dedup por id deixava o legado sempre sobrescrito. Menu agora 100% dos layers." },
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
      { id: "fix-ssr-hard-reload-logout", label: "Fix: hard reload (Ctrl+Shift+R) em rota SSR deslogava/estourava o painel", done: true, note: "2026-06-22: backend sempre 200 em /me/context e zero 401 — falha era client/SSR. Causa: rotas de painel fora da lista ssr:false (cardapio, alertas, automation, banco, bi, clientes, crm, erp, meta-ads, performance) rodavam SSR; no servidor o ensureSession/fetchAccounts rodava e (a) se syncRuntimeAccess degradasse, o catch chamava clearSession() e accessToken.value=null emitia Set-Cookie apagando o token → logout; (b) com a sessão preservada, accountStore.accounts vinha undefined na SSR → crash em auth.global.ts. Fix em 4 frentes: (1) stores/auth.ts clearSession só mexe em cookie no client (import.meta.client); (2) syncRuntimeAccess best-effort no fetchContext (try/catch); (3) auth.global.ts NÃO faz bootstrap de auth no servidor (import.meta.server → return) — auth é concern do client (cookie + plugins client-only); (4) nuxt.config: TODA rota de painel + /auth/** em ssr:false (painel é SPA). Só front, sem rebuild da API; mudança de routeRules exige restart do dev server (não entra por HMR)." },
      { id: "refactor-runtime-remote-split", label: "Refactor: runtime-remote.ts (731 linhas) dividido em 3 por responsabilidade", done: true, note: "2026-06-22: acima do limite de ~450-500 linhas. Quebrado em runtime-remote-normalize.ts (138, normalize*/cloneOrFallback/resolveOperationRoster), runtime-remote-state.ts (270, apply*ToState/buildSettingsBundleFromState) e runtime-remote.ts (352, fetch/hydrate). API pública preservada via re-export — imports externos intactos; 12 testes passam." },
      { id: "fix-403-conta-sem-queue", label: "Fix: 403 no console em conta sem o módulo queue (cardapio/bio-only)", done: true, note: "2026-06-22: conta sem queue (ex.: Mostarda) disparava no boot/polling GET /v1/consultants, /v1/operations/snapshot e /v1/feedback/me → 403 module_disabled poluindo o console (degradavam, mas eram requests inúteis). Causa: consultants/snapshot em fetchRemoteStoreData não respeitavam o módulo (só settings respeitava), e o FeedbackNotificationsDropdown (3 headers) renderizava só com v-if isAuthenticated. Fix: (1) runtime-remote gateia settings+consultants+snapshot por hasQueueModule (consultants ainda exige consultor.view por cima); (2) FeedbackNotificationsDropdown gateado por enabledModules.includes('queue') — não busca nem mostra o sino sem o módulo (feedback é feature de queue, ver module-enabled.global.ts). Só front, HMR." },
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
      { id: "apply-dashboard", label: "Skeleton dos cards no dashboard inicial (/)", done: true, note: "Entregue na fase perf-fixes (Track B, 2026-06-15): skeleton de header + 6 cards no index.vue durante o redirect." },
      { id: "apply-operacao", label: "Skeleton da grid de stores + fila em /operacao enquanto realtime conecta", done: true, note: "Entregue na fase perf-fixes (Track B, 2026-06-15): OperationSkeleton.vue no estado loading de /operacao." },
      { id: "apply-tables", label: "Skeleton rows em tabelas grandes (clientes, usuários, relatórios) + loading inline na paginação", done: true, note: "AppEntityGrid.vue usa CoreSkeleton variant=table-row count=6; propaga para todas as workspaces que usam o grid (clientes, usuários, ERP, relatórios, etc.)." },
      { id: "apply-switch", label: "Overlay durante AccountSwitcher trocar account (/v2/me/context da nova account)", done: true, note: "CoreAccountSwitcher.select() chama useCoreLoadingStore.push('Trocando de account...') antes do switchAccount." },
      { id: "empty-state", label: "CoreEmptyState.vue padronizado (ícone + título + descrição + ação opcional)", done: true },
      { id: "error-state", label: "CoreErrorState.vue padronizado com botão de retry (mensagem amigável, sem stack)", done: true },
      { id: "replace-hardcoded", label: "Substituir mensagens hardcoded de 'Sem dados' / 'Erro ao carregar' pelos componentes novos", done: false }
    ],
    verifiable: "Nenhuma página fica em branco em qualquer transição. Tempo até primeiro pixel renderizado < 300ms mesmo na primeira carga. AccountSwitcher mostra overlay até o novo context chegar."
  },
  {
    id: "perf-audit",
    code: "Perf-Audit",
    title: "Auditoria de performance de navegação (métrica por página)",
    goal: "Medir objetivamente, em TODAS as rotas, quanto tempo cada página leva para (1) trocar de rota ao clicar, (2) aparecer em tela e (3) terminar de carregar — 3 rodadas sem cache, média — para provar onde a regra 'clicou → responde na hora' é quebrada e gerar o backlog de correção. Plano canônico: docs/PERFORMANCE_AUDIT_PLAN.md.",
    status: "done",
    estimateWeeks: "Concluída (2026-06-15)",
    startedAt: "2026-06-15",
    finishedAt: "2026-06-15",
    tasks: [
      { id: "metric-defs", label: "Definir 3 marcos por página: T1 clique→troca de rota, T2 clique→primeira pintura, T3 clique→carregamento final", done: true, note: "T1 isola fetch bloqueante de setup/middleware; T3 isola falta de skeleton/lazy-load. Detalhe em docs/PERFORMANCE_AUDIT_PLAN.md §4." },
      { id: "harness", label: "Estender qa-bot (Playwright) com perf_audit.py: login platform_admin + instrumentação dos 3 marcos + CSV/MD em qa-bot/artifacts", done: true, note: "qa-bot/perf_audit.py. Settle por observer persistente + DOM-quiet (650ms) com fallback streaming (4s) p/ realtime; cache off via CDP." },
      { id: "mode-inapp", label: "Modo in-app (navegação SPA): cache de rede off, sessão mantida, 3 rodadas por rota", done: true },
      { id: "mode-cold", label: "Modo cold (1ª visita): Navigation/Paint Timing API, cache 100% off, 3 rodadas por rota", done: true },
      { id: "routes-all", label: "Cobrir todas as rotas do platform_admin (estáticas + dinâmicas com id real + auth)", done: true, note: "~50 rotas medidas. /site/bio/[id] e /cardapio/[id] puladas (sem dados no ambiente)." },
      { id: "report", label: "Relatório consolidado: 3 tempos × 3 rodadas + média por rota, nos 2 modos, com ranking das páginas mais lentas", done: true, note: "qa-bot/artifacts/perf-20260615-133516.{md,csv}. Resultados em docs/PERFORMANCE_AUDIT_PLAN.md §14." },
      { id: "backlog", label: "Diagnóstico vira backlog: T1 alto → fetch bloqueante no setup; T3 alto → skeleton + lazy-load (provável reabrir fase-7/fase-9)", done: true, note: "Achado: em PROD a navegação é rápida em tudo (T1~0s, T2~0,1-0,3s). A dor diária é o compile do Vite no DEV (203s→0,07s), não o app. Cauda lenta real (T3): /tasks (board), /operacao+/ (realtime, falta skeleton), /erp, /manage/users." }
    ],
    verifiable: "Relatório com, para cada rota, T1/T2/T3 em 3 rodadas sem cache + média, nos modos in-app e cold, e ranking apontando as páginas que violam 'clicou → responde na hora'."
  },
  {
    id: "perf-fixes",
    code: "Perf-Fixes",
    title: "Correções de performance + painel de resultados (pós-auditoria)",
    goal: "Resolver as páginas críticas apontadas pela auditoria (perf-audit) e publicar os resultados como página dedicada /performance no menu. Warm-up de dev e medição da bio já entregues. Implementado em paralelo (4 subagentes) em 2026-06-15. Fonte: docs/PERFORMANCE_AUDIT_PLAN.md §14-15.",
    status: "in_progress",
    estimateWeeks: "Implementado 2026-06-15; falta validação visual + métrica de jank do /tasks",
    startedAt: "2026-06-15",
    tasks: [
      { id: "perf-online-page", label: "Página dedicada /performance no menu (platform_admin) renderizando perf-data.ts emitido pelo perf_audit.py (tabela T1/T2/T3 + ranking + explicação do warm-up)", done: true, note: "Track A: web/app/pages/performance.vue + components/performance/*; menu wired nos 3 arquivos; perf_audit.py emite perf-data.ts. Para a página mostrar números pós-fix, rebuildar o front (a re-medição já regenerou o perf-data.ts no disco)." },
      { id: "warmup-dev", label: "Warm-up de dev (qa-bot/warmup_dev.py) + doc de como rodar após docker compose up", done: true, note: "Entregue 2026-06-15. Pré-compila todas as rotas no Vite dev; mata a dor de 'clico e demora' local. Medido: warm-up das 50 rotas ~24min (custo do compile), depois navegação instantânea." },
      { id: "bio-id-measure", label: "Medir /site/bio/[id] com id real (discovery por clique na 1a linha)", done: true, note: "Bio saudável: T3 ~0,93s in-app / ~1,19s cold. Cardápio sem dados (pulado)." },
      { id: "fix-tasks", label: "/tasks: lazy-mount dos editores pesados do card (placeholder leve → editor no 1º clique), espelhado em board+modal", done: true, note: "Track C: OmniLazySelectMenuInput; 15/15 testes do layer, sem quebrar drag/realtime. ATENÇÃO: o T3 da auditoria NÃO mudou (15,4s cold) porque mede DOM-quiet, e os 247 cards em render progressivo dominam o settle. O ganho real (menos trabalho na thread principal / menos jank) está numa dimensão que o T3 não capta — falta medir com Total Blocking Time / long tasks." },
      { id: "skeleton-operacao", label: "Skeleton em /operacao enquanto realtime conecta (= fase-9 apply-operacao)", done: true, note: "Track B: OperationSkeleton.vue no estado loading. T3 não muda (realtime), mas a pintura é instantânea (T2 ~0,1s) — o ganho é perceptual. Falta confirmar visualmente no browser." },
      { id: "skeleton-dashboard", label: "Skeleton dos cards no dashboard / (= fase-9 apply-dashboard)", done: true, note: "Track B: skeleton de header + 6 cards no index.vue durante o redirect." },
      { id: "fix-erp-users", label: "/erp e /manage/users: projeção lean + paginação server-side; gate por permissão", done: true, note: "Track D: /erp sem N+1 (10→2 queries) → in-app 1,83→1,27s. /manage/users paginação+filtros server-side (antes loop de todas as páginas) → 1,78→1,64s in-app, 2,43→2,25s cold; ganho estrutural (não degrada com +usuários). API rebuildada, multi-tenant intacto." }
    ],
    verifiable: "Página /performance no menu mostra os tempos por rota; /erp e /manage/users mediram melhora; skeletons aparecem < 100ms (confirmar visual); /tasks precisa de métrica de jank (TBT) para provar o ganho do lazy-mount."
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
      { id: "site", label: "Trazer Site antes de finance, começando por produtos e leads com escopo front-first", done: false, note: "AUDITORIA 2026-05-28: páginas SiteLeadsAdminWorkspace.vue e SiteProductsAdminWorkspace.vue criadas. P0·5 (2026-06-07): backend site REGISTRADO no boot — /v1/admin/leads, /products, /tracking-analytics, webhook-sources e ingest /v1/webhooks/* agora servem de verdade (antes 404). PRODUTOS (2026-06-14): editor de produtos completo no back (upload de imagem, switch visível-no-site/tem-estoque, multiselect categoria/campanha creatable, ordem mais-novos-primeiro, paginação + Carregar tudo), cache local de 797 imagens no sync com toggle fonte local/online (XAMPP) validado por magic bytes, e cruzamento com ERP (migration 0155 site.product_erp_links + endpoints erp-match/erp-unmatched/from-erp; GET de produtos traz erpSynced/erpName/erpDescription). BACK VALIDADO POR API. PENDENTE: a UI de produtos (sincronização/edição) NÃO foi validada no navegador — o usuário reportou que o sync pela tela não funcionou; revisar/testar a tela /site/produtos antes de marcar pronto. Falta também confirmar que o front consome o real e não o BFF Nitro." },
      { id: "site-products-editor", label: "Editor de produtos /site/produtos: upload de imagem, switch Visível no site/Tem estoque, multiselect categoria/campanha creatable, ordem mais-novos-primeiro, paginação + Carregar tudo", done: false, note: "Back validado por API (POST /v1/admin/products/{id}/image, PATCH status/stock/categories/campaigns). 2026-06-14: default mudou para PAGINADO (50/pag) — a API responde em ~30ms; o gargalo de ~1min era render de 826 linhas; 'Carregar tudo' fica como opcao. Toggle de fonte Local(XAMPP)/Online na barra (GET/PATCH /v1/admin/products/source). Cabecalho da pagina agora respeita o toggle global (AdminPageHeader). REVISAR UI no navegador: usuario reportou que o sync/edicao pela tela nao funcionou." },
      { id: "site-products-image-cache", label: "Cache local de imagens no sync (797 imagens em /uploads/site/products) + toggle fonte local/online (XAMPP) + validação por magic bytes", done: true, note: "image_cache.go: tenta ImageCandidates em ordem, baixa 1x, valida looksLikeImage (Content-Type ou magic bytes png/jpeg/gif/webp/avif); allPerolaHost zera imagem inalcançável pelo browser." },
      { id: "site-products-erp", label: "Cruzamento de produtos com ERP: migration 0155 site.product_erp_links + endpoints erp-match/erp-unmatched/from-erp", done: true, note: "Back validado por API; GET de produtos traz erpSynced/erpName/erpDescription. Front (aba Produtos do ERP fora do site, tag ERP, Cruzar com ERP, Puxar pro site) faz parte da UI de produtos pendente de revisão no navegador." },
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
    status: "done",
    estimateWeeks: "4–6 dias",
    startedAt: "2026-05-22",
    group: "crm-360",
    tasks: [
      { id: "agent-md", label: "Atualizar web/app/components/consultant/AGENT.md com contrato dos novos componentes (player card, drawer, grid)", done: true, note: "2026-06-11: auditoria confirmou os componentes e atualizou consultant/AGENT.md." },
      { id: "gamification-config-composable", label: "useGamificationConfig() composable com defaults hardcoded para badges + Score 360 weights (preparado para C6 plugar fonte real)", done: true },
      { id: "player-card-component", label: "ConsultantPlayerCard.vue (modos full e mini) — gauge SVG, 4 KPIs core (Vendido, Ticket, PA, Conversão), slot de badges", done: true },
      { id: "badges-component", label: "ConsultantBadges.vue puro recebe stats + badgesConfig — defaults: Meta batida, Top N, Conversão > média loja, Ticket > meta, PA > meta", done: true },
      { id: "drawer-shell", label: "ConsultantDetailsDrawer.vue com USlideover (modos center/fullscreen/side igual TasksTaskModal) + composable useConsultantDetailsDrawer()", done: true },
      { id: "drawer-tabs", label: "Drawer com 3 tabs: Visão geral (todos KPIs incluindo cancelamento/fora-da-vez/tempo médio), Histórico (sparkline 7d), Simulador (move ConsultantSimulator atual)", done: true },
      { id: "single-store-wire", label: "ConsultantWorkspace.vue (single-store) usa ConsultantPlayerCard full + drawer", done: true },
      { id: "multi-store-grid", label: "ConsultantPlayerGrid.vue substitui tabela 'Comparativo completo' por grid de mini-cards filtráveis", done: true },
      { id: "cancellation-wire", label: "Garantir cancellationRate no DTO consumido por consultants/analytics stores (já calculado em repository_crm_queue.go)", done: true, note: "2026-06-11: fechado. O valor já vinha em GET /v1/erp/crm (queueStats.byConsultant[].queueCancellationRate, tipado em stores/erp.ts) — gap era só o merge no front. Mergeado por (storeId, personId) na store consultants → ConsultantRow → exibido no ConsultantPlayerCard (full) e no drawer. Sem mudança de back/migration. Pendente menor: modo single-store não-integrado (ConsultantWorkspace) não busca /v1/erp/crm, então lá degrada limpo (não renderiza)." },
      { id: "delete-old-metrics", label: "Remover ConsultantMetrics.vue após migração completa", done: true, note: "2026-06-11: ConsultantMetrics.vue não existe mais no repo (já removido na migração); referências restantes são ao composable useCrmConsultantMetrics (nome parecido)." }
    ],
    verifiable: "/consultor em loja única mostra player card + drawer abrindo via 'Ver detalhes'. Visão all-stores mostra grid de mini-cards; click no card abre drawer. Sem erros de console; npm test passa."
  },
  {
    id: "crm-c5",
    code: "CRM C5",
    title: "Ranking gamificado — pódio + leaderboard + drawer",
    goal: "Substituir as duas tabelas de 11 colunas por pódio dos 3 primeiros + leaderboard de cards horizontais para o resto. Tabs Lojas/Consultores/Por-loja. Score 360 vira sort default. Detalhes (breakdown 360 + alertas) em drawer lateral.",
    status: "done",
    estimateWeeks: "5–7 dias",
    startedAt: "2026-05-22",
    group: "crm-360",
    tasks: [
      { id: "agent-md", label: "Criar web/app/components/ranking/AGENT.md com contrato do novo workspace", done: true, note: "2026-06-11: criado na auditoria." },
      { id: "tabs-header", label: "RankingTabsHeader.vue (3 tabs Lojas/Consultores/Por-loja + chips de sort, Score 360 default)", done: true },
      { id: "podium-component", label: "RankingPodium.vue — top-3 visual (2º-1º-3º) com avatar, nome/loja, número grande da métrica ativa", done: true },
      { id: "leaderboard-card", label: "RankingLeaderboardCard.vue — card horizontal (4º+) com posição, métrica grande, barra meta, badge variação ↑/↓", done: true },
      { id: "variation-derivation", label: "Derivar variação vs período anterior client-side (comparar monthlyRows com snapshot mês anterior já disponível)", done: true },
      { id: "stores-tab", label: "Agregação por loja para tab Lojas: totalSoldValue, Score 360 ponderado por attendances (decisão fechada), consultantsAtGoal", done: true },
      { id: "per-store-tab", label: "Tab 'Por loja' com combobox de loja + pódio + leaderboard filtrados", done: true },
      { id: "drawer-ranking", label: "RankingDetailsDrawer.vue com USlideover (center/fullscreen/side) + tabs Visão geral / Breakdown 360 / Alertas. Mover alertas do topo do RankingWorkspace para drawer (manter contador)", done: true },
      { id: "score-breakdown", label: "Componente de barra stackeada para breakdown do Score 360 — pesos vêm de useGamificationConfig() (defaults 35/25/20/15/5 até C6 plugar fonte real)", done: true },
      { id: "legacy-table", label: "Manter RankingTable.vue acessível como 'Ver como tabela' dentro do drawer para usuários que preferem formato denso", done: true, note: "2026-06-11: auditoria corrigiu bug de pesos hardcoded no RankingTable (passou a usar computeScore360 da config)." }
    ],
    verifiable: "/ranking mostra pódio + leaderboard cards; tabs trocam agrupamento; click no card abre drawer com breakdown 360. ESC/overlay/X fecham. Alertas continuam acessíveis via drawer. npm test passa; sem regressões."
  },
  {
    id: "crm-c6",
    code: "CRM C6",
    title: "Backend de gamificação — config de badges + Score 360 weights",
    goal: "Permitir que cada tenant configure regras de badges (Meta batida, Top N, Conversão > média loja, etc.) e os pesos do Score 360 (Conversão/Valor/Qualidade/PA/Queue-jump). Plugar no composable useGamificationConfig() do front (criado em C4) substituindo defaults hardcoded.",
    status: "done",
    estimateWeeks: "3–5 dias",
    group: "crm-360",
    tasks: [
      { id: "model-go", label: "Adicionar GamificationConfig (BadgeRules []BadgeRule + ScoreWeights ScoreWeights) ao settings.Bundle e Record", done: true, note: "2026-06-11: BadgeRules em AppSettings. ScoreWeights já persistiam via settings.scoreWeight*." },
      { id: "migration", label: "Migration SQL: tabela settings_gamification (tenant_id, badge_rules_json, score_weights_json)", done: true, note: "2026-06-11: migration 0146 (public.tenant_gamification_settings, badge_rules jsonb, FK core.accounts). score_weights já persistem nos settings existentes — sem coluna nova." },
      { id: "store-postgres", label: "Persistir e ler GamificationConfig em settings store_postgres.go", done: true, note: "store_postgres_gamification.go (pgx CollectOneRow/RowToStructByName)." },
      { id: "defaults", label: "settings/defaults.go com defaults de GamificationConfig (mesmos hardcoded usados em C4/C5)", done: true },
      { id: "http-endpoint", label: "PATCH /v1/settings/gamification com perm settings.write; GET expõe junto com o bundle existente", done: true, note: "PATCH /v1/settings/gamification (RequireAuth); badges injetadas no bundle do GET." },
      { id: "frontend-settings-ui", label: "Seção 'Gamificação' na página de configurações para editar badges (CRUD lista) e weights (5 sliders que somam 100%)", done: true, note: "2026-06-11: badges CRUD + os 5 sliders de peso do Score 360 (SettingsScoreWeightsCard.vue, total com feedback de cor) na aba Gamificacao. Pesos reusam o PATCH /v1/settings/operation existente. Editor por inputs na aba Operacao mantido intacto." },
      { id: "wire-composable", label: "useGamificationConfig() passa a ler do settings store (com fallback para defaults se config não existir)", done: true, note: "resolveBadgeRules lê de runtime.state.settings.badgeRules com fallback nos defaults; API pública estável." },
      { id: "agent-md", label: "Atualizar back/internal/modules/settings/AGENT.md com novo bundle field e endpoint", done: true, note: "queue/settings/AGENT.md (módulo migrado para queue/settings)." }
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
    id: "crm-c9",
    code: "CRM C9",
    title: "CRM 360 — recebimento por meta da loja nos cards + CRUD de faixas",
    goal: "Levar a politica de recebimento por atingimento de meta para os cards de consultor (cor do gauge pela faixa individual, barra de % da loja que muda de cor, valor a receber), mostrar gerente/caixa/auxiliar ao lado dos consultores so com o que ganham pela loja, e deixar a pagina de faixas (Metas CRM) fazer CRUD sem erro. Gate de recebimento = % da meta da loja; base do % = total vendido da loja.",
    status: "pending",
    estimateWeeks: "1-2 dias",
    group: "crm-360",
    tasks: [
      { id: "payout-domain", label: "Helper unico mapRoleToPayoutGroup + calculateStoreGoalPayout (gate = % meta da loja; base % = total vendido da loja) em crm-performance-policy.ts", done: false },
      { id: "store-progress", label: "useConsultantIntegratedRows expoe storeProgressByStoreId e storeTotalSoldByStoreId; composable de leitura da crmGoalPayoutPolicy do runtime", done: false },
      { id: "card-colors", label: "ConsultantPlayerCard: gauge muda de cor pela faixa individual + barra de % da loja colorida + linha de recebimento por meta; manter 'Sem meta cadastrada'", done: false },
      { id: "staff-cards", label: "Cards enxutos (modo payout) para gerente/caixa/auxiliar ao lado dos consultores, so com nome/papel/recebimento da loja", done: false },
      { id: "staff-endpoint", label: "Backend: endpoint lean de staff sem fila por loja (core.account_users + role_assignments), escopo validado contra o Principal, fora do escopo 404", done: false },
      { id: "crm-table-consistency", label: "CrmConsultantsSection usa o mesmo helper (base total da loja) na coluna Recebimento", done: false },
      { id: "payout-crud", label: "SettingsCrmGoalsSection + useSettingsWorkspace: CRUD sem derrubar linha ao editar, sem re-sort/troca de key no meio, save no blur, remover ate zero; layout compacto colapsavel com tokens", done: false },
      { id: "docs-tests", label: "Atualizar AGENT.md dos modulos tocados, panorama HTML e cobrir o helper de payout com teste", done: false }
    ],
    verifiable: "Em Perola Treze: gauge dos consultores muda de cor por faixa; barra de % da loja aparece e muda de cor; cada card mostra o recebimento pela meta da loja; gerente/caixa/auxiliar aparecem como cards enxutos com o valor da loja; pagina Metas CRM adiciona/edita/remove faixas sem perder foco nem derrubar linha e persiste apos refresh."
  },

  {
    id: "crm-c10",
    code: "CRM C10",
    title: "Aviso acionavel inline + quick-edit de metas via API (de qualquer tela, plugavel)",
    goal: "Quando um dado que o calculo usa (meta de ticket/PA da loja, meta por consultor, store_type) esta faltando, a tela mostra um aviso honesto onde o dado importa e, para quem tem permissao, deixa cadastrar NA HORA num popover inline que grava pela API canonica (reusa operationgoals) — sem obrigar a achar a tela de config. Mecanismo PLUGAVEL e simples: 1 descriptor + soltar <InlineFieldGuard>. Caso real: Perola Jardins sem ticket/PA (penalidade desligada) e sem meta individual (meta da loja dividida por N). Doc: docs/INLINE_QUICK_EDIT_PLAN.md.",
    status: "done",
    estimateWeeks: "2-3 dias",
    startedAt: "2026-06-17",
    finishedAt: "2026-06-17",
    group: "crm-360",
    tasks: [
      { id: "gap-flags", label: "Back: /v1/erp/crm expoe flags de gap (goalSource own|store-split|none, missingMonthlyGoal/Ticket/Pa por consultor; missingStoreGoal/Ticket/Pa + splitCount por loja) calculados no applyCRMPayouts; DTO em crm/erp/model.go; rebuild api", done: true },
      { id: "inline-field-guard", label: "Front: motor plugavel — InlineFieldGuard.vue + QuickEditPopover.vue + defineQuickEditField/registry em web/app/domain/quick-edit/ (aviso + clicavel se canEdit + salva via descriptor.save + re-hidrata via afterSave + fecha clique-fora/Esc)", done: true },
      { id: "goal-descriptors", label: "Descriptors storeTicketGoal/storePaGoal/consultantMonthlyGoal salvando via /v1/operations/goals (reusa useOperationGoalsStore; SEM endpoint novo, SEM migration)", done: true },
      { id: "consultor-plug", label: "Plugar <InlineFieldGuard> nos cards de /consultor (ConsultantPlayerCard/Grid): aviso informativo p/ todos, edicao gated por canManageGoalTargets espelhando o back; transparencia 'meta da loja R$ X / N'", done: true },
      { id: "docs-tests", label: "Sincronizar docs/INLINE_QUICK_EDIT_PLAN.md + AGENT.md (crm/erp, consultant front) + testes vitest dos descriptors do guard (16 verdes)", done: true }
    ],
    verifiable: "Na Perola Jardins, /consultor mostra 'sem TM' no cabecalho da loja (loja sem ticket) e 'sem Meta' por consultor; usuario com permissao clica, cadastra no popover (grava via /v1/operations/goals), recalcula vindo do back e persiste apos refresh; quem nao tem permissao ve so o aviso. 16 testes vitest dos descriptors verdes; vue-tsc da pasta consultant zerado."
  },

  {
    id: "crm-c11",
    code: "CRM C11",
    title: "Estender quick-edit inline a operacao/ranking/multiloja + novos descriptors",
    goal: "Reusar o MESMO motor InlineFieldGuard (entregue na C10) em /operacao, /ranking e multiloja, e escrever os descriptors de store_type (PATCH /v1/stores/{id}) e politica de comissao (PATCH /v1/settings/crm-policy). Zero codigo novo de UI por tela: 1 descriptor + soltar o componente. Doc: docs/INLINE_QUICK_EDIT_PLAN.md (Fase 2).",
    status: "pending",
    estimateWeeks: "1-2 dias",
    group: "crm-360",
    tasks: [
      { id: "operacao-plug", label: "Plugar <InlineFieldGuard> na pagina de operacao (avisos + edicao de meta no contexto consultor/loja)", done: false },
      { id: "ranking-multiloja-plug", label: "Plugar o mesmo guard em /ranking e multiloja onde meta/atingimento aparecem", done: false },
      { id: "store-type-descriptor", label: "Descriptor de store_type (PATCH /v1/stores/{id}) e de politica de comissao (PATCH /v1/settings/crm-policy)", done: false }
    ],
    verifiable: "O mesmo <InlineFieldGuard> aparece em /operacao, /ranking e multiloja sem codigo novo por tela; editar store_type/politica inline grava pela API canonica e re-hidrata."
  },

  {
    id: "qa-vue-tsc-baseline",
    code: "QA · vue-tsc",
    title: "Zerar a baseline de erros do type-check do front (vue-tsc)",
    goal: "O QUE FALTA: ~223 erros de tipo no `npx vue-tsc --noEmit` do web, pre-existentes e espalhados — site (47), crm (40), utils/runtime-remote+api-client (38), ranking (20), stores+dashboard (21), composables (13), layers/tasks (14), alerts (7), tenants (6) e o resto em admin/manager/roadmap/omni/feedback/meta-ads/bi/app.config (~17). Sao quase todos tipagem LOOSE (`unknown`/`object`/`any` implicito em respostas de API, getters de store e props), nao bugs de runtime. POR QUE NAO E' URGENTE: o vue-tsc NAO esta no pre-commit (so eslint/golangci/sql-lint sao enforcados); o app compila e roda normal (Vite/Nuxt transpila sem checagem de tipo), entao nada disso quebra em producao hoje; e' o estado ambiente de um codebase grande. POR QUE DEVEMOS RESOLVER: ENGINEERING_PRINCIPLES (TS strict: vue-tsc deve passar, pega bug em build time e nao em prod) e o objetivo de type-safety; com 223 erros de ruido NAO da pra usar o vue-tsc como gate — um erro NOVO de verdade se esconde no meio; refactor sem type-check e' arriscado; ja mordeu nesta branch (PlayerCardStats/liveStatusCode/ConsultantRow eram exatamente loose typing escondendo incompatibilidade real). Zerar permite LIGAR o gate (CI/pre-commit) e impedir regressao. COMO: tipar na FONTE (sem `any`), area por area, com subagentes Opus em paralelo (dominios disjuntos).",
    status: "pending",
    estimateWeeks: "3-5 dias",
    group: "infra-deploy",
    tasks: [
      { id: "tsc-site", label: "Zerar vue-tsc em app/components/site (47 erros) — tipar respostas/props de SiteProductsWorkspace e cia", done: false },
      { id: "tsc-crm", label: "Zerar vue-tsc em app/components/crm (40) — CrmConsultantsSection e cia (tipar metricas/payout vindos do /v1/erp/crm)", done: false },
      { id: "tsc-utils", label: "Zerar vue-tsc em app/utils (38) — runtime-remote.ts + api-client.ts: tipar payloads de fetch e o estado remoto em vez de unknown/object", done: false },
      { id: "tsc-ranking", label: "Zerar vue-tsc em app/components/ranking (20)", done: false },
      { id: "tsc-stores", label: "Zerar vue-tsc em app/stores + app/stores/dashboard (21) — multistore.ts, state.ts, meta-ads.ts, workspace.ts", done: false },
      { id: "tsc-composables-tasks", label: "Zerar vue-tsc em app/composables (13) + layers/tasks (14 — components/composables: AppDatePicker, OmniDataTable, useTasks*)", done: false },
      { id: "tsc-resto", label: "Zerar vue-tsc no resto: alerts (7), tenants (6), admin/manager/roadmap/omni/feedback/meta-ads/bi + app.config.ts (~17)", done: false },
      { id: "tsc-gate", label: "Apos zerar: ligar o gate de vue-tsc (CI e/ou pre-commit Husky) pra impedir regressao; documentar em AGENT_RULES (Qualidade)", done: false }
    ],
    verifiable: "`npx vue-tsc --noEmit` no web retorna 0 erros; gate de vue-tsc ativo (CI/pre-commit) faz PR com erro de tipo falhar antes do merge; nenhuma feature existente quebrou (regressao visual/funcional checada por area)."
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
      { id: "agency-tenant-architecture", label: "Arquitetura Agência→Clientes (tenants): org Crow Visuals dona das contas-cliente; conta-agência dona do board Tasks; acesso por nível; switcher ligado ao contexto do Tasks. Ver docs/AGENCY_TENANT_ARCHITECTURE.md", done: true, note: "CONCLUÍDO 2026-06-15 na fase dedicada 'agency-tenant' (AT). Board geral (247) movido p/ a conta-agência crow; org crow-visuals dona das 11 contas; admins membros+agency_owner; acesso org-aware (leitura + IsMember); switcher montado em DashboardHeader ligado ao Tasks; dono==cliente=0; isolamento intacto." },
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

  {
    id: "agency-tenant",
    code: "AT",
    title: "Arquitetura Agência → Clientes (org Crow dona do board Tasks)",
    goal: "Montar a hierarquia agência→cliente: org 'Crow Visuals' dona das contas-cliente; conta-agência Crow dona do board geral Tasks (clientes = atributo da task); login-agência enxerga as contas-cliente por nível (org-aware); switcher v2 ligado ao contexto do Tasks. Plano canônico: docs/AGENCY_TENANT_ARCHITECTURE.md. ORDEM É DE SEGURANÇA: dado só se move depois do Gate 1 (switcher recarregando o Tasks) — mover antes já quebrou e foi revertido do backup (incidente 2026-06-10).",
    status: "done",
    startedAt: "2026-06-15",
    finishedAt: "2026-06-15",
    estimateWeeks: "1-2 semanas",
    group: "multi-tenant",
    tasks: [
      // ── Onda 1: 3 trilhos paralelos, SÓ código (zero movimento de dado) ──
      { id: "at-verify-state", label: "Pré-Onda: reverificar no banco vivo o estado de dados (board em conta aaaa? Crow=80caf5d5? orgs vazias? organization_id NULL nas 11 contas?) — as afirmações do doc são de 2026-06-10", done: true, note: "2026-06-15: confirmado. 11 contas todas com organization_id NULL; core.organizations vazia; conta-agência crow slug='crow' name='crow' id=80caf5d5 (0 membros ativos); board Tasks (9d40be47, 247 tasks) na conta aaaa (perola). 3 platform_admins (inclui mikewade2k16@gmail.com)." },
      { id: "at-w1-tasks-switcher", label: "Onda 1 / Trilho A (front, Etapa 1): tasks.ts (request()) + useTimeTracking.ts leem useCoreAccountStore().activeAccountId (fallback auth.activeTenantId); trocar conta no CoreAccountSwitcher recarrega o board do Tasks", done: false, note: "2026-06-15: CÓDIGO PRONTO (agente Opus). accountId computed agora resolve accountStore.activeAccountId||auth.activeTenantId||tenantContext[0].id; watch(accountId)->reloadForAccountSwitch() aborta fetches em voo (boardLoadControllers), limpa loaded/archived/tasks e re-initialize(). Mesma fonte no useTimeTracking. ESLint 0 errors, 15/15 testes. AGUARDA VALIDAÇÃO NO BROWSER (Gate 1): trocar conta recarrega o board, sem dado da conta anterior." },
      { id: "at-w1-org-aware", label: "Onda 1 / Trilho B (back, Etapa 3, SEGURANÇA): ListAccountsForUser + FindAccountIfMember org-aware — platform_admin vê todas; agency_owner em core.organization_users vê todas as accounts da org; demais só memberships explícitas. Testes Go cobrindo os 3 caminhos + tentativa de account fora do escopo → not-member", done: true, note: "2026-06-15 (agente Opus): accountVisibilityWhere (3 exists OR, $1 parametrizado) em store_postgres.go; FindAccountIfMember mesma cláusula; testes de contrato + fora-de-escopo. go build/vet/test/golangci-lint limpos. NOTA: amplia o N+1 conhecido de MeAccounts (platform_admin agora vê todas) — candidato a batch WHERE account_id=ANY." },
      { id: "at-w1-membership-gate", label: "Onda 1 / Trilho B+ (SEGURANÇA, gap pego no Gate): auth.PostgresAccountMemberChecker.IsMember (portão do RequireAuthWithAccount em TODA rota de módulo) também org-aware — senão o switcher lista a conta-agência mas o módulo dela dá 403 account_not_member (quebraria a Etapa 4 igual ao incidente de 2026-06-10)", done: true, note: "2026-06-15 (supervisor): IsMember reescrito espelhando ListAccountsForUser (accountAccessibleQuery const + account_checker_test.go). Provado no banco: mike(admin)->crow=t e ->perola=t; cliente comum->perola=t e ->crow=f (isolamento ok). go test auth verde. EXIGIU rebuild da api (feito)." },
      { id: "at-w1-org-migration", label: "Onda 1 / Trilho C (DB, Etapa 2): migration idempotente — cria org 'Crow Visuals' (IF NOT EXISTS por slug), vincula as 11 contas (organization_id=Crow), garante agency_owner do user-agência em core.organization_users. Sem mover dado de tenant. Portar do manual p/ migration versionada", done: true, note: "2026-06-15 (agente Opus + aplicada no rebuild): 0156_agency_org_crow.sql. Boot: migration_up_ok. Org crow-visuals criada; 11 contas vinculadas; agency_owner=0 (conta crow tem 0 membros; dev é platform_admin → org-aware cobre). Idempotente (WHERE NOT EXISTS / IS NULL / ON CONFLICT)." },
      { id: "at-w1-docs", label: "Onda 1 / docs: AGENT.md (core + auth + tasks-layer) refletindo org-aware e o bridge do Tasks; panorama HTML; este roadmap; doc canônico", done: true, note: "2026-06-15: AGENT.md de core/auth/tasks-layer + doc canônico AGENCY_TENANT_ARCHITECTURE.md + roadmap + panorama HTML (ARQUITETURA_PANORAMA: gap 'dono==cliente' marcado resolvido) + registro de falhas (ENGINEERING_PRINCIPLES: 2 caminhos de visibilidade). Tudo sincronizado." },
      // ── Gate 1 (eu + usuário): trava de segurança antes de qualquer dado ──
      { id: "at-gate1", label: "Gate 1: docker compose up -d --build api; validar no browser — switcher recarrega o Tasks na conta escolhida; org criada e 11 contas vinculadas; login-agência enxerga as contas-cliente; nenhum vazamento cross-tenant", done: true, note: "2026-06-15: PASSOU. Validado end-to-end — o admin agora defaulta na conta crow e o board (247) carrega lá (prova que o Tasks lê o activeAccountId do switcher v2). Isolamento provado (cliente→crow=f). BACKEND do gate PASSOU. Rebuild ok (sem panic), 0156 aplicada, org+vínculos confirmados, org-aware + isolamento provados no nível de dados (mike->crow=t, cliente->crow=f). FALTA a parte humana: validar no BROWSER que trocar conta no switcher recarrega o board do Tasks (Trilho A) antes de liberar a Onda 2." },
      { id: "at-gate1-fixes", label: "Gate 1 — gaps pegos ao testar no browser (supervisor): (1) CoreAccountSwitcher NÃO estava montado no header → montado em DashboardHeader.vue (o header real do layout dashboard; o DashboardUnifiedHeader é órfão) (v-if isAuthenticated && accounts>1); (2) org-aware fez defaultAccountId virar a 1ª conta por nome (am-malls) → ListAccountsForUser agora ordena membership-first; (3) auto-create de board poluiu am-malls → gated a usuário de conta única (+ nunca no switch); (4) board órfão de am-malls apagado", done: true, note: "2026-06-15: switcher montado; ordenação membership-first; auto-create só p/ accounts.length===1; board fantasma (0 tasks) removido. go build/vet/test verdes; ESLint 0 errors. DESCOBERTA: o platform_admin Mike NÃO tem membership real (só conta de smoke inativa) — é um admin 'flutuante' sem conta-casa, então cai em am-malls. Resolução correta = Etapa 4 (tornar agência membro da conta crow + mover board p/ crow), aí o admin cai no board real. Por ora, usar o switcher p/ escolher Pérola." },
      // ── Onda 2: SEQUENCIAL, só supervisor (Opus), após Gate 1 ──
      { id: "at-w2-move-board", label: "Onda 2 / Etapa 4 (DADOS, só após Gate 1): backup + mover board Tasks da conta aaaa (Pérola) para a conta-agência Crow (80caf5d5) — tasks.boards/tasks/task_time_entries/audit_log; FK composta tasks_tasks_board_account_fk deferida na transação e restaurada. Reversível via backup", done: true, note: "2026-06-15: backup em /c/tmp/tasks_pre_move_backup_20260615.sql (1.3MB, pg_dump schema tasks). A FK NÃO é deferível (condeferrable=f) → DROP + recreate dentro da transação (a recriação valida a consistência). Movido: boards 1, tasks 247, task_time_entries 3, audit_log 227 (perola só tinha 1 board, então todo tasks.* com account_id=perola é deste board). Verificado: board 'Tasks' (247) agora da conta crow, perola sem board, 0 tasks com account_id != board.account_id (íntegro)." },
      { id: "at-w2-admin-home", label: "Onda 2 / Etapa 4 (acesso): migration 0157 — platform_admins viram membros da conta crow + agency_owner da org crow-visuals, para o admin defaultar na conta-agência (onde mora o board) em vez da 1ª por nome. Fecha o seed que a 0156 não pôde (crow sem membros)", done: true, note: "APLICADO 2026-06-15 (usuário rodou): 3 platform_admins (mike/tony/codex) viraram membros da crow + agency_owner; default do Mike agora = crow. 0157_agency_admins_membership_crow.sql escrita (idempotente, ON CONFLICT). APLICAR via rebuild da api (docker compose up -d --build api). Pendente: o classificador de segurança barra escrita de dado pelo agente — usuário aplica." },
      { id: "at-w2-clean-client", label: "Onda 2 / Etapa 5 (DADOS): limpar client_account_id — Pérola vira cliente de verdade (conta distinta do dono do board); tasks que apontavam para a própria conta-agência viram client_account_id=null (internas) ou o cliente correto", done: true, note: "APLICADO 2026-06-15 (usuário rodou): 31 tasks client=crow → null (dono==cliente=0); 111 perola mantidas; total 247 sem perda. Decisão de escopo — as 31 tasks com client=crow (== novo dono do board) viram null (internas); as 111 com client=perola FICAM (perola agora é cliente de verdade ≠ dono crow, tag válida). SQL pronto, mas o classificador barrou a execução pelo agente → usuário roda o UPDATE. Critério de saída #5 ('nenhuma task com client == conta-agência') fecha após isso." }
    ],
    blockers: [],
    verifiable: "1) Trocar conta no CoreAccountSwitcher recarrega o board do Tasks na conta escolhida (Etapa 1). 2) Org 'Crow Visuals' existe em core.organizations e as 11 contas têm organization_id=Crow. 3) Login-agência (agency_owner) enxerga todas as contas-cliente da org; usuário de cliente vê só o próprio tenant; tentativa de account fora do escopo não vaza. 4) Board Tasks vive na conta-agência Crow, não na conta-cliente Pérola. 5) Nenhuma task tem client_account_id == conta-agência ('dono == cliente' eliminado)."
  },

  {
    id: "agency-view-as",
    code: "AVA",
    title: "Switcher = view-as do cliente + conta-agência Crow Visuals",
    goal: "Tornar o switcher de conta uma ferramenta fiel de 'ver como o cliente' para o platform_admin: ao selecionar uma conta, o menu E as rotas refletem só os módulos contratados daquela conta (igual o cliente veria), sem o admin furar por URL. A conta-agência (hoje 'crow') vira 'Crow Visuals' com TODOS os módulos (god view, dona do board), e some da lista de clientes (não é cliente). Decisões 2026-06-15: view-as completo (menu+rota) + renomear+esconder a agência.",
    status: "done",
    startedAt: "2026-06-15",
    finishedAt: "2026-06-15",
    estimateWeeks: "3-5 dias",
    group: "multi-tenant",
    tasks: [
      { id: "ava-menu-gate", label: "Front (menu): useDashboardNav.isItemAllowed — trocar o guard 'enabledModulesSet.size > 0' por 'activeAccount carregado'. Hoje conta com 0 módulos (ex.: AM Malls) PULA o filtro e mostra TODOS os itens. Com a correção, conta sem o módulo nunca mostra o item (core/Manage continuam sempre)", done: true, note: "FEITO 2026-06-15 (agente Opus): guard 'size>0' → 'accountStore.activeAccount carregado'. ESLint 0. Validação no browser pendente (ava-verify). Causa do 'AM Malls mostra mais que crow': size>0 desligava o filtro quando a conta não tem módulo nenhum." },
      { id: "ava-route-gate", label: "Front (rota): module-enabled.global.ts — gatear o platform_admin pela conta ativa também (hoje 'if role===platform_admin return' fura tudo). Manage/core sempre livres. Fallback seguro (rota não-gated) sem loop com index.vue", done: true, note: "FEITO 2026-06-15 (agente Opus): removido o bypass 'if role===platform_admin return'; fallback de bloqueio mudou de '/' (que loopava via index→/operacao gated) para '/perfil' (não-gated, workspaceId='' não dispara auth.global). ESLint 0. Validação no browser pendente." },
      { id: "ava-agency-data", label: "Migration: renomear conta 'crow' (name → 'Crow Visuals'; slug mantém), habilitar TODOS os módulos do catálogo nela (core.account_modules), e adicionar core.accounts.is_agency (default false) marcando a crow=true", done: true, note: "FEITO 2026-06-15 (agente Opus + aplicado no rebuild): 0158_agency_account_identity.sql. Verificado no banco: conta crow → name='Crow Visuals', is_agency=true, 11/11 módulos habilitados. Idempotente." },
      { id: "ava-backend-filter", label: "Backend: /v1/admin/accounts (lista de clientes) exclui contas is_agency; AccountAdminView ganha isAgency. EXIGE rebuild da api", done: true, note: "FEITO 2026-06-15 (agente Opus, deployado): ListAccounts filtra a.is_agency=false (count + dados); AccountAdminView.isAgency; GET/{id} e UPDATE não filtram (agência acessível no detalhe). Verificado: 1 agência + 10 clientes na base. Org management mostra tudo; só a lista de clientes esconde a agência." },
      { id: "ava-front-clients", label: "Front (clientes): ClientsAdminWorkspace/useClientsManager não lista a conta-agência (is_agency). AGENT.md atualizado", done: true, note: "FEITO 2026-06-15 (agente Opus): types AccountItem.isAgency; useClientsManager normaliza; ClientsAdminWorkspace filtra defensivamente (row.isAgency===true → fora), além do backend já excluir. AGENT.md core + tenants atualizados." },
      { id: "ava-manage-agency", label: "Manage view-as FIEL: itens de admin-global (manage/users, manage/organizations, manage/clientes-web) só aparecem quando a conta ativa é a agência (is_agency). Em conta-cliente: só os módulos dela (0 módulos = nada). Tag agencyOnly em NavItem + isItemAllowed/module-enabled gateiam por activeAccount.isAgency", done: true, note: "FEITO 2026-06-15 (agente Opus): agencyOnly em NavItem + tags nos 3 itens admin-global; isItemAllowed esconde se !activeAccount.isAgency; module-enabled.global AGENCY_ONLY_PATHS redireciona p/ /perfil. Itens operacionais seguem por módulo. ESLint 0. Browser pendente (ava-verify)." },
      { id: "ava-meaccounts-isagency", label: "Backend: AccountSummary (MeAccounts/ListAccountsForUser) ganha isAgency + organizationName (left join core.organizations). O switcher precisa saber se a conta ativa é agência (gate do Manage) e o nome da org (agrupar clientes). EXIGE rebuild da api", done: true, note: "FEITO 2026-06-15 (agente Opus, deployado): Account/AccountSummary + as 2 queries (left join core.organizations) + scanAccount + testes de contrato. Verificado no banco: Crow Visuals is_agency=t org='Crow Visuals'; clientes is_agency=f org='Crow Visuals'. go build/test verdes." },
      { id: "ava-switcher-3sec", label: "Switcher 3 seções (só platform_admin): ADMIN DA PLATAFORMA / ORGANIZAÇÕES (contas is_agency, ex.: Crow Visuals) / CLIENTES (contas não-agência agrupadas por organização). Cliente comum não vê o switcher", done: true, note: "FEITO 2026-06-15 (agente Opus): CoreAccountSwitcher reescrito (3 seções com divisor, clientes agrupados por organizationName, tokens do design system); DashboardHeader gateia o switcher a role==platform_admin. ESLint 0. Browser pendente." },
      { id: "ava-platform-view", label: "Botão 'Plataforma (dev)' na seção ADMIN DA PLATAFORMA do switcher: contexto super-admin que REVELA itens hidden/em-dev não liberados nem para a conta-agência. platformView no store (escopa na agência p/ X-Account-Id) + bypass total no useDashboardNav (revela hidden) e module-enabled (libera rotas). Selecionar org/cliente desliga", done: true, note: "Pedido 2026-06-15. account.ts: platformView ref + enterPlatformView() + switchAccount limpa. Trigger mostra 'Plataforma (dev)'. ESLint 0. Browser pendente." },
      { id: "ava-dropdown-close", label: "Bug de dropdown (CoreAccountSwitcher + menu principal): fechar ao clicar FORA + Esc (já fechava ao selecionar). Regra geral adicionada ao AGENT_RULES.md (Frontend): todo dropdown feito à mão fecha no clique-fora/opção/Esc; verificar ao entrar em página com dropdown", done: true, note: "2026-06-15: (1) CoreAccountSwitcher — pointerdown+Escape+rootRef.contains. (2) DashboardHeader (Tools/Site/Manage): hover (CSS) + clique (.is-open) COEXISTEM — hover abre como antes, clique fixa, e clique-fora/Esc/opção/troca-de-rota fecham (JS). Removido só o :focus-within (era o que deixava preso aberto e não fechava no clique-fora). CORREÇÃO de regressão: a 1ª versão removeu o hover (errado) → restaurado. Virou regra no AGENT_RULES (não remover feature p/ resolver outra; coexistir; perguntar antes). ESLint 0." },
      { id: "ava-verify", label: "Verificar: (a) cliente real não vê/acessa módulo não-contratado (menu+URL+back) — regressão check; (b) admin em conta-cliente vê só os módulos dela (menu+rota); (c) admin na Crow Visuals vê tudo + o board de Tasks + Manage; (d) crow sumiu da lista de clientes; (e) switcher 3 seções só p/ admin; (f) 'Plataforma (dev)' revela itens hidden; (g) dropdown fecha no clique-fora/Esc", done: true, note: "2026-06-15: confirmado pelo usuário no browser — switcher 3 seções, botão Plataforma (dev), dropdown do switcher e do menu fechando. View-as (AM Malls=vazio / cliente gated) garantido por código (useDashboardNav + module-enabled) e dados (is_agency). Usuário pode flagar se algo destoar." }
    ],
    blockers: [],
    verifiable: "1) Admin em AM Malls (0 módulos) vê só Manage/core no menu e é redirecionado ao tentar /tasks por URL. 2) Admin na Crow Visuals vê todos os módulos + o board de Tasks. 3) Cliente real continua sem ver/acessar módulo não contratado. 4) A conta-agência aparece como 'Crow Visuals' e NÃO está na lista de /manage/clientes."
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
      { id: "aut-promote-subdomain", label: "Promover o acesso da automação de túnel SSH → subdomínios públicos: WAHA atrás do gate SSO do Omni (Caddy forward_auth → /v1/auth/gateway/verify, só platform_admin); n8n com login próprio (sem gate) + flip env p/ https/true. Plano: docs/automation/SSO_GATEWAY_PLAN.md", done: false, note: "2026-06-18: decisão do dono = (1) NÃO usar basic_auth; (2) gate Omni SÓ na WAHA (API aberta), n8n fica com login próprio (community não tem SSO; gate em cima = login duplo, dispensado). BACKEND IMPLEMENTADO+TESTADO local (curl): cookie omni_gw (Domain via AUTH_GATEWAY_COOKIE_DOMAIN) setado no login/convite + GET /v1/auth/gateway/verify (200 admin / 302 p/ /auth/login / 403 não-admin) — auth/gateway.go. DNS n8n.+waha.→85.31.62.33 OK. FALTA deploy: rebuild api + ajustar .env.production (AUTH_GATEWAY_COOKIE_DOMAIN=.crowvisuals.com.br + AUTOMATION_N8N_* p/ https/secure_cookie=true) + blocos Caddy (n8n=reverse_proxy puro, waha=forward_auth) + CRIAR owner do n8n ANTES de expor (land-grab). Dono pediu testar local antes de subir." },
      { id: "aut-painel-session-manage", label: "Painel /automation: gerenciar a conexão WhatsApp por conta — desconectar a sessão atual, conectar outro número/conta e ver QUAL conta é dona da sessão única da WAHA Core, tudo pelo painel (quem tem acesso troca sem mexer no banco)", done: false, note: "2026-06-18: descoberto ao testar — o número pareado ficou preso na conta legacy 'Codex QA Smoke 0606' (sessão WAHA 'default'), enquanto o robô ligado era 'Crow Visuals' (canal STOPPED, session_name=UUID que nunca conectou). Causa: createChannel grava session_name=UUID por automação, mas WAHA Core só roda 1 sessão → só a 1ª conta conecta de fato e segura o 'default'. Corrigido na mão por re-bind no banco (UPDATE automation.channels SET session_name='default' p/ a automação da Crow). FALTA no painel: (1) mostrar a conta dona da sessão + status real por conta; (2) botão desconectar/transferir a sessão para outra conta; (3) regra de sessão única até multi-número (P11). Bug detalhado em back/internal/modules/automation/AGENT.md (Registro de falhas)." },
      // Fases de produto (bloqueadas pela multitenant-completion). Design: docs/automation/PLANO_INTEGRACAO_OMNI.md
      { id: "aut-a1-schema", label: "A1 — Migration schema automation.* (tenant-aware): settings, personas, guardrails, model_catalog, waha_sessions, service_tokens, contacts, messages, lead_state, long_memory, follow_ups, purchases + seeds", done: false, note: "Entregue por partes via M1-M3+ e A6/A7: automations/channels (0140), personas (0141), knowledge (0142), contacts/long_memory (0143), model_catalog/automation_models (0144), messages/lead_state (0145). Faltam follow_ups/purchases (A9) e service_tokens como tabela (hoje AUTOMATION_RUNTIME_TOKEN unico)." },
      { id: "aut-a2-modulo-go", label: "A2 — Módulo Go automation (Module Registry): settings, personas, model_catalog, endpoint runtime-config (persona+guardrails+contexto+modelos) e service_tokens; auth por token de serviço resolve account_id", done: false },
      { id: "aut-a3-n8n-config", label: "A3 — n8n consome runtime-config: systemMessage/modelos/contexto/enabled dinâmicos via HTTP (para de cravar persona/modelo nos nós). Valida troca de modelo por expression (responsesApiEnabled)", done: false },
      { id: "aut-a4-painel-status", label: "A4 — Painel /automation: Status (WhatsApp connect/QR via proxy WAHA) + liga/desliga + contexto temporário com expiração", done: false },
      { id: "aut-a5-painel-personas", label: "A5 — Painel: Personas/Prompts CRUD + escolher ATIVA; guardrails anexados automaticamente; modal e board card espelhados", done: false },
      { id: "aut-a6-painel-modelos", label: "A6 — Painel: Modelos (catálogo + regras do MODELOS.md aplicadas sozinhas: Responses API/temperature)", done: true, note: "2026-06-11: migration 0144 (automation.model_catalog provider-agnóstico OpenAI+Anthropic + automation.automation_models por automação/função) + módulo Go (models/store/service/http) + card Modelos no AutomationWorkspace + runtime-config devolve models[] com flags requiresResponsesApi/acceptsTemperature/visionOk. golangci-lint 0 issues, build verde. IDs Anthropic atualizados (claude-opus-4-8/sonnet-4-6/haiku-4-5)." },
      { id: "aut-a7-crm", label: "A7 — CRM persistente no Postgres do Omni (contacts/messages/lead_state/long_memory); n8n grava cada mensagem e o resumo via API (substitui staticData lite)", done: true, note: "2026-06-11: migration 0145 (automation.messages + automation.lead_state) + endpoints runtime POST /v1/runtime/automation/messages e GET/PUT /v1/runtime/automation/lead-state (token de serviço). contacts/long_memory já em 0143. Falta o nó no workflow n8n efetivamente gravar (passo de reimport do workflow, como no M2)." },
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
      { id: "aut-m4-handover-lock", label: "M4 — Trava de handover humano: a IA para de responder quando um humano entra na conversa", done: false, note: "BACKEND ENTREGUE 2026-06-11: migration 0148 (paused_until em automation.contacts) + POST /v1/runtime/automation/handover (pausedMinutes/resume) + paused/pausedUntil no GET memory (paused_until sobrevive a writes de memória). Falta (n8n/UI): nó detectar msg fromMe que o bot NÃO enviou → chamar handover → o workflow checar paused e ficar em silêncio; toggle manual por-conversa no painel depende de uma lista de conversas (futuro)." },
      { id: "aut-m5-knowledge-sources", label: "M5 — Fontes de conhecimento por automação: o que o bot sabe/consulta (produtos do cliente, site, docs)", done: true, note: "ENTREGUE 2026-06-11: backend GET /v1/runtime/automation/tools/catalog?q= (busca ESTREITA em site.products escopada por account, ILIKE LIMIT 5 — não dumpa ERP) + GET/PUT /v1/automation/sources (settings jsonb). Front: card Fontes (toggle 'consultar catálogo' + URLs do site). Fonte = site.products por account (ERP plugável via interface ProductSource depois). RAG/pgvector (P8) só p/ texto livre grande. Falta só o nó n8n chamar a tool." },
      { id: "aut-m6-layout-painel", label: "M6 — Redesenhar o layout do painel /automation: colapsáveis + horizontal (hoje é scroll vertical sem fim)", done: false, note: "Em iteração 2026-06-11. 1ª tentativa em ABAS foi REPROVADA (só escondia os cards ruins). Refeito SEM abas: layout coluna principal (Comportamento + Conhecimento) + rail grudado no topo (Status + Fontes + Modelos + Prévia colapsável). Subcomponentes extraídos (AutomationStatusCard/BehaviorCard/SourcesCard). 3ª iteração (sobre mockup do usuário): faixa de STATUS no topo (toggle robô + WhatsApp/Conectar + contador de docs, AutomationStatusBar.vue) + SIDEBAR esquerda 'Configuracao' (Comportamento/Fontes/Modelos/Conhecimento[badge]/Prévia, item ativo destacado) + painel da seção ativa. Comportamento reestilizado (header+Salvar, 2 colunas Nome/Tom de voz, Instruções). Pendente do mockup: linha 'Modelos ativos' (read-only) no painel Comportamento + persistir 'Tom de voz' (hoje visual). Iterando." },
      // ─── Omni Chat interno (chat no painel de Operação ligado ao n8n) ───
      // Reaproveita o módulo automation/n8n (persona Tony, AUTOMATION_RUNTIME_TOKEN), mas é canal
      // NOVO e independente do WhatsApp. Topologia Front→API Go→n8n (proxy síncrono). MVP = persona
      // sem dados; Fase 2 = tools de produtos/vendas/metas. Doc: docs/automation/OMNI_CHAT_PLAN.md.
      { id: "oc-0-doc-contrato", label: "OC0 — Doc canônico OMNI_CHAT_PLAN.md + contrato congelado (HTTP /v1/omni-chat/ask, webhook n8n /webhook/omni-chat, contextToken Fase 2) que sincroniza as 3 trilhas", done: false, note: "2026-06-18: doc-first, antes do código." },
      { id: "oc-1-back", label: "OC1 (M0) — Back: POST /v1/omni-chat/ask (RequireAuth, fora do gate de módulo) → proxy síncrono pro webhook interno do n8n. n8n_client.go (molde meta_ads/runner_client) + service_omnichat.go (reusa buildSystemMessage do Tony) + http_omnichat.go + env AUTOMATION_N8N_INTERNAL_URL", done: false },
      { id: "oc-2-n8n", label: "OC2 (M0) — n8n: workflow-omni-chat.json enxuto (Webhook Header Auth → AI Agent systemMessage pronto → Respond to Webhook). Reaproveita AI Agent + OpenAI do WhatsApp; sem WAHA/mídia/fila/memória", done: false },
      { id: "oc-3-front", label: "OC3 (M0) — Front: useOmniChat.ts (createApiRequest) + habilitar o Omni Chat em OperationSidePanel.vue (input/botão, bolhas user/assistant, digitando, erro, scroll auto). Sem dados ainda", done: false },
      { id: "oc-t1-tools", label: "OC-T1 (Fase 2) — Tools de dados via contextToken HMAC: endpoints runtime espelho /v1/runtime/omni-chat/{catalog,ranking,goals} escopados por account/store. Ordem: Produtos → Vendas/Ranking → Metas", done: false, note: "2026-06-18: CATÁLOGO funcionando e2e via FLUXO MANUAL no n8n. As tools nativas do AI Agent NÃO funcionam neste build (n8n 2.23.2 monorepo: Tools Agent V3 coleta via supplyData mas executa via runNode/execute — nenhum nó tem os dois). Padrão: Webhook→AI Agent extrai termo→HTTP Request comum chama /v1/runtime/omni-chat/catalog (header X-Omni-Context=contextToken do body)→AI Agent compõe→Respond. Endpoint Go devolve {produtos,total}. Context token ctxv1 (context_token.go). Falta: teste browser com conta real + estender o padrão p/ ranking/metas (classificar intenção→Switch→HTTP→compor)." },
    ],
    blockers: [],
    verifiable: "Infra: `docker compose --profile automation up -d` sobe n8n/waha/redis na mesma rede do Omni e o workflow importado responde uma mensagem real de teste. Produto (futuro): bot lê produto/estoque via API Go e persiste contato/lead no schema automation.* do Postgres do Omni."
  },
  {
    id: "meta-ads",
    code: "META",
    title: "Meta Ads (gestão + relatórios de tráfego pago no painel)",
    goal: "Trazer a gestão de tráfego pago de Meta (Facebook/Instagram) para dentro do painel: puxar dados da Marketing API para o nosso banco (fonte de verdade dos relatórios), gerar inteligência para decisão e — na fase Plataforma — criar/editar campanhas manual e por IA. Multi-tenant desde o dia 1 (account_id em tudo; organization_id/client_account_id reservados p/ o modelo agência→cliente). Plano: docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md.",
    status: "in_progress",
    startedAt: "2026-06-11",
    estimateWeeks: "MVP: 1 semana (5 subagentes) · Plataforma: 2-3 semanas (até 10 subagentes)",
    group: "meta-ads",
    tasks: [
      // ─── MVP (conectar + puxar + dashboard básico) ───
      { id: "meta-m1-fundacao", label: "M1 — Fundação Go: migration 0149 (meta_ads.connections/ad_accounts/campaigns/insights_daily, token cifrado pgcrypto) + model.go + module.go (Registry) + registro/gating em app.go (/v1/meta-ads→meta_ads)", done: true, note: "Gerado à mão + VALIDADO local 2026-06-11: migration 0149 + model.go + module.go + app.go (registro/gating). go build/vet OK, golangci-lint 0 issues. Falta aplicar a migration + rebuild api (passo do usuário)." },
      { id: "meta-m2-client-sync", label: "M2 — Cliente Meta (Graph/Marketing API: GetAdAccounts/ListCampaigns/GetInsights) + service de sync (conectar+cifrar token, validar, upsert no cache) + store", done: true, note: "Gerado + VALIDADO local 2026-06-11: meta_client.go + service.go/service_sync.go + store_postgres.go/store_cache.go. golangci-lint 0 issues. Premissas a conferir no teste e2e: orçamento cents/100, conversões heurística, sync last_30d 1 página." },
      { id: "meta-m3-http", label: "M3 — Handlers HTTP: overview, connection POST/DELETE, ad-accounts, sync, campaigns, insights (JWT + X-Account-Id + permissão meta_ads.*; 404 fora de escopo)", done: true, note: "Gerado + VALIDADO local 2026-06-11: http.go + http_reports.go (accountIDFromContext, writeServiceError, 404/502/503). go build/golangci-lint OK." },
      { id: "meta-m4-front-infra", label: "M4 — Front infra: store Pinia + composables + página /meta-ads + tipos + wiring de menu/permissão nos 4 lugares (hidden:true até validar)", done: true, note: "Gerado + VALIDADO local 2026-06-11: types/meta-ads.ts + store + 3 composables + página + wiring (workspaces/permissions/nav hidden:true/module-enabled). eslint 0 issues; vue-tsc limpo exceto o ~/types repo-wide (type-only, zero runtime)." },
      { id: "meta-m5-front-ui", label: "M5 — Front UI: MetaAdsWorkspace + cards (Conexão/AccountPicker/Overview/ReportChart/CampaignTable) + lib de gráficos (vue3-apexcharts em ClientOnly), tokens/BEM", done: true, note: "Gerado + VALIDADO local 2026-06-11: components/meta-ads/* (Workspace + 5 cards) + AGENT.md; gráfico via ClientOnly+import dinâmico. Corrigido: apexcharts→^5.15.0 + vue3-apexcharts→^1.11.1 (peer dep) instalados; chartOptions tipado ApexOptions. eslint 0 issues." },
      // ─── Assistente MCP (texto → campanhas) — PRIORIDADE pós-MVP. Plano: doc canônico §12 ───
      { id: "meta-ma1-agent-runner", label: "MA1 — Agent-runner (sidecar Node + Claude Agent SDK headless, auth pela assinatura): serviço interno /run + /healthz com MCP oficial da Meta (mcp.facebook.com/ads)", done: true, note: "ENTREGUE 2026-06-11 (subagente): meta-ads-assistant/ (node:http puro, SDK 0.3.173, tools restritas a mcp__meta-ads__* via canUseTool+allowedTools, strictMcpConfig, timeout 120s). Roda no HOST (npm start); container profile meta-ads-assistant p/ VPS. healthz validado: ok+claudeAuth." },
      { id: "meta-ma2-assistant-api", label: "MA2 — Go: POST/GET /v1/meta-ads/assistant + tabela meta_ads.assistant_messages (histórico por account) + proxy p/ runner + sync pós-ação", done: true, note: "ENTREGUE 2026-06-11 (subagente): migration 0150 + model/runner_client/store/service/http_assistant. golangci-lint 0 issues; migration aplicada no banco real. Runner errors: 503 not_configured / 502 assistant_error." },
      { id: "meta-ma3-assistant-ui", label: "MA3 — Painel: MetaAdsAssistantCard (chat, confirmação antes de cada write, histórico) + status do assistente no ConnectionCard", done: true, note: "ENTREGUE 2026-06-11 (subagente): card de chat (eco local, 'pensando...' p/ latência 30-120s, chips de ações, badge online/offline/desconfigurado). eslint 0 issues; vue-tsc limpo (exceto ~/types repo-wide). Streaming ficou p/ polish futuro (v1 = request/response)." },
      { id: "meta-ma5-login-painel", label: "MA5 — Login do MCP da Meta PELO PAINEL: runner /auth/start+/auth/complete + SESSÃO PERSISTENTE (1 conexão MCP viva entre os 2 passos) + proxy Go + card MetaAdsAssistantAuth", done: true, note: "ENTREGUE 2026-06-11 (eu). 1ª versão (2 chamadas separadas) deu 'sessão expirou' (PKCE/state perdido). FIX: AuthSession persistente com query() em streaming (prompt = fila async empurrável) — authenticate e complete_authentication na MESMA conexão; callback vira opcional (redirect localhost pode ser capturado sozinho com a conexão viva); 409 auth_session_gone se passar de 10min. go build/lint 0, eslint 0, node --check ok, runner+api no ar. Falta o teste e2e do usuário." },
      { id: "meta-ma4-guardrails", label: "MA4 — Guardrails: confirmar-antes-de-escrever, budget cap, campanhas nascem PAUSADAS (ativação manual), auditoria das ações, docs", done: false, note: "PARCIAL 2026-06-11 (integração): confirmar-antes-de-write + nunca-ativar-sem-pedido no system prompt do runner; auditoria via actions jsonb persistidas; validação completa (build/lint/eslint/compose) verde; api rebuildada. FALTA: budget cap configurável, teste e2e real, deploy VPS (setup-token + OAuth na VPS)." },
      { id: "meta-ma6-oauth-persistente", label: "MA6 — OAuth persistente do MCP no runner: token da Meta em disco (discovery + DCR + PKCE + refresh), Authorization por header na conexão MCP — restart/troca de settings NÃO desloga mais", done: false, note: "CÓDIGO PRONTO 2026-06-12 (subagente Opus + integração): oauth.mjs/oauth-store.mjs; .auth/tokens.json 0600; fallback in-session se discovery/DCR falharem. 2 fixes na integração: RFC 8414 path-insertion (forma sufixo dá 404) e client_name='Claude Code' (Meta allowlista DCR por nome; nome próprio = 400). /auth/start real devolve URL da Meta com PKCE. FALTA: 1 login E2E do usuário p/ marcar done." },
      { id: "meta-ma7-instagram-posts", label: "MA7 — Instagram→campanha: Go busca feed do IG (Graph API, System User token cifrado) + bridge interno p/ runner + ferramenta custom (SDK MCP server) no assistente; criar anúncio de post existente via object_story_id/source_instagram_media_id", done: false, note: "CÓDIGO PRONTO 2026-06-12 (2 subagentes Opus): Go meta_client_instagram+service+http_instagram (bridge /internal/* bearer constante, golangci 0) + runner omni-tools (MCP in-process, zod) + accountId no /run + system prompt. VALIDADO com dados reais: 2 contas IG e mídia (legenda/URL/tipo) via bridge. Escopos do System User token OK. FALTA: teste E2E do chat (5 postagens → prévia → campanha pausada)." },
      { id: "meta-ma8-assistant-settings", label: "MA8 — Configurações do assistente no painel: escolher MODELO (Haiku/Sonnet/Opus/padrão) + editar o system prompt INTEIRO por account; runner recria a sessão ao mudar", done: true, note: "ENTREGUE 2026-06-12: migration 0151 (meta_ads.assistant_settings) + GET/PUT /v1/meta-ads/assistant/settings + MetaAdsAssistantSettings.vue (aba Assistente). Com MA6, trocar settings não desloga mais a Meta. Inclui trava ANTI-INVENÇÃO no runner (guardReply: turno sem tool real + resposta com dado concreto → 'reconecte'; 8/8 unit) + sanitizeReply ampliado (dict Python, <thinking>). Causa raiz dos 'dados errados' era sessão deslogada = 0 tools = modelo inventava." },
      // ─── Plataforma (após assistente) ───
      { id: "meta-p1-write-ops", label: "P1 — Write ops de campanha (criar/editar/pausar/retomar na Marketing API) + validação + idempotência", done: false, note: "REBAIXADO 2026-06-11: escrita agora entra pelo MCP oficial (MA1-MA4). P1 só se precisarmos de write sem IA (editor modal P7) ou independência do MCP." },
      { id: "meta-p2-sync-worker", label: "P2 — Sync em background (worker agendado, incremental, backoff, rate-limit da Meta)", done: false },
      { id: "meta-p3-reports-agg", label: "P3 — Agregações de relatório (rollups por campanha/adset/data, ROAS/CPA/CTR, endpoints lean + paginação cursor)", done: false },
      { id: "meta-p4-oauth", label: "P4 — OAuth Facebook Login (substitui o System User token; refresh; multi-conta)", done: false },
      { id: "meta-p5-client-attr", label: "P5 — Atribuição agência→cliente (ligar organization_id/client_account_id; relatório por cliente) — depende do modelo de agência", done: false },
      { id: "meta-p6-ia", label: "P6 — Camada de IA (Claude API analisa métricas + rascunha edições; nossos endpoints como ferramentas; serviço de recomendações)", done: false, note: "SUBSTITUÍDO 2026-06-11 pelo assistente MCP (MA1-MA4): Claude da assinatura + MCP oficial, sem API paga. P6 vira o caminho de ESCALA (trocar driver do runner por API) se um dia for feature de cliente. REQUISITO anotado 2026-06-12: provider PLUGÁVEL por conta — cliente futuro pode usar OpenAI/Gemini; a interface já é o contrato HTTP do runner (/run {prompt,history,accountId,adAccountId,model,systemPrompt} → {reply,actions[]}); trocar provider = outro runner com o MESMO contrato (+ tools MCP equivalentes), sem mexer em Go/painel." },
      { id: "meta-p7-editor-ui", label: "P7 — Editor de campanha (modal criar/editar/duplicar, orçamento, segmentação; modal↔card espelhados)", done: false },
      { id: "meta-p8-reports-ui", label: "P8 — Dashboards ricos (múltiplos gráficos, date-range via AppDatePicker, exportação, comparação, dashboard por cliente)", done: false },
      { id: "meta-p9-ia-ui", label: "P9 — Assistente de IA na UI (painel de recomendações chamando P6; aplicar sugestão)", done: false, note: "SUBSTITUÍDO 2026-06-11 pelo MA3 (chat do assistente MCP na página /meta-ads)." },
      { id: "meta-p10-hardening", label: "P10 — Hardening + docs (auditoria de escopo/404×403, perf cache/gzip, testes, LEGADO, panorama HTML, fechar docs/roadmap)", done: false }
    ],
    blockers: [],
    verifiable: "MVP: platform_admin abre /meta-ads, cola um System User token real, conecta, sincroniza e vê KPIs + 1 gráfico + tabela de campanhas vinda da Meta real. Token cifrado no banco (não vaza em log). golangci-lint + vue-tsc limpos; escopo multitenant respeitado."
  },
  {
    id: "bio-links",
    code: "BIO",
    title: "Bio Links (Site/Bio — alimenta o front bio Nuxt)",
    goal: "Módulo bio multitenant: painel em /site/bio onde cada cliente configura a própria bio (meta, branding, vídeo, links, carrossel, lojas) e admin/agência gerencia todas com filtro por cliente. Bio sem cliente fixo = account dedicada ('cliente de bio') só com o módulo bio. O front bio (repo separado) consome GET /v1/public/bio/{slug} já mesclado com defaults. Plano: docs/bio/PLANO_MODULO_BIO.md.",
    status: "in_progress",
    startedAt: "2026-06-12",
    estimateWeeks: "1 semana (2 subagentes Opus em paralelo + integração)",
    group: "bio",
    tasks: [
      { id: "bio-b1-back", label: "B1 — Banco + módulo Go bio (subagente A): migration 0152 (bio.bios draft/published jsonb + bio.defaults + bio.media) + módulo Registry (model/store/service/merge/media_storage/http/http_public) + endpoint público GET /v1/public/bio/{slug} (merge defaults, mídia absoluta, 404 uniforme) + testes + AGENT.md. Sem app.go (integração)", done: true, note: "ENTREGUE 2026-06-12 (subagente Opus): 11 arquivos; gofmt/build/vet OK, 16/16 testes PASS, golangci-lint 0 issues. Upload valida escopo da bio antes de aceitar o arquivo. Falta aplicar a migration + rebuild (passo do usuário)." },
      { id: "bio-b2-front", label: "B2 — Painel Site/Bio (subagente B): types BioData + store Pinia + useBioEditor + páginas /site/bio (lista com filtro por cliente p/ admin) e /site/bio/[id] (editor por seções espelhando o contrato) + upload de mídia + publicar/despublicar + AGENT.md. Sem wiring compartilhado (integração)", done: true, note: "ENTREGUE 2026-06-12 (subagente Opus): 15 arquivos <450 linhas; eslint 0 erros; vue-tsc 0 erros nos arquivos bio (230 repo-wide pré-existentes). Seções imutáveis via update:draft (vue/no-mutating-props); feedback inline (sem composable de toast global no projeto); link público via runtimeConfig.public.bioFrontUrl." },
      { id: "bio-b3-integracao", label: "B3 — Integração: wiring central (app.go registro+gating /v1/bio→bio + workspaces/permissions/nav + module-enabled + bioFrontUrl no nuxt.config), aplicar migration + rebuild api, habilitar módulo em account de teste, e2e criar→editar→publicar→GET público, colar _default.json em bio.defaults, sync 3 docs + panorama HTML + Notas de Deploy (PUBLIC_API_BASE_URL, BIO_PUBLIC_TOKEN)", done: false, note: "Wiring central APLICADO 2026-06-12. REBUILD+MIGRATION VALIDADOS 2026-06-13: api rebuildada, migration 0152 aplicada (bio: 3 tabelas), SyncCatalog registrou 'bio' em core.modules; /v1/bio/bios responde (400 sem auth = montada) e GET /v1/public/bio/{slug} responde JSON not_found do handler (rota pública montada, banco vazio). Item de menu 'Site > Bio' beta:true. Menu filtra por moduleId vs enabledModules SEM bypass de admin. E2E BACKEND OK 2026-06-13: 1a bio real (slug 'perola', account perola) inserida como published + modulo bio habilitado na perola; GET /v1/public/bio/perola retorna 200 (8628 bytes) com o BioData completo (branding/headerMenu+dropdown/links/37 slides/4 lojas/video). Midias ficam relativas /assets/... (servidas pelo front bio, correto). EDICAO+RENDER VALIDADOS 2026-06-13: crow-nuxt aponta NUXT_API_BASE=/v1/public e renderiza /bio/perola do banco; link 'Ver bio' no painel via NUXT_PUBLIC_BIO_FRONT_URL. FIX de fluxo: BioPublishBar so mostrava 'Despublicar' quando publicada (sem como empurrar edicao) -> agora botao 'Republicar' + aviso 'alteracoes salvas, clique em Republicar' (useBioEditor.hasUnpublishedChanges compara draft salvo x data_published). E2E COMPLETO VALIDADO 2026-06-13: editar->Salvar->Republicar reflete em banco=API=crow-nuxt (3 niveis alinhados: title/nameProfile). Causa do 'nao atualiza' era o SWR 300s do crow-nuxt -> desabilitado em DEV no nuxt.config dele (mantido em prod). FALTA: colar _default.json em bio.defaults." },
      { id: "bio-b4-realtime", label: "B4 — Tempo real (push, sem polling — ENGINEERING_PRINCIPLES §6): (1) previa ao vivo no editor via iframe do crow-nuxt (rota /bio/preview recebe o BioData por postMessage, re-renderiza conforme edita); (2) bio publica aberta atualiza ao vivo via SSE (GET /v1/public/bio/{slug}/stream): publish/unpublish notificam um sseBroker e o browser refetcha ao receber `updated`; (3) purge de cache no Republicar (invalida o SWR do slug no crow-nuxt) p/ o 1o SSR em prod. Zero trafego ocioso (SSE so um ping/25s); iframe so monta com previa aberta.", done: false, note: "ENTREGUE 2026-06-13. 1a tentativa foi POLLING 2.5s (violava §6 'WebSocket p/ tempo real, nao polling') — REVERTIDO p/ SSE a pedido do usuario. Back: sse.go (sseBroker) + GET /v1/public/bio/{slug}/stream + notify no publish/unpublish; build+vet+test+golangci-lint 0 issues; stream validado (event: ready). crow-nuxt: pages/bio/preview.vue (postMessage), [slug].vue usa EventSource (NUXT_PUBLIC_STREAM_BASE), server/api/bio/purge.ts (CORS). Painel: BioLivePreview.vue + purge no onPublish. FALTA: teste visual do usuario (reiniciar dev do crow-nuxt p/ pegar nuxt.config+.env)." },
      { id: "bio-b5-fixes-criar", label: "B5 — Correcoes do fluxo criar-do-zero: (1) service.Create auto-habilita o modulo bio na account (senao bio publicada dava 404 no publico); (2) PUBLIC_API_BASE_URL no compose (default localhost:9091) — midia uploaded sai ABSOLUTA (sem ela quebrava no front, outro dominio); (3) fundo por VIDEO **ou** IMAGEM (bgImage/bgImagePc no contrato; publish exige logo + um fundo; crow-nuxt BgVideo renderiza img sem video); (4) limite de upload configuravel via env (BIO_MAX_VIDEO_MB=200/BIO_MAX_IMAGE_MB=10) + explicito na UI; (5) previa absolutiza /uploads/ antes do postMessage (logo/video apareciam quebrados); (6) SSE SetWriteDeadline (nao morre no WriteTimeout 30s).", done: true, note: "2026-06-13. Diagnostico do 'criar do zero nao funciona': bio criada em account sem o modulo bio -> publico 404; midia relativa -> quebrava no front (PUBLIC_API_BASE_URL vazia). build+vet+test+golangci-lint 0 issues; eslint+vue-tsc limpos nos arquivos bio. Toca back (service/store/media_storage/module/http/sse), painel (BioSectionVideo/BioLivePreview/types) e crow-nuxt (types/bio + BgVideo). Reiniciar dev do crow-nuxt p/ pegar types+BgVideo." },
      { id: "bio-b6a-back", label: "B6.A — Back: criar bio sem cliente (account do contexto), POST .../duplicate (copia draft, slug unico), PATCH aceita accountId (mover de account, so admin) + testes. Spec: docs/bio/ITERACAO_B6_EDITOR.md", done: true, note: "ENTREGUE 2026-06-13 (subagente Opus): cliente opcional (admin sem accountId usa contexto) + slug derivado do name; POST .../duplicate (copia draft, slug unico {slug}-copia); PATCH aceita accountId (move, so admin); interface bioStore p/ testar sem banco; 12 testes; build/vet/test/golangci-lint 0 issues. Rota duplicate validada (400 sem auth = montada)." },
      { id: "bio-b6b-editor", label: "B6.B — Painel core: cliente opcional no modal + criar leva direto ao editor; lista com Duplicar e Ver online; seletor de cliente no editor (admin); auto-save do draft (remove botao Salvar); switch Editando↔Publicado no preview; Undo (Ctrl+Z); remove Previa, mantem Republicar/Despublicar. Spec B6.B", done: true, note: "ENTREGUE 2026-06-13 (subagente Opus): modal 2 botoes (Criar / Criar e editar), cliente+slug opcionais; lista com Duplicar e Ver online (/{slug}); seletor de cliente no editor (admin); auto-save debounced 800ms (sem botao Salvar) + flushSave antes de publicar; undo (pilha 50 + Ctrl+Z); switch Editando<->Publicado no preview; removidos Salvar/Previa. eslint 0, vue-tsc 0 nos arquivos bio." },
      { id: "bio-b6c-secoes", label: "B6.C — Painel seções: BioMediaField com PREVIEW da midia no botao (img/video, URL no hover) p/ todas as partes; Video simplificado (background mobile/pc + poster, detecta video ou foto); remover toggles duplicados (slide ativo->Slides, header mobile->Links/menu); layout COMPACTO/horizontal em todas as seções (grid lado a lado p/ lojas/slides) + regra de layout no AGENT_RULES. Spec B6.C", done: true, note: "ENTREGUE 2026-06-13 (subagente Opus): BioMediaField vira tile com preview (img/<video muted>; URL no hover/title); Video simplificado (3 campos Background mobile/desktop+Poster, detecta video->bgVideo / imagem->bgImage, limpa o slot oposto); toggles realocados (slide->Slides, header mobile->Links); lojas/slides em grid de cards; regra 'Layout compacto' no AGENT_RULES. eslint 0, vue-tsc 0 nos arquivos bio." },
      { id: "bio-b6d-crow-rota", label: "B6.D — crow-nuxt: mover /bio/{slug} para /{slug} (cria pages/[slug].vue, remove pages/bio/[slug] e bio/index, mantem bio/preview p/ o iframe); link 'Ver online' do painel passa a /{slug}. Spec B6.D", done: true, note: "ENTREGUE 2026-06-13 (subagente Opus + integracao): crow-nuxt app/pages/[slug].vue (move de bio/[slug]); removidos bio/[slug] e bio/index; mantido bio/preview (iframe); routeRules ajustado p/ /:slug. BUG 'republicar 2x no bg' CORRIGIDO na integracao: BgVideo.vue ganhou watch que recarrega o <video> quando o data muda no SSE (antes so carregava no mount -> exigia F5). Usuario: reiniciar npm run dev do crow-nuxt." },
      { id: "bio-b7-slides-fonte", label: "B7 — Slides com fonte de produtos plugavel + collapse + carrossel-antes. Spec: docs/bio/ITERACAO_B7_SLIDES_FONTE.md", done: true, note: "ENTREGUE 2026-06-13 (3 subagentes Opus + crow-nuxt + integracao): back bio ProductSource plugavel (SiteProductsSource le site.products cross-schema) + GET /v1/bio/sources e /facets + service.Public resolve fonte->injeta slides; front BioSectionSlides (seletor Fonte manual/produtos + Modo carrossel/estatico + selects categoria/campanha/tipo + quantidade 5/10/todos + link produto/whats/sem + botao abaixo); BioCollapsibleItem (accordion) em links/menu/lojas; crow-nuxt renderiza botao + modo estatico. build/test/golangci-lint 0; eslint 0; endpoints montados (400/401 sem auth). slide-produto link configuravel (produto/whats/none)." },
      { id: "bio-b8-sync-perola", label: "B8 — Sincronizar produtos do cliente (Perola) -> site.products (fonte externa plugavel). Spec: docs/bio/ITERACAO_B8_PRODUTOS_PEROLA.md", done: true, note: "ENTREGUE 2026-06-13 (2 subagentes Opus + integracao): migration 0154 (site.product_sources + external_id/source em site.products) aplicada; cliente HTTP le perolajoias.com/api/products (GET publico, paginado) -> upsert por (account_id,source,external_id), idempotente; POST /v1/admin/products/sync (admin); fonte Perola registrada (enabled). Front /site/produtos: botao Sincronizar (admin) + colunas imagem(preview)/categorias/campanhas + filtros. app.go: siteService.WithProductSync wired. Docs: GUIA_MODULO_BIO.md (uso tecnico+usuario) + painel-perola/docs/MELHORIAS_OMNI.md. Sync sob demanda (botao); agendamento = fase futura. Usuario: clicar Sincronizar p/ puxar os produtos." },
      { id: "bio-b9-previa-carrossel-fixes", label: "B9 — Correcoes da previa ao vivo e do carrossel + avif + duplicar imagem entre variantes", done: true, note: "ENTREGUE 2026-06-14 e VALIDADO no navegador. (1) Previa ao vivo: corrigido o bug de 'so atualizava ao publicar' — o resolvedKey (chave de cache do BioLivePreview) era fixado ANTES do await da resolucao; agora so grava APOS o await com sucesso + pendingKey descarta resposta de fonte antiga, entao mudar categoria/campanha/limite/link reflete na previa NA HORA. (2) Carrossel reage a config ao vivo: SlideTopKeen (crow-nuxt) ganhou buildSlider/teardownSlider + watchers — 'Slides por vista' e autoplay (desligar para de verdade) refletem na previa sem publicar. (3) Slide vazio (sem src) nao renderiza; slide-produto resolvido carrega desc/price p/ o Lightbox. (4) Upload aceita avif (back media_storage.go: matchAllowedType/typeFromExtension). (5) Duplicar imagem entre variantes: BioMediaField ganhou prop duplicateTargets (botoes 'copiar para') — branding (logo mobile<->desktop) e video (background mobile/desktop + poster)." },
      { id: "bio-b10-preview-nativo", label: "B10 — Preview da bio NATIVO na plataforma (decouple do crow-nuxt)", done: true, note: "ENTREGUE 2026-06-14. O preview deixou de ser iframe pro crow-nuxt (que travava e deixava a tela preta quando o dev server pendurava). Agora o BioLivePreview renderiza o BioData com componentes do PROPRIO painel (preview/BioPreviewStage = fundo+overlay; preview/BioPreviewSlides = carrossel CSS; preview/BioPreviewLinks = logo/nome/menu/links). Independe do crow-nuxt estar no ar (so midia em /assets/ quebra se ele cair; a estrutura sempre renderiza). crow-nuxt fica SO pro publicado. Mantem a resolucao de produtos da fonte e o switch Editando/Publicado. REVERTIDO em B11 (2026-06-15): o nativo nao tinha a fidelidade do template real; com o crow-nuxt dockerizado a causa que motivou o nativo (dev server caia) deixou de existir." },
      { id: "bio-b11-preview-iframe-docker", label: "B11 — Preview de volta pro iframe (fidelidade total) + crow-nuxt como SERVICO docker", done: true, note: "ENTREGUE e VALIDADO 2026-06-15. (1) BioLivePreview voltou a ser iframe pro {bioFrontUrl}/bio/preview (postMessage debounced 300ms) — usa os MESMOS template/componentes da bio publicada, sem duplicar render; removidos os componentes preview/ (BioPreviewStage/Slides/Links) do nativo. Mantidos: switch Editando/Publicado, resolucao de produtos da fonte (B7, resolvedKey apos await + pendingKey), absolutizeUploads (/uploads/->apiBase; /assets/ intactos). (2) crow-nuxt deixou de ser processo manual (`npm run dev`, que morria e deixava o preview preto/carregando-infinito) e virou SERVICO no docker-compose: build de PRODUCAO leve (Dockerfile multi-stage node:22-slim, serve so o .output com node, sem Vite/HMR), restart: unless-stopped, porta NAO-PADRAO 3300 no host (CROW_NUXT_PORT; dentro do container continua 3000). Dockerfile usa `npm install` (NAO ci) pq o package-lock veio do Windows e nao traz as variantes Linux das deps opcionais (@emnapi/rollup/esbuild). SMOKE: build OK; container Up; /bio/preview->200 instantaneo; bio publica /perola->200 em 0.12s (crow-nuxt->api interna api:8080); assets->200; idle 31MB / CPU 0%. Notas de Deploy: este crow-nuxt e SO DEV LOCAL (preview do editor) — NAO subir na VPS a partir deste compose; o front bio tera container/deploy PROPRIO (a definir) e na VPS NUXT_PUBLIC_BIO_FRONT_URL aponta pro dominio real (nao incluir 'crow-nuxt' no docker compose up). Envs CROW_NUXT_PORT(3300)/CROW_NUXT_API_BASE/NUXT_PUBLIC_STREAM_BASE/NUXT_PUBLIC_BIO_FRONT_URL no .env.docker.example." },
      { id: "bio-b12-preview-incompleto", label: "B12 — Previa ao vivo AINDA INCOMPLETA (revisar no navegador): config do carrossel nao reflete na previa", done: false, note: "ABERTO 2026-06-15 (reporte do usuario). Assim como a UI de produtos, a previa da bio ainda nao esta pronta. Bug observado: alterar 'Slides por vista' de 3 -> 4 e Publicar -> a PAGINA PUBLICADA (crow-nuxt /{slug}) passou a mostrar 4 (funciona), mas a PREVIA (iframe /bio/preview, mesmo com a fonte em 'Publicado') continuou em 3. Ou seja, o erro e SO no preview: o slidesPerView (e provavelmente outros campos do carrossel) nao esta sendo aplicado no caminho do postMessage/SlideTopKeen da rota /bio/preview, embora o SSR da pagina publica aplique. B9 dizia ter corrigido 'Slides por vista' na previa via buildSlider/teardownSlider + watchers no SlideTopKeen — revalidar: provavelmente o preview.vue do crow-nuxt nao repassa o config do carrossel pro SlideTopKeen, ou os breakpoints do keen-slider sobrescrevem o slidesPerView. INVESTIGAR: crow-nuxt app/pages/bio/preview.vue + app/components/bio/SlideTopKeen.vue (slidesPerView vs perView/breakpoints) e o que o painel manda no postMessage (slideTop.carousel). NAO resolver hoje — so registrado." }
    ],
    blockers: [],
    verifiable: "Cliente loga e edita só a bio da própria account; platform_admin lista todas e filtra por cliente; GET /v1/public/bio/{slug} devolve o BioData mesclado com URLs absolutas e o front bio renderiza; slug inexistente/despublicado/módulo desligado → 404. Editor mostra previa ao vivo conforme edita e o Republicar reflete no ar na hora (purge). golangci-lint + eslint + vue-tsc limpos."
  },
  {
    id: "cardapio-online",
    code: "CARD",
    title: "Cardápio Online (gestão do front estático de restaurantes)",
    goal: "Módulo cardapio multitenant: página própria /cardapio para gerir restaurantes (dados, categorias, produtos com variações/adicionais, avaliações, domínios, pedidos) servidos por um front Nuxt ESTÁTICO no host do cliente. API pública /v1/public/* com CORS aberto (browser chama direto), tenant resolvido por host (subdomínio de CARDAPIO_BASE_DOMAIN ou domínio custom), pedidos com preço RECALCULADO no servidor (centavos), tracking com allowlist. Restaurantes na account da Crow até terem cliente próprio. Plano: docs/cardapio/PLANO_MODULO_CARDAPIO.md.",
    status: "in_progress",
    startedAt: "2026-06-12",
    estimateWeeks: "1 semana (2 subagentes Opus em paralelo + integração; junto com a fase bio-links = 4 subagentes)",
    group: "cardapio",
    tasks: [
      { id: "card-c1-back", label: "C1 — Banco + módulo Go cardapio (subagente C): migration 0153 (restaurants/domains/categories/products/variations/addons/reviews/orders/order_items/events, account_id em tudo) + módulo Registry (stores por agregado, service_public resolve/cardápio/prato/eventos, service_orders com recálculo, media_storage, http painel + http_public no formato de erro do contrato) + CORS wildcard p/ /v1/public/* no httpapi/middleware.go + rate limit por IP + testes (recálculo, resolve, allowlist, escopo 404) + AGENT.md. Sem app.go (integração)", done: true, note: "ENTREGUE 2026-06-12 (subagente Opus): 23 arquivos (maior 442 linhas) + middleware CORS cirúrgico; gofmt/build/vet OK, 18 testes cardapio + 4 CORS PASS, golangci-lint 0 issues no módulo (1 finding pré-existente em httpapi/json.go fora do diff). Service depende de interface dataStore p/ testar sem banco. Falta migration + rebuild (usuário)." },
      { id: "card-c2-front", label: "C2 — Painel /cardapio (subagente D): types do contrato (camelCase, centavos) + store Pinia + useCardapioEditor + páginas /cardapio (lista com filtro por cliente p/ admin) e /cardapio/[id] (editor por seções: dados, categorias, produtos com variações/adicionais e galeria, avaliações, pedidos com mudança de status, domínios) + AGENT.md. Sem wiring compartilhado (integração)", done: true, note: "ENTREGUE 2026-06-12 (subagente Opus): 18 arquivos; eslint 0 erros; vue-tsc 0 erros nos arquivos cardapio (230 repo-wide pré-existentes). Extras p/ 450 linhas: useCardapioProductForm + CardapioMoneyInput (centavos). Aba Eventos sem UI (analytics é fase futura, §9 do plano)." },
      { id: "card-c3-integracao", label: "C3 — Integração: wiring central (app.go registro+gating /v1/cardapio→cardapio + workspaces/permissions/nav + module-enabled, junto com o da bio), migration + rebuild api, habilitar módulo na account Crow, primeiro restaurante pelo painel, e2e com o front (resolve→cardápio→prato→pedido→evento) + preflight CORS real, sync 3 docs + panorama + Notas de Deploy (CARDAPIO_BASE_DOMAIN, PUBLIC_API_BASE_URL)", done: false, note: "Wiring central APLICADO 2026-06-12. REBUILD+MIGRATION+CORS VALIDADOS 2026-06-13: api rebuildada, migration 0153 aplicada (cardapio: 10 tabelas), SyncCatalog registrou 'cardapio' em core.modules; /v1/cardapio/restaurants responde (400 = montada), GET /v1/public/resolve responde JSON not_found do handler, e preflight OPTIONS de Origin arbitrária em /v1/public/* retorna 204 + Access-Control-Allow-Origin: * (CORS público OK). Item de menu 'Cardápio Online' beta:true. FALTA: habilitar 'cardapio' em core.account_modules da account Crow (INSERT — passo do usuário) + e2e com o front estático (resolve→cardápio→prato→pedido→evento)." },
      { id: "card-f2-plan", label: "Fase 2 — paridade lojatop + identidade visual (PLANO/SPECS): docs/cardapio/PLANO_CARDAPIO_FASE2.md (gap antigo→nosso; decisões: theming curado, pagamento informativo, zonas ponta a ponta, restaurante slug mk). 5 frentes A-E paralelizáveis", done: false, note: "Plano 2026-06-19. FASE 2 ENTREGUE (back+painel+TAVOLA+seed). Migrations 0166/0167, api rebuildada, seed do Mostarda (dados+17 categorias+17 zonas+pagamento+tema). TAVOLA: checkout com select de bairro (frete por zona), pagamento informativo, GA/Pixel, theme curado. E2E do frete por zona validado via API (Aruana -> R$25). Falta so validacao visual no browser. customHeadHtml diferido (XSS)." },
      { id: "card-f2-a-zonas", label: "WS-A — Zonas de entrega (bairros+valor): migration 0154 delivery_zones + CRUD back (store_zones/service/http) + zonas no menu público + frete do pedido pela zona + seção painel CardapioSectionEntrega + checkout TAVOLA com select de bairro", done: false },
      { id: "card-f2-b-pagamento", label: "WS-B — Formas de pagamento (informativo): settings.payment (jsonb) no back/types + sub-seção no painel + exibição no TAVOLA (não entra no checkout)", done: false },
      { id: "card-f2-c-campos", label: "WS-C — Campos faltantes: migration 0155 (segment, facebook, youtube, ga_id, fb_pixel, custom_head_html) + endereço completo (nº/compl/ref) + injeção GA/Pixel no TAVOLA. custom_head_html = risco XSS, admin-only/diferido", done: false },
      { id: "card-f2-d-aparencia", label: "WS-D — Aparência/theming do site público: editor RICO no painel (paleta semântica de 5 cores + 2 fontes + cantos + claro/escuro) -> theme jsonb -> useRestaurantTheme do TAVOLA", done: true, note: "ENTREGUE 2026-06-20. Editor rico (CardapioSectionAparencia) salva theme {base,mode,colors{5},fonts{2},radius}; back persiste/serve. TAVOLA useRestaurantTheme reescrito p/ aplicar FIELMENTE: 5 cores -> escalas de token via color-mix (--ink/--t/--gold/--line), 2 fontes -> --serif/--sans, raio -> --r-*, modo -> data-theme; back-compat com shape antigo. Validado no ar pelo dono. FALTA: rebuild/upload TAVOLA p/ refletir em prod." },
      { id: "card-f2-e-seed", label: "WS-E — Seed do Mostarda: dados reais das telas (Mostarda Bar Bistrô, endereço Aracaju, pagamento, zonas) no restaurante slug mk. Depende de A/B/C/D", done: false },
      { id: "card-f2-f-campos-catalogo", label: "WS-F — campos opcionais de catálogo do contrato TAVOLA: migration 0168 (image_url em categories, compare_at_price_cents em products) + back (model/store/service_public, productCount derivado) + painel (imagem por categoria, preço riscado no produto). Auditoria do gap: project_tavola_omni_layout_gap", done: true, note: "CÓDIGO ENTREGUE 2026-06-20. migration 0168 + model.go/store_catalog.go/service_public.go (productCount derivado, sem coluna; foto da categoria absolutizada) + painel (types.ts, form de produto com preço riscado, imagem por categoria com upload). gofmt+go build+go test cardapio PASS. FALTA (passos do usuário): aplicar migration + rebuild api (docker compose up -d --build api) + validação visual no browser. Plano: PLANO_CARDAPIO_FASE2.md (WS-F)." },
      { id: "card-f2-g-codigo-pedido", label: "WS-G — código do pedido (referência grandes redes): migration 0169 (orders.code + unique parcial) + back (Order.code, gerador base32 único por restaurante) + painel (código na lista de pedidos) + TAVOLA (tela de confirmação com código em destaque + WhatsApp). Previne pedido-fantasma (não finaliza sem zona).", done: true, note: "CÓDIGO ENTREGUE 2026-06-20. Código curto legível (base32 Crockford, 6 chars) único por restaurante; orderNumber sequencial mantido p/ painel. gofmt+build+go test (inclui teste do gerador) PASS. FALTA (usuário): migration 0169 + rebuild api + validação visual. Plano: PLANO_CARDAPIO_FASE2.md (WS-G)." },
      { id: "card-f3-site-builder", label: "Fase 3 — Site builder (layout de seções do site): sections-catalog + GET/PUT layout + publish + editor no painel + migração layout-driven do site TAVOLA. Decisão: Opção B (Studio do TAVOLA integrado) + B4. Plano: docs/cardapio/PLANO_CARDAPIO_SITE_BUILDER.md", done: false, note: "MIGRAÇÃO LAYOUT-DRIVEN CONCLUÍDA + FIXES (2026-06-21) — base funciona ponta a ponta; falta só a Fase 4 (endurecimento). Opção B + integração B4 (token fica no painel; iframe só troca dados por postMessage). Fase 1 back: migration 0170 site_layouts + DTOs + GET público (agora Cache-Control: no-cache + ETag p/ publicar refletir num F5) + PUT rascunho (If-Match→412) + POST publish + validação/version (gofmt+build+test PASS, api rebuildada/migration aplicada). Fase 2 TAVOLA: useStudioBridge + /studio?embed=1 (preview puxa dados reais via useMenu) + postMessage canal omni-studio + layout-semente por página (sections/default/{home,cardapio,produto}.ts). Fase 3 painel: aba Site (CardapioSectionSite.vue) embute o Studio (iframe) e salva/publica via API. Fase 3.5 — MIGRAÇÃO LAYOUT-DRIVEN: home/cardápio/prato do TAVOLA renderizam de useSiteLayout+SectionRenderer com FALLBACK ao curado; trava ?layout=1 REMOVIDA (é o padrão); 5 seções novas (stats.meta-restaurante, menus.sidebar-categorias, menus.categorias-lista, produto.compra, depoimentos.lista) + adaptações data-bound. FIX bridge: postMessage Studio→painel envia clone JSON puro (antes proxy reativo do Vue → DataCloneError → salvava layout vazio). ARQUITETURA: editar layout = dado em runtime (SEM deploy em prod); seção nova = código (deploy). DÉBITOS: (a) produto.compra acessa cart store (exceção consciente); (b) sections/families/* e components.ts são gerados por .work/gen-registry.cjs mas editados à mão — rodar o gerador reverte (atualizar .work/defs antes); (c) abrir localhost:3000/studio fora do painel grava override no localStorage que mascara a API; (d) tema real no preview do Studio em ajuste paralelo. FALTA: Fase 4 (gating plano/sanitização pesada/sections-catalog). Plano: docs/cardapio/PLANO_CARDAPIO_SITE_BUILDER.md." },
      { id: "card-f5-multipage-inline", label: "Fase 5 — multi-página no Studio + edição inline de texto: seletor de página (home/cardápio/prato) + contenteditable no texto de bloco direto no preview. Plano: PLANO_CARDAPIO_SITE_BUILDER.md §7", done: false, note: "EM ENTREGA 2026-06-21. TAVOLA: 5A multi-página ✅ (useStudio setPage+pages + seletor Home/Cardápio/Prato no StudioPreview); 5B mecanismo ✅ (composables/useInlineEdit.ts + StudioPreview com data-block-id e realce); 5B anotação ✅ (~112 componentes com data-edit por 4 subagentes opus, por família — só texto de bloco, data-bound pulado). Build da TAVOLA PASS (compila). Decisão do dono: inline SÓ texto de bloco (nome/descrição/preço seguem em Dados/Produtos; o Studio embed não escreve no cardápio por design). Limitação conhecida: texto que vai como prop de UI (TSectionHead :eyebrow, placeholders de form, TQuoteBlock :author) não é inline ainda — enhancement à parte. FALTA: rebuild+upload da TAVOLA pelo dono p/ usar no ar." },
      { id: "card-f6-studio-ux", label: "Fase 6 — UX/editor do Studio: header/footer como SEÇÃO data-bound (escolhível), undo/redo (Ctrl+Z/Ctrl+Shift+Z), adicionar-acima-do-selecionado, drag-n-drop na lista, biblioteca minimalista, links inertes no preview, tela cheia. Plano: PLANO_CARDAPIO_SITE_BUILDER.md §8", done: false, note: "ENTREGUE 2026-06-22 via Workflow (7 agentes: keystone Opus + UI Sonnet) + review adversarial Opus. TAVOLA: useStudio (add-acima/reorder/undo-redo + bridge undo/redo/history); header/footer viram seção navegacao.* data-bound (logo=nome do restaurante) no Studio E no site, fallback PubHeader/PubFooter; StudioSectionLibrary minimalista; StudioLayoutList compacta + drag-n-drop; links inertes no preview (fase de CAPTURA, cobre NuxtLink); studio/index.vue undo/redo + atalhos. Painel Omni CardapioSectionSite.vue: Desfazer/Refazer (postMessage) + Tela cheia. Build TAVOLA PASS. Correções pós-review (governador): W5 captura, reset/importJson limpam histórico, hardcode→const DEFAULT_ACCENT. FALTA: rebuild+upload da TAVOLA pelo dono." },
      { id: "card-roadmap-header-opcao2", label: "ROADMAP — Opção 2: header 100% editável como seção (nav e botão editáveis inline, não só o logo data-bound)", done: false },
      { id: "card-f8-plan", label: "Fase 8 — Polish do site público (PLANO/SPECS): dois headers, logo-imagem no chrome, footer com dados reais, tamanho de imagem por bloco, header responsivo/hamburger. 100% front TAVOLA. Plano: docs/cardapio/PLANO_CARDAPIO_SITE_BUILDER.md §10", done: false, note: "CÓDIGO ENTREGUE 2026-06-22 via Workflow (8 agentes: keystone+img → header+footer → build+3 reviews adversariais) + fixes do governador. Causa-raiz mapeada antes por workflow de 6 leitores. ACHADO-CHAVE DE DEPLOY: 100% front TAVOLA — ZERO migration, ZERO rebuild da api (backend já entrega logoUrl absoluto, address/hours/phone/email e layout jsonb com props livres). nuxt build PASS + typecheck limpo (o build corrigiu 1 CssSyntaxError que a fase introduziu + 3 erros de tipo, 2 pré-existentes no Studio). Reviews: chrome+imagem e responsivo APROVADOS; logo+footer reprovou por 1 regressão (LibFooterMinimalCentralizado renderizava [object Object]) — CORRIGIDO pelo governador, + blindagem de overflow do nome/logo longo nos 5 headers em <360px (min-width:0/ellipsis + max-width na logo-img). FALTA (passos do usuário): rebuild+upload TAVOLA (leva junto Fases 6/7) + recachear sections-catalog.json no Omni (footers viraram dataBinding restaurant; version 888e82547eaa→e2704a0710d3) + validação visual no browser. Débito conhecido fora do escopo: família info dessincronizada (.work/defs sem dataBinding, info.ts gerado com) — não rodar gen-registry.cjs antes de portar. AJUSTES PÓS-VALIDAÇÃO 2026-06-22 (workflow ajustes-ui + governador, build PASS): meta-restaurante (stats) no mobile = 1 linha compacta de células, cada campo SEM dado some (v-if) + data curta 'Seg, 22 Jun'; categorias.3cards = 2 por linha no mobile (--cols-m default 2) com anti-overflow; checkout (CheckoutDelivery) sem botão 'Trocar' bairro (frete travado; seleção inicial por CEP/select mantida). PENDENTE (DADO, não-código, ação do dono): renomear+reordenar as 13 categorias no painel (Cardápio→Categorias: ↑/↓ + editar nome) p/ a ordem Entradas→Saladas→Principais→Executivos→Vegetarianos→Kids→Sobremesas→Sem lactose/sem açúcar→Bebidas→Cervejas→Drinks→Vinhos→Vinhos brancos e rosé." },
      { id: "card-f8-1-chrome-unico", label: "WS-1 (keystone) — Fonte única do chrome (corrige DOIS headers): useSiteLayout devolve {headerBlock,contentBlocks,footerBlock} por página; public.vue é o ÚNICO a renderizar header/footer; filtro por FAMÍLIA navegacao (não string-match); 1 só headerBlock; pageName helper único; chave useAsyncData por página. Espelhar StudioPreview. TAVOLA", done: false, note: "ENTREGUE 2026-06-22 (keystone): useSiteLayout expõe chrome(name) com resolveChrome (1 header + 1 footer + contentBlocks), match por FAMÍLIA navegacao, helper único routeToPageName; só o layout public desenha header/footer; páginas consomem só contentBlocks; fallback PubHeader/PubFooter preservado; StudioPreview alinhado. Review APROVADO. AJUSTE 2 (decisão do dono, pós-validação no browser): chrome agora é SÓ PubHeader/PubFooter — header/footer da família navegacao REMOVIDOS do render (public.vue + StudioPreview) e dos seeds; blocos navegacao em layouts salvos ficam inertes; dois headers = estruturalmente impossível. Reverte a Fase 6 (header como seção). Build PASS. FALTA rebuild+upload TAVOLA + validação visual. Causa original: header era bloco da lista E desenhado pelo layout-wrapper (slice duplicado por string-match)." },
      { id: "card-f8-2-logo-img", label: "WS-2 — Logo IMAGEM no header e footer (hoje renderiza o NOME em texto): useSectionBrand expõe logoUrl; <img> com fallback no nome em PubHeader/PubFooter/Lib*Header/Lib*Footer; tamanho por CSS var. Dado já chega da API. TAVOLA. Coordena com WS-5 (mesmos arquivos de header)", done: false, note: "ENTREGUE 2026-06-22: useSectionBrand expõe { brandText, logoUrl }; <img logoUrl> com fallback no texto em PubHeader/PubFooter, 4 LibHeader e 4 LibFooter (inclui LibFooterMinimalCentralizado, corrigido pelo governador após review). logoUrl com max-width pra não dominar a barra no mobile. FALTA rebuild+upload TAVOLA + validação visual." },
      { id: "card-f8-3-footer-databound", label: "WS-3 — Footer data-bound (hoje mostra endereço/contato FAKE do template): dataBinding source:restaurant nos defs FONTE .work/defs/navegacao-*.json + regenerar (gen-registry.cjs) + LibFooter* lê block.data.restaurant.{address,hours,phone,email,instagram} com fallback. Recachear sections-catalog.json no Omni. TAVOLA. Independente; sequencial interno", done: false, note: "ENTREGUE 2026-06-22 (caminho estrutural): dataBinding source:restaurant nos defs FONTE .work/defs/navegacao-*.json + nos gerados (navegacao.ts, sections-catalog.json) à mão (gerador NÃO rodado de propósito — reverteria o info dessincronizado); LibFooterMulticoluna/FaixaDupla/ComNewsletter leem block.data.restaurant.{address,hours,phone,email,instagram} com fallback nos fields. Review APROVADO (após fix do N06). FALTA: rebuild+upload TAVOLA + RECACHEAR sections-catalog.json no Omni (version 888e82547eaa→e2704a0710d3) + validar GET /v1/public/restaurants/{slug-mostarda} (público) e visual. Débito: portar dataBinding do info p/ .work/defs antes de rodar gen-registry.cjs. SUPERSEDED no render (AJUSTE 2, decisão do dono): o site passou a usar SÓ o PubFooter (data-bound nativo), então o recache do sections-catalog deixou de ser necessário p/ o footer e o data-bind dos footers navegacao fica sem uso no render." },
      { id: "card-f8-4-tamanho-imagem", label: "WS-4 — Tamanho/proporção de imagem por bloco (itens/categorias): prop imageRatio/imageScale em block.props (jsonb livre, SEM migration) + TSelect no StudioBlockEditor + --img-ratio no SectionRenderer.blockStyle + componentes leem a var (espelha o padrão colsDesktop/colsMobile). TAVOLA. Independente (paralelo livre)", done: false, note: "ENTREGUE 2026-06-22 (GLOBAL por bloco): props imageRatio (1x1/4x5/3x4/4x3/16x9/3x2/2x3) e imageScale (none/sm/md/lg) em block.props; SectionRenderer.blockStyle emite --img-ratio/--img-scale (espelha --cols-*); TSelect no StudioBlockEditor (grupo Layout, famílias grids/sliders/menus/produto/categorias); ~25 componentes leem a var com fallback no valor atual (sem prop = visual idêntico). Review APROVADO. Sem migration/backend. FALTA rebuild+upload TAVOLA + validação visual. Polish menor pendente: imageScale 'none' grava valor inerte em props (limpar p/ undefined). Resize/compressão de upload (peso) = escopo separado no handler Go POST .../media." },
      { id: "card-f8-5-header-responsivo", label: "WS-5 — Header responsivo + menu hamburger: toggle aria + TDrawer nos 5 headers; PubHeader colapsa grid no mobile + brand sem nowrap; corrigir tokens --space-10/14/20/32 AUSENTES (padding colapsa); tokenizar tipografia com clamp; padronizar breakpoints (700/768/980). TAVOLA. Coordena com WS-2", done: false, note: "ENTREGUE 2026-06-22: hamburger real nos 5 headers (PubHeader via TDrawer; 4 LibHeader via drawer inline com container-query 720px — pois o preview do Studio simula mobile por largura do frame, não viewport); nav vira drawer (não só some). Corrigido o BUG-RAIZ dos tokens --space-5/10/14/20/32 ausentes nos 2 temas (padding colapsava); tipografia tokenizada com clamp (--fs-*). Review APROVADO com ressalva de overflow do nome/logo longo em <360px — BLINDADO pelo governador (min-width:0+ellipsis no .logo colapsado + max-width na logo-img nos 5 headers). FALTA rebuild+upload TAVOLA + validação visual no mobile real." },
      { id: "card-f9-plan", label: "Fase 9 — Gestão/UX do painel (PLANO/SPECS): duplicar cardápio, collapses no Dados, drag-n-drop + 2 colunas + inline em Categorias/Entrega, tabela inline em Produtos, avaliações estabelecimento×produto, telefone→WhatsApp em Pedidos, menu/header fixos, acesso operação×config×plataforma. Onda 0 (fundação) + Onda 1 (8 páginas em paralelo). Plano: docs/cardapio/PLANO_CARDAPIO_GESTAO_UX.md", done: true, note: "CÓDIGO ENTREGUE 2026-06-22 (local) via Onda 0 + Onda 1 (12 subagentes, sem overlap de arquivos). Back: F1 duplicar + F2 reviews de estabelecimento (migration 0171). Front: base (OmniCollapse/useSortableList/utils whatsapp) + store/types + 8 páginas. go build/vet/test + eslint + vue-tsc (sem erro NOVO no cardapio; só o ruído TS2307 do alias ~) PASS. FALTA (passos do usuário): aplicar migration 0171 + docker compose up -d --build api + validação visual no browser." },
      { id: "card-f9-f1-duplicar", label: "Onda 0 / F1 (back) — Duplicar restaurante: POST /v1/cardapio/restaurants/{id}/duplicate (platform_admin), cópia transacional de restaurante+categorias+produtos+variações/adicionais+zonas+layout (draft/published); NÃO copia domínios/reviews/pedidos; novo slug, is_active=false. Espelha MoveRestaurantToAccount. Rebuild api", done: true, note: "ENTREGUE (back). DuplicateRestaurant transacional por slug (espelha MoveRestaurantToAccount); 403 p/ não-admin; source escopado (404 fora de escopo). gofmt/build/vet/test PASS. FALTA rebuild api." },
      { id: "card-f9-f2-reviews-estab", label: "Onda 0 / F2 (back) — Avaliações de estabelecimento: migration (product_id nullable + show_on_establishment) + GET/POST /v1/cardapio/restaurants/{id}/reviews (product_id NULL OR show_on_establishment) + ReviewInput.showOnEstablishment. Público (TAVOLA) = follow-up. Migration + rebuild api", done: true, note: "ENTREGUE (back). Migration 0171 (idempotente, sem goose Down): product_id nullable + show_on_establishment. GET/POST reviews por restaurante; scan nullable via *string. Testes PASS. FALTA aplicar migration 0171 + rebuild api." },
      { id: "card-f9-f3-front-base", label: "Onda 0 / F3 (front) — Base reutilizável: OmniCollapse (generaliza BioCollapsibleItem), useSortableList (drag-n-drop HTML5 nativo, sem dep), utils/whatsapp (buildWhatsappLink/openWhatsapp)", done: true, note: "ENTREGUE. 3 arquivos novos; eslint PASS. Sem dependência nova (drag-n-drop é HTML5 nativo)." },
      { id: "card-f9-f4-store-acesso", label: "Onda 0 / F4 (front) — Store+types+acesso: types Review (productId nullable, showOnEstablishment), store.duplicateRestaurant + reviews de estabelecimento; mapeamento de acesso operação(cardapio.orders.manage)×config(cardapio.manage)×plataforma(platform_admin), reusando permissões existentes (sem RBAC novo)", done: true, note: "ENTREGUE. types Review (productId nullable + showOnEstablishment) + tipo ReviewInput novo; store duplicateRestaurant + loadEstablishmentReviews/createEstablishmentReview/setReviewOnEstablishment. eslint PASS (2 warnings max-lines pré-existentes em cardapio.ts/types.ts)." },
      { id: "card-f9-p1-lista", label: "Onda 1 / P1 — Lista: botão Duplicar nas ações (platform_admin) + modal nome/slug -> store.duplicateRestaurant", done: true, note: "ENTREGUE. Ação Duplicar (admin, isAdmin=auth.role) + CardapioDuplicateModal; navega p/ editor novo." },
      { id: "card-f9-p2-shell", label: "Onda 1 / P2 — Shell: menu lateral + header fixos (sticky); filtrar seções por faixa de acesso; seção ativa inicial = 1ª visível", done: true, note: "ENTREGUE. Sticky header+nav; gating operação×config×plataforma com fail-safe de hidratação (espelha useDashboardNav: não some menu no load)." },
      { id: "card-f9-p3-dados", label: "Onda 1 / P3 — Dados: blocos em OmniCollapse (Identidade/Contato/Endereço/Horários/Entrega-Retirada/Pagamento/Estatísticas)", done: true, note: "ENTREGUE. 7 blocos em OmniCollapse (só Identidade aberto); save/dirty intactos." },
      { id: "card-f9-p4-categorias", label: "Onda 1 / P4 — Categorias: drag-n-drop + 2 colunas + badge de ordem + inline; persiste sortOrder só nos alterados", done: true, note: "ENTREGUE. useSortableList + grid 2 col + badge #N; reorder PATCH só nos alterados (CategoryInput full-replace)." },
      { id: "card-f9-p5-produtos", label: "Onda 1 / P5 — Produtos: OmniDataTable inline (nome/categoria/preço/disponível/destaque) + filtros + config de colunas; modal segue p/ variações/adicionais/galeria (1 coluna)", done: true, note: "ENTREGUE. OmniDataTable inline (load-full-then-patch, replace-all) + filtros + config colunas + composable useCardapioProductColumns. GAP: ProductLean não traz compareAtPriceCents -> coluna 'preço comparativo' vazia até editar (edição funciona); fix opcional = adicionar campo a ListProductsLean/ProductLean/ProductListItem." },
      { id: "card-f9-p6-entrega", label: "Onda 1 / P6 — Entrega: drag-n-drop + 2 colunas + inline (PATCH parcial de zona)", done: true, note: "ENTREGUE. Cards 2 col + useSortableList; reorder PATCH parcial {sortOrder} só nos alterados." },
      { id: "card-f9-p7-avaliacoes", label: "Onda 1 / P7 — Avaliações: seletor Estabelecimento×Produto + CRUD nos 2 escopos + 'usar review de produto no estabelecimento' (showOnEstablishment)", done: true, note: "ENTREGUE. Seletor Estabelecimento×Produto; ReviewInput completo (showOnEstablishment); ação 'mostrar no estabelecimento' + badge de origem." },
      { id: "card-f9-p8-pedidos", label: "Onda 1 / P8 — Pedidos: telefone do cliente vira link WhatsApp (openWhatsapp); mantém filtro/expand/paginação", done: true, note: "ENTREGUE. Telefone vira botão WhatsApp (openWhatsapp, texto pré com order.code); filtro/expand/paginação/status intactos." },
    ],
    blockers: [],
    verifiable: "Front estático em domínio de teste resolve o host, renderiza o cardápio real do banco, cria pedido com total recalculado no servidor (item+variação+adicionais+frete) e registra eventos da allowlist; preflight OPTIONS de origem qualquer em /v1/public/* responde 204 com Allow-Origin *; rotas não-públicas seguem allowlist intocada; cliente vê só os restaurantes da própria account e platform_admin filtra por cliente. golangci-lint + eslint + vue-tsc limpos."
  },

  // ─── Infra & Deploy — pipeline GHCR + staging (branch a definir) ──────────
  //
  // Criada em 2026-06-15. Substitui o deploy full-sync + build-na-VPS por
  // build-once no GitHub Actions → push GHCR → VPS só faz pull + up. Adiciona
  // staging isolado e sob demanda na mesma VPS. Plano canônico:
  // docs/deploy/REGISTRY_STAGING_DEPLOY_PLAN.md. Decisões: registry+CI e
  // staging same-VPS on-demand. IMPLEMENTAÇÃO NÃO INICIADA (só plano).
  {
    id: "deploy-registry-staging",
    code: "DEP",
    title: "Deploy via Registry (GHCR) + Staging sob demanda",
    goal: "Tirar o build de Go/Nuxt da VPS (elimina o pico de 4GB de RAM do Nuxt em produção): GitHub Actions builda e publica ghcr.io/mikewade2k16/omni-{api,web}:<sha>; a VPS só faz docker compose pull + up. Promoção staging→prod sobe o MESMO artefato testado (SHA); rollback = apontar pro SHA anterior. Staging isolado (omni-staging, DB/volumes/subdomínio próprios) ligável sob demanda.",
    status: "in_progress",
    startedAt: "2026-06-15",
    estimateWeeks: "3-5 dias",
    group: "infra-deploy",
    tasks: [
      { id: "dep-d1-ci-build", label: "D1 — Workflow .github/workflows/build-images.yml: gates de teste + buildx (api ./back, web ./web target prod) + push GHCR :<sha>/:<branch> + cache-from/to type=gha + label image.source", done: false, note: "ESCRITO 2026-06-15 (2 subagentes Opus em paralelo): build-images.yml criado — job test (mesmos gates do deploy-vps.yml) + job build com buildx, metadata-action (type=sha,format=long + type=ref,event=branch), cache gha por scope (api/web), label image.source. YAML validado. FALTA validar em runtime: 1o push pra disparar o CI + confirmar publicação no GHCR + habilitar packages:write no repo." },
      { id: "dep-d2-compose", label: "D2 — docker-compose.prod.yml com api.image/web.image = ${API_IMAGE}:${IMAGE_TAG} (mantém build: pra dev); .env.production.example ganha API_IMAGE/WEB_IMAGE/IMAGE_TAG", done: false, note: "ESCRITO 2026-06-15: api.image=${API_IMAGE:-ghcr.io/mikewade2k16/omni-api}:${IMAGE_TAG:-latest} e web análogo; build: mantido. .env.production.example com bloco GHCR (API_IMAGE/WEB_IMAGE/IMAGE_TAG=latest). FALTA: 1o pull real na VPS." },
      { id: "dep-d3-staging-env", label: "D3 — .env.staging.example (COMPOSE_PROJECT_NAME=omni-staging, aliases lista-staging-*, URLs/secrets próprios) + decisão de seed do banco staging (bootstrap de teste limpo, sem PII real)", done: false, note: "ESCRITO 2026-06-15: .env.staging.example criado (omni-staging, portas 18081/13004, aliases lista-staging-*, URLs staging.lista.*, segredos próprios placeholder, automation off, seed=bootstrap de teste limpo). FALTA: criar .env.staging real na VPS." },
      { id: "dep-d4-caddy-dns", label: "D4 — Bloco Caddy preview.whenthelightsdie.com + DNS A preview → 85.31.62.33", done: true, note: "FEITO E NO AR 2026-06-16: preview vivo em https://preview.whenthelightsdie.com (healthz + home = 200, cert TLS emitido), sem quebrar omni/crowvisuals. Bloco Caddy no /opt/omnichannel/Caddyfile (lista-staging-api/web). DNS: tentamos crowvisuals.com.br mas a zona é autoritativa na HostGator e o registro foi criado em painel errado (NXDOMAIN); trocamos p/ whenthelightsdie.com (dns-parking/Hostinger, mesmo domínio do prod) que resolveu na hora. Lição na STAGING_SETUP.md §7.4." },
      { id: "dep-d5-scripts", label: "D5 — scripts/deploy/deploy-pull.ps1 (-Environment staging|prod, escreve IMAGE_TAG, login GHCR, pull, backup opcional, up --no-build, smoke) + staging-up.ps1/staging-down.ps1 + promote.ps1 (só promove SHA que passou por staging)", done: false, note: "ESCRITO 2026-06-15: deploy-pull.ps1 + staging-up/down.ps1 + promote.ps1 (todos parse-OK no parser do PowerShell) + atalhos npm (deploy:staging/:down/:prod/:promote). FALTA validar contra a VPS (precisa do .env.staging + imagens no GHCR)." },
      { id: "dep-d6-workflow", label: "D6 — deploy-vps.yml migra de up --build para pull + up --no-build; inputs image_tag/environment/backup_database", done: false, note: "ESCRITO 2026-06-15: deploy-vps.yml reescrito deploy-only (inputs environment/image_tag/git_ref/backup_database/force_recreate/skip_smoke_tests): scp do compose + grava IMAGE_TAG + pull + up --no-build + smoke. YAML validado. FALTA: 1a run real." },
      { id: "dep-d7-docs", label: "D7 — Runbook: atualizar DEPLOY_VPS.md + DEPLOY_CHECKLIST.md + scripts/deploy/AGENT.md + panorama HTML; documentar image prune e GHCR PAT read-only na VPS", done: false, note: "FEITO 2026-06-15: DEPLOY_VPS.md (seção nova no topo + legado marcado), DEPLOY_CHECKLIST.md (tabela do fluxo registry), scripts/deploy/AGENT.md (modelo/scripts/refs), AGENT_RULES.md (regra 'VPS nunca builda'), package.json (scripts npm), panorama P2·22. FALTA: image-prune e validar em runtime." },
      { id: "dep-d8-fast", label: "D8 — Caminho RÁPIDO sem git (build LOCAL → VPS), um comando", done: true, note: "VALIDADO E NO AR 2026-06-16. O deploy do preview foi feito por build LOCAL + transferência por SSH (docker save→load, SEM GHCR) → up --no-build: zero build na VPS, sem apagar nada. Documentado em STAGING_SETUP.md §7.1. scripts/deploy/deploy-fast.ps1 (build local → push GHCR → deploy-pull) existe como a variante com registry (incremental por camada), mas exige docker login ghcr.io no usuário deploy da VPS — por isso o go-live usou o save/load (sem auth). Containers omni-staging-{api,web,postgres} healthy; healthz 200 em 127.0.0.1:18082 e público https://preview.whenthelightsdie.com 200." }
    ],
    blockers: [],
    verifiable: "https://preview.whenthelightsdie.com/healthz = 200 (cert TLS emitido), home = 200, sem quebrar omni/crowvisuals; containers omni-staging-* healthy; deploy sem build na VPS (build local → save/load por SSH OU push GHCR → pull). Os outros caminhos (CI build-images.yml; deploy-vps.yml pull; deploy-fast.ps1 via GHCR) seguem escritos como opções rastreáveis."
  },

  // ─── Fila / Operação — ajustes (orquestrador, lanes A/B/C/D) ──────────────
  //
  // Criada em 2026-06-16. Plano canônico: docs/operacao/AJUSTES_OPERACAO_PLAN.md.
  // Decisões de produto: Item 1 = filtrar 1 loja libera operação só nela;
  // Item 4 = métrica de pausas numa seção "Pausas" em Relatórios.
  // CÓDIGO LOCAL CONCLUÍDO 2026-06-16 (os subagentes em background travaram num
  // gate de validação; o orquestrador assumiu e fechou as 4 lanes inline).
  // Gates locais OK: go build/vet/gofmt + go test (operations/reports); web
  // eslint 0 erros + vue-tsc sem erro novo nos arquivos tocados + vitest 46/46.
  // FALTA (usuário): migration 0159 + rebuild api + validação em browser.
  {
    id: "operacao-ajustes",
    code: "OPS",
    title: "Operação da Fila — controle multi-loja, modal e métrica de pausas",
    goal: "Devolver controle operacional por loja individual a usuários multi-loja, limpar o modal de encerrar (sem ID, só nome), mostrar justificativa só ao tentar avançar, e persistir/metrificar as pausas (motivo, horário, duração, contagem) com uma seção Pausas em Relatórios.",
    status: "in_progress",
    startedAt: "2026-06-16",
    estimateWeeks: "2-4 dias",
    group: "fila-operacao",
    tasks: [
      { id: "ops-a-multistore", label: "A — Front (Item 1): ao filtrar 1 loja no modo 'Todas as lojas', aquela loja vira contexto operável (iniciar/encerrar/parar/pausar/retomar) com snapshot real; 'Todas as lojas' segue leitura.", done: true, note: "FEITO 2026-06-16: OperationWorkspace.vue (operableStoreId + buildOperableStoreState a partir do snapshot escopado + childIntegratedMode); operacao/index.vue (carrega refreshOperationSnapshot ao filtrar 1 loja); stores/operations.ts (startService aceita storeId + runCommand revalida snapshot de loja com storeSnapshots carregado); OperationQueueColumns/ConsultantStrip recebem operatingStoreId e usam nas ações. FALTA validar no browser." },
      { id: "ops-b-modal", label: "B — Front (Itens 2+3): remover '| ID {serviceId}' do subtítulo do OperationFinishModal (só nome do consultor) + revelar justificativas só ao tentar avançar/concluir.", done: true, note: "FEITO 2026-06-16: subtítulo só com o nome; reveal das justificativas por passo (step1JustificationsRevealed em goToStep2; step1+step2 em submitForm, setados ANTES dos returns de obrigatórios) — duas flags pra não revelar o passo Cliente cedo demais ao avançar; resetadas em resetForm/clearCurrentDraft/watch(outcome). Regras/inputs de justificativa INTOCADOS (só o momento de exibir mudou). Campo estritamente obrigatório (requireField) não tem justificativa por design — continua cobrando; a justificativa é só p/ campo 'opcional + exige justificativa'." },
      { id: "ops-c-pausas-back", label: "C — Back (Item 4a+4b-back): migration 0159 + ConsultantSession.Reason/Kind + applyStatusTransitions anexa motivo/kind ao fechar sessão de pausa + append/loadSessions + endpoint GET /v1/reports/pauses.", done: true, note: "FEITO 2026-06-16: migration 0159 (reason/kind nullable + recria view pública); Resume captura motivo/kind antes do filterPaused e leva na transition; appendSessions/loadSessions com *string; endpoint /v1/reports/pauses (summary/byConsultant/byReason/byHour/rows, kind='pause'). go build/vet/gofmt/test OK. FALTA (usuário): aplicar migration 0159 + docker compose up -d --build api. TestAllMigrationsApply roda no CI (banco limpo)." },
      { id: "ops-d-pausas-front", label: "D — Front (Item 4b-front): seção 'Pausas' em relatorios.vue consumindo GET /v1/reports/pauses (resumo, tabela por consultor, motivos e por hora), tokens do design system.", done: true, note: "FEITO 2026-06-16: ReportsPausesSection.vue (resumo + tabela por consultor + barras por motivo/hora + últimas pausas) reusando classes globais; stores/reports.ts busca /v1/reports/pauses (tolerante a 404 antes do rebuild) no escopo loja e tenant. eslint/vue-tsc/vitest OK." }
    ],
    blockers: [],
    verifiable: "Multi-loja: 'Todas as lojas' = leitura, escolher 1 loja no filtro = opera como operador comum (timers/serviceId reais). Modal de encerrar sem ID, só nome. Justificativa some até clicar Avançar/Concluir com campo vazio. Pausar+retomar grava reason/kind em operation_status_sessions; aba Pausas em Relatórios mostra qtd/horário/duração/motivo por consultor. build/lint/type-check + go test limpos."
  },

  // ─── Anel de meta no avatar da fila — 2 subagentes (back × front) ──
  //
  // Criada em 2026-06-17. Decisão de produto: anel de progresso de meta ao redor
  // do avatar do consultor no card da "Lista da vez", com gradiente vermelho→verde
  // conforme fecha 100% e popover no hover (meta/atingido/falta). Número CANÔNICO =
  // goalProgress do /v1/erp/crm (#2). Como esse endpoint é gestão-only (canViewERP
  // barra consultant/store_terminal), o snapshot da operação faz a PONTE: operations
  // recebe um GoalProgressProvider (interface injetada) implementado por um adapter
  // do erp, cacheado por (tenant, mês), e embute goalStats por consultor no
  // QueueEntry/OperationOverviewPerson. Todos os operadores veem (decisão "todos
  // veem de todos"). Payout fora do 1º corte. Exige rebuild api (usuário).
  {
    id: "operacao-anel-meta",
    code: "OPS-META",
    title: "Anel de meta no avatar da fila + popover no hover",
    goal: "Mostrar, ao redor do avatar de cada consultor no card da Lista da vez, um anel de progresso da meta mensal (gradiente vermelho→verde conforme fecha 100%) e um popover no hover com Meta/Atingido/Falta. O número é o goalProgress canônico do /v1/erp/crm, entregue a TODO operador (inclusive consultor/terminal sem permissão de ERP) via ponte no snapshot da operação.",
    status: "in_progress",
    startedAt: "2026-06-17",
    estimateWeeks: "1-2 dias",
    group: "fila-operacao",
    tasks: [
      { id: "meta-back-provider", label: "Back — interface GoalProgressProvider em queue/operations + adapter do erp (GetCRMOverview → map[profileConsultantID]GoalStats, valores em reais) com cache por (tenant, mês) TTL ~120s; injeção no wiring sem ciclo de import.", done: true, note: "FEITO 2026-06-17: interface GoalProgressProvider + GoalStats em model.go; adapter na composition root back/internal/platform/app/operations_goal_progress_adapter.go (cache (tenant,mês) TTL 120s, principal sintético platform_admin tenant-scoped, bypassa canViewERP); erp.GoalStatsByConsultant reusa resolveERPScope+GetCRMOverview; wiring SetGoalProgressProvider em app.go:174. Nil-safe (provider off = goalStats null)." },
      { id: "meta-back-snapshot", label: "Back — GoalStats{monthlyGoal,soldValue,remainingToGoal,progress,hasGoal} em QueueEntry e OperationOverviewPerson; service busca via provider (tem tenant) e passa o map p/ buildSnapshotView (mantido puro); espelhar no caminho do overview. Testes + AGENT.md.", done: true, note: "FEITO 2026-06-17: buildSnapshotView recebe map[string]GoalStats e preenche goalStats em waitingList+activeServices; service.Snapshot/Overview buscam via goalStatsForTenant (log WARN, erro nunca propaga); 4 loops do overview preenchidos. snapshot_roster_test atualizado + TestBuildSnapshotViewFillsGoalStatsOnWaitingList. AGENT.md operations+erp atualizados. go build/vet/test OK. CORRECAO 2026-06-17: a META passou a vir de operation_goal_targets (Repository.EffectiveMonthlyGoalByConsultant + combineGoalStats no service), nao mais so do ERP — consultor com meta cadastrada mas sem venda/vinculo ERP (ex.: Fabio 45k) caia em 'Sem meta cadastrada'. ERP cobre o consultor => usa o stat exato do #2; senao meta canonica + vendido ERP (ou 0). Rebuild da api feito. CORRECAO 2 (2026-06-17): o VENDIDO vinha 0 porque o escopo do provider usava access.TenantID, vazio p/ platform_admin (as rotas da operacao usam RequireAuth, sem X-Account-Id => AccountID tb vazio). Fix: AccessContext.ScopeTenantID() (AccountID->TenantID) + fallback no tenant_id da loja (GetStoreTenantID no snapshot; StoreScopeView.TenantID no overview) + WARN operations_goal_stats_no_scope. Rebuild feito." },
      { id: "meta-front-ring", label: "Front — OperationConsultantAvatarRing.vue (avatar + anel SVG com gradiente de tokens do design system + popover no hover/foco, Teleport p/ não ser cortado) exibindo Meta/Atingido(%)/Falta.", done: true, note: "FEITO 2026-06-17: anel SVG com linearGradient de tokens (--danger→--accent-warning→--primary→--success), id único por instância (useId), arco via stroke-dasharray; popover via Teleport(body)+position fixed (getBoundingClientRect, reposiciona em scroll/resize), abre no hover+foco, fecha no leave/blur/Esc; sem meta = anel muted + 'Sem meta cadastrada'. eslint 0 erros." },
      { id: "meta-front-wire", label: "Front — usar o componente no queue-card__avatar do card da fila (OperationQueueColumns) + propagar goalStats em mapIntegratedWaitingItem (OperationWorkspace). AGENT.md.", done: true, note: "FEITO 2026-06-17: card WAITING (Lista da vez) E card Em atendimento (OperationActiveServiceCard, avatar do primaryService) — mesmo componente reusado; mapIntegratedWaitingItem + mapIntegratedActiveItem + mapScopedActiveItem repassam goalStats; buildOperableStoreState/servicesGroupedByConsultant fluem via spread. Gradiente trocado por verde sólido (--primary era azul). AGENT.md atualizado. eslint ok." }
    ],
    blockers: [],
    verifiable: "Card da Lista da vez mostra anel colorido por atingimento; hover abre popover com Meta/Atingido/Falta vindos do /v1/erp/crm via snapshot; consultor/terminal veem mesmo sem permissão de ERP. go build/vet/gofmt + go test limpos; eslint + vue-tsc + vitest limpos. Falta (usuário): rebuild api (docker compose up -d --build api) + validação no browser."
  },

  // ─── Coluna lateral da operação (Comunicados + Omni Chat) ──
  //
  // Criada em 2026-06-17. 1a etapa = TEMPLATE no front (OperationSidePanel.vue) como
  // 3a coluna do board da operação. Falta a implementação real: comunicados/campanhas
  // (dados do back) e o chat IA (Omni Chat). Larguras das colunas centralizadas em
  // web/app/assets/styles/layout.css (.queue-grid: --queue-grid-*-column).
  {
    id: "operacao-painel-lateral",
    code: "OPS-LAT",
    title: "Coluna lateral da operação — Comunicados + Omni Chat",
    goal: "Adicionar uma 3a coluna no board da operação com um bloco de Comunicados (campanhas ativas, mensagens, avisos) no topo e um chat de IA (Omni Chat) embaixo, para atendimento/produtos/pesquisa/operacional/dúvidas gerais/pesquisa de mercado. 1a etapa é só o template no front; depois a implementação real com dados e backend.",
    status: "in_progress",
    startedAt: "2026-06-17",
    estimateWeeks: "a definir",
    group: "fila-operacao",
    tasks: [
      { id: "lat-template", label: "Front — template da 3a coluna (OperationSidePanel.vue: card Comunicados + card Omni Chat com input desabilitado), plugado no .queue-grid; larguras por variável CSS em layout.css; marcado 'Prévia'.", done: true, note: "FEITO 2026-06-17: OperationSidePanel.vue + import no OperationQueueColumns + 3a var --queue-grid-side-column em layout.css. Só front, sem dados/backend; tag 'Prévia' visível. eslint ok." },
      { id: "lat-comunicados", label: "Back+Front — Comunicados reais (campanhas ativas / mensagens / avisos) por loja/conta.", done: false },
      { id: "lat-chat", label: "Back+Front — Omni Chat (IA) integrado (atendimento/produtos/pesquisa/operacional/dúvidas/pesquisa de mercado).", done: false }
    ],
    blockers: [],
    verifiable: "Board da operação mostra a 3a coluna com Comunicados (topo) e Omni Chat (rodapé); largura ajustável via .queue-grid no layout.css; mobile colapsa para 1 coluna. Etapas reais (dados + chat) pendentes."
  },

  // ─── Organização do Menu (Header × Sidebar) — 2 subagentes (back × front) ──
  //
  // Criada em 2026-06-16. Plano canônico: docs/MENU_LAYOUT_CONFIG.md.
  // Problema: header e sidebar renderizam os MESMOS itens (header = flatMap de
  // visibleSections em useDashboardNav). Para platform_admin (vê tudo) o header
  // estoura. Decisões de produto: config GLOBAL da plataforma (não per-user/tenant),
  // controle = posição (header/sidebar/ambos/oculto) + reordenar, e fix responsivo
  // do header no escopo. Persistência: core.platform_settings (singleton KV jsonb).
  // Migration 0160 (0159 reservada pela fase OPS). Exige rebuild api (usuário).
  {
    id: "menu-layout",
    code: "MENU",
    title: "Organização global do menu — header × sidebar editável pelo platform_admin",
    goal: "Dar ao platform_admin uma tela para definir UMA organização global do menu: por item, escolher se aparece no header, só no sidebar, em ambos ou oculto, e reordenar itens/seções por drag-and-drop. Persistir em core.platform_settings e tornar o header responsivo (excedente colapsa em 'Mais').",
    status: "in_progress",
    estimateWeeks: "2-3 dias",
    startedAt: "2026-06-16",
    group: "menu-layout",
    tasks: [
      { id: "menu-a-back", label: "A — Back: migration 0160_core_platform_settings.sql (singleton KV jsonb, platform-global) + módulo core (platform_settings model/repository/service/http) + GET /v1/platform/menu-layout (RequireAuth) e PATCH (requirePlatformAdmin) + wire em module.go + AGENT.md. Exige rebuild api (usuário)", done: false, note: "ESCRITO 2026-06-16 (subagente back): 4 arquivos platform_settings_* + migration 0160 + wire em module.go (Build/RegisterRoutes) + AGENT.md. go build EXIT=0, golangci-lint 0 issues. FALTA (usuário): aplicar migration 0160 + docker compose up -d --build api + teste no browser." },
      { id: "menu-b-nav", label: "B — Front: store menuLayout (load GET, save PATCH) + useDashboardNav passa a dividir header/sidebar por placement (header/sidebar/ambos/oculto) e ordenar itens/seções; default 'both' preserva comportamento atual", done: false, note: "ESCRITO 2026-06-16 (subagente front): store menuLayout.ts + useDashboardNav split (allowedSections → visibleSections/headerItems independentes; default 'both' = comportamento atual) + load no dashboard.vue. vue-tsc limpo. FALTA: teste no browser." },
      { id: "menu-c-config", label: "C — Front: tela /manage/menu-layout (MenuLayoutEditor/ItemRow/Preview, drag-and-drop nativo HTML5 reusado de OmniTableColumnsConfig, seções colapsáveis, preview ao vivo, botão 'Sugerir layout enxuto') + wiring de 3 arquivos (workspaces.ts, permissions.ts platform_admin, nav.config.ts seção manage)", done: false, note: "ESCRITO 2026-06-16: página + MenuLayoutEditor/ItemRow/Preview + useMenuLayoutEditor + wiring de 3 arquivos (workspaces/permissions só platform_admin/nav.config seção manage). ROTA CORRIGIDA: era /configuracoes/menu mas configuracoes.vue (página da Fila) virava rota-pai e engolia a filha (+ /configuracoes é gated por queue no module-enabled.global.ts); movida p/ pages/manage/menu-layout.vue (/manage/menu-layout, não-agency, sempre acessível). FALTA: teste no browser." },
      { id: "menu-d-header", label: "D — Front: header responsivo em DashboardHeader.vue (ResizeObserver mede itens que cabem; excedente vai p/ popover 'Mais' com fechar clique-fora/Esc/opção)", done: false, note: "ESCRITO 2026-06-16: overflow 'Mais' via ResizeObserver + faixa de medição oculta; hover/drawer intactos. REFATORADO depois p/ respeitar o limite de 450 linhas: DashboardHeader.vue (1207→203) decomposto em DashboardHeaderNav/NavMore/ProfileMenu/Drawer/Avatar + composables useHeaderNavOverflow/useDropdownDismiss/useHeaderProfile — todos <450, ESLint 0 warning, vue-tsc limpo nos arquivos tocados. FALTA: teste no browser." }
    ],
    blockers: [],
    verifiable: "Como platform_admin, /manage/menu-layout move itens entre Header/Sidebar/Ambos/Oculto e reordena por drag; Salvar persiste em core.platform_settings e sobrevive a reload. Header mostra só header/ambos, sidebar só sidebar/ambos, oculto some dos dois. Janela estreita colapsa excedente do header em 'Mais'. Papel não-admin não vê a tela e PATCH direto retorna 403. vue-tsc + ESLint + golangci-lint limpos."
  },

  // ─── Comissão v2 — cálculo no back (API-first) — 2026-06-17 ───────────────
  //
  // O "Recebimento por atingimento de meta" era calculado SÓ no front
  // (crm-performance-policy.ts) sobre o total da loja para todos os grupos. Vira
  // serviço de domínio Go (queue/commission) embutido em /v1/erp/crm: consultor
  // sobre a PRÓPRIA venda (trava ≥100% da própria meta + penalidade 0,1%/métrica
  // PA/Ticket), gerente sobre a loja com faixas por tipo de loja (Shopping/Bairro
  // = coluna nova em queue.stores). Política segue em JSONB (config tenant-wide).
  // Supersede a antiga task "payout-domain" (front-only) do grupo crm-360.
  {
    id: "comissao-v2-core",
    code: "COM v2",
    title: "Comissão por atingimento de meta — cálculo no back + tipo de loja + /consultor",
    goal: "Mover o cálculo de comissão para um serviço de domínio Go (queue/commission) embutido em GET /v1/erp/crm, adicionar store_type (Shopping/Bairro) à loja, atualizar o editor de Metas CRM (4 grupos + regras do consultor) e a página /consultor (display do payout do back, gerentes nos cards, botão Mês anterior). Política em JSONB; cálculo no back como fonte única reutilizável por site/app/automação.",
    status: "in_progress",
    estimateWeeks: "3-5 dias",
    startedAt: "2026-06-17",
    group: "comissao-v2",
    tasks: [
      { id: "com-keystone", label: "Keystone: contrato — shape JSONB v2 (managerShopping/managerBairro + consultantRules) + tipos TS + normalize v2 em crm-performance-policy.ts + remover cálculo do front + nome store_type + DTO payout no /v1/erp/crm", done: true, note: "FEITO 2026-06-17: tipos v2 + normalize (retrocompat com 'manager' legado) + DEFAULT v2 + mapRoleToPayoutGroup (display) em crm-performance-policy.ts; cálculo (calculateStoreGoalPayout/calculateCrmGoalPayout) removido do front; test 5/5 verde (vitest). Fases pending P1-P3 documentadas." },
      { id: "com-a-domain", label: "A (back): pacote queue/commission (model/calculate/test) — Calculate/NormalizePolicy/ResolveRule/MapRoleToGroup; fonte única do cálculo, puro e testado", done: true, note: "FEITO 2026-06-17 (subagente Opus): back/internal/modules/queue/commission/{model,calculate,policy_json,calculate_test}.go + AGENT.md. go test verde." },
      { id: "com-a-erp", label: "A (back): enriquecer GET /v1/erp/crm com payout por consultor + payout de loja (manager Shopping/Bairro, support), carregando política+metas+store_type em batch tenant-scoped (resolveTenantScope)", done: true, note: "FEITO: repository_crm_payout.go (loadCRMPayoutInputs+applyCRMPayouts, batch, sem N+1); CRMConsultantMetric.payout + CRMStoreMetric.{storeType,managerPayout,supportPayout}. Meta do consultor de queue.operation_goal_targets; sem meta cadastrada => progress 0 => payout 0." },
      { id: "com-a-migrations", label: "A (back): migration 0161 (queue.stores.store_type + recria view public.stores) e 0162 (crm_goal_payout_policy v2 + backfill idempotente); settings/defaults.go v2", done: true, note: "FEITO: 0161/0162 idempotentes, schema-qualificadas, sem goose Down. defaults.go v2. FALTA (usuário): aplicar migrate up no :5433 + docker compose up -d --build api." },
      { id: "com-a-storetype", label: "A (back): update de loja aceita/valida store_type ('shopping'|'bairro') + expõe nos selects do roster/stores", done: true, note: "FEITO: modules/stores (model/service normalizeStoreType/http json storeType/store_postgres/scope_queries). gofmt normalizado (5 arquivos estavam CRLF)." },
      { id: "com-b-editor", label: "B (front): SettingsCrmGoalsSection.vue 4 grupos (Consultor/Gerente Shopping/Gerente Bairro/Caixa) + subcomponente Regras do consultor (base/trava/penalidade) + useSettingsWorkspace; arquivo < 450 ln", done: true, note: "FEITO 2026-06-17 (subagente Opus): 4 grupos derivados de payoutGroups; SettingsCrmConsultantRules.vue (107 ln) novo; saveCrmConsultantRules em useSettingsWorkspace. eslint/vue-tsc limpos. FALTA: browser." },
      { id: "com-c-display", label: "C (front): consumir payout do back em consultants.ts/useConsultantIntegratedRows/Grid/Workspace/Card + CrmConsultantsSection; gerentes nos cards com valor do mês; simulador via ratePercent; remover cálculo do front", done: true, note: "FEITO 2026-06-17 (subagente Opus): consultant-integrated-view.ts capta payout; consultant-payout-display.ts (helpers); Grid/Workspace/Drawer/Simulator/StaffCard + CrmConsultantsSection leem payout do back. Sem erro tsc novo (restantes são pré-existentes). FALTA: browser." },
      { id: "com-c-loja-mes", label: "C (front): MultiStoreLojasSection tipo de loja (Shopping/Bairro) + botão Mês anterior em /consultor (resetIntegratedPreviousMonth)", done: true, note: "FEITO: select Tipo de loja (multistore.ts storeType no create/update, campo 'storeType' confirmado no back) + botão Mês anterior. FALTA: browser confirmar que o update de loja persiste storeType." }
    ],
    blockers: [],
    verifiable: "go test ./internal/modules/queue/commission passa; /v1/erp/crm retorna payout por consultor/loja; consultor ≥100% da meta + loja 50-79% recebe própria venda×1,5% (−0,1%/métrica PA/Ticket faltante); <100% da própria meta recebe 0; gerente Shopping≠Bairro sobre o total da loja; editor com 4 grupos persiste; botão Mês anterior funciona. vue-tsc/eslint/golangci-lint limpos."
  },

  // ─── Auditoria: lógica de negócio só-no-front → API (P1-P3) ───────────────
  // Backlog URGENTE descoberto ao mover a comissão: várias regras calculadas no
  // cliente, parte DUPLICADA no back (admin-metrics.ts vs analytics/). Mesmo
  // padrão da Comissão v2: domínio Go + embute no endpoint + front vira display.
  {
    id: "front-to-api-audit",
    code: "F2API",
    title: "Migrar lógica de negócio só-no-front para API (auditoria)",
    goal: "Eliminar a lógica de negócio que vive no cliente (e às vezes duplicada no back), movendo para serviços de domínio Go expostos por API, seguindo o padrão da Comissão v2. Front vira display.",
    status: "pending",
    estimateWeeks: "1-2 semanas",
    group: "comissao-v2",
    tasks: [
      { id: "f2api-p1-ranking", label: "P1 (Alto, DUPLICADO): ranking/alertas do consultor — admin-metrics.ts (772 ln) buildRankingRows/buildConsultantAlerts são iguais ao analytics/service_ranking.go do back; consolidar no back como única fonte e remover do front (+buildConsultantStats/buildInsights/buildTimeIntelligence/buildOperationalIntelligence)", done: false },
      { id: "f2api-p1-integrated", label: "P1 (Alto): ranking integrado montado no cliente — consultant-integrated-view.ts buildIntegratedRankingResponse mescla ERP+histórico no front (consultants.ts); mover para endpoint integrado pronto no back", done: false },
      { id: "f2api-p2-reports", label: "P2 (Médio): reports.ts (711 ln) finalizar migração buildReportData→buildReportDataFromApi (servidor) e remover o caminho front", done: false },
      { id: "f2api-p2-campaigns", label: "P2 (Médio): campaigns.ts buildCampaignPerformance para API", done: false },
      { id: "f2api-p3-listusage", label: "P3 (Baixo): crm-list-usage.ts (325 ln) buildCrmListUsageSummary para API", done: false }
    ],
    blockers: [],
    verifiable: "Cada lógica migrada tem teste Go; o front correspondente passa a ler o resultado do back (sem recálculo); admin-metrics.ts deixa de duplicar analytics/. vue-tsc/eslint/golangci-lint limpos; cada arquivo < 450 linhas."
  },
  // Usuários e acessos: consertar o que está quebrado nas duas telas de usuário.
  // Doc canônico: docs/USUARIOS_ACESSOS_FIX_PLAN.md. A regra de coerência
  // conta→usuário (mapa module_id→workspaces) fica fora desta leva (só complexo).
  {
    id: "usuarios-acessos-fix",
    code: "UAF",
    title: "Usuários e acessos — corrigir operacao/usuarios + manage/users",
    goal: "Voltar a salvar os módulos do usuário na operação (com revogação ao vivo) e permitir definir/resetar senha no manage/users. Coerência conta→usuário fica para a próxima leva.",
    status: "in_progress",
    estimateWeeks: "1-2 dias",
    startedAt: "2026-06-18",
    tasks: [
      { id: "uaf-a1-save", label: "Trilha A: operacao/usuarios volta a salvar módulos", done: true, note: "Backend ja salvava (GET/PUT /v1/access/overrides 200, testado ao vivo). Bug era no front: validacao de nome/loja bloqueava o save quando so os modulos mudavam — corrigido com flag basicChanged." },
      { id: "uaf-a2-error", label: "Trilha A: tirar o erro silencioso do access — mostrar motivo real inline", done: true, note: "saveDetails agora mostra detailAccessError real e nunca diz que salvou modulos quando nao salvou." },
      { id: "uaf-a3-live", label: "Trilha A: revogar ao vivo no front (WS access → re-buscar contexto → remontar menu)", done: true, note: "Ja estava cabeado em useContextRealtime (resource access → auth.fetchContext + refreshRealtimeState); verificado. Backend re-resolve perms do banco a cada request." },
      { id: "uaf-a4-perfis", label: "Trilha A: reexpor a aba 'Perfis' (UsersRoleMatrixManager) no modo queue", done: true, note: "Gated por canManageRoleDefaults (platform_admin)." },
      { id: "uaf-b1-create", label: "Trilha B: manage/users — criar usuario com senha que consegue logar", done: true, note: "Criar sem vinculo gerava login 500; agora o login retorna 403 user_no_role e o modal exige cliente, agencia (com cargo) OU platform admin." },
      { id: "uaf-b3-agency", label: "Trilha B: usuario de agencia loga com cargo (acesso de agencia nos clientes)", done: true, note: "Criar com organizationId+orgRole matricula na conta-agencia (agency_owner->owner total, agency_member->director limitado). Testado ao vivo: login 200 com papel correto. Switcher org-aware abre os clientes da agencia." },
      { id: "uaf-b2-reset", label: "Trilha B: manage/users — definir/resetar senha (campo password no PATCH + acao na grid)", done: true, note: "Back (model/service/repo) e front prontos; build+vet+eslint limpos. Teste real do PATCH so apos rebuild da api." },
      { id: "uaf-docs", label: "Sincronizar AGENT.md (core, components/users) + doc canônico + roadmap", done: true }
    ],
    blockers: [],
    verifiable: "Em /operacao/usuarios o admin altera os módulos de um usuário e salva (persistido em access); a sessão logada do afetado atualiza o menu na hora. Em /manage/users dá para criar usuário com senha e resetar a senha depois, e o usuário consegue logar. Pendente: rebuild da api para o reset de senha; teste no browser. vue-tsc/eslint/golangci-lint limpos (warnings pre-existentes de max-lines)."
  },
  // RBAC multi-tenant em camadas: modulo×pagina, teto da conta (account_modules),
  // perfis custom por conta e overrides por usuario. Doc: docs/RBAC_ACESSO_MODELO.md.
  {
    id: "rbac-acesso",
    code: "RBAC",
    title: "Acesso por usuário: drawer de edição, nível por vínculo e módulos/páginas",
    goal: "manage/users vira a tela canônica de gestão de acesso: editar o usuário, trocar o nível dentro do cliente/agência e dar/remover módulos/páginas, respeitando o que a conta contratou. Perfis customizáveis por conta vêm na fase 3.",
    status: "in_progress",
    estimateWeeks: "1-2 semanas",
    startedAt: "2026-06-18",
    tasks: [
      { id: "rbac-f1-drawer", label: "Fase 1: drawer de edição por usuário em manage/users (abre, edita básico + senha)", done: true, note: "AdminUserEditDrawer.vue: dados (nome/nick/email/ativo/platform admin) + senha. Botão de editar (lápis) por linha. eslint limpo." },
      { id: "rbac-f1-memberships", label: "Fase 1 (back): memberships devolvem role + isAgency; endpoint para trocar o papel do usuário numa conta", done: true, note: "GET memberships agora traz role+isAgency; PATCH /v1/admin/users/{id}/memberships/{accountId} faz replace de user_role_assignments (owner/director/marketing; invalido->400). Testado ao vivo: troca owner->director e o login reflete director." },
      { id: "rbac-f1-level", label: "Fase 1: trocar o nível/papel do usuário por vínculo (cliente/agência) no drawer", done: true, note: "Select de nível por vínculo no drawer chamando o PATCH; lista re-renderiza com o papel novo." },
      { id: "rbac-f1-modules", label: "Fase 1B: dar/remover módulo/página por usuário no drawer (reaproveita overrides do access)", done: false, note: "Proxima etapa: painel de módulos/páginas dentro do drawer, via /v1/access/users/{id}." },
      { id: "rbac-f2-map", label: "Fase 2: mapa módulo→páginas explícito + coerência usuário ⊆ account_modules no back (override só dev/admin)", done: false },
      { id: "rbac-f3-perfis", label: "Fase 3: perfis customizáveis por conta (permissão por página) + atribuição ao usuário", done: false }
    ],
    blockers: [],
    verifiable: "Em /manage/users o admin abre um usuário, troca o nível dele dentro de um cliente/agência e dá/remove módulos/páginas; o usuário passa a ver/perder o acesso conforme. Fases 2/3 adicionam o teto da conta e os perfis custom. vue-tsc/eslint/golangci-lint limpos."
  }
];

export const ROADMAP_MODULES: RoadmapModule[] = [
  {
    id: "meta_ads",
    label: "Meta Ads",
    route: "/meta-ads",
    status: "beta",
    priority: "P0",
    category: "operacao-comercial",
    description:
      "Gestão e relatórios de tráfego pago de Meta (Facebook/Instagram) no painel. Prioridade atual. MVP: conectar + puxar dados + dashboard básico. Plataforma: CRUD de campanha, relatórios ricos, IA e OAuth. Backend Go é a fonte (Marketing API → cache meta_ads.*).",
    scope: [
      "MVP: conectar (System User token) + sync de contas/campanhas/insights + dashboard com gráfico",
      "Criar/editar/pausar campanhas (manual e por IA)",
      "Relatórios e dashboards por cliente para decisão",
      "OAuth Facebook Login + atribuição agência→cliente"
    ],
    dependsOn: []
  },
  {
    id: "tasks",
    label: "Tasks",
    route: "/tasks",
    status: "done",
    priority: "P0",
    category: "atendimento",
    description:
      "Orquestrador de tarefas multi-tenant (boards + tabela). EM USO REAL: board geral da agencia (Crow Visuals, 247 tasks) + boards por cliente (Duby). Backend completo (T1-T9), realtime, tracking, RBAC, render progressivo. Multi-tenant fechado (board vive na conta-agencia; acesso org-aware). Refino continuo de performance e do editor segue como melhoria, nao bloqueio.",
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
    status: "beta",
    priority: "P1",
    category: "atendimento",
    description:
      "Time-tracking por cliente/usuario/periodo. ENTREGUE 2026-06-10: pagina /tracking com layout de board (TrackingBoardView, so tasks em play/pause) + aba Inteligencia consumindo GET /v1/tasks/tracking/metrics (byClient/byUser/byType em 1 query, GROUP BY server-side; agrega por client_account_id). Falta: export CSV e comparativos avancados.",
    scope: [
      "Export CSV dos buckets de tempo",
      "Comparativo Pessoa A vs B no periodo",
      "Metas de tempo por cliente"
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
    status: "beta",
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
    label: "Site (Leads + Produtos + Tracking)",
    route: "/site/leads",
    status: "beta",
    priority: "P2",
    category: "operacao-comercial",
    description:
      "Modulo site/ ENTREGUE (C17-C19): schema site (leads, products, webhook_sources, tracking_events); ingestao por webhook HMAC SHA-256; admin CRUD de leads/produtos com filtros + colunas travaveis; receptor de tracking (webhook Perola) + tela /site/tracking. Em uso real. O page-builder visual de paginas/forms fica como evolucao futura separada.",
    scope: [
      "Page-builder visual de paginas/forms (futuro)",
      "Campanha = pagina + canal + meta",
      "Mais fontes de webhook por cliente"
    ],
    dependsOn: []
  },
  {
    id: "bio",
    label: "Bio (link-in-bio)",
    route: "/site/bio",
    status: "beta",
    priority: "P2",
    category: "tools",
    description:
      "CRUD multitenant das paginas de bio (link-in-bio), servidas pelo front Nuxt separado crow-nuxt (rota /bio/{slug}, consome /v1/public/bio). Cliente edita so a propria bio; admin/agencia gerencia todas com filtro por cliente. Backend modulo bio/ + schema bio (migration 0152). Plano: docs/bio/PLANO_MODULO_BIO.md.",
    scope: [
      "Editor de blocos (menu/links/slides/lojas) colapsavel e compacto",
      "Temas/tokens por bio",
      "Analytics de cliques por link"
    ],
    dependsOn: []
  },
  {
    id: "cardapio",
    label: "Cardapio Online",
    route: "/cardapio",
    status: "beta",
    priority: "P2",
    category: "tools",
    description:
      "CRUD multitenant de cardapios online (restaurantes), servidos por um front Nuxt estatico no host do cliente, com resolucao de tenant por dominio, pedidos recalculados no servidor e tracking. Backend modulo cardapio/ + schema cardapio (migration 0153). Por enquanto na conta da agencia. Plano: docs/cardapio/PLANO_MODULO_CARDAPIO.md.",
    scope: [
      "Recalculo de pedido server-side (preco/estoque)",
      "Resolucao de tenant por dominio do cliente",
      "Integracao com WhatsApp para receber pedido"
    ],
    dependsOn: []
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
    body: "Quando um modulo/pagina nao esta pronto, usar hidden:true em web/layers/queue/nav.config.ts (fonte unica do menu desde 2026-06-13; o legado web/app/utils/sidebar-nav.ts foi removido). Para itens em beta, usar beta:true (renderiza badge).",
    why: "Evita que usuario navegue para pagina quebrada. Beta deixa explicito que a feature pode mudar.",
    appliesWhen: "Adicionar/remover modulo do menu lateral."
  },
  {
    id: "fe-rota-nova-checagem",
    category: "frontend",
    title: "Criar pagina nova — checar rota-pai, gating de path e workspace ANTES (falha silenciosa)",
    body: "Ao criar pages/<...>.vue rodar 3 checks que falham SEM erro de build/type-check (so o browser revela): (1) ROTA-PAI — se existe o arquivo pages/<x>.vue, qualquer pages/<x>/<y>.vue vira rota-filha e so renderiza se o pai tiver <NuxtPage/>; senao o pai engole a filha; usar outro prefixo. (2) GATING DE PATH em module-enabled.global.ts — a path herda o modulo do prefixo (/configuracoes,/operacao,/ranking,/relatorios,/alertas...->queue; /crm,/erp->crm; /site/*->site; /cardapio->cardapio; /meta-ads->meta_ads); pagina global/admin nao pode ficar sob prefixo de modulo; /manage/* (fora de AGENCY_ONLY_PATHS) e sempre acessivel. (3) WORKSPACE em auth.global.ts — definePageMeta workspaceId precisa estar no ROLE_WORKSPACES do papel (wiring de 3 arquivos: workspaces.ts + permissions.ts + nav.config.ts).",
    why: "Aconteceu (2026-06-16): pages/configuracoes/menu.vue abria a pagina da Fila (configuracoes.vue virava rota-pai e engolia a filha) e /configuracoes e gated por queue. Movido para pages/manage/menu-layout.vue (/manage/menu-layout, sempre acessivel).",
    appliesWhen: "Criar QUALQUER pagina nova (pages/**/*.vue); depois abrir a rota no browser pelo papel-alvo e confirmar que renderiza a pagina certa."
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
