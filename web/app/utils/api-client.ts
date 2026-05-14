export const AUTH_TOKEN_COOKIE = "ldv_access_token";

export function getApiErrorMessage(error, fallbackMessage) {
  const baseMessage = error?.data?.error?.message || error?.message || fallbackMessage;
  const detailCause = String(error?.data?.error?.details?.cause || "").trim();

  if (!detailCause) {
    return baseMessage;
  }

  return `${baseMessage} (${detailCause})`;
}

export function getApiBase(runtimeConfig) {
  if (import.meta.server) {
    return runtimeConfig.apiInternalBase || runtimeConfig.public.apiBase;
  }

  return runtimeConfig.public.apiBase;
}

export function getWebSocketBase(runtimeConfig) {
  const configuredBase = String(runtimeConfig.public.apiWsBase || "").trim();
  const baseURL = configuredBase || getApiBase(runtimeConfig);
  const url = new URL(baseURL);

  if (url.protocol === "http:") {
    url.protocol = "ws:";
  } else if (url.protocol === "https:") {
    url.protocol = "wss:";
  }

  return url.toString();
}

// Limiar em ms a partir do qual uma requisicao aciona o loading global.
// Requests mais curtos nao ativam o overlay para evitar flicker.
const LOADING_THRESHOLD_MS = 200;

// Hooks de loading global. Sao injetados pelo plugin client-only
// `web/app/plugins/loading-bridge.client.ts` que liga o store
// `core/loading` (do layer core) a este api-client. Mantemos esse contrato
// de hooks para evitar import direto do store aqui (dependencia circular
// com stores que usam o api-client durante o setup do pinia).
let loadingHooks: { push: () => void; pop: () => void } | null = null;

export function setApiLoadingHooks(hooks: { push: () => void; pop: () => void } | null) {
  loadingHooks = hooks;
}

export function createApiRequest(runtimeConfig, getAccessToken = null) {
  return function apiRequest(path, options = {}) {
    const headers = {
      ...(options.headers || {})
    };
    const normalizedMethod = String(options.method || "GET").toUpperCase();
    const accessToken =
      typeof getAccessToken === "function"
        ? getAccessToken()
        : getAccessToken;

    if (accessToken) {
      headers.Authorization = `Bearer ${accessToken}`;
    }

    let processedOptions = { ...options };

    if (
      ["POST", "PUT", "PATCH", "DELETE"].includes(normalizedMethod) &&
      options.body &&
      typeof options.body === "object" &&
      !(options.body instanceof FormData) &&
      !(options.body instanceof Blob) &&
      !(options.body instanceof ArrayBuffer)
    ) {
      processedOptions.body = JSON.stringify(options.body);
      if (!headers["Content-Type"]) {
        headers["Content-Type"] = "application/json";
      }
    }

    const fetchPromise = $fetch(path, {
      baseURL: getApiBase(runtimeConfig),
      ...processedOptions,
      headers
    });

    // Fase 9A — feedback visual: se a requisicao passar de LOADING_THRESHOLD_MS,
    // ativa o loading global (barra fina no topo). Curtas nao acionam para
    // evitar flicker em chamadas rapidas. Hooks injetados pelo plugin
    // loading-bridge.client.ts (so existe no client; SSR ignora).
    if (loadingHooks && options.skipLoadingIndicator !== true) {
      let pushed = false;
      const timer = setTimeout(() => {
        loadingHooks?.push();
        pushed = true;
      }, LOADING_THRESHOLD_MS);

      fetchPromise.finally(() => {
        clearTimeout(timer);
        if (pushed) {
          loadingHooks?.pop();
        }
      });
    }

    return fetchPromise;
  };
}
