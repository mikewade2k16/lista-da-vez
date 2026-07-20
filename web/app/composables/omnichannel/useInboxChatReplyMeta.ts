import type { MessageType } from "~/types";

type ReplyPreviewLike = {
  content: string;
  messageType: MessageType;
} | null;

export function useInboxChatReplyMeta(options: {
  getMediaTypeLabel: (type: MessageType | null | undefined) => string;
  normalizeNameForComparison: (value: string) => string;
}) {
  function isTextMessageType(messageType: MessageType | null | undefined) {
    return !messageType || messageType === "TEXT";
  }

  function shouldShowReplyTypeMeta(messageType: MessageType | null | undefined) {
    return !isTextMessageType(messageType);
  }

  function getReplyPreviewText(reply: ReplyPreviewLike) {
    if (!reply) {
      return "";
    }

    const content = reply.content.trim();
    if (isTextMessageType(reply.messageType)) {
      return content || "Mensagem";
    }

    if (!content) {
      return "";
    }

    const mediaLabel = options.getMediaTypeLabel(reply.messageType);
    if (options.normalizeNameForComparison(content) === options.normalizeNameForComparison(mediaLabel)) {
      return "";
    }

    return content;
  }

  function getReplyTypeIcon(messageType: MessageType | null | undefined) {
    if (messageType === "IMAGE") {
      return "i-lucide-image";
    }

    if (messageType === "VIDEO") {
      return "i-lucide-clapperboard";
    }

    if (messageType === "AUDIO") {
      return "i-lucide-audio-lines";
    }

    if (messageType === "DOCUMENT") {
      return "i-lucide-file-text";
    }

    return "i-lucide-message-square";
  }

  return {
    isTextMessageType,
    shouldShowReplyTypeMeta,
    getReplyPreviewText,
    getReplyTypeIcon
  };
}
