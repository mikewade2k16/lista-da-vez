<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'

import AdminPageHeader from '~/layers/core/components/admin/AdminPageHeader.vue'
import AppDetailDialog from '~/components/ui/AppDetailDialog.vue'
import AppEntityGrid from '~/components/ui/AppEntityGrid.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useSiteStore } from '~/stores/site'
import { useUiStore } from '~/stores/ui'

const siteStore = useSiteStore()
const ui = useUiStore()

const { scopedLeads, tenantOptions, currentTenantId, isPlatformAdmin } = storeToRefs(siteStore)

const searchValue = ref('')
const tenantFilter = ref('')
const sourceFilter = ref('all')
const detailOpen = ref(false)
const selectedLeadId = ref('')

const columns = [
  { id: 'tenantName', label: 'Cliente', width: 'minmax(170px, 1fr)' },
  { id: 'name', label: 'Nome', width: 'minmax(180px, 1.2fr)', locked: true },
  { id: 'email', label: 'Email', width: 'minmax(220px, 1.35fr)' },
  { id: 'phone', label: 'Telefone', width: 'minmax(140px, 0.95fr)' },
  { id: 'source', label: 'Origem', width: 'minmax(140px, 1fr)' },
  { id: 'page', label: 'Pagina', width: 'minmax(150px, 1fr)' },
  { id: 'createdAt', label: 'Entrada', width: 'minmax(150px, 0.95fr)' },
  { id: 'actions', label: 'Acoes', width: 'minmax(150px, 0.95fr)', align: 'end', locked: true },
]

const sourceOptions = computed(() => {
  const unique = new Set(scopedLeads.value.map((lead) => lead.source))
  return [
    { label: 'Todas as origens', value: 'all' },
    ...[...unique]
      .sort((a, b) => a.localeCompare(b, 'pt-BR'))
      .map((source) => ({
        label: source,
        value: source,
      })),
  ]
})

const tenantFilterOptions = computed(() => [
  { label: 'Todos os clientes', value: '' },
  ...tenantOptions.value.map((tenant) => ({ label: tenant.label, value: tenant.id })),
])

const showTenantFilter = computed(() => isPlatformAdmin.value && tenantOptions.value.length > 1)
const effectiveTenantId = computed(() =>
  showTenantFilter.value ? tenantFilter.value : currentTenantId.value,
)

const filteredRows = computed(() => {
  const search = String(searchValue.value || '')
    .trim()
    .toLowerCase()

  return scopedLeads.value
    .filter((lead) => (!effectiveTenantId.value ? true : lead.tenantId === effectiveTenantId.value))
    .filter((lead) => (sourceFilter.value === 'all' ? true : lead.source === sourceFilter.value))
    .filter((lead) => {
      if (!search) {
        return true
      }

      return [
        lead.tenantName,
        lead.name,
        lead.email,
        lead.phone,
        lead.source,
        lead.page,
        lead.coupon,
      ]
        .join(' ')
        .toLowerCase()
        .includes(search)
    })
})

const selectedLead = computed(
  () => filteredRows.value.find((lead) => lead.id === selectedLeadId.value) || null,
)

const detailSections = computed(() => {
  if (!selectedLead.value) {
    return []
  }

  return [
    {
      id: 'identity',
      title: 'Lead',
      fields: [
        { label: 'Cliente', value: selectedLead.value.tenantName },
        { label: 'Nome', value: selectedLead.value.name },
        { label: 'Email', value: selectedLead.value.email },
        { label: 'Telefone', value: selectedLead.value.phone },
      ],
    },
    {
      id: 'tracking',
      title: 'Captacao',
      fields: [
        { label: 'Origem', value: selectedLead.value.source },
        { label: 'Pagina', value: selectedLead.value.page },
        { label: 'Cupom', value: selectedLead.value.coupon || '-' },
        { label: 'Consentimento', value: selectedLead.value.consentLabel },
      ],
    },
  ]
})

function formatDateTime(value: string) {
  const date = new Date(String(value || ''))
  if (Number.isNaN(date.getTime())) {
    return '-'
  }

  return date.toLocaleString('pt-BR')
}

function openDetails(leadId: string) {
  selectedLeadId.value = leadId
  detailOpen.value = true
}

function openWhatsapp(phone: string) {
  if (!import.meta.client) {
    return
  }

  const digits = String(phone || '').replace(/\D+/g, '')
  if (!digits) {
    return
  }

  const normalized = digits.length <= 11 ? `55${digits}` : digits
  window.open(`https://wa.me/${normalized}`, '_blank', 'noopener,noreferrer')
}

