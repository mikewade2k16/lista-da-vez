# Plano — Cardápio Fase 3: Site builder (layout de seções) — Opção B

> **Status: Opção B — Fases 1-3 ENTREGUES + MIGRAÇÃO LAYOUT-DRIVEN CONCLUÍDA (2026-06-21, back + front, ponta a ponta). Falta a Fase 4 (endurecimento).**
> O site TAVOLA (home, cardápio e página de prato) agora renderiza a partir do
> `SiteLayout` (data em runtime, sem deploy), com fallback ao render curado.
> Continuação de `PLANO_MODULO_CARDAPIO.md` e `PLANO_CARDAPIO_FASE2.md`.
> Fontes: `TAVOLA/docs/api-contract.md` (seção "Layout de seções"),
> `TAVOLA/docs/controle-layout-decisoes.md`, `TAVOLA/docs/biblioteca-secoes.md`,
> `TAVOLA/docs/studio.md`. Auditoria do gap: memória `project_tavola_omni_layout_gap`.

---

## 1. O problema

O TAVOLA tem **104 seções** (12 famílias) e um **Studio** (`/studio`) que monta o
layout da home arrastando blocos — hoje salvando só no `localStorage`. O contrato
espera que o **Omni** seja a fonte da verdade: o site lê
`GET /v1/public/restaurants/{slug}/layout?page=home` e renderiza os blocos.

Hoje o Omni **não serve layout** → o TAVOLA cai no `defaultHomeLayout` (fixo).
Tema (`theme`) e catálogo já são servidos; **layout de seções, não.**
(Não confundir com `manage/menu-layout`: aquilo é a navegação do painel admin.)

## 2. Opção escolhida: B (reaproveitar o Studio)

O `/studio` do TAVOLA vira o editor de produção: em vez de `localStorage`, o
layout é **salvo via API no Omni**. Preview WYSIWYG com os componentes reais.
(Alternativa A = editor nativo no Omni, do zero — descartada por ora; C =
import/export manual, serve só pra semear.)

### 2.1 Auth — o achado que simplifica tudo

O contrato genérico do TAVOLA propôs um namespace `/v1/admin/*` com **Bearer
token separado** — isso era pensado para um Studio **externo cross-origin**. Mas
o painel Omni **já autentica** todo `/v1/cardapio/*` por JWT (cookie
`ldv_access_token` + `X-Account-Id`, via `RequireAuth` + `scopedAccountID`), e o
`If-Match` já está liberado no CORS (`httpapi/middleware.go`). Então:

> **A escrita do layout vira mais um endpoint do CRUD do painel** —
> `PUT /v1/cardapio/restaurants/{id}/layout` e
> `POST /v1/cardapio/restaurants/{id}/layout/publish` — sob o **mesmo auth**
> (JWT + `scopedAccountID`) que categorias/zonas. **Zero auth nova.** A leitura
> (`GET /v1/public/restaurants/{slug}/layout`) é pública e cookieless, como o
> resto de `/v1/public/*`.

### 2.2 Integração do Studio (desenho "B4" — token nunca cruza pro iframe)

O Studio é um app do TAVOLA (outra origem). Em vez de mandar o JWT pro iframe
(risco que o próprio contrato alerta), o desenho seguro é:

```
Painel Omni (origem A, tem o JWT)            iframe Studio TAVOLA (origem B, SEM token)
  aba "Site" no editor do cardápio
  <iframe src="{TAVOLA}/studio?slug=X&embed=1">
        │  ← postMessage: { layout inicial }        edita visualmente (WYSIWYG)
        │  → postMessage: { layout atualizado }  ───┘  (no "Salvar"/"Publicar")
        ▼
  o PAINEL faz o PUT/POST autenticado (JWT same-origin)
```

- O **token fica no painel** (parent). O iframe só troca **dados** (o `SiteLayout`)
  por `postMessage`, com **allowlist de origem** nos dois lados. Nunca recebe o JWT.
- O **preview é o próprio Studio** (componentes reais). A rota `/preview?slug=` do
  TAVOLA continua servindo pra ver o **publicado**.
- **CSP:** quem é embutido é o TAVOLA → é o **host do TAVOLA** (Caddy/nuxt) que
  precisa permitir `frame-ancestors <origem-do-painel>`. O `frame-ancestors 'none'`
  do Omni (`security_headers.go`) **não** afeta isso (ele protege páginas do Omni).

> Alternativas (registradas, não escolhidas): **B1** iframe recebendo o token via
> postMessage (mais simples, porém expõe o JWT cross-origin); **B3** Studio
> standalone com login próprio no TAVOLA (UX pior, sem embutir). **B4 é o
> recomendado** (reusa o JWT same-origin do painel e não expõe o token).

---

## 3. Fases

### Fase 1 — Back (Omni) · esforço médio · ✅ ENTREGUE (2026-06-21)
> migration `0170_cardapio_site_layouts.sql`; `model_layout.go`; `store_layout.go`;
> `service_layout.go` (validação estrutural + version); `http_layout.go` (painel) +
> `GET .../layout` público; wiring no `module.go`. gofmt+build+test PASS.
- **Migration** `cardapio.site_layouts`: `id`, `account_id` (FK core.accounts),
  `restaurant_id` (FK cardapio.restaurants, **unique**), `draft jsonb`,
  `published jsonb`, `version bigint` (token de ETag), `created_at`, `updated_at`.
  Idempotente, schema-qualificado, sem `-- +goose Down`.
