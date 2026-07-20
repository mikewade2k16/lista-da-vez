# OMNI-F12 — Stickers / GIF / avatar (substituir o Nitro) · **P1**

Plano canônico: `docs/omnichannel/PLANO_ATENDIMENTO.md` (§9 F12, §9.2 F12, §9.1 mapa port→fusão).
Anexo técnico do front: `docs/omnichannel/SPECS_PORT_OMNICHANNEL.md` F7 · `PLANO_PORT_OMNICHANNEL.md` §8 (mapa de rotas).
Ler a skill `principios-engenharia` antes de escrever.

> ## LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono)
> A branch `refactor/multi-tenant-complete` fechou e o dono **liberou a implementação em
> 2026-07-17** (decisão **D-D**, canônico §2). O aviso de congelamento que constava aqui não vale mais.

> ## ATENÇÃO — o anexo F7 **não** é contrato campo a campo
> `SPECS_PORT_OMNICHANNEL.md` F7 tem **8 linhas** e fixa só limites e status. Os shapes desta
> spec foram **derivados do código de referência no disco** (`web-reference/`), citado linha a
> linha. Onde o anexo fala, ele manda; o resto está aqui porque **não existia em lugar nenhum**.
> Ver *Divergências*, item 1.

---

## Objetivo

As 3 rotas de BFF Nitro do legado viram endpoints Go: o `web/` **não tem Nitro** (BFF eliminado
2026-07-02, ADR 0002) e não vai ter. Quando a F12 fecha, o atendente salva/usa/apaga figurinha,
busca e anexa GIF, e o avatar do contato carrega — com a **chave do Tenor vinda do painel/banco,
nunca de env** (princípio 1), e o anti-SSRF **reusado** da F6, não reimplementado.

## Depende de / Bloqueia

| | |
|---|---|
| **Depende de** | **F6** — `.../omnichannel/ssrf.go` (guarda anti-SSRF; `OMNI-F6.md` Entrega 6 e C5, que já declara "F12 reusa") e `.../omnichannel/media_storage.go` (C3). `OMNI-F6.md:31` já lista: "**Bloqueia** … F12 (stickers reusam a allowlist e o storage)" |
| **Depende de (indireto)** | **F2** — a tabela `messaging.saved_stickers` é da migration da F2 (`OMNI-F2.md` Entrega 1: "8 tabelas do port"), e `messaging.account_config.max_upload_mb` idem (port §7). **F12 não cria nenhuma das duas** · **F3** — `platform/secretbox` para a chave do Tenor |
| **Bloqueia** | Nada. É P1, fora do piloto P0 (canônico §9) |

**Regra de fronteira:** se, ao executar a F12, faltar `ssrf.go` (F6), `media_storage.go` (F6) ou
`messaging.saved_stickers` (F2) — **parar e escalar**. Não fazer um segundo guarda de SSRF nem um
segundo storage dentro do módulo. Duas allowlists = duas verdades (princípio 1).

## Entregas

| # | Entrega | Alvo |
|---|---|---|
| 1 | `GET \| POST \| DELETE /v1/omnichannel/stickers` | `.../omnichannel/http_stickers.go` |
| 2 | Service de stickers: validação, storage via F6, poda FIFO | `.../omnichannel/service_stickers.go` |
| 3 | Repositório de stickers (filtra por `account_id` **também**) | `.../omnichannel/store_postgres_stickers.go` |
| 4 | `GET /v1/omnichannel/gif/search` + `/gif/media` | `.../omnichannel/http_gif.go` |
| 5 | Client Tenor (mapeamento `media_formats` → shape do front) | `.../omnichannel/gif_tenor.go` |
| 6 | Chave do Tenor no banco + `GET \| PUT` de status mascarado | `.../omnichannel/{secrets_gif.go,store_secrets_gif.go}` |
| 7 | Proxy de avatar (**ver C4 — bloqueio aberto**) | `.../omnichannel/http_avatar.go` |
| 8 | Front: os 3 consumidores passam pelo client autenticado (C6) | `web/app/composables/omnichannel/` |
| 9 | Migration **aditiva** — só se a F2 não tiver nascido com `storage_key` (C1) | `back/internal/platform/database/migrations/` |
| 10 | `AGENT.md` do módulo: rotas, chave do Tenor, storage de sticker | `.../omnichannel/AGENT.md` |

