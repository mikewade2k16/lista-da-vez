import { ref } from 'vue'
import { defineStore } from 'pinia'

import type { StorageSettings, StorageSettingsInput, StorageStatus } from '~/types/storage'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

interface ApiEnvelope<T> {
  data?: T
}

export const useStorageStore = defineStore('storageAdmin', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const ui = useUiStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const status = ref<StorageStatus | null>(null)
  const loading = ref(false)
  const saving = ref(false)
  const checking = ref(false)
  const errorMessage = ref('')

  function ensurePlatformAdmin() {
    if (auth.role === 'platform_admin') return true
    errorMessage.value = 'Esta area exige um administrador da plataforma.'
    return false
  }

  function readData<T>(response: ApiEnvelope<T>, fallback: string): T {
    if (!response?.data) throw new Error(fallback)
    return response.data
  }

  async function load(force = false) {
    if (loading.value || (!force && status.value)) return Boolean(status.value)
    if (!ensurePlatformAdmin()) return false

    loading.value = true
    errorMessage.value = ''
    try {
      await auth.ensureSession()
      const response = (await apiRequest('/v1/storage/status', {
        dedupe: !force,
      })) as ApiEnvelope<StorageStatus>
      status.value = readData(response, 'Resposta invalida ao carregar o storage.')
      return true
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel carregar o storage R2.')
      return false
    } finally {
      loading.value = false
    }
  }

  async function saveSettings(
    input: StorageSettingsInput,
    successMessage = 'Limites de seguranca do R2 atualizados.',
  ) {
    if (saving.value || !ensurePlatformAdmin()) return false

    saving.value = true
    errorMessage.value = ''
    try {
      await auth.ensureSession()
      const response = (await apiRequest('/v1/storage/settings', {
        method: 'PUT',
        body: input,
      })) as ApiEnvelope<StorageSettings>
      const settings = readData(response, 'Resposta invalida ao salvar os limites.')
      if (status.value) status.value = { ...status.value, settings }
      ui.success(successMessage)
      return true
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel salvar os limites do R2.')
      return false
    } finally {
      saving.value = false
    }
  }

  async function checkConnection() {
    if (checking.value || !ensurePlatformAdmin()) return false

    checking.value = true
    errorMessage.value = ''
    try {
      await auth.ensureSession()
      const response = (await apiRequest('/v1/storage/connection-check', {
        method: 'POST',
      })) as ApiEnvelope<StorageStatus>
      status.value = readData(response, 'Resposta invalida ao validar a conexao.')
      ui.success('Conexao com o Cloudflare R2 validada.')
      return true
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel validar a conexao R2.')
      return false
    } finally {
      checking.value = false
    }
  }

  return { status, loading, saving, checking, errorMessage, load, saveSettings, checkConnection }
})
