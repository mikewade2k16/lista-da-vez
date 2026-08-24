<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useCoreAccountStore } from '../../../layers/core/stores/account'
import {
  listMetaActionProposals,
  reconcileMetaActionProposal,
  type MetaAdsActionProposalView,
} from '~/domain/meta-ads/meta-ads-actions-api'
import { useAuthStore } from '~/stores/auth'
import { useMetaAdsStore } from '~/stores/meta-ads'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

const auth = useAuthStore()
const accountStore = useCoreAccountStore()
const store = useMetaAdsStore()
const runtimeConfig = useRuntimeConfig()
const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

const proposals = ref<MetaAdsActionProposalView[]>([])
const loading = ref(false)
const error = ref('')
const reconcilingId = ref('')
let generation = 0
let requestAbort: AbortController | null = null

const hasItems = computed(() => proposals.value.length > 0)

const statusLabels: Record<MetaAdsActionProposalView['status'], string> = {
  pending: 'Aguardando confirmação',
  executing: 'Executando',
  succeeded: 'Concluída',
  failed: 'Falhou',
  unknown: 'Resultado incerto',
  cancelled: 'Cancelada',
  expired: 'Expirada',
}

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('pt-BR')
}

function canReconcile(proposal: MetaAdsActionProposalView): boolean {
  return (
    store.canManageMetaAds &&
    proposal.executionAvailable &&
    (proposal.status === 'unknown' || proposal.status === 'executing')
  )
}

function resultEntities(proposal: MetaAdsActionProposalView): { label: string; id: string }[] {
  const labels: Record<string, string> = {
    campaignId: 'Campanha',
    adSetId: 'Conjunto',
    creativeId: 'Criativo',
    adId: 'Anúncio',
  }
  return Object.entries(labels).flatMap(([key, label]) => {
    const value = proposal.result[key]
    return typeof value === 'string' && value.trim() ? [{ label, id: value.trim() }] : []
  })
}

async function load(): Promise<void> {
  const accountId = accountStore.activeAccountId.trim()
  generation += 1
  const currentGeneration = generation
  requestAbort?.abort()
  const abort = new AbortController()
  requestAbort = abort
  proposals.value = []
  error.value = ''
  if (!accountId || !store.connected) return
  loading.value = true
  try {
    const result = await listMetaActionProposals(apiRequest, 25, abort.signal)
    if (
      currentGeneration !== generation ||
      abort.signal.aborted ||
      accountId !== accountStore.activeAccountId.trim()
    )
      return
    proposals.value = result
  } catch (caught) {
    if (currentGeneration !== generation || abort.signal.aborted) return
    error.value = getApiErrorMessage(caught, 'Não foi possível carregar a trilha de ações.')
  } finally {
    if (currentGeneration === generation) loading.value = false
  }
}

async function reconcile(proposal: MetaAdsActionProposalView): Promise<void> {
  const accountId = accountStore.activeAccountId.trim()
  if (!canReconcile(proposal) || reconcilingId.value || !accountId) return
  const currentGeneration = generation
  reconcilingId.value = proposal.id
  error.value = ''
  try {
    const updated = await reconcileMetaActionProposal(apiRequest, proposal.id)
    if (currentGeneration !== generation || accountId !== accountStore.activeAccountId.trim())
      return
    proposals.value = proposals.value.map((item) => (item.id === updated.id ? updated : item))
  } catch (caught) {
    if (currentGeneration !== generation) return
    error.value = getApiErrorMessage(caught, 'Não foi possível reconciliar esta ação.')
  } finally {
    if (currentGeneration === generation) reconcilingId.value = ''
  }
}

watch(
  () => [accountStore.activeAccountId, store.connected] as const,
  () => void load(),
  { immediate: true, flush: 'sync' },
)

onBeforeUnmount(() => {
  generation += 1
  requestAbort?.abort()
})
</script>

