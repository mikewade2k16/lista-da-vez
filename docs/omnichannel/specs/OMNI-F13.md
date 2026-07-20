# OMNI-F13 — LGPD + observabilidade

**Prioridade:** **P0 (mínimo — entra no piloto)** · P1 (export/anonimização)
**Plano canônico:** [`docs/omnichannel/PLANO_ATENDIMENTO.md`](../PLANO_ATENDIMENTO.md) (§9.2 F13, §10, §5.3, §13 item 7)

> ## LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono)
>
> A branch `refactor/multi-tenant-complete` fechou e o dono **liberou a implementação em
> 2026-07-17** (decisão **D-D**, canônico §2). O aviso de congelamento que constava aqui não vale mais.

Ler a skill `principios-engenharia` antes de começar. Teto ~450 linhas/arquivo **vale** (código novo).

---

## Objetivo

Dado de conversa de cliente final **não nasce sem política de retenção**. Quando a F13-mínimo
fecha: cada classe de dado tem prazo, um **job que realmente apaga** roda sobre o `platform/jobs`
da F3 (banco **e** disco), payload bruto não existe em log/erro, e o custo de LLM por conta
aparece na tela a partir do `usage` que a F9 grava. Sem isso o piloto sobe com dado pessoal de
terceiro sem prazo, sem dono e sem teto de custo.

---

## O corte P0-mínimo × P1 — **onde parar para fechar o piloto**

Esta é a **única fase do plano com prioridade partida** (canônico §9, linha F13). Quem executar
entrega a coluna P0 e **para**.

| | Entrega | Fase |
|---|---|---|
| ✅ | Classes de retenção + config por conta + defaults (C1, C2) | **P0-mínimo** |
| ✅ | Job de purge sobre `platform/jobs` — banco (C4) | **P0-mínimo** |
| ✅ | Purge de **mídia em disco** + varredura de órfãos (C5) | **P0-mínimo** |
| ✅ | Mídia no **backup** (nota de deploy 4) | **P0-mínimo** |
| ✅ | Masking de payload bruto em log/erro + retrofit dos call-sites (C6) | **P0-mínimo** |
| ✅ | `messaging.purge_runs` — evidência de que a política roda (C3) | **P0-mínimo** |
| ✅ | Custo de LLM por conta na tela + tabela de preço (C7) | **P0-mínimo** |
| ⏸ | **Export dos dados do titular** (C8.1) | **P1 — fora do piloto** |
| ⏸ | **Anonimização a pedido do titular** (C8.2) | **P1 — fora do piloto** |
| ⏸ | Backfill de `cost_usd` de runs anteriores à tabela de preço (C7) | **P1** |

**O P1 não bloqueia o piloto e não é pré-requisito de nada.** Retenção com purge cobre a
obrigação de prazo; export/anonimização a pedido do titular é atendimento a requisição
individual — hoje resolvível manualmente por `psql` enquanto a base é de piloto. Registrar em
`docs/LEGADO.md` como "atendimento manual até a F13-P1".

## Depende de / Bloqueia

| | Fases |
|---|---|
| **Depende de** | **F9** (canônico §9.2: `ai_runs` com `usage`). Na prática também precisa das tabelas de **F2/F4/F6/F7/F8** existirem para poder podá-las — ver *Divergências* nº1 |
| **Bloqueia** | Nada. É a última fase do piloto P0 (`F0 → F10 + F13-mínimo`) |
| **Front** | A aba de custo (C7) mora no drawer da **F10** — ver *Divergências* nº2 |

---

## Entregas

