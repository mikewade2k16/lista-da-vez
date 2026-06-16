# Iteração B6 — Refinamento UX do editor de Bio

> Specs para subagentes Opus (paralelo). Doc canônico do módulo: docs/bio/PLANO_MODULO_BIO.md.
> Criado 2026-06-13. Status: PLANEJADO — aguardando ok para disparar.

Pacote de refinamentos pedido pelo usuário sobre o módulo bio já entregue (B1-B5).
Dividido em 4 subagentes com territórios sem conflito. O contrato dos endpoints
novos está congelado aqui para A (back) e B (painel) trabalharem em paralelo.

## Decisões de design (validar antes de disparar)

- **D1 — "Sem cliente"**: o campo Cliente no modal vira OPCIONAL. Vazio = a bio
  pertence à **account ativa do admin** (a agência, ex.: Crow). Pode trocar
  depois. (Bio sempre tem uma account — FK NOT NULL; "sem cliente" = account da
  agência, não NULL.)
- **D4 — Após criar**: o modal tem **DOIS botões** — "Criar" (cria e volta à
  lista) e "Criar e editar" (cria e abre o editor). Sem obrigatoriedade de ir ao
  editor.
- **D6 — Salvamento**: **auto-save do rascunho** (debounced ~800ms) — some o botão
  "Salvar". **Publicar/Despublicar continuam manuais** (ir ao ar é decisão). Some
  o botão "Previa" (a prévia ao vivo já é a prévia).
- **BUG SEPARADO (investigar na integração)**: "o site real (crow-nuxt) só
  atualiza depois de clicar Republicar 2x — algo sobre recarregar", em especial o
  fundo (bg). NÃO é o draft-não-salvo (auto-save não resolve isto). Hipótese:
  timing entre o publish, o `broker.notify` (SSE) e o refetch/cache do crow-nuxt,
  ou cache do browser da mídia. Diagnosticar o ciclo real e corrigir (não assumir
  resolvido).

## Contrato dos endpoints novos (back ↔ painel)

| Verbo | Path | Body / efeito |
|---|---|---|
| POST | `/v1/bio/bios` | `{accountId?, slug?, name}` — `accountId` vazio (admin) = account do contexto; `slug` vazio = derivado do name |
| POST | `/v1/bio/bios/{id}/duplicate` | Copia `data_draft` da origem numa bio nova (status draft, novo slug `{slug}-copia` único, name `Cópia de {name}`), mesma account (ou `{accountId?}` p/ admin). Retorna a BioView nova |
| PATCH | `/v1/bio/bios/{id}` | passa a aceitar `{accountId?}` (mover de account) — **só platform_admin**; não-admin que mandar `accountId` → ignorado |

---

## Subagente A — Backend Go (`back/internal/modules/bio/`)

Território exclusivo: `back/internal/modules/bio/*`. Nenhum arquivo de front.

1. **Create com cliente opcional**: `service.Create` já usa o contexto quando
   admin não manda `accountID`. Garantir: admin sem `accountId` → usa o
   X-Account-Id do contexto (a agência). Slug vazio → derivar do name
   (`normalizeSlug` + sufixo numérico se colidir). Validar.
2. **Duplicate**: `POST /v1/bio/bios/{id}/duplicate` + `service.Duplicate` +
   `store.Create` reaproveitado. Copia `data_draft` (não o published), status
   draft, slug único (`{base}-copia`, `-copia-2`...), name `Cópia de {name}`.
   Auto-habilita módulo na account (já faz no Create — reusar `EnsureBioModuleEnabled`).
3. **Mover de account** (só admin): `PATCH` aceita `accountId`; `service.Patch`
   troca `account_id` apenas se `isAdmin`. Slug é global, então não colide.
4. Testes: duplicate (slug único, copia draft), create sem accountId (usa
   contexto), patch accountId só admin.
5. Validação: `go build/vet/test` + `golangci-lint` 0 issues; AGENT.md.

## Subagente B — Painel: core do editor + lista/modal/store

Território: `web/app/components/bio/BioListWorkspace.vue`,
`BioCreateModal.vue`, `BioEditorWorkspace.vue`, `BioPublishBar.vue`,
`BioLivePreview.vue`, `web/app/composables/useBioEditor.ts`,
`web/app/stores/bio.ts`. **NÃO tocar** `sections/*`, `BioMediaField.vue`,
`BioLinkListEditor.vue`, `BioSectionCard.vue`, `domain/bio/types.ts` (são do C).

1. **Modal**: Cliente opcional (placeholder "Sem cliente (agência)"); slug
   opcional. **DOIS botões**: "Criar" (cria e volta à lista) e "Criar e editar"
   (cria e `navigateTo` editor). Sem auto-redirect obrigatório.
2. **Lista**: coluna AÇÕES ganha **Duplicar** (ícone copy → `store.duplicateBio`
   → editor) e **Ver online** (link `bioFrontUrl + '/' + slug` em nova aba — ver
   D13: sem `/bio`). Mostrar a account/cliente (já mostra) e permitir filtro
   (já tem). Coluna cliente: para admin, editável inline OU no editor (item 4).
