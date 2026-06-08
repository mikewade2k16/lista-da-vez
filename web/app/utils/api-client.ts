export const AUTH_TOKEN_COOKIE = 'ldv_access_token'

export function getApiErrorMessage(error, fallbackMessage) {
  const baseMessage = error?.data?.error?.message || error?.message || fallbackMessage
  const detailCause = String(error?.data?.error?.details?.cause || '').trim()

  if (!detailCause) {
    return baseMessage
  }

  return `${baseMessage} (${detailCause})`
}

export function getApiBase(runtimeConfig) {
  if (import.meta.server) {
    return runtimeConfig.apiInternalBase || runtimeConfig.public.apiBase
  }

  return runtimeConfig.public.apiBase
}

export function getWebSocketBase(runtimeConfig) {
  const configuredBase = String(runtimeConfig.public.apiWsBase || '').trim()
  const baseURL = configuredBase || getApiBase(runtimeConfig)
  const url = new URL(baseURL)

  if (url.protocol === 'http:') {
    url.protocol = 'ws:'
  } else if (url.protocol === 'https:') {
    url.protocol = 'wss:'
  }

  return url.toString()
}

// Limiar em ms a partir do qual uma requisicao aciona o loading global.
// Requests mais curtos nao ativam o overlay para evitar flicker.
const LOADING_THRESHOLD_MS = 200
const inflightGetRequests = new Map<string, Promise<unknown>>()

function dedupeHeadersKey(headers) {
  return JSON.stringify(
    Object.keys(headers)
      .sort()
      .map((key) => [key.toLowerCase(), String(headers[key] || '')]),
  )
}

function stableValue(value) {
  if (value instanceof Date) {
    return value.toISOString()
  }

  if (Array.isArray(value)) {
    return value.map(stableValue)
  }

  if (value && typeof value === 'object') {
    return Object.keys(value)
      .sort()
      .reduce((acc, key) => {
        acc[key] = stableValue(value[key])
        return acc
      }, {})
  }

  return value
}

function queryValueToString(value) {
  if (value === null) {
    return ''
  }

  if (typeof value === 'object') {
    return JSON.stringify(stableValue(value))
  }

  return String(value)
}

function appendQueryValue(params, key, value) {
  if (!key || value === undefined) {
    return
  }

  if (Array.isArray(value)) {
    value.forEach((item) => appendQueryValue(params, key, item))
    return
  }

  params.append(key, queryValueToString(value))
}

function appendQueryInput(params, input) {
  if (!input) {
    return
  }

  if (input instanceof URLSearchParams) {
    input.forEach((value, key) => appendQueryValue(params, key, value))
    return
  }

  if (typeof input === 'string') {
    appendQueryInput(params, new URLSearchParams(input))
    return
  }

  if (Array.isArray(input)) {
    input.forEach(([key, value]) => appendQueryValue(params, String(key || ''), value))
    return
  }

  if (typeof input === 'object') {
    Object.keys(input)
      .sort()
      .forEach((key) => appendQueryValue(params, key, input[key]))
  }
}

function stableSearchParams(params) {
  const sortedParams = new URLSearchParams()
  Array.from(params.entries())
    .sort(([leftKey, leftValue], [rightKey, rightValue]) => {
      if (leftKey === rightKey) {
        return leftValue.localeCompare(rightValue)
      }

      return leftKey.localeCompare(rightKey)
    })
    .forEach(([key, value]) => sortedParams.append(key, value))

  return sortedParams.toString()
}

function dedupePathKey(path, options) {
  const pathString = String(path || '')
  const fallbackOrigin = 'http://dedupe.local'

  try {
    const url = new URL(pathString, fallbackOrigin)
    const params = new URLSearchParams(url.search)
    appendQueryInput(params, options.query)
    appendQueryInput(params, options.params)

    const queryString = stableSearchParams(params)
    const isAbsoluteURL = /^[a-z][a-z\d+\-.]*:\/\//i.test(pathString)
    const normalizedPath = `${isAbsoluteURL ? url.origin : ''}${url.pathname}`

    return `${normalizedPath}${queryString ? `?${queryString}` : ''}${url.hash}`
  } catch {
    const params = new URLSearchParams()
    appendQueryInput(params, options.query)
    appendQueryInput(params, options.params)
    const queryString = stableSearchParams(params)

    return `${pathString}${queryString ? `?${queryString}` : ''}`
  }
}

