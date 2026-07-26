<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'

import AdminPageHeader from '../../../layers/core/components/admin/AdminPageHeader.vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import CommunicationEditorDrawer from './CommunicationEditorDrawer.vue'
import {
  communicationPeriod,
  communicationStatus,
  communicationStatusLabel,
  type QueueCommunication,
  type QueueCommunicationInput,
} from '~/domain/operation/communications'
import { canManageCommunications } from '~/domain/utils/permissions'
import { useAuthStore } from '~/stores/auth'
import { useCommunicationsStore } from '~/stores/communications'
import { useUiStore } from '~/stores/ui'

const props = withDefaults(
  defineProps<{
    embedded?: boolean
  }>(),
  {
    embedded: false,
  },
)

const ALL_STORES = '__all_stores__'
const ALL_STATUSES = '__all_statuses__'

const auth = useAuthStore()
const communicationsStore = useCommunicationsStore()
const ui = useUiStore()
const { items, loading, saving, deletingId, errorMessage, editorErrorMessage } =
  storeToRefs(communicationsStore)

const selectedStoreId = ref(ALL_STORES)
const selectedStatus = ref(ALL_STATUSES)
const editorOpen = ref(false)
const selectedItem = ref<QueueCommunication | null>(null)

const canManage = computed(() =>
  canManageCommunications(auth.role, auth.effectivePermissionKeys, auth.permissionsResolved),
)
const stores = computed(() =>
  (auth.storeContext || []).map((store) => ({
    id: String(store?.id || '').trim(),
    name: String(store?.name || store?.code || 'Loja').trim(),
    accountId: String(store?.tenantId || '').trim(),
  })),
)
const storeOptions = computed(() => [
  { value: ALL_STORES, label: 'Todas as lojas' },
  ...stores.value.map((store) => ({ value: store.id, label: store.name })),
])
const statusOptions = [
  { value: ALL_STATUSES, label: 'Todos os status' },
  { value: 'active', label: 'Em exibição' },
  { value: 'scheduled', label: 'Agendados' },
  { value: 'draft', label: 'Rascunhos' },
  { value: 'expired', label: 'Encerrados' },
]
const accountOptions = computed(() => {
  const labels = new Map(
    (auth.tenantContext || []).map((tenant) => [
      String(tenant?.id || '').trim(),
      String(tenant?.name || tenant?.slug || 'Conta').trim(),
    ]),
  )
  return Array.from(new Set(stores.value.map((store) => store.accountId).filter(Boolean))).map(
    (accountId) => ({
      value: accountId,
      label:
        labels.get(accountId) ||
        stores.value
          .filter((store) => store.accountId === accountId)
          .map((store) => store.name)
          .join(', '),
    }),
  )
})
const defaultAccountId = computed(() => {
  const activeAccountId = String(auth.activeTenantId || '').trim()
  if (accountOptions.value.some((option) => option.value === activeAccountId)) {
    return activeAccountId
  }
  return accountOptions.value[0]?.value || activeAccountId
})
const storeNames = computed(() => new Map(stores.value.map((store) => [store.id, store.name])))
const filteredItems = computed(() =>
  items.value.filter((item) => {
    const matchesStore =
      selectedStoreId.value === ALL_STORES ||
      item.targetsAllStores ||
      item.storeIds.includes(selectedStoreId.value)
    const matchesStatus =
      selectedStatus.value === ALL_STATUSES || communicationStatus(item) === selectedStatus.value
    return matchesStore && matchesStatus
  }),
)

function scopeLabel(item: QueueCommunication): string {
  if (item.targetsAllStores) return 'Todas as lojas'
  const labels = item.storeIds.map((storeId) => storeNames.value.get(storeId)).filter(Boolean)
  return labels.length ? labels.join(', ') : `${item.storeIds.length} loja(s)`
}

function openCreate(): void {
  communicationsStore.clearEditorError()
  selectedItem.value = null
  editorOpen.value = true
}

function openEdit(item: QueueCommunication): void {
  communicationsStore.clearEditorError()
  selectedItem.value = item
  editorOpen.value = true
}

