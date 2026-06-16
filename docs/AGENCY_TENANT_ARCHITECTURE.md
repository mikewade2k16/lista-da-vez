# Arquitetura Agência → Clientes (tenants) — plano de execução

> Status: **CONCLUÍDO ✅ (2026-06-15).** Etapas 1-5 entregues e verificadas: board geral de Tasks (247)
> agora é da conta-agência `crow`; admins membros da crow + agency_owner → defaultam na agência;
> `dono==cliente=0`; isolamento intacto (cliente→crow=f); sem perda de dados (total 247).
> Documento canônico desta mudança.
> Origem: 2026-06-10, ao corrigir o "dono" do board de Tasks. Revisado em 2026-06-15 ao
> confrontar o estado descrito com o código vivo — **boa parte já estava construída**.
> Roadmap: fase `agency-tenant` (AT) em `web/app/components/roadmap/roadmap-data.ts`.
> Ver também [LEGADO.md](LEGADO.md) §4 e a memória `project_tasks_client_source`.

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
