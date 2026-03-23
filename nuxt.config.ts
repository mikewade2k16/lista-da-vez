import { fileURLToPath } from "node:url";

const legacySrcDir = fileURLToPath(new URL("./src", import.meta.url));

export default defineNuxtConfig({
  compatibilityDate: "2026-03-23",
  alias: {
    "@legacy": legacySrcDir
  },
  css: [
    "@legacy/styles/tokens.css",
    "@legacy/styles/base.css",
    "@legacy/styles/layout.css",
    "@legacy/styles/components.css",
    "@legacy/styles/presentation.css"
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
