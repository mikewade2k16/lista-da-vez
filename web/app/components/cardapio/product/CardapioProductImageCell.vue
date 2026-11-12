<script setup lang="ts">
import { ref } from 'vue'

import { PRODUCT_PHOTO_ACCEPT, productPhotoError } from '~/domain/cardapio/product-photo'

// Celula de foto da tabela/cards de Produtos. Leve de proposito (centenas de
// linhas): miniatura com lazy/async decode e um unico input de arquivo.
// Sem foto: clique (ou arraste) para enviar. Com foto: clique abre em tela
// cheia; os icones sobre a miniatura trocam (lapis) ou removem (lixeira).
// Arrastar uma foto sobre a miniatura tambem troca.
const props = defineProps<{
  name: string
  src: string
  uploading: boolean
  disabled: boolean
}>()
const emit = defineEmits<{
  upload: [file: File]
  invalid: [message: string]
  view: []
  remove: []
}>()

const input = ref<HTMLInputElement | null>(null)
const dragging = ref(false)
const broken = ref(false)

function pick() {
  if (props.disabled || props.uploading) return
  input.value?.click()
}

function onMainClick() {
  if (props.src && !broken.value) {
    emit('view')
    return
  }
  pick()
}

function send(file: File | undefined) {
  if (!file || props.disabled || props.uploading) return
  const error = productPhotoError(file)
  if (error) {
    emit('invalid', error)
    return
  }
  broken.value = false
  emit('upload', file)
}

function onChange(event: Event) {
  const target = event.target as HTMLInputElement
  send(target.files?.[0])
  target.value = ''
}

function onDrop(event: DragEvent) {
  dragging.value = false
  send(event.dataTransfer?.files?.[0])
}
</script>

<template>
  <div
    class="cardapio-photo"
    :class="{
      'cardapio-photo--empty': !src || broken,
      'cardapio-photo--drag': dragging,
      'cardapio-photo--busy': uploading,
    }"
    @dragover.prevent="dragging = true"
    @dragleave.prevent="dragging = false"
    @drop.prevent="onDrop"
  >
    <button
      type="button"
      class="cardapio-photo__main"
      :disabled="disabled && !(src && !broken)"
      :aria-busy="uploading"
      :aria-label="
        src && !broken ? `Ver foto de ${name} em tela cheia` : `Adicionar foto de ${name}`
      "
      :title="src && !broken ? 'Ver em tela cheia' : 'Clique ou arraste uma foto'"
      @click="onMainClick"
    >
      <img
        v-if="src && !broken"
        :src="src"
        :alt="name"
        loading="lazy"
        decoding="async"
        width="72"
        height="72"
        draggable="false"
        @error="broken = true"
      />
      <span v-else class="cardapio-photo__empty">
        <UIcon name="i-lucide-camera" class="cardapio-photo__icon" />
        <span>Adicionar</span>
      </span>
    </button>
    <div v-if="src && !broken && !uploading" class="cardapio-photo__tools">
      <button
        type="button"
        :disabled="disabled"
        :aria-label="`Trocar foto de ${name}`"
        title="Trocar foto"
        @click.stop="pick"
      >
        <UIcon name="i-lucide-pencil" />
      </button>
      <button
        type="button"
        class="is-danger"
        :disabled="disabled"
        :aria-label="`Remover foto de ${name}`"
        title="Remover foto"
        @click.stop="emit('remove')"
      >
        <UIcon name="i-lucide-trash-2" />
      </button>
    </div>
    <span v-if="uploading" class="cardapio-photo__busy" role="status">
      <UIcon name="i-lucide-loader-circle" class="cardapio-photo__spin" />
      <span>Enviando</span>
    </span>
    <input ref="input" type="file" :accept="PRODUCT_PHOTO_ACCEPT" hidden @change="onChange" />
  </div>
</template>

<style scoped>
.cardapio-photo {
  position: relative;
  width: 72px;
  height: 72px;
  overflow: hidden;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  transition:
    border-color 0.12s ease,
    background 0.12s ease;
}

.cardapio-photo:hover,
.cardapio-photo--drag {
  border-color: rgb(var(--primary));
}

.cardapio-photo--empty {
  border-style: dashed;
  border-color: rgb(var(--primary) / 0.55);
  background: rgb(var(--primary) / 0.06);
}

.cardapio-photo__main {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
  padding: 0;
  border: none;
  background: transparent;
  color: rgb(var(--primary));
  cursor: zoom-in;
}

.cardapio-photo--empty .cardapio-photo__main {
  cursor: pointer;
}

.cardapio-photo__main:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.cardapio-photo__main img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.cardapio-photo__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.2rem;
  font-size: 0.7rem;
  font-weight: 600;
}

.cardapio-photo__icon {
  width: 1.25rem;
  height: 1.25rem;
}

/* Icones sobre a miniatura: aparecem no hover/foco (desktop) e sempre no toque. */
.cardapio-photo__tools {
  position: absolute;
  right: 0.2rem;
  bottom: 0.2rem;
  display: flex;
  gap: 0.2rem;
  opacity: 0;
  transition: opacity 0.12s ease;
}

.cardapio-photo:hover .cardapio-photo__tools,
.cardapio-photo:focus-within .cardapio-photo__tools {
  opacity: 1;
}

@media (hover: none) {
  .cardapio-photo__tools {
    opacity: 1;
  }
}

.cardapio-photo__tools button {
  display: grid;
  place-items: center;
  width: 1.4rem;
  height: 1.4rem;
  padding: 0;
  border: none;
  border-radius: 999px;
  background: rgb(var(--surface) / 0.88);
  color: rgb(var(--text));
  font-size: 0.75rem;
  cursor: pointer;
}

.cardapio-photo__tools button.is-danger {
  color: rgb(var(--danger));
}

.cardapio-photo__tools button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.cardapio-photo__busy {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.2rem;
  background: rgb(var(--surface) / 0.82);
  color: rgb(var(--text));
  font-size: 0.68rem;
  font-weight: 600;
}

.cardapio-photo__spin {
  width: 1.1rem;
  height: 1.1rem;
  animation: cardapio-photo-spin 0.8s linear infinite;
}

@keyframes cardapio-photo-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
