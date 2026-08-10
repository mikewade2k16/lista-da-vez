import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

describe('customer intelligence SPA route contract', () => {
  it('keeps overview and nested routes client-rendered', () => {
    // Keep nuxt.config outside Vite's static import graph. Nuxt rejects direct
    // config imports while warming the dev server, even when they come from a test.
    const configPath = resolve(process.cwd(), 'nuxt.config.ts')
    const source = readFileSync(configPath, 'utf8')

    expect(source).toContain("'/inteligencia-clientes': { ssr: false }")
    expect(source).toContain("'/inteligencia-clientes/**': { ssr: false }")
  })
})
