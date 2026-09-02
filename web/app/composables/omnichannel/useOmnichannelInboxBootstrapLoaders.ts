import type { Ref } from "vue";
import type {
  Contact,
  TenantSettings,
  TenantUser,
  WhatsAppInstanceAccessResponse,
  WhatsAppInstanceRecord,
  WhatsAppStatusResponse
} from "~/types";
import {
  DEFAULT_MAX_UPLOAD_MB,
  normalizeTenantUploadLimitMb,
  type WhatsAppInstanceAccessState,
  toArrayOrEmpty
} from "~/composables/omnichannel/useOmnichannelInboxShared";

export function useOmnichannelInboxBootstrapLoaders(options: {
  tenantMaxUploadMb: Ref<number>;
  whatsappStatus: Ref<WhatsAppStatusResponse | null>;
  users: Ref<TenantUser[]>;
  contacts: Ref<Contact[]>;
  whatsappInstances: Ref<WhatsAppInstanceRecord[]>;
  whatsappInstanceAccessState: Ref<WhatsAppInstanceAccessState>;
  selectedInstanceId: Ref<string>;
  loadingWhatsAppStatus: Ref<boolean>;
  loadingUsers: Ref<boolean>;
  loadingContacts: Ref<boolean>;
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>;
  sortContacts: () => void;
  getScopeIdentity: () => string;
}) {
  const STATUS_CACHE_TTL_MS = 12_000;
  const statusRequestsInFlight = new Map<string, Promise<void>>();
  const lastStatusFetchedAtByScope = new Map<string, number>();

  function isCurrentScope(scopeIdentity: string) {
    return options.getScopeIdentity() === scopeIdentity;
  }

  function buildProviderUnavailableState() {
    return {
      instance: {
        state: "provider_unavailable"
      }
    } satisfies Record<string, unknown>;
  }

  async function loadTenantUploadLimit() {
    const scopeIdentity = options.getScopeIdentity();
    try {
      const tenantSettings = await options.apiFetch<TenantSettings>("/tenant");
      if (!isCurrentScope(scopeIdentity)) return;
      options.tenantMaxUploadMb.value = normalizeTenantUploadLimitMb(tenantSettings.maxUploadMb);
    } catch {
      if (!isCurrentScope(scopeIdentity)) return;
      options.tenantMaxUploadMb.value = DEFAULT_MAX_UPLOAD_MB;
    }
  }

  async function loadWhatsAppStatus(optionsArg: { force?: boolean } = {}) {
    const force = optionsArg.force ?? false;
    const now = Date.now();
    const scopeIdentity = options.getScopeIdentity();
    if (options.whatsappInstanceAccessState.value === "error") {
      options.whatsappStatus.value = null;
      return;
    }
    const statusRequestInFlight = statusRequestsInFlight.get(scopeIdentity);

    if (statusRequestInFlight) {
      await statusRequestInFlight;
      return;
    }

    if (
      !force &&
      options.whatsappStatus.value &&
      now - (lastStatusFetchedAtByScope.get(scopeIdentity) ?? 0) < STATUS_CACHE_TTL_MS
    ) {
      return;
    }

    const request = (async () => {
      options.loadingWhatsAppStatus.value = true;
      try {
        const query = new URLSearchParams();
        if (options.selectedInstanceId.value && options.selectedInstanceId.value !== "all") {
          query.set("instanceId", options.selectedInstanceId.value);
        }
        const statusResponse = await options.apiFetch<WhatsAppStatusResponse>(
          `/tenant/whatsapp/status${query.size ? `?${query.toString()}` : ""}`
        );
        if (!isCurrentScope(scopeIdentity)) return;
        options.whatsappStatus.value = statusResponse;
      } catch {
        if (!isCurrentScope(scopeIdentity)) return;
        const hasKnownInstance =
          options.whatsappInstances.value.length > 0 ||
          (options.selectedInstanceId.value.trim().length > 0 && options.selectedInstanceId.value !== "all");

        if (options.whatsappStatus.value) {
          options.whatsappStatus.value = {
            ...options.whatsappStatus.value,
            configured: options.whatsappStatus.value.configured || hasKnownInstance,
            providerUnavailable: hasKnownInstance,
            degraded: hasKnownInstance,
            connectionState:
              options.whatsappStatus.value.connectionState ??
              (hasKnownInstance ? buildProviderUnavailableState() : undefined),
            message: "Status temporariamente indisponivel. Mantendo ultimo estado conhecido."
          };
        } else {
          options.whatsappStatus.value = {
            configured: hasKnownInstance,
            providerUnavailable: hasKnownInstance,
            degraded: hasKnownInstance,
            connectionState: hasKnownInstance ? buildProviderUnavailableState() : undefined,
            message: hasKnownInstance
              ? "Conexao com a Evolution temporariamente indisponivel. Mantendo a inbox em modo degradado."
              : "Nao foi possivel consultar status do canal WhatsApp."
          };
        }
      } finally {
        lastStatusFetchedAtByScope.set(scopeIdentity, Date.now());
        if (isCurrentScope(scopeIdentity)) {
          options.loadingWhatsAppStatus.value = false;
        }
      }
    })();

    statusRequestsInFlight.set(scopeIdentity, request);
    try {
      await request;
    } finally {
      if (statusRequestsInFlight.get(scopeIdentity) === request) {
        statusRequestsInFlight.delete(scopeIdentity);
      }
    }
  }

  async function loadUsers() {
    const scopeIdentity = options.getScopeIdentity();
    options.loadingUsers.value = true;
    try {
      const response = await options.apiFetch<unknown>("/users");
      if (!isCurrentScope(scopeIdentity)) return;
      options.users.value = toArrayOrEmpty<TenantUser>(response);
    } finally {
      if (isCurrentScope(scopeIdentity)) options.loadingUsers.value = false;
    }
  }

  async function loadContacts() {
    const scopeIdentity = options.getScopeIdentity();
    options.loadingContacts.value = true;
    try {
      const response = await options.apiFetch<unknown>("/contacts");
      if (!isCurrentScope(scopeIdentity)) return;
      options.contacts.value = toArrayOrEmpty<Contact>(response);
      options.sortContacts();
    } finally {
      if (isCurrentScope(scopeIdentity)) options.loadingContacts.value = false;
    }
  }

  async function loadAccessibleWhatsAppInstances() {
    const scopeIdentity = options.getScopeIdentity();
    options.whatsappInstanceAccessState.value = "loading";
    try {
      const response = await options.apiFetch<WhatsAppInstanceAccessResponse>("/tenant/whatsapp/instances/access");
      if (!isCurrentScope(scopeIdentity)) return;
      const instances = Array.isArray(response.instances) ? response.instances : [];
      options.whatsappInstances.value = instances;
      options.whatsappInstanceAccessState.value = instances.length > 0
        ? "resolved-nonempty"
        : "resolved-empty";
    } catch (error) {
      if (!isCurrentScope(scopeIdentity)) return;
      options.whatsappInstances.value = [];
      options.whatsappInstanceAccessState.value = "error";
      throw error;
    }
  }

  return {
    loadTenantUploadLimit,
    loadWhatsAppStatus,
    loadUsers,
    loadContacts,
    loadAccessibleWhatsAppInstances
  };
}
