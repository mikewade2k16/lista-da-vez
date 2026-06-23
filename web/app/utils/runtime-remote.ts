import { canViewConsultants } from '~/domain/utils/permissions'
import { createEmptyState } from '~/stores/dashboard/runtime/state'
import {
  applyRemoteStoreData,
  applySettingsBundleToRuntime,
  buildSettingsBundleFromState,
} from './runtime-remote-state'

// Re-export para compatibilidade: callers historicos (stores/operations,
// runtime-remote.test, etc.) importam estas funcoes de '~/utils/runtime-remote'.
// A logica de normalizacao/estado vive agora em runtime-remote-normalize.ts e
// runtime-remote-state.ts; este arquivo concentra fetch + hidratacao remota.
export { applyOperationSnapshotToState, applySettingsBundleToState } from './runtime-remote-state'
export { applyRemoteStoreData, applySettingsBundleToRuntime, buildSettingsBundleFromState }

const SETTINGS_LOAD_STATE_LOADED = 'loaded'
const SETTINGS_LOAD_STATE_DEGRADED = 'degraded'
// 'skipped': conta sem o modulo queue nao busca /v1/settings (evita o 403
// module_disabled e o aviso de modo degradado indevido). Estado neutro — NAO
// degradado: o painel usa o fallback de settings sem exibir aviso de erro.
const SETTINGS_LOAD_STATE_SKIPPED = 'skipped'

function extractRemoteErrorMessage(
  error,
  fallbackMessage = 'Nao foi possivel carregar as configuracoes.',
) {
  const dataMessage = String(error?.data?.message || error?.response?._data?.message || '').trim()
  const directMessage = String(error?.message || '').trim()
  return dataMessage || directMessage || fallbackMessage
}

function logSettingsDegraded(eventName, payload = {}) {
  if (import.meta.server) {
    return
  }

  console.warn('[runtime-settings]', {
    event: eventName,
    ...payload,
    recordedAt: new Date().toISOString(),
  })
}

function buildFallbackSettingsBundle(currentState, storeId, options = {}) {
  const normalizedStoreId = String(storeId || '').trim()
  const fallbackBundle = buildSettingsBundleFromState(createEmptyState(), normalizedStoreId)

  if (!options?.preserveExistingSettings) {
    return fallbackBundle
  }

  return {
    ...fallbackBundle,
    ...buildSettingsBundleFromState(currentState || {}, normalizedStoreId),
    storeId: normalizedStoreId || fallbackBundle.storeId,
  }
}

function withTenantQuery(path, tenantId) {
  const normalizedTenantId = String(tenantId || '').trim()

  if (!normalizedTenantId) {
    return path
  }

  const separator = path.includes('?') ? '&' : '?'
  return `${path}${separator}tenantId=${encodeURIComponent(normalizedTenantId)}`
}

function hasResolvedTenantId(value) {
  return String(value || '').trim().length > 0
}

function resolveTenantIdForStore(state, storeId, fallbackTenantId = '') {
  const normalizedFallback = String(fallbackTenantId || '').trim()

  if (normalizedFallback) {
    return normalizedFallback
  }

  const normalizedStoreId = String(storeId || '').trim()
  const store = (Array.isArray(state?.stores) ? state.stores : []).find(
    (item) => String(item?.id || '').trim() === normalizedStoreId,
  )

  return String(store?.tenantId || '').trim()
}

export async function refreshRuntimeStoreSettings(
  runtime,
  apiRequest,
  storeId,
  tenantId = '',
  options = {},
) {
  const normalizedStoreId = String(storeId || '').trim()

  if (!normalizedStoreId) {
    return null
  }

  // Conta sem o modulo queue: nao recarrega /v1/settings (evita 403 + aviso
  // degradado indevido). Estado neutro 'skipped'; applyRuntimeSettingsStatus nao
  // marca degradado. Default true preserva o refresh de quem tem queue.
  if (options?.canFetchQueueSettings === false) {
    return {
      storeId: normalizedStoreId,
      resolvedTenantId: String(tenantId || '').trim(),
      settingsBundle: null,
      settingsLoadState: SETTINGS_LOAD_STATE_SKIPPED,
      settingsErrorMessage: '',
    }
  }

  await runtime.ensure()
  const resolvedTenantId = resolveTenantIdForStore(runtime.state, normalizedStoreId, tenantId)

  if (!hasResolvedTenantId(resolvedTenantId)) {
    const settingsErrorMessage = 'Tenant ativo nao resolvido para recarregar configuracoes.'
    logSettingsDegraded('refresh-skipped-missing-tenant', {
      storeId: normalizedStoreId,
      tenantId: resolvedTenantId,
      message: settingsErrorMessage,
    })

    return {
      storeId: normalizedStoreId,
      resolvedTenantId,
      settingsBundle: null,
      settingsLoadState: SETTINGS_LOAD_STATE_DEGRADED,
      settingsErrorMessage,
    }
  }

  try {
    const settingsBundle = await apiRequest(withTenantQuery('/v1/settings', resolvedTenantId))
    applySettingsBundleToRuntime(runtime, normalizedStoreId, settingsBundle)

    return {
      storeId: normalizedStoreId,
      resolvedTenantId,
      settingsBundle,
      settingsLoadState: SETTINGS_LOAD_STATE_LOADED,
      settingsErrorMessage: '',
    }
  } catch (error) {
    const settingsErrorMessage = extractRemoteErrorMessage(error)
    logSettingsDegraded('refresh-degraded', {
      storeId: normalizedStoreId,
      tenantId: resolvedTenantId,
      message: settingsErrorMessage,
    })

    return {
      storeId: normalizedStoreId,
      resolvedTenantId,
      settingsBundle: null,
      settingsLoadState: SETTINGS_LOAD_STATE_DEGRADED,
      settingsErrorMessage,
    }
  }
}

