import { createApiRequest } from '~/utils/api-client'
import {
  ALL_THEMES,
  OMNI_THEME_DEFAULTS,
  OMNI_THEME_LABELS,
  OMNI_THEME_VARIABLES,
  OMNI_THEMES,
  createEmptyOverrides,
  getThemeDescriptor,
  hexToRgbTriplet,
  isBaseThemeName,
  isThemeName,
  normalizeVariableKey,
  rgbTripletToHex,
  sanitizeOverrides,
  selectorByTheme,
  type OmniThemeName,
  type OmniThemeOverrides,
  type OmniThemeVariable,
  type OmniThemeVariableGroup,
  type OmniThemeVariableKind,
  type ThemeVars,
} from '../domain/theme/omni-theme-catalog'

// Re-export para os consumidores que importam estes símbolos via useOmniTheme
// (o catálogo passou a ser a fonte, mas mantemos o ponto de import estável).
export {
  OMNI_THEME_DEFAULTS,
  OMNI_THEME_LABELS,
  OMNI_THEME_VARIABLES,
  hexToRgbTriplet,
  rgbTripletToHex,
}
export type { OmniThemeName, OmniThemeVariable, OmniThemeVariableGroup, OmniThemeVariableKind }

const THEME_STORAGE_KEY = 'omni-ui-theme'
const USER_THEME_STORAGE_KEY = 'omni-ui-user-theme'
const OVERRIDES_STORAGE_KEY = 'omni-ui-theme-overrides'
const CUSTOM_THEME_NAME_KEY = 'omni-ui-theme-custom-name'
const STYLE_TAG_ID = 'omni-theme-overrides-style'

/**
 * Gerencia o tema visual do produto Omni: seleção persistida e overrides por
 * variável CSS.
 *
 * A APARÊNCIA é GLOBAL da plataforma: lida por qualquer usuário autenticado e
 * escrita apenas por platform_admin, via o módulo `theme` do back
 * (`GET/PUT /v1/platform/appearance`, guardado em core.platform_settings).
 * Desacoplado do módulo `queue` — antes o appearance vivia no queue/settings e
 * dependia de tenant não-vazio, então NÃO persistia para platform_admin (tenant
 * vazio). A preferência pessoal light/dark do usuário continua local
 * (localStorage), sobrepondo os temas-base.
 *
 * @see OMNI_THEME_DEFAULTS
 * @see OMNI_THEME_VARIABLES
 */
