# Plano — Port do módulo Omnichannel para o Omni

> ## ESTE DOCUMENTO NÃO É MAIS CANÔNICO — 2026-07-16
>
> **O canônico do módulo passou a ser [`PLANO_ATENDIMENTO.md`](PLANO_ATENDIMENTO.md).**
> Este plano foi rebaixado a **anexo técnico do FRONT** do inbox: ele continua sendo a
> referência de *como* o front legado se comporta, mas **não decide mais** o backend, o
> provider, a numeração de fases nem o escopo do módulo.
>
> **O que continua valendo aqui (e não está duplicado no canônico):**
>
> | Seção | Conteúdo que segue sendo a referência |
> |---|---|
> | §2, §3 | Inventário dos **78 arquivos / 22.642 linhas** e as fontes de verdade do front |
> | §5 | A costura: **71 verbatim** (73 − 2 pelo **D-F**) + 5 repontados + 6 adaptadores |
> | §7 | Tabelas do port (`messages`, `contacts`, `saved_stickers`, `audit_events`, `hidden_messages`, `account_config`) |
> | §8 | **Mapa de rotas Node→Go** e os **contratos ao pé da letra**: paginação `limit`+`beforeId`, `GET /conversations` sem paginação, os **3 shapes de `message.updated`** e os 2 de `conversation.updated`, data URL → `null` no realtime |
> | §10 | Divergências deliberadas (BFF, socket.io, auth N+1, base64) |
> | §13.3 | Proteções do webhook público (rate-limit, token constant-time, content-type, body limit, idempotência) |
>
> **O que foi SOBRESCRITO pelo canônico:**
>
> | Aqui | Sobrescrito por | O quê |
> |---|---|---|
> | §6 **D1** (Evolution single-provider) | `PLANO_ATENDIMENTO.md` §2 (**D-A**) | Provider virou **adapter multi-provider** — ver bloco de revisão na §6 |
> | §4 (n8n no fluxo) + §9 **F8** (IA no n8n) | `PLANO_ATENDIMENTO.md` §2 (**D-C**) | **LLM nativo em Go.** n8n sai do caminho crítico |
> | §9 (fases **F0..F9**) | `PLANO_ATENDIMENTO.md` §9 e §9.1 | Renumeradas para **F0..F14**. A numeração desta §9 é a **ANTIGA** |
> | §7 (schema) | `PLANO_ATENDIMENTO.md` §7 | `messaging.*` ganha setores/filas/estado/provider; `whatsapp_instances` e `conversations` **nascem** com as colunas novas |
> | §11, §12 (legado/deploy) | `PLANO_ATENDIMENTO.md` §14 e §13 | Listas consolidadas lá |
>
> **Mantido:** **D2** (mídia em disco + stream `Range`). **D4** (código morto do audit) está
> **DECIDIDA: fora** — decisão do dono (**D-F**, 2026-07-17): `OmnichannelAuditModule.vue` e
> `useOmnichannelAudit.ts` **não são portados** (ver §6). **D3 saiu de escopo** — o módulo é
> independente e não toca `automation.*` (ver §6). **Nenhuma decisão do §6 segue aberta.**
>
> > **LIBERADO PARA IMPLEMENTAÇÃO — 2026-07-17 (decisão do dono).** A branch
> > `refactor/multi-tenant-complete` fechou e o dono liberou a implementação (**D-D**). O aviso
> > de congelamento que constava aqui **não vale mais**. Este documento continua sendo *anexo
> > técnico* — a autorização, a ordem de execução e o escopo vêm do canônico
> > ([`PLANO_ATENDIMENTO.md`](PLANO_ATENDIMENTO.md)) e das specs `specs/OMNI-F*.md`.

Anexo técnico do front. Referência de comportamento do inbox de WhatsApp do painel legado
(`web-reference`) portado para o painel novo (`web/` + `back/` Go).

- Criado: 2026-07-16
- Rebaixado a anexo técnico: 2026-07-16
- Status: PLANO (nada implementado)
- Canônico do módulo: [`PLANO_ATENDIMENTO.md`](PLANO_ATENDIMENTO.md)
- Contratos verbatim do front: [`SPECS_PORT_OMNICHANNEL.md`](SPECS_PORT_OMNICHANNEL.md)
- Espelhado em: `web/app/components/roadmap/data/phases-part7.ts` (o roadmap segue a
  numeração **F0..F14** do canônico, não a §9 deste doc)