| # | Entrega | Alvo |
|---|---|---|
| 1 | Migration: `messaging.purge_runs` + `ai_runs.cost_priced` | `back/internal/platform/database/migrations/<próximo livre>_messaging_retention.sql` |
| 2 | Resolução da política de retenção (conta → plataforma → constante) | `back/internal/modules/omnichannel/retention_policy.go` |
| 3 | Job de purge (classes, batches, dry-run) registrado no `platform/jobs` | `.../omnichannel/retention_purge.go` |
| 4 | Store do purge (deletes/scrubs por classe, `account_id` em toda query) | `.../omnichannel/store_retention.go` |
| 5 | Purge de mídia em disco + varredura de órfãos | `.../omnichannel/retention_media.go` |
| 6 | Enfileirador diário (só enfileira — o trabalho é do `platform/jobs`) | `back/internal/platform/app/app.go` |
| 7 | Masking de payload/PII (allowlist) | `back/internal/platform/logmask/logmask.go` + `_test.go` |
| 8 | Retrofit dos call-sites de log/persistência de F4/F6/F9 | ver C6 |
| 9 | Tabela de preço de LLM + cálculo do custo | `back/internal/platform/llm/pricing.go` |
| 10 | Rota de custo agregado por conta | `.../omnichannel/http_usage.go` |
| 11 | Aba de custo no drawer de config | `web/app/components/omnichannel/config/ConfigAiUsage.vue` |
| 12 | Sincronizar os 3 docs ao fechar | `.../omnichannel/AGENT.md` · canônico · `phases-part7.ts` |

---

## Contratos

### C1 — Classes de retenção (365 / 180 / 90 / 30)

O canônico §10 fixa os **quatro prazos** mas **não diz qual tabela cai em qual classe**. O mapa
abaixo é **decisão desta spec** — se o dono discordar, muda **aqui**, não no código.

| Classe | Default | Cobre | Ação do purge |
|---|---|---|---|
| `audit` | **365d** | `messaging.audit_events`, `messaging.purge_runs` | **DELETE** por `created_at` |
| `conversation` | **180d** | `messaging.conversations` + o que pende dela (`messages`, mídia em disco, `hidden_messages`, `routing_decisions`) + `contacts` sem conversa restante | **DELETE** do agregado, ancorado em `last_message_at` |
| `ai_io` | **90d** | **Colunas** `ai_runs.input`/`output` e `routing_decisions.input` | **SCRUB** (`= '{}'::jsonb`) — a linha sobrevive |
| `ephemeral` | **30d** | `messaging.webhook_events`, `messaging.outbox` em `done`/`dead` | **DELETE** por `created_at` |

Três decisões que não são cosméticas:

1. **A classe `conversation` ancora em `conversations.last_message_at`, nunca no `created_at` da
   mensagem.** Podar mensagem velha de conversa **ativa** é apagar o histórico debaixo do
   atendente no meio do atendimento. Conversa parada há 180 dias vai inteira, **em qualquer
   `state`** (inclusive aberta — senão conversa que ninguém fecha vira retenção infinita).
2. **`ai_io` é SCRUB, não DELETE.** `ai_runs` carrega PII (`input`/`output`) **e** o custo
   (`total_tokens`, `cost_usd`). Apagar a linha aos 90 dias destrói a **própria base do custo por
   conta que esta fase entrega** (C7) — o histórico de fatura da conta evapora. Zera-se o payload;
   os contadores (que não são dado pessoal) seguem até a classe `audit`. Idem
   `routing_decisions.input`: some o snapshot de PII, ficam `rule_id`/`outcome`/`reason`, e a
   decisão continua explicável (F8, Contrato 4).
3. **PII de `ai_runs`/`routing_decisions` morre com a conversa, mesmo antes dos 90 dias.** O scrub
   roda em `min(90d, purge da conversa)`. `messaging.ai_runs.conversation_id` é `uuid not null`
   **sem FK** (F9, C9.1) → **não cascateia**: sem o scrub explícito, a conversa some e o `input`
   com o texto do cliente fica na tabela para sempre. É o vazamento mais fácil de deixar passar
   nesta fase.

`conversations.extracted_fields` (F2) **não** é classe `ai_io`: é estado vivo do roteamento e
morre com a conversa (`conversation`). Podá-lo aos 90 dias quebraria a regra de uma conversa ativa.

### C2 — Config por conta e defaults (e a armadilha do resolver da F3)

Sem migration: `core.account_modules.config jsonb` **já existe** (`0100_core_schema.sql:120`) e o
leitor é entrega da **F3** (`platform/modules/limits.go`). Convive com as chaves do canônico §5.3:

```json
{ "max_whatsapp_numbers": 2, "monthly_ai_runs": 5000,
  "retention_days": { "audit": 365, "conversation": 180, "ai_io": 90, "ephemeral": 30 } }
```

Default da plataforma em `core.platform_settings`, chave `omnichannel_retention` (padrão singleton
key-value de `0160_core_platform_settings.sql`; precedente `calendar_ai_secrets`).

