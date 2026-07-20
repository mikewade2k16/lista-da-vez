<script setup lang="ts">
import { UAvatar } from "#components";
import { resolveAvatarSource } from "~/composables/omnichannel/useAvatarProxy";

const {
  contactCard,
  canManageConversation,
  getInitials,
  direction
} = defineProps(["contactCard", "canManageConversation", "getInitials", "direction"]);

const emit = defineEmits<{
  (event: "open"): void;
  (event: "save"): void;
}>();
</script>

<template>
  <div
    class="chat-message__contact-card"
    :class="{
      'chat-message__contact-card--outbound': direction === 'OUTBOUND',
      'chat-message__contact-card--saved': contactCard?.contactId
    }"
  >
    <div class="chat-message__contact-main">
      <UAvatar
        :src="resolveAvatarSource(contactCard?.avatarUrl) || undefined"
        :alt="contactCard?.name || 'Contato'"
        :text="getInitials(contactCard?.name || contactCard?.phone || 'Contato')"
        class="chat-message__contact-avatar"
        size="lg"
      />

      <div class="chat-message__contact-content">
        <p class="chat-message__contact-name">{{ contactCard?.name }}</p>
        <p
          v-if="contactCard?.displayPhone && contactCard?.displayPhone !== contactCard?.name"
          class="chat-message__contact-phone"
        >
          {{ contactCard?.displayPhone }}
        </p>
      </div>
    </div>

    <div class="chat-message__contact-actions">
      <button
        type="button"
        class="chat-message__contact-action"
        :disabled="!contactCard?.phone || !canManageConversation"
        @click="emit('open')"
      >
        Conversar
      </button>
      <button
        type="button"
        class="chat-message__contact-action chat-message__contact-action--secondary"
        :disabled="!contactCard?.phone || !canManageConversation"
        @click="emit('save')"
      >
        {{ contactCard?.contactId ? 'Atualizar contato' : 'Salvar contato' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.chat-message__contact-card {
  display: grid;
  gap: 0;
  padding: 0.7rem 0.8rem 0;
  margin-bottom: 0.4rem;
  border: 1px solid rgb(var(--border));
  border-radius: calc(var(--radius-sm) + 2px);
  background: rgb(var(--surface-2));
  overflow: hidden;
}

.chat-message__contact-card--outbound {
  background: rgb(var(--success) / 0.2);
  border-color: rgb(var(--success) / 0.32);
}

.chat-message__contact-card--saved {
  box-shadow: inset 0 0 0 1px rgb(var(--primary) / 0.08);
}

.chat-message__contact-main {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 0.7rem;
  align-items: center;
  padding-bottom: 0.7rem;
}

.chat-message__contact-avatar {
  flex-shrink: 0;
}

.chat-message__contact-content {
  display: grid;
  gap: 0.2rem;
  min-width: 0;
}

.chat-message__contact-name,
.chat-message__contact-phone {
  margin: 0;
  min-width: 0;
  overflow-wrap: anywhere;
}

.chat-message__contact-name {
  font-size: 0.92rem;
  font-weight: 700;
}

.chat-message__contact-phone {
  font-size: 0.76rem;
  color: rgb(var(--muted));
}

.chat-message__contact-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-items: stretch;
  gap: 0;
  border-top: 1px solid rgb(var(--border));
}

.chat-message__contact-action {
  appearance: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.35rem;
  border: 0;
  border-right: 1px solid rgb(var(--border));
  background: transparent;
  color: rgb(var(--primary));
  font-size: 0.76rem;
  font-weight: 700;
  cursor: pointer;
}

.chat-message__contact-action--secondary {
  border-right: 0;
  color: rgb(var(--success));
}

.chat-message__contact-action:disabled {
  cursor: not-allowed;
  color: rgb(var(--muted));
}
</style>
