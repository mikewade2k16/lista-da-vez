# Guia do Módulo Bio

> Guia de uso do módulo **Bio** (link-in-bio) do Omni. Dois públicos: usuário
> (agência/cliente) e técnico. Doc canônico de planejamento: `PLANO_MODULO_BIO.md`.
> Iterações: `ITERACAO_B6_EDITOR.md`, `ITERACAO_B7_SLIDES_FONTE.md`,
> `ITERACAO_B8_PRODUTOS_PEROLA.md`. Atualizado em 2026-06-13.

---

# Parte 1 — Para quem vai usar (agência e cliente)

## O que é o módulo Bio

O Bio é a área do Omni onde você monta e publica uma **página de bio** — aquela
página de "link na bio" que reúne, num só lugar, os links, o vídeo de fundo, os
destaques e as lojas de uma marca. É o tipo de página que você coloca no
Instagram, no WhatsApp ou em qualquer lugar onde só cabe um link.

Você cria e edita tudo dentro do painel, vê uma **prévia ao vivo** enquanto
edita e, quando estiver pronto, clica em **Publicar**. A página fica disponível
em um endereço público no formato `seu-dominio/{slug}` (por exemplo,
`.../perola`).

Onde fica no menu: **Site → Bio**.

## Quem vê o quê

- **Cliente**: vê e edita apenas a(s) bio(s) da própria conta.
- **Agência / administrador**: vê todas as bios e pode **filtrar por cliente**,
  além de criar, duplicar e mover bios entre clientes.

## Criar uma bio

1. Abra **Site → Bio** e clique em **Criar**.
2. Preencha o **nome** (é o nome interno, para você se organizar no painel).
3. **Cliente** (só aparece para a agência): é **opcional**.
   - Se você escolher um cliente, a bio passa a pertencer àquele cliente.
   - Se deixar em branco ("Sem cliente — agência"), a bio fica na conta da
     própria agência. Você pode trocar o cliente depois, a qualquer momento.
4. **Slug** (o final do endereço público) é **opcional**. Se deixar em branco,
   o sistema gera um a partir do nome. Se o slug que você escolheu já existir,
   o painel avisa para você escolher outro.
5. Escolha como continuar:
   - **Criar**: cria a bio e volta para a lista.
   - **Criar e editar**: cria a bio e já abre o editor.

## Editar uma bio (por seções)

O editor é dividido em seções no menu lateral. Você mexe em uma de cada vez e a
prévia ao lado atualiza na hora.

- **Meta**: título da aba do navegador, favicon (o iconezinho), idioma e o ID do
  Google Tag Manager (para medição), se houver.
- **Branding**: o logo, o nome que aparece no perfil e o texto do rodapé.
- **Vídeo / Fundo**: o fundo da página. Cada campo (fundo no celular, fundo no
  computador e poster) aceita **um vídeo OU uma imagem** — você envia o arquivo
  e o sistema entende sozinho se é vídeo ou imagem. Tem também os ajustes de
  sobreposição (overlay) e alinhamento.
- **Links e menu**: os botões de link da página e o menu do topo. A **ordem da
  lista é a ordem que aparece** na página; use as setas para subir/descer cada
  item. Os blocos podem ser recolhidos para facilitar a edição quando há muitos.
