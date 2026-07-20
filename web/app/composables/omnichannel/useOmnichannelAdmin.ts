import { computed, onBeforeUnmount, onMounted, reactive, ref, watchEffect } from "vue";
import type {
  WhatsAppEndpointValidationResponse,
  TenantFailuresDashboardResponse,
  TenantHttpEndpointMetricsResponse,
  TenantSettings,
  TenantUser,
  WhatsAppInstanceUserScopePolicy,
  WhatsAppInstanceAssignableUser,
  WhatsAppInstanceManagementResponse,
  WhatsAppInstanceRecord,
  WhatsAppBootstrapResponse,
  WhatsAppQrCodeResponse,
  WhatsAppStatusResponse
} from "~/types";
import {
  ADMIN_ROLE_ITEMS,
  applyAdminFieldErrors,
  clearAdminFieldErrors,
  createClientFormState,
  createTenantFormState,
  createUserFormState,
  createWhatsAppFormState,
  extractAdminError as extractError,
  extractPairingCode
} from "~/composables/omnichannel/useOmnichannelAdminShared";
import { useOmnichannelAdminConnectionState } from "~/composables/omnichannel/useOmnichannelAdminConnectionState";
import { useOmnichannelAdminClientOps } from "~/composables/omnichannel/useOmnichannelAdminClientOps";
import { useOmnichannelAdminOperationalOps } from "~/composables/omnichannel/useOmnichannelAdminOperationalOps";
import { useOmnichannelAdminQrPolling } from "~/composables/omnichannel/useOmnichannelAdminQrPolling";
import { useOmnichannelAdminTenantOps } from "~/composables/omnichannel/useOmnichannelAdminTenantOps";

