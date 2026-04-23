export const AUTH_TOKEN_COOKIE = "ldv_access_token";

export function getApiErrorMessage(error, fallbackMessage) {
  return error?.data?.error?.message || error?.message || fallbackMessage;
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

  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  return url.toString();
}

export function createApiRequest(runtimeConfig, getAccessToken = null) {
  return function apiRequest(path, options = {}) {
    const headers = {
      ...(options.headers || {})
    };
    const accessToken =
      typeof getAccessToken === "function"
        ? getAccessToken()
        : getAccessToken;

    if (accessToken) {
      headers.Authorization = `Bearer ${accessToken}`;
    }

    return $fetch(path, {
      baseURL: getApiBase(runtimeConfig),
      ...options,
      headers
    });
  };
}
