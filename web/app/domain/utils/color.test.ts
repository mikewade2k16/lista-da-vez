import { describe, expect, it } from 'vitest'

import {
  buildCssColor,
  buildGradientValue,
  channelsToHex,
  clampAngle,
  clampPercent,
  hexToRgbChannels,
  normalizeHex,
  parseAlphaChannel,
  parseGradient,
  parseHexColor,
  parseRgbChannel,
  splitByTopLevelComma,
} from './color'

describe('clampPercent', () => {
  it('clamps and rounds into [0, 100]', () => {
    expect(clampPercent(-5)).toBe(0)
    expect(clampPercent(150)).toBe(100)
    expect(clampPercent(49.6)).toBe(50)
  })
})

describe('clampAngle', () => {
  it('normalizes into [0, 360) and defaults NaN to 180', () => {
    expect(clampAngle(Number.NaN)).toBe(180)
    expect(clampAngle(-90)).toBe(270)
    expect(clampAngle(450)).toBe(90)
    expect(clampAngle(360)).toBe(0)
  })
})

describe('normalizeHex', () => {
  it('expands, uppercases and drops the alpha byte', () => {
    expect(normalizeHex('#abc')).toBe('#AABBCC')
    expect(normalizeHex('aabbccdd')).toBe('#AABBCC')
  })

  it('returns null for invalid lengths', () => {
    expect(normalizeHex('#ab')).toBeNull()
    expect(normalizeHex('zz')).toBeNull()
  })
})

describe('parseHexColor', () => {
  it('parses opaque and alpha hex colors', () => {
    expect(parseHexColor('#336699')).toEqual({ hex: '#336699', alpha: 100, hasAlpha: false })
    expect(parseHexColor('#33669980')).toEqual({ hex: '#336699', alpha: 50, hasAlpha: true })
  })

  it('returns null for a non-hex value', () => {
    expect(parseHexColor('xyz')).toBeNull()
  })
})

describe('parseRgbChannel', () => {
  it('parses percentages, clamps and rejects garbage', () => {
    expect(parseRgbChannel('50%')).toBe(127)
    expect(parseRgbChannel('300')).toBe(255)
    expect(parseRgbChannel('-5')).toBe(0)
    expect(parseRgbChannel('abc')).toBeNull()
    expect(parseRgbChannel('')).toBeNull()
  })
})

describe('parseAlphaChannel', () => {
  it('handles missing, fractional, percentage and integer alpha', () => {
    expect(parseAlphaChannel(undefined)).toEqual({ alpha: 100, hasAlpha: false })
    expect(parseAlphaChannel('0.5')).toEqual({ alpha: 50, hasAlpha: true })
    expect(parseAlphaChannel('50%')).toEqual({ alpha: 50, hasAlpha: true })
    expect(parseAlphaChannel('75')).toEqual({ alpha: 75, hasAlpha: true })
  })
})

describe('channel <-> hex conversion', () => {
  it('round-trips between channels and hex', () => {
    expect(channelsToHex(255, 0, 128)).toBe('#FF0080')
    expect(hexToRgbChannels('#FF0080')).toEqual([255, 0, 128])
    expect(hexToRgbChannels('nope')).toBeNull()
  })
})

describe('buildCssColor', () => {
  it('returns hex when alpha is disabled and rgba when enabled', () => {
    expect(buildCssColor('#336699', false, 50)).toBe('#336699')
    expect(buildCssColor('#336699', true, 50)).toBe('rgba(51, 102, 153, 0.5)')
  })

  it('falls back to black for an invalid hex', () => {
    expect(buildCssColor('invalid', true, 50)).toBe('#000000')
  })
})

describe('buildGradientValue', () => {
  const stop = (hex: string) => ({ hex, alphaEnabled: false, alpha: 100 })

  it('emits linear/radial/conic gradients with normalized angle', () => {
    expect(
      buildGradientValue({
        type: 'linear',
        angle: 450,
        start: stop('#FFFFFF'),
        end: stop('#000000'),
      }),
    ).toBe('linear-gradient(90deg, #FFFFFF, #000000)')
    expect(
      buildGradientValue({
        type: 'radial',
        angle: 90,
        start: stop('#FFFFFF'),
        end: stop('#000000'),
      }),
    ).toBe('radial-gradient(circle, #FFFFFF, #000000)')
    expect(
      buildGradientValue({ type: 'conic', angle: 0, start: stop('#FFFFFF'), end: stop('#000000') }),
    ).toBe('conic-gradient(from 0deg, #FFFFFF, #000000)')
  })
})

describe('parseGradient', () => {
  it('parses angle and color stops', () => {
    const linear = parseGradient('linear-gradient(90deg, #fff, #000)')
    expect(linear?.type).toBe('linear')
    expect(linear?.angle).toBe(90)
    expect(linear?.start.hex).toBe('#FFFFFF')
    expect(linear?.end.hex).toBe('#000000')
  })

  it('defaults the angle to 180 when omitted', () => {
    expect(parseGradient('linear-gradient(#fff, #000)')?.angle).toBe(180)
  })

  it('returns null for a plain color', () => {
    expect(parseGradient('#fff')).toBeNull()
  })
})

describe('splitByTopLevelComma', () => {
  it('does not split on commas nested inside parentheses', () => {
    expect(splitByTopLevelComma('rgba(0,0,0,0.5), #fff')).toHaveLength(2)
  })
})

describe('gradient round-trip', () => {
  it('preserves type, angle and color stops through build then parse', () => {
    const value = buildGradientValue({
      type: 'linear',
      angle: 90,
      start: { hex: '#FFFFFF', alphaEnabled: false, alpha: 100 },
      end: { hex: '#000000', alphaEnabled: false, alpha: 100 },
    })
    const parsed = parseGradient(value)
    expect(parsed?.type).toBe('linear')
    expect(parsed?.angle).toBe(90)
    expect(parsed?.start.hex).toBe('#FFFFFF')
    expect(parsed?.end.hex).toBe('#000000')
  })
})