function getApiErrorStatusCode(error) {
  const directStatus = Number(error?.statusCode ?? error?.status ?? error?.response?.status)

  return Number.isFinite(directStatus) ? directStatus : 0
}

// Loga (sem derrubar o boot) quando consultores ou snapshot degradam. Ambos sao
// best-effort: qualquer erro (403 sem permissao/modulo, 400 loja stale, 5xx por
// schema stale na VPS, rede) degrada para vazio e o login completa. So registra
// para diagnostico — nunca re-lanca.
function logRemoteDataDegraded(eventName, error, payload = {}) {
  if (import.meta.server) {
    return
  }

  console.warn('[runtime-remote]', {
    event: eventName,
    status: getApiErrorStatusCode(error),
    message: extractRemoteErrorMessage(error, ''),
    ...payload,
    recordedAt: new Date().toISOString(),
  })
}

function resolveRuntimeRole(currentState) {
  const activeProfileId = String(currentState?.activeProfileId || '').trim()
  const profiles = Array.isArray(currentState?.profiles) ? currentState.profiles : []
  const activeProfile = profiles.find(
    (profile) => String(profile?.id || '').trim() === activeProfileId,
  )

  return String(activeProfile?.role || '').trim()
}

function resolveCanFetchConsultants(currentState, options = {}) {
  if (typeof options?.canViewConsultants === 'boolean') {
    return options.canViewConsultants
  }

  return canViewConsultants(
    options?.role || resolveRuntimeRole(currentState),
    options?.permissionKeys || [],
    Boolean(options?.permissionsResolved),
  )
}

