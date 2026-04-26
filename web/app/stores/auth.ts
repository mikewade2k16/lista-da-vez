import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { canUseAllStoresScope, getAllowedWorkspaces, normalizeAppRole } from "~/domain/utils/permissions";
import { useAppRuntimeStore } from "~/stores/app-runtime";
import { AUTH_TOKEN_COOKIE, createApiRequest, getApiBase, getApiErrorMessage } from "~/utils/api-client";
import { hydrateRuntimeStoreContext } from "~/utils/runtime-remote";
import { getWorkspacePath } from "~/utils/workspaces";

const REMEMBERED_LOGIN_STORAGE_KEY = "ldv_remembered_login";
const STORE_SCOPE_MODE_SINGLE = "single";
const STORE_SCOPE_MODE_ALL = "all";
const ROLE_PROFILE_MAP = {
  platform_admin: "perfil-platform-admin",
  owner: "perfil-proprietario",
  director: "perfil-proprietario",
  marketing: "perfil-marketing",
  manager: "perfil-gerente",
  store_terminal: "perfil-consultor",
  consultant: "perfil-consultor"
};

function normalizeContextStore(store) {
  return {
    id: String(store?.id || "").trim(),
    tenantId: String(store?.tenantId || "").trim(),
    code: String(store?.code || "").trim(),
    name: String(store?.name || "").trim(),
    city: String(store?.city || "").trim(),
    isActive: Boolean(store?.isActive ?? true),
    defaultTemplateId: String(store?.defaultTemplateId || "").trim(),
    monthlyGoal: Math.max(0, Number(store?.monthlyGoal || 0) || 0),
    weeklyGoal: Math.max(0, Number(store?.weeklyGoal || 0) || 0),
    avgTicketGoal: Math.max(0, Number(store?.avgTicketGoal || 0) || 0),
    conversionGoal: Math.max(0, Number(store?.conversionGoal || 0) || 0),
    paGoal: Math.max(0, Number(store?.paGoal || 0) || 0)
  };
}

function parseRememberedLogin(rawValue) {
  if (!rawValue) {
    return null;
  }

  try {
    const parsed = JSON.parse(rawValue);
    const email = String(parsed?.email || "").trim().toLowerCase();
    const password = String(parsed?.password || "");

    if (!email || !password) {
      return null;
    }

    return {
      email,
      password
    };
  } catch {
    return null;
  }
}

function normalizeStoreScopeMode(value) {
  return String(value || "").trim() === STORE_SCOPE_MODE_ALL
    ? STORE_SCOPE_MODE_ALL
    : STORE_SCOPE_MODE_SINGLE;
}

