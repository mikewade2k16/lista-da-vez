# AGENT_RULES.md

Regras canonicas que todo agente/IA deve ler antes de iniciar qualquer tarefa neste projeto.

Fonte de verdade: este arquivo. UI vive em `/roadmap` > aba "Regras" e exporta este arquivo. Quando a Frente B (modulo Go `roadmap`) for entregue, este `.md` passa a ser regenerado automaticamente a partir do banco.

Para entender o que esta pronto / em beta / pendente, consulte `/roadmap` > aba "Modulos".

---

## Frontend

### Componentes reutilizaveis acima de tudo
Sempre que houver repeticao de markup ou logica visual, extrair em componente proprio em `web/app/components/` ou na layer adequada. Workspaces nao podem virar arquivos gigantes; quebrar em cards/secoes/listas.

- **Por que:** Evita duplicacao e drift visual entre paginas. Facilita aplicar mudanca em um lugar so.
- **Aplica quando:** Qualquer feature nova ou refactor que adicione UI.

### Classes semanticas BEM-like
Sempre usar nomes semanticos no estilo `.nome-componente__elemento--modificador`. Nao usar utility classes inline ou IDs para estilizacao.

- **Por que:** Permite leitura rapida do escopo de cada estilo e evita colisao global.
- **Aplica quando:** Estilizacao de qualquer componente novo.

### Sem emojis em UI nem em codigo
Nao usar emojis em labels, mensagens de UI, codigo, comentarios ou commits, salvo se o usuario pedir explicitamente.

- **Por que:** Mantem consistencia visual e profissional do produto.
- **Aplica quando:** Sempre.

### Esconder pagina nao pronta via hidden no menu
Quando um modulo/pagina nao esta pronto, usar `hidden: true` em `web/app/utils/sidebar-nav.ts` E em `web/layers/queue/nav.config.ts`. Para itens em beta, usar `beta: true` (renderiza badge).

- **Por que:** Evita que usuario navegue para pagina quebrada. Beta deixa explicito que a feature pode mudar.
- **Aplica quando:** Adicionar/remover modulo do menu lateral.

---

## Backend

### Padrao de modulo Go
Cada modulo em `back/internal/modules/<nome>/` tem: `model.go` (tipos), `store_postgres.go` (persistencia), `service.go` (regras), `http.go` (handlers), `AGENT.md` (documentacao). Modulos plugaveis se registram via Module Registry quando `CORE_V2_ENABLED`.

- **Por que:** Consistencia entre modulos facilita onboarding e troca de agente.
- **Aplica quando:** Criar novo modulo backend.

### IDs como string, nunca uuid externo
Usar `string` para IDs no Go; nao importar pacote `uuid` externo. Casts e geracao ficam centralizados em `internal/platform/ids/`.

- **Por que:** Reduz dependencia externa e facilita refatoracao do esquema de IDs.
- **Aplica quando:** Qualquer struct nova com ID.

### Scan de campos NULL com `*string`
Para colunas nullable, declarar `*string` (ou `sql.NullString` se preferir) no Scan; nunca `string` puro.

- **Por que:** Evita panic em scan de NULL.
- **Aplica quando:** Implementar `store_postgres.go`.

### Permissoes vivem no banco (RBAC dinamico)
Nao hardcoded permission names em codigo Go. Permissoes vivem em `core.permissions` + `core.role_permissions`; service consulta via Module Registry.

- **Por que:** Permite agencia customizar role sem deploy.
- **Aplica quando:** Implementar checagem de permissao em handler ou service.

---

## Banco

### Migration idempotente (IF NOT EXISTS)
Toda migration usa `IF NOT EXISTS` / `CREATE OR REPLACE`. Numerar sequencialmente em `back/internal/platform/database/migrations/####_nome.sql`. Nunca renumerar migration ja aplicada.

- **Por que:** Migrations falhas no meio precisam poder ser reaplicadas sem dropar dados.
- **Aplica quando:** Criar migration nova.

### Schema-per-modulo + account_id em todas as tabelas tenant-scoped
Schemas: `core`, `queue`, `tasks`, `alerts`, `settings`, `roadmap`. Toda tabela tenant-scoped tem `account_id` NOT NULL com FK para `core.accounts`. Public schema pode ter VIEWS sobre tabelas dos schemas.

