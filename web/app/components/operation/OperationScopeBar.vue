<script setup>
import { computed } from 'vue'
import { deriveCampaignStatus } from '~/domain/utils/campaigns'

// Apos mover o filtro de loja para o nav (DashboardWorkspaceNav), esta barra
// cuida apenas do destaque de campanha comercial ativa na operacao.
const props = defineProps({
  state: {
    type: Object,
    required: true,
  },
})

function formatPeriodLabel(startsAt, endsAt) {
  if (startsAt && endsAt) {
    return `${startsAt} ate ${endsAt}`
  }

  if (startsAt) {
    return `A partir de ${startsAt}`
  }

  if (endsAt) {
    return `Ate ${endsAt}`
  }

  return ''
}

const activeCommercialCampaigns = computed(() =>
  (props.state.campaigns || [])
    .filter((campaign) => (campaign.campaignType || 'interna') === 'comercial')
    .filter((campaign) => deriveCampaignStatus(campaign) === 'ativa')
    .map((campaign) => ({
      ...campaign,
      periodLabel: formatPeriodLabel(campaign.startsAt, campaign.endsAt),
    })),
)
const primaryCampaign = computed(() => activeCommercialCampaigns.value[0] || null)
const campaignHeadline = computed(() => {
  const activeCount = activeCommercialCampaigns.value.length

  if (!activeCount) {
    return ''
  }

  if (activeCount === 1) {
    return `Campanha ativa: ${primaryCampaign.value?.name || 'Campanha comercial'}`
  }

  return `${activeCount} campanhas comerciais ativas`
})
const campaignSubline = computed(() => {
  if (!primaryCampaign.value) {
    return ''
  }

  if (activeCommercialCampaigns.value.length === 1 && primaryCampaign.value.periodLabel) {
    return primaryCampaign.value.periodLabel
  }

  return 'Abra os detalhes para consultar regras, produtos e metas.'
})
const showCampaign = computed(() => activeCommercialCampaigns.value.length > 0)
</script>

<template>
  <section v-if="showCampaign" class="operation-scope-bar">
    <div class="operation-scope-bar__campaign">
      <div class="operation-scope-bar__campaign-accent" aria-hidden="true"></div>
      <div class="operation-scope-bar__campaign-content">
        <strong class="operation-scope-bar__campaign-headline">{{ campaignHeadline }}</strong>
        <span v-if="campaignSubline" class="operation-scope-bar__campaign-subline">
          {{ campaignSubline }}
        </span>
      </div>
      <NuxtLink to="/campanhas" class="operation-scope-bar__campaign-action">Ver campanha</NuxtLink>
    </div>
  </section>
</template>

<style scoped>
.operation-scope-bar {
  display: flex;
  align-items: center;
  gap: 0.9rem;
}

.operation-scope-bar__campaign {
  display: flex;
  align-items: center;
  gap: 0.8rem;
  min-width: 0;
  flex: 1;
}

.operation-scope-bar__campaign-accent {
  width: 4px;
  align-self: stretch;
  flex-shrink: 0;
  border-radius: 999px;
  background: linear-gradient(180deg, rgb(var(--primary)) 0%, rgb(var(--ring)) 100%);
}

.operation-scope-bar__campaign-content {
  display: grid;
  gap: 0.1rem;
  min-width: 0;
  flex: 1;
}

.operation-scope-bar__campaign-headline {
  color: var(--text-main);
  font-size: 0.84rem;
  line-height: 1.25;
}

.operation-scope-bar__campaign-subline {
  color: var(--text-muted);
  font-size: 0.72rem;
  line-height: 1.2;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.operation-scope-bar__campaign-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  padding: 0.5rem 0.8rem;
  border: 1px solid rgb(var(--primary) / 0.28);
  border-radius: 10px;
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary-600));
  font-size: 0.73rem;
  font-weight: 700;
  text-decoration: none;
  transition:
    border-color 0.18s ease,
    background 0.18s ease,
    color 0.18s ease;
}

.operation-scope-bar__campaign-action:hover {
  border-color: rgb(var(--primary) / 0.48);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

@media (max-width: 900px) {
  .operation-scope-bar {
    flex-direction: column;
    align-items: stretch;
  }

  .operation-scope-bar__campaign {
    align-items: flex-start;
    flex-wrap: wrap;
  }
}
</style>
