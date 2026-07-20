import { createApiRequest } from '~/utils/api-client'
import { useAuthStore } from '~/stores/auth'

// COSTURA (F1) — adaptador temporario do modulo omnichannel portado do legado.
// Alvo de remocao: F14 (docs/LEGADO.md). O legado falava com um BFF Nitro em
// `/api/bff`; aqui o mesmo contrato aponta para o Go em `/v1/omnichannel`.
//
// Por que este arquivo existe: os 67 arquivos verbatim importam `~/composables/useApi`
// e dependem de `apiFetch` + `ApiClientError`. Reescrever os call-sites seria
// abandonar o verbatim (o front legado e a especificacao) — entao a casca fica aqui.

const OMNICHANNEL_API_PREFIX = '/v1/omnichannel'

interface ApiClientErrorOptions {
  statusCode?: number
  data?: unknown
}

// Superficie identica a do legado (web-reference/app/composables/useApi.ts:9).
// NAO e cosmetico: `useOmnichannelInboxHistory.ts` testa
// `error instanceof ApiClientError && error.statusCode === 404` para decidir o
// fim da paginacao. Deixar vazar o FetchError cru do ofetch faria o instanceof
// dar false silencioso e a paginacao de historico quebraria sem erro visivel.
export class ApiClientError extends Error {
  statusCode: number
  data: unknown

  constructor(message: string, options: ApiClientErrorOptions = {}) {
    super(message)
    this.name = 'ApiClientError'
    this.statusCode = options.statusCode ?? 500
    this.data = options.data ?? null
  }
}

function toErrorMessage(error: unknown, fallback = 'Operacao falhou') {
  if (error && typeof error === 'object' && 'data' in error) {
    const data = (error as { data?: Record<string, unknown> }).data
    // Erro do Go: { error: { message } }. Erro do legado: { message }.
    const nested = data?.error as { message?: unknown } | undefined
    if (nested && typeof nested.message === 'string' && nested.message.trim()) {
      return nested.message
    }
    if (data && typeof data.message === 'string' && data.message.trim()) {
      return data.message
    }
  }

  if (error instanceof Error && error.message.trim()) {
    return error.message
  }

  return fallback
}

function toStatusCode(error: unknown) {
  if (error && typeof error === 'object') {
    const candidate = error as {
      statusCode?: unknown
      status?: unknown
      response?: { status?: unknown }
    }
    const raw = candidate.statusCode ?? candidate.status ?? candidate.response?.status
    const parsed = Number(raw)
    if (Number.isFinite(parsed) && parsed > 0) {
      return parsed
    }
  }

  return 500
}

function withLeadingSlash(path: string) {
  if (path.startsWith('/')) {
    return path
  }

  return `/${path}`
}

export function useApi() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)
  const DEFAULT_TIMEOUT_MS = 30_000

  async function apiFetch<T>(path: string, options: Parameters<typeof $fetch<T>>[1] = {}) {
    const normalizedPath = withLeadingSlash(path)
    // X-Account-Id NAO e setado aqui de proposito: o provider global injeta
    // (plugins/account-id-bridge.client.ts -> setApiAccountIdProvider). Setar de
    // novo criaria duas fontes de conta — o bug de project_account_source_divergence.
    // Authorization idem: createApiRequest ja aplica o accessToken.
    try {
      return (await apiRequest(`${OMNICHANNEL_API_PREFIX}${normalizedPath}`, {
        ...(options as Record<string, unknown>),
        timeout: options?.timeout ?? DEFAULT_TIMEOUT_MS,
      })) as T
    } catch (error: unknown) {
      throw new ApiClientError(toErrorMessage(error), {
        statusCode: toStatusCode(error),
        data:
          error && typeof error === 'object' && 'data' in error
            ? (error as { data?: unknown }).data
            : null,
      })
    }
  }

  return { apiFetch }
}
