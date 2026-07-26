import { beforeAll, describe, expect, it, vi } from 'vitest'

interface GuardModule {
  findModulePathGuard: (
    path: string,
  ) => { prefix: string; moduleId?: string; anyModuleIds?: readonly string[] } | undefined
  isModulePathGuardEnabled: (
    guard: { prefix: string; moduleId?: string; anyModuleIds?: readonly string[] },
    enabledModules: ReadonlySet<string>,
  ) => boolean
  pathMatchesModulePrefix: (path: string, prefix: string) => boolean
}

let guards: GuardModule

beforeAll(async () => {
  vi.stubGlobal('defineNuxtRouteMiddleware', (handler: unknown) => handler)
  guards = (await import('./module-enabled.global')) as GuardModule
})

describe('customer intelligence module route guards', () => {
  it('protege a aba de transcricoes pelo modulo queue', () => {
    expect(guards.findModulePathGuard('/transcricoes')).toMatchObject({
      prefix: '/transcricoes',
      moduleId: 'queue',
    })
  })

  it('does not collide with the legacy analytics route', () => {
    expect(guards.pathMatchesModulePrefix('/inteligencia-clientes', '/inteligencia')).toBe(false)
    expect(guards.findModulePathGuard('/inteligencia')).toMatchObject({
      prefix: '/inteligencia',
      moduleId: 'queue',
    })
  })

  it('requires the owner module on specialized routes', () => {
    const segments = guards.findModulePathGuard('/inteligencia-clientes/segmentos')
    const prompts = guards.findModulePathGuard('/inteligencia-clientes/prompts')
    expect(segments).toMatchObject({ moduleId: 'customer_data' })
    expect(prompts).toMatchObject({ moduleId: 'customer_intelligence' })
    expect(guards.isModulePathGuardEnabled(segments!, new Set(['customer_intelligence']))).toBe(
      false,
    )
  })

  it('allows the overview with either independent module', () => {
    const overview = guards.findModulePathGuard('/inteligencia-clientes')
    expect(overview?.anyModuleIds).toEqual(['customer_data', 'customer_intelligence'])
    expect(guards.isModulePathGuardEnabled(overview!, new Set(['customer_data']))).toBe(true)
    expect(guards.isModulePathGuardEnabled(overview!, new Set(['customer_intelligence']))).toBe(
      true,
    )
    expect(guards.isModulePathGuardEnabled(overview!, new Set())).toBe(false)
  })
})
