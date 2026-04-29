import {
  cloneOperationTemplates,
  DEFAULT_LOSS_REASON_OPTIONS,
  DEFAULT_PAUSE_REASON_OPTIONS,
  DEFAULT_QUEUE_JUMP_REASON_OPTIONS,
  DEFAULT_OPERATION_TEMPLATE_ID,
  getOperationTemplateById
} from "~/domain/data/operation-templates";
import { DEFAULT_PROFESSION_OPTIONS } from "~/domain/data/profession-options";
import { normalizeCampaign } from "~/domain/utils/campaigns";
import { cloneValue } from "~/domain/utils/object";
import { getAllowedWorkspaces } from "~/domain/utils/permissions";
import { DEFAULT_REPORT_FILTERS, normalizeReportFilters } from "~/domain/utils/reports";
import { createOptionId, randomInt, sampleItems } from "~/stores/dashboard/runtime/shared";
import { initializeConsultantStatuses, reconcileConsultantStatuses } from "~/stores/dashboard/runtime/status";

function getDefaultOperationTemplate() {
  return getOperationTemplateById(DEFAULT_OPERATION_TEMPLATE_ID);
}

export function normalizeVisitReasonOptions(options = []) {
  return (Array.isArray(options) ? options : []).map((option) => ({
    id: String(option?.id || "").trim(),
    label: String(option?.label || option?.name || "").trim()
  })).filter((option) => option.id && option.label);
}

export function normalizeSimpleOptions(options = []) {
  return (Array.isArray(options) ? options : []).map((option) => ({
    id: String(option?.id || "").trim(),
    label: String(option?.label || option?.name || "").trim()
  })).filter((option) => option.id && option.label);
}

export function normalizeIdArray(values = []) {
  return [...new Set((Array.isArray(values) ? values : []).map((value) => String(value || "").trim()).filter(Boolean))];
}

export function normalizeDetailMap(details = {}) {
  if (!details || typeof details !== "object") {
    return {};
  }

  return Object.fromEntries(
    Object.entries(details)
      .map(([key, value]) => [String(key || "").trim(), String(value || "").trim()])
      .filter(([key]) => key)
  );
}

const DEFAULT_PROFILES = [
  { id: "perfil-platform-admin", name: "Admin Plataforma", role: "platform_admin" },
  { id: "perfil-proprietario", name: "Proprietario Grupo", role: "owner" },
  { id: "perfil-marketing", name: "Marketing Grupo", role: "marketing" },
  { id: "perfil-gerente", name: "Gerente Loja", role: "manager" },
  { id: "perfil-consultor", name: "Consultor Loja", role: "consultant" }
];

const DEFAULT_ACTIVE_PROFILE_ID = "perfil-platform-admin";
const DEFAULT_STORES = [
  { id: "loja-pj-riomar", name: "PÃ©rola Riomar", code: "PJ-RIO", city: "Aracaju" },
  { id: "loja-pj-jardins", name: "PÃ©rola Jardins", code: "PJ-JAR", city: "Aracaju" },
  { id: "loja-pj-treze", name: "PÃ©rola Treze", code: "PJ-TRE", city: "Aracaju" },
  { id: "loja-pj-garcia", name: "PÃ©rola Garcia", code: "PJ-GAR", city: "Aracaju" }
];

export function createEmptyStoreScopedState(roster = []) {
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

export function extractStoreScopedState(sourceState) {
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

export function normalizeStoreList(rawStores, fallbackStores = DEFAULT_STORES) {
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

export function normalizeActiveServicesList(rawActiveServices, timestamp) {
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
    skippedPeople: Array.isArray(service.skippedPeople) ? service.skippedPeople : [],
    parallelGroupId: String(service.parallelGroupId || ""),
    parallelStartIndex: typeof service.parallelStartIndex === "number" ? service.parallelStartIndex : null,
    startOffsetMs: Number(service.startOffsetMs || 0),
    siblingServiceIds: Array.isArray(service.siblingServiceIds) ? service.siblingServiceIds : [],
    stoppedAt: Math.max(0, Number(service.stoppedAt || 0) || 0),
    stopReason: String(service.stopReason || "").trim()
  }));
}

