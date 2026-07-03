// omni-theme-catalog.ts — CATÁLOGO data-driven dos temas do Omni.
//
// Fonte ÚNICA de verdade de QUAIS temas existem e de seus valores-padrão.
// Adicionar um tema = uma entrada em OMNI_THEMES (+ o CSS do tema em
// omni-tokens.css). Sem enum espalhado por vários mapas/switch: rótulos,
// defaults, seletor CSS e base de cor são DERIVADOS deste array. O back também
// não tem enum de tema (valida só slug), então tema novo não exige deploy do Go.

export type ThemeVars = Record<string, string>

// União mantida por type-safety; o array OMNI_THEMES abaixo é a fonte de dados.
// Adicionar um tema: incluir o id aqui e a entrada correspondente em OMNI_THEMES.
export type OmniThemeName = 'light' | 'dark' | 'apple' | 'custom' | 'liquidglass'

export type OmniThemeVariableKind = 'text' | 'rgb-triplet' | 'css-color' | 'css-gradient'
export type OmniThemeVariableGroup = 'foundation' | 'surface' | 'accent' | 'header' | 'page'

export interface OmniThemeVariable {
  key: string
  label: string
  group: OmniThemeVariableGroup
  kind: OmniThemeVariableKind
}

export type OmniThemeOverrides = Record<OmniThemeName, ThemeVars>

// OmniThemeDescriptor descreve um tema: rótulo, base de cor (light/dark — define
// a classe `.dark` + color-scheme dos componentes) e o seletor CSS onde os
// tokens do tema vivem (`:root`, `.dark`, `.theme-*`).
export interface OmniThemeDescriptor {
  id: OmniThemeName
  label: string
  base: 'light' | 'dark'
  selectorClass: string
  defaults: ThemeVars
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
  {
    key: 'admin-header-fade-bottom',
    label: 'Header Fade Bottom',
    group: 'header',
    kind: 'css-gradient',
  },
  {
    key: 'admin-header-fade-top-size',
    label: 'Header Fade Top Size',
    group: 'header',
    kind: 'text',
  },
  {
    key: 'admin-header-fade-bottom-size',
    label: 'Header Fade Bottom Size',
    group: 'header',
    kind: 'text',
  },

  {
    key: 'admin-page-header-eyebrow-display',
    label: 'Page Header Eyebrow Display',
    group: 'page',
    kind: 'text',
  },
  {
    key: 'admin-page-header-title-display',
    label: 'Page Header Title Display',
    group: 'page',
    kind: 'text',
  },
  {
    key: 'admin-page-header-description-display',
    label: 'Page Header Description Display',
    group: 'page',
    kind: 'text',
  },
]

const SHARED_THEME_DEFAULTS: ThemeVars = {
  'font-sans':
    '"Inter", ui-sans-serif, system-ui, -apple-system, "SF Pro Display", "SF Pro Text", "Segoe UI", Roboto, Arial, "Apple Color Emoji", "Segoe UI Emoji"',
  'radius-xs': '10px',
  'radius-sm': '12px',
  'radius-md': '14px',
  'radius-lg': '18px',
  'shadow-glow-color': 'rgb(var(--primary) / 0.34)',
  'shadow-xs': '0 1px 0 color-mix(in srgb, var(--shadow-color) 35%, transparent)',
  'shadow-sm': '0 8px 24px color-mix(in srgb, var(--shadow-color) 55%, transparent)',
  'shadow-md': '0 14px 40px color-mix(in srgb, var(--shadow-color) 72%, transparent)',
  'shadow-glow':
    '0 0 0 1px color-mix(in srgb, var(--shadow-glow-color) 70%, transparent), 0 14px 44px color-mix(in srgb, var(--shadow-glow-color) 50%, transparent)',
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
  'admin-page-header-description-display': 'block',
}

const LIGHT_DEFAULTS: ThemeVars = {
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
  ring: '99 102 241',
}

const DARK_DEFAULTS: ThemeVars = {
  ...SHARED_THEME_DEFAULTS,
  'shadow-color': 'rgba(2, 6, 23, 0.72)',
  bg: '6 10 18',
  surface: '13 18 29',
  'surface-2': '18 25 38',
  border: '31 41 55',
  text: '226 232 240',
  muted: '148 163 184',
  primary: '99 102 241',
  'primary-600': '79 70 229',
  success: '34 197 94',
  danger: '248 113 113',
  ring: '99 102 241',
}

const APPLE_DEFAULTS: ThemeVars = {
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
  ring: '10 132 255',
}

