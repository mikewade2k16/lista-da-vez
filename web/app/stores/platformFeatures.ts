import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

export interface ExperimentalFeatures {
  version: number
  attendanceAudioRecording: boolean
}

interface ExperimentalFeaturesResponse {
  features: ExperimentalFeatures
  updatedAt: string | null
  updatedBy: string | null
}

export function defaultExperimentalFeatures(): ExperimentalFeatures {
  return {
    version: 1,
    attendanceAudioRecording: false,
  }
}

export function normalizeExperimentalFeatures(raw: unknown): ExperimentalFeatures {
  const candidate = (raw || {}) as Partial<ExperimentalFeatures>
  return {
    version: Math.max(1, Number(candidate.version) || 1),
    attendanceAudioRecording: candidate.attendanceAudioRecording === true,
  }
}

export const usePlatformFeaturesStore = defineStore('platformFeatures', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const ui = useUiStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const features = ref<ExperimentalFeatures>(defaultExperimentalFeatures())
  const updatedAt = ref<string | null>(null)
  const updatedBy = ref<string | null>(null)
  const loaded = ref(false)
  const loading = ref(false)
  const saving = ref(false)
  const errorMessage = ref('')

  const attendanceAudioRecordingEnabled = computed(
    () => loaded.value && features.value.attendanceAudioRecording,
  )

  function hydrate(response: ExperimentalFeaturesResponse) {
    features.value = normalizeExperimentalFeatures(response?.features)
    updatedAt.value = response?.updatedAt || null
    updatedBy.value = response?.updatedBy || null
    loaded.value = true
    errorMessage.value = ''
  }

  async function load(force = false) {
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
        '/v1/platform/experimental-features',
      )) as ExperimentalFeaturesResponse
      hydrate(response)
      return true
    } catch (error) {
      loaded.value = false
      errorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel carregar os recursos experimentais.',
      )
      return false
    } finally {
      loading.value = false
    }
  }

  async function save(next: ExperimentalFeatures) {
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
      const response = (await apiRequest('/v1/platform/experimental-features', {
        method: 'PUT',
        body: { features: normalizeExperimentalFeatures(next) },
      })) as ExperimentalFeaturesResponse
      hydrate(response)
      ui.success('Recursos experimentais atualizados.')
      return true
    } catch (error) {
      errorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel atualizar os recursos experimentais.',
      )
      return false
    } finally {
      saving.value = false
    }
  }

  return {
    features,
    updatedAt,
    updatedBy,
    loaded,
    loading,
    saving,
    errorMessage,
    attendanceAudioRecordingEnabled,
    load,
    save,
  }
})
