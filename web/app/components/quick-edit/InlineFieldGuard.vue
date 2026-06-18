<script setup lang="ts" generic="TContext extends QuickEditContextBase">
import { computed, ref } from 'vue'
import QuickEditPopover from '~/components/quick-edit/QuickEditPopover.vue'
import type {
  QuickEditContextBase,
  QuickEditFieldDescriptor,
} from '~/domain/quick-edit/defineQuickEditField'

const props = defineProps<{
  descriptor: QuickEditFieldDescriptor<TContext>
  context: TContext
}>()

const open = ref(false)
const saving = ref(false)
const errorMessage = ref('')

const isMissing = computed(() => props.descriptor.isMissing(props.context))
const warningText = computed(() => props.descriptor.warning(props.context))
const canEdit = computed(() => props.descriptor.canEdit(props.context.permission))
const currentValue = computed(() => props.descriptor.current(props.context))

function toggle() {
  if (!canEdit.value) return
  errorMessage.value = ''
  open.value = !open.value
}

function close() {
  open.value = false
}

async function handleSave(value: number) {
  saving.value = true
  errorMessage.value = ''
  try {
    await props.descriptor.save(value, props.context)
    // Re-hidrata do back: a fonte continua única (API do recurso).
    await props.descriptor.afterSave(props.context)
    open.value = false
  } catch (error) {
    errorMessage.value =
      error instanceof Error && error.message
        ? error.message
        : 'Nao foi possivel salvar. Tente novamente.'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <span
    v-if="isMissing"
    class="inline-field-guard"
    @click.stop
    @keydown.enter.stop
    @keydown.space.stop
  >
    <button
      v-if="canEdit"
      type="button"
      class="inline-field-guard__chip inline-field-guard__chip--editable"
      :aria-expanded="open"
      :title="`${warningText} — clique para cadastrar`"
      @click.stop="toggle"
    >
      <span class="inline-field-guard__dot" aria-hidden="true"></span>
      <span class="inline-field-guard__text">{{ warningText }}</span>
    </button>
    <span v-else class="inline-field-guard__chip" role="note" :title="warningText">
      <span class="inline-field-guard__dot" aria-hidden="true"></span>
      <span class="inline-field-guard__text">{{ warningText }}</span>
    </span>

    <QuickEditPopover
      v-if="canEdit"
      :open="open"
      :label="descriptor.label"
      :type="descriptor.type"
      :hint="descriptor.hint"
      :current="currentValue"
      :saving="saving"
      :error-message="errorMessage"
      @save="handleSave"
      @close="close"
    />
  </span>
</template>

<style scoped>
.inline-field-guard {
  position: relative;
  display: inline-flex;
}

.inline-field-guard__chip {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  max-width: 100%;
  padding: 0.18rem 0.45rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--border) / 0.7);
  background: rgb(var(--surface-2) / 0.76);
  color: var(--text-muted);
  font-size: 0.66rem;
  font-weight: 600;
  line-height: 1.2;
  text-align: left;
}

.inline-field-guard__chip--editable {
  cursor: pointer;
  border-color: rgb(var(--ring) / 0.45);
  color: var(--text-main);
  transition:
    border-color 120ms ease,
    background 120ms ease;
}

.inline-field-guard__chip--editable:hover,
.inline-field-guard__chip--editable[aria-expanded='true'] {
  border-color: rgb(var(--primary) / 0.55);
  background: rgb(var(--primary) / 0.1);
}

.inline-field-guard__dot {
  flex-shrink: 0;
  width: 0.4rem;
  height: 0.4rem;
  border-radius: 999px;
  background: var(--accent-warning);
}

.inline-field-guard__text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
