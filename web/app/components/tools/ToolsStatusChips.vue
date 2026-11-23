<script setup lang="ts">
import { computed } from 'vue'

// Filtro de status compacto (segmentado) do encurtador e do QR, no mesmo
// formato do seletor de foto do cardapio: Todos / Ativos / Inativos + contagem.
// compact = so icone + numero (barra do mobile).
const props = defineProps<{
  total: number
  active: number
  compact?: boolean
}>()
const value = defineModel<string>({ required: true })

const chips = computed(() => [
  { value: '', label: 'Todos', icon: 'i-lucide-list', count: props.total },
  { value: 'active', label: 'Ativos', icon: 'i-lucide-circle-check', count: props.active },
  {
    value: 'inactive',
    label: 'Inativos',
    icon: 'i-lucide-circle-pause',
    count: props.total - props.active,
  },
])
</script>

<template>
  <div class="tools-status" role="group" aria-label="Filtrar por status">
    <button
      v-for="chip in chips"
      :key="chip.value"
      type="button"
      :class="{
        'is-active': value === chip.value,
        'is-warn': chip.value === 'inactive' && chip.count > 0,
      }"
      :aria-pressed="value === chip.value"
      :aria-label="`${chip.label}: ${chip.count}`"
      :title="`${chip.label}: ${chip.count}`"
      @click="value = chip.value"
    >
      <UIcon v-if="compact" :name="chip.icon" />
      <template v-else>{{ chip.label }}</template>
      <strong>{{ chip.count }}</strong>
    </button>
  </div>
</template>

<style scoped>
.tools-status {
  display: inline-flex;
  padding: 0.15rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
}

.tools-status button {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.3rem 0.6rem;
  border: none;
  border-radius: calc(var(--radius-sm) - 2px);
  background: transparent;
  color: var(--text-muted);
  font-size: 0.8rem;
  white-space: nowrap;
  cursor: pointer;
}

.tools-status strong {
  color: var(--text-main);
  font-weight: 700;
}

.tools-status button.is-warn strong {
  color: rgb(var(--danger));
}

.tools-status button.is-active {
  background: rgb(var(--primary) / 0.16);
  color: var(--text-main);
}
</style>
