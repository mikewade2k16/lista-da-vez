# OMNI-F8 — Domínio de atendimento (setores/filas/roteamento) · **P0**

Plano canônico: `docs/omnichannel/PLANO_ATENDIMENTO.md` (§7.2, §7.3, §9.2 F8, §12 risco 4).
Molde e convenções: `docs/omnichannel/SPECS_PORT_OMNICHANNEL.md`. Ler `principios-engenharia` antes.

> ## LIBERADO PARA IMPLEMENTAÇÃO — 2026-07-17 (decisão do dono)
> A branch `refactor/multi-tenant-complete` fechou e o dono **liberou a implementação em
> 2026-07-17**. O congelamento que bloqueava esta fase **não existe mais**. O que resta são os
> *blockers* técnicos entre fases — para a F8, a **F2** (ver "Depende de").

---

## Objetivo

Uma conversa que entra é roteada por **regra determinística** até uma fila, o atendente vê
**só** o que é dele, cada decisão fica auditável em `routing_decisions`, e o front verbatim
continua vendo o `status` que ele já conhece — sem saber que existe uma máquina de estados
embaixo. `conversations.state` passa a ser a **única** fonte de verdade do ciclo de vida.

> ## `PENDING` — **DECIDIDO** (dono, 2026-07-17)
> O front verbatim tem **três** valores de `status` (`OPEN|PENDING|CLOSED`) e **grava** os três.
> A F1 delegou a decisão a esta spec e pediu que fosse reportada ao dono; o dono escolheu a
> **opção A**: `pending` é o **7º `state`**, escrito pelo **12º evento `human.pending`**, e
> projeta `pending → PENDING`. **A F8 está executável.** Ver **Contrato 3.1**.

## Depende de / Bloqueia

| | |
|---|---|
| **Depende de** | **F2** (schema `messaging.*` + leitura). As colunas `state`, `department_id`, `queue_id`, `assigned_user_id`, `extracted_fields` **já nascem na migration da F2** (canônico §7.2) — a F8 **não faz ALTER** para criá-las |
| **Bloqueia** | **F7** (ações passam pela máquina), **F9** (triagem: `AIAllowed` + `extracted_fields`), **F10** (telas de config consomem estas rotas) |
| **Paralelismo** | Pode correr **∥ F4–F7** — o domínio não depende do canal (canônico §9) |

## Entregas

| # | Entrega | Alvo |
|---|---|---|
| 1 | Migration das 5 tabelas novas | `back/internal/platform/database/migrations/<próximo livre>_messaging_service_domain.sql` — **`0200` é da F2**, ver Contrato 1 |
| 2 | Máquina de estados + projeção `state→status` | `back/internal/modules/omnichannel/state.go` |
| 3 | Motor de roteamento determinístico (sem LLM) | `.../omnichannel/routing.go` |
| 4 | Service: `Transition`, `AIAllowed`, CRUD de setores/filas/membros/regras | `.../omnichannel/service_domain.go` |
| 5 | Repositório: filtro de visibilidade por fila + persistência | `.../omnichannel/store_postgres_domain.go` |
| 6 | Rotas de config + `PATCH /conversations/{id}/queue` (registrar em `module.go`) | `.../omnichannel/http_domain.go` |
| 7 | Conferir que `Permissions()` do módulo declara `omnichannel.settings.manage` e `.conversations.assign` (canônico §5.2) | `.../omnichannel/module.go` |
| 8 | Testes tabela-driven: **os 84 pares** da matriz + motor sem chamar modelo | `state_test.go`, `routing_test.go` |
| 9 | Sincronizar os 3 docs ao fechar | `.../omnichannel/AGENT.md` · `docs/omnichannel/PLANO_ATENDIMENTO.md` · `web/app/components/roadmap/data/phases-part7.ts` |

Teto de ~450 linhas/arquivo **vale** (é código novo).

---

## Contrato 1 — Migration (próximo número livre)

**Número: o próximo livre no disco na hora de escrever — NÃO `0200`.** A **F2 já reivindica
`0200_messaging_schema.sql`**, e a F8 **depende da F2** (roda depois): cravar `0200` aqui é
colisão garantida. Hoje a última no disco é `0199_calendar_drop_day_media.sql`, o que faz o
candidato natural ser **`0201_messaging_service_domain.sql`** — mas **F3/F4 podem entrar no
meio**, então **conferir o disco antes de numerar**, não presumir. Que isso não é teoria: há
**dois arquivos `0197`** (`0197_operation_validation_reason.sql`, `0197_tools_module.sql`) —
a numeração não é validada por ninguém.

