# OMNI-F9 — Triagem IA no Go

**Prioridade:** P0
**Plano canônico:** `docs/omnichannel/PLANO_ATENDIMENTO.md` (§9.2 F9, §6, §7.1, §8)

> ## LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono)
>
> A branch `refactor/multi-tenant-complete` fechou e o dono **liberou a implementação em
> 2026-07-17** (decisão **D-D**, canônico §2). O aviso de congelamento que constava aqui não vale mais.

Ler a skill `principios-engenharia` antes de começar.

---

## Objetivo

A triagem por IA roda **dentro do Go**, sem n8n no caminho crítico. Uma mensagem inbound faz o
agente extrair campos e sugerir destino; **o motor determinístico da F8 decide** o roteamento
lendo `routing_rules`. Provider/modelo/prompt vêm do painel (banco), a saída é JSON validado
contra schema versionado, e cada execução grava custo/usage em `ai_runs`.

## Depende de / Bloqueia

| Relação | Fase | Motivo |
|---|---|---|
| **Depende** | **F3** | `platform/llm` (client + structured output + `usage`), `platform/secretbox` (chave do provider), leitor de limites do `account_modules.config` |
| **Depende** | **F8** | máquina de estados (`state`), `queues`/`departments`, `routing_rules`, `routing_decisions`, motor determinístico |
| **Bloqueia** | **F10** | editor de agente + simulador consomem as rotas desta fase |
| **Bloqueia** | **F13** | custo LLM por conta lê `ai_runs` |

Não depende de F4–F7: a triagem é testável pelo `simulate` sem canal real.

---

## Entregas

| # | Entrega | Alvo |
|---|---|---|
| 1 | Migration: `ai_agents`, `ai_agent_versions`, `ai_runs`, `collect_field_defs` | `back/internal/platform/database/migrations/` (numeração: ver Notas de Deploy) |
| 2 | Domínio do agente + versionamento (publish/rollback) | `back/internal/modules/omnichannel/ai_agents.go` |
| 3 | Builder do prompt de 8 camadas | `back/internal/modules/omnichannel/ai_prompt.go` |
| 4 | Chamada LLM + validação de schema + persistência do run | `back/internal/modules/omnichannel/ai_triage.go` |
| 5 | Gate de `human_active` (hard-block) + gate de limite mensal | `back/internal/modules/omnichannel/ai_dispatch.go` |
| 6 | Store (WHERE `account_id` em toda query) | `back/internal/modules/omnichannel/store_ai.go` |
| 7 | Rotas HTTP de agente/versões/campos/runs/simulate | `back/internal/modules/omnichannel/http_ai.go` |
| 8 | Permissões `omnichannel.agents.manage` / `.audit.view` no `Permissions()` do módulo | `back/internal/modules/omnichannel/module.go` |
| 9 | Atualizar `back/internal/modules/omnichannel/AGENT.md` (nasce na F2; aqui recebe a seção de IA) | idem |

Teto de ~450 linhas/arquivo **vale** — é código novo.

---

## Contratos

### C9.1 — Migration (SQL plano idempotente, sem `-- +goose Down`, schema-qualificado)

