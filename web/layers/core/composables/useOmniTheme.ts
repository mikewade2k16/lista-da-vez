export type OmniThemeName = 'light' | 'dark' | 'apple' | 'custom'

type ThemeVars = Record<string, string>
type OmniThemeOverrides = Record<OmniThemeName, ThemeVars>

export type OmniThemeVariableKind = 'text' | 'rgb-triplet' | 'css-color' | 'css-gradient'
export type OmniThemeVariableGroup = 'foundation' | 'surface' | 'accent' | 'header' | 'page'

export interface OmniThemeVariable {
  key: string
  label: string
  group: OmniThemeVariableGroup
  kind: OmniThemeVariableKind
}

const THEME_STORAGE_KEY = 'omni-ui-theme'
const OVERRIDES_STORAGE_KEY = 'omni-ui-theme-overrides'
const CUSTOM_THEME_NAME_KEY = 'omni-ui-theme-custom-name'
const STYLE_TAG_ID = 'omni-theme-overrides-style'

const ALL_THEMES: OmniThemeName[] = ['light', 'dark', 'apple', 'custom']

export const OMNI_THEME_LABELS: Record<OmniThemeName, string> = {
  light: 'Light',
  dark: 'Dark',
  apple: 'Apple-Blue',
  custom: 'Custom'
}

export const OMNI_THEME_VARIABLES: OmniThemeVariable[] = [
  { key: 'font-sans', label: 'Font Sans', group: 'foundation', kind: 'text' },
  { key: 'radius-xs', label: 'Radius XS', group: 'foundation', kind: 'text' },
  { key: 'radius-sm', label: 'Radius SM', group: 'foundation', kind: 'text' },
  { key: 'radius-md', label: 'Radius MD', group: 'foundation', kind: 'text' },
  { key: 'radius-lg', label: 'Radius LG', group: 'foundation', kind: 'text' },
  { key: 'shadow-color', label: 'Shadow Color', group: 'foundation', kind: 'css-color' },
  { key: 'shadow-glow-color', label: 'Shadow Glow Color', group: 'foundation', kind: 'css-color' },
  { key: 'shadow-xs', label: 'Shadow XS', group: 'foundation', kind: 'text' },
  { key: 'shadow-sm', label: 'Shadow SM', group: 'foundation', kind: 'text' },
  { key: 'shadow-md', label: 'Shadow MD', group: 'foundation', kind: 'text' },
  { key: 'shadow-glow', label: 'Shadow Glow', group: 'foundation', kind: 'text' },

  { key: 'bg', label: 'Background', group: 'surface', kind: 'rgb-triplet' },
  { key: 'surface', label: 'Surface', group: 'surface', kind: 'rgb-triplet' },
  { key: 'surface-2', label: 'Surface 2', group: 'surface', kind: 'rgb-triplet' },
  { key: 'border', label: 'Border', group: 'surface', kind: 'rgb-triplet' },
  { key: 'text', label: 'Text', group: 'surface', kind: 'rgb-triplet' },
  { key: 'muted', label: 'Muted', group: 'surface', kind: 'rgb-triplet' },

  { key: 'primary', label: 'Primary', group: 'accent', kind: 'rgb-triplet' },
  { key: 'primary-600', label: 'Primary 600', group: 'accent', kind: 'rgb-triplet' },
  { key: 'success', label: 'Success', group: 'accent', kind: 'rgb-triplet' },
  { key: 'danger', label: 'Danger', group: 'accent', kind: 'rgb-triplet' },
  { key: 'ring', label: 'Ring', group: 'accent', kind: 'rgb-triplet' },

  { key: 'admin-header-brand-bg', label: 'Header Brand BG', group: 'header', kind: 'css-gradient' },
  { key: 'admin-header-panel-bg', label: 'Header Panel BG', group: 'header', kind: 'css-gradient' },
  { key: 'admin-header-brand-blur', label: 'Header Brand Blur', group: 'header', kind: 'text' },
  { key: 'admin-header-panel-blur', label: 'Header Panel Blur', group: 'header', kind: 'text' },
  { key: 'admin-header-border', label: 'Header Border', group: 'header', kind: 'css-color' },
  { key: 'admin-header-separator', label: 'Header Separator', group: 'header', kind: 'css-color' },
  { key: 'admin-header-text', label: 'Header Text', group: 'header', kind: 'css-color' },
  { key: 'admin-header-muted', label: 'Header Muted', group: 'header', kind: 'css-color' },
  { key: 'admin-header-hover-bg', label: 'Header Hover BG', group: 'header', kind: 'css-color' },
  { key: 'admin-header-active-bg', label: 'Header Active BG', group: 'header', kind: 'css-color' },
  { key: 'admin-header-shell-shadow', label: 'Header Shell Shadow', group: 'header', kind: 'text' },
  { key: 'admin-header-fade-top', label: 'Header Fade Top', group: 'header', kind: 'css-gradient' },
  { key: 'admin-header-fade-bottom', label: 'Header Fade Bottom', group: 'header', kind: 'css-gradient' },
  { key: 'admin-header-fade-top-size', label: 'Header Fade Top Size', group: 'header', kind: 'text' },
  { key: 'admin-header-fade-bottom-size', label: 'Header Fade Bottom Size', group: 'header', kind: 'text' },

  { key: 'admin-page-header-eyebrow-display', label: 'Page Header Eyebrow Display', group: 'page', kind: 'text' },
  { key: 'admin-page-header-title-display', label: 'Page Header Title Display', group: 'page', kind: 'text' },
  { key: 'admin-page-header-description-display', label: 'Page Header Description Display', group: 'page', kind: 'text' }
]

