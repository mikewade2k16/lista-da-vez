import { computed, ref } from "vue";
import type {
  WhatsAppBootstrapResponse,
  WhatsAppConversationHistoryClearResponse,
  WhatsAppInstanceManagementResponse,
  WhatsAppInstanceRecord,
  WhatsAppQrCodeResponse,
  WhatsAppStatusResponse
} from "~/types";
import { extractAdminError } from "~/composables/omnichannel/useOmnichannelAdminShared";
import { useOmnichannelAdminConnectionState } from "~/composables/omnichannel/useOmnichannelAdminConnectionState";
import { useOmnichannelAdminQrPolling } from "~/composables/omnichannel/useOmnichannelAdminQrPolling";

export const NEW_WHATSAPP_INSTANCE_VALUE = "__new__";

function normalizeText(value: unknown) {
  return String(value ?? "").trim();
}

function buildProviderUnavailableState() {
  return {
    instance: {
      state: "provider_unavailable"
    }
  } satisfies Record<string, unknown>;
}

export function useOmnichannelWhatsAppSession() {
  const { legacyRole } = useAdminSession();
  const { apiFetch } = useApi();

  const loadingInstances = ref(false);
  const refreshingStatus = ref(false);
  const fetchingQr = ref(false);
  const generatingQr = ref(false);
  const disconnecting = ref(false);
  const clearingHistory = ref(false);
  const savingDisplayName = ref(false);

  const instances = ref<WhatsAppInstanceRecord[]>([]);
  const statusResult = ref<WhatsAppStatusResponse | null>(null);
  const qrResult = ref<WhatsAppQrCodeResponse | null>(null);
  const infoMessage = ref("");
  const errorMessage = ref("");
  const selectedInstanceKey = ref(NEW_WHATSAPP_INSTANCE_VALUE);
  const displayName = ref("");
  const lastSavedDisplayName = ref("");

  let active = false;

  const canManageChannel = computed(() => legacyRole.value === "ADMIN");
  const selectedInstance = computed(() =>
    instances.value.find((entry) => entry.id === selectedInstanceKey.value) ?? null
  );
  const hasExistingInstances = computed(() => instances.value.length > 0);
  const instanceItems = computed(() => {
    const options = instances.value.map((entry) => ({
      label: entry.displayName?.trim() || entry.phoneNumber?.trim() || entry.instanceName,
      value: entry.id
    }));

    return [
      ...options,
      { label: "Nova conexao WhatsApp", value: NEW_WHATSAPP_INSTANCE_VALUE }
    ];
  });
  const internalInstanceLabel = computed(() =>
    selectedInstance.value?.instanceName?.trim() || "Sera gerado automaticamente ao criar a conexao."
  );
  const isCreatingNewInstance = computed(() => !selectedInstance.value);

  const {
    connectionState,
    connectionStateNormalized,
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

  const { startQrPolling, stopQrPolling } = useOmnichannelAdminQrPolling({
    isConnected,
    hasQrCode,
    refreshWhatsAppStatus,
    fetchQrCode
  });

  function clearMessages(preserveInfo = false) {
    errorMessage.value = "";
    if (!preserveInfo) {
      infoMessage.value = "";
    }
  }

  function syncSelection(preferredKey?: string | null) {
    const preferred = preferredKey
      ? instances.value.find((entry) => entry.id === preferredKey)
      : null;
    const current = selectedInstance.value;
    const fallback = instances.value.find((entry) => entry.isDefault) ?? instances.value[0] ?? null;
    const resolved = preferred ?? current ?? fallback;

    selectedInstanceKey.value = resolved?.id ?? NEW_WHATSAPP_INSTANCE_VALUE;
    displayName.value = resolved?.displayName?.trim() ?? "";
    lastSavedDisplayName.value = displayName.value;
  }

  async function loadInstances(options: { preferredKey?: string | null; silent?: boolean } = {}) {
    if (!canManageChannel.value) {
      instances.value = [];
      syncSelection();
      return;
    }

    loadingInstances.value = true;
    if (!options.silent) {
      clearMessages(true);
    }

    try {
      const response = await apiFetch<WhatsAppInstanceManagementResponse>("/tenant/whatsapp/instances");
      instances.value = response.instances;
      syncSelection(options.preferredKey);
    } catch (error) {
      if (!options.silent) {
        errorMessage.value = extractAdminError(error);
      }
    } finally {
      loadingInstances.value = false;
    }
  }

  async function refreshWhatsAppStatus(options: { silent?: boolean; force?: boolean } = {}) {
    void options.force;

    if (!selectedInstance.value) {
      statusResult.value = {
        configured: hasExistingInstances.value,
        message: hasExistingInstances.value
          ? "Selecione uma conexao existente ou gere um novo QR Code."
          : "Nenhuma conexao WhatsApp criada para este cliente."
      };
      return;
    }

    refreshingStatus.value = true;
    if (!options.silent) {
      clearMessages(true);
    }

    try {
      statusResult.value = await apiFetch<WhatsAppStatusResponse>(
        `/tenant/whatsapp/status?instanceId=${encodeURIComponent(selectedInstance.value.id)}`
      );
    } catch (error) {
      const message = extractAdminError(error);
      statusResult.value = {
        configured: true,
        instanceId: selectedInstance.value.id,
        instanceName: selectedInstance.value.instanceName,
        providerUnavailable: true,
        degraded: true,
        connectionState: buildProviderUnavailableState(),
        message: message || "Nao foi possivel consultar o status atual da conexao WhatsApp."
      };
      if (!options.silent) {
        errorMessage.value = message;
      }
    } finally {
      refreshingStatus.value = false;
    }
  }

  async function fetchQrCode(options: { force?: boolean; silent?: boolean } = {}) {
    if (!selectedInstance.value) {
      qrResult.value = {
        configured: false,
        message: "Gere um novo QR Code para iniciar a conexao."
      };
      return;
    }

    fetchingQr.value = true;
    if (!options.silent) {
      clearMessages(true);
    }

    try {
      const query = new URLSearchParams({
        instanceId: selectedInstance.value.id,
        force: options.force === true ? "true" : "false"
      });
      qrResult.value = await apiFetch<WhatsAppQrCodeResponse>(`/tenant/whatsapp/qrcode?${query.toString()}`);
    } catch (error) {
      const message = extractAdminError(error);
      qrResult.value = {
        configured: true,
        instanceId: selectedInstance.value.id,
        instanceName: selectedInstance.value.instanceName,
        qrCode: null,
        connectionState: buildProviderUnavailableState(),
        message: message || "QR Code ainda indisponivel."
      };
      if (!options.silent) {
        errorMessage.value = message;
      }
    } finally {
      fetchingQr.value = false;
    }
  }

  async function refreshSelectedState() {
    if (!active) {
      return;
    }

    if (!selectedInstance.value) {
      qrResult.value = {
        configured: false,
        message: "Gere um novo QR Code para criar a conexao inicial."
      };
      await refreshWhatsAppStatus({ silent: true });
      stopQrPolling();
      return;
    }

    await Promise.all([
      refreshWhatsAppStatus({ silent: true, force: true }),
      fetchQrCode({ force: false, silent: true })
    ]);

    if (isConnected.value) {
      stopQrPolling();
      return;
    }

    startQrPolling();
  }

  async function activate() {
    if (!canManageChannel.value) {
      return;
    }

    active = true;
    await loadInstances();
    await refreshSelectedState();
  }

  function deactivate() {
    active = false;
    stopQrPolling();
  }

  async function selectInstance(nextValue: string) {
    selectedInstanceKey.value = normalizeText(nextValue) || NEW_WHATSAPP_INSTANCE_VALUE;

    const nextInstance = instances.value.find((entry) => entry.id === selectedInstanceKey.value) ?? null;
    displayName.value = nextInstance?.displayName?.trim() ?? "";
    lastSavedDisplayName.value = displayName.value;
    qrResult.value = null;
    clearMessages(true);

    await refreshSelectedState();
  }

  async function persistDisplayName() {
    if (!selectedInstance.value) {
      return;
    }

    const normalized = normalizeText(displayName.value);
    if (normalized === lastSavedDisplayName.value) {
      return;
    }

    if (!normalized) {
      displayName.value = lastSavedDisplayName.value;
      return;
    }

    if (normalized.length > 0 && normalized.length < 2) {
      errorMessage.value = "Informe pelo menos 2 caracteres para o nome visual.";
      displayName.value = lastSavedDisplayName.value;
      return;
    }

    savingDisplayName.value = true;
    clearMessages(true);

    try {
      const updated = await apiFetch<WhatsAppInstanceRecord>(
        `/tenant/whatsapp/instances/${selectedInstance.value.id}`,
        {
          method: "PATCH",
          body: normalized ? { displayName: normalized } : {}
        }
      );

      const index = instances.value.findIndex((entry) => entry.id === updated.id);
      if (index >= 0) {
        instances.value[index] = updated;
      }
      syncSelection(updated.id);
      infoMessage.value = "Nome visual atualizado.";
    } catch (error) {
      errorMessage.value = extractAdminError(error);
      displayName.value = lastSavedDisplayName.value;
    } finally {
      savingDisplayName.value = false;
    }
  }

  // Anti-thrash: reconectar em rajada REINICIA a sessao Baileys e o WhatsApp a derruba com
  // 401 (loggedOut) — o QR se invalida antes de ser lido. lastConnectAt guarda o ultimo connect
  // real; cliques dentro da janela so re-buscam o QR vigente, sem reiniciar a sessao.
  let lastConnectAt = 0;
  const CONNECT_COOLDOWN_MS = 8_000;

  async function generateQrCode() {
    if (!canManageChannel.value) {
      errorMessage.value = "Perfil sem permissao para gerenciar a conexao WhatsApp.";
      return;
    }

    // Ja conectando: ignora o clique repetido (a sessao esta sendo montada).
    if (generatingQr.value) {
      return;
    }
    // Reconectou ha pouco: NAO reinicia a sessao (evita o 401). So atualiza a imagem do QR.
    if (Date.now() - lastConnectAt < CONNECT_COOLDOWN_MS) {
      await fetchQrCode({ force: true, silent: true });
      startQrPolling();
      return;
    }
    lastConnectAt = Date.now();

    generatingQr.value = true;
    clearMessages();

    try {
      const normalizedDisplayName = normalizeText(displayName.value);
      const response = await apiFetch<WhatsAppBootstrapResponse>("/tenant/whatsapp/bootstrap", {
        method: "POST",
        body: {
          instanceId: selectedInstance.value?.id ?? undefined,
          displayName: normalizedDisplayName || undefined
        }
      });

      // Bootstrap so GARANTE a instancia no banco. E o CONNECT que manda a Evolution iniciar a
      // sessao e emitir o QR — sem ele o /qrcode fica sempre vazio (o QR nunca e gerado).
      await apiFetch<WhatsAppBootstrapResponse>("/tenant/whatsapp/connect", {
        method: "POST",
        body: {
          instanceId: response.instanceId ?? selectedInstance.value?.id ?? undefined
        }
      });

      await loadInstances({
        preferredKey: response.instanceId ?? selectedInstance.value?.id ?? null,
        silent: true
      });
      await Promise.all([
        refreshWhatsAppStatus({ silent: true, force: true }),
        fetchQrCode({ force: true, silent: true })
      ]);

      if (isConnected.value) {
        stopQrPolling();
        infoMessage.value = "WhatsApp ja conectado. Desconecte a sessao atual para gerar um novo QR Code.";
        return;
      }

      startQrPolling();
      infoMessage.value = hasQrCode.value
        ? "QR Code atualizado. O codigo sera renovado automaticamente enquanto a janela estiver aberta."
        : qrUnavailableMessage.value;
    } catch (error) {
      errorMessage.value = extractAdminError(error);
    } finally {
      generatingQr.value = false;
    }
  }

  async function disconnectSession() {
    if (!selectedInstance.value) {
      errorMessage.value = "Selecione uma conexao existente para desconectar.";
      return;
    }

    disconnecting.value = true;
    clearMessages();

    try {
      await apiFetch("/tenant/whatsapp/logout", {
        method: "POST",
        body: {
          instanceId: selectedInstance.value.id
        }
      });

      qrResult.value = null;
      await refreshSelectedState();
      infoMessage.value = "Sessao desconectada. Gere um novo QR Code quando quiser reconectar.";
    } catch (error) {
      errorMessage.value = extractAdminError(error);
    } finally {
      disconnecting.value = false;
    }
  }

  async function clearConversationHistory() {
    if (!canManageChannel.value) {
      errorMessage.value = "Perfil sem permissao para limpar o historico do WhatsApp.";
      return null;
    }

    if (!selectedInstance.value) {
      errorMessage.value = "Selecione uma conexao existente para limpar o historico.";
      return null;
    }

    clearingHistory.value = true;
    clearMessages();

    try {
      const response = await apiFetch<WhatsAppConversationHistoryClearResponse>(
        "/tenant/whatsapp/conversations/clear",
        {
          method: "POST",
          body: {
            instanceId: selectedInstance.value.id
          }
        }
      );

      infoMessage.value = `${response.message}. ${response.deletedConversations} conversa(s) e ${response.deletedMessages} mensagem(ns) removida(s).`;
      return response;
    } catch (error) {
      errorMessage.value = extractAdminError(error);
      return null;
    } finally {
      clearingHistory.value = false;
    }
  }

  return {
    NEW_WHATSAPP_INSTANCE_VALUE,
    loadingInstances,
    refreshingStatus,
    fetchingQr,
    generatingQr,
    disconnecting,
    clearingHistory,
    savingDisplayName,
    instances,
    statusResult,
    qrResult,
    infoMessage,
    errorMessage,
    selectedInstanceKey,
    displayName,
    canManageChannel,
    selectedInstance,
    hasExistingInstances,
    instanceItems,
    internalInstanceLabel,
    isCreatingNewInstance,
    connectionState,
    connectionStateNormalized,
    isConnected,
    connectionBadgeColor,
    connectionStateLabel,
    qrImageSrc,
    hasQrCode,
    connectionAlertColor,
    connectionAlertTitle,
    connectionAlertDescription,
    qrUnavailableMessage,
    activate,
    deactivate,
    loadInstances,
    selectInstance,
    persistDisplayName,
    generateQrCode,
    disconnectSession,
    clearConversationHistory
  };
}
