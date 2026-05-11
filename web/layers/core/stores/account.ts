import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { AUTH_TOKEN_COOKIE, createApiRequest } from "~/utils/api-client";

export interface AccountSummary {
  id: string;
  name: string;
  slug: string;
  organizationId: string;
  planCode: string;
  modules: string[];
}

export interface RoleSummary {
  id: string;
  code: string;
  label: string;
  isLocked: boolean;
}

export interface AccountContext {
  account: AccountSummary;
  user: { id: string; name: string; email: string };
  roles: RoleSummary[];
  permissions: string[];
  org: { id: string; name: string; slug: string } | null;
}

const ACTIVE_ACCOUNT_COOKIE = "ldv_active_account_id";

export const useCoreAccountStore = defineStore("core/account", () => {
  const runtimeConfig = useRuntimeConfig();
  const tokenCookie = useCookie(AUTH_TOKEN_COOKIE);
  const activeAccountCookie = useCookie(ACTIVE_ACCOUNT_COOKIE);

  const accounts = ref<AccountSummary[]>([]);
  const activeAccountId = ref<string>("");
  const context = ref<AccountContext | null>(null);
  const loading = ref(false);
  const error = ref<string>("");

  const activeAccount = computed(() =>
    accounts.value.find((a) => a.id === activeAccountId.value) ?? null
  );

  const permissions = computed(() => context.value?.permissions ?? []);
  const enabledModules = computed(() => activeAccount.value?.modules ?? []);

  const api = createApiRequest(runtimeConfig, () => tokenCookie.value ?? "");

  async function fetchAccounts() {
    loading.value = true;
    error.value = "";
    try {
      const data = await api("/v2/me/accounts") as any;
      accounts.value = data.accounts ?? [];

      const savedId = activeAccountCookie.value;
      const found = accounts.value.find((a) => a.id === savedId);
      activeAccountId.value = found?.id ?? data.defaultAccountId ?? accounts.value[0]?.id ?? "";

      if (activeAccountId.value) {
        await fetchContext(activeAccountId.value);
      }
    } catch (e: any) {
      error.value = e?.data?.error?.message ?? e?.message ?? "Erro ao carregar accounts.";
    } finally {
      loading.value = false;
    }
  }

  async function fetchContext(accountId: string) {
    try {
      const data = await api(`/v2/me/context?accountId=${accountId}`) as any;
      context.value = data.context ?? null;
    } catch {
      context.value = null;
    }
  }

  async function switchAccount(accountId: string) {
    activeAccountId.value = accountId;
    activeAccountCookie.value = accountId;
    await fetchContext(accountId);
  }

  function hasPermission(key: string): boolean {
    return permissions.value.includes(key);
  }

  function reset() {
    accounts.value = [];
    activeAccountId.value = "";
    context.value = null;
    error.value = "";
  }

  return {
    accounts,
    activeAccountId,
    activeAccount,
    context,
    loading,
    error,
    permissions,
    enabledModules,
    fetchAccounts,
    switchAccount,
    hasPermission,
    reset
  };
});
