# OMNI-F6 — Envio via outbox + mídia

**Prioridade:** P0
**Plano canônico:** `docs/omnichannel/PLANO_ATENDIMENTO.md` (§9 tabela de fases · §9.2 F6 · §10 · §13)
**Anexo técnico (contratos verbatim):** `SPECS_PORT_OMNICHANNEL.md` F5 · `PLANO_PORT_OMNICHANNEL.md` §6 D2, §8, §10

> ## LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono)
>
> A branch multi-tenant fechou e o dono liberou a implementação. O congelamento que valia
> para esta fase **não existe mais**.

**Convenções (valem para toda spec deste módulo):** ler a skill `principios-engenharia` antes.
`account_id` **sempre** do Principal, nunca do body; repositório filtra por conta também.
Fora de escopo → **404**, nunca 403. Mudou `back/` → `docker compose up -d --build api`.
Migration nova → `build --no-cache api`. Não rodar git, não commitar — devolver os comandos.

---

## Objetivo

O atendente responde pelo painel e a mensagem chega no celular. O envio é **durável**
(sobrevive a restart e ao provider fora do ar), **ordenado por conversa** e **idempotente**.
Mídia vive **em disco** e é servida por endpoint autenticado com `Range` — **nunca base64 no
Postgres** (divergência deliberada do legado, D2).

## Depende de / Bloqueia

| | Fases |
|---|---|
| **Depende de** | **F3** (`platform/jobs`: worker + claim + retry + FIFO por `ordering_key`) · **F4** (`ChannelProvider.SendMessage`/`DownloadMedia` + adapter `mock`) · **F2** (schema + leitura + a **tabela `messaging.outbox`**) · F5 (realtime, para os eventos) |
| **Bloqueia** | **F7** (ações do inbox) · F12 (stickers reusam a allowlist e o storage) |

**Regra de fronteira:** a **tabela `messaging.outbox` é da F2** (canônico §7.1 — está no
inventário `messaging.*`, e a migration do §7 é da F2 por §9.2; ver `OMNI-F2.md` C1) e o
**worker/engine é da F3** (canônico §3 e §8 — infra transversal em
`back/internal/platform/jobs`). **F6 não cria nem reimplementa nenhum dos dois**: F6 é
**produtor** de job e dono do **handler de envio**. Se ao executar a F6 faltar a tabela (F2) ou
o `platform/jobs` (F3), **parar e escalar** — não fazer uma fila paralela dentro do módulo.

---

## Entregas

| # | Item | Alvo |
|---|---|---|
| 1 | `POST /v1/omnichannel/conversations/{id}/messages` — contrato verbatim do legado | `back/internal/modules/omnichannel/http_messages.go` |
| 2 | Serviço de envio: valida escopo → grava mídia em disco → cria `message` PENDING/OUTBOUND → atualiza conversa → **enfileira em `platform/jobs`** → publica `message.created` → audita | `.../omnichannel/service_outbound.go` |
| 3 | Handler do job (consumido pelo worker da F3): resolve instância → `ChannelProvider.SendMessage` → `SENT`/`FAILED` → publica `message.updated` → audita | `.../omnichannel/outbound_handler.go` |
| 4 | Storage de mídia em disco, **raiz privada** (fora de `UPLOADS_DIR`) | `.../omnichannel/media_storage.go` |
| 5 | `GET /v1/omnichannel/conversations/{cid}/messages/{mid}/media` — stream com `Range`, sem carregar em RAM | `.../omnichannel/http_media.go` |
| 6 | Guarda anti-SSRF reutilizável (F12 reusa) | `.../omnichannel/ssrf.go` |
| 7 | Throttle de saída por número (reagenda job, não bloqueia worker) | `.../omnichannel/outbound_throttle.go` |
| 8 | Env `OMNICHANNEL_MEDIA_DIR` + wiring | `back/internal/platform/config/config.go` · `.../app/app.go` |
| 9 | Migration aditiva **só se** a F2 não tiver nascido com as colunas de mídia (C3) | `back/internal/platform/database/migrations/` |
| 10 | `AGENT.md` do módulo: rotas, colunas de mídia, env, fronteira com `platform/jobs` | `.../omnichannel/AGENT.md` |

---

## Contratos

### C1 · POST de mensagens — **não duplicar**

Body, validações, códigos e fluxo: **`SPECS_PORT_OMNICHANNEL.md` F5 §1** (verbatim do legado —
`type` TEXT|IMAGE|AUDIO|VIDEO|DOCUMENT, `content` max 4000, `mediaUrl` 1..60MB, `mediaMimeType`,
`mediaFileName`, `mediaFileSizeBytes`, `mediaCaption`, `mediaDurationSeconds`, `metadataJson`;
TEXT exige `content`, resto exige `mediaUrl`; sucesso **200**; falha ao enfileirar → `FAILED` +
**202**; VIEWER → **403**). O que **F6 acrescenta**:

| Campo novo | Regra |
|---|---|
| `idempotencyKey` (opcional, body) | Se ausente, o servidor deriva um valor estável. Gravado **como veio**, na coluna `idempotency_key` do **`messaging.outbox`** (é lá que a chave vive — F3.2 / canônico §7.1; **não** existe essa coluna em `messaging.messages`). Quem isola as contas é o **`unique (account_id, idempotency_key)`** — **por conta, decisão do dono (2026-07-17)**; contrato em `OMNI-F3.md` F3.2, e o canônico §7.1 foi corrigido (não diz mais "`idempotency_key UNIQUE`" global, que vazava cross-tenant). **Não prefixar a chave com `account_id`** — com o unique composto o prefixo é redundante. Repetir a mesma chave **na mesma conta** → **200 com a mensagem já criada** (o `id` sai do `payload` do job já gravado), zero enfileiramento novo |
| `ordering_key` (não é do body) | **Sempre** `conversation_id`. Garante FIFO por conversa (canônico §12, risco 5) |

Eventos emitidos: `message.created` (envio HTTP → Message completo + `correlationId`) e, no
job, `message.updated` (worker → shape **mínimo**). Shapes exatos por call-site:
**`SPECS_PORT_OMNICHANNEL.md` F4 §2** — replicar sem unificar.

Retry/backoff classificado e monitor de presas: **`SPECS_PORT_OMNICHANNEL.md` F5 §2** e canônico
§8. **A política é da F3**; F6 só classifica o erro do provider e devolve ao job.

### C2 · Colunas de mídia (D2 — contrato portado, storage trocado)

O legado grava **data URL base64 na coluna**. O front **só consome o endpoint** `/media`
(port §6 D2) — então o contrato entra igual e o storage muda.

| Coluna (`messaging.messages`) | Tipo | Papel |
|---|---|---|
| `media_storage_key` | `text` | Path **relativo** à raiz privada. **Nunca serializado no JSON** |
| `media_mime_type` · `media_file_name` | `text` | Do allowlist / sanitizado |
| `media_size_bytes` | `bigint` | Tamanho **decodificado** |
| `media_caption` | `text` | |
| `media_duration_seconds` | `integer` | |
| `media_source_kind` | `text` | `disk` \| `url_encrypted` (o `/media` da F5 §3 depende dele) |

**Serialização:** `mediaUrl` na resposta = `"/v1/omnichannel/conversations/{cid}/messages/{mid}/media"`
quando há `media_storage_key`; senão `null`. **Nunca** data URL, nunca o path de disco.

**Migration:** por D2 (port §6, "decidir agora porque é schema") essas colunas **nascem na
migration da F2**. Se ao executar a F6 elas não existirem, adicionar **aditivamente**:
SQL plano idempotente (`alter table ... add column if not exists`), schema-qualificado,
**sem `-- +goose Down`** (o migrator roda o arquivo inteiro e o Down se auto-destrói).
**Numerar conferindo o disco** — hoje a última é `0199_calendar_drop_day_media.sql` e existem
**dois `0197`**; F2/F3 já terão consumido 0200+.

### C3 · Storage em disco — raiz **privada**

```
{OMNICHANNEL_MEDIA_DIR}/{accountId}/{conversationId}/{random}.{ext}
```

- Env nova: `OMNICHANNEL_MEDIA_DIR`, default `data/media/omnichannel`.
  Padrão do `getEnv` em `config.go:95` (`UPLOADS_DIR` → `data/uploads`).
- **A raiz NÃO pode ficar sob `cfg.UploadsDir`** — ver Armadilha 1. Não é preferência: é o
  isolamento da conversa.
- Segmentos sanitizados + nome aleatório, `MkdirAll 0o750`, arquivo `0o600` — espelhar
  `back/internal/modules/calendar/media_storage.go` (sanitizeSegment, randomSuffix, sniff de
  mime + allowlist, extensão casando com o mime declarado). **Copiar a mecânica, não o
  destino** (o calendário publica em `/uploads/...` de propósito; aqui é o oposto).
- **Decodificação:** `mediaUrl` chega como data URL. Teto do corpo via `MaxBytesReader`
  = limite decodificado da conta (`messaging.account_config.max_upload_mb`, port §7) **× 4/3**
  (overhead do base64) + folga. Gravar com `io.Copy(file, base64.NewDecoder(...))` — **não**
  materializar o arquivo inteiro em `[]byte` mais de uma vez. Estouro → **413**; mime fora do
  allowlist → **415**; ambos com `{message, code, details}` (F5 §1).
- `mediaUrl` como URL `http(s)` (não data URL): passa pelo guarda anti-SSRF (C5) antes de
  qualquer fetch.

