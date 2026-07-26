const shouldEnableNuxtDevtools = process.env.NUXT_DEVTOOLS === 'true'
// PWA estacionado por decisao de produto. So volta a registrar manifest/SW
// quando for explicitamente habilitado no ambiente.
const shouldEnablePwa = process.env.NUXT_PWA_ENABLED === 'true'
const shouldUsePollingWatcher =
  process.env.CHOKIDAR_USEPOLLING === 'true' || process.env.WATCHPACK_POLLING === 'true'
// Menos arquivos vigiados = menos CPU gasta no polling (obrigatorio no bind
// mount Windows->container, onde eventos de arquivo nativos nao chegam, entao o
// watcher precisa varrer periodicamente). node_modules e .git ja sao ignorados
// por padrao pelo Vite; aqui somamos os diretorios gerados/pesados.
const watcherIgnorePatterns = [
  '**/.output/**',
  '**/dist/**',
  '**/.nuxt/**',
  '**/coverage/**',
  '**/*.log',
]
const watcherInterval = Number(process.env.CHOKIDAR_INTERVAL || 350)

export default defineNuxtConfig({
  extends: ['./layers/core', './layers/queue', './layers/tasks', './layers/finance'],
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
    '/comunicados': { ssr: false },
    '/auth/**': { ssr: false },
    '/automation': { ssr: false },
    '/banco': { ssr: false },
    '/bi': { ssr: false },
    '/calendario': { ssr: false },
    '/calendario/**': { ssr: false },
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
    '/inteligencia-clientes': { ssr: false },
    '/inteligencia-clientes/**': { ssr: false },
    '/manage/**': { ssr: false },
    '/meta-ads': { ssr: false },
    '/meus-feedbacks': { ssr: false },
    '/monitoramento': { ssr: false },
    '/multiloja': { ssr: false },
    '/offline': { prerender: true },
    '/omnichannel': { ssr: false },
    '/omnichannel/**': { ssr: false },
    '/operacao/**': { ssr: false },
    '/perfil': { ssr: false },
    '/performance': { ssr: false },
    '/postagens': { ssr: false },
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
  // Vigia apenas os diretorios que o Nuxt realmente precisa (pages/layouts/
  // components/composables/...) em vez de toda a srcDir — corta bastante a qtd de
  // arquivos no polling, que e obrigatorio no bind mount do Docker no Windows.
  experimental: {
    watcher: 'chokidar-granular',
  },
  modules: ['@nuxt/ui', '@nuxt/eslint', '@pinia/nuxt', '@vite-pwa/nuxt'],
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
        // Pre-bundlados p/ o Vite NAO os "descobrir" no meio da navegacao (ao
        // abrir uma tela de graficos: meta-ads, analytics do cardapio). Descoberta
        // tardia de dep dispara re-otimizacao + full reload da pagina — a causa dos
        // travamentos aleatorios de 5-10s no dev. Listar aqui elimina esse reload.
        'apexcharts',
        'vue3-apexcharts',
        'pinia',
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
      // Pre-compile em BACKGROUND, no boot, de todas as paginas + shell — e o
      // que faz a troca de rota ser instantanea em vez de compilar sob demanda
      // a cada clique. ATENCAO aos caminhos: sao relativos a RAIZ DO VITE, que
      // no Nuxt 4 e o srcDir (web/app), NAO a raiz do web/. A versao anterior
      // usava './app/...' e resolvia para web/app/app/... (inexistente) — o
      // warmup falhava silencioso (Pre-transform error no log) e NUNCA aqueceu
      // nada, em Docker nem nativo. Globs sao suportados.
      warmup: {
        clientFiles: [
          './app.vue',
          // App inteiro: paginas, layouts, componentes, stores, composables,
          // domain e utils — transformar 1 modulo NAO transforma os imports
          // dele, entao so listar as paginas deixaria 90% do grafo frio.
          './**/*.vue',
          './**/*.ts',
          // Layers (core/queue/tasks/finance) — mesmos motivos.
          '../layers/*/**/*.vue',
          '../layers/*/**/*.ts',
        ],
      },
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
  pwa: {
    disable: !shouldEnablePwa,
    // DECISAO: registerType 'prompt' (nao 'autoUpdate'). O painel tem operacao
    // ao vivo, chat e formularios longos; a pessoa escolhe quando recarregar.
    registerType: 'prompt',
    manifest: {
      id: '/',
      name: 'Omni',
      short_name: 'Omni',
      description: 'Painel Omni: operacao, tasks, calendario e clientes.',
      lang: 'pt-BR',
      start_url: '/',
      scope: '/',
      display: 'standalone',
      theme_color: '#060a12',
      background_color: '#060a12',
      icons: [
        { src: '/pwa-192x192.png', sizes: '192x192', type: 'image/png' },
        { src: '/pwa-512x512.png', sizes: '512x512', type: 'image/png' },
        {
          src: '/maskable-icon-512x512.png',
          sizes: '512x512',
          type: 'image/png',
          purpose: 'maskable',
        },
      ],
    },
    registerWebManifestInRouteRules: true,
    client: {
      installPrompt: true,
    },
    workbox: {
      // ================= REGRA MULTI-TENANT (NAO REMOVER) =================
      // Precache SOMENTE do app shell e de assets estaticos, iguais para todos
      // os tenants. NUNCA adicionar runtimeCaching para a origem da API nem
      // para /v1/*: o cache do Service Worker nao inclui X-Account-Id na chave
      // e poderia servir dados de uma conta em outra sessao/conta.
      // =====================================================================
      globPatterns: ['**/*.{html,css,ico,png,svg,webp,woff2}'],
      globIgnores: ['perola-bi-teste.html', 'erp-agent.md'],
      navigateFallback: null,
      cleanupOutdatedCaches: true,
      maximumFileSizeToCacheInBytes: 3145728,
      runtimeCaching: [
        {
          urlPattern: ({ sameOrigin, url }) => sameOrigin && url.pathname.startsWith('/_nuxt/'),
          handler: 'CacheFirst',
          options: {
            cacheName: 'omni-build-assets',
            expiration: { maxEntries: 400, maxAgeSeconds: 2592000 },
            cacheableResponse: { statuses: [200] },
          },
        },
        {
          urlPattern: ({ sameOrigin, request }) => sameOrigin && request.mode === 'navigate',
          handler: 'NetworkOnly',
          options: {
            precacheFallback: { fallbackURL: '/offline/index.html' },
          },
        },
      ],
    },
    devOptions: {
      // Opt-in em dev para o SW nao interferir no HMR por padrao.
      enabled: process.env.NUXT_PWA_DEV === 'true',
      type: 'module',
      suppressWarnings: true,
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
    '~/assets/styles/calendar.css',
    '~/assets/styles/presentation.css',
  ],
  app: {
    head: {
      htmlAttrs: {
        lang: 'pt-BR',
      },
      title: 'Omni',
      meta: [
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        { name: 'theme-color', content: '#060a12' },
      ],
      link: [
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/icon?family=Material+Icons+Round',
        },
        ...(shouldEnablePwa ? [{ rel: 'manifest', href: '/manifest.webmanifest' }] : []),
        { rel: 'apple-touch-icon', href: '/apple-touch-icon-180x180.png' },
        { rel: 'icon', href: '/favicon.ico', sizes: 'any' },
      ],
    },
  },
})
