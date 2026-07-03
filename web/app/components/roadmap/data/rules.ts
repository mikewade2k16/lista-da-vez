import type { RoadmapRule } from "./types";

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