Teto de ~450 linhas/arquivo **vale** (código novo).

---

## Contratos

### C1 · Stickers — o front **exige data URL** (armadilha silenciosa)

Consumidor: `web-reference/app/composables/omnichannel/useInboxChatStickerAssets.ts`.

| Rota | Permissão | Contrato |
|---|---|---|
| `GET /stickers?limit=` | `omnichannel.conversations.view` | `limit` **1..200, default 36** (anexo F7). **Array direto, não envelopado** (`:54`). Ordem: `created_at desc` |
| `POST /stickers` | `omnichannel.conversations.reply` | Body `{name, dataUrl, mimeType, sizeBytes}` (`:138-146`) → **201** com o objeto criado |
| `DELETE /stickers/{id}` | `omnichannel.conversations.reply` | **204**. Sticker de outra conta → **404** |

*Permissões são decisão desta spec* (o canônico §5.2 não tem key de sticker): ver é parte de ver o
inbox; salvar/apagar é parte de compor resposta. Se o dono quiser key própria, muda **aqui**.

**Shape do item** (`:39-48` — defaults do front entre parênteses, o back devolve o real):

```
{ id, name ("figurinha.webp"), dataUrl, mimeType ("image/webp"),
  sizeBytes (0), createdAt, updatedAt?, createdByUserId }
```

> **O front DESCARTA em silêncio o que não for data URL.**
> `normalizeSavedStickerItem` (`:33-36`) faz `if (!id || !dataUrl.startsWith("data:image/")) return null`
> — e `dataUrlToStickerFile` (`:103`) casa `^data:([^;,]+);base64,(.+)$`. Devolver
> `"/v1/omnichannel/stickers/{id}/media"` em `dataUrl` **não dá erro**: a figurinha **some da
> grade** e ninguém sabe por quê. **`dataUrl` na resposta é data URL base64 real, sempre.**

**Storage:** bytes em disco via `media_storage.go` da F6 (C3: raiz privada
`OMNICHANNEL_MEDIA_DIR`, `MkdirAll 0o750`, arquivo `0o600`, sniff de mime + extensão casando).
Sticker **não tem conversa**, então o path da F6 (`{root}/{accountId}/{conversationId}/…`) vira
`{root}/{accountId}/stickers/{random}.{ext}`. A coluna guarda o **path relativo**
(`storage_key`), **nunca serializado no JSON** — o `dataUrl` é remontado na serialização.

- **Se a `messaging.saved_stickers` da F2 nasceu com `data_url text`** (base64 no Postgres, como o
  legado Prisma que a D-B rejeita): migration **aditiva** desta fase adiciona `storage_key text`,
  SQL plano idempotente (`add column if not exists`), schema-qualificado, **sem `-- +goose Down`**.
  **Numerar conferindo o disco** — a última é `0199_calendar_drop_day_media.sql`, há **dois `0197`**
  (`0197_operation_validation_reason.sql`, `0197_tools_module.sql`), e F2/F3/F6 já terão consumido
  0200+. **Não cravar número.**
- **Validação:** allowlist `image/webp|png|jpeg|jpg|gif` → **415**; tamanho decodificado
  `min(messaging.account_config.max_upload_mb, 20MB)` → **413** (anexo F7 + F6 C3 — o teto é de
  `messaging.account_config`, **não** de `core.account_modules.config`). Ambos com
  `{message, code, details}` (`SPECS_PORT_OMNICHANNEL.md` F5 §1).
- **Decodificar como a F6 (C3):** `MaxBytesReader` = teto × 4/3 + folga; `io.Copy(file, base64.NewDecoder(...))`.
  **Nunca** materializar o arquivo inteiro em `[]byte`.
