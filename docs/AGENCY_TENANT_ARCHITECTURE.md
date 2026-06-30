# Arquitetura Agência → Clientes (tenants) — plano de execução

> Status: **CONCLUÍDO ✅ (2026-06-15).** Etapas 1-5 entregues e verificadas: board geral de Tasks (247)
> agora é da conta-agência `crow`; admins membros da crow + agency_owner → defaultam na agência;
> `dono==cliente=0`; isolamento intacto (cliente→crow=f); sem perda de dados (total 247).
> Documento canônico desta mudança.
> Origem: 2026-06-10, ao corrigir o "dono" do board de Tasks. Revisado em 2026-06-15 ao
> confrontar o estado descrito com o código vivo — **boa parte já estava construída**.
> Roadmap: fase `agency-tenant` (AT) em `web/app/components/roadmap/roadmap-data.ts`.
> Ver também [LEGADO.md](LEGADO.md) §4 e a memória `project_tasks_client_source`.
> **Follow-up 2026-06-30:** login authn≠authz em 2 etapas + gating de workspace por papel custom — ver
> [§8](#8-login-authnauthz-em-2-etapas-2026-06-30) (fase `agency-login-2step` / AL).

---

## 1. Modelo-alvo (definido pelo dono do produto)

- **Crow = a agência** (o time dev/dono do projeto). É o nível mais alto.
- **Clientes = tenants** (`core.accounts`): Pérola, AM Malls, UNO, Dr Antonio Tavares, Duby,
  Juliana Oliveira, Cléo Moraes, Mostarda… Todos os clientes **pertencem à agência Crow**.
- **Módulos são da agência**; cada cliente usa os que contratou. Ex.: o módulo **Fila** hoje só a
  **Pérola** usa (operação real: 46 usuários, terminais, consultoras).
- **Login de agência** acessa os módulos de cada cliente **conforme o nível** do usuário dentro da
  agência. Um usuário de cliente só vê o próprio tenant.
- O board **Tasks** é da **agência** (board geral, com clientes como *atributo* da task). Nenhum
  cliente é "dono" das tasks — o dono é a agência.

```
Organização "Crow Visuals" (agência)
├── conta-agência "Crow"  (80caf5d5)  → dona do board geral Tasks (clientes = client_account_id)
├── conta-cliente "Pérola" (aaaaaaaa) → módulo Fila (operação real)
├── conta-cliente "AM Malls"
├── conta-cliente "UNO"
└── … demais clientes
```

## 2. Estado atual (revisado 2026-06-15)

### Já construído (não refazer)
- **Back — CRUD de organizations**: `/v1/admin/organizations` (list/create/get/update/delete) em
  `back/internal/modules/core/admin_organizations_{http,service,repository,model}.go`. Restrito a `platform_admin`.
- **Front — UI de organizations**: `web/app/pages/manage/organizations.vue`,
  `AdminOrganizationsWorkspace.vue`, `useAdminOrganizationsManager.ts`, types em `admin-organizations.ts`.
- **`core.organization_users`** já é gravada ao criar usuário com `org_role` (`agency_owner`/`agency_member`)
  em `admin_users_repository.go`.
- **`PATCH /v1/admin/accounts/{id}`** aceita `organizationId` (`''`=NULL) → dá para vincular conta a org pela UI.
- **Switcher v2 ligado ao api-client global**: `web/app/plugins/account-id-bridge.client.ts` injeta
  `X-Account-Id = useCoreAccountStore().activeAccountId`. Logo **queue/crm já seguem o switcher**.
- **`MeAccounts`** já retorna a `organization` quando as accounts compartilham a mesma org.

### Etapas — done vs falta (atualizado 2026-06-15)
1. **Etapa 1 ✅ (o nó):** Tasks agora lê `useCoreAccountStore().activeAccountId` (`tasks.ts` +
   `useTimeTracking.ts`), com `watch`/reload via AbortController. Switcher montado em `DashboardHeader.vue`.
2. **Etapa 2 ✅:** migration `0156` criou a org "Crow Visuals" e vinculou as 11 contas.
3. **Etapa 3 ✅:** `ListAccountsForUser`/`FindAccountIfMember` **e** `auth.IsMember` org-aware (ver §3).
4. **Etapa 4 ✅ (board movido):** board `9d40be47` (247 tasks) + time_entries + audit movidos de `aaaa`
   (Pérola) → `crow` (`80caf5d5`) em transação atômica. A FK composta **não é deferível** → foi
   **dropada e recriada** dentro da transação (a recriação valida a consistência; 0 inconsistências
   pós-move). Backup: `/c/tmp/tasks_pre_move_backup_20260615.sql`.
   - **Acesso ✅:** migration `0157` — platform_admins viraram **membros da conta `crow`** +
     `agency_owner` da org. Verificado: 3 admins membros+agency_owner; default do Mike = `crow`.
5. **Etapa 5 ✅:** `client_account_id` limpo — as **31** tasks com `client=crow` (== dono) viraram
   `null` (internas); as **111** com `client=perola` ficaram (Pérola é cliente de verdade ≠ dono `crow`).
   Board fantasma de `am-malls` apagado. `dono==cliente=0` confirmado no banco.

> **Quem aplicou as escritas de dado:** o classificador de segurança barra escrita de dado de tenant
> pelo agente (regra "trabalhar só local, devolver comandos"), então o **usuário rodou** os comandos
> (board move, 0157, Etapa 5). O supervisor verificou cada um no banco (read-only) após.

### Incidente (aprendizado — 2026-06-10)
Tentou-se mover o board para uma conta Crow via SQL **antes** de ligar a troca de conta. Resultado:
o board sumiu da visão do dev (conta ativa continuou `aaaa`) e o front auto-criou um board vazio.
**Revertido do backup** (`/tmp/tasks_mig_backup_20260610.csv`). **Regra: não mover dado de tenant
antes de o switcher estar ligado ao contexto do módulo (Etapa 1).**

## 3. Decisão de acesso da agência (Etapa 3 — org-aware)

`ListAccountsForUser` e `FindAccountIfMember` passam a resolver:
- **platform_admin** → todas as accounts ativas.
- **agency_owner** (linha em `core.organization_users`) → todas as accounts da **sua org**.
- **demais** → só as accounts com membership explícita em `core.account_users` (comportamento atual).

Vantagem sobre membership explícita: 1 query, sem manter N vínculos por cliente novo. Segurança:
mesmo org-aware, o repo continua filtrando por escopo permitido — usuário de cliente nunca cai no
ramo agency_owner. Cobrir os 3 caminhos + tentativa fora do escopo (→ not-member) em teste Go.

### DOIS caminhos de visibilidade precisam ficar em sync (gap pego no Gate 1)
A visibilidade de account vive em **dois** lugares que NÃO podem divergir:
1. **Leitura** (módulo core): `core.ListAccountsForUser`/`FindAccountIfMember` — alimenta
   `/v2/me/accounts` (o que o switcher LISTA).
2. **Enforcement** (auth middleware): `auth.PostgresAccountMemberChecker.IsMember` — o portão de
   `RequireAuthWithAccount` que valida o `X-Account-Id` em **TODA** rota de módulo (queue/crm/tasks).

Se só a leitura virar org-aware, o switcher LISTA a conta-agência mas **usá-la dá 403
`account_not_member`** (o board Tasks "some" ao mudar de conta) — repetindo o incidente de
2026-06-10 por outro motivo. Por isso ambos receberam a MESMA regra org-aware (mesma cláusula SQL,
replicada porque `auth` não importa `core`). `IsMember` usa `accountAccessibleQuery` (const, testada).

### Estado da Onda 1 (2026-06-15)
- **Trilho A (front):** switcher v2 → Tasks + `watch`/reload com AbortController. **Gaps pegos ao testar
  (2026-06-15):** (1) o `CoreAccountSwitcher` NÃO estava montado em lugar nenhum → montado em
  `DashboardHeader.vue` (header real do layout dashboard; o `DashboardUnifiedHeader` é órfão)
  (`v-if isAuthenticated && accounts>1`); (2) o org-aware fez o
  `defaultAccountId` virar a 1ª conta por nome (am-malls) → `ListAccountsForUser` agora ordena
  **membership-first**; (3) o auto-create de board default poluiu a am-malls → gated a usuário de
  **conta única** (e nunca no switch); board fantasma removido. ESLint 0 errors. **Falta validar no
  browser** (parte humana do Gate 1).
- **Conta-casa do admin (descoberta):** o platform_admin de teste (Mike) NÃO tem membership real
  (`core.account_users`) — é um admin "flutuante", então mesmo com membership-first cai na 1ª por nome.
  Resolução correta = **Etapa 4**: tornar a agência membro da conta `crow` + mover o board para `crow`;
  aí o admin cai no board real da agência. Até lá, usar o switcher para escolher Pérola.
- **Trilho B (back):** `ListAccountsForUser`/`FindAccountIfMember` org-aware + `IsMember` org-aware +
  testes. `go build/vet/test/golangci-lint` limpos.
- **Trilho C (DB):** `0156_agency_org_crow.sql` aplicada no rebuild (`migration_up_ok`). Org
  `crow-visuals` criada, 11 contas vinculadas, 0 agency_owner (conta `crow` sem membros; dev é
  platform_admin). Provado no banco: `mike(admin)→crow=t`, `cliente→crow=f` (isolamento ok).

## 4. Plano em ondas (paralelismo seguro)

A arquitetura é **sequencial por segurança** (dado só se move na Etapa 4, após o Gate 1). O
paralelismo máximo *seguro* é a Onda 1 com 3 trilhos de código; o movimento de dado fica na Onda 2,
sequencial, uma mão só.

**Onda 1 — 3 trilhos paralelos, SÓ código (Tudo Opus):**
- **Trilho A (front):** Etapa 1 — `tasks.ts` + `useTimeTracking.ts` → `useCoreAccountStore().activeAccountId`.
- **Trilho B (back/segurança):** Etapa 3 — org-aware em `ListAccountsForUser`/`FindAccountIfMember` + testes.
- **Trilho C (DB):** Etapa 2 — migration idempotente (org Crow + vínculo das contas + agency_owner).
- **+ docs:** AGENT.md (core, tasks-layer), panorama HTML, roadmap.

**Gate 1 (supervisor Opus + usuário):** `docker compose up -d --build api`; validar no browser que o
switcher recarrega o Tasks, org criada e contas vinculadas, login-agência enxerga as contas, sem
vazamento cross-tenant. **Trava de segurança.**

**Onda 2 — sequencial, só o supervisor (Opus), após o Gate 1:**
- **Etapa 4:** backup → mover board Tasks `aaaa`→Crow (`boards/tasks/task_time_entries/audit_log`),
  FK composta deferida na transação e restaurada.
- **Etapa 5:** limpar `client_account_id` (Pérola cliente; internas → null).

## 5. Notas de Deploy / dados

- **Etapa 2** (org + vínculo) e a **limpeza da Etapa 5**: portar para **migration idempotente**
  (`IF NOT EXISTS` por slug; numerar a próxima livre, ex.: `0156_*`) antes da VPS — não rodar só manual local.
- **Etapa 3** mexe em Go → **`docker compose up -d --build api`** (restart não basta).
- **Etapa 1** é front-only (sem rebuild da api).
- **Etapa 4** é **movimentação de dados** (account_id de board/tasks/entries/audit) — exige backup +
  checagem. A FK composta **não é deferível**, então o procedimento é **DROP + recreate** da FK na
  transação (a recriação valida). Feito local com backup. **Na VPS é dado DIFERENTE** (board/account
  ids próprios) — NÃO rodar o move cego; avaliar se o board da agência lá está mis-homed e repetir o
  procedimento se preciso.
- **Migrations portáveis** (rodam na VPS): `0156` (org Crow + vínculo) e `0157` (admins membros da crow
  + agency_owner). Idempotentes. A `0157` é no-op se a conta `crow`/org `crow-visuals` não existir no ambiente.
- **Etapa 5** (limpar `client=crow`) e o **delete do board fantasma** são reparos de dado específicos
  deste banco — manuais, não viram migration cega.
- Portas inalteradas (api=9091, web=3003, postgres host 5432 / container 5433).

## 6. Estratégia de agentes (escolhida 2026-06-15)

- **Supervisor = Opus** (sessão principal). Dono do plano, dos gates e do **movimento de dados** (Etapas 4/5).
- **Onda 1 = Tudo Opus** (3 subagentes Opus em paralelo, um por trilho). Decisão do usuário: qualidade máxima.
- Nenhum subagente roda `git` (sessão multi-agente). Nenhum subagente move dado de tenant.

## 8. Seguimento — switcher = "view-as" + conta-agência "Crow Visuals" (2026-06-15)

Após a entrega das Etapas 1-5, o dono do produto apontou 2 ajustes (fase `agency-view-as` no roadmap):

- **Switcher vira "ver como o cliente" (view-as) para o platform_admin.** O admin furava o gating e via
  tudo em qualquer conta; pior, conta com **0 módulos** (ex.: AM Malls) mostrava o menu INTEIRO por um
  guard ruim (`enabledModulesSet.size > 0` desligava o filtro). Corrigido:
  - **Menu** (`useDashboardNav.isItemAllowed`): filtra sempre que há `activeAccount` carregada (mesmo com
    0 módulos) — conta sem o módulo nunca mostra o item; `core`/Manage sempre.
  - **Rota** (`module-enabled.global.ts`): removido o bypass do `platform_admin` — o admin também é gated
    pela conta ativa. Fallback de bloqueio = `/perfil` (não-gated; `/` loopava via `index→/operacao`).
  - Backend `RequireModuleByPath` **segue isentando** platform_admin de propósito (precisa da API p/ o
    Manage); o bloqueio de rota no front é o que faz o view-as. Cliente não-admin já era gated (sem regressão).
- **Conta-agência deixa de parecer cliente.** Migration `0158`: a conta `crow` vira **name "Crow Visuals"**
  (slug mantém), ganha **`is_agency=true`** (coluna nova em `core.accounts`) e **todos os 11 módulos**
  habilitados (god view + o board de Tasks aparece no menu dela). O `GET /v1/admin/accounts` (lista de
  clientes) passa a **excluir `is_agency`** → a agência some de `/manage/clientes` (continua no switcher
  como conta-casa do admin, e gerenciável em `/manage/organizations`).

**Deploy:** `0158` é migration portável idempotente (aplicada no rebuild). Backend mexido → `--build api`.

### Refinamentos do switcher (2026-06-15)
- **Switcher em 3 seções, só `platform_admin`:** ADMIN DA PLATAFORMA · ORGANIZAÇÕES (contas `is_agency`,
  ex.: Crow Visuals) · CLIENTES (contas não-agência agrupadas por organização). `AccountSummary` ganhou
  `isAgency` + `organizationName` (MeAccounts, left join organizations). Cliente comum não vê o switcher.
- **Botão "Plataforma (dev)":** contexto super-admin (`platformView` no store) que **revela itens
  `hidden`/em desenvolvimento** não liberados nem para a conta-agência (bypass no `useDashboardNav` +
  `module-enabled`; escopa na conta-agência p/ X-Account-Id). Selecionar org/cliente desliga.
- **Dropdown fecha no clique-fora/Esc/opção:** corrigido no `CoreAccountSwitcher` e nos dropdowns do
  menu principal (`DashboardHeader` Tools/Site/Manage, agora click-controlados). Virou **regra no
  [AGENT_RULES.md](../AGENT_RULES.md)** (Frontend): todo dropdown feito à mão tem que fechar assim.

## 7. Fora de escopo (intencional)
- **Performance do board** (virtualização/render dos cards) é independente desta arquitetura e está
  sendo feita à parte. Mover de conta não muda o custo de render.

## 8. Login authn≠authz em 2 etapas (2026-06-30)

> Roadmap: fase `agency-login-2step` (AL). Origem: bug reportado em 2026-06-30 — um usuário ativo
> vinculado **só** à organização Crow Visuals (sem cliente direto), com **papel custom** ("Editor e
> Filmaker") e páginas liberadas pelo painel, levava **403 `user_no_role`** já no login (em prod e local).

### Causa-raiz
O vínculo de org (`linkUserToOrganization`) já matricula o usuário na conta-agência com um **papel-base de
fila** (`queue.owner`/`queue.director`, clonado de `queue.supervisor`, rotulado "Supervisor de Fila") —
era esse papel que o login sabia mapear. Mas salvar a aba **Papéis** é um **replace total**
(`ReplaceUserRoleAssignments`: `DELETE` de todos os assignments + reinsere só os marcados). Ao marcar só o
papel custom e desmarcar "Supervisor de Fila", o papel-base sumiu. No login, `resolveCoreAuthRoleScope` +
`CoarseRoleFromCoreRole` (queue-cêntricos) não mapeavam o papel custom → escopo vazio → `ValidateUserScope`
rejeitava. Ou seja: **a resolução de identidade no login era queue-cêntrica** e não tolerava usuário
só-agência/só-custom.

### Decisão do dono do produto: authn ≠ authz, em 2 etapas
- **Etapa 1 (login/authn):** credencial válida + usuário ativo ⇒ **autentica e recebe token**. O login
  **nunca** barra por "sem papel/escopo" (espelha o `platform_admin`, que já loga com `TenantID/AccountID`
  vazios).
- **Etapa 2 (autorização):** o que o usuário vê é resolvido **depois**, por requisição/conta, pela **RBAC
  custom** (papéis criados no painel = `core.role_permissions`, resolvidos em `/v2/me/context`). O
  papel-coarse de fila é **legado**; não é mais autoridade de acesso.

### Backend (`back/internal/modules/auth/`, `platform/app/context_http.go`)
- `buildUser`: escopo-coarse vazio **não** chama `ValidateUserScope` (que segue rígido **só** para papel
  STORE-scoped — consultant/manager/store_terminal continuam exigindo loja).
- `http.go`: `ErrInvalidRoleScope` no login vira `user_store_scope` (vínculo de loja inválido), nunca mais
  "sem papel".
- `context_http.go`: `/me/context` com `TenantID` vazio devolve `stores=[]` (guard local), sem 403; a
  semântica global de `resolveTenantFilter` fica intacta.
- **NÃO** deriva um `TenantID`-default por org: o curto-circuito legado `CanAccessTenant`
  (`queue/settings`) confia no `TenantID` do principal **sem** rechecar membership — preencher ali seria
  **over-grant**. A conta-agência é concedida pelo caminho certo: o `account_checker` org-aware valida o
  `X-Account-Id` por requisição (intocado). `principal.Permissions` de escopo vazio = base vazia + só os
  overrides explícitos do admin → sem grant tenant-wide legado vazando.

### Frontend (gating v1↔v2)
O menu/home usavam **só** as permissões do login v1 (`auth.permissionKeys` + papel-coarse), ignorando as
permissões **custom da conta ativa** (v2). Correção **aditiva** (só amplia; o back gateia de fato):
- `auth.ts`/`workspace.ts`: passam a usar **permissões EFETIVAS** = login v1 ∪ `useCoreAccountStore().permissions`
  (v2). Novos: `effectivePermissionKeys`/`effectivePermissionsResolved`/`hasCoarseRole`.
- `permissions.ts`: `getAllowedWorkspaces` revela workspace de módulo **sem `viewPermission`** (ex.: Tasks)
  por **prefixo de permissão** (`MODULE_WORKSPACE_PERMISSION_PREFIXES = { tasks: 'tasks.' }`).
- `login.vue`: quando o login vem **sem papel-coarse**, carrega a Etapa 2 (`fetchAccounts`) **antes** de
  decidir a home, para `auth.homePath` refletir o workspace que o papel custom concede (cai em Tasks, não
  em `/operacao` vazio). O guard de `workspaceId` em `auth.global.ts` já auto-corrige o landing.

### Footgun registrado
O replace de papéis do painel ainda pode remover o papel-base de fila do enroll de org. Com o login
two-step isso **deixa de derrubar o acesso**, mas fica em aberto endurecer (travar `is_locked` o papel-base
ou avisar no painel).

### Deploy
Backend mexido → `docker compose up -d --build api` (feito local). Sem migration nova: a correção
**auto-cura** usuários já quebrados no próximo login (o vínculo de org + a membership na conta-agência
seguem intactos; o replace só apagou `user_role_assignments`).

### Validação humana pendente (não dá pra automatizar sem credencial)
Logar com o usuário só-agência (ex.: `iasminpereirasnt@gmail.com`) e confirmar: (1) login sem 403; (2)
cai no workspace que o papel custom concede (Tasks), não em `/operacao` vazio; (3) menu reflete a RBAC
custom; (4) usuário de cliente comum não ganhou acesso novo.

### 2ª rodada (2026-06-30) — causa-raiz do login, Fase 0 de segurança, banner, e roadmap aprovado

**Causa-raiz real do login (terceiro gate, achado ao testar com `iasmin@omni.com`):** o login dava
`POST 200` mas o `/me/context` seguinte dava `401`. O bloqueio era o `Parse` do token em
`auth/tokens.go` — exigia `IsValidRole(claims.Role)`, e o usuário só-custom loga com `Role=""`. Fix: o
`Parse` aceita papel-coarse **vazio** (só rejeita `Role` não-vazio inválido). Login validado no browser.

**Remoção de legado (decisão do dono — "não devemos ter legado"):** o curto-circuito do `CanAccessTenant`
(que devolvia `principal.TenantID == requested` **sem rechecar membership**) foi removido em
`queue/settings` e `crm/erp` — agora **sempre** recheca no banco.

**Fase 0 de segurança (achados por varredura adversarial multiagente):**
- `queue/consultants/service.go` `ListOrphans` e `Update` (órfão): rechecam membership via novo
  `consultants.Repository.CanAccessTenant` (org-aware) em vez de confiar no `principal.TenantID` — fecha
  vazamento cross-tenant + janela pós-revogação.
- `stores/service.go` `ListAccessible`: usuário de escopo-vazio recebe **lista vazia** (não 403),
  espelhando o `/me/context`.

**Banner "Modo degradado":** a conta-agência tem `queue` habilitado (god-view, 0158), então o front
tentava `/v1/settings` sem tenant de Fila → degradado. Fix em `web/app/stores/auth.ts`
`canFetchQueueSettings`: pula quando o escopo-coarse é vazio **ou** a conta ativa é `is_agency`.

**Roadmap APROVADO — aposentar o coarse → default `core.roles` por cliente (faseado):** o papel-coarse
não deve ser enum hardcoded; os papéis legados viram `core.roles` **padrão por account** (editáveis). Plano
canônico de execução: `C:\Users\Mike\.claude\plans\cozy-sparking-bonbon.md`. **Achado estrutural:**
`access_role_permissions` (global, chaveado pelo enum coarse, sem `account_id`) é um **2º sistema de authz
paralelo** ao `core.role_permissions` — seedar roles não basta; o enforcement por requisição (permissões +
store scoping em `findCoreStoreIDs`) precisa migrar para ler `core.*` por account. `defaultRolePermissionMap`
(`access/permissions.go`) é a fonte fiel das permissões a seedar. Ordem: Fase 0 (feita) → Fase 1 (seed,
aditivo) → Fases 2-5 (migrar leitura, atrás de flag + shadow na Pérola, depois aposentar o enum). A operação
da Fila da Pérola é o oráculo de regressão e não pode quebrar.

### Fase 1 entregue (2026-06-30) — default `core.roles` por cliente, editáveis

**Achado que reduziu o risco:** a migration **0175** já tinha declarado as permissões `workspace.*` no
catálogo (`core.permissions`, módulo `core`, sempre habilitado) e **backfillado** `core.role_permissions`
por papel com o mapa fiel (`access.defaultRolePermissionMap`). E os papéis-coarse já existiam como
`core.roles` por conta. Logo a Fase 1 deixou de ser "criar do zero" e virou uniformizar + marcar + limpar.

**Migration `0176_seed_default_account_roles.sql`:** para cada conta-CLIENTE ativa (decisão do dono: TODAS,
inclusive sem Fila — as perms são `workspace.*` do módulo `core`, válidas em qualquer conta), garante os 6
papéis operacionais (`queue.owner/director/marketing/manager/consultant/store_terminal`) como `core.roles`
`is_default=true` e **editáveis**: cria os faltantes, marca `is_default`, corrige os labels clonados errados
('Supervisor de Fila' → 'Proprietario da Fila'/'Diretoria da Fila'/...) e garante as `workspace.*` (mesmo
mapa da 0175). Aditivo/idempotente; **não** remove papéis, **não** trava (o safeguard de "conta sem dono" é o
`core.owner`, já `is_locked`), **não** sobrescreve renome do cliente. Validado: **8/8 clientes com os 6
papéis**, labels limpos, perms preenchidas. O cliente gerencia tudo no painel (matriz de papéis).

**Follow-up pendente — conta NOVA (decisão do dono 2026-06-30):** em vez de hardcodar templates no código, os
papéis-padrão serão **gerenciáveis pelo painel** (platform_admin cria/edita `core.role_templates`) e a conta
nova clona via `cloneRoleTemplates` (que já clona todos os templates). Mapeado e planejado na **fase RT**
(`role-templates-painel`) do `roadmap-data.ts`. Achados da exploração: NÃO existe CRUD de templates (back nem
UI); o `SyncCatalog` **nunca deleta** template não-declarado (regra CONTRACT_FREEZE), então template criado
pelo painel (`is_system=false`) sobrevive ao reboot; falta só (a) CRUD admin de templates, (b)
`cloneRoleTemplates` setar `is_default=true`, (c) UI no painel, (d) decidir visibilidade (global × por-conta).

### Fase RT entregue (2026-06-30) — papéis-padrão gerenciáveis no painel

Decisão (sem perguntar, conforme pedido): templates do painel são **globais** (`is_system=false`,
`module_id='core'`); conta nova clona via `cloneRoleTemplates` com `is_default=true`. Entregue:
- **Back (módulo core):** `admin_role_templates_{model,repository,service,http}.go` + rotas
  `/v1/admin/role-templates*` (GET/POST/PATCH/PUT permissions/DELETE), gate `requirePlatformAdmin`
  (mesmo dos outros `/v1/admin/*`); SQL parametrizado/schema-qualificado; `is_system=true` congelado
  (PATCH/PUT/DELETE → 409, regra CONTRACT_FREEZE); keys validadas (sem `scope='platform'` nem deprecated).
- **`cloneRoleTemplates`** passa a setar `is_default=true` (paridade com a 0176 para contas novas).
- **Migration `0177`** seeda os 6 papéis de Fila como templates: `queue.owner/director/marketing/manager/
  store_terminal` `is_system=false` (editáveis no painel); `queue.consultant`/`queue.supervisor` continuam
  de sistema (protegidos por ON CONFLICT). Perms `workspace.*` do mesmo mapa fiel da 0176.
- **Front:** página `/manage/role-templates` (só platform_admin) + matriz binária de permissões (busca/
  filtro) + composable/types; gating em 4 pontos (workspace, permissions, nav `agencyOnly`, middleware).
- **Validado local:** go build/vet/test/golangci-lint limpos (11 testes novos); rota montada+gateada (GET
  sem auth → 401); `0177` `migration_up_ok`; type-check sem erro novo (só o quirk `~/types`). **Deploy:**
  migration 0177 + rebuild api. Verificação adversarial multiagente: **0 issues** (gate, freeze `is_system`,
  anti-escalonamento, gating do front). Pendente: validação no browser.

### Aba Páginas dava 404 para usuário sem papel + migration 0178 (2026-06-30)

**Sintoma:** a aba **Páginas** (módulo legado `access`) dava "Usuário não encontrado" (404) para a acilene,
que **existe** em `core.users`. **NÃO** foi causado pelas mudanças desta sessão. Causa: a aba Páginas usa o
módulo legado `access` → `users.FindAccessibleByID` ([users/core_projection.go](../back/internal/modules/users/core_projection.go))
cuja projeção **exige `role_code <> ''`** (queue-cêntrica). A acilene era membro da Pérola **sem nenhum papel
atribuído** (`SEM_PAPEL`) → projeção vazia → 404. Mesma família do bug do login (legado não enxerga quem não
tem papel-coarse).

**Migration `0178_backfill_perola_user_roles.sql`:** backfill de `core.user_role_assignments` para os
usuários ATIVOS da Pérola que estavam SEM papel, derivando do `job_title` (o "Perfil" que o painel mostra):
`Consultor de Atendimento`→`queue.consultant`, `Gerente de Loja`→`queue.manager`, `Diretoria`→`queue.director`,
`Terminal da Loja`→`queue.store_terminal`, `Gerente de Marketing`→`queue.marketing`, etc. **Aditivo/idempotente**:
só atribui a quem NÃO tem papel (NOT EXISTS) — não sobrescreve nada; data-driven por `slug='perola'` (sem
uuid). Validado: 7 usuários backfillados (6 Consultor + 1 Marketing), Pérola com **0 SEM_PAPEL**; a aba Páginas
para de dar 404. Nota: 1 usuário (`days.matos`) tem `job_title`=Diretoria mas papel `queue.consultant`
(divergência pré-existente — NÃO tocada, pois já tem papel; decisão do dono se corrige). Deploy: migration
0178 + rebuild api.

### Divergência de fonte usuário/papel: operação × /manage/users (fase US, pendente)

Mapeado 2026-06-30 (exploração). Hoje a gestão de usuário **NÃO é fonte única**: o PAPEL vive em **dois
lugares que saem de sync** — `core.user_role_assignments` (o "Perfil" do `/manage/users`, via projeção
[users/core_projection.go](../back/internal/modules/users/core_projection.go)) e
`queue.consultants.role_label` (o que a operação/Fila grava no create/edit do consultor). Editar consultor na
operação grava só em `queue.consultants`, **NÃO** em `core.user_role_assignments`; o `SyncLinkedAccess`
([queue/consultants/profile_sync.go](../back/internal/modules/queue/consultants/profile_sync.go)) é
uni-direcional (core→queue) e sincroniza só **identidade** (nome/loja/ativo), **não papel**. Sintoma: a
acilene era "consultora" na operação mas **sem papel no core** (404 na aba Páginas; corrigido na unha pela
0178). A projeção ainda filtra `role_code <> ''` → usuário sem papel core some do `/manage/users`.

**Escopo do gestor de cliente:** gerencia PAPÉIS da própria conta (`/v1/accounts/{id}/roles*`, gate
`core.roles.manage`, escopado por account) e consultores da própria loja — **mas `/manage/users` (CRUD de
usuário global) é SÓ platform_admin** (`/v1/admin/users*`, `requireAdminActor`). Falta uma gestão de usuário
escopada ao cliente. **Não há hierarquia de papel** ("usuários abaixo de mim") — escopo é só por account.

**Plano** (roadmap fase `users-single-source` / **US**): (1) operação grava papel no core; (2) projeção
única (core) para operação e `/manage/users`; (3) gestão de usuário escopada ao cliente; (4) hierarquia
opcional; (5) remover o legado (`queue.consultants.role_label` como fonte de papel, filtro `role_code <> ''`),
após as Fases 2-5. **Não é fonte única até a fase US** — sinalizado, não escondido como pronto.

**Deploy:** migration **0176** (idempotente, roda no boot; `migration_up_ok` confirmado local). API rebuildada.
