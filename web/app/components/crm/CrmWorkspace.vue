<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { CalendarDays } from 'lucide-vue-next'

import { useAuthStore } from '~/stores/auth'
import { useCrmStore } from '~/stores/crm'
import { useCrmConsultantMetrics } from '~/composables/useCrmConsultantMetrics'

const crmStore = useCrmStore()
const auth = useAuthStore()
const { overview, pending, ready, errorMessage, dateFrom, dateTo } = storeToRefs(crmStore)

const managementStoreSlug = 'gerencia-multiloja'
const selectedStore = ref('')
const consultantSearch = ref('')

const summary = computed(
  () =>
    overview.value?.summary || {
      orders: 0,
      units: 0,
      salesCents: 0,
      ticketAverageCents: 0,
      valuePerProductCents: 0,
      paScore: 0,
      monthlyGoalCents: 0,
      goalProgress: 0,
      remainingToGoalCents: 0,
      unmappedSalesCents: 0,
    },
)
const canManageConsultantLinks = computed(() => auth.role === 'platform_admin')
const storeRows = computed(() => overview.value?.stores || [])
const consultantRows = computed(() => overview.value?.consultants || [])
const queueStats = computed(() => overview.value?.queueStats || null)
const warnings = computed(() => overview.value?.warnings || [])

const commercialStoreRows = computed(() =>
  storeRows.value.filter((row) => row.storeSlug !== managementStoreSlug),
)
const managementStoreRows = computed(() =>
  storeRows.value.filter((row) => row.storeSlug === managementStoreSlug),
)

const storeOptions = computed(() =>
  commercialStoreRows.value.map((row) => ({ slug: row.storeSlug, label: row.storeLabel })),
)

const filteredStoreRows = computed(() => {
  if (!selectedStore.value) return commercialStoreRows.value
  return commercialStoreRows.value.filter((row) => row.storeSlug === selectedStore.value)
})

const {
  erpStore,
  mergedConsultants,
  managementConsultantRows,
  unmatchedCount,
  consultantLinkOptions,
  consultantLinkDraftByRow,
  refreshConsultantLinks,
  autoLinkConsultants,
  saveConsultantLink,
  removeConsultantLink,
  updateConsultantLinkDraft,
  consultantLinkKey,
  linkStatusLabel,
  linkStatusClass,
} = useCrmConsultantMetrics({
  consultantRows,
  queueStats,
  selectedStore,
  consultantSearch,
  ready,
  canManageConsultantLinks,
})

async function submitFilters() {
  await crmStore.applyFilters()
}

async function resetMonth() {
  crmStore.resetCurrentMonth()
  await crmStore.applyFilters()
}

function clearLocalFilters() {
  selectedStore.value = ''
  consultantSearch.value = ''
}
</script>

