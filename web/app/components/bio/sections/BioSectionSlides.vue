<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'

import BioSectionCard from '~/components/bio/BioSectionCard.vue'
import BioSlideManualList from '~/components/bio/BioSlideManualList.vue'
import BioSlideSource from '~/components/bio/BioSlideSource.vue'
import type {
  BioCarousel,
  BioData,
  BioMediaKind,
  BioSlide,
  BioSlideButton,
  BioSlideMode,
  BioSlideSource as BioSlideSourceConfig,
  BioSlideSourceType,
  BioSlideTop,
} from '~/domain/bio/types'
import { useBioStore } from '~/stores/bio'

// Secao slideTop (B7). Ordem: config do CARROSSEL primeiro, depois o conteudo.
// O conteudo tem uma FONTE (Imagens manuais OU Produtos do site) e um MODO
// (Carrossel OU Estatico). No modo Produtos o backend resolve site.products e
// injeta os slides; no modo Imagens os slides sao editados a mao (como sempre).
// Abaixo, um botao opcional (ex.: "Ver toda a colecao"). Tudo retrocompativel:
// bio sem source/mode/button = comportamento manual de carrossel atual.

const props = defineProps<{
  draft: BioData
  bioId: string
  uploadMedia: (kind: BioMediaKind, file: File) => Promise<string | null>
  isUploading: (kind: BioMediaKind) => boolean
}>()

const emit = defineEmits<{ (e: 'update:draft', value: BioData): void }>()

const store = useBioStore()

const slideTop = computed<BioSlideTop>(() => props.draft.slideTop || {})
const slides = computed<BioSlide[]>(() => props.draft.slideTop?.slides || [])
const carousel = computed<BioCarousel>(() => props.draft.slideTop?.carousel || {})
const source = computed<BioSlideSourceConfig>(() => props.draft.slideTop?.source || {})
const button = computed<BioSlideButton>(() => props.draft.slideTop?.button || {})

const sourceType = computed<BioSlideSourceType>(() => source.value.type || 'manual')
const mode = computed<BioSlideMode>(() => slideTop.value.mode || 'carousel')
const isProducts = computed(() => sourceType.value === 'site_products')

const SOURCE_ITEMS: { label: string; value: BioSlideSourceType }[] = [
  { label: 'Imagens (manuais)', value: 'manual' },
  { label: 'Produtos (da fonte)', value: 'site_products' },
]

const MODE_ITEMS: { label: string; value: BioSlideMode }[] = [
  { label: 'Carrossel', value: 'carousel' },
  { label: 'Estatico', value: 'static' },
]

function patchSlideTop(patch: Partial<BioSlideTop>) {
  emit('update:draft', {
    ...props.draft,
    slideTop: { engine: 'keen', ...(props.draft.slideTop || {}), ...patch },
  })
}

function setActive(value: boolean) {
  patchSlideTop({ active: value })
}

function setMode(value: BioSlideMode) {
  patchSlideTop({ mode: value })
}

function setSourceType(value: BioSlideSourceType) {
  // Ao escolher produtos pela primeira vez, default de link = produto no site.
  const next: BioSlideSourceConfig = { ...(props.draft.slideTop?.source || {}), type: value }
  if (value === 'site_products' && !next.link) {
    next.link = 'product'
  }
  patchSlideTop({ source: next })
}

function patchSource(patch: Partial<BioSlideSourceConfig>) {
  patchSlideTop({
    source: { ...(props.draft.slideTop?.source || {}), type: 'site_products', ...patch },
  })
}

function patchButton(patch: Partial<BioSlideButton>) {
  patchSlideTop({ button: { ...(props.draft.slideTop?.button || {}), ...patch } })
}

function setCarousel<K extends keyof BioCarousel>(key: K, value: BioCarousel[K]) {
  patchSlideTop({ carousel: { ...(props.draft.slideTop?.carousel || {}), [key]: value } })
}

function setSlides(next: BioSlide[]) {
  patchSlideTop({ slides: next })
}

function toNumber(value: string): number {
  const parsed = Number(String(value || '').trim())
  return Number.isFinite(parsed) ? parsed : 0
}

// Carrega as fontes ao montar (popula o seletor). Os facets so quando a fonte
// de produtos esta selecionada — evita pedir o que a tela nao usa.
onMounted(() => {
  void store.loadSources()
  if (isProducts.value) {
    void store.loadSiteProductFacets()
  }
})

watch(isProducts, (active) => {
  if (active && !store.facets.categories.length && !store.facetsPending) {
    void store.loadSiteProductFacets()
  }
})
</script>