> **ARMADILHA VERIFICADA — não reusar o fallback da F3.** O resolver da F3.4 é
> `config da conta → platform_settings → **ausente nos dois = sem limite**`. Para *limite* isso
> está certo; para *retenção* "sem limite" significa **nunca apagar** — a política deixa de
> existir exatamente quando a config falta, que é o caso comum. A cadeia da retenção é
> `conta → plataforma → **constante Go (365/180/90/30)**`, **nunca** "sem limite".

| Regra | Detalhe |
|---|---|
| Faixa válida | `1..maxDays` (default `maxDays` = 3650, em `omnichannel_retention`). Fora da faixa → **409** acionável, com `{key, value, min, max}` |
| `0` / negativo | **Rejeitado.** Não existe "desligar retenção" pelo painel — desligar obrigação legal por config de conta não é feature |
| Origem visível | O leitor devolve `{days, source}` com `source ∈ account\|platform\|default`. A tela mostra a origem (espelho do `KeyStatusView.Scope`, `calendar/secrets.go`) — princípio 5: valor honesto com procedência, não default mudo |
| Writer | **Não nasce aqui.** Igual à F3.4: hoje só por SQL; o writer de painel é da F10. A constante Go é a última linha de defesa, não a fonte de verdade |

### C3 — Migration (próximo número livre — **não cravar**)

**Conferir o disco antes de numerar.** Hoje a última é `0199_calendar_drop_day_media.sql` e há
**dois arquivos `0197`** (`0197_operation_validation_reason.sql`, `0197_tools_module.sql`) — a
numeração não é validada por ninguém. F2/F3/F4/F6/F8/F9 consomem números **antes** desta.
SQL plano idempotente, schema-qualificado, **sem `-- +goose Down`** (o migrator roda o arquivo
inteiro e o Down se auto-destrói).

```sql
create schema if not exists messaging;

-- Evidencia de que a politica rodou. Sem registro, "temos purge" e' afirmacao
-- sem prova — e e' o que um DPO/auditor pede primeiro.
create table if not exists messaging.purge_runs (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    class text not null,
    mode text not null default 'delete',   -- delete | dry_run
    cutoff_at timestamptz not null,
    rows_deleted bigint not null default 0,
    rows_scrubbed bigint not null default 0,
    files_deleted bigint not null default 0,
    bytes_freed bigint not null default 0,
    started_at timestamptz not null default now(),
    finished_at timestamptz,
    error text not null default '',        -- MASCARADO (C6)
    constraint messaging_purge_runs_class_ck
        check (class in ('audit','conversation','ai_io','ephemeral','media_orphan')),
    constraint messaging_purge_runs_mode_ck check (mode in ('delete','dry_run'))
);
create index if not exists messaging_purge_runs_account_idx
    on messaging.purge_runs (account_id, started_at desc);

-- cost_priced: distingue "custo zero" de "preco nao cadastrado" (C7). Sem a coluna
-- a tela teria de adivinhar por heuristica (tokens>0 e custo=0) e mentiria em R$ 0,00.
alter table messaging.ai_runs
    add column if not exists cost_priced boolean not null default false;
```

`purge_runs` é classe `audit` e **poda a si mesma** aos 365 dias — não é piada: sem isso a tabela
de evidência cresce para sempre.

### C4 — O job de purge (sobre o `platform/jobs` da F3 — **sem scheduler novo**)

**O trabalho roda no `platform/jobs`.** O que enfileira é um ticker diário — o **padrão já
existente na casa**, não um scheduler novo: `app.go:115` (`operationsAlertMonitorInterval`),
`app.go:132` (`feedbackAttachmentCleanupInterval`) e `cardapio/telemetry_retention.go`
(`startRetentionLoop`: primeiro disparo ~5 min após o boot, fora do caminho crítico de subida,
depois a cada 24h). **Copiar essa mecânica.**

