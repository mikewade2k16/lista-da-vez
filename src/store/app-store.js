import {
  cloneOperationTemplates,
  DEFAULT_OPERATION_TEMPLATE_ID,
  getOperationTemplateById
} from "../data/operation-templates.js";
import { DEFAULT_PROFESSION_OPTIONS } from "../data/profession-options.js";
import { applyCampaignsToHistoryEntry, normalizeCampaign } from "../utils/campaigns.js";
import { cloneValue } from "../utils/object.js";
import { getAllowedWorkspaces } from "../utils/permissions.js";
import { DEFAULT_REPORT_FILTERS, normalizeReportFilters } from "../utils/reports.js";

const CONSULTANT_COLORS = ["#168aad", "#7a6ff0", "#d17a96", "#e09f3e", "#355070", "#d90429", "#23a26d", "#4361ee"];

function getDefaultOperationTemplate() {
  return getOperationTemplateById(DEFAULT_OPERATION_TEMPLATE_ID);
}

const DEFAULT_PROFILES = [
  { id: "perfil-admin", name: "Admin Omni", role: "admin" },
  { id: "perfil-gerente", name: "Gerente Loja", role: "manager" },
  { id: "perfil-consultor", name: "Consultor Loja", role: "consultant" }
];

const DEFAULT_ACTIVE_PROFILE_ID = "perfil-admin";
const DEFAULT_STORES = [
  { id: "loja-pj-riomar", name: "Pérola Riomar", code: "PJ-RIO", city: "Aracaju" },
  { id: "loja-pj-jardins", name: "Pérola Jardins", code: "PJ-JAR", city: "Aracaju" },
  { id: "loja-pj-treze", name: "Pérola Treze", code: "PJ-TRE", city: "Aracaju" },
  { id: "loja-pj-garcia", name: "Pérola Garcia", code: "PJ-GAR", city: "Aracaju" }
];

function createEmptyStoreScopedState(roster = []) {
  const normalizedRoster = cloneValue(Array.isArray(roster) ? roster : []);

  return {
    selectedConsultantId: normalizedRoster[0]?.id || null,
    consultantSimulationAdditionalSales: 0,
    waitingList: [],
    activeServices: [],
    roster: normalizedRoster,
    consultantActivitySessions: [],
    consultantCurrentStatus: {},
    pausedEmployees: [],
    serviceHistory: []
  };
}

function extractStoreScopedState(sourceState) {
  return {
    selectedConsultantId: sourceState.selectedConsultantId,
    consultantSimulationAdditionalSales: Number(sourceState.consultantSimulationAdditionalSales || 0),
    waitingList: Array.isArray(sourceState.waitingList) ? sourceState.waitingList : [],
    activeServices: Array.isArray(sourceState.activeServices) ? sourceState.activeServices : [],
    roster: Array.isArray(sourceState.roster) ? sourceState.roster : [],
    consultantActivitySessions: Array.isArray(sourceState.consultantActivitySessions)
      ? sourceState.consultantActivitySessions
      : [],
    consultantCurrentStatus:
      sourceState.consultantCurrentStatus && typeof sourceState.consultantCurrentStatus === "object"
        ? sourceState.consultantCurrentStatus
        : {},
    pausedEmployees: Array.isArray(sourceState.pausedEmployees) ? sourceState.pausedEmployees : [],
    serviceHistory: Array.isArray(sourceState.serviceHistory) ? sourceState.serviceHistory : []
  };
}

function normalizeStoreList(rawStores, fallbackStores = DEFAULT_STORES) {
  const sourceStores = Array.isArray(rawStores) && rawStores.length ? rawStores : fallbackStores;
  const normalized = [];

  sourceStores.forEach((rawStore, index) => {
    const baseName = String(rawStore?.name || "").trim() || `Loja ${index + 1}`;
    const candidate = {
      id: String(rawStore?.id || "").trim(),
      name: baseName,
      code: String(rawStore?.code || "").trim(),
      city: String(rawStore?.city || "").trim()
    };

    if (!candidate.id) {
      candidate.id = createOptionId("loja", candidate.name, normalized);
    } else if (normalized.some((item) => item.id === candidate.id)) {
      candidate.id = createOptionId("loja", `${candidate.name}-${index + 1}`, normalized);
    }

    normalized.push(candidate);
  });

  return normalized;
}

function normalizeActiveServicesList(rawActiveServices, timestamp) {
  const now = Number(timestamp || Date.now());
  const source = Array.isArray(rawActiveServices) ? rawActiveServices : [];

  return source.map((service, index) => ({
    ...service,
    serviceId:
      service.serviceId ||
      service.serviceSessionId ||
      `${service.id || "service"}-${service.serviceStartedAt || now}-${index}`,
    serviceStartedAt: Number(service.serviceStartedAt || now),
    queueJoinedAt: Number(service.queueJoinedAt || service.serviceStartedAt || now),
    queueWaitMs: Number(service.queueWaitMs || 0),
    startMode: service.startMode || "queue",
    queuePositionAtStart: Number(service.queuePositionAtStart || index + 1),
    skippedPeople: Array.isArray(service.skippedPeople) ? service.skippedPeople : []
  }));
}

