# OMNI-F2 — Go: schema `messaging.*` + leitura

**Prioridade: P0** · Plano canônico: [`docs/omnichannel/PLANO_ATENDIMENTO.md`](../PLANO_ATENDIMENTO.md) (§7, §9.2)

> ## LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono)
> A branch `refactor/multi-tenant-complete` fechou e o dono liberou a implementação em
> **2026-07-17**. O congelamento que valia até então **não existe mais**.

Ler antes: a skill `principios-engenharia`, o canônico §7/§9.2/§11, e
[`SPECS_PORT_OMNICHANNEL.md`](../SPECS_PORT_OMNICHANNEL.md) F2 (contratos verbatim do front).

---

## Objetivo

O schema `messaging.*` existe no Postgres e o inbox portado (F1) lista **dados reais do
banco** em vez de 404. As colunas de estado/fila/provider **já nascem aqui** — F8/F4 não
fazem ALTER. Nenhum envio, nenhum webhook, nenhum realtime: F2 é **só leitura**.

**Depende de:** nenhuma — corre `∥` F1 e F3 (canônico §9 declara as três independentes).
**Bloqueia:** F4 (webhook), F5 (realtime), F6 (envio), F8 (domínio de atendimento).

---

## Entregas

| # | Entrega | Alvo |
|---|---|---|
| 1 | Migration do schema `messaging.*` (8 tabelas do port + `messaging.outbox` + colunas novas) | `back/internal/platform/database/migrations/0200_messaging_schema.sql` |
| 2 | `module.go` — catálogo (9 permissões + 3 role templates), `Build`, `handle` | `back/internal/modules/omnichannel/module.go` |
| 3 | Rotas de leitura + `accountScope` + mapa de erros | `back/internal/modules/omnichannel/http*.go` |
| 4 | Regras de negócio (projeção `state→status`, filtro de instância) | `back/internal/modules/omnichannel/service*.go` |
| 5 | Repositório (filtro por `account_id` em **toda** query) | `back/internal/modules/omnichannel/store_postgres*.go` |
| 6 | Wire Go: `registry.MustRegister` + `moduleGatingRules` | `back/internal/platform/app/app.go` (~364+ e :518) |
| 7 | `AGENT.md` do módulo (nasce aqui — o canônico §11 confirma que não existe) | `back/internal/modules/omnichannel/AGENT.md` |

**Teto de ~450 linhas/arquivo vale integralmente** (código novo); a violação consciente do
port é só do front (canônico §14.3).

---

## Contratos

### C1 · Migration `0200_messaging_schema.sql`

**Conferido no disco:** última migration = `0199_calendar_drop_day_media.sql`; existem **dois
`0197`** (`0197_operation_validation_reason.sql` + `0197_tools_module.sql`). **0200 está
livre — reconferir o disco antes de numerar**, a numeração não é validada por ninguém.

Regras (molde: `0197_tools_module.sql`, `0187_finance_module.sql`):
- SQL **plano idempotente** (`create ... if not exists`), schema-qualificado.
- **SEM `-- +goose Down`** — o migrator roda o arquivo **inteiro** e o bloco Down se
  auto-destrói (falha real: `0147_automation_contacts_fix.sql`).
- `account_id uuid not null references core.accounts(id) on delete cascade` em **todas**.
- Cabeçalho comentando o quê/porquê + link para o plano.

**As 8 tabelas** + índices obrigatórios: `PLANO_PORT_OMNICHANNEL.md` §7 e
`SPECS_PORT_OMNICHANNEL.md` F2.1 — **não duplicados aqui**. **Fonte coluna a coluna:**
`whats-test/apps/atendimento-online-api/prisma/schema.prisma` (`WhatsAppInstance`:56,
`SavedSticker`:77, `Conversation`:92, `Message`:121, `AuditEvent`:156, `Contact`:174,
`HiddenMessageForUser`:190, `AtendimentoTenantConfig`:48). camelCase → snake_case;
`tenantId` → `account_id`.

> **Exceção única — `conversations.status` NÃO é portada como coluna.** O Prisma tem
> `status ConversationStatus @default(OPEN)` (`schema.prisma:104`), mas o canônico §7.3 decide
> que **`state` é a verdade e `status` é projeção derivada na serialização**. Coluna **e**
> projeção = dois lugares gravando o mesmo fato = duas verdades (princípio 1). A F2 **não cria**
> a coluna — e, por consequência, a F8 **não tem** o que dropar. Ver A1.

**O que é NOVO desta fase** (canônico §7.2 — nasce junto, não é ALTER depois):

