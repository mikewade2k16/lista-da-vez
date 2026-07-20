import { ref } from "vue";

type EmojiCategoryId = "recents" | string;

export type EmojiCategoryDefinition = {
  id: Exclude<EmojiCategoryId, "recents">;
  label: string;
  icon: string;
  emojis: string[];
};

const EMOJI_RECENTS_STORAGE_KEY = "omni:emoji-recents:v1";
const EMOJI_RECENTS_MAX = 24;

const EMOJI_KEYWORDS: Record<string, string[]> = {
  "😂": ["riso", "engracado", "kkk", "feliz"],
  "🤣": ["rir", "meme", "kkk"],
  "❤️": ["amor", "coracao", "love"],
  "😍": ["amor", "apaixonado", "lindo"],
  "😘": ["beijo", "amor"],
  "🔥": ["fogo", "top", "quente"],
  "😭": ["choro", "triste"],
  "🙏": ["obrigado", "gratidao", "fe", "amem"],
  "👍": ["ok", "joinha", "aprovado"],
  "👏": ["palmas", "parabens"],
  "🎉": ["festa", "comemorar"],
  "🎂": ["aniversario", "bolo"],
  "🥳": ["festa", "parabens"],
  "✅": ["certo", "confirmado", "check"],
  "❌": ["erro", "nao"],
  "⚠️": ["atencao", "alerta"],
  "💬": ["mensagem", "chat"],
  "📎": ["arquivo", "anexo"],
  "📞": ["telefone", "ligar"],
  "📍": ["local", "endereco"],
  "🎧": ["audio", "musica", "fone"],
  "🎤": ["voz", "microfone"]
};

const EMOJI_PICKER_CATEGORY_META: Record<string, { label: string; icon: string }> = {
  people: { label: "Carinhas e pessoas", icon: "\u{1F600}" },
  nature: { label: "Natureza", icon: "\u{1F33F}" },
  foods: { label: "Comidas", icon: "\u{1F354}" },
  activity: { label: "Atividades", icon: "\u{26BD}" },
  places: { label: "Lugares", icon: "\u{1F5FA}" },
  objects: { label: "Objetos", icon: "\u{1F4A1}" },
  symbols: { label: "Simbolos", icon: "\u{2705}" },
  flags: { label: "Bandeiras", icon: "\u{1F3F3}" }
};

const EMOJI_PICKER_CATEGORY_IDS = ["people", "nature", "foods", "activity", "places", "objects", "symbols", "flags"];

export function useInboxChatEmojiCatalog(options: {
  normalizeNameForComparison: (value: string) => string;
}) {
  const recentEmojis = ref<string[]>([]);
  const emojiPickerCategoryDefinitions = ref<EmojiCategoryDefinition[]>([]);
  const emojiPickerSearchIndex = ref<Map<string, string>>(new Map());
  const emojiPickerLoading = ref(false);
  const emojiPickerError = ref("");

  let emojiPickerLoadPromise: Promise<void> | null = null;

  function loadRecentEmojis() {
    if (!import.meta.client) {
      recentEmojis.value = [];
      return;
    }

    const raw = localStorage.getItem(EMOJI_RECENTS_STORAGE_KEY);
    if (!raw) {
      recentEmojis.value = [];
      return;
    }

    try {
      const parsed = JSON.parse(raw) as unknown;
      if (!Array.isArray(parsed)) {
        recentEmojis.value = [];
        return;
      }

      recentEmojis.value = parsed
        .filter((entry): entry is string => typeof entry === "string" && entry.trim().length > 0)
        .slice(0, EMOJI_RECENTS_MAX);
    } catch {
      recentEmojis.value = [];
    }
  }

  function persistRecentEmojis() {
    if (!import.meta.client) {
      return;
    }

    localStorage.setItem(EMOJI_RECENTS_STORAGE_KEY, JSON.stringify(recentEmojis.value.slice(0, EMOJI_RECENTS_MAX)));
  }

  function pushRecentEmoji(emoji: string) {
    const next = [emoji, ...recentEmojis.value.filter((entry) => entry !== emoji)].slice(0, EMOJI_RECENTS_MAX);
    recentEmojis.value = next;
    persistRecentEmojis();
  }

  async function ensureEmojiPickerLoaded() {
    if (emojiPickerCategoryDefinitions.value.length > 0) {
      return;
    }

    if (emojiPickerLoadPromise) {
      await emojiPickerLoadPromise;
      return;
    }

    emojiPickerLoading.value = true;
    emojiPickerError.value = "";

    emojiPickerLoadPromise = import("@emoji-mart/data/sets/15/native.json")
      .then((module) => {
        const dataset = (module.default ?? module) as {
          categories?: Array<{ id?: string; emojis?: string[] }>;
          emojis?: Record<string, { id?: string; name?: string; keywords?: string[]; skins?: Array<{ native?: string }> }>;
          aliases?: Record<string, string>;
        };
        const categoryDefinitions: EmojiCategoryDefinition[] = [];
        const searchIndex = new Map<string, string>();

        for (const categoryId of EMOJI_PICKER_CATEGORY_IDS) {
          const category = (dataset.categories ?? []).find((entry) => entry.id === categoryId);
          if (!category?.emojis?.length) {
            continue;
          }

          const categoryEmojis: string[] = [];
          for (const rawEmojiId of category.emojis) {
            const emojiId = dataset.aliases?.[rawEmojiId] ?? rawEmojiId;
            const emojiEntry = dataset.emojis?.[emojiId];
            const nativeEmoji = emojiEntry?.skins?.[0]?.native?.trim();
            if (!nativeEmoji) {
              continue;
            }

            const searchTerms = [
              rawEmojiId,
              emojiId,
              emojiEntry?.id,
              emojiEntry?.name,
              ...(emojiEntry?.keywords ?? []),
              ...(EMOJI_KEYWORDS[nativeEmoji] ?? [])
            ]
              .filter((value): value is string => typeof value === "string" && value.trim().length > 0)
              .map((value) => options.normalizeNameForComparison(value));

            categoryEmojis.push(nativeEmoji);

            if (!searchIndex.has(nativeEmoji)) {
              searchIndex.set(nativeEmoji, searchTerms.join(" "));
            }
          }

          if (categoryEmojis.length === 0) {
            continue;
          }

          categoryDefinitions.push({
            id: categoryId,
            label: EMOJI_PICKER_CATEGORY_META[categoryId]?.label ?? categoryId,
            icon: EMOJI_PICKER_CATEGORY_META[categoryId]?.icon ?? categoryEmojis[0],
            emojis: categoryEmojis
          });
        }

        emojiPickerCategoryDefinitions.value = categoryDefinitions;
        emojiPickerSearchIndex.value = searchIndex;
      })
      .catch((error) => {
        emojiPickerCategoryDefinitions.value = [];
        emojiPickerSearchIndex.value = new Map();
        emojiPickerError.value = error instanceof Error
          ? error.message
          : "Nao foi possivel carregar a base de emojis.";
      })
      .finally(() => {
        emojiPickerLoading.value = false;
        emojiPickerLoadPromise = null;
      });

    await emojiPickerLoadPromise;
  }

  return {
    recentEmojis,
    emojiPickerCategoryDefinitions,
    emojiPickerSearchIndex,
    emojiPickerLoading,
    emojiPickerError,
    loadRecentEmojis,
    pushRecentEmoji,
    ensureEmojiPickerLoaded
  };
}
