# AGENT_RULES.md

Regras canonicas que todo agente/IA deve ler antes de iniciar qualquer tarefa neste projeto.

Fonte de verdade: este arquivo. UI vive em `/roadmap` > aba "Regras" e exporta este arquivo.
Princípios de engenharia aprofundados (pilares segurança/performance/UX, registro de falhas): [docs/ENGINEERING_PRINCIPLES.md](docs/ENGINEERING_PRINCIPLES.md). Quando a Frente B (modulo Go `roadmap`) for entregue, este `.md` passa a ser regenerado automaticamente a partir do banco.

Para entender o que esta pronto / em beta / pendente, consulte `/roadmap` > aba "Modulos".

---

## Legado, mocks e fonte da verdade (PRIORIDADE MAXIMA)

Objetivo final: **uma unica fonte de verdade no banco real**. Nada de tabela/codigo legado paralelo, front mock, dado so em localStorage ou qualquer coisa que nao persista no banco pode ficar escondido como se estivesse pronto.

### Nunca deixar legado/mock para tras — resolver o quanto antes
- Se uma feature so fica "pronta" removendo o legado, ela **NAO esta pronta** enquanto o legado existir. Tabelas legadas paralelas (ex.: `user_tenant_roles`/`user_store_roles`/`user_platform_roles` vs `core.account_users` + `core.user_role_assignments`), front mock, dado so em localStorage, BFF mock, qualquer coisa que nao seja banco real → devem ser **ELIMINADOS**, nao mantidos vivos.
- Manter DOIS sistemas em sync (ex.: gravar papel no legado E no core) e' **band-aid temporario** e PRECISA estar marcado para remocao, nunca tratado como solucao final.

### Sempre AVISAR, DOCUMENTAR e MOSTRAR no front (so admin) o que e' legado/mock
Ao tocar em qualquer modulo e encontrar legado / mock / localStorage / nao-persistido / qualquer coisa que nao seja banco real, e' **OBRIGATORIO**:
1. **Avisar o usuario na hora** — dizer explicitamente "isso aqui ainda e legado/mock, nao esta no banco real, nao esta pronto".
2. **Documentar** — no `AGENT.md` do modulo E num registro central [docs/LEGADO.md](docs/LEGADO.md) (criar se nao existir): o que e', por que e' legado, qual o alvo, status de remocao.
3. **Mostrar no front, SO para `platform_admin`** — um marcador visivel (badge/aviso tipo "MOCK", "LEGADO", "localStorage", "nao persiste") na propria tela, para ninguem desenvolver achando que esta pronto quando e' mock/localStorage/legado.

- **Por que:** ja se perdeu tempo desenvolvendo sobre coisas que pareciam prontas mas eram mock/localStorage/legado (BFF Nitro, session-simulation, etc.). Marcador visivel + documentacao + aviso evita repetir.
- **Aplica quando:** SEMPRE que encontrar ou criar dependencia legada/mock — mesmo que temporaria.

### Modelo-alvo de usuario (exemplo concreto do principio)
Um usuario = **UMA linha em `core.users`** (fonte da verdade), sem tabela legada paralela. Config/opcoes **especificas por modulo** NAO viram tabela legada nem coluna espalhada: vivem em jsonb por modulo — ex.: tabela `core.user_module_settings (user_id, module_id, config jsonb)` (ou coluna `module_settings jsonb` em core.users). Cada modulo (tela) renderiza a **projecao/opcoes dele** sobre o MESMO usuario:
- `/operacao/usuarios` = usuarios daquele cliente no modulo Fila, com as opcoes especificas da Fila.
- `/manage/users` = visao global de identidade (admin).
- outro modulo = outras opcoes especificas daquele modulo.

Papeis/permissoes migram 100% para `core.*` (`core.account_users` + `core.user_role_assignments` + `core.role_permissions`), eliminando `user_tenant_roles`/`user_store_roles`/`user_platform_roles`. O auth deve resolver papel a partir do `core.*`, nao do legado.

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

## Seguranca multi-tenant e otimizacao (PRIORIDADE MAXIMA)

> Detalhe + inventario + gaps: [docs/ENGINEERING_PRINCIPLES.md §10](docs/ENGINEERING_PRINCIPLES.md). Regra de ouro: **um usuario NUNCA ve dado de outra account/loja**; nenhum payload/parametro forjado contorna isso.

