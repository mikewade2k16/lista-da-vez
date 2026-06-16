<script setup lang="ts">
import { computed, ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRuntimeConfig } from '#app'

import { useBioStore } from '~/stores/bio'
import type { BioData, BioSlide, BioSlideSource } from '~/domain/bio/types'

// Previa ao vivo: iframe apontando para {bioFrontUrl}/bio/preview do crow-nuxt —
// assim o preview usa os MESMOS template/componentes do bio publicado (fidelidade
// total, sem duplicar render). O conteudo do editor vai por postMessage (debounced)
// e o iframe re-renderiza. crow-nuxt roda como servico docker (build de prod), entao
// fica sempre no ar; se a env nao estiver setada, mostramos um aviso.
//
// `source`: 'draft' (rascunho em edicao, padrao) ou 'published' (o que esta no ar).

const props = withDefaults(
  defineProps<{
    draft: BioData
    published?: BioData | null
    source?: 'draft' | 'published'
    isPublished?: boolean
  }>(),
  { published: null, source: 'draft', isPublished: false },
)

const emit = defineEmits<{ (e: 'update:source', value: 'draft' | 'published'): void }>()

function setSource(next: 'draft' | 'published') {
  if (next !== props.source) {
    emit('update:source', next)
  }
}

// Fonte ativa do preview: alterna entre draft (em edicao) e published (no ar).
const activeData = computed<BioData>(() =>
  props.source === 'published' ? (props.published ?? {}) : (props.draft ?? {}),
)

const config = useRuntimeConfig()
const bioFrontUrl = String((config.public as Record<string, unknown>).bioFrontUrl || '').replace(
  /\/$/,
  '',
)
// Base do back (serve /uploads/). Absolutiza a midia enviada pelo painel ANTES de
// mandar pro iframe (que roda em outro dominio e nao serve /uploads/).
const apiBase = String((config.public as Record<string, unknown>).apiBase || '').replace(/\/$/, '')
const previewUrl = bioFrontUrl ? `${bioFrontUrl}/bio/preview` : ''

// Walk recursivo trocando `/uploads/...` por `{apiBase}/uploads/...`. `/assets/...`
// (servidas pelo proprio crow-nuxt) NAO sao tocadas.
function absolutizeUploads(value: unknown): unknown {
  if (typeof value === 'string') {
    return apiBase && value.startsWith('/uploads/') ? apiBase + value : value
  }
  if (Array.isArray(value)) {
    return value.map(absolutizeUploads)
  }
  if (value && typeof value === 'object') {
    const out: Record<string, unknown> = {}
    for (const [k, v] of Object.entries(value)) {
      out[k] = absolutizeUploads(v)
    }
    return out
  }
  return value
}

const frame = ref<HTMLIFrameElement | null>(null)
const frameReady = ref(false)
let debounceTimer: ReturnType<typeof setTimeout> | null = null

// Produtos resolvidos da fonte (B7): a previa mostra os MESMOS que o publico, ANTES
// de publicar. Cacheado por chave dos filtros (so refaz fetch quando a fonte muda).
const bioStore = useBioStore()
const resolvedSlides = ref<BioSlide[]>([])
// Chave da ULTIMA resolucao bem-sucedida (so grava APOS o await — sem isso, a
// previa so atualizava ao publicar). pendingKey descarta resposta de fonte antiga.
let resolvedKey = ''
let pendingKey = ''

function activeSource(): BioSlideSource | undefined {
  return activeData.value?.slideTop?.source
}

function whatsappNumber(): string {
  return String(activeData.value?.lightbox?.whatsappNumber || '').trim()
}

function resolveKey(): string {
  const source = activeSource()
  const type = String(source?.type || '').trim()
  if (!type || type === 'manual') {
    return ''
  }
  return JSON.stringify({
    type,
    category: source?.category || '',
    campaigns: source?.campaigns || [],
    tipo: source?.tipo || '',
    limit: source?.limit || 0,
    link: source?.link || '',
    whatsapp: whatsappNumber(),
  })
}

async function ensureResolved(): Promise<void> {
  const key = resolveKey()
  if (key === resolvedKey) {
    return
  }
  if (!key) {
    resolvedSlides.value = []
    resolvedKey = ''
    pendingKey = ''
    return
  }
  pendingKey = key
  const slides = await bioStore.resolvePreviewSlides(activeSource(), whatsappNumber())
  if (pendingKey !== key) {
    return
  }
  resolvedSlides.value = slides
  resolvedKey = key
}

