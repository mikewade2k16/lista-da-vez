import type { GroupParticipant, Message } from "~/types";
import { useInboxChatComposerMentions } from "~/composables/omnichannel/useInboxChatComposerMentions";
import { useInboxChatMentionRouting } from "~/composables/omnichannel/useInboxChatMentionRouting";

export function useInboxChatMentions(options: {
  getIsGroupConversation: () => boolean;
  getActiveConversation: () => unknown | null;
  getDraft: () => string;
  setDraft: (value: string) => void;
  getGroupParticipants: () => GroupParticipant[];
  getMessageRenderItems: () => Array<{ kind: string; message?: Message }>;
  resolveComposerTextareaElement: () => HTMLTextAreaElement | null;
  resolveMessageParticipantJid: (messageEntry: Message) => string | null;
  asRecord: (value: unknown) => Record<string, unknown> | null;
  normalizeDigits: (value: string) => string;
  normalizeMentionJid: (value: string) => string;
  normalizeMentionLabel: (value: string) => string;
  sanitizeHumanLabel: (
    value: string | null | undefined,
    options?: { fallbackPhone?: string | null | undefined; fallbackLabel?: string }
  ) => string;
  onOpenMention: (payload: { jid: string | null; phone: string | null; label: string | null }) => void;
}) {
  const composerMentions = useInboxChatComposerMentions({
    getIsGroupConversation: options.getIsGroupConversation,
    getActiveConversation: options.getActiveConversation,
    getDraft: options.getDraft,
    setDraft: options.setDraft,
    getGroupParticipants: options.getGroupParticipants,
    getMessageRenderItems: options.getMessageRenderItems,
    resolveComposerTextareaElement: options.resolveComposerTextareaElement,
    asRecord: options.asRecord,
    sanitizeHumanLabel: options.sanitizeHumanLabel
  });

  const mentionRouting = useInboxChatMentionRouting({
    getGroupParticipants: options.getGroupParticipants,
    getMessageRenderItems: options.getMessageRenderItems,
    asRecord: options.asRecord,
    normalizeDigits: options.normalizeDigits,
    normalizeMentionJid: options.normalizeMentionJid,
    normalizeMentionLabel: options.normalizeMentionLabel,
    sanitizeHumanLabel: options.sanitizeHumanLabel,
    onOpenMention: options.onOpenMention
  });

  return {
    ...composerMentions,
    ...mentionRouting
  };
}
