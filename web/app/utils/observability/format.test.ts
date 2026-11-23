import { describe, expect, it } from 'vitest'

import { errorRateBand, formatMs, formatRate, scoreBand, vitalBand } from './format'

describe('format', () => {
  it('formata ms e taxa', () => {
    expect(formatMs(250)).toBe('250 ms')
    expect(formatMs(1500)).toBe('1,5 s')
    expect(formatMs(undefined)).toBe('-')
    expect(formatRate(0.034)).toBe('3,4%')
  })

  it('classifica faixas', () => {
    expect(scoreBand(90)).toBe('good')
    expect(scoreBand(70)).toBe('warn')
    expect(scoreBand(10)).toBe('bad')
    expect(vitalBand('LCP', 2000)).toBe('good')
    expect(vitalBand('LCP', 3000)).toBe('warn')
    expect(vitalBand('CLS', 0.3)).toBe('bad')
    expect(errorRateBand(0.002)).toBe('good')
  })
})
