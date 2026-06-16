<script setup lang="ts">
import OmniDataTable from '../../../layers/tasks/components/omni/table/OmniDataTable.vue'
import type { ErpUnmatchedItem } from '~/types/products'
import type { OmniTableColumn } from '~/types/omni/collection'

// Aba "Produtos do ERP (fora do site)": lista paginada (server-side) dos itens do
// ERP que ainda nao existem no site. Cada linha tem "Puxar pro site" (cria o
// produto a partir do SKU). Extraido do workspace para manter o arquivo enxuto.
const props = defineProps<{
  items: ErpUnmatchedItem[]
  total: number
  page: number
  perPage: number
  query: string
  loading?: boolean
  creatingSku?: string | null
}>()

const emit = defineEmits<{
  'update:page': [value: number]
  search: [value: string]
  pull: [sku: string]
}>()

const searchInput = ref(props.query)
let searchTimer: ReturnType<typeof setTimeout> | null = null

// Busca por sku/name com debounce — evita request a cada tecla.
watch(searchInput, (value) => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => emit('search', String(value ?? '').trim()), 380)
})

// Mantem o input em sincronia se o estado externo mudar (ex.: reset).
watch(
  () => props.query,
  (value) => {
    if (value !== searchInput.value) searchInput.value = value
  },
)

onBeforeUnmount(() => {
  if (searchTimer) clearTimeout(searchTimer)
})

const columns = computed<OmniTableColumn[]>(() => [
  { key: 'sku', label: 'SKU', type: 'text', minWidth: 110 },
  { key: 'name', label: 'Nome (ERP)', type: 'text', minWidth: 260 },
  { key: 'description', label: 'Descricao (ERP)', type: 'text', minWidth: 280 },
  { key: 'actions', label: 'Acoes', type: 'custom', align: 'right', minWidth: 140 },
])

const rows = computed(() => props.items as unknown as Array<Record<string, unknown>>)

const pageModel = computed({
  get: () => props.page,
  set: (value: number) => emit('update:page', value),
})

const rangeLabel = computed(() => {
  if (props.total === 0) return ''
  const start = (props.page - 1) * props.perPage + 1
  const end = Math.min(props.page * props.perPage, props.total)
  return `${start}-${end} de ${props.total}`
})

function rowSku(row: Record<string, unknown>) {
  return String(row.sku ?? '').trim()
}
</script>

<template>
  <section class="site-erp-panel flex h-full min-h-0 flex-col gap-3">
    <div class="site-erp-panel__bar">
      <UInput
        v-model="searchInput"
        icon="i-lucide-search"
        placeholder="Buscar por SKU ou nome..."
        size="sm"
        class="site-erp-panel__search"
      />
      <UBadge color="neutral" variant="soft" :title="`${props.total} itens do ERP fora do site`">
        {{ props.total }} itens do ERP
      </UBadge>
    </div>

    <div class="site-erp-panel__scroll flex-1 min-h-0 overflow-y-auto">
      <OmniDataTable
        :rows="rows"
        :columns="columns"
        row-key="sku"
        :loading="props.loading"
        :selectable="false"
        empty-text="Nenhum item do ERP fora do site."
      >
        <template #cell-actions="{ row }">
          <div class="flex items-center justify-end">
            <UButton
              icon="i-lucide-download"
              label="Puxar pro site"
              color="primary"
              variant="soft"
              size="sm"
              :loading="props.creatingSku === rowSku(row)"
              :disabled="Boolean(props.creatingSku) && props.creatingSku !== rowSku(row)"
              @click="emit('pull', rowSku(row))"
            />
          </div>
        </template>
      </OmniDataTable>
    </div>

    <div class="site-erp-panel__footer">
      <span v-if="rangeLabel" class="site-erp-panel__range">{{ rangeLabel }}</span>
      <UPagination
        v-model:page="pageModel"
        :total="props.total"
        :items-per-page="props.perPage"
        :sibling-count="1"
        show-edges
        size="sm"
      />
    </div>
  </section>
</template>

<style scoped>
.site-erp-panel__bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem;
}

.site-erp-panel__search {
  min-width: 16rem;
}

.site-erp-panel__footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 0.75rem;
  padding: 0.5rem 0.25rem;
}

.site-erp-panel__range {
  font-size: 0.75rem;
  color: var(--text-muted);
}
</style>
