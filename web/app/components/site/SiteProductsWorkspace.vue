<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { storeToRefs } from 'pinia'

import AppDetailDialog from '~/components/ui/AppDetailDialog.vue'
import AppEntityGrid from '~/components/ui/AppEntityGrid.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import { useSiteStore, type SiteProduct } from '~/stores/site'
import { useUiStore } from '~/stores/ui'

const siteStore = useSiteStore()
const ui = useUiStore()

const { scopedProducts, tenantOptions, currentTenantId, isPlatformAdmin } = storeToRefs(siteStore)

const searchValue = ref('')
const tenantFilter = ref('')
const sourceFilter = ref('all')
const visibilityFilter = ref('all')
const showDeleted = ref(false)
const detailOpen = ref(false)
const selectedProductId = ref('')

const createDraft = reactive({
  name: '',
  tenantId: '',
})

const columns = [
  { id: 'tenantName', label: 'Cliente', width: 'minmax(170px, 1.15fr)' },
  { id: 'name', label: 'Produto', width: 'minmax(220px, 1.5fr)', locked: true },
  { id: 'code', label: 'Codigo', width: 'minmax(150px, 1fr)' },
  { id: 'categories', label: 'Categorias', width: 'minmax(190px, 1.2fr)' },
  { id: 'campaigns', label: 'Campanhas', width: 'minmax(190px, 1.2fr)' },
  { id: 'source', label: 'Origem', width: 'minmax(120px, 0.8fr)', align: 'center' },
  { id: 'webhookStatus', label: 'Webhook', width: 'minmax(150px, 0.9fr)', align: 'center' },
  { id: 'visible', label: 'Site', width: 'minmax(92px, 0.6fr)', align: 'center' },
  { id: 'available', label: 'Estoque', width: 'minmax(104px, 0.7fr)', align: 'center' },
  { id: 'price', label: 'Preco', width: 'minmax(132px, 0.8fr)', align: 'end' },
  { id: 'updatedAt', label: 'Atualizado', width: 'minmax(152px, 0.9fr)' },
  { id: 'actions', label: 'Acoes', width: 'minmax(112px, 0.8fr)', align: 'end', locked: true },
]

const showTenantFilter = computed(() => isPlatformAdmin.value && tenantOptions.value.length > 1)
const effectiveTenantId = computed(() => {
  if (showTenantFilter.value) {
    return String(tenantFilter.value || '').trim()
  }

  return String(currentTenantId.value || '').trim()
})

const sourceOptions = [
  { label: 'Todas as origens', value: 'all' },
  { label: 'Webhook', value: 'webhook' },
  { label: 'Manual', value: 'manual' },
]

const visibilityOptions = [
  { label: 'Todos os status', value: 'all' },
  { label: 'Publicados', value: 'visible' },
  { label: 'Ocultos', value: 'hidden' },
]

const tenantFilterOptions = computed(() => [
  { label: 'Todos os clientes', value: '' },
  ...tenantOptions.value.map((tenant) => ({ label: tenant.label, value: tenant.id })),
])

const filteredRows = computed(() => {
  const search = String(searchValue.value || '')
    .trim()
    .toLowerCase()

  return scopedProducts.value
    .filter((product) => (showDeleted.value ? true : !product.isDeleted))
    .filter((product) =>
      !effectiveTenantId.value ? true : product.tenantId === effectiveTenantId.value,
    )
    .filter((product) =>
      sourceFilter.value === 'all' ? true : product.source === sourceFilter.value,
    )
    .filter((product) => {
      if (visibilityFilter.value === 'visible') {
        return product.visible
      }

      if (visibilityFilter.value === 'hidden') {
        return !product.visible
      }

      return true
    })
    .filter((product) => {
      if (!search) {
        return true
      }

      return [
        product.tenantName,
        product.name,
        product.code,
        product.categories.join(' '),
        product.campaigns.join(' '),
        product.description,
      ]
        .join(' ')
        .toLowerCase()
        .includes(search)
    })
})