const SHARED_THEME_DEFAULTS: ThemeVars = {
  'font-sans': '"Inter", ui-sans-serif, system-ui, -apple-system, "SF Pro Display", "SF Pro Text", "Segoe UI", Roboto, Arial, "Apple Color Emoji", "Segoe UI Emoji"',
  'radius-xs': '10px',
  'radius-sm': '12px',
  'radius-md': '14px',
  'radius-lg': '18px',
  'shadow-glow-color': 'rgb(var(--primary) / 0.34)',
  'shadow-xs': '0 1px 0 color-mix(in srgb, var(--shadow-color) 35%, transparent)',
  'shadow-sm': '0 8px 24px color-mix(in srgb, var(--shadow-color) 55%, transparent)',
  'shadow-md': '0 14px 40px color-mix(in srgb, var(--shadow-color) 72%, transparent)',
  'shadow-glow': '0 0 0 1px color-mix(in srgb, var(--shadow-glow-color) 70%, transparent), 0 14px 44px color-mix(in srgb, var(--shadow-glow-color) 50%, transparent)',
  'admin-header-brand-bg': 'linear-gradient(180deg, rgb(var(--surface)), rgb(var(--surface-2)))',
  'admin-header-panel-bg': 'linear-gradient(180deg, rgb(var(--surface)), rgb(var(--surface-2)))',
  'admin-header-brand-blur': '0px',
  'admin-header-panel-blur': '0px',
  'admin-header-border': 'rgb(var(--border) / 0.9)',
  'admin-header-separator': 'rgb(var(--border) / 0.9)',
  'admin-header-text': 'rgb(var(--text))',
  'admin-header-muted': 'rgb(var(--muted))',
  'admin-header-hover-bg': 'rgb(var(--primary) / 0.16)',
  'admin-header-active-bg': 'rgb(var(--primary) / 0.16)',
  'admin-header-shell-shadow': 'none',
  'admin-header-fade-top': 'none',
  'admin-header-fade-bottom': 'none',
  'admin-header-fade-top-size': '0px',
  'admin-header-fade-bottom-size': '0px',
  'admin-page-header-eyebrow-display': 'block',
  'admin-page-header-title-display': 'block',
  'admin-page-header-description-display': 'block'
}

export const OMNI_THEME_DEFAULTS: Record<Exclude<OmniThemeName, 'custom'>, ThemeVars> = {
  light: {
    ...SHARED_THEME_DEFAULTS,
    'shadow-color': 'rgba(15, 23, 42, 0.24)',
    bg: '248 250 252',
    surface: '255 255 255',
    'surface-2': '244 246 250',
    border: '226 232 240',
    text: '15 23 42',
    muted: '100 116 139',
    primary: '99 102 241',
    'primary-600': '79 70 229',
    success: '34 197 94',
    danger: '239 68 68',
    ring: '99 102 241'
  },
  dark: {
    ...SHARED_THEME_DEFAULTS,
    'shadow-color': 'rgba(2, 6, 23, 0.72)',
    bg: '6 10 18',
    surface: '13 18 29',
    'surface-2': '18 25 38',
    border: '31 41 55',
    text: '226 232 240',
    muted: '148 163 184',
    primary: '129 140 248',
    'primary-600': '165 180 252',
    success: '34 197 94',
    danger: '248 113 113',
    ring: '129 140 248'
  },
  apple: {
    ...SHARED_THEME_DEFAULTS,
    'shadow-color': 'rgba(8, 59, 125, 0.24)',
    bg: '236 246 255',
    surface: '247 252 255',
    'surface-2': '228 241 255',
    border: '176 210 242',
    text: '12 52 98',
    muted: '67 105 147',
    primary: '10 132 255',
    'primary-600': '0 122 255',
    success: '34 197 94',
    danger: '239 68 68',
    ring: '10 132 255'
  }
}

