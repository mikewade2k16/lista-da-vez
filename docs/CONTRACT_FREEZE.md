# Contract Freeze — Reestruturação Multi-Tenant

Branch: `refactor/multi-tenant-core`
Status: ativo até a conclusão da Fase 4 (mover módulo `queue`).
Plano de referência: `~/.claude/plans/preciso-que-analise-nosso-ancient-orbit.md`.

Este documento lista as **interfaces, contratos e regras arquiteturais** que NÃO podem ser quebradas durante o trabalho da reestruturação. Quebras intencionais exigem PR dedicado, atualização deste doc e aprovação explícita.

---

## 1. Interfaces Go que não podem quebrar (até Fase 4)

A Fase 4 (mover módulo `queue` para schema próprio + reescrever wiring) é a primeira janela autorizada para alterar essas assinaturas. Antes disso, qualquer mudança aqui exige justificativa.

### 1.1 `auth`

Pacote: [back/internal/modules/auth/model.go](../back/internal/modules/auth/model.go)

| Símbolo | Por que está congelado |
|---|---|
| `auth.Principal` (struct + campos `UserID`, `Role`, `TenantID`, `StoreIDs`, `Permissions`, `PermissionsResolved`, `ExpiresAt`) | Todo middleware e service consome diretamente. Renomear `TenantID → AccountID` é mudança da Fase 1; até lá, manter campo. |
| `auth.User`, `auth.UserView` | Persistido em JWT/sessão. |
| `auth.TokenManager` interface (`Issue`, `Parse`) | Abstração da emissão/parsing JWT. |
| `auth.PermissionResolver` interface (`ResolveUserPermissions`) | Consumida por `access` e middleware. |
| `auth.Role` constantes (`platform_admin`, `owner`, `director`, `marketing`, `manager`, `consultant`, `store_terminal`) | Frontend espera essas strings em `permissionKeys`. RBAC dinâmico (Fase 3) substitui — até lá, congelado. |
| `auth.Service` (login, invite, password reset, change password) | Endpoints `/v1/auth/*` consumidos pelo front em produção. |

### 1.2 `tenants`

Pacote: `back/internal/modules/tenants/`

| Símbolo | Motivo |
|---|---|
| `tenants.Service` interface pública | Reescrita só na Fase 1 (passa a virar `core.accounts`). Endpoints `/v1/tenants/*` precisam continuar respondendo. |
| Tabela `public.tenants` (schema atual) | Não dropar até Fase 4. Migration de dados (`tenants → core.accounts`) preserva por job de seed. |

### 1.3 `stores`

Pacote: `back/internal/modules/stores/`

| Símbolo | Motivo |
|---|---|
| `stores.Service` interface pública | Endpoints `/v1/stores/*` em produção. |
| `stores.StoreScopeProvider` interface | Consumida por `operations`, `catalog`, `analytics`, `reports`, `realtime`. |

### 1.4 `access`

Pacote: `back/internal/modules/access/`

| Símbolo | Motivo |
|---|---|
| `access.Service` (`ResolveUserPermissions`, `EffectivePermissionKeys`) | Backbone do RBAC. Reescrita interna OK; assinatura externa congelada até Fase 3. |
| `access.AccessControl` interface | Consumida por `operations`, `alerts`. |
| Tabelas `access_permissions`, `access_role_permissions`, `user_access_overrides` | Migração para `core.permissions` + `core.role_permissions` + `core.user_permission_overrides` na Fase 3 com cópia de dados. |

### 1.5 `realtime`

Pacote: `back/internal/modules/realtime/`

| Símbolo | Motivo |
|---|---|
| `realtime.Service` interface | WebSocket em produção. |
| `realtime.ContextPublisher`, `realtime.EventPublisher` interfaces | Acopladas a `auth`, `access`, `alerts`, `operations`, `users`. |

---

## 2. Regras arquiteturais inegociáveis

Estas regras valem da Fase 0 ao fim do trabalho. Reviewer rejeita PR que viole.

### 2.1 `account_id` (ex-`tenant_id`) vem só do `Principal`

Nenhum handler, service ou repository aceita `account_id` (ou `tenant_id` enquanto durar) vindo do request body, query string ou path param. Sempre é resolvido pelo middleware a partir do header `X-Account-Id` (após Fase 1) ou do JWT (antes da Fase 1) e injetado no `Principal`.

**Por quê**: previne escalonamento horizontal entre tenants — se o atacante autenticado conseguir mandar `account_id` arbitrário em um body, vê dados de outro cliente.

**Como aplicar**:
- Repositories recebem `accountID string` como parâmetro, mas a origem dele é sempre `principal.TenantID` (ou `principal.AccountID` após Fase 1).
- Services aceitam `Principal` (ou `AccessContext`), nunca `accountID` solto vindo de fora do middleware.
- Endpoints administrativos (ex: platform admin agindo sobre outro account) usam path param `:accountId` mas validam que `principal.IsPlatformAdmin == true` antes.

### 2.2 FKs cross-schema só entre módulo satélite e `core`