- **Por que:** Multi-tenancy com isolamento logico e queries por schema mais previsiveis.
- **Aplica quando:** Criar tabela nova.

### Mover tabela para schema: criar view publica
Quando mover tabela de `public.*` para `schema.*`, criar `CREATE OR REPLACE VIEW public.<tabela> AS SELECT * FROM schema.<tabela>` para manter compat com codigo legado.

- **Por que:** Evita quebrar queries antigas que ainda apontam para public.*.
- **Aplica quando:** Refactor de schema.

---

## Linguagens

### Go 1.26
Backend usa Go 1.26. Aproveitar generics, `max`/`min` builtins, `slices`/`maps` stdlib.

- **Por que:** Versao alinhada com infra de CI e Docker.
- **Aplica quando:** Backend.

### Vue 3 + Nuxt 4 + Pinia
Frontend usa Vue 3 (Composition API + `<script setup>`), Nuxt 4 (com layers em `web/layers/*`), Pinia para state. Tipos TS sempre que possivel.

- **Por que:** Stack escolhida pelo time; layers permitem isolar dominios.
- **Aplica quando:** Frontend.

### TypeScript strict
Codigo TS deve passar em `vue-tsc --noEmit`. Evitar `any`. Preferir tipos explicitos em props e composables.

- **Por que:** Pega bug em build time, nao em prod.
- **Aplica quando:** Qualquer codigo TS/Vue.

---

## Deploy

### VPS Hostinger com Caddy + Docker Compose
Deploy em VPS `85.31.62.33`, user `deploy`. Caddy reverse proxy em `/opt/omnichannel/Caddyfile`. Cada projeto roda em `/home/deploy/<projeto>` com `docker-compose.prod.yml`. Aliases por projeto na network proxy.

- **Por que:** Isolamento por projeto + um Caddy gerencia todos os dominios.
- **Aplica quando:** Deploy ou troubleshooting de prod.

### Feature flag em `.env.production` E `docker-compose.prod.yml`
Variaveis novas precisam de duas adicoes: `.env.production` (na VPS) E `docker-compose.prod.yml` na secao `environment`. Sem a segunda, o container nao recebe a variavel.

- **Por que:** Compose nao propaga automaticamente `.env` file inteiro; precisa de declaracao explicita.
- **Aplica quando:** Adicionar variavel de ambiente nova.

### Apos mudar upstream, restart Caddy (nao reload)
Caddy reload mantem cache do upstream antigo em alguns casos. Para garantir, fazer `docker restart omnichannel-mvp-caddy-1`.

- **Por que:** Sintoma classico: site continua mostrando versao antiga apos deploy.
- **Aplica quando:** Trocar upstream Caddy ou criar novo dominio.

---

## Padroes Gerais

### Documentar antes de implementar
Antes de codar feature nao trivial: criar fase pending no `roadmap-data.ts` (status:'pending', tasks done:false), apresentar plano ao usuario, so depois codar.

- **Por que:** Evita retrabalho e mantem roadmap como fonte de verdade para o agente.
- **Aplica quando:** Tarefa com 3+ passos ou impacto em multiplas camadas.

### Atualizar AGENT.md ao alterar modulo
Toda mudanca em modulo backend (ou layer/area significativa do front) reflete no `AGENT.md` correspondente: novos endpoints, novas tabelas, novos contratos.

- **Por que:** AGENT.md e a fonte que outros agentes leem para entender o modulo.
- **Aplica quando:** PR que mexe em modulo.

### Sem Co-Authored-By Claude em commits
Commits nao devem ter `Co-Authored-By: Claude`. Atribuicao fica so com o desenvolvedor humano.

- **Por que:** Preferencia explicita do mantenedor.
- **Aplica quando:** Toda criacao de commit.

### Validar local antes de qualquer coisa
Sempre rodar e testar local antes de propor commit ou deploy. UI changes precisam de browser test, nao so type-check.

- **Por que:** Type-check + test suite validam corretude de codigo, nao de feature.
- **Aplica quando:** Sempre.
