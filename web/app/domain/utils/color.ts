import { parseHslColor, parseHueToken } from './color-hsl'

export type GradientType = 'linear' | 'radial' | 'conic'

export interface ParsedColor {
  hex: string
  alpha: number
  hasAlpha: boolean
}

export interface ParsedGradient {
  type: GradientType
  angle: number
  start: ParsedColor
  end: ParsedColor
}

export interface GradientColorStop {
  hex: string
  alphaEnabled: boolean
  alpha: number
}

export interface GradientValueOptions {
  type: GradientType
  angle: number
  start: GradientColorStop
  end: GradientColorStop
}

export type CssVariableResolver = (name: string) => string | null | undefined

export function clampPercent(value: number) {
  return Math.max(0, Math.min(100, Math.round(value)))
}

export function clampAngle(value: number) {
  if (!Number.isFinite(value)) {
    return 180
  }

  const normalized = value % 360
  return normalized < 0 ? normalized + 360 : normalized
}

export function normalizeHex(hex: string) {
  const cleaned = hex
    .trim()
    .replace('#', '')
    .replace(/[^0-9a-fA-F]/g, '')
  if (![3, 4, 6, 8].includes(cleaned.length)) {
    return null
  }

  let expanded = cleaned
  if (cleaned.length === 3 || cleaned.length === 4) {
    expanded = cleaned
      .split('')
      .map((char) => `${char}${char}`)
      .join('')
  }

  const noAlpha = expanded.slice(0, 6)
  return `#${noAlpha.toUpperCase()}`
}

export function parseHexColor(value: string): ParsedColor | null {
  const cleaned = value
    .trim()
    .replace('#', '')
    .replace(/[^0-9a-fA-F]/g, '')
  if (![3, 4, 6, 8].includes(cleaned.length)) {
    return null
  }

  let expanded = cleaned
  if (cleaned.length === 3 || cleaned.length === 4) {
    expanded = cleaned
      .split('')
      .map((char) => `${char}${char}`)
      .join('')
  }

  const rgbPart = expanded.slice(0, 6)
  const alphaPart = expanded.length === 8 ? expanded.slice(6, 8) : null
  const alpha = alphaPart ? clampPercent((Number.parseInt(alphaPart, 16) / 255) * 100) : 100

  return {
    hex: `#${rgbPart.toUpperCase()}`,
    alpha,
    hasAlpha: Boolean(alphaPart),
  }
}

export function parseRgbChannel(token: string) {
  const trimmed = token.trim()
  if (!trimmed) {
    return null
  }

  if (trimmed.endsWith('%')) {
    const parsedPercent = Number.parseFloat(trimmed.slice(0, -1))
    if (Number.isNaN(parsedPercent)) {
      return null
    }

    return Math.round(Math.max(0, Math.min(100, parsedPercent)) * 2.55)
  }

  const parsed = Number.parseFloat(trimmed)
  if (Number.isNaN(parsed)) {
    return null
  }

  return Math.round(Math.max(0, Math.min(255, parsed)))
}

export function parseAlphaChannel(token: string | undefined) {
  if (!token) {
    return { alpha: 100, hasAlpha: false }
  }

  const trimmed = token.trim()
  if (!trimmed) {
    return { alpha: 100, hasAlpha: false }
  }

  if (trimmed.endsWith('%')) {
    const parsedPercent = Number.parseFloat(trimmed.slice(0, -1))
    if (Number.isNaN(parsedPercent)) {
      return { alpha: 100, hasAlpha: false }
    }

    return {
      alpha: clampPercent(parsedPercent),
      hasAlpha: true,
    }
  }

  const parsed = Number.parseFloat(trimmed)
  if (Number.isNaN(parsed)) {
    return { alpha: 100, hasAlpha: false }
  }

  if (parsed <= 1) {
    return {
      alpha: clampPercent(parsed * 100),
      hasAlpha: true,
    }
  }

  return {
    alpha: clampPercent(parsed),
    hasAlpha: true,
  }
}

export function channelsToHex(r: number, g: number, b: number) {
  return `#${[r, g, b]
    .map((channel) => channel.toString(16).padStart(2, '0'))
    .join('')
    .toUpperCase()}`
}