function createEmptyOverrides(): OmniThemeOverrides {
  return {
    light: {},
    dark: {},
    apple: {},
    custom: {}
  }
}

function isThemeName(value: string | null): value is OmniThemeName {
  return value === 'light' || value === 'dark' || value === 'apple' || value === 'custom'
}

function normalizeVariableKey(key: string) {
  return key.replace(/^--/, '').trim()
}

function sanitizeOverrides(value: unknown): OmniThemeOverrides {
  const fallback = createEmptyOverrides()

  if (!value || typeof value !== 'object') {
    return fallback
  }

  for (const theme of ALL_THEMES) {
    const source = (value as Record<string, unknown>)[theme]
    if (!source || typeof source !== 'object') {
      continue
    }

    for (const [rawKey, rawValue] of Object.entries(source as Record<string, unknown>)) {
      const key = normalizeVariableKey(rawKey)
      if (!key || typeof rawValue !== 'string') {
        continue
      }

      fallback[theme][key] = rawValue
    }
  }

  return fallback
}

function selectorByTheme(theme: OmniThemeName) {
  if (theme === 'light') return ':root'
  if (theme === 'dark') return '.dark'
  if (theme === 'apple') return '.theme-apple-blue'
  return '.theme-custom'
}

export function rgbTripletToHex(value: string) {
  const numbers = value.trim().match(/\d+/g)
  if (!numbers || numbers.length < 3) {
    return null
  }

  const channelNumbers = numbers
    .slice(0, 3)
    .map((rawNumber) => Math.max(0, Math.min(255, Number(rawNumber) || 0)))

  const [r = 0, g = 0, b = 0] = channelNumbers
  return `#${[r, g, b].map(channel => channel.toString(16).padStart(2, '0')).join('')}`
}

export function hexToRgbTriplet(hex: string) {
  const parsed = hex.replace('#', '').trim()
  if (!parsed) {
    return null
  }

  const full = parsed.length === 3
    ? parsed.split('').map(char => `${char}${char}`).join('')
    : parsed

  if (full.length !== 6) {
    return null
  }

  const r = Number.parseInt(full.slice(0, 2), 16)
  const g = Number.parseInt(full.slice(2, 4), 16)
  const b = Number.parseInt(full.slice(4, 6), 16)

  if ([r, g, b].some(n => Number.isNaN(n))) {
    return null
  }

  return `${r} ${g} ${b}`
}