- **DTOs** (`model_layout.go`): `SiteLayout`, `PageLayout`, `LayoutBlock`,
  `ThemeOverrides` — camelCase EXATO do `TAVOLA/app/types/layout.ts`.
- **Store** (`store_layout.go`): `GetPublishedLayout` (público), `GetDraftLayout`
  + `PutDraftLayout` (painel, retorna `version`), `PublishLayout` (copia draft→
  published, `version++`). Sempre filtra `account_id`.
- **Service** (`service_layout.go`): validação **estrutural** (pages/blocks;
  `id` não-vazio e único por página, gera se vazio; `type` não-vazio; `visible`
  bool; limite de blocos; `theme` com shape válido) + concorrência por `version`
  (If-Match → 412/428).
- **Rotas:** `GET /v1/public/restaurants/{slug}/layout?page=home` (ETag +
  **`Cache-Control: no-cache`** — mudou de `public, max-age=60`: assim publicar
  reflete num F5 do site, sem esperar o cache expirar; o ETag continua evitando
  payload repetido quando nada mudou; **404** se sem publicado → site usa o
  fallback); `PUT /v1/cardapio/restaurants/{id}/layout` (grava draft, `If-Match`
  → **412** se conflito de version); `POST /v1/cardapio/restaurants/{id}/layout/publish`
  (promove). `GET .../layout` do painel (lê o draft, autenticado) para o editor carregar.
- `AGENT.md` do módulo.

### Fase 2 — TAVOLA Studio (modo embed) · esforço médio · ✅ ENTREGUE (2026-06-21)
> `composables/useStudioBridge.ts` + `/studio?embed=1` em `pages/studio/index.vue`
> (esconde export/import/reset/salvar local; aviso "editando pelo painel");
> postMessage canal `omni-studio` (ready/init/change) com origem confiável;
> env `NUXT_PUBLIC_EMBED_PARENT_ORIGIN`. `nuxt generate` PASS. Preview agora
> puxa **dados reais** (`useMenu`) e o layout-semente foi reescrito por página
> (`sections/default/{home,cardapio,produto}.ts`) reproduzindo as páginas curadas.
> **Fix do bridge:** o `change` (Studio→painel) agora envia clone JSON puro — antes
> mandava o proxy reativo do Vue, o que dava `DataCloneError` no `postMessage` e
> salvava layout vazio.
- `useStudio.ts`: aceitar **modo embed** (`?embed=1`): recebe o layout inicial do
  parent por `postMessage` (com origin allowlist), e ao "Salvar/Publicar" **emite**
  o `SiteLayout` pro parent — **sem** tocar em token nem API. Mantém o modo local
  (localStorage) pro dev.
- `pages/studio/index.vue`: ler `slug` (`?slug=`) e `embed`; no embed, esconder
  export/import e trocar "Salvar" por "enviar ao painel".
- Header/Caddy/nuxt: `frame-ancestors` permitindo a origem do painel.

### Fase 3 — Painel Omni (aba "Site") · esforço médio · ✅ ENTREGUE (2026-06-21)
> `components/cardapio/sections/CardapioSectionSite.vue` (iframe do Studio +
> postMessage validado por origem + Salvar/Publicar) + 3 actions no
> `stores/cardapio.ts` (loadLayout/putDraftLayout com If-Match/publishLayout) +
> tipos do layout + env `studioUrl` (NUXT_PUBLIC_STUDIO_URL) + wiring na aba do
> `CardapioEditorWorkspace`. Web dev recompilou sem erro.
- Aba **Site** no `CardapioEditorWorkspace` (entre Aparência e Domínios) com o
  `<iframe>` do Studio + listener `postMessage` (valida origem) que guarda o
  layout recebido; botões **Salvar rascunho** (`PUT`) e **Publicar** (`POST`) via
  `apiRequest` (JWT já vai automático); estados salvando/publicado + erro.
- `stores/cardapio.ts`: `loadLayout/putDraftLayout/publishLayout` (com `withScope`).
- `domain/cardapio/types.ts`: tipos do layout.

### Fase 3.5 — Migração layout-driven do site (Opção B) · ✅ CONCLUÍDA (2026-06-21)
> Detalhe completo no TAVOLA: `TAVOLA/docs/migracao-layout-driven.md`.

Antes, as páginas públicas do TAVOLA eram **curadas/hardcoded** — o `SiteLayout`
existia mas **não controlava o site**. Esta migração fez **home, cardápio e página
de prato** renderizarem a partir do layout (`useSiteLayout` + `SectionRenderer`),
com **fallback ao render curado** quando não há blocos/seção válida. A trava
`?layout=1` foi **REMOVIDA** — render por layout é o **padrão** agora.

- **5 seções novas:** `stats.meta-restaurante`, `menus.sidebar-categorias`,
  `menus.categorias-lista`, `produto.compra`, `depoimentos.lista`.
- **Adaptações data-bound:** hero, categorias, editorial, info e galeria passam a
  consumir dados reais (`restaurant`/`product`/`menu`).
- **Layout-semente por página:** `sections/default/{home,cardapio,produto}.ts`
  reproduzem as páginas curadas (semente p/ o Studio abrir já com a cara real).
