import {
  createEmptyStoreScopedState,
  extractStoreScopedState,
  normalizeAppearanceState,
  normalizeStoreScopedState,
} from '~/stores/dashboard/runtime/state'
import {
  cloneOrFallback,
  normalizeConsultants,
  normalizeOperationSnapshot,
  normalizeOptions,
  normalizeProducts,
  resolveOperationRoster,
} from './runtime-remote-normalize'

// Transformacoes que aplicam o payload remoto (settings/consultants/snapshot)
// sobre o estado do runtime store, sempre devolvendo estado novo (imutavel).
// Extraido de runtime-remote.ts para manter cada arquivo dentro do limite de
// linhas (ver principios de engenharia).

// Opcoes para applyOperationSnapshotToState.
// resetFinishModal: true limpa o modal de encerrar ao aplicar o snapshot.
interface ApplySnapshotOptions {
  resetFinishModal?: boolean
}

function resolveSelectedConsultantId(currentState, storeId, roster) {
  const currentSnapshot = currentState.storeSnapshots?.[storeId] || {}
  const preferredId =
    storeId === currentState.activeStoreId
      ? currentState.selectedConsultantId
      : currentSnapshot.selectedConsultantId

  if (roster.some((consultant) => consultant.id === preferredId)) {
    return preferredId
  }

  return roster[0]?.id || null
}

function buildStoreSnapshot(currentState, storeId, roster) {
  const currentSnapshot = cloneOrFallback(currentState.storeSnapshots?.[storeId], {})

  return {
    ...currentSnapshot,
    roster,
    selectedConsultantId: resolveSelectedConsultantId(currentState, storeId, roster),
  }
}

export function applyOperationSnapshotToState(
  currentState,
  storeId,
  operationSnapshot,
  options: ApplySnapshotOptions = {},
) {
  const normalizedStoreId = String(storeId || '').trim()

  if (!normalizedStoreId) {
    return cloneOrFallback(currentState, {})
  }

  const storeDescriptor =
    (Array.isArray(currentState?.stores) ? currentState.stores : []).find(
      (store) => store.id === normalizedStoreId,
    ) || null
  const activeScopedState =
    normalizedStoreId === currentState?.activeStoreId
      ? extractStoreScopedState(currentState || {})
      : cloneOrFallback(currentState?.storeSnapshots?.[normalizedStoreId], {})
  const existingRoster =
    Array.isArray(activeScopedState?.roster) && activeScopedState.roster.length
      ? activeScopedState.roster
      : normalizedStoreId === currentState?.activeStoreId
        ? cloneOrFallback(currentState?.roster, [])
        : []
  // Sem roster de gestao (papel operador sem consultor.view), usa o roster
  // enxuto que vem dentro do snapshot da operacao para manter a faixa viva.
  const roster = existingRoster.length
    ? existingRoster
    : normalizeConsultants(operationSnapshot?.roster)
  const fallbackScopedState = normalizeStoreScopedState(
    {
      ...cloneOrFallback(activeScopedState, {}),
      roster,
    },
    createEmptyStoreScopedState(roster),
    storeDescriptor,
    Date.now(),
  )
  const nextScopedState = normalizeStoreScopedState(
    {
      ...cloneOrFallback(fallbackScopedState, {}),
      ...normalizeOperationSnapshot(operationSnapshot),
      roster,
    },
    fallbackScopedState,
    storeDescriptor,
    Date.now(),
  )
  const nextScopedStateWithMetadata = {
    ...nextScopedState,
    _operationSnapshotFetchedAt: Date.now(),
  }

  return {
    ...cloneOrFallback(currentState, {}),
    storeSnapshots: {
      ...cloneOrFallback(currentState?.storeSnapshots, {}),
      [normalizedStoreId]: nextScopedStateWithMetadata,
    },
    ...(normalizedStoreId === currentState?.activeStoreId ? nextScopedStateWithMetadata : {}),
    ...(options?.resetFinishModal
      ? {
          finishModalServiceId: null,
          finishModalDraft: null,
        }
      : {}),
  }
}

