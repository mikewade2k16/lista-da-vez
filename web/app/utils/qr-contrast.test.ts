import { describe, expect, it } from 'vitest'
import { checkQrContrast, MIN_QR_CONTRAST } from './qr-contrast'

// Mesmos casos de back/internal/modules/tools/tools_rules_test.go: front e back
// precisam aprovar/recusar exatamente as mesmas cores.
describe('checkQrContrast', () => {
  it('usa o mesmo limite do back', () => {
    expect(MIN_QR_CONTRAST).toBe(4.5)
  })

  it.each([
    ['#000000', '#ffffff', true],
    ['#112233', '#ffffff', true],
    ['#767676', '#ffffff', true],
    ['#777777', '#ffffff', false],
    ['#cccccc', '#ffffff', false],
    ['#336699', '#336699', false],
    ['#ffffff', '#000000', false],
  ])('%s sobre %s => %s', (fill, back, ok) => {
    expect(checkQrContrast(fill, back).ok).toBe(ok)
  })

  it('preto no branco da 21:1', () => {
    expect(checkQrContrast('#000000', '#ffffff').ratio).toBeCloseTo(21, 5)
  })

  it('distingue invertido, baixo contraste e formato invalido', () => {
    expect(checkQrContrast('#ffffff', '#000000')).toMatchObject({ ok: false, reason: 'inverted' })
    expect(checkQrContrast('#cccccc', '#ffffff')).toMatchObject({ ok: false, reason: 'low' })
    expect(checkQrContrast('#000', '#ffffff')).toMatchObject({ ok: false, reason: 'invalid' })
    expect(checkQrContrast('black', '#ffffff')).toMatchObject({ ok: false, reason: 'invalid' })
  })
})