export const useAuthStore = defineStore("auth", () => {
  const runtimeConfig = useRuntimeConfig();
  const runtime = useAppRuntimeStore();
  const accessToken = useCookie(AUTH_TOKEN_COOKIE, {
    sameSite: "lax",
    maxAge: 60 * 60 * 12,
    default: () => null
  });
  const activeStoreCookie = useCookie("ldv_active_store_id", {
    sameSite: "lax",
    maxAge: 60 * 60 * 24 * 30,
    default: () => null
  });
  const storeScopeCookie = useCookie("ldv_store_scope_mode", {
    sameSite: "lax",
    maxAge: 60 * 60 * 24 * 30,
    default: () => STORE_SCOPE_MODE_SINGLE
  });
  const apiRequest = createApiRequest(runtimeConfig, () => accessToken.value);

  const user = ref(null);
  const principal = ref(null);
  const tenantContext = ref([]);
  const storeContext = ref([]);
  const activeTenantId = ref("");
  const activeStoreId = ref("");
  const storeScopeMode = ref(normalizeStoreScopeMode(storeScopeCookie.value));
  const hydrated = ref(false);
  const pending = ref(false);
  const lastError = ref("");
  let ensurePromise = null;

  const role = computed(() => normalizeAppRole(principal.value?.role || ""));
  const permissionKeys = computed(() =>
    Array.isArray(principal.value?.permissions)
      ? principal.value.permissions.map((permissionKey) => String(permissionKey || "").trim()).filter(Boolean)
      : []
  );
  const permissionsResolved = computed(() => Boolean(principal.value?.permissionsResolved));
  const isAuthenticated = computed(() => Boolean(accessToken.value && user.value && principal.value));
  const mustChangePassword = computed(() => Boolean(user.value?.mustChangePassword));
  const allowedWorkspaces = computed(() => getAllowedWorkspaces(role.value, permissionKeys.value, permissionsResolved.value));
  const homeWorkspaceId = computed(() => allowedWorkspaces.value[0] || "operacao");
  const homePath = computed(() => getWorkspacePath(homeWorkspaceId.value));
  const accessibleStoreIds = computed(() =>
    storeContext.value.length
      ? storeContext.value.map((store) => String(store?.id || "").trim()).filter(Boolean)
      : Array.isArray(principal.value?.storeIds)
        ? principal.value.storeIds.map((storeId) => String(storeId || "").trim()).filter(Boolean)
        : []
  );
  const canUseAllStores = computed(() => canUseAllStoresScope(accessibleStoreIds.value));
  const isAllStoresScope = computed(() =>
    canUseAllStores.value && storeScopeMode.value === STORE_SCOPE_MODE_ALL
  );

  function syncStoreScopeMode(nextMode = storeScopeMode.value) {
    const normalizedMode = normalizeStoreScopeMode(nextMode);
    const resolvedMode = canUseAllStores.value ? normalizedMode : STORE_SCOPE_MODE_SINGLE;

    storeScopeMode.value = resolvedMode;
    storeScopeCookie.value = resolvedMode;

    return resolvedMode;
  }

  function clearSession() {
    accessToken.value = null;
    activeStoreCookie.value = null;
    storeScopeCookie.value = STORE_SCOPE_MODE_SINGLE;
    user.value = null;
    principal.value = null;
    tenantContext.value = [];
    storeContext.value = [];
    activeTenantId.value = "";
    activeStoreId.value = "";
    storeScopeMode.value = STORE_SCOPE_MODE_SINGLE;
    lastError.value = "";
  }

  async function syncRuntimeAccess() {
    await runtime.ensure();

    const runtimeState = runtime.state;
    const mappedProfileId = ROLE_PROFILE_MAP[role.value];
    const nextStores = storeContext.value.length
      ? storeContext.value.map((store) => normalizeContextStore(store))
      : runtimeState.stores || [];
    const desiredStoreId =
      nextStores.find((store) => store.id === activeStoreId.value)?.id ||
      accessibleStoreIds.value.find((storeId) => nextStores.some((store) => store.id === storeId)) ||
      nextStores[0]?.id ||
      runtimeState.activeStoreId;

    if (storeContext.value.length) {
      runtime.hydrate({
        ...runtimeState,
        stores: nextStores,
        activeStoreId: desiredStoreId || runtimeState.activeStoreId
      });
    }

    if (mappedProfileId && runtime.state.activeProfileId !== mappedProfileId) {
      await runtime.run("setActiveProfile", mappedProfileId);
    }

    if (desiredStoreId && runtime.state.activeStoreId !== desiredStoreId) {
      await runtime.run("setActiveStore", desiredStoreId);
    }

    activeStoreId.value = desiredStoreId || "";

    if (desiredStoreId && accessToken.value) {
      await hydrateRuntimeStoreContext(runtime, apiRequest, desiredStoreId);
    }
  }

  async function fetchContext() {
    if (!accessToken.value) {
      clearSession();
      hydrated.value = true;
      return null;
    }

    const response = await apiRequest("/v1/me/context");
    user.value = response.user;
    principal.value = response.principal;
    tenantContext.value = Array.isArray(response.context?.tenants) ? response.context.tenants : [];
    storeContext.value = Array.isArray(response.context?.stores)
      ? response.context.stores.map((store) => normalizeContextStore(store))
      : [];
    const fallbackActiveStoreId = String(
      response.context?.activeStoreId ||
      response.principal?.storeIds?.[0] ||
      storeContext.value[0]?.id ||
      ""
    ).trim();
    const preferredActiveStoreId = String(activeStoreCookie.value || fallbackActiveStoreId).trim();
    activeTenantId.value =
      String(response.context?.activeTenantId || response.principal?.tenantId || tenantContext.value[0]?.id || "").trim();
    activeStoreId.value = storeContext.value.some((store) => store.id === preferredActiveStoreId)
      ? preferredActiveStoreId
      : fallbackActiveStoreId;
    activeStoreCookie.value = activeStoreId.value || null;
    syncStoreScopeMode(storeScopeCookie.value);
    hydrated.value = true;
    lastError.value = "";
    await syncRuntimeAccess();
    return response;
  }

  async function ensureSession() {
    if (hydrated.value) {
      return isAuthenticated.value;
    }

    if (!ensurePromise) {
      ensurePromise = (async () => {
        if (!accessToken.value) {
          clearSession();
          hydrated.value = true;
          return false;
        }

        try {
          await fetchContext();
          return true;
        } catch (error) {
          clearSession();
          hydrated.value = true;
          lastError.value = getApiErrorMessage(error, "Nao foi possivel restaurar a sessao.");
          return false;
        } finally {
          ensurePromise = null;
        }
      })();
    }

    return ensurePromise;
  }

  async function login({ email, password }) {
    pending.value = true;
    lastError.value = "";

    try {
      const response = await $fetch("/v1/auth/login", {
        method: "POST",
        baseURL: getApiBase(runtimeConfig),
        body: {
          email,
          password
        }
      });

      accessToken.value = response.session.accessToken;
      hydrated.value = false;
      await fetchContext();
      return response;
    } catch (error) {
      clearSession();
      hydrated.value = true;
      lastError.value = getApiErrorMessage(error, "Nao foi possivel entrar na plataforma.");
      throw error;
    } finally {
      pending.value = false;
    }
  }

  function getRememberedLogin() {
    if (import.meta.server) {
      return null;
    }

    const remembered = parseRememberedLogin(window.localStorage.getItem(REMEMBERED_LOGIN_STORAGE_KEY));
    if (!remembered) {
      window.localStorage.removeItem(REMEMBERED_LOGIN_STORAGE_KEY);
      return null;
    }

    return remembered;
  }

  function saveRememberedLogin(payload = {}) {
    if (import.meta.server) {
      return;
    }

    const email = String(payload.email || "").trim().toLowerCase();
    const password = String(payload.password || "");

    if (!email || !password) {
      clearRememberedLogin();
      return;
    }

    window.localStorage.setItem(REMEMBERED_LOGIN_STORAGE_KEY, JSON.stringify({
      email,
      password
    }));
  }

  function clearRememberedLogin() {
    if (import.meta.client) {
      window.localStorage.removeItem(REMEMBERED_LOGIN_STORAGE_KEY);
    }
  }

  async function fetchInvitation(token) {
    const normalizedToken = String(token || "").trim();
    if (!normalizedToken) {
      throw new Error("Convite invalido.");
    }

    return $fetch(`/v1/auth/invitations/${encodeURIComponent(normalizedToken)}`, {
      method: "GET",
      baseURL: getApiBase(runtimeConfig)
    });
  }

  async function acceptInvitation({ token, password }) {
    pending.value = true;
    lastError.value = "";

    try {
      const response = await $fetch("/v1/auth/invitations/accept", {
        method: "POST",
        baseURL: getApiBase(runtimeConfig),
        body: {
          token,
          password
        }
      });

      accessToken.value = response.session.accessToken;
      hydrated.value = false;
      await fetchContext();
      return response;
    } catch (error) {
      lastError.value = getApiErrorMessage(error, "Nao foi possivel concluir o convite.");
      throw error;
    } finally {
      pending.value = false;
    }
  }

  async function requestPasswordReset({ email }) {
    pending.value = true;
    lastError.value = "";

    try {
      return await $fetch("/v1/auth/password-reset/request", {
        method: "POST",
        baseURL: getApiBase(runtimeConfig),
        body: {
          email
        }
      });
    } catch (error) {
      lastError.value = getApiErrorMessage(error, "Nao foi possivel enviar o codigo de recuperacao.");
      throw error;
    } finally {
      pending.value = false;
    }
  }

  async function confirmPasswordReset({ email, code, password }) {
    pending.value = true;
    lastError.value = "";

    try {
      return await $fetch("/v1/auth/password-reset/confirm", {
        method: "POST",
        baseURL: getApiBase(runtimeConfig),
        body: {
          email,
          code,
          password
        }
      });
    } catch (error) {
      lastError.value = getApiErrorMessage(error, "Nao foi possivel redefinir a senha.");
      throw error;
    } finally {
      pending.value = false;
    }
  }

  async function setActiveStore(storeId) {
    const normalizedStoreId = String(storeId || "").trim();

    if (!normalizedStoreId) {
      return;
    }

    if (accessibleStoreIds.value.length && !accessibleStoreIds.value.includes(normalizedStoreId)) {
      return;
    }

    activeStoreId.value = normalizedStoreId;
    activeStoreCookie.value = normalizedStoreId;
    const activeStore = storeContext.value.find((store) => store.id === normalizedStoreId);

    if (activeStore?.tenantId) {
      activeTenantId.value = activeStore.tenantId;
    }

    await runtime.ensure();

    if (storeContext.value.length) {
      runtime.hydrate({
        ...runtime.state,
        stores: storeContext.value.map((store) => normalizeContextStore(store)),
        activeStoreId: normalizedStoreId
      });
    }

    if (runtime.state.activeStoreId !== normalizedStoreId) {
      await runtime.run("setActiveStore", normalizedStoreId);
    }

    await hydrateRuntimeStoreContext(runtime, apiRequest, normalizedStoreId);
  }

  function setStoreScopeMode(mode) {
    return syncStoreScopeMode(mode);
  }

  async function updateProfile(payload = {}) {
    await ensureSession();

    const response = await apiRequest("/v1/auth/me/profile", {
      method: "PATCH",
      body: {
        displayName: String(payload.displayName || "").trim(),
        email: String(payload.email || "").trim().toLowerCase()
      }
    });

    user.value = response.user || user.value;
    await fetchContext();
    return response;
  }

  async function changePassword(payload = {}) {
    await ensureSession();

    const response = await apiRequest("/v1/auth/me/password", {
      method: "PATCH",
      body: {
        currentPassword: String(payload.currentPassword || ""),
        newPassword: String(payload.newPassword || "")
      }
    });

    user.value = response.user || user.value;
    await fetchContext();
    return response;
  }

  async function uploadAvatar(file) {
    await ensureSession();

    const formData = new FormData();
    formData.append("avatar", file);

    const response = await apiRequest("/v1/auth/me/avatar", {
      method: "POST",
      body: formData
    });

    user.value = response.user || user.value;
    await fetchContext();
    return response;
  }

  async function logout() {
    clearSession();
    hydrated.value = true;
  }

  return {
    accessToken,
    user,
    principal,
    tenantContext,
    storeContext,
    activeTenantId,
    activeStoreId,
    storeScopeMode,
    hydrated,
    pending,
    lastError,
    role,
    permissionKeys,
    permissionsResolved,
    isAuthenticated,
    mustChangePassword,
    allowedWorkspaces,
    homeWorkspaceId,
    homePath,
    accessibleStoreIds,
    canUseAllStores,
    isAllStoresScope,
    ensureSession,
    fetchContext,
    fetchMe: fetchContext,
    fetchInvitation,
    acceptInvitation,
    requestPasswordReset,
    confirmPasswordReset,
    login,
    logout,
    clearSession,
    getRememberedLogin,
    saveRememberedLogin,
    clearRememberedLogin,
    syncRuntimeAccess,
    setActiveStore,
    setStoreScopeMode,
    updateProfile,
    changePassword,
    uploadAvatar
  };
});