### Todo escopo (tenant/store/account) e validado contra o Principal — nunca confiado do client
`tenantId`/`storeId`/`accountId` vindo de query/body e SEMPRE filtro DENTRO do permitido, validado por `resolveTenantScope`/`resolveStoreScope`/membership (`core.account_users`). Pedido fora do escopo → `ErrForbidden`. Handler novo que aceita um desses IDs SEM validar = bug de seguranca.

- **Por que:** Sem a validacao no service, o client passa o id de outro tenant e vaza dado.
- **Aplica quando:** Qualquer handler/service que receba um id de escopo. Defesa em profundidade: a query do repo TAMBEM filtra por `tenant_id`.

### Erro uniforme: recurso fora do escopo retorna 404, nao 403
Nao revelar a existencia de recurso de outro tenant. Fora do escopo → `404 not_found` (nao `403`).

- **Por que:** `403` vs `404` diferentes vazam que o recurso existe (enumeration).

### Pedir so o necessario (otimizacao + UX de resposta imediata)
- Front NAO dispara request que a role nao consome (gatear fetch por permissao ANTES, espelhando o back). Sem 403 de ruido no bootstrap.
- Sem N+1: agregar por `WHERE id = ANY($1)`, nunca 1 query por item em loop.
- Lista grande → paginacao (cursor para as que crescem); projecao lean (so os campos que a tela usa), nao o objeto inteiro.
- Carregar primeiro o above-the-fold; detalhe (memberships, stores) so ao abrir o modal.

- **Por que:** Payload menor + menos round-trips = UI que responde na hora.
- **Aplica quando:** Qualquer endpoint de listagem ou fetch no boot/montagem de tela.

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
- **ANTES de transformar tabela-COM-DADOS em view sobre outra tabela:** comparar os DADOS (nao so as colunas) das duas. Se houver divergencia (ex.: `password_hash` diferente para o mesmo id), a view vai servir o lado errado e pode trancar usuarios. Reconciliar os dados primeiro (a fonte VIVA vence) e so entao trocar.

### NUNCA mudar senha / rodar seed que sobreescreve dados de usuario sem permissao explicita
NUNCA alterar `password_hash`, nem rodar migration/seed/bootstrap/view-swap que sobreescreva senhas, perfis ou outros dados de usuario existentes, sem permissao MUITO explicita do usuario para AQUELE comando especifico. Mesmo "autorizado a rodar a sequencia", isso NAO inclui sobrescrever senha.

- **Por que:** seed/bootstrap/troca de fonte que reescreve senha TRANCA o usuario para fora. Aconteceu em 2026-06-05: o view-swap `public.users`->`core.users` passou a servir o `password_hash` stale do `core.users` (congelado desde o seed 0101) em vez do hash vivo de `public.users`; o admin ficou sem login ("Email ou senha invalidos"). Restaurado do `users_backup` (`update core.users set password_hash = b.password_hash from users_backup b where b.id=c.id`).
- **Aplica quando:** QUALQUER comando que escreva em dados EXISTENTES (UPDATE/DELETE em users, seeds de senha `0018`/`0033`, bootstrap_owner, view-swap sobre tabela com dados). Operacao destrutiva de dados exige: (1) backup antes, (2) checagem de divergencia, (3) confirmacao explicita do que muda — e senha so com pedido direto.

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

### Oferecer paralelismo com agentes ao planejar
Sempre que um plano tiver 2+ partes independentes (front + back, testes + impl, U3 + U4), **oferecer ao usuario a opcao de rodar em paralelo com multiplos agentes** (ex.: Codex × Codex, Codex × Claude). Nao assumir sequencial por padrao.

- **Compensa:** trabalho mecanico/repetitivo, tarefas sem dependencia entre si, preservar contexto do agente principal em sessao longa.
- **Nao compensa:** keystones de design onde spec ≈ solucao (o prompt custa quase tanto quanto fazer direto); tarefa onde B depende do resultado de A.
- **Como oferecer:** ao final do plano, listar quais partes sao independentes, sugerir divisao de agentes e o tradeoff custo×velocidade.

- **Por que:** usuario pode querer velocidade (paralelo) ou economia (sequencial); sem a oferta ele nao tem a escolha.
- **Aplica quando:** qualquer plano com 3+ passos ou que envolva front + back + docs simultaneamente.

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
