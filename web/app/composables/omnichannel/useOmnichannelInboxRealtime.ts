import { nextTick, ref, type Ref } from "vue";
import type { Conversation, Message } from "~/types";
import { useAuthStore } from "~/stores/auth";
import { useCoreAccountStore } from "../../../layers/core/stores/account";
import {
  resolveRealtimeAccountId,
  useRealtimeSocket
} from "../../../layers/tasks/composables/useRealtimeSocket";
import {
  asRecord,
  isNearBottom
} from "~/composables/omnichannel/useOmnichannelInboxShared";

export type RealtimeConnectionState = "disconnected" | "connecting" | "connected" | "module_denied" | "auth_error";

// F5 — REALTIME REAL (reescreve o stub inerte da F1). LER ANTES DE MEXER.
//
// O legado falava socket.io (`io(apiBase, { auth: { token } })`) e recebia o objeto CRU do
// evento. Aqui o transporte e o WS NATIVO da casa (ticket + hub em memoria) via
// `useRealtimeSocket` (mesma base de tasks/calendar) no canal `omnichannel:account:{id}`.
//
// O que MUDA em relacao ao legado (envelope): o WS entrega o struct `Event` serializado
// (`{ type, accountId, resourceId, payload, savedAt }`), nao o objeto cru. Entao o `onMessage`
// despacha por `envelope.type` e entrega `envelope.payload` aos 3 handlers PRESERVADOS
// (`conversation.updated`, `message.created`, `message.updated`). Envelope sem `type` conhecido
// ou sem `payload` e ignorado — nunca derruba a tela.
//
// O que NAO muda (preservado byte a byte da referencia): os 3 handlers, os nomes de evento, a
// superficie de retorno e TODOS os fallbacks de polling (status 45s, stale 20s/delay 4s,
// heartbeat 5min, cooldown 5s, visibility 5min). O polling SOMA ao WS (principio 3): segura o
// inbox vivo quando o socket cai.
//
// accountId: NAO passamos `accountId` explicito — deixamos o `useRealtimeSocket` resolver pela
// MESMA fonte do REST (`accountStore.activeAccountId || auth.activeTenantId || ...`, via
// `resolveRealtimeAccountId`). Passar a fonte errada (ex.: `auth.activeTenantId` cru) faz o
// platform_admin cair no seed e o handshake nunca virar 101 → close 1006 em loop
// (web/layers/tasks/AGENT.md:315-331). NAO reintroduzir esse bug.
//
// Estados de conexao: o WS nativo NAO tem os motivos-string do socket.io
// (`ModuleAccessDenied`/`Unauthorized`) — o back responde o HTTP ANTES do upgrade, o browser so
// ve close 1006. Mapeamos o que e observavel: falha do ticket (`logTicketError`) → `auth_error`;
// `connected`/`disconnected` pelos hooks do socket. `module_denied` NAO vem do WS (rota
// `/v1/realtime/*` fica fora do gate de modulo — app.go); o sinal viria da camada REST, fora do
// escopo deste transporte. Mantemos o valor no union so por compatibilidade de quem le o estado.

