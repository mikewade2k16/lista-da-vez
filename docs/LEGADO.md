# LEGADO.md — registro central de legado / mock / não-persistido

> Regra: [AGENT_RULES.md → "Legado, mocks e fonte da verdade"](../AGENT_RULES.md). Tudo que for legado, mock, localStorage-only ou que não persista no banco real entra aqui — com alvo e status de remoção. Nada deve ser tratado como "pronto" enquanto estiver nesta lista.

Status: `ativo` (legado em uso, precisa remover) · `band-aid` (sync temporário mantendo 2 sistemas) · `removido`.

---

## 1. Papéis de usuário em tabelas LEGADAS — `removido` ✅ (2026-06-06)

**RESOLVIDO (U1–U4c):** as tabelas `user_tenant_roles`/`user_store_roles`/`user_platform_roles` foram **DROPADAS** (migration 0135). Papéis/escopo agora 100% em `core.*` (`core.account_users` + `core.user_role_assignments` + `core.roles` + `core.user_module_settings`). Auth em `AUTH_ROLES_SOURCE=core`. Histórico abaixo.

**Pendências OPCIONAIS de limpeza (não bloqueiam; o config força core):**
- ~~Remover o CÓDIGO de fallback legado do auth~~ — **REMOVIDO ✅ (2026-06-26):** apagados `resolveLegacyAuthRoleScope`, `findStoreIDs`, `resolveRole`, `legacyRoleProjection`, o tipo `authRolesSource`/`parseAuthRolesSource`/`authRolesSourceFromEnv`, o campo `rolesSource` e os 5 campos legados de `userRecord`. `resolveAuthRoleScope` agora resolve 100% pelo core. `go build` + `go test ./internal/modules/auth/...` verdes.
- ~~`AUTH_ROLES_SOURCE` removida do compose e exemplos~~ — **REMOVIDO ✅ (2026-06-29):** var excluída de `docker-compose.yml`, `docker-compose.prod.yml`, `back/.env.example`, `.env.docker.example`, `.env.staging.example` e `.env.production.example`. Nenhum código Go lê a flag (só comentário histórico em `core_role_resolver.go`).
- Neutralizar os seeds históricos `0002`/`0012`/`0015`/`0036` que criam/seedam as tabelas num DB novo (rodam antes do backfill 0133 + drop 0135, então funcionam, mas são desperdício). **Cuidado:** os seeds alimentam o backfill 0133→core num DB fresh; remover sem cuidado tira papéis demo do dev. Fazer junto com revisão da cadeia de migrations.

---
_Histórico:_

**O que era:** `user_tenant_roles`, `user_store_roles`, `user_platform_roles` (schema public, do 0001) coexistiam com o modelo novo `core.account_users` + `core.user_role_assignments` + `core.role_permissions`.

**Onde dói:**
- O **auth login** (`auth/store_postgres.go > LoadUserForAuth`) resolve papel/tenant/store SÓ do legado. Um user só em `core.account_users` (sem papel legado) → **não loga** (500).
- `/operacao/usuarios` (users module) lista por papel legado.

**Band-aid atual (2026-06-05):** `admin_users_repository.CreateUser` grava EM AMBOS — `core.account_users` (novo) E `user_tenant_roles` (legado) — para o user criado no manage logar + aparecer na operação. **Isto é legado em sync, não solução.**

**Status U2 (2026-06-05):** o auth agora tenta resolver papel/escopo por `core.*` primeiro (`core.users.is_platform_admin`, `core.account_users`, `core.user_role_assignments`, `core.roles`) e usa o legado apenas como fallback de transicao. A fonte e controlada por `AUTH_ROLES_SOURCE=core|legacy|core_with_fallback` (default `core_with_fallback`). O legado ainda existe porque `/operacao/usuarios` e alguns writes seguem dependendo dele.

