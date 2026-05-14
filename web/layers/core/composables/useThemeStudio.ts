import { OMNI_THEME_VARIABLES, hexToRgbTriplet, useOmniTheme, type OmniThemeName } from './useOmniTheme'

const THEME_ORDER: OmniThemeName[] = ['light', 'dark', 'apple', 'custom']
const GROUP_ORDER: Array<'surface' | 'accent' | 'foundation' | 'header' | 'page'> = ['surface', 'accent', 'foundation', 'header', 'page']

export const RADIUS_KEYS = ['radius-xs', 'radius-sm', 'radius-md', 'radius-lg'] as const
export const SHADOW_COLOR_KEYS = ['shadow-color', 'shadow-glow-color'] as const

const HEADER_TEXT_LINK_VALUE = 'rgb(var(--text))'
const HEADER_PRIMARY_ACTION_VALUE = 'rgb(var(--primary) / 0.16)'

export interface FontItem {
  label: string
  value: string
}

export interface ThemeStudioHeaderState {
  linkBackgrounds: boolean
  linkBlurs: boolean
  linkDividers: boolean
  linkInteractions: boolean
  textFollowTheme: boolean
  actionFollowPrimary: boolean
}

export interface ThemeStudioPageHeaderState {
  showEyebrow: boolean
  showTitle: boolean
  showDescription: boolean
}