export function useOmniTheme() {
  const colorMode = useColorMode()
  const initialized = useState<boolean>('omni-theme-initialized', () => false)
  const currentTheme = useState<OmniThemeName>('omni-theme-current', () => 'light')
  const overrides = useState<OmniThemeOverrides>('omni-theme-overrides', () => createEmptyOverrides())
  const customThemeName = useState<string>('omni-theme-custom-name', () => OMNI_THEME_LABELS.custom)

  const hasCustomTheme = computed(() => Object.keys(overrides.value.custom).length > 0)

  function normalizeThemeName(name: string) {
    const normalized = name.trim()
    return normalized || OMNI_THEME_LABELS.custom
  }

  function persistOverrides() {
    if (!import.meta.client) {
      return
    }

    localStorage.setItem(OVERRIDES_STORAGE_KEY, JSON.stringify(overrides.value))
  }

  function applyOverrides() {
    if (!import.meta.client) {
      return
    }

    const cssBlocks: string[] = []

    for (const theme of ALL_THEMES) {
      const entries = Object.entries(overrides.value[theme] || {})
        .filter(([, value]) => value && value.trim().length > 0)

      if (!entries.length) {
        continue
      }

      const selector = selectorByTheme(theme)
      const declarations = entries.map(([key, value]) => `  --${normalizeVariableKey(key)}: ${value};`).join('\n')
      cssBlocks.push(`${selector} {\n${declarations}\n}`)
    }

    const cssText = cssBlocks.join('\n\n')
    const head = document.head
    if (!head) {
      return
    }

    let styleTag = document.getElementById(STYLE_TAG_ID) as HTMLStyleElement | null
    if (!cssText) {
      styleTag?.remove()
      return
    }

    if (!styleTag) {
      styleTag = document.createElement('style')
      styleTag.id = STYLE_TAG_ID
      head.appendChild(styleTag)
    }

    styleTag.textContent = cssText
  }

  function detectThemeFromDom(): OmniThemeName {
    if (!import.meta.client) {
      return 'light'
    }

    const root = document.documentElement
    if (root.classList.contains('theme-custom')) return 'custom'
    if (root.classList.contains('theme-apple-blue')) return 'apple'
    if (root.classList.contains('dark') || colorMode.value === 'dark') return 'dark'
    return 'light'
  }

  function applyTheme(theme: OmniThemeName, persist = true) {
    currentTheme.value = theme

    if (!import.meta.client) {
      return
    }

    const root = document.documentElement
    root.classList.remove('theme-apple-blue', 'theme-custom')

    if (theme === 'dark') {
      root.classList.add('dark')
      colorMode.preference = 'dark'
    } else {
      root.classList.remove('dark')
      colorMode.preference = 'light'
    }

    if (theme === 'apple') {
      root.classList.add('theme-apple-blue')
    }

    if (theme === 'custom') {
      root.classList.add('theme-custom')
    }

    if (persist) {
      localStorage.setItem(THEME_STORAGE_KEY, theme)
    }
  }

  function initializeFromStorage() {
    if (!import.meta.client || initialized.value) {
      return
    }

    const rawOverrides = localStorage.getItem(OVERRIDES_STORAGE_KEY)
    if (rawOverrides) {
      try {
        overrides.value = sanitizeOverrides(JSON.parse(rawOverrides))
      } catch {
        overrides.value = createEmptyOverrides()
      }
    }

    applyOverrides()

    const storedTheme = localStorage.getItem(THEME_STORAGE_KEY)
    const storedCustomThemeName = localStorage.getItem(CUSTOM_THEME_NAME_KEY)
    if (storedCustomThemeName) {
      customThemeName.value = normalizeThemeName(storedCustomThemeName)
    }

    if (isThemeName(storedTheme)) {
      applyTheme(storedTheme, false)
    } else {
      applyTheme(detectThemeFromDom(), false)
    }

    initialized.value = true
  }

  function getThemeDefaults(theme: OmniThemeName): ThemeVars {
    if (theme === 'custom') {
      return { ...OMNI_THEME_DEFAULTS.light }
    }

    return { ...OMNI_THEME_DEFAULTS[theme] }
  }

  function getResolvedThemeValues(theme: OmniThemeName) {
    return {
      ...getThemeDefaults(theme),
      ...(overrides.value[theme] || {})
    }
  }

  function getThemeValue(theme: OmniThemeName, key: string) {
    const normalizedKey = normalizeVariableKey(key)
    const overrideValue = overrides.value[theme]?.[normalizedKey]
    if (overrideValue !== undefined) {
      return overrideValue
    }

    const defaults = getThemeDefaults(theme)
    return defaults[normalizedKey] ?? ''
  }

  function setThemeValue(theme: OmniThemeName, key: string, value: string) {
    const normalizedKey = normalizeVariableKey(key)
    if (!normalizedKey) {
      return
    }

    overrides.value = {
      ...overrides.value,
      [theme]: {
        ...overrides.value[theme],
        [normalizedKey]: value
      }
    }

    applyOverrides()
    persistOverrides()
  }

  function setThemeValues(theme: OmniThemeName, values: ThemeVars) {
    const nextValues: ThemeVars = {}

    for (const [rawKey, rawValue] of Object.entries(values)) {
      const key = normalizeVariableKey(rawKey)
      if (!key) {
        continue
      }

      nextValues[key] = rawValue
    }

    overrides.value = {
      ...overrides.value,
      [theme]: nextValues
    }

    applyOverrides()
    persistOverrides()
  }

  function resetThemeOverrides(theme: OmniThemeName) {
    overrides.value = {
      ...overrides.value,
      [theme]: {}
    }

    applyOverrides()
    persistOverrides()
  }

  function duplicateTheme(source: OmniThemeName, target: OmniThemeName = 'custom') {
    setThemeValues(target, getResolvedThemeValues(source))
  }

  function setCustomThemeName(name: string, persist = true) {
    const normalized = normalizeThemeName(name)
    customThemeName.value = normalized

    if (!import.meta.client || !persist) {
      return
    }

    localStorage.setItem(CUSTOM_THEME_NAME_KEY, normalized)
  }

  function getThemeLabel(theme: OmniThemeName) {
    if (theme === 'custom') {
      return customThemeName.value
    }

    return OMNI_THEME_LABELS[theme]
  }

  return {
    currentTheme,
    overrides,
    customThemeName,
    hasCustomTheme,
    initializeFromStorage,
    applyTheme,
    getThemeLabel,
    setCustomThemeName,
    getThemeValue,
    getThemeDefaults,
    getResolvedThemeValues,
    setThemeValue,
    setThemeValues,
    resetThemeOverrides,
    duplicateTheme
  }
}
