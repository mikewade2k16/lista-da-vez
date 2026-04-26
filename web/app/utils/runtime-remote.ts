import { cloneValue } from "~/domain/utils/object";
import {
  createEmptyStoreScopedState,
  extractStoreScopedState,
  normalizeStoreScopedState
} from "~/stores/dashboard/runtime/state";

function cloneOrFallback(value, fallback) {
  return cloneValue(value === undefined ? fallback : value);
}

function normalizeOptions(options = []) {
  return (Array.isArray(options) ? options : []).map((option) => ({
    id: String(option?.id || "").trim(),
    label: String(option?.label || "").trim()
  })).filter((option) => option.id && option.label);
}

function normalizeProducts(products = []) {
  return (Array.isArray(products) ? products : []).map((product) => ({
    id: String(product?.id || "").trim(),
    name: String(product?.name || "").trim(),
    code: String(product?.code || "").trim().toUpperCase(),
    category: String(product?.category || "").trim(),
    basePrice: Math.max(0, Number(product?.basePrice || 0) || 0)
  })).filter((product) => product.id && product.name);
}

function withTenantQuery(path, tenantId) {
  const normalizedTenantId = String(tenantId || "").trim();

  if (!normalizedTenantId) {
    return path;
  }

  const separator = path.includes("?") ? "&" : "?";
  return `${path}${separator}tenantId=${encodeURIComponent(normalizedTenantId)}`;
}

function resolveTenantIdForStore(state, storeId, fallbackTenantId = "") {
  const normalizedFallback = String(fallbackTenantId || "").trim();

  if (normalizedFallback) {
    return normalizedFallback;
  }

  const normalizedStoreId = String(storeId || "").trim();
  const store = (Array.isArray(state?.stores) ? state.stores : [])
    .find((item) => String(item?.id || "").trim() === normalizedStoreId);

  return String(store?.tenantId || "").trim();
}

function normalizeConsultants(consultants = []) {
  return (Array.isArray(consultants) ? consultants : []).map((consultant) => ({
    id: String(consultant?.id || "").trim(),
    storeId: String(consultant?.storeId || "").trim(),
    name: String(consultant?.name || "").trim(),
    role: String(consultant?.role || "").trim() || "Atendimento",
    initials: String(consultant?.initials || "").trim(),
    color: String(consultant?.color || "").trim() || "#168aad",
    monthlyGoal: Math.max(0, Number(consultant?.monthlyGoal || 0) || 0),
    commissionRate: Math.max(0, Number(consultant?.commissionRate || 0) || 0),
    conversionGoal: Math.max(0, Number(consultant?.conversionGoal || 0) || 0),
    avgTicketGoal: Math.max(0, Number(consultant?.avgTicketGoal || 0) || 0),
    paGoal: Math.max(0, Number(consultant?.paGoal || 0) || 0),
    active: Boolean(consultant?.active ?? true),
    access: consultant?.access && typeof consultant.access === "object"
      ? {
          userId: String(consultant.access?.userId || "").trim(),
          email: String(consultant.access?.email || "").trim().toLowerCase(),
          active: Boolean(consultant.access?.active ?? false)
        }
      : null
  })).filter((consultant) => consultant.id && consultant.name);
}

function resolveSelectedConsultantId(currentState, storeId, roster) {
  const currentSnapshot = currentState.storeSnapshots?.[storeId] || {};
  const preferredId =
    storeId === currentState.activeStoreId
      ? currentState.selectedConsultantId
      : currentSnapshot.selectedConsultantId;

  if (roster.some((consultant) => consultant.id === preferredId)) {
    return preferredId;
  }

  return roster[0]?.id || null;
}

function buildStoreSnapshot(currentState, storeId, roster) {
  const currentSnapshot = cloneOrFallback(currentState.storeSnapshots?.[storeId], {});

  return {
    ...currentSnapshot,
    roster,
    selectedConsultantId: resolveSelectedConsultantId(currentState, storeId, roster)
  };
}

