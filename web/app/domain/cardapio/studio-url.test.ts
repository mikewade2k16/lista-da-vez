import { describe, expect, it } from 'vitest'
import { resolveCardapioStudioUrl } from './studio-url'

describe('dominio do Studio por estabelecimento', () => {
  it('abre Bari no Bari e Mostarda no Mostarda, ignorando a URL fixa antiga', () => {
    const fixed = 'https://mostarda.crowvisuals.com.br'
    expect(resolveCardapioStudioUrl('bari.crowvisuals.com.br', fixed, false)).toBe(
      'https://bari.crowvisuals.com.br',
    )
    expect(resolveCardapioStudioUrl('mostarda.crowvisuals.com.br', fixed, false)).toBe(fixed)
  })

  it('mantem servidor local apenas em desenvolvimento', () => {
    expect(
      resolveCardapioStudioUrl('bari.crowvisuals.com.br', 'http://localhost:3000/', true),
    ).toBe('http://localhost:3000')
  })

  it.each([
    '',
    'example.com/other',
    'user@example.com',
    'https://example.com',
    'example.com#x',
    'bad host',
    'example.com?x=1',
  ])('bloqueia dominio ausente ou invalido: %s', (domain) => {
    expect(resolveCardapioStudioUrl(domain, 'https://mostarda.crowvisuals.com.br', false)).toBe('')
  })
})
