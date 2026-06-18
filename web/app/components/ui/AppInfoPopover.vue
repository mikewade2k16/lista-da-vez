<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    text?: string
    label?: string
    align?: 'start' | 'end'
  }>(),
  {
    text: '',
    label: 'Informação',
    align: 'end',
  },
)

const open = ref(false)
const rootRef = ref<HTMLElement | null>(null)

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

function handlePointerDown(event: PointerEvent) {
  if (!rootRef.value || rootRef.value.contains(event.target as Node)) return
  close()
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') close()
}

// Dropdown/popover SEMPRE fecha no clique-fora e no Esc (AGENT_RULES).
watch(open, (value) => {
  if (value) {
    document.addEventListener('pointerdown', handlePointerDown)
    document.addEventListener('keydown', handleKeydown)
  } else {
    document.removeEventListener('pointerdown', handlePointerDown)
    document.removeEventListener('keydown', handleKeydown)
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', handlePointerDown)
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <span ref="rootRef" class="app-info-popover" :class="`app-info-popover--${align}`">
    <button
      type="button"
      class="app-info-popover__trigger"
      :aria-label="label"
      :aria-expanded="open"
      @click.stop="toggle"
    >
      i
    </button>
    <div v-if="open" class="app-info-popover__panel" role="tooltip">
      <slot>{{ props.text }}</slot>
    </div>
  </span>
</template>

<style scoped>
.app-info-popover {
  position: relative;
  display: inline-flex;
}

.app-info-popover__trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.15rem;
  height: 1.15rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--border) / 0.8);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 800;
  font-style: italic;
  line-height: 1;
  cursor: pointer;
  transition:
    color 0.15s ease,
    border-color 0.15s ease;
}

.app-info-popover__trigger:hover,
.app-info-popover__trigger[aria-expanded='true'] {
  color: rgb(var(--primary));
  border-color: rgb(var(--primary) / 0.5);
}

.app-info-popover__panel {
  position: absolute;
  top: calc(100% + 0.4rem);
  z-index: 40;
  width: min(20rem, 78vw);
  padding: 0.7rem 0.8rem;
  border-radius: 0.7rem;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface) / 0.98);
  box-shadow: var(--shadow-card);
  color: var(--text-main);
  font-size: 0.74rem;
  font-weight: 500;
  line-height: 1.35;
  text-align: left;
  white-space: normal;
}

.app-info-popover--end .app-info-popover__panel {
  right: 0;
}

.app-info-popover--start .app-info-popover__panel {
  left: 0;
}
</style>