**Status U3 (2026-06-05):** `/operacao/usuarios` (`back/internal/modules/users`) agora le usuarios por `core.account_users` + `core.users` + `core.user_role_assignments`/`core.roles`, e le `storeIds` em `core.user_module_settings(module_id='queue').config.storeIdsByAccount`. Create/update tambem garantem `core.account_users`, `core.user_role_assignments` e settings da Fila. O que resta como legado e o **dual-write temporario** em `user_tenant_roles`/`user_store_roles`/`user_platform_roles`, mantido ate U4.

**Status U4a (2026-06-05, paralelo Claude+Codex):** os DEMAIS leitores de escopo migraram p/ `core.*`:
- `crm/erp` (scope `CanAccessTenant` + predicados ERP root + `ResolveDefaultTenantID` + fallback de loja do funcionario) e `queue/settings` (acesso + `ResolveDefaultTenantID`) — Claude.
- `stores` (`scope_queries.go`+`core_scope.go`) e `tenants` (`scope_queries.go`) leem `core.account_users`/`user_role_assignments`/`user_module_settings`; delete de loja faz dual-write — Codex.
- Resta o **dual-write na ESCRITA** (users, admin band-aid, consultants, stores delete, bootstrap) → U4b; e o DROP → U4c.

**Status U4b (2026-06-05):** dual-write legado REMOVIDO de todos os writers vivos — escrita agora é só core:
- `users` (`upsertCoreAssignmentsTx`), `stores` (`deleteCoreStoreScopeTx`), `consultants` (novo `core_scope.go`), `core/admin` (`user_role_assignments`), `bootstrap_owner` (core).
- Resta legado SÓ em: (a) o **fallback de leitura** do auth (`AUTH_ROLES_SOURCE=core_with_fallback`) — vira `core` no U4c; (b) **seeds históricos** `0002`/`0012`/`0015`/`0036` que escrevem nas tabelas legadas (rodam ANTES do drop num DB novo; reavaliar no U4c).
- **Pronto para U4c (DROP).**

**Backfill U2 (`0133_backfill_legacy_roles_to_core.sql`):**
- Cria roles `queue.<papel>` por account, clonados dos templates `queue.supervisor`/`queue.consultant`, e copia `core.role_permissions`.
- Garante `core.account_users` para roles tenant/store legados.
- Cria `core.user_role_assignments` para cada role legado.
- Reconcilia `user_platform_roles` em `core.users.is_platform_admin=true`.
- Para roles store-scoped, grava lojas em `core.user_module_settings(module_id='queue')` no formato `storeIdsByAccount`.

**Mapeamento legado -> core -> auth coarse:**
- `owner` -> `queue.owner` (template `queue.supervisor`) -> `owner`.
- `director` -> `queue.director` (template `queue.supervisor`) -> `director`.
- `marketing` -> `queue.marketing` (template `queue.consultant`) -> `marketing`.
- `manager` -> `queue.manager` (template `queue.supervisor`) -> `manager`.
- `consultant` -> `queue.consultant` (template `queue.consultant`) -> `consultant`.
- `store_terminal` -> `queue.store_terminal` (template `queue.supervisor`) -> `store_terminal`.
- Compatibilidade: `core.owner` -> `owner`, `core.admin`/`queue.supervisor` -> `director`, `core.member` -> `consultant`.

**Alvo:**
- Auth resolve papel de `core.*` (account_users + role_assignments), não do legado.
- `/operacao/usuarios` lê de `core.*`.
- Remover `user_tenant_roles`/`user_store_roles`/`user_platform_roles`.
- Config específica por módulo (ex.: opções da Fila por usuário) em `core.user_module_settings (user_id, module_id, config jsonb)` ou `core.users.module_settings jsonb` — NÃO em tabela legada.

**Direcao reversa U3:** `/operacao/usuarios` (users module) passou a criar/atualizar tambem `core.account_users` + `core.user_role_assignments`, entao user criado la aparece no `/manage/users`. O dual-write legado continua ate U4.

