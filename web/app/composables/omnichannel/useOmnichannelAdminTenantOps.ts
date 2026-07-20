import type { ComputedRef, Ref } from "vue";
import type {
  TenantSettings,
  TenantUser
} from "~/types";

export function useOmnichannelAdminTenantOps(options: {
  user: Ref<{ role?: string | null } | null>;
  canManageTenant: ComputedRef<boolean>;
  tenant: Ref<TenantSettings | null>;
  users: Ref<TenantUser[]>;
  loading: Ref<boolean>;
  savingTenant: Ref<boolean>;
  creatingUser: Ref<boolean>;
  infoMessage: Ref<string>;
  clearMessages: (preserveInfo?: boolean) => void;
  setError: (message: string) => void;
  extractError: (error: unknown) => string;
  clearFieldErrors: (target: Record<string, string>) => void;
  applyFieldErrors: (target: Record<string, string>, error: unknown) => boolean;
  tenantFieldErrors: Record<string, string>;
  userFieldErrors: Record<string, string>;
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>;
  tenantForm: {
    name: string;
    whatsappInstance: string;
    evolutionApiKey: string;
    maxChannels: number;
    maxUsers: number;
    retentionDays: number;
    maxUploadMb: number;
  };
  userForm: {
    name: string;
    email: string;
    password: string;
    role: TenantUser["role"];
  };
  whatsappForm: {
    instanceId: string;
    instanceName: string;
  };
  refreshWhatsAppStatus: (options?: { silent?: boolean; force?: boolean }) => Promise<void>;
  loadFailuresDashboard: (options?: { days?: number; silent?: boolean }) => Promise<void>;
  loadHttpEndpointMetrics: (options?: { silent?: boolean }) => Promise<void>;
  fetchQrCode: (options?: { force?: boolean; silent?: boolean }) => Promise<void>;
}) {
  async function loadTenant() {
    const data = await options.apiFetch<TenantSettings>("/tenant");
    options.tenant.value = data;
    options.tenantForm.name = data.name;
    options.tenantForm.whatsappInstance = data.whatsappInstance ?? "";
    options.tenantForm.evolutionApiKey = data.evolutionApiKey ?? "";
    options.tenantForm.maxChannels = data.maxChannels;
    options.tenantForm.maxUsers = data.maxUsers;
    options.tenantForm.retentionDays = data.retentionDays;
    options.tenantForm.maxUploadMb = data.maxUploadMb;
    if (!options.whatsappForm.instanceName) {
      options.whatsappForm.instanceName = data.whatsappInstance ?? "";
    }
  }

  async function loadUsers() {
    options.users.value = await options.apiFetch<TenantUser[]>("/users");
  }

  async function loadInitialData() {
    options.loading.value = true;
    options.clearMessages();
    options.clearFieldErrors(options.tenantFieldErrors);
    options.clearFieldErrors(options.userFieldErrors);
    try {
      const requests: Array<Promise<unknown>> = [
        loadTenant(),
        loadUsers(),
        options.refreshWhatsAppStatus({ silent: true }),
        options.loadFailuresDashboard({ silent: true }),
        options.loadHttpEndpointMetrics({ silent: true })
      ];

      if (options.canManageTenant.value) {
        requests.push(options.fetchQrCode({ force: false, silent: true }));
      }

      await Promise.all(requests);
    } catch (error) {
      options.setError(options.extractError(error));
    } finally {
      options.loading.value = false;
    }
  }

  async function saveTenant() {
    if (!options.canManageTenant.value) {
      options.setError("Perfil sem permissao para alterar configuracoes do tenant.");
      return;
    }

    options.savingTenant.value = true;
    options.clearMessages();
    options.clearFieldErrors(options.tenantFieldErrors);
    try {
      const fallbackMaxChannels = options.tenant.value?.maxChannels ?? 1;
      const fallbackMaxUsers = options.tenant.value?.maxUsers ?? 3;
      const fallbackRetentionDays = options.tenant.value?.retentionDays ?? 15;
      const fallbackMaxUploadMb = options.tenant.value?.maxUploadMb ?? 500;

      const nextMaxChannels = Number.isFinite(options.tenantForm.maxChannels)
        ? Math.trunc(options.tenantForm.maxChannels)
        : fallbackMaxChannels;
      const nextMaxUsers = Number.isFinite(options.tenantForm.maxUsers)
        ? Math.trunc(options.tenantForm.maxUsers)
        : fallbackMaxUsers;
      const nextRetentionDays = Number.isFinite(options.tenantForm.retentionDays)
        ? Math.trunc(options.tenantForm.retentionDays)
        : fallbackRetentionDays;
      const nextMaxUploadMb = Number.isFinite(options.tenantForm.maxUploadMb)
        ? Math.trunc(options.tenantForm.maxUploadMb)
        : fallbackMaxUploadMb;

      const data = await options.apiFetch<TenantSettings>("/tenant", {
        method: "PATCH",
        body: {
          name: options.tenantForm.name,
          whatsappInstance: options.tenantForm.whatsappInstance || undefined,
          evolutionApiKey: options.tenantForm.evolutionApiKey,
          maxChannels: nextMaxChannels,
          maxUsers: nextMaxUsers,
          retentionDays: nextRetentionDays,
          maxUploadMb: nextMaxUploadMb
        }
      });
      options.tenant.value = data;
      options.whatsappForm.instanceName = data.whatsappInstance ?? "";
      options.infoMessage.value = "Configuracoes do tenant salvas.";
    } catch (error) {
      const hasFieldErrors = options.applyFieldErrors(options.tenantFieldErrors, error);
      options.setError(hasFieldErrors ? "Revise os campos destacados do cliente atual." : options.extractError(error));
    } finally {
      options.savingTenant.value = false;
    }
  }

  async function createUser() {
    if (!options.canManageTenant.value) {
      options.setError("Perfil sem permissao para criar usuarios.");
      return;
    }

    options.creatingUser.value = true;
    options.clearMessages();
    options.clearFieldErrors(options.userFieldErrors);
    try {
      await options.apiFetch<TenantUser>("/users", {
        method: "POST",
        body: {
          name: options.userForm.name,
          email: options.userForm.email,
          password: options.userForm.password,
          role: options.userForm.role
        }
      });

      options.userForm.name = "";
      options.userForm.email = "";
      options.userForm.password = "";
      options.userForm.role = "AGENT";

      await loadUsers();
      options.infoMessage.value = "Usuario criado com sucesso.";
    } catch (error) {
      const hasFieldErrors = options.applyFieldErrors(options.userFieldErrors, error);
      options.setError(hasFieldErrors ? "Revise os campos destacados do usuario." : options.extractError(error));
    } finally {
      options.creatingUser.value = false;
    }
  }

  return {
    loadTenant,
    loadUsers,
    loadInitialData,
    saveTenant,
    createUser
  };
}
