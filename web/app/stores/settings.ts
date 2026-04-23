import { defineStore, storeToRefs } from "pinia";

import { cloneValue } from "~/domain/utils/object";
import { useAuthStore } from "~/stores/auth";
import { useAppRuntimeStore } from "~/stores/app-runtime";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";

const OPTION_GROUP_PATHS = {
  visitReasonOptions: "visit-reasons",
  customerSourceOptions: "customer-sources",
  queueJumpReasonOptions: "queue-jump-reasons",
  lossReasonOptions: "loss-reasons",
  professionOptions: "professions"
};

function normalizeText(value) {
  return String(value || "").trim().toLowerCase();
}

export const useSettingsStore = defineStore("settings", () => {
  const runtimeConfig = useRuntimeConfig();
  const runtime = useAppRuntimeStore();
  const auth = useAuthStore();
  const { state } = storeToRefs(runtime);
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);

  async function resolveActiveStoreId() {
    await runtime.ensure();

    if (auth.isAuthenticated) {
      await auth.ensureSession();
    }

    return String(auth.activeStoreId || runtime.state.activeStoreId || "").trim();
  }

  async function persistOperationSection(storeId) {
    await apiRequest("/v1/settings/operation", {
      method: "PATCH",
      body: {
        storeId,
        selectedOperationTemplateId: String(runtime.state.selectedOperationTemplateId || "").trim(),
        settings: cloneValue(runtime.state.settings || {})
      }
    });
  }

  async function persistOperationPatch(storeId, payload = {}) {
    const body = {
      storeId
    };

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

    await apiRequest("/v1/settings/operation", {
      method: "PATCH",
      body
    });
  }

  async function persistModalSection(storeId) {
    await apiRequest("/v1/settings/modal", {
      method: "PATCH",
      body: {
        storeId,
        modalConfig: cloneValue(runtime.state.modalConfig || {})
      }
    });
  }

  async function persistModalPatch(storeId, modalConfig = {}) {
    if (!modalConfig || Object.keys(modalConfig).length === 0) {
      return;
    }

    await apiRequest("/v1/settings/modal", {
      method: "PATCH",
      body: {
        storeId,
        modalConfig: cloneValue(modalConfig)
      }
    });
  }

  async function persistOptionSection(storeId, stateKey) {
    const groupPath = OPTION_GROUP_PATHS[stateKey];

    if (!groupPath) {
      return;
    }

    await apiRequest(`/v1/settings/options/${groupPath}`, {
      method: "PUT",
      body: {
        storeId,
        items: cloneValue(runtime.state[stateKey] || [])
      }
    });
  }

  async function persistProductSection(storeId) {
    await apiRequest("/v1/settings/products", {
      method: "PUT",
      body: {
        storeId,
        items: cloneValue(runtime.state.productCatalog || [])
      }
    });
  }

  async function persistOptionItemCreate(storeId, stateKey, item) {
    const groupPath = OPTION_GROUP_PATHS[stateKey];

    if (!groupPath || !item?.id || !item?.label) {
      return;
    }

    await apiRequest(`/v1/settings/options/${groupPath}`, {
      method: "POST",
      body: {
        storeId,
        item: {
          id: String(item.id || "").trim(),
          label: String(item.label || "").trim()
        }
      }
    });
  }

  async function persistOptionItemUpdate(storeId, stateKey, item) {
    const groupPath = OPTION_GROUP_PATHS[stateKey];

    if (!groupPath || !item?.id || !item?.label) {
      return;
    }

    await apiRequest(`/v1/settings/options/${groupPath}/${encodeURIComponent(String(item.id || "").trim())}`, {
      method: "PATCH",
      body: {
        storeId,
        label: String(item.label || "").trim()
      }
    });
  }

  async function persistOptionItemDelete(storeId, stateKey, itemId) {
    const groupPath = OPTION_GROUP_PATHS[stateKey];

    if (!groupPath || !itemId) {
      return;
    }

    await apiRequest(`/v1/settings/options/${groupPath}/${encodeURIComponent(String(itemId || "").trim())}?storeId=${encodeURIComponent(storeId)}`, {
      method: "DELETE"
    });
  }

  async function persistProductItemCreate(storeId, item) {
    if (!item?.id || !item?.name) {
      return;
    }

    await apiRequest("/v1/settings/products", {
      method: "POST",
      body: {
        storeId,
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

  async function persistProductItemUpdate(storeId, productId, payload) {
    if (!productId) {
      return;
    }

    await apiRequest(`/v1/settings/products/${encodeURIComponent(String(productId || "").trim())}`, {
      method: "PATCH",
      body: {
        storeId,
        name: String(payload?.name || "").trim(),
        code: String(payload?.code || "").trim().toUpperCase(),
        category: String(payload?.category || "").trim(),
        basePrice: Math.max(0, Number(payload?.basePrice || 0) || 0)
      }
    });
  }

  async function persistProductItemDelete(storeId, productId) {
    if (!productId) {
      return;
    }

    await apiRequest(`/v1/settings/products/${encodeURIComponent(String(productId || "").trim())}?storeId=${encodeURIComponent(storeId)}`, {
      method: "DELETE"
    });
  }

  async function mutateAndPersist(actionName, args = [], persistHandlers = []) {
    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      return { ok: false, message: "Sessao ou loja ativa indisponivel." };
    }

    const previousState = cloneValue(runtime.state);
    const localResult = await runtime.run(actionName, ...args);

    if (localResult?.ok === false) {
      return localResult;
    }

    try {
      for (const persistHandler of persistHandlers) {
        await persistHandler(storeId);
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
    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      return { ok: false, message: "Sessao ou loja ativa indisponivel." };
    }

    const previousState = cloneValue(runtime.state);
    const previousIds = new Set((previousState[stateKey] || []).map((item) => String(item?.id || "").trim()));
    const localResult = await runtime.run(actionName, label);

    if (localResult?.ok === false) {
      return localResult;
    }

    const createdItem =
      (runtime.state[stateKey] || []).find(
        (item) => !previousIds.has(String(item?.id || "").trim()) && normalizeText(item?.label) === normalizeText(label)
      ) || null;

    if (!createdItem) {
      return { ok: false, message: "Nao foi possivel identificar a opcao criada." };
    }

    try {
      await persistOptionItemCreate(storeId, stateKey, createdItem);
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
    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      return { ok: false, message: "Sessao ou loja ativa indisponivel." };
    }

    const previousState = cloneValue(runtime.state);
    const localResult = await runtime.run(actionName, optionId, label);

    if (localResult?.ok === false) {
      return localResult;
    }

    const updatedItem = (runtime.state[stateKey] || []).find((item) => String(item?.id || "").trim() === String(optionId || "").trim()) || null;
    if (!updatedItem) {
      return { ok: false, message: "Nao foi possivel identificar a opcao atualizada." };
    }

    try {
      await persistOptionItemUpdate(storeId, stateKey, updatedItem);
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
    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      return { ok: false, message: "Sessao ou loja ativa indisponivel." };
    }

    const previousState = cloneValue(runtime.state);
    const localResult = await runtime.run(actionName, optionId);

    if (localResult?.ok === false) {
      return localResult;
    }

    try {
      await persistOptionItemDelete(storeId, stateKey, optionId);
      return localResult || { ok: true };
    } catch (error) {
      runtime.hydrate(previousState);
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel salvar as configuracoes.")
      };
    }
  }

  async function mutateAndPersistProductCreate(name, category, basePrice, code) {
    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      return { ok: false, message: "Sessao ou loja ativa indisponivel." };
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
      await persistProductItemCreate(storeId, createdItem);
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
    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      return { ok: false, message: "Sessao ou loja ativa indisponivel." };
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
      await persistProductItemUpdate(storeId, productId, updatedItem);
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
    const storeId = await resolveActiveStoreId();

    if (!storeId || !auth.isAuthenticated) {
      return { ok: false, message: "Sessao ou loja ativa indisponivel." };
    }

    const previousState = cloneValue(runtime.state);
    const localResult = await runtime.run("removeCatalogProduct", productId);

    if (localResult?.ok === false) {
      return localResult;
    }

    try {
      await persistProductItemDelete(storeId, productId);
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
        (storeId) => persistOperationPatch(storeId, {
          settings: {
            [settingId]: value
          }
        })
      ]);
    },
    updateModalConfig(configKey, value) {
      return mutateAndPersist("updateModalConfig", [configKey, value], [
        (storeId) => persistModalPatch(storeId, {
          [configKey]: value
        })
      ]);
    },
    applyOperationTemplate(templateId) {
      return mutateAndPersist("applyOperationTemplate", [templateId], [
        (storeId) => persistOperationSection(storeId),
        (storeId) => persistModalSection(storeId),
        (storeId) => persistOptionSection(storeId, "visitReasonOptions"),
        (storeId) => persistOptionSection(storeId, "customerSourceOptions")
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
    addCustomerSourceOption(label) {
      return mutateAndPersistOptionCreate("addCustomerSourceOption", label, "customerSourceOptions");
    },
    updateCustomerSourceOption(optionId, label) {
      return mutateAndPersistOptionUpdate("updateCustomerSourceOption", optionId, label, "customerSourceOptions");
    },
    removeCustomerSourceOption(optionId) {
      return mutateAndPersistOptionDelete("removeCustomerSourceOption", optionId, "customerSourceOptions");
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
    addLossReasonOption(label) {
      return mutateAndPersistOptionCreate("addLossReasonOption", label, "lossReasonOptions");
    },
    updateLossReasonOption(optionId, label) {
      return mutateAndPersistOptionUpdate("updateLossReasonOption", optionId, label, "lossReasonOptions");
    },
    removeLossReasonOption(optionId) {
      return mutateAndPersistOptionDelete("removeLossReasonOption", optionId, "lossReasonOptions");
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
