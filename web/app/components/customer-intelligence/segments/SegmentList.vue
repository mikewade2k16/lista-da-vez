<script setup lang="ts">
import AppSelectField from '~/components/ui/AppSelectField.vue'
import type { CustomerSegmentListItem } from '~/domain/customer-data/segment-types'

defineProps<{
  items: CustomerSegmentListItem[]
  selectedId: string
  loading: boolean
}>()

const status = defineModel<string>('status', { default: '' })
const emit = defineEmits<{
  select: [segmentId: string]
  refresh: []
}>()

const statusOptions = [
  { value: '', label: 'Todos os status' },
  { value: 'draft', label: 'Draft' },
  { value: 'active', label: 'Ativo' },
  { value: 'archived', label: 'Arquivado' },
]
</script>

<template>
  <aside class="segment-list">
    <header>
      <h2>Segmentos</h2>
      <button type="button" :disabled="loading" @click="emit('refresh')">Atualizar</button>
    </header>
    <AppSelectField v-model="status" :options="statusOptions" compact @change="emit('refresh')" />
    <button
      v-for="segment in items"
      :key="segment.id"
      type="button"
      class="segment-list__item"
      :class="{ 'is-active': selectedId === segment.id }"
      @click="emit('select', segment.id)"
    >
      <strong>{{ segment.name }}</strong>
      <small>
        {{ segment.status }} · v{{ segment.activeVersionNumber ?? '—' }} ·
        {{ segment.memberCountBucket || 'sem contagem' }}
      </small>
      <small>{{ segment.freshnessStatus || 'freshness desconhecida' }}</small>
    </button>
    <p v-if="!loading && !items.length">Nenhum segmento neste cliente.</p>
  </aside>
</template>

<style scoped>
.segment-list {
  display: grid;
  align-content: start;
  gap: 0.65rem;
  min-width: 0;
}

.segment-list header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.segment-list h2 {
  margin: 0;
  font-size: 0.95rem;
}

.segment-list button {
  cursor: pointer;
}

.segment-list__item {
  display: grid;
  gap: 0.2rem;
  padding: 0.75rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 0.75rem;
  background: rgb(var(--surface));
  color: inherit;
  text-align: left;
}

.segment-list__item.is-active {
  border-color: rgb(var(--primary) / 0.65);
  background: rgb(var(--primary) / 0.08);
}

.segment-list small,
.segment-list p {
  color: rgb(var(--muted));
  font-size: 0.7rem;
}
</style>