export async function fetchRemoteStoreData(apiRequest, storeId, tenantId = '', options = {}) {
  const normalizedStoreId = String(storeId || '').trim()
  const storeQuery = encodeURIComponent(normalizedStoreId)
  const normalizedTenantId = String(tenantId || '').trim()
  // /v1/settings, /v1/consultants e /v1/operations/snapshot pertencem ao modulo
  // fila/operacao (queue). Conta SEM o modulo NAO dispara NENHUM dos tres: evita
  // os 403 module_disabled poluindo o console (ex.: conta so de cardapio/bio).
  // Default true preserva o comportamento de quem tem queue. Consultants exige
  // ainda a permissao consultor.view (canFetchConsultants) por cima do modulo.
  const hasQueueModule = options?.canFetchQueueSettings !== false
  const shouldFetchSettings = hasQueueModule
  const shouldFetchConsultants = hasQueueModule && options?.canFetchConsultants !== false
  const shouldFetchSnapshot = hasQueueModule
  const requestResults = await Promise.allSettled([
    !shouldFetchSettings
      ? Promise.resolve(null)
      : hasResolvedTenantId(normalizedTenantId)
        ? apiRequest(withTenantQuery('/v1/settings', normalizedTenantId))
        : Promise.reject(new Error('Tenant ativo nao resolvido para carregar configuracoes.')),
    shouldFetchConsultants
      ? apiRequest(`/v1/consultants?storeId=${storeQuery}`)
      : Promise.resolve({ consultants: [] }),
    shouldFetchSnapshot
      ? apiRequest(`/v1/operations/snapshot?storeId=${storeQuery}`)
      : Promise.resolve(null),
  ])
  const [settingsResult, consultantsResult, operationsSnapshotResult] = requestResults

  // O roster de gestao (/v1/consultants) e best-effort e NUNCA derruba o login.
  // Qualquer erro degrada para roster vazio — o snapshot da operacao ainda entrega
  // o roster enxuto (id/nome/iniciais/cor). Antes, so 403 degradava: um 500
  // ("Erro ao processar o consultor", ex.: core.users sem employee_code numa VPS
  // de schema stale) era re-lancado, o catch do login() limpava a sessao e
  // travava o usuario na tela de login.
  const consultantsLoadState = !shouldFetchConsultants
    ? 'skipped'
    : consultantsResult.status === 'rejected'
      ? 'degraded'
      : 'loaded'

  if (consultantsResult.status === 'rejected') {
    logRemoteDataDegraded('consultants-degraded', consultantsResult.reason, {
      storeId: normalizedStoreId,
      tenantId: normalizedTenantId,
    })
  }

  // operations/snapshot e o dado central da operacao, mas NAO pode derrubar o
  // login. Qualquer erro degrada para snapshot vazio (applyRemoteStoreData tolera
  // null) e o login completa; a tela re-resolve a loja correta depois. Cobre:
  // account sem o modulo queue (403 module_disabled), papel sem escopo na loja
  // (403 forbidden), loja stale de sessao anterior (400 validation), schema stale
  // na VPS (500) e falha de rede. Antes, so 400/403 degradavam e um 500 jogava o
  // usuario de volta pro login.
  if (operationsSnapshotResult.status === 'rejected') {
    logRemoteDataDegraded('snapshot-degraded', operationsSnapshotResult.reason, {
      storeId: normalizedStoreId,
      tenantId: normalizedTenantId,
    })
  }

  const operationsSnapshot =
    operationsSnapshotResult.status === 'fulfilled' ? operationsSnapshotResult.value : null
  const operationsSnapshotLoadState = !shouldFetchSnapshot
    ? 'skipped'
    : operationsSnapshotResult.status === 'fulfilled'
      ? 'loaded'
      : 'degraded'

  // Resolve o estado dos settings preservando o narrowing do PromiseSettledResult
  // (guarda direta por settingsResult.status). O 'skipped' (conta sem queue, que
  // nem disparou a request) e sobreposto depois para nao perder o narrowing.
  const settingsBundle =
    !shouldFetchSettings || settingsResult.status !== 'fulfilled' ? null : settingsResult.value
  const settingsErrorMessage =
    shouldFetchSettings && settingsResult.status === 'rejected'
      ? extractRemoteErrorMessage(settingsResult.reason)
      : ''
  const settingsLoadState = !shouldFetchSettings
    ? SETTINGS_LOAD_STATE_SKIPPED
    : settingsResult.status === 'fulfilled'
      ? SETTINGS_LOAD_STATE_LOADED
      : SETTINGS_LOAD_STATE_DEGRADED

  if (settingsLoadState === SETTINGS_LOAD_STATE_DEGRADED) {
    logSettingsDegraded('bootstrap-degraded', {
      storeId: normalizedStoreId,
      tenantId: normalizedTenantId,
      message: settingsErrorMessage,
    })
  }

  return {
    storeId: normalizedStoreId,
    resolvedTenantId: normalizedTenantId,
    settingsBundle,
    consultants:
      consultantsResult.status === 'fulfilled' &&
      Array.isArray(consultantsResult.value?.consultants)
        ? consultantsResult.value.consultants
        : [],
    operationsSnapshot,
    settingsLoadState,
    settingsErrorMessage,
    consultantsLoadState,
    operationsSnapshotLoadState,
  }
}

export async function hydrateRuntimeStoreContext(
  runtime,
  apiRequest,
  storeId,
  tenantId = '',
  options = {},
) {
  const normalizedStoreID = String(storeId || '').trim()

  if (!normalizedStoreID) {
    return null
  }

  await runtime.ensure()

  const resolvedTenantId = resolveTenantIdForStore(runtime.state, normalizedStoreID, tenantId)
  const remoteData = await fetchRemoteStoreData(apiRequest, normalizedStoreID, resolvedTenantId, {
    canFetchConsultants: resolveCanFetchConsultants(runtime.state, options),
    // Default true: so PULA /v1/settings quando o caller marca explicitamente que
    // a conta nao tem o modulo queue. Quem tem queue mantem o fluxo de hoje.
    canFetchQueueSettings: options?.canFetchQueueSettings !== false,
  })
  const settingsBundle =
    remoteData.settingsLoadState === SETTINGS_LOAD_STATE_LOADED
      ? remoteData.settingsBundle
      : buildFallbackSettingsBundle(runtime.state, normalizedStoreID, {
          preserveExistingSettings: Boolean(options?.preserveExistingSettings ?? true),
        })
  runtime.hydrate(
    applyRemoteStoreData(
      runtime.state,
      normalizedStoreID,
      settingsBundle,
      remoteData.consultants,
      remoteData.operationsSnapshot,
    ),
  )

  return {
    ...remoteData,
    settingsBundle,
  }
}
