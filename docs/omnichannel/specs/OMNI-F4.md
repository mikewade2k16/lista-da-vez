# OMNI-F4 — ChannelProvider + adapters mock/evolution + webhook inbound

**Prioridade:** P0
**Plano canônico:** `docs/omnichannel/PLANO_ATENDIMENTO.md` (§5.4, §7.1, §9.2-F4, §10, §11, §13)

> ## LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono)
> A branch `refactor/multi-tenant-complete` fechou e o dono **liberou a implementação em
> 2026-07-17** (decisão **D-D**, canônico §2). O aviso de congelamento que constava aqui não vale mais.

**Convenções** (preâmbulo do `SPECS_PORT_OMNICHANNEL.md`): `account_id` **sempre** do Principal/slug
resolvido no server, nunca do body · repositório filtra por conta também · fora de escopo → **404**,
nunca 403 · mudou `back/` → `up -d --build api`; migration nova → `build --no-cache api` · não rodar git.

---

## Objetivo

A **camada tradutora** existe: interface `ChannelProvider` + eventos canônicos que o domínio e o
front nunca furam. Dois adapters vivos (`mock` e `evolution`) e um **webhook inbound público** que
recebe mensagem real do celular, deduplica e persiste em `messaging.messages`. Fechada esta fase,
trocar de provedor = 1 adapter novo — zero mudança no domínio, zero no front.

**Fora de escopo:** integração ou leitura de qualquer outro módulo. Esta fase só lê e escreve
`messaging.*`; se, com o módulo fechado, for preciso integrar, isso vira plano próprio.

---

## Depende de / Bloqueia

| | Fases |
|---|---|
| **Depende de** | **F2** (schema `messaging.*`; `whatsapp_instances` já nasce com `provider`/`provider_config`/`credentials_ciphertext` — canônico §7.2) · **F3** (`platform/secretbox` para ler credenciais cifradas) |
| **Bloqueia** | **F5** (realtime) · **F6** (envio via outbox consome `SendMessage`/`DownloadMedia`) · **F10** (telas de config de números/providers) · **F11** (adapter Meta entra pela mesma interface) |
| **Paralelo** | **F8** (domínio de atendimento) corre ∥ — não depende do canal (canônico §9) |

---

## Entregas

| # | Entrega | Alvo |
|---|---|---|
| 1 | Interface `Provider` + eventos canônicos + `Capabilities` | `back/internal/modules/omnichannel/channel/provider.go` |
| 2 | Registry de providers (resolve por `whatsapp_instances.provider`) | `.../omnichannel/channel/registry.go` |
| 3 | Adapter **mock** (implementa os 5 métodos; determinístico) | `.../omnichannel/channel/mock/mock.go` |
| 4 | Adapter **evolution**: client HTTP + Verify/Parse + sessão | `.../omnichannel/channel/evolution/{client,adapter,parse}.go` |
| 5 | Webhook inbound público + proteções + dedupe | `.../omnichannel/http_webhook.go` · `.../omnichannel/service_inbound.go` |
| 6 | Store de `webhook_events` (insert-first, `on conflict do nothing`) | `.../omnichannel/store_webhook_events.go` |
| 7 | Rotas de sessão (bootstrap/connect/qrcode/status/logout) | `.../omnichannel/service_session.go` · `.../omnichannel/http_session.go` |
| 8 | Guard de número duplicado **dentro da conta**, no cadastro/conexão | `.../omnichannel/number_guard.go` |
| 9 | Migration `messaging.webhook_events` + índice único de número (C6) | `back/internal/platform/database/migrations/020X_messaging_webhook_events.sql` |

Pacote aninhado dentro do módulo é padrão da casa — precedente confirmado: `back/internal/modules/crm/erp/`.
Teto de ~450 linhas/arquivo **vale aqui** (é código novo, não port verbatim).

---

## Contratos

### C1 — `channel.Provider` (a interface do canônico §5.4)