SQL plano **idempotente**, schema-qualificado, **sem `-- +goose Down`** (o migrator roda o
arquivo inteiro e o Down se auto-destrói). Modelo de estilo: `0191_calendar_chat_conversations.sql`.

```sql
create schema if not exists messaging;

create table if not exists messaging.departments (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    slug text not null, name text not null,
    is_default boolean not null default false, is_active boolean not null default true,
    created_at timestamptz not null default now(), updated_at timestamptz not null default now()
);
create unique index if not exists messaging_departments_slug_uk on messaging.departments (account_id, slug);
create unique index if not exists messaging_departments_default_uk on messaging.departments (account_id) where is_default;

create table if not exists messaging.queues (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    department_id uuid not null references messaging.departments(id) on delete cascade,
    slug text not null, name text not null,
    is_default boolean not null default false, is_active boolean not null default true,
    created_at timestamptz not null default now(), updated_at timestamptz not null default now()
);
create unique index if not exists messaging_queues_slug_uk on messaging.queues (account_id, department_id, slug);
create unique index if not exists messaging_queues_default_uk on messaging.queues (account_id, department_id) where is_default;

-- queue_members E O GATE DE DADO (canonico 5.2): quem nao esta aqui nao ve a conversa.
create table if not exists messaging.queue_members (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    queue_id uuid not null references messaging.queues(id) on delete cascade,
    user_id uuid not null references core.users(id) on delete cascade,
    is_active boolean not null default true, created_at timestamptz not null default now()
);
create unique index if not exists messaging_queue_members_uk on messaging.queue_members (queue_id, user_id);
-- indice do hot path: o filtro de visibilidade roda em TODA leitura de conversa.
create index if not exists messaging_queue_members_user_idx on messaging.queue_members (user_id, account_id) where is_active;

create table if not exists messaging.routing_rules (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    name text not null, priority integer not null default 100, is_active boolean not null default true,
    conditions jsonb not null default '[]'::jsonb,
    target_queue_id uuid not null references messaging.queues(id) on delete cascade,
    created_at timestamptz not null default now(), updated_at timestamptz not null default now()
);
-- ordem de avaliacao: priority asc, id asc (desempate estavel => decisao reproduzivel).
create index if not exists messaging_routing_rules_eval_idx on messaging.routing_rules (account_id, priority, id) where is_active;

-- routing_decisions: auditoria de CADA decisao. Sem isso o roteamento e uma caixa preta.
create table if not exists messaging.routing_decisions (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    conversation_id uuid not null references messaging.conversations(id) on delete cascade,
    rule_id uuid references messaging.routing_rules(id) on delete set null,
    outcome text not null, reason text not null default '',
    input jsonb not null default '{}'::jsonb,
    target_department_id uuid references messaging.departments(id) on delete set null,
    target_queue_id uuid references messaging.queues(id) on delete set null,
    ai_run_id uuid, decided_at timestamptz not null default now(),
    constraint messaging_routing_decisions_outcome_ck
        check (outcome in ('matched','default_queue','unrouted','manual_transfer','ai_failed'))
);
create index if not exists messaging_routing_decisions_conv_idx on messaging.routing_decisions (conversation_id, decided_at desc);
```

- `ai_run_id` fica **sem FK** aqui: `ai_runs` nasce na **F9**, que adiciona a constraint.
- `input` = snapshot do que foi avaliado (`extracted_fields` + contexto). É o que torna a
  decisão explicável meses depois, sem re-rodar nada.
- **`conversations.status` não é coluna** — é projeção (§7.3 do canônico), e **a F2 já não a
  cria** (`OMNI-F2.md`, C1 e A1: a coluna `status` do Prisma não é portada). Logo **esta
  migration não dropa nada** — não há `drop column status` aqui. A invariante que as duas specs
  sustentam é a mesma: coluna **e** projeção = dois lugares gravando o mesmo fato = duas
  verdades (princípio 1). Se, ao escrever esta fase, a migration da F2 no disco **tiver** a
  coluna, isso é **divergência da F2 com o canônico §7.3 — corrigir lá**, não compensar aqui
  com um drop.
- O `CHECK` de `state` com os **7 valores** (`new`, `ai_active`, `routing`, `queued`,
  `human_active`, **`pending`**, `closed`) **nasce na F2** — `pending` incluído desde o início
  (Contrato 3.1, decisão do dono de **2026-07-17**). **A F8 não faz `ALTER`**: nem para criar o
  `CHECK`, nem para acrescentar o `pending` a um `CHECK` de 6 valores. Se, ao escrever esta fase,
  o `CHECK` no disco estiver ausente ou com 6 valores, isso é **divergência da F2 com o canônico
  §7.2 — corrigir lá**, exatamente como no bullet acima sobre a coluna `status`. A F2 já está
  sincronizada com a decisão (`OMNI-F2.md`, C1).

