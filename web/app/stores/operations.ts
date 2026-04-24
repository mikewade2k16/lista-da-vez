import { computed, ref } from "vue";
import { defineStore, storeToRefs } from "pinia";

import { applyCampaignsToHistoryEntry } from "~/domain/utils/campaigns";
import { cloneValue } from "~/domain/utils/object";
import { useAuthStore } from "~/stores/auth";
import { useAppRuntimeStore } from "~/stores/app-runtime";
import { useSettingsStore } from "~/stores/settings";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";
import { applyOperationSnapshotToState, hydrateRuntimeStoreContext } from "~/utils/runtime-remote";

function normalizeCampaignMatches(matches = []) {
  return (Array.isArray(matches) ? matches : []).map((match) => ({
    id: String(match?.id || "").trim(),
    name: String(match?.name || "").trim(),
    bonusAmount: Math.max(0, Number(match?.bonusAmount || 0) || 0)
  })).filter((match) => match.id || match.name);
}

function normalizeProductEntries(products = []) {
  return (Array.isArray(products) ? products : []).map((product, index) => {
    const id = String(product?.id || "").trim();
    const name = String(product?.name || product?.label || "").trim();
    const code = String(product?.code || "").trim().toUpperCase();

    return {
      id: id || `product-${index + 1}`,
      name,
      code,
      price: Math.max(0, Number(product?.price ?? product?.basePrice ?? 0) || 0),
      isCustom: Boolean(product?.isCustom)
    };
  }).filter((product) => product.id || product.name || product.code);
}

function normalizeText(value) {
  return String(value || "").trim();
}

function normalizeStringEntries(values = []) {
  const seen = new Set();

  return (Array.isArray(values) ? values : []).map((value) => normalizeText(value)).filter((value) => {
    if (!value || seen.has(value)) {
      return false;
    }

    seen.add(value);
    return true;
  });
}

function normalizeDetailMap(value) {
  return Object.entries(value || {}).reduce((accumulator, [rawKey, rawValue]) => {
    const key = normalizeText(rawKey);
    const detail = normalizeText(rawValue);

    if (!key || !detail) {
      return accumulator;
    }

    accumulator[key] = detail;
    return accumulator;
  }, {});
}