---

## 1. Objetivo e regra de ouro

Trazer o módulo Omnichannel do painel legado para o Omni **sem redesenhar nada**: o
front vem como está, e o que muda é o que fica **atrás** dele.

**Regra de ouro do port:** o front é a especificação. Quando o Go e o front
divergirem, **o Go se adapta ao front** — não o contrário. Refatorar o front é uma
fase posterior (F9), explícita e separada.

Isso é deliberado: o módulo legado é código maduro, em produção, com comportamento
difícil de reconstruir de memória (janela de mensagens, dedupe, reconexão, guardas de
sync). Reescrevê-lo enquanto se porta seria trocar dois problemas por quatro.

---

## 2. Fontes de verdade

| O que | Onde | Papel |
|---|---|---|
| Front do módulo | `web-reference/app/**/omnichannel/**` | O que copiar |
| Front (original versionado) | `whats-test/apps/painel-web` (branch `main`, último commit 2026-04-23) | Idem — **byte a byte igual** ao `web-reference` |
| Backend Node | `whats-test/apps/atendimento-online-api` | **O contrato** a reimplementar em Go |
| Painel alvo | `fila-atendimento/web` | Destino |
| Backend alvo | `fila-atendimento/back` | Destino |

**Verificado:** `web-reference` e `whats-test/apps/painel-web` são idênticos nos 78
arquivos do módulo (22.642 linhas, contagem por arquivo bate 1:1). `web-reference`
está no `.gitignore` ("trazido apenas para análise/referência"). Para **copiar** tanto
faz; para **consultar histórico** (`git log`/`blame`), use `whats-test`.

---

## 3. Inventário do módulo

**78 arquivos, 22.642 linhas.**

| Pasta | Arqs | Destaques |
|---|---|---|
| `app/pages/admin/omnichannel/` | 5 | **4 são só redirect**; só `inbox.vue` tem conteúdo |
| `app/composables/omnichannel/` | 50 | `useOmnichannelInbox.ts` (1.467), `useOmnichannelInboxHistory.ts` (774), `useOmnichannelAdmin.ts` (764) |
| `app/components/omnichannel/` | 23 | `InboxConversationsSidebar.vue` (1.128), `InboxChatPanel.vue` (1.110) |

Funcionalidades reais que vêm junto: gravação de áudio, GIF (Tenor), stickers, emoji,
menções em grupo, reações, encaminhar, apagar-para-mim/para-todos, seleção múltipla,
sync de histórico, sessão WhatsApp com QR, múltiplas instâncias.

> **Conflito com os princípios (registrar, não esconder):** o limite canônico é
> ~450 linhas/arquivo. O módulo chega a 1.467. O port entra **violando** esse limite
> por decisão explícita (verbatim primeiro). O ESLint vai acusar `max-lines` (warn).
> O split é a fase F9 — não fazer durante o port.

---

## 4. Arquitetura alvo

Três camadas, cada uma com um papel único:

```
Evolution API  ──webhook──>  GO  ──realtime──>  PAINEL (front verbatim)
     ^                        │  (verdade: banco)      │
     │                        │                        │
     └────envia───────────────┤                        └── config da IA
                              │
                              └──dispara──> n8n (só encanamento)
                                              │
                                              └── busca config no Go, chama modelo,
                                                  devolve resposta pro Go
```

- **Go = a verdade.** Recebe o webhook, persiste, emite realtime, envia. Todo dado e
  toda config vivem no Postgres.
- **n8n = encanamento.** Sem lógica, sem config, sem prompt embutido. Só:
  `Webhook → GET config no Go → chamar modelo → POST resposta no Go`. Assim o workflow
  pode ser exportado/importado sem perder nada — que é o objetivo declarado.
- **Painel = onde se configura.** Provider, modelo, prompt, persona, robô ligado/desligado,
  handover: tudo gravado no banco pelo painel e servido ao n8n pelo Go.

