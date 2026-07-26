<script setup>
import { computed, ref, watch } from 'vue'

import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { canManageCampaigns } from '~/domain/utils/permissions'
import {
  buildCampaignPerformance,
  deriveCampaignStatus,
  normalizeCampaign,
} from '~/domain/utils/campaigns'
import { useAuthStore } from '~/stores/auth'
import { useCampaignsStore } from '~/stores/campaigns'
import { useUiStore } from '~/stores/ui'
import CampaignEditorDrawer from './CampaignEditorDrawer.vue'

const props = defineProps({
  state: { type: Object, required: true },
  campaignType: { type: String, required: true },
  integratedScope: { type: Boolean, default: false },
  integratedHistory: { type: Array, default: () => [] },
  integratedPending: { type: Boolean, default: false },
  integratedError: { type: String, default: '' },
  stores: { type: Array, default: () => [] },
})

const auth = useAuthStore()
const campaignsStore = useCampaignsStore()
const ui = useUiStore()
const selectedStatus = ref('all')
const selectedStoreId = ref('all')
const editorOpen = ref(false)
const selectedItem = ref(null)
const saving = ref(false)

const canEdit = computed(() =>
  canManageCampaigns(auth.role, auth.effectivePermissionKeys, auth.permissionsResolved),
)
const entityCopy = computed(() =>
  props.campaignType === 'interna'
    ? {
        singular: 'corridinha',
        plural: 'Corridinhas',
        description: 'Incentivos internos com regras e premiações aplicadas aos atendimentos.',
        empty: 'Nenhuma corridinha encontrada',
      }
    : {
        singular: 'campanha',
        plural: 'Campanhas comerciais',
        description: 'Ações comerciais e promocionais aplicadas automaticamente no fechamento.',
        empty: 'Nenhuma campanha comercial encontrada',
      },
)
const campaigns = computed(() =>
  (props.state.campaigns || []).filter(
    (campaign) => (campaign.campaignType || 'interna') === props.campaignType,
  ),
)
const storeOptions = computed(() => [
  { value: 'all', label: 'Todas as lojas' },
  ...(props.stores || []).map((store) => ({
    value: String(store?.id || '').trim(),
    label: String(store?.name || store?.code || 'Loja').trim(),
  })),
])
const statusOptions = [
  { value: 'all', label: 'Todos os status' },
  { value: 'ativa', label: 'Em andamento' },
  { value: 'aguardando', label: 'Agendadas' },
  { value: 'encerrada', label: 'Encerradas' },
  { value: 'inativa', label: 'Desativadas' },
]
const historyEntries = computed(() =>
  props.integratedScope ? props.integratedHistory || [] : props.state.serviceHistory || [],
)
const scopedHistory = computed(() => {
  if (!props.integratedScope || selectedStoreId.value === 'all') return historyEntries.value
  return historyEntries.value.filter(
    (entry) => String(entry?.storeId || '').trim() === selectedStoreId.value,
  )
})
const statsByCampaign = computed(() => {
  const result = new Map(campaigns.value.map((campaign) => [campaign.id, { hits: 0, bonus: 0 }]))
  scopedHistory.value.forEach((entry) => {
    ;(Array.isArray(entry?.campaignMatches) ? entry.campaignMatches : []).forEach((match) => {
      const current = result.get(match.campaignId)
      if (!current) return
      current.hits += 1
      current.bonus += Number(match.bonusValue || 0)
    })
  })
  return result
})
const performance = computed(() => buildCampaignPerformance(campaigns.value, scopedHistory.value))
const filteredCampaigns = computed(() =>
  campaigns.value.filter(
    (campaign) =>
      selectedStatus.value === 'all' || deriveCampaignStatus(campaign) === selectedStatus.value,
  ),
)
const totalHits = computed(() =>
  [...statsByCampaign.value.values()].reduce((sum, item) => sum + item.hits, 0),
)
const totalBonus = computed(() =>
  [...statsByCampaign.value.values()].reduce((sum, item) => sum + item.bonus, 0),
)
const productOptions = computed(() =>
  (props.state.productCatalog || [])
    .filter((product) => String(product?.code || '').trim())
    .map((product) => ({
      value: String(product.code).trim().toUpperCase(),
      label: `${product.name} (${String(product.code).trim().toUpperCase()})`,
    })),
)

