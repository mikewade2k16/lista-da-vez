import { useObservabilityApi } from '~/composables/useObservability'
import type {
  ObsFieldErrors,
  ObsSettings,
  ObsSettingsKey,
  ObsTextPreview,
} from '~/types/observability-settings'
import { getApiErrorMessage } from '~/utils/api-client'
import { fieldErrorsFrom } from '~/utils/observability/settings'

// Configuracoes de alertas, e-mail e relatorio semanal (GET/PUT /settings).
export function useObservabilitySettings() {
  const api = useObservabilityApi()
  const settings = ref<ObsSettings | null>(null)
  const loading = ref(false)
  const error = ref('')

  async function load() {
    loading.value = true
    error.value = ''
    try {
      settings.value = await api.get<ObsSettings>('/settings')
    } catch (cause) {
      error.value = getApiErrorMessage(cause, 'Não foi possível carregar as configurações.')
    } finally {
      loading.value = false
    }
  }

  // Grava uma chave e atualiza o estado com o que a API devolve (valores normalizados).
  async function save(key: ObsSettingsKey, value: object) {
    settings.value = await api.put<ObsSettings>(
      `/settings/${key}`,
      value as Record<string, unknown>,
    )
  }

  function previewEmail(value: object): Promise<ObsTextPreview> {
    return api.post<ObsTextPreview>('/settings/email/preview', value as Record<string, unknown>)
  }

  function weeklyPreview(): Promise<ObsTextPreview> {
    return api.get<ObsTextPreview>('/reports/weekly/preview')
  }

  async function weeklySend() {
    await api.post('/reports/weekly/send', {})
  }

  onMounted(() => void load())
  return { settings, loading, error, load, save, previewEmail, weeklyPreview, weeklySend }
}

// Estado de "salvar este bloco": carregando, mensagem e erros por campo do 400.
export function useBlockSaver() {
  const saving = ref(false)
  const fieldErrors = ref<ObsFieldErrors>({})
  const message = ref('')
  const failed = ref(false)

  async function run(action: () => Promise<void>, successMessage = 'Salvo.') {
    saving.value = true
    fieldErrors.value = {}
    message.value = ''
    failed.value = false
    try {
      await action()
      message.value = successMessage
    } catch (cause) {
      failed.value = true
      const fields = fieldErrorsFrom(cause)
      fieldErrors.value = fields
      if (fields._) {
        message.value = fields._
      } else if (Object.keys(fields).length > 0) {
        message.value = 'Corrija os campos destacados.'
      } else {
        message.value = getApiErrorMessage(cause, 'Não foi possível salvar.')
      }
    } finally {
      saving.value = false
    }
  }

  function clear() {
    fieldErrors.value = {}
    message.value = ''
    failed.value = false
  }

  return { saving, fieldErrors, message, failed, run, clear }
}
