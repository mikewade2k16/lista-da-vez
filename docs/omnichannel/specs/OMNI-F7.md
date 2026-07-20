# OMNI-F7 — Ações do inbox

**Prioridade:** P0
**Plano canônico:** `docs/omnichannel/PLANO_ATENDIMENTO.md` (§9.2 F7, §7.3, §5.2, §5.4)
**Anexo técnico (contratos verbatim):** `SPECS_PORT_OMNICHANNEL.md` F6 · `PLANO_PORT_OMNICHANNEL.md` §8

> ## LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono)
> A branch `refactor/multi-tenant-complete` fechou e o dono **liberou a implementação em
> 2026-07-17** (decisão **D-D**, canônico §2). O aviso de congelamento que constava aqui não vale mais.

Ler a skill `principios-engenharia` antes de executar.

---

## Objetivo

Cada botão do inbox portado passa a funcionar ponta a ponta: reagir, encaminhar, apagar
para mim / para todos, encerrar/reabrir e atribuir. Duas coisas nascem diferentes do
legado: **nenhuma ação escreve `status` na mão** (tudo pela máquina de estados da F8), e
**o filtro de instância por usuário passa a existir de verdade** — hoje, no legado, ele é
código morto e todo usuário vê todas as instâncias.

## Depende de / Bloqueia

| | Fases |
|---|---|
| **Depende de** | **F6** (envio via outbox — o `forward` reusa esse caminho) · **F8** (máquina de estados, `queue_members`) · F5 (Publisher do realtime) · F4 (`ChannelProvider` + `Capabilities()`) |
| **Bloqueia** | F10 (telas de config assumem `assign` funcional) · F13 (trilha de auditoria completa) |
| **Não bloqueia** | F9 — o hard-block da IA é **lido** pela F9 a partir de `state`, não implementado aqui |

---

## Entregas

| # | Entrega | Alvo |
|---|---|---|
| 1 | Handlers das 11 rotas do port F6 (lista em `SPECS_PORT_OMNICHANNEL.md` F6 — **não recopiada aqui**) | `back/internal/modules/omnichannel/http_actions.go` |
| 2 | Serviço das ações; mapeia requisição → **evento** da máquina da F8; nunca escreve `state`/`status` direto | `back/internal/modules/omnichannel/service_actions.go` |
| 3 | **Correção do escopo de instância** (o ternário morto do legado) no repositório | `back/internal/modules/omnichannel/store_postgres_scope.go` |
| 4 | Queries das ações (hidden_messages, forward, delete-for-all), todas filtrando por `account_id` | `back/internal/modules/omnichannel/store_postgres_actions.go` |
| 5 | Auditoria de `status`/`assign` + das ações destrutivas em `messaging.audit_events` | idem #4 |
| 6 | Gate de `Capabilities()` por número antes de `reaction` e `delete-for-all` | `service_actions.go` |
| 7 | Migration da extensão do CHECK de `event_type` — **só se** a F2 modelou a coluna com CHECK (ver Contratos) | `back/internal/platform/database/migrations/` |
| 8 | Atualizar `back/internal/modules/omnichannel/AGENT.md` (criado na F2) com as rotas/eventos desta fase | AGENT.md do módulo |

Convenção de arquivos = a da F2 (`SPECS_PORT_OMNICHANNEL.md` F2.2) e a da casa
(`internal/modules/tasks/`: `http.go`, `http_relations.go`, `permissions.go`…). Teto de
~450 linhas **vale aqui** — é código novo.

---

## Contratos

### C1 · Rotas, bodies e respostas — **remissão, não cópia**

As 11 rotas são as do port: `SPECS_PORT_OMNICHANNEL.md` F6 + `PLANO_PORT_OMNICHANNEL.md` §8
(inclui `group-participants`, `sync-open`, `sync-history` e as 2 de contatos — o §9.1 do
canônico mapeia **OMNI-F6 inteira** → F7). Bodies/respostas confirmados no legado
(`.../routes/conversations/schemas.ts:50-75`) — não divergir de um campo sequer:

| Ação | Body | Resposta |
|---|---|---|
| `PATCH .../conversations/{id}/status` | `{ status: "OPEN"\|"PENDING"\|"CLOSED" }` — os **3** que o legado aceita e o front grava; `PENDING` deixou de ser indefinido por **D-E (2026-07-17)**, ver C2 | `Conversation` |
| `PATCH .../conversations/{id}/assign` | `{ assignedToId: string \| null }` | `Conversation` |
| `POST .../messages/{mid}/reaction` | `{ emoji?: string(max 32) \| null }` (`null` = remover) | — |
| `POST .../messages/forward` | `{ messageIds: 1..100, targetConversationId }` | `{ sourceConversationId, targetConversationId, createdCount, queuedCount, failedToQueueCount, failedToQueueIds[], messages[] }` |
| `POST .../messages/delete-for-me` | `{ messageIds: 1..100 }` | `{ deletedIds[], skippedIds[], conversation }` |
| `POST .../messages/delete-for-all` | `{ messageIds: 1..100 }` | `{ updatedIds[], skippedIds[], failedIds[], messages[] }` |