// Liquid Glass — base escura para a aurora ambiente brilhar e o vidro refratar.
// Espelha o bloco `.theme-liquidglass` de omni-tokens.css (mantenha os dois em
// sincronia; estes defaults alimentam o editor do Theme Studio).
const LIQUIDGLASS_DEFAULTS: ThemeVars = {
  ...SHARED_THEME_DEFAULTS,
  'shadow-color': 'rgba(2, 6, 23, 0.72)',
  bg: '8 12 22',
  surface: '17 24 39',
  'surface-2': '24 33 52',
  border: '51 65 92',
  text: '226 232 240',
  muted: '148 163 184',
  primary: '129 140 248',
  'primary-600': '99 102 241',
  success: '52 211 153',
  danger: '248 113 113',
  ring: '129 140 248',
}

export const OMNI_THEMES: OmniThemeDescriptor[] = [
  { id: 'light', label: 'Light', base: 'light', selectorClass: ':root', defaults: LIGHT_DEFAULTS },
  { id: 'dark', label: 'Dark', base: 'dark', selectorClass: '.dark', defaults: DARK_DEFAULTS },
  {
    id: 'apple',
    label: 'Apple-Blue',
    base: 'light',
    selectorClass: '.theme-apple-blue',
    defaults: APPLE_DEFAULTS,
  },
  {
    id: 'liquidglass',
    label: 'Liquid Glass',
    base: 'dark',
    selectorClass: '.theme-liquidglass',
    defaults: LIQUIDGLASS_DEFAULTS,
  },
  {
    id: 'custom',
    label: 'Custom',
    base: 'light',
    selectorClass: '.theme-custom',
    defaults: LIGHT_DEFAULTS,
  },
]

export const ALL_THEMES: OmniThemeName[] = OMNI_THEMES.map((theme) => theme.id)

export const OMNI_THEME_LABELS: Record<OmniThemeName, string> = OMNI_THEMES.reduce(
  (labels, theme) => {
    labels[theme.id] = theme.label
    return labels
  },
  {} as Record<OmniThemeName, string>,
)

export const OMNI_THEME_DEFAULTS: Record<OmniThemeName, ThemeVars> = OMNI_THEMES.reduce(
  (defaults, theme) => {
    defaults[theme.id] = theme.defaults
    return defaults
  },
  {} as Record<OmniThemeName, ThemeVars>,
)

export function getThemeDescriptor(id: OmniThemeName): OmniThemeDescriptor | undefined {
  return OMNI_THEMES.find((theme) => theme.id === id)
}

export function selectorByTheme(theme: OmniThemeName): string {
  return getThemeDescriptor(theme)?.selectorClass || ':root'
}

export function isThemeName(value: string | null): value is OmniThemeName {
  return !!value && ALL_THEMES.includes(value as OmniThemeName)
}

export function isBaseThemeName(value: string | null): value is 'light' | 'dark' {
  return value === 'light' || value === 'dark'
}

export function normalizeVariableKey(key: string): string {
  return key.replace(/^--/, '').trim()
}

export function createEmptyOverrides(): OmniThemeOverrides {
  return ALL_THEMES.reduce((overrides, theme) => {
    overrides[theme] = {}
    return overrides
  }, {} as OmniThemeOverrides)
}

export function sanitizeOverrides(value: unknown): OmniThemeOverrides {
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

export function rgbTripletToHex(value: string) {
  const numbers = value.trim().match(/\d+/g)
  if (!numbers || numbers.length < 3) {
    return null
  }

  const channelNumbers = numbers
    .slice(0, 3)
    .map((rawNumber) => Math.max(0, Math.min(255, Number(rawNumber) || 0)))

  const [r = 0, g = 0, b = 0] = channelNumbers
  return `#${[r, g, b].map((channel) => channel.toString(16).padStart(2, '0')).join('')}`
}

export function hexToRgbTriplet(hex: string) {
  const parsed = hex.replace('#', '').trim()
  if (!parsed) {
    return null
  }

  const full =
    parsed.length === 3
      ? parsed
          .split('')
          .map((char) => `${char}${char}`)
          .join('')
      : parsed

  if (full.length !== 6) {
    return null
  }

  const r = Number.parseInt(full.slice(0, 2), 16)
  const g = Number.parseInt(full.slice(2, 4), 16)
  const b = Number.parseInt(full.slice(4, 6), 16)

  if ([r, g, b].some((n) => Number.isNaN(n))) {
    return null
  }

  return `${r} ${g} ${b}`
}
