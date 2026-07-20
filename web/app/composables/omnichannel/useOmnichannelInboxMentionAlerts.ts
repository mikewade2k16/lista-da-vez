import type { Ref } from "vue";
import type { Conversation, Message } from "~/types";
import { asRecord } from "~/composables/omnichannel/useOmnichannelInboxShared";

export function useOmnichannelInboxMentionAlerts(options: {
  conversations: Ref<Conversation[]>;
  mentionAlertState: Ref<Record<string, number>>;
}) {
  function incrementMentionAlert(conversationId: string, amount = 1) {
    if (!conversationId) {
      return;
    }

    const normalizedAmount = Number.isFinite(amount) ? Math.max(1, Math.trunc(amount)) : 1;
    const currentCount = options.mentionAlertState.value[conversationId] ?? 0;

    options.mentionAlertState.value = {
      ...options.mentionAlertState.value,
      [conversationId]: currentCount + normalizedAmount
    };
  }

  function clearMentionAlert(conversationId: string) {
    if (!conversationId) {
      return;
    }

    if (!options.mentionAlertState.value[conversationId]) {
      return;
    }

    const nextState = {
      ...options.mentionAlertState.value
    };
    delete nextState[conversationId];
    options.mentionAlertState.value = nextState;
  }

  function messageHasMentions(messageEntry: Message) {
    const metadata = asRecord(messageEntry.metadataJson);
    const mentions = metadata ? asRecord(metadata.mentions) : null;
    if (!mentions) {
      return false;
    }

    if (mentions.everyOne === true) {
      return true;
    }

    return Array.isArray(mentions.mentioned) && mentions.mentioned.length > 0;
  }

  function shouldFlagMentionAlert(messageEntry: Message) {
    if (messageEntry.direction !== "INBOUND") {
      return false;
    }

    const conversationEntry = options.conversations.value.find((entry) => entry.id === messageEntry.conversationId);
    if (!conversationEntry?.externalId.endsWith("@g.us")) {
      return false;
    }

    return messageHasMentions(messageEntry);
  }

  return {
    incrementMentionAlert,
    clearMentionAlert,
    shouldFlagMentionAlert
  };
}