export function useOmnichannelAdmin() {
  const { user, coreUser, legacyRole, tenantSlug, syncSessionFromToken } = useAdminSession();
  const { apiFetch } = useApi();
  const STATUS_SILENT_CACHE_TTL_MS = 10_000;
  const QR_SILENT_CACHE_TTL_MS = 15_000;

  const loading = ref(false);
  const savingTenant = ref(false);
  const creatingUser = ref(false);
  const bootstrapping = ref(false);
  const connectingQr = ref(false);
  const connectingPairing = ref(false);
  const disconnectingWhatsApp = ref(false);
  const refreshingStatus = ref(false);
  const fetchingQr = ref(false);
  const loadingFailures = ref(false);
  const loadingHttpEndpointMetrics = ref(false);
  const validatingEndpoints = ref(false);
  const loadingWhatsAppInstances = ref(false);
  const savingWhatsAppInstance = ref(false);
  const savingWhatsAppInstanceUsers = ref(false);

  const tenant = ref<TenantSettings | null>(null);
  const users = ref<TenantUser[]>([]);
  const whatsappInstances = ref<WhatsAppInstanceRecord[]>([]);
  const whatsappInstanceUsers = ref<WhatsAppInstanceAssignableUser[]>([]);
  const statusResult = ref<WhatsAppStatusResponse | null>(null);
  const bootstrapResult = ref<WhatsAppBootstrapResponse | null>(null);
  const qrResult = ref<WhatsAppQrCodeResponse | null>(null);
  const failuresDashboard = ref<TenantFailuresDashboardResponse | null>(null);
  const httpEndpointMetrics = ref<TenantHttpEndpointMetricsResponse | null>(null);
  const endpointValidation = ref<WhatsAppEndpointValidationResponse | null>(null);

  const infoMessage = ref("");
  const errorMessage = ref("");
  const pairingCode = ref<string | null>(null);
  const failureWindowDays = ref(7);

  const tenantForm = reactive(createTenantFormState());
  const tenantFieldErrors = reactive<Record<string, string>>({});
  const clientForm = reactive(createClientFormState());
  const clientFieldErrors = reactive<Record<string, string>>({});
  const userForm = reactive(createUserFormState());
  const userFieldErrors = reactive<Record<string, string>>({});
  const whatsappForm = reactive(createWhatsAppFormState());
  const whatsappInstanceForm = reactive<{
    id: string;
    instanceName: string;
    displayName: string;
    phoneNumber: string;
    evolutionApiKey: string;
    queueLabel: string;
    userScopePolicy: WhatsAppInstanceUserScopePolicy;
    responsibleUserId: string;
    isDefault: boolean;
    isActive: boolean;
    assignedUserIds: string[];
  }>({
    id: "",
    instanceName: "",
    displayName: "",
    phoneNumber: "",
    evolutionApiKey: "",
    queueLabel: "",
    userScopePolicy: "MULTI_INSTANCE",
    responsibleUserId: "",
    isDefault: false,
    isActive: true,
    assignedUserIds: [] as string[]
  });
  const roleItems = ADMIN_ROLE_ITEMS;
  let statusRequestInFlight: Promise<void> | null = null;
  let qrRequestInFlight: Promise<void> | null = null;
  let lastStatusFetchedAt = 0;
  let lastQrFetchedAt = 0;

  const canManageTenant = computed(() => legacyRole.value === "ADMIN");
  const canViewOpsDashboard = computed(() => {
    if (!coreUser.value) {
      return false;
    }

    return legacyRole.value === "ADMIN" || legacyRole.value === "SUPERVISOR";
  });

  const {
    connectionState,
    isConnected,
    connectionBadgeColor,
    connectionStateLabel,
    qrImageSrc,
    hasQrCode,
    connectionAlertColor,
    connectionAlertTitle,
    connectionAlertDescription,
    qrUnavailableMessage
  } = useOmnichannelAdminConnectionState({
    statusResult,
    qrResult
  });

  watchEffect(() => {
    if (user.value && !canViewOpsDashboard.value) {
      void navigateTo("/admin/omnichannel/inbox");
    }
  });

  function setError(message: string) {
    errorMessage.value = message;
  }

  function clearMessages(preserveInfo = false) {
    errorMessage.value = "";
    if (!preserveInfo) {
      infoMessage.value = "";
    }
  }

  function syncSelectedWhatsAppInstance(preferredId?: string | null) {
    const preferred = preferredId
      ? whatsappInstances.value.find((entry) => entry.id === preferredId)
      : null;
    const current = whatsappForm.instanceId
      ? whatsappInstances.value.find((entry) => entry.id === whatsappForm.instanceId)
      : null;
    const selected =
      preferred ??
      current ??
      whatsappInstances.value.find((entry) => entry.isDefault) ??
      whatsappInstances.value[0] ??
      null;

    whatsappForm.instanceId = selected?.id ?? "";
    whatsappForm.instanceName = selected?.instanceName
      ?? tenant.value?.whatsappInstance
      ?? (tenantSlug.value ? `${tenantSlug.value}-whatsapp` : "");
  }

  function resetWhatsAppInstanceForm() {
    whatsappInstanceForm.id = "";
    whatsappInstanceForm.instanceName = "";
    whatsappInstanceForm.displayName = "";
    whatsappInstanceForm.phoneNumber = "";
    whatsappInstanceForm.evolutionApiKey = "";
    whatsappInstanceForm.queueLabel = "";
    whatsappInstanceForm.userScopePolicy = "MULTI_INSTANCE";
    whatsappInstanceForm.responsibleUserId = "";
    whatsappInstanceForm.isDefault = whatsappInstances.value.length < 1;
    whatsappInstanceForm.isActive = true;
    whatsappInstanceForm.assignedUserIds = [];
  }

  function startEditWhatsAppInstance(instance: WhatsAppInstanceRecord) {
    whatsappInstanceForm.id = instance.id;
    whatsappInstanceForm.instanceName = instance.instanceName;
    whatsappInstanceForm.displayName = instance.displayName ?? "";
    whatsappInstanceForm.phoneNumber = instance.phoneNumber ?? "";
    whatsappInstanceForm.evolutionApiKey = "";
    whatsappInstanceForm.queueLabel = instance.queueLabel ?? "";
    whatsappInstanceForm.userScopePolicy = instance.userScopePolicy ?? "MULTI_INSTANCE";
    whatsappInstanceForm.responsibleUserId = instance.responsibleUserId ?? "";
    whatsappInstanceForm.isDefault = instance.isDefault;
    whatsappInstanceForm.isActive = instance.isActive;
    whatsappInstanceForm.assignedUserIds = [...instance.assignedUserIds];
    syncSelectedWhatsAppInstance(instance.id);
  }

  async function loadWhatsAppInstances(options: { silent?: boolean } = {}) {
    if (!canManageTenant.value) {
      whatsappInstances.value = tenant.value?.whatsappInstances ?? [];
      syncSelectedWhatsAppInstance();
      return;
    }

    loadingWhatsAppInstances.value = true;
    if (!options.silent) {
      clearMessages(true);
    }

    try {
      const response = await apiFetch<WhatsAppInstanceManagementResponse>("/tenant/whatsapp/instances");
      whatsappInstances.value = response.instances;
      whatsappInstanceUsers.value = response.users;
      syncSelectedWhatsAppInstance();
      if (!whatsappInstanceForm.id) {
        resetWhatsAppInstanceForm();
      }
    } catch (error) {
      if (!options.silent) {
        setError(extractError(error));
      }
    } finally {
      loadingWhatsAppInstances.value = false;
    }
  }

  async function saveWhatsAppInstance() {
    if (!canManageTenant.value) {
      setError("Perfil sem permissao para gerenciar instancias WhatsApp.");
      return;
    }

    savingWhatsAppInstance.value = true;
    clearMessages();

    try {
      const payload = {
        instanceName: whatsappInstanceForm.instanceName.trim(),
        displayName: whatsappInstanceForm.displayName.trim() || undefined,
        phoneNumber: whatsappInstanceForm.phoneNumber.trim() || undefined,
        evolutionApiKey: whatsappInstanceForm.evolutionApiKey.trim() || undefined,
        queueLabel: whatsappInstanceForm.queueLabel.trim() || undefined,
        userScopePolicy: whatsappInstanceForm.userScopePolicy,
        responsibleUserId: whatsappInstanceForm.responsibleUserId || undefined,
        isDefault: whatsappInstanceForm.isDefault,
        isActive: whatsappInstanceForm.isActive
      };

      const saved = whatsappInstanceForm.id
        ? await apiFetch<WhatsAppInstanceRecord>(`/tenant/whatsapp/instances/${whatsappInstanceForm.id}`, {
            method: "PATCH",
            body: payload
          })
        : await apiFetch<WhatsAppInstanceRecord>("/tenant/whatsapp/instances", {
            method: "POST",
            body: payload
          });

      await Promise.all([
        loadTenant(),
        loadWhatsAppInstances({ silent: true })
      ]);
      syncSelectedWhatsAppInstance(saved.id);
      startEditWhatsAppInstance(
        whatsappInstances.value.find((entry) => entry.id === saved.id) ?? saved
      );
      await refreshWhatsAppStatus({ silent: true, force: true });
      infoMessage.value = whatsappInstanceForm.id
        ? "Instancia WhatsApp atualizada."
        : "Instancia WhatsApp criada.";
    } catch (error) {
      setError(extractError(error));
    } finally {
      savingWhatsAppInstance.value = false;
    }
  }

  async function saveWhatsAppInstanceUsers(instanceId: string) {
    if (!canManageTenant.value) {
      setError("Perfil sem permissao para gerenciar acessos da instancia.");
      return;
    }

    savingWhatsAppInstanceUsers.value = true;
    clearMessages(true);

    try {
      const updated = await apiFetch<WhatsAppInstanceRecord>(`/tenant/whatsapp/instances/${instanceId}/users`, {
        method: "PUT",
        body: {
          userIds: whatsappInstanceForm.assignedUserIds
        }
      });

      await loadWhatsAppInstances({ silent: true });
      startEditWhatsAppInstance(
        whatsappInstances.value.find((entry) => entry.id === updated.id) ?? updated
      );
      infoMessage.value = "Usuarios da instancia atualizados.";
    } catch (error) {
      setError(extractError(error));
    } finally {
      savingWhatsAppInstanceUsers.value = false;
    }
  }

  async function selectWhatsAppInstance(instanceId: string) {
    syncSelectedWhatsAppInstance(instanceId);
    const selected = whatsappInstances.value.find((entry) => entry.id === instanceId);
    if (selected) {
      startEditWhatsAppInstance(selected);
    }
    await Promise.all([
      refreshWhatsAppStatus({ silent: true, force: true }),
      canManageTenant.value ? fetchQrCode({ force: false, silent: true }) : Promise.resolve()
    ]);
  }

  function applyTenantState(data: TenantSettings) {
    tenant.value = data;
    whatsappInstances.value = Array.isArray(data.whatsappInstances) ? data.whatsappInstances : [];
    tenantForm.name = data.name;
    tenantForm.whatsappInstance = data.whatsappInstance ?? "";
    tenantForm.evolutionApiKey = data.evolutionApiKey ?? "";
    tenantForm.maxChannels = data.maxChannels;
    tenantForm.maxUsers = data.maxUsers;
    tenantForm.retentionDays = data.retentionDays;
    tenantForm.maxUploadMb = data.maxUploadMb;
    syncSelectedWhatsAppInstance();
  }

  async function refreshWhatsAppStatus(options: { silent?: boolean; force?: boolean } = {}) {
    const silent = options.silent ?? false;
    const force = options.force ?? false;
    const now = Date.now();

    if (statusRequestInFlight) {
      await statusRequestInFlight;
      if (!force) {
        return;
      }
    }

    if (
      silent &&
      !force &&
      statusResult.value &&
      now - lastStatusFetchedAt < STATUS_SILENT_CACHE_TTL_MS
    ) {
      return;
    }

    const request = (async () => {
      refreshingStatus.value = true;
      if (!silent) {
        clearMessages(true);
      }
      try {
        const query = new URLSearchParams();
        if (whatsappForm.instanceId) {
          query.set("instanceId", whatsappForm.instanceId);
        }
        statusResult.value = await apiFetch<WhatsAppStatusResponse>(`/tenant/whatsapp/status${query.size ? `?${query.toString()}` : ""}`);
      } catch (error) {
        if (statusResult.value) {
          statusResult.value = {
            ...statusResult.value,
            message: "Status temporariamente indisponivel. Mantendo ultimo estado conhecido."
          };
        } else {
          statusResult.value = {
            configured: false,
            message: "Nao foi possivel consultar status do canal WhatsApp."
          };
        }
        if (!silent) {
          setError(extractError(error));
        }
      } finally {
        lastStatusFetchedAt = Date.now();
        refreshingStatus.value = false;
      }
    })();

    statusRequestInFlight = request;
    try {
      await request;
    } finally {
      if (statusRequestInFlight === request) {
        statusRequestInFlight = null;
      }
    }
  }

  async function fetchQrCode(options: { force?: boolean; silent?: boolean } = {}) {
    if (!canManageTenant.value) {
      return;
    }

    const force = options.force ?? true;
    const silent = options.silent ?? false;
    const now = Date.now();

    if (qrRequestInFlight) {
      await qrRequestInFlight;
      if (!force) {
        return;
      }
    }

    if (
      !force &&
      silent &&
      qrResult.value &&
      now - lastQrFetchedAt < QR_SILENT_CACHE_TTL_MS
    ) {
      return;
    }

    const request = (async () => {
      fetchingQr.value = true;
      if (!silent) {
        clearMessages(true);
      }
      try {
        const query = new URLSearchParams();
        query.set("force", force ? "true" : "false");
        if (whatsappForm.instanceId) {
          query.set("instanceId", whatsappForm.instanceId);
        }
        const data = await apiFetch<WhatsAppQrCodeResponse>(`/tenant/whatsapp/qrcode?${query.toString()}`);
        qrResult.value = data;
        if (data.pairingCode) {
          pairingCode.value = data.pairingCode;
        }
        if (!silent) {
          infoMessage.value = data.qrCode
            ? "QR atualizado. Escaneie no app WhatsApp."
            : (data.message || qrUnavailableMessage.value);
        }
      } catch (error) {
        if (!silent) {
          setError(extractError(error));
        }
      } finally {
        lastQrFetchedAt = Date.now();
        fetchingQr.value = false;
      }
    })();

    qrRequestInFlight = request;
    try {
      await request;
    } finally {
      if (qrRequestInFlight === request) {
        qrRequestInFlight = null;
      }
    }
  }

  const { stopQrPolling, startQrPolling } = useOmnichannelAdminQrPolling({
    isConnected,
    hasQrCode,
    refreshWhatsAppStatus,
    fetchQrCode
  });

  const {
    bootstrapWhatsApp,
    connectWithQr,
    disconnectWhatsAppSession,
    generatePairingCode,
    loadFailuresDashboard,
    loadHttpEndpointMetrics,
    validateEvolutionEndpoints
  } = useOmnichannelAdminOperationalOps({
    canManageTenant,
    canViewOpsDashboard,
    bootstrapping,
    connectingQr,
    connectingPairing,
    disconnectingWhatsApp,
    loadingFailures,
    loadingHttpEndpointMetrics,
    validatingEndpoints,
    bootstrapResult,
    qrResult,
    failuresDashboard,
    httpEndpointMetrics,
    endpointValidation,
    infoMessage,
    pairingCode,
    failureWindowDays,
    whatsappForm,
    hasQrCode,
    qrUnavailableMessage,
    clearMessages,
    setError,
    extractError,
    apiFetch,
    loadTenant: async () => {
      const data = await apiFetch<TenantSettings>("/tenant");
      applyTenantState(data);
    },
    refreshWhatsAppStatus,
    fetchQrCode,
    startQrPolling,
    extractPairingCode
  });

  const {
    loadTenant,
    loadUsers,
    loadInitialData,
    saveTenant,
    createUser
  } = useOmnichannelAdminTenantOps({
    user,
    canManageTenant,
    tenant,
    users,
    loading,
    savingTenant,
    creatingUser,
    infoMessage,
    clearMessages,
    setError,
    extractError,
    clearFieldErrors: clearAdminFieldErrors,
    applyFieldErrors: applyAdminFieldErrors,
    tenantFieldErrors,
    userFieldErrors,
    apiFetch,
    tenantForm,
    userForm,
    whatsappForm,
    refreshWhatsAppStatus,
    loadFailuresDashboard,
    loadHttpEndpointMetrics,
    fetchQrCode
  });

  const {
    clients,
    loadingClients,
    savingClient,
    deletingClientId,
    editingClientId,
    selectedClientId,
    selectedClient,
    createdClientAccess,
    savingUser,
    deletingUserId,
    editingUserId,
    loadClients,
    resetClientForm,
    dismissCreatedClientAccess,
    startEditClient,
    saveClient,
    deleteClient,
    selectClient,
    resetUserForm,
    startEditUser,
    saveUser,
    deleteUser
  } = useOmnichannelAdminClientOps({
    user,
    canManageTenant,
    tenant,
    users,
    infoMessage,
    clearMessages,
    setError,
    extractError,
    clearFieldErrors: clearAdminFieldErrors,
    applyFieldErrors: applyAdminFieldErrors,
    clientFieldErrors,
    userFieldErrors,
    apiFetch,
    clientForm,
    userForm
  });

  const switchingTenant = ref(false);
  const switchTenantError = ref("");

  async function reloadAllData() {
    stopQrPolling();
    tenant.value = null;
    users.value = [];
    whatsappInstances.value = [];
    whatsappInstanceUsers.value = [];
    statusResult.value = null;
    bootstrapResult.value = null;
    qrResult.value = null;
    failuresDashboard.value = null;
    httpEndpointMetrics.value = null;
    endpointValidation.value = null;
    pairingCode.value = null;
    clearMessages(true);
    lastStatusFetchedAt = 0;
    lastQrFetchedAt = 0;

    await loadInitialData();
    if (canManageTenant.value) {
      await loadWhatsAppInstances({ silent: true });
      await loadClients();
    }
    if (canManageTenant.value && !isConnected.value) {
      startQrPolling();
    }
  }

  async function switchTenant(clientId: number) {
    if (switchingTenant.value) {
      return;
    }

    switchingTenant.value = true;
    switchTenantError.value = "";
    try {
      const response = await apiFetch<{
        token: string;
        sessionContext: {
          tenantId: string;
          tenantSlug: string;
        };
      }>("/session/context", {
        method: "POST",
        body: { clientId }
      });

      await syncSessionFromToken({
        accessToken: response.token,
        sessionContext: response.sessionContext,
        redirectToLogin: true
      });
      await reloadAllData();
    } catch (error) {
      switchTenantError.value = error instanceof Error
        ? error.message
        : "Falha ao atualizar o contexto operacional.";
    } finally {
      switchingTenant.value = false;
    }
  }

  onMounted(async () => {
    if (canViewOpsDashboard.value) {
      await loadInitialData();
      if (canManageTenant.value) {
        await loadWhatsAppInstances({ silent: true });
        await loadClients();
      }
      if (canManageTenant.value && !isConnected.value) {
        startQrPolling();
      }
    }
  });

  onBeforeUnmount(() => {
    stopQrPolling();
  });

  return {
    user,
    tenantSlug,
    loading,
    loadingClients,
    savingTenant,
    creatingUser,
    savingUser,
    savingClient,
    bootstrapping,
    connectingQr,
    connectingPairing,
    disconnectingWhatsApp,
    refreshingStatus,
    fetchingQr,
    loadingFailures,
    loadingHttpEndpointMetrics,
    validatingEndpoints,
    loadingWhatsAppInstances,
    savingWhatsAppInstance,
    savingWhatsAppInstanceUsers,
    tenant,
    clients,
    users,
    whatsappInstances,
    whatsappInstanceUsers,
    statusResult,
    bootstrapResult,
    qrResult,
    failuresDashboard,
    httpEndpointMetrics,
    endpointValidation,
    infoMessage,
    errorMessage,
    createdClientAccess,
    pairingCode,
    failureWindowDays,
    tenantForm,
    tenantFieldErrors,
    clientForm,
    clientFieldErrors,
    userForm,
    userFieldErrors,
    whatsappForm,
    whatsappInstanceForm,
    roleItems,
    canManageTenant,
    canViewOpsDashboard,
    connectionState,
    connectionStateLabel,
    isConnected,
    connectionBadgeColor,
    connectionAlertColor,
    connectionAlertTitle,
    connectionAlertDescription,
    qrImageSrc,
    hasQrCode,
    qrUnavailableMessage,
    saveTenant,
    createUser,
    deletingClientId,
    editingClientId,
    selectedClientId,
    selectedClient,
    deletingUserId,
    editingUserId,
    loadClients,
    resetClientForm,
    dismissCreatedClientAccess,
    startEditClient,
    saveClient,
    deleteClient,
    selectClient,
    resetUserForm,
    startEditUser,
    saveUser,
    deleteUser,
    refreshWhatsAppStatus,
    fetchQrCode,
    loadWhatsAppInstances,
    resetWhatsAppInstanceForm,
    startEditWhatsAppInstance,
    saveWhatsAppInstance,
    saveWhatsAppInstanceUsers,
    selectWhatsAppInstance,
    bootstrapWhatsApp,
    connectWithQr,
    disconnectWhatsAppSession,
    generatePairingCode,
    loadFailuresDashboard,
    loadHttpEndpointMetrics,
    validateEvolutionEndpoints,
    switchTenant,
    switchingTenant,
    switchTenantError,
    reloadAllData
  };
}
