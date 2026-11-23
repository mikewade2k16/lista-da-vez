import { createApiRequest, getApiBase, setApiObserver, type ApiCallInfo } from '~/utils/api-client'
import { useAuthStore } from '~/stores/auth'
import {
  buildErrorDetail,
  clampSampleRate,
  createEventQueue,
  fitBatchToLimit,
  MAX_BATCH_SIZE,
  shouldRecordApiCall,
  type ObsEvent,
} from '~/utils/observability/events'
import {
  COLLECT_PATH,
  isCollectorPath,
  isPublicApiPath,
  normalizeApiPath,
} from '~/utils/observability/path'
import { HEARTBEAT_INTERVAL_MS, shouldSendHeartbeat } from '~/utils/observability/health'
import { createVitalsCollector, measureSpaPageload } from '~/utils/observability/vitals'

// Coletor de observabilidade do front (Fase 2): correlation id por chamada, Web
// Vitals, erros JS e chamadas lentas/falhas, enviados em lote para /v1/obs/collect.
// Fino de proposito: a logica pura vive em ~/utils/observability/*.

const FLUSH_INTERVAL_MS = 10_000
const MAX_JS_ERRORS_PER_PAGE = 20

function randomHex(bytes: number): string {
  const buffer = new Uint8Array(bytes)
  crypto.getRandomValues(buffer)
  return Array.from(buffer, (b) => b.toString(16).padStart(2, '0')).join('')
}

export default defineNuxtPlugin((nuxtApp) => {
  const runtimeConfig = useRuntimeConfig()
  if (runtimeConfig.public.observabilityEnabled === false) return

  const sampleRate = clampSampleRate(runtimeConfig.public.observabilitySampleRate)
  const auth = useAuthStore()
  const router = useRouter()
  const queue = createEventQueue()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  let pageViewId = randomHex(8)
  let seq = 0
  let sampled = Math.random() < sampleRate
  let jsErrors = 0
  let firstNavigation = true
  let navStartedAt = performance.now()

  function currentPage(): string {
    const route = router.currentRoute.value
    return route.matched[route.matched.length - 1]?.path || route.path
  }
  let page = currentPage()

  function record(event: ObsEvent) {
    if (!sampled) return
    queue.push({ ...event, at: new Date().toISOString() })
  }

  const vitals = createVitalsCollector(record)
  vitals.start()

  // --- correlation id + chamadas de API -------------------------------------
  setApiObserver({
    nextRequestId(path) {
      if (isPublicApiPath(path) || isCollectorPath(path)) return ''
      seq += 1
      return `${pageViewId}-${seq}`
    },
    onSettled(info: ApiCallInfo) {
      if (isCollectorPath(info.path)) return
      const failed = info.status >= 400 || info.status === 0
      const detail = {
        method: info.method,
        path: normalizeApiPath(info.path),
        durationMs: info.durationMs,
      }
      if (failed) {
        record({
          kind: 'api_failure',
          name: `${info.method} ${detail.path}`,
          value: info.durationMs,
          status: info.status,
          correlationId: info.requestId || undefined,
          detail,
        })
      }
      if (shouldRecordApiCall(info.durationMs, failed)) {
        record({
          kind: 'api_call',
          name: `${info.method} ${detail.path}`,
          value: info.durationMs,
          status: info.status,
          correlationId: info.requestId || undefined,
          detail,
        })
      }
    },
  })

  // --- erros JS ----------------------------------------------------------------
  function recordJsError(name: string, message: unknown, stack: unknown, extra = {}) {
    const text = String(message ?? '')
    if (text.includes('ResizeObserver loop')) return
    if (jsErrors >= MAX_JS_ERRORS_PER_PAGE) return
    jsErrors += 1
    record({ kind: 'js_error', name, detail: buildErrorDetail(text, stack, extra) })
  }

  window.addEventListener('error', (event) => {
    recordJsError('error', event.message, event.error?.stack, {
      line: event.lineno,
      column: event.colno,
    })
  })
  window.addEventListener('unhandledrejection', (event) => {
    const reason = event.reason as { message?: string; stack?: string } | string | undefined
    const message = typeof reason === 'string' ? reason : reason?.message
    recordJsError(
      'unhandledrejection',
      message || 'unhandledrejection',
      typeof reason === 'object' ? reason?.stack : '',
    )
  })
  nuxtApp.hook('vue:error', (error, _instance, info) => {
    const err = error as { message?: string; stack?: string }
    recordJsError('vue', err?.message || String(error), err?.stack, { info })
  })

  // --- envio em lote -------------------------------------------------------------
  function takeBatch() {
    if (!auth.accessToken) return null
    const events = queue.drain(MAX_BATCH_SIZE)
    if (events.length === 0) return null
    return fitBatchToLimit({ pageViewId, page, events })
  }

  async function sendBatch() {
    const body = takeBatch()
    if (!body) return
    try {
      await apiRequest(COLLECT_PATH, {
        method: 'POST',
        body,
        skipLoadingIndicator: true,
        skipObserver: true,
        skipUnauthorizedHandler: true,
        dedupe: false,
      })
    } catch {
      // Descartado em silencio: observabilidade nunca atrapalha o usuario.
    }
  }

  // sendBeacon nao envia Authorization; fetch keepalive sobrevive ao fechar a aba.
  function sendBatchKeepalive() {
    const body = takeBatch()
    if (!body || !auth.accessToken) return
    try {
      void fetch(`${getApiBase(runtimeConfig)}${COLLECT_PATH}`, {
        method: 'POST',
        keepalive: true,
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${auth.accessToken}`,
        },
        body: JSON.stringify(body),
      }).catch(() => undefined)
    } catch {
      // ignorado
    }
  }

  // --- heartbeat de presenca -------------------------------------------------------
  // Nao passa pela amostragem: presenca precisa ser completa. Aba oculta, deslogado
  // ou desligado = nao envia (decisao pura em utils/observability/health).
  let lastHeartbeatAt = 0

  function maybeHeartbeat(routeChanged: boolean) {
    const allowed = shouldSendHeartbeat({
      hidden: document.visibilityState === 'hidden',
      loggedIn: Boolean(auth.accessToken),
      enabled: runtimeConfig.public.observabilityEnabled !== false,
      lastSentAt: lastHeartbeatAt,
      now: Date.now(),
      routeChanged,
    })
    if (!allowed) return
    lastHeartbeatAt = Date.now()
    queue.push({ kind: 'heartbeat', name: page, at: new Date().toISOString() })
    void sendBatch()
  }

  setInterval(() => void sendBatch(), FLUSH_INTERVAL_MS)
  setInterval(() => maybeHeartbeat(false), HEARTBEAT_INTERVAL_MS)
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState !== 'hidden') {
      maybeHeartbeat(false)
      return
    }
    vitals.flush()
    sendBatchKeepalive()
  })

  // --- navegacao -----------------------------------------------------------------
  router.beforeEach(() => {
    navStartedAt = performance.now()
  })
  router.afterEach(() => {
    if (firstNavigation) {
      firstNavigation = false
      page = currentPage()
      maybeHeartbeat(true)
      return
    }
    // Fecha a pagina anterior (vitals + lote) antes de trocar o contexto.
    vitals.flush()
    sendBatchKeepalive()
    pageViewId = randomHex(8)
    seq = 0
    jsErrors = 0
    sampled = Math.random() < sampleRate
    page = currentPage()
    measureSpaPageload(navStartedAt, record)
    maybeHeartbeat(true)
  })
})
