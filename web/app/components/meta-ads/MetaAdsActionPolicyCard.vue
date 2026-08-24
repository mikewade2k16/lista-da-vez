<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useCoreAccountStore } from '../../../layers/core/stores/account'
import {
  getMetaAdsActionPolicy,
  saveMetaAdsActionPolicy,
  type MetaAdsActionPolicy,
} from '~/domain/meta-ads/meta-ads-policy-api'
import { useAuthStore } from '~/stores/auth'
import { useMetaAdsStore } from '~/stores/meta-ads'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

const auth = useAuthStore()
const accountStore = useCoreAccountStore()
const store = useMetaAdsStore()
const runtimeConfig = useRuntimeConfig()
const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

const policy = ref<MetaAdsActionPolicy | null>(null)
const dailyBudget = ref('')
const lifetimeBudget = ref('')
const allowCreate = ref(false)
const allowDuplicate = ref(false)
const allowResume = ref(false)
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const saved = ref(false)
let generation = 0
let requestAbort: AbortController | null = null

const canEdit = computed(
  () => accountStore.activeAccount?.isAgency === true && store.canManageMetaAds,
)

function applyPolicy(next: MetaAdsActionPolicy): void {
  policy.value = next
  dailyBudget.value = next.maxDailyBudget === null ? '' : String(next.maxDailyBudget)
  lifetimeBudget.value = next.maxLifetimeBudget === null ? '' : String(next.maxLifetimeBudget)
  allowCreate.value = next.allowCreate
  allowDuplicate.value = next.allowDuplicate
  allowResume.value = next.allowResume
}

function optionalPositive(value: string): number | null | undefined {
  const normalized = value.trim().replace(',', '.')
  if (!normalized) return null
  const parsed = Number(normalized)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined
}

async function loadPolicy(accountId: string, adAccountId: string): Promise<void> {
  generation += 1
  const currentGeneration = generation
  requestAbort?.abort()
  const abort = new AbortController()
  requestAbort = abort
  policy.value = null
  error.value = ''
  saved.value = false
  if (!accountId || !adAccountId) {
    loading.value = false
    return
  }
  loading.value = true
  try {
    const next = await getMetaAdsActionPolicy(apiRequest, adAccountId, abort.signal)
    if (currentGeneration !== generation || abort.signal.aborted) return
    applyPolicy(next)
  } catch (caught) {
    if (currentGeneration !== generation || abort.signal.aborted) return
    error.value = getApiErrorMessage(caught, 'Nao foi possivel carregar a politica de seguranca.')
  } finally {
    if (currentGeneration === generation) loading.value = false
  }
}

async function savePolicy(): Promise<void> {
  const accountId = accountStore.activeAccountId.trim()
  const adAccountId = store.selectedAdAccountId.trim()
  if (!canEdit.value || !accountId || !adAccountId || saving.value) return
  const maxDailyBudget = optionalPositive(dailyBudget.value)
  const maxLifetimeBudget = optionalPositive(lifetimeBudget.value)
  if (maxDailyBudget === undefined || maxLifetimeBudget === undefined) {
    error.value = 'Informe tetos positivos ou deixe o campo vazio.'
    return
  }

  generation += 1
  const currentGeneration = generation
  requestAbort?.abort()
  const abort = new AbortController()
  requestAbort = abort
  saving.value = true
  saved.value = false
  error.value = ''
  try {
    const next = await saveMetaAdsActionPolicy(
      apiRequest,
      adAccountId,
      {
        maxDailyBudget,
        maxLifetimeBudget,
        allowCreate: allowCreate.value,
        allowDuplicate: allowDuplicate.value,
        allowResume: allowResume.value,
      },
      abort.signal,
    )
    if (
      currentGeneration !== generation ||
      abort.signal.aborted ||
      accountId !== accountStore.activeAccountId.trim() ||
      adAccountId !== store.selectedAdAccountId.trim()
    ) {
      return
    }
    applyPolicy(next)
    saved.value = true
  } catch (caught) {
    if (currentGeneration !== generation || abort.signal.aborted) return
    error.value = getApiErrorMessage(caught, 'Nao foi possivel salvar a politica de seguranca.')
  } finally {
    if (currentGeneration === generation) saving.value = false
  }
}

watch(
  () => [accountStore.activeAccountId, store.selectedAdAccountId] as const,
  ([accountId, adAccountId]) => {
    void loadPolicy(accountId, adAccountId)
  },
  { immediate: true, flush: 'sync' },
)

onBeforeUnmount(() => {
  generation += 1
  requestAbort?.abort()
})
</script>