export const useOperationsStore = defineStore("operations", () => {
  const runtimeConfig = useRuntimeConfig();
  const runtime = useAppRuntimeStore();
  const auth = useAuthStore();
  const settingsStore = useSettingsStore();
  const { state } = storeToRefs(runtime);
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);

  const waitingList = computed(() => state.value.waitingList || []);
  const activeServices = computed(() => state.value.activeServices || []);
  const finishModalDraft = computed(() => state.value.finishModalDraft);
  const overview = ref(null);
  const overviewPending = ref(false);
  const overviewError = ref("");

  async function resolveActiveStoreId() {
    await runtime.ensure();

    if (auth.isAuthenticated) {
      await auth.ensureSession();
    }

    return String(auth.activeStoreId || runtime.state.activeStoreId || "").trim();
  }

  async function refreshActiveStore() {
    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      return null;
    }

    return hydrateRuntimeStoreContext(runtime, apiRequest, storeId);
  }

  async function refreshOperationSnapshot(storeId, options = {}) {
    const normalizedStoreId = String(storeId || "").trim();

    if (!normalizedStoreId || !auth.isAuthenticated) {
      return null;
    }

    const snapshot = await apiRequest(`/v1/operations/snapshot?storeId=${encodeURIComponent(normalizedStoreId)}`);
    runtime.hydrate(
      applyOperationSnapshotToState(runtime.state, normalizedStoreId, snapshot, {
        resetFinishModal: Boolean(options?.resetFinishModal)
      })
    );

    return snapshot;
  }

  async function refreshOverview() {
    if (!auth.isAuthenticated) {
      overview.value = null;
      overviewError.value = "";
      return null;
    }

    overviewPending.value = true;
    overviewError.value = "";

    try {
      const response = await apiRequest("/v1/operations/overview");
      overview.value = response;
      return response;
    } catch (error) {
      overviewError.value = getApiErrorMessage(error, "Nao foi possivel carregar a operacao integrada.");
      throw error;
    } finally {
      overviewPending.value = false;
    }
  }

  function clearOverview() {
    overview.value = null;
    overviewPending.value = false;
    overviewError.value = "";
  }

  async function runCommand(path, body = {}, options = {}) {
    const storeId = String(options?.storeId || "").trim() || await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      return { ok: false, message: "Sessao ou loja ativa indisponivel." };
    }

    const previousState = cloneValue(runtime.state);

    try {
      const response = await apiRequest(path, {
        method: "POST",
        body: {
          storeId,
          ...body
        }
      });

      if (response?.snapshot) {
        runtime.hydrate(
          applyOperationSnapshotToState(runtime.state, storeId, response.snapshot, {
            resetFinishModal: Boolean(options?.resetFinishModal)
          })
        );
      } else if (storeId === runtime.state.activeStoreId) {
        await refreshOperationSnapshot(storeId, options);
      }

      if (options?.refreshOverview || overview.value) {
        try {
          await refreshOverview();
        } catch {
          // overview e auxiliar; nao derrubamos o fluxo principal da mutacao
        }
      }

      return { ok: true, response };
    } catch (error) {
      runtime.hydrate(previousState);

      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel atualizar a operacao.")
      };
    }
  }

  return {
    state,
    waitingList,
    activeServices,
    finishModalDraft,
    ensure: runtime.ensure,
    refreshActiveStore,
    refreshOperationSnapshot,
    refreshOverview,
    clearOverview,
    overview,
    overviewPending,
    overviewError,
    setSelectedConsultant(personId) {
      return runtime.run("setSelectedConsultant", personId);
    },
    setConsultantSimulationAdditionalSales(amount) {
      return runtime.run("setConsultantSimulationAdditionalSales", amount);
    },
    addToQueue(personId, storeId = "") {
      return runCommand("/v1/operations/queue", { personId }, {
        storeId,
        refreshOverview: Boolean(storeId)
      });
    },
    pauseEmployee(personId, reason, storeId = "") {
      return runCommand("/v1/operations/pause", { personId, reason }, {
        storeId,
        refreshOverview: Boolean(storeId)
      });
    },
    assignTask(personId, reason, storeId = "") {
      return runCommand("/v1/operations/assign-task", { personId, reason }, {
        storeId,
        refreshOverview: true
      });
    },
    resumeEmployee(personId, storeId = "") {
      return runCommand("/v1/operations/resume", { personId }, {
        storeId,
        refreshOverview: Boolean(storeId)
      });
    },
    startService(personId = null) {
      return runCommand("/v1/operations/start", { personId: personId || "" });
    },
    openFinishModal(personId) {
      return runtime.run("openFinishModal", personId);
    },
    closeFinishModal() {
      return runtime.run("closeFinishModal");
    },
    async finishService(personId, closureData) {
      const normalizedProductsSeen = normalizeProductEntries(closureData?.productsSeen);
      const normalizedProductsClosed = normalizeProductEntries(closureData?.productsClosed);
      const outcome = normalizeText(closureData?.outcome);
      const isSaleOutcome = outcome === "compra" || outcome === "reserva";
      const isLossOutcome = outcome === "nao-compra";
      const productSeen = normalizeText(closureData?.productSeen);
      const productClosed = normalizeText(closureData?.productClosed);
      const productDetails = normalizeText(closureData?.productDetails);
      const customerName = normalizeText(closureData?.customerName);
      const customerPhone = normalizeText(closureData?.customerPhone);
      const customerEmail = normalizeText(closureData?.customerEmail);
      const visitReasons = normalizeStringEntries(closureData?.visitReasons);
      const visitReasonDetails = normalizeDetailMap(closureData?.visitReasonDetails);
      const customerSources = normalizeStringEntries(closureData?.customerSources);
      const customerSourceDetails = normalizeDetailMap(closureData?.customerSourceDetails);
      const lossReasons = normalizeStringEntries(closureData?.lossReasons);
      const lossReasonDetails = normalizeDetailMap(closureData?.lossReasonDetails);
      const lossReasonId = normalizeText(closureData?.lossReasonId);
      const lossReason = normalizeText(closureData?.lossReason);
      const saleAmount = Math.max(0, Number(closureData?.saleAmount || 0) || 0);
      const customerProfession = normalizeText(closureData?.customerProfession);
      const queueJumpReason = normalizeText(closureData?.queueJumpReason);
      const notes = normalizeText(closureData?.notes);
      const activeService = (runtime.state.activeServices || []).find((item) => item.id === personId) || null;
      const activeStore =
        (runtime.state.stores || []).find((store) => store.id === runtime.state.activeStoreId) || null;
      const campaignSeed = activeService
        ? {
            serviceId: activeService.serviceId,
            storeId: runtime.state.activeStoreId,
            storeName: activeStore?.name || "",
            personId: activeService.id,
            personName: activeService.name,
            startedAt: activeService.serviceStartedAt,
            finishedAt: Date.now(),
            durationMs: Math.max(0, Date.now() - Number(activeService.serviceStartedAt || Date.now())),
            finishOutcome: outcome,
            startMode: activeService.startMode,
            queuePositionAtStart: Number(activeService.queuePositionAtStart || 1),
            queueWaitMs: Number(activeService.queueWaitMs || 0),
            skippedPeople: Array.isArray(activeService.skippedPeople) ? activeService.skippedPeople : [],
            skippedCount: Array.isArray(activeService.skippedPeople) ? activeService.skippedPeople.length : 0,
            isWindowService: Boolean(closureData?.isWindowService),
            isGift: isSaleOutcome && Boolean(closureData?.isGift),
            productSeen,
            productClosed: isSaleOutcome ? productClosed : "",
            productDetails: isSaleOutcome ? productDetails : "",
            productsSeen: normalizedProductsSeen,
            productsClosed: isSaleOutcome ? normalizedProductsClosed : [],
            productsSeenNone: Boolean(closureData?.productsSeenNone),
            visitReasonsNotInformed: Boolean(closureData?.visitReasonsNotInformed),
            customerSourcesNotInformed: Boolean(closureData?.customerSourcesNotInformed),
            customerName,
            customerPhone,
            customerEmail,
            isExistingCustomer: Boolean(closureData?.isExistingCustomer),
            visitReasons,
            visitReasonDetails,
            customerSources,
            customerSourceDetails,
            lossReasons: isLossOutcome ? lossReasons : [],
            lossReasonDetails: isLossOutcome ? lossReasonDetails : {},
            lossReasonId: isLossOutcome ? lossReasonId : "",
            lossReason: isLossOutcome ? lossReason : "",
            saleAmount: isSaleOutcome ? saleAmount : 0,
            customerProfession,
            queueJumpReason,
            notes
          }
        : null;
      const campaignResult = campaignSeed
        ? applyCampaignsToHistoryEntry(runtime.state.campaigns || [], campaignSeed)
        : { matches: [], totalBonus: 0 };
      const campaignMatches = normalizeCampaignMatches(
        Array.isArray(closureData?.campaignMatches) ? closureData.campaignMatches : campaignResult.matches
      );
      const campaignBonusTotal = Math.max(
        0,
        Number(closureData?.campaignBonusTotal ?? campaignResult.totalBonus ?? 0) || 0
      );
      const finishPayload = {
        personId,
        outcome
      };

      if (Boolean(closureData?.isWindowService)) {
        finishPayload.isWindowService = true;
      }

      if (productSeen) {
        finishPayload.productSeen = productSeen;
      }

      if (normalizedProductsSeen.length > 0) {
        finishPayload.productsSeen = normalizedProductsSeen;
      }

      if (Boolean(closureData?.productsSeenNone)) {
        finishPayload.productsSeenNone = true;
      }

      if (Boolean(closureData?.visitReasonsNotInformed)) {
        finishPayload.visitReasonsNotInformed = true;
      }

      if (Boolean(closureData?.customerSourcesNotInformed)) {
        finishPayload.customerSourcesNotInformed = true;
      }

      if (customerName) {
        finishPayload.customerName = customerName;
      }

      if (customerPhone) {
        finishPayload.customerPhone = customerPhone;
      }

      if (customerEmail) {
        finishPayload.customerEmail = customerEmail;
      }

      if (Boolean(closureData?.isExistingCustomer)) {
        finishPayload.isExistingCustomer = true;
      }

      if (visitReasons.length > 0) {
        finishPayload.visitReasons = visitReasons;
      }

      if (Object.keys(visitReasonDetails).length > 0) {
        finishPayload.visitReasonDetails = visitReasonDetails;
      }

      if (customerSources.length > 0) {
        finishPayload.customerSources = customerSources;
      }

      if (Object.keys(customerSourceDetails).length > 0) {
        finishPayload.customerSourceDetails = customerSourceDetails;
      }

      if (isSaleOutcome && Boolean(closureData?.isGift)) {
        finishPayload.isGift = true;
      }

      if (isSaleOutcome && productClosed) {
        finishPayload.productClosed = productClosed;
      }

      if (isSaleOutcome && productDetails) {
        finishPayload.productDetails = productDetails;
      }

      if (isSaleOutcome && normalizedProductsClosed.length > 0) {
        finishPayload.productsClosed = normalizedProductsClosed;
      }

      if (isSaleOutcome && saleAmount > 0) {
        finishPayload.saleAmount = saleAmount;
      }

      if (isLossOutcome && lossReasons.length > 0) {
        finishPayload.lossReasons = lossReasons;
      }

      if (isLossOutcome && Object.keys(lossReasonDetails).length > 0) {
        finishPayload.lossReasonDetails = lossReasonDetails;
      }

      if (isLossOutcome && lossReasonId) {
        finishPayload.lossReasonId = lossReasonId;
      }

      if (isLossOutcome && lossReason) {
        finishPayload.lossReason = lossReason;
      }

      if (customerProfession) {
        finishPayload.customerProfession = customerProfession;
      }

      if (queueJumpReason) {
        finishPayload.queueJumpReason = queueJumpReason;
      }

      if (notes) {
        finishPayload.notes = notes;
      }

      if (campaignMatches.length > 0) {
        finishPayload.campaignMatches = campaignMatches;
      }

      if (campaignBonusTotal > 0) {
        finishPayload.campaignBonusTotal = campaignBonusTotal;
      }

      const result = await runCommand(
        "/v1/operations/finish",
        finishPayload,
        { resetFinishModal: true }
      );

      if (result.ok !== false) {
        const hasProfession =
          customerProfession &&
          (runtime.state.professionOptions || []).some((option) => String(option?.label || "").trim() === customerProfession);

        if (customerProfession && !hasProfession) {
          await settingsStore.addProfessionOption(customerProfession);
        }
      }

      return result;
    }
  };
});
