import { ref } from 'vue'

import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiBase } from '~/utils/api-client'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import {
  defaultCalendarMediaLimits,
  type CalendarMediaItem,
  type CalendarMediaLimits,
} from '~/utils/calendar'

// Limites globais (plataforma): singleton compartilhado entre os componentes que
// usam o uploader (form de evento + drawer do dia). Uma leitura serve a todos.
const mediaLimits = ref<CalendarMediaLimits>(defaultCalendarMediaLimits())
let limitsLoaded = false

// useCalendarMedia concentra os anexos do calendario: leitura dos tetos de upload
// (globais), upload (via XHR, com progresso real para videos grandes) e anexos
// avulsos por dia. Fica fora do store para nao passar de 450 linhas e isolar o I/O
// de arquivo. Rotas em back/internal/modules/calendar (http_media.go).
export function useCalendarMedia() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  async function fetchMediaLimits(force = false): Promise<void> {
    if (limitsLoaded && !force) return
    await auth.ensureSession()
    if (!auth.isAuthenticated) return
    try {
      const res = await apiRequest('/v1/calendar/media-limits')
      mediaLimits.value = {
        ...defaultCalendarMediaLimits(),
        ...(res as Partial<CalendarMediaLimits>),
      }
      limitsLoaded = true
    } catch {
      // mantem o default
    }
  }

  // uploadMedia envia por XMLHttpRequest (o $fetch nao expoe progresso de upload).
  // Reaplica os headers que o apiRequest injeta (Authorization + X-Account-Id).
  // Resolve com o MediaItem salvo, ou null em erro (o chamador valida tamanho antes).
  function uploadMedia(
    file: File,
    onProgress?: (pct: number) => void,
  ): Promise<CalendarMediaItem | null> {
    return new Promise((resolve) => {
      const form = new FormData()
      form.append('file', file)

      const base = getApiBase(runtimeConfig).replace(/\/$/, '')
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `${base}/v1/calendar/media`)

      const token = auth.accessToken
      if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)
      const accountId = accountStore.activeAccountId
      if (accountId) xhr.setRequestHeader('X-Account-Id', accountId)

      xhr.upload.onprogress = (event) => {
        if (event.lengthComputable && onProgress) {
          onProgress(Math.round((event.loaded / event.total) * 100))
        }
      }
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            resolve(JSON.parse(xhr.responseText) as CalendarMediaItem)
          } catch {
            resolve(null)
          }
          return
        }
        resolve(null)
      }
      xhr.onerror = () => resolve(null)
      xhr.send(form)
    })
  }

  async function fetchDayMedia(date: string): Promise<CalendarMediaItem[]> {
    if (!date) return []
    await auth.ensureSession()
    if (!auth.isAuthenticated) return []
    try {
      const params = new URLSearchParams({ from: date, to: date })
      const res = await apiRequest(`/v1/calendar/day-media?${params.toString()}`)
      const days = Array.isArray(res?.days) ? res.days : []
      const match = days.find((d: { date?: string }) => d?.date === date)
      return Array.isArray(match?.media) ? (match.media as CalendarMediaItem[]) : []
    } catch {
      return []
    }
  }

  async function saveDayMedia(date: string, media: CalendarMediaItem[]): Promise<boolean> {
    if (!date) return false
    try {
      await apiRequest(`/v1/calendar/day-media/${date}`, { method: 'PUT', body: { media } })
      return true
    } catch {
      return false
    }
  }

  return { mediaLimits, fetchMediaLimits, uploadMedia, fetchDayMedia, saveDayMedia }
}
