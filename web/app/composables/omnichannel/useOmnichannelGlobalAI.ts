import { computed, ref, watch } from "vue";
import {
  fetchAutomationProfile,
  saveAutomationProfile,
  type AutomationProfile,
} from "~/domain/omnichannel/automation-api";
import { useAuthStore } from "~/stores/auth";
import { useUiStore } from "~/stores/ui";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";
import { useCoreAccountStore } from "../../../layers/core/stores/account";

export function useOmnichannelGlobalAI() {
  const auth = useAuthStore();
  const ui = useUiStore();
  const accountStore = useCoreAccountStore();
  const runtimeConfig = useRuntimeConfig();
  const api = createApiRequest(runtimeConfig, () => auth.accessToken);

  const profile = ref<AutomationProfile | null>(null);
  const loading = ref(false);
  const saving = ref(false);
  const error = ref("");

  const canManageAutomation = computed(
    () =>
      auth.role === "platform_admin" ||
      auth.effectivePermissionKeys.includes("omnichannel.settings.manage"),
  );
  const canConfigure = computed(
    () =>
      auth.role === "platform_admin" ||
      [
        "omnichannel.instances.manage",
        "omnichannel.settings.manage",
        "omnichannel.agents.manage",
      ].some((permission) => auth.effectivePermissionKeys.includes(permission)),
  );
  const enabled = computed(() => Boolean(profile.value?.enabled));
  const ready = computed(
    () =>
      canManageAutomation.value &&
      Boolean(
        profile.value?.configured &&
          profile.value.whatsappInstance?.id &&
          profile.value.aiAgent?.id,
      ),
  );

  async function load(): Promise<void> {
    const clientId = String(accountStore.activeAccountId || "").trim();
    profile.value = null;
    error.value = "";
    if (!canManageAutomation.value || !clientId || clientId === "0") return;

    loading.value = true;
    try {
      profile.value = await fetchAutomationProfile(api, clientId);
    } catch (cause) {
      error.value = getApiErrorMessage(cause, "Não foi possível consultar a IA geral.");
    } finally {
      loading.value = false;
    }
  }

  async function toggle(nextEnabled: boolean): Promise<void> {
    const current = profile.value;
    const clientId = String(accountStore.activeAccountId || "").trim();
    if (!current || !clientId || !ready.value || saving.value) return;

    saving.value = true;
    try {
      profile.value = await saveAutomationProfile(api, clientId, {
        whatsappInstanceId: current.whatsappInstance?.id ?? "",
        aiAgentId: current.aiAgent?.id ?? "",
        enabled: nextEnabled,
        closePolicy: {
          autoCloseEnabled: current.closePolicy.autoCloseEnabled,
          minimumConfidence: current.closePolicy.minimumConfidence,
          requireAllRequiredFields: current.closePolicy.requireAllRequiredFields,
          blockOnHumanRequest: current.closePolicy.blockOnHumanRequest,
          blockSensitiveTopics: current.closePolicy.blockSensitiveTopics,
        },
      });
      ui.success(nextEnabled ? "IA geral ativada." : "IA geral parada.");
    } catch (cause) {
      ui.error(getApiErrorMessage(cause, "Não foi possível atualizar a IA geral."));
    } finally {
      saving.value = false;
    }
  }

  watch(
    [() => accountStore.activeAccountId, canManageAutomation],
    () => void load(),
    { immediate: true },
  );

  return {
    profile,
    loading,
    saving,
    error,
    enabled,
    ready,
    canManageAutomation,
    canConfigure,
    load,
    toggle,
  };
}
