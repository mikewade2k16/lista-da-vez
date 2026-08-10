import { describe, expect, it } from 'vitest'

import { canEditPlanning, canViewPlanning } from './permissions'

describe('planning permissions', () => {
  it('uses resolved permissions instead of role fallback', () => {
    expect(canViewPlanning('manager', [], true)).toBe(false)
    expect(canEditPlanning('manager', [], true)).toBe(false)
  })

  it('allows edit permission to view and edit planning', () => {
    const permissions = ['workspace.planejamento.edit']
    expect(canViewPlanning('consultant', permissions, true)).toBe(true)
    expect(canEditPlanning('consultant', permissions, true)).toBe(true)
  })

  it('does not inherit access from the Multi-loja workspace', () => {
    const permissions = ['workspace.multiloja.view', 'workspace.multiloja.edit']
    expect(canViewPlanning('owner', permissions, true)).toBe(false)
    expect(canEditPlanning('owner', permissions, true)).toBe(false)
  })
})
