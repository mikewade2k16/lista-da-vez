function normalizeDisplayToken(value: string) {
  const normalized = String(value ?? '')
    .trim()
    .toLowerCase()
    .replace(/\s*!important$/, '')
    .replace(/;$/, '')
  if (!normalized) return 'block'
  if (normalized === 'none' || normalized === 'hidden' || normalized === '0' || normalized === 'false') {
    return 'none'
  }
  return 'block'
}

import { useOmniTheme } from './useOmniTheme'

export function useAdminPageHeaderVisibility() {
  const { currentTheme, getThemeValue, initializeFromStorage } = useOmniTheme()

  onMounted(() => {
    initializeFromStorage()
  })

  const showEyebrow = computed(() => {
    const value = getThemeValue(currentTheme.value, 'admin-page-header-eyebrow-display')
    return normalizeDisplayToken(value) !== 'none'
  })

  const showTitle = computed(() => {
    const value = getThemeValue(currentTheme.value, 'admin-page-header-title-display')
    return normalizeDisplayToken(value) !== 'none'
  })

  const showDescription = computed(() => {
    const value = getThemeValue(currentTheme.value, 'admin-page-header-description-display')
    return normalizeDisplayToken(value) !== 'none'
  })

  const showHeader = computed(() =>
    showEyebrow.value || showTitle.value || showDescription.value
  )

  return {
    showHeader,
    showEyebrow,
    showTitle,
    showDescription
  }
}
