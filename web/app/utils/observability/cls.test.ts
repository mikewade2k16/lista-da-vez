import { describe, expect, it } from 'vitest'

import { createClsTracker } from './cls'

describe('createClsTracker', () => {
  it('soma shifts proximos na mesma janela', () => {
    const t = createClsTracker()
    t.add({ value: 0.1, startTime: 100 })
    t.add({ value: 0.05, startTime: 600 })
    expect(t.value()).toBeCloseTo(0.15)
  })

  it('abre nova janela apos gap > 1s e fica com a maior', () => {
    const t = createClsTracker()
    t.add({ value: 0.2, startTime: 100 })
    t.add({ value: 0.05, startTime: 3000 })
    expect(t.value()).toBeCloseTo(0.2)
  })

  it('limita a janela a 5s', () => {
    const t = createClsTracker()
    for (let i = 0; i < 8; i++) t.add({ value: 0.01, startTime: 100 + i * 900 })
    // 100..6400: a janela quebra em 5s, entao a maior soma < 0.08
    expect(t.value()).toBeLessThan(0.08)
    expect(t.value()).toBeGreaterThanOrEqual(0.06)
  })

  it('ignora shifts com input recente', () => {
    const t = createClsTracker()
    t.add({ value: 0.5, startTime: 100, hadRecentInput: true })
    expect(t.value()).toBe(0)
  })
})