## Contrato 2 — Máquina de estados (`state.go`)

Estados (fonte de verdade — **7**): `new` · `ai_active` · `routing` · `queued` · `human_active` ·
**`pending`** · `closed`.

Eventos (**12**): `msg.inbound` · `msg.outbound.human` · `ai.triage.done` · `ai.triage.failed` ·
`route.matched` · `route.unmatched` · `human.assign` · `human.unassign` · **`human.pending`** ·
`queue.transfer` · `conv.close` · `conv.reopen`.

`pending` e `human.pending` entram por **decisão do dono de 2026-07-17** (opção A do
Contrato 3.1) — não re-decidir.

**Matriz completa — 7 estados × 12 eventos = 84 pares, nenhum implícito.**
Célula = estado destino · `self` = aceita e não muda de estado · `no-op` = aceita, não muda
nada (200) · `—` = **rejeita** `invalid_transition` (**409**).

| De \ Evento | `msg.inbound` | `msg.outbound.human` | `ai.triage.done` | `ai.triage.failed` | `route.matched` | `route.unmatched` | `human.assign` | `human.unassign` | `human.pending` | `queue.transfer` | `conv.close` | `conv.reopen` |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| `new` | `ai_active`¹ | `human_active`² | — | — | — | — | `human_active` | — | `pending`¹⁰ | —⁰ | `closed` | — |
| `ai_active` | self³ | `human_active`² | `routing` | `routing`⁴ | — | — | `human_active` | — | `pending`¹¹ | `queued`⁵ | `closed` | — |
| `routing` | self³ | `human_active`² | no-op⁶ | no-op⁶ | `queued` | `queued`⁷ | `human_active` | — | `pending`¹¹ | `queued`⁵ | `closed` | — |
| `queued` | self³ | `human_active`² | no-op⁶ | no-op⁶ | — | — | `human_active`⁸ | — | `pending`¹⁰ | self⁵ | `closed` | — |
| `human_active` | self³ | self | no-op⁶ | no-op⁶ | — | — | self⁸ | `queued`⁹ | `pending`¹⁰ | `queued`⁵ | `closed` | — |
| `pending` | self³ | `human_active`² | no-op⁶ | no-op⁶ | — | — | `human_active`⁸ | `queued`¹³ | no-op¹² | `queued`⁵ | `closed` | — |
| `closed` | `ai_active`¹ | `human_active`² | no-op⁶ | no-op⁶ | — | — | — | — | —¹² | — | no-op | `routing` |

0. **Decisão desta spec:** `queue.transfer` em `new` é rejeitado — conversa não triada não tem
   o que transferir; o supervisor usa `human.assign` ou espera o motor. Se o dono quiser
   permitir, muda **aqui**, não no código.
1. `ai_active` **se** houver `ai_agent` ativo para o número (F9); **senão** `routing` direto —
   sem agente configurado a conversa roteia igual, não trava. Vindo de `closed` (reabertura),
   zera `assigned_user_id` e re-roteia do zero.
2. Atendente respondeu = **tomou a conversa**: `assigned_user_id` = autor, e a IA cala a partir
   dali (hard-block).
3. `self` = atualiza `last_message_at`; estado não muda; **não re-dispara triagem** (já ocorreu).
   Em `ai_active`, guarda de debounce: não abre run novo com run em voo. **Em `pending`, `self` é
   decisão do dono (2026-07-17):** o rótulo é do **operador** e o cliente **não** o limpa. Isto
   **diverge de propósito do legado**, que grava `OPEN` no inbound
   (`webhooks/handlers/message-upsert/events.ts:40`, ver Contrato 3.1) — a divergência é
   deliberada, **não é bug a "consertar"**.
4. Falha da IA **não trava** a conversa: vai para `routing` com `extracted_fields` vazio e a
   decisão grava `outcome='ai_failed'`. Fail-open para a fila default — nunca "some".
5. Transferência manual (supervisor): `assigned_user_id = null`, `queue_id` = destino, decisão
   `outcome='manual_transfer'`. Saindo de `ai_active`, cancela o run em voo.
6. Resultado de run **tardio** (a conversa já saiu do `ai_active`): grava em `ai_runs` (F9),
   **não** muda estado, **não** re-roteia. É corrida normal, não erro — responde 200.
