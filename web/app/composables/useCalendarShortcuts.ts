import { onBeforeUnmount, onMounted } from 'vue'
import { useCalendarStore } from '~/stores/calendar'
import { shortcutComboFromEvent } from '~/domain/calendar/calendar-config'

// Atalhos de teclado do calendario/assistente (WAVE 11). O MAPA (acao -> tecla) e DADO da
// config por conta (config.shortcuts, editavel na aba Aparencia); aqui so a execucao:
// keydown global -> normaliza a tecla -> roda o handler da acao vinculada aquela tecla.
//
// Regras: (1) atalhos NAO disparam enquanto o foco esta num campo editavel (input/textarea/
// select/contenteditable) — exceto bindings com `force: true` (ex.: Esc fecha a IA mesmo
// digitando); (2) com modificador (ctrl/meta/alt) nunca dispara (nao rouba Ctrl+C etc.);
// (3) `when` restringe o contexto (ex.: so com o painel do chat aberto).

export interface CalendarShortcutBinding {
  action: string
  handler: () => void
  /** Roda mesmo com o foco num campo editavel (default false). */
  force?: boolean
  /** Contexto extra (ex.: painel aberto). Ausente = sempre. */
  when?: () => boolean
}

function isEditableTarget(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null
  if (!el) return false
  const tag = (el.tagName || '').toLowerCase()
  return tag === 'input' || tag === 'textarea' || tag === 'select' || el.isContentEditable
}

export function useCalendarShortcuts(bindings: CalendarShortcutBinding[]): void {
  const store = useCalendarStore()

  function onKeydown(event: KeyboardEvent): void {
    if (event.repeat) return
    // Combo COMPLETO (com modificadores): 'shift+t', 'ctrl+shift+k', 't'. Modificador sozinho => ''.
    const key = shortcutComboFromEvent(event)
    if (!key) return
    const map = store.config.shortcuts || {}
    const editable = isEditableTarget(event.target)
    for (const binding of bindings) {
      const bound = String(map[binding.action] || '').toLowerCase()
      if (!bound || bound !== key) continue
      if (editable && !binding.force) continue
      if (binding.when && !binding.when()) continue
      event.preventDefault()
      binding.handler()
      return
    }
  }

  onMounted(() => {
    if (typeof window !== 'undefined') window.addEventListener('keydown', onKeydown)
  })
  onBeforeUnmount(() => {
    if (typeof window !== 'undefined') window.removeEventListener('keydown', onKeydown)
  })
}