export function hexToRgbChannels(hex: string) {
  const normalized = normalizeHex(hex)
  if (!normalized) {
    return null
  }

  const parsed = normalized.replace('#', '')
  const r = Number.parseInt(parsed.slice(0, 2), 16)
  const g = Number.parseInt(parsed.slice(2, 4), 16)
  const b = Number.parseInt(parsed.slice(4, 6), 16)

  return [r, g, b] as const
}

export function resolveRgbVarColor(
  value: string,
  resolveCssVariable?: CssVariableResolver,
): ParsedColor | null {
  const varRgbMatch = value.trim().match(/^rgb\(\s*var\((--[^)]+)\)\s*(?:\/\s*([^)]+))?\)$/i)
  if (!varRgbMatch || !resolveCssVariable) {
    return null
  }

  const varName = (varRgbMatch[1] ?? '').trim()
  if (!varName) {
    return null
  }

  const resolvedVarValue = (resolveCssVariable(varName) ?? '').trim()
  if (!resolvedVarValue) {
    return null
  }

  const channelTokens = resolvedVarValue
    .split(/\s+/)
    .map((token) => token.trim())
    .filter(Boolean)
  if (channelTokens.length < 3) {
    return null
  }

  const r = parseRgbChannel(channelTokens[0] ?? '')
  const g = parseRgbChannel(channelTokens[1] ?? '')
  const b = parseRgbChannel(channelTokens[2] ?? '')
  if (r === null || g === null || b === null) {
    return null
  }

  const alphaToken = (varRgbMatch[2] ?? '').trim() || undefined
  const parsedAlpha = parseAlphaChannel(alphaToken)
  return {
    hex: channelsToHex(r, g, b),
    alpha: parsedAlpha.alpha,
    hasAlpha: parsedAlpha.hasAlpha,
  }
}

interface ColorFunctionParts {
  channelTokens: string[]
  alphaToken?: string
}

function splitColorFunctionBody(body: string): ColorFunctionParts {
  if (body.includes('/')) {
    const [rawChannels = '', rawAlpha = ''] = body.split('/').map((part) => part.trim())
    const channelTokens = rawChannels.includes(',')
      ? rawChannels
          .split(',')
          .map((token) => token.trim())
          .filter(Boolean)
      : rawChannels
          .split(/\s+/)
          .map((token) => token.trim())
          .filter(Boolean)

    return {
      channelTokens,
      alphaToken: rawAlpha,
    }
  }

  if (body.includes(',')) {
    const parts = body
      .split(',')
      .map((token) => token.trim())
      .filter(Boolean)

    return {
      channelTokens: parts.slice(0, 3),
      alphaToken: parts[3],
    }
  }

  const parts = body
    .split(/\s+/)
    .map((token) => token.trim())
    .filter(Boolean)

  return {
    channelTokens: parts.slice(0, 3),
    alphaToken: parts[3],
  }
}

function parseRgbColor(value: string): ParsedColor | null {
  const rgbMatch = value.match(/^rgba?\((.*)\)$/i)
  if (!rgbMatch) {
    return null
  }

  const body = (rgbMatch[1] ?? '').trim()
  if (!body) {
    return null
  }

  const { channelTokens, alphaToken } = splitColorFunctionBody(body)
  if (channelTokens.length < 3) {
    return null
  }

  const r = parseRgbChannel(channelTokens[0] ?? '')
  const g = parseRgbChannel(channelTokens[1] ?? '')
  const b = parseRgbChannel(channelTokens[2] ?? '')
  if (r === null || g === null || b === null) {
    return null
  }

  const parsedAlpha = parseAlphaChannel(alphaToken)

  return {
    hex: channelsToHex(r, g, b),
    alpha: parsedAlpha.alpha,
    hasAlpha: parsedAlpha.hasAlpha,
  }
}

export function parseCssColor(
  value: string,
  resolveCssVariable?: CssVariableResolver,
): ParsedColor | null {
  const normalizedValue = value.trim()
  if (!normalizedValue) {
    return null
  }

  if (normalizedValue.includes('var(')) {
    return resolveRgbVarColor(normalizedValue, resolveCssVariable)
  }

  return (
    parseHexColor(normalizedValue) ??
    parseRgbColor(normalizedValue) ??
    parseHslColor(normalizedValue)
  )
}

