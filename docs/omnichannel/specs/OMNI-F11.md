# OMNI-F11 — Adapter Meta WhatsApp Cloud (número oficial)

**Prioridade:** P1 · **Plano canônico:** [`docs/omnichannel/PLANO_ATENDIMENTO.md`](../PLANO_ATENDIMENTO.md) (§5.4, §9.2-F11, §10, §12 riscos 1/2/3, §13-item 4)

> ## LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono)
> A branch `refactor/multi-tenant-complete` fechou e o dono **liberou a implementação em
> 2026-07-17** (decisão **D-D**, canônico §2). O aviso de congelamento que constava aqui não vale mais.

**Convenções:** ler a skill `principios-engenharia` antes. `account_id` **sempre** do Principal/slug
resolvido no server, nunca do body · repositório filtra por conta também · fora de escopo → **404**,
nunca 403 · teto ~450 linhas/arquivo **vale** (código novo) · mudou `back/` → `up -d --build api`;
migration nova → `build --no-cache api` · não rodar git.

---

## Objetivo

O terceiro adapter entra **pela interface da F4, sem tocar no domínio nem no front** — e é isso que
prova que a D-A é arquitetura real e não promessa. Um número oficial da Meta recebe e envia; a janela
de 24h e os templates ficam declarados em `Capabilities()`, de modo que a UI **degrada por número** em
vez de mentir para o atendente. O risco do não-oficial sai do código e entra no **contrato com o
cliente**.

**Fora de escopo:** esta fase **não** cria a interface `Provider`, **não** cria rota de webhook nova
(entra na família da F4) e **não** lê nenhum outro módulo. Só `messaging.*`.

## Depende de / Bloqueia

| | Fases |
|---|---|
| **Depende de** | **F4** — `channel.Provider`, `Capabilities`, registry, rota `/v1/webhooks/omnichannel/{provider}/{accountSlug}`, dedupe `webhook_events`, guard "um número = um cérebro" (`OMNI-F4.md` C1–C6). **A interface já existe: esta fase escreve o adapter, não a interface** · **F3** (`platform/secretbox` para a credencial) · **F6** (`SendMessage` é chamado pelo handler do outbox) |
| **Bloqueia** | Nada. É P1, roda **depois do piloto P0** (`F0 → F10 + F13-mínimo`) |
| **Aberto** | **C8** — quem constrói a degradação do composer não tem fase dona. **Decisão do dono.** O backend desta spec fecha sem isso |

---

## Entregas

| # | Entrega | Alvo |
|---|---|---|
| 1 | Adapter Cloud: `ID()`, `VerifyWebhook`, `ParseWebhook`, `SendMessage`, `DownloadMedia`, `Capabilities` | `back/internal/modules/omnichannel/channel/meta/adapter.go` |
| 2 | Client HTTP da Graph API (baseURL + versão + Bearer, timeout próprio) | `.../channel/meta/client.go` |
| 3 | Parser do envelope Cloud (`entry[].changes[].value.{messages,statuses}`) → `[]channel.Event` | `.../channel/meta/parse.go` |
| 4 | **GET de verificação** do webhook (`hub.mode`/`hub.verify_token`/`hub.challenge`) | `.../omnichannel/http_webhook.go` (arquivo **da F4** — estender, não duplicar) |
| 5 | Guard da janela de 24h + envio de template (`metadataJson.template` — C7) | `.../omnichannel/service_outbound.go` (arquivo **da F6** — estender) |
| 6 | `GET /v1/omnichannel/whatsapp/instances/{id}/templates` + `GET /v1/omnichannel/conversations/{id}/send-window` | `.../omnichannel/http_config_instances.go` (F10) · `.../omnichannel/http_messages.go` (F6) |
| 7 | Registro do adapter no registry da F4 | `.../channel/registry.go` |
| 8 | Migration: **só** o índice parcial da janela (C7) | `back/internal/platform/database/migrations/` — **próximo número livre, conferir o disco** |
| 9 | Testes: assinatura (válida/inválida/ausente), parser com fixtures reais, guard da janela | `.../channel/meta/{verify,parse}_test.go` |
| 10 | Sincronizar os 3 docs ao fechar + **o risco 3 no contrato do cliente** (C10) | `.../omnichannel/AGENT.md` · canônico · `phases-part7.ts` |

