# Plano de Reestruturação — Plataforma SaaS Multi-Tenant

> Branch alvo: `refactor/multi-tenant-core` (a criar a partir de `migracao/nuxt`).
> `main` e `migracao/nuxt` continuam focadas no produto fila-de-atendimento atual. Este plano só vai a produção quando estiver completo (ou sobe em subdomínio dedicado).

---

## Contexto

A plataforma hoje é um modular monolith bem organizado em Go (`back/internal/modules/` com 15 módulos) servindo um produto único (fila de atendimento), com tenancy **flat** (`tenant → store → user`), roles **hardcoded** em código, permissões granulares já vivas em banco mas com catálogo fixo via migrations, e um Nuxt 4 SPA com menu **estático** filtrado por role.

A intenção é evoluir para uma plataforma SaaS multi-tenant **hierárquica** (Organization "agência" opcional + Account "cliente" + usuários), com módulos plugáveis (`queue`, `finance`, `tasks`, `omni`, `site`, `bio`, `e-commerce`, `contacts`), onde:

- Cada Account habilita só os módulos contratados.
- Cada Account tem seus próprios cargos (cargos-template do sistema, clonados e editáveis).
- Permissões vem de catálogo declarado por cada módulo.
- Módulos funcionam autônomos, mas integram quando dois estão habilitados no mesmo Account (ex: `finance` usa `contacts` se ativo, senão mantém entidade local).
- Outros módulos do usuário (financeiro, tasks notion-like, omnichannel WhatsApp) hoje vivem em repos separados e serão trazidos para este monorepo conforme as fases avançam.

### Decisões fechadas com o usuário (referência rápida)

1. **Topologia**: Monorepo modular monolith. 1 binário Go com módulos plugáveis. Preparado para extrair como microserviço no futuro.
2. **Hierarquia**: `Organization` (opcional) + `Account` (cliente). `accounts.organization_id` nullable suporta cliente direto e cliente-de-agência sem mudar schema.
3. **Banco**: 1 PostgreSQL compartilhado, **schemas separados por módulo** (`core`, `queue`, `finance`, `tasks`, `omni`, `contacts`, ...). FKs cross-schema só entre módulo satélite e `core`; entre satélites, integração via interface/Resolver e event bus.
4. **Migração**: branch nova `refactor/multi-tenant-core`, reescrita parcial (mantém código dos módulos atuais reaproveitáveis, reescreve só o core e o wiring).
5. **RBAC**: cargos-template do sistema (Owner, Admin, Operacional, Financeiro) que o Account clona e edita. Permissões = catálogo declarativo dos módulos (auto-registrado no boot).
6. **Compartilhamento de entidades**: módulo core opcional `contacts` é fonte de verdade quando habilitado; quando não, módulos consumidores caem em entidade local.
7. **Frontend**: Nuxt Layers (um layer por módulo). `app/` é o shell, `layers/<id>/` carrega pages/components/stores do módulo.

---

## A. Modelo de Domínio Core

### A.1 Terminologia

`Account` substitui `tenant`. Migração **não é rename** — `core.accounts` é tabela nova, `public.tenants` legada é mantida apenas durante a transição. FKs antigas (`stores.tenant_id` etc) passam a apontar para `core.accounts(id)` quando o módulo `queue` for movido (Fase 4).

### A.2 Schema SQL alvo (`core`)

Migration nova: [back/internal/platform/database/migrations/0100_core_schema.sql](back/internal/platform/database/migrations/0100_core_schema.sql)

Tabelas:

- `core.organizations` — agência opcional (`id`, `slug`, `name`, `is_active`).
- `core.accounts` — cliente do SaaS (`id`, `organization_id` nullable FK, `slug`, `name`, `plan_code`).
- `core.users` — identidade global (1 e-mail = 1 user). **Sem** `account_id`. Carrega `is_platform_admin`.
- `core.account_users` — membership user↔account (`is_active`, `invited_by_user_id`).
- `core.organization_users` — membership user↔organization (`org_role`: `agency_owner`/`agency_member`).
- `core.modules` — catálogo de módulos (`id`, `schema_name`, `is_core`, `requires_modules[]`, `optional_modules[]`).
- `core.account_modules` — módulos habilitados por Account (`enabled`, `config jsonb`).
- `core.permissions` — catálogo de permissões declaradas pelos módulos (`key`, `module_id`, `scope`).
- `core.role_templates` + `core.role_template_permissions` — templates de cargo declarados pelos módulos.
- `core.roles` (`account_id`, `cloned_from_template_id`, `code`, `is_default`, `is_locked`) + `core.role_permissions` — cargos efetivos da Account.
- `core.user_role_assignments` — atribuição user↔role na Account.
- `core.user_permission_overrides` — allow/deny por usuário (mantém modelo de [back/internal/modules/access/permissions.go](back/internal/modules/access/permissions.go)).
- `core.user_sessions` — sessões ativas do user (`id`, `user_id`, `revoked_at` nullable, `last_seen_at`, `user_agent`, `ip`, `created_at`). JWT carrega `sessionId`; middleware checa `revoked_at IS NULL`. Habilita logout/revogação granular.

### A.3 Login multi-account

JWT carrega `userId`, `sessionId`, `expiresAt` (**sem** `account_id`). `sessionId` viabiliza logout/revogação por sessão e auditoria — tabela `core.user_sessions` (`id`, `user_id`, `revoked_at`, `last_seen_at`, `user_agent`, `ip`) é checada no middleware; sessão revogada → 401, mesmo com JWT válido.

Frontend mantém `activeAccountId` em cookie e envia `X-Account-Id` em todo request. Middleware backend valida membership e injeta `Principal{UserID, AccountID, OrganizationID?, ...}`. Trocar de account é grátis (apenas atualiza cookie). Permite abrir 2 abas com accounts diferentes.

**Regra arquitetural inegociável** (registrar em `CONTRACT_FREEZE.md`): nenhum handler/repository aceita `account_id` vindo direto do request body/query. Todo `account_id` vem **exclusivamente** de `Principal.AccountID` (resolvido pelo middleware a partir do header). Reviewer rejeita PR que viole isso. Previne vazamento entre tenants.

### A.4 Visão de agência consolidada

Header opcional `X-Organization-Scope: <org-id>` em rotas que aceitarem. Service amplia o filtro de `account_id = principal.activeAccountId` para `account_id IN (select id from core.accounts where organization_id = principal.organizationId)`. Permissão `core.organization.consolidated_read` controla. Cada módulo declara se suporta org-scope (operacionais como `queue` provavelmente não suportam — agência não opera fila).

---

## B. Module Registry / Plug-in System

### B.1 Interface `Module` (peça central)

Pacote novo: [back/internal/platform/modules/](back/internal/platform/modules/)

```go
type Module interface {
    ID() string                              // "queue", "finance", "contacts"
    Schema() string                          // schema Postgres do módulo
    Metadata() ModuleMetadata                // label, descrição, dependências
    Permissions() []PermissionDef            // catálogo declarativo
    RoleTemplates() []RoleTemplate           // templates de cargo
    Migrations() []database.Migration        // migrations do schema próprio
    Build(deps Dependencies) (ModuleHandle, error)
}

type ModuleHandle interface {
    RegisterRoutes(r Router)
    RegisterEventHandlers(bus EventBus)
    Close() error
}
```

### B.2 Bootstrap reescrito

[back/internal/platform/app/app.go](back/internal/platform/app/app.go) passa de wiring manual para:

```go
registry.MustRegister(coreModule.New())
registry.MustRegister(contacts.New())     // core opcional
registry.MustRegister(queue.New())        // ex-operations + alerts + consultants + analytics + ...
// futuros: finance, tasks, omni, site, bio

registry.ApplyMigrations(ctx, pool)        // aplica core e cada schema de módulo
registry.SyncCatalog(ctx, pool)            // popula core.modules / core.permissions / core.role_templates
handles := registry.Build(deps)            // resolve dependências opcionais
for _, h := range handles { h.RegisterRoutes(...); h.RegisterEventHandlers(bus) }
```

### B.3 Account.modules controla rotas e menu

Middleware novo `accountModulesGuard` ([back/internal/platform/httpapi/account_guard.go](back/internal/platform/httpapi/account_guard.go)) lê `X-Account-Id`, consulta cache de `core.account_modules`, retorna `403 module_disabled` se rota for de módulo desativado. Cache invalidado por evento `account.modules.changed`.