<template>
  <article class="action-history">
    <header class="action-history__header">
      <div>
        <p class="action-history__eyebrow">Auditoria</p>
        <h3>Ações recentes do Meta Ads</h3>
        <p>Confirmações, resultados e falhas persistidos pelo backend.</p>
      </div>
      <button type="button" :disabled="loading" @click="load">
        {{ loading ? 'Atualizando…' : 'Atualizar' }}
      </button>
    </header>

    <p v-if="error" class="action-history__error" role="alert">{{ error }}</p>
    <p v-if="loading && !hasItems" class="action-history__empty">Carregando ações…</p>
    <p v-else-if="!hasItems" class="action-history__empty">
      Nenhuma ação foi preparada nesta conta ainda.
    </p>

    <ul v-else class="action-history__list">
      <li v-for="proposal in proposals" :key="proposal.id" class="action-history__item">
        <div class="action-history__item-main">
          <div class="action-history__item-line">
            <strong>{{ proposal.summary || proposal.action }}</strong>
            <span class="action-history__status" :data-status="proposal.status">
              {{ statusLabels[proposal.status] }}
            </span>
          </div>
          <p>{{ formatDate(proposal.createdAt) }} · {{ proposal.adAccountName || 'Conta Meta' }}</p>
          <p v-if="proposal.externalEntityId" class="action-history__technical">
            ID Meta: {{ proposal.externalEntityId }}
          </p>
          <p
            v-for="entity in resultEntities(proposal)"
            :key="`${proposal.id}:${entity.label}`"
            class="action-history__technical"
          >
            {{ entity.label }}: {{ entity.id }}
          </p>
          <p v-if="proposal.errorMessage" class="action-history__failure">
            {{ proposal.errorMessage }}
          </p>
        </div>
        <button
          v-if="canReconcile(proposal)"
          type="button"
          :disabled="reconcilingId === proposal.id"
          @click="reconcile(proposal)"
        >
          {{ reconcilingId === proposal.id ? 'Verificando…' : 'Verificar resultado' }}
        </button>
      </li>
    </ul>
  </article>
</template>

<style scoped>
.action-history {
  display: grid;
  gap: 1rem;
  padding: 1.1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.72);
  box-shadow: var(--shadow-card);
}

.action-history__header,
.action-history__item,
.action-history__item-line {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.8rem;
}

.action-history h3,
.action-history p {
  margin: 0;
}

.action-history h3 {
  margin: 0.15rem 0 0.2rem;
  font-size: 1rem;
}

.action-history__header p,
.action-history__item p,
.action-history__empty {
  color: var(--text-muted);
  font-size: 0.78rem;
}

.action-history__eyebrow {
  font-size: 0.66rem !important;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.action-history button {
  flex: 0 0 auto;
  padding: 0.5rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.5rem;
  background: transparent;
  color: var(--text-main);
  font: inherit;
  font-size: 0.75rem;
  font-weight: 700;
  cursor: pointer;
}

.action-history button:disabled {
  opacity: 0.55;
  cursor: wait;
}

.action-history__list {
  display: grid;
  gap: 0.65rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.action-history__item {
  padding: 0.8rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.7rem;
  background: rgb(var(--surface-2) / 0.35);
}

.action-history__item-main {
  min-width: 0;
}

.action-history__item-line strong {
  min-width: 0;
  color: var(--text-main);
  font-size: 0.82rem;
  overflow-wrap: anywhere;
}

.action-history__status {
  flex: 0 0 auto;
  padding: 0.2rem 0.45rem;
  border-radius: 999px;
  background: rgb(var(--surface-3) / 0.75);
  color: var(--text-muted);
  font-size: 0.66rem;
  font-weight: 800;
}

.action-history__status[data-status='succeeded'] {
  color: rgb(var(--success));
  background: rgb(var(--success) / 0.1);
}

.action-history__status[data-status='failed'],
.action-history__status[data-status='unknown'] {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.1);
}

.action-history__technical {
  overflow-wrap: anywhere;
}

.action-history__failure,
.action-history__error {
  color: rgb(var(--danger)) !important;
  font-size: 0.78rem;
}

@media (max-width: 720px) {
  .action-history__header,
  .action-history__item,
  .action-history__item-line {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
