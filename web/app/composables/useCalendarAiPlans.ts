import { ref } from 'vue'

import { useAuthStore } from '~/stores/auth'
import * as calendarApi from '~/domain/calendar/calendar-api'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { normalizePlan, type CalendarAiPlan, type CalendarAiPlanIndexItem } from '~/utils/calendar'

const POLL_INTERVAL_MS = 3000
const POLL_MAX_MS = 5 * 60 * 1000 // 5 minutos

/** Resultado do disparo: id do plano ou codigo de erro acionavel. */
export interface StartPlanResult {
  id: string
  errorCode: string
  errorMessage: string
}

// useCalendarAiPlans concentra o I/O do PLANO DE IA (contrato C4, SPEC-F5): index
// lean por mes, criacao + polling do resultado e exclusao. Fica fora do store por
// ser usado so no modal de IA e para manter cada arquivo < 450 linhas. O account_id
// nunca trafega no body: o back resolve pelo Principal (accountScope); aqui so
// month + clientIds viajam.
export function useCalendarAiPlans() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const plans = ref<CalendarAiPlanIndexItem[]>([])
  const loadingPlans = ref(false)
  // Plano em foco (gerado ou aberto da lista) com o content completo.
  const activePlan = ref<CalendarAiPlan | null>(null)
  const starting = ref(false)
  const polling = ref(false)

  let pollTimer = 0
  let pollDeadline = 0

  async function withSession<T>(run: () => Promise<T>, fallback: T): Promise<T> {
    await auth.ensureSession()
    if (!auth.isAuthenticated) return fallback
    try {
      return await run()
    } catch {
      return fallback
    }
  }

  // Lista lean dos planos do mes (sem content). Silencioso em erro.
  async function fetchPlans(month: string): Promise<void> {
    loadingPlans.value = true
    plans.value = await withSession(() => calendarApi.fetchAiPlans(apiRequest, month), plans.value)
    loadingPlans.value = false
  }

  // Abre um plano da lista (carrega o content completo).
  async function openPlan(id: string): Promise<void> {
    const plan = await withSession(() => calendarApi.fetchAiPlan(apiRequest, id), null)
    if (plan) activePlan.value = plan
  }

  // Dispara a geracao (POST /ai/plan). NAO engole o erro: precisamos do codigo
  // (ex.: ai_not_configured => 503) para a mensagem acionavel do modal.
  async function startPlan(month: string, clientIds: string[]): Promise<StartPlanResult> {
    starting.value = true
    activePlan.value = null
    try {
      const { id } = await calendarApi.createAiPlan(apiRequest, month, clientIds)
      // Estado local otimista: mostra "pending" na hora, antes do 1o poll.
      activePlan.value = normalizePlan({ id, month, clientIds, status: 'pending' })
      startPolling(id)
      void fetchPlans(month)
      return { id, errorCode: '', errorMessage: '' }
    } catch (err: unknown) {
      const errObj = err as { data?: { error?: { code?: string } } }
      return {
        id: '',
        errorCode: String(errObj?.data?.error?.code || ''),
        errorMessage: getApiErrorMessage(err, 'Nao foi possivel gerar o plano.'),
      }
    } finally {
      starting.value = false
    }
  }

  function clearPolling(): void {
    if (pollTimer) {
      window.clearTimeout(pollTimer)
      pollTimer = 0
    }
    polling.value = false
  }

  // Faz polling do GET /ai/plans/{id} a cada 3s (ate 5min). Para em done/error/
  // applied, ao estourar o prazo ou quando clearPolling e' chamado (fechar modal).
  function startPolling(id: string): void {
    clearPolling()
    polling.value = true
    pollDeadline = Date.now() + POLL_MAX_MS
    const tick = async (): Promise<void> => {
      const plan = await withSession(() => calendarApi.fetchAiPlan(apiRequest, id), null)
      if (plan) activePlan.value = plan
      const done = plan && plan.status !== 'pending'
      if (done || Date.now() >= pollDeadline || !polling.value) {
        clearPolling()
        return
      }
      pollTimer = window.setTimeout(() => void tick(), POLL_INTERVAL_MS)
    }
    pollTimer = window.setTimeout(() => void tick(), POLL_INTERVAL_MS)
  }

  // Marca o plano como aplicado (done -> applied). Devolve o plano atualizado ou
  // null; atualiza o status no index e no plano ativo sem refetch.
  async function markApplied(id: string): Promise<CalendarAiPlan | null> {
    const plan = await withSession(() => calendarApi.markAiPlanApplied(apiRequest, id), null)
    if (plan) {
      activePlan.value = plan
      plans.value = plans.value.map((p) => (p.id === id ? { ...p, status: plan.status } : p))
    }
    return plan
  }

  async function removePlan(id: string): Promise<boolean> {
    const ok = await withSession(async () => {
      await calendarApi.deleteAiPlan(apiRequest, id)
      return true
    }, false)
    if (ok) {
      plans.value = plans.value.filter((p) => p.id !== id)
      if (activePlan.value?.id === id) activePlan.value = null
    }
    return ok
  }

  // Anexa HTML formatado a nota de um mes que NAO e' o mes ativo (GET + append +
  // PUT). Para o mes ativo, o modal usa store.setNotesForActiveMonth (fonte unica).
  async function appendToMonthNotes(month: string, html: string): Promise<boolean> {
    return withSession(async () => {
      const current = await calendarApi.fetchNotesForMonth(apiRequest, month)
      const next = current ? `${current}${html}` : html
      await calendarApi.putNotesForMonth(apiRequest, month, next)
      return true
    }, false)
  }

  return {
    plans,
    loadingPlans,
    activePlan,
    starting,
    polling,
    fetchPlans,
    openPlan,
    startPlan,
    markApplied,
    removePlan,
    appendToMonthNotes,
    stopPolling: clearPolling,
    setActivePlan: (plan: CalendarAiPlan | null) => {
      activePlan.value = plan
    },
  }
}
