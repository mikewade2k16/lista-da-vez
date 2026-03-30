<script setup>
import { computed, reactive, ref, watch } from "vue";
import { buildOperationalIntelligence, formatCurrencyBRL, formatDurationMinutes, formatPercent } from "@core/utils/admin-metrics";
import { canManageStores } from "@core/utils/permissions";
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
const storeDrafts = ref({});
const newStore = reactive({
  name: "",
  code: "",
  city: "",
  cloneActiveRoster: true,
  defaultTemplateId: "",
  monthlyGoal: "",
  weeklyGoal: "",
  avgTicketGoal: "",
  conversionGoal: "",
  paGoal: ""
});

function createEmptyScopedData() {
  return {
    waitingList: [],
    activeServices: [],
    pausedEmployees: [],
    serviceHistory: [],
    roster: [],
    consultantCurrentStatus: {},
    consultantActivitySessions: []
  };
}

function getStoreSnapshot(snapshotByStoreId, storeId) {
  return {
    ...createEmptyScopedData(),
    ...(snapshotByStoreId?.[storeId] || {})
  };
}

const activeRole = computed(() => {
  const activeProfile =
    (props.state.profiles || []).find((profile) => profile.id === props.state.activeProfileId) ||
    props.state.profiles?.[0] ||
    null;

  return activeProfile?.role || "consultant";
});
const canEditStores = computed(() => canManageStores(activeRole.value));
const operationTemplates = computed(() => props.state.operationTemplates || []);
const snapshotByStoreId = computed(() => ({
  ...(props.state.storeSnapshots || {}),
  [props.state.activeStoreId]: {
    selectedConsultantId: props.state.selectedConsultantId,
    consultantSimulationAdditionalSales: props.state.consultantSimulationAdditionalSales,
    waitingList: props.state.waitingList,
    activeServices: props.state.activeServices,
    roster: props.state.roster,
    consultantActivitySessions: props.state.consultantActivitySessions,
    consultantCurrentStatus: props.state.consultantCurrentStatus,
    pausedEmployees: props.state.pausedEmployees,
    serviceHistory: props.state.serviceHistory
  }
}));
const rows = computed(() =>
  (props.state.stores || [])
    .map((store) => {
      const snapshot = getStoreSnapshot(snapshotByStoreId.value, store.id);
      const history = Array.isArray(snapshot.serviceHistory) ? snapshot.serviceHistory : [];
      const converted = history.filter((entry) => entry.finishOutcome === "compra" || entry.finishOutcome === "reserva");
      const soldValue = converted.reduce((sum, entry) => sum + Number(entry.saleAmount || 0), 0);
      const queueJumpCount = history.filter((entry) => entry.startMode === "queue-jump").length;
      const intelligence = buildOperationalIntelligence({
        history,
        visitReasonOptions: props.state.visitReasonOptions || [],
        customerSourceOptions: props.state.customerSourceOptions || [],
        roster: snapshot.roster || [],
        waitingList: snapshot.waitingList || [],
        activeServices: snapshot.activeServices || [],
        pausedEmployees: snapshot.pausedEmployees || [],
        consultantCurrentStatus: snapshot.consultantCurrentStatus || {},
        consultantActivitySessions: snapshot.consultantActivitySessions || [],
        settings: props.state.settings || {}
      });

      const totalPieces = history.reduce((sum, entry) => {
        return sum + (Array.isArray(entry.productsClosed) ? entry.productsClosed.length : 0);
      }, 0);
      const paScore = history.length ? totalPieces / history.length : 0;

      return {
        storeId: store.id,
        storeName: store.name,
        storeCode: store.code || "-",
        storeCity: store.city || "-",
        consultants: (snapshot.roster || []).length,
        queueCount: (snapshot.waitingList || []).length,
        activeCount: (snapshot.activeServices || []).length,
        pausedCount: (snapshot.pausedEmployees || []).length,
        attendances: history.length,
        conversionRate: intelligence.conversionRate,
        soldValue,
        ticketAverage: intelligence.ticketAverage,
        paScore,
        averageQueueWaitMs: intelligence.time.avgQueueWaitMs,
        queueJumpRate: history.length ? (queueJumpCount / history.length) * 100 : 0,
        healthScore: intelligence.healthScore,
        monthlyGoal: store.monthlyGoal || 0,
        weeklyGoal: store.weeklyGoal || 0,
        avgTicketGoal: store.avgTicketGoal || 0,
        conversionGoal: store.conversionGoal || 0,
        paGoal: store.paGoal || 0,
        defaultTemplateId: store.defaultTemplateId || ""
      };
    })
    .sort((a, b) => {
      if (b.soldValue !== a.soldValue) {
        return b.soldValue - a.soldValue;
      }

      return b.conversionRate - a.conversionRate;
    })
);
const totalAttendances = computed(() => rows.value.reduce((sum, row) => sum + row.attendances, 0));
const totalSoldValue = computed(() => rows.value.reduce((sum, row) => sum + row.soldValue, 0));
const totalQueue = computed(() => rows.value.reduce((sum, row) => sum + row.queueCount, 0));
const totalActiveServices = computed(() => rows.value.reduce((sum, row) => sum + row.activeCount, 0));
const averageHealthScore = computed(() =>
  rows.value.length ? rows.value.reduce((sum, row) => sum + row.healthScore, 0) / rows.value.length : 0
);

