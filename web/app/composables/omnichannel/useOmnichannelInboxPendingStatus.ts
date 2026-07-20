import type { Ref } from "vue";
import type { Message } from "~/types";
import type { MessagesPageResponse } from "~/composables/omnichannel/useOmnichannelInboxShared";

export function useOmnichannelInboxPendingStatus(options: {
  messages: Ref<Message[]>;
  fetchMessagesPage: (conversationId: string, beforeMessageId?: string) => Promise<MessagesPageResponse>;
  normalizeMessage: (messageEntry: Message) => Message;
  mergeMessages: (...chunks: Message[][]) => Message[];
  updateConversationPreviewFromMessage: (messageEntry: Message) => void;
}) {
  function wait(ms: number) {
    return new Promise((resolve) => {
      window.setTimeout(resolve, ms);
    });
  }

  async function reconcilePendingMessageStatus(conversationId: string, messageId: string) {
    const delays = [1_500, 3_000, 6_000, 10_000, 20_000];

    for (const delay of delays) {
      await wait(delay);

      try {
        const response = await options.fetchMessagesPage(conversationId);
        const target = response.messages.find((entry) => entry.id === messageId);
        if (!target) {
          continue;
        }

        const normalizedTarget = options.normalizeMessage(target);
        options.messages.value = options.mergeMessages(options.messages.value, [normalizedTarget]);
        options.updateConversationPreviewFromMessage(normalizedTarget);

        if (normalizedTarget.status !== "PENDING") {
          return;
        }
      } catch {
        // Polling fallback is best-effort when realtime updates are delayed.
      }
    }
  }

  return {
    reconcilePendingMessageStatus
  };
}