| Peça | Contrato |
|---|---|
| Enfileirador | Ticker 24h; para toda conta com o módulo **habilitado** (`core.account_modules where module_id = 'omnichannel' and enabled`) insere **um** job. Lista de contas vem do catálogo — nunca hardcoded |
| `kind` | `omnichannel.purge.account` |
| `idempotency_key` | `purge:{accountId}:{YYYY-MM-DD}` → `unique (account_id, idempotency_key)` da F3.2 torna re-boot/duplo tick **no-op**, de graça |
| `ordering_key` | `purge:{accountId}` → purges da mesma conta **serializam**; não disputa `ordering_key` de conversa, logo **não atrasa envio** (F3.2 é head-of-line **por chave**) |
| `payload` | `{ "accountId": "...", "date": "YYYY-MM-DD", "dryRun": false }` — **sem PII** (F3.2) |
| Retry | Classificação da F3.2: erro de banco = transitório → 5 tentativas, backoff; esgotou → `dead` + `last_error` **mascarado** |
| Ordem interna | `ephemeral` → `ai_io` → `conversation` → `audit`. Uma linha em `purge_runs` **por classe** |
| Batches | Máx. **500 linhas por batch**, transação por batch, teto de **5 min** por job. Atingiu o teto → re-enfileira `purge:{accountId}:{date}:{seq+1}` e sai. Nunca uma transação gigante segurando lock da tabela quente do inbox |
| Dry-run | `dryRun: true` conta e **não apaga**; grava `purge_runs.mode='dry_run'`. É como a política estreia em produção sem susto — ver *Verificável* 1 |

### C5 — Política de mídia (o gap que nenhum módulo tem hoje)

Mídia vive **em disco** (F6, C3): `{OMNICHANNEL_MEDIA_DIR}/{accountId}/{conversationId}/{random}.{ext}`,
referenciada por `messaging.messages.media_storage_key`. O banco não a alcança: **`DELETE` de
mensagem não libera byte nenhum** — o arquivo com o áudio do cliente fica no volume para sempre.

| Regra | Detalhe |
|---|---|
| **Ordem: arquivo primeiro, linha depois** | Dentro do batch: `select id, media_storage_key` → apaga **arquivos** → apaga **linhas**. Se o job morrer no meio, sobra linha apontando para arquivo ausente (`/media` → 404, que é **exatamente o que a retenção queria**) e o próximo batch limpa a linha. Na ordem inversa sobra **arquivo órfão invisível** — PII no disco sem nenhuma linha que a denuncie |
| Escopo de path | Só apaga sob `{OMNICHANNEL_MEDIA_DIR}/{accountId}/`. Resolver o path final e **conferir o prefixo** antes de qualquer `os.Remove` — `media_storage_key` é dado de banco, e path traversal aqui apaga arquivo de outra conta |
| Varredura de órfãos | `kind` = `omnichannel.purge.media_orphan`, **semanal**, por conta: caminha `{DIR}/{accountId}/` e remove arquivo **sem `media_storage_key` correspondente**. Cobre o upload que gravou o arquivo e falhou antes do insert (F6) |
| **Janela de carência** | A varredura **ignora arquivo com mtime < 24h**. Sem isso ela apaga o arquivo recém-gravado cuja transação ainda não commitou — o atendente manda a foto e ela some sozinha |
| Backup | O volume de mídia **não está no `pg_dump`** — ver notas de deploy 4 |
| Contabilidade | `files_deleted` e `bytes_freed` em `purge_runs` (é o único jeito de provar que o disco foi liberado) |

### C6 — Masking (payload bruto **nunca** em log, erro ou trace)

Conversa de WhatsApp de cliente final em log é vazamento de dado pessoal — **e log sai da VPS**.
Não existe helper genérico hoje: o único `mask` do repositório é `calendar/secrets.go:44`, e é para
**chave de API**, não para PII. Nasce `back/internal/platform/logmask/`.

```go
package logmask

func Text(s string) string                 // NUNCA o conteudo: "" ou "[redacted:142]" (so o tamanho)
func Phone(s string) string                // "********4321" (ultimos 4 — mesma logica do calendar/secrets.go:44)
func Email(s string) string                // "a***@dominio.com"
func Doc(s string) string                  // CPF/CNPJ: so os 2 ultimos
func JSON(raw []byte, allow []string) []byte // ALLOWLIST: chave fora da lista tem o valor mascarado
```

