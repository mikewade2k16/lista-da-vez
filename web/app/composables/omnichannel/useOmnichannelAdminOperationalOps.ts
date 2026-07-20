import type { ComputedRef, Ref } from "vue";
import type {
  HttpMetricsSortBy,
  HttpMetricsSortOrder,
  TenantFailuresDashboardResponse,
  TenantHttpEndpointMetricsResponse,
  WhatsAppBootstrapResponse,
  WhatsAppEndpointValidationResponse,
  WhatsAppQrCodeResponse,
  WhatsAppStatusResponse
} from "~/types";

export function useOmnichannelAdminOperationalOps(options: {
  canManageTenant: ComputedRef<boolean>;
  canViewOpsDashboard: ComputedRef<boolean>;
  bootstrapping: Ref<boolean>;
  connectingQr: Ref<boolean>;
  connectingPairing: Ref<boolean>;
  disconnectingWhatsApp: Ref<boolean>;
  loadingFailures: Ref<boolean>;
  loadingHttpEndpointMetrics: Ref<boolean>;
  validatingEndpoints: Ref<boolean>;
  bootstrapResult: Ref<WhatsAppBootstrapResponse | null>;
  qrResult: Ref<WhatsAppQrCodeResponse | null>;
  failuresDashboard: Ref<TenantFailuresDashboardResponse | null>;
  httpEndpointMetrics: Ref<TenantHttpEndpointMetricsResponse | null>;
  endpointValidation: Ref<WhatsAppEndpointValidationResponse | null>;
  infoMessage: Ref<string>;
  pairingCode: Ref<string | null>;
  failureWindowDays: Ref<number>;
  whatsappForm: {
    instanceId: string;
    instanceName: string;
    number: string;
  };
  hasQrCode: ComputedRef<boolean>;
  qrUnavailableMessage: ComputedRef<string>;
  clearMessages: (preserveInfo?: boolean) => void;
  setError: (message: string) => void;
  extractError: (error: unknown) => string;
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>;
  loadTenant: () => Promise<void>;
  refreshWhatsAppStatus: (options?: { silent?: boolean; force?: boolean }) => Promise<void>;
  fetchQrCode: (options?: { force?: boolean; silent?: boolean }) => Promise<void>;
  startQrPolling: () => void;
  extractPairingCode: (payload: unknown) => string | null;
}) {
  async function bootstrapWhatsApp() {
    if (!options.canManageTenant.value) {
      options.setError("Perfil sem permissao para configurar o canal WhatsApp.");
      return;
    }

    options.bootstrapping.value = true;
    options.clearMessages();
    try {
      const data = await options.apiFetch<WhatsAppBootstrapResponse>("/tenant/whatsapp/bootstrap", {
        method: "POST",
        body: {
          instanceId: options.whatsappForm.instanceId || undefined,
          instanceName: options.whatsappForm.instanceName || undefined
        }
      });

      options.bootstrapResult.value = data;
      await Promise.all([
        options.loadTenant(),
        options.refreshWhatsAppStatus({ silent: true }),
        options.fetchQrCode({ force: true, silent: true })
      ]);
      options.startQrPolling();
      options.infoMessage.value = "Instancia iniciada. Escaneie o QR com seu WhatsApp.";
    } catch (error) {
      options.setError(options.extractError(error));
    } finally {
      options.bootstrapping.value = false;
    }
  }

  async function connectWithQr() {
    if (!options.canManageTenant.value) {
      options.setError("Perfil sem permissao para conectar o WhatsApp.");
      return;
    }

    options.connectingQr.value = true;
    options.clearMessages();
    try {
      const data = await options.apiFetch<WhatsAppBootstrapResponse>("/tenant/whatsapp/connect", {
        method: "POST",
        body: {
          instanceId: options.whatsappForm.instanceId || undefined
        }
      });
      options.bootstrapResult.value = data;

      await Promise.all([
        options.refreshWhatsAppStatus({ silent: true }),
        options.fetchQrCode({ force: true, silent: true })
      ]);
      options.startQrPolling();
      options.infoMessage.value = options.hasQrCode.value
        ? "QR atualizado. Escaneie no app WhatsApp."
        : options.qrUnavailableMessage.value;
    } catch (error) {
      options.setError(options.extractError(error));
    } finally {
      options.connectingQr.value = false;
    }
  }

  async function disconnectWhatsAppSession() {
    if (!options.canManageTenant.value) {
      options.setError("Perfil sem permissao para desconectar o WhatsApp.");
      return;
    }

    options.disconnectingWhatsApp.value = true;
    options.clearMessages();
    try {
      await options.apiFetch("/tenant/whatsapp/logout", {
        method: "POST",
        body: {
          instanceId: options.whatsappForm.instanceId || undefined
        }
      });

      options.qrResult.value = null;
      options.pairingCode.value = null;
      await options.refreshWhatsAppStatus({ silent: true });
      options.infoMessage.value = "Sessao desconectada. Clique em Conectar por QR para gerar um novo codigo.";
    } catch (error) {
      options.setError(options.extractError(error));
    } finally {
      options.disconnectingWhatsApp.value = false;
    }
  }

  async function generatePairingCode() {
    if (!options.canManageTenant.value) {
      options.setError("Perfil sem permissao para gerar pairing code.");
      return;
    }

    if (!options.whatsappForm.number.trim()) {
      options.setError("Informe um numero para gerar pairing code.");
      return;
    }

    options.connectingPairing.value = true;
    options.clearMessages();
    try {
      const data = await options.apiFetch<WhatsAppBootstrapResponse>("/tenant/whatsapp/connect", {
        method: "POST",
        body: {
          instanceId: options.whatsappForm.instanceId || undefined,
          number: options.whatsappForm.number.trim()
        }
      });
      options.bootstrapResult.value = data;

      const extracted = options.extractPairingCode(data.connectResult);
      options.pairingCode.value = extracted;
      await options.refreshWhatsAppStatus({ silent: true });

      options.infoMessage.value = extracted
        ? "Pairing code gerado. Use no WhatsApp para parear."
        : "Conexao por codigo solicitada. Confira retorno da Evolution.";
    } catch (error) {
      options.setError(options.extractError(error));
    } finally {
      options.connectingPairing.value = false;
    }
  }

  async function loadFailuresDashboard(optionsArg: { days?: number; silent?: boolean } = {}) {
    options.loadingFailures.value = true;
    if (!optionsArg.silent) {
      options.clearMessages(true);
    }
    try {
      const days = optionsArg.days ?? options.failureWindowDays.value;
      const data = await options.apiFetch<TenantFailuresDashboardResponse>(`/tenant/metrics/failures?days=${days}`);
      options.failureWindowDays.value = days;
      options.failuresDashboard.value = data;
    } catch (error) {
      if (!optionsArg.silent) {
        options.setError(options.extractError(error));
      }
    } finally {
      options.loadingFailures.value = false;
    }
  }

  async function loadHttpEndpointMetrics(
    optionsArg: {
      silent?: boolean;
      limit?: number;
      sortBy?: HttpMetricsSortBy;
      order?: HttpMetricsSortOrder;
      routeContains?: string;
    } = {}
  ) {
    if (!options.canViewOpsDashboard.value) {
      return;
    }

    options.loadingHttpEndpointMetrics.value = true;
    if (!optionsArg.silent) {
      options.clearMessages(true);
    }

    try {
      const search = new URLSearchParams();
      search.set("limit", String(optionsArg.limit ?? 20));
      search.set("sortBy", optionsArg.sortBy ?? "p95");
      search.set("order", optionsArg.order ?? "desc");

      const routeContains = optionsArg.routeContains?.trim();
      if (routeContains) {
        search.set("routeContains", routeContains);
      }

      options.httpEndpointMetrics.value = await options.apiFetch<TenantHttpEndpointMetricsResponse>(
        `/tenant/metrics/http-endpoints?${search.toString()}`
      );
    } catch (error) {
      if (!optionsArg.silent) {
        options.setError(options.extractError(error));
      }
    } finally {
      options.loadingHttpEndpointMetrics.value = false;
    }
  }

  async function validateEvolutionEndpoints(optionsArg: { silent?: boolean } = {}) {
    if (!options.canViewOpsDashboard.value) {
      return;
    }

    options.validatingEndpoints.value = true;
    if (!optionsArg.silent) {
      options.clearMessages(true);
    }

    try {
      options.endpointValidation.value = await options.apiFetch<WhatsAppEndpointValidationResponse>(
        "/tenant/whatsapp/validate-endpoints",
        {
          method: "POST",
          body: {
            instanceId: options.whatsappForm.instanceId || undefined,
            instanceName: options.whatsappForm.instanceName.trim() || undefined
          },
          timeout: 120_000
        }
      );
      if (!optionsArg.silent) {
        options.infoMessage.value = "Validacao de endpoints da Evolution atualizada.";
      }
    } catch (error) {
      options.endpointValidation.value = null;
      if (!optionsArg.silent) {
        options.setError(options.extractError(error));
      }
    } finally {
      options.validatingEndpoints.value = false;
    }
  }

  return {
    bootstrapWhatsApp,
    connectWithQr,
    disconnectWhatsAppSession,
    generatePairingCode,
    loadFailuresDashboard,
    loadHttpEndpointMetrics,
    validateEvolutionEndpoints
  };
}
