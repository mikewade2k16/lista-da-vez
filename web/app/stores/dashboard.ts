import { defineStore, storeToRefs } from "pinia";
import { useAppRuntimeStore } from "~/stores/app-runtime";

export const useDashboardStore = defineStore("dashboard", () => {
  const runtime = useAppRuntimeStore();
  const { state } = storeToRefs(runtime);

  return {
    state,
    ensure: runtime.ensure,
    setWorkspace(workspaceId) {
      return runtime.run("setWorkspace", workspaceId);
    },
    hydrate: runtime.hydrate,
    setActiveProfile(profileId) {
      return runtime.run("setActiveProfile", profileId);
    },
    setActiveStore(storeId) {
      return runtime.run("setActiveStore", storeId);
    },
    setSelectedConsultant(personId) {
      return runtime.run("setSelectedConsultant", personId);
    },
    setConsultantSimulationAdditionalSales(amount) {
      return runtime.run("setConsultantSimulationAdditionalSales", amount);
    },
    createStore(payload) {
      return runtime.run("createStore", payload);
    },
    updateStore(storeId, patch) {
      return runtime.run("updateStore", storeId, patch);
    },
    archiveStore(storeId) {
      return runtime.run("archiveStore", storeId);
    },
    updateReportFilter(filterId, value) {
      return runtime.run("updateReportFilter", filterId, value);
    },
    resetReportFilters() {
      return runtime.run("resetReportFilters");
    },
    createCampaign(payload) {
      return runtime.run("createCampaign", payload);
    },
    updateCampaign(campaignId, patch) {
      return runtime.run("updateCampaign", campaignId, patch);
    },
    removeCampaign(campaignId) {
      return runtime.run("removeCampaign", campaignId);
    },
    updateSetting(settingId, value) {
      return runtime.run("updateSetting", settingId, value);
    },
    updateModalConfig(configKey, value) {
      return runtime.run("updateModalConfig", configKey, value);
    },
    applyOperationTemplate(templateId) {
      return runtime.run("applyOperationTemplate", templateId);
    },
    addVisitReasonOption(label) {
      return runtime.run("addVisitReasonOption", label);
    },
    updateVisitReasonOption(optionId, label) {
      return runtime.run("updateVisitReasonOption", optionId, label);
    },
    removeVisitReasonOption(optionId) {
      return runtime.run("removeVisitReasonOption", optionId);
    },
    addCustomerSourceOption(label) {
      return runtime.run("addCustomerSourceOption", label);
    },
    updateCustomerSourceOption(optionId, label) {
      return runtime.run("updateCustomerSourceOption", optionId, label);
    },
    removeCustomerSourceOption(optionId) {
      return runtime.run("removeCustomerSourceOption", optionId);
    },
    addQueueJumpReasonOption(label) {
      return runtime.run("addQueueJumpReasonOption", label);
    },
    updateQueueJumpReasonOption(optionId, label) {
      return runtime.run("updateQueueJumpReasonOption", optionId, label);
    },
    removeQueueJumpReasonOption(optionId) {
      return runtime.run("removeQueueJumpReasonOption", optionId);
    },
    addLossReasonOption(label) {
      return runtime.run("addLossReasonOption", label);
    },
    updateLossReasonOption(optionId, label) {
      return runtime.run("updateLossReasonOption", optionId, label);
    },
    removeLossReasonOption(optionId) {
      return runtime.run("removeLossReasonOption", optionId);
    },
    addProfessionOption(label) {
      return runtime.run("addProfessionOption", label);
    },
    updateProfessionOption(optionId, label) {
      return runtime.run("updateProfessionOption", optionId, label);
    },
    removeProfessionOption(optionId) {
      return runtime.run("removeProfessionOption", optionId);
    },
    addCatalogProduct(name, category, basePrice, code) {
      return runtime.run("addCatalogProduct", name, category, basePrice, code);
    },
    updateCatalogProduct(productId, patch) {
      return runtime.run("updateCatalogProduct", productId, patch);
    },
    removeCatalogProduct(productId) {
      return runtime.run("removeCatalogProduct", productId);
    },
    createConsultantProfile(payload) {
      return runtime.run("createConsultantProfile", payload);
    },
    updateConsultantProfile(consultantId, patch) {
      return runtime.run("updateConsultantProfile", consultantId, patch);
    },
    archiveConsultantProfile(consultantId) {
      return runtime.run("archiveConsultantProfile", consultantId);
    },
    addToQueue(personId) {
      return runtime.run("addToQueue", personId);
    },
    pauseEmployee(personId, reason) {
      return runtime.run("pauseEmployee", personId, reason);
    },
    resumeEmployee(personId) {
      return runtime.run("resumeEmployee", personId);
    },
    startService(personId = null) {
      return runtime.run("startService", personId);
    },
    openFinishModal(personId) {
      return runtime.run("openFinishModal", personId);
    },
    closeFinishModal() {
      return runtime.run("closeFinishModal");
    },
    finishService(personId, closureData) {
      return runtime.run("finishService", personId, closureData);
    }
  };
});