---

## 2. `public.users` como VIEW sobre `core.users` — `removido` ✅ (2026-06-06)

**RESOLVIDO (itens 2&3, paralelo Claude+Codex):** todos os pontos Go que liam/escreviam `users` migraram para `core.users` direto. A VIEW `public.users` + triggers `INSTEAD OF` foram **DROPADOS** (migration 0136). Zero ref crua a `users` no código vivo (verificado por grep).

---

## 3. `consultants` / `public.tenants` / `public.stores` — `removido` ✅ (2026-06-06)

**RESOLVIDO (itens 2&3, paralelo Claude+Codex):** todo o código da Fila migrou para `queue.stores`, `queue.consultants` e `core.accounts`. Os objetos legados foram **DROPADOS** (migration 0136):
- views `public.stores` e `public.consultants` (eram `select * from queue.*`);
- tabela `public.tenants` — antes do drop, as **27 FKs** `tenant_id → public.tenants(id)` foram **repontadas** para `core.accounts(id)` (todas `ON DELETE CASCADE`; `core.accounts.id == public.tenants.id`, 0 drift). Isso era necessário porque bootstrap/tenants admin já gravavam só em `core.accounts` — manter as FKs em `public.tenants` quebraria a criação de novas accounts.

**Pós-drop:** 47 FKs apontam para `core.accounts`; 0 para `public.tenants`. Backup pré-drop: `C:\tmp\omni_pre_drop_public.dump`.

---

## 4. Clientes de Tasks (`clientId` integer mock) — `band-aid` (front migrado p/ UUID real 2026-06-10, aguarda validação no browser)

**ATUALIZAÇÃO 2026-06-10 (tarde):** os 4 clientes foram criados em `core.accounts` e o front foi
migrado: o seletor de cliente de task agora PUXA `/v1/tenants` (UUID real) e grava em
`clientAccountId`. `TaskItem.clientId` virou `string` (contém o UUID). O `DEFAULT_CLIENT_OPTIONS`
hardcoded foi REMOVIDO. Restam como band-aid: (a) tasks ANTIGAS ainda têm `clientId` integer no
ui_metadata — elas mostram o cliente como legado (badge MOCK, via `isMockClient` = não-UUID) até
serem reatribuídas a um cliente real; (b) `tracking.vue` ainda não consome `/v1/tasks/tracking/metrics`;
(c) o seletor puxa TODOS os tenants ativos (falta o flag "aparece em tasks" na página de clientes).
_Histórico do estado anterior abaixo._



**O que é:** o seletor de "Cliente" de uma task usa uma lista **fixa de 4 clientes fictícios** em `web/layers/tasks/stores/tasks-client.ts` (`DEFAULT_CLIENT_OPTIONS`: crow=106, Perola=101, Dr Antonio Tavares=104, UNO=105), com `clientId` **integer**. Persistido só no `ui_metadata` (localStorage), não no `clientAccountId` real da task.

**Onde dói:**
- 3 dos 4 não existem em `core.accounts`; o único parecido (Perola) tem **id UUID** real, incompatível com o integer `101`.
- O backend de tasks já tem `clientAccountId` (UUID) e o `GET /v1/tasks/tracking/metrics` agrega por ele — enquanto o front gravar `clientId` integer mock, a **inteligência de tempo por cliente da página de tracking não casa** com nenhum cliente real.

**Sinalização (feita 2026-06-10):** cada option ganhou `isMock: true` + helper `tasksClient.isMockClient(id)`. No front, **só para `platform_admin`**: o dropdown de cliente mostra `description: "MOCK"` e o label "Cliente" na modal (`TasksTaskModal.vue`) mostra badge `MOCK`. O pipeline integer foi **mantido funcionando** de propósito (decisão do usuário) até os clientes reais serem criados.

