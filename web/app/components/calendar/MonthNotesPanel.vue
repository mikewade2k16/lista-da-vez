<script setup lang="ts">
import OmniEditor from '~/components/omni/OmniEditor.vue'

withDefaults(
  defineProps<{
    title: string
    modelValue: string
    peopleNames?: string[]
    clientNames?: string[]
    syncLabel?: string
  }>(),
  {
    peopleNames: () => [],
    clientNames: () => [],
    syncLabel: 'Salvo automaticamente',
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
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
    </header>

    <div class="calendar-notes__editor">
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