Isso repete o padrão que já funcionou no calendário (`calendar.config` → `body.ai`) e
respeita o princípio 1 (fonte única no banco, nada hardcoded).

**Decisão de fluxo:** o webhook do Evolution bate **no Go, não no n8n**. Se batesse no
n8n primeiro, o n8n teria que decidir/parsear — isso é lógica, e é justamente o que se
quer evitar. O Go persiste (a mensagem existe mesmo se a IA falhar), emite o realtime e
só então decide, pela config do banco, se dispara o n8n.

---

## 5. A costura (o coração do port)

O front chama `apiFetch('/conversations')` e espera um shape exato. O Omni fala
`/v1/*` com `Bearer` + `X-Account-Id`. **A adaptação inteira mora em ~6 arquivos de
costura** — não nos 78.

### 5.1 Onde o módulo é copiado

**Recomendado: direto em `web/app/`, espelhando os caminhos do legado**
(`app/composables/omnichannel/`, `app/components/omnichannel/`, `app/pages/omnichannel/`).

Motivo: dentro de um layer, `~` resolve para `web/app` (confirmado em
`web/layers/finance/AGENT.md:48-53`), então `import ... from "~/composables/useApi"`
**quebraria** num layer e exigiria reescrever imports nos 78 arquivos. Copiando em
`web/app/`, os caminhos do legado resolvem sozinhos e o verbatim se sustenta.

Precedente: o **calendário não é layer** — vive inteiro em `web/app/`. Mover para
`web/layers/omnichannel/` é refactor de F9.

### 5.2 Os arquivos de costura (novos, em `web/app/`)

| Arquivo | Substitui | O que faz |
|---|---|---|
| `composables/useApi.ts` | `useApi` do legado (prefixo `/api/bff`) | Expõe `apiFetch` + `ApiClientError`; mapeia `/conversations` → `/v1/omnichannel/conversations` via `createApiRequest`. **Não** passar `X-Account-Id` na mão (provider global cuida). |
| `composables/useAdminSession.ts` | idem | Mapeia `user/coreUser/token/coreToken/legacyRole/tenantSlug/logout/syncSessionFromToken` para `useAuthStore` + `useCoreAccountStore`. |
| `composables/usePageBootstrapLoading.ts` | idem | 1 uso; ligar no loading global existente. |
| `stores/session-simulation.ts` | idem | Mapeia para `platformView`/conta ativa do `useCoreAccountStore`. |
| `stores/ui.ts` | idem | 1 uso (`OmnichannelWhatsAppSessionModal.vue:14`). |
| `types/index.ts` | barrel `~/types` | Reexporta os ~25 tipos do módulo (Message, Conversation, Contact, WhatsApp*…). |

Os 6 são **adaptadores temporários** e entram no `docs/LEGADO.md` com alvo de remoção
em F9 (princípio 4).

### 5.3 Os 5 arquivos que NÃO vêm verbatim

Honestidade sobre o "vírgula por vírgula": dos 78, **71 vêm byte a byte, 5 mudam** (abaixo) e
**2 não vêm** — o código morto do audit, **D4 DECIDIDA: fora** por **D-F** (2026-07-17, §6).
Os 5 não há como evitar:

| Arquivo | Por quê |
|---|---|
| `useOmnichannelInboxRealtime.ts` | Usa **socket.io** (`io(apiBase, {auth:{token,tenantSlug,clientId}})`). O Go fala WS nativo com ticket. Bridgear socket.io em Go significaria implementar o protocolo dele — não vale. Reescrito sobre `useRealtimeSocket` + `POST /v1/ws/ticket`, **preservando os 3 eventos e os payloads**. |
| `useInboxChatGifAssets.ts` | Chama `/api/gif/search` e `/api/gif/media` (rotas Nitro). O `web/` **não tem Nitro** (BFF eliminado 2026-07-02, ADR 0002). Repontar para `/v1/omnichannel/gif/*`. |
| `useAvatarProxy.ts` | Chama `/api/avatar/whatsapp` (Nitro). Repontar para `/v1/omnichannel/avatar`. |
| `useInboxChatMediaActions.ts` | Monta URL literal `/api/bff/conversations/.../media` (`:408`). Repontar. |
| `useOmnichannelInboxOutboundPipeline.ts` | Faz `fetch` direto no `:4000` com headers na mão (`:252`), bypassando o BFF. Remover o bypass e usar só o `apiFetch` (o fallback já existe na `:270`). |

