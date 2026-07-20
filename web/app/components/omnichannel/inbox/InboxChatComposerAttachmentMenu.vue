<script setup lang="ts">
import { UAvatar, UButton, UDropdownMenu, UFormField, UInput } from "#components";
import { computed } from "vue";
import { resolveAvatarSource } from "~/composables/omnichannel/useAvatarProxy";

const {
  activeConversation,
  canManageConversation,
  fileInputRef,
  pickerAccept,
  pickerCapture,
  attachmentMenuItems,
  contactPickerOpen,
  contactPickerRef,
  contactPickerSearch,
  onUpdateContactPickerSearch,
  closeContactPicker,
  filteredSavedContacts,
  selectSavedContactForSend,
  formatContactDisplayPhone,
  getInitials,
  onFileChange
} = defineProps([
  "activeConversation",
  "canManageConversation",
  "fileInputRef",
  "pickerAccept",
  "pickerCapture",
  "attachmentMenuItems",
  "contactPickerOpen",
  "contactPickerRef",
  "contactPickerSearch",
  "onUpdateContactPickerSearch",
  "closeContactPicker",
  "filteredSavedContacts",
  "selectSavedContactForSend",
  "formatContactDisplayPhone",
  "getInitials",
  "onFileChange"
]);

const contactPickerSearchModel = computed({
  get: () => contactPickerSearch,
  set: (value: string) => onUpdateContactPickerSearch(value)
});
</script>

<template>
  <div class="chat-composer__attachment-wrap">
    <input
      :ref="fileInputRef"
      class="chat-page__file-input"
      type="file"
      :accept="pickerAccept"
      :capture="pickerCapture"
      @change="onFileChange"
    >

    <UDropdownMenu
      :items="attachmentMenuItems"
      :content="{ side: 'top', align: 'start', sideOffset: 10 }"
      :ui="{ content: 'chat-composer__menu' }"
    >
      <UButton
        size="sm"
        color="neutral"
        variant="ghost"
        :disabled="!activeConversation || !canManageConversation"
        icon="i-lucide-plus"
        aria-label="Adicionar anexo"
      />
    </UDropdownMenu>

    <div
      v-if="contactPickerOpen"
      :ref="contactPickerRef"
      class="chat-composer__contact-panel"
      role="dialog"
      aria-label="Selecionar contato salvo"
    >
      <div class="chat-composer__contact-head">
        <p class="chat-composer__contact-title">Enviar contato salvo</p>
        <UButton size="xs" color="neutral" variant="ghost" icon="i-lucide-x" @click="closeContactPicker" />
      </div>

      <UFormField label="Buscar contato" name="composerContactSearch">
        <UInput v-model="contactPickerSearchModel" icon="i-lucide-search" placeholder="Nome ou telefone" size="sm" />
      </UFormField>

      <div class="chat-composer__contact-list">
        <button
          v-for="contactEntry in filteredSavedContacts"
          :key="`send-contact-${contactEntry.id}`"
          type="button"
          class="chat-composer__contact-item"
          @click="selectSavedContactForSend(contactEntry)"
        >
          <UAvatar
            :src="resolveAvatarSource(contactEntry.avatarUrl) || undefined"
            :alt="contactEntry.name || contactEntry.phone"
            :text="getInitials(contactEntry.name || contactEntry.phone)"
            size="2xs"
          />
          <span class="chat-composer__contact-item-copy">
            <span class="chat-composer__contact-item-name">
              {{ contactEntry.name || formatContactDisplayPhone(contactEntry.phone) }}
            </span>
            <span class="chat-composer__contact-item-phone">{{ formatContactDisplayPhone(contactEntry.phone) }}</span>
          </span>
        </button>

        <p v-if="filteredSavedContacts.length === 0" class="chat-composer__contact-empty">
          Nenhum contato salvo. Use a aba Contatos para cadastrar ou salvar um contato da conversa.
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.chat-composer__attachment-wrap {
  position: relative;
  display: flex;
  align-items: center;

}

.chat-page__file-input {
  position: absolute;
  inline-size: 1px;
  block-size: 1px;
  opacity: 0;
  pointer-events: none;
  overflow: hidden;
}

.chat-composer__contact-panel {
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

.chat-composer__contact-head,
.chat-composer__contact-item {
  display: flex;
  align-items: center;
}

.chat-composer__contact-head {
  justify-content: space-between;
  gap: 0.5rem;
}

.chat-composer__contact-title {
  margin: 0;
  font-size: 0.82rem;
  font-weight: 600;
}

.chat-composer__contact-list {
  display: grid;
  gap: 0.35rem;
  max-height: 14rem;
  overflow-y: auto;
}

.chat-composer__contact-item {
  cursor: pointer;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: transparent;
  color: rgb(var(--text));
  padding: 0.45rem 0.5rem;
  gap: 0.5rem;
  text-align: left;
}

.chat-composer__contact-item-copy {
  display: grid;
  min-width: 0;
}

.chat-composer__contact-item-name {
  font-size: 0.8rem;
  font-weight: 600;
}

.chat-composer__contact-item-phone,
.chat-composer__contact-empty {
  font-size: 0.72rem;
  color: rgb(var(--muted));
}

.chat-composer__menu {
  min-width: 14rem;
}
</style>
