# AGENT — Modulo Go `bio`

Modulo plugavel (Module Registry) das paginas de **link-in-bio** dentro do Omni.
Tenant-aware (schema `bio.*`, `account_id` FK `core.accounts`). O painel Omni e o
CRUD; o front Nuxt separado (VPS) consome `GET /v1/public/bio/{slug}`
server-to-server e renderiza.

> Doc canonico: docs/bio/PLANO_MODULO_BIO.md (fases B1/B2/B3). Contrato de shape:
> `types/bio.ts`/`API-INTEGRATION.md` do repo do front bio.

## Estado: B1 (back + banco) — 2026-06-12; B6.A (refino) — 2026-06-13; B7 (fonte de produtos) — 2026-06-13; B9 (avif + previa fixes) — 2026-06-14

> B9 (2026-06-14): `media_storage.go` passou a aceitar **avif** (`matchAllowedType`
> + `typeFromExtension`). No front, o `BioMediaField` ganhou a prop
> `duplicateTargets` (botoes "copiar para") para DUPLICAR a imagem entre variantes
> — branding (logo mobile<->desktop) e video (background mobile/desktop + poster).
> Ver `web/app/components/bio/AGENT.md`.

Migration `0152_bio_schema.sql` + modulo Go completo + testes. O registro no
`app.go` (MustRegister + gating rule) e o wiring do front sao da integracao
(B3), NAO deste modulo.

B6.A (iteracao do editor — backend): cliente OPCIONAL no Create (admin sem
`accountId` usa o contexto) + slug derivado do `name` quando vazio; novo endpoint
`POST /v1/bio/bios/{id}/duplicate`; `PATCH` passa a aceitar `accountId` p/ mover
de account (so `platform_admin`). Sem migration nova (reusa o schema 0152).

B7 (fonte de produtos plugavel — backend): o `slideTop` ganha um `source` que
pode ser `manual` (slides a mao, como hoje) ou uma FONTE de produtos. 1a fonte:
`site_products` (le `site.products`). Interface `ProductSource` (Facets +
Resolve) deixa ERP/API externa entrarem depois sem mexer no resto. Tres
endpoints de painel (`/v1/bio/sources`, `/v1/bio/sources/{type}/facets`,
`/v1/bio/sources/{type}/resolve`) e a resolucao automatica no endpoint publico.
O `/resolve` serve a PREVIA do editor: devolve os mesmos slides que o publico
injetaria, para o painel mostrar os produtos da fonte ANTES de publicar. Sem
migration nova (so LE `site.products`, cross-schema, mesmo pool).

## Schema `bio.*` (migration 0152)

| Tabela | Conteudo |
|---|---|
| `bio.bios` | 1 linha por bio: `account_id`, `slug` (unico global `lower(slug)`), `name` (interno), `status` (`draft`/`published`), `data_draft jsonb`, `data_published jsonb`, `published_at` |
| `bio.defaults` | 1 linha global (`id='global'`) com `data jsonb` — equivalente ao `_default.json` do front bio. Nasce `{}` ate o admin colar o conteudo via PUT |
| `bio.media` | Metadado de cada upload: `account_id`, `bio_id`, `kind`, `path` (`/uploads/bio/{account}/...`), `mime`, `size_bytes` |

Indices: `bio_bios_slug_uidx` (`lower(slug)`), `bio_bios_account_idx`,
`bio_media_account_idx`.

## Arquivos

