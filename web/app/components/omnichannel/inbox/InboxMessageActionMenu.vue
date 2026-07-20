<script setup lang="ts">
import { UButton, UDropdownMenu } from "#components";
import { computed } from "vue";

const props = defineProps<{
  canManageConversation: boolean;
  isSelected: boolean;
  canDeleteForAll: boolean;
  reactionItems: Array<Array<{ label: string; onSelect: () => void }>>;
}>();

const emit = defineEmits<{
  (event: "reply"): void;
  (event: "toggle-select"): void;
  (event: "forward"): void;
  (event: "delete-mine"): void;
  (event: "delete-all"): void;
}>();

const actionItems = computed(() => [[
  {
    label: "Responder",
    onSelect: () => emit("reply")
  },
  {
    label: props.isSelected ? "Desmarcar" : "Selecionar",
    onSelect: () => emit("toggle-select")
  },
  {
    label: "Encaminhar",
    onSelect: () => emit("forward")
  },
  {
    label: "Apagar para mim",
    onSelect: () => emit("delete-mine")
  },
  {
    label: "Apagar para todos",
    disabled: !props.canDeleteForAll,
    onSelect: () => emit("delete-all")
  }
]]);
</script>

<template>
  <div class="chat-message-actions">
    <UDropdownMenu :items="reactionItems" :content="{ side: 'top', align: 'end', sideOffset: 8 }">
      <UButton size="xs" color="neutral" variant="ghost" icon="i-lucide-smile" :disabled="!canManageConversation" />
    </UDropdownMenu>

    <UDropdownMenu :items="actionItems" :content="{ side: 'top', align: 'end', sideOffset: 8 }">
      <UButton
        size="xs"
        color="neutral"
        variant="ghost"
        icon="i-lucide-chevron-down"
        :disabled="!canManageConversation"
      />
    </UDropdownMenu>
  </div>
</template>

<style scoped>
.chat-message-actions {
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  flex-wrap: wrap;
}
</style>