```sql
-- messaging.whatsapp_instances (além das colunas do Prisma)
    provider               text not null default 'evolution'
        check (provider in ('meta_whatsapp_cloud','evolution','waha','mock')),
    provider_config        jsonb not null default '{}'::jsonb,
    credentials_ciphertext text,   -- escrito só na F3/F4 (platform/secretbox, prefixo 'v1:')

-- messaging.conversations (além das colunas do Prisma)
    state            text not null default 'new'
        check (state in ('new','ai_active','routing','queued','human_active','pending','closed')),
    department_id    uuid,          -- SEM FK aqui: alvo nasce na F8
    queue_id         uuid,          -- SEM FK aqui: alvo nasce na F8
    assigned_user_id uuid references core.users(id) on delete set null,
    extracted_fields jsonb not null default '{}'::jsonb,
```

> **`state` nasce com 7 valores — `pending` incluído (decisão do dono, 2026-07-17).** O dono
> escolheu a **opção A** do Contrato 3.1 da F8: `pending` é o **7º `state`** da máquina, escrito
> pelo 12º evento `human.pending` (`PATCH /conversations/{id}/status` → `PENDING`). É **rótulo
> manual do operador** ("parei nesta, estou esperando algo") — ortogonal ao roteamento, sem
> produtor automático. Consequência para cá: o `CHECK` **já nasce com os 7** e a **F8 NÃO faz
> `ALTER`** — vale a mesma regra das colunas do §7.2 (nascem aqui, ninguém altera depois). Quem
> escreve `pending` é a F8; a F2 só garante que a coluna o **aceita** e o projeta (ver A1).

**CHECKs dos enums** (o Prisma vira `CHECK` + tipo Go): `channel` (`WHATSAPP|INSTAGRAM`),
`direction` (`INBOUND|OUTBOUND`), `message_type` (`TEXT|IMAGE|AUDIO|VIDEO|DOCUMENT`),
`message_status` (`PENDING|SENT|FAILED`). O `status` **de conversa está fora desta lista de
propósito** — não é coluna, logo não tem `CHECK` (ver A1). `message_status` **é** coluna e
permanece.

> **`department_id`/`queue_id` nascem sem FK de propósito:** `messaging.departments`/`queues`
> só existem na F8 — declarar a FK aqui quebra a migration. A F8 adiciona a constraint, e
> `ADD CONSTRAINT IF NOT EXISTS` **não existe no Postgres**: lá precisa de `DO $$ ...
> pg_constraint ... $$` para seguir idempotente. Registrado para a F8, não para cá.
>
> `assigned_user_id` (novo, → `core.users`) **coexiste** com `assigned_to_id` (Prisma
> `assignedToId`, texto, servido ao front com esse nome). Não fundir aqui: o front verbatim lê
> `assignedToId`; quem reconcilia é a F7/F8, via máquina de estados.

**`messaging.outbox` — a nona tabela, e o dono é a F2.** Não vem do port (as 8 do Prisma não a
têm), mas o canônico **§7.1 a lista entre as tabelas `messaging.*`** e o **§9.2 manda a F2 criar
as migrations do §7** — logo ela nasce nesta migration. A fronteira é: **a tabela é da F2; o
engine é da F3** (`platform/jobs` — claim `FOR UPDATE SKIP LOCKED`, retry classificado, worker,
dead-letter, monitor de presas). A F3 roda sobre uma interface `Store` e **não cria tabela
nenhuma** (o teste de concorrência dela usa tabela efêmera própria, por isso ela segue sem
blocker). O **produtor** de job é a F6.

- **Contrato coluna a coluna + os dois índices obrigatórios:** [`OMNI-F3.md`](OMNI-F3.md) §F3.2,
  bloco "Contrato da tabela" — **remetido, não duplicado** (duplicar cria duas verdades). Quem
  fechar a F2 **confere que `messaging.outbox` satisfaz aquele contrato**: coluna faltando =
  o claim da F3 não compila e o worker não claima.