3. **Store**: `duplicateBio(id)`, `createBio` com accountId opcional,
   `moveBioAccount(id, accountId)` (PATCH accountId).
4. **Editor (BioEditorWorkspace)**: seletor de **Cliente** editável no topo (só
   admin) → `moveBioAccount`. **Auto-save** (debounced) no `useBioEditor` (some o
   botão Salvar; indicador "Salvo"/"Salvando"). **Undo (Ctrl+Z)**: stack de
   snapshots do draft no `useBioEditor` (botão Desfazer + atalho).
5. **BioPublishBar**: remove "Salvar" e "Previa". Mantém **Republicar**
   (publica alterações) e **Despublicar**. Adiciona **switch "Editando ↔
   Publicado"** que troca o que o preview mostra (draft vs `dataPublished`) —
   para comparar. Indicador de auto-save.
6. **BioLivePreview**: aceitar uma prop `source: 'draft' | 'published'` e enviar
   ao iframe o draft (atual, com absolutize já feito) ou o `dataPublished`.
7. Validação: eslint 0 erros + vue-tsc limpo nos arquivos tocados.

## Subagente C — Painel: upload-com-preview + layout compacto das seções

Território: `web/app/components/bio/BioMediaField.vue`, `sections/*`,
`BioLinkListEditor.vue`, `BioSectionCard.vue`, `web/app/domain/bio/types.ts`,
`AGENT_RULES.md` (regra de layout). **NÃO tocar** o shell/editor/store (são do B).

1. **BioMediaField com PREVIEW**: o botão de upload mostra o **preview da mídia**
   (thumbnail de imagem OU `<video>` mudo de vídeo). A URL aparece só no
   **hover** (tooltip/título), não ocupando linha. Detecta img vs vídeo pela
   extensão/tipo. Usado em TODAS as seções (slides, vídeo/bg, branding, lojas).
2. **Vídeo e layout simplificado**: apenas **Background (mobile)**, **Background
   (desktop)** e **Poster** — cada um aceita imagem OU vídeo; o upload detecta o
   tipo e grava em `bgVideo`/`bgImage` (mobile) e `bgVideoPc`/`bgImage Pc`
   conforme o tipo. (Coordenar com o contrato: se vídeo → bgVideo; se imagem →
   bgImage.) Remover os campos separados atuais.
3. **Remover toggles duplicados** da seção Vídeo: "Slide do topo ativo" (fica só
   na seção Slides) e "Header mobile ativo" (vai para a seção Links e menu).
4. **Layout compacto/horizontal** em TODAS as seções (regra abaixo): campos de um
   item na MESMA linha (grid horizontal), lojas/slides em GRID lado a lado (não
   uma por linha), reduzir o espaço vertical ao máximo.
5. **Regra nova no AGENT_RULES.md** (seção Frontend): "Layout compacto — preferir
   horizontal, minimizar scroll vertical": agrupar campos relacionados na mesma
   linha via grid responsivo; listas (lojas/slides) em grid de cards lado a lado;
   evitar uma-propriedade-por-linha; o objetivo é caber mais em tela sem rolagem
   vertical desnecessária. Aplica a todo editor/workspace.
6. Validação: eslint 0 erros + vue-tsc limpo.

## Subagente D — crow-nuxt: rota `/{slug}` (sem `/bio`)

Território: projeto `C:\Users\Mike\Documents\Projects\crow-nuxt`.

1. **Mover a página**: criar `app/pages/[slug].vue` com o conteúdo de
   `app/pages/bio/[slug].vue` (incluindo o SSE e o live-reload). Remover
   `app/pages/bio/[slug].vue` e `app/pages/bio/index.vue`. **Manter**
   `app/pages/bio/preview.vue` (o iframe do painel usa `/bio/preview`).
2. A URL pública passa a ser `dominio/{slug}` (ex.: `/perola`). O `/` (index) é
   mais específico e continua vencendo; `/bio/preview` (2 segmentos) não colide.
3. O EventSource continua em `{streamBase}/bio/{slug}/stream` (back, não muda).
4. Validar: `/perola` e `/mostarda` renderizam; `/bio/preview` continua para o
   iframe; reiniciar o dev.

> O link "Ver online"/"Ver bio" do painel passa a montar `bioFrontUrl + '/' + slug`
> (sem `/bio`) — ajuste no subagente B, alinhado com D.

## Regras comuns (todos os subagentes)

Ler AGENT_RULES.md + docs/ENGINEERING_PRINCIPLES.md + este doc. Sem git. Não
aplicar migration/rebuild (devolver ao usuário). Máx 450 linhas/arquivo. Sem
emoji. Lint zero. Atualizar AGENT.md. **Push, não polling** (§6). Tokens do
design system, BEM. Territórios exclusivos — não tocar arquivo de outro agente.

## Integração (agente principal, após os subagentes)

Wiring final, rebuild da api, reiniciar crow-nuxt, e2e (criar sem cliente →
editor → auto-save → switch draft/published → undo → duplicar → ver online em
`/{slug}`), sync dos 3 docs + roadmap.
