<script setup lang="ts">
import { computed, ref } from 'vue'
import OmniMediaGrid, { type OmniMediaGridItem } from '~/components/ui/OmniMediaGrid.vue'

const props = defineProps<{
  productId: string
  name: string
  src: string
  uploading: boolean
  disabled: boolean
}>()
const emit = defineEmits<{ upload: [file: File] }>()
const error = ref('')
const items = computed<OmniMediaGridItem[]>(() =>
  props.src
    ? [
        {
          id: props.productId,
          name: props.name,
          type: 'image',
          url: props.src,
          removable: false,
        },
      ]
    : [],
)

function upload(files: File[]) {
  if (props.disabled || props.uploading) return
  error.value = ''
  if (files.length !== 1) {
    error.value = 'Envie uma imagem por produto.'
    return
  }
  emit('upload', files[0]!)
}
</script>

<template>
  <div
    class="cardapio-product-image"
    :class="{ 'has-image': Boolean(src) }"
    :aria-busy="uploading"
    @dragover.prevent
    @drop.prevent
  >
    <OmniMediaGrid
      :items="items"
      :readonly="disabled || uploading"
      :reorderable="false"
      :show-cover="false"
      :error="error"
      accept="image/jpeg,image/png,image/webp,image/gif"
      :upload-label="src ? `Substituir imagem de ${name}` : `Adicionar imagem de ${name}`"
      @files="upload"
    />
    <span v-if="uploading" class="cardapio-product-image__status" role="status">Enviando...</span>
  </div>
</template>

<style scoped>
.cardapio-product-image {
  width: 88px;
  max-width: 100%;
}
.cardapio-product-image :deep(.omni-media-grid__items) {
  position: relative;
  display: block;
}
.cardapio-product-image :deep(.omni-media-grid__cell) {
  width: 100%;
}
.cardapio-product-image.has-image :deep(.omni-media-grid__add) {
  position: absolute;
  z-index: 3;
  top: 0.25rem;
  right: 0.25rem;
  width: 1.5rem;
  height: 1.5rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface) / 0.9);
}
.cardapio-product-image.has-image :deep(.omni-media-grid__add svg) {
  width: 0.875rem;
  height: 0.875rem;
}
.cardapio-product-image :deep(.omni-media-grid__preview),
.cardapio-product-image :deep(.omni-media-grid__add) {
  height: 56px;
  aspect-ratio: auto;
}
.cardapio-product-image__status {
  color: rgb(var(--muted));
  font-size: 0.75rem;
}
</style>