### B.3.1 Regras inegociáveis do `SyncCatalog`

Sync que roda no boot toca `core.modules`, `core.permissions`, `core.role_templates` — nunca `core.roles` (que são da Account). Comportamento obrigatório:

1. **Cria** novas permissões/templates declarados pelos módulos.
2. **Atualiza** apenas `label`/`description` de keys existentes (mudanças cosméticas).
3. **Marca** keys removidas com `deprecated_at = now()`. **Nunca deleta automaticamente** — drop só via migration manual após validação.
4. **Nunca toca** em `core.roles` ou `core.role_permissions` da Account. Se um cliente removeu uma permissão de um role customizado, sync não restaura.
5. **Nunca toca** em `core.role_template_permissions` se o template já existir — só popula em template novo (templates são versionados pelo módulo).

Teste de regressão: snapshot de `core.permissions` antes/depois do boot em CI; diff só pode ter `+` ou flag `deprecated_at`, nunca `DELETE` real.

### B.4 Dependências opcionais

`Dependencies.Contacts contacts.Resolver` é registrado globalmente, mas o Resolver internamente checa se o módulo `contacts` está habilitado para o `accountId` do request — se não, devolve `ErrNotEnabled` e o módulo consumidor (ex: `finance`) cai no fallback (entidade local). Isso entrega o cenário "módulo conecta quando ambos existem".

### B.5 Endpoints de contexto (split lean + full)

Para evitar payload pesado quando user tem muitas accounts × muitas permissões:

- **`GET /v1/me/accounts`** (lean): lista de accounts com `id`, `name`, `organizationId`, `modules[]` (só os IDs habilitados). Sem permissões. Usado pelo `AccountSwitcher` e bootstrap inicial.
- **`GET /v1/me/context?accountId=<id>`** (full): contexto completo de **um** account — `roles[]`, `permissions[]` resolvidas, `user`, `organization`. Chamado quando o `activeAccountId` muda.
- **`POST /v1/me/active-account`**: atualiza cookie e dispara nova chamada de context.

Frontend cacheia `/me/accounts` por sessão; `/me/context` por `activeAccountId` com invalidação por evento WebSocket (`context.changed`).

---

## C. RBAC Dinâmico

### C.1 Seed de roles por account

`POST /v1/accounts` cria account, inicializa `account_modules` com defaults do plano, e para cada módulo habilitado **copia** seus `role_templates` para `core.roles` da Account (cópia editável). Role `core.owner` na account é `is_locked=true` (não pode ser deletado).

Habilitar módulo novo depois replica os templates daquele módulo, sem resetar customizações existentes.

### C.2 Clonagem e validação

Cliente clona role existente ou template. Ao salvar permissões, validador exige que cada `permission_key` esteja em `core.permissions` E que `permission.module_id` esteja em `account_modules` da account. Catálogo declarativo é a única fonte de verdade.

### C.3 Resolução de permissões

Reaproveita lógica de [back/internal/modules/access/permissions.go](back/internal/modules/access/permissions.go), agora tomando `accountID`:

```
permissões efetivas =
    UNION(role_permissions de todos os roles do user na account)
  + overrides allow ativos
  - overrides deny ativos
```

Cache `(userID, accountID) → []permKey` em memória com **TTL curto (2 minutos)** — **não** atrelado ao TTL do JWT. Invalidação imediata por eventos `role.permissions.changed`, `user.role.assignment.changed`, `user.override.changed`, `account.modules.changed`. Abstração `PermissionCache` permite trocar para Redis se virar microserviço.

Razão do TTL curto: JWT vive 12h; se permissão for removida (ex: usuário demitido perde `finance.invoices.write`), o cache não pode segurar a permissão velha por 12h. 2 min é teto de janela de exposição mesmo se o evento de invalidação falhar.

---

## D. Comunicação entre Módulos

### D.1 Manter interfaces para leitura síncrona

Padrão atual ([back/internal/platform/app/app.go](back/internal/platform/app/app.go) com adapters tipo `operations_store_scope_adapter.go`) já funciona bem para "Finance pergunta o nome do contato". Manter.

### D.2 Adicionar event bus in-process

Pacote novo: [back/internal/platform/events/bus.go](back/internal/platform/events/bus.go)

```go
type Event struct {
    ID, AccountID, Topic string
    Payload              map[string]any
    OccurredAt           time.Time
    CausationID          string  // detecta loops
    CorrelationID        string
}
type EventBus interface {
    Publish(ctx, e) error
    Subscribe(topic, handler) Subscription
}
```

Implementação inicial: goroutine pool com fila bounded por tópico. Persistência opcional em `core.event_outbox` para handlers críticos. Interface idêntica a NATS/RabbitMQ (troca trivial no futuro).

**Não introduzir broker externo agora.** Apenas a abstração.

Convenção de tópicos: `<module>.<entity>.<verb_past>` (ex: `queue.service_finished`, `finance.invoice_paid`). Reviewer recusa PRs com handler que publica evento do mesmo módulo (deve ser síncrono). Bus rejeita eventos com profundidade > 10.

---

## E. Frontend (Nuxt Layers)

### E.1 Estrutura de pastas

```
web/
  app/                              # SHELL
    pages/{auth,index.vue,account-select.vue}
    layouts/default.vue             # carrega sidebar dinâmica
    middleware/{auth.global.ts, account-required.global.ts, module-enabled.ts}
    plugins/module-registry.client.ts
    stores/{auth.ts, account.ts, modules.ts}
    nuxt.config.ts                  # extends dos layers

  layers/
    core/                           # account selector, PermissionGate, useNav, usePermission
    queue/                          # ex-fila-atendimento (operação, ranking, consultor, alertas, ERP, etc)
    finance/  tasks/  omni/  site/  bio/   # quando trazidos
```

### E.2 `extends` híbrido (recomendado)

`app/nuxt.config.ts` faz `extends: ['../layers/core', '../layers/queue', ...]` — todos os layers buildados estão no bundle. Runtime filtra pelo `useModules().enabledIds`:

- Itens fora dos módulos habilitados não aparecem no menu.
- Páginas de módulo declaram `definePageMeta({ middleware: 'module-enabled', module: 'finance' })` que redireciona se desabilitado.

Vantagens: build/deploy único; code-splitting do Nuxt já carrega chunks só na visita. Para venda assimétrica (cliente que paga só `queue`), variante de deploy futura lê `LAYERS_ENABLED` em build-time.

### E.3 Menu dinâmico via nav registry

Substitui [web/app/utils/sidebar-nav.ts](web/app/utils/sidebar-nav.ts) (estático). Cada layer expõe `nav.config.ts`:

```ts
export default {
  moduleId: "queue",
  sections: [{ id: "operacao", label: "Operação", items: [
    { id: "fila", label: "Fila", path: "/operacao", requires: ["queue.read"] },
    ...
  ]}]
}
```

Plugin [web/app/plugins/module-registry.client.ts](web/app/plugins/module-registry.client.ts) usa `import.meta.glob('../../layers/*/nav.config.ts')`, monta `useNavStore` que filtra por `accountStore.enabledModules` e por `usePermission().has()`.

### E.3.1 Convenção de nomenclatura por layer (anti-colisão)

Auto-imports do Nuxt mesclam `components/`, `composables/`, `stores/` de todos os layers — colisão silenciosa de nomes vira bug feio. Regra:

- **Components**: prefixo do moduleId em PascalCase. `QueueDashboard.vue`, `FinanceInvoiceList.vue`, `TasksBoard.vue`. `layers/core/components/` pode usar prefixo `Core` (ex: `CoreAccountSwitcher.vue`).
- **Composables**: prefixo `use<Module>...`. `useQueueContext()`, `useFinanceInvoices()`, `useTasksBoard()`. Genéricos do `core` ficam sem prefixo (`usePermission`, `useNav`, `useAuth`).
- **Stores Pinia**: id da store inclui o módulo. `defineStore('queue/context', ...)`, `defineStore('finance/invoices', ...)`. Arquivos podem ser curtos (`stores/context.ts`) já que vivem dentro de `layers/<id>/stores/`.
- **Middleware**: nomeado também com prefixo. `queue-store-required.ts`, `finance-account-billed.ts`. Globais ficam só em `app/middleware/`.
- **Pages**: roteamento por path já isola, mas evitar paths conflitantes (`layers/queue/pages/relatorios.vue` vs `layers/finance/pages/relatorios.vue` quebra). Prefixar paths quando possível: `/queue/operacao`, `/finance/invoices`, `/tasks/board`. Páginas legadas mantém path raiz durante transição.

