# AGENT - Bio (Site/Bio)

## Escopo

Painel do modulo `bio` (link-in-bio) em `web/app/components/bio/`, mais:

- `web/app/domain/bio/types.ts` — port do contrato `BioData` (PLANO_MODULO_BIO.md §4).
- `web/app/stores/bio.ts` — store Pinia (lista, bio ativa, defaults, duplicate, mover de account).
- `web/app/composables/useBioEditor.ts` — draft, dirty-check, AUTO-SAVE (debounce ~800ms), pilha de UNDO, upload.
- `web/app/pages/site/bio/index.vue` e `[id].vue` — orquestradores finos.

Doc canonico do modulo: `docs/bio/PLANO_MODULO_BIO.md` (este painel e a fase B2).

## Arquitetura

- O painel e o CRUD; o front bio (Nuxt separado, VPS) consome `GET /v1/public/bio/{slug}` e renderiza. O conteudo vive em `data_draft`/`data_published` jsonb por bio + `bio.defaults` global.
- Multitenant: cada cliente ve/edita so a(s) bio(s) da sua account. `platform_admin` ve todas, com filtro por cliente (accounts via `useTenantsStore` → `/v1/tenants`).
- `X-Account-Id` e injetado automaticamente pelo `createApiRequest`; nao passar manual.
- `accountId` de query e FILTRO dentro do permitido: nao-admin so ve a propria account (o backend retorna 404 fora do escopo). O front gateia o filtro de cliente por papel (so `platform_admin`).

## Componentes

