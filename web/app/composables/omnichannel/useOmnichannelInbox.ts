import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import type { InboxSelectOption } from "~/components/omnichannel/inbox/types";
import type {
  Contact,
  Conversation,
  Message,
  MessageType,
} from "~/types";
import { useOmnichannelInboxRealtime } from "~/composables/omnichannel/useOmnichannelInboxRealtime";
import { useOmnichannelInboxHistory } from "~/composables/omnichannel/useOmnichannelInboxHistory";
import { useOmnichannelInboxOutboundPipeline } from "~/composables/omnichannel/useOmnichannelInboxOutboundPipeline";
import { useOmnichannelInboxReadState } from "~/composables/omnichannel/useOmnichannelInboxReadState";
import { useOmnichannelInboxMentionAlerts } from "~/composables/omnichannel/useOmnichannelInboxMentionAlerts";
import { useOmnichannelInboxMessageReactions } from "~/composables/omnichannel/useOmnichannelInboxMessageReactions";
import { useOmnichannelInboxDerivedState } from "~/composables/omnichannel/useOmnichannelInboxDerivedState";
import { useOmnichannelInboxContactActions } from "~/composables/omnichannel/useOmnichannelInboxContactActions";
import { useOmnichannelInboxConversationActions } from "~/composables/omnichannel/useOmnichannelInboxConversationActions";
import { useOmnichannelInboxBootstrapLoaders } from "~/composables/omnichannel/useOmnichannelInboxBootstrapLoaders";
import { useOmnichannelInboxPendingStatus } from "~/composables/omnichannel/useOmnichannelInboxPendingStatus";
import { useOmnichannelInboxScroll } from "~/composables/omnichannel/useOmnichannelInboxScroll";
import { useOmnichannelInboxState } from "~/composables/omnichannel/useOmnichannelInboxState";
import { useOmnichannelInboxStateMutators } from "~/composables/omnichannel/useOmnichannelInboxStateMutators";
import { useOmnichannelInboxMessageActions } from "~/composables/omnichannel/useOmnichannelInboxMessageActions";
import { useInboxSyncGuard } from "~/composables/omnichannel/useInboxSyncGuard";
import { useInboxMessageWindow } from "~/composables/omnichannel/useInboxMessageWindow";
import { usePageBootstrapLoading } from "~/composables/usePageBootstrapLoading";
import {
  type AttachmentPickerMode,
  asRecord,
  formatSendError,
  isMediaMessageType,
  MEDIA_MESSAGE_TYPES,
  mediaPlaceholderByType,
  type MentionOpenPayload,
  normalizeComparableJid,
  normalizeMentionJid,
  normalizePhoneDigits,
  type OptimisticMessageOptions,
  type OutboundAttachment,
  resolveAttachmentType,
  resolveMessageType,
  toArrayOrEmpty,
  UNASSIGNED_VALUE,
  extractFirstOutboundLinkUrl
} from "~/composables/omnichannel/useOmnichannelInboxShared";

