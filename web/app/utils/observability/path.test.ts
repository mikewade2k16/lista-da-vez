import { describe, expect, it } from 'vitest'

import { isCollectorPath, isPublicApiPath, normalizeApiPath } from './path'

describe('normalizeApiPath', () => {
  it('remove query, hash e origem', () => {
    expect(normalizeApiPath('https://api.x.com/v1/tasks?x=1#h')).toBe('/v1/tasks')
    expect(normalizeApiPath('/v1/tasks?q=ana')).toBe('/v1/tasks')
  })

  it('troca ids numericos, uuid e hex longo por :id', () => {
    expect(normalizeApiPath('/v1/tasks/123/items/987')).toBe('/v1/tasks/:id/items/:id')
    expect(normalizeApiPath('/v1/x/3f2504e0-4f89-11d3-9a0c-0305e82c3301/y')).toBe('/v1/x/:id/y')
    expect(normalizeApiPath('/v1/x/507f1f77bcf86cd799439011abcd')).toBe('/v1/x/:id')
    expect(normalizeApiPath('/v1/v2/clientes')).toBe('/v1/v2/clientes')
  })

  it('garante barra inicial', () => {
    expect(normalizeApiPath('v1/me')).toBe('/v1/me')
  })

  it('detecta publico e coletor', () => {
    expect(isPublicApiPath('/v1/public/menu?x=1')).toBe(true)
    expect(isPublicApiPath('/v1/menu')).toBe(false)
    expect(isCollectorPath('/v1/obs/collect')).toBe(true)
  })
})