Nenhum desses 5 muda **comportamento** — só para onde apontam.

---

## 6. Decisões do port (todas resolvidas — nenhuma trava F0)

> **Nenhuma decisão desta seção segue aberta:** **D1 SUPERADA** por **D-A** (multi-provider,
> 2026-07-16) · **D2 mantida** · **D3 fora de escopo** (independência) · **D4 DECIDIDA: fora**
> por **D-F** (2026-07-17). O texto de cada uma fica como registro do racional.

### D1 — Provider de WhatsApp: Evolution ou WAHA? (a mais importante)

> #### REVISADA E SUPERADA — 2026-07-16 (decisão do dono)
>
> **A pergunta "Evolution **ou** WAHA?" estava errada: a resposta é *os dois, mais a Meta*.**
> O dono decidiu **adapter MULTI-PROVIDER** — `meta_whatsapp_cloud` (oficial) + `evolution` /
> `waha` (não-oficial) + `mock`, com **escolha por conta/número**.
>
> - **O texto abaixo fica** — é o registro do racional que levou à recomendação, e a análise
>   de multi-instância do Evolution × WAHA Core continua correta e é o que sustenta o
>   Evolution como **primeiro adapter real a ser implementado**.
> - **O que muda:** o port **não decide mais o provider**. Cravar um provider força a escolha
>   errada para metade da base (conta séria quer o número oficial, sem risco de ban; conta
>   pequena/piloto quer o não-oficial, sem app review nem custo por conversa). Um adapter
>   custa uma interface; um provider errado custa uma migração.
> - **Tony NÃO migra agora.** Segue no WAHA, dentro do `automation/`. O custo previsto abaixo
>   ("migrar `workflow-whatsapp.json` de WAHA para Evolution") **está cancelado**.
> - **"Decisão do usuário necessária — trava F3 e F8" está RESOLVIDO.** Nada mais trava aqui.
>
> **Canônico:** [`PLANO_ATENDIMENTO.md`](PLANO_ATENDIMENTO.md) §2 (**D-A**) — a interface
> `ChannelProvider` está na §5.4 e a ordem dos adapters na §9 (`mock` + `evolution` na F4;
> `meta_whatsapp_cloud` na F11).

| | Evolution API (legado usa) | WAHA (automation/ usa hoje) |
|---|---|---|
| Multi-instância por conta | **Sim, nativo** | **Não** no Core — só sessão `default`, 1 número para todas as contas. Multi-número exige WAHA **Plus** (pago) |
| Casa com o front do port | **Sim** (o front tem gestão de instâncias inteira) | Não — `automation.channels.session_name` é UNIQUE global |
| Impacto | Novo provider no stack; o workflow n8n atual (WAHA) precisa migrar | Zero impacto no bot atual; mas a gestão de instâncias do front fica órfã |

**Recomendação: Evolution API.** O front portado tem gestão de múltiplas instâncias
(criar, renomear, responsável, default, escopo por usuário) que **só existe** porque o
Evolution suporta. Com WAHA Core, metade da tela do admin não teria o que fazer, e
multi-tenant de verdade (um número por cliente) fica bloqueado atrás do WAHA Plus.

Custo real: migrar `workflow-whatsapp.json` de WAHA para Evolution. É trabalho, mas é
trabalho que o n8n-sem-lógica já pede de qualquer forma.

**Decisão do usuário necessária — trava F3 e F8.**

### D2 — Armazenamento de mídia

O legado guarda **base64 na coluna do Postgres** (`Message.mediaUrl`, data URLs de até
60 MB; body limit de 200 MB). Isso é insustentável e fere os pilares de performance.

O front **só consome o endpoint** `GET /conversations/{cid}/messages/{mid}/media` — o
armazenamento é invisível para ele. Ou seja: dá para portar o **contrato** e trocar o
**storage** sem tocar no front.

**Recomendação:** volume em disco (path na coluna) + endpoint que faz stream, com
suporte a `Range`. Sem base64 no banco. Decidir agora porque é schema.

