import type { RoadmapPhase } from "./types";

export const ROADMAP_PHASES_PART1: RoadmapPhase[] = [
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
      { id: "redis-cache", label: "Trocar PrincipalCache para Redis (pré-produção full)", done: false, note: "Deferido para quando subir produção com múltiplas instâncias." },
      { id: "feedback-unread-api", label: "Feedback: GET /v1/feedback(/me) devolver unread_count + preview da última mensagem por feedback, para o sino e o workspace nunca buscarem mensagens 1-a-1 (eliminar o fan-out de polling)", done: true, note: "2026-06-25: implementado. Backend (queue/feedback): Feedback.UnreadCount/LastMessage* + ToListView; query List agrega unread_count por linha (CASE pela perspectiva do viewer: dono vê respostas de terceiros, admin vê mensagens do criador) + LATERAL da última mensagem; service usa ToListView nos dois list. Front: FeedbackItem ganhou unread_count/last_message_*; sino e lista (getUnreadCount/getFeedbackPreview) passaram a ler do list, syncMessagesForFeedbacks removido (só o chamado aberto baixa a thread); upsert preserva os agregados nas mutações e applyLocalReadState zera unread ao ler. Eliminou o fan-out de N GET /messages por ciclo no painel inteiro." },
      { id: "feedback-realtime", label: "Feedback: empurrar feedback novo / resposta nova por WebSocket (módulo realtime) em vez de polling, zerando o poll do sino (60s), da lista (30s) e do chat aberto (15s)", done: false, note: "Continuação do feedback-unread-api. O realtime já tem canais de operação/tasks/presence; falta publicar evento de feedback e o front assinar. Detalhe nos AGENT.md de back/internal/modules/queue/feedback e web/app/components/feedback." }
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
    id: "perf-reaudit-login",
    code: "Perf-Reaudit",
    title: "Re-auditoria action-first: login + última página bloqueante (26/06/2026)",
    goal: "O usuário continua sentindo o painel travar ao trocar de página e ao logar. Re-auditoria confirmou: (1) a navegação entre páginas em PROD segue rápida — a dor diária no dev é o compile sob demanda do Vite, não o app; (2) das 41 páginas, 40 já são action-first — só /usuarios segura a troca de rota com await de topo; (3) o gargalo de app que ainda sobra é o LOGIN, que encadeia 4+ chamadas em sequência antes de navegar e solta o estado de loading antes de a navegação terminar (botão volta a 'Entrar' ainda na tela de login). Fonte: docs/PERFORMANCE_AUDIT_PLAN.md §17.",
    status: "in_progress",
    estimateWeeks: "Diagnóstico feito 2026-06-26; correções pendentes",
    startedAt: "2026-06-26",
    tasks: [
      { id: "reaudit-sweep", label: "Varredura action-first nas 41 páginas (skeleton imediato vs branco/await de topo)", done: true, note: "10 agentes em paralelo. Resultado: 31 OK + 9 sem fetch (action-first por construção) + 1 violação (/usuarios). Demais páginas disparam o fetch em onMounted/watch async — não suspendem a rota." },
      { id: "fix-usuarios-blocking-await", label: "/usuarios: remover o await de topo de UsersAccessManager.vue (const ctx = await useUsersAccessManager) que segura a troca de rota até /v1/users + /v1/auth/roles responderem", done: false, note: "UsersAccessManager.vue:19. Tornar a chamada síncrona e mover o ensureLoaded() (useUsersAccessManager.js:1080) para onMounted sem await, igual a clientes.vue. O AppEntityGrid já tem skeleton via :loading=usersStore.pending, mas hoje nunca aparece porque o componente nem monta antes do await. Só atinge /usuarios (mode='queue' → UsersWorkspace → UsersAccessManager); /manage/users usa AdminUsersWorkspace (já action-first)." },
      { id: "fix-login-actionfirst", label: "Login: navegar assim que /v1/me/context resolve (homePath já está pronto) e adiar syncRuntimeAccess (accounts + /v1/settings + consultants + snapshot) para background na página destino", done: false, note: "auth.ts login() → fetchContext() awaita syncRuntimeAccess inteiro com pending=true antes de navegar (mín. 4 round-trips sequenciais). homePath deriva só de principal (REDE #2), nada da parte adiada. Patch: fetchContext({deferRuntime:true}) dispara syncRuntimeAccess sem await (já é best-effort, try/catch que degrada). Cuidado: guard de in-flight em account.ts fetchAccounts p/ não duplicar /v2/me/accounts (middleware também chama)." },
      { id: "fix-login-loading-gap", label: "Login: manter o estado de loading ('Entrando...') até a navegação completar — hoje auth.pending zera no finally de login() ANTES do navigateTo, então o botão volta a 'Entrar' enquanto ainda na tela de login (sensação de travado)", done: false, note: "Menor risco: ref local 'submitting' em login.vue, botão/inputs gateados por (auth.pending || submitting), submitting solta num finally após o navigateTo. Não mexe no contrato de pending do store." },
      { id: "reaudit-verifiable-fase7", label: "Honestidade: o verifiable da fase-7 ('Login < 500ms; navegação sem latência perceptível') está contradito enquanto o login encadear chamadas e /usuarios travar — anotado aqui até as correções fecharem", done: false, note: "Login não é medido pelo perf_audit.py (só rotas pós-login). Para a página /performance refletir login seria preciso instrumentar o login no harness e re-emitir perf-data.ts." },
      { id: "warmup-dev-daily", label: "Dor diária no dev (não-bug do app): aceitar o compile do Vite ou rodar o warm-up/o build de prod local no dia a dia", done: false, note: "qa-bot/warmup_dev.py já existe (pré-compila as rotas). Alternativa: rodar o stage prod do web/Dockerfile local. Decisão de fluxo de trabalho, não de código do app." }
    ],
    verifiable: "Ao logar, a navegação começa logo após o contexto chegar e o loading só solta no paint da rota destino (sem botão 'morto' na tela de login). /usuarios troca de rota na hora e mostra skeleton enquanto carrega. Demais páginas inalteradas (já action-first)."
  },
  {
    id: "perfil-enriquecido",
    code: "Perfil+",
    title: "Pagina de perfil enriquecida (role-aware) + olhinho de senha",
    goal: "Transformar o /perfil de um form chapado num painel pessoal role-aware: alem de foto/dados/senha, mostrar contexto de acesso, seguranca/sessao e widgets puxados dos modulos que o usuario JA usa (feedback, tasks, consultor/ranking). Tudo com dado escopado ao proprio usuario (isolamento), honesto sobre o que e derivado vs persistido. Varredura de 36 ideias por 5 agentes; sintese existe-hoje vs net-new.",
    status: "in_progress",
    estimateWeeks: "Iniciado 2026-06-26",
    startedAt: "2026-06-26",
    tasks: [
      { id: "password-eye", label: "Olhinho (toggle de visibilidade) nas senhas via componente reutilizavel AppPasswordInput + feedback inline (min 8, confirmacao confere)", done: true, note: "web/app/components/ui/AppPasswordInput.vue (v-model padrao do projeto). Padrao do olho estava duplicado inline no login/esqueceu-senha; centralizado. Aplicado no /perfil; login/esqueceu podem migrar depois." },
      { id: "perfil-conta-acesso", label: "Bloco 'Conta e acesso' (read-only): papel, conta/cliente ativo, organizacao, modulos contratados", done: false, note: "Dado pronto: auth.role/getRoleLabel + useCoreAccountStore (activeAccount.name, organizationName, enabledModules). Front-only." },
      { id: "perfil-seguranca-sessao", label: "Bloco 'Seguranca e sessao': expiracao da sessao (principal.expiresAt, 12h sem refresh), sair da conta, esquecer login salvo neste navegador", done: false, note: "Dado pronto: principal.expiresAt (context_http.go:94), auth.logout, auth.clearRememberedLogin. Front-only." },
      { id: "perfil-suas-lojas", label: "Bloco 'Suas lojas': lojas acessiveis (nome/cidade) + loja ativa destacada + trocar loja + metas da loja", done: false, note: "auth.storeContext (com goals normalizados), auth.activeStoreId, auth.setActiveStore. Escopado pelo Principal no back (ListAccessible). Front-only." },
      { id: "perfil-feedback", label: "Bloco 'Seus chamados': contador por status + nao-lidas + ultima resposta + atalho enviar feedback", done: false, note: "useFeedbackStore.fetchMyFeedbacks -> GET /v1/feedback/me (ja escopado por UserID). Gated por modulo queue. Front-only." },
      { id: "perfil-timer-tasks", label: "Bloco 'Cronometro ativo': se ha um time-tracking rodando agora para o usuario (task + tempo ao vivo, ancora no servidor)", done: false, note: "GET /v1/tasks/tracking/active JA filtra pelo user no back; useTimeTracking. Caso mais limpo. Front-only." },
      { id: "perfil-nick", label: "Apelido (nick) editavel nos dados pessoais", done: false, note: "Campo nick existe no UserView, mas PATCH /v1/auth/me/profile so aceita displayName+email. Backend: UpdateProfileInput + UserRepository.UpdateProfile + store_postgres + http + fakes; rebuild da API." },
      { id: "perfil-consultor-endpoint", label: "Backend: GET /v1/analytics/me self-scoped (resolve o consultor por principal.userId, devolve stats + posicao no ranking + inputs de badge da loja do usuario)", done: false, note: "Necessario para o bloco do consultor SEM vazar o ranking dos colegas pro client (isolamento). Reusa buildRankingRows + lookup consultants.user_id. Rebuild da API." },
      { id: "perfil-consultor-bloco", label: "Bloco do consultor: minha posicao no ranking, minhas metas/metricas (conversao/ticket/PA), minhas badges do mes", done: false, note: "Consome /v1/analytics/me; reusa ConsultantBadges/useGamificationConfig. So aparece para consultor vinculado, conta com modulo queue. HONESTIDADE: badges sao do mes corrente (derivadas), nao colecao historica." },
      { id: "perfil-gamificacao-historico", label: "NET-NEW (NAO implementar agora): historico de premiacoes, streak, XP, niveis, medalhas persistidas, central de notificacoes unificada", done: false, note: "NAO existe no sistema (varredura confirmou: sem tabela de awards/points/streak). Exige tabela + job de fechamento + endpoint. Documentado para nao prometer como pronto." }
    ],
    verifiable: "No /perfil: senhas com olhinho; blocos de conta/acesso, seguranca/sessao e suas-lojas para todos; chamados e cronometro quando o modulo existe; bloco do consultor (ranking/metas/badges do mes) so para consultor vinculado, consumindo /v1/analytics/me (sem vazar dado de colega); apelido editavel. Nada de gamificacao historica inventada."
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
];
