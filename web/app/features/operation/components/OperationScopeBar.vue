<script setup>
import { computed } from "vue";
import AppSelectField from "~/components/ui/AppSelectField.vue";
import { deriveCampaignStatus } from "~/domain/utils/campaigns";

const props = defineProps({
  state: {
    type: Object,
    required: true
  },
  scopeMode: {
    type: String,
    default: "single"
  },
  stores: {
    type: Array,
    default: () => []
  },
  integratedStoreId: {
    type: String,
    default: ""
  }
});

const emit = defineEmits(["integrated-store-change"]);

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

const storeOptions = computed(() =>
  (Array.isArray(props.stores) ? props.stores : []).map((store) => ({
    value: String(store?.id || "").trim(),
    label: String(store?.name || "").trim(),
    meta: [String(store?.code || "").trim(), String(store?.city || "").trim()].filter(Boolean).join(" · ")
  }))
);

const integratedFilterOptions = computed(() => [
  { value: "", label: "Todas as lojas", meta: "Sem filtro aplicado" },
  ...storeOptions.value
]);
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
const campaignHeadline = computed(() => {
  const activeCount = activeCommercialCampaigns.value.length;

  if (!activeCount) {
    return "";
  }

  if (activeCount === 1) {
    return `Campanha ativa: ${primaryCampaign.value?.name || "Campanha comercial"}`;
  }

  return `${activeCount} campanhas comerciais ativas`;
});
const campaignSubline = computed(() => {
  if (!primaryCampaign.value) {
    return "";
  }

  if (activeCommercialCampaigns.value.length === 1 && primaryCampaign.value.periodLabel) {
    return primaryCampaign.value.periodLabel;
  }

  return "Abra os detalhes para consultar regras, produtos e metas.";
});
const showFilter = computed(() => props.scopeMode === "all");
const showCampaign = computed(() => activeCommercialCampaigns.value.length > 0);
const shouldRenderBar = computed(() => showFilter.value || showCampaign.value);

function handleIntegratedStoreChange(value) {
  emit("integrated-store-change", String(value || "").trim());
}
</script>

<template>
  <section v-if="shouldRenderBar" class="operation-scope-bar">
    <div v-if="showCampaign" class="operation-scope-bar__campaign">
      <div class="operation-scope-bar__campaign-accent" aria-hidden="true"></div>
      <div class="operation-scope-bar__campaign-content">
        <strong class="operation-scope-bar__campaign-headline">{{ campaignHeadline }}</strong>
        <span v-if="campaignSubline" class="operation-scope-bar__campaign-subline">{{ campaignSubline }}</span>
      </div>
      <NuxtLink to="/campanhas" class="operation-scope-bar__campaign-action">Ver campanha</NuxtLink>
    </div>

    <div v-if="showFilter" class="operation-scope-bar__filter">
      <span class="operation-scope-bar__filter-label">Filtro</span>
      <AppSelectField
        class="operation-scope-bar__field"
        :model-value="integratedStoreId"
        :options="integratedFilterOptions"
        placeholder="Todas as lojas"
        :show-leading-icon="false"
        testid="operation-filter-integrated-store"
        @update:model-value="handleIntegratedStoreChange"
      />
    </div>
  </section>
</template>

<style scoped>
.operation-scope-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.9rem;
  padding: 0.85rem 1rem;
  border: 1px solid rgba(125, 146, 255, 0.18);
  border-radius: 1rem;
  background: rgba(10, 16, 32, 0.72);
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
  background: linear-gradient(180deg, #818cf8 0%, #38bdf8 100%);
}

.operation-scope-bar__campaign-content {
  display: grid;
  gap: 0.1rem;
  min-width: 0;
  flex: 1;
}

.operation-scope-bar__campaign-headline {
  color: #eef2ff;
  font-size: 0.84rem;
  line-height: 1.25;
}

.operation-scope-bar__campaign-subline {
  color: #a5b4fc;
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
  border: 1px solid rgba(129, 140, 248, 0.28);
  border-radius: 10px;
  background: rgba(15, 23, 42, 0.78);
  color: #c7d2fe;
  font-size: 0.73rem;
  font-weight: 700;
  text-decoration: none;
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease;
}

.operation-scope-bar__campaign-action:hover {
  border-color: rgba(129, 140, 248, 0.48);
  background: rgba(30, 41, 59, 0.92);
  color: #eef2ff;
}

.operation-scope-bar__filter {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  flex-shrink: 0;
  margin-left: auto;
}

.operation-scope-bar__filter-label {
  color: rgba(219, 226, 255, 0.72);
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.operation-scope-bar__field {
  min-width: 13.5rem;
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

  .operation-scope-bar__filter {
    justify-content: flex-start;
    margin-left: 0;
  }

  .operation-scope-bar__field {
    min-width: 0;
    width: 100%;
  }
}
</style>