watch(
  () => props.state.stores,
  (stores) => {
    storeDrafts.value = Object.fromEntries(
      (stores || []).map((store) => [
        store.id,
        {
          name: store.name,
          code: store.code || "",
          city: store.city || "",
          defaultTemplateId: store.defaultTemplateId || "",
          monthlyGoal: store.monthlyGoal || "",
          weeklyGoal: store.weeklyGoal || "",
          avgTicketGoal: store.avgTicketGoal || "",
          conversionGoal: store.conversionGoal || "",
          paGoal: store.paGoal || ""
        }
      ])
    );
  },
  { immediate: true, deep: true }
);

async function saveStore(storeId) {
  const result = await dashboard.updateStore(storeId, storeDrafts.value[storeId]);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel atualizar loja.");
    return;
  }

  ui.success("Loja atualizada.");
}

async function createStore() {
  const result = await dashboard.createStore(newStore);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel criar loja.");
    return;
  }

  newStore.name = "";
  newStore.code = "";
  newStore.city = "";
  newStore.cloneActiveRoster = true;
  newStore.defaultTemplateId = "";
  newStore.monthlyGoal = "";
  newStore.weeklyGoal = "";
  newStore.avgTicketGoal = "";
  newStore.conversionGoal = "";
  newStore.paGoal = "";
  ui.success("Loja criada.");
}

async function archiveStore(storeId) {
  const { confirmed } = await ui.confirm({
    title: "Arquivar loja",
    message: "A loja sera removida da operacao ativa. Deseja continuar?",
    confirmLabel: "Arquivar"
  });

  if (!confirmed) {
    return;
  }

  const result = await dashboard.archiveStore(storeId);

  if (result?.ok === false) {
    ui.error(result.message || "Nao foi possivel arquivar loja.");
    return;
  }

  ui.success("Loja arquivada.");
}
</script>

