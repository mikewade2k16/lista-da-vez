import { computed, ref } from 'vue'

import type {
  PerformanceFeedbackContext,
  PerformanceFeedbackMetrics,
  PerformanceFeedbackReview,
  PerformanceFeedbackSection,
  PerformanceFeedbackSettings,
} from '~/types/performance-feedback'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { goalWeekCount } from '~/utils/goal-periods'

function currentMonth(): string {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

export function usePerformanceFeedback() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const toast = useToast()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const context = ref<PerformanceFeedbackContext | null>(null)
  const selectedStoreId = ref('')
  const selectedConsultantId = ref('')
  const month = ref(currentMonth())
  const week = ref(0)
  const feedbackSections = ref<PerformanceFeedbackSection[]>([])
  const consultantNotesHtml = ref('')
  const pending = ref(false)
  const saving = ref(false)
  const errorMessage = ref('')

  const selectedReview = computed(() => context.value?.review ?? null)
  const selectedMetrics = computed(() => context.value?.metrics ?? null)
  const canManage = computed(() => Boolean(context.value?.canManage))
  const canRespond = computed(() => Boolean(context.value?.canRespond))

  async function openFor(target: {
    storeId: string
    consultantId: string
    month?: string
    week?: number
  }): Promise<void> {
    await auth.ensureSession()
    selectedStoreId.value = String(target.storeId || '').trim()
    selectedConsultantId.value = String(target.consultantId || '').trim()
    month.value = /^\d{4}-\d{2}$/.test(String(target.month || ''))
      ? String(target.month)
      : currentMonth()
    week.value = Math.min(
      goalWeekCount(month.value),
      Math.max(0, Math.trunc(Number(target.week || 0))),
    )
    context.value = null
    hydrateDrafts()
    await load()
  }

  async function openSettingsForStore(storeId: string): Promise<void> {
    await auth.ensureSession()
    selectedStoreId.value = String(storeId || '').trim()
    selectedConsultantId.value = ''
    month.value = currentMonth()
    week.value = 0
    context.value = null
    hydrateDrafts()
    await load()
  }

  async function load(): Promise<void> {
    if (!selectedStoreId.value) {
      context.value = null
      return
    }

    pending.value = true
    errorMessage.value = ''
    try {
      const params = new URLSearchParams({
        storeId: selectedStoreId.value,
        month: month.value,
        week: String(week.value),
      })
      if (selectedConsultantId.value) {
        params.set('consultantId', selectedConsultantId.value)
      }

      const response = (await apiRequest(
        `/v1/performance-feedback/context?${params.toString()}`,
      )) as PerformanceFeedbackContext
      context.value = response
      week.value = response.period.week
      selectedConsultantId.value = response.selectedConsultant?.id || ''
      hydrateDrafts(response.review, response.settings)
    } catch (error: unknown) {
      errorMessage.value = getApiErrorMessage(
        error,
        'Não foi possível carregar os dados de feedback.',
      )
    } finally {
      pending.value = false
    }
  }

  function hydrateDrafts(
    review?: PerformanceFeedbackReview,
    settings?: PerformanceFeedbackSettings,
  ): void {
    feedbackSections.value = review
      ? review.feedbackSections.map((section) => ({ ...section }))
      : (settings?.defaultSections ?? []).map((section) => ({ ...section, contentHtml: '' }))
    consultantNotesHtml.value = review?.consultantNotesHtml ?? ''
  }

  async function saveManager(metricsSnapshot: PerformanceFeedbackMetrics): Promise<void> {
    if (!selectedConsultantId.value || !selectedStoreId.value) return

    saving.value = true
    try {
      const response = (await apiRequest('/v1/performance-feedback/manager', {
        method: 'PUT',
        body: {
          storeId: selectedStoreId.value,
          consultantId: selectedConsultantId.value,
          month: month.value,
          week: week.value,
          feedbackSections: feedbackSections.value,
          status: 'shared',
          metricsSnapshot,
          expectedVersion: selectedReview.value?.version ?? 0,
        },
      })) as { review: PerformanceFeedbackReview }
      await load()
      toast.add({
        title: 'Feedback salvo',
        description: 'O registro foi salvo no banco e entrou no histórico do consultor.',
        color: 'success',
      })
      if (context.value) context.value.review = response.review
    } catch (error: unknown) {
      toast.add({
        title: 'Não foi possível salvar',
        description: getApiErrorMessage(error, 'Revise os campos e tente novamente.'),
        color: 'error',
      })
    } finally {
      saving.value = false
    }
  }

  async function saveSettings(input: {
    cadence: PerformanceFeedbackSettings['cadence']
    defaultSections: PerformanceFeedbackSection[]
    expectedVersion: number
  }): Promise<boolean> {
    saving.value = true
    try {
      const response = (await apiRequest('/v1/performance-feedback/settings', {
        method: 'PUT',
        body: {
          ...input,
          storeId: selectedStoreId.value,
        },
      })) as { settings: PerformanceFeedbackSettings }
      if (context.value) context.value.settings = response.settings
      toast.add({
        title: 'Configuração salva',
        description: 'Os próximos feedbacks usarão esse formato como padrão.',
        color: 'success',
      })
      return true
    } catch (error: unknown) {
      toast.add({
        title: 'Não foi possível salvar a configuração',
        description: getApiErrorMessage(error, 'Revise os campos e tente novamente.'),
        color: 'error',
      })
      return false
    } finally {
      saving.value = false
    }
  }

  async function saveConsultant(): Promise<void> {
    const review = selectedReview.value
    if (!review) return

    saving.value = true
    try {
      await apiRequest(`/v1/performance-feedback/${encodeURIComponent(review.id)}/consultant`, {
        method: 'PUT',
        body: {
          consultantNotesHtml: consultantNotesHtml.value,
          expectedVersion: review.version,
        },
      })
      await load()
      toast.add({
        title: 'Devolutiva registrada',
        description: 'Sua percepção e seus compromissos foram salvos.',
        color: 'success',
      })
    } catch (error: unknown) {
      toast.add({
        title: 'Não foi possível salvar',
        description: getApiErrorMessage(error, 'Atualize a página e tente novamente.'),
        color: 'error',
      })
    } finally {
      saving.value = false
    }
  }

  return {
    context,
    selectedStoreId,
    selectedConsultantId,
    month,
    week,
    feedbackSections,
    consultantNotesHtml,
    pending,
    saving,
    errorMessage,
    selectedReview,
    selectedMetrics,
    canManage,
    canRespond,
    openFor,
    openSettingsForStore,
    load,
    saveManager,
    saveSettings,
    saveConsultant,
  }
}
