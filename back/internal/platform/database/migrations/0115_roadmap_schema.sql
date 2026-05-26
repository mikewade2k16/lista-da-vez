-- Frente B Roadmap-B1 — schema roadmap
--
-- Cria as tabelas que sustentam a aba "Modulos" e "Regras" do workspace /roadmap.
-- O front parte de seeds estaticos em web/app/components/roadmap/roadmap-data.ts
-- e este schema permite que cada account customize/adicione modulos e regras.
--
-- Decisao: itens globais (account_id IS NULL) sao seed compartilhado entre
-- todas as accounts. Edicao por uma account cria override (mesmo source_id
-- mas com account_id setado) que sobrescreve o global na resposta da API.

create schema if not exists roadmap;

-- ============================================================================
-- Modulos do produto (Tasks, Tracking, Omnichannel, etc.)
-- ============================================================================

create table if not exists roadmap.modules (
    id uuid primary key default gen_random_uuid(),
    source_id text not null,
    account_id uuid references core.accounts(id) on delete cascade,
    label text not null,
    route text not null default '',
    status text not null check (status in ('pending', 'in_progress', 'beta', 'done')),
    priority text not null check (priority in ('P0', 'P1', 'P2', 'P3')),
    category text not null default '',
    description text not null default '',
    scope jsonb not null default '[]'::jsonb,
    depends_on jsonb not null default '[]'::jsonb,
    sort_order integer not null default 100,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint roadmap_modules_unique_per_account unique (source_id, account_id)
);

create index if not exists roadmap_modules_account_idx
    on roadmap.modules (account_id, priority, sort_order);

create index if not exists roadmap_modules_global_idx
    on roadmap.modules (priority, sort_order)
    where account_id is null;

-- ============================================================================
-- Regras canonicas para agentes (front, back, banco, deploy, etc.)
-- ============================================================================

create table if not exists roadmap.rules (
    id uuid primary key default gen_random_uuid(),
    source_id text not null,
    account_id uuid references core.accounts(id) on delete cascade,
    category text not null check (category in ('frontend', 'backend', 'banco', 'linguagens', 'deploy', 'padroes-gerais')),
    title text not null,
    body text not null,
    why text not null default '',
    applies_when text not null default '',
    sort_order integer not null default 100,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint roadmap_rules_unique_per_account unique (source_id, account_id)
);

create index if not exists roadmap_rules_account_idx
    on roadmap.rules (account_id, category, sort_order);

create index if not exists roadmap_rules_global_idx
    on roadmap.rules (category, sort_order)
    where account_id is null;

-- ============================================================================
-- Seed inicial (global, account_id IS NULL) — espelha
-- web/app/components/roadmap/roadmap-data.ts > ROADMAP_MODULES e ROADMAP_RULES.
-- Usar ON CONFLICT (source_id, account_id) DO NOTHING para idempotencia.
-- ============================================================================

insert into roadmap.modules (source_id, label, route, status, priority, category, description, scope, depends_on, sort_order) values
('tasks', 'Tasks', '/tasks', 'beta', 'P0', 'atendimento',
 'Orquestrador de tarefas multi-tenant (boards + tabela). Backend pronto (Fases T1-T9). UI em refino: rolagem vertical, performance, checklist no editor, expand/restore. Em uso interno antes de liberar para tenants.',
 '["Refinar performance do board para >500 cards","Melhorar feedback de drag-and-drop entre colunas","Adicionar filtros salvos por usuario","Notificacoes in-app quando @mention"]'::jsonb,
 '[]'::jsonb, 10),
('editor', 'Editor', '/editor', 'beta', 'P1', 'tools',
 'Editor rich-text Omni baseado em Tiptap (StarterKit + TaskList + Emoji + Mention + TextAlign). Usado em descricao de tasks. Falta: salvar/abrir documentos avulsos, versionamento, sharing.',
 '["Persistir documentos avulsos (nao apenas dentro de Tasks)","Adicionar /slash commands","Suporte a colaboracao em tempo real (avaliar @tiptap/y-tiptap)"]'::jsonb,
 '["tasks"]'::jsonb, 20),
