import { computed, ref, watch } from "vue";
import { defineStore, storeToRefs } from "pinia";

import { useAuthStore } from "~/stores/auth";
import { useAppRuntimeStore } from "~/stores/app-runtime";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";

function normalizeText(value) {
  return String(value || "").trim();
}

function normalizeCode(value) {
  return normalizeText(value).toUpperCase();
}

function normalizeStore(store = {}) {
  return {
    id: normalizeText(store.id),
    tenantId: normalizeText(store.tenantId),
    code: normalizeCode(store.code),
    name: normalizeText(store.name),
    city: normalizeText(store.city),
    isActive: Boolean(store.isActive ?? true),
    defaultTemplateId: normalizeText(store.defaultTemplateId),
    monthlyGoal: Math.max(0, Number(store.monthlyGoal || 0) || 0),
    weeklyGoal: Math.max(0, Number(store.weeklyGoal || 0) || 0),
    avgTicketGoal: Math.max(0, Number(store.avgTicketGoal || 0) || 0),
    conversionGoal: Math.max(0, Number(store.conversionGoal || 0) || 0),
    paGoal: Math.max(0, Number(store.paGoal || 0) || 0)
  };
}

function normalizeNullableNumber(value) {
  const normalized = String(value ?? "").trim();
  if (!normalized) {
    return null;
  }

  const parsed = Number(normalized);
  if (!Number.isFinite(parsed)) {
    return null;
  }

  return Math.max(0, parsed);
}

function assignIfChanged(body, key, nextValue, currentValue, normalize = (value) => value) {
  const next = normalize(nextValue);
  const current = normalize(currentValue);

  if (next !== current) {
    body[key] = next;
  }
}

function buildCreatePayload(payload = {}, tenantId) {
  const body = {
    tenantId: normalizeText(tenantId),
    name: normalizeText(payload.name),
    code: normalizeCode(payload.code)
  };

  const city = normalizeText(payload.city);
  const defaultTemplateId = normalizeText(payload.defaultTemplateId);
  const monthlyGoal = normalizeNullableNumber(payload.monthlyGoal);
  const weeklyGoal = normalizeNullableNumber(payload.weeklyGoal);
  const avgTicketGoal = normalizeNullableNumber(payload.avgTicketGoal);
  const conversionGoal = normalizeNullableNumber(payload.conversionGoal);
  const paGoal = normalizeNullableNumber(payload.paGoal);

  if (city) {
    body.city = city;
  }

  if (defaultTemplateId) {
    body.defaultTemplateId = defaultTemplateId;
  }

  if (monthlyGoal !== null) {
    body.monthlyGoal = monthlyGoal;
  }

  if (weeklyGoal !== null) {
    body.weeklyGoal = weeklyGoal;
  }

  if (avgTicketGoal !== null) {
    body.avgTicketGoal = avgTicketGoal;
  }

  if (conversionGoal !== null) {
    body.conversionGoal = conversionGoal;
  }

  if (paGoal !== null) {
    body.paGoal = paGoal;
  }

  return body;
}

function buildUpdatePayload(payload = {}, currentStore = {}) {
  const body = {};

  assignIfChanged(body, "name", payload.name, currentStore.name, normalizeText);
  assignIfChanged(body, "code", payload.code, currentStore.code, normalizeCode);
  assignIfChanged(body, "city", payload.city, currentStore.city, normalizeText);
  assignIfChanged(
    body,
    "defaultTemplateId",
    payload.defaultTemplateId,
    currentStore.defaultTemplateId,
    normalizeText
  );
  assignIfChanged(body, "monthlyGoal", payload.monthlyGoal, currentStore.monthlyGoal, normalizeNullableNumber);
  assignIfChanged(body, "weeklyGoal", payload.weeklyGoal, currentStore.weeklyGoal, normalizeNullableNumber);
  assignIfChanged(body, "avgTicketGoal", payload.avgTicketGoal, currentStore.avgTicketGoal, normalizeNullableNumber);
  assignIfChanged(body, "conversionGoal", payload.conversionGoal, currentStore.conversionGoal, normalizeNullableNumber);
  assignIfChanged(body, "paGoal", payload.paGoal, currentStore.paGoal, normalizeNullableNumber);

  return body;
}