- **Poda FIFO > 200/conta** (anexo F7): após o insert, apagar os mais antigos além de 200 —
  **linha e arquivo de disco juntos**. Apagar só a linha deixa órfão que ninguém coleta (o purge da
  F13 não conhece sticker).

### C2 · `GET /gif/search` — erro é **soft**, HTTP 200

Consumidor: `useInboxChatGifAssets.ts:49-57`. Permissão: `omnichannel.conversations.reply`.

| Item | Contrato (verificado em `web-reference/server/api/gif/search.get.ts`) |
|---|---|
| Query | `q` · `limit` = `min(40, max(1, limit ?? 24))` → **1..40, default 24**. **Não é o 1..200/36 do sticker** |
| Resposta | `{provider, query, items[], error?}` — `error` **omitido** quando não há |
| Item | `{id, title, previewUrl, mediaUrl, mimeType, sourcePageUrl}`. `mimeType` = `"video/mp4"` se houver mp4, senão `"image/gif"`; `title` = `content_description` ou `"GIF"` |
| Preferência | mp4 (`mp4→tinymp4→nanomp4→loopedmp4`) antes de gif (`gif→tinygif→nanogif→mediumgif`); `previewUrl` = `tinygif→nanogif→gifpreview→gif→mp4`. Sem `mediaUrl` → item descartado |
| `q` vazio | `{provider, query:"", items:[]}` — **sem chamar o Tenor** |
| Sem chave / provider ≠ tenor / upstream falhou | **200** + `items:[]` + `error` legível |
| Upstream | `{base}/search?q=&key=&limit=&media_filter=gif,tinygif,mp4,tinymp4,nanomp4,nanogif&locale=pt_BR`, base default `https://tenor.googleapis.com/v2` |

> **Não “consertar” para 4xx/5xx.** O front lê `response.error` (`:56`) e mostra a mensagem. Um
> status de erro faz o `$fetch` **lançar**, o `catch` (`:59`) troca tudo por "Nao foi possivel
> consultar GIFs." e a mensagem acionável ("chave não configurada") **se perde** — o oposto do
> princípio 5. Falha de rede/timeout continua sendo exceção; **falha de configuração é 200 + `error`.**

**Timeout obrigatório** (10s, padrão `calendar/ai_models.go:29`) e **User-Agent próprio**: o default
do Go é barrado por WAF (registro de falhas nº7).

### C3 · `GET /gif/media?url=` — proxy do Tenor

Consumidor: `useInboxChatGifAssets.ts:84-89` (`fetch` → `.blob()`). Permissão: `omnichannel.conversations.reply`.

| Regra | Detalhe |
|---|---|
| Allowlist de host | `media.tenor.com` · `c.tenor.com` · `tenor.googleapis.com` · `www.tenor.com` (`gif/media.get.ts`) |
| Fora da allowlist / URL inválida | **400** com `{message, code, details}` |
| Anti-SSRF | Guarda da **F6 (`ssrf.go`, C5)** — IP resolvido, não hostname; `net.Dialer.Control`; **não seguir redirect** para destino não validado. Allowlist de host é **além** do guarda, não no lugar dele |
| Upstream != 2xx | **502** |
| Timeout | 10s. **O legado não tem timeout aqui** (só o avatar tem) — não portar a omissão |
| Cache | `public, max-age=3600` (conteúdo público de terceiro, não é dado de conversa) |
| Stream | `io.Copy` da resposta upstream. O legado faz `Buffer.from(await response.arrayBuffer())` inteiro — mesma dívida que a D2 elimina na F6 |

### C4 · `GET /avatar?url=` — **BLOQUEIO ABERTO: `<img src>` não manda header**

> **Esta é a decisão que a F12 não pode tomar sozinha.** O resto da spec não depende dela.

