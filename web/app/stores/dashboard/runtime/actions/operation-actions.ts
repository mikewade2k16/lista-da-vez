import { applyCampaignsToHistoryEntry } from "~/domain/utils/campaigns";
import { appendUniqueOption } from "~/stores/dashboard/runtime/shared";
import { applyStatusTransitions } from "~/stores/dashboard/runtime/status";
import { buildRandomFinishModalDraft } from "~/stores/dashboard/runtime/state";

const FINISH_OUTCOMES = new Set(["reserva", "compra", "nao-compra"]);

function createServiceId(personId) {
  return `${personId}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function deriveQueuePositionAtStart(targetService, activeServices = [], serviceHistory = []) {
  if (typeof targetService?.queuePositionAtStart === "number" && targetService.queuePositionAtStart > 0) {
    return targetService.queuePositionAtStart;
  }

  const targetConsultantId = String(targetService?.id || "").trim();
  const targetGroupId = String(targetService?.parallelGroupId || "").trim();
  const targetServiceId = String(targetService?.serviceId || "").trim();

  const hasMatchingGroup = (entry) => {
    if (targetGroupId) {
      return String(entry?.parallelGroupId || "").trim() === targetGroupId;
    }

    const siblingServiceIds = Array.isArray(entry?.siblingServiceIds) ? entry.siblingServiceIds : [];
    return siblingServiceIds.includes(targetServiceId);
  };

  const activeMatch = (Array.isArray(activeServices) ? activeServices : []).find((service) => {
    if (String(service?.serviceId || "").trim() === targetServiceId) {
      return false;
    }
    if (String(service?.id || "").trim() !== targetConsultantId) {
      return false;
    }
    if (!hasMatchingGroup(service)) {
      return false;
    }
    return typeof service?.queuePositionAtStart === "number" && service.queuePositionAtStart > 0;
  });

  if (typeof activeMatch?.queuePositionAtStart === "number" && activeMatch.queuePositionAtStart > 0) {
    return activeMatch.queuePositionAtStart;
  }

  const historyMatch = (Array.isArray(serviceHistory) ? serviceHistory : []).find((entry) => {
    if (String(entry?.personId || "").trim() !== targetConsultantId) {
      return false;
    }
    if (!hasMatchingGroup(entry)) {
      return false;
    }
    return typeof entry?.queuePositionAtStart === "number" && entry.queuePositionAtStart > 0;
  });

  return typeof historyMatch?.queuePositionAtStart === "number" && historyMatch.queuePositionAtStart > 0
    ? historyMatch.queuePositionAtStart
    : 1;
}

function deriveSequentialServiceFinishedAt(targetService, activeServices = [], serviceHistory = [], now = Date.now()) {
  const targetConsultantId = String(targetService?.id || "").trim();
  const targetGroupId = String(targetService?.parallelGroupId || "").trim();
  const targetServiceId = String(targetService?.serviceId || "").trim();
  const targetStartedAt = Number(targetService?.serviceStartedAt || 0) || 0;
  let finishedAt = 0;

  const belongsToSequence = (entry) => {
    if (targetGroupId) {
      return String(entry?.parallelGroupId || "").trim() === targetGroupId;
    }

    const siblingServiceIds = Array.isArray(entry?.siblingServiceIds) ? entry.siblingServiceIds : [];
    return siblingServiceIds.includes(targetServiceId);
  };

  const consider = (candidateStartedAt) => {
    const normalizedStartedAt = Number(candidateStartedAt || 0) || 0;
    if (normalizedStartedAt <= targetStartedAt) {
      return;
    }
    if (!finishedAt || normalizedStartedAt < finishedAt) {
      finishedAt = normalizedStartedAt;
    }
  };

  consider(targetService?.effectiveFinishedAt);
  consider(targetService?.stoppedAt);

  (Array.isArray(activeServices) ? activeServices : []).forEach((service) => {
    if (String(service?.serviceId || "").trim() === targetServiceId) {
      return;
    }
    if (String(service?.id || "").trim() !== targetConsultantId) {
      return;
    }
    if (!belongsToSequence(service)) {
      return;
    }
    consider(service?.serviceStartedAt);
  });

  (Array.isArray(serviceHistory) ? serviceHistory : []).forEach((entry) => {
    if (String(entry?.personId || "").trim() !== targetConsultantId) {
      return;
    }
    if (!belongsToSequence(entry)) {
      return;
    }
    consider(entry?.startedAt);
  });

  return finishedAt || Math.max(now, targetStartedAt);
}

export function createOperationActions({ getState, updateState }) {
  return {
    addToQueue(personId) {
      const state = getState();
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
      const state = getState();

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
      const state = getState();
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
      const state = getState();

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

    startParallelService(personId) {
      const state = getState();
      const now = Date.now();
      const consultant = state.roster.find((item) => item.id === personId);
      const consultantServices = state.activeServices.filter((item) => item.id === personId);
      const maxPerConsultant = state.settings.maxConcurrentServicesPerConsultant || 1;

      if (!consultant || consultantServices.length >= maxPerConsultant) {
        return;
      }

      const firstService = consultantServices[0];
      const parallelGroupId = firstService?.parallelGroupId || createServiceId(personId);
      const parallelStartIndex = consultantServices.length + 1;
      const startOffsetMs = Math.max(0, now - (firstService?.serviceStartedAt || now));
      const siblingServiceIds = consultantServices.map((s) => s.serviceId);

      const serviceEntry = {
        ...consultant,
        serviceId: createServiceId(personId),
        serviceStartedAt: now,
        queueJoinedAt: Number(firstService?.queueJoinedAt || now),
        queueWaitMs: Number(firstService?.queueWaitMs || 0),
        queuePositionAtStart: deriveQueuePositionAtStart(firstService || consultant, state.activeServices, state.serviceHistory),
        startMode: "parallel",
        skippedPeople: Array.isArray(firstService?.skippedPeople) ? firstService.skippedPeople : [],
        parallelGroupId,
        parallelStartIndex,
        startOffsetMs,
        siblingServiceIds
      };

      updateState({
        ...state,
        activeServices: [...state.activeServices, serviceEntry]
      });
    },

    openFinishModal(serviceId) {
      const state = getState();
      const activeService = state.activeServices.find((item) => item.serviceId === serviceId);

      if (!activeService) {
        return;
      }

      updateState({
        ...state,
        finishModalServiceId: serviceId,
        finishModalDraft: buildRandomFinishModalDraft(state, activeService)
      });
    },

    closeFinishModal() {
      const state = getState();

      updateState({
        ...state,
        finishModalServiceId: null,
        finishModalDraft: null
      });
    },

    finishService(serviceId, closureData) {
      const state = getState();

      if (!FINISH_OUTCOMES.has(closureData?.outcome)) {
        return;
      }

      const now = Date.now();
      const serviceIndex = state.activeServices.findIndex((item) => item.serviceId === serviceId);

      if (serviceIndex === -1) {
        return;
      }

      const activeService = state.activeServices[serviceIndex];
      const personId = activeService.id;
      const finishedAt = now;
      const effectiveFinishedAt = deriveSequentialServiceFinishedAt(activeService, state.activeServices, state.serviceHistory, now);
      const nextActiveServices = state.activeServices.filter((item) => item.serviceId !== serviceId);
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
        finishedAt: effectiveFinishedAt,
        durationMs: Math.max(0, effectiveFinishedAt - Number(activeService.serviceStartedAt || 0)),
        finishOutcome: closureData.outcome,
        startMode: activeService.startMode,
        queuePositionAtStart: deriveQueuePositionAtStart(activeService, state.activeServices, state.serviceHistory),
        queueWaitMs: Number(activeService.queueWaitMs || 0),
        skippedPeople: activeService.skippedPeople,
        skippedCount: activeService.skippedPeople.length,
        isWindowService: closureData.isWindowService,
        isGift: closureData.isGift,
        productSeen: closureData.productSeen,
        productClosed: closureData.productClosed,
        productDetails: closureData.productClosed || closureData.productSeen || closureData.productDetails,
        productsSeen: Array.isArray(closureData.productsSeen) ? closureData.productsSeen : [],
        productsClosed: Array.isArray(closureData.productsClosed) ? closureData.productsClosed : [],
        productsSeenNone: Boolean(closureData.productsSeenNone),
        visitReasonsNotInformed: Boolean(closureData.visitReasonsNotInformed),
        customerSourcesNotInformed: Boolean(closureData.customerSourcesNotInformed),
        customerName: closureData.customerName,
        customerPhone: closureData.customerPhone,
        customerEmail: closureData.customerEmail,
        isExistingCustomer: closureData.isExistingCustomer,
        visitReasons: closureData.visitReasons,
        visitReasonDetails: closureData.visitReasonDetails,
        customerSources: closureData.customerSources,
        customerSourceDetails: closureData.customerSourceDetails,
        lossReasons: closureData.lossReasons,
        lossReasonDetails: closureData.lossReasonDetails,
        lossReasonId: closureData.lossReasonId,
        lossReason: closureData.lossReason,
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

      const remainingConsultantServices = nextActiveServices.filter((item) => item.id === personId);
      const shouldReturnToQueue = remainingConsultantServices.length === 0;
      const nextWaitingList = shouldReturnToQueue
        ? [
            ...state.waitingList,
            {
              ...(state.roster.find((item) => item.id === personId) || activeService),
              queueJoinedAt: now
            }
          ]
        : state.waitingList;
      const statusTransitions = shouldReturnToQueue
        ? [{ personId, nextStatus: "queue" }]
        : [];

      updateState({
        ...state,
        waitingList: nextWaitingList,
        activeServices: nextActiveServices,
        professionOptions: nextProfessionOptions,
        serviceHistory: [...state.serviceHistory, finalizedHistoryEntry],
        finishModalServiceId: null,
        finishModalDraft: null,
        ...applyStatusTransitions(state, statusTransitions, now)
      });
    }
  };
}