**Numeração da migration:** **não cravar número.** Última no disco hoje é `0199_calendar_drop_day_media.sql`
e existem **dois arquivos `0197`** (`0197_operation_validation_reason.sql`, `0197_tools_module.sql`) — a
numeração não é validada por ninguém (canônico §13). F2/F3/F4/F8/F9 numeram **antes** desta fase:
pegar o próximo livre **real**. SQL plano idempotente, schema-qualificado, **sem `-- +goose Down`**.

---

## Contratos

### C1 — Rota: a família já existe (F4, C2)

```
GET  /v1/webhooks/omnichannel/meta_whatsapp_cloud/{accountSlug}   ← verificação (C2)
POST /v1/webhooks/omnichannel/meta_whatsapp_cloud/{accountSlug}   ← eventos (C3/C4)
```

**Zero rota nova.** É exatamente o que a F4 comprou ao promover `{provider}` a segmento de path
("a F11 entra sem nova família de rota" — `OMNI-F4.md` C2). Pública, sem JWT, **fora do
`moduleGatingRules()`**. Todas as proteções da **F4 C3** (rate-limit → 429, allowlist de
`Content-Type` → 415, `MaxBytesReader` → 413, slug/conta/módulo não resolvido → **404**, dedupe →
202 `{status:"duplicate"}`) valem **inalteradas** — **não redefinidas aqui**. O que muda é só o
**passo 2** (autenticidade): `VerifyWebhook` do adapter Meta.

### C2 — GET de verificação (o handshake que a Meta exige)

A Meta valida a URL **antes** de entregar qualquer evento: `GET` com
`hub.mode=subscribe` · `hub.verify_token=<token>` · `hub.challenge=<nonce>`.

| Regra | Detalhe |
|---|---|
| Resposta | `hub.mode == "subscribe"` **e** verify token confere → **200** com o **`hub.challenge` cru no corpo** (`text/plain`), **sem JSON, sem aspas** — a Meta compara byte a byte |
| Falha | Token errado/ausente ou `hub.mode` diferente → **403** *(exceção deliberada ao 404-de-escopo: é o valor que a Meta espera no handshake e não há recurso a enumerar — o slug já resolveu)*. Slug inexistente continua **404** (F4 C2) |
| Comparação | `hmac.Equal` — constant-time, mesmo sendo token e não assinatura (modelo `site/http_ingest.go:98`) |
| Origem do token | `whatsapp_instances.provider_config.verifyToken` **da conta do slug**; env `META_VERIFY_TOKEN` é **fallback de ambiente**, nunca a fonte (mesmo padrão de `EVOLUTION_API_KEY` — `OMNI-F4.md` C1) |

O GET é **idempotente e sem efeito**: não cria instância, não grava `webhook_events`, não audita
payload.

### C3 — `VerifyWebhook`: HMAC no header **da Meta**

```go
// back/internal/modules/omnichannel/channel/meta/adapter.go
func (a *Adapter) VerifyWebhook(hdr http.Header, body []byte, cred channel.Credentials) error
```

| Regra | Detalhe |
|---|---|
| Header | **`X-Hub-Signature-256: sha256=<hex>`** — imposto pela Meta. **Não é o `X-Signature` da casa** (ver Armadilhas nº1) |
| Segredo | **App Secret** da Meta (não o access token, não o verify token) |
| Corpo | HMAC-SHA256 sobre o **body cru, byte a byte, como chegou** — antes de qualquer `Unmarshal`/re-serialize (ver Armadilhas nº2) |
| Comparação | `hmac.Equal` sobre os hex — constant-time |
| Falha | **401**, com erro **sem o body** (payload bruto nunca vaza — canônico §10; `OMNI-F4.md` armadilha 3) |
| Header ausente | **401**. Nunca "sem header = aceita" |