function openEmail(email: string) {
  if (!import.meta.client || !String(email || '').trim()) {
    return
  }

  window.open(`mailto:${encodeURIComponent(email)}`, '_blank')
}

function handleDeleteLead(leadId: string) {
  const result = siteStore.removeLead(leadId)
  if (!result.ok) {
    ui.error(result.message || 'Nao foi possivel excluir o lead.')
    return
  }

  if (selectedLeadId.value === leadId) {
    detailOpen.value = false
    selectedLeadId.value = ''
  }

  ui.success('Lead removido da grade local do site.')
}
</script>

<template>
  <section class="admin-panel site-leads-workspace" data-testid="site-leads-workspace">
    <AdminPageHeader
      eyebrow="Site"
      title="Leads"
      description="Leads capturados no site em uma grade administrativa simples, no mesmo padrao visual do novo frontend."
    />

    <AppEntityGrid
      testid="site-leads-grid"
      storage-key="site-leads-grid"
      :columns="columns"
      :rows="filteredRows"
      :search-value="searchValue"
      :loading="!siteStore.ready"
      search-placeholder="Buscar por nome, email, telefone, origem ou pagina..."
      empty-title="Nenhum lead encontrado"
      empty-text="Ajuste os filtros para revisar a fila de leads do site."
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
      </template>

      <template #cell-tenantName="{ row }">
        <span class="site-leads-workspace__tenant-chip">{{ row.tenantName }}</span>
      </template>

      <template #cell-createdAt="{ row }">
        {{ formatDateTime(row.createdAt) }}
      </template>

      <template #cell-actions="{ row }">
        <div class="site-leads-workspace__actions">
          <button
            class="site-leads-workspace__icon-btn is-success"
            type="button"
            @click="openWhatsapp(row.phone)"
          >
            WhatsApp
          </button>
          <button
            class="site-leads-workspace__icon-btn"
            type="button"
            @click="openEmail(row.email)"
          >
            Email
          </button>
          <button class="site-leads-workspace__icon-btn" type="button" @click="openDetails(row.id)">
            Info
          </button>
          <button
            class="site-leads-workspace__icon-btn is-danger"
            type="button"
            @click="handleDeleteLead(row.id)"
          >
            Excluir
          </button>
        </div>
      </template>
    </AppEntityGrid>

    <AppDetailDialog
      v-model="detailOpen"
      :title="selectedLead?.name || 'Detalhes do lead'"
      :subtitle="selectedLead?.email || selectedLead?.phone || 'Lead sem contato principal.'"
      :sections="detailSections"
    >
      <section v-if="selectedLead" class="site-leads-workspace__payloads">
        <article class="site-leads-workspace__payload-card">
          <h3>Tracking</h3>
          <pre>{{ selectedLead.trackingData }}</pre>
        </article>

        <article class="site-leads-workspace__payload-card">
          <h3>Payload</h3>
          <pre>{{ selectedLead.payloadJson }}</pre>
        </article>
      </section>
    </AppDetailDialog>
  </section>
</template>

<style scoped>
.site-leads-workspace {
  display: grid;
  gap: 1rem;
}

.site-leads-workspace__tenant-chip {
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

.site-leads-workspace__actions {
  display: inline-flex;
  justify-content: end;
  gap: 0.45rem;
  flex-wrap: wrap;
}

.site-leads-workspace__icon-btn {
  min-height: 2.1rem;
  padding: 0 0.7rem;
  border: 1px solid rgb(var(--border) / 0.82);
  border-radius: 0.8rem;
  background: rgb(var(--surface-2));
  color: rgb(var(--text));
  cursor: pointer;
}

.site-leads-workspace__icon-btn.is-success {
  color: rgb(var(--success));
}

.site-leads-workspace__icon-btn.is-danger {
  color: rgb(var(--danger));
}

.site-leads-workspace__payloads {
  display: grid;
  gap: 0.85rem;
}

.site-leads-workspace__payload-card {
  display: grid;
  gap: 0.45rem;
  padding: 0.95rem;
  border-radius: 0.95rem;
  border: 1px solid rgb(var(--border) / 0.8);
  background: rgb(var(--surface-2) / 0.84);
}

.site-leads-workspace__payload-card h3 {
  margin: 0;
  font-size: 0.88rem;
}

.site-leads-workspace__payload-card pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--text-muted);
  font-size: 0.8rem;
}
</style>