**Alvo / como remover:**
1. Criar os clientes reais em `core.accounts` (já expostos por `GET /v1/tenants` + `useTenantsStore`).
2. Linkar cada mock ao account real (mapa `clientId integer → clientAccountId UUID`).
3. Trocar a fonte de `clientOptions` para os tenants reais; gravar no `clientAccountId` (UUID) da task em vez do `ui_metadata.clientId`.
4. Religar `tracking.vue` ao `GET /v1/tasks/tracking/metrics` (agrega por `clientAccountId`).
5. Remover `DEFAULT_CLIENT_OPTIONS`/`isMock` e o badge. Sai desta lista.

**Fonte de verdade confirmada:** `core.accounts` (a tabela `public.tenants` foi dropada — item 3). Ver memória `project_tasks_client_source`.

---

## 5. Overrides de acesso por usuário — módulo `access` (`/v1/access/*`) — `band-aid` (2026-06-25)

**O que é:** o sistema legado de permissões por usuário vive em `back/internal/modules/access`
(`/v1/access/roles`, `/v1/access/users/{id}/overrides`) e grava por `tenantId/storeId` com
catálogo de permissões HARDCODED (`access/model.go`) e `auth.Role`. A UI dele é
`web/app/components/users/UsersAccessPermissionPanel.vue` (na página legada `/operacao/usuarios`).

**Substituto (core, 2026-06-25):** o painel novo `web/app/components/admin/users/AdminUserModulesPanel.vue`
(aba "Módulos" do `AdminUserEditDrawer`) bate em `core.user_permission_overrides` via
`GET/PUT /v1/admin/users/{id}/accounts/{accountId}/overrides` — catálogo real do banco
(`available` = permissões dos módulos habilitados da account), valida contra `core.permissions`,
bloqueia `scope='platform'`. Fonte única, sem hardcode.

**Por que é band-aid:** os dois coexistem enquanto `/operacao/usuarios` (Fila) continuar usando o
`access`. Não remover a página legada nesta rodada (princípio: features coexistem).

**Atualização 2026-06-26 — aba "Páginas" do `/manage/users` usa de PROPÓSITO o `access` legado:**
o gating de visibilidade de PÁGINA do menu resolve do módulo legado `access`, não do core:
`useDashboardNav` → `allowedWorkspaces` → `getAllowedWorkspaces(role, auth.permissionKeys, ...)`, e
`auth.permissionKeys` vem de `/v1/me/context` → `principal.Permissions` → `access.ResolveEffectivePermissions`
(`access_role_permissions` + `user_access_overrides`). O core (`/v2/me/context`, usado por `has()`/
`CorePermissionGate`) é uma lista SEPARADA. Por isso a nova aba `AdminUserPagesPanel.vue` grava nos
overrides de access (store `access-control` → `PUT /v1/access/users/{id}/overrides`, chaves
`workspace.*.view`) — é o ÚNICO lever que muda o menu hoje. Override core de `workspace.*` não
funcionaria (as chaves nem existem em `core.permissions`; só em `access_permissions`/migration 0019).
Isso aprofunda o band-aid: alvo é mover o gating de página para o core e então migrar esta aba.

**FASE 1 (aditiva) — FEITA ✅ (2026-06-26):** as chaves `workspace.*.view/.edit` (26) foram semeadas em
`core.permissions` (declaradas no registry `core/module.go`, não-depreciadas pelo SyncCatalog) e o
`core.role_permissions` (359 linhas / 39 papéis) + `core.user_permission_overrides` (2, paridade 1:1 com
`user_access_overrides`) foram backfillados pela **migration `0175_workspace_permissions_core.sql`**. O
caminho de leitura NÃO mudou: o gating do menu segue resolvendo do `access` (`/v1/me/context`). Zero blast
radius (nada lê core `workspace.*` via `has()` hoje). É a base para a FASE 2 (switch).

