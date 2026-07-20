import type { Message, MessageType } from "~/types";

type MessageLinkPreview = {
  url: string;
  title: string;
  description: string | null;
  thumbnailUrl: string | null;
  host: string | null;
  enabled: boolean;
};

type ReplyPreview = {
  content: string;
  author: string;
  messageType: MessageType;
  messageId: string | null;
};

export function useInboxChatMessageRendering(options: {
  getActiveConversation: () => {
    externalId?: string | null;
    contactName?: string | null;
    contactPhone?: string | null;
  } | null;
  getIsGroupConversation: () => boolean;
  resolveMessageAuthor: (messageEntry: Message) => string;
  resolveOutboundOperatorLabel: (messageEntry: Message) => string;
  sanitizeHumanLabel: (
    value: string | null | undefined,
    options?: { fallbackPhone?: string | null | undefined; fallbackLabel?: string }
  ) => string;
  requestOlderMessages?: () => void;
  hasMoreMessages?: () => boolean;
  messageRowId: (messageId: string) => string;
  asRecord: (value: unknown) => Record<string, unknown> | null;
  splitLegacyReplyContent: (value: string) => { quoted: string; content: string } | null;
  parseUnsupportedLabelFromPlaceholder: (value: string) => string | null;
  isMediaPlaceholder: (value: string, placeholders: Set<string>) => boolean;
  extractFirstLinkFromText: (value: string) => string | null;
  extractLinkHost: (value: string) => string | null;
  normalizeLinkUrl: (value: string | null | undefined) => string | null;
  escapeHtml: (value: string) => string;
  escapeHtmlAttribute: (value: string) => string;
  escapeRegExp: (value: string) => string;
  getMentionDisplayFromMetadata: (messageEntry: Message, mentionToken: string) => string | null;
  collectMentionDisplayEntries: (messageEntry: Message) => Array<{ label: string; jid: string | null; phone: string | null }>;
  resolveMentionRouteTarget: (messageEntry: Message, mentionToken: string) => { jid: string | null; phone: string | null };
}) {
  const mediaPlaceholderValues = new Set(["[imagem]", "[audio]", "[video]", "[documento]", "[figurinha]", "."]);
  const unsupportedPlaceholderPrefix = "[conteudo nao suportado:";
  let replyFocusTimer: number | null = null;

  function resolveMessageType(messageEntry: Message): MessageType {
    return messageEntry.messageType ?? "TEXT";
  }

  function isStickerMessage(messageEntry: Message) {
    if (resolveMessageType(messageEntry) !== "IMAGE") {
      return false;
    }

    const metadata = options.asRecord(messageEntry.metadataJson);
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

    return Boolean(options.asRecord(metadata.sticker));
  }

  function getMediaTypeLabel(type: MessageType | null | undefined, messageEntry?: Message | null) {
    if (type === "IMAGE") {
      if (messageEntry && isStickerMessage(messageEntry)) {
        return "Figurinha";
      }

      return "Foto";
    }

    if (type === "VIDEO") {
      return "Video";
    }

    if (type === "AUDIO") {
      if (messageEntry && isAudioFileMessage(messageEntry)) {
        return "Arquivo de audio";
      }

      return "Audio";
    }

    if (type === "DOCUMENT") {
      if (messageEntry && isAudioDocumentMessage(messageEntry)) {
        return "Arquivo de audio";
      }

      return "Documento";
    }

    return "Mensagem";
  }

  function getUnsupportedMetadata(messageEntry: Message) {
    const metadata = options.asRecord(messageEntry.metadataJson);
    const unsupported = metadata ? options.asRecord(metadata.unsupported) : null;
    if (!unsupported) {
      return null;
    }

    const type = typeof unsupported.type === "string" ? unsupported.type.trim() : "";
    const label = typeof unsupported.label === "string" ? unsupported.label.trim() : "";

    return {
      type: type || "unknown",
      label: label || "tipo desconhecido"
    };
  }

  function hasUnsupportedNotice(messageEntry: Message) {
    if (getUnsupportedMetadata(messageEntry)) {
      return true;
    }

    const content = messageEntry.content?.trim().toLowerCase() ?? "";
    return content.startsWith(unsupportedPlaceholderPrefix);
  }

  function getUnsupportedLabel(messageEntry: Message) {
    const fromMetadata = getUnsupportedMetadata(messageEntry);
    if (fromMetadata?.label) {
      return fromMetadata.label;
    }

    return options.parseUnsupportedLabelFromPlaceholder(messageEntry.content) ?? "tipo desconhecido";
  }

  function buildUnsupportedOpenUrl() {
    const externalId = options.getActiveConversation()?.externalId ?? "";
    if (!externalId || externalId.endsWith("@g.us")) {
      return "https://web.whatsapp.com/";
    }

    const digits = externalId.replace(/\D/g, "");
    if (!digits) {
      return "https://web.whatsapp.com/";
    }

    return `https://wa.me/${digits}`;
  }

  function isMentionAlertMessage(messageEntry: Message) {
    if (!options.getIsGroupConversation() || messageEntry.direction !== "INBOUND") {
      return false;
    }

    const metadata = options.asRecord(messageEntry.metadataJson);
    const mentions = metadata ? options.asRecord(metadata.mentions) : null;
    if (!mentions) {
      return false;
    }

    if (mentions.everyOne === true) {
      return true;
    }

    return Array.isArray(mentions.mentioned) && mentions.mentioned.length > 0;
  }

  function getReplyPreview(messageEntry: Message): ReplyPreview | null {
    const metadata = options.asRecord(messageEntry.metadataJson);
    const reply = metadata ? options.asRecord(metadata.reply) : null;

    if (reply) {
      const content = typeof reply.content === "string" ? reply.content.trim() : "";
      const author = typeof reply.author === "string" ? reply.author.trim() : "";
      const messageType = typeof reply.messageType === "string" ? (reply.messageType as MessageType) : "TEXT";
      const messageId =
        typeof reply.messageId === "string" && reply.messageId.trim().length > 0
          ? reply.messageId.trim()
          : null;

      if (content || author || messageType !== "TEXT") {
        return {
          content: content || getMediaTypeLabel(messageType),
          author: options.sanitizeHumanLabel(author || "Mensagem anterior", {
            fallbackLabel: "Mensagem anterior"
          }),
          messageType,
          messageId
        };
      }
    }

    const fallback = options.splitLegacyReplyContent(messageEntry.content ?? "");
    if (!fallback) {
      return null;
    }

    return {
      content: fallback.quoted,
      author: "Mensagem anterior",
      messageType: "TEXT",
      messageId: null
    };
  }

  function getRenderedMessageText(messageEntry: Message) {
    const fallback = options.splitLegacyReplyContent(messageEntry.content ?? "");
    if (fallback) {
      return fallback.content;
    }

    return messageEntry.content ?? "";
  }

  function getMessageLinkPreview(messageEntry: Message): MessageLinkPreview | null {
    const metadata = options.asRecord(messageEntry.metadataJson);
    const linkPreview = metadata ? options.asRecord(metadata.linkPreview) : null;
    const text = getRenderedMessageText(messageEntry);
    const fallbackUrl = options.extractFirstLinkFromText(text);

    if (linkPreview && linkPreview.enabled === false) {
      return null;
    }

    const url = options.normalizeLinkUrl(typeof linkPreview?.url === "string" ? linkPreview.url : null) ?? fallbackUrl;
    if (!url) {
      return null;
    }

    const titleRaw = typeof linkPreview?.title === "string" ? linkPreview.title.trim() : "";
    const descriptionRaw = typeof linkPreview?.description === "string"
      ? linkPreview.description.trim()
      : "";
    const thumbnailRaw = typeof linkPreview?.thumbnailUrl === "string"
      ? linkPreview.thumbnailUrl.trim()
      : "";
    const host = options.extractLinkHost(url);

    return {
      url,
      title: titleRaw || host || url,
      description: descriptionRaw || null,
      thumbnailUrl: thumbnailRaw || null,
      host,
      enabled: true
    };
  }

  function shouldRenderMessageText(messageEntry: Message, hasContactCard: boolean) {
    if (hasUnsupportedNotice(messageEntry)) {
      return false;
    }

    if (hasContactCard) {
      return false;
    }

    const text = getRenderedMessageText(messageEntry).trim();
    if (!text) {
      return false;
    }

    const metadata = options.asRecord(messageEntry.metadataJson);
    const hasContactMetadata = Boolean(metadata && options.asRecord(metadata.contact));

    if (
      resolveMessageType(messageEntry) !== "TEXT" &&
      hasContactMetadata &&
      /^Contato:\s*/i.test(text)
    ) {
      return false;
    }

    if (resolveMessageType(messageEntry) !== "TEXT" && options.isMediaPlaceholder(text, mediaPlaceholderValues)) {
      return false;
    }

    return true;
  }

  function hasImagePreview(messageEntry: Message) {
    return resolveMessageType(messageEntry) === "IMAGE" && Boolean(messageEntry.mediaUrl);
  }

  function hasVideoPreview(messageEntry: Message) {
    return resolveMessageType(messageEntry) === "VIDEO" && Boolean(messageEntry.mediaUrl);
  }

  function getAudioFlags(messageEntry: Message) {
    const metadata = options.asRecord(messageEntry.metadataJson);
    const audioMetadata = metadata ? options.asRecord(metadata.audio) : null;
    const explicitVoiceNote = audioMetadata?.voiceNote === true;
    const explicitAudioFile = audioMetadata?.voiceNote === false;
    const mediaKind = typeof metadata?.mediaKind === "string" ? metadata.mediaKind.trim().toLowerCase() : "";

    return {
      explicitVoiceNote,
      explicitAudioFile,
      mediaKind
    };
  }

  function isAudioFileMessage(messageEntry: Message) {
    const messageType = resolveMessageType(messageEntry);
    const mimeType = messageEntry.mediaMimeType?.trim().toLowerCase() ?? "";
    const audioFlags = getAudioFlags(messageEntry);

    if (messageType === "DOCUMENT") {
      return mimeType.startsWith("audio/") || audioFlags.mediaKind === "audio_file";
    }

    if (messageType === "AUDIO") {
      return audioFlags.explicitAudioFile;
    }

    return false;
  }

  function isVoiceNoteMessage(messageEntry: Message) {
    const messageType = resolveMessageType(messageEntry);
    const audioFlags = getAudioFlags(messageEntry);

    if (messageType !== "AUDIO") {
      return false;
    }

    if (audioFlags.explicitAudioFile || audioFlags.mediaKind === "audio_file") {
      return false;
    }

    return true;
  }

  function isAudioDocumentMessage(messageEntry: Message) {
    return isAudioFileMessage(messageEntry);
  }

  function hasAudioPreview(messageEntry: Message) {
    return (isVoiceNoteMessage(messageEntry) || isAudioFileMessage(messageEntry)) && Boolean(messageEntry.mediaUrl);
  }

  function hasDocumentPreview(messageEntry: Message) {
    return (
      resolveMessageType(messageEntry) === "DOCUMENT" &&
      !isAudioDocumentMessage(messageEntry) &&
      Boolean(messageEntry.mediaUrl)
    );
  }

  function hasPendingMediaPreview(messageEntry: Message) {
    const messageType = resolveMessageType(messageEntry);
    if (messageType === "TEXT") {
      return false;
    }

    if (
      hasImagePreview(messageEntry) ||
      hasVideoPreview(messageEntry) ||
      hasAudioPreview(messageEntry) ||
      hasDocumentPreview(messageEntry)
    ) {
      return false;
    }

    return !hasUnsupportedNotice(messageEntry);
  }

  function getPendingMediaLabel(messageEntry: Message) {
    const label = getMediaTypeLabel(resolveMessageType(messageEntry), messageEntry);
    const directionLabel = messageEntry.direction === "OUTBOUND" ? "enviado" : "recebido";
    return `${label} ${directionLabel}. Carregando preview...`;
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

  function getReplyAuthorLabel(messageEntry: Message) {
    const reply = getReplyPreview(messageEntry);
    if (!reply) {
      return "";
    }

    const activeConversation = options.getActiveConversation();
    const fallbackPhone = activeConversation?.contactPhone || activeConversation?.externalId || null;
    const conversationContactLabel = options.sanitizeHumanLabel(activeConversation?.contactName, {
      fallbackPhone,
      fallbackLabel: "Contato"
    });

    if (!options.getIsGroupConversation()) {
      const normalizedReplyAuthor = normalizeNameForComparison(reply.author);
      const normalizedOperator = normalizeNameForComparison(options.resolveOutboundOperatorLabel(messageEntry));
      if (messageEntry.direction === "OUTBOUND") {
        if (
          normalizedReplyAuthor &&
          (
            normalizedReplyAuthor === normalizedOperator ||
            ["voce", "atendente", "agente", "agent", "operator", "operador"].includes(normalizedReplyAuthor)
          )
        ) {
          return conversationContactLabel;
        }
      }

      if (isTechnicalWhatsAppIdentifier(reply.author)) {
        return conversationContactLabel;
      }
    }

    return options.sanitizeHumanLabel(reply.author, {
      fallbackPhone,
      fallbackLabel: "Mensagem anterior"
    });
  }

  function getReplyTargetText(messageEntry: Message) {
    const text = getRenderedMessageText(messageEntry).trim();
    if (text && !(resolveMessageType(messageEntry) !== "TEXT" && options.isMediaPlaceholder(text, mediaPlaceholderValues))) {
      return text;
    }

    return getMediaTypeLabel(resolveMessageType(messageEntry), messageEntry);
  }

  function getReplyTargetAuthorLabel(messageEntry: Message) {
    if (messageEntry.direction === "OUTBOUND") {
      const operatorLabel = options.resolveOutboundOperatorLabel(messageEntry);
      if (operatorLabel) {
        return operatorLabel;
      }
    }

    return options.resolveMessageAuthor(messageEntry);
  }

  function hasReplyJumpTarget(messageEntry: Message) {
    return Boolean(getReplyPreview(messageEntry)?.messageId);
  }

  function escapeSelectorValue(value: string) {
    if (typeof CSS !== "undefined" && typeof CSS.escape === "function") {
      return CSS.escape(value);
    }

    return value.replace(/["\\]/g, "\\$&");
  }

  function focusMessageRowElement(target: HTMLElement | null) {
    if (!target) {
      return false;
    }

    target.scrollIntoView({
      behavior: "smooth",
      block: "center"
    });

    target.classList.add("chat-message-row--reply-focus");

    if (replyFocusTimer !== null) {
      window.clearTimeout(replyFocusTimer);
    }

    replyFocusTimer = window.setTimeout(() => {
      target.classList.remove("chat-message-row--reply-focus");
      replyFocusTimer = null;
    }, 1600);

    return true;
  }

  function focusMessageRowById(messageId: string) {
    if (!import.meta.client) {
      return false;
    }

    const target = document.getElementById(options.messageRowId(messageId));
    return focusMessageRowElement(target);
  }

  function focusMessageRowByExternalId(externalMessageId: string) {
    if (!import.meta.client) {
      return false;
    }

    const safeExternalMessageId = escapeSelectorValue(externalMessageId);
    const target = document.querySelector<HTMLElement>(
      `[data-message-external-id="${safeExternalMessageId}"]`
    );
    return focusMessageRowElement(target);
  }

  function normalizeMessageIdentifier(value: string) {
    return value.trim().toLowerCase();
  }

  function compactMessageIdentifier(value: string) {
    return normalizeMessageIdentifier(value).replace(/[^a-z0-9]/g, "");
  }

  function identifiersLikelyMatch(left: string, right: string) {
    const normalizedLeft = normalizeMessageIdentifier(left);
    const normalizedRight = normalizeMessageIdentifier(right);
    if (!normalizedLeft || !normalizedRight) {
      return false;
    }

    if (
      normalizedLeft === normalizedRight ||
      normalizedLeft.endsWith(normalizedRight) ||
      normalizedRight.endsWith(normalizedLeft)
    ) {
      return true;
    }

    const compactLeft = compactMessageIdentifier(normalizedLeft);
    const compactRight = compactMessageIdentifier(normalizedRight);
    if (!compactLeft || !compactRight) {
      return false;
    }

    return (
      compactLeft === compactRight ||
      compactLeft.endsWith(compactRight) ||
      compactRight.endsWith(compactLeft)
    );
  }

  function focusMessageRowByLooseMatch(messageId: string) {
    if (!import.meta.client) {
      return false;
    }

    const rows = document.querySelectorAll<HTMLElement>(".chat-message-row");
    for (const row of rows) {
      const externalId = row.dataset.messageExternalId?.trim();
      if (externalId && identifiersLikelyMatch(externalId, messageId)) {
        return focusMessageRowElement(row);
      }
    }

    return false;
  }

  function onReplyPreviewClick(messageEntry: Message) {
    const preview = getReplyPreview(messageEntry);
    if (!preview?.messageId) {
      return;
    }

    if (focusMessageRowById(preview.messageId)) {
      return;
    }

    if (focusMessageRowByExternalId(preview.messageId)) {
      return;
    }

    if (focusMessageRowByLooseMatch(preview.messageId)) {
      return;
    }

    if (options.requestOlderMessages && options.hasMoreMessages?.()) {
      options.requestOlderMessages();
      window.setTimeout(() => {
        if (focusMessageRowById(preview.messageId)) {
          return;
        }
        if (focusMessageRowByExternalId(preview.messageId)) {
          return;
        }
        focusMessageRowByLooseMatch(preview.messageId);
      }, 320);
    }
  }

  function renderMessageHtml(value: string, messageEntry: Message) {
    const mentionAnchors: string[] = [];
    const mentionPlaceholderPrefix = "__OMNI_MENTION__";

    let escapedValue = options.escapeHtml(value);
    const explicitMentionEntries = options.collectMentionDisplayEntries(messageEntry)
      .map((entry) => ({
        ...entry,
        label: options.sanitizeHumanLabel(entry.label, {
          fallbackPhone: entry.phone ?? entry.jid,
          fallbackLabel: "Contato"
        })
      }))
      .filter((entry) => entry.label.includes(" "))
      .sort((left, right) => right.label.length - left.label.length);

    for (const entry of explicitMentionEntries) {
      const labelToken = `@${entry.label}`;
      const labelPattern = new RegExp(
        `(^|[\\s(])(${options.escapeRegExp(options.escapeHtml(labelToken))})(?=\\s|$|[),.!?])`,
        "g"
      );

      escapedValue = escapedValue.replace(labelPattern, (_match: string, prefix: string) => {
        const routeTarget = options.resolveMentionRouteTarget(messageEntry, labelToken);
        const jidAttr = routeTarget.jid
          ? ` data-mention-jid="${options.escapeHtmlAttribute(routeTarget.jid)}"`
          : "";
        const phoneAttr = routeTarget.phone
          ? ` data-mention-phone="${options.escapeHtmlAttribute(routeTarget.phone)}"`
          : "";

        const anchor = `<a href="#" class="chat-message__mention chat-message__mention-link"${jidAttr}${phoneAttr}>${options.escapeHtml(labelToken)}</a>`;
        const placeholder = `${mentionPlaceholderPrefix}${mentionAnchors.length}__`;
        mentionAnchors.push(anchor);
        return `${prefix}${placeholder}`;
      });
    }

    const withMentionPlaceholders = escapedValue.replace(
      /(^|[\s(])(@(?:[0-9A-Za-z._-]{3,80}(?::[0-9A-Za-z._-]{1,20})?(?:@lid)?))(?=\s|$|[),.!?])/g,
      (_match: string, prefix: string, mention: string) => {
        const display = options.getMentionDisplayFromMetadata(messageEntry, mention);
        const renderedMention = `@${options.sanitizeHumanLabel(display || mention, {
          fallbackPhone: mention,
          fallbackLabel: "Contato"
        })}`;
        const routeTarget = options.resolveMentionRouteTarget(messageEntry, mention);
        const jidAttr = routeTarget.jid
          ? ` data-mention-jid="${options.escapeHtmlAttribute(routeTarget.jid)}"`
          : "";
        const phoneAttr = routeTarget.phone
          ? ` data-mention-phone="${options.escapeHtmlAttribute(routeTarget.phone)}"`
          : "";

        const anchor = `<a href="#" class="chat-message__mention chat-message__mention-link"${jidAttr}${phoneAttr}>${options.escapeHtml(renderedMention)}</a>`;
        const placeholder = `${mentionPlaceholderPrefix}${mentionAnchors.length}__`;
        mentionAnchors.push(anchor);
        return `${prefix}${placeholder}`;
      }
    );

    const withExternalLinks = withMentionPlaceholders.replace(
      /(?:https?:\/\/|www\.)[^\s<>"']+/gi,
      (rawLink: string) => {
        const sanitized = rawLink.replace(/[),.!?;:]+$/g, "");
        const trailing = rawLink.slice(sanitized.length);
        const href = options.normalizeLinkUrl(sanitized);
        if (!href) {
          return rawLink;
        }

        const external = `<a href="${options.escapeHtmlAttribute(href)}" class="chat-message__external-link" target="_blank" rel="noopener noreferrer">${options.escapeHtml(sanitized)}</a>`;
        return `${external}${options.escapeHtml(trailing)}`;
      }
    );

    const withMentionsRestored = withExternalLinks.replace(
      new RegExp(`${mentionPlaceholderPrefix}(\\d+)__`, "g"),
      (_match: string, indexValue: string) => {
        const index = Number(indexValue);
        return mentionAnchors[index] ?? "";
      }
    );

    return withMentionsRestored.replace(/\n/g, "<br>");
  }

  function disposeMessageRendering() {
    if (replyFocusTimer !== null) {
      window.clearTimeout(replyFocusTimer);
      replyFocusTimer = null;
    }
  }

  return {
    getMediaTypeLabel,
    hasUnsupportedNotice,
    getUnsupportedLabel,
    buildUnsupportedOpenUrl,
    resolveMessageType,
    isMentionAlertMessage,
    isStickerMessage,
    isVoiceNoteMessage,
    isAudioFileMessage,
    isAudioDocumentMessage,
    getReplyPreview,
    getRenderedMessageText,
    getMessageLinkPreview,
    shouldRenderMessageText,
    hasImagePreview,
    hasVideoPreview,
    hasAudioPreview,
    hasDocumentPreview,
    hasPendingMediaPreview,
    getPendingMediaLabel,
    getReplyAuthorLabel,
    getReplyTargetText,
    getReplyTargetAuthorLabel,
    hasReplyJumpTarget,
    onReplyPreviewClick,
    renderMessageHtml,
    disposeMessageRendering
  };
}
