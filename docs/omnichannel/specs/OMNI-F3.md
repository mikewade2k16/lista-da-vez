# OMNI-F3 — Infra transversal (`platform/`)

**Prioridade:** P0 · **Plano canônico:** [`docs/omnichannel/PLANO_ATENDIMENTO.md`](../PLANO_ATENDIMENTO.md) (§8, §9 F3, §12 risco 5)

> ## LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono)
>
> A branch multi-tenant fechou e o dono liberou a implementação. O congelamento que valia
> para esta fase **não existe mais**.

---

## Objetivo

Três peças de plataforma que **não pertencem ao omnichannel** passam a existir em
`back/internal/platform/`: fila durável com ordem garantida por conversa, cifragem de
segredos em repouso, e client LLM nativo com saída validada. Quando a F3 fecha, a F6
tem onde enfileirar, a F4 tem onde guardar credencial de provider, e a F9 tem como
chamar modelo sem depender do n8n.

## Depende de / Bloqueia

| | Fases |
|---|---|
| **Depende de** | **Nenhuma.** Corre em paralelo com F1 e F2 (canônico §9) |
| **Bloqueia** | **F4** (`credentials_ciphertext`), **F6** (envio via outbox), **F9** (triagem IA) |

## Convenções

Ler a skill `principios-engenharia` antes de escrever. Teto ~450 linhas/arquivo — **aqui
vale**, é código novo. `account_id` sempre do Principal, nunca do body; fora de escopo →
404. Mudou `back/` → `docker compose up -d --build api`. Não rodar git, não commitar —
devolver os comandos ao usuário.

---

## Entregas

| # | Pacote / arquivo | Entrega |
|---|---|---|
| F3.1 | `back/internal/platform/secretbox/secretbox.go` + `_test.go` | AES-256-GCM, prefixo `v1:`, `Mask() → {set,last4}` |
| F3.2 | `back/internal/platform/jobs/{jobs,store,retry,worker}.go` + `worker_concurrency_test.go` | Outbox engine, `FOR UPDATE SKIP LOCKED`, FIFO por `ordering_key`, retry classificado, dead-letter, monitor de presas |
| F3.3 | `back/internal/platform/llm/{llm,schema,openai_compat}.go` + `_test.go` | Adapters `openai`/`gemini`/`glm`, structured output validado, `Usage` |
| F3.4 | `back/internal/platform/modules/limits.go` | Leitor de `core.account_modules.config` + defaults + erro 409 |
| F3.5 | `back/internal/platform/app/app.go` + `platform/modules/module.go` | Boot: fail-fast do `OMNI_SECRETS_KEY`; injetar `SecretBox` em `modules.Dependencies` |

**F3 não traz migration.** O `secretbox` não tem tabela; os limites usam
`core.account_modules.config jsonb`, que **já existe** (`0100_core_schema.sql:120` —
confirmado no disco); e a tabela `outbox` está no inventário `messaging.*` do canônico §7.1,
criada pela **F2** (ver *Divergências*). Quem for numerar a primeira migration de
`messaging.*`: **0200 está livre**, mas **confira o disco** — há **dois arquivos `0197`**
(`0197_operation_validation_reason.sql` e `0197_tools_module.sql`); a última é `0199_calendar_drop_day_media.sql`.

---

## F3.1 — `platform/secretbox`

**Por que existe:** `calendar/secrets.go` entrega `{set,last4}` — o contrato de **saída**
está certo e é o modelo a seguir (`mask()`, `secrets.go:44`). Mas `calendar.ai_secrets.api_key`
é `text not null default ''` (`0189_calendar_ai_secrets.sql`) e `PutAccountSecret` grava a
chave **crua** (`store_secrets.go:39-52`). **Mascaramento é de saída, não é cifragem** — é
essa a razão de o pacote nascer em `platform/`.

```go
package secretbox

type Status struct {                       // espelha calendar.KeyStatus (secrets.go:21)
    Set   bool   `json:"set"`
    Last4 string `json:"last4"`
}

type Box struct{ /* aead cipher.AEAD */ }

func New(key []byte) (*Box, error)         // len(key) != 32 => erro. Nunca loga a key.
func FromEnv() (*Box, error)               // OMNI_SECRETS_KEY; ausente/inválida => erro (fail-fast no boot)
func (b *Box) Encrypt(plaintext string) (string, error) // => "v1:" + base64std(nonce||ciphertext)
func (b *Box) Decrypt(encoded string) (string, error)   // prefixo desconhecido => ErrUnknownVersion
func Mask(plaintext string) Status         // mesma regra do calendar: <=4 chars => o valor todo
```

