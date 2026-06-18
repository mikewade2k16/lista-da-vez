<script setup lang="ts">
import { GripVertical } from 'lucide-vue-next'
import type { MenuPlacement } from '~/stores/menuLayout'
import type { MenuLayoutEditorItem } from './useMenuLayoutEditor'

const props = defineProps<{
  item: MenuLayoutEditorItem
  placement: MenuPlacement
  scopeId: string
  dragging: boolean
}>()

const emit = defineEmits<{
  'update:placement': [placement: MenuPlacement]
  dragstart: [DragEvent]
  dragover: [DragEvent]
  drop: [DragEvent]
  dragend: []
}>()

const PLACEMENT_OPTIONS: { value: MenuPlacement; label: string }[] = [
  { value: 'header', label: 'Header' },
  { value: 'sidebar', label: 'Sidebar' },
  { value: 'both', label: 'Ambos' },
  { value: 'hidden', label: 'Oculto' },
]

function selectPlacement(value: MenuPlacement) {
  if (value !== props.placement) emit('update:placement', value)
}
</script>

<template>
  <div
    class="menu-layout-row"
    :class="{ 'menu-layout-row--dragging': props.dragging }"
    draggable="true"
    @dragstart="emit('dragstart', $event)"
    @dragover="emit('dragover', $event)"
    @drop="emit('drop', $event)"
    @dragend="emit('dragend')"
  >
    <span class="menu-layout-row__handle" aria-hidden="true">
      <GripVertical :size="16" :stroke-width="2.1" />
    </span>

    <span class="menu-layout-row__label">
      {{ props.item.label }}
      <span v-if="props.item.hidden" class="menu-layout-row__flag">dev</span>
    </span>

    <div
      class="menu-layout-row__segmented"
      role="radiogroup"
      :aria-label="`Posicao de ${props.item.label}`"
    >
      <button
        v-for="option in PLACEMENT_OPTIONS"
        :key="option.value"
        class="menu-layout-row__seg"
        :class="{ 'is-active': props.placement === option.value }"
        type="button"
        role="radio"
        :aria-checked="props.placement === option.value ? 'true' : 'false'"
        @click="selectPlacement(option.value)"
      >
        {{ option.label }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.menu-layout-row {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  padding: 0.45rem 0.6rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: var(--surface);
  cursor: grab;
}

.menu-layout-row--dragging {
  opacity: 0.5;
  border-color: rgb(var(--primary) / 0.4);
}

.menu-layout-row__handle {
  display: inline-grid;
  place-items: center;
  flex-shrink: 0;
  color: var(--text-muted);
}

.menu-layout-row__label {
  min-width: 0;
  flex: 1;
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-main);
  font-size: 0.85rem;
  font-weight: 700;
}

.menu-layout-row__flag {
  flex-shrink: 0;
  padding: 0.05rem 0.4rem;
  border-radius: 999px;
  background: rgb(var(--muted) / 0.16);
  color: var(--text-muted);
  font-size: 0.62rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.menu-layout-row__segmented {
  display: inline-flex;
  flex-shrink: 0;
  gap: 0.15rem;
  padding: 0.15rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: var(--bg-muted);
}

.menu-layout-row__seg {
  appearance: none;
  border: 0;
  border-radius: calc(var(--radius-sm) - 2px);
  padding: 0.3rem 0.55rem;
  background: transparent;
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 750;
  cursor: pointer;
  transition:
    color 0.16s ease,
    background 0.16s ease;
}

.menu-layout-row__seg:hover {
  color: var(--text-main);
}

.menu-layout-row__seg.is-active {
  background: rgb(var(--primary));
  color: rgb(255 255 255);
}
</style>