### D3 — `messaging.*` × `automation.*` (evitar duas fontes de verdade)

> #### SUPERADA / FORA DE ESCOPO — 2026-07-16 (regra de independência do dono)
>
> O módulo `omnichannel` é **independente**: não lê, não escreve e não depende de
> `automation.*`. A divisão proposta abaixo **não se aplica** — o `messaging.*` nasce
> completo, e o que existe em `automation.*` é irrelevante para este módulo. Nada aqui
> deprecia, migra ou remove tabela de outro módulo. O texto abaixo fica como registro
> histórico do que se propôs.

Hoje já existem `automation.messages` e `automation.contacts` — **vazias na prática**
(nenhum workflow n8n chama `POST /v1/runtime/automation/messages`; o Go não tem função
de leitura). Se o omnichannel criar `messaging.messages`, ficam **duas** tabelas de
mensagem = duas verdades = fere o princípio 1.

### D4 — `OmnichannelAuditModule.vue` + `useOmnichannelAudit.ts` são código morto

> #### DECIDIDA: FORA — 2026-07-17 (decisão do dono)
>
> **`OmnichannelAuditModule.vue` + `useOmnichannelAudit.ts` NÃO são portados** (**D-F**;
> canônico [`PLANO_ATENDIMENTO.md`](PLANO_ATENDIMENTO.md) §2). **Deixou de ser decisão
> pendente e deixou de travar F0/F1** — não há mais pergunta ao dono aqui.
>
> - **Racional:** nunca renderizam, **nem no legado**. Não é remover funcionalidade
>   (princípio 3): é **não importar código inalcançável**.
> - **Bônus:** `useOmnichannelAudit.ts` era a única dependência de
>   `~/components/docs/ProjectDocsModule.vue` — o arraste some junto.
> - **Contagem:** os arquivos que vêm byte a byte **caem 2** — **71 verbatim** (era 73)
>   + 5 repontados. A conta final por fase (**67 byte a byte / 72 copiados**, já sem os 4
>   redirects que a `SPECS_PORT_OMNICHANNEL.md` F1.1 manda não copiar) está em
>   `specs/OMNI-F1.md` **C1** — mesma base, não duplicar aqui.

As páginas `auditoria.vue` e `docs.vue` redirecionam para fora do módulo — o componente
**nunca renderiza**, nem no legado. Ele é a única dependência de
`~/components/docs/ProjectDocsModule.vue`.

---

## 7. Schema proposto (`messaging.*`)

Espelha o Prisma do legado, em convenção Omni (snake_case, `account_id` em tudo,
migrations idempotentes e schema-qualificadas).

| Tabela | Origem | Notas |
|---|---|---|
| `messaging.whatsapp_instances` | `WhatsAppInstance` | `UNIQUE(account_id, instance_name)` |
| `messaging.conversations` | `Conversation` | **`UNIQUE(account_id, external_id, channel, instance_scope_key)`** — chave de dedupe |
| `messaging.messages` | `Message` | índices `(account_id, created_at)`, `(conversation_id, created_at)` |
| `messaging.contacts` | `Contact` | `UNIQUE(account_id, phone)` |
| `messaging.saved_stickers` | `SavedSticker` | poda FIFO acima de 200/conta |
| `messaging.audit_events` | `AuditEvent` | |
| `messaging.hidden_messages` | `HiddenMessageForUser` | "apagar para mim"; `UNIQUE(user_id, message_id)` |
| `messaging.account_config` | `AtendimentoTenantConfig` | `retention_days`, `max_upload_mb` |

**Mapeamentos obrigatórios:**
- `tenantId` (core do legado) → **`account_id`** (`core.accounts`). O legado resolve
  tenant por `x-selected-tenant-slug`/`x-client-id`; o Omni resolve por `X-Account-Id`
  no Principal. **`account_id` nunca vem do body** (princípio 2).
- `instanceScopeKey` = o `instance_name` (não o id). É a chave real de particionamento.
- Enums viram tipos Go + `CHECK`: `channel`, `status`, `direction`, `message_type`,
  `message_status`.

