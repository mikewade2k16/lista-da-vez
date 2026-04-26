import { computed, ref, watch } from "vue";
import { defineStore } from "pinia";

import { canManageUserPasswords, canManageUsers } from "~/domain/utils/permissions";
import { useAuthStore } from "~/stores/auth";
import { createApiRequest, getApiBase, getApiErrorMessage } from "~/utils/api-client";

function normalizeText(value) {
  return String(value || "").trim();
}

function normalizeEmail(value) {
  return normalizeText(value).toLowerCase();
}

function normalizeStoreIds(storeIds = []) {
  const seen = new Set();
  return (Array.isArray(storeIds) ? storeIds : [])
    .map((storeId) => normalizeText(storeId))
    .filter((storeId) => {
      if (!storeId || seen.has(storeId)) {
        return false;
      }

      seen.add(storeId);
      return true;
    });
}

function normalizeBoolean(value, fallback = true) {
  if (value === undefined || value === null) {
    return fallback;
  }

  return Boolean(value);
}

function normalizeInvitation(invitation) {
  if (!invitation || typeof invitation !== "object") {
    return null;
  }

  return {
    inviteUrl: normalizeText(invitation.inviteUrl),
    invitation: invitation.invitation || null
  };
}

function normalizeUser(user) {
  if (!user || typeof user !== "object") {
    return null;
  }

  return {
    ...user,
    displayName: normalizeText(user.displayName),
    email: normalizeEmail(user.email),
    employeeCode: normalizeText(user.employeeCode),
    jobTitle: normalizeText(user.jobTitle),
    tenantId: normalizeText(user.tenantId),
    storeIds: normalizeStoreIds(user.storeIds),
    managedBy: normalizeText(user.managedBy),
    managedResourceId: normalizeText(user.managedResourceId),
    active: normalizeBoolean(user.active, true),
    onboarding: user.onboarding || {
      status: "needs_invite",
      hasPassword: false,
      mustChangePassword: false,
      invitationExpiresAt: null
    }
  };
}

function isStoreScopedRole(role) {
  return role === "consultant" || role === "manager" || role === "store_terminal";
}

function isConsultantManagedUser(user) {
  return normalizeText(user?.managedBy) === "consultants" || normalizeText(user?.role) === "consultant";
}

function canOverrideConsultantManaged(role) {
  return normalizeText(role) === "platform_admin";
}

