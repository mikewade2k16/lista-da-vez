const shouldEnableNuxtDevtools = process.env.NUXT_DEVTOOLS === "true";
const shouldUsePollingWatcher =
  process.env.CHOKIDAR_USEPOLLING === "true" ||
  process.env.WATCHPACK_POLLING === "true";
const watcherIgnorePatterns = ["**/.output/**", "**/dist/**"];
const watcherInterval = Number(process.env.CHOKIDAR_INTERVAL || 350);

export default defineNuxtConfig({
  extends: ["./layers/core", "./layers/queue", "./layers/tasks"],
  compatibilityDate: "2026-03-23",
  devtools: {
    enabled: shouldEnableNuxtDevtools
  },
  routeRules: {
    "/": { ssr: false },
    "/campanhas": { ssr: false },
    "/configuracoes": { ssr: false },
    "/consultor": { ssr: false },
    "/dados": { ssr: false },
    "/editor": { ssr: false },
    "/feedback": { ssr: false },
    "/finance": { ssr: false },
    "/inteligencia": { ssr: false },
    "/manage/**": { ssr: false },
    "/meus-feedbacks": { ssr: false },
    "/monitoramento": { ssr: false },
    "/multiloja": { ssr: false },
    "/omnichannel": { ssr: false },
    "/operacao/**": { ssr: false },
    "/perfil": { ssr: false },
    "/ranking": { ssr: false },
    "/relatorios": { ssr: false },
    "/site/**": { ssr: false },
    "/tasks": { ssr: false },
    "/team/**": { ssr: false },
    "/themes": { ssr: false },
    "/tools/**": { ssr: false },
    "/tracking": { ssr: false },
    "/usuarios": { ssr: false }
  },
  modules: ["@nuxt/ui", "@pinia/nuxt"],
  ui: {
    fonts: false,
    experimental: {
      componentDetection: true
    }
  },
  icon: {
    provider: "server",
    fallbackToApi: false,
    collections: ["lucide"]
  },
  vite: {
    optimizeDeps: {
      include: [
        "@tiptap/extension-image",
        "@tiptap/extension-link",
        "@tiptap/extension-drag-handle",
        "@tiptap/extension-emoji",
        "@tiptap/extension-mention",
        "@tiptap/extension-placeholder",
        "@tiptap/extension-text-align",
        "@tiptap/extension-underline",
        "@tiptap/starter-kit",
        "@tiptap/suggestion",
        "@tiptap/vue-3",
        "lucide-vue-next"
      ]
    },
    server: {
      watch: shouldUsePollingWatcher
        ? {
            ignored: watcherIgnorePatterns,
            usePolling: true,
            interval: watcherInterval
          }
        : {
            ignored: watcherIgnorePatterns
          }
    }
  },
  runtimeConfig: {
    apiInternalBase:
      process.env.NUXT_API_INTERNAL_BASE ||
      process.env.NUXT_PUBLIC_API_BASE ||
      "http://localhost:8080",
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || "http://localhost:8080",
      apiWsBase: process.env.NUXT_PUBLIC_API_WS_BASE || ""
    }
  },
  css: [
    "~/assets/styles/omni-design-system.css",
    "~/assets/styles/tokens.css",
    "~/assets/styles/base.css",
    "~/assets/styles/layout.css",
    "~/assets/styles/components.css",
    "~/assets/styles/presentation.css"
  ],
  app: {
    head: {
      htmlAttrs: {
        lang: "pt-BR"
      },
      title: "Fila de Atendimento MVP",
      meta: [
        { name: "viewport", content: "width=device-width, initial-scale=1" }
      ],
      link: [
        {
          rel: "stylesheet",
          href: "https://fonts.googleapis.com/icon?family=Material+Icons+Round"
        }
      ]
    }
  }
});