```sql
create schema if not exists messaging;

create table if not exists messaging.ai_agents (
    id uuid not null default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    slug text not null,
    name text not null default '',
    enabled boolean not null default false,
    active_version_id uuid,
    created_by text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (id)
);
create unique index if not exists messaging_ai_agents_account_slug_uidx
    on messaging.ai_agents (account_id, slug);

-- Versao PUBLICADA e IMUTAVEL: editar = criar version nova. Rollback = repontar
-- ai_agents.active_version_id para uma version anterior (nunca reescrever a linha).
create table if not exists messaging.ai_agent_versions (
    id uuid not null default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    agent_id uuid not null references messaging.ai_agents(id) on delete cascade,
    version integer not null,
    status text not null default 'draft', -- draft | published | archived
    provider text not null default '',    -- do painel; NUNCA supor
    model text not null default '',
    temperature numeric(3,2) not null default 0.20,
    layers jsonb not null default '{}'::jsonb,        -- as 8 camadas editaveis
    output_schema jsonb not null default '{}'::jsonb, -- JSON Schema da saida
    schema_version text not null default 'v1',
    published_at timestamptz,
    published_by text not null default '',
    created_at timestamptz not null default now(),
    primary key (id)
);
create unique index if not exists messaging_ai_agent_versions_agent_version_uidx
    on messaging.ai_agent_versions (agent_id, version);

create table if not exists messaging.collect_field_defs (
    id uuid not null default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    agent_id uuid not null references messaging.ai_agents(id) on delete cascade,
    key text not null,
    label text not null default '',
    field_type text not null default 'text', -- text|number|email|phone|date|enum
    enum_options jsonb not null default '[]'::jsonb,
    required boolean not null default false,
    sort_order integer not null default 0,
    primary key (id)
);
create unique index if not exists messaging_collect_field_defs_agent_key_uidx
    on messaging.collect_field_defs (account_id, agent_id, key);

-- Uma linha por TENTATIVA de triagem, inclusive as que nao chamaram o modelo
-- (blocked/limit_exceeded) — a trilha precisa explicar o silencio da IA.
create table if not exists messaging.ai_runs (
    id uuid not null default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    conversation_id uuid not null,
    agent_id uuid,
    agent_version_id uuid,
    message_id uuid,
    status text not null default 'ok', -- ok|schema_invalid|provider_error|blocked|limit_exceeded
    provider text not null default '',
    model text not null default '',
    schema_version text not null default '',
    input jsonb not null default '{}'::jsonb,  -- MASCARADO (§10 do canonico)
    output jsonb not null default '{}'::jsonb,
    prompt_tokens integer not null default 0,
    completion_tokens integer not null default 0,
    total_tokens integer not null default 0,
    cost_usd numeric(12,6) not null default 0,
    latency_ms integer not null default 0,
    error text not null default '',
    created_at timestamptz not null default now(),
    primary key (id)
);
create index if not exists messaging_ai_runs_account_created_idx
    on messaging.ai_runs (account_id, created_at desc);
create index if not exists messaging_ai_runs_conversation_idx
    on messaging.ai_runs (conversation_id, created_at desc);
```

### C9.2 — Prompt de 8 camadas

> **Derivação — confirmar com o dono antes de codar.** O canônico (§9.2) exige "prompt em 8
> camadas" mas **não enumera as camadas**; a spec externa que as definia não é versionada e
> não está no disco. A lista abaixo é proposta, não verbatim.

Ordem fixa; cada camada é um campo em `ai_agent_versions.layers` e é editável no painel (F10):

| # | Camada | Fonte |
|---|---|---|
| 1 | Identidade / persona | `layers.identity` |
| 2 | Objetivo da triagem | `layers.goal` |
| 3 | Contexto da conta | `layers.context` |
| 4 | Catálogo de destinos (setores/filas — **só para sugerir**) | derivado de `departments`/`queues` (F8) |
| 5 | Campos a coletar | derivado de `collect_field_defs` |
| 6 | Guardrails (o que não prometer) | `layers.guardrails` |
| 7 | Histórico da conversa (janela) | derivado de `messaging.messages` |
| 8 | Contrato de saída (JSON Schema + `schema_version`) | `output_schema` |

Camadas 4/5/7 são **montadas server-side do banco** — não são texto livre; o painel não pode
digitar uma fila que não existe.

### C9.3 — Saída (JSON schema-validado, `schema_version: v1`)

```json
{
  "intent": "string",
  "confidence": 0.87,
  "extracted_fields": { "<key de collect_field_defs>": "valor" },
  "suggested_department": "vendas|null",
  "suggested_queue": "vendas-sp|null",
  "needs_human": false,
  "reply_draft": "string|null"
}
```