('tracking', 'Tracking', '/tracking', 'pending', 'P1', 'atendimento',
 'Visao de time-tracking por consultor: tempos por task, relatorios diarios/semanais. Depende de Tasks 100% (presence + tracking ja existem no backend T7).',
 '["Agregacao por consultor/dia/semana","Export CSV","Comparativo Pessoa A vs B no periodo"]'::jsonb,
 '["tasks"]'::jsonb, 30),
('omnichannel', 'Omnichannel', '/omnichannel', 'pending', 'P2', 'atendimento',
 'Conversas unificadas WhatsApp/Instagram/Email/Webchat com handoff humano e bot. Page existe mas vazia. Escopo grande: webhook providers + threads + roteamento.',
 '["Conectores WhatsApp Cloud API + Instagram Direct","Schema messaging.* com threads","Roteamento por fila + handoff","Bot simples por palavra-chave"]'::jsonb,
 '[]'::jsonb, 40),
('team', 'Team (Equipe + Escalas)', '/team/equipe', 'pending', 'P2', 'operacao-comercial',
 'Gestao de equipe e escalas. Pagina existe mas sem CRUD real. Compartilha schema core.users + roles ja existentes.',
 '["CRUD de equipe com avatar e cargo","Calendario de escalas (turnos)","Aprovacao de troca de turno"]'::jsonb,
 '[]'::jsonb, 50),
('site', 'Site (Campanhas + Paginas + Forms)', '/campanhas', 'pending', 'P3', 'operacao-comercial',
 'Builder visual de paginas/forms + campanhas atreladas a pagina. Pagina /campanhas existe mas sem builder. Forms ainda nao implementado.',
 '["Builder drag-drop simples para pagina","Geracao de form com webhook","Campanha = pagina + canal de divulgacao + meta","Tracking de conversao"]'::jsonb,
 '[]'::jsonb, 60),
('inteligencia', 'Inteligencia', '/inteligencia', 'pending', 'P2', 'indicadores',
 'Insights gerados por LLM sobre dados de vendas e atendimento. Cards "Por que conversao caiu?" / "Quais produtos faltam mais?". Sera consumidor pesado do backend de BI.',
 '["Prompts canonicos para 5 perguntas frequentes","Cache de resposta por janela (dia/semana)","Exportar insight como PDF"]'::jsonb,
 '["bi"]'::jsonb, 70),
('relatorios', 'Relatorios', '/relatorios', 'pending', 'P2', 'indicadores',
 'Reports estaticos exportaveis (PDF/CSV). Backend reports/ existe parcial. Faltam templates e UI de configuracao.',
 '["Template Ranking Mensal Consultor (PDF)","Template Vendas por Loja (CSV + PDF)","Agendamento de envio recorrente por email"]'::jsonb,
 '[]'::jsonb, 80),
('bi', 'BI', '/bi', 'pending', 'P2', 'indicadores',
 'Dashboards customizaveis. Modulo backend bi/ ja criado mas sem UI. Definir entre dashboard hardcoded vs builder.',
 '["Decidir entre Metabase embedded vs builder proprio","MVP com 3 dashboards fixos (vendas, atendimento, estoque)","Filtros por loja/consultor/periodo"]'::jsonb,
 '[]'::jsonb, 90),
('finance', 'Finance', '/finance', 'pending', 'P3', 'indicadores',
 'Comissoes, metas financeiras, fechamento mensal. Hoje nao existe. Depende de Vendas (ERP) ja integrada.',
 '["Calculo de comissao por consultor com regras configuraveis","Fechamento mensal exportavel","Integracao com folha (fora do escopo inicial)"]'::jsonb,
 '[]'::jsonb, 100),
('monitoramento', 'Monitoramento', '/monitoramento', 'pending', 'P2', 'indicadores',
 'Pagina interna de health: uptime API, jobs ERP, sync FTP, fila de atendimento em tempo real. Pega de healthz + module registry + alerts.',
 '["Painel de modulos ativos (do module registry)","Historico de jobs ERP","Latencia /healthz dos ultimos 7 dias"]'::jsonb,
 '[]'::jsonb, 110),
('qr-tools', 'Tools secundarias (QR / Encurtador / Scripts)', '/tools/qr-code', 'pending', 'P3', 'tools',
 'Ferramentas auxiliares hoje meio implementadas. Atualmente ocultas do menu. Reativar so quando tiver demanda real.',
 '["QR Code com tracking de cliques","Encurtador integrado com tracking","Scripts: snippets reutilizaveis de mensagens"]'::jsonb,
 '[]'::jsonb, 120)
