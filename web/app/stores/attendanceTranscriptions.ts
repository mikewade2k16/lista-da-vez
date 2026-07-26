import { ref } from 'vue'
import { defineStore } from 'pinia'

import type {
  AttendanceAnalysisConfig,
  AttendanceTranscription,
  AttendanceTranscriptionsResponse,
} from '~/domain/operation/attendance-transcriptions'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

export const useAttendanceTranscriptionsStore = defineStore('attendanceTranscriptions', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const items = ref<AttendanceTranscription[]>([])
  const total = ref(0)
  const limit = ref(30)
  const offset = ref(0)
  const loading = ref(false)
  const errorMessage = ref('')
  const audioLoadingId = ref('')
  const transcribingId = ref('')
  const analyzingId = ref('')
  const config = ref<AttendanceAnalysisConfig | null>(null)
  const configLoading = ref(false)
  const configSaving = ref(false)
  const configErrorMessage = ref('')
  const transcriptionErrorMessage = ref('')
  const audioErrorMessage = ref('')
  const activeAudioId = ref('')
  const activeAudioUrl = ref('')
  const recordingAccountIds = new Map<string, string>()

  function accountHeaders(accountId: string) {
    const normalizedAccountId = String(accountId || '').trim()
    return normalizedAccountId ? { 'X-Account-Id': normalizedAccountId } : {}
  }

  function accountIdsForStore(storeId: string) {
    const normalizedStoreId = String(storeId || '').trim()
    const stores = normalizedStoreId
      ? (auth.storeContext || []).filter(
          (store) => String(store?.id || '').trim() === normalizedStoreId,
        )
      : auth.storeContext || []
    const accountIds = stores.map((store) => String(store?.tenantId || '').trim()).filter(Boolean)
    const fallback = String(auth.activeTenantId || '').trim()
    if (accountIds.length === 0 && fallback) accountIds.push(fallback)
    return Array.from(new Set(accountIds))
  }

  function configurationAccountIds() {
    const accountIds = (auth.storeContext || [])
      .map((store) => String(store?.tenantId || '').trim())
      .filter(Boolean)
    const activeAccountId = String(auth.activeTenantId || '').trim()
    if (activeAccountId) accountIds.push(activeAccountId)
    return Array.from(new Set(accountIds))
  }

  function replaceAudioUrl(url = '') {
    if (activeAudioUrl.value) {
      URL.revokeObjectURL(activeAudioUrl.value)
    }
    activeAudioUrl.value = url
  }

  async function load(storeId = '', consultantId = '', nextOffset = 0) {
    if (loading.value) return false
    loading.value = true
    errorMessage.value = ''
    try {
      const query = new URLSearchParams({
        limit: String(limit.value),
        offset: String(Math.max(0, nextOffset)),
      })
      if (storeId) query.set('storeId', storeId)
      if (consultantId) query.set('consultantId', consultantId)
      const accountIds = accountIdsForStore(storeId)
      const scopes = accountIds.length ? accountIds : ['']
      const isMultiAccount = scopes.length > 1
      if (isMultiAccount) {
        query.set('limit', '100')
        query.set('offset', '0')
      }
      const responses = await Promise.all(
        scopes.map(async (accountId) => {
          const response = (await apiRequest(`/v1/operations/transcriptions?${query.toString()}`, {
            headers: accountHeaders(accountId),
            dedupe: false,
          })) as AttendanceTranscriptionsResponse
          ;(response.items || []).forEach((item) => recordingAccountIds.set(item.id, accountId))
          return response
        }),
      )
      const combinedItems = responses
        .flatMap((response) => (Array.isArray(response?.items) ? response.items : []))
        .sort((left, right) => Number(right.startedAt || 0) - Number(left.startedAt || 0))
      const normalizedOffset = Math.max(0, nextOffset)
      items.value = isMultiAccount
        ? combinedItems.slice(normalizedOffset, normalizedOffset + limit.value)
        : combinedItems
      total.value = isMultiAccount
        ? responses.reduce((sum, response) => sum + Math.max(0, Number(response.total || 0)), 0)
        : Math.max(0, Number(responses[0]?.total || 0))
      offset.value = isMultiAccount
        ? normalizedOffset
        : Math.max(0, Number(responses[0]?.offset || 0))
      return true
    } catch (error) {
      items.value = []
      total.value = 0
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel carregar as transcricoes.')
      return false
    } finally {
      loading.value = false
    }
  }

  async function requestTranscription(recordingId: string) {
    if (!recordingId || transcribingId.value) return false
    transcribingId.value = recordingId
    transcriptionErrorMessage.value = ''
    try {
      const response = (await apiRequest(
        `/v1/operations/transcriptions/${encodeURIComponent(recordingId)}/transcribe`,
        {
          method: 'POST',
          headers: accountHeaders(recordingAccountIds.get(recordingId) || ''),
          skipLoadingIndicator: true,
        },
      )) as { recording?: AttendanceTranscription }
      if (response.recording?.id) {
        items.value = items.value.map((item) =>
          item.id === response.recording?.id ? response.recording : item,
        )
      }
      return true
    } catch (error) {
      transcriptionErrorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel iniciar a transcricao.',
      )
      return false
    } finally {
      transcribingId.value = ''
    }
  }

  async function requestAnalysis(recordingId: string) {
    if (!recordingId || analyzingId.value) return false
    analyzingId.value = recordingId
    transcriptionErrorMessage.value = ''
    try {
      const response = (await apiRequest(
        `/v1/operations/transcriptions/${encodeURIComponent(recordingId)}/analyze`,
        {
          method: 'POST',
          headers: accountHeaders(recordingAccountIds.get(recordingId) || ''),
          skipLoadingIndicator: true,
        },
      )) as { recording?: AttendanceTranscription }
      if (response.recording?.id) {
        items.value = items.value.map((item) =>
          item.id === response.recording?.id ? response.recording : item,
        )
      }
      return true
    } catch (error) {
      transcriptionErrorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel iniciar a analise.',
      )
      return false
    } finally {
      analyzingId.value = ''
    }
  }

  async function loadConfig() {
    if (configLoading.value) return false
    configLoading.value = true
    configErrorMessage.value = ''
    try {
      const response = (await apiRequest('/v1/operations/transcriptions/config', {
        headers: accountHeaders(String(auth.activeTenantId || '').trim()),
        dedupe: false,
      })) as { config?: AttendanceAnalysisConfig }
      config.value = response.config || null
      return Boolean(config.value)
    } catch (error) {
      configErrorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel carregar a configuracao.',
      )
      return false
    } finally {
      configLoading.value = false
    }
  }

  async function saveConfig(nextConfig: AttendanceAnalysisConfig) {
    if (configSaving.value) return false
    configSaving.value = true
    configErrorMessage.value = ''
    try {
      const accountIds = configurationAccountIds()
      const scopes = accountIds.length ? accountIds : ['']
      const responses = await Promise.all(
        scopes.map(
          (accountId) =>
            apiRequest('/v1/operations/transcriptions/config', {
              method: 'PUT',
              headers: accountHeaders(accountId),
              body: nextConfig,
            }) as Promise<{ config?: AttendanceAnalysisConfig }>,
        ),
      )
      config.value = responses.find((response) => response.config)?.config || null
      return Boolean(config.value)
    } catch (error) {
      configErrorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel salvar a configuracao.',
      )
      return false
    } finally {
      configSaving.value = false
    }
  }

  async function loadAudio(recordingId: string) {
    if (!recordingId || audioLoadingId.value) return false
    if (activeAudioId.value === recordingId && activeAudioUrl.value) return true

    audioLoadingId.value = recordingId
    audioErrorMessage.value = ''
    try {
      const blob = (await apiRequest(
        `/v1/operations/transcriptions/${encodeURIComponent(recordingId)}/audio`,
        {
          headers: accountHeaders(recordingAccountIds.get(recordingId) || ''),
          responseType: 'blob',
          dedupe: false,
          skipLoadingIndicator: true,
        },
      )) as Blob
      replaceAudioUrl(URL.createObjectURL(blob))
      activeAudioId.value = recordingId
      return true
    } catch (error) {
      replaceAudioUrl()
      activeAudioId.value = ''
      audioErrorMessage.value = getApiErrorMessage(error, 'Nao foi possivel carregar este audio.')
      return false
    } finally {
      audioLoadingId.value = ''
    }
  }

  function clearAudio() {
    replaceAudioUrl()
    activeAudioId.value = ''
    audioErrorMessage.value = ''
  }

  return {
    items,
    total,
    limit,
    offset,
    loading,
    errorMessage,
    audioLoadingId,
    transcribingId,
    analyzingId,
    config,
    configLoading,
    configSaving,
    configErrorMessage,
    transcriptionErrorMessage,
    audioErrorMessage,
    activeAudioId,
    activeAudioUrl,
    load,
    loadAudio,
    requestTranscription,
    requestAnalysis,
    loadConfig,
    saveConfig,
    clearAudio,
  }
})