**Atenção:** `MessageStatus` no legado é só `PENDING|SENT|FAILED`. **Não existe
DELIVERED nem READ** — não há tracking de ACK. Se quisermos, é feature nova (F9), não port.

---

## 8. Mapa de rotas (Node → Go)

Prefixo Go: `/v1/omnichannel`. O shim do front adiciona o prefixo — por isso os 78
arquivos não sabem que ele existe.

| Node (legado) | Go (novo) | Fase |
|---|---|---|
| `GET /conversations` | `GET /v1/omnichannel/conversations` | F2 |
| `GET /conversations/{id}/messages` | idem | F2 |
| `GET /conversations/{cid}/messages/{mid}` | idem | F2 |
| `GET /contacts` · `POST /contacts` · `PATCH /contacts/{id}` | idem | F2 |
| `GET /tenant` · `PATCH /tenant` | `GET|PATCH /v1/omnichannel/account` | F2 |
| `GET /tenant/whatsapp/instances` (+ access) | `/v1/omnichannel/whatsapp/instances` | F2 |
| `POST /tenant/whatsapp/bootstrap|connect|logout` | idem | F3 |
| `GET /tenant/whatsapp/status|qrcode` | idem | F3 |
| `POST /webhooks/evolution/{slug}` | `POST /v1/webhooks/evolution/{accountSlug}` | F3 |
| *(socket.io)* | `GET /v1/realtime/omnichannel` | F4 |
| `POST /conversations/{id}/messages` | idem | F5 |
| `GET /conversations/{cid}/messages/{mid}/media` | idem | F5 |
| `POST .../reaction` · `/forward` · `/delete-for-me` · `/delete-for-all` | idem | F6 |
| `PATCH /conversations/{id}/status` · `/assign` | idem | F6 |
| `GET /conversations/{id}/group-participants` | idem | F6 |
| `POST /conversations/sync-open` · `.../sync-history` | idem | F6 |
| `GET|POST|DELETE /stickers` | idem | F7 |
| `/api/gif/search` · `/api/gif/media` (Nitro) | `/v1/omnichannel/gif/*` | F7 |
| `/api/avatar/whatsapp` (Nitro) | `/v1/omnichannel/avatar` | F7 |
| `GET /tenant/metrics/failures|http-endpoints` | — | **Não portar** (ver §10) |
| `/users` · `/clients` · `/session/context` | — | **Não portar** — o Omni já tem |
| `/api/admin/containers` | — | **Nunca portar** — sem auth, roda `execSync` de docker |

### Contratos que o Go tem que respeitar ao pé da letra

1. **Paginação de mensagens é `limit` + `beforeId`, não cursor.** `limit` 1..200
   (default 100). Ordena `created_at DESC`, aplica `take`, **inverte o array** (devolve ASC).
   `hasMore` = existe mensagem mais antiga que a primeira.
2. **`GET /conversations` não pagina** e ordena por `last_message_at DESC`.
3. **Três shapes distintos de `message.updated`** e **dois de `conversation.updated`**
   (o do webhook não tem `instanceName`/`instanceDisplayName`; o de `mapConversation`
   tem). Replicar por call-site — unificar quebra o front.
4. **Data URL vira `null` no realtime** (`sanitizeMediaUrlForRealtime`) — o front busca
   pelo endpoint `/media`. Não trafegar base64 no WS.
5. **Fora de escopo → 404, nunca 403** (princípio 2, enumeration).

### Bug do legado a NÃO portar

`whatsapp-instances.ts:681-683`: o ternário
`isTenantAdmin || activeInstances.length <= 1 ? activeInstances : activeInstances`
retorna o mesmo nos dois ramos — **o filtro de instância por usuário é inoperante hoje**
(todo usuário vê todas as instâncias). Portar o comportamento **corrigido** e avisar,
porque isso é isolamento (princípio 2).

---

## 9. Fases

