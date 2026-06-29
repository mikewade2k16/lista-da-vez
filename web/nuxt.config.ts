const shouldEnableNuxtDevtools = process.env.NUXT_DEVTOOLS === 'true'
const shouldUsePollingWatcher =
  process.env.CHOKIDAR_USEPOLLING === 'true' || process.env.WATCHPACK_POLLING === 'true'
const watcherIgnorePatterns = ['**/.output/**', '**/dist/**']
const watcherInterval = Number(process.env.CHOKIDAR_INTERVAL || 350)

export default defineNuxtConfig({
  extends: ['./layers/core', './layers/queue', './layers/tasks'],
  compatibilityDate: '2026-03-23',
  devtools: {
    enabled: shouldEnableNuxtDevtools,
  },
  // O painel inteiro e SPA (client-rendered): cada pagina autenticada e ssr:false.
  // Manter TODA rota nova de painel aqui — uma rota fora da lista roda SSR e o
  // bootstrap de auth no servidor e fragil (ver auth.global.ts: deslogava/estourava
  // no hard reload). Em ordem alfabetica para facilitar conferir o que falta.
  routeRules: {
    '/': { ssr: false },
    '/alertas': { ssr: false },
    '/auth/**': { ssr: false },
    '/automation': { ssr: false },
    '/banco': { ssr: false },
    '/bi': { ssr: false },
    '/campanhas': { ssr: false },
    '/cardapio': { ssr: false },
    '/cardapio/**': { ssr: false },
    '/clientes': { ssr: false },
    '/configuracoes': { ssr: false },
    '/consultor': { ssr: false },
    '/crm': { ssr: false },
    '/dados': { ssr: false },
    '/editor': { ssr: false },
    '/erp': { ssr: false },
    '/feedback': { ssr: false },
    '/finance': { ssr: false },
    '/inteligencia': { ssr: false },
    '/manage/**': { ssr: false },
    '/meta-ads': { ssr: false },
    '/meus-feedbacks': { ssr: false },
    '/monitoramento': { ssr: false },
    '/multiloja': { ssr: false },
    '/omnichannel': { ssr: false },
    '/operacao/**': { ssr: false },
    '/perfil': { ssr: false },
    '/performance': { ssr: false },
    '/ranking': { ssr: false },
    '/relatorios': { ssr: false },
    '/roadmap': { ssr: false },
    '/site/**': { ssr: false },
    '/tasks': { ssr: false },
    '/team/**': { ssr: false },
    '/themes': { ssr: false },
    '/tools/**': { ssr: false },
    '/tracking': { ssr: false },
    '/usuarios': { ssr: false },
  },
  modules: ['@nuxt/ui', '@nuxt/eslint', '@pinia/nuxt'],
  colorMode: {
    preference: 'dark',
    fallback: 'dark',
  },
  eslint: {
    config: {
      stylistic: false, // Prettier cuida de estilo, evita conflito
      standalone: true, // habilita plugins Vue e TypeScript de comunidade
    },
  },
  ui: {
    fonts: false,
    experimental: {
      componentDetection: true,
    },
  },
  icon: {
    provider: 'server',
    fallbackToApi: false,
    collections: ['lucide'],
  },
  vite: {
    optimizeDeps: {
      include: [
        '@tiptap/extension-image',
        '@tiptap/extension-link',
        '@tiptap/extension-drag-handle',
        '@tiptap/extension-emoji',
        '@tiptap/extension-mention',
        '@tiptap/extension-placeholder',
        '@tiptap/extension-task-item',
        '@tiptap/extension-task-list',
        '@tiptap/extension-text-align',
        '@tiptap/extension-underline',
        '@tiptap/starter-kit',
        '@tiptap/suggestion',
        '@tiptap/vue-3',
        'lucide-vue-next',
      ],
    },
    // NAO externalizar @tiptap/y-tiptap: o drag-handle (@tiptap/extension-drag-handle,
    // usado pelo OmniEditor via UEditorDragHandle) o importa ESTATICAMENTE. Marca-lo como
    // `build.rollupOptions.external` deixa um bare specifier "@tiptap/y-tiptap" no bundle do
    // browser -> o navegador nao resolve (sem import map) -> "Failed to resolve module
    // specifier" -> o chunk do editor nao avalia -> o modal de tasks abre EM BRANCO em
    // producao (em dev funciona porque `build.rollupOptions.external` nao se aplica ao dev
    // server do Vite). Deixar o Rollup bundlar normalmente: yjs e todos os peers
    // (y-protocols, prosemirror-*, lib0) ja estao em node_modules e o editor e lazy.
    server: {
      watch: shouldUsePollingWatcher
        ? {
            ignored: watcherIgnorePatterns,
            usePolling: true,
            interval: watcherInterval,
          }
        : {
            ignored: watcherIgnorePatterns,
          },
    },
  },
  runtimeConfig: {
    apiInternalBase:
      process.env.NUXT_API_INTERNAL_BASE ||
      process.env.NUXT_PUBLIC_API_BASE ||
      'http://localhost:8080',
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || 'http://localhost:8080',
      apiWsBase: process.env.NUXT_PUBLIC_API_WS_BASE || '',
      themeStudioEnabled: process.env.NUXT_PUBLIC_ENABLE_THEME_STUDIO === 'true',
      // Base do front bio publico (repo separado); usada no link "Ver bio" do
      // painel Site/Bio. Vazia = link oculto.
      bioFrontUrl: process.env.NUXT_PUBLIC_BIO_FRONT_URL || '',
      // Base do Studio do TAVOLA (repo separado) embutido por iframe na aba Site
      // do editor de cardapio (desenho B4). A origem desta URL e a UNICA aceita
      // nas mensagens postMessage do iframe. Sobrescrevivel por NUXT_PUBLIC_STUDIO_URL.
      studioUrl: process.env.NUXT_PUBLIC_STUDIO_URL || 'http://localhost:3000',
    },
  },
  css: [
    '~/assets/styles/omni-design-system.css',
    '~/assets/styles/tokens.css',
    '~/assets/styles/base.css',
    '~/assets/styles/layout.css',
    '~/assets/styles/components.css',
    '~/assets/styles/tasks-modal.css',
    '~/assets/styles/presentation.css',
  ],
  app: {
    head: {
      htmlAttrs: {
        lang: 'pt-BR',
      },
      title: 'Omni',
      meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }],
      link: [
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/icon?family=Material+Icons+Round',
        },
      ],
    },
  },
})
