<script setup lang="ts">
// Galeria de imagens do produto. Recebe a lista de URLs e expoe o upload via
// callback do pai (que conhece o restaurantId/store). Upload preenche a galeria.
const props = defineProps<{
  modelValue: string[]
  uploading: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string[]): void
  (e: 'upload', file: File): void
}>()

function remove(index: number) {
  emit(
    'update:modelValue',
    props.modelValue.filter((_, i) => i !== index),
  )
}

function onFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) {
    emit('upload', file)
  }
  input.value = ''
}
</script>

<template>
  <div class="cardapio-gallery">
    <div class="cardapio-gallery__head">
      <span class="cardapio-gallery__title">Galeria</span>
      <label class="cardapio-gallery__upload">
        <input type="file" accept="image/*" hidden :disabled="uploading" @change="onFile" />
        {{ uploading ? 'Enviando...' : 'Enviar imagem' }}
      </label>
    </div>
    <p v-if="!modelValue.length" class="cardapio-gallery__empty">Nenhuma imagem na galeria.</p>
    <div v-else class="cardapio-gallery__grid">
      <div v-for="(url, index) in modelValue" :key="index" class="cardapio-gallery__item">
        <img :src="url" alt="Imagem do produto" class="cardapio-gallery__img" />
        <button
          type="button"
          class="cardapio-gallery__remove"
          aria-label="Remover imagem"
          @click="remove(index)"
        >
          &times;
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.cardapio-gallery {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.cardapio-gallery__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.cardapio-gallery__title {
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-gallery__upload {
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  padding: 0.35rem 0.7rem;
  border-radius: var(--radius-sm);
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-gallery__empty {
  font-size: 0.82rem;
  color: var(--text-muted);
}

.cardapio-gallery__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(80px, 1fr));
  gap: 0.55rem;
}

.cardapio-gallery__item {
  position: relative;
  aspect-ratio: 1;
  border-radius: var(--radius-sm);
  overflow: hidden;
  border: 1px solid var(--line-soft);
}

.cardapio-gallery__img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.cardapio-gallery__remove {
  position: absolute;
  top: 0.25rem;
  right: 0.25rem;
  width: 1.3rem;
  height: 1.3rem;
  border: none;
  border-radius: 999px;
  background: rgb(var(--text) / 0.55);
  color: rgb(var(--surface));
  font-size: 0.95rem;
  line-height: 1;
  cursor: pointer;
}
</style>