export function normalizeServiceHistoryList(rawHistory, fallbackStoreId, fallbackStoreName, timestamp) {
  const now = Number(timestamp || Date.now());
  const source = Array.isArray(rawHistory) ? rawHistory : [];

  return source.map((entry, index) => {
    const normalizedVisitReasons = normalizeIdArray(entry.visitReasons);
    const normalizedCustomerSources = normalizeIdArray(
      Array.isArray(entry.customerSources)
        ? entry.customerSources
        : entry.customerSource
          ? [entry.customerSource]
          : []
    );
    const normalizedLossReasons = normalizeIdArray(
      Array.isArray(entry.lossReasons)
        ? entry.lossReasons
        : entry.lossReasonId
          ? [entry.lossReasonId]
          : []
    );

    return {
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
      productsSeen: Array.isArray(entry.productsSeen) ? entry.productsSeen : [],
      productsClosed: Array.isArray(entry.productsClosed) ? entry.productsClosed : [],
      productsSeenNone: Boolean(entry.productsSeenNone),
      visitReasonsNotInformed: Boolean(entry.visitReasonsNotInformed),
      customerSourcesNotInformed: Boolean(entry.customerSourcesNotInformed),
      customerName: entry.customerName || "",
      customerPhone: entry.customerPhone || "",
      customerEmail: entry.customerEmail || "",
      isExistingCustomer: Boolean(entry.isExistingCustomer),
      visitReasons: normalizedVisitReasons,
      visitReasonDetails: normalizeDetailMap(entry.visitReasonDetails),
      customerSources: normalizedCustomerSources,
      customerSourceDetails:
        entry.customerSourceDetails && typeof entry.customerSourceDetails === "object"
          ? normalizeDetailMap(entry.customerSourceDetails)
          : entry.customerSource
            ? { [entry.customerSource]: entry.customerSourceDetail || "" }
            : {},
      lossReasons: normalizedLossReasons,
      lossReasonDetails: normalizeDetailMap(entry.lossReasonDetails),
      lossReasonId: String(entry.lossReasonId || normalizedLossReasons[0] || "").trim(),
      lossReason: String(entry.lossReason || "").trim(),
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
    };
  });
}

