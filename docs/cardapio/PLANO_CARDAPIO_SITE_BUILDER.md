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