| Arquivo | Responsabilidade |
|---|---|
| `module.go` | Adaptador do Registry: ID `bio`, schema `bio`, permissoes, role templates, Build + handle (rotas painel + publica) |
| `model.go` | DTOs: `Bio`, `BioSummary` (lean), `BioView`, `BioDefaults`, `Media`, requests + (B7) `SourceInfo`, `SourceFacets`, `SourceFilter`, `ResolvedSlide`, `slideTopSource` e as constantes de `type` (`manual`/`site_products`) e `link` (`product`/`whatsapp`/`none`) |
| `store_postgres.go` | CRUD `bio.bios`/`bio.defaults`/`bio.media`; `Create`/`CreateWithDraft` (cria ja com draft, usado na duplicacao), `Patch` (nome/slug/draft + `account_id` opcional p/ mover), `AccountExists` (valida destino do move); scan nullable `*string`/`*time.Time`; SQL parametrizado; queries filtram `account_id` quando escopadas. `PublicLookup` devolve `(data_published, account_id)` — o account_id alimenta a resolucao de fontes (B7) |
| `service.go` | Escopo (`resolveScope`/`scopeForLookup`), slug (`normalizeSlug`/`slugify`/`uniqueSlug`), `Create` (cliente opcional + slug derivado), `Duplicate`, `Patch` (move de account so admin), publish (draft->published + minimos), defaults, publico (resolve a fonte do slideTop antes do absolutize). `RegisterSource` registra fontes plugaveis. Depende da interface `bioStore` (testavel sem banco) |
| `slug.go` | `normalizeSlug` (valida formato `^[a-z0-9-]+$`), `slugify` (alias para `stringsx.Slugify` — regra canonica NFD, 2026-06-29), `uniqueSlug` (tenta base, sufixo -2/-3... ate nao colidir) |
| `product_source.go` | **(B7)** Interface `ProductSource` (`Facets`/`Resolve`) + `SiteProductsSource` que LE `site.products` (cross-schema, mesmo pool, so SELECT). Facets via `jsonb_array_elements_text` (categorias/campanhas) + distinct de `tipo`; Resolve filtra `category`/`campaigns`/`tipo` + `status='active' and is_active`, ordem estavel (`lower(name), id`), limit 0=todos. Monta o `href` do slide conforme o `link` (whatsapp -> `wa.me`; product/none -> sem URL hoje) |
| `service_sources.go` | **(B7)** `Sources` (catalogo de fontes), `Facets` (delega para a fonte; type desconhecido -> 404), `ResolvePreview` (resolve a fonte com filtros vindos da query, sem depender de bio publicada — alimenta a previa do editor) e `resolveSlideSource` (injeta os produtos resolvidos em `slideTop.slides`; fonte ausente/manual/erro/vazia -> mantem os slides manuais) |
| `merge.go` | `deepMerge` (objeto recursivo; array/primitivo substitui) + `absolutizeUploads` + `jsonHasNonEmptyPath` |
| `media_storage.go` | Upload local em `UPLOADS_DIR/bio/{account}/`, perms `0o750`/`0o600`, allowlist mime; kinds `video`/`background`/`poster`/`logo`/`favicon`/`slide`/`store`; limites configuraveis por env (`BIO_MAX_VIDEO_MB` default 200, `BIO_MAX_IMAGE_MB` default 10). **avif aceito (2026-06-14)**: `matchAllowedType` (`image/avif`) + `typeFromExtension` (`.avif`) reconhecem avif por magic bytes e por extensao (allowlist: mp4/webm/webp/avif/png/jpeg/svg/ico) |
| `http.go` | Rotas do painel (JWT + X-Account-Id); publish/unpublish notificam o `sseBroker` |
| `http_sources.go` | **(B7)** Rotas do painel das fontes de produto: `GET /v1/bio/sources`, `GET /v1/bio/sources/{type}/facets` e `GET /v1/bio/sources/{type}/resolve` (JWT + escopo; gateadas por modulo como o resto de `/v1/bio`) |
| `http_public.go` | `GET /v1/public/bio/{slug}` + `GET /v1/public/bio/{slug}/stream` (SSE), sem JWT |
| `sse.go` | `sseBroker` (hub em memoria por slug) + handler SSE de tempo real |
| `service_test.go` | merge (array/objeto/primitivo), absolutize, escopo, publish, slug/`slugify`. Usa `fakeStore` (implementa `bioStore` em memoria) p/ cobrir Create (cliente opcional + slug derivado/colisao), Duplicate (copia draft nao published, slug unico, nova account, fora-do-escopo 404) e Patch accountId (so admin; nao-admin ignora; destino inexistente 404) |
| `service_sources_test.go` | **(B7)** `fakeSource` (ProductSource em memoria). Cobre `Sources`, `Facets` (delegacao + 404 de fonte desconhecida), `Public` resolvendo `site_products` (slides injetados, filtro/link/whatsapp propagados) e os fallbacks p/ slides manuais (source manual/ausente/desconhecido/erro/vazio). Helpers de href (whatsapp/produto/digitsOnly) |

## Endpoints do painel (gating `/v1/bio` -> modulo `bio`, JWT)