<template>
  <section class="admin-panel" data-testid="multistore-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Visao consolidada multi-loja</h2>
      <p class="admin-panel__text">Comparativo operacional para acompanhar performance entre lojas.</p>
    </header>

    <section class="metric-grid" data-testid="multistore-summary">
      <article class="metric-card"><span class="metric-card__label">Lojas ativas</span><strong class="metric-card__value">{{ rows.length }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Atendimentos consolidados</span><strong class="metric-card__value">{{ totalAttendances }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Vendas consolidadas</span><strong class="metric-card__value">{{ formatCurrencyBRL(totalSoldValue) }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Fila atual total</span><strong class="metric-card__value">{{ totalQueue }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Em atendimento agora</span><strong class="metric-card__value">{{ totalActiveServices }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Score medio operacional</span><strong class="metric-card__value">{{ Math.round(averageHealthScore) }}</strong></article>
    </section>

    <article class="insight-card insight-card--wide" data-testid="multistore-comparison-table">
      <h3 class="insight-card__title">Comparativo consolidado por loja</h3>
      <div class="insight-table-wrap">
        <table class="insight-table">
          <thead>
            <tr>
              <th>Loja</th>
              <th>Consultores</th>
              <th>Atendimentos</th>
              <th>Conversao</th>
              <th>Meta conv.</th>
              <th>Vendas</th>
              <th>Meta mensal</th>
              <th>% Meta</th>
              <th>Ticket medio</th>
              <th>Meta ticket</th>
              <th>P.A.</th>
              <th>Meta P.A.</th>
              <th>Score</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!rows.length"><td colspan="14">Sem lojas cadastradas.</td></tr>
            <tr v-for="row in rows" :key="row.storeId">
              <td>{{ row.storeName }}</td>
              <td>{{ row.consultants }}</td>
              <td>{{ row.attendances }}</td>
              <td>{{ formatPercent(row.conversionRate) }}</td>
              <td>{{ row.conversionGoal ? formatPercent(row.conversionGoal) : '-' }}</td>
              <td>{{ formatCurrencyBRL(row.soldValue) }}</td>
              <td>{{ row.monthlyGoal ? formatCurrencyBRL(row.monthlyGoal) : '-' }}</td>
              <td>
                <span v-if="row.monthlyGoal" :class="row.soldValue >= row.monthlyGoal ? 'multistore-goal--hit' : 'multistore-goal--miss'">
                  {{ formatPercent(row.monthlyGoal ? (row.soldValue / row.monthlyGoal) * 100 : 0) }}
                </span>
                <span v-else>-</span>
              </td>
              <td>{{ formatCurrencyBRL(row.ticketAverage) }}</td>
              <td>{{ row.avgTicketGoal ? formatCurrencyBRL(row.avgTicketGoal) : '-' }}</td>
              <td>{{ row.paScore.toFixed(2) }}</td>
              <td>{{ row.paGoal ? row.paGoal.toFixed(2) : '-' }}</td>
              <td>{{ Math.round(row.healthScore) }}</td>
              <td>
                <span v-if="row.storeId === state.activeStoreId" class="insight-tag">Ativa</span>
                <button v-else class="option-row__save" type="button" @click="dashboard.setActiveStore(row.storeId)">Abrir</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </article>

    <article class="insight-card insight-card--wide" data-testid="multistore-meeting">
      <header class="intel-card__header">
        <h3 class="insight-card__title">Painel de reuniao gerencial</h3>
        <span class="insight-tag">{{ rows.length }} lojas</span>
      </header>
      <div class="meeting-grid">
        <div v-for="row in rows" :key="row.storeId" class="meeting-card">
          <div class="meeting-card__header">
            <h4 class="meeting-card__name">{{ row.storeName }}</h4>
            <span class="meeting-card__score">Score {{ Math.round(row.healthScore) }}</span>
          </div>
          <div class="meeting-goal-list">
            <div class="meeting-goal-row">
              <span class="meeting-goal-row__label">Vendas vs meta mensal</span>
              <div class="meeting-goal-row__bar">
                <div class="meeting-goal-row__track">
                  <div
                    :class="['meeting-goal-row__fill', row.monthlyGoal && row.soldValue >= row.monthlyGoal ? 'meeting-goal-row__fill--hit' : 'meeting-goal-row__fill--miss']"
                    :style="{ width: row.monthlyGoal ? `${Math.min(100, (row.soldValue / row.monthlyGoal) * 100).toFixed(1)}%` : '0%' }"
                  ></div>
                </div>
                <span :class="['meeting-goal-row__value', row.monthlyGoal && row.soldValue >= row.monthlyGoal ? 'meeting-goal-row__value--hit' : 'meeting-goal-row__value--miss']">
                  {{ row.monthlyGoal ? formatPercent((row.soldValue / row.monthlyGoal) * 100) : formatCurrencyBRL(row.soldValue) }}
                </span>
              </div>
            </div>
            <div class="meeting-goal-row">
              <span class="meeting-goal-row__label">Conversao vs meta</span>
              <div class="meeting-goal-row__bar">
                <div class="meeting-goal-row__track">
                  <div
                    :class="['meeting-goal-row__fill', row.conversionGoal && row.conversionRate >= row.conversionGoal ? 'meeting-goal-row__fill--hit' : 'meeting-goal-row__fill--miss']"
                    :style="{ width: row.conversionGoal ? `${Math.min(100, (row.conversionRate / row.conversionGoal) * 100).toFixed(1)}%` : `${Math.min(100, row.conversionRate).toFixed(1)}%` }"
                  ></div>
                </div>
                <span :class="['meeting-goal-row__value', row.conversionGoal && row.conversionRate >= row.conversionGoal ? 'meeting-goal-row__value--hit' : 'meeting-goal-row__value--miss']">
                  {{ formatPercent(row.conversionRate) }}
                </span>
              </div>
            </div>
            <div class="meeting-goal-row">
              <span class="meeting-goal-row__label">Ticket medio vs meta</span>
              <div class="meeting-goal-row__bar">
                <div class="meeting-goal-row__track">
                  <div
                    :class="['meeting-goal-row__fill', row.avgTicketGoal && row.ticketAverage >= row.avgTicketGoal ? 'meeting-goal-row__fill--hit' : 'meeting-goal-row__fill--miss']"
                    :style="{ width: row.avgTicketGoal ? `${Math.min(100, (row.ticketAverage / row.avgTicketGoal) * 100).toFixed(1)}%` : '0%' }"
                  ></div>
                </div>
                <span :class="['meeting-goal-row__value', row.avgTicketGoal && row.ticketAverage >= row.avgTicketGoal ? 'meeting-goal-row__value--hit' : 'meeting-goal-row__value--miss']">
                  {{ formatCurrencyBRL(row.ticketAverage) }}
                </span>
              </div>
            </div>
            <div class="meeting-goal-row">
              <span class="meeting-goal-row__label">P.A. vs meta</span>
              <div class="meeting-goal-row__bar">
                <div class="meeting-goal-row__track">
                  <div
                    :class="['meeting-goal-row__fill', row.paGoal && row.paScore >= row.paGoal ? 'meeting-goal-row__fill--hit' : 'meeting-goal-row__fill--miss']"
                    :style="{ width: row.paGoal ? `${Math.min(100, (row.paScore / row.paGoal) * 100).toFixed(1)}%` : '0%' }"
                  ></div>
                </div>
                <span :class="['meeting-goal-row__value', row.paGoal && row.paScore >= row.paGoal ? 'meeting-goal-row__value--hit' : 'meeting-goal-row__value--miss']">
                  {{ row.paScore.toFixed(2) }}
                </span>
              </div>
            </div>
            <div class="meeting-goal-row">
              <span class="meeting-goal-row__label">Atendimentos</span>
              <div class="meeting-goal-row__bar">
                <div class="meeting-goal-row__track">
                  <div
                    class="meeting-goal-row__fill meeting-goal-row__fill--hit"
                    :style="{ width: totalAttendances ? `${((row.attendances / totalAttendances) * 100).toFixed(1)}%` : '0%' }"
                  ></div>
                </div>
                <span class="meeting-goal-row__value meeting-goal-row__value--hit">{{ row.attendances }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </article>

    <article v-if="canEditStores" class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Gestao de lojas</h3>
        <p class="settings-card__text">Identificacao, template operacional e metas por loja.</p>
      </header>

      <div class="option-list">
        <form
          v-for="store in state.stores"
          :key="store.id"
          class="multistore-form"
          @submit.prevent="saveStore(store.id)"
        >
          <div class="multistore-form__row">
            <input v-model="storeDrafts[store.id].name" class="product-row__input" type="text" placeholder="Nome">
            <input v-model="storeDrafts[store.id].code" class="product-row__input" type="text" placeholder="Codigo">
            <input v-model="storeDrafts[store.id].city" class="product-row__input" type="text" placeholder="Cidade">
            <select v-model="storeDrafts[store.id].defaultTemplateId" class="product-row__input">
              <option value="">Template padrão</option>
              <option v-for="t in operationTemplates" :key="t.id" :value="t.id">{{ t.label }}</option>
            </select>
          </div>
          <div class="multistore-form__row">
            <label class="multistore-form__field">
              <span class="multistore-form__label">Meta mensal (R$)</span>
              <input v-model="storeDrafts[store.id].monthlyGoal" class="product-row__input" type="number" min="0" step="100" placeholder="0">
            </label>
            <label class="multistore-form__field">
              <span class="multistore-form__label">Meta semanal (R$)</span>
              <input v-model="storeDrafts[store.id].weeklyGoal" class="product-row__input" type="number" min="0" step="100" placeholder="0">
            </label>
            <label class="multistore-form__field">
              <span class="multistore-form__label">Ticket médio alvo (R$)</span>
              <input v-model="storeDrafts[store.id].avgTicketGoal" class="product-row__input" type="number" min="0" step="100" placeholder="0">
            </label>
            <label class="multistore-form__field">
              <span class="multistore-form__label">Conversão alvo (%)</span>
              <input v-model="storeDrafts[store.id].conversionGoal" class="product-row__input" type="number" min="0" max="100" step="1" placeholder="0">
            </label>
            <label class="multistore-form__field">
              <span class="multistore-form__label">P.A. alvo</span>
              <input v-model="storeDrafts[store.id].paGoal" class="product-row__input" type="number" min="0" step="0.1" placeholder="0">
            </label>
          </div>
          <div class="multistore-form__actions">
            <button class="option-row__save" type="submit">Salvar</button>
            <button class="product-row__remove" type="button" @click="archiveStore(store.id)">Arquivar</button>
          </div>
        </form>
      </div>

      <form class="multistore-form multistore-form--add" @submit.prevent="createStore">
        <div class="multistore-form__row">
          <input v-model="newStore.name" class="product-add__input" type="text" placeholder="Nome da loja *" data-testid="multistore-new-name">
          <input v-model="newStore.code" class="product-add__input" type="text" placeholder="Codigo curto">
          <input v-model="newStore.city" class="product-add__input" type="text" placeholder="Cidade">
          <select v-model="newStore.defaultTemplateId" class="product-add__input">
            <option value="">Template padrão</option>
            <option v-for="t in operationTemplates" :key="t.id" :value="t.id">{{ t.label }}</option>
          </select>
        </div>
        <div class="multistore-form__row">
          <label class="multistore-form__field">
            <span class="multistore-form__label">Meta mensal (R$)</span>
            <input v-model="newStore.monthlyGoal" class="product-add__input" type="number" min="0" step="100" placeholder="0">
          </label>
          <label class="multistore-form__field">
            <span class="multistore-form__label">Meta semanal (R$)</span>
            <input v-model="newStore.weeklyGoal" class="product-add__input" type="number" min="0" step="100" placeholder="0">
          </label>
          <label class="multistore-form__field">
            <span class="multistore-form__label">Ticket médio alvo (R$)</span>
            <input v-model="newStore.avgTicketGoal" class="product-add__input" type="number" min="0" step="100" placeholder="0">
          </label>
          <label class="multistore-form__field">
            <span class="multistore-form__label">Conversão alvo (%)</span>
            <input v-model="newStore.conversionGoal" class="product-add__input" type="number" min="0" max="100" step="1" placeholder="0">
          </label>
          <label class="multistore-form__field">
            <span class="multistore-form__label">P.A. alvo</span>
            <input v-model="newStore.paGoal" class="product-add__input" type="number" min="0" step="0.1" placeholder="0">
          </label>
        </div>
        <div class="multistore-form__actions">
          <label class="settings-toggle">
            <input v-model="newStore.cloneActiveRoster" type="checkbox">
            <span>Copiar consultores da loja ativa</span>
          </label>
          <button class="product-add__button" type="submit" data-testid="multistore-new-submit">Adicionar loja</button>
        </div>
      </form>
    </article>
  </section>
</template>
