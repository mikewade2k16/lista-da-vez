import { describe, expect, it } from 'vitest'
import { normalizeProductTags, productTagOptions } from './product-tags'
import {
  emptyProductForm,
  formToPayload,
  productToForm,
} from '~/composables/useCardapioProductForm'
import type { Product } from './types'

describe('tags dos produtos', () => {
  it('remove vazios e duplicados sem separar virgulas dentro de uma tag', () => {
    expect(normalizeProductTags([' Novo ', '', 'novo', 'Picante', 'Combo, especial'])).toEqual([
      'Novo',
      'Picante',
      'Combo, especial',
    ])
  })

  it('deriva as opcoes somente dos produtos fornecidos e remove repeticoes', () => {
    expect(productTagOptions([{ tags: ['Novo', 'Picante'] }, { tags: ['novo'] }, {}])).toEqual([
      { label: 'Novo', value: 'Novo' },
      { label: 'Picante', value: 'Picante' },
    ])
    expect(productTagOptions([])).toEqual([])
  })

  it('hidrata uma copia das tags e envia zero ou varias tags sem perder os demais campos', () => {
    const tags = ['Novo', 'Combo, especial']
    const product = {
      ...emptyProductForm(),
      tags,
      diet: [],
      allergens: [],
      pairing: null,
    } as unknown as Product
    const form = productToForm(product)
    expect(form.tags).toEqual(tags)
    expect(form.tags).not.toBe(tags)
    expect(formToPayload(form).tags).toEqual(tags)
    form.tags = []
    expect(formToPayload(form).tags).toEqual([])
    expect(product.tags).toEqual(tags)
  })
})