**Alvo / como remover:**
1. Migrar `/operacao/usuarios` para consumir o override core (ou aposentar a página em favor de `/manage/users`).
2. **FASE 2 (switch) — PENDENTE (precisa OK + canary):** mover o gating de página para o core:
   `auth.permissionKeys`/`getAllowedWorkspaces` passam a ler de `/v2/me/context` (RBAC core, já com as
   chaves semeadas na FASE 1). Pré-requisito: verificar paridade por-usuário (core vs access) e rodar em
   canary antes do switch atômico. Então a aba "Páginas" troca para o override core e o `access` pode sair.
3. Remover o módulo `back/internal/modules/access` e a rota `/v1/access/*`.
4. Remover `UsersAccessPermissionPanel.vue` / `stores/access-control.ts`.
5. Plugar `LegacyMarker` na seção de overrides do `/operacao/usuarios` enquanto durar.

---

## 6. Módulo Finance — front sobre mock BFF — RESOLVIDO 2026-07-02 (AC-12)

**RESOLVIDO 2026-07-02 (AC-12):** back Go real em `back/internal/modules/finance/`
(migration `0187_finance_module.sql`, schema `finance.*`, rotas `/v1/finance/*`);
`web/server/` removido por completo (mock BFF Nitro + `financeMockStore.ts`);
composables `useFinancesManager`/`useFinancesConfigManager`/`useFinanceConfigEditor`
migrados de `$fetch` para `createApiRequest` (X-Account-Id pelo provider global);
`LegacyMarker` retirado de `/finance`; workspace `finance` registrado (gating por
módulo + workspace). Dados persistem no banco real (sobrevivem a restart). ADR:
`docs/adr/0002-remove-bff-nitro-mock.md`; módulo: `back/internal/modules/finance/AGENT.md`.

---

## 7. Spec externa re-especificava tenants/users/RBAC/memberships — DESCARTADO (2026-07-16)

**Nota de escopo:** isto **não é legado em uso** — nada disto foi implementado e nada precisa ser
removido. Fica registrado aqui para ninguém **re-implementar** este modelo lendo a spec original e
achando que é trabalho pendente.

**O que era:** a spec MVP externa do módulo de Atendimento WhatsApp
(`Omni_Atendimento_Spec_MVP_WhatsApp_v0.1.md`, **não versionada no repo**) trazia o seu próprio
modelo de `tenants`, `users`, RBAC e `memberships`.

**Por que foi descartado:** a plataforma **já tem** tudo isso. Aceitar a spec inteira criaria uma
segunda plataforma dentro da plataforma — duas tabelas de usuário, dois RBAC, duas verdades (fere
os princípios 1 e 2 de uma vez).

| Descartado da spec | O que a plataforma já tem | Onde vive |
|---|---|---|
| Modelo de `tenants` | `core.accounts` (a tabela `tenants` foi consolidada — item 3) | `core.accounts` + `X-Account-Id` no Principal |
| Modelo de `users` | `core.users` + sessão/JWT | módulo `core` |
| RBAC próprio | RBAC declarativo no banco, permissões seedadas pelo Module Registry no boot | `core.permissions` / `core.roles` |
| `memberships` | `core.account_users` + papéis/overrides | módulo `core` |
| Gate de módulo por conta | `core.account_modules` + `moduleGatingRules()` | `back/internal/platform/app/app.go:518` |

**O que SOBROU da spec** — os gaps reais que a plataforma de fato não tem — segue vivo como fases
do plano canônico: outbox/fila durável, cifragem de segredos em repouso (item 8 abaixo),
setores/filas/atribuição e política de mídia. É esse o valor da spec.

**Fonte de verdade:** [`omnichannel/PLANO_ATENDIMENTO.md`](omnichannel/PLANO_ATENDIMENTO.md) §3.
**Implementação CONGELADA** até a branch `refactor/multi-tenant-complete` fechar.

---

## 8. Segredos do calendário gravados em texto puro — `ativo` (pendência, 2026-07-16)

