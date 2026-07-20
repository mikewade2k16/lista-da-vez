import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch, type Ref } from "vue";
import type { EmojiCategoryDefinition } from "~/composables/omnichannel/useInboxChatEmojiCatalog";

type EmojiPanelTab = "emoji" | "gif" | "stickers";
type EmojiCategoryId = "recents" | string;

export function useInboxChatEmojiPanel(options: {
  getActiveConversation: () => unknown | null;
  getCanManageConversation: () => boolean;
  getDraft: () => string;
  setDraft: (value: string) => void;
  normalizeNameForComparison: (value: string) => string;
  recentEmojis: Ref<string[]>;
  emojiPickerCategoryDefinitions: Ref<EmojiCategoryDefinition[]>;
  emojiPickerSearchIndex: Ref<Map<string, string>>;
  gifSearch: Ref<string>;
  pushRecentEmoji: (emoji: string) => void;
  loadRecentEmojis: () => void;
  loadSavedStickers: () => Promise<void>;
  ensureEmojiPickerLoaded: () => Promise<void>;
  clearGifSearchTimer: () => void;
  queueGifSearch: () => void;
  closeContactPicker: () => void;
  focusEmojiSearchInput: () => void;
  isInsideEmojiPanel: (target: HTMLElement) => boolean;
  resolveComposerTextareaElement: () => HTMLTextAreaElement | null;
  pruneDraftMentionSelections: () => void;
  updateMentionContextFromCursor: () => void;
}) {
  const emojiPanelTab = ref<EmojiPanelTab>("emoji");
  const emojiSearch = ref("");
  const emojiCategory = ref<EmojiCategoryId>("recents");
  const emojiPanelOpen = ref(false);

  const emojiCategoryDefinitionsMap = computed(() => {
    const map = new Map<EmojiCategoryId, { label: string; icon: string; emojis: string[] }>();
    map.set("recents", {
      label: "Recentes",
      icon: "\u{1F558}",
      emojis: options.recentEmojis.value.length > 0
        ? options.recentEmojis.value
        : (options.emojiPickerCategoryDefinitions.value[0]?.emojis.slice(0, 24) ?? [])
    });

    for (const entry of options.emojiPickerCategoryDefinitions.value) {
      map.set(entry.id, {
        label: entry.label,
        icon: entry.icon,
        emojis: entry.emojis
      });
    }

    return map;
  });

  const emojiCategories = computed(() => {
    return (["recents", ...options.emojiPickerCategoryDefinitions.value.map((entry) => entry.id)] as EmojiCategoryId[])
      .map((id) => {
        const definition = emojiCategoryDefinitionsMap.value.get(id);
        if (!definition) {
          return null;
        }

        return {
          id,
          label: definition.label,
          icon: definition.icon,
          emojis: definition.emojis
        };
      })
      .filter((entry): entry is { id: EmojiCategoryId; label: string; icon: string; emojis: string[] } => Boolean(entry));
  });

  const showEmojiCategoryTabs = computed(() => emojiSearch.value.trim().length === 0);

  const filteredEmojiList = computed(() => {
    const normalizedQuery = options.normalizeNameForComparison(emojiSearch.value);
    const categories = emojiCategories.value;

    if (!normalizedQuery) {
      const activeCategory = categories.find((entry) => entry.id === emojiCategory.value) || categories[0];
      return activeCategory?.emojis ?? [];
    }

    const uniqueEmojis = new Set<string>();
    for (const category of categories) {
      for (const emoji of category.emojis) {
        uniqueEmojis.add(emoji);
      }
    }

    return [...uniqueEmojis].filter((emoji) => {
      if (emoji.includes(normalizedQuery)) {
        return true;
      }

      const searchText = options.emojiPickerSearchIndex.value.get(emoji) ?? "";
      return searchText.includes(normalizedQuery);
    });
  });

  const activeEmojiCategoryLabel = computed(() => {
    const activeCategory = emojiCategories.value.find((entry) => entry.id === emojiCategory.value);
    return activeCategory?.label ?? "Emojis";
  });

  watch(
    () => emojiCategories.value.map((entry) => entry.id),
    (ids) => {
      if (ids.length === 0) {
        return;
      }

      if (!ids.includes(emojiCategory.value)) {
        emojiCategory.value = ids[0];
      }
    },
    { immediate: true }
  );

  watch(
    () => emojiSearch.value,
    (value) => {
      if (value.trim().length > 0) {
        emojiPanelTab.value = "emoji";
      }
    }
  );

  watch(
    () => emojiPanelTab.value,
    (value) => {
      if (value === "gif") {
        options.queueGifSearch();
        return;
      }

      if (value === "stickers") {
        void options.loadSavedStickers();
      }
    }
  );

  watch(
    () => options.gifSearch.value,
    () => {
      if (emojiPanelTab.value === "gif") {
        options.queueGifSearch();
      }
    }
  );

  function updateEmojiPanelTab(value: EmojiPanelTab) {
    emojiPanelTab.value = value;
  }

  function updateEmojiSearch(value: string) {
    emojiSearch.value = value;
  }

  function updateEmojiCategory(value: EmojiCategoryId) {
    emojiCategory.value = value;
  }

  function closeEmojiPanel(panelOptions: { resetSearch?: boolean } = {}) {
    emojiPanelOpen.value = false;
    options.clearGifSearchTimer();
    if (panelOptions.resetSearch !== false) {
      emojiSearch.value = "";
      options.gifSearch.value = "";
    }
  }

  function toggleEmojiPanel() {
    if (!options.getActiveConversation() || !options.getCanManageConversation()) {
      return;
    }

    options.closeContactPicker();
    emojiPanelOpen.value = !emojiPanelOpen.value;
    emojiPanelTab.value = "emoji";
    if (emojiPanelOpen.value) {
      void options.ensureEmojiPickerLoaded();
      void nextTick(() => {
        options.focusEmojiSearchInput();
      });
    }
  }

  function insertTextAtComposerCursor(value: string) {
    const textareaElement = options.resolveComposerTextareaElement();
    if (!textareaElement) {
      options.setDraft(`${options.getDraft()}${value}`);
      return;
    }

    const draftValue = options.getDraft();
    const start = textareaElement.selectionStart ?? draftValue.length;
    const end = textareaElement.selectionEnd ?? start;
    const nextDraft = `${draftValue.slice(0, start)}${value}${draftValue.slice(end)}`;
    options.setDraft(nextDraft);

    void nextTick(() => {
      const target = options.resolveComposerTextareaElement();
      if (!target) {
        return;
      }

      const nextCursor = start + value.length;
      target.focus();
      target.setSelectionRange(nextCursor, nextCursor);
      options.pruneDraftMentionSelections();
      options.updateMentionContextFromCursor();
    });
  }

  function insertEmoji(emoji: string) {
    insertTextAtComposerCursor(emoji);
    options.pushRecentEmoji(emoji);
  }

  function onEmojiPanelPointerDownOutside(event: MouseEvent) {
    if (!emojiPanelOpen.value) {
      return;
    }

    const target = event.target as HTMLElement | null;
    if (!target) {
      return;
    }

    if (options.isInsideEmojiPanel(target)) {
      return;
    }

    closeEmojiPanel();
  }

  onMounted(() => {
    options.loadRecentEmojis();
    void options.loadSavedStickers();
    if (import.meta.client) {
      document.addEventListener("mousedown", onEmojiPanelPointerDownOutside);
    }
  });

  onBeforeUnmount(() => {
    closeEmojiPanel();
    if (import.meta.client) {
      document.removeEventListener("mousedown", onEmojiPanelPointerDownOutside);
    }
  });

  return {
    emojiPanelTab,
    emojiSearch,
    emojiCategory,
    emojiPanelOpen,
    emojiCategories,
    showEmojiCategoryTabs,
    filteredEmojiList,
    activeEmojiCategoryLabel,
    updateEmojiPanelTab,
    updateEmojiSearch,
    updateEmojiCategory,
    toggleEmojiPanel,
    closeEmojiPanel,
    insertEmoji
  };
}