function normalizeOperationSnapshot(snapshot = {}) {
  return {
    waitingList: Array.isArray(snapshot?.waitingList)
      ? snapshot.waitingList.map((item) => ({
          ...item,
          queueJoinedAt: Math.max(0, Number(item?.queueJoinedAt || 0) || 0)
        }))
      : [],
    activeServices: Array.isArray(snapshot?.activeServices)
      ? snapshot.activeServices.map((item) => ({
          ...item,
          serviceStartedAt: Math.max(0, Number(item?.serviceStartedAt || 0) || 0),
          queueJoinedAt: Math.max(0, Number(item?.queueJoinedAt || 0) || 0),
          queueWaitMs: Math.max(0, Number(item?.queueWaitMs || 0) || 0),
          queuePositionAtStart: Math.max(1, Number(item?.queuePositionAtStart || 1) || 1),
          skippedPeople: Array.isArray(item?.skippedPeople) ? item.skippedPeople : []
        }))
      : [],
    pausedEmployees: Array.isArray(snapshot?.pausedEmployees)
      ? snapshot.pausedEmployees.map((item) => ({
          personId: String(item?.personId || "").trim(),
          reason: String(item?.reason || "").trim(),
          kind: String(item?.kind || "pause").trim() || "pause",
          startedAt: Math.max(0, Number(item?.startedAt || 0) || 0)
        })).filter((item) => item.personId)
      : [],
    consultantActivitySessions: Array.isArray(snapshot?.consultantActivitySessions)
      ? snapshot.consultantActivitySessions.map((item) => ({
          personId: String(item?.personId || "").trim(),
          status: String(item?.status || "").trim(),
          startedAt: Math.max(0, Number(item?.startedAt || 0) || 0),
          endedAt: Math.max(0, Number(item?.endedAt || 0) || 0),
          durationMs: Math.max(0, Number(item?.durationMs || 0) || 0)
        })).filter((item) => item.personId)
      : [],
    consultantCurrentStatus:
      snapshot?.consultantCurrentStatus && typeof snapshot.consultantCurrentStatus === "object"
        ? Object.fromEntries(
            Object.entries(snapshot.consultantCurrentStatus).map(([consultantId, value]) => [
              String(consultantId || "").trim(),
              {
                status: String(value?.status || "").trim(),
                startedAt: Math.max(0, Number(value?.startedAt || 0) || 0)
              }
            ]).filter(([consultantId]) => consultantId)
          )
        : {},
    serviceHistory: Array.isArray(snapshot?.serviceHistory) ? snapshot.serviceHistory : []
  };
}

export function applyOperationSnapshotToState(currentState, storeId, operationSnapshot, options = {}) {
  const normalizedStoreId = String(storeId || "").trim();

  if (!normalizedStoreId) {
    return cloneOrFallback(currentState, {});
  }

  const storeDescriptor =
    (Array.isArray(currentState?.stores) ? currentState.stores : []).find((store) => store.id === normalizedStoreId) ||
    null;
  const activeScopedState =
    normalizedStoreId === currentState?.activeStoreId
      ? extractStoreScopedState(currentState || {})
      : cloneOrFallback(currentState?.storeSnapshots?.[normalizedStoreId], {});
  const roster =
    Array.isArray(activeScopedState?.roster) && activeScopedState.roster.length
      ? activeScopedState.roster
      : normalizedStoreId === currentState?.activeStoreId
        ? cloneOrFallback(currentState?.roster, [])
        : [];
  const fallbackScopedState = normalizeStoreScopedState(
    {
      ...cloneOrFallback(activeScopedState, {}),
      roster
    },
    createEmptyStoreScopedState(roster),
    storeDescriptor,
    Date.now()
  );
  const nextScopedState = normalizeStoreScopedState(
    {
      ...cloneOrFallback(fallbackScopedState, {}),
      ...normalizeOperationSnapshot(operationSnapshot),
      roster
    },
    fallbackScopedState,
    storeDescriptor,
    Date.now()
  );

  return {
    ...cloneOrFallback(currentState, {}),
    storeSnapshots: {
      ...cloneOrFallback(currentState?.storeSnapshots, {}),
      [normalizedStoreId]: nextScopedState
    },
    ...(normalizedStoreId === currentState?.activeStoreId ? nextScopedState : {}),
    ...(options?.resetFinishModal
      ? {
          finishModalPersonId: null,
          finishModalDraft: null
        }
      : {})
  };
}

