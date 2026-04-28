import { onBeforeUnmount, onMounted, ref, watch } from "vue";

import { useAuthStore } from "~/stores/auth";
import { useAccessControlStore } from "~/stores/access-control";
import { useAppRuntimeStore } from "~/stores/app-runtime";
import { useMultiStoreStore } from "~/stores/multistore";
import { useUsersStore } from "~/stores/users";
import { createApiRequest, getWebSocketBase } from "~/utils/api-client";
import { refreshRuntimeStoreSettings } from "~/utils/runtime-remote";

function buildSocketURL(runtimeConfig, tenantId, accessToken) {
  const url = new URL("/v1/realtime/context", getWebSocketBase(runtimeConfig));
  url.searchParams.set("tenantId", String(tenantId || "").trim());
  url.searchParams.set("access_token", String(accessToken || "").trim());
  return url.toString();
}

export function useContextRealtime() {
  const runtimeConfig = useRuntimeConfig();
  const auth = useAuthStore();
	const accessControl = useAccessControlStore();
  const runtime = useAppRuntimeStore();
  const multiStore = useMultiStoreStore();
  const usersStore = useUsersStore();
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);

  const status = ref("idle");
  const lastEvent = ref(null);

  let socket = null;
  let reconnectTimer = null;
  let reconnectAttempt = 0;
  let stopWatcher = null;
  let currentConnectionKey = "";
  let intentionallyClosed = false;
  let refreshPromise = null;
  let refreshQueued = false;
  let settingsRefreshPromise = null;
  let settingsRefreshQueued = false;
  let queuedSettingsStoreId = "";

  async function refreshContextState() {
    if (refreshPromise) {
      refreshQueued = true;
      return refreshPromise;
    }

    refreshPromise = (async () => {
      try {
        await auth.fetchContext();

        const followUps = [];
        if (auth.role === "platform_admin" || auth.role === "owner" || auth.role === "director" || auth.role === "marketing") {
          followUps.push(multiStore.refreshOverview().catch(() => null));
        }

        if (auth.role === "platform_admin" || auth.role === "owner") {
          followUps.push(multiStore.refreshManagedStores().catch(() => null));
          followUps.push(usersStore.refreshUsers({ silent: true }).catch(() => null));
        }

        await Promise.allSettled(followUps);
      } finally {
        refreshPromise = null;
        if (refreshQueued) {
          refreshQueued = false;
          await refreshContextState();
        }
      }
    })();

    return refreshPromise;
  }

  async function refreshActiveStoreSettings(storeId = "") {
    const normalizedStoreId = String(storeId || auth.activeStoreId || runtime.state.activeStoreId || "").trim();

    if (!normalizedStoreId || !auth.isAuthenticated || !auth.accessToken) {
      return null;
    }

    if (settingsRefreshPromise) {
      settingsRefreshQueued = true;
      queuedSettingsStoreId = normalizedStoreId;
      return settingsRefreshPromise;
    }

    settingsRefreshPromise = refreshRuntimeStoreSettings(runtime, apiRequest, normalizedStoreId, auth.activeTenantId)
      .catch(() => null)
      .finally(async () => {
        settingsRefreshPromise = null;

        if (settingsRefreshQueued) {
          const nextStoreId = queuedSettingsStoreId;
          settingsRefreshQueued = false;
          queuedSettingsStoreId = "";
          await refreshActiveStoreSettings(nextStoreId);
        }
      });

    return settingsRefreshPromise;
  }

  function clearReconnectTimer() {
    if (reconnectTimer) {
      window.clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  }

  function disconnect() {
    intentionallyClosed = true;
    clearReconnectTimer();

    if (socket) {
      socket.close();
      socket = null;
    }

    currentConnectionKey = "";
    status.value = "idle";
  }

  function scheduleReconnect() {
    clearReconnectTimer();

    if (!auth.isAuthenticated || !auth.activeTenantId || !auth.accessToken) {
      return;
    }

    const delayMs = Math.min(10000, 1000 * Math.max(1, 2 ** reconnectAttempt));
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null;
      connect();
    }, delayMs);
  }

  function connect() {
    if (import.meta.server) {
      return;
    }

    const tenantId = String(auth.activeTenantId || auth.tenantContext?.[0]?.id || "").trim();
    const accessToken = String(auth.accessToken || "").trim();

    if (!auth.isAuthenticated || !tenantId || !accessToken) {
      disconnect();
      return;
    }

    const nextConnectionKey = `${tenantId}:${accessToken}`;
    if (socket && currentConnectionKey === nextConnectionKey && socket.readyState <= WebSocket.OPEN) {
      return;
    }

    intentionallyClosed = false;
    clearReconnectTimer();

    if (socket) {
      socket.close();
      socket = null;
    }

    currentConnectionKey = nextConnectionKey;
    status.value = "connecting";

    const nextSocket = new WebSocket(buildSocketURL(runtimeConfig, tenantId, accessToken));
    socket = nextSocket;

    nextSocket.addEventListener("open", () => {
      reconnectAttempt = 0;
      status.value = "connected";
    });

    nextSocket.addEventListener("message", async (message) => {
      try {
        const payload = JSON.parse(String(message.data || "{}"));
        lastEvent.value = payload;

        if (payload?.type !== "context.updated") {
          return;
        }

        if (String(payload?.tenantId || "").trim() !== String(auth.activeTenantId || "").trim()) {
          return;
        }

        if (String(payload?.resource || "").trim() === "settings") {
          const activeStoreId = String(auth.activeStoreId || runtime.state.activeStoreId || "").trim();
          const payloadTenantId = String(payload?.resourceId || payload?.tenantId || "").trim();

          if (!payloadTenantId || payloadTenantId === String(auth.activeTenantId || "").trim()) {
            await refreshActiveStoreSettings(activeStoreId);
          }

          return;
        }

        await refreshContextState();

				if (["access", "user"].includes(String(payload?.resource || "").trim())) {
					await accessControl.refreshRealtimeState();
				}
      } catch {
        return;
      }
    });

    nextSocket.addEventListener("close", () => {
      if (socket === nextSocket) {
        socket = null;
      }

      if (intentionallyClosed) {
        status.value = "idle";
        return;
      }

      reconnectAttempt += 1;
      status.value = "reconnecting";
      scheduleReconnect();
    });

    nextSocket.addEventListener("error", () => {
      status.value = "error";
    });
  }

  onMounted(() => {
    stopWatcher = watch(
      [
        () => auth.isAuthenticated,
        () => auth.activeTenantId,
        () => auth.accessToken
      ],
      () => {
        connect();
      },
      {
        immediate: true
      }
    );
  });

  onBeforeUnmount(() => {
    if (typeof stopWatcher === "function") {
      stopWatcher();
      stopWatcher = null;
    }

    disconnect();
  });

  return {
    status,
    lastEvent
  };
}
