import { useOmniTheme } from '../composables/useOmniTheme'

export default defineNuxtPlugin(() => {
  const { initializeFromStorage } = useOmniTheme()
  initializeFromStorage()
})