```go
package channel // back/internal/modules/omnichannel/channel

// Provider é a camada tradutora: domínio e front SÓ veem o shape canônico (§5.4).
type Provider interface {
    ID() string // "mock" | "evolution" | "waha" | "meta_whatsapp_cloud"
    // Evolution/WAHA: token constant-time. Meta (F11): HMAC X-Hub-Signature-256.
    // NUNCA embutir body no erro (payload bruto não vaza — canônico §10).
    VerifyWebhook(hdr http.Header, body []byte, cred Credentials) error
    // Payload do provider é DINÂMICO: parsear defensivamente, nunca presumir campo.
    ParseWebhook(ctx context.Context, hdr http.Header, body []byte) ([]Event, error)
    SendMessage(ctx context.Context, cred Credentials, out OutboundMessage) (SendResult, error)       // F6
    DownloadMedia(ctx context.Context, cred Credentials, ref MediaRef) (io.ReadCloser, MediaMeta, error) // F6
    Capabilities() Capabilities
}

// Event é o evento canônico. ExternalEventID é a CHAVE DE DEDUPE (ver C3/C4).
type Event struct {
    Kind            EventKind // message_received | message_status | session_status | qr_updated | ignored
    ExternalEventID string
    InstanceName    string    // = instance_scope_key (port §7: é o NOME, não o id)
    OccurredAt      time.Time // timestamp DO PROVIDER, não now()
    Message         *InboundMessage
    Status          *StatusUpdate
    Session         *SessionUpdate
}

// Capabilities sustenta o multi-provider na UI: a tela degrada POR NÚMERO
// (canônico §12 risco 2), em vez de mentir que todo número faz tudo.
type Capabilities struct {
    SupportsTemplates bool  // Meta: true. Evolution/WAHA: false
    Requires24hWindow bool  // Meta: true — fora da janela a UI EXIGE template
    SupportsReaction  bool
    SupportsSticker   bool
    SupportsGroups    bool
    MaxMediaBytes     int64
}
```

`Credentials` resolve de `whatsapp_instances.credentials_ciphertext` via `platform/secretbox` (F3),
**por instância** — o que resolve o "avaliar API key por conta" do canônico §13-item 3: env
(`EVOLUTION_API_KEY`) vira **fallback de ambiente**, não a fonte. Credencial **nunca** volta ao
front — só `{set,last4}` (canônico §10).

### C2 — Rota do webhook

```
POST /v1/webhooks/omnichannel/{provider}/{accountSlug}     ← pública, sem JWT
```

**Revisa** o `/v1/webhooks/evolution/{accountSlug}` do `SPECS_PORT_OMNICHANNEL.md` F3 (cravado em
Evolution): com a D-A o `{provider}` vira segmento e a F11 entra **sem nova família de rota**. O
segmento `omnichannel/` evita colisão com o namespace de webhook já existente do `site`
(`/v1/webhooks/leads|products|tracking/{sourceSlug}` — `site/http_ingest.go:25-27`).

**Fora do gate de módulo, por design:** `/v1/webhooks` não está em `moduleGatingRules()`
(`app.go:518` — o comentário da função lista `webhooks` entre as não gateadas). Registro a partir do
`handle.RegisterRoutes` do próprio módulo (precedente: `cardapio/module.go:134`; `calendar/module.go:168,172`).

> **Consequência que a spec resolve:** sem o middleware, ninguém checa módulo/conta. **O service faz
> o equivalente**: slug inexistente **ou** conta inativa **ou** módulo `omnichannel` não habilitado
> para a conta → **404** (nunca 403, nunca "module_disabled" — não revelar existência).

### C3 — Proteções, na ordem (**detalhe em `SPECS_PORT_OMNICHANNEL.md` F3, item 3 — não duplicado aqui**)

| # | Proteção | Status nesta fase |
|---|---|---|
| 1 | Rate-limit `provider:slug:ip` (600/min, block 5 min) → **429** | Herdado. Usar limitador **em memória por escopo+IP** — precedente `cardapio/rate_limit.go` (`allow(scope, ip, limit, window)`), que é o padrão da casa **para rota pública sem JWT**. `httpapi.RateLimit` é por identidade/JWT — não serve aqui |
| 2 | Autenticidade → **401** | **MUDOU:** não é mais um token global de env. `VerifyWebhook` **por provider**, com credencial **por instância** (C1). Comparação **constant-time** (`hmac.Equal`) — modelo confirmado: `site/http_ingest.go:98` |
| 3 | Allowlist de `Content-Type` → **415** | Herdado |
| 4 | `Content-Length`/body acima do limite → **413** | Herdado. `http.MaxBytesReader` (modelo: `site/http_ingest.go:88`) |
| 5 | Conta/slug não resolvido → **404** | Herdado + estendido (C2) |
| 6 | Dedupe idempotente → **202 `{status:"duplicate"}`** | **MUDOU:** ver C4 |