**Mecânica reaproveitável, header diferente.** `site/http_ingest.go` já resolve exatamente este
problema e é o modelo a copiar: `http.MaxBytesReader` antes de ler (`:88`), `hmac.Equal` na
comparação (`:98`), `computeSignature` = `hmac.New(sha256.New, secret)` + `hex.EncodeToString`
(`:112-116`). **O que não se copia é o header:** lá é `X-Signature: sha256=<hex>` (padrão próprio,
estilo GitHub/Stripe); aqui é `X-Hub-Signature-256`. Mesmo prefixo `sha256=` nos dois — o
`TrimPrefix` é igual.

**Resolução do segredo (o ovo e a galinha):** não dá para confiar no payload para descobrir de quem
é a credencial — o payload é justamente o que ainda não foi autenticado. A conta vem do
**`{accountSlug}` do path, resolvido no server** (F4 C2). Daí:

1. Resolver a conta pelo slug → **404** se não existir/estiver inativa/sem o módulo.
2. Carregar os App Secrets **distintos** das instâncias `meta_whatsapp_cloud` **daquela conta**
   (via `platform/secretbox` — F3). Normal: **um** (um app Meta por conta).
3. Verificar contra cada candidato com `hmac.Equal`; nenhum casou → **401**.
4. **Só depois** parsear e resolver a instância exata por `metadata.phone_number_id` (C4).
5. `phone_number_id` que não bate com nenhuma instância da conta → **202 `{status:"ignored"}`** +
   audit. **Nunca auto-criar instância a partir do webhook** (F4, armadilha 1).

### C4 — `ParseWebhook`: envelope Cloud → evento canônico

O domínio **não vê nada disto** — só `[]channel.Event` (F4 C1). Payload é **dinâmico**: parsear
defensivamente, campo ausente **nunca** entra em pânico.

| Ponto do envelope | Vira |
|---|---|
| `entry[].changes[].value.messages[]` | `Event.Kind = message_received` + `InboundMessage` |
| `entry[].changes[].value.statuses[]` | `Event.Kind = message_status` + `StatusUpdate` |
| `entry[].changes[].value.metadata.phone_number_id` | Resolve a **instância** (C3, passo 4) |
| `messages[].id` (`wamid.…`) | Base do `ExternalEventID` |
| `messages[].timestamp` (**UNIX em segundos, como string**) | `Event.OccurredAt` — **não `now()`** (F4, armadilha 5) |
| `object != "whatsapp_business_account"` ou `field != "messages"` | `Kind = ignored` → **202**, sem erro |

- **Um POST traz N eventos.** `entry[]` e `changes[]` são arrays: iterar, nunca assumir `[0]`.
- **`ExternalEventID` composto** (F4, armadilha 2): o UNIQUE canônico é
  `(provider, external_event_id)` — **global, não por conta**. Compor `{instanceName}:{wamid}` para
  o evento de mensagem. Para `statuses[]`, o `wamid` **se repete** a cada mudança de status: compor
  com o status e o timestamp (`{instanceName}:{wamid}:{status}:{ts}`) — senão a 2ª atualização do
  mesmo `wamid` é comida pelo dedupe e o status congela.
- `DELIVERED`/`READ` **existem** no `statuses[]` da Cloud e **não são portados**: o enum do front é
  `PENDING|SENT|FAILED` (`web-reference/app/types/index.ts:89`, verificado). Mapear
  `sent→SENT`, `failed→FAILED`; `delivered`/`read` → **`Kind = ignored`**. Ampliar o enum é feature
  nova, avaliada na F14 (canônico §15).

### C5 — Credenciais e `provider_config`

Cifradas via `platform/secretbox` (F3), **por instância**, em `whatsapp_instances.credentials_ciphertext`
(coluna nasce na F2 — canônico §7.2). **Nunca** em coluna crua, **nunca** em log, **nunca** de volta ao
front — só `{set,last4}`.

| Campo | Onde | Papel |
|---|---|---|
| `appSecret` | credencial (cifrada) | HMAC do inbound (C3) |
| `accessToken` | credencial (cifrada) | `Authorization: Bearer` na Graph API |
| `verifyToken` | credencial (cifrada) | handshake do GET (C2) |
| `phoneNumberId` · `wabaId` | `provider_config jsonb` (claro) | roteamento de envio e listagem de templates. **Não são segredo** |
| `graphVersion` | `provider_config` → default de env | **Nunca cravar a versão no código.** A Meta descontinua versão; versão morta = número mudo |