export function applyRemoteStoreData(
  currentState,
  storeId,
  settingsBundle,
  consultants,
  operationSnapshot = null,
) {
  const roster = resolveOperationRoster(consultants, operationSnapshot)
  const storeDescriptor =
    (Array.isArray(currentState?.stores) ? currentState.stores : []).find(
      (store) => store.id === storeId,
    ) || null
  const nextSnapshot = normalizeStoreScopedState(
    {
      ...buildStoreSnapshot(currentState, storeId, roster),
      // So sobrescreve as listas operacionais quando um snapshot REAL veio da API.
      // operationSnapshot=null significa "NAO BUSQUEI" (conta agencia/sem modulo
      // queue pulam o fetch em canFetchQueueSettings; degradacao por erro tambem
      // cai aqui) — nesses casos PRESERVA o que ja esta carregado. Antes,
      // normalizeOperationSnapshot(null) espalhava listas VAZIAS por cima do
      // snapshot existente: um context.updated qualquer (ex.: alerta materializado
      // no mesmo tick do grace do auto-encerramento) re-rodava syncRuntimeAccess e
      // ZERAVA fila/atendimentos/historico/pendencias do board sem nenhuma request.
      ...(operationSnapshot ? normalizeOperationSnapshot(operationSnapshot) : {}),
      roster,
    },
    createEmptyStoreScopedState(roster),
    storeDescriptor,
    Date.now(),
  )
  const nextSnapshotWithMetadata = {
    ...nextSnapshot,
    // fetchedAt so avanca quando houve fetch de verdade; sem snapshot, preserva o
    // carimbo anterior (o trust do board nao pode ficar "fresco" sem dado novo).
    _operationSnapshotFetchedAt: operationSnapshot
      ? Date.now()
      : Number(currentState?.storeSnapshots?.[storeId]?._operationSnapshotFetchedAt || 0),
  }

  return {
    ...cloneOrFallback(currentState, {}),
    ...nextSnapshotWithMetadata,
    activeStoreId: storeId,
    storeSnapshots: {
      ...cloneOrFallback(currentState.storeSnapshots, {}),
      [storeId]: nextSnapshotWithMetadata,
    },
    operationTemplates: Array.isArray(settingsBundle?.operationTemplates)
      ? cloneOrFallback(settingsBundle.operationTemplates, [])
      : cloneOrFallback(currentState.operationTemplates, []),
    selectedOperationTemplateId: String(
      settingsBundle?.selectedOperationTemplateId || currentState.selectedOperationTemplateId || '',
    ).trim(),
    settings: settingsBundle?.settings
      ? cloneOrFallback(settingsBundle.settings, {})
      : cloneOrFallback(currentState.settings, {}),
    appearance: normalizeAppearanceState(settingsBundle?.appearance, currentState?.appearance),
    modalConfig: {
      ...cloneOrFallback(currentState.modalConfig, {}),
      ...cloneOrFallback(settingsBundle?.modalConfig, {}),
    },
    visitReasonOptions: Array.isArray(settingsBundle?.visitReasonOptions)
      ? normalizeOptions(settingsBundle.visitReasonOptions)
      : cloneOrFallback(currentState.visitReasonOptions, []),
    customerSourceOptions: Array.isArray(settingsBundle?.customerSourceOptions)
      ? normalizeOptions(settingsBundle.customerSourceOptions)
      : cloneOrFallback(currentState.customerSourceOptions, []),
    pauseReasonOptions:
      Array.isArray(settingsBundle?.pauseReasonOptions) && settingsBundle.pauseReasonOptions.length
        ? normalizeOptions(settingsBundle.pauseReasonOptions)
        : cloneOrFallback(currentState.pauseReasonOptions, []),
    cancelReasonOptions: Array.isArray(settingsBundle?.cancelReasonOptions)
      ? normalizeOptions(settingsBundle.cancelReasonOptions)
      : cloneOrFallback(currentState.cancelReasonOptions, []),
    stopReasonOptions: Array.isArray(settingsBundle?.stopReasonOptions)
      ? normalizeOptions(settingsBundle.stopReasonOptions)
      : cloneOrFallback(currentState.stopReasonOptions, []),
    queueJumpReasonOptions: Array.isArray(settingsBundle?.queueJumpReasonOptions)
      ? normalizeOptions(settingsBundle.queueJumpReasonOptions)
      : cloneOrFallback(currentState.queueJumpReasonOptions, []),
    lossReasonOptions: Array.isArray(settingsBundle?.lossReasonOptions)
      ? normalizeOptions(settingsBundle.lossReasonOptions)
      : cloneOrFallback(currentState.lossReasonOptions, []),
    professionOptions: Array.isArray(settingsBundle?.professionOptions)
      ? normalizeOptions(settingsBundle.professionOptions)
      : cloneOrFallback(currentState.professionOptions, []),
    productCatalog: Array.isArray(settingsBundle?.productCatalog)
      ? normalizeProducts(settingsBundle.productCatalog)
      : cloneOrFallback(currentState.productCatalog, []),
  }
}

