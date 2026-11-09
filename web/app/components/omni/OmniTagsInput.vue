<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    modelValue: string[]
    options: { label: string; value: string | number }[]
    placeholder?: string
    size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl'
    creatable?: boolean
    disabled?: boolean
  }>(),
  { placeholder: 'Selecionar tags', size: 'md', creatable: false, disabled: false },
)

const emit = defineEmits<{ 'update:modelValue': [value: string[]] }>()
const search = ref('')
const items = computed(() => [
  ...new Set([...props.options.map((option) => String(option.value)), ...props.modelValue]),
])

function createTag(value: string) {
  const tag = value.trim()
  if (!tag) return
  const existing = items.value.find((item) => item.toLowerCase() === tag.toLowerCase())
  emit('update:modelValue', [...new Set([...props.modelValue, existing ?? tag])])
  search.value = ''
}
</script>

<template>
  <UInputMenu
    v-model:search-term="search"
    :model-value="modelValue"
    :items="items"
    :placeholder="placeholder"
    :size="size"
    :disabled="disabled"
    :create-item="creatable"
    multiple
    clear
    open-on-click
    open-on-focus
    class="omni-tags-input"
    :class="{ 'is-compact': size === 'sm' }"
    @update:model-value="emit('update:modelValue', $event)"
    @clear="emit('update:modelValue', [])"
    @create="createTag"
  >
    <template #create-item-label="{ item }">Criar "{{ item }}"</template>
  </UInputMenu>
</template>

<style scoped>
.omni-tags-input {
  width: 100%;
  min-width: 0;
}
.omni-tags-input.is-compact :deep([data-slot='base']) {
  min-height: 2rem;
  max-height: 2rem;
  overflow: hidden;
  flex-wrap: nowrap;
}
.omni-tags-input.is-compact :deep([data-slot='tagsItem']) {
  max-width: 6rem;
  min-width: 0;
}
.omni-tags-input.is-compact :deep([data-slot='tagsItemText']) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