export function useOmnichannelInbox() {
  const config = useRuntimeConfig();
  const { user, token, tenantSlug, logout: performLogout, syncSessionFromToken } = useAdminSession();
  const sessionSimulation = useSessionSimulationStore();
  const { apiFetch } = useApi();
  const shellClientId = computed(() => {
    if (!sessionSimulation.canSimulate) {
      return 0;
    }

    const resolved = Number(sessionSimulation.effectiveClientId || sessionSimulation.clientId || 0);
    return Number.isFinite(resolved) && resolved > 0 ? resolved : 0;
  });

  const {
    conversations,
    contacts,
    users,
    whatsappInstances,
    messages,
    visibleMessagesConversationId,
    activeConversationId,
    leftCollapsed,
    rightCollapsed,
    showFilters,
    sidebarView,
    loadingConversations,
    loadingContacts,
    loadingUsers,
    loadingWhatsAppStatus,
    loadingMessages,
    loadingOlderMessages,
    loadingGroupParticipants,
    savingContact,
    creatingContact,
    pendingSendCount,
    sendingMessage,
    sendError,
    contactActionError,
    updatingStatus,
    updatingAssignee,
    hasMoreMessages,
    showLoadOlderMessagesButton,
    showScrollToLatestButton,
    stickyDateLabel,
    showStickyDate,
    draft,
    search,
    channel,
    status,
    instanceId,
    replyTarget,
    attachment,
    tenantMaxUploadMb,
    assigneeModel,
    whatsappStatus,
    notesByConversation,
    groupParticipantsByConversation,
    readState,
    mentionAlertState,
    chatBodyRef
  } = useOmnichannelInboxState();
  const {
    pageLoading: pageBootstrapping,
    runPageBootstrap,
    startPageBootstrap,
    finishPageBootstrap
  } = usePageBootstrapLoading({
    minimumDelayMs: 700,
    initiallyLoading: true
  });
  const realtimeMessageHydrationLocks = new Set<string>();
  const groupParticipantsRefreshAtByConversation = new Map<string, number>();
  const groupParticipantsInFlightByConversation = new Set<string>();
  const historySyncAttemptAtByConversation = new Map<string, number>();
  const historySyncInFlightByConversation = new Set<string>();
  let selectConversationHandler: ((conversationId: string) => Promise<void>) | null = null;

  const channelFilterItems: InboxSelectOption[] = [
    { label: "Todos canais", value: "all" },
    { label: "WhatsApp", value: "WHATSAPP" },
    { label: "Instagram", value: "INSTAGRAM" }
  ];

  const statusFilterItems: InboxSelectOption[] = [
    { label: "Todos status", value: "all" },
    { label: "Abertos", value: "OPEN" },
    { label: "Pendentes", value: "PENDING" },
    { label: "Encerrados", value: "CLOSED" }
  ];

  const statusActionItems: InboxSelectOption[] = [
    { label: "Aberto", value: "OPEN" },
    { label: "Pendente", value: "PENDING" },
    { label: "Encerrado", value: "CLOSED" }
  ];

  const readStateStorageKey = computed(() => {
    return `omni:read-state:${user.value?.tenantId ?? "tenant"}:${user.value?.id ?? "user"}`;
  });
  const outboundOperatorLabelStorageKey = computed(() => {
    return `omni:outbound-operator-label:${user.value?.tenantId ?? "tenant"}:${user.value?.id ?? "user"}`;
  });
  const instanceFilterStorageKey = computed(() => {
    return `omni:instance-filter:${user.value?.tenantId ?? "tenant"}:${user.value?.id ?? "user"}`;
  });
  const showOutboundOperatorLabel = ref(true);

  function loadOutboundOperatorLabelPreference() {
    if (!import.meta.client) {
      return;
    }

    const raw = localStorage.getItem(outboundOperatorLabelStorageKey.value);
    if (raw === "0") {
      showOutboundOperatorLabel.value = false;
      return;
    }

    if (raw === "1") {
      showOutboundOperatorLabel.value = true;
      return;
    }

    showOutboundOperatorLabel.value = true;
  }

  function updateShowOutboundOperatorLabel(value: boolean) {
    showOutboundOperatorLabel.value = Boolean(value);
    if (!import.meta.client) {
      return;
    }

    localStorage.setItem(outboundOperatorLabelStorageKey.value, showOutboundOperatorLabel.value ? "1" : "0");
  }

  function sanitizeSelectedInstanceId(candidate?: string | null) {
    const normalized = String(candidate ?? "").trim();
    if (!normalized || normalized === "all") {
      return "all";
    }

    return whatsappInstances.value.some((entry) => entry.id === normalized) ? normalized : "all";
  }

  function loadInstanceFilterPreference() {
    if (!import.meta.client) {
      instanceId.value = sanitizeSelectedInstanceId(instanceId.value);
      return;
    }

    const raw = localStorage.getItem(instanceFilterStorageKey.value);
    instanceId.value = sanitizeSelectedInstanceId(raw);
  }

  function persistInstanceFilterPreference(value: string) {
    instanceId.value = sanitizeSelectedInstanceId(value);
    if (!import.meta.client) {
      return;
    }

    localStorage.setItem(instanceFilterStorageKey.value, instanceId.value);
  }

  function ensureAuthOrRedirect() {
    if (!token.value) {
      void navigateTo("/admin/login");
    }
  }

  function messageRowId(messageId: string) {
    return `chat-message-${messageId}`;
  }

  function isStickerMessage(messageEntry: Message) {
    if (resolveMessageType(messageEntry) !== "IMAGE") {
      return false;
    }

    const metadata = asRecord(messageEntry.metadataJson);
    if (!metadata) {
      return false;
    }

    const mediaPayloadKey = typeof metadata.mediaPayloadKey === "string" ? metadata.mediaPayloadKey.trim() : "";
    if (mediaPayloadKey === "stickerMessage") {
      return true;
    }

    const mediaKind = typeof metadata.mediaKind === "string" ? metadata.mediaKind.trim().toLowerCase() : "";
    if (mediaKind === "sticker") {
      return true;
    }

    return Boolean(asRecord(metadata.sticker));
  }

  function messageNeedsMediaHydration(messageEntry: Message) {
    const messageType = resolveMessageType(messageEntry);
    if (!isMediaMessageType(messageType)) {
      return false;
    }

    if (typeof messageEntry.mediaUrl === "string" && messageEntry.mediaUrl.trim().length > 0) {
      return false;
    }

    const metadata = asRecord(messageEntry.metadataJson);
    if (!metadata) {
      return true;
    }

    const hasMediaUrl = metadata.hasMediaUrl;
    if (typeof hasMediaUrl === "boolean") {
      return hasMediaUrl;
    }

    return true;
  }

  function normalizeMessage(messageEntry: Message): Message {
    return {
      ...messageEntry,
      senderUserId: messageEntry.senderUserId ?? null,
      messageType: resolveMessageType(messageEntry),
      mediaUrl: messageEntry.mediaUrl ?? null,
      mediaMimeType: messageEntry.mediaMimeType ?? null,
      mediaFileName: messageEntry.mediaFileName ?? null,
      mediaFileSizeBytes: messageEntry.mediaFileSizeBytes ?? null,
      mediaCaption: messageEntry.mediaCaption ?? null,
      mediaDurationSeconds: messageEntry.mediaDurationSeconds ?? null,
      metadataJson: messageEntry.metadataJson ?? null
    };
  }

  function revokeAttachmentPreview() {
    const url = attachment.value?.previewUrl;
    if (url) {
      URL.revokeObjectURL(url);
    }
  }

  function clearAttachment(options?: { revokePreview?: boolean }) {
    if (options?.revokePreview !== false) {
      revokeAttachmentPreview();
    }
    attachment.value = null;
  }

  function normalizeNameForComparison(value: string | null | undefined) {
    return (
      value
        ?.normalize("NFKD")
        .replace(/[^\w\s]/g, "")
        .trim()
        .toLowerCase() ?? ""
    );
  }

  function isTechnicalWhatsAppIdentifier(value: string | null | undefined) {
    const normalized = value?.trim().toLowerCase() ?? "";
    if (!normalized) {
      return false;
    }

    if (
      normalized.endsWith("@s.whatsapp.net") ||
      normalized.endsWith("@g.us") ||
      normalized.endsWith("@lid")
    ) {
      return true;
    }

    const compact = normalized.replace(/\s+/g, "");
    return /^\+?\d{7,20}$/.test(compact);
  }

  function setAttachmentFromFile(file: File | null, mode: AttachmentPickerMode, durationSeconds?: number | null) {
    clearAttachment();

    if (!file) {
      return;
    }

    if (file.size > maxAttachmentSizeBytes.value) {
      sendError.value = `Arquivo acima do limite do tenant (${tenantMaxUploadMb.value}MB por arquivo).`;
      return;
    }

    const type = resolveAttachmentType(file, mode);
    const previewUrl =
      type === "IMAGE" || type === "VIDEO" || type === "AUDIO"
        ? URL.createObjectURL(file)
        : null;

    attachment.value = {
      file,
      type,
      durationSeconds: typeof durationSeconds === "number" && Number.isFinite(durationSeconds)
        ? Math.max(0, Math.floor(durationSeconds))
        : null,
      previewUrl,
      sendAsSticker: mode === "sticker",
      sendAsVoiceNote: mode === "voice"
    };
    sendError.value = "";
  }

  function buildReplyMetadata(messageEntry: Message) {
    const type = resolveMessageType(messageEntry);
    const fallbackByType: Record<MessageType, string> = {
      TEXT: "",
      IMAGE: "[imagem]",
      AUDIO: "[audio]",
      VIDEO: "[video]",
      DOCUMENT: "[documento]"
    };

    const content = messageEntry.content?.trim() || fallbackByType[type];
    const conversationEntry = conversations.value.find((entry) => entry.id === messageEntry.conversationId) ?? null;
    const fallbackPhone =
      conversationEntry?.contactPhone?.trim() ||
      conversationEntry?.externalId?.replace(/\D/g, "") ||
      "";
    const fallbackAuthor = conversationEntry?.contactName?.trim() || fallbackPhone || "Contato";
    const senderName = messageEntry.senderName?.trim() || "";
    const normalizedSender = normalizeNameForComparison(senderName);
    const normalizedCurrentUser = normalizeNameForComparison(user.value?.name ?? "");
    const senderLooksCurrentUser =
      Boolean(normalizedSender) &&
      (
        normalizedSender === normalizedCurrentUser ||
        ["voce", "atendente", "agente", "agent", "operator", "operador"].includes(normalizedSender)
      );
    const senderLooksTechnical = isTechnicalWhatsAppIdentifier(senderName);
    const inboundReplyAuthor = senderName && !senderLooksCurrentUser
      && !senderLooksTechnical
      ? senderName
      : fallbackAuthor;

    return {
      messageId: messageEntry.id,
      externalMessageId: messageEntry.externalMessageId?.trim() || null,
      author: messageEntry.direction === "OUTBOUND" ? "Voce" : inboundReplyAuthor,
      content: content.slice(0, 280),
      messageType: type,
      mediaUrl: messageEntry.mediaUrl ?? null,
      fromMe: messageEntry.direction === "OUTBOUND",
      authorJid: messageEntry.direction === "INBOUND"
        ? (typeof messageEntry.metadataJson?.participantJid === "string"
          ? messageEntry.metadataJson.participantJid
          : null)
        : null
    };
  }

  function findGroupParticipantForMention(jid: string) {
    const normalizedJid = normalizeMentionJid(jid);
    const mentionDigits = normalizePhoneDigits(jid);

    return activeGroupParticipants.value.find((participant) => {
      const participantJid = normalizeMentionJid(participant.jid);
      const participantPhone = normalizePhoneDigits(participant.phone);

      if (normalizedJid && participantJid && participantJid === normalizedJid) {
        return true;
      }

      if (mentionDigits && participantPhone) {
        return participantPhone.endsWith(mentionDigits) || mentionDigits.endsWith(participantPhone);
      }

      return false;
    }) ?? null;
  }

  function buildOutboundLinkPreviewMetadata(value: string, enabled: boolean) {
    const url = extractFirstOutboundLinkUrl(value);
    if (!url) {
      return null;
    }

    let host: string | null = null;
    try {
      host = new URL(url).hostname.replace(/^www\./i, "");
    } catch {
      host = null;
    }

    return {
      enabled,
      url,
      title: host || url,
      source: "composer"
    };
  }

  function createOptimisticMessage(options: OptimisticMessageOptions): Message {
    const nowIso = new Date().toISOString();
    const generatedId = `temp-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const type = options.attachment?.type ?? "TEXT";
    const normalizedText = options.text.trim();
    const isSticker = Boolean(options.attachment?.sendAsSticker);
    const placeholderContent = isSticker ? "[figurinha]" : mediaPlaceholderByType(type);
    const metadataJson: Record<string, unknown> = {};

    if (options.reply) {
      metadataJson.reply = buildReplyMetadata(options.reply);
    }

    if (isSticker) {
      metadataJson.media = {
        sendAsSticker: true,
        kind: "sticker"
      };
      metadataJson.sticker = {
        source: "composer"
      };
    }

    if (options.attachment?.sendAsVoiceNote) {
      metadataJson.media = {
        ...(asRecord(metadataJson.media) ?? {}),
        sendAsVoiceNote: true,
        kind: "voice"
      };
    }

    return normalizeMessage({
      id: generatedId,
      tenantId: user.value?.tenantId ?? "temp-tenant",
      conversationId: options.conversationId,
      senderUserId: user.value?.id ?? null,
      direction: "OUTBOUND",
      messageType: type,
      senderName: user.value?.name ?? "Voce",
      senderAvatarUrl: null,
      content: normalizedText || placeholderContent,
      mediaUrl: options.attachment?.previewUrl ?? null,
      mediaMimeType: options.attachment?.file.type || null,
      mediaFileName: options.attachment?.file.name ?? null,
      mediaFileSizeBytes: options.attachment?.file.size ?? null,
      mediaCaption: isSticker ? null : (normalizedText || null),
      mediaDurationSeconds: options.attachment?.type === "AUDIO"
        ? (options.attachment.durationSeconds ?? null)
        : null,
      metadataJson: Object.keys(metadataJson).length > 0 ? metadataJson : null,
      status: "PENDING",
      externalMessageId: null,
      createdAt: nowIso,
      updatedAt: nowIso
    });
  }

  function sortConversations() {
    const conversationsList = toArrayOrEmpty<Conversation>(conversations.value);
    conversations.value = conversationsList.sort((left, right) => {
      return Number(new Date(right.lastMessageAt)) - Number(new Date(left.lastMessageAt));
    });
  }

  function sortContacts() {
    const contactsList = toArrayOrEmpty<Contact>(contacts.value);
    contacts.value = contactsList.sort((left, right) => {
      const leftTime = Number(new Date(left.lastConversationAt || left.updatedAt));
      const rightTime = Number(new Date(right.lastConversationAt || right.updatedAt));
      return rightTime - leftTime;
    });
  }

  function upsertContact(contactEntry: Contact) {
    const index = contacts.value.findIndex((item) => item.id === contactEntry.id);
    if (index >= 0) {
      contacts.value[index] = {
        ...contacts.value[index],
        ...contactEntry
      };
    } else {
      contacts.value.push(contactEntry);
    }

    sortContacts();
  }

  function syncSavedContactIntoMessages(contactEntry: Contact) {
    const normalizedPhone = normalizePhoneDigits(contactEntry.phone);
    if (!normalizedPhone) {
      return;
    }

    messages.value = messages.value.map((messageEntry) => {
      const metadata = asRecord(messageEntry.metadataJson);
      const directContact = metadata ? asRecord(metadata.contact) : null;
      const firstArrayContact = metadata ? asRecord(toArrayOrEmpty(metadata.contacts)[0]) : null;
      const legacyContentMatch = messageEntry.content?.match(/^Contato:\s*(.+?)(?:\s+\(([^)]+)\))?$/i) ?? null;
      const fallbackLegacyPhone = legacyContentMatch?.[2]?.trim() ?? "";
      const fallbackLegacyName = legacyContentMatch?.[1]?.trim() ?? "";
      const hasVcardPayload =
        (typeof directContact?.vcard === "string" && directContact.vcard.trim().length > 0) ||
        (typeof firstArrayContact?.vcard === "string" && firstArrayContact.vcard.trim().length > 0);
      const isContactCardTextMessage =
        resolveMessageType(messageEntry) === "TEXT" &&
        (Boolean(legacyContentMatch) || hasVcardPayload);

      if (!isContactCardTextMessage) {
        return messageEntry;
      }

      const contactPhone = normalizePhoneDigits(
        typeof directContact?.phone === "string"
          ? directContact.phone
          : typeof directContact?.phoneNumber === "string"
            ? directContact.phoneNumber
            : typeof directContact?.number === "string"
              ? directContact.number
              : fallbackLegacyPhone || null
      );

      if (!contactPhone || contactPhone !== normalizedPhone) {
        return messageEntry;
      }

      const normalizedContactNode = {
        ...(directContact ?? {}),
        name: contactEntry.name || fallbackLegacyName || contactEntry.phone,
        displayName: contactEntry.name,
        fullName: contactEntry.name,
        phone: contactEntry.phone,
        phoneNumber: contactEntry.phone,
        number: contactEntry.phone,
        avatarUrl: contactEntry.avatarUrl,
        contactId: contactEntry.id
      };

      return normalizeMessage({
        ...messageEntry,
        metadataJson: {
          ...metadata,
          contact: normalizedContactNode,
          contacts: [normalizedContactNode]
        }
      });
    });
  }

  function mergeMessages(...chunks: Message[][]) {
    const map = new Map<string, Message>();

    for (const chunk of chunks) {
      for (const messageEntry of chunk) {
        const previous = map.get(messageEntry.id);
        const nextMessage = normalizeMessage(messageEntry);
        const mergedMessage = previous
          ? normalizeMessage({
            ...previous,
            ...nextMessage,
            mediaUrl: nextMessage.mediaUrl ?? previous.mediaUrl ?? null,
            mediaMimeType: nextMessage.mediaMimeType ?? previous.mediaMimeType ?? null,
            mediaFileName: nextMessage.mediaFileName ?? previous.mediaFileName ?? null,
            mediaFileSizeBytes: nextMessage.mediaFileSizeBytes ?? previous.mediaFileSizeBytes ?? null,
            mediaCaption: nextMessage.mediaCaption ?? previous.mediaCaption ?? null,
            mediaDurationSeconds: nextMessage.mediaDurationSeconds ?? previous.mediaDurationSeconds ?? null,
            metadataJson: nextMessage.metadataJson ?? previous.metadataJson ?? null
          })
          : nextMessage;
        map.set(messageEntry.id, {
          ...mergedMessage
        });
      }
    }

    return [...map.values()].sort((left, right) => {
      const leftTime = Number(new Date(left.createdAt));
      const rightTime = Number(new Date(right.createdAt));

      if (leftTime !== rightTime) {
        return leftTime - rightTime;
      }

      return left.id.localeCompare(right.id);
    });
  }

  const {
    incrementMentionAlert,
    clearMentionAlert,
    shouldFlagMentionAlert
  } = useOmnichannelInboxMentionAlerts({
    conversations,
    mentionAlertState
  });

  const {
    getReadAt,
    loadReadState,
    saveReadState,
    bootstrapReadState,
    markConversationAsRead,
    isConversationUnread
  } = useOmnichannelInboxReadState({
    readState,
    activeConversationId,
    messages,
    conversations,
    readStateStorageKey,
    clearMentionAlert
  });

  const {
    assigneeItems,
    instanceFilterItems,
    activeConversation,
    activeConversationLabel,
    filteredContacts,
    isGroupConversation,
    canManageConversation,
    canSaveActiveContact,
    activeGroupParticipants,
    whatsappConnectionState,
    isWhatsAppConfigured,
    isWhatsAppConnected,
    whatsappBannerMessage,
    internalNotes,
    hasAttachment,
    attachmentType,
    attachmentName,
    attachmentMimeType,
    attachmentSizeBytes,
    attachmentDurationSeconds,
    attachmentPreviewUrl,
    maxAttachmentSizeBytes,
    filteredConversations,
    unreadConversationIds,
    mentionConversationIds,
    mentionConversationCounts,
    firstUnreadMessageId,
    messageRenderItems
  } = useOmnichannelInboxDerivedState({
    user,
    users,
    whatsappInstances,
    conversations,
    contacts,
    messages,
    activeConversationId,
    search,
    channel,
    status,
    instanceId,
    attachment,
    tenantMaxUploadMb,
    whatsappStatus,
    loadingWhatsAppStatus,
    notesByConversation,
    groupParticipantsByConversation,
    readState,
    mentionAlertState,
    isConversationUnread
  });

  function upsertConversation(conversationEntry: Conversation) {
    const index = conversations.value.findIndex((item) => item.id === conversationEntry.id);
    if (index >= 0) {
      conversations.value[index] = {
        ...conversations.value[index],
        ...conversationEntry
      };
    } else {
      conversations.value.push(conversationEntry);
    }

    sortConversations();

    if (activeConversationId.value === conversationEntry.id) {
      assigneeModel.value = conversationEntry.assignedToId ?? UNASSIGNED_VALUE;
    }

    if (conversationEntry.contactId) {
      const linkedContact = contacts.value.find((item) => item.id === conversationEntry.contactId);
      if (linkedContact) {
        upsertContact({
          ...linkedContact,
          name: conversationEntry.contactName || linkedContact.name,
          phone: conversationEntry.contactPhone || linkedContact.phone,
          avatarUrl: conversationEntry.contactAvatarUrl ?? linkedContact.avatarUrl,
          lastConversationId: conversationEntry.id,
          lastConversationAt: conversationEntry.lastMessageAt,
          lastConversationChannel: conversationEntry.channel,
          lastConversationStatus: conversationEntry.status
        });
      }
    }
  }

  function updateConversationPreviewFromMessage(messageEntry: Message) {
    const conversationEntry = conversations.value.find((item) => item.id === messageEntry.conversationId);
    if (!conversationEntry) {
      return;
    }

    const normalizedMessage = normalizeMessage(messageEntry);
    const incomingMessageAt = Number(new Date(normalizedMessage.createdAt));
    const currentLastMessageAt = Number(new Date(conversationEntry.lastMessageAt));
    const incomingMessageAtSafe = Number.isFinite(incomingMessageAt) ? incomingMessageAt : 0;
    const currentLastMessageAtSafe = Number.isFinite(currentLastMessageAt) ? currentLastMessageAt : 0;
    const previousLastMessageId = conversationEntry.lastMessage?.id ?? null;
    const isCurrentPreviewMessage = previousLastMessageId === normalizedMessage.id;
    const shouldAdvancePreview = incomingMessageAtSafe >= currentLastMessageAtSafe;

    // Regras de negocio: historico antigo nunca pode substituir preview/topo da inbox.
    if (!shouldAdvancePreview && !isCurrentPreviewMessage) {
      return;
    }

    const previewContent =
      normalizedMessage.content?.trim() ||
      (normalizedMessage.messageType !== "TEXT"
        ? normalizedMessage.mediaCaption?.trim() ||
        (normalizedMessage.messageType === "IMAGE"
          ? isStickerMessage(normalizedMessage)
            ? "[figurinha]"
            : "[imagem]"
          : normalizedMessage.messageType === "AUDIO"
            ? "[audio]"
            : normalizedMessage.messageType === "VIDEO"
              ? "[video]"
              : "[documento]")
        : "");

    conversationEntry.lastMessageAt = shouldAdvancePreview
      ? normalizedMessage.createdAt
      : conversationEntry.lastMessageAt;
    conversationEntry.status = "OPEN";
    conversationEntry.lastMessage = {
      id: normalizedMessage.id,
      content: previewContent,
      messageType: normalizedMessage.messageType,
      mediaUrl: normalizedMessage.mediaUrl,
      direction: normalizedMessage.direction,
      status: normalizedMessage.status,
      createdAt: shouldAdvancePreview
        ? normalizedMessage.createdAt
        : (conversationEntry.lastMessage?.createdAt ?? normalizedMessage.createdAt)
    };

    if (shouldAdvancePreview && previousLastMessageId !== normalizedMessage.id) {
      sortConversations();
    }
  }

  function conversationMatchesMentionTarget(
    conversationEntry: Conversation,
    targetJid: string | null,
    targetPhone: string | null
  ) {
    if (conversationEntry.channel !== "WHATSAPP") {
      return false;
    }

    const conversationJid = normalizeComparableJid(conversationEntry.externalId);
    const conversationPhone = normalizePhoneDigits(conversationEntry.externalId);

    if (targetJid && conversationJid && conversationJid === targetJid) {
      return true;
    }

    if (targetPhone && conversationPhone) {
      if (conversationPhone.endsWith(targetPhone) || targetPhone.endsWith(conversationPhone)) {
        return true;
      }
    }

    return false;
  }

  async function openMentionConversation(payload: MentionOpenPayload) {
    const targetJid = normalizeComparableJid(payload.jid) ?? normalizeComparableJid(payload.phone);
    const targetPhone = normalizePhoneDigits(payload.phone) ?? normalizePhoneDigits(payload.jid);

    if (!targetJid && !targetPhone) {
      return;
    }

    const existingConversation = conversations.value.find((conversationEntry) =>
      conversationMatchesMentionTarget(conversationEntry, targetJid, targetPhone)
    );

    if (existingConversation) {
      await selectConversation(existingConversation.id);
      return;
    }

    if (!canManageConversation.value) {
      sendError.value = "Seu perfil nao pode criar conversa a partir de mencao.";
      return;
    }

    const externalId =
      (targetJid && targetJid.endsWith("@s.whatsapp.net") ? targetJid : null) ??
      (targetPhone ? `${targetPhone}@s.whatsapp.net` : null) ??
      (targetJid && targetJid.includes("@lid") ? targetJid : null) ??
      targetJid;
    if (!externalId) {
      return;
    }

    try {
      const createdConversation = await apiFetch<Conversation>("/conversations", {
        method: "POST",
        body: {
          channel: "WHATSAPP",
          externalId,
          contactName: payload.label?.trim() || undefined,
          contactPhone: targetPhone || undefined
        }
      });

      upsertConversation(createdConversation);
      await selectConversation(createdConversation.id);
    } catch (error) {
      sendError.value = formatSendError(error, "Nao foi possivel abrir a conversa da mencao.");
    }
  }

  const {
    loadTenantUploadLimit,
    loadWhatsAppStatus,
    loadUsers,
    loadContacts,
    loadAccessibleWhatsAppInstances
  } = useOmnichannelInboxBootstrapLoaders({
    tenantMaxUploadMb,
    whatsappStatus,
    users,
    contacts,
    whatsappInstances,
    selectedInstanceId: instanceId,
    loadingWhatsAppStatus,
    loadingUsers,
    loadingContacts,
    apiFetch,
    sortContacts
  });

  const {
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
  } = useOmnichannelInboxHistory({
    apiFetch,
    conversations,
    messages,
    visibleMessagesConversationId,
    activeConversationId,
    selectedInstanceId: instanceId,
    loadingWhatsAppStatus,
    isWhatsAppConfigured,
    isWhatsAppConnected,
    loadingConversations,
    loadingMessages,
    loadingOlderMessages,
    loadingGroupParticipants,
    hasMoreMessages,
    chatBodyRef,
    mentionAlertState,
    groupParticipantsByConversation,
    realtimeMessageHydrationLocks,
    groupParticipantsRefreshAtByConversation,
    groupParticipantsInFlightByConversation,
    historySyncAttemptAtByConversation,
    historySyncInFlightByConversation,
    sortConversations,
    bootstrapReadState,
    getReadAt,
    getSelectConversation: () => selectConversationHandler,
    normalizeMessage,
    mergeMessages,
    updateConversationPreviewFromMessage,
    messageNeedsMediaHydration
  });

  const { reconcilePendingMessageStatus } = useOmnichannelInboxPendingStatus({
    messages,
    fetchMessagesPage,
    normalizeMessage,
    mergeMessages,
    updateConversationPreviewFromMessage
  });

  // --- Code 1: Message Window (always max 50 messages) ---
  const messageWindow = useInboxMessageWindow({
    messages,
    hasMoreMessages,
    chatBodyRef,
    activeConversationId,
    apiFetch,
    normalizeMessage
  });

  // --- Code 2: Sync Guard (freshness check + send verification) ---
  const syncGuard = useInboxSyncGuard({
    activeConversationId,
    messages,
    apiFetch,
    refreshConversationMessages,
    syncConversationHistory,
    normalizeMessage,
    mergeMessages,
    updateConversationPreviewFromMessage
  });

  const {
    scheduleStickyDateRefresh,
    scrollToBottom,
    scrollToLatest,
    selectConversation,
    onChatScroll,
    onChatBodyMounted,
    requestOlderMessages,
    cancelScheduledStickyRefresh
  } = useOmnichannelInboxScroll({
    chatBodyRef,
    messages,
    hasMoreMessages,
    loadingOlderMessages,
    showLoadOlderMessagesButton,
    showScrollToLatestButton,
    showStickyDate,
    stickyDateLabel,
    firstUnreadMessageId,
    conversations,
    activeConversationId,
    assigneeModel,
    draft,
    replyTarget,
    clearAttachment,
    loadConversationMessages,
    loadGroupParticipants,
    loadOlderMessages,
    markConversationAsRead,
    messageRowId
  });
  selectConversationHandler = selectConversation;

  const {
    setReplyTarget,
    clearReplyTarget,
    updateDraft,
    updateAttachment,
    updateSearch,
    updateChannel,
    updateStatus,
    updateSidebarView,
    updateShowFilters,
    updateLeftCollapsed,
    updateRightCollapsed,
    updateAssigneeModel,
    updateInternalNotes
  } = useOmnichannelInboxStateMutators({
    replyTarget,
    draft,
    search,
    channel,
    status,
    instanceId,
    sidebarView,
    showFilters,
    leftCollapsed,
    rightCollapsed,
    assigneeModel,
    contactActionError,
    internalNotes,
    setAttachmentFromFile
  });

  async function handleInstanceFilterChange(value: string) {
    const nextValue = sanitizeSelectedInstanceId(value);
    if (nextValue === instanceId.value) {
      return;
    }

    persistInstanceFilterPreference(nextValue);
    activeConversationId.value = null;
    messages.value = [];
    visibleMessagesConversationId.value = null;
    hasMoreMessages.value = false;
    showLoadOlderMessagesButton.value = false;
    showScrollToLatestButton.value = false;
    await Promise.allSettled([
      loadWhatsAppStatus({ force: true }),
      loadConversations({ skipOpenSync: true })
    ]);
  }

  async function refreshAfterConversationHistoryClear() {
    activeConversationId.value = null;
    messages.value = [];
    visibleMessagesConversationId.value = null;
    hasMoreMessages.value = false;
    showLoadOlderMessagesButton.value = false;
    showScrollToLatestButton.value = false;
    clearReplyTarget();

    await Promise.allSettled([
      loadWhatsAppStatus({ force: true }),
      loadConversations({ skipOpenSync: true }),
      loadContacts()
    ]);
  }

  const {
    saveActiveConversationContact,
    createContactAndOpenConversation,
    saveContactFromMessageCard,
    openContactConversation,
    importingContacts,
    contactImportPreview,
    contactImportResult,
    previewWhatsAppContactsImport,
    applyWhatsAppContactsImport,
    clearWhatsAppContactsImportPreview
  } = useOmnichannelInboxContactActions({
    activeConversationId,
    canSaveActiveContact,
    savingContact,
    creatingContact,
    contactActionError,
    sidebarView,
    apiFetch,
    upsertContact,
    upsertConversation,
    syncSavedContactIntoMessages,
    loadContacts,
    loadConversations,
    selectConversation
  });

  const { sendMessage, sendContactCard } = useOmnichannelInboxOutboundPipeline({
    publicApiBase: config.public.apiBase,
    token,
    tenantSlug: computed(() => tenantSlug.value),
    shellClientId,
    user,
    canManageConversation,
    isGroupConversation,
    activeConversationId,
    draft,
    attachment,
    replyTarget,
    pendingSendCount,
    sendError,
    messages,
    apiFetch,
    buildReplyMetadata,
    buildOutboundLinkPreviewMetadata,
    findGroupParticipantForMention,
    createOptimisticMessage,
    normalizeMessage,
    mergeMessages,
    updateConversationPreviewFromMessage,
    clearAttachment,
    scrollToBottom,
    markConversationAsRead,
    scheduleStickyDateRefresh,
    reconcilePendingMessageStatus: (conversationId: string, messageId: string) => {
      syncGuard.verifySend(conversationId, messageId);
      return reconcilePendingMessageStatus(conversationId, messageId);
    }
  });

  const { reactToMessage } = useOmnichannelInboxMessageReactions({
    canManageConversation,
    activeConversationId,
    messages,
    sendError,
    user,
    apiFetch,
    normalizeMessage,
    mergeMessages,
    updateConversationPreviewFromMessage,
    loadConversationMessages
  });

  const {
    closeConversation,
    updateConversationStatus,
    updateConversationAssignee,
    openSandboxTestConversation
  } = useOmnichannelInboxConversationActions({
    canManageConversation,
    activeConversationId,
    updatingStatus,
    updatingAssignee,
    assigneeModel,
    sendError,
    apiFetch,
    upsertConversation,
    selectConversation
  });

  const {
    realtimeConnectionState,
    connectSocket,
    disconnectSocket,
    startWhatsAppStatusPolling,
    stopWhatsAppStatusPolling
  } = useOmnichannelInboxRealtime({
    publicApiBase: config.public.apiBase,
    token,
    tenantSlug: computed(() => tenantSlug.value),
    shellClientId,
    conversations,
    messages,
    activeConversationId,
    visibleMessagesConversationId,
    selectedInstanceId: instanceId,
    chatBodyRef,
    loadConversations,
    refreshConversationMessages,
    loadWhatsAppStatus,
    upsertConversation,
    normalizeMessage,
    mergeMessages,
    updateConversationPreviewFromMessage,
    updateConversationCacheFromMessage,
    scheduleGroupParticipantsRefresh,
    shouldFlagMentionAlert,
    incrementMentionAlert,
    scrollToBottom,
    markConversationAsRead,
    messageNeedsMediaHydration,
    hydrateRealtimeMediaMessage,
    scheduleStickyDateRefresh,
    enforceMessageWindow: () => messageWindow.enforceWindow()
  });

  const {
    processingMessageAction,
    deleteMessagesForMe,
    deleteMessagesForAll,
    forwardMessagesToConversation
  } = useOmnichannelInboxMessageActions({
    canManageConversation,
    activeConversationId,
    conversations,
    messages,
    apiFetch,
    upsertConversation,
    mergeMessages,
    updateConversationPreviewFromMessage,
    scheduleStickyDateRefresh,
    loadConversationMessages
  });

  const switchingTenant = ref(false);
  const switchTenantError = ref("");

  async function switchTenant(clientId: number) {
    if (switchingTenant.value) {
      return;
    }

    switchingTenant.value = true;
    switchTenantError.value = "";
    const bootstrapHandle = startPageBootstrap();
    try {
      syncGuard.stopSync();
      disconnectSocket();
      stopWhatsAppStatusPolling();

      const response = await apiFetch<{
        token: string;
        sessionContext: {
          tenantId: string;
          tenantSlug: string;
        };
      }>("/session/context", {
        method: "POST",
        body: { clientId }
      });

      await syncSessionFromToken({
        accessToken: response.token,
        sessionContext: response.sessionContext,
        redirectToLogin: true
      });

      activeConversationId.value = null;
      messages.value = [];
      visibleMessagesConversationId.value = null;
      hasMoreMessages.value = false;
      showLoadOlderMessagesButton.value = false;
      showScrollToLatestButton.value = false;
      conversations.value = [];
      contacts.value = [];
      users.value = [];
      whatsappInstances.value = [];

      await Promise.allSettled([
        loadTenantUploadLimit(),
        loadAccessibleWhatsAppInstances()
      ]);
      loadInstanceFilterPreference();

      await Promise.allSettled([
        loadWhatsAppStatus({ force: true }),
        loadConversations()
      ]);
      await Promise.allSettled([
        loadUsers(),
        loadContacts()
      ]);
      startWhatsAppStatusPolling();
      connectSocket();
      syncGuard.startSync();
    } catch (error) {
      switchTenantError.value = error instanceof Error
        ? error.message
        : "Falha ao trocar de tenant. Verifique se a API esta rodando.";
      startWhatsAppStatusPolling();
      connectSocket();
    } finally {
      switchingTenant.value = false;
      await finishPageBootstrap(bootstrapHandle);
    }
  }

  function logout() {
    disconnectSocket();
    stopWhatsAppStatusPolling();
    void performLogout();
  }

  onMounted(async () => {
    ensureAuthOrRedirect();
    if (!token.value) {
      return;
    }

    loadOutboundOperatorLabelPreference();
    loadReadState();
    await runPageBootstrap(async () => {
      await Promise.allSettled([
        loadTenantUploadLimit(),
        loadAccessibleWhatsAppInstances()
      ]);
      loadInstanceFilterPreference();

      await Promise.allSettled([
        loadWhatsAppStatus({ force: true }),
        loadConversations()
      ]);

      await Promise.allSettled([
        loadUsers(),
        loadContacts()
      ]);

      startWhatsAppStatusPolling();
      connectSocket();
      syncGuard.startSync();
    });
  });

  onBeforeUnmount(() => {
    syncGuard.stopSync();
    disconnectSocket();
    stopWhatsAppStatusPolling();
    groupParticipantsRefreshAtByConversation.clear();
    groupParticipantsInFlightByConversation.clear();
    historySyncAttemptAtByConversation.clear();
    historySyncInFlightByConversation.clear();
    clearAttachment();
    cancelScheduledStickyRefresh();
  });

  watch(
    () => outboundOperatorLabelStorageKey.value,
    () => {
      loadOutboundOperatorLabelPreference();
    }
  );

  watch(
    () => [instanceFilterStorageKey.value, whatsappInstances.value.map((entry) => entry.id).join(",")],
    () => {
      loadInstanceFilterPreference();
    }
  );

  return {
    user,
    tenantSlug,
    leftCollapsed,
    rightCollapsed,
    showFilters,
    sidebarView,
    loadingConversations,
    loadingContacts,
    loadingUsers,
    loadingWhatsAppStatus,
    loadingMessages,
    loadingOlderMessages,
    loadingGroupParticipants,
    pageBootstrapping,
    hasMoreMessages,
    showLoadOlderMessagesButton,
    showScrollToLatestButton,
    savingContact,
    creatingContact,
    importingContacts,
    processingMessageAction,
    pendingSendCount,
    sendingMessage,
    sendError,
    contactActionError,
    contactImportPreview,
    contactImportResult,
    realtimeConnectionState,
    whatsappStatus,
    whatsappConnectionState,
    isWhatsAppConfigured,
    isWhatsAppConnected,
    whatsappBannerMessage,
    updatingStatus,
    updatingAssignee,
    stickyDateLabel,
    showStickyDate,
    draft,
    hasAttachment,
    attachmentType,
    attachmentName,
    attachmentMimeType,
    attachmentSizeBytes,
    attachmentDurationSeconds,
    attachmentPreviewUrl,
    tenantMaxUploadMb,
    search,
    channel,
    status,
    instanceId,
    replyTarget,
    assigneeModel,
    channelFilterItems,
    statusFilterItems,
    statusActionItems,
    instanceFilterItems,
    assigneeItems,
    conversations,
    whatsappInstances,
    activeConversation,
    activeConversationId,
    activeConversationLabel,
    contacts,
    filteredContacts,
    isGroupConversation,
    canSaveActiveContact,
    canManageConversation,
    activeGroupParticipants,
    internalNotes,
    filteredConversations,
    unreadConversationIds,
    mentionConversationIds,
    mentionConversationCounts,
    messageRenderItems,
    showOutboundOperatorLabel,
    onChatBodyMounted,
    onChatScroll,
    scrollToLatest,
    requestOlderMessages,
    setReplyTarget,
    clearReplyTarget,
    updateDraft,
    updateAttachment,
    clearAttachment,
    updateSearch,
    updateChannel,
    updateStatus,
    updateInstanceId: handleInstanceFilterChange,
    updateSidebarView,
    updateShowFilters,
    updateLeftCollapsed,
    updateRightCollapsed,
    updateAssigneeModel,
    updateInternalNotes,
    updateShowOutboundOperatorLabel,
    selectConversation,
    openContactConversation,
    createContactAndOpenConversation,
    previewWhatsAppContactsImport,
    applyWhatsAppContactsImport,
    clearWhatsAppContactsImportPreview,
    saveContactFromMessageCard,
    saveActiveConversationContact,
    deleteMessagesForMe,
    deleteMessagesForAll,
    forwardMessagesToConversation,
    closeConversation,
    sendMessage,
    sendContactCard,
    reactToMessage,
    updateConversationStatus,
    updateConversationAssignee,
    refreshAfterConversationHistoryClear,
    loadContacts,
    loadGroupParticipants,
    openMentionConversation,
    openSandboxTestConversation,
    switchTenant,
    switchingTenant,
    switchTenantError,
    logout
  };
}
