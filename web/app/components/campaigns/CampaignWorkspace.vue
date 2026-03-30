<script setup>
import { computed, reactive, ref, watch } from "vue";
import { canManageCampaigns } from "@core/utils/permissions";
import { buildCampaignPerformance, deriveCampaignStatus, normalizeCampaign } from "@core/utils/campaigns";
import { useDashboardStore } from "~/stores/dashboard";
import { useUiStore } from "~/stores/ui";

const props = defineProps({
  state: {
    type: Object,
    required: true
  }
});

const dashboard = useDashboardStore();
const ui = useUiStore();
const drafts = ref({});
const newCampaign = reactive(normalizeCampaign({}));
const typeFilter = ref("todas");

function formatCurrency(value) {
  return new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" }).format(Number(value || 0));
}

const activeRole = computed(() => {
  const activeProfile =
    (props.state.profiles || []).find((profile) => profile.id === props.state.activeProfileId) ||
    props.state.profiles?.[0] ||
    null;

  return activeProfile?.role || "consultant";
});
const canEditCampaigns = computed(() => canManageCampaigns(activeRole.value));
const campaignStats = computed(() => {
  const statsByCampaignId = new Map((props.state.campaigns || []).map((campaign) => [campaign.id, { hits: 0, bonus: 0 }]));

  (props.state.serviceHistory || []).forEach((entry) => {
    const matches = Array.isArray(entry.campaignMatches) ? entry.campaignMatches : [];

    matches.forEach((match) => {
      const current = statsByCampaignId.get(match.campaignId);

      if (!current) {
        return;
      }

      current.hits += 1;
      current.bonus += Number(match.bonusValue || 0);
    });
  });

  return statsByCampaignId;
});
const totalBonus = computed(() =>
  [...campaignStats.value.values()].reduce((sum, item) => sum + Number(item.bonus || 0), 0)
);
const totalHits = computed(() =>
  [...campaignStats.value.values()].reduce((sum, item) => sum + Number(item.hits || 0), 0)
);
const activeCampaignCount = computed(() =>
  (props.state.campaigns || []).filter((campaign) => campaign.isActive).length
);
const filteredCampaigns = computed(() => {
  const campaigns = props.state.campaigns || [];
  if (typeFilter.value === "todas") return campaigns;
  return campaigns.filter((c) => (c.campaignType || "interna") === typeFilter.value);
});
const performance = computed(() =>
  buildCampaignPerformance(props.state.campaigns || [], props.state.serviceHistory || [])
);

const STATUS_LABEL = { ativa: "Em andamento", aguardando: "Aguardando", encerrada: "Encerrada", inativa: "Desativada" };
const STATUS_CLASS = { ativa: "campaign-status--ativa", aguardando: "campaign-status--aguardando", encerrada: "campaign-status--encerrada", inativa: "campaign-status--inativa" };

function statusOf(campaign) {
  return deriveCampaignStatus(campaign);
}

function buildDraft(campaign) {
  return normalizeCampaign({
    ...campaign,
    sourceIds: [...(campaign.sourceIds || [])],
    reasonIds: [...(campaign.reasonIds || [])]
  });
}

watch(
  () => props.state.campaigns,
  (campaigns) => {
    drafts.value = Object.fromEntries((campaigns || []).map((campaign) => [campaign.id, buildDraft(campaign)]));
  },
  { immediate: true, deep: true }
);

function updateDraftField(campaignId, field, value) {
  if (!drafts.value[campaignId]) {
    return;
  }

  drafts.value[campaignId] = {
    ...drafts.value[campaignId],
    [field]: value
  };
}

function toggleDraftListValue(campaignId, field, value) {
  const currentValues = drafts.value[campaignId]?.[field] || [];
  const nextValues = currentValues.includes(value)
    ? currentValues.filter((item) => item !== value)
    : [...currentValues, value];

  updateDraftField(campaignId, field, nextValues);
}

function updateNewCampaignField(field, value) {
  newCampaign[field] = value;
}

function toggleNewCampaignListValue(field, value) {
  const currentValues = newCampaign[field] || [];
  newCampaign[field] = currentValues.includes(value)
    ? currentValues.filter((item) => item !== value)
    : [...currentValues, value];
}

async function saveCampaign(campaignId) {
  const result = await dashboard.updateCampaign(campaignId, normalizeCampaign(drafts.value[campaignId]));

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel atualizar campanha.");
    return;
  }

  ui.success("Campanha atualizada.");
}

async function createCampaign() {
  const result = await dashboard.createCampaign(normalizeCampaign(newCampaign));

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel criar campanha.");
    return;
  }

  Object.assign(newCampaign, normalizeCampaign({}));
  ui.success("Campanha criada.");
}