| Regra | Detalhe |
|---|---|
| Cifra | AES-256-GCM (`crypto/aes` + `cipher.NewGCM`). Nonce **12 bytes de `crypto/rand` por Encrypt**, prefixado ao ciphertext |
| Chave | `OMNI_SECRETS_KEY` = **32 bytes crus em base64** (`openssl rand -base64 32`, 44 chars). Nunca default, nunca hardcoded |
| Rotação | Prefixo `v1:` obrigatório. `Decrypt` despacha por prefixo; uma `v2:` futura convive lendo `v1:` |
| Saída | Segredo **nunca** volta ao front. Só `Status`. Nunca em log, nem em erro, nem em trace |
| Fail-fast | Sem `OMNI_SECRETS_KEY` a **api não sobe** (canônico §13.2). Erro claro, não warning |

**Injeção:** adicionar `SecretBox *secretbox.Box` a `modules.Dependencies`
(`platform/modules/module.go:111`). **Precedente confirmado:** `PasswordHasher` já é um
helper de cripto compartilhado ali (`module.go:136`).

---

## F3.2 — `platform/jobs`

Engine **genérico** sobre uma interface `Store`; a tabela concreta é do consumidor
(`messaging.outbox`, F2/§7.1). **Confirmado no disco:** não existe nenhum `SKIP LOCKED` no
repositório hoje — é código 100% novo, sem precedente para copiar.

### Contrato da tabela (o que o engine exige)

Quem criar `messaging.outbox` (F2) precisa satisfazer isto:

| Coluna | Tipo | Papel |
|---|---|---|
| `id` | `uuid primary key default gen_random_uuid()` | padrão da casa (`0181_calendar_schema.sql:12`) |
| `account_id` | `uuid not null references core.accounts(id) on delete cascade` | escopo — obrigatório |
| `ordering_key` | `text not null` | **FIFO**. No omnichannel = `conversation_id` |
| `idempotency_key` | `text not null` | `unique (account_id, idempotency_key)` — **por conta**, decisão do dono (2026-07-17); ver o bloco abaixo |
| `kind` | `text not null` | tipo do job (despacha o handler) |
| `payload` | `jsonb not null default '{}'::jsonb` | **sem PII crua** |
| `status` | `text not null default 'pending'` | `pending\|processing\|done\|failed\|dead` |
| `attempts` / `max_attempts` | `int not null default 0` / `int not null` | `max_attempts` vem da **classificação** |
| `run_after` | `timestamptz not null default now()` | backoff |
| `locked_at` / `locked_by` | `timestamptz` / `text not null default ''` | monitor de presas |
| `last_error` | `text not null default ''` | **mascarado** |
| `created_at` / `updated_at` | `timestamptz not null default now()` | ordem FIFO = `(created_at, id)` |

Índices obrigatórios: `(account_id, ordering_key, created_at, id) where status in ('pending','processing')` e `(status, run_after)`.

> **`idempotency_key` é escopado por conta — decisão do dono (2026-07-17).**
> O unique é **`unique (account_id, idempotency_key)`**, nunca UNIQUE global. A chave vem **do
> cliente**: com UNIQUE global a conta A manda `abc`, a conta B manda `abc`, e a colisão dedupa o
> job de B contra o de A — **suprimindo o envio alheio**. É vazamento cross-tenant e fere o
> princípio 2 (isolamento).
> Esta spec já havia divergido do canônico exatamente aqui; em **2026-07-17 o dono decidiu que a
> divergência vira a norma** e o **canônico §7.1 foi corrigido** — deixa de dizer
> "`idempotency_key UNIQUE`" global. Por isso o item saiu de *Divergências*: não é mais divergência,
> é o contrato.
> **Consequência:** onde alguma spec mandava **prefixar a chave com o `account_id`** como mitigação
> do UNIQUE global, o prefixo virou **desnecessário** — o unique composto é o mecanismo, e os dois
> juntos só escondem qual está valendo.

### O claim — o coração da fase (§12 risco 5)

`SKIP LOCKED` puro dá throughput e **inverte a ordem**: dois jobs da mesma conversa em
workers diferentes = o cliente vê a resposta antes da pergunta. A garantia vem do
predicado **head-of-line**: só é elegível o job mais antigo *não finalizado* da chave, e
só se estiver vencido.