async function saveCommunication(payload: {
  accountId: string
  input: QueueCommunicationInput
}): Promise<void> {
  const result = selectedItem.value
    ? await communicationsStore.update(selectedItem.value, payload.input)
    : await communicationsStore.create(payload.accountId, payload.input)
  if (!result) return
  editorOpen.value = false
  selectedItem.value = null
  ui.success('Comunicado salvo.')
}

async function removeCommunication(item: QueueCommunication): Promise<void> {
  const answer = (await ui.confirm({
    title: 'Excluir comunicado',
    message: `O comunicado “${item.title}” deixará de ser exibido. Deseja continuar?`,
    confirmLabel: 'Excluir',
  })) as { confirmed?: boolean }
  if (!answer.confirmed) return
  if (await communicationsStore.remove(item)) {
    ui.success('Comunicado excluído.')
  } else if (editorErrorMessage.value) {
    ui.error(editorErrorMessage.value)
  }
}

onMounted(() => communicationsStore.load())
</script>

<template>
  <section
    class="communications-workspace"
    :class="{ 'communications-workspace--embedded': props.embedded }"
  >
    <div class="communications-workspace__heading">
      <AdminPageHeader
        v-if="!props.embedded"
        eyebrow="Fila de atendimento"
        title="Comunicados"
        description="Publique avisos no painel da operação para todas as lojas ou apenas para lojas específicas."
      />
      <div v-else class="communications-workspace__copy">
        <h2>Comunicados</h2>
        <p>Avise todas as lojas ou selecione onde cada comunicado será exibido.</p>
      </div>
      <AppPanelButton v-if="canManage" class="communications-workspace__new" @click="openCreate">
        <UIcon name="i-lucide-plus" aria-hidden="true" />
        Novo comunicado
      </AppPanelButton>
    </div>

    <div class="communications-workspace__filters">
      <AppSelectField
        v-model="selectedStoreId"
        class="communications-workspace__filter"
        label="Loja"
        :options="storeOptions"
      />
      <AppSelectField
        v-model="selectedStatus"
        class="communications-workspace__filter"
        label="Status"
        :options="statusOptions"
      />
    </div>

    <div v-if="loading && items.length === 0" class="communications-state">
      <UIcon class="communications-state__spin" name="i-lucide-loader-circle" />
      <strong>Carregando comunicados…</strong>
    </div>

    <div v-else-if="errorMessage" class="communications-state communications-state--error">
      <UIcon name="i-lucide-circle-alert" />
      <strong>{{ errorMessage }}</strong>
      <AppPanelButton variant="ghost" @click="communicationsStore.load()">
        Tentar novamente
      </AppPanelButton>
    </div>

    <div v-else-if="filteredItems.length === 0" class="communications-state">
      <UIcon name="i-lucide-megaphone" />
      <strong>Nenhum comunicado encontrado</strong>
      <p v-if="canManage">Crie o primeiro comunicado ou ajuste os filtros.</p>
    </div>

    <div v-else class="communications-grid">
      <article
        v-for="item in filteredItems"
        :key="`${item.accountId}:${item.id}`"
        class="communication-card"
      >
        <header class="communication-card__header">
          <div>
            <h2>{{ item.title }}</h2>
            <span class="communication-card__status" :class="`is-${communicationStatus(item)}`">
              {{ communicationStatusLabel(communicationStatus(item)) }}
            </span>
          </div>
          <span class="communication-card__order">Ordem {{ item.displayOrder }}</span>
        </header>

        <p class="communication-card__excerpt">
          {{ item.excerpt || item.body }}
        </p>

        <dl class="communication-card__meta">
          <div>
            <dt>
              <UIcon name="i-lucide-store" />
              Lojas
            </dt>
            <dd>{{ scopeLabel(item) }}</dd>
          </div>
          <div>
            <dt>
              <UIcon name="i-lucide-calendar-clock" />
              Exibição
            </dt>
            <dd>{{ communicationPeriod(item) }}</dd>
          </div>
        </dl>

        <footer v-if="canManage" class="communication-card__actions">
          <AppPanelButton variant="ghost" @click="openEdit(item)">
            <UIcon name="i-lucide-pencil" />
            Editar
          </AppPanelButton>
          <AppPanelButton
            variant="ghost"
            :disabled="Boolean(deletingId)"
            @click="removeCommunication(item)"
          >
            <UIcon name="i-lucide-trash-2" />
            {{ deletingId === item.id ? 'Excluindo…' : 'Excluir' }}
          </AppPanelButton>
        </footer>
      </article>
    </div>

    <CommunicationEditorDrawer
      v-model:open="editorOpen"
      :item="selectedItem"
      :default-account-id="defaultAccountId"
      :account-options="accountOptions"
      :stores="stores"
      :saving="saving"
      :error-message="editorErrorMessage"
      @save="saveCommunication"
    />
  </section>