7. Nenhuma regra casou → fila default do setor default (`outcome='default_queue'`). **Sem
   default configurado** → `queued` com `queue_id NULL` e `outcome='unrouted'` (ver Contrato 4).
8. Guarda: o usuário destino é `queue_member` **ativo** da fila da conversa **ou** tem
   `omnichannel.settings.manage`. Fora disso → **404** do usuário destino (não 403).
9. Devolver para a fila: `assigned_user_id = null`. Se `queue_id is null` (foi atribuída direto
   do `new`), vai para `routing` e re-roteia — não vira órfã.
10. `human.pending` = **rótulo manual do operador** (evidência no Contrato 3.1), **ortogonal ao
    roteamento**: **preserva `assigned_user_id`** (não atribui e não limpa) e **preserva
    `queue_id`** — igual ao legado. Em `new` é **aceito** pelo mesmo critério que o `new` já
    aceita `human.assign` e `conv.close` do operador: rejeitar só o rótulo seria incoerente. Se
    uma conversa marcada em `new` for devolvida depois (`human.unassign`), a nota 13 cai na nota 9
    e ela re-roteia — não vira órfã. **Decisão desta spec (2026-07-17).**
11. Saindo de `ai_active`/`routing`, `human.pending` **cancela o run em voo** (mesma mecânica da
    nota 5) e `AIAllowed(pending)` = **false** — a IA cala, igual à nota 2. Resultado tardio do
    run cai na nota 6 (no-op). Decisão de roteamento tardia bate em `route.matched`/
    `route.unmatched` = `—`: é a **mesma** corrida que `queued`, `human_active` e `closed` já têm
    hoje, resolvida com o **mesmo** critério — o `pending` não inventa caso novo.
    **Decisão desta spec (2026-07-17).**
12. `human.pending` em `pending` = **no-op** (200): marcar duas vezes não muda nada — mesmo
    critério do `conv.close` em `closed`. Em `closed` é **rejeitado**: `PENDING` é rótulo de
    conversa **viva**, e aceitá-lo aqui reabriria a conversa **em silêncio**, sem zerar
    `assigned_user_id` e sem re-rotear — o oposto exato da nota 1. Para marcar uma conversa
    fechada, reabrir antes (`conv.reopen`); o 409 traz mensagem acionável (princípio 5).
    **Decisão desta spec (2026-07-17).**
13. `human.unassign` em `pending` → `queued` **pela nota 9 inteira** (inclusive o `queue_id is
    null` → `routing` + re-roteia). O rótulo é **do operador**: devolver a conversa para a fila
    derruba o rótulo junto — parada **e** sem dono é exatamente a conversa órfã que o princípio 5
    proíbe (ninguém volta nela e ela some dos dois filtros). **Decisão desta spec (2026-07-17).**

**Concorrência:** toda transição roda em **uma transação** com
`select ... from messaging.conversations where id = $1 and account_id = $2 for update`.
Sem o lock, `msg.inbound` e `human.assign` concorrentes fazem lost update — é o risco 5 do
canônico na versão do domínio.

```go
func Apply(from State, ev Event, tc TransitionContext) (Outcome, error) // ErrInvalidTransition -> 409
type TransitionContext struct { HasActiveAgent bool; HasQueue bool }    // resolvem as notas 1 e 9
type Outcome struct { To State; NoOp bool }

func (s *Service) Transition(ctx context.Context, p auth.Principal, convID string, ev Event, payload TransitionPayload) (Conversation, error)
func AIAllowed(s State) bool // true SOMENTE em `new` e `ai_active` — o hard-block do canonico §6
```

`AIAllowed` substitui o `paused_until` da spec externa: timer expira sozinho e o bot volta a
falar por cima do atendente; **estado é mais honesto que timer** (canônico §6).

## Contrato 3 — Projeção `state → status` (risco 4 do canônico)

O front verbatim **não pode ser tocado** (D-B). `status` é **derivado na serialização**, nunca
lido do banco:

| `state` (verdade) | `status` | `assignedTo` |
|---|---|---|
| `new` · `ai_active` · `routing` · `queued` | `OPEN` | `null` |
| `human_active` | `OPEN` | `assigned_user_id` |
| **`pending`** | **`PENDING`** | preserva o `assigned_user_id` que houver (a nota 10 não mexe nele; pode ser `null`) |
| `closed` | `CLOSED` | preserva o último `assigned_user_id` |

**Projeção inversa** — o front verbatim fala `status`; o back traduz para evento. Os shapes de
body são os do legado (`PLANO_PORT_OMNICHANNEL.md` §8) — **não redefinir aqui**:

