<script setup lang="ts">
import { ref } from 'vue'
import OmniEditor from '~/components/omni/OmniEditor.vue'

withDefaults(
  defineProps<{
    title: string
    modelValue: string
    peopleNames?: string[]
    clientNames?: string[]
    syncLabel?: string
    // Presenca (SPEC-F9): "Fulano editando" quando OUTRO usuario edita a nota deste mes.
    editingLabel?: string
  }>(),
  {
    peopleNames: () => [],
    clientNames: () => [],
    syncLabel: 'Salvo automaticamente',
    editingLabel: '',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  focus: []
  blur: []
}>()

// focusin/focusout bolham do contenteditable do OmniEditor; emitimos focus/blur do
// CONJUNTO (nao a cada filho) para a presenca marcar/liberar o campo "notes:YYYY-MM".
const focused = ref(false)
function onFocusIn(): void {
  if (focused.value) return
  focused.value = true
  emit('focus')
}
function onFocusOut(event: FocusEvent): void {
  const next = event.relatedTarget as Node | null
  const container = event.currentTarget as HTMLElement
  if (next && container.contains(next)) return
  focused.value = false
  emit('blur')
}
</script>

<template>
  <aside class="calendar-notes" aria-label="Anotações do mês">
    <header class="calendar-notes__header">
      <div class="calendar-notes__heading">
        <UIcon
          name="i-lucide-notebook-pen"
          class="calendar-notes__heading-icon"
          aria-hidden="true"
        />
        <div class="calendar-notes__heading-text">
          <strong class="calendar-notes__title">Anotações de {{ title }}</strong>
          <span class="calendar-notes__subtitle">{{ syncLabel }}</span>
        </div>
      </div>
      <span v-if="editingLabel" class="calendar-notes__presence" :title="editingLabel">
        <UIcon name="i-lucide-pencil-line" aria-hidden="true" />
        {{ editingLabel }}
      </span>
    </header>

    <div class="calendar-notes__editor" @focusin="onFocusIn" @focusout="onFocusOut">
      <OmniEditor
        :model-value="modelValue"
        content-type="html"
        :people="peopleNames"
        :clients="clientNames"
        placeholder="Escreva o foco do mês, pendências e ideias…"
        min-height="100%"
        max-height="100%"
        @update:model-value="emit('update:modelValue', $event)"
      />
    </div>

    <footer class="calendar-notes__footer">
      <UIcon name="i-lucide-cloud-check" aria-hidden="true" />
      <span>{{ syncLabel }}</span>
    </footer>
  </aside>
</template>
