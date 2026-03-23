import { mockQueueState } from "../data/mock-queue.js";
import { normalizeCampaign } from "../utils/campaigns.js";
import { cloneValue } from "../utils/object.js";
import { DEFAULT_REPORT_FILTERS, normalizeReportFilters } from "../utils/reports.js";

const STORAGE_KEY = "nexo-queue-state";

function normalizeQueueState(rawState) {
  const baseState = cloneValue(mockQueueState);
  const now = Date.now();
  const sameConfigSchema = rawState?.configSchemaVersion === baseState.configSchemaVersion;
  const templateIds = new Set((baseState.operationTemplates || []).map((template) => template.id));
  const baseRosterById = new Map(baseState.roster.map((consultant) => [consultant.id, consultant]));
  const roster =
    Array.isArray(rawState?.roster) && rawState.roster.length
      ? rawState.roster.map((consultant) => ({
          ...baseRosterById.get(consultant.id),
          ...consultant
        }))
      : baseState.roster;
  const profiles =
    Array.isArray(rawState?.profiles) && rawState.profiles.length
      ? rawState.profiles
      : baseState.profiles;
  const activeProfileId = profiles.some((profile) => profile.id === rawState?.activeProfileId)
    ? rawState.activeProfileId
    : profiles[0]?.id || baseState.activeProfileId;
  const activeWorkspace = [
    "operacao",
    "consultor",
    "ranking",
    "dados",
    "inteligencia",
    "relatorios",
    "campanhas",
    "multiloja",
    "configuracoes"
  ].includes(
    rawState?.activeWorkspace
  )
    ? rawState.activeWorkspace
    : baseState.activeWorkspace;
  const stores =
    Array.isArray(rawState?.stores) && rawState.stores.length
      ? rawState.stores
      : baseState.stores;
  const activeStoreId = stores.some((store) => store.id === rawState?.activeStoreId)
    ? rawState.activeStoreId
    : stores[0]?.id || baseState.activeStoreId;
  const storeSnapshots =
    rawState?.storeSnapshots && typeof rawState.storeSnapshots === "object"
      ? rawState.storeSnapshots
      : cloneValue(baseState.storeSnapshots || {});
  const selectedConsultantId = roster.some((consultant) => consultant.id === rawState?.selectedConsultantId)
    ? rawState.selectedConsultantId
    : roster[0]?.id || null;
  const selectedOperationTemplateId = templateIds.has(rawState?.selectedOperationTemplateId)
    ? rawState.selectedOperationTemplateId
    : baseState.selectedOperationTemplateId;
  const reportFilters = normalizeReportFilters(rawState?.reportFilters || DEFAULT_REPORT_FILTERS);
  const campaigns = Array.isArray(rawState?.campaigns)
    ? rawState.campaigns.map((campaign) => normalizeCampaign(campaign))
    : cloneValue(baseState.campaigns || []);
  const activeServices = Array.isArray(rawState?.activeServices)
    ? rawState.activeServices.map((service, index) => ({
        ...service,
        serviceId:
          service.serviceId ||
          service.serviceSessionId ||
          `${service.id || "service"}-${service.serviceStartedAt || now}-${index}`,
        serviceStartedAt: service.serviceStartedAt || now,
        queueJoinedAt: Number(service.queueJoinedAt || service.serviceStartedAt || now),
        queueWaitMs: Number(service.queueWaitMs || 0),
        startMode: service.startMode || "queue",
        queuePositionAtStart: service.queuePositionAtStart || index + 1,
        skippedPeople: Array.isArray(service.skippedPeople) ? service.skippedPeople : []
      }))
    : rawState?.inService
      ? [
          {
            ...rawState.inService,
            serviceId:
              rawState.inService.serviceId ||
              rawState.inService.serviceSessionId ||
              `legacy-${rawState.inService.id}`,
            serviceStartedAt: rawState.inService.serviceStartedAt || now,
            startMode: rawState.inService.startMode || "queue",
            queuePositionAtStart: rawState.inService.queuePositionAtStart || 1,
            skippedPeople: Array.isArray(rawState.inService.skippedPeople) ? rawState.inService.skippedPeople : []
          }
        ]
      : [];
  const serviceHistory = Array.isArray(rawState?.serviceHistory)
    ? rawState.serviceHistory.map((entry, index) => ({
        ...entry,
        serviceId:
          entry.serviceId ||
          entry.serviceSessionId ||
          `${entry.personId || "service"}-${entry.startedAt || now}-${index}`,
        storeId: entry.storeId || activeStoreId || "",
        storeName: entry.storeName || "",
        finishOutcome: entry.finishOutcome || "nao-compra",
        startMode: entry.startMode || "queue",
        queuePositionAtStart: entry.queuePositionAtStart || 1,
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
      }))
    : [];

  return {
    ...baseState,
    ...rawState,
    configSchemaVersion: baseState.configSchemaVersion,
    profiles,
    activeProfileId,
    stores,
    activeStoreId,
    storeSnapshots,
    activeWorkspace,
    selectedConsultantId,
    consultantSimulationAdditionalSales: Number(rawState?.consultantSimulationAdditionalSales || 0),
    operationTemplates: cloneValue(baseState.operationTemplates || []),
    selectedOperationTemplateId,
    reportFilters,
    campaigns,
    finishModalDraft: null,
    waitingList: Array.isArray(rawState?.waitingList)
      ? rawState.waitingList.map((item) => ({
          ...item,
          queueJoinedAt: Number(item.queueJoinedAt || now)
        }))
      : baseState.waitingList,
    activeServices,
    roster,
    visitReasonOptions:
      sameConfigSchema && Array.isArray(rawState?.visitReasonOptions) && rawState.visitReasonOptions.length
        ? rawState.visitReasonOptions
        : baseState.visitReasonOptions,
    customerSourceOptions:
      sameConfigSchema && Array.isArray(rawState?.customerSourceOptions) && rawState.customerSourceOptions.length
        ? rawState.customerSourceOptions
        : baseState.customerSourceOptions,
    professionOptions:
      sameConfigSchema && Array.isArray(rawState?.professionOptions) && rawState.professionOptions.length
        ? rawState.professionOptions
        : baseState.professionOptions,
    productCatalog:
      sameConfigSchema && Array.isArray(rawState?.productCatalog) && rawState.productCatalog.length
        ? rawState.productCatalog
        : baseState.productCatalog,
    modalConfig: {
      ...baseState.modalConfig,
      ...(sameConfigSchema ? rawState?.modalConfig : {})
    },
    consultantActivitySessions: Array.isArray(rawState?.consultantActivitySessions)
      ? rawState.consultantActivitySessions
      : [],
    consultantCurrentStatus:
      rawState?.consultantCurrentStatus && typeof rawState.consultantCurrentStatus === "object"
        ? rawState.consultantCurrentStatus
        : {},
    pausedEmployees: Array.isArray(rawState?.pausedEmployees) ? rawState.pausedEmployees : [],
    settings: {
      ...baseState.settings,
      ...rawState?.settings
    },
    serviceHistory
  };
}

