<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { PRODUCT_PHOTO_ACCEPT, productPhotoError } from '~/domain/cardapio/product-photo'

// Foto do produto em modal (versao do site, nao a miniatura), para conferir antes
// de trocar. Fundo desfocado; nome e botoes (Trocar, Remover, fechar) flutuam no
// topo. Fecha com Esc, no X ou clicando fora da foto. Teleportado para o body
// para ficar acima da barra do app e do botao flutuante.
const props = defineProps<{
  name: string
  src: string
  uploading: boolean
  removing: boolean
}>()
const emit = defineEmits<{
  close: []
  upload: [file: File]
  invalid: [message: string]
  remove: []
}>()

const input = ref<HTMLInputElement | null>(null)
const loaded = ref(false)

function pick() {
  if (props.uploading || props.removing) return
  input.value?.click()
}

function onChange(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  target.value = ''
  if (!file) return
  const error = productPhotoError(file)
  if (error) {
    emit('invalid', error)
    return
  }
  loaded.value = false
  emit('upload', file)
}

// Envio terminou (com sucesso ou erro): para de mostrar o carregamento; se a url
// mudou, o @load da foto nova confirma de novo.
watch(
  () => props.uploading,
  (busy) => {
    if (!busy) loaded.value = true
  },
)

function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') emit('close')
}

onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <Teleport to="body">
    <div class="cardapio-viewer" role="dialog" aria-modal="true" :aria-label="`Foto de ${name}`">
      <header class="cardapio-viewer__bar" @click.self="emit('close')">
        <p class="cardapio-viewer__name">{{ name }}</p>
        <div class="cardapio-viewer__actions">
          <UButton
            icon="i-lucide-pencil"
            label="Trocar foto"
            color="neutral"
            variant="soft"
            :loading="uploading"
            :disabled="uploading || removing"
            @click="pick"
          />
          <UButton
            icon="i-lucide-trash-2"
            label="Remover"
            color="error"
            variant="soft"
            :loading="removing"
            :disabled="uploading || removing"
            @click="emit('remove')"
          />
          <UButton
            icon="i-lucide-x"
            color="neutral"
            variant="ghost"
            aria-label="Fechar"
            title="Fechar (Esc)"
            @click="emit('close')"
          />
        </div>
      </header>
      <div class="cardapio-viewer__stage" @click.self="emit('close')">
        <UIcon
          v-if="!loaded || uploading"
          name="i-lucide-loader-circle"
          class="cardapio-viewer__spin"
        />
        <img
          v-if="src"
          v-show="loaded && !uploading"
          :src="src"
          :alt="name"
          @load="loaded = true"
          @error="loaded = true"
        />
      </div>
      <input ref="input" type="file" :accept="PRODUCT_PHOTO_ACCEPT" hidden @change="onChange" />
    </div>
  </Teleport>
</template>

<style scoped>
.cardapio-viewer {
  position: fixed;
  inset: 0;
  z-index: 700;
  /* Modal: o painel continua visivel atras, desfocado e escurecido. */
  background: rgb(0 0 0 / 0.55);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
}

/* Nome e botoes flutuam sobre o fundo (sem barra). */
.cardapio-viewer__bar {
  position: absolute;
  top: 0.75rem;
  right: 0.75rem;
  left: 0.75rem;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.cardapio-viewer__name,
.cardapio-viewer__actions {
  border: 1px solid rgb(255 255 255 / 0.12);
  border-radius: 999px;
  background: rgb(var(--surface) / 0.78);
  box-shadow: var(--shadow-md);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.cardapio-viewer__name {
  min-width: 0;
  margin: 0;
  padding: 0.45rem 0.95rem;
  overflow: hidden;
  color: var(--text-main);
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cardapio-viewer__actions {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 0.25rem;
  padding: 0.25rem;
}

.cardapio-viewer__actions :deep(button) {
  border-radius: 999px;
}

.cardapio-viewer__stage {
  position: absolute;
  inset: 0;
}

/* Foto centralizada, sempre inteira, com margem para os controles flutuantes. */
.cardapio-viewer__stage img {
  position: absolute;
  inset: 4.25rem 1.5rem 1.5rem;
  margin: auto;
  max-width: calc(100% - 3rem);
  max-height: calc(100% - 5.75rem);
  object-fit: contain;
  border-radius: var(--radius-md);
  box-shadow: 0 20px 60px rgb(0 0 0 / 0.5);
}

.cardapio-viewer__spin {
  position: absolute;
  inset: 0;
  margin: auto;
  width: 2rem;
  height: 2rem;
  color: rgb(255 255 255 / 0.85);
  animation: cardapio-viewer-spin 0.8s linear infinite;
}

@keyframes cardapio-viewer-spin {
  to {
    transform: rotate(360deg);
  }
}

/* Mobile: botoes so com icone; foto quase de borda a borda. */
@media (max-width: 720px) {
  .cardapio-viewer__bar {
    top: 0.5rem;
    right: 0.5rem;
    left: 0.5rem;
  }

  .cardapio-viewer__actions :deep(.truncate),
  .cardapio-viewer__actions :deep([data-slot='label']) {
    display: none;
  }

  .cardapio-viewer__stage img {
    inset: 3.75rem 0.4rem 0.75rem;
    max-width: calc(100% - 0.8rem);
    max-height: calc(100% - 4.5rem);
  }
}
</style>