`resolveAvatarSource` (`useAvatarProxy.ts:8-31`) devolve **string de URL** que vai direto no `:src`
de `<img>`/`<UAvatar>` em **8+ call-sites verificados** (`InboxChatHeader.vue:47`,
`InboxConversationsSidebar.vue:230,242,258,806`, `InboxDetailsSidebar.vue:195`,
`InboxChatComposerInput.vue:68`, `InboxChatComposerAttachmentMenu.vue:99`,
`InboxMessageContactCard.vue:28`, `InboxAudioMessagePlayer.vue:221`,
`useInboxChatMessageIdentity.ts:419,424,434`).

**O navegador carrega isso como imagem: sem `Authorization`, sem `X-Account-Id`.** Sob
`RequireAuth` + `moduleGatingRules` (`app.go:518`, prefixo `/v1/omnichannel`) **todo avatar 401**.
Não é hipótese — é a diferença entre este endpoint e o `/media` da F6, que o front busca com
`fetch` + `Authorization` → `blob` → `objectURL` (`useInboxChatMediaActions.ts:411-440`).
**O Nitro do legado não tinha auth nenhuma** (`avatar/whatsapp.get.ts` — zero checagem).

| # | Opção | Consequência |
|---|---|---|
| **A** *(recomendada)* | Rota **pública** `GET /v1/public/omnichannel/avatar?url=` — fora do gate, sem JWT, allowlist dos 4 hosts + rate-limit por IP | Espelha o legado e o precedente **confirmado** de rota fora do gate (canônico §11: `/v1/public/*` de bio/cardápio, `/s/{slug}`, `/q/{slug}`). **Exposição real ≈ zero:** não há dado de conta na resposta e quem chama **já tem** a URL do CDN — abrir a URL direto no browser dá o mesmo. Custo honesto: vira proxy aberto **para esses 4 hosts** (banda) |
| **B** | Ticket curto na query (padrão `POST /v1/ws/ticket`, `realtime/http.go:10`; `?ticket=`, `tickets.go:118`) | Ticket **em URL** vaza em log/Referer; TTL de **30s** (`tickets.go:17`) briga com cache de imagem; e `resolveAvatarSource` é **síncrona e pura** — teria de receber o ticket, o que já não é "repontar" |
| **C** | `fetch`+`blob`+`objectURL` como o `/media` | Auth completa, mas `resolveAvatarSource` vira assíncrona/reativa e é chamada **dentro de computeds** em 8+ call-sites: deixa de ser repontar e vira **reescrever o front** — fere a D-B na prática |

**Enquanto não houver decisão, a F12 entrega C1/C2/C3 e o avatar fica fora** — o front cai no
`:src` da URL crua (comportamento atual de host não-allowlisted, `useAvatarProxy.ts:25-27`).
Não implementar sob `RequireAuth` "para decidir depois": entrega 401 em toda foto.

**Contrato do proxy (vale em qualquer opção)** — de `avatar/whatsapp.get.ts`:

| Item | Regra |
|---|---|
| Allowlist | `pps.whatsapp.net` · `mmg.whatsapp.net` · `mmx.whatsapp.net` · `lookaside.whatsapp.com` (**idêntica ao front**, `useAvatarProxy.ts:1-6` — divergir = avatar que o front proxia e o back recusa) |
| Host fora da allowlist | **403** |
| Protocolo ≠ `http`/`https` | **422** — **não** o 400 do legado (F6 C5 e `SPECS_PORT_OMNICHANNEL.md` F5 §3 fixam 422; ver *Divergências* 4) |
| `url` ausente/inválida | **400** |
| Redirect | **NÃO seguir** (F6 C5). O legado usa `redirect: "follow"` com allowlist só na URL inicial = **buraco de SSRF**. Port **corrigido**, não verbatim |
| Timeout | 10s (`AbortController` no legado) |
| Upstream falhou / != 2xx / corpo vazio | **204** + `cache-control: public, max-age=120` — o front trata como "sem avatar" e cai nas iniciais |
| Sucesso | content-type do upstream (só o tipo, sem `;charset`), `cache-control: public, max-age=300, stale-while-revalidate=600` |

