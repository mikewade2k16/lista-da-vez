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

function normalizeCatalogProductSearchResponse(response, fallback = {}) {
  return {
    sourceKey: normalizeText(response?.sourceKey || fallback.sourceKey || "erp_current"),
    term: normalizeText(response?.term || fallback.term).toUpperCase(),
    limit: Math.max(1, Number(response?.limit || fallback.limit || 10) || 10),
    items: normalizeProductEntries(response?.items)
  };
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

const SERVER_CLOCK_OFFSET_STORAGE_KEY = "ldv_server_clock_offset_ms";

function readStoredServerClockOffset() {
  if (import.meta.server) {
    return 0;
  }

  const parsed = Number(window.sessionStorage.getItem(SERVER_CLOCK_OFFSET_STORAGE_KEY) || 0);
  return Number.isFinite(parsed) ? parsed : 0;
}

function writeStoredServerClockOffset(offsetMs) {
  if (import.meta.server) {
    return;
  }

  const normalizedOffset = Number(offsetMs || 0) || 0;

  if (!normalizedOffset) {
    window.sessionStorage.removeItem(SERVER_CLOCK_OFFSET_STORAGE_KEY);
    return;
  }

  window.sessionStorage.setItem(SERVER_CLOCK_OFFSET_STORAGE_KEY, String(normalizedOffset));
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

  function applyServerClockOffset(offsetMs) {
    const normalizedOffset = Number(offsetMs || 0) || 0;
    const currentOffset = Number(runtime.state?.serverClockOffsetMs || 0) || 0;

    if (normalizedOffset === currentOffset) {
      writeStoredServerClockOffset(normalizedOffset);
      return normalizedOffset;
    }

    runtime.replace({
      ...runtime.state,
      serverClockOffsetMs: normalizedOffset
    });
    writeStoredServerClockOffset(normalizedOffset);
    return normalizedOffset;
  }

  function syncStoredServerClockOffset() {
    const currentOffset = Number(runtime.state?.serverClockOffsetMs || 0) || 0;
    if (currentOffset) {
      return currentOffset;
    }

    const storedOffset = readStoredServerClockOffset();
    if (!storedOffset) {
      return 0;
    }

    return applyServerClockOffset(storedOffset);
  }

  function captureServerClockOffset(savedAt) {
    const serverTimestamp = Date.parse(String(savedAt || "").trim());
    if (!Number.isFinite(serverTimestamp)) {
      return Number(runtime.state?.serverClockOffsetMs || 0) || 0;
    }

    return applyServerClockOffset(serverTimestamp - Date.now());
  }

  async function resolveActiveStoreId() {
    await runtime.ensure();

    if (auth.isAuthenticated) {
      await auth.ensureSession();
    }

    return String(auth.activeStoreId || runtime.state.activeStoreId || "").trim();
  }

  async function refreshActiveStore() {
    syncStoredServerClockOffset();
    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      return null;
    }

    const runtimeContext = await hydrateRuntimeStoreContext(runtime, apiRequest, storeId, auth.activeTenantId);
    auth.applyRuntimeSettingsStatus(runtimeContext);
    return runtimeContext;
  }

  async function refreshOperationSnapshot(storeId, options = {}) {
    syncStoredServerClockOffset();
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
    syncStoredServerClockOffset();
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

  async function searchCatalogProducts(input = {}) {
    syncStoredServerClockOffset();
    const normalizedTerm = normalizeText(input?.term).toUpperCase();
    const normalizedSourceKey = normalizeText(input?.sourceKey || "erp_current") || "erp_current";
    const normalizedLimit = Math.max(1, Math.min(25, Number(input?.limit || 10) || 10));
    const storeId = normalizeText(input?.storeId) || await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated || normalizedTerm.length < 3) {
      return normalizeCatalogProductSearchResponse(null, {
        sourceKey: normalizedSourceKey,
        term: normalizedTerm,
        limit: normalizedLimit
      });
    }

    const params = new URLSearchParams({
      storeId,
      term: normalizedTerm,
      limit: String(normalizedLimit)
    });

    if (normalizedSourceKey) {
      params.set("sourceKey", normalizedSourceKey);
    }

    const response = await apiRequest(`/v1/catalog/products/search?${params.toString()}`);
    return normalizeCatalogProductSearchResponse(response, {
      sourceKey: normalizedSourceKey,
      term: normalizedTerm,
      limit: normalizedLimit
    });
  }

  function clearOverview() {
    overview.value = null;
    overviewPending.value = false;
    overviewError.value = "";
  }

  async function runCommand(path, body = {}, options = {}) {
    syncStoredServerClockOffset();
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

      captureServerClockOffset(response?.savedAt);

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
    searchCatalogProducts,
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
    openFinishModal(serviceId) {
      return runtime.run("openFinishModal", serviceId);
    },
    closeFinishModal() {
      return runtime.run("closeFinishModal");
    },
    startParallelService(personId, storeId = "") {
      return runCommand("/v1/operations/services/parallel", { personId }, {
        storeId,
        refreshOverview: Boolean(storeId)
      });
    },
    serviceAction(serviceId, action, reason = "", options = {}) {
      const normalizedAction = normalizeText(action).toLowerCase();
      const payload = {
        serviceId,
        action: normalizedAction
      };

      if (normalizedAction === "stop") {
        payload.stopReason = normalizeText(reason);
      }

      if (normalizedAction === "cancel") {
        payload.cancelReason = normalizeText(reason);
      }

      return runCommand("/v1/operations/finish", payload, {
        storeId: normalizeText(options?.storeId),
        refreshOverview: Boolean(options?.storeId),
        resetFinishModal: true
      });
    },
    async finishService(serviceId, closureData, options = {}) {
      const normalizedProductsSeen = normalizeProductEntries(closureData?.productsSeen);
      const normalizedProductsClosed = normalizeProductEntries(closureData?.productsClosed);
      const outcome = normalizeText(closureData?.outcome);
      const isSaleOutcome = outcome === "compra" || outcome === "reserva";
      const isLossOutcome = outcome === "nao-compra";
      const productSeen = normalizeText(closureData?.productSeen);
      const productClosed = normalizeText(closureData?.productClosed);
      const purchaseCode = normalizeText(closureData?.purchaseCode);
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
      const serviceContext = options?.service && typeof options.service === "object" ? options.service : null;
      const activeService = serviceContext || (runtime.state.activeServices || []).find((item) => item.serviceId === serviceId) || null;
      const targetStoreId = normalizeText(options?.storeId || activeService?.storeId) || runtime.state.activeStoreId;
      const activeStore =
        (runtime.state.stores || []).find((store) => store.id === targetStoreId) || null;
      const campaignSeed = activeService
        ? {
            serviceId: activeService.serviceId,
            storeId: targetStoreId,
            storeName: normalizeText(options?.storeName || activeService?.storeName || activeStore?.name),
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
            purchaseCode: outcome === "compra" ? purchaseCode : "",
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
        serviceId,
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

      if (outcome === "compra" && purchaseCode) {
        finishPayload.purchaseCode = purchaseCode;
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
        {
          storeId: targetStoreId,
          resetFinishModal: true,
          refreshOverview: Boolean(targetStoreId)
        }
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