| Regra | Detalhe |
|---|---|
| **Allowlist, nunca denylist** | Denylist não conhece o campo que o provider adiciona mês que vem — e o provider muda sem avisar. O que não está na lista **é mascarado**. `JSON` preserva a **estrutura** (chaves e formato), mascara os **valores folha** |
| Log estruturado | Campos explícitos, no modelo do `logAI` (`calendar/ai_dispatch.go`): `op`, `account_id`, `conversation_id`, `error`. **Nunca** `%+v`/`%v` de struct que carregue payload — é o vazamento acidental clássico |
| Erro | `last_error` (outbox) e `purge_runs.error` = classe + status code + mensagem do **nosso** lado. **Nunca** o corpo de resposta do provider (ele devolve o payload de volta) |
| Trace | **Não há stack de tracing hoje** — nenhum `otel`/`opentelemetry` no `back/go.mod` (verificado). A regra do canônico §10 vale para quando existir: nenhum atributo de span carrega payload |

**Retrofit — a F13 é a DONA da regra, mas os call-sites nascem antes dela.** Auditar e corrigir,
não presumir limpos:

| Call-site | Fase dona | O que conferir |
|---|---|---|
| `messaging.webhook_events.payload_masked` | F4 (C5) | Já é especificado como mascarado (telefone → últimos 4, corpo omitido). Confirmar que usa o `logmask` e **não** uma cópia local |
| `messaging.outbox.payload` / `last_error` | F3.2 / F6 | "sem PII crua" e "mascarado" já constam. Confirmar na prática |
| `messaging.ai_runs.input` | F9 (C9.1) | "MASCARADO" já consta. Confirmar que o mascaramento é do `logmask` e cobre a jsonb inteira |
| `messaging.audit_events.payload_json` | F7 (C5) | **Não há regra escrita de masking para esta coluna em nenhuma fase.** É gravada em ações de mensagem — passar pelo `logmask.JSON` com allowlist |
| Handler do webhook (rota pública) | F4 | O ponto de maior risco: erro de parse tentando logar o body para "ajudar a depurar" |

### C7 — Custo de LLM por conta

Fonte: `messaging.ai_runs` (F9, C9.1) — `provider`, `model`, `prompt_tokens`, `completion_tokens`,
`total_tokens`, `cost_usd numeric(12,6)`. **Nenhuma fase diz de onde sai o preço** que preenche
`cost_usd` (ver *Divergências* nº3) — a F13 fecha o gap.

**Tabela de preço** em `core.platform_settings`, chave `omnichannel_llm_pricing`, escrita por
`platform_admin` (mesma restrição do `0160`), lida por `platform/llm/pricing.go`:

```json
{ "openai/gpt-4o-mini": { "inputPer1k": 0.00015, "outputPer1k": 0.0006, "currency": "USD" } }
```

| Regra | Detalhe |
|---|---|
| **Congelado na escrita** | `cost_usd` é calculado **no dispatch da F9**, com o preço vigente naquele instante, e nunca recalculado. Recalcular no `read` faz a fatura do mês passado mudar quando o preço muda — histórico que se reescreve não é histórico |
| Preço ausente | `cost_priced = false`, `cost_usd = 0`. A tela mostra **"preço não cadastrado para {provider}/{model}"**, jamais "US$ 0,00" (princípio 5: aviso honesto, não default que minta) |
| Retrofit | Um call-site: onde a F9 grava o `ai_runs` (`ai_triage.go`/`ai_dispatch.go`). A F13 **não** reescreve o dispatch, só liga o preço |
| Moeda | **USD** — é o que a coluna é (`cost_usd`) e o que os provedores cobram. Conversão para BRL exige taxa de câmbio, que **não tem fonte na plataforma**: fora de escopo, não inventar |
| Backfill | Runs anteriores à tabela de preço ficam `cost_priced = false`. Backfill → **P1** |

**Rota** (dentro do gate de módulo, `RequireAuthWithAccount`, `account_id` do Principal):

| Rota | Permissão | Resposta |
|---|---|---|
| `GET /v1/omnichannel/ai/usage?from=&to=` | `omnichannel.audit.view` | `{ totals: {runs, promptTokens, completionTokens, totalTokens, costUsd}, byModel: [{provider, model, runs, totalTokens, costUsd, priced}], unpricedModels: [], limit: {monthlyAiRuns, used, source} \| null }` |

- **Agrega; não lista.** `GET /agents/{id}/runs` (F9, C9.5) já lista run a run — não duplicar.
- `limit` casa com `monthly_ai_runs` do `account_modules.config` (leitor da F3): custo e teto na
  mesma tela, senão o número não decide nada. `null` = **"Sem limite cadastrado"** na tela, não `0`.