- **Sem deploy para editar:** o site lê o layout da API no browser, então editar
  no Studio é **dado em runtime** (não exige deploy). Deploy só para **seção nova
  de código** (ver "Arquitetura — duas camadas" abaixo).

### Fase 4 — Endurecimento (depois, não-MVP)
- `GET /v1/public/sections-catalog` (cópia/proxy do catálogo do TAVOLA) +
  validar `type` contra ele.
- **Gating por plano**: hoje o restaurante **não tem** conceito de plano no Omni;
  exigiria criá-lo. Adiar.
- **Sanitização forte** de `props`/`theme.tokens` (DOMPurify-like, allowlist de
  URL/CSS). No B4 quem edita é o **dono autenticado** (XSS auto-infligido), então
  o risco é baixo no MVP — mas entra aqui para robustez.
- Histórico/versões e rollback.

---

## 3.6 Arquitetura — duas camadas (registre claramente)

O site lê o layout da API **no browser**. Disso decorre a regra que define quando
precisa (ou não) de deploy em produção:

| Camada | O que é | Onde mora | Precisa deploy? |
|---|---|---|---|
| **Conteúdo** | ordem/visibilidade dos blocos, props, tema, conteúdo | layout publicado (banco, servido pela API) | **NÃO** — é dado em runtime; editar no Studio + Publicar já reflete num F5 |
| **Código** | **seção nova** (componente Vue + `SectionDef`) | bundle do TAVOLA | **SIM** — registrar a seção e regenerar o site estático |

> Resumindo: **editar layout = dado (sem deploy); seção nova = código (deploy).**

## 3.7 Débitos e ressalvas (carregar p/ a Fase 4)

- **(a) `produto.compra` acessa o cart store** — exceção **consciente** à regra
  "seção não acessa store". É o único caminho p/ add-to-cart dentro de um bloco.
- **(b) Famílias geradas editadas à mão** — `sections/families/*` e
  `sections/components.ts` do TAVOLA são **GERADOS** por `.work/gen-registry.cjs`,
  mas foram editados à mão nesta entrega. **Se rodarem o gerador, reverte** — antes
  de gerar de novo é preciso atualizar `.work/defs`.
- **(c) Override do Studio no localStorage** — abrir `localhost:3000/studio` **fora**
  do painel grava override no `localStorage` (`tavola:studio:layout`) que **mascara
  a API**. No diagnóstico, limpar essa chave (ou usar o embed, que não persiste local).
- **(d) Tema real no preview do Studio** está sendo ajustado **em paralelo** (outra
  trilha) — o preview embed já puxa dados reais, mas o tema fiel ainda está em curso.

## 4. Decisões a confirmar (antes da Fase 1)

1. **Integração = B4** (token no painel; iframe só troca dados por postMessage). ✅ recomendado.
2. **Draft + Publicar** (o site só muda ao publicar) vs publicar direto. ✅ recomendo draft+publicar.
3. **MVP** entrega Fases 1-3 (montar/salvar/publicar layout); **gating de plano,
   sanitização pesada e sections-catalog** ficam na Fase 4. ✅ recomendo.

## 5. Notas de Deploy (quando implementar)
- Migration nova (próximo número livre) + **rebuild api**.
- TAVOLA: env/Caddy com `frame-ancestors` da origem do painel; **rebuild web**
  do TAVOLA (o Studio embed); o `/studio?embed=1` precisa estar acessível pela
  origem que o painel embute (URL configurável no painel).
- Omni painel: rebuild web (aba nova).
- Env nova no painel: URL base do Studio do TAVOLA (pro `iframe src`).

## 6. Próximo passo
Confirmar as 3 decisões da seção 4 e então abrir a **Fase 1 (back)** — é a base
das outras duas e não depende do desenho visual. Fases 2 e 3 podem ser paralelas
depois da 1.

---

## 7. Fase 5 — Multi-página no Studio + edição inline de texto (em entrega 2026-06-21)

> **Status:** 5A multi-página ✅ (`setPage`/`pages` no `useStudio` + seletor
> Home/Cardápio/Prato no `StudioPreview`). 5B mecanismo ✅ (`composables/useInlineEdit.ts`
> + `StudioPreview` com `data-block-id` e realce). 5B anotação `data-edit` ✅ em
> ~112 componentes (4 subagentes, por família; só texto de bloco, data-bound pulado).
> **Iteração 2 (2026-06-21):** (a) o mecanismo grava SÓ se o texto mudou (`useInlineEdit`
> guarda o valor no focus) — não congela um default ao focar/desfocar; (b) `TSectionHead`
> ganhou a prop `editKey` → ~20 eyebrows de seção viraram inline (2 subagentes); (c)
> **header real no Studio** — o `StudioPreview` renderiza `PubHeader`/`PubFooter`
> data-bound (chrome REAL = site) e os seeds `default/{home,cardapio,produto}.ts` não
> têm mais `navegacao.*` (decisão do dono = **Opção 1**: header é chrome, não bloco
> editável; logo = nome do restaurante via Dados). Build PASS nas duas iterações.
> **Pendente:** rebuild/upload da TAVOLA pelo dono. **Limites que ficam:** *placeholders*
> de formulário (atributo de `<input>`, não aceitam contenteditable) e `TQuoteBlock
> :author` seguem só no painel; **header editável como seção** = Opção 2 (não pedida).

