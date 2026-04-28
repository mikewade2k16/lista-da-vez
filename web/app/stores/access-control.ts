import { computed, ref } from "vue";
import { defineStore } from "pinia";

import { normalizePermissionKeys } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";

function normalizePermission(definition) {
  return {
    key: String(definition?.key || "").trim(),
    scope: String(definition?.scope || "").trim(),
    description: String(definition?.description || "").trim()
  };
}

function normalizeRoleEntry(entry) {
  return {
    role: String(entry?.role || "").trim(),
    label: String(entry?.label || "").trim(),
    scope: String(entry?.scope || "").trim(),
    permissionKeys: normalizePermissionKeys(entry?.permissionKeys || [])
  };
}

function normalizeOverride(override) {
  return {
    id: String(override?.id || "").trim(),
    userId: String(override?.userId || "").trim(),
    permissionKey: String(override?.permissionKey || "").trim(),
    effect: String(override?.effect || "").trim(),
    tenantId: String(override?.tenantId || "").trim(),
    storeId: String(override?.storeId || "").trim(),
    note: String(override?.note || "").trim(),
    isActive: Boolean(override?.isActive ?? true)
  };
}

function normalizeUserAccess(access) {
  return {
    userId: String(access?.userId || "").trim(),
    role: String(access?.role || "").trim(),
    tenantId: String(access?.tenantId || "").trim(),
    storeIds: Array.isArray(access?.storeIds) ? access.storeIds.map((storeId) => String(storeId || "").trim()).filter(Boolean) : [],
    permissions: Array.isArray(access?.permissions) ? access.permissions.map(normalizePermission).filter((permission) => permission.key) : [],
    basePermissionKeys: normalizePermissionKeys(access?.basePermissionKeys || []),
    effectivePermissionKeys: normalizePermissionKeys(access?.effectivePermissionKeys || []),
    overrides: Array.isArray(access?.overrides) ? access.overrides.map(normalizeOverride).filter((override) => override.permissionKey) : []
  };
}