export function useThemeStudio() {
  const {
    currentTheme,
    customThemeName,
    hasCustomTheme,
    initializeFromStorage,
    applyTheme,
    getThemeLabel,
    setCustomThemeName,
    getThemeValue,
    setThemeValue,
    resetThemeOverrides,
    duplicateTheme
  } = useOmniTheme()

  const selectedTheme = ref<OmniThemeName>('light')
  const search = ref('')
  const showDetailedEditors = ref(false)
  const customThemeNameDraft = ref('')

  const headerState = reactive<ThemeStudioHeaderState>({
    linkBackgrounds: true,
    linkBlurs: true,
    linkDividers: true,
    linkInteractions: true,
    textFollowTheme: true,
    actionFollowPrimary: true
  })
  const pageHeaderState = reactive<ThemeStudioPageHeaderState>({
    showEyebrow: true,
    showTitle: true,
    showDescription: true
  })

  const fontItems: FontItem[] = [
    {
      label: 'Inter',
      value: '"Inter", ui-sans-serif, system-ui, -apple-system, "SF Pro Display", "SF Pro Text", "Segoe UI", Roboto, Arial, "Apple Color Emoji", "Segoe UI Emoji"'
    },
    {
      label: 'Montserrat',
      value: '"Montserrat", ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, Arial, "Apple Color Emoji", "Segoe UI Emoji"'
    },
    {
      label: 'Manrope',
      value: '"Manrope", ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, Arial, "Apple Color Emoji", "Segoe UI Emoji"'
    },
    {
      label: 'Poppins',
      value: '"Poppins", ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, Arial, "Apple Color Emoji", "Segoe UI Emoji"'
    },
    {
      label: 'Nunito Sans',
      value: '"Nunito Sans", ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, Arial, "Apple Color Emoji", "Segoe UI Emoji"'
    },
    {
      label: 'System Sans',
      value: 'ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, Arial, "Apple Color Emoji", "Segoe UI Emoji"'
    }
  ]

  const isSimpleMode = computed(() => !showDetailedEditors.value)

  const themeLabels = computed<Record<OmniThemeName, string>>(() => ({
    light: getThemeLabel('light'),
    dark: getThemeLabel('dark'),
    apple: getThemeLabel('apple'),
    custom: getThemeLabel('custom')
  }))

  const groupedVariables = computed(() => {
    const groups = {
      foundation: [] as typeof OMNI_THEME_VARIABLES,
      surface: [] as typeof OMNI_THEME_VARIABLES,
      accent: [] as typeof OMNI_THEME_VARIABLES,
      header: [] as typeof OMNI_THEME_VARIABLES,
      page: [] as typeof OMNI_THEME_VARIABLES
    }

    const query = search.value.trim().toLowerCase()
    const hiddenInSimpleMode = new Set(['foundation', 'header', 'page'])

    for (const variable of OMNI_THEME_VARIABLES) {
      if (isSimpleMode.value && hiddenInSimpleMode.has(variable.group)) {
        continue
      }

      if (query && !`${variable.label} ${variable.key}`.toLowerCase().includes(query)) {
        continue
      }

      groups[variable.group].push(variable)
    }

    return groups
  })

  const groupedSections = computed(() =>
    GROUP_ORDER
      .map(group => ({
        key: group,
        items: groupedVariables.value[group]
      }))
      .filter(section => section.items.length > 0)
  )

  const variableValueMap = computed<Record<string, string>>(() => {
    const valueMap: Record<string, string> = {}
    for (const variable of OMNI_THEME_VARIABLES) {
      valueMap[variable.key] = variableValue(variable.key)
    }
    return valueMap
  })

  const radiusValueMap = computed<Record<string, number>>(() =>
    Object.fromEntries(RADIUS_KEYS.map(key => [key, radiusNumber(key)]))
  )

  const tripletModelValueMap = computed<Record<string, string>>(() => {
    const valueMap: Record<string, string> = {}
    for (const variable of OMNI_THEME_VARIABLES) {
      if (variable.kind !== 'rgb-triplet') {
        continue
      }

      valueMap[variable.key] = tripletModelValue(variable.key)
    }
    return valueMap
  })

  onMounted(() => {
    initializeFromStorage()
    selectedTheme.value = currentTheme.value
    customThemeNameDraft.value = customThemeName.value
    syncHeaderControlState()
    syncPageHeaderControlState()
  })

  watch(currentTheme, value => {
    selectedTheme.value = value
    syncHeaderControlState()
    syncPageHeaderControlState()
  })

  watch(selectedTheme, () => {
    syncHeaderControlState()
    syncPageHeaderControlState()
  })

  watch(customThemeName, value => {
    customThemeNameDraft.value = value
  })

  function toGroupLabel(group: string) {
    if (group === 'surface') return 'Cores Base'
    if (group === 'accent') return 'Acentos'
    if (group === 'foundation') return 'Foundation'
    if (group === 'page') return 'Page Headers'
    return 'Header'
  }

  function selectTheme(theme: OmniThemeName) {
    selectedTheme.value = theme
    applyTheme(theme)
  }

  function applySelectedTheme() {
    applyTheme(selectedTheme.value)
  }

  function applyThemeIfNeeded() {
    if (currentTheme.value !== selectedTheme.value) {
      applyTheme(selectedTheme.value)
    }
  }

  function resetSelectedThemeOverrides() {
    resetThemeOverrides(selectedTheme.value)
    syncHeaderControlState()
    syncPageHeaderControlState()
  }

  function createCustomFromSelected() {
    const normalizedName = customThemeNameDraft.value.trim() || 'Tema Custom'

    duplicateTheme(selectedTheme.value, 'custom')
    setCustomThemeName(normalizedName)

    selectedTheme.value = 'custom'
    applyTheme('custom')
  }

  function saveCustomThemeName() {
    setCustomThemeName(customThemeNameDraft.value)
  }

  function setCustomThemeNameDraft(value: string) {
    customThemeNameDraft.value = value
  }

  function setSearch(value: string) {
    search.value = value
  }

  function setShowDetailedEditors(value: boolean) {
    showDetailedEditors.value = value
  }

  function variableValue(key: string) {
    return getThemeValue(selectedTheme.value, key)
  }

  function onVariableTextValue(key: string, value: string | number | undefined) {
    setThemeValue(selectedTheme.value, key, String(value ?? ''))
    applyThemeIfNeeded()

    if (key.startsWith('admin-header-')) {
      syncHeaderControlState()
    } else if (key.startsWith('admin-page-header-')) {
      syncPageHeaderControlState()
    }
  }

  function parseRgbChannel(token: string) {
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

  function normalizeTriplet(r: number, g: number, b: number) {
    return `${Math.max(0, Math.min(255, r))} ${Math.max(0, Math.min(255, g))} ${Math.max(0, Math.min(255, b))}`
  }

  function cssColorToTriplet(value: string) {
    const trimmed = value.trim()
    if (!trimmed) {
      return null
    }

    const directMatch = trimmed.match(/^(\d{1,3})\s+(\d{1,3})\s+(\d{1,3})$/)
    if (directMatch) {
      return normalizeTriplet(
        Number.parseInt(directMatch[1] ?? '0', 10),
        Number.parseInt(directMatch[2] ?? '0', 10),
        Number.parseInt(directMatch[3] ?? '0', 10)
      )
    }

    const hexTriplet = hexToRgbTriplet(trimmed)
    if (hexTriplet) {
      return hexTriplet
    }

    const rgbMatch = trimmed.match(/^rgba?\((.*)\)$/i)
    if (!rgbMatch) {
      return null
    }

    const body = (rgbMatch[1] ?? '').trim()
    if (!body) {
      return null
    }

    let rgbTokens: string[] = []
    if (body.includes('/')) {
      const [rawRgb = ''] = body.split('/').map(token => token.trim())
      rgbTokens = rawRgb.includes(',')
        ? rawRgb.split(',').map(token => token.trim()).filter(Boolean)
        : rawRgb.split(/\s+/).map(token => token.trim()).filter(Boolean)
    } else {
      rgbTokens = body.includes(',')
        ? body.split(',').map(token => token.trim()).filter(Boolean)
        : body.split(/\s+/).map(token => token.trim()).filter(Boolean)
    }

    if (rgbTokens.length < 3) {
      return null
    }

    const r = parseRgbChannel(rgbTokens[0] ?? '')
    const g = parseRgbChannel(rgbTokens[1] ?? '')
    const b = parseRgbChannel(rgbTokens[2] ?? '')
    if (r === null || g === null || b === null) {
      return null
    }

    return normalizeTriplet(r, g, b)
  }

  function tripletModelValue(key: string) {
    const rawValue = variableValue(key).trim()
    if (!rawValue) {
      return ''
    }

    if (/^rgba?\(/i.test(rawValue) || rawValue.startsWith('#')) {
      return rawValue
    }

    if (rawValue.includes('var(')) {
      return rawValue.startsWith('rgb(') ? rawValue : `rgb(${rawValue})`
    }

    const triplet = cssColorToTriplet(rawValue)
    if (!triplet) {
      return rawValue
    }

    return `rgb(${triplet})`
  }

  function onTripletColorValueChange(key: string, value: string | number | undefined) {
    const triplet = cssColorToTriplet(String(value ?? ''))
    if (!triplet) {
      return
    }

    setThemeValue(selectedTheme.value, key, triplet)
    applyThemeIfNeeded()
  }

  function onCssValueChange(key: string, value: string | number | undefined) {
    setThemeValue(selectedTheme.value, key, String(value ?? ''))
    applyThemeIfNeeded()

    if (key.startsWith('admin-header-')) {
      syncHeaderControlState()
    } else if (key.startsWith('admin-page-header-')) {
      syncPageHeaderControlState()
    }
  }

  function toNumberValue(value: string | number | undefined, fallback = 0) {
    if (typeof value === 'number') {
      return Number.isFinite(value) ? value : fallback
    }

    const parsed = Number.parseFloat(String(value ?? '').trim())
    return Number.isFinite(parsed) ? parsed : fallback
  }

  function parsePx(value: string) {
    return Math.max(0, Math.round(toNumberValue(value.replace('px', '').trim(), 0)))
  }

  function radiusNumber(key: string) {
    return parsePx(variableValue(key))
  }

  function onRadiusChange(key: string, value: string | number | undefined) {
    const numericValue = Math.max(0, Math.round(toNumberValue(value, radiusNumber(key))))
    setThemeValue(selectedTheme.value, key, `${numericValue}px`)
    applyThemeIfNeeded()
  }

  function onFontChange(value: string | number | undefined) {
    setThemeValue(selectedTheme.value, 'font-sans', String(value ?? ''))
    applyThemeIfNeeded()
  }

  function syncHeaderControlState() {
    const brand = variableValue('admin-header-brand-bg')
    const panel = variableValue('admin-header-panel-bg')
    const brandBlur = variableValue('admin-header-brand-blur')
    const panelBlur = variableValue('admin-header-panel-blur')
    const border = variableValue('admin-header-border')
    const separator = variableValue('admin-header-separator')
    const hover = variableValue('admin-header-hover-bg')
    const active = variableValue('admin-header-active-bg')
    const text = variableValue('admin-header-text')

    headerState.linkBackgrounds = brand === panel
    headerState.linkBlurs = brandBlur === panelBlur
    headerState.linkDividers = border === separator
    headerState.linkInteractions = hover === active
    headerState.textFollowTheme = text.trim() === HEADER_TEXT_LINK_VALUE
    headerState.actionFollowPrimary = hover.trim() === HEADER_PRIMARY_ACTION_VALUE && active.trim() === HEADER_PRIMARY_ACTION_VALUE
  }

  function normalizeDisplayToken(value: string) {
    const normalized = String(value ?? '').trim().toLowerCase()
    if (!normalized) return 'block'
    if (normalized === 'none') return 'none'
    return 'block'
  }

  function syncPageHeaderControlState() {
    const eyebrow = normalizeDisplayToken(variableValue('admin-page-header-eyebrow-display'))
    const title = normalizeDisplayToken(variableValue('admin-page-header-title-display'))
    const description = normalizeDisplayToken(variableValue('admin-page-header-description-display'))

    pageHeaderState.showEyebrow = eyebrow !== 'none'
    pageHeaderState.showTitle = title !== 'none'
    pageHeaderState.showDescription = description !== 'none'
  }

  function setPageHeaderVisibility(key: 'admin-page-header-eyebrow-display' | 'admin-page-header-title-display' | 'admin-page-header-description-display', visible: boolean) {
    setThemeValue(selectedTheme.value, key, visible ? 'block' : 'none')
    applyThemeIfNeeded()
    syncPageHeaderControlState()
  }

  function onPageHeaderEyebrowVisibilityChange(value: boolean) {
    setPageHeaderVisibility('admin-page-header-eyebrow-display', value)
  }

  function onPageHeaderTitleVisibilityChange(value: boolean) {
    setPageHeaderVisibility('admin-page-header-title-display', value)
  }

  function onPageHeaderDescriptionVisibilityChange(value: boolean) {
    setPageHeaderVisibility('admin-page-header-description-display', value)
  }

  function onPageHeaderDisableAll() {
    setThemeValue(selectedTheme.value, 'admin-page-header-eyebrow-display', 'none')
    setThemeValue(selectedTheme.value, 'admin-page-header-title-display', 'none')
    setThemeValue(selectedTheme.value, 'admin-page-header-description-display', 'none')
    applyThemeIfNeeded()
    syncPageHeaderControlState()
  }

  function normalizePxValue(value: string | number | undefined, fallback = 0, max = 200) {
    const numericValue = Math.max(0, Math.min(max, Math.round(toNumberValue(value, fallback))))
    return `${numericValue}px`
  }

  function onHeaderBackgroundChange(value: string | number | undefined) {
    const nextValue = String(value ?? '')
    setThemeValue(selectedTheme.value, 'admin-header-brand-bg', nextValue)
    if (headerState.linkBackgrounds) {
      setThemeValue(selectedTheme.value, 'admin-header-panel-bg', nextValue)
    }
    applyThemeIfNeeded()
  }

  function onHeaderPanelBackgroundChange(value: string | number | undefined) {
    setThemeValue(selectedTheme.value, 'admin-header-panel-bg', String(value ?? ''))
    applyThemeIfNeeded()
  }

  function onHeaderBackgroundLinkChange(value: boolean) {
    headerState.linkBackgrounds = value
    if (headerState.linkBackgrounds) {
      const brandValue = variableValue('admin-header-brand-bg')
      setThemeValue(selectedTheme.value, 'admin-header-panel-bg', brandValue)
      applyThemeIfNeeded()
    }
  }

  function onHeaderBrandBlurChange(value: string | number | undefined) {
    const fallback = parsePx(variableValue('admin-header-brand-blur'))
    const nextValue = normalizePxValue(value, fallback, 120)
    setThemeValue(selectedTheme.value, 'admin-header-brand-blur', nextValue)
    if (headerState.linkBlurs) {
      setThemeValue(selectedTheme.value, 'admin-header-panel-blur', nextValue)
    }
    applyThemeIfNeeded()
  }

  function onHeaderPanelBlurChange(value: string | number | undefined) {
    const fallback = parsePx(variableValue('admin-header-panel-blur'))
    const nextValue = normalizePxValue(value, fallback, 120)
    setThemeValue(selectedTheme.value, 'admin-header-panel-blur', nextValue)
    applyThemeIfNeeded()
  }

  function onHeaderBlurLinkChange(value: boolean) {
    headerState.linkBlurs = value
    if (headerState.linkBlurs) {
      const brandBlurValue = variableValue('admin-header-brand-blur')
      setThemeValue(selectedTheme.value, 'admin-header-panel-blur', brandBlurValue)
      applyThemeIfNeeded()
    }
  }

  function onHeaderDividerChange(value: string | number | undefined) {
    const nextValue = String(value ?? '')
    setThemeValue(selectedTheme.value, 'admin-header-border', nextValue)
    if (headerState.linkDividers) {
      setThemeValue(selectedTheme.value, 'admin-header-separator', nextValue)
    }
    applyThemeIfNeeded()
  }

  function onHeaderSeparatorChange(value: string | number | undefined) {
    setThemeValue(selectedTheme.value, 'admin-header-separator', String(value ?? ''))
    applyThemeIfNeeded()
  }

  function onHeaderDividerLinkChange(value: boolean) {
    headerState.linkDividers = value
    if (headerState.linkDividers) {
      const borderValue = variableValue('admin-header-border')
      setThemeValue(selectedTheme.value, 'admin-header-separator', borderValue)
      applyThemeIfNeeded()
    }
  }

  function onHeaderTextFollowChange(value: boolean) {
    headerState.textFollowTheme = value
    if (headerState.textFollowTheme) {
      setThemeValue(selectedTheme.value, 'admin-header-text', HEADER_TEXT_LINK_VALUE)
      applyThemeIfNeeded()
    }
  }

  function onHeaderTextChange(value: string | number | undefined) {
    setThemeValue(selectedTheme.value, 'admin-header-text', String(value ?? ''))
    applyThemeIfNeeded()
  }

  function onHeaderActionPrimaryChange(value: boolean) {
    headerState.actionFollowPrimary = value
    if (headerState.actionFollowPrimary) {
      setThemeValue(selectedTheme.value, 'admin-header-hover-bg', HEADER_PRIMARY_ACTION_VALUE)
      setThemeValue(selectedTheme.value, 'admin-header-active-bg', HEADER_PRIMARY_ACTION_VALUE)
      applyThemeIfNeeded()
    }
  }

  function onHeaderInteractionChange(value: string | number | undefined) {
    const nextValue = String(value ?? '')
    setThemeValue(selectedTheme.value, 'admin-header-hover-bg', nextValue)
    if (headerState.linkInteractions) {
      setThemeValue(selectedTheme.value, 'admin-header-active-bg', nextValue)
    }
    applyThemeIfNeeded()
  }

  function onHeaderActiveChange(value: string | number | undefined) {
    setThemeValue(selectedTheme.value, 'admin-header-active-bg', String(value ?? ''))
    applyThemeIfNeeded()
  }

  function onHeaderInteractionLinkChange(value: boolean) {
    headerState.linkInteractions = value
    if (headerState.linkInteractions) {
      const hoverValue = variableValue('admin-header-hover-bg')
      setThemeValue(selectedTheme.value, 'admin-header-active-bg', hoverValue)
      applyThemeIfNeeded()
    }
  }

  function onHeaderShellShadowChange(value: string | number | undefined) {
    setThemeValue(selectedTheme.value, 'admin-header-shell-shadow', String(value ?? ''))
    applyThemeIfNeeded()
  }

  function onHeaderFadeTopChange(value: string | number | undefined) {
    setThemeValue(selectedTheme.value, 'admin-header-fade-top', String(value ?? ''))
    applyThemeIfNeeded()
  }

  function onHeaderFadeBottomChange(value: string | number | undefined) {
    setThemeValue(selectedTheme.value, 'admin-header-fade-bottom', String(value ?? ''))
    applyThemeIfNeeded()
  }

  function onHeaderFadeTopSizeChange(value: string | number | undefined) {
    const fallback = parsePx(variableValue('admin-header-fade-top-size'))
    setThemeValue(selectedTheme.value, 'admin-header-fade-top-size', normalizePxValue(value, fallback, 80))
    applyThemeIfNeeded()
  }

  function onHeaderFadeBottomSizeChange(value: string | number | undefined) {
    const fallback = parsePx(variableValue('admin-header-fade-bottom-size'))
    setThemeValue(selectedTheme.value, 'admin-header-fade-bottom-size', normalizePxValue(value, fallback, 80))
    applyThemeIfNeeded()
  }

  return {
    search,
    selectedTheme,
    showDetailedEditors,
    customThemeNameDraft,
    hasCustomTheme,
    fontItems,
    isSimpleMode,
    themeOrder: THEME_ORDER,
    themeLabels,
    groupedSections,
    variableValueMap,
    radiusValueMap,
    tripletModelValueMap,
    headerState,
    pageHeaderState,
    radiusKeys: RADIUS_KEYS,
    shadowColorKeys: SHADOW_COLOR_KEYS,
    headerTextLinkValue: HEADER_TEXT_LINK_VALUE,
    toGroupLabel,
    selectTheme,
    applySelectedTheme,
    resetSelectedThemeOverrides,
    createCustomFromSelected,
    saveCustomThemeName,
    setCustomThemeNameDraft,
    setSearch,
    setShowDetailedEditors,
    onVariableTextValue,
    onTripletColorValueChange,
    onCssValueChange,
    onRadiusChange,
    onFontChange,
    onHeaderBackgroundChange,
    onHeaderPanelBackgroundChange,
    onHeaderBackgroundLinkChange,
    onHeaderBrandBlurChange,
    onHeaderPanelBlurChange,
    onHeaderBlurLinkChange,
    onHeaderDividerChange,
    onHeaderSeparatorChange,
    onHeaderDividerLinkChange,
    onHeaderTextFollowChange,
    onHeaderTextChange,
    onHeaderActionPrimaryChange,
    onHeaderInteractionChange,
    onHeaderActiveChange,
    onHeaderInteractionLinkChange,
    onHeaderShellShadowChange,
    onHeaderFadeTopChange,
    onHeaderFadeBottomChange,
    onHeaderFadeTopSizeChange,
    onHeaderFadeBottomSizeChange,
    onPageHeaderEyebrowVisibilityChange,
    onPageHeaderTitleVisibilityChange,
    onPageHeaderDescriptionVisibilityChange,
    onPageHeaderDisableAll
  }
}