export function useOmniTheme() {
  const runtimeConfig = useRuntimeConfig()
  const colorMode = useColorMode()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)
  const initialized = useState<boolean>('omni-theme-initialized', () => false)
  const currentTheme = useState<OmniThemeName>('omni-theme-current', () => 'dark')
  const overrides = useState<OmniThemeOverrides>('omni-theme-overrides', () =>
    createEmptyOverrides(),
  )
  const customThemeName = useState<string>('omni-theme-custom-name', () => OMNI_THEME_LABELS.custom)

  // Ler a aparência global vale para qualquer autenticado; ESCREVER (persistir)
  // é só platform_admin — o back gateia o PUT, e evita 403 para usuário comum.
  const isAuthenticated = computed(() => auth.isAuthenticated && Boolean(auth.accessToken))
  const canManageAppearance = computed(
    () => isAuthenticated.value && auth.role === 'platform_admin',
  )
  const canAccessThemeStudio = computed(
    () =>
      import.meta.dev ||
      runtimeConfig.public.themeStudioEnabled === true ||
      auth.allowedWorkspaces.includes('themes'),
  )
  const advancedThemesEnabled = computed(() => canAccessThemeStudio.value || isAuthenticated.value)

  const hasCustomTheme = computed(() => Object.keys(overrides.value.custom).length > 0)
  let persistTimer: ReturnType<typeof window.setTimeout> | null = null

  function normalizeThemeName(name: string) {
    const normalized = name.trim()
    return normalized || OMNI_THEME_LABELS.custom
  }

  function readStoredUserThemePreference(): 'light' | 'dark' | null {
    if (!import.meta.client) {
      return null
    }

    const storedThemePreference = localStorage.getItem(USER_THEME_STORAGE_KEY)
    return isBaseThemeName(storedThemePreference) ? storedThemePreference : null
  }

  function resolvePreferredTheme(theme: OmniThemeName): OmniThemeName {
    if (!isBaseThemeName(theme)) {
      return theme
    }

    return readStoredUserThemePreference() ?? theme
  }

  function cloneOverridesSnapshot(
    source: OmniThemeOverrides = overrides.value,
  ): OmniThemeOverrides {
    return ALL_THEMES.reduce((nextValue, theme) => {
      nextValue[theme] = { ...(source?.[theme] || {}) }
      return nextValue
    }, createEmptyOverrides())
  }

  function normalizeAppearanceSnapshot(appearance: unknown) {
    const source = (appearance ?? {}) as {
      activeTheme?: unknown
      customThemeName?: unknown
      overrides?: unknown
    }
    const activeTheme = String(source.activeTheme || '').trim()

    return {
      activeTheme: (isThemeName(activeTheme) ? activeTheme : 'dark') as OmniThemeName,
      customThemeName: normalizeThemeName(String(source.customThemeName || '')),
      overrides: sanitizeOverrides(source.overrides),
    }
  }

  function buildAppearanceSnapshot() {
    return {
      activeTheme: currentTheme.value,
      customThemeName: normalizeThemeName(customThemeName.value),
      overrides: cloneOverridesSnapshot(),
    }
  }

  function persistOverrides() {
    if (!import.meta.client || !advancedThemesEnabled.value || canManageAppearance.value) {
      return
    }

    localStorage.setItem(OVERRIDES_STORAGE_KEY, JSON.stringify(overrides.value))
  }

  function getFallbackBaseTheme(): OmniThemeName {
    return readStoredUserThemePreference() ?? 'dark'
  }

  function clearAdvancedThemeStorage() {
    if (!import.meta.client) {
      return
    }

    localStorage.removeItem(OVERRIDES_STORAGE_KEY)
    localStorage.removeItem(CUSTOM_THEME_NAME_KEY)
  }

  function clearLocalThemeStorage() {
    if (!import.meta.client) {
      return
    }

    localStorage.removeItem(THEME_STORAGE_KEY)
    clearAdvancedThemeStorage()
  }

  function applyOverrides() {
    if (!import.meta.client) {
      return
    }

    const cssBlocks: string[] = []

    for (const theme of ALL_THEMES) {
      const entries = Object.entries(overrides.value[theme] || {}).filter(
        ([, value]) => value && value.trim().length > 0,
      )

      if (!entries.length) {
        continue
      }

      const selector = selectorByTheme(theme)
      const declarations = entries
        .map(([key, value]) => `  --${normalizeVariableKey(key)}: ${value};`)
        .join('\n')
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

  function applyRemoteAppearance(appearance: unknown, options: { markInitialized?: boolean } = {}) {
    const normalizedAppearance = normalizeAppearanceSnapshot(appearance)
    overrides.value = normalizedAppearance.overrides
    customThemeName.value = normalizedAppearance.customThemeName
    applyOverrides()
    applyTheme(resolvePreferredTheme(normalizedAppearance.activeTheme), false)

    // Só o admin escreve o global; ao carregar, limpamos o local dele para o
    // remoto não ser sombreado. Usuário comum mantém a preferência local.
    if (canManageAppearance.value) {
      clearLocalThemeStorage()
    }

    if (options.markInitialized !== false) {
      initialized.value = true
    }
  }

  async function loadRemoteAppearance(): Promise<void> {
    if (!import.meta.client || !isAuthenticated.value) {
      return
    }

    try {
      const response = (await apiRequest('/v1/platform/appearance', { method: 'GET' })) as {
        appearance?: unknown
      }
      applyRemoteAppearance(response?.appearance ?? {}, { markInitialized: true })
    } catch (error) {
      console.error('[omni-theme] failed to load platform appearance', error)
    }
  }

  async function persistRemoteAppearance(snapshot = buildAppearanceSnapshot()) {
    if (!import.meta.client || !canManageAppearance.value) {
      return
    }

    await apiRequest('/v1/platform/appearance', {
      method: 'PUT',
      body: {
        appearance: snapshot,
      },
    })
  }

  function scheduleRemoteAppearancePersist() {
    if (!import.meta.client || !canManageAppearance.value) {
      return
    }

    if (persistTimer) {
      window.clearTimeout(persistTimer)
      persistTimer = null
    }

    const snapshot = buildAppearanceSnapshot()
    persistTimer = window.setTimeout(async () => {
      persistTimer = null

      try {
        await persistRemoteAppearance(snapshot)
      } catch (error) {
        console.error('[omni-theme] failed to persist remote appearance', error)
      }
    }, 250)
  }

  // Aplica o tema resolvido: classes do html vêm do CATÁLOGO (base light/dark +
  // selectorClass do tema), sem switch hardcoded — adicionar tema não mexe aqui.
  function applyTheme(theme: OmniThemeName, persist = true) {
    const resolvedTheme =
      advancedThemesEnabled.value || theme === 'light' || theme === 'dark'
        ? theme
        : getFallbackBaseTheme()

    currentTheme.value = resolvedTheme

    if (!import.meta.client) {
      return
    }

    const root = document.documentElement
    for (const descriptor of OMNI_THEMES) {
      if (descriptor.selectorClass.startsWith('.') && descriptor.selectorClass !== '.dark') {
        root.classList.remove(descriptor.selectorClass.slice(1))
      }
    }
    root.classList.remove('dark')

    const descriptor = getThemeDescriptor(resolvedTheme)
    if ((descriptor?.base ?? 'light') === 'dark') {
      root.classList.add('dark')
      colorMode.preference = 'dark'
    } else {
      colorMode.preference = 'light'
    }

    if (
      descriptor &&
      descriptor.selectorClass.startsWith('.') &&
      descriptor.selectorClass !== '.dark'
    ) {
      root.classList.add(descriptor.selectorClass.slice(1))
    }

    if (persist) {
      if (canManageAppearance.value) {
        scheduleRemoteAppearancePersist()
      } else {
        localStorage.setItem(THEME_STORAGE_KEY, resolvedTheme)
      }
    }
  }

  function initializeFromStorage() {
    if (!import.meta.client || initialized.value) {
      return
    }

    if (advancedThemesEnabled.value) {
      const rawOverrides = localStorage.getItem(OVERRIDES_STORAGE_KEY)
      if (rawOverrides) {
        try {
          overrides.value = sanitizeOverrides(JSON.parse(rawOverrides))
        } catch {
          overrides.value = createEmptyOverrides()
        }
      }
    } else {
      overrides.value = createEmptyOverrides()
      clearAdvancedThemeStorage()
    }

    applyOverrides()

    const storedTheme = localStorage.getItem(THEME_STORAGE_KEY)
    const storedUserThemePreference = readStoredUserThemePreference()
    const storedCustomThemeName = localStorage.getItem(CUSTOM_THEME_NAME_KEY)
    if (advancedThemesEnabled.value && storedCustomThemeName) {
      customThemeName.value = normalizeThemeName(storedCustomThemeName)
    } else {
      customThemeName.value = OMNI_THEME_LABELS.custom
    }

    const fallbackTheme = getFallbackBaseTheme()
    const resolvedStoredTheme: OmniThemeName | null =
      storedTheme === 'light' ||
      storedTheme === 'dark' ||
      (advancedThemesEnabled.value && isThemeName(storedTheme))
        ? (storedTheme as OmniThemeName)
        : null

    const resolvedTheme =
      resolvedStoredTheme === 'light' || resolvedStoredTheme === 'dark'
        ? (storedUserThemePreference ?? resolvedStoredTheme)
        : (resolvedStoredTheme ?? fallbackTheme)

    applyTheme(resolvedTheme, resolvedTheme !== storedTheme)

    initialized.value = true

    // A aparência GLOBAL da plataforma sobrepõe o local assim que chega —
    // inclusive para platform_admin (que antes não persistia por tenant vazio).
    // O watch de isAuthenticated também dispara isto no login.
    if (isAuthenticated.value) {
      void loadRemoteAppearance()
    }
  }

  function getThemeDefaults(theme: OmniThemeName): ThemeVars {
    return { ...OMNI_THEME_DEFAULTS[theme] }
  }

  function getResolvedThemeValues(theme: OmniThemeName) {
    return {
      ...getThemeDefaults(theme),
      ...(overrides.value[theme] || {}),
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
    if (!advancedThemesEnabled.value) {
      return
    }

    const normalizedKey = normalizeVariableKey(key)
    if (!normalizedKey) {
      return
    }

    overrides.value = {
      ...overrides.value,
      [theme]: {
        ...overrides.value[theme],
        [normalizedKey]: value,
      },
    }

    applyOverrides()
    if (canManageAppearance.value) {
      scheduleRemoteAppearancePersist()
    } else {
      persistOverrides()
    }
  }

  function setThemeValues(theme: OmniThemeName, values: ThemeVars) {
    if (!advancedThemesEnabled.value) {
      return
    }

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
      [theme]: nextValues,
    }

    applyOverrides()
    if (canManageAppearance.value) {
      scheduleRemoteAppearancePersist()
    } else {
      persistOverrides()
    }
  }

  function resetThemeOverrides(theme: OmniThemeName) {
    if (!advancedThemesEnabled.value) {
      return
    }

    overrides.value = {
      ...overrides.value,
      [theme]: {},
    }

    applyOverrides()
    if (canManageAppearance.value) {
      scheduleRemoteAppearancePersist()
    } else {
      persistOverrides()
    }
  }

  function duplicateTheme(source: OmniThemeName, target: OmniThemeName = 'custom') {
    if (!advancedThemesEnabled.value) {
      return
    }

    setThemeValues(target, getResolvedThemeValues(source))
  }

  function setCustomThemeName(name: string, persist = true) {
    if (!advancedThemesEnabled.value) {
      return
    }

    const normalized = normalizeThemeName(name)
    customThemeName.value = normalized

    if (!import.meta.client || !persist) {
      return
    }

    if (canManageAppearance.value) {
      scheduleRemoteAppearancePersist()
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

  // Ao autenticar (ou já autenticado no boot), carrega a aparência GLOBAL e
  // sobrepõe o estado local.
  watch(
    () => auth.isAuthenticated,
    (authenticated) => {
      if (!import.meta.client || !authenticated) {
        return
      }

      void loadRemoteAppearance()
    },
    { immediate: true },
  )

  return {
    currentTheme,
    overrides,
    customThemeName,
    hasCustomTheme,
    advancedThemesEnabled,
    canManageAppearance,
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
    duplicateTheme,
  }
}
