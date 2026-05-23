import { ref } from 'vue'

export type DrawerMode = 'center' | 'fullscreen' | 'side'
export type DrawerTab = 'overview' | 'history' | 'simulator'

const isOpen = ref(false)
const currentConsultantId = ref<string | null>(null)
const mode = ref<DrawerMode>('center')
const initialTab = ref<DrawerTab>('overview')

export function useConsultantDetailsDrawer() {
  function open(consultantId: string, options: { initialTab?: DrawerTab } = {}) {
    currentConsultantId.value = consultantId
    initialTab.value = options.initialTab || 'overview'
    isOpen.value = true
  }

  function close() {
    isOpen.value = false
  }

  function setMode(nextMode: DrawerMode) {
    mode.value = nextMode
  }

  function toggleFullscreen() {
    mode.value = mode.value === 'fullscreen' ? 'center' : 'fullscreen'
  }

  return {
    isOpen,
    currentConsultantId,
    mode,
    initialTab,
    open,
    close,
    setMode,
    toggleFullscreen,
  }
}
