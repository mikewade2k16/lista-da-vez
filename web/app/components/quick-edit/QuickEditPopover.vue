<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import type { QuickEditFieldType } from '~/domain/quick-edit/defineQuickEditField'

const props = withDefaults(
  defineProps<{
    open: boolean
    label: string
    type: QuickEditFieldType
    hint?: string
    current?: number | null
    saving?: boolean
    errorMessage?: string
  }>(),
  {
    hint: '',
    current: null,
    saving: false,
    errorMessage: '',
  },
)

const emit = defineEmits<{
  (e: 'save', value: number): void
  (e: 'close'): void
}>()

const rootRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLInputElement | null>(null)
const draft = ref('')

const prefix = computed(() => (props.type === 'currency' ? 'R$' : ''))
const suffix = computed(() => (props.type === 'percent' ? '%' : ''))
const inputStep = computed(() =>
  props.type === 'number' ? '0.01' : props.type === 'percent' ? '0.1' : '0.01',
)

// Semeia o rascunho com o valor atual ao abrir e foca o input.
watch(
  () => props.open,
  (value) => {
    if (!value) return
    draft.value = props.current === null || props.current === undefined ? '' : String(props.current)
    void nextTick(() => {
      inputRef.value?.focus()
      inputRef.value?.select()
    })
  },
  { immediate: true },
)

function close() {
  emit('close')
}

function submit() {
  if (props.saving) return
  const parsed = Number(String(draft.value).replace(',', '.'))
  if (!Number.isFinite(parsed) || parsed < 0) {
    emit('save', 0)
    return
  }
  emit('save', parsed)
}

// Dropdown/popover SEMPRE fecha no clique-fora e no Esc (AGENT_RULES).
function handlePointerDown(event: PointerEvent) {
  if (!rootRef.value || rootRef.value.contains(event.target as Node)) return
  close()
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.stopPropagation()
    close()
  }
}

watch(
  () => props.open,
  (value) => {
    if (value) {
      document.addEventListener('pointerdown', handlePointerDown)
      document.addEventListener('keydown', handleKeydown)
    } else {
      document.removeEventListener('pointerdown', handlePointerDown)
      document.removeEventListener('keydown', handleKeydown)
    }
  },
)

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', handlePointerDown)
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div v-if="open" ref="rootRef" class="quick-edit-popover" role="dialog" :aria-label="label">
    <form class="quick-edit-popover__form" @submit.prevent="submit">
      <label class="quick-edit-popover__label">
        <span class="quick-edit-popover__label-text">{{ label }}</span>
        <span class="quick-edit-popover__input-wrap">
          <span v-if="prefix" class="quick-edit-popover__affix" aria-hidden="true">
            {{ prefix }}
          </span>
          <input
            ref="inputRef"
            v-model="draft"
            class="quick-edit-popover__input"
            type="number"
            inputmode="decimal"
            min="0"
            :step="inputStep"
            :disabled="saving"
          />
          <span v-if="suffix" class="quick-edit-popover__affix" aria-hidden="true">
            {{ suffix }}
          </span>
        </span>
      </label>

      <p v-if="hint" class="quick-edit-popover__hint">{{ hint }}</p>
      <p v-if="errorMessage" class="quick-edit-popover__error" role="alert">{{ errorMessage }}</p>

      <div class="quick-edit-popover__actions">
        <button
          type="button"
          class="quick-edit-popover__btn quick-edit-popover__btn--ghost"
          :disabled="saving"
          @click="close"
        >
          Cancelar
        </button>
        <button type="submit" class="quick-edit-popover__btn" :disabled="saving">
          {{ saving ? 'Salvando...' : 'Salvar' }}
        </button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.quick-edit-popover {
  position: absolute;
  top: calc(100% + 0.4rem);
  left: 0;
  z-index: 50;
  width: min(16rem, 80vw);
  padding: 0.75rem 0.8rem;
  border-radius: var(--radius-card);
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface) / 0.98);
  box-shadow: var(--shadow-card);
  text-align: left;
}

.quick-edit-popover__form {
  display: grid;
  gap: 0.55rem;
}

.quick-edit-popover__label {
  display: grid;
  gap: 0.3rem;
}

.quick-edit-popover__label-text {
  font-size: 0.74rem;
  font-weight: 700;
  color: var(--text-main);
}

.quick-edit-popover__input-wrap {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.4rem 0.55rem;
  border-radius: var(--radius-soft);
  border: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface-2) / 0.76);
}

.quick-edit-popover__affix {
  font-size: 0.8rem;
  font-weight: 700;
  color: var(--text-muted);
}

.quick-edit-popover__input {
  flex: 1;
  min-width: 0;
  border: none;
  background: transparent;
  color: var(--text-main);
  font-size: 0.9rem;
  font-weight: 600;
  outline: none;
}

.quick-edit-popover__hint {
  margin: 0;
  font-size: 0.7rem;
  color: var(--text-muted);
}

.quick-edit-popover__error {
  margin: 0;
  font-size: 0.7rem;
  font-weight: 600;
  color: rgb(var(--danger));
}

.quick-edit-popover__actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.4rem;
}

.quick-edit-popover__btn {
  padding: 0.4rem 0.8rem;
  border-radius: var(--radius-soft);
  border: none;
  background: rgb(var(--primary));
  color: rgb(255 255 255);
  font-size: 0.76rem;
  font-weight: 700;
  cursor: pointer;
  transition: opacity 120ms ease;
}

.quick-edit-popover__btn:disabled {
  cursor: wait;
  opacity: 0.7;
}

.quick-edit-popover__btn--ghost {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}
</style>
