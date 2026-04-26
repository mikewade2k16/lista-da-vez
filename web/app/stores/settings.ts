import { defineStore, storeToRefs } from "pinia";

import { cloneValue } from "~/domain/utils/object";
import { useAuthStore } from "~/stores/auth";
import { useAppRuntimeStore } from "~/stores/app-runtime";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";

// Configuracoes de operacao deixaram de ser por loja: agora sao tenant-wide.
// Os endpoints continuam aceitando storeId no payload por compatibilidade com
// clientes antigos, mas o backend ignora esse campo e usa o tenant do principal
// autenticado. A store envia o payload limpo, sem storeId.
const OPTION_GROUP_PATHS = {
  visitReasonOptions: "visit-reasons",
  customerSourceOptions: "customer-sources",
  pauseReasonOptions: "pause-reasons",
  queueJumpReasonOptions: "queue-jump-reasons",
  lossReasonOptions: "loss-reasons",
  professionOptions: "professions"
};

function normalizeText(value) {
  return String(value || "").trim().toLowerCase();
}

function appendTenantQuery(path, tenantId) {
  const normalizedTenantId = String(tenantId || "").trim();

  if (!normalizedTenantId) {
    return path;
  }

  const separator = path.includes("?") ? "&" : "?";
  return `${path}${separator}tenantId=${encodeURIComponent(normalizedTenantId)}`;
}

