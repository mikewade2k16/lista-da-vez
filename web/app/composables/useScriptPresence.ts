import { computed, type ComputedRef, type Ref } from 'vue'
import { useCalendarPresence } from '~/composables/useCalendarPresence'

// Presença e travas de UM roteiro (canal presence:script:{id}). Reusa a presença genérica
// (useCalendarPresence com scope 'script'). Chaves de campo, iguais às do back
// (tools.ScriptCellKey): "title", "description" e "cell:<rowId>:<audio|video>".
// A trava é exclusiva e expira em 30 s sem heartbeat; o servidor também a confere ao gravar.

type Source<T> = T | Ref<T> | ComputedRef<T> | (() => T)

export const scriptCellKey = (field: string, rowId = '') =>
  field === 'title' || field === 'description' ? field : `cell:${rowId}:${field}`

export function useScriptPresence(options: {
  enabled: Source<boolean>
  accountId: Source<string>
  scriptId: Source<string>
}) {
  const presence = useCalendarPresence({
    enabled: options.enabled,
    accountId: options.accountId,
    scope: 'script',
    resourceId: options.scriptId,
  })
  /** Quem (outra pessoa) está editando cada campo: fieldKey -> nome. */
  const lockers = computed(() => {
    const map: Record<string, string> = {}
    for (const user of presence.participants.value)
      if (user.fieldKey && !map[user.fieldKey]) map[user.fieldKey] = user.displayName
    return map
  })
  const editors = computed(() =>
    presence.participants.value.filter((user) => user.fieldKey).map((user) => user.displayName),
  )
  const rowLocker = (rowId: string) =>
    lockers.value[scriptCellKey('audio', rowId)] ||
    lockers.value[scriptCellKey('video', rowId)] ||
    ''
  return {
    lockers,
    editors,
    viewers: presence.participants,
    rowLocker,
    focus: presence.focusField,
    blur: presence.blurField,
    activeFieldKey: presence.activeFieldKey,
  }
}
