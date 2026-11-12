<script setup lang="ts">
import OmniTagsInput from '~/components/omni/OmniTagsInput.vue'
import type { Category, ProductBulkAction } from '~/domain/cardapio/types'

// Modal das acoes em lote que pedem um valor: adicionar tag ou mudar categoria
// dos produtos selecionados. Quem aplica e o useCardapioProductBulk.
defineProps<{
  count: number
  busy: boolean
  tagOptions: Array<{ label: string; value: string }>
  categories: Category[]
}>()
const editor = defineModel<'add_tag' | 'set_category' | ''>('editor', { required: true })
const tag = defineModel<string[]>('tag', { required: true })
const category = defineModel<string>('category', { required: true })
const emit = defineEmits<{ apply: [action: ProductBulkAction, value: string] }>()

function apply() {
  if (!editor.value) return
  emit('apply', editor.value, editor.value === 'add_tag' ? (tag.value[0] ?? '') : category.value)
}
</script>

<template>
  <UModal
    :open="Boolean(editor)"
    :title="editor === 'add_tag' ? 'Adicionar tag aos produtos' : 'Mudar categoria dos produtos'"
    @update:open="!$event && (editor = '')"
  >
    <template #body>
      <p>{{ count }} produto(s) selecionado(s).</p>
      <OmniTagsInput
        v-if="editor === 'add_tag'"
        v-model="tag"
        :options="tagOptions"
        placeholder="Escolha ou crie uma tag"
        creatable
      />
      <USelect
        v-else
        v-model="category"
        :items="categories.map((item) => ({ label: item.name, value: item.id }))"
        placeholder="Escolha a categoria"
      />
    </template>
    <template #footer>
      <UButton
        label="Cancelar"
        color="neutral"
        variant="soft"
        :disabled="busy"
        @click="editor = ''"
      />
      <UButton
        label="Aplicar"
        :loading="busy"
        :disabled="busy || (editor === 'add_tag' ? tag.length !== 1 : !category)"
        @click="apply"
      />
    </template>
  </UModal>
</template>
