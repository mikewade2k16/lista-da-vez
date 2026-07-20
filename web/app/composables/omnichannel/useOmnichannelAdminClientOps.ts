import { computed, ref, type ComputedRef, type Ref } from "vue";
import type { AuthUser, ClientRecord, TenantSettings, TenantUser, UserRole } from "~/types";
import {
  createClientFormState,
  createUserFormState
} from "~/composables/omnichannel/useOmnichannelAdminShared";

type ClientFormState = ReturnType<typeof createClientFormState>;
type UserFormState = ReturnType<typeof createUserFormState>;
type CreatedClientAccess = {
  clientName: string;
  adminName: string;
  adminEmail: string;
  adminPassword: string;
};

export function useOmnichannelAdminClientOps(options: {
  user: Ref<AuthUser | null>;
  canManageTenant: ComputedRef<boolean>;
  tenant: Ref<TenantSettings | null>;
  users: Ref<TenantUser[]>;
  infoMessage: Ref<string>;
  clearMessages: (preserveInfo?: boolean) => void;
  setError: (message: string) => void;
  extractError: (error: unknown) => string;
  clearFieldErrors: (target: Record<string, string>) => void;
  applyFieldErrors: (target: Record<string, string>, error: unknown) => boolean;
  clientFieldErrors: Record<string, string>;
  userFieldErrors: Record<string, string>;
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>;
  clientForm: ClientFormState;
  userForm: UserFormState;
}) {
  const clients = ref<ClientRecord[]>([]);
  const loadingClients = ref(false);
  const savingClient = ref(false);
  const deletingClientId = ref<string | null>(null);
  const editingClientId = ref<string | null>(null);
  const selectedClientId = ref<string | null>(null);
  const createdClientAccess = ref<CreatedClientAccess | null>(null);

  const savingUser = ref(false);
  const deletingUserId = ref<string | null>(null);
  const editingUserId = ref<string | null>(null);

  const activeClientId = computed(
    () => selectedClientId.value || options.tenant.value?.id || options.user.value?.tenantId || null
  );

  const selectedClient = computed(() => {
    if (!activeClientId.value) {
      return null;
    }

    const foundClient = clients.value.find((entry) => entry.id === activeClientId.value);
    if (foundClient) {
      return foundClient;
    }

    const currentTenant = options.tenant.value;
    if (currentTenant && currentTenant.id === activeClientId.value) {
      return currentTenant;
    }

    return null;
  });

  function resetClientForm() {
    Object.assign(options.clientForm, createClientFormState());
    editingClientId.value = null;
    options.clearFieldErrors(options.clientFieldErrors);
  }

  function dismissCreatedClientAccess() {
    createdClientAccess.value = null;
  }

  function startEditClient(client: ClientRecord) {
    editingClientId.value = client.id;
    options.clearFieldErrors(options.clientFieldErrors);
    options.clientForm.slug = client.slug;
    options.clientForm.name = client.name;
    options.clientForm.evolutionApiKey = client.evolutionApiKey ?? "";
    options.clientForm.maxChannels = client.maxChannels;
    options.clientForm.maxUsers = client.maxUsers;
    options.clientForm.retentionDays = client.retentionDays;
    options.clientForm.maxUploadMb = client.maxUploadMb;
    options.clientForm.adminName = "";
    options.clientForm.adminEmail = "";
    options.clientForm.adminPassword = "";
  }

  function resetUserForm() {
    Object.assign(options.userForm, createUserFormState());
    editingUserId.value = null;
    options.clearFieldErrors(options.userFieldErrors);
  }

  function startEditUser(tenantUser: TenantUser) {
    editingUserId.value = tenantUser.id;
    options.clearFieldErrors(options.userFieldErrors);
    options.userForm.name = tenantUser.name;
    options.userForm.email = tenantUser.email;
    options.userForm.password = "";
    options.userForm.role = tenantUser.role;
  }

  async function loadClientUsers(nextClientId?: string | null) {
    const clientId = nextClientId || activeClientId.value;
    if (!clientId) {
      options.users.value = [];
      return;
    }

    selectedClientId.value = clientId;
    options.users.value = await options.apiFetch<TenantUser[]>(`/clients/${clientId}/users`);
  }

  async function loadClients() {
    if (!options.canManageTenant.value) {
      return;
    }

    loadingClients.value = true;
    try {
      clients.value = await options.apiFetch<ClientRecord[]>("/clients");

      if (!selectedClientId.value) {
        const currentTenantId = options.tenant.value?.id || options.user.value?.tenantId || null;
        const defaultClient =
          clients.value.find((entry) => entry.id === currentTenantId) ||
          clients.value[0] ||
          null;
        selectedClientId.value = defaultClient?.id ?? currentTenantId;
      }

      await loadClientUsers(selectedClientId.value);
    } catch (error) {
      // 403 = not a platform admin — silently skip client list (client admin sees own tenant only)
      const statusCode = (error as { statusCode?: number; data?: { statusCode?: number } })?.statusCode
        ?? (error as { data?: { statusCode?: number } })?.data?.statusCode;
      if (statusCode !== 403) {
        options.setError(options.extractError(error));
      }
      clients.value = [];
    } finally {
      loadingClients.value = false;
    }
  }

  async function saveClient() {
    if (!options.canManageTenant.value) {
      options.setError("Perfil sem permissao para gerenciar clientes.");
      return;
    }

    savingClient.value = true;
    options.clearMessages();
    options.clearFieldErrors(options.clientFieldErrors);

    try {
      const isEditing = Boolean(editingClientId.value);
      const initialAdminAccess = !isEditing
        ? {
            adminName: options.clientForm.adminName.trim(),
            adminEmail: options.clientForm.adminEmail.trim(),
            adminPassword: options.clientForm.adminPassword.trim()
          }
        : null;
      const payload: Record<string, unknown> = {
        slug: options.clientForm.slug.trim() || undefined,
        name: options.clientForm.name,
        evolutionApiKey: options.clientForm.evolutionApiKey || undefined,
        maxChannels: options.clientForm.maxChannels,
        maxUsers: options.clientForm.maxUsers,
        retentionDays: options.clientForm.retentionDays,
        maxUploadMb: options.clientForm.maxUploadMb
      };

      if (!isEditing) {
        payload.adminName = options.clientForm.adminName;
        payload.adminEmail = options.clientForm.adminEmail;
        payload.adminPassword = options.clientForm.adminPassword;
      }

      const savedClient = isEditing
        ? await options.apiFetch<ClientRecord>(`/clients/${editingClientId.value}`, {
            method: "PATCH",
            body: payload
          })
        : await options.apiFetch<ClientRecord>("/clients", {
            method: "POST",
            body: payload
          });

      await loadClients();
      selectedClientId.value = savedClient.id;
      await loadClientUsers(savedClient.id);
      createdClientAccess.value = !isEditing && initialAdminAccess
        ? {
            clientName: savedClient.name,
            adminName: initialAdminAccess.adminName,
            adminEmail: initialAdminAccess.adminEmail,
            adminPassword: initialAdminAccess.adminPassword
          }
        : null;
      resetClientForm();
      options.infoMessage.value = isEditing
        ? "Cliente atualizado com sucesso."
        : "Cliente criado com sucesso.";
    } catch (error) {
      const hasFieldErrors = options.applyFieldErrors(options.clientFieldErrors, error);
      options.setError(hasFieldErrors ? "Revise os campos destacados do cliente." : options.extractError(error));
    } finally {
      savingClient.value = false;
    }
  }

  async function deleteClient(clientId: string) {
    if (!options.canManageTenant.value) {
      options.setError("Perfil sem permissao para excluir clientes.");
      return;
    }

    deletingClientId.value = clientId;
    options.clearMessages();
    options.clearFieldErrors(options.clientFieldErrors);

    try {
      await options.apiFetch(`/clients/${clientId}`, {
        method: "DELETE"
      });

      if (editingClientId.value === clientId) {
        resetClientForm();
      }

      if (selectedClientId.value === clientId) {
        selectedClientId.value = options.tenant.value?.id || options.user.value?.tenantId || null;
      }

      await loadClients();
      options.infoMessage.value = "Cliente removido com sucesso.";
    } catch (error) {
      options.setError(options.extractError(error));
    } finally {
      deletingClientId.value = null;
    }
  }

  async function selectClient(clientId: string) {
    if (clientId === selectedClientId.value) {
      return;
    }

    resetUserForm();
    await loadClientUsers(clientId);
  }

  async function saveUser() {
    if (!options.canManageTenant.value) {
      options.setError("Perfil sem permissao para gerenciar usuarios.");
      return;
    }

    const clientId = activeClientId.value;
    if (!clientId) {
      options.setError("Selecione um cliente antes de salvar usuarios.");
      return;
    }

    const trimmedPassword = options.userForm.password.trim();
    const isEditing = Boolean(editingUserId.value);
    if (!isEditing && !trimmedPassword) {
      options.setError("Senha inicial obrigatoria para criar usuario.");
      return;
    }

    savingUser.value = true;
    options.clearMessages();
    options.clearFieldErrors(options.userFieldErrors);

    try {
      const payload: Record<string, unknown> = {
        name: options.userForm.name,
        email: options.userForm.email,
        role: options.userForm.role as UserRole
      };

      if (trimmedPassword) {
        payload.password = trimmedPassword;
      }

      await options.apiFetch<TenantUser>(
        isEditing
          ? `/clients/${clientId}/users/${editingUserId.value}`
          : `/clients/${clientId}/users`,
        {
          method: isEditing ? "PATCH" : "POST",
          body: payload
        }
      );

      await loadClientUsers(clientId);
      resetUserForm();
      options.infoMessage.value = isEditing
        ? "Usuario atualizado com sucesso."
        : "Usuario criado com sucesso.";
    } catch (error) {
      const hasFieldErrors = options.applyFieldErrors(options.userFieldErrors, error);
      options.setError(hasFieldErrors ? "Revise os campos destacados do usuario." : options.extractError(error));
    } finally {
      savingUser.value = false;
    }
  }

  async function deleteUser(userId: string) {
    if (!options.canManageTenant.value) {
      options.setError("Perfil sem permissao para excluir usuarios.");
      return;
    }

    const clientId = activeClientId.value;
    if (!clientId) {
      options.setError("Selecione um cliente antes de excluir usuarios.");
      return;
    }

    deletingUserId.value = userId;
    options.clearMessages();
    options.clearFieldErrors(options.userFieldErrors);

    try {
      await options.apiFetch(`/clients/${clientId}/users/${userId}`, {
        method: "DELETE"
      });

      await loadClientUsers(clientId);
      if (editingUserId.value === userId) {
        resetUserForm();
      }
      options.infoMessage.value = "Usuario removido com sucesso.";
    } catch (error) {
      options.setError(options.extractError(error));
    } finally {
      deletingUserId.value = null;
    }
  }

  return {
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
    loadClientUsers,
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
  };
}