<template>
  <article v-if="store.selectedAdAccountId" class="policy-card">
    <header class="policy-card__header">
      <div>
        <p class="policy-card__eyebrow">Barreira financeira</p>
        <h3>Politica de acoes da IA</h3>
        <p>
          Define tetos server-side para esta conta de anuncios. A confirmacao visual ou por texto
          nunca ignora estes limites.
        </p>
      </div>
      <span class="policy-card__currency">{{ policy?.currency || '---' }}</span>
    </header>

    <p v-if="loading" class="policy-card__muted">Carregando politica...</p>

    <form v-else-if="policy" class="policy-card__form" @submit.prevent="savePolicy">
      <div class="policy-card__budgets">
        <label>
          <span>Teto diario</span>
          <input
            v-model="dailyBudget"
            inputmode="decimal"
            placeholder="Ex.: 250,00"
            :disabled="!canEdit || saving"
          />
        </label>
        <label>
          <span>Teto total</span>
          <input
            v-model="lifetimeBudget"
            inputmode="decimal"
            placeholder="Ex.: 3000,00"
            :disabled="!canEdit || saving"
          />
        </label>
      </div>

      <div class="policy-card__toggles">
        <label>
          <input v-model="allowCreate" type="checkbox" :disabled="!canEdit || saving" />
          Criar campanhas e promover posts
        </label>
        <label>
          <input v-model="allowDuplicate" type="checkbox" :disabled="!canEdit || saving" />
          Duplicar campanhas
        </label>
        <label>
          <input v-model="allowResume" type="checkbox" :disabled="!canEdit || saving" />
          Reativar campanhas
        </label>
      </div>

      <p class="policy-card__notice">
        Pausar reduz gasto e nao depende destes toggles. Criar, duplicar e promover um post sempre
        nascem pausados. A promoção cria campanha, conjunto, criativo e anúncio com recibo separado
        por etapa; reativar só fica disponível quando o backend comprova o orçamento CBO atual
        dentro do teto. As escritas continuam bloqueadas globalmente até a homologação da conexão
        Meta.
      </p>
      <p v-if="!canEdit" class="policy-card__notice">
        Politica herdada em modo leitura. Somente a agencia com permissao de gestao pode altera-la.
      </p>
      <p v-if="error" class="policy-card__error">{{ error }}</p>
      <p v-if="saved" class="policy-card__success">Politica salva.</p>

      <button v-if="canEdit" type="submit" :disabled="saving">
        {{ saving ? 'Salvando...' : 'Salvar politica' }}
      </button>
    </form>

    <p v-else-if="error" class="policy-card__error">{{ error }}</p>
  </article>
</template>

<style scoped>
.policy-card {
  display: grid;
  gap: 1rem;
  padding: 1.1rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.9rem;
  background: rgb(var(--surface-2) / 0.42);
}

.policy-card__header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}

.policy-card__header h3 {
  margin: 0.15rem 0 0.25rem;
  font-size: 1rem;
}

.policy-card__header p,
.policy-card__notice,
.policy-card__muted {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.8rem;
}

.policy-card__eyebrow {
  font-size: 0.66rem !important;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.policy-card__currency {
  align-self: flex-start;
  padding: 0.3rem 0.5rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
  font-size: 0.72rem;
  font-weight: 800;
}

.policy-card__form {
  display: grid;
  gap: 0.9rem;
}

.policy-card__budgets {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.policy-card__budgets label {
  display: grid;
  gap: 0.35rem;
  color: var(--text-muted);
  font-size: 0.75rem;
  font-weight: 700;
}

.policy-card__budgets input {
  width: 100%;
  padding: 0.62rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.55rem;
  background: rgb(var(--surface-1) / 0.7);
  color: var(--text-main);
}

.policy-card__toggles {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.55rem;
}

.policy-card__toggles label {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.78rem;
}

.policy-card__error,
.policy-card__success {
  margin: 0;
  font-size: 0.78rem;
}

.policy-card__error {
  color: var(--danger, #dc2626);
}

.policy-card__success {
  color: var(--success, #15803d);
}

.policy-card button {
  justify-self: start;
  padding: 0.58rem 0.85rem;
  border: 0;
  border-radius: 0.55rem;
  background: rgb(var(--primary));
  color: white;
  font: inherit;
  font-size: 0.8rem;
  font-weight: 800;
  cursor: pointer;
}

.policy-card button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

@media (max-width: 720px) {
  .policy-card__budgets,
  .policy-card__toggles {
    grid-template-columns: 1fr;
  }
}
</style>