- `from`/`to` default = mês corrente. Índice que serve: `messaging_ai_runs_account_created_idx`
  (`account_id, created_at desc`), **já criado pela F9**.

**Front:** aba nova no drawer da F10 (`OmnichannelConfigDrawer.vue`, deep-link `?config=usage`) —
`ConfigAiUsage.vue`, design system da casa, tokens (nunca hex). Gate:
`isPlatformAdmin || has('omnichannel.audit.view')` — `platform_admin` tem `has()` = **false** no
front e a aba sumiria justamente para quem administra.

### C8 — **P1** (fora do piloto — não implementar no mínimo)

- **C8.1 Export do titular.** Dado de um contato (conversas, mensagens, mídia, campos extraídos)
  em arquivo. Decisões abertas **a confirmar com o dono**: formato, se a mídia vai junto, quem
  autoriza (`omnichannel.audit.view` ou permissão nova) e como o arquivo é entregue sem virar link
  público eterno.
- **C8.2 Anonimização a pedido.** Substituir PII do contato preservando a linha (o histórico
  operacional/estatístico sobrevive; a pessoa some). **A confirmar:** anonimizar ou apagar (a LGPD
  aceita os dois em situações diferentes), e o que fazer com a conversa **aberta** de um titular
  que pede exclusão.

---

## Armadilhas / o que NÃO fazer

| Não faça | Porquê |
|---|---|
| Escrever política sem job | "Política sem job é política que não existe" (canônico §9.2 F13). Config de retenção que ninguém executa é pior que nada: dá a impressão de conformidade |
| Inventar scheduler/cron novo | O trabalho roda no `platform/jobs` (F3). O ticker **só enfileira**, no padrão de `app.go:115/132` e `cardapio/telemetry_retention.go` |
| Reusar o fallback "ausente = sem limite" da F3.4 | Em retenção, "sem limite" = **nunca apagar**. Cadeia própria, terminando em constante Go (C2) |
| `DELETE` de `ai_runs` na classe `ai_io` | Destrói a base do custo por conta que **esta mesma fase** entrega. Scrub das colunas, linha preservada (C1.2) |
| Esquecer o scrub de `ai_runs` ao podar a conversa | `ai_runs.conversation_id` **não tem FK** (F9, C9.1) → não cascateia → PII sobrevive à conversa apagada (C1.3) |
| Podar mensagem por `created_at` da mensagem | Conversa ativa perde histórico no meio do atendimento. Âncora é `conversations.last_message_at` (C1.1) |
| Apagar a linha antes do arquivo | Órfão invisível: PII no disco sem nenhuma linha que a aponte (C5) |
| Varrer órfãos sem janela de carência | Apaga o upload em voo cuja transação ainda não commitou (C5) |
| Uma transação para podar tudo | Lock na tabela quente do inbox. Batches de 500 + teto de tempo (C4) |
| Denylist de campos no masking | Não conhece o campo que o provider adiciona depois. **Allowlist** (C6) |
| Logar struct com `%+v` "só para depurar" | É como o payload bruto chega no log — e log sai da VPS (C6) |
| Recalcular `cost_usd` na leitura | Reescreve a fatura do mês passado quando o preço muda (C7) |
| Mostrar "US$ 0,00" para modelo sem preço | Default que minta (princípio 5). `cost_priced = false` → aviso explícito (C7) |
| Implementar export/anonimização no P0 | É **P1**, explicitamente fora do piloto. Fazer aqui atrasa o piloto por escopo que o dono cortou |
| Cravar o número da migration | Dois `0197` no disco; a última é `0199`; F2/F3/F4/F6/F8/F9 consomem antes. Conferir o disco (C3) |
| `-- +goose Down` | O migrator roda o arquivo **inteiro**: o Down se auto-destrói no mesmo boot |

## Segurança