### C5 · Chave do Tenor — **do banco, nunca de env** (princípio 1)

O Nitro lê `NUXT_TENOR_API_KEY` do runtime config (`search.get.ts`) — **é exatamente isso que
morre aqui.** A chave é da **plataforma** (um app Tenor para o produto), não da conta:

| Item | Regra |
|---|---|
| Persistência | `core.platform_settings`, chave `'omnichannel_gif'`, `{provider, baseUrl, apiKey}`. **Precedente confirmado:** `calendar/secrets.go:11` e `store_secrets.go:54-57` usam `core.platform_settings` chave `'calendar_ai_secrets'` para keys globais. **Sem migration** — a tabela existe (`0160_core_platform_settings.sql`) |
| Cifragem | `apiKey` gravada via **`platform/secretbox` (F3)**, com prefixo `v1:`. **Não repetir o gap do calendário**, que grava a chave crua (canônico §3 e §14.5) |
| Saída | **Sempre `{set, last4}`**, nunca a chave. Modelo: `calendar/secrets.go:20-23,43` (`mask`) |
| Rotas | `GET \| PUT /v1/omnichannel/gif/settings` — **só `platform_admin`** (checado no handler). Espelha `GET \| PUT /v1/calendar/ai-keys/global` (`calendar/http_secrets.go:29-32`: `RequireAuth` + papel no handler). `apiKey` vazia = limpar |
| Sem chave | `search` → **200** + `items:[]` + `error` **acionável** apontando o painel (C2). **Nunca** citar env var |

> **Por que a rota de escrita é da F12 e não da F10:** a F10 é telas de números/setores/agente
> (canônico §9.2). Sem writer, a chave só entra por SQL e a fase não fecha ("vem do painel").
> Mesmo raciocínio, resultado oposto ao da F3.4, que **não** inventou rota porque a F10 já era dona.

### C6 · Front — os 3 consumidores não usam o client autenticado hoje

A F1 já **reponta** os 3 (canônico §9.2 F1; `PLANO_PORT_OMNICHANNEL.md:179-180`), mas reponta
**para onde**, não **como**. Com o backend no ar, o "como" passa a importar:

| Consumidor | Hoje | Precisa |
|---|---|---|
| `useInboxChatGifAssets.ts:49` | `$fetch("/api/gif/search")` — **sem auth** | `apiFetch` do módulo (injeta `Authorization` + `X-Account-Id`) |
| `useInboxChatGifAssets.ts:84` | `fetch("/api/gif/media?url=")` → `.blob()` — **sem auth** | `fetch` **com headers montados à mão** — `apiFetch` devolve JSON parseado e **não serve para blob**. Espelhar `fetchMessageMediaBlob` (`useInboxChatMediaActions.ts:411-440`) |
| `useInboxChatStickerAssets.ts:54,138,178` | já usa `apiFetch` | nada |
| `useAvatarProxy.ts` | `:src` cru | **C4** |

---

## Armadilhas / o que NÃO fazer

| Não faça | Porque |
|---|---|
| Devolver URL em `dataUrl` do sticker | O front **descarta em silêncio** o que não começa com `data:image/` (C1). Some da grade, sem erro |
| Servir sticker sob `UPLOADS_DIR` | `/uploads/` é `http.FileServer` **sem auth e fora do gate** (`app.go:241-243`; `OMNI-F6.md` Armadilha 1). Raiz privada da F6, sempre |
| Reimplementar allowlist/anti-SSRF | É da **F6 (`ssrf.go`)**. Duas allowlists divergem no primeiro host novo |
| Portar `redirect: "follow"` do avatar | Allowlist só na URL inicial + redirect seguido = SSRF (C4) |
| Trocar o erro soft do `search` por 4xx/5xx | Apaga a mensagem acionável no `catch` do front (C2) |
| Ler a chave do Tenor de env | Princípio 1. Nem como fallback: fallback silencioso é a env vencendo o painel |
| Logar a chave, a URL com `key=`, ou o payload do Tenor | Canônico §10. A chave vai na query string do upstream — **não logar a URL montada** |
| Apagar a linha do sticker sem apagar o arquivo | Órfão em disco que nenhum purge coleta (C1) |
| Confiar no `sizeBytes` do body | Vem do cliente (`:144`). O teto é do **tamanho decodificado**, medido no servidor |
| Cravar número de migration | Dois `0197` no disco; F2/F3/F6 consomem 0200+ (C1) |
| `Cache-Control: public` em sticker | Sticker é dado de conta: `private`. `public` vale só para avatar/GIF (terceiro) |

