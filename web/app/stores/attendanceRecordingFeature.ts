import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

interface AttendanceRecordingFeature {
  accountId: string
  enabled: boolean
  updatedAt: string | null
  updatedBy?: string
}

interface AttendanceRecordingFeatureResponse {
  feature?: Partial<AttendanceRecordingFeature>
}

export const useAttendanceRecordingFeatureStore = defineStore('attendanceRecordingFeature', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const ui = useUiStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const accountId = ref('')
  const enabled = ref(false)
  const updatedAt = ref<string | null>(null)
  const updatedBy = ref('')
  const loaded = ref(false)
  const loading = ref(false)
  const saving = ref(false)
  const errorMessage = ref('')

  const activeAccountId = computed(() => String(auth.activeTenantId || '').trim())

  function hydrate(response: AttendanceRecordingFeatureResponse) {
    const feature = response?.feature || {}
    accountId.value = String(feature.accountId || activeAccountId.value).trim()
    enabled.value = feature.enabled === true
    updatedAt.value = feature.updatedAt || null
    updatedBy.value = String(feature.updatedBy || '').trim()
    loaded.value = true
    errorMessage.value = ''
  }

  function resetForAccount(nextAccountId = activeAccountId.value) {
    accountId.value = String(nextAccountId || '').trim()
    enabled.value = false
    updatedAt.value = null
    updatedBy.value = ''
    loaded.value = false
    errorMessage.value = ''
  }

  async function load(force = false) {
    const scope = activeAccountId.value
    if (accountId.value && scope && accountId.value !== scope) resetForAccount(scope)
    if (loading.value || (loaded.value && !force)) return loaded.value

    loading.value = true
    errorMessage.value = ''
    try {
      await auth.ensureSession()
      if (!auth.accessToken) {
        errorMessage.value = 'Sessao indisponivel.'
        return false
      }
      const response = (await apiRequest(
        '/v1/operations/transcriptions/feature',
      )) as AttendanceRecordingFeatureResponse
      hydrate(response)
      return true
    } catch (error) {
      loaded.value = false
      enabled.value = false
      errorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel carregar o modo de gravacao desta conta.',
      )
      return false
    } finally {
      loading.value = false
    }
  }

  async function save(nextEnabled: boolean) {
    if (saving.value) return false
    if (auth.role !== 'platform_admin') {
      errorMessage.value = 'Esta acao exige um administrador da plataforma.'
      return false
    }

    saving.value = true
    errorMessage.value = ''
    try {
      await auth.ensureSession()
      if (!auth.accessToken) {
        errorMessage.value = 'Sessao indisponivel.'
        return false
      }
      const response = (await apiRequest('/v1/operations/transcriptions/feature', {
        method: 'PUT',
        body: { enabled: nextEnabled === true },
      })) as AttendanceRecordingFeatureResponse
      hydrate(response)
      ui.success(
        nextEnabled ? 'Gravacao ativada para esta conta.' : 'Gravacao desativada para esta conta.',
      )
      return true
    } catch (error) {
      errorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel atualizar o modo de gravacao desta conta.',
      )
      return false
    } finally {
      saving.value = false
    }
  }

  return {
    accountId,
    enabled,
    updatedAt,
    updatedBy,
    loaded,
    loading,
    saving,
    errorMessage,
    load,
    save,
    resetForAccount,
  }
})
