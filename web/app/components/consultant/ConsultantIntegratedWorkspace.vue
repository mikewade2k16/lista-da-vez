<script setup>
import { computed, ref } from "vue";
import AppSelectField from "~/components/ui/AppSelectField.vue";
import { formatCurrencyBRL, formatDurationMinutes, formatPercent } from "~/domain/utils/admin-metrics";

const FILTER_ALL = "all";

const props = defineProps({
  roster: {
    type: Array,
    default: () => []
  },
  ranking: {
    type: Object,
    default: null
  },
  overview: {
    type: Object,
    default: null
  },
  pending: {
    type: Boolean,
    default: false
  },
  errorMessage: {
    type: String,
    default: ""
  }
});

const searchTerm = ref("");
const storeFilter = ref(FILTER_ALL);
const statusFilter = ref(FILTER_ALL);
const goalFilter = ref(FILTER_ALL);

function buildRowKey(storeId, consultantId) {
  return `${String(storeId || "").trim()}:${String(consultantId || "").trim()}`;
}

function normalizeStatusEntry(code, label) {
  return {
    code,
    label
  };
}

function resolveRankingRow(map, consultant) {
  return (
    map.get(buildRowKey(consultant.storeId, consultant.id)) ||
    map.get(buildRowKey("", consultant.id)) ||
    null
  );
}

const monthlyRowsMap = computed(() =>
  new Map(
    (props.ranking?.monthlyRows || []).map((row) => [buildRowKey(row.storeId, row.consultantId), row])
  )
);
const dailyRowsMap = computed(() =>
  new Map(
    (props.ranking?.dailyRows || []).map((row) => [buildRowKey(row.storeId, row.consultantId), row])
  )
);
const statusMap = computed(() => {
  const nextMap = new Map();

  (props.overview?.activeServices || []).forEach((item) => {
    nextMap.set(buildRowKey(item.storeId, item.personId), normalizeStatusEntry("service", "Em atendimento"));
  });

  (props.overview?.waitingList || []).forEach((item) => {
    nextMap.set(buildRowKey(item.storeId, item.personId), normalizeStatusEntry("queue", "Na fila"));
  });

  (props.overview?.pausedEmployees || []).forEach((item) => {
    const code = String(item.pauseKind || "").trim() === "assignment" ? "assignment" : "paused";
    const label = code === "assignment" ? "Em tarefa" : "Pausado";
    nextMap.set(buildRowKey(item.storeId, item.personId), normalizeStatusEntry(code, label));
  });

  (props.overview?.availableConsultants || []).forEach((item) => {
    nextMap.set(buildRowKey(item.storeId, item.personId), normalizeStatusEntry("available", "Disponivel"));
  });

  return nextMap;
});
const storeOptions = computed(() => {
  const storesById = new Map();

  (props.roster || []).forEach((consultant) => {
    const storeId = String(consultant.storeId || "").trim();
    const storeName = String(consultant.storeName || "").trim();

    if (!storeId || !storeName || storesById.has(storeId)) {
      return;
    }

    storesById.set(storeId, {
      value: storeId,
      label: storeName
    });
  });

  return [
    { value: FILTER_ALL, label: "Todas as lojas" },
    ...[...storesById.values()].sort((left, right) => left.label.localeCompare(right.label))
  ];
});
const statusOptions = [
  { value: FILTER_ALL, label: "Todos os status" },
  { value: "available", label: "Disponivel" },
  { value: "service", label: "Em atendimento" },
  { value: "queue", label: "Na fila" },
  { value: "paused", label: "Pausado" },
  { value: "assignment", label: "Em tarefa" }
];
const goalOptions = [
  { value: FILTER_ALL, label: "Todas as metas" },
  { value: "at-goal", label: "Batendo meta" },
  { value: "off-goal", label: "Abaixo da meta" },
  { value: "no-goal", label: "Sem meta cadastrada" }
];

