import { ref } from "vue";
import { defineStore } from "pinia";
import { mockQueueState } from "@core/data/mock-queue";
import { createAppStore } from "@core/domain/app-store";
import { cloneValue } from "@core/utils/object";
import { loadQueueState, saveQueueState } from "~/utils/queue-storage";

export const useDashboardStore = defineStore("dashboard", () => {
  const state = ref(cloneValue(mockQueueState));
  let runtimeStore = null;
  let unsubscribe = null;
  let initialized = false;
  let persistenceEnabled = false;

  function replaceState(nextState) {
    state.value = cloneValue(nextState || mockQueueState);
  }

  function getRuntimeStore() {
    if (!runtimeStore) {
      runtimeStore = createAppStore();
    }

    return runtimeStore;
  }

  function bindRuntimeStore(store) {
    if (unsubscribe) {
      return;
    }

    unsubscribe = store.subscribe((nextState) => {
      replaceState(nextState);

      if (import.meta.client && persistenceEnabled) {
        saveQueueState(nextState);
      }
    });
  }

  async function ensure() {
    const store = getRuntimeStore();

    if (!initialized) {
      bindRuntimeStore(store);

      if (import.meta.client) {
        const hydratedState = await loadQueueState();
        store.hydrate(hydratedState);
        persistenceEnabled = true;
      } else {
        replaceState(store.getState());
      }

      initialized = true;
    } else {
      replaceState(store.getState());
    }

    return store;
  }

  async function withStore(handler) {
    const store = await ensure();

    if (!store) {
      return null;
    }

    const result = await handler(store);
    replaceState(store.getState());
    return result;
  }

  return {
    state,
    ensure,
    setWorkspace(workspaceId) {
      return withStore((store) => store.setWorkspace(workspaceId));
    },
    hydrate(nextState) {
      const store = getRuntimeStore();
      bindRuntimeStore(store);
      store.hydrate(nextState);
      if (import.meta.client) {
        persistenceEnabled = true;
        saveQueueState(store.getState());
      }
      initialized = true;
      replaceState(store.getState());
      return store.getState();
    },
    setActiveProfile(profileId) {
      return withStore((store) => store.setActiveProfile(profileId));
    },
    setActiveStore(storeId) {
      return withStore((store) => store.setActiveStore(storeId));
    },
    setSelectedConsultant(personId) {
      return withStore((store) => store.setSelectedConsultant(personId));
    },
    setConsultantSimulationAdditionalSales(amount) {
      return withStore((store) => store.setConsultantSimulationAdditionalSales(amount));
    },
    createStore(payload) {
      return withStore((store) => store.createStore(payload));
    },
    updateStore(storeId, patch) {
      return withStore((store) => store.updateStore(storeId, patch));
    },
    archiveStore(storeId) {
      return withStore((store) => store.archiveStore(storeId));
    },
    updateReportFilter(filterId, value) {
      return withStore((store) => store.updateReportFilter(filterId, value));
    },
    resetReportFilters() {
      return withStore((store) => store.resetReportFilters());
    },
    createCampaign(payload) {
      return withStore((store) => store.createCampaign(payload));
    },
    updateCampaign(campaignId, patch) {
      return withStore((store) => store.updateCampaign(campaignId, patch));
    },
    removeCampaign(campaignId) {
      return withStore((store) => store.removeCampaign(campaignId));
    },
    updateSetting(settingId, value) {
      return withStore((store) => store.updateSetting(settingId, value));
    },
    updateModalConfig(configKey, value) {
      return withStore((store) => store.updateModalConfig(configKey, value));
    },
    applyOperationTemplate(templateId) {
      return withStore((store) => store.applyOperationTemplate(templateId));
    },
    addVisitReasonOption(label) {
      return withStore((store) => store.addVisitReasonOption(label));
    },
    updateVisitReasonOption(optionId, label) {
      return withStore((store) => store.updateVisitReasonOption(optionId, label));
    },
    removeVisitReasonOption(optionId) {
      return withStore((store) => store.removeVisitReasonOption(optionId));
    },
    addCustomerSourceOption(label) {
      return withStore((store) => store.addCustomerSourceOption(label));
    },
    updateCustomerSourceOption(optionId, label) {
      return withStore((store) => store.updateCustomerSourceOption(optionId, label));
    },
    removeCustomerSourceOption(optionId) {
      return withStore((store) => store.removeCustomerSourceOption(optionId));
    },
    addProfessionOption(label) {
      return withStore((store) => store.addProfessionOption(label));
    },
    updateProfessionOption(optionId, label) {
      return withStore((store) => store.updateProfessionOption(optionId, label));
    },
    removeProfessionOption(optionId) {
      return withStore((store) => store.removeProfessionOption(optionId));
    },
    addCatalogProduct(name, category, basePrice) {
      return withStore((store) => store.addCatalogProduct(name, category, basePrice));
    },
    updateCatalogProduct(productId, patch) {
      return withStore((store) => store.updateCatalogProduct(productId, patch));
    },
    removeCatalogProduct(productId) {
      return withStore((store) => store.removeCatalogProduct(productId));
    },
    createConsultantProfile(payload) {
      return withStore((store) => store.createConsultantProfile(payload));
    },
    updateConsultantProfile(consultantId, patch) {
      return withStore((store) => store.updateConsultantProfile(consultantId, patch));
    },
    archiveConsultantProfile(consultantId) {
      return withStore((store) => store.archiveConsultantProfile(consultantId));
    },
    addToQueue(personId) {
      return withStore((store) => store.addToQueue(personId));
    },
    pauseEmployee(personId, reason) {
      return withStore((store) => store.pauseEmployee(personId, reason));
    },
    resumeEmployee(personId) {
      return withStore((store) => store.resumeEmployee(personId));
    },
    startService(personId = null) {
      return withStore((store) => store.startService(personId));
    },
    openFinishModal(personId) {
      return withStore((store) => store.openFinishModal(personId));
    },
    closeFinishModal() {
      return withStore((store) => store.closeFinishModal());
    },
    finishService(personId, closureData) {
      return withStore((store) => store.finishService(personId, closureData));
    }
  };
});