**REGRA CENTRAL — a IA SUGERE, o motor DECIDE.** `suggested_*` é **entrada** do motor
determinístico da F8, nunca destino final. Quem roteia é o código lendo `routing_rules`, e a
decisão é gravada em `routing_decisions` (F8). Nenhum caminho pode escrever
`conversation.queue_id` a partir do output do modelo.

Saída que não valida contra `output_schema` → **1 retry** → falhando, `ai_runs.status =
schema_invalid` e a conversa segue **pelo fallback determinístico da F8** (sem campos
extraídos). A triagem nunca trava a conversa.

### C9.4 — Interfaces Go

```go
// O client de LLM vem da F3 (platform/llm): consumir, NÃO redefinir aqui.
type TriageInput struct{ AccountID, ConversationID, MessageID string }

type TriageOutput struct {
    Intent                           string
    Confidence                       float64
    ExtractedFields                  map[string]any
    SuggestedDepartment, SuggestedQueue string
    NeedsHuman                       bool
    ReplyDraft                       string
}

type TriageService interface {
    // Run monta o prompt, chama o LLM, valida o schema e grava o ai_runs.
    // Devolve a SUGESTAO — quem decide o roteamento é o motor da F8.
    Run(ctx context.Context, in TriageInput) (TriageOutput, error)
}
```

### C9.5 — Rotas (`/v1/omnichannel`, sob o gate de módulo, `RequireAuthWithAccount`)

| Rota | Permissão | Nota |
|---|---|---|
| `GET|POST /v1/omnichannel/agents` | `omnichannel.agents.manage` | |
| `GET|PATCH /v1/omnichannel/agents/{id}` | `omnichannel.agents.manage` | `enabled`, nome |
| `GET|POST /v1/omnichannel/agents/{id}/versions` | `omnichannel.agents.manage` | POST cria **draft** |
| `POST /v1/omnichannel/agents/{id}/versions/{v}/publish` | `omnichannel.agents.manage` | publica e repointa `active_version_id` |
| `POST /v1/omnichannel/agents/{id}/rollback` | `omnichannel.agents.manage` | body `{versionId}`; repointa, não reescreve |
| `GET|POST|PATCH|DELETE /v1/omnichannel/agents/{id}/collect-fields` | `omnichannel.agents.manage` | |
| `POST /v1/omnichannel/agents/{id}/simulate` | `omnichannel.agents.manage` | body `{versionId?, messages[]}` + resposta com o traço → **C9.7**; **não** persiste conversa; grava `ai_runs`; consome limite |
| `GET /v1/omnichannel/agents/{id}/runs` | `omnichannel.audit.view` | `limit` 1..200 default 50 + `beforeId` (mesmo padrão do `SPECS_PORT_OMNICHANNEL.md` F2) |

A **chave** do provider nunca entra nesses shapes: resolve server-side pelo `secretbox` (F3) e
sai mascarada `{set,last4}` — modelo do `calendar/secrets.go`.

**A F10 CONSOME estas rotas — não recria.** A F10 é *telas* (canônico §9.2 F10); o dono do contrato
é quem tem a tabela. Os paths `/agents/*` acima são os **definitivos** e casam com a permission key
`omnichannel.agents.manage` (canônico §5.2) — **não existe `/ai-agents/*`**. Publish é
`POST /agents/{id}/versions/{v}/publish` (**versão no path**, o recurso publicado é a version), e
não `POST /agents/{id}/publish` com `{versionId}` no body.

### C9.6 — Gates do dispatch (ordem exata, antes de qualquer chamada ao modelo)

| # | Gate | Efeito |
|---|---|---|
| 1 | `conversation.state == human_active` | **HARD-BLOCK.** Não chama LLM. `ai_runs.status = blocked`, tokens 0. Substitui o `paused_until` (§6 do canônico) |
| 2 | `ai_agents.enabled = false` ou sem `active_version_id` | não roda; sem `ai_runs` |
| 3 | `monthly_ai_runs` (do `account_modules.config`, leitor da F3) estourado | não chama LLM. `ai_runs.status = limit_exceeded`, tokens 0. **Inbound degrada para o fallback da F8**; só `simulate` responde **409** acionável |
| 4 | provider/modelo/chave ausentes | não roda; `ai_runs.status = provider_error` |