const consultantRows = computed(() =>
  (props.roster || []).map((consultant) => {
    const monthly = resolveRankingRow(monthlyRowsMap.value, consultant) || {};
    const daily = resolveRankingRow(dailyRowsMap.value, consultant) || {};
    const liveStatus = statusMap.value.get(buildRowKey(consultant.storeId, consultant.id)) || normalizeStatusEntry("available", "Disponivel");
    const monthlyGoal = Math.max(0, Number(consultant.monthlyGoal || 0) || 0);
    const soldValue = Math.max(0, Number(monthly.soldValue || 0) || 0);
    const dailySoldValue = Math.max(0, Number(daily.soldValue || 0) || 0);
    const attendances = Math.max(0, Number(monthly.attendances || 0) || 0);
    const conversions = Math.max(0, Number(monthly.conversions || 0) || 0);
    const progress = monthlyGoal > 0 ? (soldValue / monthlyGoal) * 100 : 0;

    return {
      ...consultant,
      liveStatusCode: liveStatus.code,
      liveStatusLabel: liveStatus.label,
      monthlyGoal,
      soldValue,
      dailySoldValue,
      attendances,
      conversions,
      conversionRate: Math.max(0, Number(monthly.conversionRate || 0) || 0),
      ticketAverage: Math.max(0, Number(monthly.ticketAverage || 0) || 0),
      paScore: Math.max(0, Number(monthly.paScore || 0) || 0),
      qualityScore: Math.max(0, Number(monthly.qualityScore || 0) || 0),
      avgDurationMs: Math.max(0, Number(monthly.avgDurationMs || 0) || 0),
      queueJumpServices: Math.max(0, Number(monthly.queueJumpServices || 0) || 0),
      progress,
      hitGoal: monthlyGoal > 0 && soldValue >= monthlyGoal,
      remainingToGoal: Math.max(0, monthlyGoal - soldValue)
    };
  })
);

const filteredRows = computed(() => {
  const normalizedSearch = String(searchTerm.value || "").trim().toLowerCase();

  return consultantRows.value.filter((row) => {
    if (storeFilter.value !== FILTER_ALL && row.storeId !== storeFilter.value) {
      return false;
    }

    if (statusFilter.value !== FILTER_ALL && row.liveStatusCode !== statusFilter.value) {
      return false;
    }

    if (goalFilter.value === "at-goal" && !row.hitGoal) {
      return false;
    }

    if (goalFilter.value === "off-goal" && (row.hitGoal || row.monthlyGoal <= 0)) {
      return false;
    }

    if (goalFilter.value === "no-goal" && row.monthlyGoal > 0) {
      return false;
    }

    if (!normalizedSearch) {
      return true;
    }

    return [
      row.name,
      row.storeName,
      row.storeCode,
      row.storeCity,
      row.role
    ].some((value) => String(value || "").toLowerCase().includes(normalizedSearch));
  });
});

const summary = computed(() => {
  const rows = filteredRows.value;
  const totalConsultants = rows.length;
  const totalSoldValue = rows.reduce((sum, row) => sum + row.soldValue, 0);
  const consultantsAtGoal = rows.filter((row) => row.hitGoal).length;
  const activeNow = rows.filter((row) => row.liveStatusCode === "service").length;
  const queuedNow = rows.filter((row) => row.liveStatusCode === "queue").length;
  const pausedNow = rows.filter((row) => row.liveStatusCode === "paused" || row.liveStatusCode === "assignment").length;
  const totalAttendances = rows.reduce((sum, row) => sum + row.attendances, 0);
  const totalConversions = rows.reduce((sum, row) => sum + row.conversions, 0);

  return {
    totalConsultants,
    totalSoldValue,
    consultantsAtGoal,
    activeNow,
    queuedNow,
    pausedNow,
    conversionRate: totalAttendances > 0 ? (totalConversions / totalAttendances) * 100 : 0
  };
});

const storeRows = computed(() => {
  const grouped = new Map();

  filteredRows.value.forEach((row) => {
    const key = row.storeId;
    const current = grouped.get(key) || {
      storeId: row.storeId,
      storeName: row.storeName,
      consultants: 0,
      consultantsAtGoal: 0,
      totalGoal: 0,
      totalSoldValue: 0,
      totalAttendances: 0,
      totalConversions: 0
    };

    current.consultants += 1;
    current.consultantsAtGoal += row.hitGoal ? 1 : 0;
    current.totalGoal += row.monthlyGoal;
    current.totalSoldValue += row.soldValue;
    current.totalAttendances += row.attendances;
    current.totalConversions += row.conversions;
    grouped.set(key, current);
  });

  return [...grouped.values()]
    .map((row) => ({
      ...row,
      progress: row.totalGoal > 0 ? (row.totalSoldValue / row.totalGoal) * 100 : 0,
      conversionRate: row.totalAttendances > 0 ? (row.totalConversions / row.totalAttendances) * 100 : 0
    }))
    .sort((left, right) => right.totalSoldValue - left.totalSoldValue);
});

const topPerformers = computed(() =>
  [...filteredRows.value]
    .sort((left, right) => {
      if (right.soldValue !== left.soldValue) {
        return right.soldValue - left.soldValue;
      }

      return right.conversionRate - left.conversionRate;
    })
    .slice(0, 5)
);
</script>