> Pedido do dono: editar **todas as páginas** (não só a home) e **editar o texto
> das seções clicando** direto no preview. Tudo **TAVOLA-side** (Studio); backend,
> painel e bridge ficam **inalterados** (já tratam o `SiteLayout` inteiro — `init`
> carrega e `change` emite todas as páginas). É **CÓDIGO** → exige rebuild + upload
> da TAVOLA (sem migration, sem backend). Decisões do dono: inline **de verdade**
> (contenteditable) + executar com subagentes.

### 5A — Multi-página (seletor de página no Studio) · pequeno
- `useStudio`: `pageName` deixa de ser fixo em `'home'`; adicionar `setPage(name)` e a
  lista de páginas editáveis (`home`, `cardapio`, `produto`). `blocks`/`ensurePage` já
  operam por `pageName` — só faltava poder trocar.
- **Seed multi-página:** garantir que o layout-semente tenha as 3 páginas (de
  `sections/default/...`), pra um restaurante novo já abrir com as 3 editáveis.
- **UI:** seletor de página no topbar do Studio (Home / Cardápio / Prato). Trocar a
  página re-renderiza o preview e a lista de blocos daquela página.
- **Zero** mudança em backend/painel/bridge.

### 5B — Edição inline de texto (contenteditable no preview) · médio-grande

> **⚠️ Achado (recon 2026-06-21) — texto data-bound vs texto de bloco:** muitas
> seções são **data-bound** — o texto exibido vem do **restaurante/produto** (nome,
> tagline, descrição, preço), NÃO de `block.fields` (que viram só fallback). Ex.:
> `hero.cinematografico` mostra `restaurant.tagline/name/description`; o menu mostra
> nome/preço dos produtos. Consequências:
> - Inline **funciona** para o texto que renderiza de `block.fields` (CTAs, copy
>   editorial, títulos/kickers custom de seções não-data).
> - Texto **data-bound** é editado em **Dados/Produtos** (onde já funciona). Editá-lo
>   inline **pelo preview** exigiria o Studio **escrever na API de cardápio** — mas o
>   embed **não tem token** por design (B4, o painel é quem escreve). Logo,
>   inline-on-data é uma **fase à parte** (maior), não o MVP.
>
> **MVP do 5B = inline só nos campos `text`/`textarea` que de fato renderizam de
> `block.fields`** (o agente verifica por componente; não anota texto data-bound).

**Convenção (CONTRATO entre mecanismo e seções) — fonte única:**
- O **SectionRenderer** envolve cada bloco com `data-block-id="<id>"`.
- Cada componente de seção marca seus **textos editáveis** com `data-edit="<fieldKey>"`,
  onde `<fieldKey>` = a `key` do campo correspondente no `SectionDef` (ex.: um `<h2>`
  de título ganha `data-edit="title"`). Só texto **plano** (título, subtítulo, kicker,
  descrição, label, preço-texto…); imagem/lista/estrutura continuam no painel lateral.

**Mecanismo (StudioPreview em modo edição):**
- Ao montar/atualizar, para cada `[data-edit]` dentro de um `[data-block-id]`: liga
  `contenteditable="plaintext-only"`, realça no hover/focus.
- No `input`/`blur` (debounce ~200ms): lê `textContent`, sobe até o `data-block-id`
  ancestral e o `data-edit` (key), e grava via `s.updateField(blockId, key, texto)`.
- Só texto plano (sanitiza paste removendo HTML). **Evitar loop:** não re-renderizar o
  nó enquanto ele está em foco/edição (o writeback já dispara o `change` da bridge
  debounced → o painel salva; a re-render só vale quando o campo perde foco).

**Workstreams (subagentes):**
- **Mecanismo** (keystone — feito sem paralelizar): `StudioPreview` edit-mode +
  `SectionRenderer` `data-block-id` + util de writeback/sanitização.
- **Anotações** (N agentes em paralelo, arquivos disjuntos por família): adicionar
  `data-edit` às seções **usadas nos seeds** (home/cardapio/produto) primeiro; demais
  famílias depois. Partição por pasta `components/library/<família>/`.

### Notas de Deploy (Fase 5)
- TAVOLA: **rebuild + upload** (o Studio embed faz parte do bundle servido). Sem
  migration, sem rebuild de api/painel.
- Atualizar `TAVOLA/docs/studio.md` (hoje diz "não tem multi-página") e o `AGENT.md`
  da TAVOLA quando entregar.

---

## 8. Fase 6 — Melhorias de UX/editor do Studio (ENTREGUE 2026-06-22)

> Lote pedido pelo dono. **Execução: Workflow (governador = Opus no main loop) —
> keystone PRIMEIRO (estado do `useStudio` + protocolo da bridge), depois fan-out de
> UI em paralelo, e build+review no fim.** Tudo segue a skill `principios-engenharia`.
> Opção 2 (header 100% editável, nav/botão inline) fica no **roadmap**.
>
> **ENTREGUE 2026-06-22** via Workflow (7 agentes: keystone Opus + UI Sonnet) + review
> adversarial Opus. Decisão do dono em W7: **header = SEÇÃO data-bound** (templates
> escolhíveis na biblioteca; logo = nome do restaurante; fallback PubHeader/PubFooter
> quando a página não tem header block — Studio E site). `nuxt build` verde. Correções
> pós-review (governador): W5 links inertes movidos p/ **fase de captura** (cobre
> NuxtLink/`TButton :to`, não só `<a href>`); `reset()`/`importJson()` agora limpam o
> histórico undo/redo; hardcode `#c9a86a` → const `DEFAULT_ACCENT`. **Pendente:**
> rebuild + upload da TAVOLA pelo dono.

