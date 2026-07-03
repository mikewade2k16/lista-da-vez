import type { RoadmapPhase } from "./types";

export const ROADMAP_PHASES_PART2: RoadmapPhase[] = [
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
      { id: "users-parallel", label: "Abrir frente nova de usuários no front novo reaproveitando UsersWorkspace, sem remover /usuarios legado da fila", done: false, note: "2026-06-25: gestão completa de acessos em /manage/users (AdminUsersWorkspace + AdminUserEditDrawer em abas Dados/Vínculos/Papéis/Módulos/Senha sobre o OmniEntityDrawer reutilizável). Backend core: delegação multi-tenant (/v1/admin/users* já não é só platform_admin — agency_owner e core.users.manage por account, escopo resolvido no banco, 404 fora do escopo, identity-global só platform_admin), add/remove membership, link/unlink organização, overrides por usuário (core.user_permission_overrides), papéis core.roles custom + matriz, GET/PUT papéis por membro. Revisão adversarial de segurança sem falha crítica (M1/M2 corrigidos). Criar cliente sem usuário (adminEmail opcional) e vincular usuário sem cliente (bugs) resolvidos. PENDENTE: validação no browser (sem npm rodado) + /security-review final + aposentar /v1/access/* legado (ver docs/LEGADO.md item 5). /operacao/usuarios legado intacto." },
      { id: "clients-parallel", label: "Abrir frente nova de clientes/tenants no front novo mantendo /clientes legado intacto até fechar a estratégia tenant", done: false, note: "AUDITORIA 2026-05-28: tela manage/clientes-web.vue + composable useClientsManager.ts já existem, mas batem no BFF mock /api/admin/clients (web/server/utils/clients-repository.ts in-memory). Não é fonte de verdade. Será reescrito contra API Go real na multitenant-completion." },
      { id: "sequencing", label: "Deixar finance e demais módulos pesados para depois do lote simples validado no painel", done: false }
    ],
    verifiable: "Roadmap deixa explícito o lote simples como próxima onda; /usuarios e /clientes atuais permanecem operacionais; finance só começa depois desse recorte ser validado."
  },
  {
    id: "fase-14",
    code: "Fase 14",
    title: "Módulo Finance",
    goal: "Front em web/layers/finance/ sobre API Go real (back/internal/modules/finance, migration 0187, rotas /v1/finance/*); mock BFF removido (web/server/ extinto) em AC-12. Falta só a integração opcional com contacts.",
    status: "pending",
    estimateWeeks: "2-3 semanas",
    tasks: [
      { id: "sequencing", label: "Iniciar somente após validar profile, team, site e a frente nova de users/clientes no roadmap", done: false },
      { id: "frontend-layer", label: "Criar web/layers/finance/ com página finance.vue portada para o path /finance", done: true, note: "PORTADO 2026-06-30: layer web/layers/finance/ com pages/finance.vue como PORT FIEL do web-reference (layout UDashboardGroup + UDashboardSidebar redimensionável + UDashboardPanel, mesmas classes/estilos finances-page__*). Correção: no Nuxt UI v4 o 'Pro' foi unificado no @nuxt/ui, então UDashboard* existem no pacote community 4.8.0 (premissa anterior de 'trocar por grid' revertida). Lógica nos composables (sheet editor direto + provide(FINANCE_CONFIG_KEY) p/ o painel); helpers e tipos separados. Placeholder DemoWorkspacePage removido; layer registrado no nuxt.config; nav gateada por moduleId 'finance'; página com workspaceId '' (senão auth.global redirecionava p/ /operacao)." },
      { id: "components", label: "Portar FinanceLineCard.vue, FinanceRecurringGroupCard.vue, FinanceConfigPanel.vue e reusar OmniMoneyInput", done: true, note: "FinanceLineCard/FinanceRecurringGroupCard/FinanceConfigPanel (slideover) portados fiéis; OmniMoneyInput reaproveitado do layer tasks (não duplicado). Componentes Omni + UDashboard* via auto-import." },
      { id: "mock-bff", label: "Mock BFF temporário marcado (Nitro in-memory) para clicar a tela até o back existir", done: true, note: "web/server/api/admin/finance-* + web/server/utils/financeMockStore.ts (in-memory, só dev/SSR). Marcado com LegacyMarker kind='mock' (só admin) e registrado em docs/LEGADO.md #6. REMOVIDO 2026-07-02 (web/server/ extinto)." },
      { id: "backend", label: "Criar back/internal/modules/finance/ com schema finance.*, lançamentos, categorias, recorrências e ajustes", done: true, note: "IMPLEMENTADO 2026-07-02 (AC-12): módulo Go back/internal/modules/finance/ (module/model/service/service_config/store_sheets/store_config/http/errors), migration 0187_finance_module.sql (schema finance.* + config_state), rotas /v1/finance/* registradas com gating de módulo em app.go." },
      { id: "contacts-integration", label: "Integrar com contacts quando habilitado; usar entidade local quando contacts estiver desligado", done: false, note: "Módulo contacts ainda não existe; clientName sai de core.accounts. Sem OptionalModules declarado (evita referência morta). Único item que falta da fase." },
      { id: "permissions", label: "Declarar permissões finance.sheets.view/manage, finance.config.manage, finance.recurring.manage e role templates", done: true, note: "Implementadas em module.go: finance.sheets.view/manage, finance.config.view/manage, finance.recurring.manage + role templates finance.manager/finance.viewer; SyncCatalog upserta no boot." },
      { id: "acceptance", label: "Criar lançamento, efetivar recorrência, ajustar valor e consultar histórico via API Go", done: true, note: "Persistência real via API Go /v1/finance/* (dados sobrevivem a restart do container). Validação visual no browser pendente de aprovação de rebuild web." }
    ],
    verifiable: "/finance roda sobre API Go real (mock removido, web/server/ extinto); operações persistem no banco e o módulo respeita account_modules. Falta só a integração opcional com contacts (módulo ainda inexistente)."
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
];
