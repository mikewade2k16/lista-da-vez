<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRuntimeConfig } from '#app'

import type { BioMediaKind } from '~/domain/bio/types'

// Campo de midia reutilizavel e COMPACTO. O proprio botao de upload e o PREVIEW
// da midia: thumbnail <img> para imagem, <video muted> para video (detectado
// pela extensao/tipo da URL). A URL aparece so no HOVER (atributo title), nunca
// numa linha de texto. O campo de colar URL manual fica escondido atras de um
// toggle. O upload em si fica no composable do editor; este componente so
// dispara `onUpload(kind, file)` e escreve o resultado no v-model.

// Alvos para DUPLICAR o valor atual deste campo em outras variantes (ex.: copiar
// o logo mobile para o desktop). Cada alvo recebe a URL atual via `apply`.
interface BioMediaDuplicateTarget {
  label: string
  apply: (value: string) => void
}

const props = defineProps<{
  modelValue?: string
  label: string
  kind: BioMediaKind
  accept?: string
  hint?: string
  preview?: boolean
  uploading?: boolean
  duplicateTargets?: BioMediaDuplicateTarget[]
  onUpload: (kind: BioMediaKind, file: File) => Promise<string | null>
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const fileInput = ref<HTMLInputElement | null>(null)
const showUrlInput = ref(false)

const VIDEO_EXT = /\.(mp4|webm|ogg|mov|m4v)(\?.*)?$/i
const IMAGE_EXT = /\.(png|jpe?g|gif|webp|avif|svg|bmp|ico)(\?.*)?$/i

const value = computed(() => String(props.modelValue || ''))
const hasValue = computed(() => Boolean(value.value))
const showPreview = computed(() => props.preview !== false && hasValue.value)

// So oferecemos "copiar para" quando ha valor e ha pelo menos um alvo.
const duplicateTargets = computed(() => props.duplicateTargets || [])
const showDuplicate = computed(
  () => hasValue.value && !props.uploading && duplicateTargets.value.length > 0,
)

function duplicateTo(target: BioMediaDuplicateTarget) {
  target.apply(value.value)
}

// Para EXIBIR o preview, resolvemos a URL ao dominio que serve o arquivo: o
// painel roda em outro dominio, entao /uploads/ vem do back e /assets/ do front
// bio. Sem isto, a imagem ja salva (ex.: /assets/.../slide.jpg) aparece quebrada.
const config = useRuntimeConfig()
const apiBase = String((config.public as Record<string, unknown>).apiBase || '').replace(/\/$/, '')
const bioFrontUrl = String((config.public as Record<string, unknown>).bioFrontUrl || '').replace(
  /\/$/,
  '',
)
const previewSrc = computed(() => {
  const v = value.value
  if (!v || /^(https?:|data:|blob:)/i.test(v)) {
    return v
  }
  if (v.startsWith('/uploads/')) {
    return apiBase ? apiBase + v : v
  }
  if (v.startsWith('/assets/')) {
    return bioFrontUrl ? bioFrontUrl + v : v
  }
  return v
})

// Detecta video por extensao da URL; cai para imagem quando nao reconhece.
const isVideo = computed(() => {
  if (!value.value) {
    return false
  }
  if (VIDEO_EXT.test(value.value)) {
    return true
  }
  if (IMAGE_EXT.test(value.value)) {
    return false
  }
  // Sem extensao reconhecida: usa o kind como dica (video/background podem ser ambos).
  return props.kind === 'video'
})

function pickFile() {
  fileInput.value?.click()
}

function clearValue() {
  emit('update:modelValue', '')
}

async function onFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) {
    return
  }
  const url = await props.onUpload(props.kind, file)
  if (url) {
    emit('update:modelValue', url)
  }
  // Permite re-selecionar o mesmo arquivo depois de um erro.
  target.value = ''
}
</script>