### Workstreams
| # | O quê | Arquivos | Modelo sugerido |
|---|---|---|---|
| **K (keystone)** | `useStudio`: `addAboveSelected` (W3), `reorder(from,to)` (W2), **histórico undo/redo** (W4) + protocolo da bridge (`undo`/`redo`/`history`) | `composables/useStudio.ts`, `composables/useStudioBridge.ts` | **Opus** |
| **W1** | Biblioteca: tirar badge de plano (Grátis/Pro); item minimalista (ícone+nome, sem card largo) → SEM rolagem lateral; fontes menores | `components/studio/StudioSectionLibrary.vue` | Sonnet |
| **W2-ui** | Lista da direita compacta (sem rolagem lateral) + **drag-n-drop** consumindo `reorder()` | `components/studio/StudioLayoutList.vue` | Sonnet |
| **W4-ui** | Botões Desfazer/Refazer (no painel, imagem 1) + atalhos Ctrl+Z / Ctrl+Shift+Z | `StudioPreview`/topbar + `CardapioSectionSite.vue` (painel) | Sonnet |
| **W5** | Links inertes no preview: `<a>`/NuxtLink NÃO navegam (clicar produto não sai do Studio) | `components/studio/StudioPreview.vue` | Sonnet |
| **W6** | Tela cheia: botão que põe o editor em fullscreen | `web/app/components/cardapio/sections/CardapioSectionSite.vue` (Omni) | Sonnet |
| **W7** | **Header (a decidir)** | depende da decisão | Opus |
| **Roadmap** | **Opção 2** — header editável como seção (nav/botão inline + usado no site real) | — | depois |

### Ordem (governador)
1. **K (Opus)** — sozinho, define a API nova do `useStudio` + bridge (sem isso o resto não tem o que consumir).
2. **Fan-out (Sonnet, paralelo)** — W1, W2-ui, W4-ui, W5, W6 (arquivos disjuntos).
3. **Verify** — `nuxt build` da TAVOLA + review do diff + sync docs.

> Conflito resolvido: W2/W3/W4 tocam `useStudio.ts` → tudo isso é o **keystone (K)**,
> feito por UM agente Opus primeiro; os agentes de UI só consomem a API.

---

## 9. Fase 7 — Controle de layout + upload + hero custom (ENTREGUE 2026-06-22)

> **Status (2026-06-22):** entregue via Workflow (11 agentes: KA/KB/KC Opus + F1/fan-out
> Sonnet) + review adversarial Opus. Build verde. **Fix pós-review (governador):** o hero
> data-bound mostrava SEMPRE o texto do template (defaults não-vazios do def mascaravam o
> fallback) → agora o `SectionRenderer` passa os props CRUS e o hero prioriza **override
> do usuário → dado real do restaurante → default do def** (mostra dados reais por padrão;
> edição inline sobrescreve). Upload cross-iframe aprovado (requestId/origin/timeout,
> `store.uploadMedia` real). Ressalva (média): o controle "Colunas" aparece p/ famílias de
> grid, mas seções de layout FIXO (2 partes) dessas famílias ignoram `--cols-*` (inerte
> nelas) — as de GRADE consomem. **Pendente:** rebuild + upload da TAVOLA pelo dono.

> Pedido do dono. **Workflow (governador Opus): keystone → fan-out → review.** Decisões:
> **hero = custom inline** (o bloco VENCE o dado do restaurante; dado vira o default);
> **qtd-por-linha desktop/mobile é prioridade**. Header duplicado = rascunho local
> (Reset limpa) + swap fix em `addBlock` (header/footer = 1 por página, TROCA em vez
> de duplicar) — JÁ feito. Tudo segue `principios-engenharia`.

### Workstreams
- **KA (Opus) — layout por bloco (genérico):** props de bloco `colsDesktop` (1–6) /
  `colsMobile` (1–3) e espaçamento `mt`/`mb`/`py`/`px`. `SectionRenderer` envolve cada
  seção aplicando margem/padding e expõe `--cols-d`/`--cols-m` na seção. `StudioBlockEditor`
  ganha um painel "Layout" (espaçamento p/ todas; colunas p/ famílias de grid). Props
  de bloco (NÃO defs gerados).
- **KB (Opus) — upload de imagem (cross-iframe):** `StudioField` (tipo `image`) ganha
  botão de upload → `postMessage 'upload-request'` (com o arquivo) → painel
  (`CardapioSectionSite.vue`) sobe pela API de upload do cardápio (tem o JWT) → devolve
  `'upload-result' {url}` → `StudioField` grava a URL no campo. Bridge relê as msgs.
- **KC (Opus) — hero custom inline:** `heroes/*.vue` passam a deixar o BLOCO vencer o
  dado (dado = default quando o campo está vazio); título/eyebrow/lede com `data-edit`
  (inline); imagem do bloco (uploadável via KB).
- **F1 (Sonnet) — biblioteca com collapses** por família (abre/fecha cada grupo).
- **Fan-out colunas (Sonnet):** grids/sliders/menus/produto/categorias leem
  `--cols-d`/`--cols-m` e aplicam no `grid-template-columns` (desktop + media mobile).

