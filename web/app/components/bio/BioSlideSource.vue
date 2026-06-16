<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue'

import { useBioStore } from '~/stores/bio'
import type { BioSlideLinkMode, BioSlideSource, BioSlideSourceFacets } from '~/domain/bio/types'

// Config do modo "Produtos do site" (B7): selects de categoria / tipo, chips de
// campanha (multi), quantidade (5/10/todos) e o destino do clique do slide
// (produto / WhatsApp / sem link). So edita o objeto `source`; o backend
// resolve os produtos de site.products no endpoint publico. Layout compacto:
// filtros num grid responsivo, campanhas em chips toggle.

const props = defineProps<{
  source: BioSlideSource
  facets: BioSlideSourceFacets
  facetsPending: boolean
  facetsError: string
}>()

const emit = defineEmits<{ (e: 'update:source', value: Partial<BioSlideSource>): void }>()

// USelect (Reka UI) proibe SelectItem com value vazio; usamos o sentinela 'all'
// e convertemos para '' (sem filtro) ao gravar.
const ALL = 'all'

const LIMIT_ITEMS = [
  { label: '5 produtos', value: '5' },
  { label: '10 produtos', value: '10' },
  { label: 'Todos', value: '0' },
]

const LINK_ITEMS: { label: string; value: BioSlideLinkMode }[] = [
  { label: 'Produto no site', value: 'product' },
  { label: 'WhatsApp da bio', value: 'whatsapp' },
  { label: 'Sem link', value: 'none' },
]

const categoryItems = computed(() => [
  { label: 'Todas as categorias', value: ALL },
  ...props.facets.categories.map((category) => ({ label: category, value: category })),
])

const tipoItems = computed(() => [
  { label: 'Todos os tipos', value: ALL },
  ...props.facets.tipos.map((tipo) => ({ label: tipo, value: tipo })),
])

const categoryValue = computed(() => props.source.category || ALL)
const tipoValue = computed(() => props.source.tipo || ALL)
const limitValue = computed(() => String(props.source.limit ?? 5))
const linkValue = computed<BioSlideLinkMode>(() => props.source.link || 'product')
const selectedCampaigns = computed(() => props.source.campaigns || [])

function setCategory(value: string) {
  emit('update:source', { category: value === ALL ? undefined : value })
}

function setTipo(value: string) {
  emit('update:source', { tipo: value === ALL ? undefined : value })
}

function setLimit(value: string) {
  emit('update:source', { limit: Number(value) || 0 })
}

function setLink(value: BioSlideLinkMode) {
  emit('update:source', { link: value })
}

function toggleCampaign(campaign: string) {
  const current = selectedCampaigns.value
  const next = current.includes(campaign)
    ? current.filter((item) => item !== campaign)
    : [...current, campaign]
  emit('update:source', { campaigns: next.length ? next : undefined })
}

function isCampaignActive(campaign: string): boolean {
  return selectedCampaigns.value.includes(campaign)
}

// Quantos produtos os filtros atuais encontram (categoria E campanha se combinam
// = AND). Mostra o resultado no editor para deixar EXPLICITO quando o combo da
// vazio (ex.: Esmeralda + Black Friday = 0) — sem isso, o preview cai silencioso
// nos slides manuais e parece que a fonte "nao funciona". Le do nosso banco
// (site.products), independente da disponibilidade da origem externa.
const store = useBioStore()
const COUNT_CAP = 50
const matchCount = ref<number | null>(null)
const matchPending = ref(false)
let probeTimer: ReturnType<typeof setTimeout> | null = null

const filterKey = computed(() =>
  JSON.stringify({
    category: props.source.category || '',
    campaigns: props.source.campaigns || [],
    tipo: props.source.tipo || '',
  }),
)

async function probeCount() {
  matchPending.value = true
  try {
    const slides = await store.resolvePreviewSlides({ ...props.source, limit: COUNT_CAP }, '')
    matchCount.value = slides.length
  } catch {
    matchCount.value = null
  } finally {
    matchPending.value = false
  }
}

watch(filterKey, () => {
  if (probeTimer) {
    clearTimeout(probeTimer)
  }
  probeTimer = setTimeout(probeCount, 350)
})