export function splitByTopLevelComma(value: string) {
  const result: string[] = []
  let current = ''
  let depth = 0

  for (const char of value) {
    if (char === '(') {
      depth += 1
      current += char
      continue
    }

    if (char === ')') {
      depth = Math.max(0, depth - 1)
      current += char
      continue
    }

    if (char === ',' && depth === 0) {
      if (current.trim()) {
        result.push(current.trim())
      }

      current = ''
      continue
    }

    current += char
  }

  if (current.trim()) {
    result.push(current.trim())
  }

  return result
}

export function extractGradientColorToken(token: string) {
  const trimmed = token.trim()
  if (!trimmed) {
    return ''
  }

  if (trimmed.includes(')')) {
    const closeIndex = trimmed.lastIndexOf(')')
    return trimmed.slice(0, closeIndex + 1).trim()
  }

  return trimmed.split(/\s+/)[0] ?? ''
}

export function parseAngleToken(value: string) {
  const token = value.trim().toLowerCase()
  if (!token) {
    return 180
  }

  const directionMap: Record<string, number> = {
    'to top': 0,
    'to right': 90,
    'to bottom': 180,
    'to left': 270,
    'to top right': 45,
    'to right top': 45,
    'to bottom right': 135,
    'to right bottom': 135,
    'to bottom left': 225,
    'to left bottom': 225,
    'to top left': 315,
    'to left top': 315,
  }

  if (directionMap[token] !== undefined) {
    return directionMap[token]
  }

  return parseHueToken(token) ?? 180
}

export function parseGradient(
  value: string,
  resolveCssVariable?: CssVariableResolver,
): ParsedGradient | null {
  const gradientMatch = value.trim().match(/^(linear|radial|conic)-gradient\((.*)\)$/i)
  if (!gradientMatch) {
    return null
  }

  const type = (gradientMatch[1] || 'linear').toLowerCase() as GradientType
  const args = splitByTopLevelComma(gradientMatch[2] || '')
  if (args.length < 2) {
    return null
  }

  let angle = 180
  let colorArgs = [...args]
  const firstArg = (args[0] || '').trim()

  if (type === 'linear' || type === 'conic') {
    const hasAngle = /(deg|turn|rad)\b/i.test(firstArg) || firstArg.startsWith('to ')
    if (hasAngle) {
      angle = parseAngleToken(firstArg)
      colorArgs = args.slice(1)
    }
  } else if (type === 'radial') {
    const firstColor = parseCssColor(extractGradientColorToken(firstArg), resolveCssVariable)
    if (!firstColor) {
      colorArgs = args.slice(1)
    }
  }

  if (colorArgs.length < 2) {
    return null
  }

  const start = parseCssColor(
    extractGradientColorToken(colorArgs[0] || ''),
    resolveCssVariable,
  ) ?? {
    hex: '#0A84FF',
    alpha: 100,
    hasAlpha: false,
  }
  const end = parseCssColor(extractGradientColorToken(colorArgs[1] || ''), resolveCssVariable) ?? {
    hex: '#3B82F6',
    alpha: 100,
    hasAlpha: false,
  }

  return {
    type,
    angle,
    start,
    end,
  }
}

export function buildCssColor(hex: string, alphaEnabled: boolean, alpha: number) {
  const normalized = normalizeHex(hex)
  if (!normalized) {
    return '#000000'
  }

  if (!alphaEnabled) {
    return normalized
  }

  const channels = hexToRgbChannels(normalized)
  if (!channels) {
    return normalized
  }

  const [r, g, b] = channels
  const alphaFraction = Math.max(0, Math.min(1, alpha / 100))
  const alphaValue = Number(alphaFraction.toFixed(2))
  return `rgba(${r}, ${g}, ${b}, ${alphaValue})`
}

export function buildGradientValue(options: GradientValueOptions) {
  const startColor = buildCssColor(
    options.start.hex,
    options.start.alphaEnabled,
    options.start.alpha,
  )
  const endColor = buildCssColor(options.end.hex, options.end.alphaEnabled, options.end.alpha)
  const angle = Math.round(clampAngle(options.angle))

  if (options.type === 'radial') {
    return `radial-gradient(circle, ${startColor}, ${endColor})`
  }

  if (options.type === 'conic') {
    return `conic-gradient(from ${angle}deg, ${startColor}, ${endColor})`
  }

  return `linear-gradient(${angle}deg, ${startColor}, ${endColor})`
}