| Verbo | Path | Notas |
|---|---|---|
| GET | `/v1/bio/bios?accountId=&status=&q=` | Lista lean. `accountId` e filtro dentro do permitido: nao-admin so a propria account; pedir outra -> `404` |
| POST | `/v1/bio/bios` | `{accountId?, slug?, name}` — **cliente OPCIONAL**: nao-admin ignora `accountId` e usa o contexto; admin sem `accountId` usa o contexto (a agencia). **Slug OPCIONAL**: vazio -> derivado do `name` via `slugify`, com sufixo numerico (`-2`, `-3`...) se colidir; slug informado que ja exista -> `409 slug_taken`. **Auto-habilita o modulo `bio` na account** (`EnsureBioModuleEnabled`) — senao a bio publicada nessa account daria 404 no endpoint publico (que exige o modulo) |
| POST | `/v1/bio/bios/{id}/duplicate` | `{accountId?}` (body opcional). Cria uma bio NOVA (status `draft`) copiando `data_draft` da origem (NAO o published), `name` = "Copia de {name}", slug unico derivado de `{slug}-copia` (`-copia-2`...). Mesma account da origem; admin pode duplicar p/ outra via `accountId` (destino validado, 404 se inexistente). Auto-habilita o modulo na account destino. Retorna a BioView nova (**201**). Fora do escopo -> `404` |
| GET | `/v1/bio/bios/{id}` | Draft + published + meta. Fora do escopo -> `404` |
| PATCH | `/v1/bio/bios/{id}` | `{name?, slug?, dataDraft?, accountId?}` (dataDraft substitui o draft inteiro). **`accountId` move a bio para outra account e e honrado APENAS para `platform_admin`** — nao-admin que mandar `accountId` tem o campo IGNORADO (nunca troca de account). Destino validado (404 se inexistente); slug e global, nao recolide |
| POST | `/v1/bio/bios/{id}/publish` | Copia draft->published; valida no JSON mesclado: `branding.logo.srcMobile` + um fundo (`video.bgVideo` OU `video.bgImage`). Notifica o `sseBroker` |
| POST | `/v1/bio/bios/{id}/unpublish` | Volta para draft (publico passa a 404) |
| DELETE | `/v1/bio/bios/{id}` | |
| GET | `/v1/bio/bios/{id}/preview` | JSON mesclado do draft (previa no painel), midia relativa |
| POST | `/v1/bio/bios/{id}/media` | multipart (`kind`+`file`) -> `{"url":"/uploads/bio/..."}` |
| GET/PUT | `/v1/bio/defaults` | So `platform_admin` |
| GET | `/v1/bio/sources` | **(B7)** Fontes de produto disponiveis para a account: `{"sources":[{type,label,available}]}`. MVP: `[{type:"site_products", label:"Produtos do site", available:true}]` |
| GET | `/v1/bio/sources/{type}/facets` | **(B7)** Valores distintos da fonte para popular os selects do editor: `{categories[], campaigns[], tipos[]}` (sempre arrays, nunca null). So produtos ATIVOS da account. `type` desconhecido -> `404` |
| GET | `/v1/bio/sources/{type}/resolve` | **(B7)** Resolve a fonte para a PREVIA do editor: query `category`, `campaigns` (csv), `tipo`, `limit`, `link`, `whatsapp` -> `{"slides":[{src,title,href,desc?,price?}]}`. Mesmo resultado que o publico injetaria em `slideTop.slides`; o painel usa para mostrar os produtos ANTES de publicar. `type` desconhecido -> `404` |

## B7 — Fonte de produtos do slideTop (contrato back <-> front)

O `slideTop` (dentro do `dataDraft`/`dataPublished` jsonb) ganha campos OPCIONAIS
e retrocompativeis. O painel preenche; o back so precisa entender o `source` para
resolver no publico:

```jsonc
slideTop: {
  active, engine, carousel{...}, slides[],   // (manual, como hoje)
  mode?: 'carousel' | 'static',              // D8 — modo de exibicao (so o front usa)
  source?: {
    type: 'manual' | 'site_products',        // 'manual'/ausente = slides a mao
    category?: string,                        // 1 categoria (filtro AND)
    campaigns?: string[],                     // overlap: casa qualquer uma (filtro AND com as demais)
    tipo?: string,                            // 1 tipo (filtro AND)
    limit?: number,                           // 0/ausente = todos; senao N (D4: 5/10/0)
    link?: 'product' | 'whatsapp' | 'none'    // D6 — para onde o slide-produto leva
  },
  button?: { text: string, href: string }     // D5 — 1 botao abaixo do carrossel (so o front/crow-nuxt renderiza)
}
```

