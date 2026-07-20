import { ref, type ComputedRef } from "vue";

export function useOmnichannelAdminQrPolling(options: {
  isConnected: ComputedRef<boolean>;
  hasQrCode: ComputedRef<boolean>;
  refreshWhatsAppStatus: (options?: { silent?: boolean; force?: boolean }) => Promise<void>;
  fetchQrCode: (options?: { force?: boolean; silent?: boolean }) => Promise<void>;
}) {
  const POLL_INTERVAL_MS = 18_000;
  const QR_REFRESH_INTERVAL_MS = 45_000;
  const FORCE_QR_MIN_INTERVAL_MS = 180_000;

  const qrPollingTimer = ref<ReturnType<typeof setTimeout> | null>(null);
  const qrPollingInFlight = ref(false);
  let qrPollingActive = false;
  let lastQrRefreshAt = 0;
  let lastForcedQrAt = 0;

  function clearPollingTimer() {
    if (!qrPollingTimer.value) {
      return;
    }

    clearTimeout(qrPollingTimer.value);
    qrPollingTimer.value = null;
  }

  function scheduleNextPoll(delay = POLL_INTERVAL_MS) {
    if (!qrPollingActive) {
      return;
    }

    clearPollingTimer();
    qrPollingTimer.value = setTimeout(() => {
      void runPollingCycle();
    }, delay);
  }

  async function runPollingCycle() {
    if (!qrPollingActive) {
      return;
    }

    if (import.meta.client && document.visibilityState === "hidden") {
      scheduleNextPoll();
      return;
    }

    if (options.isConnected.value) {
      stopQrPolling();
      return;
    }

    if (qrPollingInFlight.value) {
      scheduleNextPoll();
      return;
    }

    qrPollingInFlight.value = true;
    try {
      await options.refreshWhatsAppStatus({ silent: true });

      if (options.isConnected.value) {
        stopQrPolling();
        return;
      }

      const now = Date.now();
      const shouldRefreshQr = !options.hasQrCode.value || now - lastQrRefreshAt >= QR_REFRESH_INTERVAL_MS;
      if (shouldRefreshQr) {
        const shouldForceQrRefresh =
          !options.hasQrCode.value && now - lastForcedQrAt >= FORCE_QR_MIN_INTERVAL_MS;

        await options.fetchQrCode({
          force: shouldForceQrRefresh,
          silent: true
        });

        lastQrRefreshAt = Date.now();
        if (shouldForceQrRefresh) {
          lastForcedQrAt = lastQrRefreshAt;
        }
      }
    } finally {
      qrPollingInFlight.value = false;
      scheduleNextPoll();
    }
  }

  function stopQrPolling() {
    qrPollingActive = false;
    clearPollingTimer();
  }

  function startQrPolling() {
    stopQrPolling();
    qrPollingActive = true;
    lastQrRefreshAt = 0;
    lastForcedQrAt = 0;
    scheduleNextPoll(0);
  }

  return {
    qrPollingTimer,
    qrPollingInFlight,
    stopQrPolling,
    startQrPolling
  };
}