- `BioListWorkspace.vue` — tabela (nome/slug/cliente/status/atualizado) + busca + filtro de status + filtro por cliente (so admin) + acoes editar/**duplicar**/**ver online**/excluir. "Ver online" abre `bioFrontUrl + '/' + slug` (SEM `/bio`) em nova aba.
- `BioCreateModal.vue` — nome (obrigatorio) + slug (OPCIONAL, deriva do nome) + select de cliente (so admin, OPCIONAL; placeholder "Sem cliente (agencia)"). DOIS botoes: "Criar" (volta a lista) e "Criar e editar" (abre o editor). Emite `submit` com flag `edit`. Slug global, regex `^[a-z0-9-]+$`.
- `BioEditorWorkspace.vue` — shell do editor: seletor de **Cliente** (so admin) → `moveBioAccount` + `BioPublishBar` no topo + sidebar de secoes + painel ativo + preview ao vivo. Listener de `Ctrl+Z`/`Cmd+Z` → `editor.undo()` (ignora foco em input/textarea).
- `BioPublishBar.vue` — status draft/published, indicador de **auto-save** (Salvando.../Salvo/Erro), **switch "Editando ↔ Publicado"** (controla a fonte do preview via `update:source`), botao **Desfazer** (`undo`), Republicar/Despublicar, link "Ver online". SEM Salvar/Previa (auto-save + preview ao vivo substituem).
- `sections/BioSectionMeta.vue` — meta (lang, title, favicon, gtmId).
- `sections/BioSectionBranding.vue` — branding (nome do perfil, logo, logo do rodape). `logo.srcMobile` obrigatorio para publicar.
- `sections/BioSectionVideo.vue` — "Fundo e layout". TRES campos unificados (Background mobile, Background desktop, Poster) que aceitam imagem OU video; o upload detecta o tipo pela extensao da URL e grava em `bgVideo`/`bgVideoPc` (video) ou `bgImage`/`bgImagePc` (imagem), limpando o slot oposto. Mais overlay + layout (alignItems, delay, template). Para publicar, basta um fundo + o logo. NAO contem os toggles "Slide do topo ativo" (vive em Slides) nem "Header mobile ativo" (vive em Links).
- `sections/BioSectionLinks.vue` — `headerMenu[]` (com o toggle `layout.headerMobileActive`) + `links[]` via `BioLinkListEditor`.
- `sections/BioSectionSlides.vue` — slideTop: toggle `slideTop.active`, slides[] em GRID de cards lado a lado, config do carrossel keen.
- `sections/BioSectionStores.vue` — storeLocator (lojas[] em GRID de cards, bounds, openOnQuery) + lightbox (whatsappNumber).
- `BioLinkListEditor.vue` — editor reutilizavel de lista ordenada (label/href/action) em grid de cards. **A ordem do array e a ordem de exibicao**; reordenar com botoes subir/descer.
- `BioMediaField.vue` — campo de midia reutilizavel e compacto: o proprio botao de upload e o PREVIEW (thumbnail `<img>` ou `<video muted>` detectado pela extensao da URL). A URL aparece so no hover (`title`); campo de colar URL manual escondido atras de um toggle. Botao "x" limpa o valor. Aceita **avif** (a extensao avif entra no preview de imagem; o back valida no upload via `media_storage.go`). Props: `label`, `kind`, `accept`, `preview`, `model-value`, `uploading`, `on-upload`, `@update:model-value` e **`duplicateTargets?: BioMediaDuplicateTarget[]`**. **`duplicateTargets` (2026-06-14)**: quando ha valor + pelo menos um alvo, mostra botoes "copiar para" que DUPLICAM o valor atual em outra variante — branding (logo mobile<->desktop) e video (background mobile/desktop + poster). Cada alvo emite a copia para o pai aplicar no campo destino; sem valor ou sem alvos, os botoes nao aparecem.
- `BioSectionCard.vue` — shell visual de uma secao (titulo + slot).
- `BioLivePreview.vue` — **preview via IFRAME pro crow-nuxt (2026-06-15, revertido de volta do nativo)**: usa os MESMOS template/componentes da bio publicada (fidelidade total, sem duplicar render). `<iframe :src="{bioFrontUrl}/bio/preview">`; o conteudo do editor vai por `postMessage` (debounced 300ms) e o iframe re-renderiza ao receber. crow-nuxt agora roda como **servico docker (build de prod, `restart: unless-stopped`)** — fica sempre no ar, entao o problema antigo (preview preto / "carregando infinito" quando o `npm run dev` manual travava) deixou de existir. Idle ~31MB; so renderiza quando o editor pede. Se `NUXT_PUBLIC_BIO_FRONT_URL` (`config.public.bioFrontUrl`) nao estiver setado, mostra aviso em vez do iframe. **Por que reverteu o nativo (B10):** o preview proprio do painel nao tinha a fidelidade do template real; com o crow-nuxt dockerizado a causa que motivou o nativo (dev server caia) sumiu. Os componentes `preview/` (BioPreviewStage/Slides/Links) foram REMOVIDOS. **`source`** (prop) alterna draft (em edicao, padrao) / published (no ar) via switch que emite `update:source`. **(B7)** Quando `slideTop.source.type` e uma fonte (!= manual), resolve os produtos via `store.resolvePreviewSlides` e INJETA em `slideTop.slides` antes do postMessage — mostra os MESMOS produtos que o publico, ANTES de publicar; cacheado por chave dos filtros (`resolvedKey` so grava APOS o await + `pendingKey` descarta resposta de fonte antiga -> reflete NA HORA). `absolutizeUploads`: troca `/uploads/...` por `{apiBase}/uploads/...` (o iframe roda em outro dominio e nao serve `/uploads/`); `/assets/...` (servidas pelo proprio crow-nuxt) NAO sao tocadas — aplicado TAMBEM nos slides resolvidos. `onMessage(__bioPreviewReady)` -> `postDraft()` (re-posta quando o iframe sinaliza pronto). **crow-nuxt roda no host na porta NAO-PADRAO 3300** (`NUXT_PUBLIC_BIO_FRONT_URL`); o servico docker e SO DEV LOCAL — no deploy/VPS o front bio tera container proprio (nao subir o `crow-nuxt` deste compose). **PENDENTE (B12):** a previa ainda NAO reflete a config do carrossel — mudar "Slides por vista" (slidesPerView) e publicar atualiza a pagina publica mas NAO a previa (o slidesPerView nao chega/aplica no `SlideTopKeen` pela rota `/bio/preview`). Bug so no preview; investigar `preview.vue`/`SlideTopKeen.vue` do crow-nuxt + o `slideTop.carousel` enviado no postMessage.

