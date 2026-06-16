<script setup lang="ts">
interface RosterEntry {
  id: string
  name: string
  [key: string]: unknown
}

withDefaults(
  defineProps<{
    roster?: RosterEntry[]
    selectedConsultantId?: string
  }>(),
  {
    roster: () => [],
    selectedConsultantId: '',
  },
)

const emit = defineEmits<{
  (e: 'select', consultantId: string): void
}>()
</script>

<template>
  <div class="admin-selector" data-testid="consultant-selector">
    <button
      v-for="consultant in roster"
      :key="consultant.id"
      type="button"
      class="admin-selector__button"
      :class="{ 'admin-selector__button--active': consultant.id === selectedConsultantId }"
      @click="emit('select', consultant.id)"
    >
      {{ consultant.name }}
    </button>
  </div>
</template>