export const useSettingsStore = defineStore("settings", () => {
  const runtimeConfig = useRuntimeConfig();
  const runtime = useAppRuntimeStore();
  const auth = useAuthStore();
  const { state } = storeToRefs(runtime);
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);

  async function ensureAuthenticated() {
    await runtime.ensure();

    if (auth.isAuthenticated) {
      await auth.ensureSession();
    }

    return auth.isAuthenticated;
  }

  function settingsPath(path) {
    return appendTenantQuery(path, auth.activeTenantId || auth.tenantContext?.[0]?.id);
  }

  async function persistOperationSection() {
    await apiRequest(settingsPath("/v1/settings/operation"), {
      method: "PATCH",
      body: {
        selectedOperationTemplateId: String(runtime.state.selectedOperationTemplateId || "").trim(),
        settings: cloneValue(runtime.state.settings || {})
      }
    });
  }

  async function persistOperationPatch(payload = {}) {
    const body = {};

    if (Object.prototype.hasOwnProperty.call(payload, "selectedOperationTemplateId")) {
      const selectedOperationTemplateId = String(payload.selectedOperationTemplateId || "").trim();
      if (selectedOperationTemplateId) {
        body.selectedOperationTemplateId = selectedOperationTemplateId;
      }
    }

    if (payload.settings && Object.keys(payload.settings).length > 0) {
      body.settings = cloneValue(payload.settings);
    }

    if (!body.selectedOperationTemplateId && !body.settings) {
      return;
    }

    await apiRequest(settingsPath("/v1/settings/operation"), {
      method: "PATCH",
      body
    });
  }

  async function persistModalSection() {
    await apiRequest(settingsPath("/v1/settings/modal"), {
      method: "PATCH",
      body: {
        modalConfig: cloneValue(runtime.state.modalConfig || {})
      }
    });
  }

  async function persistModalPatch(modalConfig = {}) {
    if (!modalConfig || Object.keys(modalConfig).length === 0) {
      return;
    }

    await apiRequest(settingsPath("/v1/settings/modal"), {
      method: "PATCH",
      body: {
        modalConfig: cloneValue(modalConfig)
      }
    });
  }

  async function persistOptionSection(stateKey) {
    const groupPath = OPTION_GROUP_PATHS[stateKey];

    if (!groupPath) {
      return;
    }

    await apiRequest(settingsPath(`/v1/settings/options/${groupPath}`), {
      method: "PUT",
      body: {
        items: cloneValue(runtime.state[stateKey] || [])
      }
    });
  }

  async function persistProductSection() {
    await apiRequest(settingsPath("/v1/settings/products"), {
      method: "PUT",
      body: {
        items: cloneValue(runtime.state.productCatalog || [])
      }
    });
  }

  async function persistOptionItemCreate(stateKey, item) {
    const groupPath = OPTION_GROUP_PATHS[stateKey];

    if (!groupPath || !item?.id || !item?.label) {
      return;
    }

    await apiRequest(settingsPath(`/v1/settings/options/${groupPath}`), {
      method: "POST",
      body: {
        item: {
          id: String(item.id || "").trim(),
          label: String(item.label || "").trim()
        }
      }
    });
  }

  async function persistOptionItemUpdate(stateKey, item) {
    const groupPath = OPTION_GROUP_PATHS[stateKey];

    if (!groupPath || !item?.id || !item?.label) {
      return;
    }

    await apiRequest(settingsPath(`/v1/settings/options/${groupPath}/${encodeURIComponent(String(item.id || "").trim())}`), {
      method: "PATCH",
      body: {
        label: String(item.label || "").trim()
      }
    });
  }

  async function persistOptionItemDelete(stateKey, itemId) {
    const groupPath = OPTION_GROUP_PATHS[stateKey];

    if (!groupPath || !itemId) {
      return;
    }

    await apiRequest(settingsPath(`/v1/settings/options/${groupPath}/${encodeURIComponent(String(itemId || "").trim())}`), {
      method: "DELETE"
    });
  }

  async function persistProductItemCreate(item) {
    if (!item?.id || !item?.name) {
      return;
    }

    await apiRequest(settingsPath("/v1/settings/products"), {
      method: "POST",
      body: {
        item: {
          id: String(item.id || "").trim(),
          name: String(item.name || "").trim(),
          code: String(item.code || "").trim().toUpperCase(),
          category: String(item.category || "").trim(),
          basePrice: Math.max(0, Number(item.basePrice || 0) || 0)
        }
      }
    });
  }

  async function persistProductItemUpdate(productId, payload) {
    if (!productId) {
      return;
    }

    await apiRequest(settingsPath(`/v1/settings/products/${encodeURIComponent(String(productId || "").trim())}`), {
      method: "PATCH",
      body: {
        name: String(payload?.name || "").trim(),
        code: String(payload?.code || "").trim().toUpperCase(),
        category: String(payload?.category || "").trim(),
        basePrice: Math.max(0, Number(payload?.basePrice || 0) || 0)
      }
    });
  }

  async function persistProductItemDelete(productId) {
    if (!productId) {
      return;
    }

    await apiRequest(settingsPath(`/v1/settings/products/${encodeURIComponent(String(productId || "").trim())}`), {
      method: "DELETE"
    });
  }

  async function mutateAndPersist(actionName, args = [], persistHandlers = []) {
    const isAuthenticated = await ensureAuthenticated();

    if (!isAuthenticated) {
      return { ok: false, message: "Sessao indisponivel." };
    }

    const previousState = cloneValue(runtime.state);
    const localResult = await runtime.run(actionName, ...args);

    if (localResult?.ok === false) {
      return localResult;
    }

    try {
      for (const persistHandler of persistHandlers) {
        await persistHandler();
      }

      return localResult || { ok: true };
    } catch (error) {
      runtime.hydrate(previousState);

      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel salvar as configuracoes.")
      };
    }
  }

  async function mutateAndPersistOptionCreate(actionName, label, stateKey) {
    const isAuthenticated = await ensureAuthenticated();

    if (!isAuthenticated) {
      return { ok: false, message: "Sessao indisponivel." };
    }

    const previousState = cloneValue(runtime.state);
    const localResult = await runtime.run(actionName, label);

    if (localResult?.ok === false) {
      return localResult;
    }

    try {
      await persistOptionSection(stateKey);
      return localResult || { ok: true };
    } catch (error) {
      runtime.hydrate(previousState);
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel salvar as configuracoes.")
      };
    }
  }

  async function mutateAndPersistOptionUpdate(actionName, optionId, label, stateKey) {
    const isAuthenticated = await ensureAuthenticated();

    if (!isAuthenticated) {
      return { ok: false, message: "Sessao indisponivel." };
    }

    const previousState = cloneValue(runtime.state);
    const localResult = await runtime.run(actionName, optionId, label);

    if (localResult?.ok === false) {
      return localResult;
    }

    try {
      await persistOptionSection(stateKey);
      return localResult || { ok: true };
    } catch (error) {
      runtime.hydrate(previousState);
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel salvar as configuracoes.")
      };
    }
  }

  async function mutateAndPersistOptionDelete(actionName, optionId, stateKey) {
    const isAuthenticated = await ensureAuthenticated();

    if (!isAuthenticated) {
      return { ok: false, message: "Sessao indisponivel." };
    }

    const previousState = cloneValue(runtime.state);
    const localResult = await runtime.run(actionName, optionId);

    if (localResult?.ok === false) {
      return localResult;
    }

    try {
      await persistOptionSection(stateKey);
      return localResult || { ok: true };
    } catch (error) {
      runtime.hydrate(previousState);
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel salvar as configuracoes.")
      };
    }
  }

  async function mutateAndPersistOptionReorder(actionName, optionIds, stateKey) {
    return mutateAndPersist(actionName, [optionIds], [
      () => persistOptionSection(stateKey)
    ]);
  }

  async function mutateAndPersistProductCreate(name, category, basePrice, code) {
    const isAuthenticated = await ensureAuthenticated();

    if (!isAuthenticated) {
      return { ok: false, message: "Sessao indisponivel." };
    }

    const previousState = cloneValue(runtime.state);
    const previousIds = new Set((previousState.productCatalog || []).map((item) => String(item?.id || "").trim()));
    const localResult = await runtime.run("addCatalogProduct", name, category, basePrice, code);

    if (localResult?.ok === false) {
      return localResult;
    }

    const createdItem =
      (runtime.state.productCatalog || []).find(
        (item) => !previousIds.has(String(item?.id || "").trim()) && normalizeText(item?.name) === normalizeText(name)
      ) || null;

    if (!createdItem) {
      return { ok: false, message: "Nao foi possivel identificar o produto criado." };
    }

    try {
      await persistProductItemCreate(createdItem);
      return localResult || { ok: true };
    } catch (error) {
      runtime.hydrate(previousState);
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel salvar as configuracoes.")
      };
    }
  }

  async function mutateAndPersistProductUpdate(productId, payload) {
    const isAuthenticated = await ensureAuthenticated();

    if (!isAuthenticated) {
      return { ok: false, message: "Sessao indisponivel." };
    }

    const previousState = cloneValue(runtime.state);
    const localResult = await runtime.run("updateCatalogProduct", productId, payload);

    if (localResult?.ok === false) {
      return localResult;
    }

    const updatedItem = (runtime.state.productCatalog || []).find((item) => String(item?.id || "").trim() === String(productId || "").trim()) || null;
    if (!updatedItem) {
      return { ok: false, message: "Nao foi possivel identificar o produto atualizado." };
    }

    try {
      await persistProductItemUpdate(productId, updatedItem);
      return localResult || { ok: true };
    } catch (error) {
      runtime.hydrate(previousState);
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel salvar as configuracoes.")
      };
    }
  }

  async function mutateAndPersistProductDelete(productId) {
    const isAuthenticated = await ensureAuthenticated();

    if (!isAuthenticated) {
      return { ok: false, message: "Sessao indisponivel." };
    }

    const previousState = cloneValue(runtime.state);
    const localResult = await runtime.run("removeCatalogProduct", productId);

    if (localResult?.ok === false) {
      return localResult;
    }

    try {
      await persistProductItemDelete(productId);
      return localResult || { ok: true };
    } catch (error) {
      runtime.hydrate(previousState);
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel salvar as configuracoes.")
      };
    }
  }

  return {
    state,
    ensure: runtime.ensure,
    updateSetting(settingId, value) {
      return mutateAndPersist("updateSetting", [settingId, value], [
        () => persistOperationPatch({
          settings: {
            [settingId]: value
          }
        })
      ]);
    },
    updateModalConfig(configKey, value) {
      return mutateAndPersist("updateModalConfig", [configKey, value], [
        () => persistModalPatch({
          [configKey]: value
        })
      ]);
    },
    applyOperationTemplate(templateId) {
      return mutateAndPersist("applyOperationTemplate", [templateId], [
        () => persistOperationSection(),
        () => persistModalSection(),
        () => persistOptionSection("visitReasonOptions"),
        () => persistOptionSection("customerSourceOptions")
      ]);
    },
    addVisitReasonOption(label) {
      return mutateAndPersistOptionCreate("addVisitReasonOption", label, "visitReasonOptions");
    },
    updateVisitReasonOption(optionId, label) {
      return mutateAndPersistOptionUpdate("updateVisitReasonOption", optionId, label, "visitReasonOptions");
    },
    removeVisitReasonOption(optionId) {
      return mutateAndPersistOptionDelete("removeVisitReasonOption", optionId, "visitReasonOptions");
    },
    reorderVisitReasonOptions(optionIds) {
      return mutateAndPersistOptionReorder("reorderVisitReasonOptions", optionIds, "visitReasonOptions");
    },
    addCustomerSourceOption(label) {
      return mutateAndPersistOptionCreate("addCustomerSourceOption", label, "customerSourceOptions");
    },
    updateCustomerSourceOption(optionId, label) {
      return mutateAndPersistOptionUpdate("updateCustomerSourceOption", optionId, label, "customerSourceOptions");
    },
    removeCustomerSourceOption(optionId) {
      return mutateAndPersistOptionDelete("removeCustomerSourceOption", optionId, "customerSourceOptions");
    },
    reorderCustomerSourceOptions(optionIds) {
      return mutateAndPersistOptionReorder("reorderCustomerSourceOptions", optionIds, "customerSourceOptions");
    },
    addPauseReasonOption(label) {
      return mutateAndPersistOptionCreate("addPauseReasonOption", label, "pauseReasonOptions");
    },
    updatePauseReasonOption(optionId, label) {
      return mutateAndPersistOptionUpdate("updatePauseReasonOption", optionId, label, "pauseReasonOptions");
    },
    removePauseReasonOption(optionId) {
      return mutateAndPersistOptionDelete("removePauseReasonOption", optionId, "pauseReasonOptions");
    },
    reorderPauseReasonOptions(optionIds) {
      return mutateAndPersistOptionReorder("reorderPauseReasonOptions", optionIds, "pauseReasonOptions");
    },
    addQueueJumpReasonOption(label) {
      return mutateAndPersistOptionCreate("addQueueJumpReasonOption", label, "queueJumpReasonOptions");
    },
    updateQueueJumpReasonOption(optionId, label) {
      return mutateAndPersistOptionUpdate("updateQueueJumpReasonOption", optionId, label, "queueJumpReasonOptions");
    },
    removeQueueJumpReasonOption(optionId) {
      return mutateAndPersistOptionDelete("removeQueueJumpReasonOption", optionId, "queueJumpReasonOptions");
    },
    reorderQueueJumpReasonOptions(optionIds) {
      return mutateAndPersistOptionReorder("reorderQueueJumpReasonOptions", optionIds, "queueJumpReasonOptions");
    },
    addLossReasonOption(label) {
      return mutateAndPersistOptionCreate("addLossReasonOption", label, "lossReasonOptions");
    },
    updateLossReasonOption(optionId, label) {
      return mutateAndPersistOptionUpdate("updateLossReasonOption", optionId, label, "lossReasonOptions");
    },
    removeLossReasonOption(optionId) {
      return mutateAndPersistOptionDelete("removeLossReasonOption", optionId, "lossReasonOptions");
    },
    reorderLossReasonOptions(optionIds) {
      return mutateAndPersistOptionReorder("reorderLossReasonOptions", optionIds, "lossReasonOptions");
    },
    addProfessionOption(label) {
      return mutateAndPersistOptionCreate("addProfessionOption", label, "professionOptions");
    },
    updateProfessionOption(optionId, label) {
      return mutateAndPersistOptionUpdate("updateProfessionOption", optionId, label, "professionOptions");
    },
    removeProfessionOption(optionId) {
      return mutateAndPersistOptionDelete("removeProfessionOption", optionId, "professionOptions");
    },
    reorderProfessionOptions(optionIds) {
      return mutateAndPersistOptionReorder("reorderProfessionOptions", optionIds, "professionOptions");
    },
    addCatalogProduct(name, category, basePrice, code) {
      return mutateAndPersistProductCreate(name, category, basePrice, code);
    },
    updateCatalogProduct(productId, payload) {
      return mutateAndPersistProductUpdate(productId, payload);
    },
    removeCatalogProduct(productId) {
      return mutateAndPersistProductDelete(productId);
    }
  };
});
