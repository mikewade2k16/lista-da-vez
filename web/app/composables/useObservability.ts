import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { useAuthStore } from '~/stores/auth'

// Acesso a API do painel de observabilidade (/v1/admin/observability/*, so
// platform_admin). Envelope { data }. Cada aba usa useObsQuery para carregar,
// recarregar ao mudar parametros e cancelar a requisicao anterior.

const BASE = '/v1/admin/observability'

type Params = Record<string, string | number | undefined>

export function useObservabilityApi() {
  const auth = useAuthStore()
  const apiRequest = createApiRequest(useRuntimeConfig(), () => auth.accessToken)

  async function get<T>(path: string, query: Params = {}, signal?: AbortSignal): Promise<T> {
    const cleaned = Object.fromEntries(
      Object.entries(query).filter(([, value]) => value !== undefined && value !== ''),
    )
    const response = (await apiRequest(`${BASE}${path}`, {
      query: cleaned,
      signal,
      skipLoadingIndicator: true,
    })) as { data?: T } | T
    return ((response as { data?: T })?.data ?? response) as T
  }

  async function post<T>(path: string, body: Record<string, unknown>): Promise<T> {
    const response = (await apiRequest(`${BASE}${path}`, {
      method: 'POST',
      body,
      skipLoadingIndicator: true,
    })) as { data?: T } | T
    return ((response as { data?: T })?.data ?? response) as T
  }

  return { get, post }
}

export function useObsQuery<T>(path: string, params: () => Params = () => ({})) {
  const api = useObservabilityApi()
  const data = ref<T | null>(null) as Ref<T | null>
  const loading = ref(false)
  const error = ref('')
  let controller: AbortController | null = null

  async function reload() {
    controller?.abort()
    controller = new AbortController()
    const current = controller
    loading.value = true
    error.value = ''
    try {
      const result = await api.get<T>(path, params(), current.signal)
      if (current.signal.aborted) return
      data.value = result
    } catch (cause) {
      if (current.signal.aborted) return
      error.value = getApiErrorMessage(
        cause,
        'Nao foi possivel carregar os dados de observabilidade.',
      )
    } finally {
      if (controller === current) loading.value = false
    }
  }

  watch(params, () => void reload(), { deep: true })
  onMounted(() => void reload())
  onBeforeUnmount(() => controller?.abort())

  return { data, loading, error, reload }
}

// Atualizacao automatica: roda `tick` a cada `intervalMs` so com a aba do navegador
// visivel, sem sobrepor requisicoes (`busy`), e retoma ao voltar. Limpa no unmount.
export function useObsAutoRefresh(tick: () => void, busy: () => boolean, intervalMs: number) {
  let timer: ReturnType<typeof setInterval> | null = null

  function run() {
    if (document.hidden || busy()) return
    tick()
  }
  function start() {
    if (timer) return
    timer = setInterval(run, intervalMs)
  }
  function stop() {
    if (!timer) return
    clearInterval(timer)
    timer = null
  }
  function onVisibility() {
    if (document.hidden) {
      stop()
      return
    }
    run()
    start()
  }

  onMounted(() => {
    document.addEventListener('visibilitychange', onVisibility)
    if (!document.hidden) start()
  })
  onBeforeUnmount(() => {
    document.removeEventListener('visibilitychange', onVisibility)
    stop()
  })
}
