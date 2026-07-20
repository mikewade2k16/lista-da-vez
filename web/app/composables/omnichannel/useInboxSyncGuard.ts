/**
 * useInboxSyncGuard
 *
 * Responsabilidade UNICA: garantir que as mensagens da conversa ativa
 * estao atualizadas e que envios pendentes sao eventualmente marcados
 * como FAILED quando o backend nao confirma a entrega.
 *
 * Funciona de forma INDEPENDENTE do socket/realtime — mesmo quando o
 * socket mostra "connected", este guard verifica a verdade no servidor.
 *
 * Duas funcoes:
 *   1. checkFreshness() — compara a ultima mensagem local com a do servidor
 *   2. verifySend()     — monitora mensagens PENDING e marca FAILED apos timeout
 */
import type { Ref } from "vue";
import type { Message } from "~/types";

interface SyncGuardOptions {
  activeConversationId: Ref<string | null>;
  messages: Ref<Message[]>;
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>;
  refreshConversationMessages: (conversationId: string) => Promise<void>;
  syncConversationHistory: (conversationId: string) => Promise<unknown>;
  normalizeMessage: (messageEntry: Message) => Message;
  mergeMessages: (...chunks: Message[][]) => Message[];
  updateConversationPreviewFromMessage: (messageEntry: Message) => void;
}

const FRESHNESS_CHECK_INTERVAL_MS = 20_000;
const SEND_VERIFY_DELAYS = [5_000, 10_000, 20_000];
const PENDING_TIMEOUT_MS = 60_000;

export function useInboxSyncGuard(options: SyncGuardOptions) {
  let freshnessTimer: ReturnType<typeof setTimeout> | null = null;
  let freshnessActive = false;
  let freshnessInFlight = false;
  const sendVerifyTimers = new Map<string, ReturnType<typeof setTimeout>>();

  // ---------------------------------------------------------------
  // 1. Freshness check — runs every FRESHNESS_CHECK_INTERVAL_MS
  // ---------------------------------------------------------------

  async function checkFreshness(): Promise<void> {
    const conversationId = options.activeConversationId.value;
    if (!conversationId) return;

    try {
      // First: pull any missing messages from Evolution API into the DB.
      // syncConversationHistory has its own 2-min cooldown, so calling it
      // every 20s is safe — it silently no-ops when the cooldown is active.
      await options.syncConversationHistory(conversationId);

      const response = await options.apiFetch<{
        messages: Message[];
        hasMore: boolean;
      }>(`/conversations/${conversationId}/messages?limit=1`);

      if (!response.messages || response.messages.length === 0) return;

      const serverLatest = response.messages[0];
      if (!serverLatest) {
        return;
      }
      const localMessages = options.messages.value;

      if (localMessages.length === 0) {
        await options.refreshConversationMessages(conversationId);
        return;
      }

      const foundLocally = localMessages.some((m) => m.id === serverLatest.id);
      if (!foundLocally) {
        await options.refreshConversationMessages(conversationId);
      }
    } catch {
      // Silent — never break the UI
    }
  }

  async function runFreshnessCycle(): Promise<void> {
    if (!freshnessActive || freshnessInFlight) return;

    if (typeof document !== "undefined" && document.visibilityState === "hidden") {
      scheduleFreshnessCycle();
      return;
    }

    freshnessInFlight = true;
    try {
      await checkFreshness();
    } finally {
      freshnessInFlight = false;
      scheduleFreshnessCycle();
    }
  }

  function scheduleFreshnessCycle(): void {
    if (!freshnessActive) return;
    clearFreshnessTimer();
    freshnessTimer = setTimeout(() => void runFreshnessCycle(), FRESHNESS_CHECK_INTERVAL_MS);
  }

  function clearFreshnessTimer(): void {
    if (freshnessTimer) {
      clearTimeout(freshnessTimer);
      freshnessTimer = null;
    }
  }

  // ---------------------------------------------------------------
  // 2. Send verification — monitors PENDING messages after send
  // ---------------------------------------------------------------

  function verifySend(conversationId: string, messageId: string): void {
    const existing = sendVerifyTimers.get(messageId);
    if (existing) clearTimeout(existing);

    const createdAt = Date.now();
    let attemptIndex = 0;

    const doVerify = async (): Promise<void> => {
      sendVerifyTimers.delete(messageId);

      const localMsg = options.messages.value.find((m) => m.id === messageId);
      if (!localMsg || localMsg.status !== "PENDING") return;

      if (Date.now() - createdAt > PENDING_TIMEOUT_MS) {
        const idx = options.messages.value.findIndex((m) => m.id === messageId);
        const pendingMessage = idx >= 0 ? options.messages.value[idx] : null;
        if (pendingMessage && pendingMessage.status === "PENDING") {
          const failedMsg = options.normalizeMessage({
            ...pendingMessage,
            status: "FAILED" as Message["status"],
            updatedAt: new Date().toISOString(),
          });
          options.messages.value[idx] = failedMsg;
          options.updateConversationPreviewFromMessage(failedMsg);
        }
        return;
      }

      try {
        const response = await options.apiFetch<{
          messages: Message[];
          hasMore: boolean;
        }>(`/conversations/${conversationId}/messages?limit=10`);

        const serverMsg = response.messages.find((m) => m.id === messageId);
        if (serverMsg) {
          const normalized = options.normalizeMessage(serverMsg);
          if (normalized.status !== "PENDING") {
            options.messages.value = options.mergeMessages(options.messages.value, [normalized]);
            options.updateConversationPreviewFromMessage(normalized);
            return;
          }
        }
      } catch {
        // Silent
      }

      attemptIndex++;
      const nextDelay =
        attemptIndex < SEND_VERIFY_DELAYS.length
          ? SEND_VERIFY_DELAYS[attemptIndex]
          : SEND_VERIFY_DELAYS[SEND_VERIFY_DELAYS.length - 1];

      const timer = setTimeout(() => void doVerify(), nextDelay);
      sendVerifyTimers.set(messageId, timer);
    };

    const timer = setTimeout(() => void doVerify(), SEND_VERIFY_DELAYS[0]);
    sendVerifyTimers.set(messageId, timer);
  }

  // ---------------------------------------------------------------
  // 3. Lifecycle
  // ---------------------------------------------------------------

  function startSync(): void {
    freshnessActive = true;
    scheduleFreshnessCycle();
  }

  function stopSync(): void {
    freshnessActive = false;
    freshnessInFlight = false;
    clearFreshnessTimer();

    for (const timer of sendVerifyTimers.values()) {
      clearTimeout(timer);
    }
    sendVerifyTimers.clear();
  }

  return {
    startSync,
    stopSync,
    checkFreshness,
    verifySend,
  };
}
