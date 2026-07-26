<script setup lang="ts">
const props = defineProps<{
  pageIndex: number
  pageSize: number
  itemCount: number
  hasNext: boolean
  loading: boolean
  refreshing: boolean
}>()

const emit = defineEmits<{
  previous: []
  next: []
  refresh: []
}>()

const rangeLabel = computed(() => {
  if (props.itemCount === 0) return 'Nenhum item nesta página'
  const first = props.pageIndex * props.pageSize + 1
  return `Exibindo ${first}–${first + props.itemCount - 1}`
})
</script>

<template>
  <div class="sp-pagination">
    <div class="sp-pagination__state" aria-live="polite">
      <strong>Página {{ pageIndex + 1 }}</strong>
      <span>{{ rangeLabel }}</span>
    </div>
    <div class="sp-pagination__actions">
      <UButton
        type="button"
        color="neutral"
        variant="ghost"
        size="sm"
        icon="i-lucide-refresh-cw"
        label="Atualizar"
        :loading="refreshing"
        :disabled="loading"
        @click="emit('refresh')"
      />
      <UButton
        type="button"
        color="neutral"
        variant="soft"
        size="sm"
        icon="i-lucide-chevron-left"
        label="Anterior"
        :disabled="loading || pageIndex === 0"
        @click="emit('previous')"
      />
      <UButton
        type="button"
        color="neutral"
        variant="soft"
        size="sm"
        trailing-icon="i-lucide-chevron-right"
        label="Próxima"
        :disabled="loading || !hasNext"
        @click="emit('next')"
      />
    </div>
  </div>
</template>

<style scoped>
.sp-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.8rem;
  margin-bottom: 0.8rem;
}

.sp-pagination__state {
  display: grid;
  gap: 0.1rem;
}

.sp-pagination__state strong {
  color: rgb(var(--text));
  font-size: 0.78rem;
}

.sp-pagination__state span {
  color: rgb(var(--muted));
  font-size: 0.7rem;
}

.sp-pagination__actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.35rem;
}

@media (max-width: 580px) {
  .sp-pagination {
    align-items: stretch;
    flex-direction: column;
  }

  .sp-pagination__actions {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>