<template>
  <div class="bio-media-field">
    <div class="bio-media-field__head">
      <label class="bio-media-field__label">{{ label }}</label>
      <button
        type="button"
        class="bio-media-field__toggle"
        :aria-pressed="showUrlInput"
        @click="showUrlInput = !showUrlInput"
      >
        <UIcon name="i-lucide-link" />
        URL
      </button>
    </div>

    <div class="bio-media-field__body">
      <button
        type="button"
        class="bio-media-field__tile"
        :class="{ 'bio-media-field__tile--filled': showPreview }"
        :title="value || 'Enviar arquivo'"
        :disabled="uploading"
        @click="pickFile"
      >
        <span v-if="uploading" class="bio-media-field__tile-state">
          <UIcon name="i-lucide-loader-circle" class="bio-media-field__spin" />
        </span>
        <template v-else-if="showPreview">
          <video
            v-if="isVideo"
            :src="previewSrc"
            class="bio-media-field__media"
            muted
            playsinline
            preload="metadata"
          ></video>
          <img
            v-else
            :src="previewSrc"
            :alt="label"
            class="bio-media-field__media"
            loading="lazy"
          />
          <span class="bio-media-field__overlay">
            <UIcon name="i-lucide-pencil" />
          </span>
        </template>
        <span v-else class="bio-media-field__tile-state">
          <UIcon name="i-lucide-upload" />
          <span class="bio-media-field__tile-text">Enviar</span>
        </span>
      </button>

      <button
        v-if="hasValue && !uploading"
        type="button"
        class="bio-media-field__clear"
        aria-label="Remover midia"
        :title="value"
        @click="clearValue"
      >
        <UIcon name="i-lucide-x" />
      </button>

      <input
        ref="fileInput"
        type="file"
        class="bio-media-field__input"
        :accept="accept || '*/*'"
        @change="onFileChange"
      />
    </div>

    <div v-if="showDuplicate" class="bio-media-field__dupes">
      <span class="bio-media-field__dupes-lead">Copiar para</span>
      <button
        v-for="target in duplicateTargets"
        :key="target.label"
        type="button"
        class="bio-media-field__dupe"
        :title="`Usar esta midia em: ${target.label}`"
        @click="duplicateTo(target)"
      >
        <UIcon name="i-lucide-copy" />
        {{ target.label }}
      </button>
    </div>

    <UInput
      v-if="showUrlInput"
      class="bio-media-field__url"
      :model-value="value"
      placeholder="Cole uma URL"
      @update:model-value="emit('update:modelValue', String($event ?? ''))"
    />

    <p v-if="hint" class="bio-media-field__hint">{{ hint }}</p>
  </div>
</template>

<style scoped>
.bio-media-field {
  display: grid;
  gap: 0.3rem;
}

.bio-media-field__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.bio-media-field__label {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-muted);
}

.bio-media-field__toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  padding: 0.1rem 0.35rem;
  font-size: 0.66rem;
  font-weight: 700;
  color: var(--text-muted);
  background: transparent;
  border: 1px solid var(--line-soft);
  border-radius: 0.35rem;
  cursor: pointer;
}

.bio-media-field__toggle[aria-pressed='true'] {
  color: rgb(var(--primary));
  border-color: rgb(var(--primary) / 0.4);
}

.bio-media-field__body {
  display: flex;
  align-items: flex-start;
  gap: 0.35rem;
}

.bio-media-field__tile {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 86px;
  height: 64px;
  padding: 0;
  overflow: hidden;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.5);
  color: var(--text-muted);
  cursor: pointer;
}

.bio-media-field__tile--filled {
  border-style: solid;
}

.bio-media-field__tile:hover:not(:disabled) {
  border-color: rgb(var(--primary) / 0.5);
  color: rgb(var(--primary));
}

.bio-media-field__tile:disabled {
  cursor: progress;
  opacity: 0.8;
}

.bio-media-field__media {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.bio-media-field__overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1rem;
  color: rgb(var(--surface));
  background: rgb(0 0 0 / 0.32);
  opacity: 0;
  transition: opacity 0.12s ease;
}

.bio-media-field__tile:hover .bio-media-field__overlay {
  opacity: 1;
}

.bio-media-field__tile-state {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  gap: 0.15rem;
  font-size: 1.05rem;
}

.bio-media-field__tile-text {
  font-size: 0.66rem;
  font-weight: 700;
}

.bio-media-field__spin {
  animation: bio-media-spin 0.8s linear infinite;
}

@keyframes bio-media-spin {
  to {
    transform: rotate(360deg);
  }
}

.bio-media-field__clear {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.5rem;
  height: 1.5rem;
  flex-shrink: 0;
  border: 1px solid var(--line-soft);
  border-radius: 0.4rem;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
}

.bio-media-field__clear:hover {
  color: rgb(var(--danger));
  border-color: rgb(var(--danger) / 0.4);
}

.bio-media-field__input {
  display: none;
}

.bio-media-field__dupes {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.25rem;
}

.bio-media-field__dupes-lead {
  font-size: 0.64rem;
  font-weight: 700;
  color: var(--text-muted);
}

.bio-media-field__dupe {
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  padding: 0.1rem 0.35rem;
  font-size: 0.64rem;
  font-weight: 700;
  color: var(--text-muted);
  background: transparent;
  border: 1px solid var(--line-soft);
  border-radius: 0.35rem;
  cursor: pointer;
}

.bio-media-field__dupe:hover {
  color: rgb(var(--primary));
  border-color: rgb(var(--primary) / 0.4);
}

.bio-media-field__url {
  width: 100%;
}

.bio-media-field__hint {
  margin: 0;
  font-size: 0.7rem;
  color: var(--text-muted);
  line-height: 1.4;
}
</style>