| Chamada do front | Evento |
|---|---|
| `PATCH /conversations/{id}/status` → `CLOSED` | `conv.close` |
| `PATCH /conversations/{id}/status` → `OPEN` numa conversa `closed` | `conv.reopen` |
| `PATCH /conversations/{id}/status` → `OPEN` numa conversa já aberta | no-op (200, devolve a conversa) |
| `PATCH /conversations/{id}/status` → **`PENDING`** | **`human.pending`** (dono, 2026-07-17 — Contrato 3.1) |
| `PATCH /conversations/{id}/assign` com usuário | `human.assign` |
| `PATCH /conversations/{id}/assign` limpando o usuário | `human.unassign` |
| `POST /conversations/{id}/messages` (F6) | `msg.outbound.human` |
| webhook inbound (F4) | `msg.inbound` |

## Contrato 3.1 — `PENDING`: **DECIDIDA — opção A** (decisão do dono, 2026-07-17)

**`pending` é o 7º `state` da máquina**, escrito pelo **12º evento `human.pending`**
(`PATCH /conversations/{id}/status` → `PENDING`), e projeta `pending → PENDING`.

A F1 registrou o gap e delegou aqui (`OMNI-F1.md:133-136`); o canônico §7.3 não mapeava
`PENDING`; o §12 risco 4 era exatamente isto. Sem decisão a F8 não fechava — o filtro
"Pendentes" devolveria zero para sempre e o botão "Pendente" gravaria um valor que a máquina
rejeita. **O dono decidiu em 2026-07-17. Está resolvido — não re-decidir.** A evidência abaixo
fica registrada porque é ela que sustenta *por que* a opção A venceu.

**O front exige três valores — verificado no disco (não é suposição):**

| Fato | Evidência |
|---|---|
| `status` tem 3 valores | `web-reference/app/types/index.ts:91` — `ConversationStatus = "OPEN" \| "PENDING" \| "CLOSED"` |
| `PENDING` é **gravável**, não só filtro | `useOmnichannelInbox.ts:140` — `{ label: "Pendente", value: "PENDING" }` em `statusActionItems` (o de `:134` é o filtro) |
| O botão faz PATCH | `useOmnichannelInboxConversationActions.ts:16,28-31` — `updateConversationStatus(statusValue)` → `PATCH /conversations/{id}/status` `{ status }` |
| O legado aceita os 3 | `whats-test/apps/atendimento-online-api/prisma/schema.prisma:15-19` (`enum ConversationStatus { OPEN PENDING CLOSED }`) + `src/routes/conversations/schemas.ts:55-57` (`z.nativeEnum`) |

