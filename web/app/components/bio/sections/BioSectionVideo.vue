<script setup lang="ts">
import { computed } from 'vue'

import BioMediaField from '~/components/bio/BioMediaField.vue'
import BioSectionCard from '~/components/bio/BioSectionCard.vue'
import type {
  BioAlignItems,
  BioData,
  BioLayout,
  BioMediaKind,
  BioVideo,
  BioVideoOverlay,
} from '~/domain/bio/types'

// Secao fundo + layout SIMPLIFICADA. Tres campos unificados — Background mobile,
// Background desktop e Poster — cada um aceita IMAGEM ou VIDEO; o upload detecta
// o tipo pela extensao da URL e grava em bgVideo/bgVideoPc (video) ou
// bgImage/bgImagePc (imagem). Mantem overlay + ajustes de layout. Os toggles de
// "Slide do topo ativo" (secao Slides) e "Header mobile ativo" (secao Links)
// NAO ficam aqui.

const props = defineProps<{
  draft: BioData
  bioId: string
  uploadMedia: (kind: BioMediaKind, file: File) => Promise<string | null>
  isUploading: (kind: BioMediaKind) => boolean
}>()

const emit = defineEmits<{ (e: 'update:draft', value: BioData): void }>()

const ALIGN_OPTIONS: { label: string; value: BioAlignItems }[] = [
  { label: 'Topo (start)', value: 'start' },
  { label: 'Centro (center)', value: 'center' },
  { label: 'Base (end)', value: 'end' },
]

// Limites informativos (espelham os defaults do back BIO_MAX_VIDEO_MB /
// BIO_MAX_IMAGE_MB). Se mudar a env no back, atualizar aqui.
const maxVideoMb = 200
const maxImageMb = 10

const VIDEO_EXT = /\.(mp4|webm|ogg|mov|m4v)(\?.*)?$/i

const overlay = computed<BioVideoOverlay>(() => props.draft.video?.overlay || {})
const layout = computed<BioLayout>(() => props.draft.layout || {})

// Campo de fundo unificado: mostra o video se houver, senao a imagem.
const bgMobile = computed(() => props.draft.video?.bgVideo || props.draft.video?.bgImage || '')
const bgDesktop = computed(() => props.draft.video?.bgVideoPc || props.draft.video?.bgImagePc || '')
const poster = computed(() => props.draft.video?.poster || '')

function patchVideo(patch: Partial<BioVideo>) {
  emit('update:draft', {
    ...props.draft,
    video: { ...(props.draft.video || { bgVideo: '' }), ...patch },
  })
}

// Grava o fundo no slot certo (video vs imagem) detectando o tipo pela URL e
// limpando o slot oposto para nao deixar lixo de uma troca de tipo.
function setBackground(scope: 'mobile' | 'desktop', url: string) {
  const isVideo = VIDEO_EXT.test(url)
  if (scope === 'mobile') {
    patchVideo({ bgVideo: isVideo ? url : '', bgImage: isVideo ? '' : url })
    return
  }
  patchVideo({ bgVideoPc: isVideo ? url : '', bgImagePc: isVideo ? '' : url })
}

function setOverlay<K extends keyof BioVideoOverlay>(key: K, value: BioVideoOverlay[K]) {
  const baseVideo = props.draft.video || ({ bgVideo: '' } as BioVideo)
  emit('update:draft', {
    ...props.draft,
    video: { ...baseVideo, overlay: { ...(baseVideo.overlay || {}), [key]: value } },
  })
}

function setLayout<K extends keyof BioLayout>(key: K, value: BioLayout[K]) {
  emit('update:draft', {
    ...props.draft,
    layout: { ...(props.draft.layout || {}), [key]: value },
  })
}

function toNumber(value: string): number {
  const parsed = Number(String(value || '').trim())
  return Number.isFinite(parsed) ? parsed : 0
}
</script>

