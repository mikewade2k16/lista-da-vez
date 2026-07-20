<script setup lang="ts">
import { UAvatar, UTextarea } from "#components";
import { computed } from "vue";
import { resolveAvatarSource } from "~/composables/omnichannel/useAvatarProxy";

const {
  activeConversation,
  canManageConversation,
  draft,
  onUpdateDraft,
  composerInputRef,
  isMentionPickerVisible,
  mentionOptions,
  mentionSelectedIndex,
  pickMentionOption,
  loadingGroupParticipants,
  getInitials,
  onComposerKeydown,
  onComposerInput,
  onComposerCursorUpdate
} = defineProps([
  "activeConversation",
  "canManageConversation",
  "draft",
  "onUpdateDraft",
  "composerInputRef",
  "isMentionPickerVisible",
  "mentionOptions",
  "mentionSelectedIndex",
  "pickMentionOption",
  "loadingGroupParticipants",
  "getInitials",
  "onComposerKeydown",
  "onComposerInput",
  "onComposerCursorUpdate"
]);

const draftModel = computed({
  get: () => draft,
  set: (value: string) => onUpdateDraft(value)
});
</script>

<template>
  <div :ref="composerInputRef" class="chat-composer__input-wrap" data-chat-composer-input>
    <UTextarea
      v-model="draftModel"
      class="chat-composer__input"
      :disabled="!activeConversation || !canManageConversation"
      :rows="1"
      autoresize
      placeholder="Digite uma mensagem"
      @keydown="onComposerKeydown"
      @input="onComposerInput"
      @click="onComposerCursorUpdate"
      @keyup="onComposerCursorUpdate"
    />

    <div v-if="isMentionPickerVisible" class="chat-composer__mentions">
      <button
        v-for="(option, index) in mentionOptions"
        :key="`mention-option-${option.jid}`"
        type="button"
        class="chat-composer__mention-item"
        :class="{ 'chat-composer__mention-item--active': index === mentionSelectedIndex }"
        @click="pickMentionOption(option)"
      >
        <UAvatar :src="resolveAvatarSource(option.avatarUrl) || undefined" :text="getInitials(option.name)" size="2xs" />
        <span class="chat-composer__mention-name">{{ option.name }}</span>
        <span class="chat-composer__mention-phone">{{ option.phone }}</span>
      </button>
      <p v-if="loadingGroupParticipants && mentionOptions.length === 0" class="chat-composer__mention-empty">
        Carregando participantes...
      </p>
    </div>
  </div>
</template>

<style scoped>
.chat-composer__input-wrap {
  position: relative;
  min-width: 0;
  width: 100% !important;
}

.chat-composer__input {
  width: 100%;
}

.chat-composer__mentions {
  position: absolute;
  bottom: calc(100% + 0.4rem);
  left: 0;
  right: 0;
  z-index: 20;
  display: grid;
  gap: 0.35rem;
  padding: 0.45rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  box-shadow: var(--shadow-md);
  max-height: 14rem;
  overflow-y: auto;
}

.chat-composer__mention-item {
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: transparent;
  color: rgb(var(--text));
  padding: 0.45rem 0.5rem;
  text-align: left;
}

.chat-composer__mention-item--active {
  border-color: rgb(var(--primary) / 0.45);
  background: rgb(var(--surface-2));
}

.chat-composer__mention-name {
  font-size: 0.8rem;
  font-weight: 600;
}

.chat-composer__mention-phone,
.chat-composer__mention-empty {
  font-size: 0.72rem;
  color: rgb(var(--muted));
}

:deep(.chat-composer__input textarea) {
  min-height: 1.75rem;
  max-height: 10rem;
}
</style>
