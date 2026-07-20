import { computed } from "vue";
import type { Conversation, GroupParticipant, Message } from "~/types";
import { resolveAvatarSource } from "~/composables/omnichannel/useAvatarProxy";

type GroupAvatarLookup = {
  byJid: Map<string, string>;
  byDigits: Map<string, string>;
  byName: Map<string, string>;
};

export function useInboxChatMessageIdentity(options: {
  getActiveConversation: () => Conversation | null;
  getCurrentUserId: () => string | null;
  getCurrentUserName: () => string | null;
  getShowOutboundOperatorLabel: () => boolean;
  getIsGroupConversation: () => boolean;
  getGroupParticipants: () => GroupParticipant[];
  getMessageRenderItems: () => Array<{ kind: string; message?: Message }>;
  asRecord: (value: unknown) => Record<string, unknown> | null;
  normalizeDigits: (value: string) => string;
  normalizeJidForComparison: (value: string) => string;
  normalizeNameForComparison: (value: string) => string;
  sanitizeHumanLabel: (
    value: string | null | undefined,
    options?: { fallbackPhone?: string | null | undefined; fallbackLabel?: string }
  ) => string;
}) {
  function isGenericOutboundSenderLabel(value: string | null | undefined) {
    const normalized = options.normalizeNameForComparison(value || "");
    if (!normalized) {
      return true;
    }

    return ["voce", "atendente", "agente", "agent", "operator", "operador"].includes(normalized);
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

  function isWeakOutboundSenderLabel(value: string | null | undefined) {
    return isGenericOutboundSenderLabel(value) || isTechnicalWhatsAppIdentifier(value);
  }

  function resolveCurrentUserDisplayName() {
    return options.sanitizeHumanLabel(options.getCurrentUserName(), {
      fallbackLabel: ""
    });
  }

  function isLikelyCurrentUserLabel(value: string | null | undefined) {
    const normalizedValue = options.normalizeNameForComparison(value || "");
    if (!normalizedValue) {
      return false;
    }

    if (isGenericOutboundSenderLabel(value)) {
      return true;
    }

    const currentUserLabel = resolveCurrentUserDisplayName();
    const normalizedCurrentUser = options.normalizeNameForComparison(currentUserLabel || "");
    if (!normalizedCurrentUser) {
      return false;
    }

    return normalizedValue === normalizedCurrentUser;
  }

  function resolveMessageParticipantJid(messageEntry: Message) {
    const metadata = options.asRecord(messageEntry.metadataJson);
    if (!metadata) {
      return null;
    }

    const participantJid = metadata.participantJid;
    if (typeof participantJid === "string" && participantJid.trim().length > 0) {
      return participantJid.trim();
    }

    return null;
  }

  const groupAvatarLookup = computed<GroupAvatarLookup>(() => {
    const byJid = new Map<string, string>();
    const byDigits = new Map<string, string>();
    const byName = new Map<string, string>();

    const registerAvatar = (params: { jid?: string | null; phone?: string | null; name?: string | null; avatarUrl?: string | null }) => {
      const avatarUrl = resolveAvatarSource(params.avatarUrl?.trim());
      if (!avatarUrl) {
        return;
      }

      const normalizedJid = options.normalizeJidForComparison(params.jid || "");
      if (normalizedJid) {
        byJid.set(normalizedJid, avatarUrl);
        const jidDigits = options.normalizeDigits(normalizedJid);
        if (jidDigits) {
          byDigits.set(jidDigits, avatarUrl);
        }
      }

      const phoneDigits = options.normalizeDigits(params.phone || "");
      if (phoneDigits) {
        byDigits.set(phoneDigits, avatarUrl);
      }

      const normalizedName = options.normalizeNameForComparison(params.name || "");
      if (normalizedName) {
        byName.set(normalizedName, avatarUrl);
      }
    };

    for (const participant of options.getGroupParticipants()) {
      registerAvatar({
        jid: participant.jid,
        phone: participant.phone,
        name: participant.name,
        avatarUrl: participant.avatarUrl
      });
    }

    for (const item of options.getMessageRenderItems()) {
      if (item.kind !== "message" || !item.message) {
        continue;
      }

      registerAvatar({
        jid: resolveMessageParticipantJid(item.message),
        name: item.message.senderName,
        avatarUrl: item.message.senderAvatarUrl
      });
    }

    return { byJid, byDigits, byName };
  });

  function findAvatarByDigits(digits: string) {
    if (!digits) {
      return null;
    }

    const direct = groupAvatarLookup.value.byDigits.get(digits);
    if (direct) {
      return direct;
    }

    for (const [candidateDigits, avatarUrl] of groupAvatarLookup.value.byDigits.entries()) {
      if (candidateDigits.endsWith(digits) || digits.endsWith(candidateDigits)) {
        return avatarUrl;
      }
    }

    return null;
  }

  function resolveMatchedGroupParticipant(messageEntry: Message) {
    if (!options.getIsGroupConversation()) {
      return null;
    }

    const participantJid = resolveMessageParticipantJid(messageEntry);
    const normalizedParticipantJid = options.normalizeJidForComparison(participantJid || "");
    if (normalizedParticipantJid) {
      for (const participant of options.getGroupParticipants()) {
        const candidateJid = options.normalizeJidForComparison(participant.jid || "");
        if (!candidateJid) {
          continue;
        }

        if (
          candidateJid === normalizedParticipantJid ||
          candidateJid.endsWith(normalizedParticipantJid) ||
          normalizedParticipantJid.endsWith(candidateJid)
        ) {
          return participant;
        }
      }
    }

    const participantDigits = options.normalizeDigits(participantJid || "");
    if (participantDigits) {
      for (const participant of options.getGroupParticipants()) {
        const candidateDigits = [
          options.normalizeDigits(participant.phone || ""),
          options.normalizeDigits(participant.jid || "")
        ].filter((entry) => entry.length > 0);

        if (
          candidateDigits.some((candidate) =>
            candidate === participantDigits ||
            candidate.endsWith(participantDigits) ||
            participantDigits.endsWith(candidate)
          )
        ) {
          return participant;
        }
      }
    }

    const normalizedSenderName = options.normalizeNameForComparison(messageEntry.senderName || "");
    if (!normalizedSenderName) {
      return null;
    }

    for (const participant of options.getGroupParticipants()) {
      const candidateName = options.normalizeNameForComparison(participant.name || "");
      if (!candidateName) {
        continue;
      }

      if (
        candidateName === normalizedSenderName ||
        candidateName.includes(normalizedSenderName) ||
        normalizedSenderName.includes(candidateName)
      ) {
        return participant;
      }
    }

    return null;
  }

  function resolveMessageAuthor(messageEntry: Message) {
    if (messageEntry.direction === "OUTBOUND") {
      const currentUserLabel = resolveCurrentUserDisplayName();
      const rawSenderName = messageEntry.senderName?.trim() ?? "";
      const senderName = options.sanitizeHumanLabel(rawSenderName, {
        fallbackLabel: ""
      });
      const senderLooksWeak = isWeakOutboundSenderLabel(rawSenderName) || isWeakOutboundSenderLabel(senderName);

      if (currentUserLabel && (!senderName || senderLooksWeak || isLikelyCurrentUserLabel(rawSenderName))) {
        return currentUserLabel;
      }

      if (senderName && !senderLooksWeak) {
        return senderName;
      }

      if (currentUserLabel) {
        return currentUserLabel;
      }

      return "Atendente";
    }

    if (options.getIsGroupConversation()) {
      const matchedParticipant = resolveMatchedGroupParticipant(messageEntry);
      const matchedParticipantName = matchedParticipant?.name?.trim();
      if (matchedParticipantName) {
        return options.sanitizeHumanLabel(matchedParticipantName, {
          fallbackPhone: matchedParticipant?.phone || matchedParticipant?.jid,
          fallbackLabel: "Participante"
        });
      }

      const matchedParticipantPhone = options.normalizeDigits(matchedParticipant?.phone || matchedParticipant?.jid || "");
      if (matchedParticipantPhone) {
        return matchedParticipantPhone;
      }

      const senderName = messageEntry.senderName?.trim();
      if (senderName) {
        return options.sanitizeHumanLabel(senderName, {
          fallbackPhone: resolveMessageParticipantJid(messageEntry),
          fallbackLabel: "Participante"
        });
      }

      const participantDigits = options.normalizeDigits(resolveMessageParticipantJid(messageEntry) || "");
      if (participantDigits) {
        return participantDigits;
      }
    }

    const activeConversation = options.getActiveConversation();
    const fallbackPhone =
      activeConversation?.contactPhone ||
      activeConversation?.externalId ||
      resolveMessageParticipantJid(messageEntry);
    const senderName = messageEntry.senderName?.trim() ?? "";
    const hasConversationContactName = Boolean(activeConversation?.contactName?.trim());

    if (senderName && !isLikelyCurrentUserLabel(senderName) && !isTechnicalWhatsAppIdentifier(senderName)) {
      return options.sanitizeHumanLabel(senderName, {
        fallbackPhone,
        fallbackLabel: "Contato"
      });
    }

    if (hasConversationContactName) {
      return options.sanitizeHumanLabel(activeConversation?.contactName, {
        fallbackPhone,
        fallbackLabel: "Contato"
      });
    }

    if (senderName && !isLikelyCurrentUserLabel(senderName)) {
      return options.sanitizeHumanLabel(senderName, {
        fallbackPhone,
        fallbackLabel: "Contato"
      });
    }

    return options.sanitizeHumanLabel(activeConversation?.contactName, {
      fallbackPhone,
      fallbackLabel: "Contato"
    });
  }

  function resolveOutboundOperatorLabel(messageEntry: Message) {
    if (messageEntry.direction !== "OUTBOUND") {
      return "";
    }

    if (!options.getShowOutboundOperatorLabel()) {
      return "";
    }

    const rawSenderName = messageEntry.senderName?.trim() ?? "";
    const senderName = options.sanitizeHumanLabel(rawSenderName, {
      fallbackLabel: ""
    });
    const senderLooksWeak = isWeakOutboundSenderLabel(rawSenderName) || isWeakOutboundSenderLabel(senderName);
    if (senderName && !senderLooksWeak) {
      return senderName;
    }

    const currentUserName = resolveCurrentUserDisplayName();
    if (currentUserName) {
      return currentUserName;
    }

    if (!messageEntry.senderUserId && !options.getCurrentUserId()) {
      return "";
    }

    return "Atendente";
  }

  function resolveGroupParticipantAvatar(messageEntry: Message) {
    if (!options.getIsGroupConversation()) {
      return undefined;
    }

    const participantJid = resolveMessageParticipantJid(messageEntry);
    const normalizedParticipantJid = options.normalizeJidForComparison(participantJid || "");
    if (normalizedParticipantJid) {
      const byJid = groupAvatarLookup.value.byJid.get(normalizedParticipantJid);
      if (byJid) {
        return byJid;
      }
    }

    const participantDigitsCandidates = [
      options.normalizeDigits(participantJid || ""),
      options.normalizeDigits(messageEntry.senderName || "")
    ].filter((entry) => entry.length > 0);

    for (const digits of participantDigitsCandidates) {
      const byDigits = findAvatarByDigits(digits);
      if (byDigits) {
        return byDigits;
      }
    }

    const normalizedSenderName = options.normalizeNameForComparison(messageEntry.senderName || "");
    if (normalizedSenderName) {
      const directByName = groupAvatarLookup.value.byName.get(normalizedSenderName);
      if (directByName) {
        return directByName;
      }

      for (const [candidateName, avatarUrl] of groupAvatarLookup.value.byName.entries()) {
        if (!candidateName) {
          continue;
        }

        if (
          candidateName === normalizedSenderName ||
          candidateName.includes(normalizedSenderName) ||
          normalizedSenderName.includes(candidateName)
        ) {
          return avatarUrl;
        }
      }
    }

    return undefined;
  }

  function resolveMessageAvatarUrl(messageEntry: Message) {
    const activeConversation = options.getActiveConversation();
    const isGroupConversation = options.getIsGroupConversation();
    if (messageEntry.direction === "OUTBOUND") {
      // Never trust provider echo/avatar for local outbound bubbles in direct chat.
      return undefined;
    }

    if (isGroupConversation) {
      const fromMessage = messageEntry.senderAvatarUrl?.trim();
      if (fromMessage) {
        return resolveAvatarSource(fromMessage);
      }

      const fromGroupParticipants = resolveGroupParticipantAvatar(messageEntry);
      if (fromGroupParticipants) {
        return resolveAvatarSource(fromGroupParticipants);
      }

      return undefined;
    }

    if (!activeConversation) {
      return undefined;
    }

    return resolveAvatarSource(activeConversation.contactAvatarUrl || undefined);
  }

  function resolveAudioAvatarUrl(messageEntry: Message) {
    return resolveMessageAvatarUrl(messageEntry);
  }

  function messageRowId(messageId: string) {
    return `chat-message-${messageId}`;
  }

  return {
    resolveMessageParticipantJid,
    resolveMatchedGroupParticipant,
    resolveMessageAuthor,
    resolveOutboundOperatorLabel,
    resolveMessageAvatarUrl,
    resolveAudioAvatarUrl,
    messageRowId
  };
}