### C4 — Dedupe: tabela, não Redis

O `SPECS_PORT` F3 previa **idempotência em Redis** (`processing`/`done`/release) + cache de QR em
Redis TTL 120s. **O canônico manda tabela** (`webhook_events UNIQUE(provider, external_event_id)` —
§7.1/§9.2-F4) e **vence**. Confirmado no disco: `back/go.mod` **não tem client Redis** (deps: `pgx/v5`,
`gorilla/websocket`, `jlaffaye/ftp`, `pkg/sftp`, `x/crypto`, `x/text`) — seria dependência nova para
resolver o que o Postgres resolve melhor.

**Padrão obrigatório — linha de dedupe e escrita de domínio na MESMA transação:**

```
BEGIN
  insert into messaging.webhook_events (...) on conflict do nothing returning id
  → 0 linhas ⇒ duplicado ⇒ ROLLBACK, responde 202 {status:"duplicate"}
  → 1 linha  ⇒ persiste conversation/message
COMMIT
```

Efeito **exactly-once** sem lock distribuído: se o processamento falha, o rollback leva a linha de
dedupe junto e o **retry do provider reprocessa** (o Redis do legado marcava `done` **antes** de
processar — evento perdido em crash). Dois eventos idênticos concorrentes: o 2º insert bloqueia no
índice único até o 1º commitar e cai no conflito → 202.

### C5 — Migration

**Numerar conferindo o disco.** Última hoje: `0199_calendar_drop_day_media.sql`; existem **dois
arquivos `0197`** — a numeração não é validada por ninguém (canônico §13). **F2 e F3 numeram antes
desta fase**: pegar o próximo livre real, não presumir `0200`. SQL **plano idempotente**,
schema-qualificado, **SEM `-- +goose Down`** (o migrator roda o arquivo inteiro e o Down se
auto-destrói — falha real, ver `0147_automation_contacts_fix.sql`):

```sql
create table if not exists messaging.webhook_events (
    id                uuid        primary key default gen_random_uuid(),
    account_id        uuid        not null references core.accounts(id) on delete cascade,
    provider          text        not null,
    external_event_id text        not null,
    event_kind        text        not null default 'unknown',
    instance_name     text,
    payload_masked    jsonb       not null default '{}'::jsonb,
    received_at       timestamptz not null default now()
);

create unique index if not exists messaging_webhook_events_provider_event_uidx
    on messaging.webhook_events (provider, external_event_id);

create index if not exists messaging_webhook_events_account_received_idx
    on messaging.webhook_events (account_id, received_at desc);

-- Backstop da C6: o mesmo número não fica em duas instâncias da mesma conta.
-- PARCIAL de propósito: phone_number é nullable (só resolve depois de conectar) e
-- em Postgres NULLs não colidem — sem o filtro, o índice não diria nada de útil.
create unique index if not exists messaging_whatsapp_instances_account_phone_uidx
    on messaging.whatsapp_instances (account_id, phone_number)
    where phone_number is not null;
```

A coluna `phone_number` **já existe** (F2 a porta do Prisma); esta fase só acrescenta o índice.

`payload_masked` = cópia **mascarada** para triagem (telefone → últimos 4; corpo de texto →
omitido). **Nunca o body cru** (canônico §10). Retenção/purge desta tabela → F13.

### C6 — Um número, uma instância (validação interna)

Validado **no cadastro/conexão da instância**, não no runtime. Duas instâncias do `omnichannel` na
mesma conta apontando para o **mesmo número** = mensagem duplicada e conversa partida ao meio.

- **Escopo da checagem: só `messaging.whatsapp_instances` da própria conta.** A fonte é o
  `phone_number` da instância (coluna portada do Prisma na F2 — `WhatsAppInstance.phoneNumber:61`).
  Nenhuma leitura de schema de outro módulo.