function normalizeServiceHistoryList(rawHistory, fallbackStoreId, fallbackStoreName, timestamp) {
  const now = Number(timestamp || Date.now());
  const source = Array.isArray(rawHistory) ? rawHistory : [];

  return source.map((entry, index) => ({
    ...entry,
    serviceId:
      entry.serviceId ||
      entry.serviceSessionId ||
      `${entry.personId || "service"}-${entry.startedAt || now}-${index}`,
    storeId: entry.storeId || fallbackStoreId || "",
    storeName: entry.storeName || fallbackStoreName || "",
    finishOutcome: entry.finishOutcome || "nao-compra",
    startMode: entry.startMode || "queue",
    queuePositionAtStart: Number(entry.queuePositionAtStart || 1),
    skippedPeople: Array.isArray(entry.skippedPeople) ? entry.skippedPeople : [],
    isWindowService: Boolean(entry.isWindowService),
    isGift: Boolean(entry.isGift),
    productSeen: entry.productSeen || entry.productDetails || "",
    productClosed: entry.productClosed || "",
    productDetails: entry.productDetails || entry.productClosed || entry.productSeen || "",
    customerName: entry.customerName || "",
    customerPhone: entry.customerPhone || "",
    customerEmail: entry.customerEmail || "",
    isExistingCustomer: Boolean(entry.isExistingCustomer),
    visitReasons: Array.isArray(entry.visitReasons) ? entry.visitReasons : [],
    visitReasonDetails:
      entry.visitReasonDetails && typeof entry.visitReasonDetails === "object" ? entry.visitReasonDetails : {},
    customerSources: Array.isArray(entry.customerSources)
      ? entry.customerSources
      : entry.customerSource
        ? [entry.customerSource]
        : [],
    customerSourceDetails:
      entry.customerSourceDetails && typeof entry.customerSourceDetails === "object"
        ? entry.customerSourceDetails
        : entry.customerSource
          ? { [entry.customerSource]: entry.customerSourceDetail || "" }
          : {},
    saleAmount: Number(entry.saleAmount || 0),
    customerProfession: entry.customerProfession || "",
    queueJumpReason: entry.queueJumpReason || "",
    notes: entry.notes || "",
    campaignMatches: Array.isArray(entry.campaignMatches) ? entry.campaignMatches : [],
    campaignBonusTotal: Number(entry.campaignBonusTotal || 0),
    skippedCount:
      typeof entry.skippedCount === "number"
        ? entry.skippedCount
        : Array.isArray(entry.skippedPeople)
          ? entry.skippedPeople.length
          : 0
  }));
}

function normalizeStoreScopedState(rawScopedState, fallbackScopedState, storeDescriptor, timestamp) {
  const now = Number(timestamp || Date.now());
  const fallback = fallbackScopedState || createEmptyStoreScopedState();
  const roster =
    Array.isArray(rawScopedState?.roster) && rawScopedState.roster.length
      ? rawScopedState.roster
      : cloneValue(fallback.roster || []);
  const selectedConsultantId = roster.some((consultant) => consultant.id === rawScopedState?.selectedConsultantId)
    ? rawScopedState.selectedConsultantId
    : roster[0]?.id || null;
  const scopedState = {
    selectedConsultantId,
    consultantSimulationAdditionalSales: Math.max(
      0,
      Number(rawScopedState?.consultantSimulationAdditionalSales ?? fallback.consultantSimulationAdditionalSales ?? 0) || 0
    ),
    waitingList: Array.isArray(rawScopedState?.waitingList)
      ? rawScopedState.waitingList.map((item) => ({
          ...item,
          queueJoinedAt: Number(item.queueJoinedAt || now)
        }))
      : [],
    activeServices: normalizeActiveServicesList(rawScopedState?.activeServices, now),
    roster,
    consultantActivitySessions: Array.isArray(rawScopedState?.consultantActivitySessions)
      ? rawScopedState.consultantActivitySessions
      : [],
    consultantCurrentStatus:
      rawScopedState?.consultantCurrentStatus && typeof rawScopedState.consultantCurrentStatus === "object"
        ? rawScopedState.consultantCurrentStatus
        : {},
    pausedEmployees: Array.isArray(rawScopedState?.pausedEmployees) ? rawScopedState.pausedEmployees : [],
    serviceHistory: normalizeServiceHistoryList(
      rawScopedState?.serviceHistory,
      storeDescriptor?.id || "",
      storeDescriptor?.name || "",
      now
    )
  };
  const hasAnyStatus = Object.keys(scopedState.consultantCurrentStatus).length > 0;

  return {
    ...scopedState,
    consultantCurrentStatus: hasAnyStatus
      ? reconcileConsultantStatuses(scopedState, now)
      : initializeConsultantStatuses(scopedState, now)
  };
}

function syncStoreSnapshots(nextState) {
  if (!nextState?.activeStoreId) {
    return nextState;
  }

  return {
    ...nextState,
    storeSnapshots: {
      ...(nextState.storeSnapshots || {}),
      [nextState.activeStoreId]: extractStoreScopedState(nextState)
    }
  };
}

function createEmptyState() {
  const defaultTemplate = getDefaultOperationTemplate();
  const stores = cloneValue(DEFAULT_STORES);
  const activeStoreId = stores[0]?.id || "loja-principal";
  const scopedState = createEmptyStoreScopedState();

  return {
    isReady: false,
    configSchemaVersion: 4,
    brandName: "Omni",
    pageTitle: "Fila de atendimento",
    profiles: cloneValue(DEFAULT_PROFILES),
    activeProfileId: DEFAULT_ACTIVE_PROFILE_ID,
    stores,
    activeStoreId,
    storeSnapshots: {
      [activeStoreId]: cloneValue(scopedState)
    },
    activeWorkspace: "operacao",
    selectedConsultantId: scopedState.selectedConsultantId,
    consultantSimulationAdditionalSales: scopedState.consultantSimulationAdditionalSales,
    operationTemplates: cloneOperationTemplates(),
    selectedOperationTemplateId: DEFAULT_OPERATION_TEMPLATE_ID,
    reportFilters: normalizeReportFilters(DEFAULT_REPORT_FILTERS),
    campaigns: [],
    waitingList: scopedState.waitingList,
    activeServices: scopedState.activeServices,
    roster: scopedState.roster,
    finishModalDraft: null,
    visitReasonOptions: cloneValue(defaultTemplate?.visitReasonOptions || []),
    customerSourceOptions: cloneValue(defaultTemplate?.customerSourceOptions || []),
    professionOptions: cloneValue(DEFAULT_PROFESSION_OPTIONS),
    productCatalog: [],
    modalConfig: {
      title: "Fechar atendimento",
      productSeenLabel: "Produto visto pelo cliente",
      productSeenPlaceholder: "Busque e selecione um produto",
      productClosedLabel: "Produto reservado/comprado",
      productClosedPlaceholder: "Busque e selecione o produto fechado",
      notesLabel: "Observacoes",
      notesPlaceholder: "Detalhes adicionais do atendimento",
      queueJumpReasonLabel: "Motivo do atendimento fora da vez",
      queueJumpReasonPlaceholder: "Cliente fixo, troca, retirada, cliente chamado pelo consultor...",
      customerSectionLabel: "Dados do cliente",
      showEmailField: true,
      showProfessionField: true,
      showNotesField: true,
      showVisitReasonDetails: true,
      showCustomerSourceDetails: true,
      requireProduct: true,
      requireVisitReason: true,
      requireCustomerSource: true,
      requireCustomerNamePhone: true
    },
    consultantActivitySessions: scopedState.consultantActivitySessions,
    consultantCurrentStatus: scopedState.consultantCurrentStatus,
    pausedEmployees: scopedState.pausedEmployees,
    settings: {
      maxConcurrentServices: Number(defaultTemplate?.settings?.maxConcurrentServices || 10),
      timingFastCloseMinutes: Number(defaultTemplate?.settings?.timingFastCloseMinutes || 5),
      timingLongServiceMinutes: Number(defaultTemplate?.settings?.timingLongServiceMinutes || 25),
      timingLowSaleAmount: Number(defaultTemplate?.settings?.timingLowSaleAmount || 1200),
      testModeEnabled: false,
      autoFillFinishModal: false
    },
    serviceHistory: scopedState.serviceHistory,
    finishModalPersonId: null
  };
}

