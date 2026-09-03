import { describe, expect, it } from 'vitest'

import { isAssistantRoute } from './assistant-routes'

describe('isAssistantRoute', () => {
  it.each(['/calendario', '/calendario/configuracao', '/meta-ads', '/meta-ads/campanhas'])(
    'keeps the assistant on %s',
    (path) => expect(isAssistantRoute(path)).toBe(true),
  )

  it.each(['/tasks', '/operacao', '/relatorios', '/perfil', ''])(
    'hides the assistant on %s',
    (path) => expect(isAssistantRoute(path)).toBe(false),
  )
})
