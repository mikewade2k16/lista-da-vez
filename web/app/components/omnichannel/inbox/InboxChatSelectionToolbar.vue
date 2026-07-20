<script setup lang="ts">
import { UButton, UCard } from "#components";

defineProps<{
  selectedCount: number;
  canDeleteForAll: boolean;
  canForward: boolean;
  statusMessage: string;
}>();

const emit = defineEmits<{
  (event: "clear"): void;
  (event: "delete-mine"): void;
  (event: "delete-all"): void;
  (event: "forward"): void;
}>();
</script>

<template>
  <UCard class="chat-selection-toolbar">
    <div class="chat-selection-toolbar__content">
      <div class="chat-selection-toolbar__meta">
        <p class="chat-selection-toolbar__title">{{ selectedCount }} selecionada(s)</p>
        <p v-if="statusMessage" class="chat-selection-toolbar__status">{{ statusMessage }}</p>
      </div>

      <div class="chat-selection-toolbar__actions">
        <UButton size="xs" color="neutral" variant="ghost" @click="emit('clear')">
          Limpar
        </UButton>
        <UButton size="xs" color="neutral" variant="outline" :disabled="!canForward" @click="emit('forward')">
          Encaminhar
        </UButton>
        <UButton size="xs" color="warning" variant="outline" @click="emit('delete-mine')">
          Apagar p/ mim
        </UButton>
        <UButton size="xs" color="error" variant="solid" :disabled="!canDeleteForAll" @click="emit('delete-all')">
          Apagar p/ todos
        </UButton>
      </div>
    </div>
  </UCard>
</template>

<style scoped>
.chat-selection-toolbar {
  margin-bottom: 0.6rem;
}

.chat-selection-toolbar__content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.chat-selection-toolbar__meta,
.chat-selection-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  flex-wrap: wrap;
}

.chat-selection-toolbar__meta {
  min-width: 0;
}

.chat-selection-toolbar__title,
.chat-selection-toolbar__status {
  margin: 0;
  font-size: 0.76rem;
}

.chat-selection-toolbar__title {
  font-weight: 700;
}

.chat-selection-toolbar__status {
  color: rgb(var(--muted));
}
</style>