- **`idempotency_key`:** `unique (account_id, idempotency_key)` — **decisão do dono, 2026-07-17**.
  A chave vem do cliente; UNIQUE **global** deixa a conta A colidir com a chave da conta B e
  suprimir o envio dela (fere o princípio 2 — isolamento). O que era divergência da F3 com o
  canônico (`OMNI-F3.md` §Divergências #2) **virou a norma**: o **canônico §7.1 muda** e deixa de
  escrever "`idempotency_key` UNIQUE" global. Não é mais ponto aberto — não reabrir. Como o
  UNIQUE global saiu, **prefixar a chave com o `account_id` deixou de fazer sentido**: a
  unicidade já é por conta, a chave vai crua no banco.
- F2 **não** implementa worker, claim, retry nem handler. Só o DDL.

### C2 · Rotas de leitura

Prefixo `/v1/omnichannel`. Mapa Node→Go: `PLANO_PORT_OMNICHANNEL.md` §8.
Lista exata + paginação: `SPECS_PORT_OMNICHANNEL.md` F2.3 — **remetido, não duplicado**.

| Rota | Nota desta fase |
|---|---|
| `GET /conversations` | `instanceId?`; ordena `last_message_at DESC`; **sem paginação** |
| `GET /conversations/{id}/messages` | **`limit` (1..200, default 100) + `beforeId` — NÃO cursor.** Contrato campo a campo em `SPECS_PORT` F2.3 |
| `GET /conversations/{cid}/messages/{mid}` | |
| `GET/POST /contacts` · `PATCH /contacts/{id}` | |
| `GET/PATCH /account` | Shape = `mapTenantResponse` (ver C4) |
| `GET /whatsapp/instances` · `GET /whatsapp/instances/access` | Filtro corrigido (ver A2) |

**Shapes:** o contrato é `web-reference/app/types/index.ts` (`Message`:93, `Conversation`:149,
`Contact`:134, `WhatsAppInstanceRecord`:39, `TenantSettings`:14). JSON camelCase.
Divergir um campo quebra o front.

> **`Message` e `Contact` têm `tenantId: string` OBRIGATÓRIO** (`types/index.ts:95` e `:136`);
> `Conversation` **não tem**. Serializar `account_id → tenantId` nesses dois — omitir quebra
> a tipagem do front verbatim.

### C3 · `module.go`

Molde: `back/internal/modules/calendar/module.go` (`ID`/`Metadata`/`Permissions`/
`RoleTemplates`/`Build` → `handle` com `RegisterRoutes`/`RegisterEventHandlers`/`Close`).

| Item | Valor |
|---|---|
| `ID()` | `"omnichannel"` |
| `Metadata()` | `SchemaName: "messaging"`, `Label: "Omnichannel"`, `IsCore: false`, `SortOrder: 47` (livres no disco: 45=bio, 46=calendar, 50=automation/finance) |
| `Permissions()` | As **9 keys** do canônico §5.2, `Scope: "account"` |
| `RoleTemplates()` | `attendant` · `supervisor` · `manager` (canônico §5.2) |
| `Build(deps)` | `deps.Pool` + `deps.Logger` + `deps.AuthMiddleware`. **F2 não precisa de env var nenhuma** |

**Wire** (`app.go`): `registry.MustRegister(omnichannel.New())` (~364+) e
`moduleGatingRules()` += `{Prefix: "/v1/omnichannel", ModuleID: "omnichannel"}` (:518,
`httpapi.ModulePathRule{Prefix, ModuleID}`). `SyncCatalog` no boot seeda as permissões **e**
auto-habilita nas contas `is_agency` (`catalog_postgres.go:147`) — o módulo aparece sozinho
para a agência; por cliente é `PUT /v1/admin/accounts/{id}/modules` (`core/admin_http.go:31`).

> **Se a F1 já criou `module.go`** só com catálogo (precedente `queue`/`crm`: declaram
> catálogo e o `Build` devolve handle **sem rotas**, confirmado em `app.go`), a F2 **estende**
> esse arquivo — não cria um segundo nem duplica o catálogo.

### C4 · `GET /account` — o ponto onde a fusão morde

Shape verbatim = `whats-test/.../src/routes/tenant/tenant-response-mapper.ts`
(`mapTenantResponse`). Mas **três campos mudam de fonte** por causa da plataforma e da D-A:

| Campo | Legado | **Aqui** |
|---|---|---|
| `id` · `slug` · `name` | tabela `tenant` | **`core.accounts`** — nunca duplicar em `messaging.*` |
| `maxChannels` · `maxUsers` | colunas do tenant | **`core.account_modules.config jsonb`** (`max_whatsapp_numbers`, canônico §5.3; coluna já existe em `0100_core_schema.sql:120`). Default em `core.platform_settings`. **Sem migration nova** |
| `hasEvolutionApiKey` | `Boolean(env.EVOLUTION_API_KEY)` — global do ambiente | Derivar de `credentials_ciphertext is not null` na instância. Env global não sobrevive à D-A (multi-provider por conta/número) |

`retentionDays`/`maxUploadMb` vêm de `messaging.account_config` (Prisma
`AtendimentoTenantConfig`: defaults **15** e **500**, os do legado — a retenção por classe do
canônico §10 é **F13**, não confundir). `evolutionApiKey` responde **sempre `null`** (o legado
já faz isso — manter).

---

## Armadilhas / o que NÃO fazer

**A1 · `status` não é coluna — e a projeção tem TRÊS valores, não dois.** Duas coisas, nesta
ordem:

1. **Não criar a coluna.** Canônico §7.3: `state` é a verdade, `status` é **derivado na
   serialização**. O Prisma tem `status` (`schema.prisma:104`), mas ele **não é portado** — sem
   coluna, sem `CHECK`, sem `default` (ver C1). Quem grava ciclo de vida grava `state`.
2. **A projeção tem os TRÊS valores — `PENDING` está decidido.** O contrato do front é
   `ConversationStatus = "OPEN" | "PENDING" | "CLOSED"` (`web-reference/app/types/index.ts:91`)
   e ele **renderiza o caso PENDING** (`InboxConversationsSidebar.vue:316,328`,
   `InboxDetailsSidebar.vue:92,104`, `useInboxChatPresentation.ts:48,60`). O **tipo Go de saída**
   aceita os três e o serializador nunca emite string fora dessa lista.

**A projeção desta fase é `state → status` com três saídas** — `pending → PENDING` entra junto
com `OPEN`/`CLOSED`. **Não é mais lacuna aberta e a F8 não decide isso**: o dono decidiu em
**2026-07-17** (opção A do Contrato 3.1 da F8 — `pending` é o 7º `state`, ver C1) e o canônico
**§7.2/§7.3 ganharam a linha `pending → PENDING`**. A F2 já nasce com o `CHECK` dos 7 valores e
com a projeção dos 3. O que continua sendo da F8 são as **transições** (quem escreve `pending`,
via `human.pending`) — não a existência do estado nem o mapa da projeção.

**A2 · O filtro de instância por usuário está QUEBRADO no legado — portar corrigido.**
`whats-test/.../src/services/whatsapp-instances.ts:681-683`:
```ts
const accessibleInstances = isTenantAdmin || activeInstances.length <= 1
    ? activeInstances
    : activeInstances;   // <- os dois ramos devolvem a MESMA coisa
```
Ou seja: **todo usuário vê todas as instâncias**. É isolamento (princípio 2) e o port §8 manda
corrigir. O único vínculo por usuário que **existe de fato** é
`whatsapp_instances.responsible_user_id` (Prisma `:66`, indexado por `[tenantId, responsibleUserId]`).
Regra desta fase: não-admin vê `responsible_user_id is null or responsible_user_id = <principal>`.
**O gate de dado definitivo é `queue_members`, e chega na F8** (canônico §5.2) — não inventar
um segundo gate aqui.

**A3 · `assignedUserIds` e `userScopePolicy` não têm fonte.** No legado são **constantes
hardcoded** (`assignedUserIds: []`, `userScopePolicy: "MULTI_INSTANCE"` —
`routes-whatsapp-instances.ts:41,48,58,268,275,393,400`): não há tabela por trás. Emitir os
campos com os mesmos valores fixos (o front os tipa) e **registrar em `docs/LEGADO.md`** como
vestígio (princípio 4) — não inventar tabela para eles nesta fase.

**A4 · Não criar tabela de outra fase.** `webhook_events` é **F4**;
`departments`/`queues`/`queue_members`/`routing_rules`/`routing_decisions`/`ai_*` são
**F8/F9** (canônico §7.1). F2 = as 8 do port + **`outbox`** (C1) + as colunas do §7.2. Nada além.
**`outbox` é a exceção deliberada, e é dela que ninguém pode fugir:** a tabela é daqui — se a F2
não a criar, ninguém cria (a F3 é engine sobre `Store`, a F6 é só produtora) e a F6 para. O que
**não** se faz aqui é o worker/claim/retry: isso é F3.

**A5 · Não escrever `credentials_ciphertext`.** A coluna nasce aqui e fica **vazia** — quem
cifra é `platform/secretbox` (**F3**). Gravar chave crua nela repete exatamente o gap do
`calendar/secrets.go` (grava a chave em texto puro; `{set,last4}` é máscara de **saída**, não
cifragem) — o gap que este plano existe para não repetir.

**A6 · Não importar pacote de uuid.** Padrão da casa: `string` + cast no SQL
(`where account_id = $1::uuid`, como `calendar/store_postgres.go:74`). Coluna nullable →
scan em `*string`, nunca no tipo puro.

---

## Segurança

| Regra | Como, aqui |
|---|---|
| `account_id` **sempre** do Principal | Helper `accountScope(r)` no molde de `calendar/http.go:314` (lê `X-Account-Id`, cai para `principal.TenantID`, vazio → 403 `no_account`). **Nunca do body** — nem em `POST /contacts`, nem em `PATCH` |
| Repositório filtra por conta **também** | `where account_id = $1::uuid` em **toda** query, inclusive nas que já receberam id validado no service (defesa em profundidade, princípio 2) |
| Fora de escopo → **404** | `GET /conversations/{id}` de outra conta = `not_found`. **Nunca 403** — 403 confirma que o recurso existe (enumeration) |
| Gate de módulo | `moduleGatingRules` → conta sem o módulo leva 403 `module_disabled`; `platform_admin` tem bypass |
| Não logar payload | Log estruturado com campos explícitos (`operação`, `account_id`, `user_id`, `error`). Nunca a struct inteira interpolada |

> **Gap honesto — não existe middleware de permissão no Go.** No disco só há `RequireAuth` /
> `RequireAuthWithAccount` / `RequireRoles` (`auth/middleware.go:32,81,50`) e
> `RequireModuleByPath` (`httpapi/account_guard.go:118`); `modules.Dependencies` **não expõe**
> serviço de permissões (`ResolveEffectivePermissions` vive no módulo `access`). Hoje as keys
> `omnichannel.*` gateiam o **front**; a F2 as **declara** (para o gating da F1 funcionar) e
> protege as rotas com `RequireAuth` + gate de módulo + escopo de conta. Enforcement por key
> vira load-bearing na **escrita** (F6/F7) — decidir lá entre middleware novo ou checagem no
> service. **Não fingir que a key protege a rota agora.** No front, `platform_admin` tem
> `has()` = false: todo gating precisa de `isPlatformAdmin || has(...)`.

---

## Verificável

Prova no browser/banco, por um humano — não "os testes passam":

1. `docker compose exec postgres psql -U omni -d omni -c "\dt messaging.*"` lista as **9
   tabelas** (as 8 do port + `outbox`) (local = `omni`/`omni`, `docker-compose.yml:10-11`; na
   VPS o user/db é `listaatendimento`). `\d messaging.conversations` mostra `state`
   (com o `CHECK` dos **7** valores, `pending` incluído — se faltar, a F8 vai precisar do `ALTER`
   que esta fase existe para evitar), `department_id`, `queue_id`, `assigned_user_id`,
   `extracted_fields`;
   `\d messaging.whatsapp_instances` mostra `provider`, `provider_config`,
   `credentials_ciphertext`; `\d messaging.outbox` mostra `ordering_key`, `idempotency_key`
   (`unique (account_id, idempotency_key)`), `status`, `attempts`, `run_after`, `locked_at` e
   os dois índices do contrato de F3.2.
2. `migrate status` chega em **0200** (se parar em 0199 sem erro → é o cache do embed.FS,
   ver Notas de Deploy).
3. Logado com conta que tem o módulo, `/omnichannel` abre e o inbox lista **vazio, do banco**
   — sem 404 no console (a F1 devolvia 404 em tudo).
4. `insert` de uma conversa **na mão** no banco (com o `account_id` da conta ativa) → dá
   refresh → **a conversa aparece na tela**. É a prova de que o dado vem do banco.
5. Repetir o `GET /v1/omnichannel/conversations` trocando **só** o `X-Account-Id` para outra
   conta → a conversa **não aparece**. `GET /conversations/{id}` dela → **404**, não 403.
6. Conta **sem** o módulo habilitado → 403 `module_disabled`.
7. `GET /whatsapp/instances` com usuário não-admin que não é `responsible_user_id` de
   nenhuma instância → **não** lista todas (prova de que A2 foi corrigido, não portado).

---

## Notas de Deploy

**Ordem exata:** migration → **build `--no-cache` da api** → subir a api. Sem env var nova,
sem container novo, sem Caddy nesta fase.

| # | Item | Comando / detalhe |
|---|---|---|
| 1 | Migration `0200_messaging_schema.sql` | Idempotente, **sem `-- +goose Down`**. Roda no boot da api |
| 2 | **Rebuild da api** | `docker compose build --no-cache api` **e depois** `docker compose up -d api` |
| 3 | Env vars | **Nenhuma.** `OMNI_SECRETS_KEY` é **F3**; `EVOLUTION_*` é **F4** (canônico §13) |

> **Armadilha que já queimou tempo:** as migrations são **`embed.FS`**. `up -d --build api`
> pode **reusar a camada do `go build`** e **não re-embutir** o `.sql` novo. Sintoma: `migrate
> status` para na migration anterior, **sem erro**. Cura: `docker compose build --no-cache api`.
> Portas são fixas (api=9091) — não alterar.