export function applySettingsBundleToState(currentState, storeId, settingsBundle) {
  const normalizedStoreId = String(storeId || '').trim()

  return {
    ...cloneOrFallback(currentState, {}),
    activeStoreId: normalizedStoreId || currentState?.activeStoreId,
    operationTemplates: Array.isArray(settingsBundle?.operationTemplates)
      ? cloneOrFallback(settingsBundle.operationTemplates, [])
      : cloneOrFallback(currentState.operationTemplates, []),
    selectedOperationTemplateId: String(
      settingsBundle?.selectedOperationTemplateId || currentState.selectedOperationTemplateId || '',
    ).trim(),
    settings: settingsBundle?.settings
      ? cloneOrFallback(settingsBundle.settings, {})
      : cloneOrFallback(currentState.settings, {}),
    appearance: normalizeAppearanceState(settingsBundle?.appearance, currentState?.appearance),
    modalConfig: {
      ...cloneOrFallback(currentState.modalConfig, {}),
      ...cloneOrFallback(settingsBundle?.modalConfig, {}),
    },
    visitReasonOptions: Array.isArray(settingsBundle?.visitReasonOptions)
      ? normalizeOptions(settingsBundle.visitReasonOptions)
      : cloneOrFallback(currentState.visitReasonOptions, []),
    customerSourceOptions: Array.isArray(settingsBundle?.customerSourceOptions)
      ? normalizeOptions(settingsBundle.customerSourceOptions)
      : cloneOrFallback(currentState.customerSourceOptions, []),
    pauseReasonOptions:
      Array.isArray(settingsBundle?.pauseReasonOptions) && settingsBundle.pauseReasonOptions.length
        ? normalizeOptions(settingsBundle.pauseReasonOptions)
        : cloneOrFallback(currentState.pauseReasonOptions, []),
    cancelReasonOptions: Array.isArray(settingsBundle?.cancelReasonOptions)
      ? normalizeOptions(settingsBundle.cancelReasonOptions)
      : cloneOrFallback(currentState.cancelReasonOptions, []),
    stopReasonOptions: Array.isArray(settingsBundle?.stopReasonOptions)
      ? normalizeOptions(settingsBundle.stopReasonOptions)
      : cloneOrFallback(currentState.stopReasonOptions, []),
    queueJumpReasonOptions: Array.isArray(settingsBundle?.queueJumpReasonOptions)
      ? normalizeOptions(settingsBundle.queueJumpReasonOptions)
      : cloneOrFallback(currentState.queueJumpReasonOptions, []),
    lossReasonOptions: Array.isArray(settingsBundle?.lossReasonOptions)
      ? normalizeOptions(settingsBundle.lossReasonOptions)
      : cloneOrFallback(currentState.lossReasonOptions, []),
    professionOptions: Array.isArray(settingsBundle?.professionOptions)
      ? normalizeOptions(settingsBundle.professionOptions)
      : cloneOrFallback(currentState.professionOptions, []),
    productCatalog: Array.isArray(settingsBundle?.productCatalog)
      ? normalizeProducts(settingsBundle.productCatalog)
      : cloneOrFallback(currentState.productCatalog, []),
  }
}

export function applySettingsBundleToRuntime(runtime, storeId, settingsBundle) {
  runtime.replace(applySettingsBundleToState(runtime.state, storeId, settingsBundle))
  return runtime.state
}

export function buildSettingsBundleFromState(state, storeId) {
  return {
    storeId,
    operationTemplates: cloneOrFallback(state.operationTemplates, []),
    selectedOperationTemplateId: String(state.selectedOperationTemplateId || '').trim(),
    settings: cloneOrFallback(state.settings, {}),
    appearance: normalizeAppearanceState(state.appearance),
    modalConfig: cloneOrFallback(state.modalConfig, {}),
    visitReasonOptions: cloneOrFallback(state.visitReasonOptions, []),
    customerSourceOptions: cloneOrFallback(state.customerSourceOptions, []),
    pauseReasonOptions: cloneOrFallback(state.pauseReasonOptions, []),
    cancelReasonOptions: cloneOrFallback(state.cancelReasonOptions, []),
    stopReasonOptions: cloneOrFallback(state.stopReasonOptions, []),
    queueJumpReasonOptions: cloneOrFallback(state.queueJumpReasonOptions, []),
    lossReasonOptions: cloneOrFallback(state.lossReasonOptions, []),
    professionOptions: cloneOrFallback(state.professionOptions, []),
    productCatalog: cloneOrFallback(state.productCatalog, []),
  }
}