### C6 — `Capabilities()` do Cloud

A struct é **da F4** (`OMNI-F4.md` C1) — aqui só os valores. É estático **por provider**; o estado
da janela é **por conversa** (C7), e são coisas diferentes.

| Campo | Cloud | Evolution/WAHA | Por quê |
|---|---|---|---|
| `SupportsTemplates` | **true** | false | Só a Cloud tem template aprovado |
| `Requires24hWindow` | **true** | false | **A assimetria** (canônico §12, risco 2) |
| `SupportsReaction` | true | true | |
| `SupportsSticker` | true | true | |
| `SupportsGroups` | **false** | true | A Cloud **não** faz grupo. Declarar honestamente |
| `MaxMediaBytes` | limite da Cloud por tipo — **conferir a doc vigente na hora** | | Valor errado = 413 do provider virando FAILED sem explicação |

### C7 — Janela de 24h e templates (o coração da fase)

**A regra:** fora da janela de 24h desde a **última mensagem do cliente** (INBOUND), a Cloud recusa
qualquer mensagem que não seja **template aprovado**. O não-oficial não tem essa restrição — por isso
isto é `Capabilities`, não `if provider == "meta"` espalhado pelo código.

**Como a janela é calculada** (derivada, nunca coluna de estado — princípio 1):

```sql
select max(created_at) from messaging.messages
where conversation_id = $1 and direction = 'INBOUND';
```

Janela aberta ⇔ `now() - max(created_at) < 24h`. Sem INBOUND nenhum ⇒ **fechada** (conversa iniciada
pela empresa). Índice parcial (a **única** migration desta fase — o `(conversation_id, created_at)`
da F2 obriga a varrer a conversa de trás para frente filtrando `direction`):

```sql
create index if not exists messaging_messages_last_inbound_idx
    on messaging.messages (conversation_id, created_at desc)
    where direction = 'INBOUND';
```

**Como o template chega no back — sem furar o contrato verbatim:**

`MessageType` do front é `TEXT|IMAGE|AUDIO|VIDEO|DOCUMENT` (`web-reference/app/types/index.ts:90`,
verificado) — **não existe `TEMPLATE`**, e criar um quebraria a D-B. A saída é a que o port já usa
para reply/quote e sticker (`OMNI-F6.md` C1 e canônico §9.2-F6): **`metadataJson`**, que é
`Record<string, unknown>` no front.

```
POST /v1/omnichannel/conversations/{id}/messages     (rota da F6 — contrato inalterado)
{ "type": "TEXT", "content": "<render do template, para o histórico>",
  "metadataJson": { "template": { "name": "…", "language": "pt_BR", "params": ["…"] } } }
```

- **Aditivo, zero campo novo no body.** O front verbatim ignora o que não conhece; nenhuma união de
  tipos muda.
- `content` é o texto renderizado — o histórico não pode virar `{{1}}` para o atendente.
- **O guard é do backend, não da UI** (a UI é conveniência; a autoridade é o servidor):

| Situação | Resposta |
|---|---|
| Janela **aberta** | Envia normal. `metadataJson.template` presente → envia como template mesmo assim (é legítimo) |
| Janela **fechada** + sem `metadataJson.template` | **409** `outside_24h_window`, com `{windowClosedAt, requiresTemplate:true}` e mensagem acionável (princípio 5). **Nunca** enfileirar para falhar depois no provider |
| Janela **fechada** + com template | Envia |
| Provider **sem** `Requires24hWindow` | Guard **não roda** — é o Evolution/WAHA de sempre |

- **Onde roda:** no `POST` (rejeita **antes** de criar a `message` e o job — F6 fluxo), **e** o
  adapter classifica o erro `131047` (*re-engagement*, **conferir o código na doc vigente**) como
  **unrecoverable → `max_attempts = 1`** (classificação da F3): retentar 5× uma mensagem que a Meta
  já recusou por política é queimar rate limit sem chance de sucesso.

**Rotas novas (2):**