function formatCurrency(value) {
  return new Intl.NumberFormat('pt-BR', {
    style: 'currency',
    currency: 'BRL',
  }).format(Number(value || 0))
}

function formatPeriod(campaign) {
  if (!campaign.startsAt && !campaign.endsAt) return 'Sem período definido'
  const format = (value) => {
    if (!value) return 'livre'
    const [year, month, day] = String(value).split('-')
    return year && month && day ? `${day}/${month}/${year}` : value
  }
  return `${format(campaign.startsAt)} até ${format(campaign.endsAt)}`
}

function statusLabel(campaign) {
  return {
    ativa: 'Em andamento',
    aguardando: 'Agendada',
    encerrada: 'Encerrada',
    inativa: 'Desativada',
  }[deriveCampaignStatus(campaign)]
}

function ruleSummary(campaign) {
  const rules = []
  if (campaign.minSaleAmount > 0)
    rules.push(`Venda mínima ${formatCurrency(campaign.minSaleAmount)}`)
  if (campaign.maxServiceMinutes > 0) rules.push(`Até ${campaign.maxServiceMinutes} min`)
  if (campaign.queueJumpOnly) rules.push('Fora da vez')
  if (campaign.productCodes?.length) rules.push(`${campaign.productCodes.length} produto(s)`)
  return rules.length ? rules.join(' · ') : 'Todos os atendimentos elegíveis'
}

function rewardSummary(campaign) {
  const rewards = []
  if (campaign.bonusFixed > 0) rewards.push(formatCurrency(campaign.bonusFixed))
  if (campaign.bonusRate > 0) rewards.push(`${(campaign.bonusRate * 100).toFixed(1)}% da venda`)
  return rewards.length ? rewards.join(' + ') : 'Sem premiação configurada'
}

function openCreate() {
  selectedItem.value = null
  editorOpen.value = true
}

function openEdit(campaign) {
  selectedItem.value = campaign
  editorOpen.value = true
}

async function saveCampaign(payload) {
  saving.value = true
  try {
    const result = selectedItem.value
      ? await campaignsStore.updateCampaign(
          selectedItem.value.id,
          normalizeCampaign({ ...payload, campaignType: props.campaignType }),
        )
      : await campaignsStore.createCampaign(
          normalizeCampaign({ ...payload, campaignType: props.campaignType }),
        )
    if (result?.ok === false) {
      ui.error(result.message || `Não foi possível salvar a ${entityCopy.value.singular}.`)
      return
    }
    editorOpen.value = false
    selectedItem.value = null
    ui.success(`${entityCopy.value.singular === 'campanha' ? 'Campanha' : 'Corridinha'} salva.`)
  } finally {
    saving.value = false
  }
}

async function removeCampaign(campaign) {
  const { confirmed } = await ui.confirm({
    title: `Excluir ${entityCopy.value.singular}`,
    message: `“${campaign.name}” será removida da configuração atual. Deseja continuar?`,
    confirmLabel: 'Excluir',
  })
  if (!confirmed) return
  await campaignsStore.removeCampaign(campaign.id)
  ui.success(`${entityCopy.value.singular === 'campanha' ? 'Campanha' : 'Corridinha'} removida.`)
}

watch(
  () => storeOptions.value.map((option) => option.value).join('|'),
  () => {
    if (!storeOptions.value.some((option) => option.value === selectedStoreId.value)) {
      selectedStoreId.value = 'all'
    }
  },
)
</script>

