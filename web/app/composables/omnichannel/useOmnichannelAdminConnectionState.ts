import { computed, type Ref } from "vue";
import type { WhatsAppQrCodeResponse, WhatsAppStatusResponse } from "~/types";

export function useOmnichannelAdminConnectionState(options: {
  statusResult: Ref<WhatsAppStatusResponse | null>;
  qrResult: Ref<WhatsAppQrCodeResponse | null>;
}) {
  const connectionState = computed(() => {
    const state =
      (options.statusResult.value?.connectionState as { instance?: { state?: string } } | undefined)?.instance?.state ??
      (options.statusResult.value?.providerUnavailable ? "provider_unavailable" : "unknown");
    return String(state);
  });

  const connectionStateNormalized = computed(() => connectionState.value.trim().toLowerCase());

  const isConnected = computed(() => {
    const value = connectionStateNormalized.value;
    return value === "open" || value === "connected";
  });

  const connectionBadgeColor = computed(() => {
    if (isConnected.value) {
      return "success";
    }
    if (connectionStateNormalized.value === "connecting") {
      return "warning";
    }
    if (
      connectionStateNormalized.value === "refused" ||
      connectionStateNormalized.value === "provider_unavailable" ||
      connectionStateNormalized.value === "close" ||
      connectionStateNormalized.value === "closed" ||
      connectionStateNormalized.value === "disconnected" ||
      connectionStateNormalized.value === "logout"
    ) {
      return "error";
    }
    return "neutral";
  });

  const qrImageSrc = computed(() => options.qrResult.value?.qrCode ?? null);
  const hasQrCode = computed(() => Boolean(qrImageSrc.value));

  const connectionStateLabel = computed(() => {
    const value = connectionStateNormalized.value;
    if (value === "open" || value === "connected") {
      return "Conectado";
    }

    if (value === "connecting") {
      return hasQrCode.value ? "Desconectado (aguardando QR)" : "Desconectado (iniciando QR)";
    }

    if (value === "refused") {
      return "Pareamento recusado";
    }

    if (value === "provider_unavailable") {
      return "Evolution indisponivel";
    }

    if (value === "close" || value === "closed" || value === "disconnected" || value === "logout") {
      return "Desconectado";
    }

    return "Status desconhecido";
  });

  const connectionAlertColor = computed(() => {
    if (isConnected.value) {
      return "success";
    }
    if (connectionStateNormalized.value === "connecting" || hasQrCode.value) {
      return "warning";
    }
    return "error";
  });

  const connectionAlertTitle = computed(() => {
    if (isConnected.value) {
      return "WhatsApp conectado";
    }

    if (hasQrCode.value) {
      return "WhatsApp desconectado (QR pronto para leitura)";
    }

    if (connectionStateNormalized.value === "provider_unavailable") {
      return "Conexao com a Evolution indisponivel";
    }

    if (connectionStateNormalized.value === "refused") {
      return "WhatsApp recusou o ultimo pareamento";
    }

    if (connectionStateNormalized.value === "connecting") {
      return "WhatsApp desconectado (aguardando QR)";
    }

    return "WhatsApp desconectado";
  });

  const connectionAlertDescription = computed(() => {
    if (isConnected.value) {
      return "A inbox esta apta para receber e enviar mensagens em tempo real.";
    }

    if (hasQrCode.value) {
      return "Escaneie o QR Code abaixo no WhatsApp para reconectar esta instancia.";
    }

    if (connectionStateNormalized.value === "provider_unavailable") {
      return options.statusResult.value?.message?.trim() ||
        "Nao foi possivel consultar a Evolution agora. O restante da inbox continua disponivel.";
    }

    if (connectionStateNormalized.value === "refused") {
      return "O ultimo QR foi recusado ou expirou na Evolution. Clique em Conectar por QR para resetar a sessao e gerar um novo codigo.";
    }

    if (connectionStateNormalized.value === "connecting") {
      return "A instancia iniciou conexao, mas ainda nao publicou QR. Clique em Atualizar QR e aguarde.";
    }

    return "Nenhuma sessao ativa. Clique em Conectar por QR para gerar o codigo de pareamento.";
  });

  const qrUnavailableMessage = computed(() => {
    if (hasQrCode.value) {
      return "";
    }

    if (options.qrResult.value?.message && options.qrResult.value.message.trim().length > 0) {
      return options.qrResult.value.message.trim();
    }

    if (isConnected.value) {
      return "Instancia ja conectada. Desconecte a sessao atual para gerar um novo QR Code.";
    }

    if (connectionStateNormalized.value === "connecting") {
      return "Aguardando emissao do QR Code pela instancia...";
    }

    if (connectionStateNormalized.value === "refused") {
      return "O ultimo QR foi recusado ou expirou. Clique em Conectar por QR para gerar um novo codigo.";
    }

    return "QR ainda indisponivel. Clique em Conectar por QR.";
  });

  return {
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
  };
}
