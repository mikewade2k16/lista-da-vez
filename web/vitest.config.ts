import { fileURLToPath } from 'node:url'

import { defineConfig } from 'vitest/config'

// Configuracao minima do Vitest para o monorepo web. Inclui apenas testes puros (utilitarios e
// modulos sem dependencia do Nuxt runtime). Para testar composables completos (useTaskRelations,
// useTaskPresence) sera preciso adicionar `@nuxt/test-utils` + `happy-dom` numa proxima rodada.
export default defineConfig({
  resolve: {
    alias: {
      '~': fileURLToPath(new URL('./app', import.meta.url)),
      '@': fileURLToPath(new URL('./app', import.meta.url)),
    },
  },
  define: {
    'import.meta.client': 'true',
    'import.meta.server': 'false',
  },
  test: {
    environment: 'node',
    setupFiles: ['./test/setup.ts'],
    include: [
      'app/**/*.test.ts',
      'app/**/__tests__/**/*.ts',
      'layers/**/*.test.ts',
      'layers/**/__tests__/**/*.ts',
    ],
    exclude: ['node_modules/**', '.nuxt/**', '.output/**'],
  },
})
