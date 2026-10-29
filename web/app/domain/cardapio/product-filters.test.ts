import { describe, expect, it } from 'vitest'
import { productDateRange, productMatchesFilters } from './product-filters'

const now = new Date(2026, 9, 29, 15)
const product = {
  name: 'Açaí',
  categoryId: 'c1',
  isAvailable: true,
  imageUrl: '',
  createdAt: new Date(2026, 9, 29, 10).toISOString(),
}
describe('filtros de cadastro e foto dos produtos', () => {
  it.each([
    ['today', 29],
    ['yesterday', 28],
    ['this_week', 26],
    ['last_7_days', 23],
    ['this_month', 1],
  ])('%s usa dias locais e limites exclusivos no fim', (createdPeriod, day) => {
    const range = productDateRange({ createdPeriod }, now)
    expect(range.from?.getDate()).toBe(day)
    expect(range.from?.getHours()).toBe(0)
    expect(range.to?.getDate()).toBe(createdPeriod === 'yesterday' ? 29 : 30)
  })
  it('combina data, foto, busca, categoria e disponibilidade sem incluir antigos', () => {
    const filters = {
      createdPeriod: 'today',
      image: 'without',
      search: 'acai',
      categoryId: 'c1',
      availability: true,
    }
    const range = productDateRange(filters, now)
    expect(productMatchesFilters(product, filters, range)).toBe(true)
    expect(productMatchesFilters({ ...product, imageUrl: ' /foto.jpg ' }, filters, range)).toBe(
      false,
    )
    expect(
      productMatchesFilters(
        { ...product, createdAt: new Date(2026, 9, 28, 23, 59).toISOString() },
        filters,
        range,
      ),
    ).toBe(false)
    expect(productMatchesFilters({ ...product, isAvailable: false }, filters, range)).toBe(false)
  })
  it('inclui todo o ultimo dia do intervalo e permite apenas uma das datas', () => {
    const range = productDateRange(
      { createdPeriod: 'custom', createdFrom: '2026-10-28', createdTo: '2026-10-29' },
      now,
    )
    expect(
      productMatchesFilters(
        { ...product, createdAt: new Date(2026, 9, 29, 23, 59).toISOString() },
        {},
        range,
      ),
    ).toBe(true)
    expect(
      productMatchesFilters(
        { ...product, createdAt: new Date(2026, 9, 30).toISOString() },
        {},
        range,
      ),
    ).toBe(false)
    expect(productDateRange({ createdPeriod: 'custom', createdTo: '2026-10-29' }).from).toBeNull()
  })
  it('sem data nao exclui produtos antigos e reconhece foto vazia', () => {
    expect(productMatchesFilters({ ...product, createdAt: undefined }, {})).toBe(true)
    expect(
      productMatchesFilters({ ...product, createdAt: undefined }, { createdPeriod: 'today' }),
    ).toBe(false)
    expect(productMatchesFilters({ ...product, imageUrl: '  ' }, { image: 'without' })).toBe(true)
    expect(productMatchesFilters({ ...product, imageUrl: '/foto.jpg' }, { image: 'with' })).toBe(
      true,
    )
  })
  it('semana comeca segunda mesmo no domingo e atravessa meses', () => {
    expect(
      productDateRange({ createdPeriod: 'this_week' }, new Date(2026, 10, 1)).from?.getDate(),
    ).toBe(26)
    expect(
      productDateRange({ createdPeriod: 'last_7_days' }, new Date(2026, 10, 1)).from?.getMonth(),
    ).toBe(9)
  })
})
