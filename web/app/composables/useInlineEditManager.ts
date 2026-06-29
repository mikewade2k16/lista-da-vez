import type { Ref } from 'vue'

// Mecanica COMPARTILHADA de edicao inline com salvamento otimista, reusada pelos
// managers de grade (useAdminUsersManager/useProductsManager/useLeadsManager/
// useAdminOrganizationsManager/useClientsManager/useAccountRolesManager).
//
// Extrai SO o que era identico e repetido em cada manager:
//   - savingMap (estado de "salvando" granular por chave `${id}:${field}`)
//   - setSaving / rowIsSaving
//   - pendingTimers + debounce do updateField (mesmo PATCH_DELAY_MS por manager)
//   - cleanup dos timers no onBeforeUnmount
//
// O que continua ESPECIFICO de cada manager (e por isso NAO entra aqui): a lista
// reativa, o normalize, e o persistPatch (URL/headers/endpoint diferentes). Cada
// manager segue dono do seu applyPatch/patchLocal/persistPatch e apenas delega a
// agenda de debounce + o savingMap para este composable. Assim o retorno publico,
// a reatividade e o comportamento observavel de cada manager ficam IDENTICOS.

// Atraso padrao do debounce de campos de texto. Identico em todos os managers que
// usam (380ms); exposto como default para o caller poder sobrescrever se precisar.
export const INLINE_PATCH_DELAY_MS = 380

export interface InlineEditManager {
  // Mapa reativo de salvamentos em voo, chaveado por `${id}:${field}`. Mantido como
  // ref<Record<string, boolean>> para preservar EXATAMENTE o shape que os managers
  // ja expoem no return (a UI le `savingMap.value[...]`).
  savingMap: Ref<Record<string, boolean>>
  setSaving: (key: string, value: boolean) => void
  // True se QUALQUER campo da linha `id` esta salvando (chave começa com `${id}:`).
  rowIsSaving: (id: string) => boolean
  // Agenda a persistencia de um campo: debounce por `timerKey` (cancela o anterior)
  // ou dispara na hora quando immediate. Substitui o bloco de pendingTimers que cada
  // updateField repetia, com a MESMA semantica (set->clearTimeout->setTimeout/delete).
  schedulePatch: (timerKey: string, run: () => void, opts?: { immediate?: boolean }) => void
  // Cancela um debounce pendente (sem disparar). Util para casos especificos.
  cancelPatch: (timerKey: string) => void
}

export function useInlineEditManager(options?: { delayMs?: number }): InlineEditManager {
  const delayMs = options?.delayMs ?? INLINE_PATCH_DELAY_MS

  const savingMap = ref<Record<string, boolean>>({})
  // Mesmo padrao imutavel dos managers: cria um novo objeto a cada mudanca para
  // garantir a reatividade do computed/template que le savingMap.value.
  function setSaving(key: string, value: boolean) {
    const next = { ...savingMap.value }
    if (value) next[key] = true
    else delete next[key]
    savingMap.value = next
  }

  function rowIsSaving(id: string) {
    return Object.keys(savingMap.value).some((k) => k.startsWith(`${id}:`))
  }

  const pendingTimers = new Map<string, ReturnType<typeof setTimeout>>()

  function cancelPatch(timerKey: string) {
    const timer = pendingTimers.get(timerKey)
    if (timer) {
      clearTimeout(timer)
      pendingTimers.delete(timerKey)
    }
  }

  function schedulePatch(timerKey: string, run: () => void, opts?: { immediate?: boolean }) {
    // Cancela o debounce anterior do mesmo campo (mesma semantica do
    // `if (pendingTimers.has(timerKey)) clearTimeout(...)` original).
    cancelPatch(timerKey)

    if (opts?.immediate) {
      run()
      return
    }

    pendingTimers.set(
      timerKey,
      setTimeout(() => {
        pendingTimers.delete(timerKey)
        run()
      }, delayMs),
    )
  }

  onBeforeUnmount(() => {
    for (const timer of pendingTimers.values()) clearTimeout(timer)
    pendingTimers.clear()
  })

  return { savingMap, setSaving, rowIsSaving, schedulePatch, cancelPatch }
}