## Segurança

| Item | Regra |
|---|---|
| Escopo | `account_id` **sempre** do Principal (`RequireAuthWithAccount`), **nunca** do body/query — em `saved_stickers` e no teto de `messaging.account_config` |
| Defesa em profundidade | Todo `select`/`delete` de sticker carrega `account_id = $1` **no repositório**, além da validação do service (princípio 2) |
| Fora de escopo | **404, nunca 403** — sticker de outra conta no `DELETE` responde 404 (403 confirma existência: enumeration) |
| Permissão ≠ escopo | Sem `conversations.reply` → **403**. Sticker de outra conta → **404** |
| SSRF | Guarda da F6 em **`/gif/media` e `/avatar`**. Allowlist de host é camada extra, não substituta |
| Segredo | Tenor via `secretbox` (F3). Nunca em coluna crua, nunca em log, nunca de volta ao front (só `{set,last4}`) |
| Upload | Allowlist de mime por **sniff**, não pelo `mimeType` declarado; extensão casa com o mime (F6 C3) |
| Rota pública (se C4=A) | Rate-limit por IP + allowlist estrita. **Nenhum** dado de conta na resposta |

## Verificável

Um humano prova no browser/banco/curl:

1. **Sticker ponta a ponta.** No inbox, salvar uma figurinha `.webp` pelo painel → aparece na grade
   **sem refresh**; recarregar a página → continua lá; mandar numa conversa → chega no celular;
   apagar → some. `select id, storage_key, length(coalesce(data_url,'')) from messaging.saved_stickers`
   → `storage_key` preenchido; **o arquivo existe** em `{OMNICHANNEL_MEDIA_DIR}/{accountId}/stickers/`.
2. **O data URL é real.** `curl -H "Authorization: Bearer <t>" -H "X-Account-Id: <a>" .../v1/omnichannel/stickers?limit=36`
   → **array direto** (não `{items:[]}`) e todo `dataUrl` começa com `data:image/`.
3. **Limites.** `POST` de um `.svg` → **415**. `POST` acima de `min(max_upload_mb, 20MB)` → **413**,
   com `{message, code, details}`. Ambos com mensagem legível na tela, não erro cru.
4. **Poda FIFO.** Inserir 205 stickers na conta → `select count(*)` = **200**, os 5 mais antigos
   sumiram **e os arquivos deles não estão mais no disco**.
5. **Isolamento.** `DELETE /stickers/{id}` de sticker da conta B com `X-Account-Id` da conta A →
   **404** (não 403). `GET /stickers` na conta B não lista os da conta A.
6. **GIF sem chave (o teste do princípio 1).** Limpar a chave (`PUT /v1/omnichannel/gif/settings`
   com `apiKey:""`) → buscar GIF no painel mostra **aviso acionável apontando o painel**, com
   **HTTP 200** no Network (não 4xx/5xx) e **sem citar env var**. Gravar a chave pelo painel →
   busca volta **sem restart do container**. `select config from core.platform_settings where key='omnichannel_gif'`
   → `apiKey` começa com **`v1:`** e **não** é legível.
7. **GIF anexa.** Buscar "tchau", clicar num resultado → vira anexo no compositor → enviar → chega
   no celular. `GET /gif/media?url=https://evil.example.com/x.gif` → **400**.