### C4 · GET media — stream com `Range`

Query `disposition` (`inline`|`attachment`), `download`, exclusão de `hidden_messages` do
usuário → 404, rehidratação (`requiresMediaDecrypt` / `media_source_kind = url_encrypted`)
com `message.updated` one-shot, e `Cache-Control: private, max-age=60`:
**`SPECS_PORT_OMNICHANNEL.md` F5 §3** — não duplicado aqui. O que **F6 fixa**:

| Regra | Detalhe |
|---|---|
| Stream | `os.Open` + **`http.ServeContent`** (stdlib): resolve `Range`, `If-Range`, `206`, `Content-Range` e `Accept-Ranges` de graça. **Não há uso de `ServeContent` no repo hoje — F6 é o primeiro.** |
| Content-Type | Setar **antes** do `ServeContent` a partir de `media_mime_type` (não deixar sniffar) |
| Nunca | `io.ReadAll` do arquivo. O legado faz `Buffer.from(await res.arrayBuffer())` inteiro — é exatamente o que a D2 elimina |
| Cache | `private`. **Jamais** `public` — é conteúdo de conversa |

### C5 · Anti-SSRF

Base: F5 §3 (host interno → **403**; protocolo ≠ `http`/`https` → **422**). Como fazer certo:

- Validar o **IP resolvido**, não o hostname (DNS rebinding). Bloquear `IsLoopback`,
  `IsPrivate`, `IsLinkLocal*`, `IsUnspecified`, CGNAT `100.64.0.0/10` e o metadata
  `169.254.169.254`.
- Checar no **`Control` do dialer** (`net.Dialer.Control`) — assim o IP verificado é o mesmo em
  que se conecta (TOCTOU) — e **não seguir redirect** para destino não validado.