const selectedProduct = computed(
  () => filteredRows.value.find((product) => product.id === selectedProductId.value) || null,
)

const metrics = computed(() => {
  const base = filteredRows.value
  return {
    total: base.length,
    visible: base.filter((product) => product.visible && !product.isDeleted).length,
    webhook: base.filter((product) => product.source === 'webhook' && !product.isDeleted).length,
    issues: base.filter((product) => product.webhookStatus === 'error').length,
  }
})

const detailSections = computed(() => {
  if (!selectedProduct.value) {
    return []
  }

  const product = selectedProduct.value
  return [
    {
      id: 'identity',
      title: 'Identificacao',
      fields: [
        { label: 'Cliente', value: product.tenantName },
        { label: 'Produto', value: product.name },
        { label: 'Codigo', value: product.code },
        { label: 'Tipo', value: product.type },
      ],
    },
    {
      id: 'catalog',
      title: 'Catalogo do site',
      fields: [
        { label: 'Categorias', value: product.categories },
        { label: 'Campanhas', value: product.campaigns },
        { label: 'Visivel no site', value: product.visible ? 'Sim' : 'Nao' },
        { label: 'Disponivel', value: product.available ? 'Sim' : 'Nao' },
      ],
    },
    {
      id: 'webhook',
      title: 'Webhook',
      description: 'Estado local mockado para validar a futura ingestao via webhook.',
      fields: [
        { label: 'Origem', value: product.source },
        { label: 'Status', value: product.webhookStatus },
        { label: 'Endpoint', value: product.webhookEndpoint },
        { label: 'Ultimo sync', value: formatDateTime(product.lastWebhookSync) },
      ],
    },
  ]
})

function formatCurrency(value: number) {
  return new Intl.NumberFormat('pt-BR', {
    style: 'currency',
    currency: 'BRL',
  }).format(Number(value || 0))
}

function formatDateTime(value: string) {
  const date = new Date(String(value || ''))
  if (Number.isNaN(date.getTime())) {
    return '-'
  }

  return date.toLocaleString('pt-BR')
}

function updateProduct(productId: string, patch: Partial<SiteProduct>) {
  const result = siteStore.updateProduct(productId, patch)
  if (!result.ok) {
    ui.error(result.message || 'Nao foi possivel atualizar o produto.')
  }
}

function handleCreateProduct() {
  const tenantId = effectiveTenantId.value || currentTenantId.value || createDraft.tenantId
  const result = siteStore.createProduct({
    name: createDraft.name || 'Novo produto',
    tenantId,
    source: 'manual',
    webhookStatus: 'pending',
    visible: false,
    available: true,
  })

  if (!result.ok || !result.product) {
    ui.error(result.message || 'Nao foi possivel criar o produto.')
    return
  }

  createDraft.name = ''
  selectedProductId.value = result.product.id
  detailOpen.value = true
  ui.success('Produto criado no workspace local do site.')
}

function handleArchive(productId: string) {
  const result = siteStore.archiveProduct(productId)
  if (!result.ok) {
    ui.error(result.message || 'Nao foi possivel arquivar o produto.')
    return
  }

  ui.success('Produto arquivado no catalogo local.')
}

function handleRestore(productId: string) {
  const result = siteStore.restoreProduct(productId)
  if (!result.ok) {
    ui.error(result.message || 'Nao foi possivel reativar o produto.')
    return
  }

  ui.success('Produto reativado no catalogo local.')
}

function openDetails(productId: string) {
  selectedProductId.value = productId
  detailOpen.value = true
}
</script>

