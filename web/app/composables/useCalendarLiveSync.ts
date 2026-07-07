import { computed, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useAuthStore } from '~/stores/auth'
import { useCalendarStore } from '~/stores/calendar'
import { useUiStore } from '~/stores/ui'
import { useCalendarPresence } from '~/composables/useCalendarPresence'
import { useCalendarRealtime } from '~/composables/useCalendarRealtime'
import type { CalendarEvent } from '~/utils/calendar'

// Sincronizacao ao vivo do calendario (SPEC-F9): junta realtime (invalidacao), presenca
// (indicador de quem edita) e o tratamento do conflito 409 (C12) num so lugar, para a
// pagina /calendario ficar < 450 linhas. Recebe os refs de estado do form da pagina
// (formOpen/editingEvent/formDate) porque o conflito re-hidrata o form e a presenca do
// evento depende do evento em edicao.

export interface CalendarLiveSyncRefs {
  formOpen: Ref<boolean>
  editingEvent: Ref<CalendarEvent | null>
  formDate: Ref<string>
}

export function useCalendarLiveSync(refs: CalendarLiveSyncRefs) {
  const store = useCalendarStore()
  const ui = useUiStore()
  const auth = useAuthStore()
  const { activeNotesMonthKey } = storeToRefs(store)
  const { formOpen, editingEvent, formDate } = refs

  // A conta e' resolvida DENTRO dos composables (cadeia resolveRealtimeAccountId, nunca so
  // auth.activeTenantId); a troca de conta reconecta sozinha (a base cuida).
  const realtimeEnabled = computed(() => auth.isAuthenticated)

  // Ultimo plano de IA que mudou de status via realtime; repassado ao CalendarAiPlanModal
  // para encerrar o polling antes do proximo tick (o modal tem sua propria instancia).
  const lastPlanEvent = ref<{ id: string; status: string; at: number } | null>(null)

  useCalendarRealtime({
    enabled: realtimeEnabled,
    onWindowInvalidate: () => void store.refetchWindow(),
    onNoteUpdated: (monthKey) => void store.reloadNoteFromRemote(monthKey),
    onConfigUpdated: () => void store.fetchConfig(),
    onPlanUpdated: (id, status) => {
      lastPlanEvent.value = { id, status, at: Date.now() }
    },
  })

  // Refetch de SEGURANCA ao voltar pra aba/pagina do calendario. O realtime (WS) so entrega
  // enquanto a pagina esta conectada; se o calendario nao estava aberto quando algo mudou
  // (ex.: mudei a data de uma TASK no board e o evento-espelho foi sincronizado no back),
  // voltar ao calendario refaz o fetch e mostra o estado fresco SEM precisar recarregar.
  function refetchOnReturn(): void {
    if (typeof document !== 'undefined' && document.visibilityState !== 'visible') return
    void store.refetchWindow()
  }
  onMounted(() => {
    if (typeof document !== 'undefined') {
      document.addEventListener('visibilitychange', refetchOnReturn)
    }
    if (typeof window !== 'undefined') {
      window.addEventListener('focus', refetchOnReturn)
    }
  })
  onBeforeUnmount(() => {
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', refetchOnReturn)
    }
    if (typeof window !== 'undefined') {
      window.removeEventListener('focus', refetchOnReturn)
    }
  })

  const presence = useCalendarPresence({ enabled: realtimeEnabled })
  const presenceParticipants = presence.participants
  // Badge "Fulano editando" nas notas do mes em foco e no evento em edicao.
  const notesPresenceLabel = computed(() =>
    presence.fieldLabel(`notes:${activeNotesMonthKey.value}`),
  )
  const eventPresenceLabel = computed(() =>
    editingEvent.value ? presence.fieldLabel(`event:${editingEvent.value.id}`) : '',
  )

  function onNotesFocus(): void {
    presence.focusField(`notes:${activeNotesMonthKey.value}`)
  }
  function onNotesBlur(): void {
    presence.blurField(`notes:${activeNotesMonthKey.value}`)
  }

  // Marca presenca no evento aberto para edicao (fieldKey event:<id>); some ao fechar.
  watch(
    () => (formOpen.value && editingEvent.value ? `event:${editingEvent.value.id}` : ''),
    (key, prev) => {
      if (prev && prev !== key) presence.blurField(prev)
      if (key) presence.focusField(key)
    },
  )

  // C12: o evento foi alterado por outra pessoa desde que abrimos o form. Avisa e oferece
  // recarregar (re-hidrata o form com a versao do banco, ja no store apos o refetch do 409).
  // Sem confirmar, o rascunho do usuario fica intacto — nunca descarta em silencio.
  async function handleEventConflict(id: string): Promise<void> {
    const answer = await ui.confirm({
      title: 'Item alterado por outra pessoa',
      message:
        'Este item foi alterado por outra pessoa enquanto você editava. Recarregar traz a versão mais recente e substitui a sua edição atual.',
      confirmLabel: 'Recarregar',
      cancelLabel: 'Manter minha edição',
    })
    if (!answer?.confirmed) return
    const fresh = store.getEventById(id)
    if (fresh) {
      editingEvent.value = fresh
      formDate.value = fresh.date
    } else {
      // Evento removido por outra pessoa: fecha o form.
      formOpen.value = false
      ui.info('O item foi removido por outra pessoa.')
    }
  }

  return {
    presenceParticipants,
    notesPresenceLabel,
    eventPresenceLabel,
    lastPlanEvent,
    onNotesFocus,
    onNotesBlur,
    handleEventConflict,
  }
}