```sql
with candidates as (
    select j.id
    from messaging.outbox j
    where j.status = 'pending'
      and j.run_after <= now()
      and not exists (
          select 1
          from messaging.outbox b
          where b.account_id = j.account_id
            and b.ordering_key = j.ordering_key
            and b.status in ('pending', 'processing')
            and (b.created_at, b.id) < (j.created_at, j.id)
      )
    order by j.created_at, j.id
    limit $1
    for update skip locked
)
update messaging.outbox o
set status = 'processing', attempts = o.attempts + 1,
    locked_at = now(), locked_by = $2, updated_at = now()
from candidates c
where o.id = c.id
returning o.id, o.account_id, o.ordering_key, o.kind, o.payload, o.attempts, o.max_attempts;
```

Por que o predicado é `in ('pending','processing')` e **não** só `'processing'`: um job em
**backoff** volta para `pending` com `run_after` no futuro. Checando só `processing`, o
sucessor passa na frente do antecessor que está esperando retry — **inversão silenciosa,
com o provider saudável**. Head-of-line blocking por chave é o comportamento **correto**
aqui. Não usar `DISTINCT ON` para isso: o Postgres recusa `FOR UPDATE` com `DISTINCT`, e o
predicado acima já garante no máximo um candidato por chave.

### Retry classificado (canônico §8; herdado do legado — `SPECS_PORT_OMNICHANNEL.md` F5)

| Classe | `max_attempts` |
|---|---|
| Transitório (rede/timeout/conexão) | 5 |
| `401` · `403` · `404` · `405` e `400`/`422` conhecidos | **1 (unrecoverable)** |
| `429` | 5 |
| `5xx` | 4 |
| Sem status (resposta não-HTTP) | 4 |
| Outros | 3 |

Backoff exponencial com jitter: `run_after = now() + min(5s * 2^(attempts-1), 5min)` ±20%.
Esgotou as tentativas ou classe unrecoverable → `dead` + `last_error` mascarado (dead-letter).

### Worker

- Goroutine por instância, `time.NewTicker` (padrão da casa: `realtime/service.go`, `httpapi/rate_limit.go`), parada por `context.CancelFunc` no `Handle.Close()` (`platform/modules/module.go:70`). **Sem BullMQ, sem Redis** — o estado é do banco (princípio 1).
- **Monitor de presas:** `processing` com `locked_at` > 10 min volta para `pending`, até **20 por ciclo, a cada 5 min**, **com filtro de conta**. O legado varre a tabela inteira sem tenant — **não portar esse comportamento** (canônico §8).

---

## F3.3 — `platform/llm`

**Reaproveitar o que está confirmado:** `calendar/ai_models.go:40-44` já mapeia os três
provedores e registra que **todos falam a camada OpenAI-compatible** (`{data:[{id}]}`) —
logo **um adapter OpenAI-compatible cobre os três**, parametrizado por `baseURL`. Manter o
`User-Agent` próprio (`ai_models.go:33`): o default do Go é barrado por WAF (registro de falhas nº7).

```go
package llm

type Schema struct {                        // versionado: schema muda => Version sobe
    Name       string
    Version    int
    Definition json.RawMessage              // JSON Schema
}
type Usage struct{ PromptTokens, CompletionTokens, TotalTokens int }
type Request struct {
    Provider, Model, BaseURL string
    SystemPrompt, UserPrompt string
    Temperature              float64
    Schema                   *Schema        // nil = texto livre
    APIKey                   string         // crua; NUNCA logada
}
type Response struct {
    Text      string
    JSON      json.RawMessage               // preenchido só com Schema != nil
    Usage     Usage
    Model     string
    LatencyMs int
}
type Client interface{ Complete(ctx context.Context, req Request) (Response, error) }
```

| Regra | Detalhe |
|---|---|
| JSON mode | `response_format` do provedor **+ validação do JSON contra o `Schema` no Go**. Strict mode do provedor **não é prova** — validar sempre |
| Violação | `ErrSchemaViolation` (o caller decide: repetir, cair para regra default, registrar). Nunca entregar JSON não validado ao domínio |
| `Usage` | Devolvido por `Complete`. **Persistir em `ai_runs` é da F9** — F3 não cria essa tabela |
| Chave | Resolvida pelo caller (padrão `resolveDispatchKey`, `calendar/ai_dispatch.go:105`). Vazia → `ErrKeyMissing` (409 acionável, não 500) |
| Erros | `ErrKeyMissing` · `ErrSchemaViolation` · `ErrProviderUnavailable` (502). Timeout próprio, endpoint travado não segura handler |
| Log | Provider, modelo, tokens, latência, `account_id`. **Nunca** prompt, resposta ou chave |