### C9.7 — `simulate` (dry-run): contrato desta fase, consumido pela F10

Reconciliação com a F10, que tabelava `POST /ai-agents/{id}/simulate` com body
`{versionId?, message:{text}, contact?}`. **O dono é a F9** (tem as tabelas e o `ai_runs`), então o
path e o body são os daqui; o que a F10 trouxe de substância **fica**:

- **`messages[]`, não `message` único.** A camada **7** do prompt (C9.2) é o *histórico da conversa*
  — simulador de uma mensagem só não exercita justamente a camada que mais muda o resultado, e o
  operador publicaria confiando num teste que não testa.
- **`versionId?` fica** (veio da F10): sem ele não dá para **testar o rascunho antes de publicar**,
  que é o motivo de o simulador existir.

```
POST /v1/omnichannel/agents/{id}/simulate
body: { versionId?: string,
        messages: [{ role: "contact"|"agent", text: string }],
        contact?: { name?: string } }
resp: { output: {...}, schemaVersion: string, valid: boolean, validationErrors: [],
        extractedFields: {...}, matchedRule: { id, name, priority } | null,
        wouldRoute: { departmentId, queueId } | null,
        usage: { promptTokens, completionTokens, totalTokens, costUsd } }
```

- `versionId` ausente = `active_version_id` (versão publicada).
- **Grava `ai_runs` e consome o limite mensal** — a simulação chama o modelo de verdade, com custo
  real que precisa aparecer na F13. Limite estourado → **409** acionável (gate 3 da C9.6). *Isto
  está decidido aqui; a F10 não re-decide.*
- **NUNCA** envia mensagem, **NUNCA** grava em `outbox`, **NUNCA** cria conversa, **NUNCA** muda
  `state`. Roda o LLM + o motor determinístico da F8 (`Decide`) e devolve o traço.
- `matchedRule`/`wouldRoute` vêm do motor da F8 — é o que prova "**IA sugere, motor decide**" para
  quem está configurando, sem precisar mandar mensagem de verdade.
- `output` passa pela mesma validação de schema da C9.3; inválido → `valid:false` +
  `validationErrors` preenchido (a tela mostra), **não** 500.

---

## Armadilhas / o que NÃO fazer

| Não faça | Por quê |
|---|---|
| **Supor provider** (`openai` porque "é o mais comum") | Provider/modelo/prompt vêm **do banco**. Regra da casa (`feedback_ai_config_from_panel`): NUNCA supor provider — checar o registro. Hoje o calendário usa `openai|gemini|glm` |
| Ligar `suggested_queue` direto no roteamento | Mata a auditabilidade (`routing_decisions`) e torna o roteamento não-testável sem chamar modelo. A IA **preenche campos; a regra decide** |
| Trocar `human_active` por timer/`paused_until` | Janela expira sozinha e o bot fala por cima do atendente. Estado é mais honesto que timer (§6) |
| Editar uma version `published` | Publicada é **imutável**. Editar = version nova; rollback = repointar `active_version_id`. Sem isso o `ai_runs` aponta para um prompt que não existe mais |
| Escrever um client LLM novo | `platform/llm` é da F3. Não importar de `modules/calendar` (é escopo do calendário, não plataforma) |
| Logar payload/prompt bruto | §10 do canônico: **mascarado, nunca em log** — nem em erro, nem em trace. Vale para `ai_runs.input` |
| Fazer a triagem depender do n8n | D-C: a triagem **sobrevive ao n8n desligado**. n8n = periferia |
| Cravar o número da migration | Há **dois `0197`**; a última é `0199`. F2 e F8 consomem números antes desta — conferir o disco |
| `-- +goose Down` | O migrator roda o arquivo **inteiro**: o Down se auto-destrói no mesmo boot |
| Deixar a conversa travada quando a IA falha | Schema inválido / limite / provider fora → **fallback determinístico da F8**, sempre |

---

## Segurança