- O número só é conhecido **após conectar**: validar no cadastro **e** quando o número resolve
  (`connect`/`status` trazem o número do provider — C7).
- Colisão → **409** com erro acionável (princípio 5): dizer **qual** instância já usa o número.
- **A garantia é do banco:** índice único parcial `(account_id, phone_number)` (C5). A checagem no
  service é UX — quem fecha a corrida é a constraint.

> **Risco operacional registrado (fora do código):** apontar o mesmo número de WhatsApp para este
> módulo **e** para um sistema externo ao mesmo tempo é responsabilidade de quem opera. O módulo
> não tem como ver esse outro lado e **não tenta** — registrado como risco, não gateado por código.

### C7 — Sessão (bootstrap/connect/qrcode/status/logout)

Contrato **já detalhado** em `SPECS_PORT_OMNICHANNEL.md` F3 itens 1 e 2 (client Evolution: header
`apikey`, timeout 30s, `createInstance`/`connect`/`fetchInstances`/`logout`/`setWebhook`; sessão:
limite de canais → 409, promoção de default, re-escopo de conversas `default`, `status` com cache +
dedupe de in-flight e auto-reparo de webhook, QR normalizado para data URL) — **não duplicado aqui**.

**Só o que mudou:** o **cache de QR (TTL 120s) vai para memória**, não Redis (C4). Precedente de
client HTTP de provider: `meta_ads/runner_client.go:49-53` (baseURL + token + `http.Client{Timeout}`,
nunca exposto ao cliente — o painel passa pela API Go).

---

## Armadilhas / o que NÃO fazer

| # | Armadilha | Regra |
|---|---|---|
| 1 | **Auto-criar instância desconhecida** (o legado faz — `SPECS_PORT` F3 item 3) | **NÃO portar.** Webhook é input não-confiável; auto-criar linha escopada por conta a partir dele fura o cadastro — e é justamente onde o C6 valida. Instância desconhecida → **202 `{status:"ignored"}`** + audit |
| 2 | **Colisão de `external_event_id` entre contas** | O UNIQUE do canônico é `(provider, external_event_id)` — **global**, não por conta. Se dois clientes gerarem o mesmo id, o evento do segundo some silenciosamente. **Compor o id com o escopo da instância** (`{instanceName}:{providerMessageId}`) — respeita o UNIQUE canônico e mata a colisão |
| 3 | **Payload bruto em log/erro/trace** | Proibido (canônico §10). Erro de `ParseWebhook` **não** carrega o body. Só `payload_masked` no banco |
| 4 | **Emitir realtime aqui** | Realtime é **F5**. F4 pode deixar o *seam* (`Publisher` com default no-op, padrão `calendar/publisher.go:41-55`), mas **não** inlinar publicação no handler do webhook |
| 5 | **`created_at` = `now()`** | Inbound grava `status: SENT` e `created_at` = **timestamp do provider** (`SPECS_PORT` F3 item 4) |
| 6 | **Unificar shapes por conveniência** | Os shapes por call-site do port não se unificam (`SPECS_PORT` F4) — quebra o front |
| 7 | **Rate-limit em memória ≠ multi-instância** | Limitação real e conhecida (o próprio `httpapi/rate_limit.go:27` registra: "Nao serve para deploy multi-instancia sem broker"). Hoje a API é um container só — **registrar**, não esconder |
| 8 | **Filtro de instância por usuário inoperante** | Bug do legado (`whatsapp-instances.ts:681-683`, ternário que retorna o mesmo nos dois ramos). Portar **corrigido** — é isolamento (`PLANO_PORT_OMNICHANNEL.md` §8) |

---

## Segurança

| Regra | Detalhe |
|---|---|
| **`account_id` NUNCA do body** | No webhook não há Principal: a conta resolve do `{accountSlug}` do path **no server**, e o evento só grava dentro dela. Nada no payload do provider escolhe conta |
| **Repositório filtra por conta também** | Defesa em profundidade (princípio 2), mesmo com o service já tendo resolvido |
| **Fora de escopo → 404, nunca 403** | Enumeration. Vale para slug inexistente, conta inativa e módulo desabilitado (C2) |
| **Credenciais só via `secretbox`** (F3) | Nunca coluna crua, nunca log, nunca de volta ao front (só `{set,last4}`) — canônico §10 |
| **Constant-time** | Toda comparação de token/assinatura via `hmac.Equal` — modelo `site/http_ingest.go:98` |
| **Body sempre limitado** | `MaxBytesReader` **antes** de ler — modelo `site/http_ingest.go:88` |