- **Resolucao (publico):** em `GET /v1/public/bio/{slug}`, se `slideTop.source.type`
  for uma fonte registrada (!= `manual`/ausente), o back RESOLVE os produtos e
  SOBRESCREVE `slideTop.slides` com `[{src, title, href?, desc?, price?}]` ANTES do
  merge/absolutize. `src` = imagem do produto (absolutizada como o resto se
  `/uploads/...`); `title` = nome; `desc` = descricao; `price` = preco BRL (omitido
  se `<= 0`); `href` conforme o `link`:
  - `whatsapp` -> `https://wa.me/<digitos>?text=...` usando `lightbox.whatsappNumber`
    da propria bio; sem numero -> sem href.
  - `product` / `none` -> sem href hoje (`site.products` ainda nao tem URL de
    produto; quando a fonte trouxer, e so popular). O `button` cobre o "ver colecao".
- **Fallback:** fonte ausente/`manual`/desconhecida, erro de query ou zero
  produtos -> mantem os `slides[]` manuais (a bio NUNCA quebra por causa da fonte).
- **Previa (editor):** o painel chama `/v1/bio/sources/{type}/resolve` com os
  filtros do `source` do draft e injeta o resultado em `slideTop.slides` da copia
  enviada ao iframe do crow-nuxt (`BioLivePreview`). Mesmo shape
  `{src,title,href,desc?,price?}` do publico -> a previa bate com o que vai ao ar.
  No crow-nuxt, `SlideTopKeen` usa `href || whatsapp` (slide-produto traz `href`;
  slide manual traz `whatsapp`) e FILTRA slides sem `src` (nao renderiza slide
  vazio; a `<section>` some se sobrar zero). O `ResolvedSlide` agora carrega `Desc`
  (descricao) e `Price` (preco formatado BRL via `formatPriceBRL`; `<= 0` -> vazio,
  nao inventa) p/ o Lightbox do crow-nuxt mostrar nome/preco.
- **Carrossel reativo na previa (fix 2026-06-14):** o `SlideTopKeen` (crow-nuxt)
  criava o KeenSlider so no `onMounted` e nao reagia a mudanca de `cfg`/`slides`/
  `mode` — entao "Slides por vista" nao mudava e o autoplay DESLIGADO continuava
  rodando na previa. Corrigido com `buildSlider`/`teardownSlider` + watchers:
  `cfg` (deep) reconfigura perView/spacing/loop IN-PLACE e re-avalia o autoplay
  (`scheduleAutoplay` limpa o timer quando `autoplay=false`); `mode` e contagem de
  slides recriam o slider. Agora a config do carrossel reflete na previa NA HORA.
- **Previa ao vivo (fix 2026-06-14):** o `BioLivePreview` so atualizava o produto
  da fonte ao PUBLICAR. Causa: a chave de cache (`resolvedKey`) era fixada ANTES do
  `await` da resolucao — uma resolucao que falhasse/concorresse "engolia" a mudanca.
  Corrigido: a chave so e gravada APOS o await com sucesso + `pendingKey` descarta
  resposta de fonte antiga (troca de filtro no meio do fetch). Agora mudar
  categoria/campanha/limite/link reflete na previa NA HORA, sem publicar.
- **`mode` e `button`** sao consumidos pelo front (editor) e pelo crow-nuxt (render);
  o back so os carrega/serve no jsonb, nao os interpreta. A injecao de slides
  resolvidos sobrescreve so `slideTop.slides` (preserva `mode`/`carousel`/`button`).

### Fonte `site_products` (cross-schema)

`SiteProductsSource` LE a tabela `site.products` (schema `site`) pelo MESMO pool
(`deps.Pool`), apenas `SELECT`. NAO importa o modulo `site` — depende so do shape
estavel da tabela (colunas `account_id`, `name`, `image`, `categories` jsonb[],
`campaigns` jsonb[], `tipo`, `status`, `is_active`). "Produto ativo" =
`status='active' AND is_active=true`. Toda query filtra `account_id = $1::uuid`
(escopo ja resolvido contra o Principal no handler), e SQL parametrizado. Ordem
estavel (`lower(name), id`). Facets via `jsonb_array_elements_text` + `distinct`
(sem N+1). Novas fontes (ERP, API do cliente) implementam `ProductSource` e se
registram via `svc.RegisterSource(type, src)` no `Build` — sem tocar no resto.

## Endpoint publico (sem JWT, fora do gating)

`GET /v1/public/bio/{slug}`:

1. Valida `^[a-z0-9-]+$` (normaliza lowercase).
2. 1 query (join `core.accounts` + `core.account_modules`): bio `published`,
   account ativa, modulo `bio` habilitado, devolve `data_published` + `account_id`.
   Qualquer falha -> `404 not_found`.
