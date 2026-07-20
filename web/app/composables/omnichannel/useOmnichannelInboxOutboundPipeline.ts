import { nextTick, type ComputedRef, type Ref } from "vue";
import type {
  GroupParticipant,
  Message,
  MessageType
} from "~/types";
import type {
  OptimisticMessageOptions,
  OutboundAttachment,
  SendMessageOptions
} from "~/composables/omnichannel/useOmnichannelInboxShared";
import {
  asRecord,
  extractMentionsFromText,
  formatSendError,
  normalizeContactPhone,
  normalizeMentionJid,
  normalizePhoneDigits,
  readFileAsDataUrl
} from "~/composables/omnichannel/useOmnichannelInboxShared";

interface InboxSessionUser {
  id?: string | null;
  tenantId?: string | null;
  name?: string | null;
}

export function useOmnichannelInboxOutboundPipeline(options: {
  publicApiBase: string;
  token: Ref<string | null>;
  tenantSlug: Ref<string | null>;
  shellClientId: Ref<number>;
  user: Ref<InboxSessionUser | null>;
  canManageConversation: ComputedRef<boolean>;
  isGroupConversation: ComputedRef<boolean>;
  activeConversationId: Ref<string | null>;
  draft: Ref<string>;
  attachment: Ref<OutboundAttachment | null>;
  replyTarget: Ref<Message | null>;
  pendingSendCount: Ref<number>;
  sendError: Ref<string>;
  messages: Ref<Message[]>;
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>;
  buildReplyMetadata: (messageEntry: Message) => Record<string, unknown>;
  buildOutboundLinkPreviewMetadata: (value: string, enabled: boolean) => Record<string, unknown> | null;
  findGroupParticipantForMention: (jid: string) => GroupParticipant | null | undefined;
  createOptimisticMessage: (options: OptimisticMessageOptions) => Message;
  normalizeMessage: (messageEntry: Message) => Message;
  mergeMessages: (...chunks: Message[][]) => Message[];
  updateConversationPreviewFromMessage: (messageEntry: Message) => void;
  clearAttachment: (options?: { revokePreview?: boolean }) => void;
  scrollToBottom: () => void;
  markConversationAsRead: (messageEntry?: Message) => void;
  scheduleStickyDateRefresh: () => void;
  reconcilePendingMessageStatus: (conversationId: string, messageId: string) => Promise<void>;
}) {
  function buildOutboundMetadataJson(params: {
    reply: Message | null;
    text: string;
    mentionedJids?: string[];
    linkPreviewEnabled?: boolean;
    attachment?: OutboundAttachment | null;
  }) {
    const metadataJson: Record<string, unknown> = {};

    if (params.reply) {
      metadataJson.reply = options.buildReplyMetadata(params.reply);
    }

    if (options.isGroupConversation.value) {
      const byText = params.text ? extractMentionsFromText(params.text) : null;
      const explicitMentioned = (params.mentionedJids ?? [])
        .map((entry) => normalizeMentionJid(entry))
        .filter((entry): entry is string => Boolean(entry));

      const mergedMentioned = [...new Set([...(byText?.mentioned ?? []), ...explicitMentioned])];
      const everyOne = Boolean(byText?.everyOne);

      if (everyOne || mergedMentioned.length > 0) {
        const displayByJid: Record<string, string> = {};
        const displayByPhone: Record<string, string> = {};

        for (const mentionedJid of mergedMentioned) {
          const normalizedMentionedJid = normalizeMentionJid(mentionedJid);
          const participant = options.findGroupParticipantForMention(mentionedJid);
          const displayName = participant?.name?.trim() ?? "";
          const displayPhone =
            normalizePhoneDigits(participant?.phone) ??
            normalizePhoneDigits(participant?.jid) ??
            normalizePhoneDigits(mentionedJid);

          if (normalizedMentionedJid && displayName) {
            displayByJid[normalizedMentionedJid] = displayName;
          }

          if (displayPhone && displayName) {
            displayByPhone[displayPhone] = displayName;
          }
        }

        metadataJson.mentions = {
          everyOne,
          mentioned: mergedMentioned,
          ...(Object.keys(displayByJid).length > 0 ? { displayByJid } : {}),
          ...(Object.keys(displayByPhone).length > 0 ? { displayByPhone } : {})
        };
      }
    }

    if (params.attachment?.sendAsSticker) {
      metadataJson.media = {
        ...(asRecord(metadataJson.media) ?? {}),
        sendAsSticker: true,
        kind: "sticker"
      };
      if (!params.text.trim()) {
        metadataJson.sticker = {
          source: "composer"
        };
      }
    }

    if (params.attachment?.sendAsVoiceNote) {
      metadataJson.media = {
        ...(asRecord(metadataJson.media) ?? {}),
        sendAsVoiceNote: true,
        kind: "voice"
      };
    }

    if (!params.attachment) {
      const linkPreview = options.buildOutboundLinkPreviewMetadata(
        params.text,
        params.linkPreviewEnabled !== false
      );
      if (linkPreview) {
        metadataJson.linkPreview = linkPreview;
      }
    }

    return metadataJson;
  }

  async function buildMediaMessagePayload(params: {
    type: MessageType;
    text: string;
    attachment: OutboundAttachment;
    metadataJson: Record<string, unknown>;
  }) {
    const mediaUrl = await readFileAsDataUrl(params.attachment.file);
    const payload: Record<string, unknown> = {
      type: params.type,
      content: params.text || undefined,
      mediaUrl,
      mediaMimeType: params.attachment.file.type || undefined,
      mediaFileName: params.attachment.file.name,
      mediaFileSizeBytes: params.attachment.file.size
    };

    if (params.type !== "AUDIO") {
      payload.mediaCaption = params.text || undefined;
    } else if (typeof params.attachment.durationSeconds === "number" && Number.isFinite(params.attachment.durationSeconds)) {
      payload.mediaDurationSeconds = Math.max(0, Math.floor(params.attachment.durationSeconds));
    }

    if (Object.keys(params.metadataJson).length > 0) {
      payload.metadataJson = params.metadataJson;
    }

    return payload;
  }

  async function buildOutboundRequestPayload(params: {
    text: string;
    attachment: OutboundAttachment | null;
    metadataJson: Record<string, unknown>;
  }) {
    if (!params.attachment) {
      const payload: Record<string, unknown> = {
        type: "TEXT",
        content: params.text
      };

      if (Object.keys(params.metadataJson).length > 0) {
        payload.metadataJson = params.metadataJson;
      }

      return payload;
    }

    const outboundText = params.attachment.sendAsSticker ? "" : params.text;

    return buildMediaMessagePayload({
      type: params.attachment.type,
      text: outboundText,
      attachment: params.attachment,
      metadataJson: params.metadataJson
    });
  }

  function resolveDirectTimeoutMs(attachmentEntry: OutboundAttachment | null) {
    if (!attachmentEntry) {
      return 90_000;
    }

    const baseByType: Record<MessageType, number> = {
      TEXT: 90_000,
      IMAGE: 120_000,
      VIDEO: 240_000,
      AUDIO: 120_000,
      DOCUMENT: 180_000
    };

    const perMbMsByType: Record<MessageType, number> = {
      TEXT: 0,
      IMAGE: 25_000,
      VIDEO: 40_000,
      AUDIO: 20_000,
      DOCUMENT: 30_000
    };

    const sizeInMb = Math.max(1, Math.ceil(attachmentEntry.file.size / (1024 * 1024)));
    const base = baseByType[attachmentEntry.type];
    const perMb = perMbMsByType[attachmentEntry.type];

    return Math.min(300_000, Math.max(base, sizeInMb * perMb));
  }

  async function sendConversationMessage(
    conversationId: string,
    payload: Record<string, unknown>,
    forceDirect = false,
    timeoutMs = 90_000
  ) {
    const path = `/conversations/${conversationId}/messages`;

    // F1 — REPONTADO (nao verbatim). O legado tinha aqui um bypass que falava
    // direto com o Fastify no :4000, montando Authorization / x-selected-tenant-slug
    // / x-client-id na mao e pulando o BFF. No Omni todo trafego passa pelo
    // apiFetch, que delega ao createApiRequest — token e X-Account-Id vem do
    // provider global. Montar header de conta aqui seria uma segunda fonte de
    // escopo (project_account_source_divergence) e fere o isolamento multi-tenant.
    //
    // O comportamento e preservado: `forceDirect` sobrevive como sinal de "envio
    // pesado" e mantem o timeout largo que o caminho direto usava para upload de
    // midia — exatamente o que o fallback do legado (:270) ja fazia.
    return options.apiFetch<Message>(path, {
      method: "POST",
      body: payload,
      ...(forceDirect && import.meta.client
        ? { timeout: Math.max(120_000, timeoutMs + 30_000) }
        : {})
    });
  }

  async function sendMessage(optionsOverride?: SendMessageOptions) {
    if (!options.canManageConversation.value) {
      options.sendError.value = "Seu perfil e somente leitura nesta inbox.";
      return;
    }

    if (!options.activeConversationId.value) {
      return;
    }

    const conversationId = options.activeConversationId.value;
    const text = options.draft.value.trim();
    const currentAttachment = options.attachment.value
      ? {
        ...options.attachment.value
      }
      : null;
    const currentReply = options.replyTarget.value;

    if (!text && !currentAttachment) {
      return;
    }

    options.sendError.value = "";
    options.pendingSendCount.value += 1;

    const optimisticMessage = options.createOptimisticMessage({
      conversationId,
      text,
      attachment: currentAttachment,
      reply: currentReply
    });

    options.messages.value = options.mergeMessages(options.messages.value, [optimisticMessage]);
    options.updateConversationPreviewFromMessage(optimisticMessage);
    options.draft.value = "";
    options.replyTarget.value = null;
    if (currentAttachment) {
      options.clearAttachment({ revokePreview: false });
    }

    try {
      const metadataJson = buildOutboundMetadataJson({
        reply: currentReply,
        text,
        mentionedJids: optionsOverride?.mentionedJids,
        linkPreviewEnabled: optionsOverride?.linkPreviewEnabled,
        attachment: currentAttachment
      });
      const body = await buildOutboundRequestPayload({
        text,
        attachment: currentAttachment,
        metadataJson
      });
      const directTimeoutMs = resolveDirectTimeoutMs(currentAttachment);

      const created = await sendConversationMessage(
        conversationId,
        body,
        Boolean(currentAttachment),
        directTimeoutMs
      );

      options.messages.value = options.mergeMessages(
        options.messages.value.filter((entry) => entry.id !== optimisticMessage.id),
        [created]
      );
      options.updateConversationPreviewFromMessage(created);

      await nextTick();
      options.scrollToBottom();
      options.markConversationAsRead(created);
      options.scheduleStickyDateRefresh();

      if (created.status === "PENDING") {
        void options.reconcilePendingMessageStatus(created.conversationId, created.id);
      }
    } catch (error) {
      options.messages.value = options.mergeMessages(
        options.messages.value,
        [
          {
            ...optimisticMessage,
            status: "FAILED",
            updatedAt: new Date().toISOString()
          }
        ]
      );

      if (!currentAttachment && text && !options.draft.value.trim()) {
        options.draft.value = text;
      }

      if (!currentAttachment && currentReply && !options.replyTarget.value) {
        options.replyTarget.value = currentReply;
      }

      options.sendError.value = formatSendError(error);
    } finally {
      if (currentAttachment?.previewUrl?.startsWith("blob:")) {
        URL.revokeObjectURL(currentAttachment.previewUrl);
      }
      options.pendingSendCount.value = Math.max(0, options.pendingSendCount.value - 1);
    }
  }

  async function sendContactCard(payload: {
    name: string;
    phone: string;
    contactId?: string | null;
    avatarUrl?: string | null;
  }) {
    if (!options.canManageConversation.value) {
      options.sendError.value = "Seu perfil e somente leitura nesta inbox.";
      return;
    }

    if (!options.activeConversationId.value) {
      return;
    }

    const conversationId = options.activeConversationId.value;
    const normalizedName = payload.name.trim();
    const normalizedPhone = normalizeContactPhone(payload.phone);
    if (!normalizedName && !normalizedPhone) {
      return;
    }

    const label = normalizedName || normalizedPhone;
    const contactEntry: Record<string, unknown> = {
      name: label,
      displayName: label,
      fullName: label,
      phone: normalizedPhone || undefined,
      phoneNumber: normalizedPhone || undefined,
      number: normalizedPhone || undefined,
      source: payload.contactId ? "saved-contact" : "composer",
      contactId: payload.contactId ?? undefined,
      avatarUrl: payload.avatarUrl ?? undefined
    };
    const content = `Contato: ${label}${normalizedPhone ? ` (${normalizedPhone})` : ""}`.trim();
    const metadataJson: Record<string, unknown> = {
      contact: contactEntry,
      contacts: [contactEntry]
    };

    options.sendError.value = "";
    options.pendingSendCount.value += 1;

    const nowIso = new Date().toISOString();
    const optimisticMessage = options.normalizeMessage({
      id: `temp-contact-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      tenantId: options.user.value?.tenantId ?? "temp-tenant",
      conversationId,
      senderUserId: options.user.value?.id ?? null,
      direction: "OUTBOUND",
      messageType: "TEXT",
      senderName: options.user.value?.name ?? "Voce",
      senderAvatarUrl: null,
      content,
      mediaUrl: null,
      mediaMimeType: null,
      mediaFileName: null,
      mediaFileSizeBytes: null,
      mediaCaption: null,
      mediaDurationSeconds: null,
      metadataJson,
      status: "PENDING",
      externalMessageId: null,
      createdAt: nowIso,
      updatedAt: nowIso
    } as Message);

    options.messages.value = options.mergeMessages(options.messages.value, [optimisticMessage]);
    options.updateConversationPreviewFromMessage(optimisticMessage);

    try {
      const created = await sendConversationMessage(conversationId, {
        type: "TEXT",
        content,
        metadataJson
      });

      options.messages.value = options.mergeMessages(
        options.messages.value.filter((entry) => entry.id !== optimisticMessage.id),
        [created]
      );
      options.updateConversationPreviewFromMessage(created);
      await nextTick();
      options.scrollToBottom();
      options.markConversationAsRead(created);
      options.scheduleStickyDateRefresh();

      if (created.status === "PENDING") {
        void options.reconcilePendingMessageStatus(created.conversationId, created.id);
      }
    } catch (error) {
      options.messages.value = options.mergeMessages(
        options.messages.value,
        [
          {
            ...optimisticMessage,
            status: "FAILED",
            updatedAt: new Date().toISOString()
          }
        ]
      );
      options.updateConversationPreviewFromMessage({
        ...optimisticMessage,
        status: "FAILED"
      });
      options.sendError.value = formatSendError(error, "Nao foi possivel enviar contato.");
    } finally {
      options.pendingSendCount.value = Math.max(0, options.pendingSendCount.value - 1);
    }
  }

  return {
    sendMessage,
    sendContactCard
  };
}