async function removeCampaign(campaignId) {
  const { confirmed } = await ui.confirm({
    title: "Excluir campanha",
    message: "Essa campanha sera removida da configuracao atual. Deseja continuar?",
    confirmLabel: "Excluir"
  });

  if (!confirmed) {
    return;
  }

  await dashboard.removeCampaign(campaignId);
  ui.success("Campanha removida.");
}
</script>

<template>
  <section class="admin-panel" data-testid="campaigns-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Campanhas e regras comerciais</h2>
      <p class="admin-panel__text">Regras aplicadas automaticamente no fechamento para auditoria e bonus.</p>
    </header>

    <section class="metric-grid metric-grid--tight" data-testid="campaigns-summary">
      <article class="metric-card"><span class="metric-card__label">Campanhas cadastradas</span><strong class="metric-card__value">{{ state.campaigns.length }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Campanhas ativas</span><strong class="metric-card__value">{{ activeCampaignCount }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Aplicacoes no historico</span><strong class="metric-card__value">{{ totalHits }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Bonus acumulado</span><strong class="metric-card__value">{{ formatCurrency(totalBonus) }}</strong></article>
    </section>

    <div class="campaign-type-filter">
      <button data-testid="campaigns-filter-todas" :class="['campaign-type-filter__btn', typeFilter === 'todas' && 'is-active']" type="button" @click="typeFilter = 'todas'">Todas</button>
      <button data-testid="campaigns-filter-interna" :class="['campaign-type-filter__btn', typeFilter === 'interna' && 'is-active']" type="button" @click="typeFilter = 'interna'">Internas</button>
      <button data-testid="campaigns-filter-comercial" :class="['campaign-type-filter__btn', typeFilter === 'comercial' && 'is-active']" type="button" @click="typeFilter = 'comercial'">Comerciais</button>
    </div>

    <form v-if="canEditCampaigns" class="settings-card campaign-card" data-testid="campaigns-new-form" @submit.prevent="createCampaign">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Nova campanha</h3>
        <p class="settings-card__text">Cadastro completo da regra comercial.</p>
      </header>

      <div class="campaign-grid">
        <label class="settings-field"><span>Nome</span><input v-model="newCampaign.name" type="text" data-testid="campaigns-new-name"></label>
        <label class="settings-field"><span>Descricao</span><input v-model="newCampaign.description" type="text"></label>
        <label class="settings-field">
          <span>Tipo</span>
          <select :value="newCampaign.campaignType" @change="updateNewCampaignField('campaignType', $event.target.value)">
            <option value="interna">Interna (corrida / incentivo)</option>
            <option value="comercial">Comercial (marketing / promocao)</option>
          </select>
        </label>
        <label class="settings-field"><span>Inicio</span><input v-model="newCampaign.startsAt" type="date"></label>
        <label class="settings-field"><span>Fim</span><input v-model="newCampaign.endsAt" type="date"></label>
        <label class="settings-field">
          <span>Desfecho alvo</span>
          <select :value="newCampaign.targetOutcome" @change="updateNewCampaignField('targetOutcome', $event.target.value)">
            <option value="compra-reserva">Compra ou reserva</option>
            <option value="compra">Compra</option>
            <option value="reserva">Reserva</option>
            <option value="nao-compra">Nao compra</option>
            <option value="qualquer">Qualquer desfecho</option>
          </select>
        </label>
        <label class="settings-field">
          <span>Cliente recorrente</span>
          <select :value="newCampaign.existingCustomerFilter" @change="updateNewCampaignField('existingCustomerFilter', $event.target.value)">
            <option value="all">Todos</option>
            <option value="yes">Somente sim</option>
            <option value="no">Somente nao</option>
          </select>
        </label>
        <label class="settings-field"><span>Venda minima (R$)</span><input :value="newCampaign.minSaleAmount" type="number" min="0" step="1" @input="updateNewCampaignField('minSaleAmount', $event.target.value)"></label>
        <label class="settings-field"><span>Duracao maxima (min)</span><input :value="newCampaign.maxServiceMinutes" type="number" min="0" step="1" @input="updateNewCampaignField('maxServiceMinutes', $event.target.value)"></label>
        <label class="settings-field"><span>Bonus fixo (R$)</span><input :value="newCampaign.bonusFixed" type="number" min="0" step="0.01" @input="updateNewCampaignField('bonusFixed', $event.target.value)"></label>
        <label class="settings-field"><span>Bonus percentual</span><input :value="newCampaign.bonusRate" type="number" min="0" max="1" step="0.001" @input="updateNewCampaignField('bonusRate', $event.target.value)"></label>
      </div>

      <div class="campaign-grid campaign-grid--toggles">
        <label class="settings-toggle"><input :checked="newCampaign.isActive" type="checkbox" @change="updateNewCampaignField('isActive', $event.target.checked)"><span>Campanha ativa</span></label>
        <label class="settings-toggle"><input :checked="newCampaign.queueJumpOnly" type="checkbox" @change="updateNewCampaignField('queueJumpOnly', $event.target.checked)"><span>Somente fora da vez</span></label>
      </div>

      <div class="campaign-grid campaign-grid--options">
        <div class="settings-field">
          <span>Origens alvo</span>
          <div class="campaign-option-list">
            <label v-for="option in state.customerSourceOptions" :key="option.id" class="settings-toggle">
              <input type="checkbox" :checked="newCampaign.sourceIds.includes(option.id)" @change="toggleNewCampaignListValue('sourceIds', option.id)">
              <span>{{ option.label }}</span>
            </label>
          </div>
        </div>
        <div class="settings-field">
          <span>Motivos alvo</span>
          <div class="campaign-option-list">
            <label v-for="option in state.visitReasonOptions" :key="option.id" class="settings-toggle">
              <input type="checkbox" :checked="newCampaign.reasonIds.includes(option.id)" @change="toggleNewCampaignListValue('reasonIds', option.id)">
              <span>{{ option.label }}</span>
            </label>
          </div>
        </div>
      </div>

      <div class="report-actions">
        <button class="option-add__button" type="submit" data-testid="campaigns-new-submit">Criar campanha</button>
      </div>
    </form>

    <div class="settings-grid campaign-list" data-testid="campaigns-list">
      <article v-if="!filteredCampaigns.length" class="settings-card">
        <span class="insight-empty">{{ state.campaigns.length ? 'Nenhuma campanha nessa categoria.' : 'Nenhuma campanha cadastrada.' }}</span>
      </article>

      <form
        v-for="campaign in filteredCampaigns"
        :key="campaign.id"
        class="settings-card campaign-card"
        @submit.prevent="saveCampaign(campaign.id)"
      >
        <header class="settings-card__header">
          <div class="campaign-card__title-row">
            <h3 class="settings-card__title">{{ campaign.name || "Campanha sem nome" }}</h3>
            <span :class="['campaign-status', STATUS_CLASS[statusOf(campaign)]]">{{ STATUS_LABEL[statusOf(campaign)] }}</span>
            <span class="insight-tag insight-tag--sm">{{ campaign.campaignType === 'comercial' ? 'Comercial' : 'Interna' }}</span>
          </div>
          <p class="settings-card__text">{{ campaign.description || "Sem descricao" }}</p>
        </header>

        <div class="insight-time-grid">
          <span class="insight-tag">Aplicacoes: <strong>{{ campaignStats.get(campaign.id)?.hits || 0 }}</strong></span>
          <span class="insight-tag">Bonus total: <strong>{{ formatCurrency(campaignStats.get(campaign.id)?.bonus || 0) }}</strong></span>
        </div>

        <template v-if="performance.get(campaign.id)?.hasPeriod">
          <div class="campaign-perf">
            <div class="campaign-perf__col campaign-perf__col--hit">
              <span class="campaign-perf__label">Dentro da campanha</span>
              <strong class="campaign-perf__value">{{ performance.get(campaign.id).hit.total }} atend.</strong>
              <span class="campaign-perf__sub">{{ performance.get(campaign.id).hit.conversionRate.toFixed(1) }}% conv. · {{ formatCurrency(performance.get(campaign.id).hit.ticketAverage) }} ticket</span>
            </div>
            <div class="campaign-perf__divider">vs</div>
            <div class="campaign-perf__col campaign-perf__col--nohit">
              <span class="campaign-perf__label">Fora da campanha (mesmo período)</span>
              <strong class="campaign-perf__value">{{ performance.get(campaign.id).nonHit.total }} atend.</strong>
              <span class="campaign-perf__sub">{{ performance.get(campaign.id).nonHit.conversionRate.toFixed(1) }}% conv. · {{ formatCurrency(performance.get(campaign.id).nonHit.ticketAverage) }} ticket</span>
            </div>
          </div>
        </template>

        <div class="campaign-grid">
          <label class="settings-field"><span>Nome</span><input :value="drafts[campaign.id]?.name || ''" type="text" :disabled="!canEditCampaigns" @input="updateDraftField(campaign.id, 'name', $event.target.value)"></label>
          <label class="settings-field"><span>Descricao</span><input :value="drafts[campaign.id]?.description || ''" type="text" :disabled="!canEditCampaigns" @input="updateDraftField(campaign.id, 'description', $event.target.value)"></label>
          <label class="settings-field">
            <span>Tipo</span>
            <select :value="drafts[campaign.id]?.campaignType || 'interna'" :disabled="!canEditCampaigns" @change="updateDraftField(campaign.id, 'campaignType', $event.target.value)">
              <option value="interna">Interna (corrida / incentivo)</option>
              <option value="comercial">Comercial (marketing / promocao)</option>
            </select>
          </label>
          <label class="settings-field"><span>Inicio</span><input :value="drafts[campaign.id]?.startsAt || ''" type="date" :disabled="!canEditCampaigns" @input="updateDraftField(campaign.id, 'startsAt', $event.target.value)"></label>
          <label class="settings-field"><span>Fim</span><input :value="drafts[campaign.id]?.endsAt || ''" type="date" :disabled="!canEditCampaigns" @input="updateDraftField(campaign.id, 'endsAt', $event.target.value)"></label>
          <label class="settings-field">
            <span>Desfecho alvo</span>
            <select :value="drafts[campaign.id]?.targetOutcome || 'compra-reserva'" :disabled="!canEditCampaigns" @change="updateDraftField(campaign.id, 'targetOutcome', $event.target.value)">
              <option value="compra-reserva">Compra ou reserva</option>
              <option value="compra">Compra</option>
              <option value="reserva">Reserva</option>
              <option value="nao-compra">Nao compra</option>
              <option value="qualquer">Qualquer desfecho</option>
            </select>
          </label>
          <label class="settings-field">
            <span>Cliente recorrente</span>
            <select :value="drafts[campaign.id]?.existingCustomerFilter || 'all'" :disabled="!canEditCampaigns" @change="updateDraftField(campaign.id, 'existingCustomerFilter', $event.target.value)">
              <option value="all">Todos</option>
              <option value="yes">Somente sim</option>
              <option value="no">Somente nao</option>
            </select>
          </label>
          <label class="settings-field"><span>Venda minima (R$)</span><input :value="drafts[campaign.id]?.minSaleAmount || 0" type="number" min="0" step="1" :disabled="!canEditCampaigns" @input="updateDraftField(campaign.id, 'minSaleAmount', $event.target.value)"></label>
          <label class="settings-field"><span>Duracao maxima (min)</span><input :value="drafts[campaign.id]?.maxServiceMinutes || 0" type="number" min="0" step="1" :disabled="!canEditCampaigns" @input="updateDraftField(campaign.id, 'maxServiceMinutes', $event.target.value)"></label>
          <label class="settings-field"><span>Bonus fixo (R$)</span><input :value="drafts[campaign.id]?.bonusFixed || 0" type="number" min="0" step="0.01" :disabled="!canEditCampaigns" @input="updateDraftField(campaign.id, 'bonusFixed', $event.target.value)"></label>
          <label class="settings-field"><span>Bonus percentual</span><input :value="drafts[campaign.id]?.bonusRate || 0" type="number" min="0" max="1" step="0.001" :disabled="!canEditCampaigns" @input="updateDraftField(campaign.id, 'bonusRate', $event.target.value)"></label>
        </div>

        <div class="campaign-grid campaign-grid--toggles">
          <label class="settings-toggle"><input type="checkbox" :checked="Boolean(drafts[campaign.id]?.isActive)" :disabled="!canEditCampaigns" @change="updateDraftField(campaign.id, 'isActive', $event.target.checked)"><span>Campanha ativa</span></label>
          <label class="settings-toggle"><input type="checkbox" :checked="Boolean(drafts[campaign.id]?.queueJumpOnly)" :disabled="!canEditCampaigns" @change="updateDraftField(campaign.id, 'queueJumpOnly', $event.target.checked)"><span>Somente fora da vez</span></label>
        </div>

        <div class="campaign-grid campaign-grid--options">
          <div class="settings-field">
            <span>Origens alvo</span>
            <div class="campaign-option-list">
              <label v-for="option in state.customerSourceOptions" :key="option.id" class="settings-toggle">
                <input type="checkbox" :checked="Boolean(drafts[campaign.id]?.sourceIds?.includes(option.id))" :disabled="!canEditCampaigns" @change="toggleDraftListValue(campaign.id, 'sourceIds', option.id)">
                <span>{{ option.label }}</span>
              </label>
            </div>
          </div>
          <div class="settings-field">
            <span>Motivos alvo</span>
            <div class="campaign-option-list">
              <label v-for="option in state.visitReasonOptions" :key="option.id" class="settings-toggle">
                <input type="checkbox" :checked="Boolean(drafts[campaign.id]?.reasonIds?.includes(option.id))" :disabled="!canEditCampaigns" @change="toggleDraftListValue(campaign.id, 'reasonIds', option.id)">
                <span>{{ option.label }}</span>
              </label>
            </div>
          </div>
        </div>

        <div v-if="canEditCampaigns" class="report-actions">
          <button class="option-row__save" type="submit">Salvar campanha</button>
          <button class="option-row__remove" type="button" @click="removeCampaign(campaign.id)">Excluir campanha</button>
        </div>
      </form>
    </div>
  </section>
</template>