> **Risco novo que a D-C cria — SSRF.** No calendário quem chamava o provedor era o n8n;
> com o LLM no Go, quem faz o request de saída é o **container da api**. `BaseURL` vem do
> painel: uma base apontando para host interno vira SSRF. **Mitigação:** `BaseURL` só é
> aceita se bater com o mapa canônico server-side (espelho de `ai_models.go:40-44`) ou com
> uma allowlist em `core.platform_settings`; fora disso → erro. É exatamente o cuidado que
> `ai_models.go:35-39` já documenta para a listagem — agora vale para o dispatch também.

---

## F3.4 — Limites por conta

Sem migration (`core.account_modules.config` já existe). Leitor em
`platform/modules/limits.go` — o pacote que **já é dono** de `core.account_modules`
(`catalog_postgres.go`).

```json
{ "max_whatsapp_numbers": 2, "monthly_ai_runs": 5000 }
```

| Regra | Detalhe |
|---|---|
| Resolução | `core.account_modules.config` (da conta) → default em `core.platform_settings` (padrão do `calendar_ai_secrets`, `0160_core_platform_settings.sql:21`) → **ausente nos dois = sem limite** |
| Estouro | `ErrLimitExceeded` → **409** com código `limit_exceeded` e `{limit, current, key}`. Mensagem acionável, nunca falha silenciosa (princípio 5) |
| Aplicação | F3 entrega **leitor + erro**. Quem aplica: F4 (`max_whatsapp_numbers`) e F9 (`monthly_ai_runs`) |

> **Gap verificado — não existe writer.** `AdminSetModulesInput` é
> `{ Enable, Disable []string }` (`core/admin_model.go:100-103`); todo insert grava
> `'{}'::jsonb` (`core/admin_repository.go:218`, `bio/store_postgres.go:150`); e **nenhum
> Go lê `config` hoje**. Gravar um limite **hoje só por SQL**. F3 **não inventa rota**: o
> writer de painel é da **F10**. Enquanto isso, a tela que exibir limite mostra "Sem limite
> cadastrado" — honesto, sem default que minta (princípio 5).

---

## Armadilhas / o que NÃO fazer

| Não faça | Porque |
|---|---|
| `SKIP LOCKED` sem o predicado head-of-line | É o **risco 5** do canônico. Ordem inverte com provider saudável e o bug só aparece sob carga |
| Checar só `status = 'processing'` no `not exists` | Job em backoff está `pending`: o sucessor ultrapassa |
| `DISTINCT ON` no claim | Postgres recusa `FOR UPDATE` com `DISTINCT` |
| Indexar/`WHERE` sobre coluna cifrada | GCM usa nonce aleatório: mesmo texto → ciphertext diferente. Ciphertext **não** é chave de busca |
| `OMNI_SECRETS_KEY` com default ou fallback | Perder a chave = perder os segredos. Default silencioso vira produção cifrada com chave de dev |
| Gravar ciphertext sem o prefixo `v1:` | Sem prefixo não há rotação — só migração manual |
| Confiar no strict mode do provedor | Validar o JSON contra o `Schema` no Go, sempre |
| Aceitar `BaseURL` livre do painel | SSRF a partir da api (ver F3.3) |
| Monitor de presas varrendo a tabela toda | Comportamento do legado, sem tenant (canônico §8) |
| Criar `ai_runs` / `messaging.outbox` aqui | `ai_runs` é F9; `outbox` é F2 (§7.1) |
| Migrar os segredos do calendário nesta fase | Pendência registrada (§14.5), **não bloqueante**. Alvo: depois da F3 |

## Segurança

| Item | Regra |
|---|---|
| Escopo | `account_id` **sempre** do Principal, nunca do body. Todo `select`/`update` de `jobs` filtra por `account_id` **também no repositório** — defesa em profundidade (princípio 2) |
| Fora de escopo | **404, nunca 403** — 403 confirma que o recurso existe (enumeration) |
| `idempotency_key` | **`unique (account_id, idempotency_key)`** — escopado por conta (decisão do dono, 2026-07-17; canônico §7.1 corrigido). Chave global deixa a conta A colidir com a da conta B e derrubar job alheio. Não prefixar a chave com o `account_id`: o unique composto já é o mecanismo |
| Segredos | Só via `secretbox`. Nunca em coluna crua, nunca em log, nunca de volta ao front (só `{set,last4}`) |
| `last_error` / `payload` | Mascarados. Payload bruto **nunca** em log, nem em erro, nem em trace (canônico §10) |
| Logs | Campos explícitos (`op`, `account_id`, `job_id`, `error`) — nunca a struct interpolada. Modelo: `calendar/ai_dispatch.go:248` |

## Verificável

Um humano prova a fase assim:

