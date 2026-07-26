<script setup lang="ts">
import { ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import AppEntityGrid from '~/components/ui/AppEntityGrid.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useCustomerIntelligenceAccess } from '~/composables/customer-intelligence/useCustomerIntelligenceAccess'
import type { CustomerSubjectListItem } from '~/domain/customer-data/profile-types'
import { useCustomerIntelligenceStore } from '~/stores/customer-intelligence'

const access = useCustomerIntelligenceAccess()
const store = useCustomerIntelligenceStore()
const { subjects, subjectsHasMore, subjectsStatus, subjectsError, clientAccountId } =
  storeToRefs(store)
const search = ref('')
const lifecycleStatus = ref('')

const columns = [
  { id: 'name', label: 'Cliente', width: 'minmax(220px, 1.4fr)', locked: true },
  { id: 'lifecycle', label: 'Etapa', width: 'minmax(120px, 0.7fr)' },
  { id: 'identity', label: 'Identidade', width: 'minmax(150px, 0.8fr)' },
  { id: 'updatedAt', label: 'Atualizado', width: 'minmax(150px, 0.8fr)' },
  { id: 'actions', label: '', width: 'minmax(90px, 0.45fr)', align: 'end' },
]
const lifecycleOptions = [
  { value: '', label: 'Todas as etapas' },
  { value: 'lead', label: 'Lead' },
  { value: 'customer', label: 'Cliente' },
  { value: 'inactive', label: 'Inativo' },
]

async function refresh(): Promise<void> {
  if (!access.clientScopeReady.value || !access.canViewSubjects.value) return
  await store.loadSubjects({
    query: search.value,
    lifecycleStatus: lifecycleStatus.value,
  })
}

function subjectRow(row: unknown): CustomerSubjectListItem {
  return row as CustomerSubjectListItem
}

watch(
  [() => access.clientScopeReady.value, clientAccountId],
  ([ready]) => {
    if (ready) void refresh()
  },
  { immediate: true },
)
</script>

<template>
  <CustomerIntelligencePageShell
    title="Inteligencia de clientes"
    description="Perfis explicaveis por relacionamento, com Customer Data autoritativo e inteligencia opcional."
  >
    <CustomerIntelligenceStatus
      v-if="!access.canViewSubjects.value"
      title="Customer Data indisponivel"
      :error="{
        kind: access.hasCustomerDataModule.value ? 'forbidden' : 'capability_off',
        message: '',
        reasonCode: access.hasCustomerDataModule.value
          ? 'customer_data_subjects_view_required'
          : 'customer_data_module_disabled',
        statusCode: access.hasCustomerDataModule.value ? 403 : 0,
      }"
    />

    <AppEntityGrid
      v-else
      :columns="columns"
      :rows="subjects"
      row-key="subjectId"
      :search-value="search"
      search-placeholder="Buscar nome, alias ou identidade permitida"
      :loading="subjectsStatus === 'loading'"
      empty-title="Nenhum cliente encontrado"
      empty-text="O backend nao retornou relacionamentos neste client scope."
      storage-key="customer-intelligence-subject-columns"
      @update:search-value="search = $event"
    >
      <template #filters>
        <AppSelectField
          v-model="lifecycleStatus"
          :options="lifecycleOptions"
          compact
          @change="refresh"
        />
        <button class="ci-action" type="button" @click="refresh">Buscar</button>
      </template>

      <template #cell-name="{ row }">
        <div class="ci-subject">
          <strong>{{ subjectRow(row).relationship.displayName || 'Cliente sem nome' }}</strong>
          <small>
            {{ subjectRow(row).subjectType }} · relacionamento
            {{ subjectRow(row).relationship.id.slice(0, 8) }}
          </small>
        </div>
      </template>
      <template #cell-lifecycle="{ row }">
        {{ subjectRow(row).relationship.lifecycleStatus }}
      </template>
      <template #cell-identity="{ row }">
        {{ subjectRow(row).primaryIdentities?.[0]?.maskedValue || 'Nao disponibilizada' }}
      </template>
      <template #cell-updatedAt="{ row }">
        {{
          subjectRow(row).relationship.updatedAt
            ? new Date(subjectRow(row).relationship.updatedAt).toLocaleString('pt-BR')
            : '—'
        }}
      </template>
      <template #cell-actions="{ row }">
        <NuxtLink
          class="ci-action"
          :to="`/inteligencia-clientes/${encodeURIComponent(subjectRow(row).relationship.id)}`"
        >
          Abrir
        </NuxtLink>
      </template>
      <template #empty>
        <CustomerIntelligenceStatus
          v-if="subjectsError"
          :error="subjectsError"
          title="Lista indisponivel"
        />
      </template>
    </AppEntityGrid>

    <button
      v-if="access.canViewSubjects.value && subjectsHasMore"
      class="ci-action ci-action--more"
      type="button"
      :disabled="subjectsStatus === 'loading'"
      @click="store.loadSubjects({ query: search, lifecycleStatus }, true)"
    >
      Carregar mais
    </button>
  </CustomerIntelligencePageShell>
</template>

<style scoped>
.ci-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.25rem;
  padding: 0 0.8rem;
  border: 1px solid rgb(var(--primary) / 0.25);
  border-radius: 999px;
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
  font-size: 0.76rem;
  font-weight: 700;
  text-decoration: none;
  cursor: pointer;
}

.ci-action--more {
  justify-self: center;
}

.ci-subject {
  display: grid;
  gap: 0.15rem;
}

.ci-subject small {
  color: rgb(var(--muted));
}
</style>
