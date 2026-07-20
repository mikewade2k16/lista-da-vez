import type { ComputedRef, Ref } from "vue";
import type { Message } from "~/types";
import {
  asRecord,
  formatSendError,
  normalizeReactionEmoji
} from "~/composables/omnichannel/useOmnichannelInboxShared";

interface InboxReactionUser {
  id?: string | null;
  name?: string | null;
}

export function useOmnichannelInboxMessageReactions(options: {
  canManageConversation: ComputedRef<boolean>;
  activeConversationId: Ref<string | null>;
  messages: Ref<Message[]>;
  sendError: Ref<string>;
  user: Ref<InboxReactionUser | null>;
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>;
  normalizeMessage: (messageEntry: Message) => Message;
  mergeMessages: (...chunks: Message[][]) => Message[];
  updateConversationPreviewFromMessage: (messageEntry: Message) => void;
  loadConversationMessages: (
    conversationId: string,
    loadOptions?: { forceRefresh?: boolean }
  ) => Promise<void>;
}) {
  const SELF_ACTOR_KEY = "wa:self";

  function getMessageReactionItems(messageEntry: Message) {
    const metadata = asRecord(messageEntry.metadataJson);
    const reactions = metadata ? asRecord(metadata.reactions) : null;
    const items = Array.isArray(reactions?.items) ? reactions.items : [];
    return items
      .map((item) => asRecord(item))
      .filter((item): item is Record<string, unknown> => Boolean(item))
      .map((item) => ({
        actorKey: typeof item.actorKey === "string" ? item.actorKey : "",
        actorUserId: typeof item.actorUserId === "string" ? item.actorUserId : null,
        actorName: typeof item.actorName === "string" ? item.actorName : null,
        actorJid: typeof item.actorJid === "string" ? item.actorJid : null,
        emoji: normalizeReactionEmoji(typeof item.emoji === "string" ? item.emoji : null),
        updatedAt: typeof item.updatedAt === "string" ? item.updatedAt : new Date().toISOString(),
        source: item.source === "whatsapp" ? "whatsapp" : "agent"
      }))
      .filter((item) => item.actorKey.trim().length > 0 && Boolean(item.emoji));
  }

  function applyOptimisticReaction(params: {
    messageId: string;
    emoji: string | null;
    actorUserId: string | null;
    actorName: string | null;
  }) {
    const messageEntry = options.messages.value.find((entry) => entry.id === params.messageId);
    if (!messageEntry) {
      return;
    }

    const currentItems = getMessageReactionItems(messageEntry).filter(
      (entry) => (
        entry.actorKey !== SELF_ACTOR_KEY &&
        !(params.actorUserId && entry.actorUserId === params.actorUserId)
      )
    );

    if (params.emoji) {
      currentItems.push({
        actorKey: SELF_ACTOR_KEY,
        actorUserId: params.actorUserId,
        actorName: params.actorName,
        actorJid: null,
        emoji: params.emoji,
        updatedAt: new Date().toISOString(),
        source: "agent"
      });
    }

    const summary = currentItems.reduce<Record<string, number>>((accumulator, entry) => {
      const emoji = entry.emoji;
      if (!emoji) {
        return accumulator;
      }
      accumulator[emoji] = (accumulator[emoji] ?? 0) + 1;
      return accumulator;
    }, {});

    const metadata = asRecord(messageEntry.metadataJson) ?? {};
    const updatedMessage = options.normalizeMessage({
      ...messageEntry,
      metadataJson: {
        ...metadata,
        reactions: {
          items: currentItems,
          summary,
          updatedAt: new Date().toISOString()
        }
      },
      updatedAt: new Date().toISOString()
    });

    options.messages.value = options.mergeMessages(options.messages.value, [updatedMessage]);
    options.updateConversationPreviewFromMessage(updatedMessage);
  }

  async function reactToMessage(params: { messageId: string; emoji: string | null }) {
    if (!options.canManageConversation.value) {
      options.sendError.value = "Seu perfil e somente leitura nesta inbox.";
      return;
    }

    const conversationId = options.activeConversationId.value;
    if (!conversationId) {
      return;
    }

    const normalizedEmoji = normalizeReactionEmoji(params.emoji);
    options.sendError.value = "";

    applyOptimisticReaction({
      messageId: params.messageId,
      emoji: normalizedEmoji,
      actorUserId: options.user.value?.id ?? null,
      actorName: options.user.value?.name ?? null
    });

    try {
      const updated = await options.apiFetch<Message>(
        `/conversations/${conversationId}/messages/${params.messageId}/reaction`,
        {
          method: "POST",
          body: {
            emoji: normalizedEmoji
          }
        }
      );

      options.messages.value = options.mergeMessages(options.messages.value, [updated]);
      options.updateConversationPreviewFromMessage(updated);
    } catch (error) {
      options.sendError.value = formatSendError(error, "Nao foi possivel enviar a reacao.");
      void options.loadConversationMessages(conversationId, { forceRefresh: true });
    }
  }

  return {
    reactToMessage
  };
}