<template>
  <section class="admin-panel" data-testid="consultant-integrated-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Consultores em todas as lojas</h2>
      <p class="admin-panel__text">
        Comparativo consolidado de meta, conversao, ticket e status operacional do tenant ativo.
      </p>
    </header>

    <article v-if="errorMessage" class="insight-card">
      <p class="settings-card__text">{{ errorMessage }}</p>
    </article>

    <article v-else-if="pending && !consultantRows.length" class="insight-card">
      <p class="settings-card__text">Carregando comparativo consolidado dos consultores...</p>
    </article>

    <template v-else>
      <section class="metric-grid" data-testid="consultant-integrated-summary">
        <article class="metric-card">
          <span class="metric-card__label">Consultores visiveis</span>
          <strong class="metric-card__value">{{ summary.totalConsultants }}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Batendo meta</span>
          <strong class="metric-card__value">{{ summary.consultantsAtGoal }}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Vendido no mes</span>
          <strong class="metric-card__value">{{ formatCurrencyBRL(summary.totalSoldValue) }}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Conversao media</span>
          <strong class="metric-card__value">{{ formatPercent(summary.conversionRate) }}</strong>
        </article>
        <article class="metric-card metric-card--soft">
          <span class="metric-card__label">Em atendimento agora</span>
          <strong class="metric-card__value">{{ summary.activeNow }}</strong>
        </article>
        <article class="metric-card metric-card--soft">
          <span class="metric-card__label">Fila / pausas</span>
          <strong class="metric-card__value">{{ summary.queuedNow }} / {{ summary.pausedNow }}</strong>
        </article>
      </section>

      <article class="settings-card consultant-integrated-filters">
        <div class="consultant-integrated-filters__grid">
          <label class="settings-field consultant-integrated-filters__search">
            <span>Buscar consultor</span>
            <input v-model="searchTerm" type="text" placeholder="Nome, loja ou cargo">
          </label>
          <label class="settings-field">
            <span>Loja</span>
            <AppSelectField
              :model-value="storeFilter"
              :options="storeOptions"
              placeholder="Filtrar loja"
              @update:model-value="storeFilter = $event"
            />
          </label>
          <label class="settings-field">
            <span>Status</span>
            <AppSelectField
              :model-value="statusFilter"
              :options="statusOptions"
              placeholder="Filtrar status"
              @update:model-value="statusFilter = $event"
            />
          </label>
          <label class="settings-field">
            <span>Meta</span>
            <AppSelectField
              :model-value="goalFilter"
              :options="goalOptions"
              placeholder="Filtrar meta"
              @update:model-value="goalFilter = $event"
            />
          </label>
        </div>
      </article>

      <div class="consultant-integrated-grid">
        <article class="insight-card">
          <header class="intel-card__header">
            <h3 class="insight-card__title">Top consultores</h3>
            <span class="insight-tag">{{ topPerformers.length }} em destaque</span>
          </header>
          <div class="consultant-mini-list">
            <div v-if="!topPerformers.length" class="insight-empty">
              Nenhum consultor encontrado para os filtros atuais.
            </div>
            <article v-for="row in topPerformers" :key="`${row.storeId}-${row.id}`" class="consultant-mini-card">
              <div class="consultant-mini-card__head">
                <strong>{{ row.name }}</strong>
                <span class="consultant-status" :class="`consultant-status--${row.liveStatusCode}`">{{ row.liveStatusLabel }}</span>
              </div>
              <span class="metric-card__text">{{ row.storeName }}</span>
              <div class="consultant-mini-card__meta">
                <span>{{ formatCurrencyBRL(row.soldValue) }}</span>
                <span>{{ formatPercent(row.conversionRate) }}</span>
                <span>{{ row.hitGoal ? "Meta batida" : "Meta em andamento" }}</span>
              </div>
            </article>
          </div>
        </article>

        <article class="insight-card insight-card--wide">
          <header class="intel-card__header">
            <h3 class="insight-card__title">Meta por loja</h3>
            <span class="insight-tag">{{ storeRows.length }} lojas</span>
          </header>
          <div class="insight-table-wrap">
            <table class="insight-table">
              <thead>
                <tr>
                  <th>Loja</th>
                  <th>Consultores</th>
                  <th>Batendo meta</th>
                  <th>Vendido</th>
                  <th>Meta total</th>
                  <th>Progresso</th>
                  <th>Conversao</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!storeRows.length">
                  <td colspan="7">Nenhuma loja encontrada para o filtro selecionado.</td>
                </tr>
                <tr v-for="row in storeRows" :key="row.storeId">
                  <td>{{ row.storeName }}</td>
                  <td>{{ row.consultants }}</td>
                  <td>{{ row.consultantsAtGoal }}</td>
                  <td>{{ formatCurrencyBRL(row.totalSoldValue) }}</td>
                  <td>{{ formatCurrencyBRL(row.totalGoal) }}</td>
                  <td>{{ formatPercent(row.progress) }}</td>
                  <td>{{ formatPercent(row.conversionRate) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </article>
      </div>

      <article class="insight-card insight-card--wide">
        <header class="intel-card__header">
          <h3 class="insight-card__title">Comparativo completo</h3>
          <span class="insight-tag">{{ filteredRows.length }} consultores</span>
        </header>
        <div class="insight-table-wrap">
          <table class="insight-table">
            <thead>
              <tr>
                <th>Consultor</th>
                <th>Loja</th>
                <th>Status</th>
                <th>Meta</th>
                <th>Vendido</th>
                <th>Hoje</th>
                <th>Conversao</th>
                <th>Ticket</th>
                <th>P.A.</th>
                <th>Tempo</th>
                <th>Fora da vez</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!filteredRows.length">
                <td colspan="11">Nenhum consultor encontrado para os filtros selecionados.</td>
              </tr>
              <tr v-for="row in filteredRows" :key="`${row.storeId}-${row.id}`">
                <td>
                  <div class="consultant-table__person">
                    <strong>{{ row.name }}</strong>
                    <small>{{ row.role }}</small>
                  </div>
                </td>
                <td>{{ row.storeName }}</td>
                <td>
                  <span class="consultant-status" :class="`consultant-status--${row.liveStatusCode}`">{{ row.liveStatusLabel }}</span>
                </td>
                <td>
                  <div class="consultant-table__goal">
                    <strong>{{ formatCurrencyBRL(row.monthlyGoal) }}</strong>
                    <small v-if="row.monthlyGoal > 0">{{ formatPercent(row.progress) }}</small>
                    <small v-else>Sem meta</small>
                  </div>
                </td>
                <td>
                  <div class="consultant-table__goal">
                    <strong>{{ formatCurrencyBRL(row.soldValue) }}</strong>
                    <small v-if="row.monthlyGoal > 0">
                      {{ row.hitGoal ? "Meta batida" : `Faltam ${formatCurrencyBRL(row.remainingToGoal)}` }}
                    </small>
                  </div>
                </td>
                <td>{{ formatCurrencyBRL(row.dailySoldValue) }}</td>
                <td>{{ formatPercent(row.conversionRate) }}</td>
                <td>{{ formatCurrencyBRL(row.ticketAverage) }}</td>
                <td>{{ row.paScore.toFixed(2) }}</td>
                <td>{{ formatDurationMinutes(row.avgDurationMs) }}</td>
                <td>{{ row.queueJumpServices }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </article>
    </template>
  </section>
</template>

<style scoped>
.consultant-integrated-filters__grid {
  display: grid;
  grid-template-columns: minmax(0, 1.7fr) repeat(3, minmax(0, 1fr));
  gap: 0.85rem;
}

.consultant-integrated-filters__search {
  min-width: 0;
}

.consultant-integrated-grid {
  display: grid;
  grid-template-columns: minmax(18rem, 24rem) minmax(0, 1fr);
  gap: 1rem;
}

.consultant-mini-list {
  display: grid;
  gap: 0.75rem;
}

.consultant-mini-card {
  display: grid;
  gap: 0.35rem;
  padding: 0.9rem;
  border: 1px solid rgba(125, 146, 255, 0.16);
  border-radius: 0.9rem;
  background: rgba(13, 19, 36, 0.72);
}

.consultant-mini-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.consultant-mini-card__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 0.8rem;
  color: rgba(226, 232, 240, 0.78);
  font-size: 0.8rem;
}

.consultant-status {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 1.75rem;
  padding: 0 0.6rem;
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.22);
  font-size: 0.72rem;
  font-weight: 700;
  white-space: nowrap;
}

.consultant-status--available {
  background: rgba(34, 197, 94, 0.14);
  color: #86efac;
}

.consultant-status--service {
  background: rgba(59, 130, 246, 0.14);
  color: #93c5fd;
}

.consultant-status--queue {
  background: rgba(250, 204, 21, 0.14);
  color: #fde68a;
}

.consultant-status--paused,
.consultant-status--assignment {
  background: rgba(244, 114, 182, 0.14);
  color: #f9a8d4;
}

.consultant-table__person,
.consultant-table__goal {
  display: grid;
  gap: 0.18rem;
}

.consultant-table__person small,
.consultant-table__goal small {
  color: rgba(148, 163, 184, 0.92);
  font-size: 0.72rem;
}

@media (max-width: 1100px) {
  .consultant-integrated-filters__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .consultant-integrated-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 720px) {
  .consultant-integrated-filters__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
