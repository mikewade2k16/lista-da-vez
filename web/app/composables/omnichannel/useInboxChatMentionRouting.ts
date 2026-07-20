import type { GroupParticipant, Message } from "~/types";

export function useInboxChatMentionRouting(options: {
  getGroupParticipants: () => GroupParticipant[];
  getMessageRenderItems: () => Array<{ kind: string; message?: Message }>;
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
  function buildMentionJidCandidates(mentionToken: string) {
    const rawMention = (mentionToken.startsWith("@") ? mentionToken.slice(1) : mentionToken).trim();
    if (!rawMention) {
      return [];
    }

    const candidates = new Set<string>();
    const normalizedRaw = options.normalizeMentionJid(rawMention);
    if (normalizedRaw) {
      candidates.add(normalizedRaw);
    }

    if (rawMention.includes("@lid")) {
      candidates.add(rawMention);
    }

    const mentionDigits = options.normalizeDigits(rawMention);
    if (mentionDigits) {
      candidates.add(`${mentionDigits}@s.whatsapp.net`);
      candidates.add(`${mentionDigits}@lid`);
    }

    return [...candidates];
  }

  function getMentionDisplayFromMetadata(messageEntry: Message, mentionToken: string) {
    const metadata = options.asRecord(messageEntry.metadataJson);
    const mentions = metadata ? options.asRecord(metadata.mentions) : null;
    if (!mentions) {
      return null;
    }

    const mentionJidCandidates = buildMentionJidCandidates(mentionToken);
    const mentionedValue = mentionToken.startsWith("@") ? mentionToken.slice(1) : mentionToken;
    const mentionDigits = options.normalizeDigits(mentionedValue);

    const displayByPhone = options.asRecord(mentions.displayByPhone);
    if (displayByPhone && mentionDigits) {
      const direct = displayByPhone[mentionDigits];
      if (typeof direct === "string" && direct.trim().length > 0) {
        return options.sanitizeHumanLabel(direct.trim(), {
          fallbackPhone: mentionDigits,
          fallbackLabel: "Contato"
        });
      }

      for (const [phoneKey, value] of Object.entries(displayByPhone)) {
        if (typeof value !== "string") {
          continue;
        }

        const normalizedPhoneKey = options.normalizeDigits(phoneKey);
        if (!normalizedPhoneKey) {
          continue;
        }

        if (normalizedPhoneKey.endsWith(mentionDigits) || mentionDigits.endsWith(normalizedPhoneKey)) {
          const candidate = value.trim();
          if (candidate.length > 0) {
            return options.sanitizeHumanLabel(candidate, {
              fallbackPhone: mentionDigits,
              fallbackLabel: "Contato"
            });
          }
        }
      }
    }

    const displayByJid = options.asRecord(mentions.displayByJid);
    if (displayByJid) {
      for (const [jidKey, value] of Object.entries(displayByJid)) {
        if (typeof value !== "string") {
          continue;
        }

        const normalizedJid = options.normalizeMentionJid(jidKey);
        if (!normalizedJid || !mentionJidCandidates.includes(normalizedJid)) {
          continue;
        }

        const candidate = value.trim();
        if (candidate.length > 0) {
          return options.sanitizeHumanLabel(candidate, {
            fallbackPhone: mentionDigits || normalizedJid,
            fallbackLabel: "Contato"
          });
        }
      }

      if (!mentionDigits) {
        return null;
      }

      for (const [jidKey, value] of Object.entries(displayByJid)) {
        if (typeof value !== "string") {
          continue;
        }

        const jidDigits = options.normalizeDigits(jidKey);
        if (!jidDigits) {
          continue;
        }

        if (jidDigits.endsWith(mentionDigits) || mentionDigits.endsWith(jidDigits)) {
          const candidate = value.trim();
          if (candidate.length > 0) {
            return options.sanitizeHumanLabel(candidate, {
              fallbackPhone: mentionDigits,
              fallbackLabel: "Contato"
            });
          }
        }
      }
    }

    if (mentionDigits) {
      for (const participant of options.getGroupParticipants()) {
        const participantDigitsCandidates = [
          options.normalizeDigits(participant.phone || ""),
          options.normalizeDigits(participant.jid || "")
        ].filter((entry) => entry.length > 0);

        for (const participantDigits of participantDigitsCandidates) {
          if (participantDigits.endsWith(mentionDigits) || mentionDigits.endsWith(participantDigits)) {
            const candidate = participant.name?.trim();
            if (candidate) {
              return options.sanitizeHumanLabel(candidate, {
                fallbackPhone: participant.phone || participant.jid,
                fallbackLabel: "Contato"
              });
            }
          }
        }
      }

      for (const item of options.getMessageRenderItems()) {
        if (item.kind !== "message" || !item.message) {
          continue;
        }

        const metadataFromMessage = options.asRecord(item.message.metadataJson);
        const participantJidFromMessage = metadataFromMessage && typeof metadataFromMessage.participantJid === "string"
          ? metadataFromMessage.participantJid
          : "";
        if (!participantJidFromMessage) {
          continue;
        }

        const participantDigits = options.normalizeDigits(participantJidFromMessage);
        if (!participantDigits) {
          continue;
        }

        if (participantDigits.endsWith(mentionDigits) || mentionDigits.endsWith(participantDigits)) {
          const candidate = item.message.senderName?.trim();
          if (candidate) {
            return options.sanitizeHumanLabel(candidate, {
              fallbackPhone: participantDigits,
              fallbackLabel: "Contato"
            });
          }
        }
      }
    }

    if (mentionDigits) {
      return mentionDigits;
    }

    return "Contato";
  }

  function collectMentionDisplayEntries(messageEntry: Message) {
    const metadata = options.asRecord(messageEntry.metadataJson);
    const mentions = metadata ? options.asRecord(metadata.mentions) : null;
    if (!mentions) {
      return [];
    }

    const entries = new Map<string, { label: string; jid: string | null; phone: string | null }>();
    const upsertEntry = (labelValue: string, jidValue: string | null, phoneValue: string | null) => {
      const safeLabel = options.sanitizeHumanLabel(labelValue, {
        fallbackPhone: phoneValue ?? jidValue,
        fallbackLabel: "Contato"
      });
      const normalizedLabel = options.normalizeMentionLabel(safeLabel);
      if (!normalizedLabel) {
        return;
      }

      const current = entries.get(normalizedLabel);
      if (!current) {
        entries.set(normalizedLabel, {
          label: safeLabel.trim().replace(/\s+/g, " "),
          jid: jidValue,
          phone: phoneValue
        });
        return;
      }

      entries.set(normalizedLabel, {
        label: current.label,
        jid:
          current.jid && !current.jid.includes("@lid")
            ? current.jid
            : (jidValue && !jidValue.includes("@lid") ? jidValue : current.jid ?? jidValue),
        phone: current.phone ?? phoneValue
      });
    };

    const displayByJid = options.asRecord(mentions.displayByJid);
    if (displayByJid) {
      for (const [jidKey, value] of Object.entries(displayByJid)) {
        if (typeof value !== "string" || !value.trim()) {
          continue;
        }

        const normalizedJid = options.normalizeMentionJid(jidKey);
        const phone = options.normalizeDigits(jidKey) || null;
        upsertEntry(value, normalizedJid, phone);
      }
    }

    const displayByPhone = options.asRecord(mentions.displayByPhone);
    if (displayByPhone) {
      for (const [phoneKey, value] of Object.entries(displayByPhone)) {
        if (typeof value !== "string" || !value.trim()) {
          continue;
        }

        const phone = options.normalizeDigits(phoneKey) || null;
        upsertEntry(value, null, phone);
      }
    }

    return [...entries.values()];
  }

  function resolveMentionRouteTarget(messageEntry: Message, mentionToken: string) {
    const mentionedValue = mentionToken.startsWith("@") ? mentionToken.slice(1) : mentionToken;
    const mentionDigits = options.normalizeDigits(mentionedValue);
    const mentionJidCandidates = buildMentionJidCandidates(mentionToken);

    if (!mentionDigits) {
      const normalizedLabel = options.normalizeMentionLabel(mentionedValue);
      if (normalizedLabel) {
        for (const entry of collectMentionDisplayEntries(messageEntry)) {
          if (options.normalizeMentionLabel(entry.label) !== normalizedLabel) {
            continue;
          }

          if (entry.jid || entry.phone) {
            return {
              jid: entry.jid,
              phone: entry.phone
            };
          }
        }

        for (const participant of options.getGroupParticipants()) {
          if (options.normalizeMentionLabel(participant.name) !== normalizedLabel) {
            continue;
          }

          return {
            jid: options.normalizeMentionJid(participant.jid),
            phone: options.normalizeDigits(participant.phone || participant.jid) || null
          };
        }
      }
    }

    if (!mentionDigits) {
      if (mentionJidCandidates.length > 0) {
        return {
          jid: mentionJidCandidates[0],
          phone: null
        };
      }

      return {
        jid: null,
        phone: null
      };
    }

    const metadata = options.asRecord(messageEntry.metadataJson);
    const mentions = metadata ? options.asRecord(metadata.mentions) : null;

    const displayByJid = mentions ? options.asRecord(mentions.displayByJid) : null;
    if (displayByJid) {
      for (const [jidKey] of Object.entries(displayByJid)) {
        const normalizedJid = options.normalizeMentionJid(jidKey);
        if (!normalizedJid || !mentionJidCandidates.includes(normalizedJid)) {
          continue;
        }

        return {
          jid: normalizedJid,
          phone: mentionDigits || null
        };
      }

      for (const jidKey of Object.keys(displayByJid)) {
        const jidDigits = options.normalizeDigits(jidKey);
        if (!jidDigits) {
          continue;
        }

        if (jidDigits.endsWith(mentionDigits) || mentionDigits.endsWith(jidDigits)) {
          return {
            jid: options.normalizeMentionJid(jidKey),
            phone: mentionDigits
          };
        }
      }
    }

    const mentionedValues = mentions ? mentions.mentioned : null;
    if (Array.isArray(mentionedValues)) {
      for (const item of mentionedValues) {
        if (typeof item !== "string") {
          continue;
        }

        const normalizedMentionedJid = options.normalizeMentionJid(item);
        if (normalizedMentionedJid && mentionJidCandidates.includes(normalizedMentionedJid)) {
          return {
            jid: normalizedMentionedJid,
            phone: mentionDigits || null
          };
        }

        const jidDigits = options.normalizeDigits(item);
        if (!jidDigits) {
          continue;
        }

        if (jidDigits.endsWith(mentionDigits) || mentionDigits.endsWith(jidDigits)) {
          return {
            jid: options.normalizeMentionJid(item),
            phone: mentionDigits
          };
        }
      }
    }

    for (const participant of options.getGroupParticipants()) {
      const participantJid = options.normalizeMentionJid(participant.jid);
      if (!participantJid) {
        continue;
      }

      const digitsByPhone = options.normalizeDigits(participant.phone || "");
      const digitsByJid = options.normalizeDigits(participantJid);
      const hasPhoneMatch =
        Boolean(digitsByPhone) &&
        (digitsByPhone.endsWith(mentionDigits) || mentionDigits.endsWith(digitsByPhone));
      const hasJidMatch =
        Boolean(digitsByJid) &&
        (digitsByJid.endsWith(mentionDigits) || mentionDigits.endsWith(digitsByJid));

      if (hasPhoneMatch || hasJidMatch) {
        return {
          jid: participantJid,
          phone: mentionDigits
        };
      }
    }

    return {
      jid:
        mentionJidCandidates.find((candidate) => candidate.endsWith("@lid")) ??
        `${mentionDigits}@s.whatsapp.net`,
      phone: mentionDigits
    };
  }

  function onMessageTextClick(event: MouseEvent) {
    const target = event.target as HTMLElement | null;
    const mentionLink = target?.closest(".chat-message__mention-link") as HTMLElement | null;
    if (!mentionLink) {
      return;
    }

    event.preventDefault();

    const jid = mentionLink.dataset.mentionJid?.trim() || null;
    const phone = mentionLink.dataset.mentionPhone?.trim() || null;
    const text = mentionLink.textContent?.trim() ?? "";
    const label = text.replace(/^@/, "").trim() || null;

    options.onOpenMention({
      jid,
      phone,
      label
    });
  }

  return {
    getMentionDisplayFromMetadata,
    collectMentionDisplayEntries,
    resolveMentionRouteTarget,
    onMessageTextClick
  };
}