export function applyRemoteStoreData(currentState, storeId, settingsBundle, consultants, operationSnapshot = null) {
  const roster = normalizeConsultants(consultants);
  const storeDescriptor =
    (Array.isArray(currentState?.stores) ? currentState.stores : []).find((store) => store.id === storeId) || null;
  const nextSnapshot = normalizeStoreScopedState(
    {
      ...buildStoreSnapshot(currentState, storeId, roster),
      ...normalizeOperationSnapshot(operationSnapshot),
      roster
    },
    createEmptyStoreScopedState(roster),
    storeDescriptor,
    Date.now()
  );

  return {
    ...cloneOrFallback(currentState, {}),
    activeStoreId: storeId,
    storeSnapshots: {
      ...cloneOrFallback(currentState.storeSnapshots, {}),
      [storeId]: nextSnapshot
    },
    operationTemplates: Array.isArray(settingsBundle?.operationTemplates)
      ? cloneOrFallback(settingsBundle.operationTemplates, [])
      : cloneOrFallback(currentState.operationTemplates, []),
    selectedOperationTemplateId:
      String(settingsBundle?.selectedOperationTemplateId || currentState.selectedOperationTemplateId || "").trim(),
    settings: settingsBundle?.settings
      ? cloneOrFallback(settingsBundle.settings, {})
      : cloneOrFallback(currentState.settings, {}),
    modalConfig: {
      ...cloneOrFallback(currentState.modalConfig, {}),
      ...cloneOrFallback(settingsBundle?.modalConfig, {})
    },
    visitReasonOptions: Array.isArray(settingsBundle?.visitReasonOptions)
      ? normalizeOptions(settingsBundle.visitReasonOptions)
      : cloneOrFallback(currentState.visitReasonOptions, []),
    customerSourceOptions: Array.isArray(settingsBundle?.customerSourceOptions)
      ? normalizeOptions(settingsBundle.customerSourceOptions)
      : cloneOrFallback(currentState.customerSourceOptions, []),
    pauseReasonOptions: Array.isArray(settingsBundle?.pauseReasonOptions) && settingsBundle.pauseReasonOptions.length
      ? normalizeOptions(settingsBundle.pauseReasonOptions)
      : cloneOrFallback(currentState.pauseReasonOptions, []),
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
      : cloneOrFallback(currentState.productCatalog, [])
  };
}

export function applySettingsBundleToState(currentState, storeId, settingsBundle) {
  const normalizedStoreId = String(storeId || "").trim();

  return {
    ...cloneOrFallback(currentState, {}),
    activeStoreId: normalizedStoreId || currentState?.activeStoreId,
    operationTemplates: Array.isArray(settingsBundle?.operationTemplates)
      ? cloneOrFallback(settingsBundle.operationTemplates, [])
      : cloneOrFallback(currentState.operationTemplates, []),
    selectedOperationTemplateId:
      String(settingsBundle?.selectedOperationTemplateId || currentState.selectedOperationTemplateId || "").trim(),
    settings: settingsBundle?.settings
      ? cloneOrFallback(settingsBundle.settings, {})
      : cloneOrFallback(currentState.settings, {}),
    modalConfig: {
      ...cloneOrFallback(currentState.modalConfig, {}),
      ...cloneOrFallback(settingsBundle?.modalConfig, {})
    },
    visitReasonOptions: Array.isArray(settingsBundle?.visitReasonOptions)
      ? normalizeOptions(settingsBundle.visitReasonOptions)
      : cloneOrFallback(currentState.visitReasonOptions, []),
    customerSourceOptions: Array.isArray(settingsBundle?.customerSourceOptions)
      ? normalizeOptions(settingsBundle.customerSourceOptions)
      : cloneOrFallback(currentState.customerSourceOptions, []),
    pauseReasonOptions: Array.isArray(settingsBundle?.pauseReasonOptions) && settingsBundle.pauseReasonOptions.length
      ? normalizeOptions(settingsBundle.pauseReasonOptions)
      : cloneOrFallback(currentState.pauseReasonOptions, []),
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
      : cloneOrFallback(currentState.productCatalog, [])
  };
}

