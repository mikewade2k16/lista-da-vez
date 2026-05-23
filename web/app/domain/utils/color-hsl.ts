import type { ParsedColor } from './color'

function clampPercent(value: number) {
  return Math.max(0, Math.min(100, Math.round(value)))
}

function clampAngle(value: number) {
  if (!Number.isFinite(value)) {
    return 180
  }

  const normalized = value % 360
  return normalized < 0 ? normalized + 360 : normalized
}

function channelsToHex(r: number, g: number, b: number) {
  return `#${[r, g, b]
    .map((channel) => channel.toString(16).padStart(2, '0'))
    .join('')
    .toUpperCase()}`
}

function parseAlphaChannel(token: string | undefined) {
  if (!token?.trim()) {
    return { alpha: 100, hasAlpha: false }
  }

  const trimmed = token.trim()
  const parsed = Number.parseFloat(trimmed.endsWith('%') ? trimmed.slice(0, -1) : trimmed)
  if (Number.isNaN(parsed)) {
    return { alpha: 100, hasAlpha: false }
  }

  if (trimmed.endsWith('%')) {
    return { alpha: clampPercent(parsed), hasAlpha: true }
  }

  return {
    alpha: clampPercent(parsed <= 1 ? parsed * 100 : parsed),
    hasAlpha: true,
  }
}

function splitColorFunctionBody(body: string) {
  if (body.includes('/')) {
    const [rawChannels = '', rawAlpha = ''] = body.split('/').map((part) => part.trim())
    return {
      channelTokens: splitChannelTokens(rawChannels),
      alphaToken: rawAlpha,
    }
  }

  const parts = splitChannelTokens(body)
  return {
    channelTokens: parts.slice(0, 3),
    alphaToken: parts[3],
  }
}

function splitChannelTokens(value: string) {
  return value
    .split(value.includes(',') ? ',' : /\s+/)
    .map((token) => token.trim())
    .filter(Boolean)
}

export function parseHueToken(token: string) {
  const trimmed = token.trim().toLowerCase()
  if (!trimmed) {
    return null
  }

  if (trimmed.endsWith('deg')) {
    return clampAngle(Number.parseFloat(trimmed.slice(0, -3)))
  }

  if (trimmed.endsWith('turn')) {
    return clampAngle(Number.parseFloat(trimmed.slice(0, -4)) * 360)
  }

  if (trimmed.endsWith('rad')) {
    return clampAngle((Number.parseFloat(trimmed.slice(0, -3)) * 180) / Math.PI)
  }

  const parsed = Number.parseFloat(trimmed)
  return Number.isNaN(parsed) ? null : clampAngle(parsed)
}

function parsePercentToken(token: string) {
  const trimmed = token.trim()
  const parsed = trimmed.endsWith('%') ? Number.parseFloat(trimmed.slice(0, -1)) : Number.NaN
  return Number.isNaN(parsed) ? null : Math.max(0, Math.min(100, parsed)) / 100
}

function hslToRgbChannels(hue: number, saturation: number, lightness: number) {
  const chroma = (1 - Math.abs(2 * lightness - 1)) * saturation
  const huePrime = hue / 60
  const secondary = chroma * (1 - Math.abs((huePrime % 2) - 1))
  const match = lightness - chroma / 2
  const sectors = [
    [chroma, secondary, 0],
    [secondary, chroma, 0],
    [0, chroma, secondary],
    [0, secondary, chroma],
    [secondary, 0, chroma],
    [chroma, 0, secondary],
  ] as const
  const [red, green, blue] = sectors[Math.min(5, Math.floor(huePrime))] ?? sectors[0]

  return [
    Math.round((red + match) * 255),
    Math.round((green + match) * 255),
    Math.round((blue + match) * 255),
  ] as const
}

export function parseHslColor(value: string): ParsedColor | null {
  const hslMatch = value.match(/^hsla?\((.*)\)$/i)
  const body = (hslMatch?.[1] ?? '').trim()
  if (!body) {
    return null
  }

  const { channelTokens, alphaToken } = splitColorFunctionBody(body)
  const hue = parseHueToken(channelTokens[0] ?? '')
  const saturation = parsePercentToken(channelTokens[1] ?? '')
  const lightness = parsePercentToken(channelTokens[2] ?? '')
  if (hue === null || saturation === null || lightness === null) {
    return null
  }

  const [r, g, b] = hslToRgbChannels(hue, saturation, lightness)
  const parsedAlpha = parseAlphaChannel(alphaToken)
  return {
    hex: channelsToHex(r, g, b),
    alpha: parsedAlpha.alpha,
    hasAlpha: parsedAlpha.hasAlpha,
  }
}