export function normalizeStoreScopedState(rawScopedState, fallbackScopedState, storeDescriptor, timestamp) {
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

export function syncStoreSnapshots(nextState) {
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

export function createEmptyState() {
  const defaultTemplate = getDefaultOperationTemplate();
  const stores = cloneValue(DEFAULT_STORES);
  const activeStoreId = stores[0]?.id || "loja-principal";
  const scopedState = createEmptyStoreScopedState();

  return {
    isReady: false,
    configSchemaVersion: 4,
    serverClockOffsetMs: 0,
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
    visitReasonOptions: normalizeVisitReasonOptions(defaultTemplate?.visitReasonOptions || []),
    customerSourceOptions: cloneValue(defaultTemplate?.customerSourceOptions || []),
    pauseReasonOptions: cloneValue(DEFAULT_PAUSE_REASON_OPTIONS),
    cancelReasonOptions: [],
    stopReasonOptions: [],
    queueJumpReasonOptions: cloneValue(DEFAULT_QUEUE_JUMP_REASON_OPTIONS),
    lossReasonOptions: cloneValue(DEFAULT_LOSS_REASON_OPTIONS),
    professionOptions: cloneValue(DEFAULT_PROFESSION_OPTIONS),
    productCatalog: [],
    modalConfig: {
      title: "Fechar atendimento",
      productSeenLabel: "Interesses do cliente",
      productSeenPlaceholder: "Busque e selecione interesses",
      productClosedLabel: "",
      productClosedPlaceholder: "Busque e selecione o produto fechado",
      notesLabel: "Observações",
      notesPlaceholder: "Detalhes adicionais do atendimento",
      queueJumpReasonLabel: "Motivo do atendimento fora da vez",
      queueJumpReasonPlaceholder: "Busque e selecione o motivo fora da vez",
      lossReasonLabel: "Motivo da perda",
      lossReasonPlaceholder: "Busque e selecione o motivo da perda",
      customerSectionLabel: "Dados do cliente",
      customerNameLabel: "Nome do cliente",
      customerPhoneLabel: "Telefone",
      customerEmailLabel: "E-mail",
      customerProfessionLabel: "Profissão",
      existingCustomerLabel: "Já era cliente",
      productSeenNotesLabel: "Observação dos interesses",
      productSeenNotesPlaceholder: "Descreva referência, pedido específico, contexto do cliente ou justificativa quando não houver interesse identificado.",
      visitReasonLabel: "Motivo da visita",
      customerSourceLabel: "Origem do cliente",
      cancelReasonLabel: "Motivo do cancelamento",
      cancelReasonPlaceholder: "Informe ou selecione o motivo do cancelamento",
      cancelReasonOtherLabel: "Detalhe do cancelamento",
      cancelReasonOtherPlaceholder: "Explique por que o atendimento foi cancelado",
      stopReasonLabel: "Motivo da parada",
      stopReasonPlaceholder: "Informe ou selecione o motivo da parada",
      stopReasonOtherLabel: "Detalhe da parada",
      stopReasonOtherPlaceholder: "Explique por que o atendimento foi parado",
      showCustomerNameField: true,
      showCustomerPhoneField: true,
      showEmailField: true,
      showProfessionField: true,
      showNotesField: true,
      showProductSeenField: true,
      showProductSeenNotesField: true,
      showProductClosedField: true,
      showVisitReasonField: true,
      showCustomerSourceField: true,
      showExistingCustomerField: true,
      showQueueJumpReasonField: true,
      showLossReasonField: true,
      showCancelReasonField: true,
      showStopReasonField: true,
      allowProductSeenNone: true,
      visitReasonSelectionMode: "multiple",
      visitReasonDetailMode: "shared",
      lossReasonSelectionMode: "single",
      lossReasonDetailMode: "off",
      customerSourceSelectionMode: "single",
      customerSourceDetailMode: "shared",
      cancelReasonInputMode: "text",
      stopReasonInputMode: "text",
      requireCustomerNameField: true,
      requireCustomerPhoneField: true,
      requireEmailField: false,
      requireProfessionField: false,
      requireNotesField: false,
      requireProduct: true,
      requireProductSeenField: true,
      requireProductSeenNotesField: false,
      requireProductClosedField: true,
      requireVisitReason: true,
      requireCustomerSource: true,
      requireCustomerNamePhone: true,
      requireProductSeenNotesWhenNone: true,
      productSeenNotesMinChars: 20,
      requireQueueJumpReasonField: true,
      requireLossReasonField: true,
      requireCancelReasonField: false,
      requireStopReasonField: false
    },
    consultantActivitySessions: scopedState.consultantActivitySessions,
    consultantCurrentStatus: scopedState.consultantCurrentStatus,
    pausedEmployees: scopedState.pausedEmployees,
    settings: {
      maxConcurrentServices: Number(defaultTemplate?.settings?.maxConcurrentServices || 10),
      maxConcurrentServicesPerConsultant: Number(defaultTemplate?.settings?.maxConcurrentServicesPerConsultant || 1),
      timingFastCloseMinutes: Number(defaultTemplate?.settings?.timingFastCloseMinutes || 5),
      timingLongServiceMinutes: Number(defaultTemplate?.settings?.timingLongServiceMinutes || 25),
      timingLowSaleAmount: Number(defaultTemplate?.settings?.timingLowSaleAmount || 1200),
      serviceCancelWindowSeconds: Number(defaultTemplate?.settings?.serviceCancelWindowSeconds || 30),
      testModeEnabled: false,
      autoFillFinishModal: false,
      alertMinConversionRate: 0,
      alertMaxQueueJumpRate: 0,
      alertMinPaScore: 0,
      alertMinTicketAverage: 0
    },
    serviceHistory: scopedState.serviceHistory,
    finishModalServiceId: null
  };
}

export function applyOperationTemplateToState(state, templateId) {
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
    visitReasonOptions: normalizeVisitReasonOptions(template.visitReasonOptions || []),
    customerSourceOptions: cloneValue(template.customerSourceOptions || [])
  };
}

export function buildRandomFinishModalDraft(state, service) {
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
  const queueJumpReasons = state.queueJumpReasonOptions.length
    ? state.queueJumpReasonOptions
    : DEFAULT_QUEUE_JUMP_REASON_OPTIONS;
  const pauseReasons = state.pauseReasonOptions.length ? state.pauseReasonOptions : DEFAULT_PAUSE_REASON_OPTIONS;
  const lossReasons = state.lossReasonOptions.length ? state.lossReasonOptions : DEFAULT_LOSS_REASON_OPTIONS;
  const isLossReasonMultiple = state.modalConfig?.lossReasonSelectionMode === "multiple";
  const lossReasonConfiguredDetailMode = ["off", "shared", "per-item"].includes(state.modalConfig?.lossReasonDetailMode)
    ? state.modalConfig.lossReasonDetailMode
    : "off";
  const selectedLossReasons =
    outcome === "nao-compra"
      ? sampleItems(
          lossReasons,
          Math.max(
            1,
            Math.min(lossReasons.length, isLossReasonMultiple && lossReasons.length > 1 ? 2 : 1)
          )
        )
      : [];
  const lossReasonDetails =
    lossReasonConfiguredDetailMode === "off"
      ? {}
      : lossReasonConfiguredDetailMode === "per-item"
        ? Object.fromEntries(selectedLossReasons.map((item) => [item.id, `Perda por ${item.label.toLowerCase()}`]))
        : Object.fromEntries(selectedLossReasons.map((item) => [item.id, "Cliente ainda avaliando a compra."]));
  const names = ["Ana", "Carla", "Bruna", "Mariana", "Paula", "Julia", "Erika"];
  const professions = state.professionOptions.length ? state.professionOptions : DEFAULT_PROFESSION_OPTIONS;
  const customerName = `${names[randomInt(0, names.length - 1)]} Teste`;

  const seenProductEntry = {
    id: seenProduct.id,
    name: seenProduct.name,
    code: String(seenProduct.code || "").trim(),
    price: seenProduct.basePrice
  };
  const closedProductEntry = closedProduct
    ? {
        id: closedProduct.id,
        name: closedProduct.name,
        code: String(closedProduct.code || "").trim(),
        price: closedProduct.basePrice
      }
    : null;

  return {
    outcome,
    isWindowService: Math.random() < 0.3,
    isGift: outcome === "compra" || outcome === "reserva" ? Math.random() < 0.5 : false,
    isExistingCustomer: Math.random() < 0.5,
    productSeen: seenProduct.name,
    productSeenNotes: `Cliente demonstrou interesse em ${seenProduct.name.toLowerCase()}.`,
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
    lossReasons: selectedLossReasons.map((item) => item.id),
    lossReasonDetails,
    lossReasonId: selectedLossReasons[0]?.id || "",
    lossReason: selectedLossReasons.map((item) => item.label).join(", "),
    queueJumpReason:
      service.startMode === "queue-jump"
        ? queueJumpReasons[randomInt(0, queueJumpReasons.length - 1)]?.label || ""
        : "",
    pauseReason: pauseReasons[randomInt(0, pauseReasons.length - 1)]?.label || "",
    notes: "Preenchimento automatico em modo teste."
  };
}

export function hydrateState(nextState = {}) {
  const sourceState = nextState && typeof nextState === "object" ? nextState : {};
  const baseState = createEmptyState();
  const now = Date.now();
  const stores = normalizeStoreList(sourceState.stores, baseState.stores);
  const activeStoreId = stores.some((store) => store.id === sourceState.activeStoreId)
    ? sourceState.activeStoreId
    : stores[0]?.id || baseState.activeStoreId;
  const activeStoreDescriptor = stores.find((store) => store.id === activeStoreId) || stores[0] || null;
  const legacyScopedState = {
    selectedConsultantId: sourceState.selectedConsultantId,
    consultantSimulationAdditionalSales: sourceState.consultantSimulationAdditionalSales,
    waitingList: sourceState.waitingList,
    activeServices:
      sourceState.activeServices ||
      (sourceState.inService ? [sourceState.inService] : []),
    roster: sourceState.roster,
    consultantActivitySessions: sourceState.consultantActivitySessions,
    consultantCurrentStatus: sourceState.consultantCurrentStatus,
    pausedEmployees: sourceState.pausedEmployees,
    serviceHistory: sourceState.serviceHistory
  };
  const activeScopedFallback = createEmptyStoreScopedState(
    Array.isArray(sourceState.roster) && sourceState.roster.length ? sourceState.roster : []
  );
  const normalizedLegacyActiveSnapshot = normalizeStoreScopedState(
    legacyScopedState,
    activeScopedFallback,
    activeStoreDescriptor,
    now
  );
  const rawSnapshots =
    sourceState.storeSnapshots && typeof sourceState.storeSnapshots === "object"
      ? sourceState.storeSnapshots
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
  const sourceFinishModalIdentifier = String(
    sourceState.finishModalServiceId || sourceState.finishModalPersonId || ""
  ).trim();
  const resolvedFinishModalService = sourceFinishModalIdentifier
    ? resolvedActiveSnapshot.activeServices.find((service) => service.serviceId === sourceFinishModalIdentifier) ||
      resolvedActiveSnapshot.activeServices.find((service) => service.id === sourceFinishModalIdentifier)
    : null;
  const finishModalServiceId = resolvedFinishModalService?.serviceId || null;
  const profiles =
    Array.isArray(sourceState.profiles) && sourceState.profiles.length
      ? sourceState.profiles
      : baseState.profiles;
  const activeProfileId = profiles.some((profile) => profile.id === sourceState.activeProfileId)
    ? sourceState.activeProfileId
    : profiles[0]?.id || baseState.activeProfileId;
  const activeProfile = profiles.find((profile) => profile.id === activeProfileId) || profiles[0] || null;
  const allowedWorkspaces = getAllowedWorkspaces(activeProfile?.role);
  const selectedWorkspace = allowedWorkspaces.includes(sourceState.activeWorkspace)
    ? sourceState.activeWorkspace
    : allowedWorkspaces[0] || "operacao";

  return {
    ...baseState,
    ...sourceState,
    configSchemaVersion: baseState.configSchemaVersion,
    serverClockOffsetMs: Number(sourceState.serverClockOffsetMs || 0) || 0,
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
      sourceState.selectedOperationTemplateId || baseState.selectedOperationTemplateId,
    reportFilters: normalizeReportFilters(sourceState.reportFilters || baseState.reportFilters),
    campaigns: Array.isArray(sourceState.campaigns)
      ? sourceState.campaigns.map((item) => normalizeCampaign(item))
      : cloneValue(baseState.campaigns),
    finishModalDraft: finishModalServiceId ? cloneValue(sourceState.finishModalDraft || null) : null,
    waitingList: resolvedActiveSnapshot.waitingList,
    activeServices: resolvedActiveSnapshot.activeServices,
    roster: resolvedActiveSnapshot.roster,
    visitReasonOptions:
      Array.isArray(sourceState.visitReasonOptions) && sourceState.visitReasonOptions.length
        ? normalizeVisitReasonOptions(sourceState.visitReasonOptions)
        : baseState.visitReasonOptions,
    customerSourceOptions:
      Array.isArray(sourceState.customerSourceOptions) && sourceState.customerSourceOptions.length
        ? sourceState.customerSourceOptions
        : baseState.customerSourceOptions,
    pauseReasonOptions:
      Array.isArray(sourceState.pauseReasonOptions) && sourceState.pauseReasonOptions.length
        ? normalizeSimpleOptions(sourceState.pauseReasonOptions)
        : baseState.pauseReasonOptions,
    cancelReasonOptions:
      Array.isArray(sourceState.cancelReasonOptions)
        ? normalizeSimpleOptions(sourceState.cancelReasonOptions)
        : baseState.cancelReasonOptions,
    stopReasonOptions:
      Array.isArray(sourceState.stopReasonOptions)
        ? normalizeSimpleOptions(sourceState.stopReasonOptions)
        : baseState.stopReasonOptions,
    queueJumpReasonOptions:
      Array.isArray(sourceState.queueJumpReasonOptions) && sourceState.queueJumpReasonOptions.length
        ? normalizeSimpleOptions(sourceState.queueJumpReasonOptions)
        : baseState.queueJumpReasonOptions,
    lossReasonOptions:
      Array.isArray(sourceState.lossReasonOptions) && sourceState.lossReasonOptions.length
        ? normalizeSimpleOptions(sourceState.lossReasonOptions)
        : baseState.lossReasonOptions,
    professionOptions:
      Array.isArray(sourceState.professionOptions) && sourceState.professionOptions.length
        ? sourceState.professionOptions
        : baseState.professionOptions,
    productCatalog:
      Array.isArray(sourceState.productCatalog) && sourceState.productCatalog.length
        ? sourceState.productCatalog.map((product) => ({
            ...product,
            code: String(product?.code || "").trim(),
            name: String(product?.name || "").trim(),
            category: String(product?.category || "").trim(),
            basePrice: Math.max(0, Number(product?.basePrice || 0) || 0)
          }))
        : baseState.productCatalog,
    modalConfig: {
      ...baseState.modalConfig,
      ...sourceState.modalConfig
    },
    pausedEmployees: resolvedActiveSnapshot.pausedEmployees,
    consultantActivitySessions: resolvedActiveSnapshot.consultantActivitySessions,
    consultantCurrentStatus: resolvedActiveSnapshot.consultantCurrentStatus,
    settings: {
      ...baseState.settings,
      ...sourceState.settings
    },
    serviceHistory: resolvedActiveSnapshot.serviceHistory,
    isReady: true,
    finishModalServiceId
  };
}