| Rota | Permissão | Devolve |
|---|---|---|
| `GET /v1/omnichannel/whatsapp/instances/{id}/templates` | `omnichannel.instances.manage` | Lista **viva** da Graph API (`{wabaId}/message_templates`) + cache curto **em memória**. **Sem tabela**: quem aprova é a Meta — copiar o status para o Postgres cria segunda verdade (princípio 1) e a tela mostraria "aprovado" para template já reprovado |
| `GET /v1/omnichannel/conversations/{id}/send-window` | `omnichannel.conversations.view` + visibilidade da F8 | `{open, expiresAt, requiresTemplate}` |

**`send-window` é rota própria, não campo do `GET /conversations`:** o cálculo é por conversa e na
lista viraria N+1 (canônico §9.2-F2: a lista **não** pagina). A tela pede ao abrir a conversa.

### C8 — BLOQUEIO ABERTO: quem degrada o composer (decisão do dono)

> **O backend desta spec fecha sem esta decisão.** O que não fecha sem ela é **um item do
> Verificável do canônico §9.2-F11**: *"fora da janela de 24h a UI exige template"*.

**O gap, verificado — não é suposição:**

| Fato | Evidência |
|---|---|
| O canônico exige a UI | §9.2-F11 e §12 risco 2 ("a UI degrada por número") |
| O roadmap põe a UI na F10 | `phases-part7.ts`, task `omni-f10-capabilities` (`[frontend]`) |
| **Mas a F10 se proíbe de tocar no inbox** | `OMNI-F10.md`, tabela de Escopo: *"Inbox verbatim (F1) — não se toca"* |
| E o composer **é** o inbox verbatim | D-B: o front do inbox é port byte a byte |

Ou seja: o seletor de template e o bloqueio do campo livre moram no **composer**, e **nenhuma fase
tem dono para tocá-lo**. A F10 entrega o badge de capabilities nas **telas de config** (que são dela)
— não o composer.

| # | Opção | Consequência |
|---|---|---|
| **A** *(recomendada)* | Patch **contido** no composer verbatim, nesta fase: consome `send-window` + `templates`, bloqueia o campo livre e mostra o seletor | Cumpre o Verificável do canônico. **Custo:** o front deixa de ser byte a byte → **entra no `docs/LEGADO.md` com alvo F14** (que já vai refatorar esses arquivos). Honesto e rastreável |
| **B** | F11 **backend-only**; a UI vira fase própria | D-B intacta. **Custo:** até lá, o atendente digita, manda e leva **409** — erro **honesto e acionável** (não falha silenciosa), mas descoberto **depois** de escrever. O Verificável do canônico §9.2-F11 fica **parcial** e a F11 não fecha inteira |
| **C** | Ampliar o escopo da F10 para incluir o composer | Só empurra a mesma decisão para outra spec — a F10 continuaria furando a própria regra |

**Enquanto não houver decisão: vale o B por omissão** — o guard 409 do C7 já protege o cliente final
(mensagem nunca sai errada), e é isso que torna o backend entregável sozinho.

### C9 — Embedded Signup → **P2, fora desta fase**

Onboarding self-service (o cliente conecta o próprio número pelo painel, sem sair dele) **exige app
review da Meta** — processo de negócio, com prazo e revisor humano, que **não depende de código**.
Cravar isto na F11 significa a fase inteira ficar refém de uma aprovação externa.

**O que a F11 faz no lugar:** onboarding **manual** — o operador cola `phoneNumberId`, `wabaId`,
`accessToken`, `appSecret` e `verifyToken` na tela de números da F10 (C5). Funciona hoje, sem
review, e é o caminho que o cliente sério já percorre com o parceiro Meta.

**Risco 1 do canônico registrado, não resolvido:** a Cloud cobra **por conversa** e mantém *quality
rating* que **degrada o número**. É **risco de produto** — quem decide preço e política é a Meta.
Entra no C10, não no código.

### C10 — Registro de risco **no contrato com o cliente** (risco 3, entrega de doc)

**Isto é entrega da fase, não recomendação.** Uma linha no código não protege ninguém de um número
banido.

