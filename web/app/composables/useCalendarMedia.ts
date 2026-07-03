import { ref } from 'vue'

import { useAuthStore } from '~/stores/auth'
import * as calendarApi from '~/domain/calendar/calendar-api'
import { createApiRequest, getApiBase } from '~/utils/api-client'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import {
  defaultCalendarMediaLimits,
  type CalendarMediaItem,
  type CalendarMediaLimits,
} from '~/utils/calendar'
import { capturePosterFromVideo } from '~/utils/calendar-poster'

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
      mediaLimits.value = await calendarApi.fetchMediaLimits(apiRequest)
      limitsLoaded = true
    } catch {
      // mantem o default
    }
  }

  // Salva os tetos GLOBAIS (so platform_admin; o back retorna 403 fora disso).
  // Atualiza o singleton compartilhado em sucesso (o uploader passa a validar
  // contra o novo teto sem recarregar).
  async function saveMediaLimits(next: CalendarMediaLimits): Promise<boolean> {
    try {
      mediaLimits.value = await calendarApi.putMediaLimits(apiRequest, next)
      limitsLoaded = true
      return true
    } catch {
      return false
    }
  }

  // uploadMedia envia por XMLHttpRequest (o $fetch nao expoe progresso de upload).
  // Reaplica os headers que o apiRequest injeta (Authorization + X-Account-Id).
  // Resolve com o MediaItem salvo, ou null em erro (o chamador valida tamanho antes).
  // onError recebe o code do back (invalid_media/media_too_large/...) + status HTTP
  // para o chamador montar um aviso acionavel, nunca um "falhou" seco.
  function uploadMedia(
    file: File,
    onProgress?: (pct: number) => void,
    onError?: (code: string, status: number) => void,
  ): Promise<CalendarMediaItem | null> {
    return new Promise((resolve) => {
      const form = new FormData()
      form.append('file', file)

      const base = getApiBase(runtimeConfig).replace(/\/$/, '')
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `${base}/v1/calendar/media`)
      // Teto de tempo do upload: sem isso, um proxy travado (ex.: port-forward
      // do Docker no Windows) deixa o envio pendurado em 0% para sempre.
      xhr.timeout = 15 * 60 * 1000

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
        // Extrai o code do payload de erro do back ({error:{code,message}}).
        let code = ''
        try {
          const body = JSON.parse(xhr.responseText) as { error?: { code?: string } }
          code = String(body?.error?.code || '')
        } catch {
          // corpo nao-JSON: mantem code vazio
        }
        if (onError) onError(code, xhr.status)
        resolve(null)
      }
      xhr.onerror = () => {
        if (onError) onError('network', 0)
        resolve(null)
      }
      xhr.ontimeout = () => {
        if (onError) onError('timeout', 0)
        resolve(null)
      }
      xhr.onabort = () => {
        if (onError) onError('network', 0)
        resolve(null)
      }
      xhr.send(form)
    })
  }

  // uploadVideoWithPoster sobe o video (com progresso) e, em seguida, tenta
  // capturar+subir um poster do primeiro frame silenciosamente. Falha do poster
  // NAO falha o upload: o item volta sem posterUrl (fallback = <video>). O
  // posterUrl so e aceito se o back devolver um /uploads/calendar/... valido.
  async function uploadVideoWithPoster(
    file: File,
    onProgress?: (pct: number) => void,
    onError?: (code: string, status: number) => void,
  ): Promise<CalendarMediaItem | null> {
    const item = await uploadMedia(file, onProgress, onError)
    if (!item) return null
    try {
      const poster = await capturePosterFromVideo(file)
      if (!poster) return item
      const posterItem = await uploadMedia(poster)
      if (posterItem?.url) return { ...item, posterUrl: posterItem.url }
    } catch {
      // sem poster: mantem o item original
    }
    return item
  }

  return {
    mediaLimits,
    fetchMediaLimits,
    saveMediaLimits,
    uploadMedia,
    uploadVideoWithPoster,
    capturePosterFromVideo,
  }
}
