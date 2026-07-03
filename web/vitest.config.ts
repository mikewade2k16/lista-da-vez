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
    // Testes de realtime (useOperationsRealtime/useTasksRealtime) e de stores
    // (cardapio) compartilham o mock global unico de `$fetch` (test/setup.ts) e
    // alternam entre timers reais e fake timers. Sob execucao paralela de
    // arquivos, o agendamento de microtasks/timers do worker fica sujeito a
    // contencao de CPU: o fetch do ticket (timers reais) as vezes nao resolve a
    // tempo do numero fixo de flushes, e `advanceTimersByTimeAsync` de um arquivo
    // interfere no agendamento do outro — deixando esses testes intermitentes.
    // Serializar os arquivos remove a corrida sem enfraquecer nenhum assert; a
    // suite e pequena e o custo de tempo e desprezivel.
    fileParallelism: false,
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
