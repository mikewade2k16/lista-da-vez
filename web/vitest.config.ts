import { defineConfig } from 'vitest/config'

// Configuracao minima do Vitest para o monorepo web. Inclui apenas testes puros (utilitarios e
// modulos sem dependencia do Nuxt runtime). Para testar composables completos (useTaskRelations,
// useTaskPresence) sera preciso adicionar `@nuxt/test-utils` + `happy-dom` numa proxima rodada.
export default defineConfig({
  test: {
    environment: 'node',
    include: [
      'layers/**/*.test.ts',
      'layers/**/__tests__/**/*.ts'
    ],
    exclude: [
      'node_modules/**',
      '.nuxt/**',
      '.output/**'
    ]
  }
})
