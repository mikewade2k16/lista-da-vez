import { nextTick, type Ref, watch } from "vue";
import type { Conversation, GroupParticipant, Message } from "~/types";
import { ApiClientError } from "~/composables/useApi";
import type {
  GroupParticipantsResponse,
  MessagesPageResponse,
  SyncConversationHistoryResponse,
  SyncOpenConversationsResponse
} from "~/composables/omnichannel/useOmnichannelInboxShared";
import {
  MESSAGE_PAGE_SIZE,
  toArrayOrEmpty
} from "~/composables/omnichannel/useOmnichannelInboxShared";

export function useOmnichannelInboxHistory(options: {
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>;
  conversations: Ref<Conversation[]>;
  messages: Ref<Message[]>;
  visibleMessagesConversationId: Ref<string | null>;
  activeConversationId: Ref<string | null>;
  selectedInstanceId: Ref<string>;
  loadingWhatsAppStatus: Ref<boolean>;
  isWhatsAppConfigured: Ref<boolean>;
  isWhatsAppConnected: Ref<boolean>;
  loadingConversations: Ref<boolean>;
  loadingMessages: Ref<boolean>;
  loadingOlderMessages: Ref<boolean>;
  loadingGroupParticipants: Ref<boolean>;
  hasMoreMessages: Ref<boolean>;
  chatBodyRef: Ref<HTMLElement | null>;
  mentionAlertState: Ref<Record<string, number>>;
  groupParticipantsByConversation: Record<string, GroupParticipant[]>;
  realtimeMessageHydrationLocks: Set<string>;
  groupParticipantsRefreshAtByConversation: Map<string, number>;
  groupParticipantsInFlightByConversation: Set<string>;
  historySyncAttemptAtByConversation: Map<string, number>;
  historySyncInFlightByConversation: Set<string>;
  sortConversations: () => void;
  bootstrapReadState: () => void;
  getReadAt: (conversationId: string) => number | null;
  getSelectConversation: () => ((conversationId: string) => Promise<void>) | null;
  normalizeMessage: (messageEntry: Message) => Message;
  mergeMessages: (...chunks: Message[][]) => Message[];
  updateConversationPreviewFromMessage: (messageEntry: Message) => void;
  messageNeedsMediaHydration: (messageEntry: Message) => boolean;
}) {
  const HISTORY_SYNC_COOLDOWN_MS = 120_000;
  const OPEN_CONVERSATIONS_SYNC_COOLDOWN_MS = 90_000;
  const CONVERSATION_CACHE_STALE_MS = 45_000;
  let conversationsRequestInFlight: Promise<void> | null = null;
  let openConversationsSyncRequestInFlight: Promise<SyncOpenConversationsResponse | null> | null = null;
  let openConversationsLastAttemptAt = 0;
  let openConversationsBootstrapCompleted = false;
  const messagesRequestInFlightByConversation = new Map<string, Promise<void>>();
  const conversationMessagesCache = new Map<string, {
    messages: Message[];
    hasMore: boolean;
    lastLoadedAt: number;
  }>();

  function canRunProviderSync() {
    if (options.loadingWhatsAppStatus.value) {
      return false;
    }

    if (!options.isWhatsAppConfigured.value) {
      return false;
    }

    return options.isWhatsAppConnected.value;
  }

  function cloneMessages(entries: Message[]) {
    return entries.map((entry) => options.normalizeMessage(entry));
  }

  function getConversationCache(conversationId: string) {
    return conversationMessagesCache.get(conversationId) ?? null;
  }

  function persistConversationCache(
    conversationId: string,
    messageEntries: Message[],
    hasMore: boolean,
    lastLoadedAt = Date.now()
  ) {
    conversationMessagesCache.set(conversationId, {
      messages: cloneMessages(messageEntries),
      hasMore,
      lastLoadedAt
    });
  }

  function applyConversationSnapshot(conversationId: string, snapshot: { messages: Message[]; hasMore: boolean }) {
    options.visibleMessagesConversationId.value = conversationId;
    options.messages.value = cloneMessages(snapshot.messages);
    options.hasMoreMessages.value = snapshot.hasMore;
  }

  function resolveConversationPreviewTimestamp(conversationId: string) {
    const conversationEntry = options.conversations.value.find((entry) => entry.id === conversationId);
    return Number(new Date(conversationEntry?.lastMessageAt ?? 0));
  }

  function resolveLatestMessageTimestamp(messageEntries: Message[]) {
    const lastMessage = messageEntries[messageEntries.length - 1] ?? null;
    return Number(new Date(lastMessage?.createdAt ?? 0));
  }

  function conversationCacheHasPreviewGap(
    conversationId: string,
    cacheEntry: { messages: Message[]; hasMore: boolean; lastLoadedAt: number }
  ) {
    const previewTimestamp = resolveConversationPreviewTimestamp(conversationId);
    const latestCachedTimestamp = resolveLatestMessageTimestamp(cacheEntry.messages);

    if (!Number.isFinite(previewTimestamp) || previewTimestamp <= 0) {
      return false;
    }

    if (!Number.isFinite(latestCachedTimestamp) || latestCachedTimestamp <= 0) {
      return true;
    }

    return previewTimestamp > latestCachedTimestamp;
  }

  function conversationNeedsUnreadBoundary(
    conversationId: string,
    messageEntries: Message[],
    hasMore: boolean
  ) {
    if (!hasMore || messageEntries.length < 1) {
      return false;
    }

    const readAt = options.getReadAt(conversationId);
    if (!readAt) {
      return false;
    }

    const conversationPreviewTimestamp = resolveConversationPreviewTimestamp(conversationId);
    if (!Number.isFinite(conversationPreviewTimestamp) || conversationPreviewTimestamp <= readAt) {
      return false;
    }

    const oldestLoadedTimestamp = Number(new Date(messageEntries[0]?.createdAt ?? 0));
    if (!Number.isFinite(oldestLoadedTimestamp)) {
      return false;
    }

    return oldestLoadedTimestamp > readAt;
  }

  function wait(ms: number) {
    return new Promise((resolve) => {
      setTimeout(resolve, ms);
    });
  }

  watch(
    () => ({
      conversationId: options.visibleMessagesConversationId.value,
      hasMore: options.hasMoreMessages.value,
      messages: options.messages.value
    }),
    (snapshot) => {
      if (!snapshot.conversationId) {
        return;
      }

      persistConversationCache(snapshot.conversationId, snapshot.messages, snapshot.hasMore);
    },
    {
      deep: true
    }
  );

  async function fetchMessagesPage(conversationId: string, beforeId?: string) {
    const query = new URLSearchParams({
      limit: String(MESSAGE_PAGE_SIZE)
    });

    if (beforeId) {
      query.set("beforeId", beforeId);
    }

    return options.apiFetch<MessagesPageResponse>(`/conversations/${conversationId}/messages?${query.toString()}`);
  }

  async function fetchMessageById(conversationId: string, messageId: string) {
    return options.apiFetch<Message>(`/conversations/${conversationId}/messages/${messageId}`);
  }

  async function handleConversationNotFound(conversationId: string) {
    conversationMessagesCache.delete(conversationId);

    if (options.activeConversationId.value === conversationId) {
      options.activeConversationId.value = null;
      options.messages.value = [];
      options.visibleMessagesConversationId.value = null;
      options.hasMoreMessages.value = false;
    }

    options.conversations.value = options.conversations.value.filter((entry) => entry.id !== conversationId);

    const fallbackConversation = options.conversations.value[0];
    const selectConversation = options.getSelectConversation();
    if (fallbackConversation && selectConversation && options.activeConversationId.value === null) {
      try {
        await selectConversation(fallbackConversation.id);
      } catch {
        // fallback selection is best-effort after stale conversation removal.
      }
    }
  }

  async function refreshConversationMessages(
    conversationId: string,
    refreshOptions: {
      silent?: boolean;
      syncHistory?: boolean;
      ensureUnreadBoundary?: boolean;
    } = {}
  ) {
    const cached = getConversationCache(conversationId);
    const isActiveConversation = options.activeConversationId.value === conversationId;
    const shouldShowLoader = isActiveConversation && refreshOptions.silent !== true && !cached;

    if (shouldShowLoader) {
      options.loadingMessages.value = true;
    }

    try {
      const response = await fetchMessagesPage(conversationId);
      const baseMessages =
        options.visibleMessagesConversationId.value === conversationId
          ? options.messages.value
          : cached?.messages ?? [];
      const nextMessages = baseMessages.length > 0
        ? options.mergeMessages(baseMessages, response.messages)
        : options.mergeMessages(response.messages);
      const nextHasMore =
        response.hasMore ||
        cached?.hasMore ||
        (
          options.visibleMessagesConversationId.value === conversationId &&
          options.hasMoreMessages.value
        ) ||
        false;

      if (isActiveConversation) {
        options.visibleMessagesConversationId.value = conversationId;
        options.messages.value = nextMessages;
        options.hasMoreMessages.value = nextHasMore;

        if (refreshOptions.ensureUnreadBoundary !== false) {
          await ensureUnreadBoundaryLoaded(conversationId);
        }

        persistConversationCache(conversationId, options.messages.value, options.hasMoreMessages.value);
      } else {
        persistConversationCache(conversationId, nextMessages, nextHasMore);
      }

      if (refreshOptions.syncHistory !== false) {
        void syncConversationHistoryInBackground(conversationId);
      }
    } catch (error) {
      if (error instanceof ApiClientError && error.statusCode === 404) {
        await handleConversationNotFound(conversationId);
        return;
      }

      throw error;
    } finally {
      if (shouldShowLoader) {
        options.loadingMessages.value = false;
      }
    }
  }

  function updateConversationCacheFromMessage(messageEntry: Message) {
    const conversationId = messageEntry.conversationId;
    const cached = getConversationCache(conversationId);
    const isVisibleConversation = options.visibleMessagesConversationId.value === conversationId;
    const baseMessages = isVisibleConversation
      ? options.messages.value
      : cached?.messages ?? [];

    if (!isVisibleConversation && baseMessages.length < 1) {
      return;
    }

    const nextMessages = options.mergeMessages(baseMessages, [messageEntry]);
    const nextHasMore = isVisibleConversation
      ? options.hasMoreMessages.value
      : (cached?.hasMore ?? false);

    if (isVisibleConversation) {
      options.messages.value = nextMessages;
      options.hasMoreMessages.value = nextHasMore;
    }

    persistConversationCache(conversationId, nextMessages, nextHasMore);
  }

  async function hydrateRealtimeMediaMessage(conversationId: string, messageId: string) {
    const key = `${conversationId}:${messageId}`;
    if (options.realtimeMessageHydrationLocks.has(key)) {
      return;
    }

    options.realtimeMessageHydrationLocks.add(key);
    const delays = [0, 350, 900, 1800];

    try {
      for (const delay of delays) {
        if (delay > 0) {
          await new Promise((resolve) => {
            setTimeout(resolve, delay);
          });
        }

        try {
          const messageEntry = options.normalizeMessage(await fetchMessageById(conversationId, messageId));
          options.messages.value = options.mergeMessages(options.messages.value, [messageEntry]);
          options.updateConversationPreviewFromMessage(messageEntry);

          if (!options.messageNeedsMediaHydration(messageEntry)) {
            return;
          }
        } catch {
          // best-effort hydration for realtime payloads sanitized by the API.
        }
      }
    } finally {
      options.realtimeMessageHydrationLocks.delete(key);
    }
  }

  async function syncOpenConversations(force = false) {
    if (!canRunProviderSync()) {
      return null;
    }

    const now = Date.now();
    if (!force && now - openConversationsLastAttemptAt < OPEN_CONVERSATIONS_SYNC_COOLDOWN_MS) {
      return null;
    }

    if (openConversationsSyncRequestInFlight) {
      return openConversationsSyncRequestInFlight;
    }

    const request = (async () => {
      openConversationsLastAttemptAt = Date.now();
      try {
        return await options.apiFetch<SyncOpenConversationsResponse>("/conversations/sync-open", {
          method: "POST",
          timeout: 120_000,
          body: {
            limitConversations: 200,
            includeGroups: true
          }
        });
      } catch (error) {
        if (
          error instanceof ApiClientError &&
          [400, 404, 405, 409, 422, 503].includes(error.statusCode)
        ) {
          return null;
        }

        console.error("Nao foi possivel sincronizar conversas abertas do WhatsApp.", error);
        return null;
      }
    })();

    openConversationsSyncRequestInFlight = request;
    try {
      return await request;
    } finally {
      if (openConversationsSyncRequestInFlight === request) {
        openConversationsSyncRequestInFlight = null;
      }
    }
  }

  async function syncOpenConversationsInBackground() {
    if (openConversationsBootstrapCompleted) {
      return;
    }

    const result = await syncOpenConversations();
    if (!result) {
      return;
    }

    openConversationsBootstrapCompleted = true;

    if (result.createdCount < 1 && result.updatedCount < 1) {
      return;
    }

    await loadConversations({ skipOpenSync: true });
  }

  async function loadConversations(loadOptions: { skipOpenSync?: boolean } = {}) {
    if (conversationsRequestInFlight) {
      await conversationsRequestInFlight;
      return;
    }

    const request = (async () => {
      options.loadingConversations.value = true;
      try {
        let response: unknown = null;
        let lastError: unknown = null;

        for (let attempt = 0; attempt < 3; attempt += 1) {
          try {
            const query = new URLSearchParams();
            if (options.selectedInstanceId.value && options.selectedInstanceId.value !== "all") {
              query.set("instanceId", options.selectedInstanceId.value);
            }
            response = await options.apiFetch<unknown>(`/conversations${query.size ? `?${query.toString()}` : ""}`, {
              timeout: 30_000
            });
            break;
          } catch (error) {
            lastError = error;
            if (attempt < 2) {
              await wait(600 * (attempt + 1));
            }
          }
        }

        if (response === null) {
          throw lastError ?? new Error("Falha ao carregar conversas.");
        }

        options.conversations.value = toArrayOrEmpty<Conversation>(response);
        const conversationIds = new Set(options.conversations.value.map((entry) => entry.id));
        options.mentionAlertState.value = Object.fromEntries(
          Object.entries(options.mentionAlertState.value).filter(([conversationId, count]) => {
            return Number(count) > 0 && conversationIds.has(conversationId);
          })
        );
        options.sortConversations();
        options.bootstrapReadState();

        if (
          options.activeConversationId.value &&
          !options.conversations.value.some((entry) => entry.id === options.activeConversationId.value)
        ) {
          options.activeConversationId.value = null;
          options.messages.value = [];
          options.visibleMessagesConversationId.value = null;
        }

        if (!options.activeConversationId.value && options.conversations.value.length > 0) {
          const firstConversation = options.conversations.value[0];
          const selectConversation = options.getSelectConversation();
          if (firstConversation && selectConversation) {
            await selectConversation(firstConversation.id);
          }
        }
      } catch (error) {
        console.error("Nao foi possivel carregar conversas no momento.", error);
      } finally {
        options.loadingConversations.value = false;
        if (!loadOptions.skipOpenSync && !openConversationsBootstrapCompleted) {
          void syncOpenConversationsInBackground();
        }
      }
    })();

    conversationsRequestInFlight = request;
    try {
      await request;
    } finally {
      if (conversationsRequestInFlight === request) {
        conversationsRequestInFlight = null;
      }
    }
  }

  async function ensureUnreadBoundaryLoaded(conversationId: string) {
    const readAt = options.getReadAt(conversationId);
    if (!readAt) {
      return;
    }

    let guard = 0;

    while (
      options.hasMoreMessages.value &&
      options.messages.value.length > 0 &&
      Number(new Date(options.messages.value[0]?.createdAt ?? 0)) > readAt &&
      guard < 6
    ) {
      const oldestMessage = options.messages.value[0];
      if (!oldestMessage) {
        return;
      }

      const oldestId = oldestMessage.id;
      const response = await fetchMessagesPage(conversationId, oldestId);
      options.messages.value = options.mergeMessages(response.messages, options.messages.value);
      options.hasMoreMessages.value = response.hasMore;
      guard += 1;
    }
  }

  async function loadConversationMessages(
    conversationId: string,
    loadOptions: {
      forceRefresh?: boolean;
    } = {}
  ) {
    const existingRequest = messagesRequestInFlightByConversation.get(conversationId);
    if (existingRequest) {
      await existingRequest;
      return;
    }

    const request = (async () => {
      const cached = getConversationCache(conversationId);
      const canUseCache = !loadOptions.forceRefresh && cached && cached.messages.length > 0;

      if (canUseCache) {
        applyConversationSnapshot(conversationId, cached);

        const needsImmediateRefresh =
          conversationCacheHasPreviewGap(conversationId, cached) ||
          Date.now() - cached.lastLoadedAt > CONVERSATION_CACHE_STALE_MS;
        const needsUnreadBoundary = conversationNeedsUnreadBoundary(
          conversationId,
          cached.messages,
          cached.hasMore
        );

        try {
          if (needsImmediateRefresh) {
            await refreshConversationMessages(conversationId, {
              silent: true,
              syncHistory: true,
              ensureUnreadBoundary: true
            });
          } else if (needsUnreadBoundary) {
            await ensureUnreadBoundaryLoaded(conversationId);
            persistConversationCache(conversationId, options.messages.value, options.hasMoreMessages.value);
          }
        } catch (error) {
          console.error("Nao foi possivel atualizar mensagens da conversa em background.", error);
        }

        return;
      }

      options.loadingMessages.value = true;
      try {
        const response = await fetchMessagesPage(conversationId);
        options.visibleMessagesConversationId.value = conversationId;
        options.messages.value = options.mergeMessages(response.messages);
        options.hasMoreMessages.value = response.hasMore;

        await ensureUnreadBoundaryLoaded(conversationId);
        persistConversationCache(conversationId, options.messages.value, options.hasMoreMessages.value);
        void syncConversationHistoryInBackground(conversationId);
      } catch (error) {
        if (error instanceof ApiClientError && error.statusCode === 404) {
          await handleConversationNotFound(conversationId);
          return;
        }

        console.error("Nao foi possivel carregar mensagens da conversa.", error);
      } finally {
        options.loadingMessages.value = false;
      }
    })();

    messagesRequestInFlightByConversation.set(conversationId, request);
    try {
      await request;
    } finally {
      if (messagesRequestInFlightByConversation.get(conversationId) === request) {
        messagesRequestInFlightByConversation.delete(conversationId);
      }
    }
  }

  async function syncConversationHistory(conversationId: string) {
    if (!canRunProviderSync()) {
      return null;
    }

    const now = Date.now();
    const lastAttemptAt = options.historySyncAttemptAtByConversation.get(conversationId) ?? 0;
    if (now - lastAttemptAt < HISTORY_SYNC_COOLDOWN_MS) {
      return null;
    }

    if (options.historySyncInFlightByConversation.has(conversationId)) {
      return null;
    }

    const conversationEntry = options.conversations.value.find((entry) => entry.id === conversationId);
    if (!conversationEntry || conversationEntry.channel !== "WHATSAPP") {
      return null;
    }

    options.historySyncAttemptAtByConversation.set(conversationId, now);
    options.historySyncInFlightByConversation.add(conversationId);

    try {
      return await options.apiFetch<SyncConversationHistoryResponse>(
        `/conversations/${conversationId}/messages/sync-history`,
        {
          method: "POST",
          timeout: 120_000,
          body: {
            maxMessages: 300
          }
        }
      );
    } catch {
      return null;
    } finally {
      options.historySyncInFlightByConversation.delete(conversationId);
    }
  }

  async function syncConversationHistoryInBackground(conversationId: string) {
    const result = await syncConversationHistory(conversationId);
    if (!result || result.processedCount < 1) {
      return;
    }

    if (options.activeConversationId.value !== conversationId) {
      return;
    }

    try {
      await refreshConversationMessages(conversationId, {
        silent: true,
        syncHistory: false,
        ensureUnreadBoundary: true
      });
    } catch {
      // best-effort refresh after sync.
    }
  }

  async function loadGroupParticipants(conversationId: string, force = false) {
    if (options.groupParticipantsInFlightByConversation.has(conversationId)) {
      return;
    }

    const conversationEntry = options.conversations.value.find((entry) => entry.id === conversationId);
    if (!conversationEntry?.externalId.endsWith("@g.us")) {
      options.groupParticipantsByConversation[conversationId] = [];
      return;
    }

    if (!force && options.groupParticipantsByConversation[conversationId]?.length) {
      return;
    }

    options.loadingGroupParticipants.value = true;
    options.groupParticipantsInFlightByConversation.add(conversationId);
    try {
      const response = await options.apiFetch<GroupParticipantsResponse>(
        `/conversations/${conversationId}/group-participants`,
        {
          timeout: 15_000
        }
      );
      options.groupParticipantsByConversation[conversationId] = response.participants ?? [];
    } catch {
      if (!options.groupParticipantsByConversation[conversationId]) {
        options.groupParticipantsByConversation[conversationId] = [];
      }
    } finally {
      options.groupParticipantsInFlightByConversation.delete(conversationId);
      options.loadingGroupParticipants.value = false;
    }
  }

  function shouldRefreshGroupParticipantsFromMessage(messageEntry: Message) {
    const conversationEntry = options.conversations.value.find((entry) => entry.id === messageEntry.conversationId);
    if (!conversationEntry || !conversationEntry.externalId.endsWith("@g.us")) {
      return false;
    }

    if (messageEntry.direction !== "INBOUND") {
      return false;
    }

    return true;
  }

  function scheduleGroupParticipantsRefresh(messageEntry: Message, force = false) {
    if (!shouldRefreshGroupParticipantsFromMessage(messageEntry)) {
      return;
    }

    const conversationId = messageEntry.conversationId;
    const now = Date.now();
    const lastRunAt = options.groupParticipantsRefreshAtByConversation.get(conversationId) ?? 0;

    if (!force && now - lastRunAt < 45_000) {
      return;
    }

    options.groupParticipantsRefreshAtByConversation.set(conversationId, now);
    void loadGroupParticipants(conversationId, true);
  }

  async function loadOlderMessages() {
    if (
      !options.activeConversationId.value ||
      !options.hasMoreMessages.value ||
      options.loadingOlderMessages.value ||
      !options.messages.value.length
    ) {
      return;
    }

    const container = options.chatBodyRef.value;
    if (!container) {
      return;
    }

    options.loadingOlderMessages.value = true;

    try {
      const previousHeight = container.scrollHeight;
      const previousTop = container.scrollTop;
      const oldestMessage = options.messages.value[0];
      if (!oldestMessage) {
        return;
      }

      const oldestMessageId = oldestMessage.id;

      const response = await fetchMessagesPage(options.activeConversationId.value, oldestMessageId);
      options.messages.value = options.mergeMessages(response.messages, options.messages.value);
      options.hasMoreMessages.value = response.hasMore;

      await nextTick();

      const nextHeight = container.scrollHeight;
      const diff = nextHeight - previousHeight;
      container.scrollTop = previousTop + diff;
    } finally {
      options.loadingOlderMessages.value = false;
    }
  }

  return {
    fetchMessagesPage,
    hydrateRealtimeMediaMessage,
    loadConversations,
    loadConversationMessages,
    refreshConversationMessages,
    syncConversationHistory,
    loadGroupParticipants,
    loadOlderMessages,
    scheduleGroupParticipantsRefresh,
    updateConversationCacheFromMessage
  };
}