**O que é:** `back/internal/modules/calendar/secrets.go` grava a **chave de API CRUA** no banco. O
`{set,last4}` que o front recebe é **mascaramento de SAÍDA** — `mask()` corta os últimos 4
caracteres da chave em texto puro — e **não** cifragem em repouso. `PutAccountKey` / `PutGlobalKey`
passam a chave apenas trimada direto para o store; não há cifragem em nenhum ponto do caminho.

**Onde dói:** o contrato de saída `{set,last4}` faz parecer que a chave está protegida. Não está:
dump do banco, backup ou qualquer acesso de leitura ao Postgres expõem a chave de API do provider
de IA de todas as contas. É **gap de segurança real**, não conveniência de implementação.

**Não bloqueante:** o calendário funciona e não regride por causa disto — por isso é pendência
registrada, não impedimento. Mas não é "pronto" enquanto estiver nesta lista.

**Alvo / como remover:**
1. A **F3** do plano de atendimento entrega `back/internal/platform/secretbox` (AES-256-GCM, chave
   via env `OMNI_SECRETS_KEY`, prefixo `v1:` para rotação). O pacote nasce em `platform/` — e não
   dentro do módulo omnichannel — justamente porque o calendário é o segundo consumidor previsível.
2. Migrar a escrita/leitura de `calendar/secrets.go` para o `secretbox`, re-cifrando as chaves já
   gravadas em texto puro.
3. Manter o contrato de saída `{set,last4}` **como está** — ele já é o modelo certo a seguir; o que
   falta é a cifragem embaixo dele.
4. Rotacionar as chaves depois da migração (o texto puro já esteve no banco e nos backups).
5. Sai desta lista quando `secrets.go` não gravar mais texto puro.

**Fonte de verdade:** [`omnichannel/PLANO_ATENDIMENTO.md`](omnichannel/PLANO_ATENDIMENTO.md) §8 e
§14.6. **Status:** o `secretbox` já foi entregue pela F3 (2026-07-17); falta o passo 2 (migrar
`calendar/secrets.go` para ele) — continua `ativo` até lá.

---

## 9. Módulo de Atendimento WhatsApp — dívida assumida no port — `band-aid` (2026-07-17)

**O que é:** o front do módulo (`web/app/**/omnichannel/**`) entrou **verbatim** do painel legado
(`web-reference`), byte a byte, para não reescrever código maduro enquanto se porta (decisão D-B,
[`omnichannel/PLANO_ATENDIMENTO.md`](omnichannel/PLANO_ATENDIMENTO.md) §2). Isso traz dívida
**consciente e listada**, com alvo na **F14**:

| # | Vestígio | Onde | Alvo |
|---|---|---|---|
| a | **Front sobre backend parcial** — a leitura (F2) é real; canal, tempo real, envio e ações ainda não existem (404 proposital). Badge `PARCIAL (F2)` na página avisa o admin | `web/app/pages/omnichannel/index.vue` | badge sai ao fechar a F7 |
| b | **5 adaptadores de costura** — `useApi`/`useAdminSession`/`usePageBootstrapLoading`/`session-simulation`/`types` traduzem o legado para o Omni. São temporários | `web/app/composables/`, `web/app/stores/`, `web/app/types/` | **F14** (módulo passa a usar `createApiRequest`/auth do Omni direto) |
| c | **Arquivos acima de 450 linhas** — o maior tem 1.467 (`useOmnichannelInbox.ts`). O ESLint acusa `max-lines` (~460 warnings): **esperado e consciente**, não corrigir antes da fase de split | `web/app/**/omnichannel/**` | **F14** (split incremental com smoke a cada passo) |
| d | **Módulo em `web/app/`, não em layer** — dentro de layer o `~` resolveria para `web/app` e os imports do legado quebrariam. Precedente: o calendário também não é layer | `web/app/**/omnichannel/**` | **F14** (mover para `web/layers/omnichannel/` com imports relativos) |
| e | **Stub inerte de realtime** — `socket.io-client` não existe no web; o import verbatim derrubaria o build do Vite. Trocado por stub que preserva a lógica dos 3 eventos | `useOmnichannelInboxRealtime.ts` | **F5** (reescrita sobre WS nativo com ticket) |
| f | **`assignedUserIds` / `userScopePolicy` — RESOLVIDO (persistem)** — deixaram de ser constantes fixas: o front tem controles reais (seletor de política + `PUT .../instances/{id}/users`) e mantê-los fixos deixaria esses controles mortos (o save reverteria na releitura). Agora gravam em `whatsapp_instances.provider_config` (jsonb, chaves próprias) via a gestão de instância — **coluna existente, não tabela nova**. `scanInstance`/`parseInstanceScope` leem com fallback nos defaults do legado (`MULTI_INSTANCE`/`[]`). `assignedUserIds` é filtrado p/ membros ativos da conta (isolamento) | `back/internal/modules/omnichannel/store_instances.go`, `service_instances.go` | — (fonte real no banco; o gate de DADO definitivo segue sendo `queue_members` na F8) |
| f2 | **`validate-endpoints` valida CONFIG, não faz probe vivo** — não há "ping" na `channel.Provider`/`SessionManager` e a rota de envio (`SendMessage`) é F6 (probá-la enviaria mensagens). A validação deriva status honesto da config real (provider, `baseURL`, presença de credencial); `httpStatus: null`; o caso `ok` **não afirma** conectividade. Não é mock (retorna erros reais quando a config falta), mas também não é o probe completo do legado | `omnichannel/service_instance_ops.go` | probe real na fase de envio (F6) |
| g | **QR cache e rate-limit do webhook em MEMÓRIA** — não há Redis no `go.mod` (decisão de dedupe por tabela, spec C4). O cache de QR (TTL 120s) e o limitador `provider:slug:ip` do webhook vivem em memória: **não sobrevivem a restart nem a deploy multi-instância** (registro idêntico ao `httpapi/rate_limit.go`). Hoje a api é um container só | `omnichannel/qr_cache.go`, `omnichannel/rate_limit.go` | broker compartilhado quando houver >1 instância da api |
| h | **Adapter `mock` não autentica o webhook** (`VerifyWebhook` retorna nil) — não há segredo compartilhado com um provedor real; o mock só roda em ambiente de teste. A rota segue protegida por rate-limit/content-type/content-length. O `evolution` real fará verificação constant-time | `omnichannel/channel/mock/mock.go` | não removível (é o ponto do mock); os providers reais autenticam |
| i | **Rate-limit do webhook sem "block 5 min"** — o legado bloqueava o par por 5 min após estourar; aqui é só janela deslizante 600/min → 429 (sem período de bloqueio prolongado). Simplificação consciente | `omnichannel/rate_limit.go` | refinamento futuro se necessário |
| j | **Auto-set do webhook da Evolution depende de `provider_config.webhookUrl`** — o adapter `evolution.Connect` só configura o webhook da instância (auto-reparo) se `cred.Config["webhookUrl"]` chegar preenchido. Hoje `service_session.credentialsFor` monta `Credentials.Config` só do `provider_config` do banco, que nasce SEM `webhookUrl` (F2: a URL vem de `WEBHOOK_RECEIVER_BASE_URL`, ainda não injetada). Sem isso, a Evolution não recebe a URL de callback e o inbound real não chega. **O adapter está correto contra a interface; falta a Fundação passar `webhookUrl` (compor `WEBHOOK_RECEIVER_BASE_URL` + `/v1/webhooks/omnichannel/evolution/{slug}`) no `Config` ou no `provider_config`** | `omnichannel/service_session.go` (`credentialsFor`), `omnichannel/channel/evolution/adapter.go` | wiring da F4/F10 (setup de números) |
| k | **`MESSAGES_UPDATE` de deleção → `ignored`** — a interface `channel.Event` não modela "mensagem apagada" (só `message_status` SENT/FAILED). Deleção/edição inbound da Evolution vira `EventIgnored` nesta fase; o ACK vira `message_status` normalmente | `omnichannel/channel/evolution/parse.go` | fase que modelar deleção inbound (F9 avalia DELIVERED/READ + deleção) |
| l | **Escopo de instância DIVERGE do legado — de propósito** — no legado o ternário `isTenantAdmin \|\| active.length <= 1 ? active : active` devolve o mesmo nos dois ramos (`whatsapp-instances.ts:681-683`): o filtro por `responsible_user_id` **nunca rodou** e todo usuário vê todas as instâncias. A F7 porta **corrigido** (`AccessibleScopeKeys`): admin vê todas; conta com ≤1 ativa todos veem a única; demais só as suas (`responsible_user_id`); vazio → vê nada. **Muda o comportamento vs legado** (é isolamento, princípio 2). O gate de DADO definitivo (`queue_members`, F8) soma com AND. *Pendência:* a leitura (F2, `service.go`) ainda usa a versão parcial — unificar depois | `omnichannel/store_postgres_scope.go`, `service_actions.go` | unificar leitura+ações num `AccessibleScopeKeys` só |
| m | **Ações síncronas ao provider ainda 409** — `reaction`, `delete-for-all`, `group-participants`, `sync-open`, `sync-history`, `import-whatsapp` chamam o provedor na hora, mas o `channel.Provider` da F4 **não expõe** os métodos (só a capability `SupportsReaction`). A F7 entrega a rota, o escopo, a permissão, o gate de `Capabilities()` e a auditoria, mas responde **409 acionável** (`provider_action_unavailable`/`action_unsupported`) — `delete-for-all` marca os elegíveis em `failedIds`. **Nunca sucesso fingido.** `forward` (via outbox F6), `status`/`assign` (via FSM), `delete-for-me` (hidden_messages) e `open-conversation` funcionam ponta a ponta | `omnichannel/service_actions_messages.go`, `http_actions.go` | F4 estende `channel.Provider` com `SendReaction`/`DeleteForAll`/etc. + implementa no `evolution`; então troca o `ErrProviderActionUnavailable` pela chamada real (502 na falha) |