// Hooks de loading global. Sao injetados pelo plugin client-only
// `web/app/plugins/loading-bridge.client.ts` que liga o store
// `core/loading` (do layer core) a este api-client. Mantemos esse contrato
// de hooks para evitar import direto do store aqui (dependencia circular
// com stores que usam o api-client durante o setup do pinia).
let loadingHooks: { push: () => void; pop: () => void } | null = null

export function setApiLoadingHooks(hooks: { push: () => void; pop: () => void } | null) {
  loadingHooks = hooks
}

// Provider do X-Account-Id global. Injetado pelo plugin client-only
// `account-id-bridge.client.ts`, que liga o `core/account.activeAccountId` a este
// api-client sem import direto do store aqui (mesma razao do loadingHooks: evitar
// dependencia circular com stores que usam o api-client no setup do pinia).
let accountIdProvider: (() => string) | null = null

export function setApiAccountIdProvider(provider: (() => string) | null) {
  accountIdProvider = provider
}

export function createApiRequest(runtimeConfig, getAccessToken = null) {
  return function apiRequest(path, options = {}) {
    const headers = {
      ...(options.headers || {}),
    }
    const normalizedMethod = String(options.method || 'GET').toUpperCase()
    const accessToken = typeof getAccessToken === 'function' ? getAccessToken() : getAccessToken

    if (accessToken) {
      headers.Authorization = `Bearer ${accessToken}`
    }

    // X-Account-Id global: toda rota multi-tenant (queue/crm/tasks/...) exige o
    // header e o backend gateia via RequireModuleByPath. Injetado a partir do
    // provider (activeAccountId; fallback temporario para activeTenantId no boot)
    // sem cada call-site precisar passar.
    // Preserva quem ja envia manualmente (ex.: upload de video, managers do site).
    const accountId = typeof accountIdProvider === 'function' ? accountIdProvider() : ''
    if (accountId && !headers['X-Account-Id']) {
      headers['X-Account-Id'] = accountId
    }

    const processedOptions = { ...options }
    delete processedOptions.dedupe

    if (
      ['POST', 'PUT', 'PATCH', 'DELETE'].includes(normalizedMethod) &&
      options.body &&
      typeof options.body === 'object' &&
      !(options.body instanceof FormData) &&
      !(options.body instanceof Blob) &&
      !(options.body instanceof ArrayBuffer)
    ) {
      processedOptions.body = JSON.stringify(options.body)
      if (!headers['Content-Type']) {
        headers['Content-Type'] = 'application/json'
      }
    }

    const baseURL = getApiBase(runtimeConfig)
    const requestKey =
      normalizedMethod === 'GET' && options.dedupe !== false
        ? [normalizedMethod, baseURL, dedupePathKey(path, options), dedupeHeadersKey(headers)].join(
            '|',
          )
        : ''

    if (requestKey && inflightGetRequests.has(requestKey)) {
      return inflightGetRequests.get(requestKey)
    }

    const fetchPromise = $fetch(path, {
      baseURL,
      ...processedOptions,
      headers,
    })

    if (requestKey) {
      const trackedPromise = fetchPromise.finally(() => {
        inflightGetRequests.delete(requestKey)
      })
      inflightGetRequests.set(requestKey, trackedPromise)
    }

    // Fase 9A — feedback visual: se a requisicao passar de LOADING_THRESHOLD_MS,
    // ativa o loading global (barra fina no topo). Curtas nao acionam para
    // evitar flicker em chamadas rapidas. Hooks injetados pelo plugin
    // loading-bridge.client.ts (so existe no client; SSR ignora).
    if (loadingHooks && options.skipLoadingIndicator !== true) {
      let pushed = false
      const timer = setTimeout(() => {
        loadingHooks?.push()
        pushed = true
      }, LOADING_THRESHOLD_MS)

      fetchPromise.finally(() => {
        clearTimeout(timer)
        if (pushed) {
          loadingHooks?.pop()
        }
      })
    }

    return requestKey ? inflightGetRequests.get(requestKey) || fetchPromise : fetchPromise
  }
}