### Ordem (governador)
KA / KB / KC / F1 em paralelo (arquivos disjuntos) → **fan-out de colunas após KA**
(consome `--cols-*`) → build + review adversarial. Eu reviso e integro no fim.

---

## 10. Fase 8 — Polish do site público (chrome real, logo-imagem, footer com dados, tamanho de imagem, mobile)

> **Status: CÓDIGO ENTREGUE (2026-06-22) via Workflow (8 agentes) + fixes do governador.
> `nuxt build` PASS + typecheck limpo. Pendente rebuild+upload da TAVOLA pelo dono.**
> Lote pedido pelo dono em cima das Fases 6/7 (ainda **pendentes de rebuild+upload da
> TAVOLA** — o deploy desta fase leva as três juntas). Mapeamento de causa-raiz feito
> por workflow de 6 leitores + síntese adversarial. **Achado-chave do deploy: os 5
> problemas são 100% front TAVOLA — ZERO migration, ZERO rebuild da api.** O backend
> já entrega `logoUrl` absoluto, `address/hours/phone/email` e o layout jsonb com
> `props` livres.
>
> **Entrega:** keystone (chrome único + hook da logo) ‖ tamanho de imagem → header
> (logo-img + hamburger) ‖ footer (data-bound + logo-img) → build + 3 reviews
> adversariais. Reviews chrome+imagem e responsivo **aprovados**; logo+footer reprovou
> por 1 regressão (`LibFooterMinimalCentralizado` → `[object Object]`) **corrigida pelo
> governador**, junto da blindagem de overflow do nome/logo longo nos 5 headers em
> `<360px` (`min-width:0`+ellipsis no `.logo` colapsado + `max-width` na `.logo-img`).
> **Notas de Deploy ATUALIZADAS:** além de build+upload TAVOLA, **recachear o
> `public/sections-catalog.json` no Omni** (footers viraram `dataBinding:restaurant`;
> version `888e82547eaa`→`e2704a0710d3`). **Débito carregado:** família `info`
> dessincronizada (`info.ts` gerado tem `dataBinding` ausente dos `.work/defs/info-*.json`)
> — portar antes de rodar `gen-registry.cjs`, senão reverte.
>
> **Ajuste 2 (decisão do dono, 2026-06-22) — chrome = só PubHeader/PubFooter:**
> após validação no browser (a logo real subiu pelo painel e funcionou), o dono
> pediu para **remover o header/footer "do Studio"** e usar **só o `PubHeader`/
> `PubFooter` padrão** (que já são data-bound: logo-imagem + endereço/horários/
> contato reais). Mudanças: `layouts/public.vue` e `components/studio/StudioPreview.vue`
> renderizam SEMPRE PubHeader/PubFooter (não mais o bloco `navegacao.*`); os seeds
> `sections/default/{home,cardapio,produto}.ts` perderam os blocos `b_header`/
> `b_footer`. Blocos `navegacao.*` que sobrem em layouts já salvos (ex.: `mk`) ficam
> **inertes** (ignorados no render). Isso REVERTE a decisão da Fase 6 ("header como
> seção escolhível") e torna o "dois headers" estruturalmente impossível (só existe
> um componente de chrome). **Consequência boa p/ deploy:** como o site passa a usar
> o `PubFooter` (data-bound nativo), o **recache do `sections-catalog.json` deixa de
> ser necessário** p/ o footer mostrar dados reais — o WS-3 (footer data-bound via
> defs) fica como trabalho não-usado no render (a família `navegacao` segue no
> catálogo, sem uso). `nuxt build` PASS.

