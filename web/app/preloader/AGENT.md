# Preloader de boot

Tela exibida enquanto o painel SPA carrega (antes do Vue montar e ate a primeira
pagina resolver). Implementado com o `spaLoadingTemplate` nativo do Nuxt 4,
apontado em `web/nuxt.config.ts` para `preloader/preloader.html`.

## Contrato

- **Arquivo unico da animacao:** `preloader.html`. Trocar a animacao = substituir
  esse arquivo. Nenhuma pagina, layout, store ou plugin referencia o preloader.
- HTML + CSS puro. Sem JS, sem imports, sem tokens do app (o CSS do app ainda nao
  carregou nesse momento). Imagem/SVG deve ir inline (data URI ou `<svg>`).
- Classes e keyframes com prefixo `omni-preloader` (o arquivo e injetado no
  `<body>` do HTML do SPA; nomes genericos colidiriam com o app).
- Raiz em overlay fixo de tela cheia (`position: fixed; inset: 0` e z-index alto):
  o app monta por baixo enquanto o preloader ainda esta visivel.
- Manter `role="status"` e respeitar `prefers-reduced-motion`.

## Ciclo de vida (Nuxt, `experimental.spaLoadingTemplateLocation = 'body'`)

O Nuxt injeta o conteudo em `<div id="__nuxt-loader">` e o remove sozinho no hook
`app:suspense:resolve` (primeira pagina pronta, ja apos os middlewares de auth).
Nao remover manualmente nem duplicar a logica em componente Vue.

Navegacao entre paginas nao usa este preloader: a barra fina de
`layers/core/components/CoreLoadingOverlay.vue` cobre trocas de rota.

Em dev o Nitro recarrega o template ao salvar o arquivo; basta dar F5.