</template>

<style scoped>
.communications-workspace {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
  gap: 0.8rem;
  padding: 0.75rem 1rem 1rem;
  overflow-y: auto;
}

.communications-workspace--embedded {
  flex: 0 0 auto;
  padding: 0;
  overflow: visible;
}

.communications-workspace__heading,
.communications-workspace__filters,
.communication-card__header,
.communication-card__actions {
  display: flex;
  align-items: center;
}

.communications-workspace__heading {
  justify-content: space-between;
  gap: 1rem;
}

.communications-workspace__copy h2 {
  margin: 0;
  color: var(--text-main);
  font-size: 0.92rem;
}

.communications-workspace__copy p {
  margin: 0.2rem 0 0;
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.communications-workspace__new,
.communication-card__actions :deep(.app-panel-button) {
  gap: 0.42rem;
}

.communications-workspace__filters {
  gap: 0.55rem;
}

.communications-workspace__filter {
  width: min(15rem, 100%);
}

.communications-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 19rem), 1fr));
  gap: 0.65rem;
}

.communication-card {
  display: grid;
  min-width: 0;
  gap: 0.65rem;
  padding: 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: 14px;
  background: var(--bg-panel);
}

.communication-card__header {
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.7rem;
}

.communication-card__header h2 {
  margin: 0 0 0.35rem;
  color: var(--text-main);
  font-size: 0.9rem;
}

.communication-card__status,
.communication-card__order {
  display: inline-flex;
  padding: 0.2rem 0.45rem;
  border-radius: 999px;
  color: rgb(var(--muted));
  background: rgb(var(--surface-2) / 0.72);
  font-size: 0.63rem;
  font-weight: 800;
}

.communication-card__status.is-active {
  color: rgb(var(--success));
  background: rgb(var(--success) / 0.11);
}

.communication-card__status.is-scheduled {
  color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.11);
}

.communication-card__status.is-expired {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.1);
}

.communication-card__excerpt {
  display: -webkit-box;
  margin: 0;
  overflow: hidden;
  color: rgb(var(--muted));
  font-size: 0.75rem;
  line-height: 1.5;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
}

.communication-card__meta {
  display: grid;
  gap: 0.45rem;
  margin: 0;
}

.communication-card__meta div {
  min-width: 0;
}

.communication-card__meta dt {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  color: rgb(var(--muted));
  font-size: 0.64rem;
  font-weight: 800;
  text-transform: uppercase;
}

.communication-card__meta dd {
  margin: 0.15rem 0 0;
  overflow: hidden;
  color: var(--text-main);
  font-size: 0.72rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.communication-card__actions {
  justify-content: flex-end;
  gap: 0.4rem;
  padding-top: 0.1rem;
}

.communication-card__actions :deep(.app-panel-button) {
  min-height: 2rem;
  padding: 0 0.7rem;
  border-radius: 9px;
  font-size: 0.69rem;
}

.communications-state {
  display: grid;
  min-height: 13rem;
  place-items: center;
  align-content: center;
  gap: 0.5rem;
  padding: 1.5rem;
  border: 1px dashed rgb(var(--border));
  border-radius: 14px;
  color: rgb(var(--muted));
  text-align: center;
}

.communications-state p {
  margin: 0;
}

.communications-state--error {
  color: rgb(var(--danger));
}

.communications-state__spin {
  animation: communications-spin 900ms linear infinite;
}

@keyframes communications-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 640px) {
  .communications-workspace {
    padding: 0.65rem;
  }

  .communications-workspace__heading,
  .communications-workspace__filters {
    align-items: stretch;
    flex-direction: column;
  }

  .communications-workspace__filter {
    width: 100%;
  }
}
</style>