> **NUMERAÇÃO ANTIGA — 2026-07-16.** As fases desta seção (**OMNI-F0..F9**) foram
> **substituídas** pelas **F0..F14** da fusão. O **mapa de renumeração completo** (OMNI-F0..F9
> → F0..F14, com o que mudou em cada uma) está em
> **[`PLANO_ATENDIMENTO.md`](PLANO_ATENDIMENTO.md) §9.1** — não é repetido aqui, porque
> duplicar tabela de fases é criar duas verdades (princípio 1).
>
> Ao ler qualquer "F*n*" **nesta seção e nas §11/§12**, traduza pelo §9.1 antes de agir. Os
> desvios que mais confundem: **F3 → F4** (vira `ChannelProvider`, não Evolution cravado),
> **F8 → F9** (a IA vai para o **Go**, não para o n8n), **F9 → F14** (refactor).
> A fila de saída descrita abaixo sai do módulo e vira **`platform/jobs`** na **F3 nova**
> (canônico §8) — a classificação de retry e o monitor com filtro de conta seguem valendo,
> só mudam de casa.

Cada fase tem entregável verificável **no browser**, não só compilando.

| # | Fase | Entrega | Verificável |
|---|---|---|---|
| **F0** | Decisões + fundação | D1..D4 resolvidas; `docs/LEGADO.md`; roadmap | Decisões registradas |
| **F1** | Front verbatim + costura | 71 arqs verbatim (**D-F**: −2) + 5 repontados + 6 de costura; rota/nav/workspace/permissão/módulo; remove demo | `/omnichannel` abre e **parece** o legado. Requests 404 — **badge "MOCK/SEM BACKEND"** visível pra admin |
| **F2** | Go: schema + leitura | migrations `messaging.*`; GET conversations/messages/contacts/account/instances | Inbox lista de verdade (vazio, mas real) |
| **F3** | Go: Evolution + inbound | client Evolution; bootstrap/connect/QR/status/logout; webhook + idempotência | Ler QR no painel, conectar, **mandar msg do celular e ela aparecer** (após refresh) |
| **F4** | Realtime | `/v1/realtime/omnichannel`; reescrita do `useOmnichannelInboxRealtime` | Mensagem aparece **ao vivo**, sem refresh |
| **F5** | Envio + mídia | POST messages; outbox + worker (retry/backoff); `/media` com stream | **Responder pelo painel** e chegar no celular |
| **F6** | Ações | reaction, forward, delete-for-me/all, status, assign, group-participants, sync | Cada ação da UI funciona |
| **F7** | Stickers/GIF/avatar | endpoints Go que substituem o Nitro; chave Tenor no painel | Sticker/GIF/emoji funcionam |
| **F8** | IA no n8n | config no painel; `/v1/runtime/omnichannel/*`; workflow sem lógica; handover | Bot responde; **trocar prompt no painel muda a resposta**; export/import do workflow não perde nada |
| **F9** | Refactor | split >450 linhas; remover costura; `web/app` → layer; DELIVERED/READ | Lint verde; sem adaptadores |

**Fila de saída (F5):** o legado usa BullMQ/Redis. Proposta: **tabela outbox + worker
goroutine** com retry/backoff — sem dep nova, e o estado fica no banco (princípio 1).
Replicar a classificação de retry do legado (transitório → 5 tentativas; 401/403/404 →
1; 429 → 5; 5xx → 4) e o monitor de PENDING presas (>10 min), **com filtro de conta**
(o legado varre a tabela inteira, sem tenant).

**Paralelização:** F2 e F3 podem correr junto (schema/leitura × Evolution/webhook) se
o schema de F2 for mergeado primeiro. F1 é independente das duas e pode começar já.

---

## 10. Divergências deliberadas

O que **não** vem igual, e por quê:

| Legado | Omni | Motivo |
|---|---|---|
| BFF Nitro (`/api/bff`) | Go direto | BFF eliminado 2026-07-02 (ADR 0002) — não reintroduzir |
| socket.io | WS nativo + ticket | Padrão da casa; `POST /v1/ws/ticket` |
| Auth delegada ao core por HTTP a **cada request** (`GET /me`, sem cache) | JWT local do Omni | O legado faz N+1 de rede por request. O Omni já resolve isso |
| `tenantId` + slug/clientId | `account_id` + `X-Account-Id` | Modelo do Omni |
| base64 no Postgres | disco + stream (D2) | Performance |
| Métricas HTTP in-memory | — | Não sobrevive a restart nem a réplica; o Omni tem observabilidade própria |
| `/users`, `/clients`, `/session/context` | — | O Omni já tem, melhor |
| Filtro de instância inoperante | Corrigido | Isolamento (princípio 2) |