**Não bloqueante:** a tela abre e a leitura funciona. Cada item sai na fase indicada; a F14 é quando o
`LEGADO.md` esvazia dos itens a–d. Os itens g–i são de infra e saem quando houver broker/número real.
O item `m` sai quando a F4 estender a interface do `channel.Provider` (métodos de reação/deleção síncronas).

**Fonte de verdade:** [`omnichannel/PLANO_ATENDIMENTO.md`](omnichannel/PLANO_ATENDIMENTO.md) §14 +
[`omnichannel/ESTADO.md`](omnichannel/ESTADO.md).

---

## Infra do princípio
- [x] **Marcador visível no front (só `platform_admin`)** — `web/app/components/admin/LegacyMarker.vue` (badge "LEGADO"/"MOCK"/"localStorage", visível só p/ platform_admin). Plugado em `/operacao/usuarios` (item 1). **Plugar nas demais telas que dependem de legado/mock conforme forem encontradas.**
- [x] `core.user_module_settings (user_id, module_id, config jsonb)` criada (migration 0132) — destino da config por módulo (estágio U1 do [USER_MODEL_UNIFICATION_PLAN.md](USER_MODEL_UNIFICATION_PLAN.md)).
- [ ] Auditar front por mocks/localStorage remanescentes e listar aqui (+ plugar LegacyMarker).
- [x] Estágios U2–U4 (auth lê core, operacao lê core, dropar `user_*_roles`) — concluído (0135).
- [x] Itens 2&3 — dropar views `public.users/stores/consultants` + tabela `public.tenants` (0136, repontando FKs p/ `core.accounts`) — concluído.
