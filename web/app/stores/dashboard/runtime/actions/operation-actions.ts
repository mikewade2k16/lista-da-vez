import { applyCampaignsToHistoryEntry } from "~/domain/utils/campaigns";
import { appendUniqueOption } from "~/stores/dashboard/runtime/shared";
import { applyStatusTransitions } from "~/stores/dashboard/runtime/status";
import { buildRandomFinishModalDraft } from "~/stores/dashboard/runtime/state";

const FINISH_OUTCOMES = new Set(["reserva", "compra", "nao-compra"]);

function createServiceId(personId) {
  return `${personId}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
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

    openFinishModal(personId) {
      const state = getState();
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
      const state = getState();

      updateState({
        ...state,
        finishModalPersonId: null,
        finishModalDraft: null
      });
    },

    finishService(personId, closureData) {
      const state = getState();

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