- **Slides** (o carrossel de destaques no topo): aqui você escolhe duas coisas:
  - **De onde vem o conteúdo**:
    - **Imagens** (manual): você sobe as imagens uma a uma, como sempre.
    - **Produtos do site** (fonte automática): o carrossel se monta sozinho a
      partir dos produtos do cliente. Você escolhe a **categoria**, a
      **campanha**, o **tipo** e a **quantidade** (5, 10 ou todos). Pode definir
      para onde o clique no produto leva (o link do produto no site, o WhatsApp
      ou nada) e adicionar um **botão** abaixo do carrossel (por exemplo, "Ver
      toda a coleção", apontando para a categoria no site do cliente).
  - **Como exibir**: em **carrossel** (deslizando) ou em **imagem(ns)
    estática(s)**.
- **Lojas**: o localizador de lojas (endereços/pontos), com os limites do mapa e
  o lightbox.

## Prévia ao vivo e o switch "Editando / Publicado"

Enquanto você edita, a prévia ao lado mostra o resultado em tempo real. Há um
**switch** que alterna o que a prévia mostra:

- **Editando**: o rascunho atual (o que você está mexendo agora).
- **Publicado**: a versão que está no ar publicamente.

Isso serve para você comparar o que está editando com o que o público vê.

## Salvar, publicar e despublicar

- **Salvar é automático**: o rascunho é gravado sozinho enquanto você edita
  (não existe botão "Salvar"). Um indicador mostra "Salvando..." / "Salvo".
- **Publicar / Republicar**: ir ao ar é uma decisão sua, então continua manual.
  Ao publicar, o rascunho vira a versão pública. Se você já publicou e fez
  mudanças, o botão vira **Republicar** para enviar as alterações.
- **Despublicar**: tira a página do ar (o endereço público passa a não
  encontrar a bio). O rascunho continua salvo para você publicar de novo depois.

Observação: para publicar, a bio precisa ter pelo menos o **logo** e **um fundo**
(vídeo ou imagem). O sistema avisa se faltar.

## Outras ações

- **Duplicar**: cria uma cópia da bio (como rascunho, com um novo slug). Útil
  para começar uma nova a partir de uma pronta.
- **Trocar o cliente** (só agência): no topo do editor (ou na lista) você pode
  mover a bio para outro cliente.
- **Ver online**: abre a página pública em uma nova aba, no endereço
  `seu-dominio/{slug}`.

## Quando a página atualiza no ar

Depois que você **Republica**, a página pública se atualiza praticamente na
hora, sem o visitante precisar recarregar (o site escuta as atualizações em
tempo real). Enquanto a bio estiver só em rascunho, nada muda no ar.

---

# Parte 2 — Técnica

## Visão geral da arquitetura

```
  +------------------+        painel (CRUD, JWT)        +------------------+
  |   Painel Omni    |  ----------------------------->  |   API Go (back)  |
  | (web, Nuxt 3003) |   /v1/bio/...                    |  módulo `bio`    |
  +------------------+                                  |  schema bio.*    |
                                                        +---------+--------+
                                                                  |
                          público (server-to-server / browser)    |
                          GET /v1/public/bio/{slug}  +  /stream    |
                                                                  v
  +----------------------------+      consome o JSON      +------------------+
  |  Front bio (crow-nuxt)     |  <---------------------  |  API pública     |
  |  rota /{slug}, SSR + SWR   |   merge(defaults+pub)    |  (sem JWT, CORS *)|
  |  EventSource p/ live-reload|   + mídia absolutizada   +------------------+
  +----------------------------+
```

Três peças:

1. **Painel Omni** (`web`, Nuxt na porta 3003) — o CRUD. Edita o rascunho,
   publica, gerencia mídia. Fala com a API autenticado (JWT + `X-Account-Id`).
2. **API Go** (`back`, porta 9091) — o módulo `bio`, plugável via Module
   Registry (padrão `automation`/`meta_ads`), com schema Postgres `bio.*`.
   Serve tanto o painel (rotas `/v1/bio`, gated pelo módulo) quanto o público
   (rotas `/v1/public/bio`, sem JWT, fora do gate).
3. **Front bio** (`crow-nuxt`) — projeto Nuxt **separado** (não é o painel),
   rodando em outro domínio/porta. Renderiza a página pública consumindo a API
   pública e ouve atualizações via SSE.

## Módulo Go `bio`

Local: `back/internal/modules/bio/`. Registro no `app.go`
(`registry.MustRegister(bio.New())`) e a regra de gating
(`{Prefix: "/v1/bio", ModuleID: "bio"}`) são feitos na integração — a rota
pública `/v1/public/bio` fica **fora** do gate (prefixo diferente).

Arquivos principais:

| Arquivo | Responsabilidade |
|---|---|
| `module.go` | Adaptador do Registry: ID `bio`, schema `bio`, permissões, role templates, build/handle |
| `model.go` | DTOs: `Bio`, `BioSummary` (lean), `BioView`, `BioDefaults`, `Media`, tipos de `source`/`button`/facets, requests |
| `store_postgres.go` | CRUD `bio.bios`/`bio.defaults`/`bio.media`; scan nullable `*string`/`*time.Time`; SQL parametrizado; filtra `account_id` quando escopado |
| `service.go` | Escopo, slug, create (cliente opcional + slug derivado), duplicate, patch (mover de account só admin), publish, defaults, público |
| `merge.go` | `deepMerge` (objeto recursivo; array/primitivo substitui) + `absolutizeUploads` |
| `media_storage.go` | Upload local em `UPLOADS_DIR/bio/{account}/`, perms `0o750`/`0o600`, allowlist de mime, limites por env |
| `http.go` | Rotas do painel (JWT + `X-Account-Id`) |
| `http_public.go` | `GET /v1/public/bio/{slug}` + `/stream` (SSE), sem JWT |
| `sse.go` | `sseBroker` (hub em memória por slug) + handler SSE |
| `service_test.go` | Testes de merge, escopo, publish, slug, create/duplicate/patch |

### Schema `bio.*` (migration `0152_bio_schema.sql`)

Idempotente, schema-qualificado, **sem `-- +goose Down`** (o migrator roda o
arquivo inteiro).

| Tabela | Conteúdo |
|---|---|
| `bio.bios` | 1 linha por bio: `account_id` (FK `core.accounts`), `slug` (único global por `lower(slug)`), `name` (interno), `status` (`draft`/`published`), `data_draft jsonb`, `data_published jsonb`, `published_at` |
| `bio.defaults` | 1 linha global (`id='global'`), `data jsonb` — equivale ao `_default.json` do front bio. Nasce `{}` até o admin colar o conteúdo via `PUT /v1/bio/defaults` |
| `bio.media` | Metadado de cada upload: `account_id`, `bio_id`, `kind`, `path`, `mime`, `size_bytes` |

Índices: `bio_bios_slug_uidx` (`lower(slug)`), `bio_bios_account_idx`,
`bio_media_account_idx`.

### Endpoints do painel (gating `/v1/bio` → módulo `bio`, JWT)

| Verbo | Path | Notas |
|---|---|---|
| GET | `/v1/bio/bios?accountId=&status=&q=` | Lista lean. `accountId` é filtro dentro do permitido; não-admin só vê a própria account (pedir outra → `404`) |
| POST | `/v1/bio/bios` | `{accountId?, slug?, name}`. Cliente **opcional** (vazio = account do contexto); slug **opcional** (derivado do nome; slug já existente → `409 slug_taken`). Auto-habilita o módulo `bio` na account |
| POST | `/v1/bio/bios/{id}/duplicate` | `{accountId?}`. Copia `data_draft` (não o published) numa bio nova (status `draft`, slug `{slug}-copia` único, name `Cópia de {name}`). Retorna **201** |
| GET | `/v1/bio/bios/{id}` | Draft + published + meta. Fora do escopo → `404` |
| PATCH | `/v1/bio/bios/{id}` | `{name?, slug?, dataDraft?, accountId?}`. `dataDraft` substitui o rascunho inteiro. `accountId` move de account — honrado **só** para `platform_admin` (não-admin é ignorado) |
| POST | `/v1/bio/bios/{id}/publish` | Copia draft→published; valida no JSON mesclado: `branding.logo.srcMobile` + um fundo (`video.bgVideo` OU `video.bgImage`). Notifica o `sseBroker` |
| POST | `/v1/bio/bios/{id}/unpublish` | Volta para draft (público passa a `404`) |
| DELETE | `/v1/bio/bios/{id}` | Remove a bio |
| GET | `/v1/bio/bios/{id}/preview` | JSON mesclado do draft (prévia no painel), mídia relativa |
| POST | `/v1/bio/bios/{id}/media` | multipart (`kind` + `file`) → `{"url":"/uploads/bio/..."}` |
| GET/PUT | `/v1/bio/defaults` | Só `platform_admin` |
| GET | `/v1/bio/sources` | Fontes disponíveis para a account (`[{type, label, available}]`); MVP devolve `site_products` |
| GET | `/v1/bio/sources/site_products/facets` | `{categories[], campaigns[], tipos[]}` distintos de `site.products` da account — popula os selects do editor de slides |

### Endpoint público (sem JWT, fora do gating)

`GET /v1/public/bio/{slug}`:

1. Valida `^[a-z0-9-]+$` (normaliza para minúsculas).
2. 1 query com join `core.accounts` + `core.account_modules`: bio `published`,
   account ativa, módulo `bio` habilitado. Qualquer falha → `404 not_found`
   (não vaza existência).
3. Se `slideTop.source.type != manual`, resolve a fonte de produtos e injeta
   `slides[]` ANTES do merge/absolutize.
4. Responde `deepMerge(bio.defaults.data, data_published)` com mídia
   absolutizada (`PUBLIC_API_BASE_URL` + path). `Cache-Control: public, max-age=60`.
5. Se `BIO_PUBLIC_TOKEN` estiver setada, exige `Authorization: Bearer <token>`
   (default desligado = aberto).

`GET /v1/public/bio/{slug}/stream` (SSE): o browser da bio abre um `EventSource`
nesse endpoint; conexão ociosa (`: ping` a cada 25s). Quando a bio é
(re)publicada/despublicada, o serviço chama `broker.notify(slug)` e a conexão
emite `event: updated` — o front então refetcha o JSON e atualiza sem F5. Hub em
memória, por slug; envia só o sinal (não o conteúdo). **Push, não polling**
(ENGINEERING_PRINCIPLES §6).

> **CORS público intencional**: as rotas `/v1/public/*` usam
> `Access-Control-Allow-Origin: *` DE PROPÓSITO — o front do bio roda em outro
> domínio e conecta direto (SSE/fetch do browser). É cookie-less e read-only;
> não vaza dado (recursos públicos, sem credenciais). Exceção consciente ao
> "sem `*` em prod".

### Permissões e role templates

- `bio.view` / `bio.manage` / `bio.publish` (scope `account`).
- Templates: `bio.manager` (as 3), `bio.editor` (view + manage),
  `bio.viewer` (view).

### Segurança / multi-tenant (resumo)

- `account_id` nunca vem do body para escopo — vem do Principal/`X-Account-Id`;
  o `accountId` de query é filtro validado contra o Principal.
- Fora do escopo → `404` (nunca `403`).
- SQL 100% parametrizado (`$1::uuid`); índices em `account_id` e `lower(slug)`.
- Lista lean (sem jsonb); jsonb só no GET por id.
- Upload escopado por account no path; perms `0o750`/`0o600`.

## Fonte de produtos (B7 + B8)

A seção Slides pode montar o carrossel a partir de produtos reais, não só
imagens manuais. A arquitetura é plugável:

- **`ProductSource` (interface, no módulo `bio`)**: `Facets(ctx, accountID)` →
  categorias/campanhas/tipos distintos; `Resolve(ctx, accountID, filtro, limit)`
  → `[]ResolvedSlide{src, title, href}`.
- **`SiteProductsSource` (1ª fonte real, MVP — B7)**: lê a tabela
  `site.products` (schema `site`) por account, filtrando por
  category/campaigns/tipo e `status` ativo, com `limit`. Cross-schema usando o
  mesmo pool; só LÊ a tabela (não acopla ao módulo `site`).
- **Resolução no BACK**: quando `slideTop.source.type != manual`, o endpoint
  público resolve os produtos e injeta em `slideTop.slides[]` já prontos
  (`src=imageUrl`, `title=name`, `href`/`whatsapp`). O front continua recebendo
  apenas `slides[]` (mais o `button` opcional abaixo do carrossel).

### Sync da API do cliente → `site.products` (B8)

`site.products` é **populado** puxando da API do site do próprio cliente. A bio
sempre lê do nosso `site.products` — a sync mantém esse espelho atualizado.

- **1º cliente: Pérola** (`https://perolajoias.com/api/products/`, GET público).
- **Config de fonte externa por account** (ex.: `site.product_sources`:
  `account_id`, `type=external_api`, `base_url`, `enabled`).
- **Cliente HTTP** lê a API paginada (`limit=100`, segue `has_more`), com
  timeout/contexto, parseia os JSON-arrays de `categories`/`campaigns`.
- **Upsert** em `site.products` por `account_id + external_id` (não duplica;
  atualiza mudados; marca como inativo/removido os `deleted_at`), em batch.
- **Endpoint** `POST /v1/admin/products/sync?accountId=` dispara a sync da
  account → `{inserted, updated, skipped}`. Agendamento periódico fica como fase
  futura (começa sob demanda/botão no painel).
- O contrato e as melhorias pedidas à API do cliente estão documentados em
  `painel-perola/docs/MELHORIAS_OMNI.md`.

## Front bio (crow-nuxt)

Projeto Nuxt **separado** em `C:\Users\Mike\Documents\Projects\crow-nuxt`, dev na
porta 3000 (≠ painel 3003).

- **Rota pública**: `app/pages/[slug].vue` → URL `/{slug}` (ex.: `/perola`), na
  raiz. `/` (index) e `/bio/preview` (iframe do painel) são mais específicos e
  não colidem. O link "Ver online" do painel monta `bioFrontUrl + '/' + slug`.
- **Fonte dos dados (SSR)**: `server/api/bio/[slug].get.ts`. Se `NUXT_API_BASE`
  estiver setada, faz `GET {apiBase}/bio/{slug}`; senão lê JSONs locais de
  `public/bio-data/` (fallback).
- **Cache**: `routeRules` com SWR 300s **só em produção**; em dev `{ cache:
  false }` (senão as edições demoram a aparecer).
- **Tempo real**: `[slug].vue` abre `EventSource` em
  `{NUXT_PUBLIC_STREAM_BASE}/bio/{slug}/stream`; ao receber `event: updated`,
  faz `refresh()`.
- **Prévia no painel**: `app/pages/bio/preview.vue` renderiza o BioData recebido
  por `postMessage` — usado num iframe no editor (`BioLivePreview.vue`).
- **Fundo vídeo OU imagem**: `app/components/bio/BgVideo.vue` mostra `<video>` se
  houver `bgVideo`, senão `<img>` com `bgImage`.

> Gotcha: nunca usar `robots: false` (nem chaves de módulos ausentes) em
> `routeRules` do crow-nuxt — derruba o worker SSR no boot.

## Fluxo draft → published

```
  editar no painel  -->  data_draft (auto-save, PATCH dataDraft)
        |
        v
  clicar Publicar/Republicar
        |
        v
  copia data_draft -> data_published  (valida logo + 1 fundo)
        |                                   |
        v                                   v
  status = published                  broker.notify(slug)  -- SSE -->  crow-nuxt refetcha
        |
        v
  GET /v1/public/bio/{slug}
    = deepMerge(defaults, data_published)
    + slides resolvidos da fonte (se source != manual)
    + mídia absolutizada (PUBLIC_API_BASE_URL)
```

Despublicar zera o público (volta a `404`); o rascunho permanece.

## Envs de deploy

| Env | Onde | Para quê |
|---|---|---|
| `PUBLIC_API_BASE_URL` | back | **Obrigatória p/ mídia uploaded funcionar**: absolutiza `/uploads/...` no endpoint público. Sem ela a mídia sai relativa e quebra no front (outro domínio). Default docker dev: `http://localhost:9091`. Mídia `/assets/...` (servida pelo próprio front) não é tocada |
| `BIO_PUBLIC_TOKEN` | back | Opcional (default vazio = endpoint público aberto). Se setada, exige `Authorization: Bearer <token>` |
| `BIO_MAX_VIDEO_MB` / `BIO_MAX_IMAGE_MB` | back | Limites de upload (default 200 / 10) |
| `UPLOADS_DIR` | back | Raiz dos uploads, compartilhada com o file server `/uploads/` |
| `NUXT_API_BASE` | crow-nuxt | Base da API pública p/ o SSR: `GET {base}/bio/{slug}` (ex.: `http://localhost:9091/v1/public`) |
| `NUXT_PUBLIC_STREAM_BASE` | crow-nuxt | URL do back acessível pelo **browser** p/ o SSE (ex.: `http://localhost:9091/v1/public`) |
| `NUXT_PUBLIC_BIO_FRONT_URL` | painel (web) | URL do front bio p/ montar o link "Ver online" (default docker dev: `http://localhost:3000`) |
| `CARDAPIO_*` | back | Envs do módulo irmão `cardapio`, que divide o `app.go` com o `bio` (ver `docs/cardapio/`). Listadas aqui só como lembrete: não afetam o bio, mas convivem no mesmo deploy |

### Ordem de deploy (resumo)

1. Migration `0152_bio_schema.sql` (+ migration de `site.product_sources`/
   `external_id` da B8, se ainda não aplicada).
2. **Rebuild da api** (mudou Go): `docker compose up -d --build api`.
3. Setar as envs do back (`PUBLIC_API_BASE_URL`, limites de upload) em
   `.env.production` E no `docker-compose.prod.yml`.
4. Setar `NUXT_API_BASE` e `NUXT_PUBLIC_STREAM_BASE` no crow-nuxt e reiniciar.
5. Habilitar o módulo `bio` na account de teste; e2e (criar → editar → publicar
   → GET público em `/{slug}`).