export function useOmnichannelInboxRealtime(options: {
  publicApiBase: string;
  token: Ref<string | null>;
  tenantSlug: Ref<string | null>;
  shellClientId: Ref<number>;
  conversations: Ref<Conversation[]>;
  messages: Ref<Message[]>;
  activeConversationId: Ref<string | null>;
  visibleMessagesConversationId: Ref<string | null>;
  selectedInstanceId: Ref<string>;
  chatBodyRef: Ref<HTMLElement | null>;
  loadConversations: () => Promise<void>;
  refreshConversationMessages: (conversationId: string) => Promise<void>;
  loadWhatsAppStatus: () => Promise<void>;
  upsertConversation: (conversationEntry: Conversation) => void;
  normalizeMessage: (messageEntry: Message) => Message;
  mergeMessages: (...chunks: Message[][]) => Message[];
  updateConversationPreviewFromMessage: (messageEntry: Message) => void;
  updateConversationCacheFromMessage: (messageEntry: Message) => void;
  scheduleGroupParticipantsRefresh: (messageEntry: Message, force?: boolean) => void;
  shouldFlagMentionAlert: (messageEntry: Message) => boolean;
  incrementMentionAlert: (conversationId: string, amount?: number) => void;
  scrollToBottom: () => void;
  markConversationAsRead: (messageEntry?: Message) => void;
  messageNeedsMediaHydration: (messageEntry: Message) => boolean;
  hydrateRealtimeMediaMessage: (conversationId: string, messageId: string) => Promise<void>;
  scheduleStickyDateRefresh: () => void;
  enforceMessageWindow?: () => void;
}) {
  const auth = useAuthStore();
  const accountStore = useCoreAccountStore();

  const realtimeConnectionState = ref<RealtimeConnectionState>("disconnected");
  const socketEnabled = ref(false);
  let socketManuallyClosed = false;
  let reconnectSyncInFlight = false;
  let conversationsRefreshInFlight: Promise<void> | null = null;
  let lastConversationsRefreshAt = 0;
  let whatsappStatusPollTimer: ReturnType<typeof setTimeout> | null = null;
  let whatsappStatusPollingActive = false;
  let whatsappStatusPollingInFlight = false;
  let staleFallbackPollTimer: ReturnType<typeof setTimeout> | null = null;
  let staleFallbackPollingActive = false;
  let staleFallbackPollingInFlight = false;
  let connectedHeartbeatTimer: ReturnType<typeof setTimeout> | null = null;
  let connectedHeartbeatActive = false;
  let visibilityChangeHandler: (() => void) | null = null;
  const WHATSAPP_STATUS_POLL_INTERVAL_MS = 45_000;
  const CONVERSATIONS_REFRESH_COOLDOWN_MS = 5_000;
  const STALE_FALLBACK_POLL_INTERVAL_MS = 20_000;
  const STALE_FALLBACK_POLL_START_DELAY_MS = 4_000;
  const CONNECTED_HEARTBEAT_INTERVAL_MS = 5 * 60_000;
  const PAGE_VISIBILITY_STALE_THRESHOLD_MS = 5 * 60_000;

  function conversationMatchesSelectedInstance(conversation: {
    id: string;
    instanceId?: string | null;
  }) {
    const selectedInstanceId = options.selectedInstanceId.value;
    if (!selectedInstanceId || selectedInstanceId === "all") {
      return true;
    }

    return conversation.instanceId === selectedInstanceId;
  }

  function clearWhatsAppStatusPollTimer() {
    if (!whatsappStatusPollTimer) {
      return;
    }

    clearTimeout(whatsappStatusPollTimer);
    whatsappStatusPollTimer = null;
  }

  function scheduleWhatsAppStatusPoll(delay = WHATSAPP_STATUS_POLL_INTERVAL_MS) {
    if (!whatsappStatusPollingActive) {
      return;
    }

    clearWhatsAppStatusPollTimer();
    whatsappStatusPollTimer = setTimeout(() => {
      void runWhatsAppStatusPollCycle();
    }, delay);
  }

  async function runWhatsAppStatusPollCycle() {
    if (!whatsappStatusPollingActive) {
      return;
    }

    if (import.meta.client && document.visibilityState === "hidden") {
      scheduleWhatsAppStatusPoll();
      return;
    }

    if (whatsappStatusPollingInFlight) {
      scheduleWhatsAppStatusPoll();
      return;
    }

    whatsappStatusPollingInFlight = true;
    try {
      await options.loadWhatsAppStatus();
    } finally {
      whatsappStatusPollingInFlight = false;
      scheduleWhatsAppStatusPoll();
    }
  }

  function clearStaleFallbackPollTimer() {
    if (!staleFallbackPollTimer) {
      return;
    }

    clearTimeout(staleFallbackPollTimer);
    staleFallbackPollTimer = null;
  }

  function scheduleStaleFallbackPoll(delay = STALE_FALLBACK_POLL_INTERVAL_MS) {
    if (!staleFallbackPollingActive) {
      return;
    }

    clearStaleFallbackPollTimer();
    staleFallbackPollTimer = setTimeout(() => {
      void runStaleFallbackPollCycle();
    }, delay);
  }

  async function runStaleFallbackPollCycle() {
    if (!staleFallbackPollingActive) {
      return;
    }

    if (import.meta.client && document.visibilityState === "hidden") {
      scheduleStaleFallbackPoll();
      return;
    }

    if (staleFallbackPollingInFlight) {
      scheduleStaleFallbackPoll();
      return;
    }

    staleFallbackPollingInFlight = true;
    try {
      await refreshConversationsFromRealtime({
        force: true,
        reloadActive: true
      });
    } finally {
      staleFallbackPollingInFlight = false;
      scheduleStaleFallbackPoll();
    }
  }

  function startStaleFallbackPolling() {
    if (staleFallbackPollingActive) {
      return;
    }

    staleFallbackPollingActive = true;
    scheduleStaleFallbackPoll(STALE_FALLBACK_POLL_START_DELAY_MS);
  }

  function stopStaleFallbackPolling() {
    staleFallbackPollingActive = false;
    staleFallbackPollingInFlight = false;
    clearStaleFallbackPollTimer();
  }

  async function refreshConversationsFromRealtime(optionsArg: { force?: boolean; reloadActive?: boolean } = {}) {
    const force = optionsArg.force ?? false;
    const reloadActive = optionsArg.reloadActive ?? false;
    const now = Date.now();

    if (conversationsRefreshInFlight) {
      await conversationsRefreshInFlight;
      return;
    }

    if (!force && now - lastConversationsRefreshAt < CONVERSATIONS_REFRESH_COOLDOWN_MS) {
      return;
    }

    const request = (async () => {
      await options.loadConversations();

      if (!reloadActive) {
        return;
      }

      const activeConversationId = options.activeConversationId.value;
      if (activeConversationId) {
        await options.refreshConversationMessages(activeConversationId);
      }
    })();

    conversationsRefreshInFlight = request;
    try {
      await request;
    } finally {
      lastConversationsRefreshAt = Date.now();
      if (conversationsRefreshInFlight === request) {
        conversationsRefreshInFlight = null;
      }
    }
  }

  function clearConnectedHeartbeatTimer() {
    if (!connectedHeartbeatTimer) return;
    clearTimeout(connectedHeartbeatTimer);
    connectedHeartbeatTimer = null;
  }

  function scheduleConnectedHeartbeat() {
    if (!connectedHeartbeatActive) return;
    clearConnectedHeartbeatTimer();
    connectedHeartbeatTimer = setTimeout(() => {
      void runConnectedHeartbeatCycle();
    }, CONNECTED_HEARTBEAT_INTERVAL_MS);
  }

  async function runConnectedHeartbeatCycle() {
    if (!connectedHeartbeatActive) return;

    if (import.meta.client && document.visibilityState === "hidden") {
      scheduleConnectedHeartbeat();
      return;
    }

    const reloadActive = Boolean(options.activeConversationId.value);
    await refreshConversationsFromRealtime({ force: true, reloadActive });
    scheduleConnectedHeartbeat();
  }

  function startConnectedHeartbeat() {
    connectedHeartbeatActive = true;
    scheduleConnectedHeartbeat();
  }

  function stopConnectedHeartbeat() {
    connectedHeartbeatActive = false;
    clearConnectedHeartbeatTimer();
  }

  function handleVisibilityChange() {
    if (!import.meta.client || document.visibilityState !== "visible") return;
    const timeSinceLastSync = Date.now() - lastConversationsRefreshAt;
    if (timeSinceLastSync < PAGE_VISIBILITY_STALE_THRESHOLD_MS) return;
    const reloadActive = Boolean(options.activeConversationId.value);
    void refreshConversationsFromRealtime({ force: true, reloadActive });
  }

  function addVisibilityChangeListener() {
    if (!import.meta.client) return;
    removeVisibilityChangeListener();
    visibilityChangeHandler = handleVisibilityChange;
    document.addEventListener("visibilitychange", visibilityChangeHandler);
  }

  function removeVisibilityChangeListener() {
    if (!visibilityChangeHandler) return;
    document.removeEventListener("visibilitychange", visibilityChangeHandler);
    visibilityChangeHandler = null;
  }

  // === 3 handlers PRESERVADOS (byte a byte da referencia) — o WS entrega envelope.payload ===

  function handleConversationUpdated(payload: Conversation) {
    if (!conversationMatchesSelectedInstance(payload)) {
      const existingConversation = options.conversations.value.find((entry) => entry.id === payload.id);
      if (existingConversation) {
        options.conversations.value = options.conversations.value.filter((entry) => entry.id !== payload.id);
        if (options.activeConversationId.value === payload.id) {
          options.activeConversationId.value = null;
          options.messages.value = [];
        }
      }
      return;
    }

    options.upsertConversation(payload);
  }

  async function handleMessageCreated(payload: Message) {
    const payloadRecord = asRecord(payload);
    const correlationId =
      payloadRecord && typeof payloadRecord.correlationId === "string"
        ? payloadRecord.correlationId.trim()
        : "";
    const isHistoryBackfillEvent = correlationId.startsWith("sync-history:");
    const normalizedPayload = options.normalizeMessage(payload);

    if (isHistoryBackfillEvent) {
      return;
    }

    options.updateConversationPreviewFromMessage(normalizedPayload);
    options.scheduleGroupParticipantsRefresh(normalizedPayload);
    const hasMentionAlert = options.shouldFlagMentionAlert(normalizedPayload);
    const isVisibleActiveConversation =
      normalizedPayload.conversationId === options.activeConversationId.value &&
      options.visibleMessagesConversationId.value === normalizedPayload.conversationId;

    if (isVisibleActiveConversation) {
      const shouldStickToBottom = isNearBottom(options.chatBodyRef.value);

      options.messages.value = options.mergeMessages(options.messages.value, [normalizedPayload]);
      if (options.enforceMessageWindow) {
        options.enforceMessageWindow();
      }
      await nextTick();

      if (normalizedPayload.direction === "OUTBOUND" || shouldStickToBottom) {
        options.scrollToBottom();
      }

      if (normalizedPayload.direction === "OUTBOUND" || shouldStickToBottom) {
        options.markConversationAsRead(normalizedPayload);
      }

      if (hasMentionAlert && !shouldStickToBottom) {
        options.incrementMentionAlert(normalizedPayload.conversationId);
      }

      if (options.messageNeedsMediaHydration(normalizedPayload)) {
        void options.hydrateRealtimeMediaMessage(normalizedPayload.conversationId, normalizedPayload.id);
      }

      options.scheduleStickyDateRefresh();
      return;
    }

    options.updateConversationCacheFromMessage(normalizedPayload);

    if (!options.conversations.value.find((entry) => entry.id === normalizedPayload.conversationId)) {
      await refreshConversationsFromRealtime();
    }

    if (hasMentionAlert) {
      options.incrementMentionAlert(normalizedPayload.conversationId);
    }
  }

  function handleMessageUpdated(payload: Partial<Message> & { id: string }) {
    const payloadRecord = asRecord(payload);
    const messageId = payloadRecord && typeof payloadRecord.id === "string" ? payloadRecord.id : "";
    const correlationId =
      payloadRecord && typeof payloadRecord.correlationId === "string"
        ? payloadRecord.correlationId.trim()
        : "";
    const isHistoryBackfillEvent = correlationId.startsWith("sync-history:");
    if (!messageId) {
      return;
    }

    const messageIndex = options.messages.value.findIndex((entry) => entry.id === messageId);
    let mergedMessage: Message | null = null;

    if (messageIndex >= 0) {
      mergedMessage = options.normalizeMessage({
        ...options.messages.value[messageIndex],
        ...payload
      } as Message);
      options.messages.value[messageIndex] = mergedMessage;
    }

    const isFullMessagePayload =
      payloadRecord !== null &&
      typeof payloadRecord.conversationId === "string" &&
      payloadRecord.conversationId.trim().length > 0 &&
      typeof payloadRecord.direction === "string" &&
      typeof payloadRecord.createdAt === "string";

    if (isFullMessagePayload) {
      const normalizedPayload = options.normalizeMessage(payload as Message);
      const isVisibleActiveConversation =
        normalizedPayload.conversationId === options.activeConversationId.value &&
        options.visibleMessagesConversationId.value === normalizedPayload.conversationId;
      options.scheduleGroupParticipantsRefresh(normalizedPayload);
      if (isVisibleActiveConversation) {
        options.messages.value = options.mergeMessages(options.messages.value, [normalizedPayload]);
      }

      options.updateConversationCacheFromMessage(normalizedPayload);

      if (!isHistoryBackfillEvent) {
        options.updateConversationPreviewFromMessage(normalizedPayload);
      }

      if (options.messageNeedsMediaHydration(normalizedPayload)) {
        void options.hydrateRealtimeMediaMessage(normalizedPayload.conversationId, normalizedPayload.id);
      }

      if (
        options.shouldFlagMentionAlert(normalizedPayload) &&
        normalizedPayload.conversationId !== options.activeConversationId.value
      ) {
        options.incrementMentionAlert(normalizedPayload.conversationId);
      }

      return;
    }

    if (mergedMessage && !isHistoryBackfillEvent) {
      options.updateConversationPreviewFromMessage(mergedMessage);

      if (
        mergedMessage.conversationId === options.activeConversationId.value &&
        options.messageNeedsMediaHydration(mergedMessage)
      ) {
        void options.hydrateRealtimeMediaMessage(mergedMessage.conversationId, mergedMessage.id);
      }
    }

    for (const conversationEntry of options.conversations.value) {
      if (conversationEntry.lastMessage?.id !== messageId) {
        continue;
      }

      if (payload.status) {
        conversationEntry.lastMessage.status = payload.status;
      }

      if (typeof payload.content === "string" && payload.content.trim().length > 0) {
        conversationEntry.lastMessage.content = payload.content;
      }

      if (typeof payload.mediaUrl === "string" && payload.mediaUrl.trim().length > 0) {
        conversationEntry.lastMessage.mediaUrl = payload.mediaUrl;
      }

      break;
    }
  }

  // === Adapter de envelope: despacha o Event serializado do WS para os handlers preservados ===

  function dispatchRealtimeEnvelope(envelope: Record<string, unknown>) {
    const type = typeof envelope.type === "string" ? envelope.type : "";
    if (!type || type === "realtime.connected") {
      return;
    }

    // Isolamento por conta (defesa em profundidade): o topico ja e account-scoped no back, mas
    // descartamos qualquer evento de outra conta pela MESMA fonte de conta do WS/REST.
    const eventAccountId = String(envelope.accountId ?? "").trim();
    const currentAccountId = resolveRealtimeAccountId(auth, accountStore, "");
    if (eventAccountId && currentAccountId && eventAccountId !== currentAccountId) {
      return;
    }

    const payload =
      envelope.payload && typeof envelope.payload === "object"
        ? (envelope.payload as Record<string, unknown>)
        : null;
    if (!payload) {
      return;
    }

    switch (type) {
      case "conversation.updated":
        handleConversationUpdated(payload as unknown as Conversation);
        return;
      case "message.created":
        void handleMessageCreated(payload as unknown as Message);
        return;
      case "message.updated":
        handleMessageUpdated(payload as unknown as Partial<Message> & { id: string });
        return;
      default:
        return;
    }
  }

  // === Transporte WS nativo (ticket + reconnect + isolamento por conta) via useRealtimeSocket ===

  function handleSocketOpen() {
    if (socketManuallyClosed) {
      return;
    }

    realtimeConnectionState.value = "connected";
    stopStaleFallbackPolling();
    startConnectedHeartbeat();
    addVisibilityChangeListener();

    if (reconnectSyncInFlight) {
      return;
    }

    reconnectSyncInFlight = true;
    void (async () => {
      await refreshConversationsFromRealtime({
        force: true,
        reloadActive: true
      });
    })().finally(() => {
      reconnectSyncInFlight = false;
    });
  }

  function handleSocketClosed() {
    if (socketManuallyClosed) {
      return;
    }

    realtimeConnectionState.value = "disconnected";
    stopConnectedHeartbeat();
    removeVisibilityChangeListener();
    startStaleFallbackPolling();
  }

  function handleTicketError() {
    if (socketManuallyClosed) {
      return;
    }

    realtimeConnectionState.value = "auth_error";
    startStaleFallbackPolling();
  }

  const socket = useRealtimeSocket({
    enabled: () => socketEnabled.value,
    scope: "account",
    path: "/v1/realtime/omnichannel",
    scopeDefault: "account",
    normalizeScope: () => "account",
    isValid: ({ accountId }) => Boolean(accountId),
    watchSources: [
      () => socketEnabled.value,
      () => auth.isAuthenticated,
      () => auth.accessToken,
      () => auth.activeTenantId,
      () => accountStore.activeAccountId
    ],
    onOpen: handleSocketOpen,
    onMessage: (payload) => dispatchRealtimeEnvelope(payload),
    logClose: handleSocketClosed,
    logTicketError: handleTicketError
  });

  function connectSocket() {
    socketManuallyClosed = false;

    if (!options.token.value) {
      realtimeConnectionState.value = "disconnected";
      startStaleFallbackPolling();
      return;
    }

    realtimeConnectionState.value = "connecting";
    socketEnabled.value = true;
    void socket.ensureConnection();
  }

  function disconnectSocket() {
    socketManuallyClosed = true;
    reconnectSyncInFlight = false;
    realtimeConnectionState.value = "disconnected";
    stopStaleFallbackPolling();
    stopConnectedHeartbeat();
    removeVisibilityChangeListener();
    socketEnabled.value = false;
    socket.disconnect();
  }

  function stopWhatsAppStatusPolling() {
    whatsappStatusPollingActive = false;
    clearWhatsAppStatusPollTimer();
  }

  function startWhatsAppStatusPolling() {
    stopWhatsAppStatusPolling();
    whatsappStatusPollingActive = true;
    scheduleWhatsAppStatusPoll();
  }

  return {
    realtimeConnectionState,
    connectSocket,
    disconnectSocket,
    startWhatsAppStatusPolling,
    stopWhatsAppStatusPolling
  };
}
