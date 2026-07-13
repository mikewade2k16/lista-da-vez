import type {
  QrCodeItem,
  QrCodesListResponse,
  QrCodeMutationResponse,
  CreateQrCodePayload,
  UpdateQrCodePayload,
} from '~/types/tools'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

const DEFAULT_FETCH_LIMIT = 200

// useQrcodesManager: estado + CRUD dos QR codes (/v1/tools/qr-codes). A imagem PNG
// e gerada no cliente a partir de qrUrl+cores+size (ver ToolsQrCodeWorkspace).
export function useQrcodesManager() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const qrcodes = ref<QrCodeItem[]>([])
  const loading = ref(false)
  const creating = ref(false)
  const deletingId = ref<string | null>(null)
  const errorMessage = ref('')
  const savingMap = ref<Record<string, boolean>>({})

  function setSaving(key: string, value: boolean) {
    savingMap.value = { ...savingMap.value, [key]: value }
  }

  async function fetchQrCodes() {
    loading.value = true
    errorMessage.value = ''
    try {
      const response = (await apiRequest(
        `/v1/tools/qr-codes?limit=${DEFAULT_FETCH_LIMIT}`,
      )) as QrCodesListResponse
      qrcodes.value = Array.isArray(response?.data) ? response.data : []
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Falha ao carregar QR Codes.')
    } finally {
      loading.value = false
    }
  }

  async function createQrCode(payload: CreateQrCodePayload) {
    creating.value = true
    errorMessage.value = ''
    try {
      const body: CreateQrCodePayload = { targetUrl: payload.targetUrl }
      if (payload.slug) {
        body.slug = payload.slug
      }
      if (payload.fillColor) {
        body.fillColor = payload.fillColor
      }
      if (payload.backColor) {
        body.backColor = payload.backColor
      }
      if (typeof payload.size === 'number') {
        body.size = payload.size
      }
      if (typeof payload.isActive === 'boolean') {
        body.isActive = payload.isActive
      }
      if (payload.accountId) {
        body.accountId = payload.accountId
      }
      const response = (await apiRequest('/v1/tools/qr-codes', {
        method: 'POST',
        body,
      })) as QrCodeMutationResponse
      if (response?.data) {
        qrcodes.value = [response.data, ...qrcodes.value]
      }
      return response?.data ?? null
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Falha ao criar QR Code.')
      return null
    } finally {
      creating.value = false
    }
  }

  async function updateQrCode(
    id: string,
    payload: UpdateQrCodePayload,
    action: 'update' | 'toggle',
  ) {
    setSaving(`${id}:${action}`, true)
    errorMessage.value = ''
    try {
      const body: UpdateQrCodePayload = {}
      if (payload.targetUrl !== undefined) {
        body.targetUrl = payload.targetUrl
      }
      if (payload.slug !== undefined) {
        body.slug = payload.slug
      }
      if (payload.fillColor !== undefined) {
        body.fillColor = payload.fillColor
      }
      if (payload.backColor !== undefined) {
        body.backColor = payload.backColor
      }
      if (payload.size !== undefined) {
        body.size = payload.size
      }
      if (payload.isActive !== undefined) {
        body.isActive = payload.isActive
      }
      const response = (await apiRequest(`/v1/tools/qr-codes/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        body,
      })) as QrCodeMutationResponse
      if (response?.data) {
        qrcodes.value = qrcodes.value.map((item) => (item.id === id ? response.data : item))
      }
      return response?.data ?? null
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Falha ao atualizar QR Code.')
      return null
    } finally {
      setSaving(`${id}:${action}`, false)
    }
  }

  async function deleteQrCode(id: string) {
    deletingId.value = id
    setSaving(`${id}:delete`, true)
    errorMessage.value = ''
    try {
      await apiRequest(`/v1/tools/qr-codes/${encodeURIComponent(id)}`, { method: 'DELETE' })
      qrcodes.value = qrcodes.value.filter((item) => item.id !== id)
      return true
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Falha ao excluir QR Code.')
      return false
    } finally {
      deletingId.value = null
      setSaving(`${id}:delete`, false)
    }
  }

  return {
    qrcodes,
    loading,
    creating,
    deletingId,
    errorMessage,
    savingMap,
    fetchQrCodes,
    createQrCode,
    updateQrCode,
    deleteQrCode,
  }
}
