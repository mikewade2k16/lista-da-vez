const FALLBACK_ALERT_COLOR = "#f59e0b"

const legacyThemeColors: Record<string, string> = {
  amber: FALLBACK_ALERT_COLOR,
  red: "#ef4444",
  blue: "#3b82f6",
  green: "#10b981",
  purple: "#a855f7",
  slate: "#64748b"
}

function normalizeRawHex(value: unknown) {
  const trimmed = String(value || "").trim()
  const legacy = legacyThemeColors[trimmed.toLowerCase()]
  const raw = legacy || trimmed

  if (/^#[0-9a-f]{3}$/i.test(raw)) {
    return raw
      .slice(1)
      .split("")
      .map((char) => char + char)
      .join("")
      .toUpperCase()
  }

  if (/^#[0-9a-f]{6}$/i.test(raw)) {
    return raw.slice(1).toUpperCase()
  }

  if (/^[0-9a-f]{6}$/i.test(raw)) {
    return raw.toUpperCase()
  }

  return ""
}

export function isValidAlertHexColor(value: unknown) {
  return Boolean(normalizeRawHex(value))
}

export function normalizeAlertHexColor(value: unknown, fallback = FALLBACK_ALERT_COLOR) {
  const normalized = normalizeRawHex(value)
  if (normalized) {
    return `#${normalized}`
  }

  const fallbackHex = normalizeRawHex(fallback) || normalizeRawHex(FALLBACK_ALERT_COLOR)
  return `#${fallbackHex}`
}

export function alertHexToRgb(color: unknown) {
  const hex = normalizeAlertHexColor(color).slice(1)
  return [
    Number.parseInt(hex.slice(0, 2), 16),
    Number.parseInt(hex.slice(2, 4), 16),
    Number.parseInt(hex.slice(4, 6), 16)
  ] as const
}

export function alertRgbTriplet(color: unknown) {
  return alertHexToRgb(color).join(", ")
}

function mixChannel(source: number, target: number, amount: number) {
  return Math.round(source + (target - source) * amount)
}

export function mixAlertColor(color: unknown, target: "#000000" | "#ffffff", amount: number) {
  const [red, green, blue] = alertHexToRgb(color)
  const targetValue = target === "#ffffff" ? 255 : 0
  const boundedAmount = Math.max(0, Math.min(1, Number(amount || 0)))
  const mixed = [red, green, blue].map((channel) => mixChannel(channel, targetValue, boundedAmount))

  return `#${mixed.map((channel) => channel.toString(16).padStart(2, "0")).join("")}`
}

export function alertGradient(color: unknown) {
  const start = normalizeAlertHexColor(color)
  const end = mixAlertColor(start, "#000000", 0.28)
  return `linear-gradient(135deg, ${start} 0%, ${end} 100%)`
}

export function alertBannerStyle(color: unknown) {
  const normalized = normalizeAlertHexColor(color)
  const dark = mixAlertColor(normalized, "#000000", 0.42)
  const darker = mixAlertColor(normalized, "#000000", 0.58)
  const light = mixAlertColor(normalized, "#ffffff", 0.24)

  return {
    "--alert-color": normalized,
    "--alert-color-rgb": alertRgbTriplet(normalized),
    "--alert-color-dark": dark,
    "--alert-color-darker": darker,
    "--alert-color-light": light
  }
}

export function alertCardStyle(color: unknown) {
  const normalized = normalizeAlertHexColor(color)

  return {
    "--service-card-alert-rgb": alertRgbTriplet(normalized),
    "--service-card-alert-bg-rgb": alertRgbTriplet(mixAlertColor(normalized, "#000000", 0.62)),
    "--service-card-alert-text": mixAlertColor(normalized, "#ffffff", 0.18)
  }
}