CI pode rodar `nuxt build --analyze` e falhar se detectar nome de componente duplicado.

### E.4 Migração do código atual

| Atual | Destino |
|---|---|
| `pages/auth/`, `pages/index.vue`, `pages/perfil.vue` | `app/pages/` |
| `pages/operacao/`, `pages/consultor.vue`, `pages/ranking.vue`, `pages/alertas.vue`, `pages/dados.vue`, `pages/inteligencia.vue`, `pages/relatorios.vue`, `pages/feedback.vue`, `pages/erp.vue`, `pages/configuracoes.vue`, `pages/clientes.vue`, `pages/usuarios.vue`, `pages/multiloja.vue`, `pages/banco.vue`, `pages/campanhas.vue`, `pages/monitoramento.vue` | `layers/queue/pages/` |
| `pages/finance.vue`, `pages/tasks.vue`, `pages/omnichannel.vue`, `pages/tracking.vue`, `pages/site/`, `pages/team/`, `pages/tools/`, `pages/manage/` | placeholders agora; movem para `layers/<id>/` na Fase 6 |
| [web/app/stores/auth.ts](web/app/stores/auth.ts) | cinde em `app/stores/auth.ts` (sessão) + `app/stores/account.ts` (multi-account) + `layers/queue/stores/queue-context.ts` (`activeStoreId`, `storeScopeMode`) |
| [web/app/middleware/auth.global.ts](web/app/middleware/auth.global.ts) | `app/middleware/` (mantém global) |
| [web/app/domain/utils/permissions.ts](web/app/domain/utils/permissions.ts) | `layers/core/composables/usePermission.ts` |
| [web/app/utils/sidebar-nav.ts](web/app/utils/sidebar-nav.ts) | **deletar** após nav registry pronto |
| [web/app/utils/api-client.ts](web/app/utils/api-client.ts) | `layers/core/utils/` + injetar header `X-Account-Id` |

---

## F. Plano de Execução em Fases

### Fase 0 — Fundação (1–2 semanas)

- Criar branch `refactor/multi-tenant-core` a partir de `migracao/nuxt`.
- `docs/CONTRACT_FREEZE.md` lista interfaces atuais que **não podem** quebrar até Fase 4: `auth.Service`, `auth.Principal`, `tenants.Service`, `stores.Service`, `access.Service`, `realtime.Service`.
- `docs/SCHEMA_TARGET.md` com diagrama dos schemas Postgres alvo.
- Feature-flag `CORE_V2_ENABLED` no backend para gatear código novo.
- **Verificável**: projeto compila e roda igual ao main.

### Fase 1 — Schema core novo (2–3 semanas)

- Migration `0100_core_schema.sql` cria seção A.2 completa.
- Job de seed: `public.tenants` → `core.accounts` (mesmo `id`); cria `account_users` para users existentes.
- Endpoints novos sob flag: `GET /v2/me/context`, `POST /v2/accounts/:id/switch`.
- Endpoint legado `GET /v1/me/context` permanece intacto.
- **Verificável**: login antigo funciona; login com flag retorna `accounts[]` e `activeAccountId`.

### Fase 2 — Module Registry e refactor do bootstrap (2 semanas)

- Pacote `back/internal/platform/modules/` com `Registry`, `Module`, `Dependencies`, `EventBus`.
- [back/internal/platform/app/app.go](back/internal/platform/app/app.go) reescrito; módulos atuais embrulhados em adapters `Module` finos (não reescrevem o módulo).
- `core.modules`, `core.permissions`, `core.role_templates` populados pelo `SyncCatalog` no boot.
- `accountModulesGuard` ativo (todos os módulos atuais marcados como habilitados para todos os accounts existentes — não quebra nada).
- **Verificável**: rotas atuais respondem igual, agora gated pelo guard.

### Fase 3 — RBAC dinâmico (2 semanas)

- Service `core.rbac`: `CloneTemplateToAccount`, `CreateRole`, `UpdateRolePermissions`, `AssignRoleToUser`.
- Endpoint `/v1/accounts/:id/roles` CRUD.
- Migração de dados: roles atuais (Owner, Manager, Director, etc) viram `core.roles` por account.
- `Principal.Permissions` resolvido pelo path novo; antigo `access.Service.ResolveUserPermissions` continua como fallback.
- **Verificável**: UI de roles permite clonar template e ajustar permissões; mudança reflete no login do user.

### Fase 4 — Mover módulo `queue` (quebrada em 4A → 4D, ~4–5 semanas)

Esta fase concentra mais risco do plano (banco + domínio + rotas + frontend). Quebrada em sub-fases independentes para reduzir blast-radius e permitir reverter parcialmente.

#### Fase 4A — Fundação do schema `queue` (1 semana)

- Cria schema `queue` vazio + migrations base em `back/internal/modules/queue/migrations/*.sql`.
- Migra **só** tabelas estáveis e de baixa volatilidade: `stores`, `consultants`, `settings`, `catalog`. FKs passam a apontar para `core.accounts(id)`.
- Módulos atuais (`operations`, `alerts`, etc) **continuam** lendo dessas tabelas via views compatíveis em `public.*` (apontando para `queue.*`) durante a transição. Zero quebra.
- **Verificável**: produto roda igual; queries SELECT batem nas views novas; testes existentes passam.

#### Fase 4B — Domínio operacional principal (1–2 semanas)

- Migra `operations`, `feedback` para `queue.*` (tabelas core do dia-a-dia).
- Reescreve `back/internal/modules/operations/` integrando ao módulo `queue` (subpacote `queue/operations/`).
- Endpoints `/v1/operations/*` mantêm shape (compatibilidade do front).
- **Verificável**: fluxo golden de operação (entrada → pausa → atendimento → fim) idêntico em staging.

#### Fase 4C — Analytics, alertas, ERP (1 semana)

- Migra `alerts`, `analytics`, `reports`, `erp` para `queue.*`.
- Cada um vira subpacote `queue/<nome>/`.
- ERP é o mais delicado (FTP ingestion + projeções) — testar rebuild de projeções com base no schema novo.
- **Verificável**: dashboards de relatório, alertas e sincronização ERP funcionam idênticos.

#### Fase 4D — Frontend layer `queue` (1 semana, paralelo a 4C)

- Cria `web/layers/queue/` com `nav.config.ts`.
- Move pages/stores listadas em E.4 para o layer.
- Shell `app/` fica minimal.
- **Verificável**: trocar account no `AccountSwitcher` recarrega menu; rota `/operacao` continua funcionando dentro do layer.

> Cada sub-fase entrega um deploy reversível. Se 4B quebrar, 4A continua válido. Se 4D atrasar, 4A/4B/4C já estão em produção.

### Fase 5 — Frontend layers + menu dinâmico (1–2 semanas, paralelo à Fase 4)

- `app/plugins/module-registry.client.ts` lendo `nav.config.ts` dos layers.
- `layers/core/` com `AccountSwitcher`, `PermissionGate`, `usePermission`, `useNav`.
- [web/app/utils/sidebar-nav.ts](web/app/utils/sidebar-nav.ts) deletado.
- [web/app/components/dashboard/DashboardSidebarNav.vue](web/app/components/dashboard/DashboardSidebarNav.vue) reescrito para consumir `useNavStore`.
- **Verificável**: trocar account no AccountSwitcher recarrega menu; desabilitar módulo no banco esconde itens.

### Fase 6 — Orquestração dos módulos satélites

Fase guarda-chuva para a entrada dos módulos. Depois da Fase 10, a execução deixa de ser uma lista genérica e vira trilhas próprias nas Fases 11-20. A primeira trilha agora é o Theme Studio, porque os módulos importados dependem dos tokens/temas do front de referência.

Para cada módulo:

1. Criar `back/internal/modules/<id>/` com `Module` impl.
2. Schema próprio + migrations.
3. Layer `web/layers/<id>/`.
4. Declarar permissões + role templates.
5. Declarar dependências opcionais (ex: `finance` usa `contacts` se ativo, senão fallback local).
6. Habilitar no account-piloto via `core.account_modules`.
7. Validar: menu aparece quando habilitado, rota/API bloqueiam quando desabilitado.