`Conversation` **não tem campo `state`** (`web-reference/app/types/index.ts:149-174`). O
`state` é verdade de servidor e **não vaza no JSON**; o front só conhece `status`
(projeção — canônico §7.3).

### C2 · A costura com a máquina de estados (F8 é a dona; a F7 **consome**)

A F8 **exporta**; a F7 **chama** (`OMNI-F8.md`, Contrato 2 e fim do Contrato 5 — "a F8 NÃO
cria `/assign` nem `/status`: são rotas do port (F7); a F8 exporta `Service.Transition`"):

```go
func (s *Service) Transition(ctx context.Context, p auth.Principal, convID string,
    ev Event, payload TransitionPayload) (Conversation, error) // ErrInvalidTransition -> 409
```

O **mapa requisição → evento já está tabelado na F8** (`OMNI-F8.md`, Contrato 3, "projeção
inversa") — **não replicar aqui**. Em resumo do que a F7 dispara:
`conv.close` · `conv.reopen` · `human.assign` · `human.unassign` · **`human.pending`**.

| Regra | Comportamento |
|---|---|
| `assign` com usuário | `human.assign` ⇒ `human_active` ⇒ **hard-block da IA** (`AIAllowed` = true só em `new`/`ai_active`) |
| `status: PENDING` | **`human.pending` ⇒ `pending`** — o **12º evento** e o **7º `state`** da máquina, por **decisão do dono D-E (2026-07-17)**, opção A do Contrato 3.1 da F8. É **rótulo manual do operador** ("parei nesta, estou esperando algo"): sem produtor automático e sem limpeza automática. **Não é mais o 409 `invalid_transition` interino** que esta spec herdou |
| `status: OPEN` em conversa já aberta | **no-op**: 200 com a `Conversation` (regra de ouro do port: o Go se adapta ao front) |
| `ErrInvalidTransition` | **409** com erro acionável (`{message, code}`), nunca no-op silencioso (princípio 5) |
| `assign` para usuário fora da conta | **404** — o legado checa o usuário antes de tocar a conversa |

A F8 roda cada transição em transação com `select ... for update` — a F7 **não** faz seu
próprio `UPDATE` da conversa, nem lê `state` para decidir destino.

### C3 · Correção do escopo de instância (é isolamento, não cosmético)

**O bug, confirmado no disco:**
`whats-test/apps/atendimento-online-api/src/services/whatsapp-instances.ts:681-683`

```ts
const accessibleInstances = isTenantAdmin || activeInstances.length <= 1
  ? activeInstances
  : activeInstances;   // <- mesmo valor nos dois ramos: o filtro NUNCA rodou
```

Que o filtro era **projetado** está provado no schema: `WhatsAppInstance.responsibleUserId`
(`prisma/schema.prisma:66`) e o índice `@@index([tenantId, responsibleUserId])` (`:74`)
existem e não servem a ninguém. Todo o acesso do legado passa por esse resolver
(`routes/conversations/access.ts:9-27` → `buildConversationInstanceScopeWhere`), então
listar e agir herdam o mesmo furo.

**Comportamento corrigido** (reconstrói a intenção legível do próprio código morto — o guard
`<= 1` evita trancar todo mundo fora quando só há um número):

| Ator | Instâncias visíveis |
|---|---|
| Admin da conta / `platform_admin` | todas as ativas |
| Qualquer usuário, quando a conta tem **≤ 1** instância ativa | a única ativa |
| Demais | ativas com `responsible_user_id = <Principal.UserID>` |
| Resultado vazio | **vê nada** — nunca cair em "vê tudo" |

**Onde mora:** no **repositório** (`store_postgres_scope.go`), como `WHERE` — não só no
service, não só no front (defesa em profundidade, princípio 2). Toda rota desta fase resolve
a conversa por esse `WHERE`; fora do escopo → **404**.

**Convivência com a fila da F8 (não é ou-um-ou-outro):** o gate de instância (F7) e o gate
de fila (F8: `queue_members` + atribuídas a mim — canônico §5.2) **se somam com `AND`**, e o
mais restritivo vence. Unir com `OR` reabriria o furo por outro caminho. Duas features
diferentes coexistem (princípio 3).

### C4 · `Capabilities()` — a UI degrada por número (§5.4, §12 risco 2)

O legado chama o Evolution direto em `reaction` (`routes-operational-reaction.ts:127-146`,
502 na falha) e em `delete-for-all` (`routes-message-write-delete-for-all.ts:112-129`, 409
sem WhatsApp configurado). Na fusão isso vira **`ChannelProvider` (F4)**, nunca um client de
provider cravado no handler. Respostas: ação não suportada pelo número (`Capabilities()`) →
**409** acionável (o botão não pode mentir); mensagem sem `externalMessageId` → **409**
(precedente do legado); provider falhou → **502**.

`reaction` e `delete-for-all` são **síncronas** ao provider — não vão pelo outbox. Isso é o
contrato do front (espera 200/502 na hora), não esquecimento. **`forward` é a exceção**:
reusa o caminho de envio da **F6** (outbox, `idempotency_key`, FIFO por conversa) — é o que
os campos `queuedCount`/`failedToQueueCount` da resposta significam. Não escrever um segundo
caminho de envio.

### C5 · Auditoria

`status` e `assign` gravam em `messaging.audit_events` com os tipos que já vêm do port
(`CONVERSATION_STATUS_CHANGED`, `CONVERSATION_ASSIGNED`), payload `{before, after, changedBy}`
— igual ao legado (`routes-operational-assign.ts:160-175`). Auditar **só quando muda de
fato** (o legado compara `before != after`).

O enum do port (`prisma/schema.prisma:40-46`) tem **5 tipos e nenhum** para ação destrutiva.
`delete-for-all` é irreversível e visível para o cliente final; `forward` publica conteúdo em
outra conversa. F7 estende com `MESSAGE_DELETED_FOR_ALL` e `MESSAGE_FORWARDED`
(`delete-for-me` não entra: é ocultação por usuário, sem efeito externo).

**Migration só se necessária** — conferir como a F2 modelou `event_type`: se for `text` livre,
**não há migration nesta fase**. Se for CHECK, SQL plano idempotente, schema-qualificado,
**sem `-- +goose Down`** (o migrator roda o arquivo inteiro e o Down se auto-destrói):

```sql
alter table messaging.audit_events
  drop constraint if exists audit_events_event_type_check;

alter table messaging.audit_events
  add constraint audit_events_event_type_check
  check (event_type in (
    'MESSAGE_OUTBOUND_QUEUED', 'MESSAGE_OUTBOUND_SENT', 'MESSAGE_OUTBOUND_FAILED',
    'CONVERSATION_STATUS_CHANGED', 'CONVERSATION_ASSIGNED',
    'MESSAGE_FORWARDED', 'MESSAGE_DELETED_FOR_ALL'
  ));
```

O **nome real da constraint vem da F2** — conferir, não presumir. **Numeração:** a última no
disco hoje é `0199_calendar_drop_day_media.sql`, mas a F2 já reivindica
`0200_messaging_schema.sql` (`OMNI-F2.md`, entrega 1) e F3/F8 entram antes desta fase.
**Conferir o disco na hora de numerar** — há dois arquivos `0197`, prova de que ninguém valida
a sequência (canônico §13).

---

## Armadilhas / o que NÃO fazer

| Não fazer | Por quê |
|---|---|
| Escrever `status` (ou `state`) direto no `UPDATE` | É o ponto mais frágil da fusão (§12 risco 4). Uma escrita fora da máquina = inbox mostrando conversa fechada como aberta |
| **Derivar o destino do `unassign`/`reopen` aqui** | A F8 tabela **TODAS** as transições (`OMNI-F8.md` Contrato 2/3 — `human.unassign` e `conv.reopen` já estão lá). A F7 emite o evento e usa a `Conversation` devolvida. Se algum caso faltar na tabela dela, reabrir a F8 — não improvisar |
| Implementar o hard-block da IA nesta fase | `assign` ⇒ `human_active`; **quem lê o estado e cala é a F9**. Duplicar o bloqueio cria duas verdades |
| Portar o ternário do legado "porque é verbatim" | O verbatim é o **front** (D-B). Backend com filtro inoperante é furo de isolamento |
| Chamar o Evolution direto no handler | Adapter é o `ChannelProvider` da F4. Provider cravado quebra o multi-provider (D-A) |
| Segundo caminho de envio no `forward` | O envio é da F6 (outbox). Duas filas = ordem invertida para o cliente (§12 risco 5) |
| Devolver 403 para conversa/mensagem de outro escopo | 403 confirma que o recurso existe (enumeration) → **404** |
| Confiar que `messageId` pertence à conversa | O legado casa `conversationId` + `tenantId` na query da mensagem. Sem isso, IDs de outra conversa passam |
| Botão morto / falha silenciosa | Sem capability ou sem `externalMessageId` → **409 acionável** (princípio 5) |

---

## Segurança

- `account_id` **sempre** do `Principal` (`auth.Principal.AccountID`, resolvido pelo
  `RequireAuthWithAccount` — `back/internal/modules/auth/middleware.go:81`), **nunca** do
  body. O repositório filtra por `account_id` **também** (defesa em profundidade).
- Fora de escopo → **404**, nunca 403. Vale para conversa, mensagem e **usuário do assign**.
- **`forward` valida as DUAS conversas** (origem *e* destino) dentro do escopo do ator, cada
  uma com seu 404 (`routes-message-write-forward.ts:98-117`). Sem isso, encaminhar vira
  gravação em conversa que o ator não pode ver.
- **`delete-for-all` só em `direction = OUTBOUND`** — o legado filtra assim; ninguém apaga
  mensagem do cliente.
- Permissões (canônico §5.2): `conversations.reply` → reaction/forward/delete;
  `conversations.assign` → assign; `conversations.close` → status. É **permissão**, não
  escopo → **403** quando falta (o legado usa `requireConversationWrite`,
  `src/lib/guards.ts:54-61`). Reusar o guard que a F2 estabeleceu no módulo; helper
  disponível: `access.HasPermission` (`back/internal/modules/access/permissions.go:215`).
- Realtime: `conversation.updated` sai pelo Publisher injetado na F5 (padrão
  `calendar.WithPublisher` — `back/internal/modules/calendar/publisher.go:51-53`), no canal
  `omnichannel:account:{id}`. `status`/`assign` emitem **com** `instanceName` (shape de
  `mapConversation` — `SPECS_PORT_OMNICHANNEL.md` F4). Data URL vira `null`: **nunca base64 no WS**.

---

## Verificável

Um humano prova no browser + banco, **sem ler código**:

1. **Ação a ação:** reagir (emoji aparece no celular), remover a reação (`emoji: null`),
   encaminhar 2 mensagens (chegam no celular), apagar para mim (some da tela, **continua** em
   `messaging.messages`), apagar para todos (vira apagada no celular), encerrar, reabrir, atribuir
   e **marcar como pendente** (`status: PENDING` → `select state ...` mostra `pending`, e o filtro
   "Pendentes" do inbox passa a devolver a conversa — D-E).
2. **Estado ≠ status:** atribuir a um atendente → `select state, assigned_user_id from
   messaging.conversations where id = ...` mostra `human_active`; a UI mostra a conversa
   **aberta** com o responsável preenchido.
3. **Hard-block:** com a F9 no ar, mandar mensagem numa conversa atribuída → **a IA não
   responde**. Desatribuir → volta ao comportamento da tabela da F8.
4. **O bug morto do legado, agora vivo:** conta com **2+ instâncias ativas**, instância B com
   `responsible_user_id` = outro usuário. Logado como atendente responsável só pela A: conversa
   da B **não aparece** na lista e `PATCH /assign` nela → **404** (não 403). Como admin da
   conta → as duas aparecem. *Sem este passo a fase não fechou.*
5. **Isolamento entre contas:** trocar o `X-Account-Id` e repetir qualquer ação com o id de
   conversa da conta original → **404**.
6. **Auditoria:** `select event_type, payload_json from messaging.audit_events order by created_at desc limit 5`
   mostra `CONVERSATION_ASSIGNED` com `{before, after, changedBy}` e `MESSAGE_DELETED_FOR_ALL`.
   Atribuir de novo para o **mesmo** usuário → **não** cria linha nova.
7. **Degradação por número:** número cujo provider não suporta reação → **409 acionável** e a
   UI avisa; nada de erro genérico nem botão morto.

---

## Notas de Deploy

| # | Item | Detalhe |
|---|---|---|
| 1 | Migration do CHECK de `event_type` | **Só se** a F2 usou CHECK. Numerar **conferindo o disco** (a F2 leva a `0200`; F3/F8 vêm antes desta fase). Idempotente, **sem `-- +goose Down`** |
| 2 | Build da API | Mudou `back/` → `docker compose up -d --build api`. **Se houver migration nova → `docker compose build --no-cache api`**: as migrations são `embed.FS` e o cache do `go build` pode não re-embutir o `.sql` (sintoma: `migrate status` para na anterior, **sem erro**) |
| 3 | Env vars | **Nenhuma nova nesta fase** — provider e credenciais vêm da F4/F3 |
| 4 | Front | Nenhuma alteração: os call-sites já existem no verbatim da F1 (`useOmnichannelInboxConversationActions.ts:28,53`, `useOmnichannelInboxMessageActions.ts:75,115,165`, `useOmnichannelInboxMessageReactions.ts:130`) |

Não rodar git. Não commitar. Devolver os comandos ao usuário.
