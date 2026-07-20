import { computed, nextTick, ref, watch } from "vue";
import type { GroupParticipant, Message } from "~/types";

export function useInboxChatComposerMentions(options: {
  getIsGroupConversation: () => boolean;
  getActiveConversation: () => unknown | null;
  getDraft: () => string;
  setDraft: (value: string) => void;
  getGroupParticipants: () => GroupParticipant[];
  getMessageRenderItems: () => Array<{ kind: string; message?: Message }>;
  resolveComposerTextareaElement: () => HTMLTextAreaElement | null;
  asRecord: (value: unknown) => Record<string, unknown> | null;
  sanitizeHumanLabel: (
    value: string | null | undefined,
    options?: { fallbackPhone?: string | null | undefined; fallbackLabel?: string }
  ) => string;
}) {
  const mentionAnchorStart = ref<number | null>(null);
  const mentionAnchorEnd = ref<number | null>(null);
  const mentionQuery = ref("");
  const mentionSelectedIndex = ref(0);
  const draftMentionSelections = ref<Array<{ jid: string; label: string }>>([]);

  const mentionOptions = computed(() => {
    const candidateMap = new Map<string, GroupParticipant>();
    for (const participant of options.getGroupParticipants()) {
      if (!participant?.jid) {
        continue;
      }

      const safeName = options.sanitizeHumanLabel(participant.name, {
        fallbackPhone: participant.phone || participant.jid,
        fallbackLabel: "Participante"
      });
      candidateMap.set(participant.jid, {
        ...participant,
        name: safeName
      });
    }

    for (const item of options.getMessageRenderItems()) {
      if (item.kind !== "message" || !item.message) {
        continue;
      }

      const metadata = options.asRecord(item.message.metadataJson);
      const participantJid = metadata && typeof metadata.participantJid === "string"
        ? metadata.participantJid
        : "";

      if (!participantJid) {
        continue;
      }

      const fallbackPhone = participantJid.replace(/\D/g, "");
      candidateMap.set(participantJid, {
        jid: participantJid,
        name: options.sanitizeHumanLabel(item.message.senderName?.trim(), {
          fallbackPhone: fallbackPhone || participantJid,
          fallbackLabel: "Participante"
        }),
        phone: fallbackPhone,
        avatarUrl: item.message.senderAvatarUrl
      });
    }

    const query = mentionQuery.value.trim().toLowerCase();
    const candidates = [...candidateMap.values()];

    const filtered = query.length === 0
      ? candidates
      : candidates.filter((entry) => {
        const normalizedName = entry.name.trim().toLowerCase();
        const normalizedPhone = entry.phone.replace(/\D/g, "");
        const normalizedJid = entry.jid.toLowerCase();

        return (
          normalizedName.includes(query) ||
          normalizedPhone.includes(query.replace(/\D/g, "")) ||
          normalizedJid.includes(query)
        );
      });

    return filtered
      .sort((left, right) => left.name.localeCompare(right.name, "pt-BR", { sensitivity: "base" }))
      .slice(0, 10);
  });

  const isMentionPickerVisible = computed(() => {
    if (!options.getIsGroupConversation() || !options.getActiveConversation()) {
      return false;
    }

    if (mentionAnchorStart.value === null || mentionAnchorEnd.value === null) {
      return false;
    }

    return mentionOptions.value.length > 0;
  });

  watch(
    () => options.getDraft(),
    (value) => {
      if (!value.trim()) {
        draftMentionSelections.value = [];
        resetMentionContext();
        return;
      }

      pruneDraftMentionSelections();
    }
  );

  watch(
    () => {
      const activeConversation = options.getActiveConversation() as { id?: string | null } | null;
      return activeConversation?.id ?? null;
    },
    () => {
      draftMentionSelections.value = [];
      resetMentionContext();
    }
  );

  watch(
    () => mentionOptions.value.length,
    (length) => {
      if (length <= 0) {
        mentionSelectedIndex.value = 0;
        return;
      }

      if (mentionSelectedIndex.value >= length) {
        mentionSelectedIndex.value = length - 1;
      }
    }
  );

  function resetMentionContext() {
    mentionAnchorStart.value = null;
    mentionAnchorEnd.value = null;
    mentionQuery.value = "";
    mentionSelectedIndex.value = 0;
  }

  function extractMentionQueryFromCursor(draftValue: string, cursorIndex: number) {
    if (cursorIndex <= 0 || cursorIndex > draftValue.length) {
      return null;
    }

    const left = draftValue.slice(0, cursorIndex);
    const mentionIndex = left.lastIndexOf("@");
    if (mentionIndex < 0) {
      return null;
    }

    if (mentionIndex > 0) {
      const previousChar = left.slice(mentionIndex - 1, mentionIndex);
      if (!/[\s(\[{]/.test(previousChar)) {
        return null;
      }
    }

    const query = left.slice(mentionIndex + 1);
    if (/[\n\r\t]/.test(query) || query.includes("@")) {
      return null;
    }

    if (/\s/.test(query)) {
      return null;
    }

    return {
      mentionIndex,
      query
    };
  }

  function updateMentionContextFromCursor() {
    const textareaElement = options.resolveComposerTextareaElement();
    if (!textareaElement) {
      resetMentionContext();
      return;
    }

    const cursorIndex = textareaElement.selectionStart ?? options.getDraft().length;
    const mentionContext = extractMentionQueryFromCursor(options.getDraft(), cursorIndex);
    if (!mentionContext) {
      resetMentionContext();
      return;
    }

    mentionAnchorStart.value = mentionContext.mentionIndex;
    mentionAnchorEnd.value = cursorIndex;
    mentionQuery.value = mentionContext.query;
    mentionSelectedIndex.value = 0;
  }

  function upsertDraftMentionSelection(jid: string, label: string) {
    const normalizedJid = jid.trim().toLowerCase();
    if (!normalizedJid) {
      return;
    }

    const next = draftMentionSelections.value.filter((entry) => entry.jid !== normalizedJid);
    next.push({
      jid: normalizedJid,
      label
    });
    draftMentionSelections.value = next;
  }

  function pruneDraftMentionSelections() {
    const draftValue = options.getDraft();
    draftMentionSelections.value = draftMentionSelections.value.filter((entry) => draftValue.includes(entry.label));
  }

  function pickMentionOption(option: GroupParticipant) {
    const start = mentionAnchorStart.value;
    const end = mentionAnchorEnd.value;
    if (start === null || end === null) {
      return;
    }

    const mentionLabel = `@${option.name}`;
    const currentDraft = options.getDraft();
    const nextDraft = `${currentDraft.slice(0, start)}${mentionLabel} ${currentDraft.slice(end)}`;
    options.setDraft(nextDraft);
    upsertDraftMentionSelection(option.jid, mentionLabel);
    resetMentionContext();

    void nextTick(() => {
      const textareaElement = options.resolveComposerTextareaElement();
      if (!textareaElement) {
        return;
      }

      const cursor = start + mentionLabel.length + 1;
      textareaElement.focus();
      textareaElement.setSelectionRange(cursor, cursor);
    });
  }

  function handleMentionKeydown(event: KeyboardEvent) {
    if (!isMentionPickerVisible.value) {
      return false;
    }

    if (event.key === "ArrowDown") {
      event.preventDefault();
      mentionSelectedIndex.value = (mentionSelectedIndex.value + 1) % mentionOptions.value.length;
      return true;
    }

    if (event.key === "ArrowUp") {
      event.preventDefault();
      mentionSelectedIndex.value =
        (mentionSelectedIndex.value - 1 + mentionOptions.value.length) % mentionOptions.value.length;
      return true;
    }

    if (event.key === "Enter" || event.key === "Tab") {
      event.preventDefault();
      const option = mentionOptions.value[mentionSelectedIndex.value];
      if (option) {
        pickMentionOption(option);
      }
      return true;
    }

    if (event.key === "Escape") {
      event.preventDefault();
      resetMentionContext();
      return true;
    }

    return false;
  }

  function buildMentionSendPayload() {
    pruneDraftMentionSelections();
    const mentionedJids = draftMentionSelections.value
      .filter((entry) => options.getDraft().includes(entry.label))
      .map((entry) => entry.jid);

    return [...new Set(mentionedJids)];
  }

  function clearMentionDraftState() {
    resetMentionContext();
    draftMentionSelections.value = [];
  }

  return {
    mentionQuery,
    mentionSelectedIndex,
    mentionOptions,
    isMentionPickerVisible,
    resetMentionContext,
    updateMentionContextFromCursor,
    pruneDraftMentionSelections,
    pickMentionOption,
    handleMentionKeydown,
    buildMentionSendPayload,
    clearMentionDraftState
  };
}
