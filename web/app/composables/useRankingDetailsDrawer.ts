import { ref } from 'vue'

export type RankingDrawerMode = 'center' | 'fullscreen' | 'side'

const isOpen = ref(false)
const currentRowKey = ref<string | null>(null)
const mode = ref<RankingDrawerMode>('center')

export function useRankingDetailsDrawer() {
  function open(rowKey: string) {
    currentRowKey.value = rowKey
    isOpen.value = true
  }

  function close() {
    isOpen.value = false
  }

  function setMode(nextMode: RankingDrawerMode) {
    mode.value = nextMode
  }

  function toggleFullscreen() {
    mode.value = mode.value === 'fullscreen' ? 'center' : 'fullscreen'
  }

  return {
    isOpen,
    currentRowKey,
    mode,
    open,
    close,
    setMode,
    toggleFullscreen,
  }
}