---

## Verificável

1. **QR e conexão:** abrir `/omnichannel`, ler o QR no painel, conectar um número real.
2. **Mensagem real:** mandar mensagem do celular → `select id, content, created_at from
   messaging.messages order by created_at desc limit 1;` traz a mensagem, na conversa certa, com
   `created_at` = horário **do provider** (não o do insert).
3. **Sem assinatura → 401:** `curl -i -X POST $BASE/v1/webhooks/omnichannel/evolution/{slug}
   -H 'Content-Type: application/json' -d '{}'` → **401**.
4. **Repetido não duplica:** reenviar o **mesmo** payload assinado 2× → 2ª resposta **202
   `{status:"duplicate"}`**; `count(*)` em `webhook_events` para aquele `external_event_id` → **1**;
   `messaging.messages` **não** ganha linha nova.
5. **Isolamento:** webhook no slug da conta A com evento da conta B → nada aparece em B. Slug
   inexistente → **404** (não 403).
6. **Um número, uma instância:** cadastrar/conectar numa 2ª instância da **mesma conta** um número
   que já está em outra → **409** acionável na tela, nomeando a instância que já o usa. A mesma
   tentativa direto no banco (`insert`/`update` em `messaging.whatsapp_instances`) → violação do
   índice único da C5.
7. **Mock sem container:** com `provider = 'mock'`, os fluxos (2), (4) e (5) rodam **sem Evolution no
   ar** — é o que destrava F5/F6/F8 sem infra.
8. **Payload não vaza:** `docker compose logs api | grep -i <texto-da-mensagem>` → **vazio**.

---

## Notas de Deploy

**Ordem exata:** migration → env vars → **`docker compose build --no-cache api`** → `up -d api` →
container do provider → Caddy.

| # | Item | Detalhe |
|---|---|---|
| 1 | Migration `messaging.webhook_events` + índice único parcial de número (C6) | Próximo número **livre no disco** (C5). Idempotente, sem `-- +goose Down`. O índice parcial **falha se já houver número duplicado** numa conta: conferir antes (`select account_id, phone_number, count(*) ... group by 1,2 having count(*) > 1`) |
| 2 | `WEBHOOK_RECEIVER_BASE_URL` | URL que o provider chama de volta (canônico §13-item 5) |
| 3 | `EVOLUTION_BASE_URL` · `EVOLUTION_API_KEY` | **Fallback de ambiente apenas** — a credencial real é por instância, cifrada (C1) |
| 4 | `OMNI_SECRETS_KEY` | Já obrigatória desde a F3. **Sem ela o módulo não sobe** (fail-fast, canônico §13-item 2) |
| 5 | Container `evolution` | Novo serviço no compose, **profile próprio**, volumes + backup; Caddy **se** precisar de rota pública |
| 6 | Caddy | A rota `/v1/webhooks/*` precisa chegar na API de fora. Armadilha registrada: `cat >` no Caddyfile **não pega** no inode do bind-mount — reload não basta, `docker restart` do container do Caddy |
| 7 | Rebuild | Mexeu em `back/` → `docker compose up -d --build api`. **Migration nova → `build --no-cache api`**: migrations são `embed.FS` e o cache da camada `go build` pode não re-embutir o `.sql`. Sintoma: `migrate status` para na migration anterior, **sem erro** |

---

**Referências:** canônico [`PLANO_ATENDIMENTO.md`](../PLANO_ATENDIMENTO.md) ·
anexo técnico [`PLANO_PORT_OMNICHANNEL.md`](../PLANO_PORT_OMNICHANNEL.md) ·
contratos verbatim [`SPECS_PORT_OMNICHANNEL.md`](../SPECS_PORT_OMNICHANNEL.md) (F3 = proteções do
webhook e sessão; F4 = shapes de realtime) · princípios `skill principios-engenharia`.