export const useUsersStore = defineStore("users", () => {
  const runtimeConfig = useRuntimeConfig();
  const auth = useAuthStore();
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);

  const users = ref([]);
  const roleCatalog = ref([]);
  const pending = ref(false);
  const ready = ref(false);
  const errorMessage = ref("");

  const manageable = computed(() => canManageUsers(auth.role, auth.permissionKeys, auth.permissionsResolved));
  const activeTenantId = computed(() => normalizeText(auth.activeTenantId || auth.tenantContext?.[0]?.id));
  const availableStores = computed(() =>
    (auth.storeContext || []).filter((store) => !activeTenantId.value || store.tenantId === activeTenantId.value)
  );
  const assignableRoles = computed(() =>
    roleCatalog.value.filter((role) => auth.role === "platform_admin" || role.id !== "platform_admin")
  );

  function upsertLocalUser(user) {
    const normalizedUser = normalizeUser(user);
    if (!normalizedUser) {
      return null;
    }

    const existingIndex = users.value.findIndex(
      (currentUser) => normalizeText(currentUser?.id) === normalizeText(normalizedUser.id)
    );

    if (existingIndex >= 0) {
      users.value.splice(existingIndex, 1, normalizedUser);
    } else {
      users.value = [...users.value, normalizedUser];
    }

    ready.value = true;
    return normalizedUser;
  }

  async function ensureRoleCatalog() {
    if (roleCatalog.value.length) {
      return roleCatalog.value;
    }

    const response = await $fetch("/v1/auth/roles", {
      method: "GET",
      baseURL: getApiBase(runtimeConfig)
    });
    roleCatalog.value = Array.isArray(response?.roles) ? response.roles : [];
    return roleCatalog.value;
  }

  function clearState() {
    users.value = [];
    ready.value = false;
    errorMessage.value = "";
  }

  function buildListQuery() {
    const params = new URLSearchParams();

    if (activeTenantId.value && auth.role !== "platform_admin") {
      params.set("tenantId", activeTenantId.value);
    }
    return params.toString();
  }

  async function refreshUsers(options = {}) {
    await auth.ensureSession();
    await ensureRoleCatalog();

    const silent = Boolean(options?.silent);

    if (!auth.isAuthenticated || !manageable.value) {
      clearState();
      return [];
    }

    if (!silent) {
      pending.value = true;
    }
    errorMessage.value = "";

    try {
      const response = await apiRequest(`/v1/users?${buildListQuery()}`);
      users.value = Array.isArray(response?.users) ? response.users.map((user) => normalizeUser(user)).filter(Boolean) : [];
      ready.value = true;
      return users.value;
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, "Nao foi possivel carregar os usuarios.");
      throw error;
    } finally {
      if (!silent) {
        pending.value = false;
      }
    }
  }

  async function ensureLoaded() {
    await ensureRoleCatalog();

    if (!auth.isAuthenticated || !manageable.value) {
      clearState();
      return false;
    }

    if (ready.value) {
      return true;
    }

    try {
      await refreshUsers();
      return true;
    } catch {
      return false;
    }
  }

  async function createUser(payload = {}) {
    await ensureRoleCatalog();
    await auth.ensureSession();

    if (!auth.isAuthenticated || !manageable.value) {
      return { ok: false, message: "Sem permissao para gerenciar usuarios." };
    }

    const role = normalizeText(payload.role || "store_terminal");
    const password = normalizeText(payload.password);
    const employeeCode = normalizeText(payload.employeeCode);
    if (role === "consultant") {
      return { ok: false, message: "Consultores devem ser criados na gestao de consultores." };
    }
    if (password && !canManageUserPasswords(auth.role, auth.permissionKeys, auth.permissionsResolved)) {
      return { ok: false, message: "Somente o admin da plataforma pode definir senha manualmente." };
    }

    const tenantId =
      role === "platform_admin"
        ? ""
        : normalizeText(payload.tenantId || activeTenantId.value);
    const storeIds = isStoreScopedRole(role)
      ? normalizeStoreIds(payload.storeIds).slice(0, 1)
      : [];

    try {
      const response = await apiRequest("/v1/users", {
        method: "POST",
        body: {
          displayName: normalizeText(payload.displayName),
          email: normalizeEmail(payload.email),
          employeeCode,
          password,
          role,
          tenantId,
          storeIds,
          active: normalizeBoolean(payload.active, true)
        }
      });

      upsertLocalUser(response?.user);
      return {
        ok: true,
        user: response.user,
        invitation: normalizeInvitation(response.invitation ? {
          invitation: response.invitation.invitation,
          inviteUrl: response.invitation.inviteUrl
        } : response.invitation)
      };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel criar usuario.")
      };
    }
  }

  async function inviteUser(userId) {
    await auth.ensureSession();

    if (!auth.isAuthenticated || !manageable.value) {
      return { ok: false, message: "Sem permissao para gerenciar usuarios." };
    }

    const currentUser = users.value.find((user) => user.id === userId);
    if (currentUser && isConsultantManagedUser(currentUser) && !canOverrideConsultantManaged(auth.role)) {
      return { ok: false, message: "Esse acesso de consultor usa senha inicial e deve ser gerenciado na aba Consultores." };
    }

    try {
      const response = await apiRequest(`/v1/users/${encodeURIComponent(String(userId || "").trim())}/invite`, {
        method: "POST"
      });

      upsertLocalUser(response?.user);
      return {
        ok: true,
        user: response.user,
        invitation: normalizeInvitation(response.invitation ? {
          invitation: response.invitation.invitation,
          inviteUrl: response.invitation.inviteUrl
        } : response.invitation)
      };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel gerar o convite.")
      };
    }
  }

  async function updateUser(userId, payload = {}) {
    await ensureRoleCatalog();
    await auth.ensureSession();

    if (!auth.isAuthenticated || !manageable.value) {
      return { ok: false, message: "Sem permissao para gerenciar usuarios." };
    }

    const currentUser = users.value.find((user) => user.id === userId);
    if (!currentUser) {
      return { ok: false, message: "Usuario nao encontrado." };
    }
    if (isConsultantManagedUser(currentUser) && !canOverrideConsultantManaged(auth.role)) {
      return { ok: false, message: "Esse acesso de consultor deve ser gerenciado na aba Consultores." };
    }

    const role = normalizeText(payload.role || currentUser.role);
    const body = {};
    const nextDisplayName = normalizeText(payload.displayName ?? currentUser.displayName);
    const nextEmail = normalizeEmail(payload.email ?? currentUser.email);
    const nextEmployeeCode = normalizeText(payload.employeeCode ?? currentUser.employeeCode);
    const nextPassword = normalizeText(payload.password);
    const nextTenantId = role === "platform_admin"
      ? ""
      : normalizeText(payload.tenantId ?? currentUser.tenantId ?? activeTenantId.value);
    const nextStoreIds = isStoreScopedRole(role)
      ? normalizeStoreIds(payload.storeIds ?? currentUser.storeIds).slice(0, 1)
      : [];
    const nextActive = normalizeBoolean(payload.active, currentUser.active);

    if (nextDisplayName !== normalizeText(currentUser.displayName)) {
      body.displayName = nextDisplayName;
    }

    if (nextEmail !== normalizeEmail(currentUser.email)) {
      body.email = nextEmail;
    }

    if (nextEmployeeCode !== normalizeText(currentUser.employeeCode)) {
	  body.employeeCode = nextEmployeeCode;
	}

    if (role !== normalizeText(currentUser.role)) {
      body.role = role;
    }

    if (nextTenantId !== normalizeText(currentUser.tenantId)) {
      body.tenantId = nextTenantId;
    }

    if (JSON.stringify(nextStoreIds) !== JSON.stringify(normalizeStoreIds(currentUser.storeIds))) {
      body.storeIds = nextStoreIds;
    }

    if (nextActive !== Boolean(currentUser.active)) {
      body.active = nextActive;
    }

    if (nextPassword) {
      if (!canManageUserPasswords(auth.role, auth.permissionKeys, auth.permissionsResolved)) {
        return { ok: false, message: "Somente o admin da plataforma pode alterar senhas pelo painel." };
      }
      body.password = nextPassword;
    }

    if (!Object.keys(body).length) {
      return { ok: true, noChange: true };
    }

    try {
      const response = await apiRequest(`/v1/users/${encodeURIComponent(String(userId || "").trim())}`, {
        method: "PATCH",
        body
      });

      upsertLocalUser(response?.user);
      return {
        ok: true,
        user: response.user
      };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel atualizar usuario.")
      };
    }
  }

  async function archiveUser(userId) {
    await auth.ensureSession();

    if (!auth.isAuthenticated || !manageable.value) {
      return { ok: false, message: "Sem permissao para gerenciar usuarios." };
    }

    const currentUser = users.value.find((user) => user.id === userId);
    if (currentUser && isConsultantManagedUser(currentUser) && !canOverrideConsultantManaged(auth.role)) {
      return { ok: false, message: "Esse acesso de consultor deve ser inativado pela gestao de consultores." };
    }

    try {
      const response = await apiRequest(`/v1/users/${encodeURIComponent(String(userId || "").trim())}/archive`, {
        method: "POST"
      });

      upsertLocalUser(response?.user);
      return {
        ok: true,
        user: response.user
      };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel inativar usuario.")
      };
    }
  }

  async function resetPassword(userId, password) {
    await auth.ensureSession();

    if (!auth.isAuthenticated || !manageable.value) {
      return { ok: false, message: "Sem permissao para gerenciar usuarios." };
    }
    if (!canManageUserPasswords(auth.role, auth.permissionKeys, auth.permissionsResolved)) {
      return { ok: false, message: "Somente o admin da plataforma pode resetar senhas pelo painel." };
    }

    try {
      const response = await apiRequest(`/v1/users/${encodeURIComponent(String(userId || "").trim())}/reset-password`, {
        method: "POST",
        body: {
          password: normalizeText(password)
        }
      });

      upsertLocalUser(response?.user);
      return {
        ok: true,
        user: response.user,
        temporaryPassword: normalizeText(response.temporaryPassword)
      };
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, "Nao foi possivel redefinir a senha.")
      };
    }
  }

  if (import.meta.client) {
    watch(
      () => [auth.isAuthenticated, auth.activeTenantId, auth.role],
      ([isAuthenticated, tenantId, role], [previousAuthenticated, previousTenantId, previousRole]) => {
        if (!isAuthenticated || !canManageUsers(role, auth.permissionKeys, auth.permissionsResolved)) {
          clearState();
          return;
        }

        if (!previousAuthenticated || previousTenantId !== tenantId || previousRole !== role) {
          void refreshUsers().catch(() => {});
        }
      }
    );
  }

  return {
    users,
    roleCatalog,
    assignableRoles,
    availableStores,
    pending,
    ready,
    errorMessage,
    manageable,
    ensureLoaded,
    refreshUsers,
    createUser,
    inviteUser,
    updateUser,
    archiveUser,
    resetPassword
  };
});
