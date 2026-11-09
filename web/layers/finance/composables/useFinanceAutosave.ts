import { reactive, computed, onScopeDispose } from 'vue'
import { createFinanceSaveQueue } from '../utils/finance-save-queue'

export function useFinanceAutosave(delay: number) {
  const state = reactive({ pending: false, saving: false, failed: false })
  const queue = createFinanceSaveQueue(delay, (next) => Object.assign(state, next))
  const dirty = computed(() => state.pending || state.saving || state.failed)
  const label = computed(() => {
    if (state.failed) return 'Falha ao salvar — tente novamente'
    if (state.saving) return 'Salvando...'
    if (state.pending) return 'Alterações pendentes'
    return 'Salvo automaticamente'
  })
  function beforeUnload(event: BeforeUnloadEvent) {
    if (!dirty.value) return
    event.preventDefault()
    event.returnValue = ''
  }
  if (import.meta.client) window.addEventListener('beforeunload', beforeUnload)
  onBeforeRouteLeave(async () => (await queue.flush()) || false)
  onScopeDispose(() => {
    if (import.meta.client) window.removeEventListener('beforeunload', beforeUnload)
    if (dirty.value) void queue.flush()
  })
  return { ...queue, state, dirty, label }
}