| O que registrar | Redação-guia |
|---|---|
| Evolution/WAHA são **não-oficiais** | Usam WhatsApp não-oficial; o WhatsApp pode **banir o número** a qualquer momento, sem aviso e sem recurso |
| Conta séria usa **Cloud** | O número oficial é o único caminho com garantia contratual da Meta |
| Rate limit é **mitigação** | O throttle por número (F6) **reduz** a chance de ban. **Não é garantia** — prometer que não toma ban é promessa que não se pode cumprir |
| Cloud tem **custo por conversa** | Cobrança da Meta, repassada; e *quality rating* pode degradar o número (risco 1) |
| Janela de 24h | Fora dela, **só template aprovado**. Aprovação é da Meta, com prazo dela |

Vive no material comercial/contratual do cliente + `docs/LEGADO.md` se algum cliente ficar em
não-oficial **por decisão registrada**. **A spec não redige contrato jurídico** — entrega os fatos
técnicos para quem redige.

---

## Armadilhas / o que NÃO fazer

| # | Armadilha | Regra |
|---|---|---|
| 1 | **Reusar o header `X-Signature` da casa** | O `site/http_ingest.go` usa `X-Signature` (padrão próprio). A **Meta exige `X-Hub-Signature-256`** e não negocia. Copiar a **mecânica** (`hmac.Equal`, `MaxBytesReader`, `computeSignature`); **não** o nome do header |
| 2 | **Assinar o body re-serializado** | HMAC é sobre os **bytes crus como chegaram**. `Unmarshal` + `Marshal` reordena chave e normaliza espaço → assinatura **nunca** bate, com credencial correta. Ler o body **uma vez**, guardar `[]byte`, verificar, **depois** parsear |
| 3 | **`hub.challenge` em JSON** | A Meta quer o nonce **cru** em `text/plain`. `WriteJSON` devolve `"123"` com aspas → verificação falha e a Meta **nunca** entrega evento |
| 4 | **`ExternalEventID` = `wamid` puro para `statuses[]`** | O `wamid` se repete a cada mudança de status: o dedupe come a 2ª atualização e o status congela em SENT para sempre |
| 5 | **Presumir `entry[0].changes[0]`** | São arrays: um POST traz N eventos. Iterar |
| 6 | **Cravar `graphVersion` no código** | Versão da Graph API é descontinuada por prazo. Versão morta = número mudo, sem deploy que salve. Vem de `provider_config`/env (C5) |
| 7 | **Enfileirar fora da janela e "deixar o provider dizer"** | Vira FAILED sem explicação para o atendente. O guard é **no POST**, com 409 acionável (C7, princípio 5) |
| 8 | **Retentar `131047` 5×** | Recusa por política, não erro transitório. Classe **unrecoverable → 1 tentativa** (F3) |
| 9 | **Espalhar `if provider == "meta_whatsapp_cloud"` no domínio** | É exatamente o que a interface da F4 existe para evitar. A assimetria se expressa em `Capabilities()` e no guard genérico, que lê `Requires24hWindow` |
| 10 | **Copiar templates para o Postgres** | Quem aprova/reprova é a Meta: a cópia mostra "aprovado" para template já reprovado (segunda verdade, princípio 1) |
| 11 | **Auto-criar instância a partir do webhook** | `phone_number_id` desconhecido → **202 `ignored`** + audit (F4, armadilha 1). Webhook é input não-confiável |
| 12 | **Mapear `delivered`/`read` para o enum do front** | Não existem em `MessageStatus` (`PENDING|SENT|FAILED`, verificado). Feature nova → F14 (canônico §15) |
| 13 | **Body/payload em log, erro ou trace** | Proibido (canônico §10). Erro de assinatura diz **que** falhou, não **o que** veio |
| 14 | **Numerar a migration às cegas** | Dois `0197` no disco; F2–F9 numeram antes. Conferir o disco |
| 15 | **Resolver a credencial pelo payload** | O payload é o que ainda **não** foi autenticado. A conta vem do **slug do path** (C3) |

## Segurança

