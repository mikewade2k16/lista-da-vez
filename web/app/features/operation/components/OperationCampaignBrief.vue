<script setup>
import { computed } from "vue";
import { deriveCampaignStatus } from "~/domain/utils/campaigns";

const props = defineProps({
  state: {
    type: Object,
    required: true
  }
});

function formatPeriodLabel(startsAt, endsAt) {
  if (startsAt && endsAt) {
    return `${startsAt} ate ${endsAt}`;
  }

  if (startsAt) {
    return `A partir de ${startsAt}`;
  }

  if (endsAt) {
    return `Ate ${endsAt}`;
  }

  return "";
}

const activeCommercialCampaigns = computed(() =>
  (props.state.campaigns || [])
    .filter((campaign) => (campaign.campaignType || "interna") === "comercial")
    .filter((campaign) => deriveCampaignStatus(campaign) === "ativa")
    .map((campaign) => ({
      ...campaign,
      periodLabel: formatPeriodLabel(campaign.startsAt, campaign.endsAt)
    }))
);

const primaryCampaign = computed(() => activeCommercialCampaigns.value[0] || null);
const headline = computed(() => {
  const activeCount = activeCommercialCampaigns.value.length;

  if (!activeCount) {
    return "";
  }

  if (activeCount === 1) {
    return `Campanha ativa: ${primaryCampaign.value?.name || "Campanha comercial"}`;
  }

  return `${activeCount} campanhas comerciais ativas`;
});

const subline = computed(() => {
  if (!primaryCampaign.value) {
    return "";
  }

  if (activeCommercialCampaigns.value.length === 1 && primaryCampaign.value.periodLabel) {
    return primaryCampaign.value.periodLabel;
  }

  return "Abra os detalhes para consultar regras, produtos e metas.";
});
</script>

<template>
  <section v-if="activeCommercialCampaigns.length" class="operation-campaign-brief" data-testid="operation-campaign-brief">
    <div class="operation-campaign-brief__accent" aria-hidden="true"></div>
    <div class="operation-campaign-brief__content">
      <strong class="operation-campaign-brief__headline">{{ headline }}</strong>
      <span v-if="subline" class="operation-campaign-brief__subline">{{ subline }}</span>
    </div>
    <NuxtLink to="/campanhas" class="operation-campaign-brief__action">
      Ver campanha
    </NuxtLink>
  </section>
</template>
