import { fileURLToPath } from "node:url";

const coreDir = fileURLToPath(new URL("../core", import.meta.url));
const workspaceRoutes = [
  "/operacao",
  "/consultor",
  "/ranking",
  "/dados",
  "/inteligencia",
  "/relatorios",
  "/campanhas",
  "/multiloja",
  "/configuracoes"
];

export default defineNuxtConfig({
  compatibilityDate: "2026-03-23",
  modules: ["@pinia/nuxt"],
  alias: {
    "@core": coreDir
  },
  css: [
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
  },
  nitro: {
    prerender: {
      routes: workspaceRoutes
    }
  }
});