| Item | Regra |
|---|---|
| **`account_id` nunca do body** | No webhook não há Principal: a conta resolve do `{accountSlug}` do path **no server** (F4 C2). Nada no payload da Meta escolhe conta |
| **Autenticidade** | HMAC-SHA256 do body cru em `X-Hub-Signature-256`, `hmac.Equal` constant-time. Sem header → **401**. Sem exceção "de teste", sem bypass por env |
| **Body limitado** | `MaxBytesReader` **antes** de ler (modelo `site/http_ingest.go:88`) → **413** |
| **Repositório filtra por conta** | Defesa em profundidade (princípio 2), mesmo com o slug já resolvido |
| **Fora de escopo → 404** | Slug inexistente, conta inativa, módulo desabilitado (F4 C2). **403 só** no handshake do GET (C2) e em falta de permissão — nunca para esconder existência |
| **Segredos** | `appSecret`/`accessToken`/`verifyToken` **só** via `secretbox` (F3). Nunca crus, nunca em log, nunca de volta ao front (só `{set,last4}`) |
| **`send-window` e `templates`** | Rotas **autenticadas**, dentro do gate do módulo. `send-window` respeita a **visibilidade por fila** da F8 (Contrato 5) — conversa fora da fila do atendente → **404** |
| **SSRF** | O `baseURL` da Graph é **fixo server-side** (`graph.facebook.com`), parametrizado só na **versão**. Nunca aceitar host do painel — é o risco que a `OMNI-F3.md` F3.3 já registra para o LLM |

## Verificável

Um humano prova com um número oficial real:

1. **Handshake.** Cadastrar a URL no painel da Meta →
   `curl -i "$BASE/v1/webhooks/omnichannel/meta_whatsapp_cloud/{slug}?hub.mode=subscribe&hub.verify_token=<token>&hub.challenge=12345"`
   → **200**, corpo **exatamente** `12345` (sem aspas, sem JSON). Token errado → **403**. Slug
   inexistente → **404**. No painel da Meta, a URL fica **verificada**.
2. **Recebe.** Mandar mensagem do celular para o número oficial →
   `select id, content, created_at from messaging.messages order by created_at desc limit 1;` → a
   mensagem, na conversa certa, com `created_at` = **timestamp da Meta** (não o do insert).
3. **Assinatura inválida → 401.** `curl -i -X POST .../meta_whatsapp_cloud/{slug} -H 'Content-Type: application/json' -H 'X-Hub-Signature-256: sha256=deadbeef' -d '{}'` → **401**.
   **Sem** o header → **401**. Assinatura **válida** do mesmo body → **200/202**.
4. **Repetido não duplica.** Reenviar o mesmo POST assinado 2× → 2ª resposta **202
   `{status:"duplicate"}`**; `messaging.messages` não ganha linha.
5. **Status não congela.** Mesmo `wamid` com `sent` e depois `failed` → **as duas** são processadas
   (o dedupe **não** come a segunda) e a `message` termina em `FAILED`.
6. **Envia (janela aberta).** Responder do painel dentro de 24h da última mensagem do cliente →
   chega no celular, status **SENT**.
7. **Janela fechada → 409.** Numa conversa cujo último INBOUND tem >24h,
   `POST .../conversations/{id}/messages {"type":"TEXT","content":"oi"}` → **409
   `outside_24h_window`** com `requiresTemplate:true`. **Nenhuma** linha em `messaging.outbox`, e
   `GET .../conversations/{id}/send-window` → `{open:false}`.
8. **Template passa.** O mesmo POST **com** `metadataJson.template` → **200**, chega no celular.
9. **Assimetria.** A mesma conversa num número `evolution` → o POST de texto **passa**, sem 409: o
   guard só roda com `Requires24hWindow`.
10. **Capabilities.** `GET .../whatsapp/instances/{id}/capabilities` → `SupportsTemplates:true`,
    `Requires24hWindow:true`, `SupportsGroups:false` no número Cloud; o oposto no `evolution`.
11. **Não vaza.** `docker compose logs api | grep -i <texto-da-mensagem>` → **vazio**. Idem para o
    `accessToken` e o `appSecret`.
12. **Testes.** `go test ./internal/modules/omnichannel/channel/meta/... -v` — assinatura
    válida/inválida/ausente, parser com **fixture real de `messages` e de `statuses`**, envelope de
    outro `object` → `ignored`.