8. **Avatar** *(só se C4 decidido).* Foto do contato carrega no header e na sidebar, **sem erro no
   console** e sem 401 no Network. `GET /avatar?url=http://169.254.169.254/latest/meta-data` →
   bloqueado pelo guarda da F6. `GET /avatar?url=ftp://x` → **422**. URL de host allowlisted que
   redireciona para host interno → **não seguido**.

## Notas de Deploy

**Ordem exata:** migration (se houver) → build da api → smoke.

| # | Passo | Detalhe |
|---|---|---|
| 1 | Migration aditiva `<próximo número livre>_messaging_stickers_storage.sql` | **Só se** a F2 não nasceu com `storage_key` (C1). **Conferir o disco** — dois `0197`; última `0199`; F2/F3/F6 já consumiram 0200+. Idempotente, sem `-- +goose Down` |
| 2 | `docker compose build --no-cache api` | **Obrigatório se houver migration** — migrations são `embed.FS` e o cache da camada `go build` pode **não re-embutir** o `.sql` novo. Sintoma: `migrate status` para na anterior, **sem erro** (canônico §13) |
| 3 | `docker compose up -d --build api` | Mexeu em `back/` → rebuild; restart não basta |
| 4 | Rebuild do web | Só a Entrega 8 (C6) toca `web/` |

**Sem env var nova — e isso é o ponto da fase:** `NUXT_TENOR_API_KEY` do legado **não é portada**.
A chave entra pelo painel, cifrada (C5). `OMNICHANNEL_MEDIA_DIR` já é da F6.
**Sem container novo.** Se C4 = opção A, o avatar passa a ser rota pública: conferir se o Caddy
roteia `/v1/public/*` (já roteia — bio/cardápio usam) **antes** de anunciar a fase pronta.

---

## Divergências e questões abertas (registradas, não decididas por conta própria)

| # | Ponto | Situação | Encaminhamento |
|---|---|---|---|
| 1 | **O anexo F7 não é contrato campo a campo** | O prompt desta fase e o canônico §9.2 remetem ao anexo "campo a campo", mas `SPECS_PORT_OMNICHANNEL.md` F7 tem **8 linhas** (limites e status, só) | Shapes derivados do `web-reference/` **com citação de linha**. Não é duplicação: **não existiam** em lugar nenhum |
| 2 | **Auth do avatar** | `<img src>` não manda header; sob o gate, 401 em toda foto (C4) | **Decisão do dono.** Recomendada: **A** (rota pública). Sem decisão, o avatar **não entra** na F12 |
| 3 | **Limite do `gif/search`** | Canônico/anexo só citam `1..200 default 36` (que é do **sticker**). O Nitro clampa GIF em **1..40 default 24** (`search.get.ts`) | Spec adota o verificado no código. Se o dono quiser outro, muda aqui |
| 4 | **Protocolo inválido: 400 vs 422** | Legado do avatar → **400**; F6 C5 + `SPECS_PORT_OMNICHANNEL.md` F5 §3 → **422** | Adotado **422** (consistência com o `/media`). Divergência consciente com o legado |
| 5 | **Sticker de até 20MB vira data URL** | Teto `min(max_upload_mb, 20MB)` × `limit=200` = resposta de **até ~4GB** (base64 infla ~33%). No default (36) ainda dá ~960MB | **Recomendação: teto próprio de sticker (~1MB)** — o WhatsApp usa ~512KB. Precisa do ok do dono: o `min(...)` é do canônico/anexo. **Enquanto isso o contrato fica como está** |
| 6 | **`saved_stickers`: `data_url` vs `storage_key`** | O legado guarda base64 no Postgres (o que a D-B rejeita); a D2/F6 manda pro disco. A F2 é dona da tabela e **não detalha as colunas** | Se a F2 nascer com `data_url`, isto é **divergência da F2 com a D2 — corrigir lá**; a F12 só adiciona `storage_key` aditivamente (C1) |