export const useAccessControlStore = defineStore("access-control", () => {
  const runtimeConfig = useRuntimeConfig();
  const auth = useAuthStore();
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);

  const permissions = ref([]);
  const roleMatrix = ref([]);
  const userAccessById = ref({});
  const pendingRoles = ref(false);
  const pendingUserIds = ref({});
  const errorMessage = ref("");

  const roleLookup = computed(() => new Map(roleMatrix.value.map((entry) => [entry.role, entry])));

  function clearState() {
    permissions.value = [];
    roleMatrix.value = [];
    userAccessById.value = {};
    pendingRoles.value = false;
    pendingUserIds.value = {};
    errorMessage.value = "";
  }

  async function refreshRoleMatrix() {
    await auth.ensureSession();
    if (!auth.isAuthenticated) {
      clearState();
      return [];
    }

    pendingRoles.value = true;
    errorMessage.value = "";

    try {
      const response = await apiRequest("/v1/access/roles");
      permissions.value = Array.isArray(response?.permissions)
        ? response.permissions.map(normalizePermission).filter((permission) => permission.key)
        : [];
      roleMatrix.value = Array.isArray(response?.roles)
        ? response.roles.map(normalizeRoleEntry).filter((entry) => entry.role)
        : [];
      return roleMatrix.value;
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, "Nao foi possivel carregar a matriz de acesso.");
      throw error;
    } finally {
      pendingRoles.value = false;
    }
  }

  async function ensureRoleMatrix() {
    if (roleMatrix.value.length && permissions.value.length) {
      return roleMatrix.value;
    }

    try {
      return await refreshRoleMatrix();
    } catch {
      return [];
    }
  }

  async function loadUserAccess(userId) {
    const normalizedUserId = String(userId || "").trim();
    if (!normalizedUserId) {
      return null;
    }

    pendingUserIds.value = {
      ...pendingUserIds.value,
      [normalizedUserId]: true
    };
    errorMessage.value = "";

    try {
      const response = await apiRequest(`/v1/access/users/${encodeURIComponent(normalizedUserId)}`);
      const normalizedAccess = normalizeUserAccess(response?.access);
      userAccessById.value = {
        ...userAccessById.value,
        [normalizedUserId]: normalizedAccess
      };
      if (normalizedAccess.permissions.length) {
        permissions.value = normalizedAccess.permissions;
      }
      return normalizedAccess;
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, "Nao foi possivel carregar o acesso do usuario.");
      throw error;
    } finally {
      pendingUserIds.value = {
        ...pendingUserIds.value,
        [normalizedUserId]: false
      };
    }
  }

  async function saveRolePermissions(roleId, permissionKeys) {
    const normalizedRoleId = String(roleId || "").trim();
    if (!normalizedRoleId) {
      return { ok: false, message: "Perfil invalido." };
    }

    pendingRoles.value = true;
    errorMessage.value = "";

    try {
      const response = await apiRequest(`/v1/access/roles/${encodeURIComponent(normalizedRoleId)}`, {
        method: "PUT",
        body: {
          permissionKeys: normalizePermissionKeys(permissionKeys)
        }
      });

      const entry = normalizeRoleEntry(response?.role);
      roleMatrix.value = roleMatrix.value.map((current) => (current.role === entry.role ? entry : current));
      if (!roleLookup.value.has(entry.role)) {
        roleMatrix.value = [...roleMatrix.value, entry];
      }

      return { ok: true, role: entry };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel salvar o padrao do perfil.")
      };
    } finally {
      pendingRoles.value = false;
    }
  }

  async function saveUserOverrides(userId, overrides) {
    const normalizedUserId = String(userId || "").trim();
    if (!normalizedUserId) {
      return { ok: false, message: "Usuario invalido." };
    }

    pendingUserIds.value = {
      ...pendingUserIds.value,
      [normalizedUserId]: true
    };
    errorMessage.value = "";

    try {
      const response = await apiRequest(`/v1/access/users/${encodeURIComponent(normalizedUserId)}/overrides`, {
        method: "PUT",
        body: {
          overrides: Array.isArray(overrides)
            ? overrides.map((override) => ({
              permissionKey: String(override?.permissionKey || "").trim(),
              effect: String(override?.effect || "").trim(),
              note: String(override?.note || "").trim()
            })).filter((override) => override.permissionKey && override.effect)
            : []
        }
      });

      const normalizedAccess = normalizeUserAccess(response?.access);
      userAccessById.value = {
        ...userAccessById.value,
        [normalizedUserId]: normalizedAccess
      };

      return { ok: true, access: normalizedAccess };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel salvar as permissoes do usuario.")
      };
    } finally {
      pendingUserIds.value = {
        ...pendingUserIds.value,
        [normalizedUserId]: false
      };
    }
  }

  function getUserAccess(userId) {
    return userAccessById.value[String(userId || "").trim()] || null;
  }

  function isUserPending(userId) {
    return Boolean(pendingUserIds.value[String(userId || "").trim()]);
  }

  async function refreshRealtimeState() {
    await auth.ensureSession();
    if (!auth.isAuthenticated) {
      clearState();
      return;
    }

    const refreshes = [];
    if (roleMatrix.value.length || permissions.value.length) {
      refreshes.push(refreshRoleMatrix().catch(() => []));
    }

    for (const userId of Object.keys(userAccessById.value).map((entry) => String(entry || "").trim()).filter(Boolean)) {
      refreshes.push(loadUserAccess(userId).catch(() => null));
    }

    if (!refreshes.length) {
      return;
    }

    await Promise.allSettled(refreshes);
  }

  return {
    permissions,
    roleMatrix,
    errorMessage,
    pendingRoles,
    roleLookup,
    clearState,
    ensureRoleMatrix,
    refreshRoleMatrix,
    loadUserAccess,
    saveRolePermissions,
    saveUserOverrides,
    getUserAccess,
    isUserPending,
		refreshRealtimeState
  };
});