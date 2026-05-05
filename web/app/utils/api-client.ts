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

    return $fetch(path, {
      baseURL: getApiBase(runtimeConfig),
      ...processedOptions,
      headers
    });
  };
}
