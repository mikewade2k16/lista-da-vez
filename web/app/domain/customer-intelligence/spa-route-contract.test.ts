import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('customer intelligence SPA route contract', () => {
  it('keeps overview and nested routes client-rendered', () => {
    const configPath = fileURLToPath(new URL('../../../nuxt.config.ts', import.meta.url))
    const source = readFileSync(configPath, 'utf8')

    expect(source).toContain("'/inteligencia-clientes': { ssr: false }")
    expect(source).toContain("'/inteligencia-clientes/**': { ssr: false }")
  })
})