function buildConsultantClonePayload(consultant = {}, storeId) {
  return {
    storeId,
    name: normalizeText(consultant.name),
    role: normalizeText(consultant.role) || "Atendimento",
    color: normalizeText(consultant.color) || "#168aad",
    monthlyGoal: Math.max(0, Number(consultant.monthlyGoal || 0) || 0),
    commissionRate: Math.max(0, Number(consultant.commissionRate || 0) || 0),
    conversionGoal: Math.max(0, Number(consultant.conversionGoal || 0) || 0),
    avgTicketGoal: Math.max(0, Number(consultant.avgTicketGoal || 0) || 0),
    paGoal: Math.max(0, Number(consultant.paGoal || 0) || 0)
  };
}

export const useMultiStoreStore = defineStore("multistore", () => {
  const runtimeConfig = useRuntimeConfig();
  const runtime = useAppRuntimeStore();
  const auth = useAuthStore();
  const { state: runtimeState } = storeToRefs(runtime);
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);
  const overview = ref(null);
  const managedStores = ref([]);
  const pending = ref(false);
  const managedStoresPending = ref(false);
  const ready = ref(false);
  const errorMessage = ref("");

  const state = computed(() => ({
    ...runtimeState.value,
    stores: auth.storeContext?.length ? auth.storeContext : runtimeState.value.stores || [],
    managedStores: managedStores.value.length ? managedStores.value : auth.storeContext?.length ? auth.storeContext : runtimeState.value.stores || [],
    activeStoreId: String(auth.activeStoreId || runtimeState.value.activeStoreId || "").trim()
  }));

  async function ensureLoaded() {
    await runtime.ensure();

    if (auth.isAuthenticated) {
      await auth.ensureSession();
      await refreshOverview();
      await refreshManagedStores();
    }

    return true;
  }

  async function refreshContext() {
    if (!auth.isAuthenticated) {
      return null;
    }

    const response = await auth.fetchContext();
    await refreshOverview();
    await refreshManagedStores();
    return response;
  }

  async function refreshManagedStores() {
    if (!auth.isAuthenticated) {
      managedStores.value = [];
      return [];
    }

    managedStoresPending.value = true;

    try {
      const params = new URLSearchParams();
      const tenantId = normalizeText(auth.activeTenantId || auth.tenantContext?.[0]?.id);
      if (tenantId) {
        params.set("tenantId", tenantId);
      }
      params.set("includeInactive", "true");

      const response = await apiRequest(`/v1/stores?${params.toString()}`);
      managedStores.value = Array.isArray(response?.stores)
        ? response.stores.map((store) => normalizeStore(store))
        : [];
      return managedStores.value;
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, "Nao foi possivel carregar as lojas.");
      throw error;
    } finally {
      managedStoresPending.value = false;
    }
  }

  async function refreshOverview() {
    if (!auth.isAuthenticated) {
      overview.value = null;
      ready.value = false;
      errorMessage.value = "";
      return null;
    }

    pending.value = true;
    errorMessage.value = "";

    try {
      const params = new URLSearchParams();
      const tenantId = normalizeText(auth.activeTenantId || auth.tenantContext?.[0]?.id);
      if (tenantId) {
        params.set("tenantId", tenantId);
      }

      const response = await apiRequest(`/v1/reports/multistore-overview?${params.toString()}`);
      overview.value = response;
      ready.value = true;
      return response;
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, "Nao foi possivel carregar a visao multiloja.");
      throw error;
    } finally {
      pending.value = false;
    }
  }

  async function createStore(payload = {}) {
    await ensureLoaded();

    if (!auth.isAuthenticated) {
      return { ok: false, message: "Sessao indisponivel." };
    }

    const tenantId = normalizeText(payload.tenantId || auth.activeTenantId || auth.tenantContext?.[0]?.id);
    const requestBody = buildCreatePayload(payload, tenantId);

    if (!requestBody.tenantId || !requestBody.name || !requestBody.code) {
      return { ok: false, message: "Preencha nome, codigo e tenant da loja." };
    }

    try {
      const response = await apiRequest("/v1/stores", {
        method: "POST",
        body: requestBody
      });

      let warningMessage = "";
      if (payload.cloneActiveRoster && Array.isArray(runtime.state.roster) && runtime.state.roster.length) {
        try {
          for (const consultant of runtime.state.roster) {
            await apiRequest("/v1/consultants", {
              method: "POST",
              body: buildConsultantClonePayload(consultant, response.store.id)
            });
          }
        } catch (error) {
          warningMessage = getApiErrorMessage(
            error,
            "Loja criada, mas nao foi possivel copiar os consultores da loja ativa."
          );
        }
      }

      await refreshContext();
      return {
        ok: true,
        store: response.store,
        warningMessage
      };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel criar loja.")
      };
    }
  }

  async function updateStore(storeId, payload = {}) {
    await ensureLoaded();

    if (!auth.isAuthenticated) {
      return { ok: false, message: "Sessao indisponivel." };
    }

    const currentStore = (managedStores.value.length ? managedStores.value : state.value.stores || []).find(
      (store) => store.id === storeId
    );
    if (!currentStore) {
      return { ok: false, message: "Loja nao encontrada." };
    }

    const requestBody = buildUpdatePayload(payload, currentStore);
    if (!Object.keys(requestBody).length) {
      return { ok: true, noChange: true };
    }

    try {
      const response = await apiRequest(`/v1/stores/${encodeURIComponent(String(storeId || "").trim())}`, {
        method: "PATCH",
        body: requestBody
      });

      await refreshContext();
      return {
        ok: true,
        store: response.store
      };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel atualizar loja.")
      };
    }
  }

  async function archiveStore(storeId) {
    await ensureLoaded();

    if (!auth.isAuthenticated) {
      return { ok: false, message: "Sessao indisponivel." };
    }

    try {
      const response = await apiRequest(`/v1/stores/${encodeURIComponent(String(storeId || "").trim())}/archive`, {
        method: "POST"
      });

      await refreshContext();
      return {
        ok: true,
        store: response.store
      };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel arquivar loja.")
      };
    }
  }

  async function restoreStore(storeId) {
    await ensureLoaded();

    if (!auth.isAuthenticated) {
      return { ok: false, message: "Sessao indisponivel." };
    }

    try {
      const response = await apiRequest(`/v1/stores/${encodeURIComponent(String(storeId || "").trim())}/restore`, {
        method: "POST"
      });

      await refreshContext();
      return {
        ok: true,
        store: response.store
      };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel restaurar loja.")
      };
    }
  }

  async function deleteStore(storeId) {
    await ensureLoaded();

    if (!auth.isAuthenticated) {
      return { ok: false, message: "Sessao indisponivel." };
    }

    try {
      const response = await apiRequest(`/v1/stores/${encodeURIComponent(String(storeId || "").trim())}`, {
        method: "DELETE"
      });

      await refreshContext();
      return {
        ok: true,
        storeId: response.storeId
      };
    } catch (error) {
      const dependencies = Array.isArray(error?.data?.error?.details?.dependencies)
        ? error.data.error.details.dependencies
        : [];
      const dependencyMessage = dependencies.length
        ? ` Vinculos encontrados: ${dependencies.map((item) => `${item.label} (${item.count})`).join(", ")}.`
        : "";

      return {
        ok: false,
        blockedDependencies: dependencies,
        message: `${getApiErrorMessage(error, "Nao foi possivel remover loja.")}${dependencyMessage}`
      };
    }
  }

  async function setActiveStore(storeId) {
    return auth.setActiveStore(storeId);
  }

  if (import.meta.client) {
    watch(
      () => [auth.isAuthenticated, auth.activeTenantId],
      ([isAuthenticated, tenantId], [previousAuthenticated, previousTenantId]) => {
        if (!isAuthenticated) {
          overview.value = null;
          managedStores.value = [];
          ready.value = false;
          errorMessage.value = "";
          return;
        }

        if (!previousAuthenticated || tenantId !== previousTenantId) {
          void refreshOverview().catch(() => {});
          void refreshManagedStores().catch(() => {});
        }
      }
    );
  }

  return {
    state,
    overview,
    managedStores,
    pending,
    managedStoresPending,
    ready,
    errorMessage,
    ensureLoaded,
    refreshOverview,
    refreshManagedStores,
    createStore,
    updateStore,
    archiveStore,
    restoreStore,
    deleteStore,
    setActiveStore
  };
});