## Notas de Deploy

**Ordem exata:** migration → env vars → **`build --no-cache api`** → `up -d api` → Caddy → cadastro
da URL no painel da Meta.

| # | Item | Detalhe |
|---|---|---|
| 1 | Migration do índice parcial (C7) | **Próximo número livre — conferir o disco** (dois `0197`; última hoje `0199`; F2–F9 numeram antes). Idempotente, sem `-- +goose Down`. `create index` em `messaging.messages` **trava escrita** na tabela: avaliar `concurrently` **fora** da migration se a tabela já tiver volume |
| 2 | `META_APP_SECRET` · `META_VERIFY_TOKEN` (canônico §13-item 4) | **Fallback de ambiente apenas** — a fonte é a credencial cifrada **por instância** (C5). Ler no padrão `getEnv` de `back/internal/platform/config/config.go:229`. **Nunca** commitar |
| 3 | `META_GRAPH_VERSION` | Default da versão da Graph (C5). Sobrescrevível por `provider_config`. Sem cravar no código (armadilha 6) |
| 4 | `OMNI_SECRETS_KEY` | Já obrigatória desde a **F3** — sem ela a api **não sobe**. As credenciais Meta são inúteis sem ela |
| 5 | `WEBHOOK_RECEIVER_BASE_URL` | Já da F4. A URL cadastrada na Meta é `{base}/v1/webhooks/omnichannel/meta_whatsapp_cloud/{accountSlug}` |
| 6 | **Rebuild** | Mexeu em `back/` → `docker compose up -d --build api`. **Migration nova → `docker compose build --no-cache api`**: migrations são `embed.FS` e o cache da camada `go build` pode **não re-embutir** o `.sql`. Sintoma: `migrate status` para na anterior, **sem erro** |
| 7 | Caddy | `/v1/webhooks/*` já precisa chegar na API desde a F4. **A Meta exige HTTPS com certificado válido** e **não** aceita porta não-padrão. Armadilha registrada: `cat >` no Caddyfile **não pega** no inode do bind-mount — `docker restart` do container do Caddy, reload não basta |
| 8 | **Sem container novo** | A Cloud é API da Meta: nada de Evolution/WAHA para o número oficial. **Sem dependência nova no `go.mod`** — `crypto/hmac` + `net/http` da stdlib bastam (o `go.mod` não tem client Redis nem SDK da Meta, e esta fase não acrescenta nenhum) |

---

## Divergências com o canônico (registradas, não decididas por conta própria)

| # | Ponto | O canônico diz | Esta spec faz | Por quê |
|---|---|---|---|---|
| 1 | **Dono da degradação do composer** | §9.2-F11 exige "fora da janela a UI exige template"; o roadmap põe a UI na F10 | **Registra como bloqueio aberto** (C8) e entrega o **guard 409 no backend** | A `OMNI-F10.md` se proíbe de tocar o inbox verbatim ("Inbox verbatim (F1) — não se toca"), e o composer **é** o inbox. Nenhuma fase tem dono. **Decisão do dono** — o backend fecha sem ela |
| 2 | **403 no handshake do GET** | §10: fora de escopo → **404, nunca 403** | **403** quando o verify token não confere (C2) | O 404-de-escopo existe contra *enumeration*; aqui o slug **já resolveu** e não há recurso a enumerar — é o valor que a Meta espera no handshake. **Slug inexistente continua 404** |
| 3 | **`META_APP_SECRET` como env** | §13-item 4 lista a env | Env é **fallback**; a fonte é a credencial **por instância**, cifrada | Multi-tenant de verdade: um App Secret global obriga todas as contas ao mesmo app Meta. É a mesma correção que a F4 fez com `EVOLUTION_API_KEY` (`OMNI-F4.md` C1) |
| 4 | **Migration nesta fase** | §9.2-F11 não menciona migration | **Uma**: o índice parcial da janela (C7) | A janela é derivada de `messaging.messages` e é lida em **todo** envio; o índice `(conversation_id, created_at)` da F2 não cobre o filtro por `direction`. Alternativa (coluna `last_inbound_at`) seria **segunda verdade** (princípio 1) |
