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
  }
];

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
    status: "done",
    estimateWeeks: "1 semana",
    startedAt: "2026-05-11",
    finishedAt: "2026-05-11",
    tasks: [
      { id: "schema-create", label: "Criar schema queue + migrations base", done: true },
      { id: "move-stable", label: "Migrar stores, consultants, settings, catalog → queue.* com FK para core.accounts(id)", done: true, note: "Migration 0104. FKs internas para queue.*; compat views em public.*." },
      { id: "compat-views", label: "Views de compatibilidade public.* → queue.* durante transição", done: true, note: "Views auto-updatable (PostgreSQL) — código Go legado sem alteração." }
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
      { id: "module-rewrite", label: "Reescrever back/internal/modules/operations/ como subpacote queue/operations/", done: false, note: "Adiado: código Go continua em public.* via views compat. Será reescrito quando o módulo queue chegar do outro projeto." },
      { id: "shape-compat", label: "Endpoints /v1/operations/* mantêm shape (front não muda)", done: true, note: "Garantido pelas views compat — Go lê public.* que aponta para queue.*." }
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
    finishedAt: "2026-05-11",
    tasks: [
      { id: "move-aux", label: "Migrar alerts, analytics, reports, erp → queue.*", done: true, note: "Migration 0106: tenant_alert_settings, alert_instances, alert_actions, erp_sync_runs, erp_item_raw, erp_item_current." },
      { id: "subpackages", label: "Cada um vira subpacote queue/<nome>/", done: false, note: "Adiado junto com rewrite do módulo operations — mesmo motivo." },
      { id: "erp-rebuild", label: "ERP: testar rebuild de projeções com base no schema novo", done: false, note: "Pendente validação em staging após aplicar migrations 0104-0106." }
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
    finishedAt: "2026-05-11",
    tasks: [
      { id: "layer-create", label: "Criar web/layers/queue/ com nav.config.ts", done: true, note: "nav.config.ts com todas as seções existentes; sobrescreve legado via module-registry plugin." },
      { id: "pages-move", label: "Mover pages/stores listadas em E.4 para o layer", done: false, note: "Adiado: pages ainda em web/app/pages/. Mover quando módulo queue chegar do outro projeto." },
      { id: "shell-minimal", label: "Shell app/ fica minimal", done: false, note: "Depende da movimentação das pages (item acima)." }
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
      { id: "delete-static", label: "Deletar web/app/utils/sidebar-nav.ts", done: false, note: "Pendente: sidebar-nav.ts ainda é fallback no plugin. Remover após validação em staging que layer queue cobre todos os itens." },
      { id: "sidebar-rewrite", label: "DashboardSidebarNav.vue reescrito para consumir useNavStore", done: true }
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
      { id: "module-order", label: "Definir ordem de entrada dos módulos e confirmar o primeiro módulo", done: true, note: "Ordem atual detalhada nas Fases 11-20: Theme Studio antes dos módulos; depois Tasks, Omni, Finance, Contacts/Admin, Site, Indicators, Tools, Team e Bio." },
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
    status: "in_progress",
    estimateWeeks: "1 semana",
    startedAt: "2026-05-11",
    tasks: [
      { id: "indices-stats", label: "Migration 0107 — ANALYZE pós-Fase 4 + índice cobertura sessions(id) WHERE revoked_at IS NULL", done: true },
      { id: "load-user-fast", label: "auth.LoadUserForAuth consolida findRecord + findStoreIDs em 1 query única (LATERAL)", done: true, note: "2 round-trips → 1 no lookup de user no hot-path." },
      { id: "resolve-perms-fast", label: "access.ResolveEffectivePermissions combina role_permissions + overrides em 1 query (UNION/EXCEPT)", done: true, note: "2 round-trips → 1 na resolução de permissões. Fallback DefaultRolePermissions preservado." },
      { id: "auth-token-consolidated", label: "AuthenticateToken usa LoadUserForAuth + ResolveEffectivePermissions (4 → 2 queries por request autenticado)", done: true },
      { id: "logout-endpoint", label: "POST /v1/auth/logout idempotente (preparado para revogação real na 7D)", done: true },
      { id: "logout-frontend", label: "Frontend auth.logout() chama backend antes de clearSession; falha de rede tratada como sucesso", done: true },
      { id: "middleware-auth-skip", label: "auth.global.ts pula ensureSession em rotas /auth/*", done: true, note: "Mata cascata de ensureSession na tela de login pós-logout." },
      { id: "bootstrap-parallel", label: "core/account.ts paraleliza /v2/me/accounts + /v2/me/context speculativo via Promise.all", done: true, note: "Quando o accountId do cookie bate com a lista, salva 1 round-trip no bootstrap." },
      { id: "principal-cache", label: "TTL em memória para Principal (sync.Map/ristretto) com invalidação por eventos — sessão, role, permission, account_modules", done: false, note: "Pendente: 7D. Só aplicar após medir o ganho real das 7A-7C em staging." },
      { id: "session-revogation", label: "JWT carrega sessionId + middleware checa core.user_sessions.revoked_at no mesmo lookup", done: false, note: "Pendente: parte da 7D quando o PrincipalCache existir. Sem cache, query extra de sessão anularia o ganho da 7A." },
      { id: "redis-cache", label: "Trocar PrincipalCache para Redis (pré-produção full)", done: false, note: "Pendente: 7E. Só quando subir produção com múltiplas instâncias." }
    ],
    verifiable: "Login < 500ms; navegação entre páginas sem latência perceptível; logout < 200ms sem bugs."
  },
  {
    id: "fase-8",
    code: "Fase 8",
    title: "Split CRM + Queue",
    goal: "Separar fila-atendimento (queue) de dados/dashboards CRM (ERP + catalog) em módulos independentes.",
    status: "pending",
    estimateWeeks: "2–3 semanas (bloqueada até Fase 7 entregar)",
    tasks: [
      { id: "migration-crm", label: "Migration 0108 — schema crm + mover erp_* de queue.* → crm.* (views compat em public.*)", done: false },
      { id: "module-crm", label: "back/internal/modules/crm/ com erp/ + catalog/ + dashboard/ implementando interface Module", done: false },
      { id: "resolver-crm", label: "crm.Resolver registrado em Dependencies para queue consumir opcionalmente", done: false },
      { id: "module-queue", label: "Consolidar back/internal/modules/queue/ — operations + alerts + analytics + reports + feedback + consultants + settings", done: false, note: "Termina pendências da Fase 4 (module-rewrite, subpackages)." },
      { id: "catalog-adapter", label: "queue/catalog_adapter.go — usa crm.Resolver se habilitado, senão entidade local", done: false },
      { id: "layer-crm", label: "web/layers/crm/ com nav.config.ts + pages /crm, /erp", done: false },
      { id: "nav-queue-cleanup", label: "Remover /crm, /erp do web/layers/queue/nav.config.ts", done: false },
      { id: "docs", label: "AGENT.md de crm, queue, erp (deprecated), catalog (deprecated) + CONTRACT_FREEZE.md com crm.Resolver", done: false }
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
    id: "fase-14",
    code: "Fase 14",
    title: "Módulo Finance",
    goal: "Trazer financeiro depois de Tasks/Omni, mantendo /finance como placeholder até a fase começar.",
    status: "pending",
    estimateWeeks: "2-3 semanas",
    tasks: [
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
    goal: "Definir a fonte de verdade de contatos/clientes e portar as páginas de gestão apenas quando for seguro substituir o legado.",
    status: "pending",
    estimateWeeks: "2-3 semanas",
    tasks: [
      { id: "decision", label: "Decidir se contacts substitui /clientes atual ou entra primeiro como módulo opcional paralelo", done: false },
      { id: "backend", label: "Criar back/internal/modules/contacts/ com Resolver consumível por finance, omni, site e queue", done: false },
      { id: "pages", label: "Avaliar port de admin/manage/clientes.vue, users.vue e modulos.vue sem quebrar CRUDs atuais", done: false },
      { id: "components", label: "Portar componentes manager/clients somente se a página nova for escolhida", done: false },
      { id: "account-modules", label: "Mapear admin/manage/modulos.vue para gestão de core.account_modules no futuro", done: false },
      { id: "acceptance", label: "Resolver de contacts funciona habilitado; consumidores fazem fallback quando desabilitado", done: false }
    ],
    verifiable: "Contacts pode ser habilitado por account, expõe Resolver estável e não substitui /clientes antes da decisão explícita."
  },
  {
    id: "fase-16",
    code: "Fase 16",
    title: "Módulo Site",
    goal: "Trazer páginas de produtos e leads do site/e-commerce para um módulo isolado.",
    status: "pending",
    estimateWeeks: "1-2 semanas",
    tasks: [
      { id: "backend", label: "Criar back/internal/modules/site/ com produtos publicados, leads, configurações e permissões", done: false },
      { id: "pages", label: "Portar admin/site/produtos.vue e admin/site/leads.vue para web/layers/site/pages/", done: false },
      { id: "contacts-integration", label: "Decidir se leads entram em contacts quando contacts estiver habilitado", done: false },
      { id: "nav", label: "Adicionar nav.config.ts do site e proteger rotas com module-enabled", done: false },
      { id: "acceptance", label: "Cadastrar produto, alternar visibilidade no site e consultar lead via API Go", done: false }
    ],
    verifiable: "Produtos e leads funcionam no layer site, com fallback claro para contacts e sem afetar /crm ou /erp."
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
    goal: "Avaliar e portar as telas de treinamento/candidatos como módulo de equipe, se fizerem parte do produto final.",
    status: "pending",
    estimateWeeks: "1-2 semanas",
    tasks: [
      { id: "product-decision", label: "Confirmar se team entra no produto ou fica fora do escopo imediato", done: false },
      { id: "backend", label: "Modelar candidatos, treinamentos, anexos e estados de processo quando aprovado", done: false },
      { id: "pages", label: "Portar admin/team/treinamento.vue e candidatos.vue", done: false },
      { id: "files", label: "Definir estratégia para anexos/CVs antes de subir a tela de candidatos", done: false },
      { id: "acceptance", label: "Criar candidato/treinamento e validar permissões por account", done: false }
    ],
    verifiable: "Team só aparece se for aprovado como módulo; telas não entram como sobras soltas do web-reference."
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
    status: "pending",
    estimateWeeks: "7–10 dias",
    group: "tasks-backend",
    tasks: [
      { id: "store", label: "web/layers/tasks/stores/tasks.ts: Pinia store com fetchBoards, fetchBoard, createTask, moveTask, applyRealtimeEvent", done: false },
      { id: "realtime", label: "useTasksRealtime.ts: clone de useOperationsRealtime, tópicos tasks:account + tasks:board, reconexão exponencial", done: false },
      { id: "tracking", label: "useTaskTracking.ts: server-backed, clockOffset, tick local 1s", done: false },
      { id: "relations", label: "useTaskRelations.ts: lazy load + cache + re-fetch em task.relation_added", done: false },
      { id: "can", label: "useCan.ts: computed contra useMeContext().permissions", done: false },
      { id: "wipe", label: "Boot detecta localStorage legado (omni.admin.tasks.workspace.v1) e descarta com aviso", done: false },
      { id: "pagination", label: "Paginação cursor-based limit=50; infinite scroll na table view", done: false },
      { id: "client-view", label: "Perspective derivada de permissões reais; servidor filtra; front não esconde dados", done: false }
    ],
    verifiable: "/tasks carrega via REST, zero localStorage; drag → REST+WS < 300ms; F5 mantém estado; client_viewer vê só boards com share."
  },
  {
    id: "tasks-t6",
    code: "Tasks T6",
    title: "Tracking server-side autoritativo",
    goal: "StartTracking/PauseTracking/ResumeTracking/StopTracking persistidos no banco; timer sincronizado por WS com clockOffset.",
    status: "pending",
    estimateWeeks: "2–3 dias",
    group: "tasks-backend",
    tasks: [
      { id: "service", label: "tasks/service_tracking.go: 6 métodos + partial unique (user_id, task_id) WHERE stopped_at IS NULL", done: false },
      { id: "optimistic-lock", label: "version check em transação com FOR UPDATE; ErrVersionConflict → 409", done: false },
      { id: "ws-events", label: "task.time_started/paused/resumed/stopped publicados no WS após cada mutation", done: false },
      { id: "frontend", label: "useTaskTracking.ts: tick local com serverOffset; reconcilia via WS; modal + card mostram timer", done: false }
    ],
    verifiable: "User A inicia → User B vê timer correndo; servidor reinicia → timer correto; máquina travada 5min → valor real do server."
  },
  {
    id: "tasks-t7",
    code: "Tasks T7",
    title: "Presence (avatares + field locking)",
    goal: "PresenceStore em memória com TTL 30s; protocolo heartbeat/field_focus/field_blur; front exibe avatares e badge 'editando campo X'.",
    status: "pending",
    estimateWeeks: "2–3 dias",
    group: "tasks-backend",
    tasks: [
      { id: "presence-store", label: "realtime/presence.go: PresenceStore TTL 30s + ticker de limpeza + publish presence.user_left", done: false },
      { id: "protocol", label: "Protocolo cliente→server: presence.heartbeat, field_focus, field_blur", done: false },
      { id: "frontend", label: "useTaskPresence.ts: abre presence:task:{id} ao abrir modal; heartbeat 15s", done: false },
      { id: "badge", label: "Front exibe badge 'Fulano editando título' (não trava input; 409 na conflict de save)", done: false },
      { id: "future-yjs", label: "tasks.task_doc_snapshots vazia criada para futuro cursor Y.js/Tiptap", done: false }
    ],
    verifiable: "Abrir modal em 2 abas → avatares mútuos visíveis; focar campo → badge na outra aba; sair sem heartbeat 30s → sumiu."
  },
  {
    id: "tasks-t8",
    code: "Tasks T8",
    title: "Segurança, audit, hardening",
    goal: "Audit log com retention, rate limit WS+REST, validação rigorosa de IDs, defense in depth em 3 camadas confirmada por testes.",
    status: "pending",
    estimateWeeks: "2 dias",
    group: "tasks-backend",
    tasks: [
      { id: "audit-endpoint", label: "GET /v1/tasks/:id/audit (perm tasks.boards.manage); retention 180d para não-premium", done: false },
      { id: "rate-limit", label: "WS: 30 events/s por conexão (close 1008); REST: 60 req/min; metrics: 1 req/3s", done: false },
      { id: "validation", label: "Nunca aceitar account_id do body — sempre derivar do Principal; client_account_id via share OU manage", done: false },
      { id: "404-not-403", label: "Cross-account → 404 (nunca 403); integration test confirma em todos os endpoints", done: false },
      { id: "logs", label: "slog estruturado em cada mutation: accountId, taskId, userId; erros sem IDs de outras accounts", done: false }
    ],
    verifiable: "Fuzz 100 IDs de outros tenants → 100% 404; WS rate-limit fecha conexão em 30+1 events/s; payload de client_viewer auditado manualmente."
  },
  {
    id: "tasks-t9",
    code: "Tasks T9",
    title: "Testes E2E + observabilidade",
    goal: "Cobertura > 70% no service Go (scope, DTO, tracking, version conflict); testes Vitest no front (store, realtime, useCan); smoke E2E 12 passos.",
    status: "pending",
    estimateWeeks: "2–3 dias",
    group: "tasks-backend",
    tasks: [
      { id: "service-test", label: "tasks/service_test.go: CRUD com 3 perspectives (agency, client_viewer, outro tenant)", done: false },
      { id: "scope-test", label: "tasks/scope_test.go: fuzz 100 IDs de outros accounts → 100% 404", done: false },
      { id: "tracking-test", label: "tasks/tracking_test.go: version conflict, 1 entry ativa por (user, task)", done: false },
      { id: "dto-test", label: "tasks/dto_test.go: snapshot JSON agency vs client_viewer (campos ausentes, não escondidos)", done: false },
      { id: "front-tests", label: "Vitest: useTasksStore mutations, useTasksRealtime reconnect+jitter, useCan matriz de perfis", done: false },
      { id: "smoke-e2e", label: "Smoke E2E 12 passos: migrate fresh → seed → login agência → criar task → WS → presence → tracking → share → curl 404 → inspect payload", done: false }
    ],
    verifiable: "Cobertura > 70% no service; scope test 100%; front tests passando; smoke E2E sem falha em staging."
  }
];