### 10.1 Problema 1 — DOIS headers no site live (Studio mostra só 1)
- **Causa-raiz:** o header é, ao mesmo tempo, **(a)** um bloco dentro da MESMA lista de
  blocos da página e **(b)** algo que o layout-wrapper renderiza por fora. No site live a
  responsabilidade é DIVIDIDA: `layouts/public.vue` (chama `useSiteLayout`, renderiza
  header da seção OU `PubHeader` — linhas 73-74 = header #1) e a página
  (`pages/index.vue:35`, `cardapio.vue:19`, `prato/[productSlug].vue:50` chamam
  `useSiteLayout` **de novo** e filtram conteúdo por string-match frágil
  `!b.type.startsWith('navegacao.')`). Quando a separação não bate (header com `type`
  fora do prefixo `navegacao.`, header duplicado na lista publicada, ou `pageName`
  derivado da rota ≠ nome hardcoded `'home'/'cardapio'/'produto'`), empilha 2 `<header>`.
  No Studio não acontece: `StudioPreview.vue` é o **dono único** da montagem e o
  `layouts/studio.vue` é casca vazia. Agravante: as 2 instâncias de `useSiteLayout`
  compartilham a chave fixa `useAsyncData('site-layout')`.
- **Onde corrige:** 100% front TAVOLA. **precisaBackend: não.**
- **Passos:** (1) estender `useSiteLayout` p/ devolver `{headerBlock, contentBlocks,
  footerBlock}` já fatiado por página (fonte única); (2) `public.vue` é o ÚNICO a
  renderizar header/footer; páginas consomem só `contentBlocks` e nunca renderizam
  bloco `navegacao.*`; (3) endurecer o filtro por **família** `navegacao` (não por
  prefixo de string) e escolher 1 e só 1 `headerBlock`; (4) extrair `pageName` p/ um
  helper único usado por layout E páginas; (5) chave `useAsyncData` por página (ou
  compartilhar a mesma instância); (6) espelhar no `StudioPreview` + AGENT.md da TAVOLA.

### 10.2 Problema 2 — LOGO renderizada como TEXTO (nunca a imagem enviada)
- **Causa-raiz:** o dado existe ponta-a-ponta (`banco logo_url → API logoUrl
  absolutizado em service_public.go:164 → Restaurant.logoUrl no front`), mas **nenhum
  chrome consome** `restaurant.logoUrl`: `PubHeader.vue:22`, `PubFooter.vue:20/44` e
  todos os `Lib*Header/Lib*Footer` imprimem `restaurant.name.toUpperCase()` via
  `useSectionBrand.ts:36`. É ausência de consumo, não falta de dado.
- **Onde corrige:** 100% front TAVOLA. Upload de logo já existe no painel
  (`CardapioSectionDados.vue:60-80 → POST .../media`). **precisaBackend: não.**
- **Passos:** (1) estender `useSectionBrand` (ou criar `useSectionLogo`) p/ expor
  `logoUrl` mantendo a cadeia `block.data.restaurant → inject('sectionData')`;
  (2) renderizar `<img :src=logoUrl :alt=name>` com **fallback no texto** do nome;
  (3) tamanho de exibição controlado por CSS var (conecta com Problema 5); (4) validar
  no `StudioPreview` + AGENT.md. **Compartilha os arquivos de header com o Problema 5 →
  mesmo agente ou coordenação.**

### 10.3 Problema 3 — FOOTER com endereço/horário/contato FAKE do template
- **Causa-raiz:** o footer ativo é o bloco `navegacao.footer-multicoluna`
  (`LibFooterMulticoluna.vue`, default em `sections/default/home.ts:44`). Ele lê
  endereço/horário/telefone/email de `block.fields` (defaults fixos do fake "Tavola
  Bistro": *R. Bandeira Paulista 514, reservas@tavola.bistro*), **nunca** de
  `block.data.restaurant`. Estrutural: a família `navegacao` **não declara
  `dataBinding`** (defs fonte `.work/defs/navegacao-1.json`/`navegacao-2.json`), ao
  contrário de `info` (`source:restaurant`). Por isso o `SectionRenderer` não injeta
  `block.data.restaurant` no footer. O NOME aparece real só porque `useSectionBrand`
  usa um 2º caminho (`inject('sectionData')`). O `PubFooter` é data-bound, mas só
  renderiza como fallback quando NÃO há bloco `navegacao.footer-*`.
- **Onde corrige:** front TAVOLA (caminho comum). Backend/painel já entregam tudo.
  **precisaBackend: não** (só se o requisito virar "lojista escolhe campos do footer" =
  `settings.footer` estruturado, aí entra Go).
- **Passos (ordem obrigatória):** (1) adicionar `dataBinding {source:'restaurant'}` aos
  defs **FONTE** `.work/defs/navegacao-1.json`/`navegacao-2.json` (NÃO editar os `.ts`
  gerados); (2) `LibFooterMulticoluna` (e `LibFooterFaixaDupla`/`LibFooterComNewsletter`)
  lerem `block.data.restaurant.{address,hours,phone,email,instagram}` com fallback nos
  fields; (3) rodar `node .work/gen-registry.cjs` e versionar os GERADOS
  (`sections/families/navegacao.ts`, `components.ts`, `public/sections-catalog.json`);
  (4) **recachear o `sections-catalog.json` no Omni** (o painel busca/cacheia p/ validar
  layout). **Alternativa barata:** trocar o footer default por `PubFooter` (já
  data-bound) — perde edição no Studio. (0) **Validar antes:** `GET
  /v1/public/restaurants/{slug-mostarda}` p/ confirmar que os campos estão preenchidos
  (se vazios, há 2º motivo no cadastro). Endpoint público, sem credencial.

### 10.4 Problema 4 — TAMANHO/proporção de imagem (itens/categorias) não configurável
- **Causa-raiz:** o tamanho é constante de apresentação cravada no componente: `ratio`
  literal no `TPlaceholder` (`ratio="3x4"`, `"1x1"`…) ou `aspect-ratio`/`width` fixo em
  CSS escopado (`TDishCard.vue:44`, `LibMenuComFotosPequenas.vue:55`). Nenhum lê
  `block.props`. **Já existe o precedente exato:** `colsDesktop/colsMobile` viram CSS
  vars `--cols-d/--cols-m` no `SectionRenderer.blockStyle`. `TPlaceholder` já aceita a
  prop `ratio` (só recebe valor fixo do template).
- **Onde corrige:** front TAVOLA. `LayoutBlock.props` é jsonb livre e `validateSiteLayout`
  não valida props → **precisaBackend: não.**
- **Passos:** (1) prop genérica por bloco `imageRatio` (1x1/4x5/3x4/4x3/16x9) e opcional
  `imageScale` (none/sm/md/lg), vivendo em `block.props` como `mt/mb/cols`;
  (2) `StudioBlockEditor` grupo "Layout" ganha `TSelect`; (3) `SectionRenderer.blockStyle`
  emite `--img-ratio`/`--img-h`; (4) componentes trocam `ratio` literal por
  `:ratio="f.imageRatio ?? 'XxY'"` e `<img>` direto usa `var(--img-ratio, 4/5)`.
  **Decisão de produto:** GLOBAL por bloco (recomendado, barato) vs por SEÇÃO via
  registry (exige editar `.work/defs` e regenerar). Resize/compressão de UPLOAD (peso) =
  escopo separado no handler Go `POST .../media`.

### 10.5 Problema 5 — Header não responsivo (sem hamburger, quebra no mobile)
- **Causa-raiz:** nenhum dos 5 headers (`PubHeader` + 4 `LibHeader`) tem hamburger; o
  padrão é esconder a nav (`display:none`) sem substituto — no celular some "A casa"/
  "Cardápio". Agravantes: `PubHeader.vue:44` usa grid `auto/1fr/auto` que não recolhe a
  coluna central (vão morto); brand `white-space:nowrap` 22px estoura com nome longo;
  tipografia px sem `clamp()`; **bug de tokens** — `TSection` e vários `Lib*` usam
  `--space-10/14/20/32` que **não existem** nos temas (só 1/2/3/4/6/8/12/16/24) → padding
  colapsa. Tema só controla cores/fontes/raio (`useRestaurantTheme.ts`), nunca escala.
- **Onde corrige:** 100% front TAVOLA. **precisaBackend: não** (escala tipográfica por
  tema seria opcional, e o `theme` jsonb é livre — ainda sem migration).
- **Passos:** (1) hamburger real nos 5 headers (toggle `aria-expanded/aria-controls` +
  drawer; reusar `TDrawer`); (2) `PubHeader` colapsa p/ 2 colunas no mobile + brand sem
  `nowrap`; (3) corrigir tokens `--space-10/14/20/32` (adicionar nos temas ou trocar p/
  escala existente); (4) tokenizar tipografia com `clamp()` (como `.d-*`); (5) padronizar
  breakpoints (hoje 700/768/980 convivem); (6) `LibHeaderLojaComCarrinho` empilha busca em
  2ª linha; (7) espelhar no `StudioPreview` + AGENT.md.

### 10.6 Decisão de fonte (princípio fonte única) — confirmado
- **Logo:** dado do tenant → upload no painel → banco `cardapio.restaurants.logo_url` →
  API `logoUrl` → TAVOLA renderiza `<img>`. **Nunca hardcoded no TAVOLA** (nem arquivo
  bundlado, nem string fixa). Você guarda a logo **no painel**, não na pasta do TAVOLA.
- **Footer (endereço/horário/contato):** dado do tenant, já no banco e já na API; aparece
  fake só porque o footer lê `block.fields` em vez de `block.data.restaurant`.
- **Tamanho de imagem:** config de APRESENTAÇÃO → fonte é o `SiteLayout` (jsonb
  `cardapio.site_layouts`) via `block.props`, servido por `/v1/public/.../layout`.

### 10.7 Notas de Deploy (Fase 8)
- **Problemas 1, 2, 4, 5:** SÓ front TAVOLA. `nuxt generate` (SPA, ssr:false) + upload do
  TAVOLA. **ZERO migration, ZERO mudança no Go, ZERO rebuild da api.**
- **Problema 3 (caminho estrutural):** editar defs FONTE `.work/defs/navegacao-*.json` →
  rodar `node .work/gen-registry.cjs` → versionar os GERADOS → **recachear o
  `public/sections-catalog.json` no Omni** após o deploy do TAVOLA. Sem migration/rebuild
  api. (Caminho barato = trocar footer por `PubFooter`, sem gerador/catálogo.)
- **Resumo:** nenhum problema exige banco. Deploy desta fase = **build + upload TAVOLA**.

### 10.8 Workstreams / paralelização (subagentes)
| WS | O quê | Arquivos | Dependência |
|---|---|---|---|
| **WS-1 (keystone)** | Fonte única do chrome (corrige 2 headers) | `useSiteLayout.ts`, `layouts/public.vue`, `pages/{index,cardapio,prato}.vue`, `StudioPreview.vue` | **PRIMEIRO** (base de WS-2/WS-5) |
| **WS-2** | Logo-imagem no chrome | `useSectionBrand.ts`, `PubHeader/PubFooter`, `Lib*Header/Lib*Footer` | após/junto WS-1; **coordena com WS-5** (mesmos headers) |
| **WS-5** | Hamburger + responsivo + tokens de espaço | `PubHeader`, 4 `LibHeader`, `tavola.css`/`brasa.css`, `TSection`, `TButton` | após/junto WS-1; **coordena com WS-2** |
| **WS-4** | Tamanho de imagem por bloco | `SectionRenderer.blockStyle`, `StudioBlockEditor`, `TPlaceholder`, `Lib*` grid/menu, `TDishCard` | **independente** (paralelo livre) |
| **WS-3** | Footer data-bound | `.work/defs/navegacao-*.json` + gerador + `LibFooter*` | **independente**; sequencial interno (defs→gerar→componentes→recachear) |

> **Ordem (governador):** WS-1 primeiro (chrome único) → WS-2+WS-5 juntos nos headers →
> WS-4 e WS-3 totalmente em paralelo desde o início. Build do TAVOLA + review
> adversarial no fim. Tudo segue a skill `principios-engenharia`.
