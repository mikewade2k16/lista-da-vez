import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import {
  fetchCustomerRelationshipProfile,
  fetchCustomerSubjects,
} from '~/domain/customer-data/profile-api'
import type {
  CustomerRelationshipProfile,
  CustomerSubjectListFilters,
  CustomerSubjectListItem,
} from '~/domain/customer-data/profile-types'
import {
  fetchRelationshipFacts,
  fetchRelationshipSummaries,
  fetchRelationshipTimeline,
} from '~/domain/customer-intelligence/api'
import {
  classifyCustomerApiError,
  type CustomerApiErrorState,
} from '~/domain/customer-intelligence/api-error'
import type {
  IntelligenceFactView,
  IntelligenceSummaryView,
  IntelligenceTimelineItem,
} from '~/domain/customer-intelligence/types'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest } from '~/utils/api-client'

type LoadStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'error'

interface CustomerIntelligenceReadAccess {
  subjects: boolean
  deterministicProfile: boolean
  intelligenceProfile: boolean
}

interface RequestToken {
  controller: AbortController
  generation: number
  scopeKey: string
}

function normalizeId(value: unknown): string {
  return String(value ?? '').trim()
}

export const useCustomerIntelligenceStore = defineStore('customer-intelligence', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const api = createApiRequest(runtimeConfig, () => auth.accessToken)
  const activeRequests = new Set<AbortController>()
  let generation = 0

  const ownerAccountId = ref('')
  const clientAccountId = ref('')
  const readAccess = ref<CustomerIntelligenceReadAccess>({
    subjects: false,
    deterministicProfile: false,
    intelligenceProfile: false,
  })

  const subjects = ref<CustomerSubjectListItem[]>([])
  const subjectsCursor = ref('')
  const subjectsHasMore = ref(false)
  const subjectsStatus = ref<LoadStatus>('idle')
  const subjectsError = ref<CustomerApiErrorState | null>(null)

  const relationshipId = ref('')
  const deterministicProfile = ref<CustomerRelationshipProfile | null>(null)
  const facts = ref<IntelligenceFactView[]>([])
  const summaries = ref<IntelligenceSummaryView[]>([])
  const timeline = ref<IntelligenceTimelineItem[]>([])
  const profileStatus = ref<LoadStatus>('idle')
  const intelligenceStatus = ref<LoadStatus>('idle')
  const profileError = ref<CustomerApiErrorState | null>(null)
  const intelligenceError = ref<CustomerApiErrorState | null>(null)

  const scopeKey = computed(
    () => `${normalizeId(ownerAccountId.value)}:${normalizeId(clientAccountId.value)}`,
  )

  function abortActiveRequests(): void {
    generation += 1
    for (const controller of activeRequests) controller.abort()
    activeRequests.clear()
  }

  function clearSubjects(): void {
    subjects.value = []
    subjectsCursor.value = ''
    subjectsHasMore.value = false
    subjectsStatus.value = 'idle'
    subjectsError.value = null
  }

  function clearProfile(): void {
    relationshipId.value = ''
    deterministicProfile.value = null
    facts.value = []
    summaries.value = []
    timeline.value = []
    profileStatus.value = 'idle'
    intelligenceStatus.value = 'idle'
    profileError.value = null
    intelligenceError.value = null
  }

  function clearSensitiveState(): void {
    clearSubjects()
    clearProfile()
  }

  function setScope(nextOwnerAccountId: string, nextClientAccountId: string): void {
    const owner = normalizeId(nextOwnerAccountId)
    const client = normalizeId(nextClientAccountId)
    if (ownerAccountId.value === owner && clientAccountId.value === client) return
    abortActiveRequests()
    clearSensitiveState()
    ownerAccountId.value = owner
    clientAccountId.value = client
  }

  function setReadAccess(next: CustomerIntelligenceReadAccess): void {
    const lostAccess =
      (readAccess.value.subjects && !next.subjects) ||
      (readAccess.value.deterministicProfile && !next.deterministicProfile) ||
      (readAccess.value.intelligenceProfile && !next.intelligenceProfile)
    readAccess.value = { ...next }
    if (!lostAccess) return
    abortActiveRequests()
    if (!next.subjects) clearSubjects()
    if (!next.deterministicProfile && !next.intelligenceProfile) clearProfile()
    if (!next.intelligenceProfile) {
      facts.value = []
      summaries.value = []
      timeline.value = []
      intelligenceStatus.value = 'idle'
      intelligenceError.value = null
    }
  }

  function beginRequest(): RequestToken {
    const controller = new AbortController()
    activeRequests.add(controller)
    return { controller, generation, scopeKey: scopeKey.value }
  }

  function finishRequest(token: RequestToken): void {
    activeRequests.delete(token.controller)
  }

  function isCurrent(token: RequestToken): boolean {
    return (
      !token.controller.signal.aborted &&
      token.generation === generation &&
      token.scopeKey === scopeKey.value
    )
  }

  async function loadSubjects(
    filters: Omit<CustomerSubjectListFilters, 'clientAccountId' | 'cursor'> = {},
    append = false,
  ): Promise<void> {
    if (!readAccess.value.subjects || !clientAccountId.value) {
      clearSubjects()
      return
    }
    const token = beginRequest()
    subjectsStatus.value = 'loading'
    subjectsError.value = null
    try {
      const page = await fetchCustomerSubjects(
        api,
        {
          ...filters,
          clientAccountId: clientAccountId.value,
          cursor: append ? subjectsCursor.value : '',
        },
        token.controller.signal,
      )
      if (!isCurrent(token)) return
      if (append) {
        const known = new Set(subjects.value.map((item) => item.relationship.id))
        subjects.value = [
          ...subjects.value,
          ...page.items.filter((item) => !known.has(item.relationship.id)),
        ]
      } else {
        subjects.value = page.items
      }
      subjectsCursor.value = page.nextCursor
      subjectsHasMore.value = page.hasMore
      subjectsStatus.value = subjects.value.length ? 'ready' : 'empty'
    } catch (cause) {
      if (!isCurrent(token)) return
      const error = classifyCustomerApiError(cause, 'Nao foi possivel carregar os clientes.')
      if (error.kind === 'aborted') return
      subjectsError.value = error
      subjectsStatus.value = 'error'
    } finally {
      finishRequest(token)
    }
  }

  async function loadDeterministicProfile(nextRelationshipId: string): Promise<void> {
    const id = normalizeId(nextRelationshipId)
    if (!readAccess.value.deterministicProfile || !clientAccountId.value || !id) {
      clearProfile()
      return
    }
    clearProfile()
    relationshipId.value = id
    const token = beginRequest()
    profileStatus.value = 'loading'
    try {
      const profile = await fetchCustomerRelationshipProfile(
        api,
        id,
        clientAccountId.value,
        token.controller.signal,
      )
      if (!isCurrent(token) || relationshipId.value !== id) return
      deterministicProfile.value = profile
      profileStatus.value = 'ready'
    } catch (cause) {
      if (!isCurrent(token)) return
      const error = classifyCustomerApiError(cause, 'Nao foi possivel carregar o perfil.')
      if (error.kind === 'aborted') return
      profileError.value = error
      profileStatus.value = 'error'
    } finally {
      finishRequest(token)
    }
  }

  async function loadIntelligenceProfile(nextRelationshipId: string): Promise<void> {
    const id = normalizeId(nextRelationshipId)
    if (!readAccess.value.intelligenceProfile || !clientAccountId.value || !id) {
      facts.value = []
      summaries.value = []
      timeline.value = []
      intelligenceStatus.value = 'idle'
      intelligenceError.value = null
      return
    }
    relationshipId.value = id
    const token = beginRequest()
    intelligenceStatus.value = 'loading'
    intelligenceError.value = null
    try {
      const [factsResult, summariesResult, timelineResult] = await Promise.allSettled([
        fetchRelationshipFacts(api, id, clientAccountId.value, '', token.controller.signal),
        fetchRelationshipSummaries(api, id, clientAccountId.value, token.controller.signal),
        fetchRelationshipTimeline(api, id, clientAccountId.value, '', token.controller.signal),
      ])
      if (!isCurrent(token) || relationshipId.value !== id) return

      facts.value = factsResult.status === 'fulfilled' ? factsResult.value.items : []
      summaries.value = summariesResult.status === 'fulfilled' ? summariesResult.value : []
      timeline.value = timelineResult.status === 'fulfilled' ? timelineResult.value.items : []
      const firstFailure = [factsResult, summariesResult, timelineResult].find(
        (result) => result.status === 'rejected',
      )
      if (firstFailure?.status === 'rejected') {
        intelligenceError.value = classifyCustomerApiError(
          firstFailure.reason,
          'A inteligencia do cliente esta parcialmente indisponivel.',
        )
      }
      intelligenceStatus.value =
        facts.value.length || summaries.value.length || timeline.value.length ? 'ready' : 'empty'
    } finally {
      finishRequest(token)
    }
  }

  function dispose(): void {
    abortActiveRequests()
    clearSensitiveState()
  }

  return {
    ownerAccountId,
    clientAccountId,
    readAccess,
    scopeKey,
    subjects,
    subjectsCursor,
    subjectsHasMore,
    subjectsStatus,
    subjectsError,
    relationshipId,
    deterministicProfile,
    facts,
    summaries,
    timeline,
    profileStatus,
    intelligenceStatus,
    profileError,
    intelligenceError,
    setScope,
    setReadAccess,
    loadSubjects,
    loadDeterministicProfile,
    loadIntelligenceProfile,
    clearProfile,
    dispose,
  }
})