on conflict (source_id, account_id) do nothing;

insert into roadmap.rules (source_id, category, title, body, why, applies_when, sort_order) values
('fe-componentes-reutilizaveis', 'frontend', 'Componentes reutilizaveis acima de tudo',
 'Sempre que houver repeticao de markup ou logica visual, extrair em componente proprio em web/app/components/ ou na layer adequada. Workspaces nao podem virar arquivos gigantes; quebrar em cards/secoes/listas.',
 'Evita duplicacao e drift visual entre paginas. Facilita aplicar mudanca em um lugar so.',
 'Qualquer feature nova ou refactor que adicione UI.', 10),
('fe-classes-semanticas', 'frontend', 'Classes semanticas BEM-like',
 'Sempre usar nomes semanticos no estilo .nome-componente__elemento--modificador. Nao usar utility classes inline ou IDs para estilizacao.',
 'Permite leitura rapida do escopo de cada estilo e evita colisao global.',
 'Estilizacao de qualquer componente novo.', 20),
('fe-sem-emojis', 'frontend', 'Sem emojis em UI nem em codigo',
 'Nao usar emojis em labels, mensagens de UI, codigo, comentarios ou commits, salvo se o usuario pedir explicitamente.',
 'Mantem consistencia visual e profissional do produto.',
 'Sempre.', 30),
('fe-feature-flag-hidden', 'frontend', 'Esconder pagina nao pronta via hidden no menu',
 'Quando um modulo/pagina nao esta pronto, usar hidden:true em web/app/utils/sidebar-nav.ts E em web/layers/queue/nav.config.ts. Para itens em beta, usar beta:true (renderiza badge).',
 'Evita que usuario navegue para pagina quebrada. Beta deixa explicito que a feature pode mudar.',
 'Adicionar/remover modulo do menu lateral.', 40),
('be-padrao-modulo-go', 'backend', 'Padrao de modulo Go',
 'Cada modulo em back/internal/modules/<nome>/ tem: model.go (tipos), store_postgres.go (persistencia), service.go (regras), http.go (handlers), AGENT.md (documentacao). Modulos plugaveis se registram via Module Registry quando CORE_V2_ENABLED.',
 'Consistencia entre modulos facilita onboarding e troca de agente.',
 'Criar novo modulo backend.', 10),
('be-ids-strings', 'backend', 'IDs como string, nunca uuid externo',
 'Usar string para IDs no Go; nao importar pacote uuid externo. Casts e geracao ficam centralizados em internal/platform/ids/.',
 'Reduz dependencia externa e facilita refatoracao do esquema de IDs.',
 'Qualquer struct nova com ID.', 20),
('be-scan-nullable-string', 'backend', 'Scan de campos NULL com *string',
 'Para colunas nullable, declarar *string (ou sql.NullString se preferir) no Scan; nunca string puro.',
 'Evita panic em scan de NULL.',
 'Implementar store_postgres.go.', 30),
('be-perms-no-banco', 'backend', 'Permissoes vivem no banco (RBAC dinamico)',
 'Nao hardcoded permission names em codigo Go. Permissoes vivem em core.permissions + core.role_permissions; service consulta via Module Registry.',
 'Permite agencia customizar role sem deploy.',
 'Implementar checagem de permissao em handler ou service.', 40),
('banco-migration-idempotente', 'banco', 'Migration idempotente (IF NOT EXISTS)',
 'Toda migration usa IF NOT EXISTS / CREATE OR REPLACE. Numerar sequencialmente em back/internal/platform/database/migrations/####_nome.sql. Nunca renumerar migration ja aplicada.',
 'Migrations falhas no meio precisam poder ser reaplicadas sem dropar dados.',
 'Criar migration nova.', 10),
('banco-schema-multitenant', 'banco', 'Schema-per-modulo + account_id em todas as tabelas tenant-scoped',
 'Schemas: core, queue, tasks, alerts, settings, roadmap. Toda tabela tenant-scoped tem account_id NOT NULL com FK para core.accounts. Public schema pode ter VIEWS sobre tabelas dos schemas.',
 'Multi-tenancy com isolamento logico e queries por schema mais previsiveis.',
 'Criar tabela nova.', 20),