---

## 11. Legado/mock a marcar (princípio 4)

Entra em `docs/LEGADO.md` no F1, com badge admin visível na tela:

1. **F1 = front sem backend.** A tela existe e não carrega nada. Badge
   "SEM BACKEND — F1" enquanto F2/F3 não fecharem. Ninguém pode achar que está pronto.
2. **Os 6 arquivos de costura** — adaptadores, alvo de remoção em F9.
3. **Arquivos >450 linhas** — violação consciente, alvo F9.
4. **Módulo em `web/app/` em vez de layer** — alvo F9.

---

## 12. Notas de Deploy

Registrar aqui **antes** de subir (regra: mudanças que afetam deploy são anotadas com
ordem exata).

**Ordem:** migrations → env vars → build api → build web → import n8n.

| Item | Fase | Detalhe |
|---|---|---|
| Migrations `messaging.*` | F2 | Numerar a partir da última. **Migration nova exige `docker compose build --no-cache api`** — o cache do `go build` não re-embute o `.sql` (embed.FS) |
| `EVOLUTION_BASE_URL` | F3 | Se D1 = Evolution. Sem ela, o worker marca SENT com `provider: mock-local` |
| `EVOLUTION_API_KEY` | F3 | Global do ambiente no legado. **Avaliar por-conta no Omni** (multi-tenant) |
| `EVOLUTION_WEBHOOK_TOKEN` | F3 | Valida o header `x-webhook-token` (timingSafeEqual) |
| `WEBHOOK_RECEIVER_BASE_URL` | F3 | URL que o Evolution chama de volta |
| Container `evolution-api` | F3 | Novo serviço no compose (profile próprio) + Caddy se precisar de rota pública |
| Volume de mídia | F5 | Se D2 = disco. Backup precisa incluir |
| Chave Tenor | F7 | **No painel/banco**, não em env (regra: config vem do painel) |
| Re-import n8n | F8 | `npm run n8n:import` — o n8n roda do banco, não do arquivo |
| Rebuild api | F2..F8 | Toda fase que mexe em `back/` → `docker compose up -d --build api` |

---

## 13. Riscos

1. ~~**D1 (Evolution × WAHA) trava metade do plano.** Sem decisão, F3/F8 não começam.~~
   **RESOLVIDO 2026-07-16** — D1 decidida como **multi-provider** (ver §6). Não trava mais
   nada. Os riscos que a decisão *introduziu* (pricing/janela 24h da Meta, ban do
   não-oficial, degradação por `Capabilities()`) estão em
   [`PLANO_ATENDIMENTO.md`](PLANO_ATENDIMENTO.md) §12.
2. **O Evolution é um container novo em produção.** Não é só código — é infra, backup,
   Caddy e um número de WhatsApp real. Testar em staging antes.
3. **O webhook é rota pública.** Herdar do legado as proteções: rate-limit por
   slug+IP, `x-webhook-token`, allowlist de content-type, limite de body,
   idempotência em Redis. Rota pública sem isso é incidente.
4. **Multi-tenant é o risco de segurança principal.** O legado resolve tenant de um
   jeito que o Omni não usa. Todo repositório filtra por `account_id` também
   (defesa em profundidade), e fora de escopo é 404.
5. **F1 entrega uma tela que não funciona.** Sem o badge, vira dívida invisível.
6. **22.642 linhas de código que ninguém aqui escreveu.** O `git log` do `whats-test`
   é o único lugar que explica o porquê das decisões. Não perder essa pasta.

---

## 14. O que este plano NÃO cobre

- Instagram/Email/Webchat (o roadmap previa; o legado só tem WhatsApp + enum `INSTAGRAM`
  não implementado).
- `DELIVERED`/`READ` — não existe no legado, é feature nova.
- Roteamento por fila / distribuição automática — o legado só tem `assign` manual.
- **Integração com outros módulos** (incl. `automation`) — o módulo é independente e não
  toca em tabela alheia. Quando ele fechar, se for preciso integrar, vira plano próprio.
