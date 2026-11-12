<script setup lang="ts">
import type Sortable from 'sortablejs'
const props = withDefaults(defineProps<{ tag?: string; handle: string; disabled?: boolean }>(), {
  tag: 'div',
  disabled: false,
})
const emit = defineEmits<{ move: [from: number, to: number] }>()
const host = ref<HTMLElement>()
let sortable: Sortable | undefined
let disposed = false
onMounted(async () => {
  const { default: SortableList } = await import('sortablejs')
  if (disposed || !host.value) return
  sortable = new SortableList(host.value, {
    animation: 150,
    handle: props.handle,
    disabled: props.disabled,
    filter: '.ql-editor, .ql-container, .row-remove, .btn-delete',
    preventOnFilter: false,
    onEnd(event) {
      const { oldIndex: from, newIndex: to, item } = event
      if (from === undefined || to === undefined || from === to || !host.value) return
      const siblings = [...host.value.children].filter((node) => node !== item)
      host.value.insertBefore(item, siblings[from] || null)
      emit('move', from, to)
    },
  })
})
watch(
  () => props.disabled,
  (value) => sortable?.option('disabled', value),
)
onBeforeUnmount(() => {
  disposed = true
  sortable?.destroy()
})
</script>
<template>
  <component :is="tag" ref="host"><slot></slot></component>
</template>