| Item | Regra |
|---|---|
| Escopo | `account_id` **sempre** do Principal, **nunca** do body — em `/ai/usage` e em toda leitura de `purge_runs`. O job recebe `accountId` no payload porque **ele mesmo é o produtor** (enfileirador server-side), nunca uma requisição |
| Defesa em profundidade | Todo `select`/`update`/`delete` do purge carrega `account_id = $1` **na query**, além da validação do service. Purge sem filtro de conta é o pior bug possível desta fase: apaga dado de todos os tenants |
| Fora de escopo | **404, nunca 403** (403 confirma existência — enumeration) |
| Path de mídia | Path resolvido **e conferido contra o prefixo** `{DIR}/{accountId}/` antes de `os.Remove`. `media_storage_key` é dado de banco: tratar como não-confiável |
| Preço / config global | `core.platform_settings` é platform-global (sem `account_id`, por decisão do `0160`): escrita **só** `platform_admin`, na camada de serviço |
| Logs do purge | `op`, `account_id`, `class`, contagens. **Nunca** id/telefone/texto das linhas apagadas — o log da poda não pode virar a cópia de segurança da PII que a poda apagou |
| Dado do titular | `logmask` em **toda** persistência de payload (C6). Payload bruto não existe em log, erro ou trace (canônico §10) |

## Verificável

Um humano prova (browser/banco/`psql`):

1. **Dry-run primeiro.** Enfileirar o job com `dryRun: true` → `select class, mode, rows_deleted, files_deleted from messaging.purge_runs order by started_at desc` mostra as contagens **e nada foi apagado** (`count(*)` das tabelas inalterado). É assim que a política estreia em produção.
2. **Purge de conversa.** Inserir conversa com `last_message_at = now() - interval '200 days'` + mensagem com mídia real em `{OMNICHANNEL_MEDIA_DIR}/{acct}/{conv}/`. Rodar o job → a linha some, **o arquivo some do disco** (`ls` no volume), `purge_runs` registra `rows_deleted`/`files_deleted`/`bytes_freed` > 0.
3. **Conversa ativa não é podada.** Conversa com `last_message_at = now()` contendo mensagem de 200 dias → **nada é apagado**. Prova que a âncora é a conversa, não a mensagem.
4. **Custo sobrevive ao purge.** Depois do passo 2: `select input, output, total_tokens, cost_usd from messaging.ai_runs where conversation_id = '<a conversa apagada>'` → `input`/`output` = `{}` e **`total_tokens`/`cost_usd` intactos**. A tela de custo do mês continua mostrando o valor.
5. **`ai_io` aos 90 dias.** Run com `created_at = now() - interval '100 days'` em conversa **ativa** → `input`/`output` zerados, contadores intactos, conversa intocada.
6. **Masking.** Mandar mensagem do celular com texto conhecido (`"meu cpf é 111.444.777-35"`). Depois: `docker compose logs api | grep -i "111.444.777"` → **zero linhas**; `grep` pelo texto → **zero linhas**; `select payload_masked from messaging.webhook_events order by created_at desc limit 1` → sem o corpo, telefone só com os últimos 4.
7. **Custo na tela.** Aba de custo (`?config=usage`) bate com `select provider, model, count(*), sum(total_tokens), sum(cost_usd) from messaging.ai_runs where account_id = '<uuid>' and created_at >= date_trunc('month', now()) group by 1,2`.
8. **Preço não cadastrado.** Remover o modelo de `omnichannel_llm_pricing` → nova triagem grava `cost_priced = false` → a tela mostra **"preço não cadastrado"**, e **não** US$ 0,00.
9. **Retenção por conta.** `update core.account_modules set config = jsonb_set(config, '{retention_days,conversation}', '30') where account_id = '<A>' and module_id = 'omnichannel';` → A poda em 30 dias, B (sem a chave) segue em 180. A tela mostra `source: account` em A e `source: default` em B. Valor `0` → **409**.
10. **Isolamento.** Duas contas com conversas velhas; rodar o purge **só** da conta A → as linhas de B continuam lá, e `purge_runs` de A não menciona B.
11. **Órfão de mídia.** Criar arquivo em `{DIR}/{acct}/{conv}/orfao.jpg` sem linha correspondente → varredura **não** apaga enquanto mtime < 24h; com mtime forçado para 48h atrás (`touch -d`) → apaga e contabiliza.

## Notas de Deploy

**Ordem exata:** migration → `build --no-cache api` → `up -d api` → backup da mídia → seed do preço.