1. **FIFO sob concorrência (o teste dedicado do risco 5).**
   `worker_concurrency_test.go`, padrão de integração da casa: `TEST_DATABASE_URL` + `t.Skip` se ausente (`platform/database/app_role_ensure_test.go:31-35`). Tabela **efêmera criada pelo próprio teste** com o contrato de F3.2 (F3 não depende da F2). Cenário: **8 workers**, 1 conta, **20 `ordering_key` × 25 jobs**, handler com sleep aleatório 0–5 ms e **falha transitória injetada em ~10%** para exercitar o retry.
   **Assertivas:** (a) por `ordering_key`, a ordem de conclusão é **exatamente** a de inserção; (b) nenhum job roda duas vezes com sucesso; (c) todos terminam em `done` ou `dead`; (d) **sub-teste dedicado ao backoff** — job que falha e reagenda **não** é ultrapassado pelo sucessor.
   ```bash
   TEST_DATABASE_URL="postgres://omni:omni_dev@localhost:5432/omni?sslmode=disable" \
     go test ./internal/platform/jobs/ -run TestWorkerFIFO -race -count=5 -v
   ```
   `-race` e `-count=5` são parte do critério: passar uma vez não prova ausência de corrida.
2. **Segredo cifrado e lido.** `go test ./internal/platform/secretbox/ -v`: round-trip; saída **começa com `v1:`**; duas cifragens do mesmo texto dão **ciphertexts diferentes** (nonce); chave errada **falha** (não devolve lixo); `Mask("sk-abc1234")` → `{set:true,last4:"1234"}`.
3. **Fail-fast no boot.** Subir a api **sem** `OMNI_SECRETS_KEY` → **container não fica de pé** e `docker compose logs api` traz o erro nomeando a env. Com a chave → sobe normal.
4. **LLM devolve JSON validado.** Teste com servidor HTTP fake: resposta conforme o schema → `Response.JSON` preenchido + `Usage` com tokens; resposta **fora** do schema → `ErrSchemaViolation` (e **não** um JSON qualquer). `BaseURL` para host interno → recusado.
5. **Limites.** `update core.account_modules set config = '{"max_whatsapp_numbers":2}'::jsonb where account_id = '<uuid>' and module_id = 'omnichannel';` → o leitor devolve 2; sem a chave → cai no default de `core.platform_settings`; sem os dois → sem limite.

## Notas de Deploy

**Ordem exata:** env var → build da api.

| # | Item | Detalhe |
|---|---|---|
| 1 | **`OMNI_SECRETS_KEY`** | **OBRIGATÓRIA — sem ela a api NÃO SOBE** (fail-fast, canônico §13.2). 32 bytes em base64: `openssl rand -base64 32`. Adicionar em `.env` (dev) **e** no ambiente da VPS **antes** do deploy. **Perder a chave = perder todo segredo cifrado** — guardar fora do repositório. Nunca commitar |
| 2 | Rebuild da api | `docker compose up -d --build api` — mexeu em `back/` |

**Sem migration nesta fase** → o `build --no-cache` do `embed.FS` **não se aplica** aqui.
Ele volta a valer na F2/F4, e vale o alerta do canônico §13: as migrations são `embed.FS` e
o cache da camada `go build` pode **não re-embutir** o `.sql` novo — sintoma é `migrate
status` parar na migration anterior, **sem erro**.

---

## Divergências com o canônico (registradas, não decididas por conta própria)

| # | Ponto | O canônico diz | Esta spec faz | Por quê |
|---|---|---|---|---|
| 1 | Dono da tabela `outbox` | §7.1 lista `outbox` entre as tabelas `messaging.*` (criadas pela migration da **F2**); §8 põe "outbox + worker" na **F3**; §9 dá **"Blockers: nenhum"** à F3 | Engine genérico sobre `Store`; a tabela real é da F2; o **teste de concorrência cria tabela efêmera própria** | É a única leitura que honra as três afirmações ao mesmo tempo e mantém a F3 sem blocker. **Decidido: a tabela é da F2, o worker/engine é da F3** — não reabrir. **Quem fechar a F2 tem de conferir que `messaging.outbox` satisfaz o contrato de F3.2** |

**Resolvida e removida desta seção:** o **escopo do `idempotency_key`** era a divergência nº 2
(esta spec fazia `unique (account_id, idempotency_key)` contra o "`idempotency_key UNIQUE`" global do
canônico §7.1). Em **2026-07-17 o dono decidiu pela divergência**: o unique **por conta** virou a
norma e o canônico §7.1 foi corrigido. Deixou de ser divergência — o contrato está em **F3.2**.