export async function loadQueueState() {
  if (typeof window === "undefined") {
    return cloneValue(mockQueueState);
  }

  const storedValue = window.localStorage.getItem(STORAGE_KEY);

  if (!storedValue) {
    return cloneValue(mockQueueState);
  }

  try {
    return normalizeQueueState(JSON.parse(storedValue));
  } catch {
    return cloneValue(mockQueueState);
  }
}

export function saveQueueState(state) {
  if (typeof window === "undefined") {
    return;
  }

  const persistedState = {
    configSchemaVersion: state.configSchemaVersion,
    brandName: state.brandName,
    pageTitle: state.pageTitle,
    profiles: state.profiles,
    activeProfileId: state.activeProfileId,
    stores: state.stores,
    activeStoreId: state.activeStoreId,
    storeSnapshots: state.storeSnapshots,
    activeWorkspace: state.activeWorkspace,
    selectedConsultantId: state.selectedConsultantId,
    consultantSimulationAdditionalSales: state.consultantSimulationAdditionalSales,
    selectedOperationTemplateId: state.selectedOperationTemplateId,
    reportFilters: state.reportFilters,
    campaigns: state.campaigns,
    waitingList: state.waitingList,
    activeServices: state.activeServices,
    roster: state.roster,
    visitReasonOptions: state.visitReasonOptions,
    customerSourceOptions: state.customerSourceOptions,
    professionOptions: state.professionOptions,
    productCatalog: state.productCatalog,
    modalConfig: state.modalConfig,
    consultantActivitySessions: state.consultantActivitySessions,
    consultantCurrentStatus: state.consultantCurrentStatus,
    pausedEmployees: state.pausedEmployees,
    settings: state.settings,
    serviceHistory: state.serviceHistory
  };

  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(persistedState));
}