const FINISH_OUTCOMES = new Set(["reserva", "compra", "nao-compra"]);

function createServiceId(personId) {
  return `${personId}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function getConsultantStatus(state, consultantId) {
  if (state.activeServices.some((item) => item.id === consultantId)) {
    return "service";
  }

  if (state.waitingList.some((item) => item.id === consultantId)) {
    return "queue";
  }

  if (state.pausedEmployees.some((item) => item.personId === consultantId)) {
    return "paused";
  }

  return "available";
}

function getConsultantStatusStartedAt(state, consultantId, timestamp) {
  const now = Number(timestamp || Date.now());
  const activeService = state.activeServices.find((item) => item.id === consultantId);

  if (activeService) {
    return Number(activeService.serviceStartedAt || now);
  }

  const waitingItem = state.waitingList.find((item) => item.id === consultantId);

  if (waitingItem) {
    return Number(waitingItem.queueJoinedAt || now);
  }

  const pausedItem = state.pausedEmployees.find((item) => item.personId === consultantId);

  if (pausedItem) {
    return Number(pausedItem.startedAt || now);
  }

  return now;
}

function initializeConsultantStatuses(state, timestamp) {
  const now = Number(timestamp || Date.now());
  const statusMap = {};

  state.roster.forEach((consultant) => {
    const status = getConsultantStatus(state, consultant.id);

    statusMap[consultant.id] = {
      status,
      startedAt: getConsultantStatusStartedAt(state, consultant.id, now)
    };
  });

  return statusMap;
}

function reconcileConsultantStatuses(state, timestamp) {
  const now = Number(timestamp || Date.now());
  const currentStatus =
    state.consultantCurrentStatus && typeof state.consultantCurrentStatus === "object"
      ? state.consultantCurrentStatus
      : {};
  const normalized = {};

  state.roster.forEach((consultant) => {
    const consultantId = consultant.id;
    const derivedStatus = getConsultantStatus(state, consultantId);
    const expectedStartedAt = getConsultantStatusStartedAt(state, consultantId, now);
    const previous = currentStatus[consultantId];

    if (previous && previous.status === derivedStatus) {
      normalized[consultantId] = {
        status: derivedStatus,
        startedAt:
          derivedStatus === "available"
            ? Number(previous.startedAt || now)
            : expectedStartedAt
      };
      return;
    }

    normalized[consultantId] = {
      status: derivedStatus,
      startedAt: derivedStatus === "available" ? now : expectedStartedAt
    };
  });

  return normalized;
}

function applyStatusTransitions(state, transitions, timestamp) {
  const now = Number(timestamp || Date.now());
  const currentStatus = { ...state.consultantCurrentStatus };
  const sessions = [...state.consultantActivitySessions];

  transitions.forEach(({ personId, nextStatus }) => {
    if (!personId || !nextStatus) {
      return;
    }

    const previous = currentStatus[personId] || { status: "available", startedAt: now };

    if (previous.status === nextStatus) {
      if (!currentStatus[personId]) {
        currentStatus[personId] = previous;
      }
      return;
    }

    sessions.push({
      personId,
      status: previous.status,
      startedAt: previous.startedAt,
      endedAt: now,
      durationMs: Math.max(0, now - previous.startedAt)
    });

    currentStatus[personId] = {
      status: nextStatus,
      startedAt: now
    };
  });

  return {
    consultantActivitySessions: sessions,
    consultantCurrentStatus: currentStatus
  };
}

function randomInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

function sampleItems(items, count) {
  const pool = [...items];
  const picked = [];

  while (pool.length > 0 && picked.length < count) {
    const index = randomInt(0, pool.length - 1);
    picked.push(pool[index]);
    pool.splice(index, 1);
  }

  return picked;
}

function slugifyLabel(label) {
  return String(label || "")
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");
}

function createOptionId(prefix, label, existingItems) {
  const base = `${prefix}-${slugifyLabel(label) || "item"}`;
  let candidate = base;
  let cursor = 2;

  while (existingItems.some((item) => item.id === candidate)) {
    candidate = `${base}-${cursor}`;
    cursor += 1;
  }

  return candidate;
}

function findOptionByLabel(options, label) {
  const normalizedLabel = String(label || "").trim().toLowerCase();

  if (!normalizedLabel) {
    return null;
  }

  return (options || []).find((item) => String(item?.label || "").trim().toLowerCase() === normalizedLabel) || null;
}

function appendUniqueOption(options, prefix, label) {
  const normalizedLabel = String(label || "").trim();

  if (!normalizedLabel) {
    return {
      item: null,
      items: Array.isArray(options) ? options : []
    };
  }

  const currentItems = Array.isArray(options) ? options : [];
  const existing = findOptionByLabel(currentItems, normalizedLabel);

  if (existing) {
    return {
      item: existing,
      items: currentItems
    };
  }

  const nextItem = {
    id: createOptionId(prefix, normalizedLabel, currentItems),
    label: normalizedLabel
  };

  return {
    item: nextItem,
    items: [...currentItems, nextItem]
  };
}

function buildConsultantInitials(name) {
  const parts = String(name || "")
    .trim()
    .split(/\s+/)
    .filter(Boolean);

  if (!parts.length) {
    return "CO";
  }

  const first = parts[0].charAt(0);
  const second = parts.length > 1 ? parts[1].charAt(0) : parts[0].charAt(1) || "X";

  return `${first}${second}`.toUpperCase();
}

function buildConsultantColor(existingRoster) {
  const usedColors = new Set((existingRoster || []).map((item) => item.color));
  const availableColor = CONSULTANT_COLORS.find((color) => !usedColors.has(color));

  return availableColor || CONSULTANT_COLORS[Math.floor(Math.random() * CONSULTANT_COLORS.length)];
}

function getActiveProfile(state) {
  return (state.profiles || []).find((profile) => profile.id === state.activeProfileId) || state.profiles?.[0] || null;
}

function getCurrentRole(state) {
  return getActiveProfile(state)?.role || "consultant";
}

function applyOperationTemplateToState(state, templateId) {
  const template = getOperationTemplateById(templateId);

  if (!template) {
    return state;
  }

  return {
    ...state,
    selectedOperationTemplateId: template.id,
    settings: {
      ...state.settings,
      ...cloneValue(template.settings || {})
    },
    modalConfig: {
      ...state.modalConfig,
      ...cloneValue(template.modalConfig || {})
    },
    visitReasonOptions: cloneValue(template.visitReasonOptions || []),
    customerSourceOptions: cloneValue(template.customerSourceOptions || [])
  };
}

function buildRandomFinishModalDraft(state, service) {
  if (!state.settings.testModeEnabled || !state.settings.autoFillFinishModal) {
    return null;
  }

  const outcomes = ["compra", "reserva", "nao-compra"];
  const outcome = outcomes[randomInt(0, outcomes.length - 1)];
  const products = state.productCatalog.length ? state.productCatalog : [{ name: "Produto teste", basePrice: 1000 }];
  const seenProduct = products[randomInt(0, products.length - 1)];
  const closedProduct =
    outcome === "compra" || outcome === "reserva" ? products[randomInt(0, products.length - 1)] : null;
  const visitReasonCount = state.visitReasonOptions.length ? 1 : 0;
  const sourceCount = state.customerSourceOptions.length ? 1 : 0;
  const visitReasons = sampleItems(state.visitReasonOptions, visitReasonCount).map((item) => item.id);
  const customerSources = sampleItems(state.customerSourceOptions, sourceCount).map((item) => item.id);
  const names = ["Ana", "Carla", "Bruna", "Mariana", "Paula", "Julia", "Erika"];
  const professions = state.professionOptions.length ? state.professionOptions : DEFAULT_PROFESSION_OPTIONS;
  const customerName = `${names[randomInt(0, names.length - 1)]} Teste`;

  const seenProductEntry = { id: seenProduct.id, name: seenProduct.name, price: seenProduct.basePrice };
  const closedProductEntry = closedProduct ? { id: closedProduct.id, name: closedProduct.name, price: closedProduct.basePrice } : null;

  return {
    outcome,
    isWindowService: Math.random() < 0.3,
    isGift: outcome === "compra" || outcome === "reserva" ? Math.random() < 0.5 : false,
    isExistingCustomer: Math.random() < 0.5,
    productSeen: seenProduct.name,
    productClosed: closedProduct ? closedProduct.name : "",
    productsSeen: [seenProductEntry],
    productsClosed: closedProductEntry ? [closedProductEntry] : [],
    customerName,
    customerPhone: `21${randomInt(900000000, 999999999)}`,
    customerEmail: `${customerName.toLowerCase().replace(/\s+/g, ".")}@exemplo.com`,
    customerProfession: professions[randomInt(0, professions.length - 1)]?.label || "",
    visitReasons,
    visitReasonDetails: Object.fromEntries(
      visitReasons.map((reasonId) => [reasonId, `Detalhe teste ${reasonId}`])
    ),
    customerSources,
    customerSourceDetails: Object.fromEntries(
      customerSources.map((sourceId) => [sourceId, `Origem teste ${sourceId}`])
    ),
    queueJumpReason: service.startMode === "queue-jump" ? "Cliente recorrente pediu atendimento direto" : "",
    notes: "Preenchimento automatico em modo teste."
  };
}

export function createAppStore(initialState = createEmptyState()) {
  let state = initialState;
  const listeners = new Set();

  function emitChange() {
    listeners.forEach((listener) => listener(state));
  }

  function updateState(nextState) {
    state = syncStoreSnapshots(nextState);
    emitChange();
  }

  return {
    getState() {
      return state;
    },

    subscribe(listener) {
      listeners.add(listener);

      return () => {
        listeners.delete(listener);
      };
    },

    hydrate(nextState) {
      const baseState = createEmptyState();
      const now = Date.now();
      const stores = normalizeStoreList(nextState.stores, baseState.stores);
      const activeStoreId = stores.some((store) => store.id === nextState.activeStoreId)
        ? nextState.activeStoreId
        : stores[0]?.id || baseState.activeStoreId;
      const activeStoreDescriptor = stores.find((store) => store.id === activeStoreId) || stores[0] || null;
      const legacyScopedState = {
        selectedConsultantId: nextState.selectedConsultantId,
        consultantSimulationAdditionalSales: nextState.consultantSimulationAdditionalSales,
        waitingList: nextState.waitingList,
        activeServices:
          nextState.activeServices ||
          (nextState.inService ? [nextState.inService] : []),
        roster: nextState.roster,
        consultantActivitySessions: nextState.consultantActivitySessions,
        consultantCurrentStatus: nextState.consultantCurrentStatus,
        pausedEmployees: nextState.pausedEmployees,
        serviceHistory: nextState.serviceHistory
      };
      const activeScopedFallback = createEmptyStoreScopedState(
        Array.isArray(nextState.roster) && nextState.roster.length ? nextState.roster : []
      );
      const normalizedLegacyActiveSnapshot = normalizeStoreScopedState(
        legacyScopedState,
        activeScopedFallback,
        activeStoreDescriptor,
        now
      );
      const rawSnapshots =
        nextState.storeSnapshots && typeof nextState.storeSnapshots === "object"
          ? nextState.storeSnapshots
          : {};
      const normalizedStoreSnapshots = {};

      stores.forEach((storeDescriptor) => {
        const rawSnapshot =
          rawSnapshots[storeDescriptor.id] ||
          (storeDescriptor.id === activeStoreId ? legacyScopedState : null);
        const fallbackSnapshot =
          storeDescriptor.id === activeStoreId
            ? normalizedLegacyActiveSnapshot
            : createEmptyStoreScopedState(cloneValue(normalizedLegacyActiveSnapshot.roster));

        normalizedStoreSnapshots[storeDescriptor.id] = normalizeStoreScopedState(
          rawSnapshot,
          fallbackSnapshot,
          storeDescriptor,
          now
        );
      });

      const resolvedActiveSnapshot =
        normalizedStoreSnapshots[activeStoreId] || normalizedLegacyActiveSnapshot;
      const profiles =
        Array.isArray(nextState.profiles) && nextState.profiles.length
          ? nextState.profiles
          : baseState.profiles;
      const activeProfileId = profiles.some((profile) => profile.id === nextState.activeProfileId)
        ? nextState.activeProfileId
        : profiles[0]?.id || baseState.activeProfileId;
      const activeProfile = profiles.find((profile) => profile.id === activeProfileId) || profiles[0] || null;
      const allowedWorkspaces = getAllowedWorkspaces(activeProfile?.role);
      const selectedWorkspace = allowedWorkspaces.includes(nextState.activeWorkspace)
        ? nextState.activeWorkspace
        : allowedWorkspaces[0] || "operacao";
      const hydratedState = {
        ...baseState,
        ...nextState,
        configSchemaVersion: baseState.configSchemaVersion,
        profiles,
        activeProfileId,
        stores,
        activeStoreId,
        storeSnapshots: normalizedStoreSnapshots,
        activeWorkspace: selectedWorkspace,
        selectedConsultantId: resolvedActiveSnapshot.selectedConsultantId,
        consultantSimulationAdditionalSales: resolvedActiveSnapshot.consultantSimulationAdditionalSales,
        operationTemplates: cloneOperationTemplates(),
        selectedOperationTemplateId:
          nextState.selectedOperationTemplateId || baseState.selectedOperationTemplateId,
        reportFilters: normalizeReportFilters(nextState.reportFilters || baseState.reportFilters),
        campaigns: Array.isArray(nextState.campaigns)
          ? nextState.campaigns.map((item) => normalizeCampaign(item))
          : cloneValue(baseState.campaigns),
        finishModalDraft: null,
        waitingList: resolvedActiveSnapshot.waitingList,
        activeServices: resolvedActiveSnapshot.activeServices,
        roster: resolvedActiveSnapshot.roster,
        visitReasonOptions:
          Array.isArray(nextState.visitReasonOptions) && nextState.visitReasonOptions.length
            ? nextState.visitReasonOptions
            : baseState.visitReasonOptions,
        customerSourceOptions:
          Array.isArray(nextState.customerSourceOptions) && nextState.customerSourceOptions.length
            ? nextState.customerSourceOptions
            : baseState.customerSourceOptions,
        professionOptions:
          Array.isArray(nextState.professionOptions) && nextState.professionOptions.length
            ? nextState.professionOptions
            : baseState.professionOptions,
        productCatalog:
          Array.isArray(nextState.productCatalog) && nextState.productCatalog.length
            ? nextState.productCatalog
            : baseState.productCatalog,
        modalConfig: {
          ...baseState.modalConfig,
          ...nextState.modalConfig
        },
        pausedEmployees: resolvedActiveSnapshot.pausedEmployees,
        consultantActivitySessions: resolvedActiveSnapshot.consultantActivitySessions,
        consultantCurrentStatus: resolvedActiveSnapshot.consultantCurrentStatus,
        settings: {
          ...baseState.settings,
          ...nextState.settings
        },
        serviceHistory: resolvedActiveSnapshot.serviceHistory,
        isReady: true,
        finishModalPersonId: null
      };

      updateState(hydratedState);
    },

    setActiveProfile(profileId) {
      const nextProfile = state.profiles.find((profile) => profile.id === profileId);

      if (!nextProfile) {
        return;
      }

      const allowedWorkspaces = getAllowedWorkspaces(nextProfile.role);
      const activeWorkspace = allowedWorkspaces.includes(state.activeWorkspace)
        ? state.activeWorkspace
        : allowedWorkspaces[0] || "operacao";

      updateState({
        ...state,
        activeProfileId: profileId,
        activeWorkspace,
        finishModalPersonId: null,
        finishModalDraft: null
      });
    },

    setActiveStore(storeId) {
      const nextStore = state.stores.find((store) => store.id === storeId);

      if (!nextStore || storeId === state.activeStoreId) {
        return;
      }

      const now = Date.now();
      const currentStoreId = state.activeStoreId;
      const currentSnapshot = extractStoreScopedState(state);
      const targetSnapshot = normalizeStoreScopedState(
        state.storeSnapshots?.[storeId],
        createEmptyStoreScopedState(cloneValue(state.roster)),
        nextStore,
        now
      );

      updateState({
        ...state,
        activeStoreId: storeId,
        storeSnapshots: {
          ...(state.storeSnapshots || {}),
          [currentStoreId]: currentSnapshot,
          [storeId]: targetSnapshot
        },
        ...targetSnapshot,
        finishModalPersonId: null,
        finishModalDraft: null
      });
    },

    createStore({ name, city, code, cloneActiveRoster = true }) {
      const normalizedName = String(name || "").trim();

      if (!normalizedName) {
        return { ok: false, message: "Nome da loja e obrigatorio." };
      }

      const storeId = createOptionId("loja", normalizedName, state.stores);
      const nextStore = {
        id: storeId,
        name: normalizedName,
        city: String(city || "").trim(),
        code: String(code || "").trim()
      };
      const baseRoster = cloneActiveRoster ? cloneValue(state.roster) : [];
      const snapshot = normalizeStoreScopedState(
        createEmptyStoreScopedState(baseRoster),
        createEmptyStoreScopedState(baseRoster),
        nextStore,
        Date.now()
      );

      updateState({
        ...state,
        stores: [...state.stores, nextStore],
        storeSnapshots: {
          ...(state.storeSnapshots || {}),
          [storeId]: snapshot
        }
      });

      return { ok: true, storeId };
    },

    updateStore(storeId, patch) {
      const existingStore = state.stores.find((store) => store.id === storeId);

      if (!existingStore) {
        return { ok: false, message: "Loja nao encontrada." };
      }

      const name = String((patch?.name ?? existingStore.name) || "").trim();

      if (!name) {
        return { ok: false, message: "Nome da loja e obrigatorio." };
      }

      const updatedStore = {
        ...existingStore,
        name,
        city: String((patch?.city ?? existingStore.city) || "").trim(),
        code: String((patch?.code ?? existingStore.code) || "").trim()
      };

      updateState({
        ...state,
        stores: state.stores.map((store) => (store.id === storeId ? updatedStore : store))
      });

      return { ok: true };
    },

    archiveStore(storeId) {
      const existingStore = state.stores.find((store) => store.id === storeId);

      if (!existingStore) {
        return { ok: false, message: "Loja nao encontrada." };
      }

      if (state.stores.length <= 1) {
        return { ok: false, message: "Mantenha pelo menos uma loja ativa no sistema." };
      }

      const nextStores = state.stores.filter((store) => store.id !== storeId);
      const nextStoreSnapshots = { ...(state.storeSnapshots || {}) };

      delete nextStoreSnapshots[storeId];

      if (storeId !== state.activeStoreId) {
        updateState({
          ...state,
          stores: nextStores,
          storeSnapshots: nextStoreSnapshots
        });

        return { ok: true };
      }

      const nextActiveStoreId = nextStores[0]?.id;
      const nextActiveStoreDescriptor = nextStores.find((store) => store.id === nextActiveStoreId) || null;
      const nextSnapshot = normalizeStoreScopedState(
        nextStoreSnapshots[nextActiveStoreId],
        createEmptyStoreScopedState(cloneValue(state.roster)),
        nextActiveStoreDescriptor,
        Date.now()
      );

      updateState({
        ...state,
        stores: nextStores,
        activeStoreId: nextActiveStoreId,
        storeSnapshots: {
          ...nextStoreSnapshots,
          [nextActiveStoreId]: nextSnapshot
        },
        ...nextSnapshot,
        finishModalPersonId: null,
        finishModalDraft: null
      });

      return { ok: true };
    },

    setWorkspace(workspaceId) {
      const allowedWorkspaces = getAllowedWorkspaces(getCurrentRole(state));

      if (!allowedWorkspaces.includes(workspaceId)) {
        return;
      }

      updateState({
        ...state,
        activeWorkspace: workspaceId
      });
    },

    updateReportFilter(filterId, value) {
      if (!(filterId in state.reportFilters)) {
        return;
      }

      const normalizedValue = Array.isArray(value)
        ? [...new Set(value.map((item) => String(item || "").trim()).filter(Boolean))]
        : ["minSaleAmount", "maxSaleAmount"].includes(filterId) && value !== ""
          ? String(Math.max(0, Number(value) || 0))
          : String(value ?? "");

      updateState({
        ...state,
        reportFilters: {
          ...state.reportFilters,
          [filterId]: normalizedValue
        }
      });
    },

    resetReportFilters() {
      updateState({
        ...state,
        reportFilters: normalizeReportFilters(DEFAULT_REPORT_FILTERS)
      });
    },

    createCampaign(campaignInput) {
      const name = String(campaignInput?.name || "").trim();

      if (!name) {
        return { ok: false, message: "Nome da campanha e obrigatorio." };
      }

      const campaignId = createOptionId("campanha", name, state.campaigns);
      const campaign = normalizeCampaign({
        ...campaignInput,
        id: campaignId,
        name
      });

      updateState({
        ...state,
        campaigns: [...state.campaigns, campaign]
      });

      return { ok: true };
    },

    updateCampaign(campaignId, patch) {
      const existing = state.campaigns.find((campaign) => campaign.id === campaignId);

      if (!existing) {
        return { ok: false, message: "Campanha nao encontrada." };
      }

      const nextCampaign = normalizeCampaign({
        ...existing,
        ...patch,
        id: campaignId
      });

      if (!nextCampaign.name) {
        return { ok: false, message: "Nome da campanha e obrigatorio." };
      }

      updateState({
        ...state,
        campaigns: state.campaigns.map((campaign) => (campaign.id === campaignId ? nextCampaign : campaign))
      });

      return { ok: true };
    },

    removeCampaign(campaignId) {
      updateState({
        ...state,
        campaigns: state.campaigns.filter((campaign) => campaign.id !== campaignId)
      });
    },

    updateSetting(settingId, value) {
      if (!(settingId in state.settings)) {
        return;
      }

      updateState({
        ...state,
        settings: {
          ...state.settings,
          [settingId]: value
        }
      });
    },

    updateModalConfig(configKey, value) {
      if (!(configKey in state.modalConfig)) {
        return;
      }

      updateState({
        ...state,
        modalConfig: {
          ...state.modalConfig,
          [configKey]: value
        }
      });
    },

    applyOperationTemplate(templateId) {
      updateState(applyOperationTemplateToState(state, templateId));
    },

    addVisitReasonOption(label) {
      const normalized = String(label || "").trim();

      if (!normalized) {
        return;
      }

      updateState({
        ...state,
        visitReasonOptions: [
          ...state.visitReasonOptions,
          {
            id: createOptionId("motivo", normalized, state.visitReasonOptions),
            label: normalized
          }
        ]
      });
    },

    updateVisitReasonOption(optionId, label) {
      const normalized = String(label || "").trim();

      if (!normalized) {
        return;
      }

      updateState({
        ...state,
        visitReasonOptions: state.visitReasonOptions.map((item) =>
          item.id === optionId ? { ...item, label: normalized } : item
        )
      });
    },

    removeVisitReasonOption(optionId) {
      updateState({
        ...state,
        visitReasonOptions: state.visitReasonOptions.filter((item) => item.id !== optionId)
      });
    },

    addCustomerSourceOption(label) {
      const normalized = String(label || "").trim();

      if (!normalized) {
        return;
      }

      updateState({
        ...state,
        customerSourceOptions: [
          ...state.customerSourceOptions,
          {
            id: createOptionId("origem", normalized, state.customerSourceOptions),
            label: normalized
          }
        ]
      });
    },

    updateCustomerSourceOption(optionId, label) {
      const normalized = String(label || "").trim();

      if (!normalized) {
        return;
      }

      updateState({
        ...state,
        customerSourceOptions: state.customerSourceOptions.map((item) =>
          item.id === optionId ? { ...item, label: normalized } : item
        )
      });
    },

    removeCustomerSourceOption(optionId) {
      updateState({
        ...state,
        customerSourceOptions: state.customerSourceOptions.filter((item) => item.id !== optionId)
      });
    },

    addProfessionOption(label) {
      const { item, items } = appendUniqueOption(state.professionOptions, "profissao", label);

      if (!item || items === state.professionOptions) {
        return;
      }

      updateState({
        ...state,
        professionOptions: items
      });
    },

    updateProfessionOption(optionId, label) {
      const normalized = String(label || "").trim();

      if (!normalized) {
        return;
      }

      const duplicate = state.professionOptions.find(
        (item) => item.id !== optionId && String(item.label || "").trim().toLowerCase() === normalized.toLowerCase()
      );

      if (duplicate) {
        return;
      }

      updateState({
        ...state,
        professionOptions: state.professionOptions.map((item) =>
          item.id === optionId ? { ...item, label: normalized } : item
        )
      });
    },

    removeProfessionOption(optionId) {
      updateState({
        ...state,
        professionOptions: state.professionOptions.filter((item) => item.id !== optionId)
      });
    },

    addCatalogProduct(name, category, basePrice) {
      const normalizedName = String(name || "").trim();
      const normalizedCategory = String(category || "").trim();
      const price = Math.max(0, Number(basePrice) || 0);

      if (!normalizedName) {
        return;
      }

      const id = createOptionId("produto", normalizedName, state.productCatalog);

      updateState({
        ...state,
        productCatalog: [
          ...state.productCatalog,
          {
            id,
            name: normalizedName,
            category: normalizedCategory || "Sem categoria",
            basePrice: price
          }
        ]
      });
    },

    updateCatalogProduct(productId, patch) {
      updateState({
        ...state,
        productCatalog: state.productCatalog.map((product) =>
          product.id === productId
            ? {
                ...product,
                ...patch,
                name: String((patch.name ?? product.name) || "").trim() || product.name,
                category: String((patch.category ?? product.category) || "").trim() || "Sem categoria",
                basePrice: Math.max(0, Number(patch.basePrice ?? product.basePrice) || 0)
              }
            : product
        )
      });
    },

    removeCatalogProduct(productId) {
      updateState({
        ...state,
        productCatalog: state.productCatalog.filter((product) => product.id !== productId)
      });
    },

    createConsultantProfile({ name, role, color, monthlyGoal, commissionRate }) {
      const normalizedName = String(name || "").trim();
      const normalizedRole = String(role || "Atendimento").trim() || "Atendimento";
      const goal = Math.max(0, Number(monthlyGoal) || 0);
      const commission = Math.max(0, Number(commissionRate) || 0);

      if (!normalizedName) {
        return { ok: false, message: "Nome do consultor e obrigatorio." };
      }

      const consultantId = createOptionId("consultor", normalizedName, state.roster);
      const consultant = {
        id: consultantId,
        name: normalizedName,
        role: normalizedRole,
        initials: buildConsultantInitials(normalizedName),
        color: color?.trim() || buildConsultantColor(state.roster),
        monthlyGoal: goal,
        commissionRate: commission
      };
      const now = Date.now();
      const consultantCurrentStatus = {
        ...state.consultantCurrentStatus,
        [consultantId]: {
          status: "available",
          startedAt: now
        }
      };

      updateState({
        ...state,
        roster: [...state.roster, consultant],
        consultantCurrentStatus,
        selectedConsultantId: consultantId
      });

      return { ok: true };
    },

    updateConsultantProfile(consultantId, patch) {
      const existing = state.roster.find((consultant) => consultant.id === consultantId);

      if (!existing) {
        return { ok: false, message: "Consultor nao encontrado." };
      }

      const name = String((patch.name ?? existing.name) || "").trim();
      const role = String((patch.role ?? existing.role) || "").trim() || "Atendimento";
      const monthlyGoal = Math.max(0, Number(patch.monthlyGoal ?? existing.monthlyGoal) || 0);
      const commissionRate = Math.max(0, Number(patch.commissionRate ?? existing.commissionRate) || 0);
      const color = String((patch.color ?? existing.color) || "").trim() || existing.color;
      const initials = buildConsultantInitials(name || existing.name);
      const nextConsultant = {
        ...existing,
        name: name || existing.name,
        role,
        initials,
        color,
        monthlyGoal,
        commissionRate
      };

      updateState({
        ...state,
        roster: state.roster.map((consultant) => (consultant.id === consultantId ? nextConsultant : consultant)),
        waitingList: state.waitingList.map((item) => (item.id === consultantId ? { ...item, ...nextConsultant } : item)),
        activeServices: state.activeServices.map((item) => (item.id === consultantId ? { ...item, ...nextConsultant } : item))
      });

      return { ok: true };
    },

    archiveConsultantProfile(consultantId) {
      const consultant = state.roster.find((item) => item.id === consultantId);

      if (!consultant) {
        return { ok: false, message: "Consultor nao encontrado." };
      }

      const isInQueue = state.waitingList.some((item) => item.id === consultantId);
      const isInService = state.activeServices.some((item) => item.id === consultantId);
      const isPaused = state.pausedEmployees.some((item) => item.personId === consultantId);

      if (isInQueue || isInService || isPaused) {
        return {
          ok: false,
          message: "Retire o consultor de fila, atendimento ou pausa antes de arquivar."
        };
      }

      const nextCurrentStatus = { ...state.consultantCurrentStatus };
      delete nextCurrentStatus[consultantId];

      updateState({
        ...state,
        roster: state.roster.filter((item) => item.id !== consultantId),
        consultantCurrentStatus: nextCurrentStatus,
        selectedConsultantId:
          state.selectedConsultantId === consultantId
            ? state.roster.find((item) => item.id !== consultantId)?.id || null
            : state.selectedConsultantId
      });

      return { ok: true };
    },

    setSelectedConsultant(personId) {
      if (!state.roster.some((consultant) => consultant.id === personId)) {
        return;
      }

      updateState({
        ...state,
        selectedConsultantId: personId
      });
    },

    setConsultantSimulationAdditionalSales(amount) {
      updateState({
        ...state,
        consultantSimulationAdditionalSales: Math.max(0, Number(amount) || 0)
      });
    },

    addToQueue(personId) {
      const now = Date.now();
      const person = state.roster.find((item) => item.id === personId);
      const isAlreadyWaiting = state.waitingList.some((item) => item.id === personId);
      const isInService = state.activeServices.some((item) => item.id === personId);
      const isPaused = state.pausedEmployees.some((item) => item.personId === personId);

      if (!person || isAlreadyWaiting || isInService || isPaused) {
        return;
      }

      updateState({
        ...state,
        waitingList: [...state.waitingList, { ...person, queueJoinedAt: now }],
        ...applyStatusTransitions(state, [{ personId, nextStatus: "queue" }], now)
      });
    },

    pauseEmployee(personId, reason) {
      if (!reason?.trim()) {
        return;
      }
      const now = Date.now();

      const alreadyPaused = state.pausedEmployees.some((item) => item.personId === personId);
      const isInService = state.activeServices.some((item) => item.id === personId);

      if (alreadyPaused || isInService) {
        return;
      }

      updateState({
        ...state,
        waitingList: state.waitingList.filter((item) => item.id !== personId),
        pausedEmployees: [
          ...state.pausedEmployees,
          {
            personId,
            reason: reason.trim(),
            startedAt: now
          }
        ],
        ...applyStatusTransitions(state, [{ personId, nextStatus: "paused" }], now)
      });
    },

    resumeEmployee(personId) {
      const now = Date.now();
      const pausedEntry = state.pausedEmployees.find((item) => item.personId === personId);
      const consultant = state.roster.find((item) => item.id === personId);
      const isAlreadyWaiting = state.waitingList.some((item) => item.id === personId);
      const isInService = state.activeServices.some((item) => item.id === personId);

      if (!pausedEntry) {
        return;
      }

      const nextWaitingList =
        !consultant || isAlreadyWaiting || isInService
          ? state.waitingList
          : [...state.waitingList, { ...consultant, queueJoinedAt: now }];
      const nextStatus = isInService ? "service" : "queue";

      updateState({
        ...state,
        waitingList: nextWaitingList,
        pausedEmployees: state.pausedEmployees.filter((item) => item.personId !== personId),
        ...applyStatusTransitions(state, [{ personId, nextStatus }], now)
      });
    },

    startService(personId = null) {
      if (state.waitingList.length === 0) {
        return;
      }
      const now = Date.now();

      if (state.activeServices.length >= state.settings.maxConcurrentServices) {
        return;
      }

      const targetIndex =
        personId === null ? 0 : state.waitingList.findIndex((item) => item.id === personId);

      if (targetIndex === -1) {
        return;
      }

      const nextPerson = state.waitingList[targetIndex];
      const remainingQueue = state.waitingList.filter((item) => item.id !== nextPerson.id);
      const skippedPeople = state.waitingList.slice(0, targetIndex).map((person) => ({
        id: person.id,
        name: person.name
      }));
      const queueJoinedAt = Number(nextPerson.queueJoinedAt || now);
      const serviceEntry = {
        ...nextPerson,
        serviceId: createServiceId(nextPerson.id),
        serviceStartedAt: now,
        queueJoinedAt,
        queueWaitMs: Math.max(0, now - queueJoinedAt),
        queuePositionAtStart: targetIndex + 1,
        startMode: targetIndex === 0 ? "queue" : "queue-jump",
        skippedPeople
      };

      updateState({
        ...state,
        waitingList: remainingQueue,
        activeServices: [...state.activeServices, serviceEntry],
        ...applyStatusTransitions(state, [{ personId: nextPerson.id, nextStatus: "service" }], now)
      });
    },

    openFinishModal(personId) {
      const activeService = state.activeServices.find((item) => item.id === personId);

      if (!activeService) {
        return;
      }

      updateState({
        ...state,
        finishModalPersonId: personId,
        finishModalDraft: buildRandomFinishModalDraft(state, activeService)
      });
    },

    closeFinishModal() {
      updateState({
        ...state,
        finishModalPersonId: null,
        finishModalDraft: null
      });
    },

    finishService(personId, closureData) {
      if (!FINISH_OUTCOMES.has(closureData?.outcome)) {
        return;
      }
      const now = Date.now();

      const serviceIndex = state.activeServices.findIndex((item) => item.id === personId);

      if (serviceIndex === -1) {
        return;
      }

      const activeService = state.activeServices[serviceIndex];
      const finishedAt = now;
      const nextActiveServices = state.activeServices.filter((item) => item.id !== personId);
      const activeStore = state.stores.find((store) => store.id === state.activeStoreId) || null;
      const normalizedProfession = String(closureData.customerProfession || "").trim();
      const nextProfessionOptions = normalizedProfession
        ? appendUniqueOption(state.professionOptions, "profissao", normalizedProfession).items
        : state.professionOptions;
      const historyEntry = {
        serviceId: activeService.serviceId,
        storeId: state.activeStoreId,
        storeName: activeStore?.name || "",
        personId: activeService.id,
        personName: activeService.name,
        startedAt: activeService.serviceStartedAt,
        finishedAt,
        durationMs: finishedAt - activeService.serviceStartedAt,
        finishOutcome: closureData.outcome,
        startMode: activeService.startMode,
        queuePositionAtStart: activeService.queuePositionAtStart,
        queueWaitMs: Number(activeService.queueWaitMs || 0),
        skippedPeople: activeService.skippedPeople,
        skippedCount: activeService.skippedPeople.length,
        isWindowService: closureData.isWindowService,
        isGift: closureData.isGift,
        productSeen: closureData.productSeen,
        productClosed: closureData.productClosed,
        productDetails: closureData.productClosed || closureData.productSeen || closureData.productDetails,
        customerName: closureData.customerName,
        customerPhone: closureData.customerPhone,
        customerEmail: closureData.customerEmail,
        isExistingCustomer: closureData.isExistingCustomer,
        visitReasons: closureData.visitReasons,
        visitReasonDetails: closureData.visitReasonDetails,
        customerSources: closureData.customerSources,
        customerSourceDetails: closureData.customerSourceDetails,
        saleAmount: Math.max(0, Number(closureData.saleAmount || 0)),
        customerProfession: normalizedProfession,
        queueJumpReason: closureData.queueJumpReason,
        notes: closureData.notes
      };
      const campaignResult = applyCampaignsToHistoryEntry(state.campaigns, historyEntry);
      const finalizedHistoryEntry = {
        ...historyEntry,
        campaignMatches: campaignResult.matches,
        campaignBonusTotal: campaignResult.totalBonus
      };

      updateState({
        ...state,
        waitingList: [
          ...state.waitingList,
          {
            ...(state.roster.find((item) => item.id === personId) || activeService),
            queueJoinedAt: now
          }
        ],
        activeServices: nextActiveServices,
        professionOptions: nextProfessionOptions,
        serviceHistory: [...state.serviceHistory, finalizedHistoryEntry],
        finishModalPersonId: null,
        finishModalDraft: null,
        ...applyStatusTransitions(state, [{ personId, nextStatus: "queue" }], now)
      });
    }
  };
}