export function applySettingsBundleToRuntime(runtime, storeId, settingsBundle) {
  runtime.replace(applySettingsBundleToState(runtime.state, storeId, settingsBundle));
  return runtime.state;
}

export async function refreshRuntimeStoreSettings(runtime, apiRequest, storeId, tenantId = "") {
  const normalizedStoreId = String(storeId || "").trim();

  if (!normalizedStoreId) {
    return null;
  }

  await runtime.ensure();
  const resolvedTenantId = resolveTenantIdForStore(runtime.state, normalizedStoreId, tenantId);
  const settingsBundle = await apiRequest(withTenantQuery("/v1/settings", resolvedTenantId));
  applySettingsBundleToRuntime(runtime, normalizedStoreId, settingsBundle);
  return settingsBundle;
}

export function buildSettingsBundleFromState(state, storeId) {
  return {
    storeId,
    selectedOperationTemplateId: String(state.selectedOperationTemplateId || "").trim(),
    settings: cloneOrFallback(state.settings, {}),
    modalConfig: cloneOrFallback(state.modalConfig, {}),
    visitReasonOptions: cloneOrFallback(state.visitReasonOptions, []),
    customerSourceOptions: cloneOrFallback(state.customerSourceOptions, []),
    pauseReasonOptions: cloneOrFallback(state.pauseReasonOptions, []),
    queueJumpReasonOptions: cloneOrFallback(state.queueJumpReasonOptions, []),
    lossReasonOptions: cloneOrFallback(state.lossReasonOptions, []),
    professionOptions: cloneOrFallback(state.professionOptions, []),
    productCatalog: cloneOrFallback(state.productCatalog, [])
  };
}

export async function fetchRemoteStoreData(apiRequest, storeId, tenantId = "") {
  const storeQuery = encodeURIComponent(String(storeId || "").trim());
  const [settingsBundle, consultantsResponse, operationsSnapshot] = await Promise.all([
    apiRequest(withTenantQuery("/v1/settings", tenantId)),
    apiRequest(`/v1/consultants?storeId=${storeQuery}`),
    apiRequest(`/v1/operations/snapshot?storeId=${storeQuery}`)
  ]);

  return {
    settingsBundle,
    consultants: Array.isArray(consultantsResponse?.consultants) ? consultantsResponse.consultants : [],
    operationsSnapshot
  };
}

export async function hydrateRuntimeStoreContext(runtime, apiRequest, storeId, tenantId = "") {
  const normalizedStoreID = String(storeId || "").trim();

  if (!normalizedStoreID) {
    return null;
  }

  await runtime.ensure();

  const resolvedTenantId = resolveTenantIdForStore(runtime.state, normalizedStoreID, tenantId);
  const remoteData = await fetchRemoteStoreData(apiRequest, normalizedStoreID, resolvedTenantId);
  runtime.hydrate(
    applyRemoteStoreData(
      runtime.state,
      normalizedStoreID,
      remoteData.settingsBundle,
      remoteData.consultants,
      remoteData.operationsSnapshot
    )
  );

  return remoteData;
}
