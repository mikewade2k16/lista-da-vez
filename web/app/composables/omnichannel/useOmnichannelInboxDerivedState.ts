import { computed, type Ref } from "vue";
import type { InboxRenderItem, InboxSelectOption } from "~/components/omnichannel/inbox/types";
import type {
  Contact,
  Conversation,
  GroupParticipant,
  Message,
  TenantUser,
  UserRole,
  WhatsAppInstanceRecord,
  WhatsAppStatusResponse
} from "~/types";
import {
  type OutboundAttachment,
  type ReadStateEntry,
  UNASSIGNED_VALUE,
  buildDateKey,
  canWriteInboxByRole,
  formatDateHeader,
  normalizePhoneDigits,
  toArrayOrEmpty
} from "~/composables/omnichannel/useOmnichannelInboxShared";

export function useOmnichannelInboxDerivedState(options: {
  user: Ref<{
    id?: string | null;
    role?: UserRole | null;
    tenantId?: string | null;
  } | null>;
  users: Ref<TenantUser[]>;
  whatsappInstances: Ref<WhatsAppInstanceRecord[]>;
  conversations: Ref<Conversation[]>;
  contacts: Ref<Contact[]>;
  messages: Ref<Message[]>;
  activeConversationId: Ref<string | null>;
  search: Ref<string>;
  channel: Ref<string>;
  status: Ref<string>;
  instanceId: Ref<string>;
  attachment: Ref<OutboundAttachment | null>;
  tenantMaxUploadMb: Ref<number>;
  whatsappStatus: Ref<WhatsAppStatusResponse | null>;
  loadingWhatsAppStatus: Ref<boolean>;
  notesByConversation: Record<string, string>;
  groupParticipantsByConversation: Record<string, GroupParticipant[]>;
  readState: Ref<Record<string, ReadStateEntry>>;
  mentionAlertState: Ref<Record<string, number>>;
  isConversationUnread: (conversationEntry: Conversation) => boolean;
}) {
  function isTechnicalConversationLabel(value: string | null | undefined) {
    const normalized = value?.trim().toLowerCase() ?? "";
    if (!normalized) {
      return true;
    }

    if (
      normalized.endsWith("@s.whatsapp.net") ||
      normalized.endsWith("@g.us") ||
      normalized.endsWith("@lid")
    ) {
      return true;
    }

    const digitsOnly = normalized.replace(/\D/g, "");
    return digitsOnly.length >= 7 && !/[a-z]/i.test(normalized);
  }

  function sanitizeConversationLabel(conversationEntry: Conversation) {
    const contactName = conversationEntry.contactName?.trim() ?? "";
    const normalizedPhone = normalizePhoneDigits(conversationEntry.contactPhone ?? conversationEntry.externalId);
    const isGroup = conversationEntry.externalId.endsWith("@g.us");

    if (contactName && !isTechnicalConversationLabel(contactName)) {
      return contactName;
    }

    if (!isGroup && normalizedPhone) {
      return normalizedPhone;
    }

    if (isGroup && normalizedPhone) {
      return `Grupo ${normalizedPhone.slice(-4)}`;
    }

    return isGroup ? "Grupo" : "Contato";
  }

  const assigneeItems = computed<InboxSelectOption[]>(() => {
    const usersList = toArrayOrEmpty<TenantUser>(options.users.value);
    return [
      { label: "Nao atribuido", value: UNASSIGNED_VALUE },
      ...usersList.map((entry) => ({
        label: `${entry.name} (${entry.role})`,
        value: entry.id
      }))
    ];
  });

  const instanceFilterItems = computed<InboxSelectOption[]>(() => {
    const instanceItems = toArrayOrEmpty<WhatsAppInstanceRecord>(options.whatsappInstances.value);
    return [
      { label: "Todas instancias", value: "all" },
      ...instanceItems.map((entry) => ({
        label: entry.displayName || entry.instanceName,
        value: entry.id
      }))
    ];
  });

  const activeConversation = computed<Conversation | null>(() => {
    if (!options.activeConversationId.value) {
      return null;
    }

    const conversationsList = toArrayOrEmpty<Conversation>(options.conversations.value);
    return conversationsList.find((conversationEntry) => conversationEntry.id === options.activeConversationId.value) ?? null;
  });

  const activeConversationLabel = computed(() => {
    if (!activeConversation.value) {
      return null;
    }

    return sanitizeConversationLabel(activeConversation.value);
  });

  const filteredContacts = computed(() => {
    const contactsList = toArrayOrEmpty<Contact>(options.contacts.value);
    const term = options.search.value.trim().toLowerCase();

    if (!term) {
      return contactsList;
    }

    return contactsList.filter((contactEntry) => {
      const name = contactEntry.name?.toLowerCase() ?? "";
      const phone = contactEntry.phone ?? "";
      return name.includes(term) || phone.includes(normalizePhoneDigits(term) ?? "") || phone.includes(term);
    });
  });

  const isGroupConversation = computed(() => {
    return Boolean(activeConversation.value?.externalId?.endsWith("@g.us"));
  });

  const canManageConversation = computed(() => canWriteInboxByRole(options.user.value?.role));
  const canSaveActiveContact = computed(() => {
    if (!activeConversation.value || isGroupConversation.value) {
      return false;
    }

    if (activeConversation.value.contactId) {
      return false;
    }

    return Boolean(
      normalizePhoneDigits(activeConversation.value.contactPhone ?? activeConversation.value.externalId)
    );
  });

  const activeGroupParticipants = computed<GroupParticipant[]>(() => {
    const conversationId = options.activeConversationId.value;
    if (!conversationId) {
      return [];
    }

    return options.groupParticipantsByConversation[conversationId] ?? [];
  });

  const readStateStorageKey = computed(() => {
    return `omni:read-state:${options.user.value?.tenantId ?? "tenant"}:${options.user.value?.id ?? "user"}`;
  });

  const hasKnownWhatsAppInstances = computed(() => {
    return toArrayOrEmpty<WhatsAppInstanceRecord>(options.whatsappInstances.value).length > 0;
  });

  const whatsappConnectionState = computed(() => {
    const value =
      (options.whatsappStatus.value?.connectionState as { instance?: { state?: string } } | undefined)?.instance
        ?.state ??
      (options.whatsappStatus.value?.providerUnavailable ? "provider_unavailable" : "unknown");
    return String(value).toLowerCase();
  });

  const isWhatsAppConfigured = computed(() => {
    return Boolean(options.whatsappStatus.value?.configured || hasKnownWhatsAppInstances.value);
  });

  const isWhatsAppConnected = computed(() => {
    const state = whatsappConnectionState.value;
    return state === "open" || state === "connected";
  });

  const whatsappBannerMessage = computed(() => {
    if (options.loadingWhatsAppStatus.value) {
      return "";
    }

    const statusMessage = options.whatsappStatus.value?.message?.trim() ?? "";

    if (!isWhatsAppConfigured.value) {
      return statusMessage || "Nenhum WhatsApp conectado para este tenant. Configure no Admin.";
    }

    if (!isWhatsAppConnected.value) {
      if (whatsappConnectionState.value === "provider_unavailable") {
        return statusMessage || "Conexao com a Evolution temporariamente indisponivel. Tente novamente em instantes.";
      }

      if (whatsappConnectionState.value === "connecting") {
        return statusMessage || "WhatsApp desconectado. QR aguardando leitura no Admin.";
      }

      return statusMessage || `WhatsApp desconectado (estado: ${whatsappConnectionState.value}). Conecte por QR no Admin.`;
    }

    return "";
  });

  const internalNotes = computed({
    get() {
      const conversationId = options.activeConversationId.value;
      if (!conversationId) {
        return "";
      }

      return options.notesByConversation[conversationId] ?? "";
    },
    set(value: string) {
      const conversationId = options.activeConversationId.value;
      if (!conversationId) {
        return;
      }

      options.notesByConversation[conversationId] = value;
    }
  });

  const hasAttachment = computed(() => Boolean(options.attachment.value));
  const attachmentType = computed(() => options.attachment.value?.type ?? null);
  const attachmentName = computed(() => options.attachment.value?.file.name ?? null);
  const attachmentMimeType = computed(() => options.attachment.value?.file.type || null);
  const attachmentSizeBytes = computed(() => options.attachment.value?.file.size ?? null);
  const attachmentDurationSeconds = computed(() => options.attachment.value?.durationSeconds ?? null);
  const attachmentPreviewUrl = computed(() => options.attachment.value?.previewUrl ?? null);
  const maxAttachmentSizeBytes = computed(() => options.tenantMaxUploadMb.value * 1024 * 1024);

  const filteredConversations = computed(() => {
    const normalizedSearch = options.search.value.trim().toLowerCase();
    const conversationsList = toArrayOrEmpty<Conversation>(options.conversations.value);

    return conversationsList.filter((conversationEntry) => {
      if (options.channel.value !== "all" && conversationEntry.channel !== options.channel.value) {
        return false;
      }

      if (options.status.value !== "all" && conversationEntry.status !== options.status.value) {
        return false;
      }

      if (options.instanceId.value !== "all" && conversationEntry.instanceId !== options.instanceId.value) {
        return false;
      }

      if (!normalizedSearch) {
        return true;
      }

      const joinedText = [
        conversationEntry.contactName ?? "",
        conversationEntry.contactPhone ?? "",
        conversationEntry.externalId,
        conversationEntry.lastMessage?.content ?? ""
      ]
        .join(" ")
        .toLowerCase();

      return joinedText.includes(normalizedSearch);
    });
  });

  const unreadConversationIds = computed(() => {
    const conversationsList = toArrayOrEmpty<Conversation>(options.conversations.value);
    return conversationsList
      .filter((conversationEntry) => options.isConversationUnread(conversationEntry))
      .map((conversationEntry) => conversationEntry.id);
  });

  const mentionConversationIds = computed(() => {
    return Object.entries(options.mentionAlertState.value)
      .filter(([, count]) => Number(count) > 0)
      .map(([conversationId]) => conversationId);
  });

  const mentionConversationCounts = computed<Record<string, number>>(() => {
    const next: Record<string, number> = {};

    for (const [conversationId, count] of Object.entries(options.mentionAlertState.value)) {
      const normalizedCount = Number.isFinite(count) ? Math.max(0, Math.trunc(count)) : 0;
      if (normalizedCount > 0) {
        next[conversationId] = normalizedCount;
      }
    }

    return next;
  });

  const firstUnreadMessageId = computed(() => {
    const conversationId = options.activeConversationId.value;
    if (!conversationId) {
      return null;
    }

    const entry = options.readState.value[conversationId];
    if (!entry?.lastReadAt) {
      return null;
    }

    const readTimestamp = Number(new Date(entry.lastReadAt));
    if (!Number.isFinite(readTimestamp)) {
      return null;
    }

    const firstUnread = options.messages.value.find((messageEntry) => {
      return Number(new Date(messageEntry.createdAt)) > readTimestamp;
    });

    return firstUnread?.id ?? null;
  });

  const messageRenderItems = computed<InboxRenderItem[]>(() => {
    const items: InboxRenderItem[] = [];
    const unreadMessageId = firstUnreadMessageId.value;
    let unreadInserted = false;
    let previousDateKey = "";

    for (const messageEntry of options.messages.value) {
      const dateKey = buildDateKey(messageEntry.createdAt);
      const dateLabel = formatDateHeader(messageEntry.createdAt);

      if (dateKey !== previousDateKey) {
        items.push({
          kind: "date",
          key: `date-${dateKey}`,
          label: dateLabel
        });
        previousDateKey = dateKey;
      }

      if (!unreadInserted && unreadMessageId && messageEntry.id === unreadMessageId) {
        items.push({ kind: "unread", key: "unread-marker" });
        unreadInserted = true;
      }

      items.push({
        kind: "message",
        key: messageEntry.id,
        message: messageEntry,
        dateKey,
        dateLabel
      });
    }

    return items;
  });

  return {
    assigneeItems,
    instanceFilterItems,
    activeConversation,
    activeConversationLabel,
    filteredContacts,
    isGroupConversation,
    canManageConversation,
    canSaveActiveContact,
    activeGroupParticipants,
    readStateStorageKey,
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
  };
}