<template>
  <BioSectionCard
    title="Fundo e layout"
    description="Fundo da pagina (imagem OU video) por dispositivo, poster, overlay e ajustes de layout. Para publicar, basta um fundo + o logo."
  >
    <div class="bio-subsection">
      <h3 class="bio-subsection__title">Fundo (imagem ou video)</h3>
      <p class="bio-subsection__hint">
        Cada campo aceita imagem ou video — o tipo e detectado no upload. Limites: video ate
        {{ maxVideoMb }} MB, imagem ate {{ maxImageMb }} MB.
      </p>
      <div class="bio-media-grid">
        <BioMediaField
          label="Background (mobile)"
          kind="background"
          accept="image/*,video/mp4,video/webm"
          :model-value="bgMobile"
          :uploading="isUploading('background') || isUploading('video')"
          :on-upload="uploadMedia"
          :duplicate-targets="[
            { label: 'Desktop', apply: (v) => setBackground('desktop', v) },
            { label: 'Poster', apply: (v) => patchVideo({ poster: v }) },
          ]"
          @update:model-value="setBackground('mobile', String($event ?? ''))"
        />
        <BioMediaField
          label="Background (desktop)"
          kind="background"
          accept="image/*,video/mp4,video/webm"
          :model-value="bgDesktop"
          :uploading="isUploading('background') || isUploading('video')"
          :on-upload="uploadMedia"
          :duplicate-targets="[
            { label: 'Mobile', apply: (v) => setBackground('mobile', v) },
            { label: 'Poster', apply: (v) => patchVideo({ poster: v }) },
          ]"
          @update:model-value="setBackground('desktop', String($event ?? ''))"
        />
        <BioMediaField
          label="Poster"
          kind="poster"
          accept="image/*,video/mp4,video/webm"
          :model-value="poster"
          :uploading="isUploading('poster')"
          :on-upload="uploadMedia"
          :duplicate-targets="[
            { label: 'Mobile', apply: (v) => setBackground('mobile', v) },
            { label: 'Desktop', apply: (v) => setBackground('desktop', v) },
          ]"
          @update:model-value="patchVideo({ poster: String($event ?? '') })"
        />
      </div>
    </div>

    <div class="bio-subsection">
      <h3 class="bio-subsection__title">Overlay</h3>
      <div class="bio-section-grid">
        <div class="bio-field bio-field--switch">
          <label class="bio-field__label">Ativar overlay</label>
          <USwitch
            :model-value="overlay.active ?? false"
            @update:model-value="setOverlay('active', Boolean($event))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Cor do overlay</label>
          <UInput
            :model-value="overlay.color || ''"
            placeholder="#000000 ou rgb(...)"
            @update:model-value="setOverlay('color', String($event ?? ''))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Opacidade (0 a 1)</label>
          <UInput
            type="number"
            step="0.05"
            :model-value="overlay.opacity ?? 0"
            @update:model-value="setOverlay('opacity', toNumber(String($event ?? '')))"
          />
        </div>
      </div>
    </div>

    <div class="bio-subsection">
      <h3 class="bio-subsection__title">Layout</h3>
      <div class="bio-section-grid">
        <div class="bio-field">
          <label class="bio-field__label">Alinhamento</label>
          <USelect
            :model-value="layout.alignItems || 'center'"
            :items="ALIGN_OPTIONS"
            value-key="value"
            @update:model-value="setLayout('alignItems', $event as BioAlignItems)"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Delay da animacao (ms)</label>
          <UInput
            type="number"
            :model-value="layout.animDelayProjection ?? 0"
            @update:model-value="setLayout('animDelayProjection', toNumber(String($event ?? '')))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Template do corpo</label>
          <UInput
            :model-value="layout.bodyTemplateName || ''"
            placeholder="default"
            @update:model-value="setLayout('bodyTemplateName', String($event ?? ''))"
          />
        </div>
      </div>
    </div>
  </BioSectionCard>
</template>

<style scoped>
.bio-media-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 0.75rem;
}

.bio-section-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.75rem 1rem;
}

.bio-field {
  display: grid;
  gap: 0.3rem;
}

.bio-field--switch {
  align-content: start;
}

.bio-field__label {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-muted);
}

.bio-subsection {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  padding: 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.35);
}

.bio-subsection__title {
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--text-main);
}

.bio-subsection__hint {
  font-size: 0.74rem;
  color: var(--text-muted);
  margin: 0;
}
</style>