async function postDraft() {
  if (!frameReady.value || !frame.value?.contentWindow || !bioFrontUrl) {
    return
  }
  await ensureResolved()
  // Clona para objeto plano (postMessage de Proxy reativo lanca DataCloneError) +
  // absolutiza /uploads/. targetOrigin restrito ao front da bio.
  const plain = absolutizeUploads(JSON.parse(JSON.stringify(activeData.value ?? {}))) as Record<
    string,
    unknown
  >
  // Quando a fonte e de produtos, injeta os slides resolvidos em slideTop.slides —
  // espelha o resolveSlideSource do publico. Sem produtos, cai nos slides manuais.
  const type = String(activeSource()?.type || '').trim()
  if (type && type !== 'manual' && resolvedSlides.value.length) {
    const current = plain.slideTop
    const slideTop = (
      current && typeof current === 'object' ? current : (plain.slideTop = {})
    ) as Record<string, unknown>
    slideTop.slides = absolutizeUploads(JSON.parse(JSON.stringify(resolvedSlides.value)))
  }
  frame.value.contentWindow.postMessage({ __bioPreview: true, data: plain }, bioFrontUrl)
}

function onMessage(event: MessageEvent) {
  if (bioFrontUrl && event.origin !== bioFrontUrl) {
    return
  }
  if (event.data && typeof event.data === 'object' && event.data.__bioPreviewReady === true) {
    frameReady.value = true
    void postDraft()
  }
}

watch(
  activeData,
  () => {
    if (debounceTimer) {
      clearTimeout(debounceTimer)
    }
    debounceTimer = setTimeout(() => {
      void postDraft()
    }, 300)
  },
  { deep: true },
)

onMounted(() => window.addEventListener('message', onMessage))
onBeforeUnmount(() => {
  window.removeEventListener('message', onMessage)
  if (debounceTimer) {
    clearTimeout(debounceTimer)
  }
})
</script>

<template>
  <div class="bio-live-preview">
    <div class="bio-live-preview__bar">
      <div class="bio-live-preview__title">
        <UIcon name="i-lucide-radio" class="bio-live-preview__dot" />
        <span>Previa</span>
      </div>
      <div class="bio-live-preview__source" role="group" aria-label="Fonte da previa">
        <UButton
          :color="source !== 'published' ? 'primary' : 'neutral'"
          :variant="source !== 'published' ? 'soft' : 'ghost'"
          size="xs"
          label="Editando"
          title="Versao em edicao (rascunho)"
          @click="setSource('draft')"
        />
        <UButton
          :color="source === 'published' ? 'primary' : 'neutral'"
          :variant="source === 'published' ? 'soft' : 'ghost'"
          size="xs"
          label="Publicado"
          title="Versao no ar"
          :disabled="!isPublished"
          @click="setSource('published')"
        />
      </div>
    </div>
    <div v-if="!previewUrl" class="bio-live-preview__empty">
      Configure NUXT_PUBLIC_BIO_FRONT_URL para habilitar a previa ao vivo.
    </div>
    <div v-else class="bio-live-preview__device">
      <iframe
        ref="frame"
        :src="previewUrl"
        class="bio-live-preview__frame"
        title="Previa ao vivo da bio"
      ></iframe>
    </div>
  </div>
</template>

<style scoped>
.bio-live-preview {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  position: sticky;
  top: 0;
}

.bio-live-preview__title {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

.bio-live-preview__source {
  display: inline-flex;
  align-items: center;
  gap: 0.15rem;
  padding: 0.12rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  background: rgb(var(--surface-2) / 0.5);
}

.bio-live-preview__bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.4rem;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-muted);
}

.bio-live-preview__dot {
  color: rgb(var(--success));
}

.bio-live-preview__empty {
  padding: 1rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-soft);
  color: var(--text-muted);
  font-size: 0.85rem;
}

.bio-live-preview__device {
  width: 100%;
  height: 78vh;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  overflow: hidden;
  background: #000;
  box-shadow: var(--shadow-card);
}

.bio-live-preview__frame {
  width: 100%;
  height: 100%;
  border: 0;
}
</style>