## Contrato da API (codificado contra, sem esperar o back)

- `GET /v1/bio/bios?accountId=&status=&q=` — lista lean `BioSummary`.
- `POST /v1/bio/bios` `{accountId?, slug?, name}` — `accountId` vazio (admin) = account do contexto (a agencia); `slug` vazio = derivado do name.
- `POST /v1/bio/bios/{id}/duplicate` — copia o `data_draft` numa bio nova (status draft, slug unico, name "Copia de ..."). Retorna `{bio}` (ou a `BioView` direta).
- `GET /v1/bio/bios/{id}` — `BioDetail` (dataDraft + dataPublished + meta).
- `PATCH /v1/bio/bios/{id}` `{name?, slug?, dataDraft?, accountId?}` — `dataDraft` substitui o draft inteiro; `accountId` move de account (so `platform_admin`; ignorado para nao-admin).
- `POST .../publish` | `POST .../unpublish` | `DELETE .../{id}`.
- `GET .../preview` — `BioData` mesclado do draft.
- `POST .../media` — multipart `kind`+`file` → `{url}`.
- `GET/PUT /v1/bio/defaults` — so `platform_admin`.

## Regras locais

- Tudo no contrato `BioData` e opcional; as secoes garantem que o objeto aninhado exista antes de gravar (`ensure*`).
- `useBioEditor` mantem um clone profundo do `dataDraft`; dirty-check compara JSON do draft + name/slug com o snapshot salvo. Salvar envia o `dataDraft` INTEIRO (semantica do PATCH).
- **Auto-save**: watch profundo no draft+name+slug salva sozinho (debounce ~800ms) quando dirty — sem botao Salvar. `saveState` (idle/saving/saved/error) alimenta o indicador. `flushSave()` forca o save pendente antes de publicar.
- **Undo**: pilha de snapshots do draft+name+slug (limite ~50), empilhada por mudanca estavel (debounce ~500ms). `undo()` restaura o anterior e deixa o auto-save persistir. `Ctrl+Z`/`Cmd+Z` no editor (fora de input). `canUndo` habilita o botao.
- Tokens do design system (`rgb(var(--primary))`, `var(--text-muted)`, `var(--line-soft)`, `var(--shadow-card)`, `var(--radius-card)`...). Nunca hex hardcoded, nunca emoji.
- Raiz das workspaces com `flex:1; min-height:0; overflow-y:auto` (ou `.page-workspace`) para rolar como as outras paginas.
- Feedback imediato: salvar/publicar desabilita botao + spinner; erros inline; estados vazios orientativos.

## Pendencias de integracao (fase B3 — agente principal, NAO este painel)

Os 4 arquivos de wiring (falha silenciosa se faltar um) sao aplicados na integracao:

1. `web/app/utils/workspaces.ts` — entry `site_bio_web`.
2. `web/app/domain/utils/permissions.ts` — `WORKSPACE_ACCESS_DEFINITIONS` + `site_bio_web` nos `ROLE_WORKSPACES`.
3. `web/layers/queue/nav.config.ts` — child do `site-menu` com `hidden: true`, `moduleId: 'bio'`.
4. `web/app/middleware/module-enabled.global.ts` — `{ prefix: '/site/bio', moduleId: 'bio' }`.

Ate la, as paginas existem mas nao tem item de menu (esperado). Opcional de deploy: `NUXT_PUBLIC_BIO_FRONT_URL` no `runtimeConfig.public.bioFrontUrl` — usado nos links "Ver online" (lista + barra), que montam `bioFrontUrl + '/' + slug` SEM `/bio` (rota publica do subagente D).
