import { computed, ref, watch } from "vue";
import { defineStore } from "pinia";

import { canAccessClients, canManageTenants } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";

function normalizeText(value) {
  return String(value || "").trim();
}

function normalizeBoolean(value, fallback = true) {
  if (value === undefined || value === null) {
    return fallback;
  }

  return Boolean(value);
}

function normalizeSlug(value) {
  return normalizeText(value)
    .toLowerCase()
    .replace(/[_\s]+/g, "-")
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
}

function normalizeTenant(tenant = {}) {
  return {
    id: normalizeText(tenant.id),
    name: normalizeText(tenant.name),
    slug: normalizeSlug(tenant.slug),
    active: normalizeBoolean(tenant.active, true)
  };
}

function buildUpdatePayload(payload = {}, currentTenant = {}) {
  const body = {};
  const nextName = normalizeText(payload.name ?? currentTenant.name);
  const nextSlug = normalizeSlug(payload.slug ?? currentTenant.slug);
  const nextActive = normalizeBoolean(payload.active, currentTenant.active);

  if (nextName !== normalizeText(currentTenant.name)) {
    body.name = nextName;
  }

  if (nextSlug !== normalizeSlug(currentTenant.slug)) {
    body.slug = nextSlug;
  }

  if (nextActive !== normalizeBoolean(currentTenant.active, true)) {
    body.isActive = nextActive;
  }

  return body;
}

export const useTenantsStore = defineStore("tenants", () => {
  const runtimeConfig = useRuntimeConfig();
  const auth = useAuthStore();
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);

  const tenants = ref([]);
  const pending = ref(false);
  const ready = ref(false);
  const errorMessage = ref("");

  const viewable = computed(() => canAccessClients(auth.role, auth.permissionKeys, auth.permissionsResolved));
  const manageable = computed(() => canManageTenants(auth.role, auth.permissionKeys, auth.permissionsResolved));
  const canCreate = computed(() => manageable.value && normalizeText(auth.role) === "platform_admin");

  function clearState() {
    tenants.value = [];
    pending.value = false;
    ready.value = false;
    errorMessage.value = "";
  }

  async function refreshTenants({ includeInactive = true } = {}) {
    await auth.ensureSession();

    if (!auth.isAuthenticated || !viewable.value) {
      clearState();
      return [];
    }

    pending.value = true;
    errorMessage.value = "";

    try {
      const params = new URLSearchParams();
      if (includeInactive) {
        params.set("includeInactive", "true");
      }

      const response = await apiRequest(`/v1/tenants?${params.toString()}`);
      tenants.value = Array.isArray(response?.tenants)
        ? response.tenants.map((tenant) => normalizeTenant(tenant)).filter((tenant) => tenant.id)
        : [];
      ready.value = true;
      return tenants.value;
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, "Nao foi possivel carregar os clientes.");
      throw error;
    } finally {
      pending.value = false;
    }
  }

  async function ensureLoaded() {
    if (!auth.isAuthenticated) {
      await auth.ensureSession();
    }

    if (!auth.isAuthenticated || !viewable.value) {
      clearState();
      return false;
    }

    if (ready.value) {
      return true;
    }

    try {
      await refreshTenants();
      return true;
    } catch {
      return false;
    }
  }

  async function refreshContext() {
    if (!auth.isAuthenticated) {
      return null;
    }

    const response = await auth.fetchContext();
    await refreshTenants();
    return response;
  }

  async function createTenant(payload = {}) {
    await auth.ensureSession();

    if (!auth.isAuthenticated || !canCreate.value) {
      return { ok: false, message: "Somente o admin da plataforma pode criar clientes." };
    }

    const body = {
      name: normalizeText(payload.name),
      slug: normalizeSlug(payload.slug || payload.name),
      isActive: normalizeBoolean(payload.active, true)
    };

    if (!body.name || !body.slug) {
      return { ok: false, message: "Preencha nome e slug do cliente." };
    }

    try {
      const response = await apiRequest("/v1/tenants", {
        method: "POST",
        body
      });

      await refreshContext();
      return {
        ok: true,
        tenant: normalizeTenant(response?.tenant)
      };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel criar o cliente.")
      };
    }
  }

  async function updateTenant(tenantId, payload = {}) {
    await ensureLoaded();

    if (!auth.isAuthenticated || !manageable.value) {
      return { ok: false, message: "Sem permissao para editar clientes." };
    }

    const currentTenant = tenants.value.find((tenant) => tenant.id === normalizeText(tenantId));
    if (!currentTenant) {
      return { ok: false, message: "Cliente nao encontrado." };
    }

    const body = buildUpdatePayload(payload, currentTenant);
    if (!Object.keys(body).length) {
      return { ok: true, noChange: true, tenant: currentTenant };
    }

    if (!normalizeText(body.name ?? currentTenant.name) || !normalizeSlug(body.slug ?? currentTenant.slug)) {
      return { ok: false, message: "Preencha nome e slug validos." };
    }

    try {
      const response = await apiRequest(`/v1/tenants/${encodeURIComponent(normalizeText(tenantId))}`, {
        method: "PATCH",
        body
      });

      await refreshContext();
      return {
        ok: true,
        tenant: normalizeTenant(response?.tenant)
      };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel atualizar o cliente.")
      };
    }
  }

  async function archiveTenant(tenantId) {
    await ensureLoaded();

    if (!auth.isAuthenticated || !manageable.value) {
      return { ok: false, message: "Sem permissao para editar clientes." };
    }

    try {
      const response = await apiRequest(`/v1/tenants/${encodeURIComponent(normalizeText(tenantId))}/archive`, {
        method: "POST"
      });

      await refreshContext();
      return {
        ok: true,
        tenant: normalizeTenant(response?.tenant)
      };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel arquivar o cliente.")
      };
    }
  }

  async function restoreTenant(tenantId) {
    await ensureLoaded();

    if (!auth.isAuthenticated || !manageable.value) {
      return { ok: false, message: "Sem permissao para editar clientes." };
    }

    try {
      const response = await apiRequest(`/v1/tenants/${encodeURIComponent(normalizeText(tenantId))}/restore`, {
        method: "POST"
      });

      await refreshContext();
      return {
        ok: true,
        tenant: normalizeTenant(response?.tenant)
      };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel reativar o cliente.")
      };
    }
  }

  watch(
    () => auth.isAuthenticated,
    (isAuthenticated) => {
      if (!isAuthenticated) {
        clearState();
      }
    }
  );

  return {
    tenants,
    pending,
    ready,
    errorMessage,
    viewable,
    manageable,
    canCreate,
    ensureLoaded,
    refreshTenants,
    createTenant,
    updateTenant,
    archiveTenant,
    restoreTenant
  };
});