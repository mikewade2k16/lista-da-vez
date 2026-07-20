import { useInboxChatEmojiCatalog } from "~/composables/omnichannel/useInboxChatEmojiCatalog";
import { useInboxChatGifAssets } from "~/composables/omnichannel/useInboxChatGifAssets";
import { useInboxChatStickerAssets } from "~/composables/omnichannel/useInboxChatStickerAssets";

export { type EmojiCategoryDefinition } from "~/composables/omnichannel/useInboxChatEmojiCatalog";
export { type GifSearchResultItem } from "~/composables/omnichannel/useInboxChatGifAssets";
export { type SavedStickerItem } from "~/composables/omnichannel/useInboxChatStickerAssets";

export function useInboxChatEmojiAssets(options: {
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>;
  asRecord: (value: unknown) => Record<string, unknown> | null;
  normalizeNameForComparison: (value: string) => string;
  onPickAttachment: (payload: { file: File; mode: "sticker" | "gif" }) => void;
  onClosePanel: () => void;
}) {
  const emojiCatalog = useInboxChatEmojiCatalog({
    normalizeNameForComparison: options.normalizeNameForComparison
  });

  const gifAssets = useInboxChatGifAssets({
    onPickAttachment: (payload) => options.onPickAttachment(payload),
    onClosePanel: options.onClosePanel
  });

  const stickerAssets = useInboxChatStickerAssets({
    apiFetch: options.apiFetch,
    asRecord: options.asRecord,
    onPickAttachment: (payload) => options.onPickAttachment(payload),
    onClosePanel: options.onClosePanel
  });

  return {
    ...emojiCatalog,
    ...gifAssets,
    ...stickerAssets
  };
}
