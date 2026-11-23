import { onMounted, ref, watch } from 'vue'

import { useAdminPreferences } from '~/composables/useAdminPreferences'

export type ToolsViewMode = 'cards' | 'table'

// Visualizacao guardada nas mesmas preferencias de tela das colunas.
export function useToolsViewMode(preferenceKey: string, defaultMode: ToolsViewMode = 'cards') {
  const { ensureLoaded, readStringArray, writeStringArray } = useAdminPreferences()
  const path = ['ui', 'view_mode', preferenceKey]
  const mode = ref<ToolsViewMode>(defaultMode)
  const hydrated = ref(false)

  onMounted(async () => {
    try {
      await ensureLoaded()
      const saved = readStringArray(path, [defaultMode])[0]
      mode.value = saved === 'table' || saved === 'cards' ? saved : defaultMode
    } catch {
      mode.value = defaultMode
    }
    hydrated.value = true
  })

  watch(mode, (value) => {
    if (!hydrated.value) return
    try {
      writeStringArray(path, [value])
    } catch {
      // armazenamento indisponivel: a escolha vale so nesta visita
    }
  })

  return mode
}