<template>
  <section class="admin-panel crm-panel" data-testid="crm-panel">
    <header class="admin-panel__header crm-panel__header">
      <div>
        <h2 class="admin-panel__title">CRM comercial via ERP</h2>
        <p class="admin-panel__text">
          Metas cadastradas no sistema cruzadas com pedidos do ERP. Leitura comercial por loja e
          consultor.
        </p>
      </div>

      <!-- filtros de periodo -->
      <form class="crm-filters" @submit.prevent="submitFilters">
        <div class="crm-filters__date-wrap">
          <label class="crm-filters__label">Periodo</label>
          <AppDatePicker
            :model-value="dateFrom"
            :end-date="dateTo"
            @update:model-value="dateFrom = $event"
            @update:end-date="dateTo = $event"
          >
            <template #default="{ label }">
              <button type="button" class="crm-date-trigger">
                <CalendarDays :size="14" />
                <span>{{ label || 'Todas as vendas' }}</span>
              </button>
            </template>
          </AppDatePicker>
        </div>

        <div class="crm-filters__actions">
          <button class="crm-btn crm-btn--ghost" type="button" @click="resetMonth">
            Mes atual
          </button>
          <button class="crm-btn" type="submit" :disabled="pending">
            {{ pending ? 'Atualizando...' : 'Atualizar' }}
          </button>
        </div>
      </form>
    </header>

    <!-- filtros de loja e consultor (local, sem nova requisição) -->
    <div v-if="ready" class="crm-local-filters">
      <select v-model="selectedStore" class="crm-select" title="Filtrar por loja">
        <option value="">Todas as lojas</option>
        <option v-for="s in storeOptions" :key="s.slug" :value="s.slug">
          {{ s.label }}
        </option>
      </select>

      <input
        v-model="consultantSearch"
        class="crm-search"
        type="text"
        placeholder="Buscar consultor..."
        autocomplete="off"
      />

      <button
        v-if="selectedStore || consultantSearch"
        class="crm-btn crm-btn--ghost crm-btn--sm"
        type="button"
        @click="clearLocalFilters"
      >
        Limpar filtros
      </button>
    </div>

    <article v-if="errorMessage" class="settings-card">
      <p class="settings-card__text">{{ errorMessage }}</p>
    </article>

    <article v-else-if="pending && !ready" class="settings-card">
      <p class="settings-card__text">Carregando CRM do ERP...</p>
    </article>

    <section v-else class="crm-panel__content">
      <CrmSummarySection
        :summary="summary"
        :queue-stats="queueStats"
        :warnings="warnings"
        :unmatched-count="unmatchedCount"
      />

      <CrmStoresSection
        :filtered-store-rows="filteredStoreRows"
        :management-store-rows="managementStoreRows"
        :queue-stats="queueStats"
        :summary="summary"
        :date-from="overview?.dateFrom"
        :date-to="overview?.dateTo"
      />

      <CrmConsultantsSection
        :merged-consultants="mergedConsultants"
        :management-consultant-rows="managementConsultantRows"
        :can-manage-consultant-links="canManageConsultantLinks"
        :loading-consultant-links="erpStore.loadingConsultantLinks"
        :saving-consultant-link="erpStore.savingConsultantLink"
        :consultant-link-options="consultantLinkOptions"
        :consultant-link-drafts="consultantLinkDraftByRow"
        :queue-stats-available="!!queueStats"
        :consultant-link-key="consultantLinkKey"
        :link-status-label="linkStatusLabel"
        :link-status-class="linkStatusClass"
        @auto-link="autoLinkConsultants"
        @refresh-links="refreshConsultantLinks"
        @save-link="saveConsultantLink"
        @remove-link="removeConsultantLink"
        @update-draft="updateConsultantLinkDraft"
      />

      <CrmConsultantsManagementSection :rows="managementConsultantRows" />
    </section>
  </section>
</template>

<style scoped>
.crm-panel {
  gap: 1.25rem;
}

.crm-panel__header {
  align-items: flex-start;
  gap: 1rem;
}

.crm-panel__content {
  display: grid;
  gap: 1rem;
}

.crm-filters {
  display: flex;
  gap: 0.75rem;
  align-items: flex-end;
  flex-wrap: wrap;
}

.crm-filters__date-wrap {
  display: grid;
  gap: 0.3rem;
}

.crm-filters__label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.07em;
  text-transform: uppercase;
  color: rgb(var(--muted));
}

.crm-filters__actions {
  display: flex;
  gap: 0.5rem;
}

.crm-date-trigger {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 13rem;
  min-height: 42px;
  padding: 0 0.85rem;
  border-radius: 12px;
  border: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface) / 0.95);
  color: rgb(var(--text));
  font-size: 0.88rem;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
  white-space: nowrap;
}

/* filtros locais */
.crm-local-filters {
  display: flex;
  gap: 0.75rem;
  align-items: center;
  flex-wrap: wrap;
}

.crm-select,
.crm-search {
  min-height: 38px;
  padding: 0 0.85rem;
  border-radius: 10px;
  border: 1px solid rgb(var(--border) / 0.88);
  background: rgb(var(--surface) / 0.95);
  color: rgb(var(--text));
  font-size: 0.88rem;
}

.crm-select {
  min-width: 160px;
  cursor: pointer;
}

.crm-search {
  min-width: 200px;
}

.crm-search::placeholder {
  color: rgb(var(--muted) / 0.72);
}

.crm-btn {
  min-height: 42px;
  border: none;
  border-radius: 12px;
  padding: 0.75rem 1rem;
  background: rgb(var(--primary));
  color: rgb(255 255 255);
  font-weight: 700;
  cursor: pointer;
  white-space: nowrap;
}

.crm-btn:disabled {
  cursor: wait;
  opacity: 0.72;
}

.crm-btn--ghost {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.crm-btn--sm {
  min-height: 38px;
  padding: 0.45rem 0.75rem;
  font-size: 0.84rem;
  border-radius: 10px;
}

@media (max-width: 860px) {
  .crm-panel__header,
  .crm-section__header {
    grid-template-columns: 1fr;
    display: grid;
  }

  .crm-filters {
    flex-direction: column;
    align-items: flex-start;
  }

  .crm-filters__actions {
    width: 100%;
  }

  .crm-btn {
    flex: 1 1 0;
  }
}
</style>