onMounted(probeCount)
</script>

<template>
  <div class="bio-slide-source">
    <p v-if="facetsError" class="bio-slide-source__error">{{ facetsError }}</p>
    <p v-else-if="facetsPending" class="bio-slide-source__hint">
      Carregando filtros de produtos...
    </p>

    <p v-if="matchPending" class="bio-slide-source__hint">Verificando produtos...</p>
    <p v-else-if="matchCount === 0" class="bio-slide-source__warn">
      Nenhum produto com esses filtros. Categoria e campanha se COMBINAM (E) — deixe so um, ou
      escolha um par que exista.
    </p>
    <p v-else-if="matchCount !== null" class="bio-slide-source__count">
      {{ matchCount >= COUNT_CAP ? COUNT_CAP + '+' : matchCount }} produto(s) com esses filtros.
    </p>

    <div class="bio-section-grid">
      <div class="bio-field">
        <label class="bio-field__label">Categoria</label>
        <USelect
          :model-value="categoryValue"
          :items="categoryItems"
          value-key="value"
          @update:model-value="setCategory(String($event ?? 'all'))"
        />
      </div>
      <div class="bio-field">
        <label class="bio-field__label">Tipo</label>
        <USelect
          :model-value="tipoValue"
          :items="tipoItems"
          value-key="value"
          @update:model-value="setTipo(String($event ?? 'all'))"
        />
      </div>
      <div class="bio-field">
        <label class="bio-field__label">Quantidade</label>
        <USelect
          :model-value="limitValue"
          :items="LIMIT_ITEMS"
          value-key="value"
          @update:model-value="setLimit(String($event ?? '5'))"
        />
      </div>
      <div class="bio-field">
        <label class="bio-field__label">Link do slide</label>
        <USelect
          :model-value="linkValue"
          :items="LINK_ITEMS"
          value-key="value"
          @update:model-value="setLink($event as BioSlideLinkMode)"
        />
      </div>
    </div>

    <div class="bio-field">
      <label class="bio-field__label">Campanhas</label>
      <p v-if="!facets.campaigns.length" class="bio-slide-source__hint">
        Nenhuma campanha cadastrada nos produtos desta conta.
      </p>
      <div v-else class="bio-slide-source__chips">
        <button
          v-for="campaign in facets.campaigns"
          :key="campaign"
          type="button"
          class="bio-slide-source__chip"
          :class="{ 'bio-slide-source__chip--active': isCampaignActive(campaign) }"
          :aria-pressed="isCampaignActive(campaign)"
          @click="toggleCampaign(campaign)"
        >
          {{ campaign }}
        </button>
      </div>
      <p class="bio-slide-source__hint">
        Sem campanha marcada = considera todas. Marque para filtrar.
      </p>
    </div>
  </div>
</template>

<style scoped>
.bio-slide-source {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.bio-section-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 0.6rem 0.85rem;
}

.bio-field {
  display: grid;
  gap: 0.3rem;
}

.bio-field__label {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-muted);
}

.bio-slide-source__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.bio-slide-source__chip {
  padding: 0.3rem 0.65rem;
  border: 1px solid var(--line-soft);
  border-radius: 999px;
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-muted);
  font-size: 0.74rem;
  font-weight: 600;
  cursor: pointer;
}

.bio-slide-source__chip:hover {
  border-color: rgb(var(--primary) / 0.4);
  color: rgb(var(--primary));
}

.bio-slide-source__chip--active {
  border-color: rgb(var(--primary) / 0.6);
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
}

.bio-slide-source__hint {
  margin: 0;
  font-size: 0.72rem;
  color: var(--text-muted);
}

.bio-slide-source__error {
  margin: 0;
  font-size: 0.74rem;
  color: rgb(var(--danger));
}

.bio-slide-source__warn {
  margin: 0;
  font-size: 0.74rem;
  font-weight: 600;
  color: rgb(var(--warning, var(--danger)));
}

.bio-slide-source__count {
  margin: 0;
  font-size: 0.74rem;
  font-weight: 600;
  color: rgb(var(--success));
}
</style>
