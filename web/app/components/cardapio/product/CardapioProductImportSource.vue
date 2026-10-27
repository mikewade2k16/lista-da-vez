<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useCardapioStore } from '~/stores/cardapio'
import type { RestaurantListItem } from '~/domain/cardapio/types'
import type { ProductTransferDocument } from '~/domain/cardapio/product-transfer'
import { normalizeProductSearch } from '~/domain/cardapio/product-search'
import { getApiErrorMessage } from '~/utils/api-client'

const props = defineProps<{ restaurantId: string; disabled: boolean }>()
const emit = defineEmits<{
  loaded: [document: ProductTransferDocument, name: string]
  reset: []
}>()
const store = useCardapioStore()
const sources = ref<RestaurantListItem[]>([])
const sourceId = ref<string>()
const document = ref<ProductTransferDocument | null>(null)
const selectedSlugs = ref<string[]>([])
const query = ref('')
const loading = ref(false)
const error = ref('')
let requestVersion = 0

const options = computed(() =>
  sources.value.map((source) => ({
    label: source.accountName ? `${source.name} · ${source.accountName}` : source.name,
    value: source.id,
  })),
)
const visibleProducts = computed(
  () =>
    document.value?.products.filter((product) =>
      normalizeProductSearch(product.name).includes(normalizeProductSearch(query.value)),
    ) ?? [],
)
const allSelected = computed(
  () =>
    visibleProducts.value.length > 0 &&
    visibleProducts.value.every((product) => selectedSlugs.value.includes(product.slug)),
)

async function loadSources() {
  loading.value = true
  error.value = ''
  try {
    sources.value = (await store.loadProductImportSources()).filter(
      (source) => source.id !== props.restaurantId,
    )
  } catch (caught) {
    error.value = getApiErrorMessage(caught, 'Não foi possível carregar os cardápios.')
  } finally {
    loading.value = false
  }
}
onMounted(loadSources)

async function loadSource(id: string) {
  sourceId.value = id
  const version = ++requestVersion
  document.value = null
  selectedSlugs.value = []
  query.value = ''
  error.value = ''
  emit('reset')
  const source = sources.value.find((item) => item.id === id)
  if (!source) return
  loading.value = true
  try {
    const result = await store.exportProducts(source.id, source.accountId)
    if (version !== requestVersion) return
    document.value = result
    selectedSlugs.value = result.products.map((product) => product.slug)
  } catch (caught) {
    if (version === requestVersion)
      error.value = getApiErrorMessage(caught, 'Não foi possível carregar os produtos da origem.')
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

function selectProduct(slug: string, enabled: boolean | 'indeterminate') {
  const next = new Set(selectedSlugs.value)
  if (enabled === true) next.add(slug)
  else next.delete(slug)
  selectedSlugs.value = [...next]
  emit('reset')
}
function selectVisible(enabled: boolean | 'indeterminate') {
  const next = new Set(selectedSlugs.value)
  for (const product of visibleProducts.value) {
    if (enabled === true) next.add(product.slug)
    else next.delete(product.slug)
  }
  selectedSlugs.value = [...next]
  emit('reset')
}
function review() {
  const source = sources.value.find((item) => item.id === sourceId.value)
  if (!source || !document.value) return
  const selected = new Set(selectedSlugs.value)
  emit(
    'loaded',
    {
      ...document.value,
      products: document.value.products.filter((item) => selected.has(item.slug)),
    },
    source.name,
  )
}
</script>

<template>
  <div class="cardapio-import-source">
    <USelectMenu
      :model-value="sourceId"
      :items="options"
      value-key="value"
      placeholder="Escolher cardápio de origem"
      aria-label="Cardápio de origem"
      :loading="loading"
      :disabled="disabled || loading"
      @update:model-value="loadSource"
    />
    <p class="cardapio-import-source__hint">
      Copia produtos, fotos, variações e adicionais. O cardápio de origem permanece como está.
    </p>
    <UAlert
      v-if="error"
      color="error"
      :description="error"
      title="Não foi possível carregar a origem"
    >
      <template #actions>
        <UButton
          label="Tentar novamente"
          variant="ghost"
          @click="sourceId ? loadSource(sourceId) : loadSources()"
        />
      </template>
    </UAlert>
    <p v-else-if="!loading && !sources.length" class="cardapio-import-source__hint">
      Nenhum outro cardápio disponível para sua conta.
    </p>
    <template v-if="document">
      <UInput
        v-model="query"
        icon="i-lucide-search"
        placeholder="Buscar produtos na origem..."
        aria-label="Buscar produtos na origem"
      />
      <UCheckbox
        :model-value="allSelected"
        label="Selecionar todos os resultados"
        :disabled="disabled"
        @update:model-value="selectVisible"
      />
      <div class="cardapio-import-source__products">
        <UCheckbox
          v-for="product in visibleProducts"
          :key="product.slug"
          :label="product.name"
          :model-value="selectedSlugs.includes(product.slug)"
          :disabled="disabled"
          @update:model-value="selectProduct(product.slug, $event)"
        />
        <p v-if="!visibleProducts.length" class="cardapio-import-source__hint">
          Nenhum produto encontrado.
        </p>
      </div>
      <UButton
        :label="`Revisar ${selectedSlugs.length} produto(s)`"
        icon="i-lucide-arrow-right"
        :disabled="disabled || !selectedSlugs.length"
        @click="review"
      />
    </template>
  </div>
</template>

<style scoped>
.cardapio-import-source {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.cardapio-import-source__hint {
  color: var(--text-muted);
  font-size: 0.82rem;
}
.cardapio-import-source__products {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  max-height: 15rem;
  overflow-y: auto;
  padding: 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
}
</style>