| # | Item | Detalhe |
|---|---|---|
| 1 | Migration `<próximo livre>_messaging_retention.sql` | `purge_runs` + `ai_runs.cost_priced`. **Conferir o disco antes de numerar** (dois `0197`; última `0199`; F2/F3/F4/F6/F8/F9 consomem antes). Idempotente, **sem `-- +goose Down`** |
| 2 | `docker compose build --no-cache api` | **Obrigatório:** migrations são `embed.FS` e o cache da camada `go build` pode **não re-embutir** o `.sql` novo. Sintoma: `migrate status` para na anterior, **sem erro** |
| 3 | `docker compose up -d api` | Mudou `back/` → rebuild; restart não basta |
| 4 | **Mídia no backup** | Canônico §13 item 7. `scripts/backup/backup-db.sh` é **`pg_dump` puro** e `docs/BACKUP_RESTORE.md:210-211` **registra o gap por escrito**: *"Volume `api_uploads` — midias/uploads da api nao entram neste backup... Backup manual do volume Docker por enquanto."* O volume de mídia da F6 nasce com o **mesmo gap**. Incluir o volume no `backup-db.sh` (mesmo `flock`, mesma retenção diária/semanal, mesmo `MIN_BYTES`, mesmo off-site `rclone`) e **atualizar o runbook** — senão a mídia da conversa não tem backup nenhum |
| 5 | Seed do preço | `core.platform_settings` chave `omnichannel_llm_pricing` com os modelos em uso. **Sem ela o custo grava `cost_priced = false`** e a tela avisa (não quebra) |
| 6 | Retenção da VPS | Rodar o **dry-run** (C4) na primeira subida em produção **antes** de deixar o `delete` correr. `select * from messaging.purge_runs where mode = 'dry_run'` é a conferência |

**Sem env var nova** (retenção vem do banco — princípio 1) e **sem container novo**.
`OMNICHANNEL_MEDIA_DIR` e `OMNI_SECRETS_KEY` já são entregas de F6 e F3.

---

## Divergências com o canônico / prompt (registradas, não decididas por conta própria)

| # | Ponto | O canônico diz | Esta spec faz | Por quê |
|---|---|---|---|---|
| 1 | Blockers | §9.2 F13: *"Blockers: F9"* | Depende de **F9** formalmente; na prática o purge só poda tabela que exista (F2/F4/F6/F7/F8) e o retrofit de masking (C6) toca call-sites de F4/F6/F9. A **F7** declara, do lado dela, que bloqueia a F13 (`OMNI-F7.md:28`) | Como F13 é a **última** do piloto P0 (`F0 → F10 + F13-mínimo`), a ordem se resolve sozinha e nada trava. Registrado para quem executar fora dessa ordem: cada classe poda **só o que existe** |
| 2 | Onde mora a tela de custo | §9.2 F13 pede "custo LLM por conta na tela"; F13 não depende da F10 | Aba no **drawer da F10** (`OmnichannelConfigDrawer.vue`) | Criar página própria duplicaria o host de config. Se a F10 não estiver no ar, a aba não tem onde encaixar — **dependência de front real**, ainda que não seja blocker de dado. Ambas são P0 do mesmo piloto |
| 3 | Origem do preço do LLM | §7.1 e a F9 (C9.1) criam `ai_runs.cost_usd`, e a F9 quer que `sum(cost_usd)` bata com o painel do provider (Verificável 5) — mas **nenhuma fase diz de onde sai o preço** | F13 cria `omnichannel_llm_pricing` em `core.platform_settings` + `platform/llm/pricing.go` e **retrofita** o cálculo no dispatch da F9 | Sem tabela de preço, `cost_usd` fica `0` e a tela desta fase mostra fatura zerada. Gap real, não redecisão |
| 4 | Mapa classe → tabela | §10 fixa **365/180/90/30** mas **não diz qual dado cai em qual classe** | Mapa da C1 (`audit`/`conversation`/`ai_io`/`ephemeral`), com `ai_io` = **scrub** e não delete | Preencher o vazio é entrega desta fase. **Se o dono discordar do mapa, muda na C1** — não no código |
| 5 | Auditoria de conversa apagada | §10 pede `audit` em 365d | `routing_decisions` tem `conversation_id ... on delete cascade` (F8, Contrato 1) → auditoria **ligada a conversa morre com ela**, aos 180d. A classe 365 vale para auditoria de **nível conta** (`audit_events`, `purge_runs`) | Não é contradição a corrigir: manter a trilha de roteamento de um titular **depois** de apagar os dados dele seria o oposto do que a retenção quer. Registrado para não parecer descuido |
