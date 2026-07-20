import type { Message } from "~/types";

export interface MessageReactionEntry {
  actorKey: string;
  actorUserId: string | null;
  actorName: string | null;
  actorJid: string | null;
  emoji: string;
}

export interface MessageReactionBadge {
  emoji: string;
  count: number;
  reactedByCurrentUser: boolean;
  actors: string[];
}

const QUICK_REACTION_EMOJIS = ["👍", "❤️", "😂", "😮", "😢", "🙏"];

export function useInboxChatReactions(options: {
  getCurrentUserId: () => string | null;
  getCanManageConversation: () => boolean;
  asRecord: (value: unknown) => Record<string, unknown> | null;
  onSetReaction: (payload: { messageId: string; emoji: string | null }) => void;
}) {
  const SELF_ACTOR_KEY = "wa:self";

  function normalizeReactionEmoji(value: string | null | undefined) {
    const trimmed = typeof value === "string" ? value.trim() : "";
    if (!trimmed) {
      return null;
    }

    return [...trimmed].slice(0, 8).join("") || null;
  }

  function getMessageReactionEntries(messageEntry: Message) {
    const metadata = options.asRecord(messageEntry.metadataJson);
    const reactions = metadata ? options.asRecord(metadata.reactions) : null;
    const rawItems = Array.isArray(reactions?.items) ? reactions.items : [];
    const entries: MessageReactionEntry[] = [];

    for (const item of rawItems) {
      const record = options.asRecord(item);
      if (!record) {
        continue;
      }

      const actorKey = typeof record.actorKey === "string" ? record.actorKey.trim() : "";
      const emoji = normalizeReactionEmoji(typeof record.emoji === "string" ? record.emoji : null);
      if (!actorKey || !emoji) {
        continue;
      }

      entries.push({
        actorKey,
        actorUserId: typeof record.actorUserId === "string" ? record.actorUserId : null,
        actorName: typeof record.actorName === "string" ? record.actorName : null,
        actorJid: typeof record.actorJid === "string" ? record.actorJid : null,
        emoji
      });
    }

    return entries;
  }

  function getReactionBadges(messageEntry: Message) {
    const grouped = new Map<string, MessageReactionBadge>();
    const entries = getMessageReactionEntries(messageEntry);

    for (const entry of entries) {
      const existing = grouped.get(entry.emoji);
      const actorLabel = entry.actorName?.trim() || entry.actorJid?.trim() || "Contato";
      const reactedByCurrentUser = Boolean(
        entry.actorKey === SELF_ACTOR_KEY ||
        (entry.actorUserId && entry.actorUserId === options.getCurrentUserId())
      );

      if (!existing) {
        grouped.set(entry.emoji, {
          emoji: entry.emoji,
          count: 1,
          reactedByCurrentUser,
          actors: [actorLabel]
        });
        continue;
      }

      existing.count += 1;
      existing.reactedByCurrentUser = existing.reactedByCurrentUser || reactedByCurrentUser;
      if (!existing.actors.includes(actorLabel)) {
        existing.actors.push(actorLabel);
      }
    }

    return [...grouped.values()].sort((left, right) => right.count - left.count);
  }

  function getCurrentUserReaction(messageEntry: Message) {
    const entries = getMessageReactionEntries(messageEntry);
    const current = entries.find((entry) =>
      entry.actorKey === SELF_ACTOR_KEY ||
      entry.actorUserId === options.getCurrentUserId()
    );
    return current?.emoji ?? null;
  }

  function setMessageReaction(messageEntry: Message, emoji: string | null) {
    options.onSetReaction({
      messageId: messageEntry.id,
      emoji: normalizeReactionEmoji(emoji)
    });
  }

  function toggleReactionBadge(messageEntry: Message, badge: MessageReactionBadge) {
    if (!options.getCanManageConversation()) {
      return;
    }

    const currentReaction = getCurrentUserReaction(messageEntry);
    const nextEmoji = currentReaction === badge.emoji ? null : badge.emoji;
    setMessageReaction(messageEntry, nextEmoji);
  }

  function buildReactionMenuItems(messageEntry: Message) {
    const currentReaction = getCurrentUserReaction(messageEntry);
    const items = QUICK_REACTION_EMOJIS.map((emoji) => ({
      label: currentReaction === emoji ? `${emoji} Remover` : `${emoji}`,
      onSelect: () => {
        const nextEmoji = currentReaction === emoji ? null : emoji;
        setMessageReaction(messageEntry, nextEmoji);
      }
    }));

    if (currentReaction) {
      items.push({
        label: "Remover reacao",
        onSelect: () => setMessageReaction(messageEntry, null)
      });
    }

    return [items];
  }

  return {
    getReactionBadges,
    buildReactionMenuItems,
    toggleReactionBadge
  };
}
