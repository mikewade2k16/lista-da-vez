import { computed, effectScope, ref } from "vue";
import { describe, expect, it, vi } from "vitest";
import { ApiClientError } from "~/composables/useApi";
import type { Conversation, Message } from "~/types";
import { useOmnichannelInboxContactActions } from "./useOmnichannelInboxContactActions";
import { useOmnichannelInboxDerivedState } from "./useOmnichannelInboxDerivedState";
import { useOmnichannelInboxHistory } from "./useOmnichannelInboxHistory";
import { useOmnichannelInboxOutboundPipeline } from "./useOmnichannelInboxOutboundPipeline";

describe("compose seguro depois de reset", () => {
  it("mantem o placeholder fora da lista no 404 e o publica somente depois do primeiro envio", async () => {
    const now = "2026-08-28T10:00:00.000Z";
    const placeholder: Conversation = {
      id: "conversation-reset",
      instanceId: "instance-1",
      instanceScopeKey: "instance-1",
      instanceName: "instance-1",
      instanceDisplayName: "Principal",
      channel: "WHATSAPP",
      status: "OPEN",
      externalId: "5511999999999@s.whatsapp.net",
      contactId: "contact-1",
      contactName: "Contato",
      contactAvatarUrl: null,
      contactPhone: "5511999999999",
      assignedToId: null,
      createdAt: now,
      updatedAt: now,
      lastMessageAt: now,
      lastMessage: null
    };
    const createdMessage: Message = {
      id: "message-1",
      tenantId: "tenant-1",
      conversationId: placeholder.id,
      senderUserId: "user-1",
      direction: "OUTBOUND",
      messageType: "TEXT",
      senderName: "Agente",
      senderAvatarUrl: null,
      content: "primeira mensagem",
      mediaUrl: null,
      mediaMimeType: null,
      mediaFileName: null,
      mediaFileSizeBytes: null,
      mediaCaption: null,
      mediaDurationSeconds: null,
      metadataJson: null,
      status: "SENT",
      origin: "human",
      replyTo: null,
      providerStatusAt: null,
      providerErrorCode: "",
      mediaState: "",
      canRetryMedia: false,
      externalMessageId: "provider-1",
      createdAt: now,
      updatedAt: now
    };
    const conversations = ref<Conversation[]>([]);
    const composeConversation = ref<Conversation | null>(null);
    const activeConversationId = ref<string | null>(null);
    const messages = ref<Message[]>([]);
    const visibleMessagesConversationId = ref<string | null>(null);
    const apiFetch = vi.fn(async (path: string, init?: Record<string, unknown>) => {
      if (path === "/contacts/contact-1/open-conversation") return placeholder;
      if (path.includes("/messages?") && init?.method !== "POST") {
        throw new ApiClientError("not found", { statusCode: 404 });
      }
      if (path === `/conversations/${placeholder.id}/messages` && init?.method === "POST") {
        return createdMessage;
      }
      throw new Error(`unexpected ${path}`);
    });
    const scope = effectScope();
    const history = scope.run(() => useOmnichannelInboxHistory({
      apiFetch: apiFetch as never,
      conversations,
      messages,
      visibleMessagesConversationId,
      activeConversationId,
      selectedInstanceId: ref("all"), whatsappInstanceAccessState: ref("resolved-nonempty"),
      search: ref(""), channel: ref("all"), status: ref("all"),
      loadingWhatsAppStatus: ref(false), isWhatsAppConfigured: ref(false), isWhatsAppConnected: ref(false),
      loadingConversations: ref(false), loadingMoreConversations: ref(false), loadingMessages: ref(false),
      loadingOlderMessages: ref(false), loadingGroupParticipants: ref(false), conversationsError: ref(""),
      messagesError: ref(""), hasMoreConversations: ref(false), nextConversationsCursor: ref(null),
      hasMoreMessages: ref(false), chatBodyRef: ref(null), mentionAlertState: ref({}),
      groupParticipantsByConversation: {}, realtimeMessageHydrationLocks: new Set(),
      groupParticipantsRefreshAtByConversation: new Map(), groupParticipantsInFlightByConversation: new Set(),
      historySyncAttemptAtByConversation: new Map(), historySyncInFlightByConversation: new Set(),
      sortConversations: vi.fn(), bootstrapReadState: vi.fn(), getReadAt: () => null,
      getSelectConversation: () => null, normalizeMessage: (entry) => entry,
      mergeMessages: (...chunks) => chunks.flat(), updateConversationPreviewFromMessage: vi.fn(),
      messageNeedsMediaHydration: () => false,
      isComposeOnlyConversation: (id) => composeConversation.value?.id === id
    }));
    if (!history) throw new Error("history unavailable");

    const derived = useOmnichannelInboxDerivedState({
      user: ref({ id: "user-1", role: "ADMIN", tenantId: "tenant-1" }), users: ref([]),
      effectivePermissionKeys: ref(["omnichannel.conversations.reply"]),
      whatsappInstances: ref([]), whatsappInstanceAccessState: ref("resolved-nonempty"),
      conversations, composeConversation, contacts: ref([]), messages,
      activeConversationId, search: ref(""), channel: ref("all"), status: ref("all"), instanceId: ref("all"),
      attachment: ref(null), tenantMaxUploadMb: ref(10), whatsappStatus: ref(null),
      loadingWhatsAppStatus: ref(false), notesByConversation: {}, groupParticipantsByConversation: {},
      readState: ref({}), mentionAlertState: ref({}), isConversationUnread: () => false
    });
    const contactActions = useOmnichannelInboxContactActions({
      activeConversationId, canSaveActiveContact: computed(() => true), savingContact: ref(false),
      creatingContact: ref(false), contactActionError: ref(""), sidebarView: ref("contacts"),
      apiFetch: apiFetch as never, upsertContact: vi.fn(), upsertConversation: vi.fn(),
      setComposeConversation: (entry) => { composeConversation.value = entry; }, syncSavedContactIntoMessages: vi.fn(),
      loadContacts: vi.fn(), loadConversations: vi.fn(), selectConversation: async (id) => {
        activeConversationId.value = id;
        await history.loadConversationMessages(id, { forceRefresh: true });
      }
    });

    await contactActions.openContactConversation("contact-1");
    expect(conversations.value).toEqual([]);
    expect(activeConversationId.value).toBe(placeholder.id);
    expect(derived.activeConversation.value?.id).toBe(placeholder.id);

    const onCreated = vi.fn(async () => {
      conversations.value = [{ ...placeholder, lastMessage: createdMessage }];
      composeConversation.value = null;
    });
    const outbound = useOmnichannelInboxOutboundPipeline({
      publicApiBase: "", token: ref("token"), tenantSlug: ref("tenant"), shellClientId: ref(0),
      user: ref({ id: "user-1", tenantId: "tenant-1", name: "Agente" }),
      canManageConversation: computed(() => true), isGroupConversation: computed(() => false),
      activeConversationId, draft: ref("primeira mensagem"), selectedAIReplyDraftId: ref("draft-1"),
      attachment: ref(null), replyTarget: ref(null),
      pendingSendCount: ref(0), sendError: ref(""), messages, apiFetch: apiFetch as never,
      buildReplyMetadata: () => ({}), buildOutboundLinkPreviewMetadata: () => null,
      findGroupParticipantForMention: () => null, createOptimisticMessage: () => ({ ...createdMessage, id: "optimistic" }),
      normalizeMessage: (entry) => entry, mergeMessages: (...chunks) => [...new Map(chunks.flat().map((m) => [m.id, m])).values()],
      updateConversationPreviewFromMessage: vi.fn(), clearAttachment: vi.fn(), scrollToBottom: vi.fn(),
      markConversationAsRead: vi.fn(), scheduleStickyDateRefresh: vi.fn(), reconcilePendingMessageStatus: vi.fn(),
      onAuthoritativeMessageCreated: onCreated
    });
    await outbound.sendMessage();

    expect(onCreated).toHaveBeenCalledWith(placeholder.id);
    expect(apiFetch).toHaveBeenCalledWith(
      `/conversations/${placeholder.id}/messages`,
      expect.objectContaining({ body: expect.objectContaining({ aiReplyDraftId: "draft-1" }) })
    );
    expect(conversations.value.map((entry) => entry.id)).toEqual([placeholder.id]);
    expect(messages.value.some((entry) => entry.id === createdMessage.id)).toBe(true);
    scope.stop();
  });
});
