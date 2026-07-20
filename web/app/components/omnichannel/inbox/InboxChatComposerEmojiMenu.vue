<script setup lang="ts">
import { UButton, UIcon, UInput } from "#components";
import { computed } from "vue";

const {
  activeConversation,
  canManageConversation,
  emojiPanelOpen,
  emojiPanelRef,
  emojiSearchInputRef,
  emojiTriggerRef,
  toggleEmojiPanel,
  EMOJI_PANEL_TAB_ITEMS,
  emojiPanelTab,
  onUpdateEmojiPanelTab,
  emojiSearch,
  onUpdateEmojiSearch,
  emojiPickerLoading,
  emojiPickerError,
  showEmojiCategoryTabs,
  emojiCategories,
  emojiCategory,
  updateEmojiCategory,
  activeEmojiCategoryLabel,
  filteredEmojiList,
  insertEmoji,
  gifSearch,
  onUpdateGifSearch,
  gifLoading,
  gifError,
  gifResults,
  pickGifResult,
  savedStickersLoading,
  savedStickers,
  stickerError,
  selectSavedSticker,
  removeSavedSticker,
  openStickerFromEmojiPanel
} = defineProps([
  "activeConversation",
  "canManageConversation",
  "emojiPanelOpen",
  "emojiPanelRef",
  "emojiSearchInputRef",
  "emojiTriggerRef",
  "toggleEmojiPanel",
  "EMOJI_PANEL_TAB_ITEMS",
  "emojiPanelTab",
  "onUpdateEmojiPanelTab",
  "emojiSearch",
  "onUpdateEmojiSearch",
  "emojiPickerLoading",
  "emojiPickerError",
  "showEmojiCategoryTabs",
  "emojiCategories",
  "emojiCategory",
  "updateEmojiCategory",
  "activeEmojiCategoryLabel",
  "filteredEmojiList",
  "insertEmoji",
  "gifSearch",
  "onUpdateGifSearch",
  "gifLoading",
  "gifError",
  "gifResults",
  "pickGifResult",
  "savedStickersLoading",
  "savedStickers",
  "stickerError",
  "selectSavedSticker",
  "removeSavedSticker",
  "openStickerFromEmojiPanel"
]);

const emojiPanelTabModel = computed({
  get: () => emojiPanelTab,
  set: (value: string) => onUpdateEmojiPanelTab(value)
});

const emojiSearchModel = computed({
  get: () => emojiSearch,
  set: (value: string) => onUpdateEmojiSearch(value)
});

const gifSearchModel = computed({
  get: () => gifSearch,
  set: (value: string) => onUpdateGifSearch(value)
});
</script>

<template>
  <div class="chat-composer__emoji-wrap">
    <button
      :ref="emojiTriggerRef"
      type="button"
      class="chat-composer__emoji-trigger"
      :class="{ 'chat-composer__emoji-trigger--active': emojiPanelOpen }"
      :disabled="!activeConversation || !canManageConversation"
      aria-label="Abrir painel de emoji"
      @click="toggleEmojiPanel"
    >
      <UIcon name="i-lucide-smile-plus" />
    </button>

    <div
      v-if="emojiPanelOpen"
      :ref="emojiPanelRef"
      class="chat-composer__emoji-panel"
      role="dialog"
      aria-label="Painel de emoji, GIF e figurinhas"
    >
      <div class="chat-composer__emoji-tab-row">
        <button
          v-for="item in EMOJI_PANEL_TAB_ITEMS"
          :key="item.id"
          type="button"
          class="chat-composer__emoji-tab"
          :class="{ 'chat-composer__emoji-tab--active': emojiPanelTabModel === item.id }"
          @click="emojiPanelTabModel = item.id"
        >
          {{ item.label }}
        </button>
      </div>

      <template v-if="emojiPanelTabModel === 'emoji'">
        <div :ref="emojiSearchInputRef">
          <UInput v-model="emojiSearchModel" icon="i-lucide-search" placeholder="Pesquisar emoji" size="sm" />
        </div>

        <div v-if="emojiPickerLoading" class="chat-composer__emoji-coming-soon">Carregando emojis...</div>
        <p v-else-if="emojiPickerError" class="chat-composer__emoji-empty">{{ emojiPickerError }}</p>
        <template v-else>
          <div v-if="showEmojiCategoryTabs" class="chat-composer__emoji-categories">
            <button
              v-for="category in emojiCategories"
              :key="category.id"
              type="button"
              class="chat-composer__emoji-category"
              :class="{ 'chat-composer__emoji-category--active': emojiCategory === category.id }"
              :title="category.label"
              @click="updateEmojiCategory(category.id)"
            >
              {{ category.icon }}
            </button>
          </div>

          <p class="chat-composer__emoji-section-title">
            {{ emojiSearchModel.trim().length > 0 ? `Busca: ${emojiSearchModel.trim()}` : activeEmojiCategoryLabel }}
          </p>

          <div class="chat-composer__emoji-grid">
            <button
              v-for="emoji in filteredEmojiList"
              :key="`emoji-${emoji}`"
              type="button"
              class="chat-composer__emoji-item"
              @click="insertEmoji(emoji)"
            >
              {{ emoji }}
            </button>
          </div>

          <p v-if="filteredEmojiList.length === 0" class="chat-composer__emoji-empty">
            Nenhum emoji encontrado para essa busca.
          </p>
        </template>
      </template>

      <template v-else-if="emojiPanelTabModel === 'gif'">
        <UInput v-model="gifSearchModel" icon="i-lucide-search" placeholder="Pesquisar GIF" size="sm" />

        <div v-if="gifLoading" class="chat-composer__emoji-coming-soon">Carregando GIFs...</div>
        <div v-else-if="gifError" class="chat-composer__emoji-empty">{{ gifError }}</div>
        <div v-else-if="gifResults.length === 0" class="chat-composer__emoji-empty">Digite um termo para buscar GIFs.</div>
        <div v-else class="chat-composer__gif-grid">
          <button
            v-for="item in gifResults"
            :key="`gif-${item.id}`"
            type="button"
            class="chat-composer__gif-item"
            :title="item.title || 'GIF'"
            :disabled="!canManageConversation"
            @click="pickGifResult(item)"
          >
            <img
              v-if="item.previewUrl || item.mediaUrl"
              :src="item.previewUrl || item.mediaUrl || ''"
              :alt="item.title || 'GIF'"
              loading="lazy"
            >
            <span v-else>Sem preview</span>
          </button>
        </div>
      </template>

      <template v-else>
        <div class="chat-composer__sticker-pane">
          <p class="chat-composer__emoji-section-title">Envie figurinha personalizada (WEBP, PNG ou JPG).</p>

          <p v-if="savedStickersLoading" class="chat-composer__emoji-empty">Carregando figurinhas salvas...</p>
          <div v-else-if="savedStickers.length > 0" class="chat-composer__sticker-grid">
            <div v-for="item in savedStickers" :key="`saved-sticker-${item.id}`" class="chat-composer__sticker-card">
              <button
                type="button"
                class="chat-composer__sticker-item"
                :disabled="!canManageConversation"
                :title="item.name"
                @click="selectSavedSticker(item)"
              >
                <img :src="item.dataUrl" :alt="item.name" loading="lazy">
              </button>
              <button
                type="button"
                class="chat-composer__sticker-remove"
                title="Remover figurinha"
                @click="removeSavedSticker(item.id)"
              >
                <UIcon name="i-lucide-x" />
              </button>
            </div>
          </div>
          <p v-else class="chat-composer__emoji-empty">Nenhuma figurinha salva ainda.</p>
          <p v-if="stickerError" class="chat-composer__emoji-empty">{{ stickerError }}</p>

          <UButton
            size="sm"
            color="neutral"
            variant="outline"
            icon="i-lucide-sticky-note"
            :disabled="!canManageConversation"
            @click="openStickerFromEmojiPanel"
          >
            Adicionar figurinha
          </UButton>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.chat-composer__emoji-wrap {
  position: relative;
  display: flex;
  align-items: center;
}

