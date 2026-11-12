import { describe, expect, it } from 'vitest'

import { CARDAPIO_SECTIONS, resolveCardapioSection } from './editor-sections'

const all = CARDAPIO_SECTIONS.map((section) => section.id)

describe('resolveCardapioSection', () => {
  it('abre Produtos quando a URL nao informa a aba', () => {
    expect(resolveCardapioSection(undefined, all)).toBe('produtos')
    expect(resolveCardapioSection('inexistente', all)).toBe('produtos')
  })

  it('respeita a aba pedida quando liberada', () => {
    expect(resolveCardapioSection('dados', all)).toBe('dados')
  })

  it('cai na primeira aba liberada quando Produtos nao esta disponivel', () => {
    expect(resolveCardapioSection(undefined, ['dados', 'pedidos'])).toBe('dados')
  })
})
