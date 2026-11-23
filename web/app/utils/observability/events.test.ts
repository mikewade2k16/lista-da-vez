import { describe, expect, it } from 'vitest'

import { clampSampleRate, createEventQueue, fitBatchToLimit, shouldRecordApiCall } from './events'

describe('shouldRecordApiCall', () => {
  it('sempre grava falhas e chamadas lentas', () => {
    expect(shouldRecordApiCall(50, true, () => 0.99)).toBe(true)
    expect(shouldRecordApiCall(1500, false, () => 0.99)).toBe(true)
  })

  it('amostra 10% das rapidas', () => {
    expect(shouldRecordApiCall(50, false, () => 0.05)).toBe(true)
    expect(shouldRecordApiCall(50, false, () => 0.5)).toBe(false)
  })
})

describe('createEventQueue', () => {
  it('drena em lotes e descarta o mais antigo quando cheia', () => {
    const q = createEventQueue(3)
    for (let i = 0; i < 5; i++) q.push({ kind: 'vital', name: `e${i}` })
    expect(q.size()).toBe(3)
    expect(q.drain(2).map((e) => e.name)).toEqual(['e2', 'e3'])
    expect(q.size()).toBe(1)
  })
})

describe('clampSampleRate', () => {
  it('limita entre 0 e 1 e usa 1 para invalido', () => {
    expect(clampSampleRate(5)).toBe(1)
    expect(clampSampleRate(-1)).toBe(0)
    expect(clampSampleRate('abc')).toBe(1)
    expect(clampSampleRate('0.25')).toBe(0.25)
  })
})

describe('fitBatchToLimit', () => {
  it('reduz o lote ate caber no limite', () => {
    const events = Array.from({ length: 10 }, (_, i) => ({
      kind: 'js_error' as const,
      name: `e${i}`,
      detail: { stack: 'x'.repeat(1000) },
    }))
    const out = fitBatchToLimit({ pageViewId: 'a', page: '/x', events }, 3500)
    expect(out.events.length).toBeLessThan(10)
    expect(JSON.stringify(out).length).toBeLessThanOrEqual(3500)
  })
})
