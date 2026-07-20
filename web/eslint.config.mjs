// ESLint flat config — Omni web
// Baseline conservadora: regras Nuxt + plugin Vue + plugin TS,
// com max-lines marcado como WARN (não bloqueia) até a Fase 7 (fatiamento).

import withNuxt from './.nuxt/eslint.config.mjs'
import unusedImports from 'eslint-plugin-unused-imports'

export default withNuxt(
  {
    name: 'omni/ignores',
    ignores: [
      '.nuxt/**',
      '.output/**',
      'dist/**',
      'node_modules/**',
      '**/*.test.ts',
      // Documentos e dados gerados — não fazem parte de qualidade de código
      'app/components/roadmap/database-schema-data.ts',
      'app/components/roadmap/roadmap-data.ts',
      'app/components/roadmap/data/**',
      // Port verbatim do omnichannel (F1): não lintar em massa até a F14 (refactor
      // deliberado). Evita os erros no-useless-escape e os ~460 warnings max-lines
      // herdados do legado bloquearem o commit. Alinhamento de UI = edição pontual.
      // NÃO inclui config/ (house-standard). Ver docs/LEGADO.md e a memória do conflito.
      'app/composables/omnichannel/**',
      'app/components/omnichannel/inbox/**',
      'app/components/omnichannel/OmnichannelInboxModule.vue',
      'app/pages/omnichannel/**',
    ],
  },
  {
    name: 'omni/plugins',
    plugins: {
      'unused-imports': unusedImports,
    },
  },
  {
    name: 'omni/rules-base',
    // regras universais (não dependem de plugin externo)
    rules: {
      // Imports — plugin local declarado em omni/plugins
      'unused-imports/no-unused-imports': 'warn',
      'unused-imports/no-unused-vars': [
        'warn',
        { vars: 'all', varsIgnorePattern: '^_', args: 'after-used', argsIgnorePattern: '^_' },
      ],

      // Bloqueia regressão de arquivos gigantes (alvo da Fase 7)
      // WARN durante a Fase 6 — vira ERROR após a Fase 7.5.
      'max-lines': ['warn', { max: 500, skipBlankLines: true, skipComments: true }],

      // Higiene geral
      'no-console': ['warn', { allow: ['warn', 'error'] }],
      'no-debugger': 'error',
      'prefer-const': 'warn',
      eqeqeq: ['warn', 'smart'],
    },
  },
  {
    name: 'omni/rules-ts',
    files: ['**/*.{ts,vue}'],
    rules: {
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': 'off', // delegado para unused-imports
      '@typescript-eslint/ban-ts-comment': 'warn',
      // Rebaixadas pragmaticamente — viram error após Fase 7
      '@typescript-eslint/no-dynamic-delete': 'warn',
      '@typescript-eslint/unified-signatures': 'warn',
    },
  },
  {
    name: 'omni/rules-vue',
    files: ['**/*.vue'],
    rules: {
      'vue/multi-word-component-names': 'off', // pages/layouts ficam fora
      'vue/no-multiple-template-root': 'off', // Vue 3 permite fragmentos
      'vue/html-self-closing': [
        'warn',
        { html: { void: 'always', normal: 'never', component: 'always' } },
      ],
      'vue/component-name-in-template-casing': ['warn', 'PascalCase'],
      'vue/attributes-order': 'warn',
      'vue/no-unused-vars': 'warn',
      'vue/require-default-prop': 'off',
      // Rebaixada — vira error após Fase 7
      'vue/no-use-v-if-with-v-for': 'warn',
    },
  },
  {
    name: 'omni/rules-baseline-overrides',
    // Bloco final — sobrescreve regras setadas como error pelos defaults Nuxt
    rules: {
      'no-empty': 'warn',
      'no-console': ['warn', { allow: ['warn', 'error'] }],
    },
  },
)