<template>
  <section class="admin-panel site-workspace" data-testid="site-products-workspace">
    <AdminPageHeader
      eyebrow="Site"
      title="Produtos"
      description="Catalogo do site com edicao inline front-first, preparado para futura carga via webhook."
    />

    <section class="site-workspace__hero">
      <article class="site-workspace__metric-card">
        <span>Total no recorte</span>
        <strong>{{ metrics.total }}</strong>
      </article>
      <article class="site-workspace__metric-card is-positive">
        <span>Publicados</span>
        <strong>{{ metrics.visible }}</strong>
      </article>
      <article class="site-workspace__metric-card">
        <span>Via webhook</span>
        <strong>{{ metrics.webhook }}</strong>
      </article>
      <article
        class="site-workspace__metric-card"
        :class="metrics.issues ? 'is-warning' : 'is-muted'"
      >
        <span>Alertas</span>
        <strong>{{ metrics.issues }}</strong>
      </article>
    </section>

    <section class="site-workspace__create-card">
      <div>
        <h2>Novo produto</h2>
        <p>Cria um rascunho local para validar a UX antes da integracao real com o backend/site.</p>
      </div>

      <div class="site-workspace__create-form">
        <label class="site-workspace__field">
          <span>Nome</span>
          <input v-model="createDraft.name" type="text" placeholder="Ex.: Poltrona Vesper" />
        </label>

        <AppSelectField
          v-if="showTenantFilter"
          v-model="createDraft.tenantId"
          :options="tenantFilterOptions.slice(1)"
          label="Cliente do rascunho"
          placeholder="Selecionar cliente"
        />

        <button class="site-workspace__primary-btn" type="button" @click="handleCreateProduct">
          Novo produto
        </button>
      </div>
    </section>

    <AppEntityGrid
      testid="site-products-grid"
      storage-key="site-products-grid"
      :columns="columns"
      :rows="filteredRows"
      :search-value="searchValue"
      :loading="!siteStore.ready"
      search-placeholder="Buscar por nome, codigo, categoria ou campanha..."
      empty-title="Nenhum produto nesse recorte"
      empty-text="Ajuste os filtros ou crie um rascunho novo para preencher a grade do site."
      @update:search-value="searchValue = $event"
    >
      <template #toolbar-filters>
        <AppSelectField
          v-if="showTenantFilter"
          v-model="tenantFilter"
          :options="tenantFilterOptions"
          label="Cliente"
          placeholder="Todos os clientes"
          compact
        />

        <AppSelectField
          v-model="sourceFilter"
          :options="sourceOptions"
          label="Origem"
          placeholder="Todas as origens"
          compact
        />

        <AppSelectField
          v-model="visibilityFilter"
          :options="visibilityOptions"
          label="Site"
          placeholder="Todos os status"
          compact
        />

        <label class="site-workspace__toolbar-switch">
          <span>Mostrar arquivados</span>
          <AppToggleSwitch v-model="showDeleted" compact />
        </label>
      </template>

      <template #cell-tenantName="{ row }">
        <span class="site-workspace__tenant-chip">{{ row.tenantName }}</span>
      </template>

      <template #cell-name="{ row }">
        <div class="site-workspace__stack-field">
          <input
            class="site-workspace__input"
            :value="row.name"
            type="text"
            @input="updateProduct(row.id, { name: ($event.target as HTMLInputElement).value })"
          />
          <small>{{ row.type }}</small>
        </div>
      </template>

      <template #cell-code="{ row }">
        <input
          class="site-workspace__input"
          :value="row.code"
          type="text"
          @input="updateProduct(row.id, { code: ($event.target as HTMLInputElement).value })"
        />
      </template>

      <template #cell-categories="{ row }">
        <input
          class="site-workspace__input"
          :value="row.categories.join(', ')"
          type="text"
          @input="
            updateProduct(row.id, {
              categories: ($event.target as HTMLInputElement).value.split(','),
            })
          "
        />
      </template>

      <template #cell-campaigns="{ row }">
        <input
          class="site-workspace__input"
          :value="row.campaigns.join(', ')"
          type="text"
          @input="
            updateProduct(row.id, {
              campaigns: ($event.target as HTMLInputElement).value.split(','),
            })
          "
        />
      </template>

      <template #cell-source="{ row }">
        <span class="site-workspace__status-pill" :class="`is-${row.source}`">
          {{ row.source === 'webhook' ? 'Webhook' : 'Manual' }}
        </span>
      </template>

      <template #cell-webhookStatus="{ row }">
        <div class="site-workspace__status-stack">
          <span class="site-workspace__status-pill" :class="`is-${row.webhookStatus}`">
            {{ row.webhookStatus }}
          </span>
          <small>{{ formatDateTime(row.lastWebhookSync) }}</small>
        </div>
      </template>

      <template #cell-visible="{ row }">
        <AppToggleSwitch
          :model-value="row.visible"
          compact
          @update:model-value="updateProduct(row.id, { visible: $event })"
        />
      </template>

      <template #cell-available="{ row }">
        <AppToggleSwitch
          :model-value="row.available"
          compact
          @update:model-value="updateProduct(row.id, { available: $event })"
        />
      </template>

      <template #cell-price="{ row }">
        <div class="site-workspace__stack-field is-end">
          <input
            class="site-workspace__input site-workspace__input--sm"
            :value="row.price"
            inputmode="decimal"
            type="text"
            @input="updateProduct(row.id, { price: ($event.target as HTMLInputElement).value })"
          />
          <small>{{ formatCurrency(row.price) }}</small>
        </div>
      </template>

      <template #cell-updatedAt="{ row }">
        <div class="site-workspace__status-stack">
          <span>{{ formatDateTime(row.updatedAt) }}</span>
          <small v-if="row.isDeleted">Arquivado</small>
        </div>
      </template>

      <template #cell-actions="{ row }">
        <div class="site-workspace__actions">
          <button class="site-workspace__icon-btn" type="button" @click="openDetails(row.id)">
            Info
          </button>

          <button
            v-if="!row.isDeleted"
            class="site-workspace__icon-btn is-danger"
            type="button"
            @click="handleArchive(row.id)"
          >
            Arquivar
          </button>

          <button
            v-else
            class="site-workspace__icon-btn is-success"
            type="button"
            @click="handleRestore(row.id)"
          >
            Reativar
          </button>
        </div>
      </template>
    </AppEntityGrid>

    <AppDetailDialog
      v-model="detailOpen"
      :title="selectedProduct?.name || 'Detalhes do produto'"
      :subtitle="selectedProduct?.description || 'Sem descricao cadastrada.'"
      :sections="detailSections"
    >
      <section v-if="selectedProduct" class="site-workspace__dialog-extra">
        <label class="site-workspace__field">
          <span>Descricao do produto</span>
          <textarea
            class="site-workspace__textarea"
            :value="selectedProduct.description"
            rows="6"
            @input="
              updateProduct(selectedProduct.id, {
                description: ($event.target as HTMLTextAreaElement).value,
              })
            "
          ></textarea>
        </label>
      </section>
    </AppDetailDialog>
  </section>
