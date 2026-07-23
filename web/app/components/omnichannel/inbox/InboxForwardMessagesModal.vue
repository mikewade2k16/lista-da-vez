<script setup lang="ts">
import { UButton, UInput, UModal } from "#components";
import { computed, ref, watch } from "vue";
import type { Conversation } from "~/types";

const props = defineProps<{
  open: boolean;
  loading: boolean;
  selectedCount: number;
  conversations: Conversation[];
  activeConversationId: string | null;
}>();

const emit = defineEmits<{
  (event: "update:open", value: boolean): void;
  (event: "select-conversation", conversationId: string): void;
}>();

const search = ref("");

function isTechnicalConversationLabel(value: string | null | undefined) {
  const normalized = value?.trim().toLowerCase() ?? "";
  if (!normalized) {
    return true;
  }

  if (
    normalized.endsWith("@s.whatsapp.net") ||
    normalized.endsWith("@g.us") ||
    normalized.endsWith("@lid")
  ) {
    return true;
  }

  const digitsOnly = normalized.replace(/\D/g, "");
  return digitsOnly.length >= 7 && !/[a-z]/i.test(normalized);
}

function getConversationDisplayPhone(conversationEntry: Conversation) {
  const contactDigits = conversationEntry.contactPhone?.replace(/\D/g, "") ?? "";
  if (contactDigits) {
    return contactDigits;
  }

  return conversationEntry.externalId.replace(/\D/g, "");
}

function getConversationDisplayName(conversationEntry: Conversation) {
  const contactName = conversationEntry.contactName?.trim() ?? "";
  if (contactName && !isTechnicalConversationLabel(contactName)) {
    return contactName;
  }

  const displayPhone = getConversationDisplayPhone(conversationEntry);
  if (displayPhone) {
    if (conversationEntry.externalId.trim().toLowerCase().endsWith("@g.us")) {
      return "Grupo sem nome";
    }
    return displayPhone;
  }

  return conversationEntry.externalId.trim().toLowerCase().endsWith("@g.us")
    ? "Grupo sem nome"
    : "Contato";
}

const filteredConversations = computed(() => {
  const needle = search.value.trim().toLowerCase();
  return props.conversations.filter((conversationEntry) => {
    if (conversationEntry.id === props.activeConversationId) {
      return false;
    }

    if (!needle) {
      return true;
    }

    const candidates = [
      getConversationDisplayName(conversationEntry),
      getConversationDisplayPhone(conversationEntry)
    ]
      .filter((value): value is string => typeof value === "string" && value.trim().length > 0)
      .map((value) => value.toLowerCase());

    return candidates.some((value) => value.includes(needle));
  });
});

function closeModal() {
  emit("update:open", false);
}

function selectConversation(conversationId: string) {
  emit("select-conversation", conversationId);
}

watch(
  () => props.open,
  (value) => {
    if (!value) {
      search.value = "";
    }
  }
);
</script>

<template>
  <UModal
    :open="open"
    title="Encaminhar mensagens"
    :description="`${selectedCount} selecionada(s)`"
    @update:open="emit('update:open', $event)"
  >
    <template #body>
      <div class="forward-modal">
        <div class="forward-modal__header">
          <UButton size="xs" color="neutral" variant="ghost" @click="closeModal">
            Fechar
          </UButton>
        </div>

        <UInput
          v-model="search"
          class="forward-modal__search"
          size="sm"
          color="neutral"
          variant="outline"
          icon="i-lucide-search"
          placeholder="Buscar conversa destino"
        />

        <div class="forward-modal__list">
          <button
            v-for="conversationEntry in filteredConversations"
            :key="conversationEntry.id"
            type="button"
            class="forward-modal__item"
            :disabled="loading"
            @click="selectConversation(conversationEntry.id)"
          >
            <span class="forward-modal__item-name">
              {{ getConversationDisplayName(conversationEntry) }}
            </span>
            <span class="forward-modal__item-meta">
              {{ conversationEntry.channel }} | {{ conversationEntry.status }}
            </span>
          </button>

          <p v-if="!filteredConversations.length" class="forward-modal__empty">
            Nenhuma conversa disponivel para encaminhamento.
          </p>
        </div>
      </div>
    </template>
  </UModal>
</template>

<style scoped>
.forward-modal {
  padding: 1rem;
  display: grid;
  gap: 0.75rem;
}

.forward-modal__header {
  display: flex;
  align-items: flex-start;
  justify-content: flex-end;
}

.forward-modal__list {
  display: grid;
  gap: 0.45rem;
  max-height: min(22rem, 60vh);
  overflow-y: auto;
}

.forward-modal__item {
  width: 100%;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2));
  padding: 0.75rem;
  text-align: left;
  display: grid;
  gap: 0.2rem;
  cursor: pointer;
}

.forward-modal__item:disabled {
  cursor: wait;
  opacity: 0.65;
}

.forward-modal__item-name {
  font-size: 0.88rem;
  font-weight: 700;
}

.forward-modal__item-meta {
  font-size: 0.72rem;
  color: rgb(var(--muted));
}

.forward-modal__empty {
  margin: 0;
  font-size: 0.78rem;
  color: rgb(var(--muted));
}
</style>