('banco-view-publica', 'banco', 'Mover tabela para schema: criar view publica',
 'Quando mover tabela de public.* para schema.*, criar CREATE OR REPLACE VIEW public.<tabela> AS SELECT * FROM schema.<tabela> para manter compat com codigo legado.',
 'Evita quebrar queries antigas que ainda apontam para public.*.',
 'Refactor de schema.', 30),
('lang-go-version', 'linguagens', 'Go 1.26',
 'Backend usa Go 1.26. Aproveitar generics, max/min builtins, slices/maps stdlib.',
 'Versao alinhada com infra de CI e Docker.',
 'Backend.', 10),
('lang-vue-nuxt', 'linguagens', 'Vue 3 + Nuxt 4 + Pinia',
 'Frontend usa Vue 3 (Composition API + <script setup>), Nuxt 4 (com layers em web/layers/*), Pinia para state. Tipos TS sempre que possivel.',
 'Stack escolhida pelo time; layers permitem isolar dominios.',
 'Frontend.', 20),
('lang-typescript-strict', 'linguagens', 'TypeScript strict',
 'Codigo TS deve passar em vue-tsc --noEmit. Evitar any. Preferir tipos explicitos em props e composables.',
 'Pega bug em build time, nao em prod.',
 'Qualquer codigo TS/Vue.', 30),
('deploy-vps-caddy', 'deploy', 'VPS Hostinger com Caddy + Docker Compose',
 'Deploy em VPS 85.31.62.33, user deploy. Caddy reverse proxy em /opt/omnichannel/Caddyfile. Cada projeto roda em /home/deploy/<projeto> com docker-compose.prod.yml. Nginx-style aliases por projeto na network proxy.',
 'Isolamento por projeto + um Caddy gerencia todos os dominios.',
 'Deploy ou troubleshooting de prod.', 10),
('deploy-feature-flag', 'deploy', 'Feature flag em .env.production E docker-compose.prod.yml',
 'Variaveis novas precisam de duas adicoes: .env.production (na VPS) E docker-compose.prod.yml na secao environment. Sem a segunda, o container nao recebe a variavel.',
 'Compose nao propaga automaticamente .env file inteiro; precisa de declaracao explicita.',
 'Adicionar variavel de ambiente nova.', 20),
('deploy-caddy-restart', 'deploy', 'Apos mudar upstream, restart Caddy (nao reload)',
 'Caddy reload mantem cache do upstream antigo em alguns casos. Para garantir, fazer docker restart omnichannel-mvp-caddy-1.',
 'Sintoma classico: site continua mostrando versao antiga apos deploy.',
 'Trocar upstream Caddy ou criar novo dominio.', 30),
('geral-doc-first', 'padroes-gerais', 'Documentar antes de implementar',
 'Antes de codar feature nao trivial: criar fase pending no roadmap-data.ts (status:pending, tasks done:false), apresentar plano ao usuario, so depois codar.',
 'Evita retrabalho e mantem roadmap como fonte de verdade para o agente.',
 'Tarefa com 3+ passos ou impacto em multiplas camadas.', 10),
('geral-agent-md', 'padroes-gerais', 'Atualizar AGENT.md ao alterar modulo',
 'Toda mudanca em modulo backend (ou layer/area significativa do front) reflete no AGENT.md correspondente: novos endpoints, novas tabelas, novos contratos.',
 'AGENT.md e a fonte que outros agentes leem para entender o modulo.',
 'PR que mexe em modulo.', 20),
('geral-sem-coauthor', 'padroes-gerais', 'Sem Co-Authored-By Claude em commits',
 'Commits nao devem ter Co-Authored-By: Claude. Atribuicao fica so com o desenvolvedor humano.',
 'Preferencia explicita do mantenedor.',
 'Toda criacao de commit.', 30),
('geral-local-first', 'padroes-gerais', 'Validar local antes de qualquer coisa',
 'Sempre rodar e testar local antes de propor commit ou deploy. UI changes precisam de browser test, nao so type-check.',
 'Type-check + test suite validam corretude de codigo, nao de feature.',
 'Sempre.', 40)
on conflict (source_id, account_id) do nothing;