<template>
  <BioSectionCard
    title="Slides do topo"
    description="Destaque no topo da bio. Configure o carrossel, escolha a fonte (imagens manuais ou produtos) e o modo de exibicao."
  >
    <div class="bio-field bio-field--switch">
      <label class="bio-field__label">Slide do topo ativo</label>
      <USwitch
        :model-value="slideTop.active ?? false"
        @update:model-value="setActive(Boolean($event))"
      />
    </div>

    <div class="bio-subsection">
      <h3 class="bio-subsection__title">Carrossel</h3>
      <div class="bio-section-grid">
        <div class="bio-field bio-field--switch">
          <label class="bio-field__label">Loop</label>
          <USwitch
            :model-value="carousel.loop ?? false"
            @update:model-value="setCarousel('loop', Boolean($event))"
          />
        </div>
        <div class="bio-field bio-field--switch">
          <label class="bio-field__label">Autoplay</label>
          <USwitch
            :model-value="carousel.autoplay ?? false"
            @update:model-value="setCarousel('autoplay', Boolean($event))"
          />
        </div>
        <div class="bio-field bio-field--switch">
          <label class="bio-field__label">Pausar no mouse</label>
          <USwitch
            :model-value="carousel.pauseOnHover ?? false"
            @update:model-value="setCarousel('pauseOnHover', Boolean($event))"
          />
        </div>
        <div class="bio-field bio-field--switch">
          <label class="bio-field__label">Setas de navegacao</label>
          <USwitch
            :model-value="carousel.nav ?? false"
            @update:model-value="setCarousel('nav', Boolean($event))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Intervalo autoplay (ms)</label>
          <UInput
            type="number"
            :model-value="carousel.autoplayMs ?? 0"
            @update:model-value="setCarousel('autoplayMs', toNumber(String($event ?? '')))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Slides por vista</label>
          <UInput
            type="number"
            :model-value="carousel.perView ?? 0"
            @update:model-value="setCarousel('perView', toNumber(String($event ?? '')))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Espacamento (px)</label>
          <UInput
            type="number"
            :model-value="carousel.spacing ?? 0"
            @update:model-value="setCarousel('spacing', toNumber(String($event ?? '')))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Limite de slides</label>
          <UInput
            type="number"
            :model-value="carousel.limit ?? 0"
            @update:model-value="setCarousel('limit', toNumber(String($event ?? '')))"
          />
        </div>
      </div>
      <p class="bio-subsection__hint">
        Os breakpoints responsivos do carrossel sao editados pelo defaults global ou pela API.
      </p>
    </div>

    <div class="bio-subsection">
      <h3 class="bio-subsection__title">Conteudo</h3>
      <div class="bio-section-grid">
        <div class="bio-field">
          <label class="bio-field__label">Fonte</label>
          <USelect
            :model-value="sourceType"
            :items="SOURCE_ITEMS"
            value-key="value"
            @update:model-value="setSourceType($event as BioSlideSourceType)"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Modo</label>
          <USelect
            :model-value="mode"
            :items="MODE_ITEMS"
            value-key="value"
            @update:model-value="setMode($event as BioSlideMode)"
          />
        </div>
      </div>

      <BioSlideSource
        v-if="isProducts"
        :source="source"
        :facets="store.facets"
        :facets-pending="store.facetsPending"
        :facets-error="store.facetsError"
        @update:source="patchSource"
      />

      <BioSlideManualList
        v-else
        :slides="slides"
        :upload-media="uploadMedia"
        :is-uploading="isUploading"
        @update:slides="setSlides"
      />
    </div>

    <div class="bio-subsection">
      <h3 class="bio-subsection__title">Botao abaixo do carrossel</h3>
      <p class="bio-subsection__hint">
        Opcional. Ex.: "Ver toda a colecao" levando a categoria no site.
      </p>
      <div class="bio-section-grid">
        <div class="bio-field">
          <label class="bio-field__label">Texto do botao</label>
          <UInput
            :model-value="button.text || ''"
            placeholder="Ver toda a colecao"
            @update:model-value="patchButton({ text: String($event ?? '') })"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Link do botao</label>
          <UInput
            :model-value="button.href || ''"
            placeholder="https://..."
            @update:model-value="patchButton({ href: String($event ?? '') })"
          />
        </div>
      </div>
    </div>
  </BioSectionCard>
</template>

<style scoped>
.bio-section-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 0.6rem 0.85rem;
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
  margin: 0;
  font-size: 0.72rem;
  color: var(--text-muted);
}
</style>