3. **(B7)** Se `slideTop.source.type` for uma fonte registrada, resolve os
   produtos da fonte (filtrados pelo `account_id` da bio) e injeta em
   `slideTop.slides` ANTES do merge/absolutize. Erro/vazio -> slides manuais.
4. Responde `deepMerge(bio.defaults.data, data_published)` com midia
   absolutizada (`PUBLIC_API_BASE_URL` + path). `Cache-Control: public, max-age=60`.
5. Se `BIO_PUBLIC_TOKEN` setada, exige `Authorization: Bearer <token>` (default
   desligado).

### Tempo real — `GET /v1/public/bio/{slug}/stream` (SSE)

Push, NAO polling (ENGINEERING_PRINCIPLES §6). O browser da bio publica abre um
`EventSource` nesse endpoint; a conexao fica ociosa (so um `: ping` a cada 25s).
Quando a bio e (re)publicada/despublicada, `handlePublish`/`handleUnpublish`
chamam `broker.notify(slug)` e a conexao emite `event: updated` — o front entao
refetcha o `GET /v1/public/bio/{slug}` e atualiza SEM recarregar a pagina. O hub
(`sseBroker`) e em memoria, por slug; nao envia conteudo (so o sinal). O front do
bio (crow-nuxt) liga isso via env `NUXT_PUBLIC_STREAM_BASE` (URL do back acessivel
pelo browser). Sem rate-limit dedicado por enquanto (melhoria futura: limite de
conexoes SSE por IP).

> CORS: as rotas `/v1/public/*` usam `Access-Control-Allow-Origin: *` (cookie-less,
> read-only) DE PROPOSITO — o front do bio/cardapio roda em outro dominio e
> conecta direto (SSE/fetch do browser). Excecao intencional ao "sem `*` em prod"
> do §5; nao vaza dado (sem credenciais, recursos publicos).

## Permissoes e role templates

- `bio.view` / `bio.manage` / `bio.publish` (scope `account`).
- Templates: `bio.manager` (3), `bio.editor` (view+manage), `bio.viewer` (view).

## Seguranca / multi-tenant

- `account_id` nunca vem do body para escopo — vem do Principal/X-Account-Id; o
  `accountId` de query e filtro validado contra o Principal.
- Fora do escopo -> `404 not_found` (nunca 403; nao vaza existencia).
- SQL 100% parametrizado (`$1::uuid`); indices em `account_id` e `lower(slug)`.
- Lista lean (sem jsonb); jsonb so no GET por id.
- Upload escopado por account no path (`/uploads/bio/{account}/`); perms
  `0o750`/`0o600` (lint gosec G301/G306).
- **(B7)** A leitura cross-schema de `site.products` SEMPRE filtra
  `account_id = $1::uuid` (escopo do painel resolvido contra o Principal; no
  publico, o `account_id` da propria bio). So `SELECT`; nunca escreve em `site.*`.

## Envs (deploy — ver §6 do plano)

- `PUBLIC_API_BASE_URL` — **obrigatoria p/ midia uploaded funcionar**: absolutiza
  `/uploads/...` no endpoint publico. Sem ela a midia sai relativa e quebra no
  front (que roda em outro dominio). Default no docker-compose: `http://localhost:9091`.
  Midias `/assets/...` (servidas pelo proprio front bio) NAO sao tocadas.
- `NUXT_PUBLIC_STREAM_BASE` (no front crow-nuxt) — URL do back p/ o SSE.
- `BIO_MAX_VIDEO_MB` / `BIO_MAX_IMAGE_MB` — limites de upload (default 200 / 10).

## Fundo: video OU imagem

O bloco `video` aceita `bgVideo`/`bgVideoPc` (video) E `bgImage`/`bgImagePc`
(imagem). O front bio renderiza video quando ha `bgVideo`; senao usa `bgImage`
como background. Publicar exige UM dos dois (+ logo). Upload de imagem de fundo
usa `kind=background`.
- `BIO_PUBLIC_TOKEN` — opcional (default vazio = endpoint aberto).
- `UPLOADS_DIR` — raiz dos uploads (compartilhada com o file server `/uploads/`).

## Wiring pendente (integracao B3, NAO neste modulo)

- `app.go`: `registry.MustRegister(bio.New())` no bloco CoreV2.
- `moduleGatingRules()`: `{Prefix: "/v1/bio", ModuleID: "bio"}` (a rota publica
  `/v1/public/bio` fica FORA do gate — prefixo diferente).