- `account_id` **sempre** do Principal, **nunca** do body — em todas as rotas e no `simulate`.
- Store filtra por `account_id` em **toda** query (defesa em profundidade, princípio 2), mesmo
  com o service já tendo validado.
- Agente / version / run / campo de outra conta → **404, nunca 403** (403 vaza existência).
- Chave do provider: só via `platform/secretbox` (F3). Nunca em coluna crua, nunca em log,
  nunca de volta pro front — só `{set,last4}`.
- `ai_runs.input` mascarado antes de persistir (telefone, e-mail, documento).
- Permissão gateia **feature**; fila gateia **dado** (§5.2). `agents.manage` não dá acesso a
  conversa nenhuma — a leitura de conversa continua sob o filtro de fila da F8.

---

## Verificável

| # | Prova (browser/banco) | Fecha |
|---|---|---|
| 1 | Publicar version com prompt A → mandar msg → IA extrai campo X. Publicar version B com prompt novo → **mesma msg, comportamento muda**, sem tocar código/env | config 100% do painel |
| 2 | `ai_runs` (último) mostra `suggested_queue`; `routing_decisions` (F8) mostra **qual regra** casou e a fila final. Editar a `routing_rule` → mesma sugestão da IA, **fila final diferente** | IA sugere, motor decide |
| 3 | **Parar o container do n8n** → mandar msg → triagem roda e a conversa cai na fila | D-C |
| 4 | Atribuir a conversa a um humano → nova msg do celular → **zero** chamada ao modelo; `ai_runs.status = blocked` | hard-block `human_active` |
| 5 | `select account_id, sum(total_tokens), sum(cost_usd) from messaging.ai_runs group by 1` bate com o painel do provider (ordem de grandeza) | custo por conta |
| 6 | `monthly_ai_runs = 1` no `account_modules.config` → 2ª triagem grava `limit_exceeded`, **não** chama o modelo, e a conversa **ainda assim** cai na fila pelo fallback; `simulate` → 409 | limite |
| 7 | `GET /v1/omnichannel/agents/{id}` com `X-Account-Id` de outra conta → **404** | isolamento |
| 8 | Publicar version 2, rollback para a 1 → comportamento volta; `ai_runs` antigos seguem apontando para a version com que rodaram | imutabilidade |

---

## Notas de Deploy

**Ordem exata:** migration → build api (`--no-cache`) → verificar `migrate status`.

| # | Item | Detalhe |
|---|---|---|
| 1 | Migration `messaging.ai_*` + `collect_field_defs` | **Próximo número livre a partir de 0200** — F2 e F8 consomem números antes. Conferir o disco (há dois `0197`; a última é `0199_calendar_drop_day_media.sql`). Idempotente, sem `-- +goose Down` |
| 2 | Rebuild da API | Mudou `back/` → `docker compose up -d --build api` |
| 3 | **Migration nova → `docker compose build --no-cache api`** | Migrations são `embed.FS`: o cache da camada `go build` pode **não re-embutir** o `.sql` novo. Sintoma: `migrate status` para na anterior, **sem erro** |
| 4 | `OMNI_SECRETS_KEY` | **Já é entrega da F3** (obrigatória, fail-fast no boot). Esta fase só consome — não introduz env nova |

Sem env var nova, sem container novo, sem rota pública nesta fase.

---

## Referência cruzada

- Canônico → [`../PLANO_ATENDIMENTO.md`](../PLANO_ATENDIMENTO.md) — §5.2 permissões · §6 arquitetura · §7.1 tabelas · §8 infra F3 · §10 segurança
- Contratos do port (paginação, shapes) → [`../SPECS_PORT_OMNICHANNEL.md`](../SPECS_PORT_OMNICHANNEL.md)
- Padrão de módulo Go (`Module`/`Option`/`Build`/`handle`) → `back/internal/modules/calendar/module.go`
- Saída mascarada `{set,last4}` → `back/internal/modules/calendar/secrets.go` (**não** cifra em repouso — é a razão do `secretbox` da F3)