</template>

<style scoped>
.site-workspace {
  display: grid;
  gap: 1rem;
}

.site-workspace__hero {
  display: grid;
  gap: 0.85rem;
  grid-template-columns: repeat(auto-fit, minmax(11rem, 1fr));
}

.site-workspace__metric-card,
.site-workspace__create-card {
  border: 1px solid var(--line-soft);
  border-radius: 1rem;
  background: rgb(var(--surface) / 0.92);
  box-shadow: var(--shadow-card);
}

.site-workspace__metric-card {
  display: grid;
  gap: 0.4rem;
  padding: 1rem;
}

.site-workspace__metric-card span {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.site-workspace__metric-card strong {
  font-size: 1.6rem;
  color: rgb(var(--text));
}

.site-workspace__metric-card.is-positive strong {
  color: rgb(var(--success));
}

.site-workspace__metric-card.is-warning strong {
  color: rgb(var(--warning));
}

.site-workspace__metric-card.is-muted strong {
  color: rgb(var(--muted));
}

.site-workspace__create-card {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
  padding: 1rem;
}

.site-workspace__create-card h2 {
  margin: 0;
  font-size: 1rem;
}

.site-workspace__create-card p {
  margin: 0.3rem 0 0;
  max-width: 42rem;
  color: var(--text-muted);
  font-size: 0.9rem;
}

.site-workspace__create-form {
  display: flex;
  align-items: end;
  gap: 0.8rem;
  flex-wrap: wrap;
}

.site-workspace__field {
  display: grid;
  gap: 0.35rem;
  min-width: 14rem;
}

.site-workspace__field span {
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.site-workspace__input,
.site-workspace__textarea,
.site-workspace__field input {
  width: 100%;
  min-width: 0;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 0.8rem;
  background: rgb(var(--surface-2) / 0.9);
  color: rgb(var(--text));
  padding: 0.7rem 0.8rem;
}

.site-workspace__input--sm {
  text-align: right;
}

.site-workspace__textarea {
  resize: vertical;
  min-height: 8rem;
}

.site-workspace__primary-btn,
.site-workspace__icon-btn {
  border: 1px solid transparent;
  border-radius: 0.8rem;
  cursor: pointer;
  transition: 0.18s ease;
}

.site-workspace__primary-btn {
  min-height: 2.85rem;
  padding: 0 1rem;
  background: rgb(var(--primary));
  color: rgb(var(--primary-contrast));
  font-weight: 700;
}

.site-workspace__primary-btn:hover {
  filter: brightness(1.06);
}

.site-workspace__toolbar-switch {
  display: inline-flex;
  align-items: center;
  gap: 0.55rem;
  min-height: 2.6rem;
  padding: 0 0.25rem;
  color: var(--text-muted);
  font-size: 0.84rem;
}

.site-workspace__tenant-chip {
  display: inline-flex;
  align-items: center;
  min-height: 2rem;
  padding: 0.2rem 0.65rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
  font-size: 0.82rem;
  font-weight: 700;
}

.site-workspace__stack-field,
.site-workspace__status-stack {
  display: grid;
  gap: 0.28rem;
}

.site-workspace__stack-field.is-end,
.site-workspace__status-stack {
  justify-items: end;
}

.site-workspace__stack-field small,
.site-workspace__status-stack small {
  color: var(--text-muted);
  font-size: 0.74rem;
}

.site-workspace__status-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 1.9rem;
  padding: 0.15rem 0.65rem;
  border-radius: 999px;
  font-size: 0.76rem;
  font-weight: 700;
  text-transform: capitalize;
}

.site-workspace__status-pill.is-webhook,
.site-workspace__status-pill.is-healthy {
  background: rgb(var(--success) / 0.14);
  color: rgb(var(--success));
}

.site-workspace__status-pill.is-manual,
.site-workspace__status-pill.is-pending {
  background: rgb(var(--warning) / 0.16);
  color: rgb(var(--warning));
}

.site-workspace__status-pill.is-error {
  background: rgb(var(--danger) / 0.14);
  color: rgb(var(--danger));
}

.site-workspace__actions {
  display: inline-flex;
  align-items: center;
  justify-content: end;
  gap: 0.45rem;
  flex-wrap: wrap;
}

.site-workspace__icon-btn {
  min-height: 2.1rem;
  padding: 0 0.7rem;
  background: rgb(var(--surface-2));
  border-color: rgb(var(--border) / 0.8);
  color: rgb(var(--text));
  font-size: 0.8rem;
}

.site-workspace__icon-btn.is-danger {
  color: rgb(var(--danger));
}

.site-workspace__icon-btn.is-success {
  color: rgb(var(--success));
}

.site-workspace__dialog-extra {
  display: grid;
  gap: 0.75rem;
}

@media (max-width: 920px) {
  .site-workspace__create-card {
    align-items: stretch;
  }

  .site-workspace__create-form {
    width: 100%;
  }

  .site-workspace__field {
    min-width: min(100%, 14rem);
    flex: 1 1 12rem;
  }
}
</style>