<template>
  <section class="campaign-list" data-testid="campaigns-panel">
    <div class="campaign-list__heading">
      <div>
        <h2>{{ entityCopy.plural }}</h2>
        <p>{{ entityCopy.description }}</p>
      </div>
      <AppPanelButton v-if="canEdit" @click="openCreate">
        <UIcon name="i-lucide-plus" aria-hidden="true" />
        Nova {{ entityCopy.singular }}
      </AppPanelButton>
    </div>

    <div class="campaign-list__toolbar">
      <AppSelectField
        v-if="integratedScope"
        v-model="selectedStoreId"
        class="campaign-list__filter"
        label="Loja"
        :options="storeOptions"
      />
      <AppSelectField
        v-model="selectedStatus"
        class="campaign-list__filter"
        label="Status"
        :options="statusOptions"
      />
      <div class="campaign-list__summary" data-testid="campaigns-summary">
        <span>
          <strong>{{ campaigns.length }}</strong>
          cadastradas
        </span>
        <span>
          <strong>{{ totalHits }}</strong>
          aplicações
        </span>
        <span>
          <strong>{{ formatCurrency(totalBonus) }}</strong>
          em bônus
        </span>
      </div>
    </div>

    <div v-if="integratedError" class="campaign-list__state is-error">
      <UIcon name="i-lucide-circle-alert" />
      <strong>{{ integratedError }}</strong>
    </div>
    <div
      v-else-if="integratedScope && integratedPending && !historyEntries.length"
      class="campaign-list__state"
    >
      <UIcon class="campaign-list__spin" name="i-lucide-loader-circle" />
      <strong>Carregando histórico consolidado…</strong>
    </div>
    <div v-else-if="filteredCampaigns.length === 0" class="campaign-list__state">
      <UIcon :name="campaignType === 'interna' ? 'i-lucide-trophy' : 'i-lucide-badge-percent'" />
      <strong>{{ entityCopy.empty }}</strong>
      <p v-if="canEdit">Crie o primeiro item ou ajuste o filtro de status.</p>
    </div>

    <div v-else class="campaign-grid">
      <article v-for="campaign in filteredCampaigns" :key="campaign.id" class="campaign-card">
        <header class="campaign-card__header">
          <div>
            <h3>{{ campaign.name }}</h3>
            <span class="campaign-card__status" :class="`is-${deriveCampaignStatus(campaign)}`">
              {{ statusLabel(campaign) }}
            </span>
          </div>
          <span class="campaign-card__reward">
            <UIcon :name="campaignType === 'interna' ? 'i-lucide-trophy' : 'i-lucide-gift'" />
            {{ rewardSummary(campaign) }}
          </span>
        </header>

        <p class="campaign-card__description">
          {{ campaign.description || 'Sem descrição adicional.' }}
        </p>

        <dl class="campaign-card__meta">
          <div>
            <dt>
              <UIcon name="i-lucide-calendar-range" />
              Período
            </dt>
            <dd>{{ formatPeriod(campaign) }}</dd>
          </div>
          <div>
            <dt>
              <UIcon name="i-lucide-list-checks" />
              Regras
            </dt>
            <dd>{{ ruleSummary(campaign) }}</dd>
          </div>
        </dl>

        <div class="campaign-card__numbers">
          <span>
            <strong>{{ statsByCampaign.get(campaign.id)?.hits || 0 }}</strong>
            aplicações
          </span>
          <span>
            <strong>{{ formatCurrency(statsByCampaign.get(campaign.id)?.bonus || 0) }}</strong>
            acumulado
          </span>
          <span>
            <strong>
              {{ performance.get(campaign.id)?.hit?.conversionRate.toFixed(1) || '0.0' }}%
            </strong>
            conversão
          </span>
        </div>

        <footer v-if="canEdit" class="campaign-card__actions">
          <AppPanelButton variant="ghost" @click="openEdit(campaign)">
            <UIcon name="i-lucide-pencil" />
            Editar
          </AppPanelButton>
          <AppPanelButton variant="ghost" @click="removeCampaign(campaign)">
            <UIcon name="i-lucide-trash-2" />
            Excluir
          </AppPanelButton>
        </footer>
      </article>
    </div>

    <CampaignEditorDrawer
      v-model:open="editorOpen"
      :item="selectedItem"
      :campaign-type="campaignType"
      :customer-source-options="state.customerSourceOptions || []"
      :visit-reason-options="state.visitReasonOptions || []"
      :product-options="productOptions"
      :saving="saving"
      @save="saveCampaign"
    />
  </section>
</template>

<style scoped src="~/assets/styles/components/campaign-workspace.css"></style>