Quando os schemas Postgres por módulo existirem (Fase 4 em diante):

- `queue.*`, `finance.*`, `tasks.*`, `omni.*`, `contacts.*` PODEM ter FKs apontando para `core.accounts(id)`, `core.users(id)`, `core.organizations(id)`.
- `queue.*` NÃO tem FK para `finance.*`. `finance.*` NÃO tem FK para `tasks.*`. Etc.
- Integração entre módulos satélites: via interface Resolver (in-process) ou event bus.
- IDs cross-módulo (ex: `finance.invoices.contact_id` quando módulo `contacts` está habilitado) são `uuid` livres, sem FK.

**Por quê**: extrair um módulo para microserviço no futuro fica trivial — não há FK travando o drop do schema.

### 2.3 Catálogo de permissões é declarativo, não migration

Permissões e role templates são declarados em código pelo módulo (interface `Module.Permissions()` / `Module.RoleTemplates()`) e sincronizados em `core.permissions` / `core.role_templates` no boot via `SyncCatalog`.

`SyncCatalog`:
- Cria entradas novas.
- Atualiza apenas `label`/`description` de existentes.
- Marca removidas com `deprecated_at = now()`. **Nunca deleta.**
- **Nunca toca** em `core.roles` / `core.role_permissions` (são da Account).
- **Nunca sobrescreve** `core.role_template_permissions` de templates já existentes.

CI tem teste de regressão: snapshot de `core.permissions` antes/depois do boot só pode ter `+` ou flag `deprecated_at`.

### 2.4 Comunicação entre módulos: interfaces para leitura, event bus para efeitos

- **Leitura síncrona** (Finance pergunta nome de contato): interface registrada em `Dependencies` no `Module.Build()`.
- **Efeitos colaterais** (queue.service_finished → finance cria comissão): event bus in-process.
- Convenção de tópicos: `<module>.<entity>.<verb_past>` (ex: `queue.service_finished`).
- Reviewer rejeita handler que publica evento do mesmo módulo (deve ser síncrono).
- Bus rejeita eventos com profundidade (`causationId` chain) > 10.

### 2.5 Frontend: cada módulo é Nuxt Layer

A partir da Fase 4D:

- Módulos vivem em `web/layers/<id>/` com `nuxt.config.ts`, `nav.config.ts` próprio, pages/components/stores próprios.
- `web/app/` é o shell (auth, layout root, account selector, plugin de registry).
- Convenção anti-colisão (auto-import):
  - Components: prefixo `<Module>` em PascalCase. `QueueDashboard.vue`, `FinanceInvoiceList.vue`.
  - Composables: `use<Module>...`. `useQueueContext()`, `useFinanceInvoices()`.
  - Stores Pinia: id inclui o módulo. `defineStore('queue/context', ...)`.
  - Middleware nomeado também com prefixo. `queue-store-required.ts`.

### 2.6 JWT carrega só `userId`, `sessionId`, `expiresAt` (após Fase 1)

`accountId` NÃO entra no JWT. Frontend manda em `X-Account-Id`. Trocar de account é grátis (só atualiza cookie). Sessão revogada via `core.user_sessions.revoked_at` rejeita o JWT mesmo válido.

### 2.7 Cache de permissões com TTL curto (2 min) + invalidação por evento

Cache `(userID, accountID) → []permKey` em memória, **TTL = 2 minutos**, NUNCA atrelado ao TTL do JWT. Invalidação imediata por eventos:
- `role.permissions.changed`
- `user.role.assignment.changed`
- `user.override.changed`
- `account.modules.changed`

Razão do TTL curto: JWT vive 12h, permissão removida não pode ficar ativa por horas.

---

## 3. O que fica intocado em produção

Branches `main` e `migracao/nuxt` **não são afetadas** por este trabalho. Continuam servindo o produto atual de fila-de-atendimento até:

- A reestruturação atingir paridade total com o produto atual (smoke pós-Fase 4 obrigatório); OU
- Subir em subdomínio dedicado para testes de cliente-piloto, mantendo o domínio principal no código atual.

---

## 4. Checklist obrigatório por PR durante a reestruturação

- [ ] PR não modifica `auth.Principal`, `tenants.Service`, `stores.Service`, `access.Service`, `realtime.Service` sem aprovação explícita e atualização deste doc.
- [ ] Nenhum handler/service/repository novo aceita `account_id` (ou `tenant_id`) direto de body/query/path (exceto rotas platform-admin com checagem explícita).
- [ ] Migration nova não cria FK entre schemas satélites (apenas satélite → `core`).
- [ ] Se introduz componente novo no front, segue convenção de prefixo do layer.
- [ ] Se introduz handler de evento, segue convenção `<module>.<entity>.<verb_past>` e não publica evento do mesmo módulo.
- [ ] Atualizou o `AGENT.md` do módulo tocado.
- [ ] Se completou ou avançou tarefa do plano, atualizou [web/app/components/roadmap/roadmap-data.ts](../web/app/components/roadmap/roadmap-data.ts).
