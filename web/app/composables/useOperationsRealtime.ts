import { onBeforeUnmount, onMounted, ref, watch } from "vue";

import { useAuthStore } from "~/stores/auth";
import { useOperationsStore } from "~/stores/operations";
import { getWebSocketBase } from "~/utils/api-client";

function buildSocketURL(runtimeConfig, storeId, accessToken) {
  const url = new URL("/v1/realtime/operations", getWebSocketBase(runtimeConfig));
  url.searchParams.set("storeId", String(storeId || "").trim());
  url.searchParams.set("access_token", String(accessToken || "").trim());
  return url.toString();
}

function resolveSourceValue(source, fallback = "single") {
  if (typeof source === "function") {
    const value = source();
    return value == null ? fallback : value;
  }

  if (source && typeof source === "object" && "value" in source) {
    return source.value == null ? fallback : source.value;
  }

  return source == null ? fallback : source;
}

export function useOperationsRealtime(options = {}) {
  const runtimeConfig = useRuntimeConfig();
  const auth = useAuthStore();
  const operationsStore = useOperationsStore();

  const status = ref("idle");
  const lastEvent = ref(null);

  const sockets = new Map();
  const reconnectTimers = new Map();
  const reconnectAttempts = new Map();
  const silencedCloses = new Set();

  let stopWatcher = null;
  let snapshotRefreshPromise = null;
  let snapshotRefreshQueued = false;
  let overviewRefreshPromise = null;
  let overviewRefreshQueued = false;

  function desiredStoreIds() {
    if (!auth.isAuthenticated || !auth.accessToken) {
      return [];
    }

    const mode = String(resolveSourceValue(options.scopeMode, "single") || "single").trim();
    const ids = mode === "all"
      ? auth.accessibleStoreIds
      : [auth.activeStoreId];

    return [...new Set((Array.isArray(ids) ? ids : []).map((value) => String(value || "").trim()).filter(Boolean))];
  }

  function updateStatus() {
    if (!sockets.size) {
      status.value = "idle";
      return;
    }

    const socketEntries = [...sockets.values()].map((entry) => entry.socket);
    if (socketEntries.some((socket) => socket.readyState === WebSocket.OPEN)) {
      status.value = "connected";
      return;
    }

    if (socketEntries.some((socket) => socket.readyState === WebSocket.CONNECTING)) {
      status.value = "connecting";
      return;
    }

    if (reconnectTimers.size > 0) {
      status.value = "reconnecting";
      return;
    }

    status.value = "error";
  }

  async function refreshSnapshot(storeId) {
    const normalizedStoreId = String(storeId || "").trim();
    if (!normalizedStoreId) {
      return;
    }

    if (snapshotRefreshPromise) {
      snapshotRefreshQueued = true;
      return snapshotRefreshPromise;
    }

    snapshotRefreshPromise = operationsStore
      .refreshOperationSnapshot(normalizedStoreId)
      .catch(() => null)
      .finally(async () => {
        snapshotRefreshPromise = null;

        if (snapshotRefreshQueued) {
          snapshotRefreshQueued = false;
          await refreshSnapshot(normalizedStoreId);
        }
      });

    return snapshotRefreshPromise;
  }

  async function refreshOverview() {
    if (overviewRefreshPromise) {
      overviewRefreshQueued = true;
      return overviewRefreshPromise;
    }

    overviewRefreshPromise = operationsStore
      .refreshOverview()
      .catch(() => null)
      .finally(async () => {
        overviewRefreshPromise = null;

        if (overviewRefreshQueued) {
          overviewRefreshQueued = false;
          await refreshOverview();
        }
      });

    return overviewRefreshPromise;
  }

  function clearReconnectTimer(storeId) {
    const timer = reconnectTimers.get(storeId);
    if (timer) {
      window.clearTimeout(timer);
    }
    reconnectTimers.delete(storeId);
  }

  function disconnectStore(storeId) {
    clearReconnectTimer(storeId);

    const entry = sockets.get(storeId);
    if (!entry) {
      updateStatus();
      return;
    }

    silencedCloses.add(storeId);
    entry.socket.close();
    sockets.delete(storeId);
    reconnectAttempts.delete(storeId);
    updateStatus();
  }

  function disconnectAll() {
    for (const storeId of [...sockets.keys()]) {
      disconnectStore(storeId);
    }

    for (const storeId of [...reconnectTimers.keys()]) {
      clearReconnectTimer(storeId);
    }

    updateStatus();
  }

  function scheduleReconnect(storeId) {
    clearReconnectTimer(storeId);

    if (!desiredStoreIds().includes(storeId)) {
      updateStatus();
      return;
    }

    const attempt = reconnectAttempts.get(storeId) || 0;
    const delayMs = Math.min(10000, 1000 * Math.max(1, 2 ** attempt));
    const timer = window.setTimeout(() => {
      reconnectTimers.delete(storeId);
      ensureSocket(storeId);
    }, delayMs);

    reconnectTimers.set(storeId, timer);
    updateStatus();
  }

  function ensureSocket(storeId) {
    const normalizedStoreId = String(storeId || "").trim();
    const accessToken = String(auth.accessToken || "").trim();

    if (!normalizedStoreId || !accessToken || !desiredStoreIds().includes(normalizedStoreId)) {
      disconnectStore(normalizedStoreId);
      return;
    }

    const connectionKey = `${normalizedStoreId}:${accessToken}`;
    const currentEntry = sockets.get(normalizedStoreId);
    if (currentEntry && currentEntry.key === connectionKey && currentEntry.socket.readyState <= WebSocket.OPEN) {
      updateStatus();
      return;
    }

    if (currentEntry) {
      disconnectStore(normalizedStoreId);
    }

    const nextSocket = new WebSocket(buildSocketURL(runtimeConfig, normalizedStoreId, accessToken));
    sockets.set(normalizedStoreId, {
      key: connectionKey,
      socket: nextSocket
    });
    updateStatus();

    nextSocket.addEventListener("open", () => {
      reconnectAttempts.set(normalizedStoreId, 0);
      updateStatus();
    });

    nextSocket.addEventListener("message", async (message) => {
      try {
        const payload = JSON.parse(String(message.data || "{}"));
        lastEvent.value = payload;

        if (payload?.type !== "operation.updated") {
          return;
        }

        const payloadStoreId = String(payload?.storeId || "").trim();
        const mode = String(resolveSourceValue(options.scopeMode, "single") || "single").trim();

        if (mode === "all") {
          await refreshOverview();

          if (payloadStoreId && payloadStoreId === String(auth.activeStoreId || "").trim()) {
            await refreshSnapshot(payloadStoreId);
          }

          return;
        }

        if (payloadStoreId && payloadStoreId === String(auth.activeStoreId || "").trim()) {
          await refreshSnapshot(payloadStoreId);
        }
      } catch {
        // ignoramos payloads invalidos do socket
      }
    });

    nextSocket.addEventListener("close", () => {
      if (silencedCloses.has(normalizedStoreId)) {
        silencedCloses.delete(normalizedStoreId);
        updateStatus();
        return;
      }

      sockets.delete(normalizedStoreId);
      reconnectAttempts.set(normalizedStoreId, (reconnectAttempts.get(normalizedStoreId) || 0) + 1);
      scheduleReconnect(normalizedStoreId);
    });

    nextSocket.addEventListener("error", () => {
      updateStatus();
    });
  }

  function syncConnections() {
    if (import.meta.server) {
      return;
    }

    const expectedStoreIds = new Set(desiredStoreIds());
    for (const storeId of [...sockets.keys()]) {
      if (!expectedStoreIds.has(storeId)) {
        disconnectStore(storeId);
      }
    }

    if (!expectedStoreIds.size) {
      disconnectAll();
      return;
    }

    for (const storeId of expectedStoreIds) {
      ensureSocket(storeId);
    }

    updateStatus();
  }

  onMounted(() => {
    stopWatcher = watch(
      [
        () => auth.isAuthenticated,
        () => auth.activeStoreId,
        () => auth.accessToken,
        () => auth.accessibleStoreIds.join(","),
        () => String(resolveSourceValue(options.scopeMode, "single") || "single")
      ],
      () => {
        syncConnections();
      },
      { immediate: true }
    );
  });

  onBeforeUnmount(() => {
    if (typeof stopWatcher === "function") {
      stopWatcher();
      stopWatcher = null;
    }

    disconnectAll();
  });

  return {
    status,
    lastEvent
  };
}