**Ordem inicial criada a partir do inventário da Fase 10:**

1. Fase 11 — `theme-studio`
2. Fase 12 — `tasks`
3. Fase 13 — `omni`
4. Fase 14 — `finance`
5. Fase 15 — `contacts/admin`
6. Fase 16 — `site`
7. Fase 17 — `indicators`
8. Fase 18 — `tools`
9. Fase 19 — `team`
10. Fase 20 — `bio`

---

### Fase 7 — Otimização de performance (1 semana, BLOQUEANTE para Fase 8)

> **Contexto**: após as Fases 0–5 o painel ficou lento — login lento, navegação lenta, logout chega a travar. Diagnóstico mostra que o problema é estrutural: cada request autenticado faz 6–8 queries no banco (sem cache), `/v2/me/context` faz mais 5–6, e o logout tem um bug que dispara loop de middleware. Fase 8 (criar mais módulos) **só faz sentido** depois disto — adicionar código em cima de fundamento lento amplifica o problema.
>
> **Estratégia**: trabalhar em camadas progressivas. Começar com queries + índices (sem cache), medir o ganho, só então decidir se TTL em memória vale o custo. Redis fica para o final (pré-produção).

#### Diagnóstico (medições realizadas)

| Sintoma | Causa raiz | Local |
|---|---|---|
| Login lento (~900ms+) | 3 requests em série: `/v1/auth/login` → `/v1/me/context` → `syncRuntimeAccess` | [web/app/stores/auth.ts:309-335](web/app/stores/auth.ts#L309-L335) |
| Página lenta | Auth middleware faz **6-8 queries por request** — `users.FindByID` tem correlated subqueries N+1 (4 SELECTs por role); `ResolveUserPermissions` faz +2 queries. **Sem cache.** | [back/internal/modules/auth/middleware.go](back/internal/modules/auth/middleware.go), [back/internal/modules/users/store_postgres.go:101-146](back/internal/modules/users/store_postgres.go#L101-L146) |
| `/v2/me/context` pesado | Query UNION de 3-vias (`role_permissions` + overrides allow - overrides deny) sem índices específicos. Sem cache. | [back/internal/modules/core/rbac_repository.go:ListPermissionsForUser](back/internal/modules/core/rbac_repository.go) |
| **Logout trava / bug** | `auth.logout()` só limpa state local; `navigateTo('/auth/login')` dispara `auth.global.ts` que chama `ensureSession()` novamente. Sem guard para rotas `/auth/*`. Sem endpoint backend de logout. | [web/app/stores/auth.ts:553-556](web/app/stores/auth.ts#L553-L556), [web/app/middleware/auth.global.ts:24](web/app/middleware/auth.global.ts#L24) |
| Stores duplicadas | `web/app/stores/auth.ts` (legado) + `web/layers/core/stores/account.ts` (novo) coexistem; potencialmente fazem fetches paralelos para `/v1/me/context` E `/v2/me/accounts` + `/v2/me/context` | dois arquivos acima |

**O que NÃO é problema** (já validado): bundle frontend, compat views Postgres, SyncCatalog no boot (já é batched via UPSERT), AccountModulesGuard (cache de 60s funciona).

#### Fase 7A — Otimização de queries + índices (2-3 dias, **começar aqui**)

Objetivo: reduzir queries por request sem introduzir cache ainda. Medir antes/depois.

- **Migration 0107**: criar índices que faltam:
  - `core.user_role_assignments(user_id, account_id)`
  - `core.role_permissions(role_id)`
  - `core.user_permission_overrides(user_id, account_id) WHERE revoked_at IS NULL`
  - `core.account_users(user_id, account_id) WHERE is_active = true`
- **Eliminar N+1 em `users.FindByID`**: substituir correlated subqueries por 1 JOIN único em [back/internal/modules/users/store_postgres.go:101-146](back/internal/modules/users/store_postgres.go#L101-L146). Esperado: 4 queries → 1.
- **Consolidar `ResolveUserContext`** em [back/internal/modules/core/rbac_repository.go](back/internal/modules/core/rbac_repository.go): unificar `ListRolesForUser` + `ListPermissionsForUser` em uma única CTE (`WITH user_roles AS ... SELECT roles, permissions FROM ...`). Esperado: 2 queries → 1.
- **Verificável**: `EXPLAIN ANALYZE` em cada query do hot-path mostra uso dos índices novos. Login dev em staging: medir antes (baseline atual) e depois.

#### Fase 7B — Fix logout (1 dia, **alta prioridade**)

- **Backend**: criar endpoint `POST /v1/auth/logout` em [back/internal/modules/auth/](back/internal/modules/auth/) que marca `core.user_sessions.revoked_at = now()` da sessão atual + publica evento `user.session.revoked` no event bus. AGENT.md do módulo atualizado.
- **Backend**: middleware de auth resolve o Principal a partir do `sessionId` e **inclui o check `revoked_at IS NULL` no mesmo lookup** — não é query separada. Crítico: o lookup do Principal vira a unidade que será cacheada em 7D, então `Principal{UserID, AccountID, Permissions[], SessionRevokedAt}` é cacheado **junto**. Sem cache, esse lookup já existe (não adiciona query nova); com cache, vira 0 queries em hit + invalidação por evento de revogação. Fazer naïve (cache só de permissões + query separada de `user_sessions` por request) **anula o ganho** da 7A.
- **Frontend**: `web/app/stores/auth.ts:logout()` chama `/v1/auth/logout` antes de `clearSession()`. Trata falha de rede como sucesso (token local invalidado de qualquer forma).
- **Frontend**: `web/app/middleware/auth.global.ts` adiciona guard `if (to.path.startsWith('/auth/')) return;` no topo, antes de qualquer fetch. Mata o loop.
- **Verificável**: logout completa em < 200ms; recarregar `/auth/login` não dispara `ensureSession()`; sessão revogada no banco bloqueia requests novos com o mesmo token **imediatamente** (evento dispara invalidação do cache, não espera TTL expirar).

#### Fase 7C — Paralelizar e consolidar contexto (1-2 dias)

- **Frontend**: paralelizar com `Promise.all([/v2/me/accounts, /v2/me/context])` em [web/layers/core/stores/account.ts](web/layers/core/stores/account.ts). Eliminar a cascata atual.
- **Frontend**: eliminar duplicação `stores/auth.ts` + `layers/core/stores/account.ts`. Manter apenas `core/account.ts` como single source. Deletar `syncRuntimeAccess` / `hydrateRuntimeStoreContext` que faziam fetches redundantes.
- **Backend (opcional)**: avaliar endpoint combinado `GET /v2/me/bootstrap` que retorne `{ user, accounts[], activeContext }` numa só round-trip (substitui 2 chamadas separadas).
- **Verificável**: login completa em < 500ms; só 1 request adicional após `/v1/auth/login`.

#### Fase 7D — Medir e decidir cache (após 7A-7C)

Depois das otimizações estruturais, medir novamente. Se ainda houver lentidão perceptível:

- **TTL em memória** (sync.Map ou ristretto): cache `(sessionId) → Principal` com TTL 2-5min. A unidade cacheada **inclui** estado da sessão (resolvido em 7B junto com o lookup do user) — não há query separada de `user_sessions` em request com cache hit.
- **Abstração `PrincipalCache`** (interface mínima: `Get`, `Set`, `Invalidate(userID, accountID)`, `InvalidateAll()`) para futura troca por Redis sem alterar middleware.
- **Invalidação por eventos é REQUISITO, não opcional**. Sem invalidação reativa, TTL de 2-5min vira janela de exposição: usuário demitido continua com `finance.invoices.write` por minutos; módulo desabilitado continua acessível; sessão revogada (logout) continua válida. Eventos mínimos que **devem** invalidar:

| Evento | Quem publica | O que invalida |
|---|---|---|
| `user.session.revoked` | `auth.Logout` + `auth.RevokeAllSessions` | entrada exata da sessão |
| `user.role.assignment.changed` | `core.rbac.AssignRoleToUser` | todos os Principals do `(userID, accountID)` afetado |
| `role.permissions.changed` | `core.rbac.UpdateRolePermissions` | todos os Principals do `accountID` cujo role mudou |
| `user.permission.override.changed` | endpoint de override allow/deny | entradas do `(userID, accountID)` |
| `account.modules.changed` | endpoint de habilitar/desabilitar módulo | todos os Principals do `accountID` |

- **Teste obrigatório**: integração que (1) loga user A, (2) confirma permissão X em cache hit, (3) admin remove X via endpoint, (4) próximo request de user A não tem mais X — **sem** esperar TTL. Mesmo teste para logout (revogar sessão → próximo request 401).
- **Verificável**: requests autenticados em hot-path fazem 0 queries de auth quando cache hit; mudança de permissão/role/módulo reflete em < 1s no próximo request do user afetado.

#### Fase 7E — Redis (futuro, pré-produção)

Só quando subir produção full com múltiplas instâncias. Troca o backend do `PrincipalCache` sem alterar interface.

> **Critério de saída da Fase 7**: login < 500ms; navegação entre páginas sem latência perceptível; logout < 200ms sem bugs. Só então iniciar Fase 8.

---

### Fase 8 — Split CRM + Queue (2-3 semanas)

> **Contexto**: o produto atual mistura dois domínios: **operação de fila** (atendimento, alertas, ranking, relatórios da operação) e **dados de cliente** (ERP + dashboards de vendas + catálogo de produtos). Eles têm ciclos de vida e users diferentes — separá-los agora, antes de trazer os outros módulos da Fase 6, evita que o `queue` cresça misturado.
>
> **Regra de dependência**: CRM **não depende** de queue. Queue **opcionalmente** consome CRM (se habilitado, lê dados de cliente/produto; se não, mantém entidade local). Implementa o padrão B.4 do plano.

#### Escopo definido com o usuário

**Vai para CRM agora:**
- ERP completo (ingest FTP + tabelas `erp_*` — hoje em `queue.*`, vão para `crm.*`)
- Página `/crm` (dashboards de vendas por cliente/consultor)
- Catalog (busca de produtos — hoje consome ERP, vira cliente nativo do CRM)

**Fica fora do CRM por enquanto** (decisão explícita):
- `/clientes` (CRUD de clientes) — permanece em queue até decidirmos quando promover a master data CRM
- `/finance` (comissões/bonificações) — módulo separado quando chegar do outro projeto

**Fica em QUEUE:**
- operations, alerts, feedback, consultants, settings, realtime, analytics operacional, reports da operação, ranking, dados, inteligencia, configuracoes, multiloja, campanhas

#### Fase 8A — Backend: módulo CRM (1 semana)

- **Migration 0108**: criar schema `crm` e mover tabelas:
  - De `queue.*`: `erp_sync_runs`, `erp_sync_files`, `erp_item_raw`, `erp_customer_raw`, `erp_employee_raw`, `erp_order_raw`, `erp_item_current` → `crm.*`
  - FKs cross-schema: `crm.erp_*.tenant_id → public.tenants(id)` (mantém compat); `store_id → queue.stores(id)` (cross-module, mas só queue precisa de stores)
  - Views compat em `public.*` apontando para `crm.*` durante transição (mesmo padrão das migrations 0104-0106)
- **Reorganizar código**:
  - `back/internal/modules/erp/` → `back/internal/modules/crm/erp/`
  - `back/internal/modules/catalog/` → `back/internal/modules/crm/catalog/`
  - Criar `back/internal/modules/crm/dashboard/` para endpoints `/v1/crm/*`
- **Module impl**: `back/internal/modules/crm/module.go` implementa interface `Module` da Fase 2:
  - `ID() = "crm"`, `Schema() = "crm"`, `Permissions() = []{crm.erp.sync, crm.dashboard.read, crm.catalog.read, ...}`
  - `RoleTemplates()` = `crm-admin`, `crm-viewer`
  - Sem `requires_modules` (CRM é autônomo)
- **Resolver pattern**: `crm.Resolver` interface registrada em `Dependencies` para que queue possa consumir opcionalmente. Quando `core.account_modules` não tem `crm` habilitado, retorna `ErrNotEnabled`.
- **Verificável**: `go build ./...` passa; com `crm` habilitado na account, endpoint `/v1/crm/dashboard` responde; com `crm` desabilitado, mesmo endpoint retorna 403 via `accountModulesGuard`.

#### Fase 8B — Backend: módulo QUEUE consolidado (paralelo a 8A, 3-4 dias)

> Continuação do que ficou adiado da Fase 4 (tasks `module-rewrite`, `subpackages`).

- **Reorganizar código** (não move tabelas — só código Go; tabelas já estão em `queue.*` desde migrations 0104-0106):
  - `back/internal/modules/operations/` → `back/internal/modules/queue/operations/`
  - `back/internal/modules/alerts/` → `back/internal/modules/queue/alerts/`
  - `back/internal/modules/analytics/` → `back/internal/modules/queue/analytics/`
  - `back/internal/modules/reports/` → `back/internal/modules/queue/reports/`
  - `back/internal/modules/feedback/` → `back/internal/modules/queue/feedback/`
  - `back/internal/modules/consultants/` → `back/internal/modules/queue/consultants/`
  - `back/internal/modules/settings/` → `back/internal/modules/queue/settings/`
- **Module impl**: `back/internal/modules/queue/module.go` consolida:
  - `ID() = "queue"`, `Schema() = "queue"`, `Permissions()` = união das permissões dos submódulos
  - `Dependencies()` declara `crm` como **opcional**
  - Construtor recebe `deps.CRM crm.Resolver` (pode ser `nil` ou `NotEnabled`)
- **Adapter de catalog**: catalog atual lê ERP raw. No queue, criar `queue/catalog_adapter.go` que tenta `deps.CRM.SearchProducts(...)`; se `ErrNotEnabled`, fallback para busca local em entidade simplificada (criar `queue.products_local` se necessário).
- **Verificável**: endpoints `/v1/operations/*`, `/v1/alerts/*`, `/v1/reports/*` mantêm shape; flow golden de operação idêntico; produtos aparecem via CRM quando habilitado, fallback local quando não.

#### Fase 8C — Frontend: layer CRM (3 dias)

- Criar `web/layers/crm/` com `nuxt.config.ts` + `nav.config.ts`:
  ```ts
  // web/layers/crm/nav.config.ts
  export default {
    moduleId: "crm",
    sections: [
      { id: "crm-indicators", label: "CRM", items: [
        { id: "crm-dashboard", label: "Dashboard CRM", icon: "chart", path: "/crm" }
      ]},
      { id: "crm-data", label: "Dados externos", items: [
        { id: "erp", label: "ERP", icon: "boxes", path: "/erp" }
      ]}
    ]
  };
  ```
- Mover páginas:
  - `web/app/pages/crm.vue` (ou `pages/crm/index.vue`) → `web/layers/crm/pages/index.vue`
  - `web/app/pages/erp.vue` → `web/layers/crm/pages/erp.vue` (ou `erp/index.vue` se virar subseção)
- Componentes/composables prefixados (regra E.3.1): `CrmDashboard.vue`, `useCrmInvoices`, `defineStore('crm/dashboard', ...)`.
- Atualizar [web/nuxt.config.ts](web/nuxt.config.ts): `extends: ["../layers/core", "../layers/queue", "../layers/crm"]`.
- Remover `crm` e `erp` de [web/layers/queue/nav.config.ts](web/layers/queue/nav.config.ts) (item `manage-menu > erp` e seção `indicators`).
- **Verificável**: ao desabilitar `crm` em `core.account_modules`, itens de menu somem e rota `/crm` retorna 403 (via guard).

#### Fase 8D — Documentação + cleanup (1 dia)

- Atualizar `AGENT.md` dos módulos tocados (regra do usuário): `crm/AGENT.md`, `queue/AGENT.md`, `erp/AGENT.md` (deprecated/movido), `catalog/AGENT.md` (idem).
- Adicionar Fase 7 e Fase 8 em [web/app/components/roadmap/roadmap-data.ts](web/app/components/roadmap/roadmap-data.ts).
- Atualizar `docs/CONTRACT_FREEZE.md` com a interface `crm.Resolver` que queue depende opcionalmente.
- **Verificável**: AGENT.md de cada módulo afetado descreve o estado novo; roadmap mostra fases 7 e 8.

> **Saída da Fase 8**: dois módulos independentes (`crm` autônomo, `queue` consome `crm` opcionalmente), com nav próprio por layer, prontos para receber finance/tasks/omni na Fase 6.

---

### Fase 9 — UX de loading / feedback visual (3-5 dias, paralela à Fase 7)

> **Contexto**: mesmo depois das otimizações da Fase 7, a primeira carga de cada página vai ter delay natural (hidratação da SPA + fetch inicial). Hoje o painel não dá nenhum feedback visual durante esse intervalo — usuário fica com tela em branco ou navegação parada e acha que travou. Isso é tão crítico quanto a performance bruta: o que importa é o **percebido**.
>
> **Princípio**: nunca deixar o usuário olhando para nada. Loading sempre presente, com 3 níveis de fidelidade (overlay global, skeleton da página, spinner local em ação).

#### Fase 9A — Padrões visuais base (1-2 dias)

- **Componente `CoreLoadingOverlay.vue`** em [web/layers/core/components/](web/layers/core/components/) — barra de progresso fina no topo da janela + leve fade do conteúdo. Acionada por:
  - Navegação entre rotas (Nuxt route transitions).
  - Bootstrap inicial (auth + accounts + context).
- **Componente `CoreSkeleton.vue`** — blocos cinza com shimmer animation. Variantes:
  - `<CoreSkeleton variant="card" />`, `variant="table-row"`, `variant="text"`, `variant="avatar"`.
  - Usado dentro de cada página enquanto dados do fetch chegam.
- **Composable `useCoreLoading()`** — controla estado global de loading com referência contada (push/pop). Acionado pelo `api-client.ts` interceptando requests longos (> 200ms).
- **Critério visual**: nunca mostrar página em branco por mais de 200ms; sempre ter algo (overlay, skeleton, ou conteúdo final).

#### Fase 9B — Aplicar nas páginas críticas (2-3 dias)

Páginas onde o impacto de feedback é maior (e onde o usuário mais sentiu lentidão):

- **Login / bootstrap** (`/auth/login` → home): overlay aparece imediatamente após submit, sumiu quando o context terminou de carregar.
- **Dashboard inicial** (`/`): skeleton dos cards de métricas enquanto fetcheia.
- **Operação** (`/operacao`): skeleton da grid de stores + fila enquanto realtime conecta.
- **Tabelas grandes** (clientes, usuários, relatórios): skeleton rows enquanto carrega + paginação com loading inline.
- **Troca de account** (`AccountSwitcher`): overlay durante o `/v2/me/context` da nova account.

#### Fase 9C — Estados vazios e de erro (1 dia)

- **Componente `CoreEmptyState.vue`** — ícone + título + descrição + ação opcional. Usado quando fetch volta com lista vazia.
- **Componente `CoreErrorState.vue`** — quando fetch falha. Botão de retry. Mensagem amigável (não vazar `error.stack`).
- **Padronizar uso em todas as workspaces** — substitui mensagens hardcoded de "Sem dados" / "Erro ao carregar".

> **Critério de saída da Fase 9**: nenhuma página fica em branco em qualquer transição. Usuário sempre vê algo se mexendo (overlay, skeleton, ou shimmer). Tempo até primeiro pixel renderizado < 300ms mesmo na primeira carga.

---

### Fase 10 — Inventário do front de referência + estratégia de design system (antes da Fase 6)

> **Contexto**: o outro projeto do usuário (que tem os módulos finance/tasks/omni) já foi trazido para `web-reference/` e contém um design system mais maduro, incluindo uma página de temas (`/admin/themes`), tokens globais, componentes de formulário/tabela e páginas de módulo.
>
> **Decisão atualizada**: o front-end atual **permanece como está por enquanto**. Não vamos substituir selects, tabelas, modais ou páginas já existentes só porque existe um componente parecido no front de referência. A Fase 10 agora serve para mapear o design system novo e preparar a estratégia de importação dos módulos. As páginas novas que vierem do outro projeto entram com o visual delas; depois da migração completa decidimos quais páginas atuais continuam, quais saem e quais recebem update de design/componentes.

#### Fase 10A — Inventário do front de referência

- `web-reference/` já existe e fica fora do build do Nuxt (`.gitignore`), usado como fonte de leitura/análise.
- Assistant lê e produz `docs/COMPONENT_INVENTORY.md` com:
  - Componentes reutilizáveis, path, descrição, props/eventos públicos e dependências.
  - Páginas/módulos encontrados (`finance`, `tasks`, `omni`, clientes, usuários, temas, etc.) e destino provável em `web/layers/<id>/`.
  - Itens do design system: tokens CSS, composables de tema, componentes base, dependências Nuxt UI/Tailwind, página de Theme Studio.
  - Classificação: `design-system`, `module-page`, `module-component`, `legacy-overlap` ou `candidate-core`.
- **Verificável**: inventário revisado com o usuário antes de portar qualquer componente.

#### Fase 10B — Design system de referência

- Mapear o design system do `web-reference/`:
  - `app/assets/css/tokens.css`
  - `app/composables/useOmniTheme.ts`
  - `app/composables/useThemeStudio.ts`
  - `app/pages/admin/themes.vue`
  - `app/components/theme/**`
- Definir como esse sistema entra no projeto atual sem quebrar o front existente:
  - Quais tokens globais entram agora.
  - Quais tokens ficam encapsulados nos novos layers.
  - Quais componentes de tema viram `Core*` no futuro.
  - Como evitar conflito entre tokens atuais (`web/app/assets/styles/*.css`) e os tokens do `web-reference/`.
- **Regra da fase**: adaptar tokens/variantes ao design system trazido do front de referência; não ao design system antigo do projeto atual.
- **Verificável**: `docs/COMPONENT_INVENTORY.md` inclui seção "Design system e temas" com decisão de integração.

#### Fase 10C — Estratégia para páginas atuais vs páginas novas

- Páginas atuais do produto fila-atendimento continuam usando os componentes e visual atuais enquanto a migração dos módulos não termina.
- Não substituir agora o uso atual em páginas existentes (`clientes`, `usuarios`, tabelas, selects, modais, etc.).
- Quando uma página do módulo novo substituir uma página atual (ex: clientes talvez venha do geral), a página nova entra com o visual dela e a página antiga fica marcada como candidata a remoção/depreciação.
- Depois que finance/tasks/omni e módulos relacionados estiverem importados, revisar:
  - Páginas que permanecem no visual atual.
  - Páginas que serão removidas.
  - Páginas que serão atualizadas para o design system novo.
- **Verificável**: roadmap/lista de decisão por página antes de qualquer substituição visual ampla.

#### Fase 10D — Migração incremental de módulos

- Componentes específicos de cada módulo migram junto com o módulo correspondente na Fase 6 (`finance`, `tasks`, `omni`, `site`, `bio`, etc.).
- Componentes realmente compartilháveis só vão para `web/layers/core/components/` com prefixo `Core` depois de:
  - aparecerem em mais de um módulo, ou
  - serem necessários para o shell/design system comum.
- Manter a regra E.3.1 para evitar colisões de auto-import:
  - `Core*` apenas para base compartilhada.
  - `Finance*`, `Tasks*`, `Omni*`, etc. dentro dos seus layers.
- **Verificável**: cada PR de módulo documenta quais componentes vieram do `web-reference/`, quais ficaram específicos e quais viraram candidatos a `Core`.

**Atualização 2026-05-12 — Nuxt UI e ordem dos módulos**

- Decisão confirmada: os módulos/páginas importados do front de referência vão usar Nuxt UI como base visual.
- A documentação local para LLMs já existe em `web-reference/Nuxt-ui-llms/llms.txt` e `web-reference/Nuxt-ui-llms/llms-full.txt`.
- Finance não será o primeiro módulo importado; `/finance` permanece como placeholder até sua vez.
- Antes de seguir com módulos ativos, a Fase 11 deve estabilizar Theme Studio/tokens para evitar telas importadas com contraste e estados visuais quebrados.
- Fase 10 encerrada em 2026-05-12: a migração de componentes específicos foi transferida para a Fase 6, junto com cada módulo importado.

> **Saída da Fase 10**: inventário completo do front de referência, estratégia de design system/temas definida, e regra clara de migração: preservar o front atual agora, importar páginas novas com o visual delas, e só depois consolidar o que continua, sai ou vira componente Core.

---

### Fase 11 — Design System / Theme Studio (3-5 dias) — concluída em 2026-05-12

> **Fonte da Fase 10**: `web-reference/app/pages/admin/themes.vue`, `web-reference/app/composables/useOmniTheme.ts`, `web-reference/app/composables/useThemeStudio.ts`, `web-reference/app/components/theme/**`, `AdminPageHeader.vue` e tokens do front de referência.

- [x] Trazer `useOmniTheme.ts` para o layer core/design-system com inicialização global no app.
- [x] Trazer `useThemeStudio.ts`, `components/theme/**` e a página `/admin/themes` para uma rota dev-only (`/themes`).
- [x] Unificar tokens do `web-reference` com `omni-design-system.css` sem quebrar o shell atual.
- [x] Fazer `AdminPageHeader`, dashboard/sidebar/header e páginas importadas consumirem os tokens corretos.
- [x] Validar `light`, `dark`, `apple` e `custom` em `/themes` e `/tasks`.
- **Verificável**: Theme Studio aplica e persiste tema; trocar tema altera tokens globais; `/tasks` fica legível em todos os temas; rota/menu ficam dev-only.

### Fase 12 — Tasks Orchestrator / Notion-like (1-2 semanas)

> **Fonte da Fase 10**: `web-reference/app/pages/admin/tasks.vue`, `web-reference/app/composables/useTasksWorkspace.ts`, `web-reference/app/types/tasks.ts`, além de `OmniDataTable` e `OmniSelectMenuInput` como base visual.

> **Conceito atualizado**: o nome inicial continua `Tasks`, mas a tela deve se comportar como um orquestrador notion-like. Uma pagina pode representar tarefas, campanhas, producao, aprovacoes ou outro fluxo usando o mesmo template configuravel.

- [x] Criar `web/layers/tasks/` com pagina, composable, types, store local e componentes importados do web-reference.
- [x] Portar a base visual de `admin/tasks.vue`, `OmniDataTable` e `OmniSelectMenuInput`, integrada ao Theme Studio.
- [x] Habilitar `/tasks` no menu/rota para acesso dev/admin inicial.
- [x] Documentar o escopo em `docs/TASKS_ORCHESTRATOR_PHASE12.md`.
- [x] Trocar modelo interno de projeto/tarefa para `page/template/view/field/item`, mantendo `Tasks` como primeira pagina.
- [x] Permitir criar mais de uma pagina/base usando o mesmo template.
- [x] Configurar views board/tabela: agrupamento, ordenacao, filtros, campos visiveis e densidade.
- [x] Colunas configuraveis: renomear, colorir, reordenar por drag, adicionar/remover e mapear itens ao excluir.
- [x] Editar dados direto no card e na tabela com `OmniSelectMenuInput`/inputs inline; abrir modal somente no clique neutro do card.
- [x] Botao de criar item por coluna, menu de edicao da coluna e movimentacao de cards/colunas.
- [x] Configurar layout do card: campos exibidos, ordem, labels, badges e cores.
- [x] Configurar layout do modal por secoes e campos, implementando o modal depois do board/tabela.
- [x] Fase T0.5: quebrar `tasks.vue` de ~2955 para 832 linhas totais, extraindo `useTasksPageContext` e sub-componentes.
- [x] Fase T1: criar `0108_tasks_schema_foundation.sql` e `back/internal/modules/tasks/` com schema `tasks.*`, Module Registry, RBAC declarativo e endpoints REST/tracking basicos.
- [x] Fase T2: plugar realtime para tasks/presence/notifications via publisher real, com WS autenticado, PresenceStore TTL 30s, rate limit 30 events/s e docs atualizados.
- [x] Fase T3: criar modulo notifications (migration 0109, InAppAdapter, stubs email/WhatsApp/push e triggers em tasks).
- [x] Fase T4: registry de resolvers cross-module (crm/erp/operations) e endpoint `relations:expand` com cache 60s.
- [ ] Fase T5: trocar localStorage por API Go/Pinia store real.
- **Verificavel**: backend passa em `go test ./...`; `/tasks` segue front-first ate a T5; roadmap detalhado em `web/app/components/roadmap/roadmap-data.ts` e `docs/tasks-orquestrador-plano.html`.

### Fase 13 — Módulo Omni / Omnichannel (2-4 semanas)

> **Fonte da Fase 10**: `web-reference/app/pages/admin/omnichannel/*`, `components/omnichannel/**`, `composables/omnichannel/**`, dependências `socket.io-client` e `emoji-mart`.

- Criar `back/internal/modules/omni/` com schema `omni.*`: canais, conversas, mensagens, contatos vinculados, auditoria e eventos.
- Adicionar dependências do front somente nesta fase.
- Portar páginas: `index`, `inbox`, `operacao`, `auditoria` e `docs` conforme decisão de produto.
- Portar `OmnichannelInboxModule.vue` e subcomponentes de chat/composer/anexos/audio/reactions/sessão.
- Integrar realtime ao backend Go, removendo dependência de BFF/mock do front de referência.
- **Verificável**: inbox abre, lista conversas, envia mensagem de teste, recebe atualização realtime, registra auditoria e respeita `account_modules`.

### Fase 14 — Módulo Finance (2-3 semanas)

> **Fonte da Fase 10**: `web-reference/app/pages/admin/finance.vue`, `FinanceLineCard.vue`, `FinanceRecurringGroupCard.vue`, `OmniMoneyInput.vue`.

- Criar `back/internal/modules/finance/` com schema `finance.*`: lançamentos, categorias, recorrências, ajustes e histórico.
- Criar `web/layers/finance/` e substituir o placeholder `/finance` pela página portada.
- Portar `FinanceLineCard`, `FinanceRecurringGroupCard` e `OmniMoneyInput` inicialmente dentro do layer finance.
- Integrar com `contacts` quando habilitado; usar entidade local quando `contacts` estiver desligado.
- Declarar permissões como `finance.read`, `finance.write`, `finance.recurring.manage` e role templates.
- **Verificável**: criar lançamento, efetivar recorrência, ajustar valor e consultar histórico via API Go.

### Fase 15 — Módulo Contacts / Admin (2-3 semanas)

> **Fonte da Fase 10**: `admin/manage/clientes.vue`, `admin/manage/users.vue`, `admin/manage/modulos.vue`, `components/manager/clients/**`, `useClientsManager.ts`, `useUsersManager.ts`.

- Decidir se `contacts` substitui `/clientes` atual ou entra primeiro como módulo opcional paralelo.
- Criar `back/internal/modules/contacts/` com `Resolver` consumível por `finance`, `omni`, `site` e `queue`.
- Portar páginas de gestão apenas quando for seguro substituir ou conviver com os CRUDs atuais.
- Mapear `admin/manage/modulos.vue` para gestão futura de `core.account_modules`.
- Manter `/clientes` e `/usuarios` atuais até decisão explícita de troca visual/funcional.
- **Verificável**: `contacts.Resolver` funciona quando habilitado; consumidores fazem fallback quando desabilitado; nenhuma página legada é substituída por acidente.

### Fase 16 — Módulo Site (1-2 semanas)

> **Fonte da Fase 10**: `admin/site/produtos.vue`, `admin/site/leads.vue`.

- Criar `back/internal/modules/site/` com produtos publicados, leads, configurações e permissões.
- Criar `web/layers/site/` e portar páginas de produtos/leads.
- Decidir se leads sincronizam com `contacts` quando o módulo estiver habilitado.
- Proteger rotas com `module-enabled` e registrar menu próprio no `nav.config.ts`.
- **Verificável**: cadastrar produto, alternar visibilidade no site e consultar lead via API Go.

### Fase 17 — Módulo Indicators (2-3 semanas)

> **Fonte da Fase 10**: `admin/indicadores/index.vue`, `admin/indicadores/configuracoes.vue`, `components/indicators/**`, `useIndicatorsWorkspace*`.

- Decidir destino de domínio: módulo próprio `indicators`, parte de `analytics`, ou parte de `crm`.
- Criar schema/APIs para templates, avaliações, governança, evidências, filtros e exportações.
- Portar páginas de operação e configuração.
- Trocar mocks/live do `web-reference` por dados reais do backend.
- **Verificável**: criar avaliação, configurar template, filtrar período e exportar sem dados mockados.

### Fase 18 — Módulo Tools (1-2 semanas)

> **Fonte da Fase 10**: `admin/tools/qr-code.vue`, `admin/tools/encurtador-link.vue`, `admin/tools/scripts.vue`, `useShortLinksManager.ts`, tipos de short-links.

- Decidir se `tools` fica como módulo único ou se vira módulos menores (`qrcodes`, `short-links`, `scripts`).
- Criar APIs Go para QR Code, encurtador de link e scripts, evitando duplicar BFF.
- Portar as páginas aprovadas para `web/layers/tools/`.
- Declarar permissões por ferramenta.
- **Verificável**: gerar QR, criar link curto e listar scripts com persistência real.

### Fase 19 — Módulo Team (1-2 semanas)

> **Fonte da Fase 10**: `admin/team/treinamento.vue`, `admin/team/candidatos.vue`.

- Confirmar se `team` entra no produto ou fica fora do escopo imediato.
- Modelar candidatos, treinamentos, anexos e estados de processo quando aprovado.
- Definir estratégia para anexos/CVs antes de subir a tela de candidatos.
- Portar páginas para `web/layers/team/` só após a decisão de produto.
- **Verificável**: criar candidato/treinamento e validar permissões por account.

### Fase 20 — Módulo Bio (descoberta)

> **Fonte da Fase 10**: o módulo `bio` existe no plano original, mas não apareceu como página concreta no `web-reference` analisado.

- Localizar a fonte real do módulo Bio ou confirmar que será criado do zero.
- Definir escopo: links, perfil público, temas, analytics e integrações com `site`/`contacts`.
- Criar `back/internal/modules/bio/` e `web/layers/bio/` somente após escopo ou fonte visual validada.
- **Verificável**: Bio só vira implementação depois de descoberta validada; nada entra como placeholder solto.

---

## G. Arquivos Críticos

### Criar

- [back/internal/platform/modules/registry.go](back/internal/platform/modules/registry.go) — Registry, applyMigrations cross-schema, syncCatalog.
- [back/internal/platform/modules/module.go](back/internal/platform/modules/module.go) — interfaces `Module`, `ModuleHandle`, `Dependencies`, `PermissionDef`, `RoleTemplate`.
- [back/internal/platform/events/bus.go](back/internal/platform/events/bus.go) — event bus in-process.
- [back/internal/modules/core/](back/internal/modules/core/) — módulo `core`: accounts, organizations, users globais, rbac, account-modules.
- [back/internal/platform/database/migrations/0100_core_schema.sql](back/internal/platform/database/migrations/0100_core_schema.sql) — schema `core` completo.
- [back/internal/platform/httpapi/account_guard.go](back/internal/platform/httpapi/account_guard.go) — middleware `accountModulesGuard`.
- [web/layers/core/nuxt.config.ts](web/layers/core/nuxt.config.ts) + `composables/{useNav.ts, usePermission.ts}` + `components/{AccountSwitcher.vue, PermissionGate.vue}`.
- [web/layers/queue/nuxt.config.ts](web/layers/queue/nuxt.config.ts) + `nav.config.ts`.
- [web/app/plugins/module-registry.client.ts](web/app/plugins/module-registry.client.ts) — monta menu via `import.meta.glob` dos `nav.config.ts`.
- [web/app/stores/account.ts](web/app/stores/account.ts) — `accounts[]`, `activeAccountId`, `switchAccount`.

### Modificar

- [back/internal/platform/app/app.go](back/internal/platform/app/app.go) — wiring manual → `registry.Build(deps)`.
- [back/internal/modules/auth/model.go](back/internal/modules/auth/model.go) + [back/internal/modules/auth/service.go](back/internal/modules/auth/service.go) — `Principal.TenantID` → `Principal.AccountID`; JWT só carrega `userId`; `User` perde `TenantID`/`StoreIDs` (passam a `core.account_users` + `queue.user_stores`).
- [back/internal/platform/app/context_http.go](back/internal/platform/app/context_http.go) — `/v1/me/context` retorna shape novo (B.5).
- [back/internal/modules/access/service.go](back/internal/modules/access/service.go) — `ResolveUserPermissions` recebe `accountID`, lê `core.role_permissions` via union.
- [web/app/stores/auth.ts](web/app/stores/auth.ts) — cisão em `auth.ts` (sessão) + `account.ts` (multi-account); api-client injeta `X-Account-Id`.
- [web/nuxt.config.ts](web/nuxt.config.ts) — `extends` dos layers.
- [back/internal/modules/auth/AGENT.md](back/internal/modules/auth/AGENT.md), [back/internal/modules/access/AGENT.md](back/internal/modules/access/AGENT.md), [back/internal/modules/tenants/AGENT.md](back/internal/modules/tenants/AGENT.md) — refletir mudanças (regra do usuário: AGENT.md sempre acompanha alteração no módulo).

### Remover / Deprecar

- [web/app/utils/sidebar-nav.ts](web/app/utils/sidebar-nav.ts) — substituído por nav registry.
- `roleCatalog` hardcoded em [back/internal/modules/auth/roles.go](back/internal/modules/auth/roles.go) — vira seed em `core.role_templates` declarado pelo módulo `core`.

---

## H. Riscos e Trade-offs

1. **FKs cross-schema travam extração futura**. Mitigar: FKs cross-schema **só** entre módulo satélite e `core`. Entre satélites, integração via Resolver/event bus, IDs como UUID livre. Documentar em `CONTRACT_FREEZE.md` desde o dia 1.

2. **Catálogo declarativo perde rastreabilidade histórica**. Mitigar: `SyncCatalog` detecta keys removidas, marca `deprecated_at` em vez de dropar, emite warning. Permite migration manual `core.migrate_permission(old, new)`. CI snapshot do catálogo.

3. **Header `X-Account-Id` é spoofable se middleware falhar**. Mitigar: `accountModulesGuard` é middleware **global** no chain raiz. `Principal.AccountID` vem só do middleware (nunca do request body/query/params). **Regra inegociável** documentada em `CONTRACT_FREEZE.md`: nenhum repository/service aceita `account_id` como parâmetro vindo direto do handler — sempre via `Principal`. Reviewer rejeita PR que viole. Teste de integração obrigatório por módulo: "user A não vê dados de account onde não tem membership".

4. **Event bus in-process esconde dependências cíclicas**. Mitigar: `Event.causationId` + `correlationId`; bus rejeita profundidade > 10; convenção `<module>.<entity>.<verb_past>`; reviewer recusa handler que publica evento do mesmo módulo.

5. **Bundle frontend infla com todos os layers**. Mitigar: code-splitting do Nuxt já isola chunks; mensurar antes de otimizar; alerta se `Initial JS` passar de 500KB gzipped; deploy variante por `LAYERS_ENABLED` postergável até cliente real exigir.

6. **Auto-import de Nuxt Layers cria colisão silenciosa**. Mitigar: convenção de prefixos por moduleId (seção E.3.1) obrigatória; CI roda `nuxt build --analyze` e falha em duplicatas; PR review rejeita componente sem prefixo do layer.

7. **Sessão JWT longa não revoga**. Mitigar: `sessionId` no JWT + tabela `core.user_sessions` consultada no middleware (com cache curto); endpoint `POST /v1/auth/revoke-all-sessions` para o user; logout faz `revoked_at = now()` da sessão atual.

---

## I. Verificação End-to-End

Em cada fase:

- **Backend**: `go test ./...` em `back/` passa; novo teste de integração por fase (Fase 1: troca de account; Fase 2: account_modules guard bloqueia módulo desativado; Fase 3: clonagem de role); container Docker sobe com `docker-compose up`.
- **Frontend**: `npm run build` em `web/` passa; testar manualmente fluxos golden:
  - Login → account selector aparece se user pertence a múltiplas accounts.
  - Trocar account no `AccountSwitcher` recarrega menu.
  - Permissões removidas em uma role somem do menu sem refresh duro.
  - Desabilitar módulo no banco esconde menu e bloqueia rota direta.
- **Smoke pós-Fase 4**: produto fila-atendimento atual funciona idêntico no schema `queue` novo (snapshot manual de operação → entrada → pausa → atendimento → fim → relatório).
- **Smoke pós-Fase 6** (cada módulo): habilitar `finance` no account-piloto → menu mostra `finance` → criar registro `finance` → consultar via API → desabilitar → menu/rota somem.

Não há produção até a Fase 4 estar validada com paridade total ao produto atual; se quiser subir antes, usar subdomínio dedicado e accounts-piloto. `main` e `migracao/nuxt` permanecem servindo o produto vigente durante todo o trabalho.