.chat-composer__emoji-panel {
  position: absolute;
  bottom: calc(100% + 0.55rem);
  left: 0;
  z-index: 35;
  width: min(24rem, calc(100vw - 2rem));
  max-height: 24rem;
  display: grid;
  gap: 0.55rem;
  padding: 0.65rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  box-shadow: var(--shadow-md);
}

.chat-composer__emoji-trigger {
  width: 2rem;
  height: 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: rgb(var(--muted));
}

.chat-composer__emoji-trigger--active {
  background: rgb(var(--primary) / 0.18);
  color: rgb(var(--primary));
}

.chat-composer__emoji-tab-row {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.35rem;
}

.chat-composer__emoji-tab,
.chat-composer__emoji-category,
.chat-composer__emoji-item {
  cursor: pointer;
  border: 1px solid rgb(var(--border));
  background: transparent;
  color: rgb(var(--muted));
}

.chat-composer__emoji-tab {
  border-radius: 999px;
  padding: 0.22rem 0.45rem;
  font-size: 0.74rem;
}

.chat-composer__emoji-tab--active,
.chat-composer__emoji-category--active {
  color: rgb(var(--primary));
  border-color: rgb(var(--primary) / 0.5);
  background: rgb(var(--primary) / 0.12);
}

.chat-composer__emoji-categories {
  display: flex;
  gap: 0.35rem;
  flex-wrap: wrap;
}

.chat-composer__emoji-category,
.chat-composer__emoji-item {
  border-radius: 0.5rem;
  padding: 0.25rem 0.45rem;
}

.chat-composer__emoji-grid,
.chat-composer__gif-grid,
.chat-composer__sticker-grid {
  display: grid;
  gap: 0.35rem;
  grid-template-columns: repeat(auto-fill, minmax(3rem, 1fr));
}

.chat-composer__gif-item,
.chat-composer__sticker-item {
  cursor: pointer;
  display: grid;
  place-items: center;
  min-height: 3.5rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: transparent;
}

.chat-composer__gif-item img,
.chat-composer__sticker-item img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: inherit;
}

.chat-composer__sticker-card {
  position: relative;
}

.chat-composer__sticker-remove {
  cursor: pointer;
  position: absolute;
  top: 0.2rem;
  right: 0.2rem;
  width: 1.2rem;
  height: 1.2rem;
  border: 0;
  border-radius: 999px;
  background: rgb(0 0 0 / 0.6);
  color: white;
}

.chat-composer__emoji-section-title {
  margin: 0;
  font-size: 0.82rem;
  font-weight: 600;
}

.chat-composer__emoji-empty,
.chat-composer__emoji-coming-soon {
  font-size: 0.72rem;
  color: rgb(var(--muted));
}
</style>