- Helper único, reusado por F12 (`SPECS_PORT_OMNICHANNEL.md` F7 §3: "mesma allowlist do
  `/media`"). Não existe helper no `back/` hoje — nasce aqui.

### C6 · Rate limit por número

**Mitigação de ban do não-oficial — mitigação, não garantia** (canônico §10 e §12, risco 3).

- Aplicado no **handler do job**, antes do `SendMessage`, com bucket por
  `whatsapp_instances.id` (o número), não por usuário.
- Cota estourada → **reagendar o job** (`next_attempt_at`), **preservando o `ordering_key`**.
  **Nunca** bloquear o worker com sleep: trava as outras conversas e mata o FIFO.
- Valor em `core.account_modules.config jsonb` (a coluna já existe — `0100_core_schema.sql:120`),
  no mesmo nível dos limites do canônico §5.3: `{"outbound_messages_per_minute": 20}`.
  Default em `core.platform_settings` (`0160_core_platform_settings.sql`).
- **Não** usar `httpapi.RateLimit`: os buckets dele são in-memory e o próprio arquivo declara
  que não servem a multi-instância (`rate_limit.go:26-27`). O reagendamento no banco é durável.

---

## Armadilhas / o que NÃO fazer

1. **`/uploads/` é `http.FileServer` SEM auth e SEM gate de módulo.** Confirmado:
   `app.go:241-243` monta `GET /uploads/` sobre `http.Dir(cfg.UploadsDir)`, e `moduleGatingRules`
   (`app.go:513-517`) declara `uploads` como **não gateado**. Gravar mídia de conversa sob
   `UPLOADS_DIR` = **qualquer um com a URL baixa a mídia de qualquer conta, sem token**. Por isso
   a raiz é separada (C3). O calendário/tasks publicam em `/uploads/` **de propósito**; aqui não.
2. **Não reimplementar a fila.** `SKIP LOCKED`, retry, dead-letter e FIFO são da F3. Fila paralela
   dentro do módulo = duas verdades (princípio 1).
3. **Não portar o monitor de presas sem filtro de conta** — o legado varre a tabela inteira
   (F5 §2). Sem filtro é vazamento cross-tenant.
4. **Não trafegar base64 no WS** (port §8.4). Com disco não há data URL para sanitizar, mas o
   guarda continua: `mediaUrl` no realtime é a URL do endpoint ou `null`.
5. **`idempotency_key` com `UNIQUE` global.** O unique é **`unique (account_id, idempotency_key)`**,
   por conta — **decisão do dono (2026-07-17)**, e o canônico §7.1 já foi corrigido (não pede mais o
   `UNIQUE` global). Chave global vinda do cliente deixa A colidir com B e suprimir envio alheio.
   **F6 segue o contrato de F3.2.** E não prefixar a chave com `account_id` "por garantia": o unique
   composto já isola — os dois mecanismos juntos só escondem qual está valendo.
6. **VIEWER → 403, conversa de outra conta → 404.** Permissão e escopo são coisas diferentes; o
   404 é o que impede enumeration.
7. **Migration nova exige `docker compose build --no-cache api`** — migrations são `embed.FS` e o
   cache do `go build` pode não re-embutir o `.sql`. Sintoma: `migrate status` para na anterior,
   sem erro.
8. **Payload bruto do provider nunca em log** — nem em erro, nem em trace (canônico §10).
9. **Teto ~450 linhas/arquivo vale aqui** — é código Go novo. A dispensa do verbatim é só do front.

---

## Segurança

| Item | Regra |
|---|---|
| Escopo | `account_id` **do Principal**, nunca do body. Conversa/mensagem de outra conta → **404** |
| Defesa em profundidade | O `WHERE account_id` vai **também** na query do repositório, mesmo com o service já validando |
| Mídia | Raiz privada fora de `UPLOADS_DIR`; servida **só** pelo endpoint autenticado; `Cache-Control: private`; path de disco nunca no JSON |
| SSRF | C5 — IP resolvido, não hostname; sem redirect cego |
| Idempotência | Escopada por conta pelo `unique (account_id, idempotency_key)` do `messaging.outbox` — **por conta, decisão do dono (2026-07-17)**; contrato em `OMNI-F3.md` F3.2. A chave do cliente **não** é prefixada — o unique composto é o mecanismo |
| Segredos do provider | Só via `platform/secretbox` (F3). Nunca coluna crua, nunca em log, nunca de volta pro front (`{set,last4}`) |
| Log | Sem payload bruto, sem mídia, sem credencial |

---

## Verificável

Um humano prova no browser/banco, sem ler código:

1. **Chega no celular.** Responder uma conversa pelo painel → a mensagem chega no aparelho; a
   linha em `messaging.messages` vai de `PENDING` → `SENT` e o balão atualiza **ao vivo** (F5).
2. **Idempotência.** `POST` duas vezes com o mesmo `idempotencyKey` →
   `select count(*) from messaging.outbox where account_id = '<uuid>' and idempotency_key = '...'`
   devolve **1** (a chave vive no **outbox**, não em `messaging.messages`); a 2ª resposta é 200
   com o mesmo `id` de mensagem e **nenhuma linha nova** em `messaging.messages`.
3. **FIFO.** Disparar 5 mensagens seguidas na mesma conversa → chegam no celular **na ordem
   enviada** (o `ordering_key` é a conversa).
4. **Durabilidade.** Derrubar o provider (`docker compose stop evolution` ou adapter `mock` em
   modo falha) → após os retries a mensagem vira **`FAILED`** com `audit_event`
   `MESSAGE_OUTBOUND_FAILED`; subir o provider e reenviar funciona.
5. **Range de verdade.** `curl -i -H "Range: bytes=0-99" -H "Authorization: Bearer <token>" \
   ".../messages/{mid}/media"` → **206** + `Content-Range: bytes 0-99/<total>` +
   `Accept-Ranges: bytes`. Áudio longo com seek na UI **não rebaixa** o arquivo inteiro.
6. **Mídia não vaza.** Achar o `media_storage_key` no banco e tentar
   `GET /uploads/<qualquer tentativa de path>` **sem token** → **404**. Nenhum caminho serve a
   mídia da conversa fora do endpoint autenticado.
7. **Isolamento.** `X-Account-Id` de outra conta no `/media` e no POST → **404** (não 403).
8. **Sem base64 no banco.** `select media_storage_key, length(coalesce(media_url,'')) ...` — não
   há data URL persistida; a coluna guarda path.

---

## Notas de Deploy

**Ordem exata:** migration (se houver) → env var → volume → **build da api** → verificar.

| # | Item | Detalhe |
|---|---|---|
| 1 | Migration aditiva de mídia (**só se** a F2 não nasceu com as colunas) | Numerar **conferindo o disco** (última: `0199`; **dois `0197`**; F2/F3 consomem 0200+). Idempotente, schema-qualificada, **sem `-- +goose Down`** |
| 2 | **`OMNICHANNEL_MEDIA_DIR`** | Default `data/media/omnichannel`. **Não pode ficar sob `UPLOADS_DIR`** (`data/uploads`) — senão o FileServer público expõe a mídia |
| 3 | **Volume de mídia** | Canônico §13 item 7. Novo volume no compose (dev e prod) + **incluir no backup**. Mídia não está no Postgres: backup do banco **não** a cobre |
| 4 | Build da API | Mexeu em `back/` → `docker compose up -d --build api`. **Com migration nova → `docker compose build --no-cache api`** e depois `up -d api` |
| 5 | Verificação pós-deploy | `migrate status` na última esperada + item 5 e 6 do "Verificável" |

Sem `n8n:import` nesta fase (F6 não toca `automation/export/*.json`).
