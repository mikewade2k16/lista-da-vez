<script setup lang="ts">
import { ref } from 'vue'

// Foto do perfil: clique ou arraste uma imagem sobre o circulo para enviar
// (mesmo gesto da foto de produto no cardapio). Aceita qualquer tamanho: a pagina
// reduz e converte a foto (prepareAvatarImage) antes de enviar.
const ACCEPT = 'image/*'

const props = defineProps<{
  src: string
  initials: string
  pending: boolean
}>()
const emit = defineEmits<{
  upload: [file: File]
  invalid: [message: string]
}>()

const input = ref<HTMLInputElement | null>(null)
const dragging = ref(false)

function pick() {
  if (!props.pending) input.value?.click()
}

function send(file: File | undefined) {
  if (!file || props.pending) return
  if (!file.type.startsWith('image/')) {
    emit('invalid', 'Escolha um arquivo de imagem.')
    return
  }
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
  <div class="profile-avatar">
    <button
      type="button"
      class="profile-avatar__drop"
      :class="{ 'is-dragging': dragging, 'is-pending': pending }"
      :disabled="pending"
      aria-label="Enviar nova foto"
      title="Clique ou arraste uma foto"
      @click="pick"
      @dragover.prevent="dragging = true"
      @dragleave.prevent="dragging = false"
      @drop.prevent="onDrop"
    >
      <img v-if="src" :src="src" alt="Foto do usuario" class="profile-avatar__image" />
      <span v-else class="profile-avatar__initials">{{ initials || 'US' }}</span>
      <span class="profile-avatar__overlay" aria-hidden="true">
        <UIcon
          :name="pending ? 'i-lucide-loader-circle' : 'i-lucide-camera'"
          class="profile-avatar__icon"
          :class="{ 'animate-spin': pending }"
        />
      </span>
    </button>
    <p class="profile-avatar__hint">
      {{ pending ? 'Enviando...' : 'Clique ou arraste uma foto' }}
      <span>A foto é ajustada automaticamente</span>
    </p>
    <input ref="input" type="file" :accept="ACCEPT" hidden @change="onChange" />
  </div>
</template>

<style scoped>
.profile-avatar {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.6rem;
}

.profile-avatar__drop {
  position: relative;
  display: grid;
  place-items: center;
  width: 8.5rem;
  height: 8.5rem;
  padding: 0;
  overflow: hidden;
  border: 2px dashed var(--line-soft);
  border-radius: 999px;
  background: rgb(var(--surface-2));
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease;
}

.profile-avatar__drop:hover,
.profile-avatar__drop:focus-visible,
.profile-avatar__drop.is-dragging {
  border-color: rgb(var(--primary));
  box-shadow: 0 0 0 4px rgb(var(--primary) / 0.15);
  outline: none;
}

.profile-avatar__drop.is-pending {
  cursor: progress;
}

.profile-avatar__image {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.profile-avatar__initials {
  font-size: 2.2rem;
  font-weight: 700;
  color: rgb(var(--primary));
}

.profile-avatar__overlay {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  background: rgb(0 0 0 / 0.45);
  opacity: 0;
  transition: opacity 0.15s ease;
}

.profile-avatar__drop:hover .profile-avatar__overlay,
.profile-avatar__drop:focus-visible .profile-avatar__overlay,
.profile-avatar__drop.is-dragging .profile-avatar__overlay,
.profile-avatar__drop.is-pending .profile-avatar__overlay {
  opacity: 1;
}

.profile-avatar__icon {
  width: 1.6rem;
  height: 1.6rem;
  color: #fff;
}

.profile-avatar__hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.1rem;
  margin: 0;
  font-size: 0.82rem;
  color: var(--text-main);
  text-align: center;
}

.profile-avatar__hint span {
  font-size: 0.74rem;
  color: var(--text-muted);
}
</style>