**O que o legado faz com `PENDING` (decisivo — verificado):** *nada automático.* Todo write de
status feito por lógica é `OPEN` (`webhooks/handlers/message-upsert/events.ts:40` e
`context.ts:458`, `routes-message-write-send.ts:185`, `routes-message-write-forward.ts:254`,
`contacts.ts:1117`). O **único** ponto que grava `PENDING` é o PATCH manual do operador
(`routes-operational-status.ts:123-125`). Ou seja: **`PENDING` é rótulo do operador** ("parei
nesta, estou esperando algo"), sem produtor automático e sem limpeza automática — e responder
na conversa devolve ela para `OPEN` (`routes-message-write-send.ts:185`).

> **Por isso o candidato herdado da F1 (`queued` → `PENDING`) está descartado com evidência** —
> proposto por esta spec e **ratificado pelo dono em 2026-07-17** (D-E).
> `queued` é produzido pelo **motor**, não pelo operador: toda conversa roteada cai em `queued`.
> Mapear `queued → PENDING` faria o filtro "Abertos" ficar quase vazio e o "Pendentes" listar
> tudo que está em fila — inclusive o que ninguém marcou. Seria trocar "sempre zero" por
> "sempre tudo". `PENDING` é **ortogonal** ao roteamento.

### Por que a opção A venceu (as três foram avaliadas — 2026-07-17)

| # | Opção | Veredito |
|---|---|---|
| **A** | **7º `state` = `pending`** + 12º evento `human.pending` | **ESCOLHIDA (dono, 2026-07-17).** Mantém o front **verbatim** (D-B) **e** `state` como fonte única do ciclo de vida (princípio 1) — é a única que não sacrifica um dos dois. Custo aceito: canônico §7.2/§7.3, `CHECK` de 7 valores na **F2**, e a matriz em **7 × 12 = 84 pares** |
| **B** | Coluna/flag separada (`is_pending`) fora da máquina | **Descartada:** `state` deixaria de ser a única verdade do ciclo de vida — dois lugares descrevendo "em que pé está a conversa" fere o princípio 1 |
| **C** | Remover a ação "Pendente" da UI | **Descartada:** fere a **D-B** (o front deixa de ser verbatim) e remove funcionalidade existente (princípio 3) |

**O 409 `invalid_transition` interino do `PATCH status → PENDING` morreu**: era o comportamento
honesto *enquanto não havia decisão*; agora `PENDING` é **transição válida** pelo evento
`human.pending`. (O 409 continua vivo, e correto, para os pares `—` da matriz — ex.:
`human.pending` numa conversa `closed`, nota 12.)

### Consequências da decisão — onde cada uma foi aplicada

| Onde | O quê | Estado |
|---|---|---|
| **Contrato 2** (matriz) | linha `pending` + coluna `human.pending` = **84 pares**, nenhum implícito (notas 10-13) | **aplicado aqui** |
| **Contrato 2** (nota 3) | `msg.inbound` em `pending` = `self` | **aplicado aqui** |
| **Contrato 3** (projeção) | `pending → PENDING` e `PATCH status → PENDING` = `human.pending` | **aplicado aqui** |
| **Entrega 8** · **Verificável 8** | contadores 66 → **84** | **aplicado aqui** |
| **Contrato 1** | o `CHECK` de 7 valores nasce na F2 — **a F8 não faz `ALTER`** | **aplicado aqui** |
| Canônico §7.2/§7.3 | `pending` como 7º state + linha `pending → PENDING` na projeção | **já aplicado** (`PLANO_ATENDIMENTO.md`, D-E e §7.3) |
| **Spec da F2** | `CHECK` de `conversations.state` nasce com os **7** valores (`new`, `ai_active`, `routing`, `queued`, `human_active`, `pending`, `closed`) | **já aplicado** (`OMNI-F2.md`, C1) |

## Contrato 4 — Motor de roteamento (`routing.go`)

**IA sugere; o motor decide.** `Decide` recebe `extracted_fields` já preenchido e **não chama
modelo** — é testável sem LLM.

```go
type RoutingEngine interface { Decide(ctx context.Context, accountID string, conv Conversation) (Decision, error) }
type Decision struct { RuleID, QueueID, DepartmentID *string; Outcome, Reason string; Input map[string]any }
```

- Avalia `routing_rules` ativas em `priority asc, id asc`; **first-match-wins**.
- `conditions` jsonb = array de `{field, op, value}` com **AND** entre elas. `op` ∈
  `eq` · `neq` · `contains` · `in` · `exists`. `field` = chave de `extracted_fields` ou campo
  canônico (`message.text`, `contact.phone`, `instance.name`). Array vazio = casa sempre.
- Sem match → fila default do setor default (`default_queue`). Sem default → `unrouted`
  (`queue_id NULL`).
- **Toda** chamada grava uma linha em `routing_decisions`, inclusive `unrouted`.

## Contrato 5 — Visibilidade ("permissão gateia feature; fila gateia dado")

Predicado **no repositório** (defesa em profundidade — não só no service, não só no front),
aplicado a **toda** leitura de conversa (list, messages, ações da F6/F7):

```sql
where c.account_id = $1::uuid
  and (
    $2                                        -- escopo amplo (ver abaixo)
    or c.assigned_user_id = $3::uuid
    or (c.queue_id is not null and exists (
          select 1 from messaging.queue_members qm
          where qm.queue_id = c.queue_id and qm.user_id = $3::uuid and qm.is_active = true))
  )
```

- **Escopo amplo** = `platform_admin` (`auth.RolePlatformAdmin`) **ou** `omnichannel.settings.manage`.
  *Decisão desta spec, derivada do §5.2 + princípio 5:* conversa `unrouted` tem `queue_id NULL`
  e não seria vista por **ninguém** — quem configura filas precisa vê-la para consertar a
  config. Se o dono discordar, muda aqui.
- Resolver a permissão efetiva **na conta** antes de cair no `principal.Permissions` — padrão
  confirmado em `back/internal/modules/realtime/service_calendar.go:140-150`
  (`PermissionsResolved` + `hasAnyString`).
- Conversa fora do escopo → **404**, nunca 403.

## Contrato 6 — Rotas novas

Todas sob `/v1/omnichannel` (o gate de módulo já cobre o prefixo — `app.go:518`), com
`RequireAuthWithAccount` (`auth/middleware.go:81` — injeta `AccountID` no Principal a partir do
`X-Account-Id` e valida membership org-aware; `platform_admin` passa por
`account_checker.go:23-45`).

| Rota | Permissão | Nota |
|---|---|---|
| `GET\|POST /settings/departments` · `PATCH\|DELETE /settings/departments/{id}` | `omnichannel.settings.manage` | DELETE = **soft** (`is_active=false`); conversas na fila do setor continuam visíveis (princípio 3) |
| `GET\|POST /settings/queues` · `PATCH\|DELETE /settings/queues/{id}` | `omnichannel.settings.manage` | `?departmentId=` no GET |
| `GET\|POST /settings/queues/{id}/members` · `DELETE /settings/queues/{id}/members/{userId}` | `omnichannel.settings.manage` | usuário não-membro da conta (`core.account_users`) → **404** |
| `GET\|POST /settings/routing-rules` · `PATCH\|DELETE /settings/routing-rules/{id}` | `omnichannel.settings.manage` | `target_queue_id` inativa/de outra conta → **404** |
| `PUT /settings/routing-rules/order` | `omnichannel.settings.manage` | body `{ruleIds:[]}`; reordena `priority` em **uma** transação. Id ausente/de outra conta → **404** e a ordem **não muda** (tudo ou nada) |
| `PATCH /conversations/{id}/queue` | `omnichannel.conversations.assign` | body `{queueId}` → evento `queue.transfer` |
| `GET /conversations/{id}/routing-decisions` | `omnichannel.conversations.view` + visibilidade | `decided_at desc` |

**A F8 NÃO cria** `/conversations/{id}/assign` nem `/status` — são rotas do port (F7,
`PLANO_PORT_OMNICHANNEL.md` §8). A F8 exporta `Service.Transition`; a F7 chama.

**Esta é a superfície HTTP de config de setores/filas/regras — a F10 CONSOME estas rotas, não
recria.** A F10 é *telas* (canônico §9.2 F10); o dono do contrato é quem tem a tabela. Os paths
`/settings/*` acima são os **definitivos**: não existe `/departments`, `/queues` nem
`/routing-rules` sem o prefixo `/settings`.

**Membros da fila = `POST`/`DELETE` incremental, NÃO `PUT` de conjunto completo** — decisão desta
spec, reconciliando com a F10, que tabelava `PUT /queues/{id}/members` com o `userId[]` inteiro:

- `queue_members` tem `is_active` e é o **gate de dado**: `POST` (re)ativa um membro, `DELETE`
  desativa. Cada chamada é idempotente e valida **aquele** usuário contra `core.account_users`
  (→ 404 se não for da conta) — um `userId[]` inteiro obriga a validar em lote e a decidir o que
  fazer quando um id do meio é inválido.
- Um `PUT` de conjunto completo faz **lost update silencioso**: dois supervisores editando a mesma
  fila, o último a salvar remove quem o outro acabou de incluir — e ninguém vê. Em gate de dado,
  isso é atendente perdendo acesso à conversa sem explicação.
- A tela da F10 faz o **diff** da seleção contra o que veio do back e dispara as duas rotas.

---

## Armadilhas / o que NÃO fazer

- **Não escrever `status` na mão.** Não existe coluna; nenhum `update ... set status`. Quem
  quiser mudar ciclo de vida chama `Transition`. Este é o risco 4 do canônico.
- **Não re-decidir o `PENDING`.** É o `state` `pending`, decisão do dono de **2026-07-17**
  (Contrato 3.1) — implementar como está tabelado, não reabrir. **Não mapear `queued → PENDING`**
  "porque é o que sobra": está descartado **com evidência** — `queued` é produzido pelo *motor*,
  não pelo operador, e mapeá-lo trocaria "filtro sempre vazio" por "filtro sempre cheio".
  `pending` só é escrito pelo evento `human.pending`. E nunca aceitar o PATCH em silêncio
  gravando `OPEN`: o operador marca, some, e ninguém sabe.
- **Não usar timer/`paused_until`** para calar a IA. O hard-block é o estado `human_active`.
- **Não filtrar visibilidade só no service ou só no front.** O repositório filtra também.
- **404 é escopo; 403 é permissão.** Conversa de outra fila/conta → 404. Sem
  `conversations.reply` → 403 (o port já faz assim: `SPECS_PORT_OMNICHANNEL.md` F5, "VIEWER → 403").
- **Transição inválida → 409 `invalid_transition`**, com mensagem acionável. Nunca 500, nunca
  ignorar em silêncio.
- **Não deixar conversa órfã silenciosa.** `unrouted` é estado honesto e visível para quem pode
  consertar (princípio 5), não um default que minta.
- **Não implementar round-robin/balanceamento por carga** — explicitamente fora de escopo
  (canônico §15). Regra determinística, só.
- **`platform_admin` tem `has()` = false no front** — quando a F10 construir as telas, gatear com
  `isPlatformAdmin || has(...)`, senão o módulo some para quem administra.
- **Não numerar a migration às cegas** (dois `0197` no disco) nem confiar no cache do Docker
  (ver Notas de Deploy).

## Segurança

| Item | Regra |
|---|---|
| Escopo | `account_id` **sempre** do Principal (`RequireAuthWithAccount`), **nunca** do body — em `departments`, `queues`, `queue_members`, `routing_rules`, `routing_decisions` |
| Defesa em profundidade | Todo `select`/`update` do repositório carrega `account_id = $1` **além** da validação do service |
| Fora de escopo | **404**, nunca 403 (403 vaza existência — enumeration) |
| FKs cruzadas | `queue.department_id`, `routing_rules.target_queue_id`, `queue_members.user_id` validados **contra a conta do Principal** antes do insert → fora da conta = **404** |
| Dado do atendente | Fila é gate de **dado**: `conversations.view` sem `queue_member` e sem atribuição = não vê |

## Verificável

Um humano prova no browser/banco (F10 ainda não existe → config via API/`psql`):

1. Criar setor `Vendas`, fila `Vendas/Novos`, regra `priority=10`,
   `conditions=[{"field":"message.text","op":"contains","value":"preço"}]` → fila. Mandar
   "quanto é o preço?" do celular. `select state, queue_id from messaging.conversations` →
   `queued` + a fila certa; `select outcome, rule_id, reason from messaging.routing_decisions
   where conversation_id = …` → **explica a decisão**.
2. Atendente A (`queue_member` de `Vendas/Novos`) vê a conversa no inbox. Atendente B (membro
   só de outra fila) **não vê** no `GET /v1/omnichannel/conversations` e leva **404** em
   `GET /v1/omnichannel/conversations/{id}/messages`.
3. `PATCH /assign` para A → `state='human_active'`; **o front verbatim mostra a conversa aberta
   com o atendente** (projeção). Com a F9 no ar: mandar mensagem e o bot **não** responde.
4. `PATCH /status {CLOSED}` → `state='closed'`, front mostra CLOSED. Cliente manda nova
   mensagem → volta para `ai_active`/`routing`, front mostra **OPEN** de novo e
   `assigned_user_id` está nulo.
5. Apagar as regras e não deixar fila default → nova conversa fica `queued` com `queue_id NULL`
   e `routing_decisions.outcome='unrouted'`; some do inbox do atendente e **aparece** para quem
   tem `settings.manage`.
6. `PATCH /conversations/{id}/queue` numa conversa `closed` → **409 `invalid_transition`**.
7. `X-Account-Id` de outra conta em qualquer `/settings/*` → **404**.
8. `go test ./back/internal/modules/omnichannel/...` cobre **os 84 pares** da matriz.
9. **`PENDING` (decisão do dono, 2026-07-17 — Contrato 3.1), provado no front verbatim:** numa
   conversa atribuída, marcar **"Pendente"** → `select state from messaging.conversations` =
   `pending` e o filtro **"Pendentes"** do inbox **lista a conversa** (hoje ele devolveria zero
   para sempre). O cliente mandar mensagem **não** tira o rótulo (nota 3: continua em
   `pending`/`PENDING`). **Responder** na conversa → `state='human_active'`, ela **sai** do filtro
   "Pendentes" e volta para "Abertos" com o atendente. Marcar "Pendente" numa conversa **CLOSED**
   → **409 `invalid_transition`** (nota 12), não uma reabertura silenciosa.

## Notas de Deploy

Ordem exata:

| # | Passo | Detalhe |
|---|---|---|
| 1 | Migration `<próximo livre>_messaging_service_domain.sql` | **Não é `0200`** — esse é da F2 (Contrato 1). Candidato natural: `0201`. Conferir o disco antes de numerar (dois `0197`; última `0199`; F3/F4 podem entrar no meio). Idempotente, sem `-- +goose Down` |
| 2 | `docker compose build --no-cache api` | **Obrigatório:** migrations são `embed.FS` — o cache da camada `go build` pode não re-embutir o `.sql` novo. Sintoma: `migrate status` para na anterior, **sem erro** |
| 3 | `docker compose up -d api` | Mudou `back/` → rebuild, restart não basta |

Sem env var nova. Sem container novo. Sem mudança no `web/` (as telas são a **F10**).
