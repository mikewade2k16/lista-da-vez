<script setup>
import { computed, ref } from "vue";
import { buildReportData } from "@core/utils/reports";
import { exportReportCsv, exportReportPdf } from "~/utils/report-export";
import ReportsFilterToolbar from "~/components/reports/ReportsFilterToolbar.vue";
import ReportsQualityTable from "~/components/reports/ReportsQualityTable.vue";
import ReportsResultsTable from "~/components/reports/ReportsResultsTable.vue";
import { useDashboardStore } from "~/stores/dashboard";
import { useUiStore } from "~/stores/ui";

const CHART_WIDTH = 480;
const CHART_HEIGHT = 72;

const props = defineProps({
  state: {
    type: Object,
    required: true
  }
});

const dashboard = useDashboardStore();
const ui = useUiStore();
const filtersExpanded = ref(false);
const expandedGroup = ref(null);

function formatPercent(value) {
  return `${Number(value || 0).toFixed(1)}%`;
}

function formatCurrency(value) {
  return new Intl.NumberFormat("pt-BR", {
    style: "currency",
    currency: "BRL"
  }).format(Number(value || 0));
}

function getInitials(name) {
  return String(name || "")
    .split(" ")
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0].toUpperCase())
    .join("");
}

const report = computed(() =>
  buildReportData({
    history: props.state.serviceHistory || [],
    roster: props.state.roster || [],
    visitReasonOptions: props.state.visitReasonOptions || [],
    customerSourceOptions: props.state.customerSourceOptions || [],
    filters: props.state.reportFilters || {}
  })
);
const outcomeItems = computed(() => {
  const total = report.value.metrics.totalAttendances || 1;

  return [
    { label: "Compra", count: report.value.chartData.outcomeCounts.compra, color: "#22c55e" },
    { label: "Reserva", count: report.value.chartData.outcomeCounts.reserva, color: "#38bdf8" },
    { label: "Nao compra", count: report.value.chartData.outcomeCounts["nao-compra"], color: "#475569" }
  ].map((item) => ({
    ...item,
    width: `${((item.count / total) * 100).toFixed(1)}%`
  }));
});
const hourlyBars = computed(() => {
  const allHours = Array.from({ length: 24 }, (_, index) => String(index).padStart(2, "0"));
  const maxValue = Math.max(...report.value.chartData.hourlyData.map((item) => item.attendances), 1);
  const barWidth = CHART_WIDTH / 24;

  return allHours.map((hour, index) => {
    const item = report.value.chartData.hourlyData.find((entry) => entry.hour === hour);
    const count = item ? item.attendances : 0;
    const conversions = item ? item.conversions : 0;
    const height = count > 0 ? Math.max(3, (count / maxValue) * CHART_HEIGHT) : 0;
    const conversionHeight = conversions > 0 ? Math.max(2, (conversions / maxValue) * CHART_HEIGHT) : 0;
    const x = index * barWidth;

    return {
      hour,
      x: (x + 1).toFixed(1),
      width: (barWidth - 2).toFixed(1),
      height: height.toFixed(1),
      y: (CHART_HEIGHT - height).toFixed(1),
      conversionHeight: conversionHeight.toFixed(1),
      conversionY: (CHART_HEIGHT - conversionHeight).toFixed(1)
    };
  });
});
const hourLabels = [
  { label: "00h", x: 10 },
  { label: "06h", x: 130 },
  { label: "12h", x: 250 },
  { label: "18h", x: 370 }
];
const goalRows = computed(() => {
  const rosterWithAnyGoal = (props.state.roster || []).filter((consultant) =>
    Number(consultant.monthlyGoal || 0) > 0 ||
    Number(consultant.conversionGoal || 0) > 0 ||
    Number(consultant.avgTicketGoal || 0) > 0 ||
    Number(consultant.paGoal || 0) > 0
  );

  return rosterWithAnyGoal.map((consultant) => {
    const aggregate = report.value.chartData.consultantAgg.find((item) => item.consultantId === consultant.id) || {
      attendances: 0,
      conversions: 0,
      saleAmount: 0
    };
    const monthlyGoal = Number(consultant.monthlyGoal || 0);
    const conversionGoal = Number(consultant.conversionGoal || 0);
    const avgTicketGoal = Number(consultant.avgTicketGoal || 0);
    const paGoal = Number(consultant.paGoal || 0);

    const soldValue = Number(aggregate.saleAmount || 0);
    const conversionRate = aggregate.attendances > 0 ? (aggregate.conversions / aggregate.attendances) * 100 : 0;
    const ticketAverage = aggregate.conversions > 0 ? soldValue / aggregate.conversions : 0;

    const historyForConsultant = (props.state.serviceHistory || []).filter((entry) => entry.personId === consultant.id);
    const totalPieces = historyForConsultant.reduce((sum, entry) => {
      return sum + (Array.isArray(entry.productsClosed) ? entry.productsClosed.length : 0);
    }, 0);
    const paScore = historyForConsultant.length ? totalPieces / historyForConsultant.length : 0;

    const progress = monthlyGoal > 0 ? Math.min(100, (soldValue / monthlyGoal) * 100) : 0;

    return {
      consultantId: consultant.id,
      consultantName: consultant.name,
      consultantColor: consultant.color,
      initials: getInitials(consultant.name),
      attendances: aggregate.attendances,
      soldValue,
      saleAmountLabel: formatCurrency(soldValue),
      monthlyGoal,
      goalLabel: monthlyGoal ? formatCurrency(monthlyGoal) : null,
      progress,
      remaining: Math.max(0, monthlyGoal - soldValue),
      remainingLabel: formatCurrency(Math.max(0, monthlyGoal - soldValue)),
      conversionRate,
      conversionRateLabel: formatPercent(conversionRate),
      conversionGoal,
      conversionGoalLabel: conversionGoal ? formatPercent(conversionGoal) : null,
      conversionHit: conversionGoal > 0 && conversionRate >= conversionGoal,
      ticketAverage,
      ticketAverageLabel: formatCurrency(ticketAverage),
      avgTicketGoal,
      avgTicketGoalLabel: avgTicketGoal ? formatCurrency(avgTicketGoal) : null,
      ticketHit: avgTicketGoal > 0 && ticketAverage >= avgTicketGoal,
      paScore,
      paScoreLabel: paScore.toFixed(2),
      paGoal,
      paGoalLabel: paGoal ? paGoal.toFixed(2) : null,
      paHit: paGoal > 0 && paScore >= paGoal
    };
  });
});
const teamGoalSummary = computed(() => {
  const totalGoal = (props.state.roster || []).reduce((sum, item) => sum + Number(item.monthlyGoal || 0), 0);
  const totalSold = report.value.chartData.consultantAgg.reduce((sum, item) => sum + Number(item.saleAmount || 0), 0);

  return {
    totalGoalLabel: formatCurrency(totalGoal),
    totalSoldLabel: formatCurrency(totalSold),
    progress: totalGoal > 0 ? Math.min(100, (totalSold / totalGoal) * 100) : 0
  };
});

function toggleFilters() {
  filtersExpanded.value = !filtersExpanded.value;

  if (!filtersExpanded.value) {
    expandedGroup.value = null;
  }
}

function toggleGroup(groupId) {
  expandedGroup.value = expandedGroup.value === groupId ? null : groupId;
}

function updateFilter(filterId, value) {
  void dashboard.updateReportFilter(filterId, value);
}

function toggleFilterValue(filterId, value) {
  const currentValues = Array.isArray(props.state.reportFilters?.[filterId]) ? props.state.reportFilters[filterId] : [];
  const nextValues = currentValues.includes(value)
    ? currentValues.filter((item) => item !== value)
    : [...currentValues, value];

  void dashboard.updateReportFilter(filterId, nextValues);
}

function clearFilter(filterId, filterValue = null) {
  if (Array.isArray(props.state.reportFilters?.[filterId])) {
    const currentValues = props.state.reportFilters[filterId] || [];
    const nextValues = filterValue ? currentValues.filter((item) => item !== filterValue) : [];

    void dashboard.updateReportFilter(filterId, nextValues);
    return;
  }

  void dashboard.updateReportFilter(filterId, "");
}

function resetFilters() {
  void dashboard.resetReportFilters();
}

function exportCsv() {
  exportReportCsv(report.value);
}

function exportPdf() {
  if (!exportReportPdf(report.value)) {
    ui.error("Nao foi possivel abrir a janela de impressao.");
  }
}
</script>

<template>
  <section class="admin-panel" data-testid="reports-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Relatorios operacionais</h2>
      <p class="admin-panel__text">
        Leitura de performance, tempo medio e qualidade de preenchimento do fechamento.
      </p>
    </header>

    <ReportsFilterToolbar
      :filters="report.filters"
      :roster="state.roster || []"
      :visit-reason-options="state.visitReasonOptions || []"
      :customer-source-options="state.customerSourceOptions || []"
      :campaigns="state.campaigns || []"
      :filters-expanded="filtersExpanded"
      :expanded-group="expandedGroup"
      @toggle-filters="toggleFilters"
      @toggle-group="toggleGroup"
      @toggle-value="toggleFilterValue"
      @update-filter="updateFilter"
      @clear-filter="clearFilter"
      @reset-filters="resetFilters"
      @export-csv="exportCsv"
      @export-pdf="exportPdf"
    />

    <section class="metric-grid" data-testid="reports-summary">
      <article class="metric-card"><span class="metric-card__label">Atendimentos</span><strong class="metric-card__value">{{ report.metrics.totalAttendances }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Conversao</span><strong class="metric-card__value">{{ formatPercent(report.metrics.conversionRate) }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Valor vendido</span><strong class="metric-card__value">{{ report.metrics.soldValueLabel }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Ticket medio</span><strong class="metric-card__value">{{ report.metrics.averageTicketLabel }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Media de atendimento</span><strong class="metric-card__value">{{ report.metrics.averageDurationLabel }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Media de espera</span><strong class="metric-card__value">{{ report.metrics.averageQueueWaitLabel }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Fora da vez</span><strong class="metric-card__value">{{ formatPercent(report.metrics.queueJumpRate) }}</strong></article>
      <article class="metric-card"><span class="metric-card__label">Bonus campanhas</span><strong class="metric-card__value">{{ report.metrics.campaignBonusTotalLabel }}</strong></article>
    </section>

    <div class="report-chart-grid">
      <article class="insight-card">
        <header class="intel-card__header">
          <h3 class="insight-card__title">Desfecho dos atendimentos</h3>
          <span class="insight-tag">{{ report.metrics.totalAttendances }} total</span>
        </header>
        <div v-for="item in outcomeItems" :key="item.label" class="dist-bar-row">
          <span class="dist-bar-row__label" :style="{ color: item.color }">{{ item.label }}</span>
          <div class="dist-bar-row__track"><div class="dist-bar-row__fill" :style="{ width: item.width, background: item.color }"></div></div>
          <span class="dist-bar-row__count">{{ item.count }}</span>
        </div>
      </article>

      <article class="insight-card">
        <header class="intel-card__header"><h3 class="insight-card__title">Atendimentos por hora</h3></header>
        <span v-if="!report.chartData.hourlyData.length" class="insight-empty">Sem dados para o periodo.</span>
        <div v-else class="chart-hourly-wrap">
          <svg :viewBox="`0 0 ${CHART_WIDTH} ${CHART_HEIGHT + 18}`" width="100%">
            <g v-for="item in hourlyBars" :key="item.hour">
              <rect :x="item.x" :y="item.y" :width="item.width" :height="item.height" fill="#1e293b" rx="2" />
              <rect v-if="Number(item.conversionHeight) > 0" :x="item.x" :y="item.conversionY" :width="item.width" :height="item.conversionHeight" fill="#22c55e" rx="2" />
            </g>
            <text v-for="item in hourLabels" :key="item.label" :x="item.x" :y="CHART_HEIGHT + 13" font-size="9" fill="#94a3b8" text-anchor="middle">{{ item.label }}</text>
          </svg>
          <div class="chart-legend">
            <span class="chart-legend__item chart-legend__item--base">Atendimentos</span>
            <span class="chart-legend__item chart-legend__item--success">Conversoes</span>
          </div>
        </div>
      </article>
    </div>

    <article class="insight-card insight-card--wide">
      <header class="intel-card__header"><h3 class="insight-card__title">Meta mensal dos consultores</h3></header>
      <span v-if="!goalRows.length" class="insight-empty">Nenhum consultor com meta definida. Configure metas em Configuracoes &gt; Consultores.</span>
      <template v-else>
        <div class="team-goal-summary">
          <div class="team-goal-summary__header">
            <span class="metric-card__label">Meta da equipe</span>
            <span class="metric-card__text">{{ teamGoalSummary.totalSoldLabel }} de {{ teamGoalSummary.totalGoalLabel }}</span>
          </div>
          <div class="progress-bar progress-bar--team">
            <span class="progress-bar__fill" :style="{ '--progress': `${teamGoalSummary.progress.toFixed(1)}%` }"></span>
          </div>
        </div>

        <div v-for="item in goalRows" :key="item.consultantId" class="consultant-goal-row">
          <span class="consultant-goal-row__avatar" :style="{ '--avatar-accent': item.consultantColor }">{{ item.initials }}</span>
          <div class="consultant-goal-row__body">
            <div class="consultant-goal-row__header">
              <strong class="consultant-goal-row__name">{{ item.consultantName }}</strong>
              <span class="insight-tag">{{ item.attendances }} atend</span>
              <span v-if="item.monthlyGoal && item.progress >= 100" class="insight-tag insight-tag--success">Meta R$ atingida</span>
            </div>
            <template v-if="item.monthlyGoal">
              <div class="progress-bar">
                <span class="progress-bar__fill" :style="{ '--progress': `${item.progress.toFixed(1)}%`, background: `linear-gradient(90deg, ${item.consultantColor}88, ${item.consultantColor})` }"></span>
              </div>
              <div class="consultant-goal-row__footer">
                <span class="metric-card__text">{{ item.saleAmountLabel }} vendido</span>
                <span class="metric-card__text">Meta: {{ item.goalLabel }}</span>
                <span v-if="item.remaining > 0" class="metric-card__text">Falta: {{ item.remainingLabel }}</span>
              </div>
            </template>
            <div class="consultant-goal-indicators">
              <span v-if="item.conversionGoal" :class="['consultant-goal-badge', item.conversionHit ? 'consultant-goal-badge--hit' : 'consultant-goal-badge--miss']">
                Conv. {{ item.conversionRateLabel }} / meta {{ item.conversionGoalLabel }}
              </span>
              <span v-if="item.avgTicketGoal" :class="['consultant-goal-badge', item.ticketHit ? 'consultant-goal-badge--hit' : 'consultant-goal-badge--miss']">
                Ticket {{ item.ticketAverageLabel }} / meta {{ item.avgTicketGoalLabel }}
              </span>
              <span v-if="item.paGoal" :class="['consultant-goal-badge', item.paHit ? 'consultant-goal-badge--hit' : 'consultant-goal-badge--miss']">
                P.A. {{ item.paScoreLabel }} / meta {{ item.paGoalLabel }}
              </span>
            </div>
          </div>
        </div>
      </template>
    </article>

    <div class="report-dist-grid">
      <article class="insight-card">
        <header class="intel-card__header"><h3 class="insight-card__title">Produtos fechados</h3></header>
        <span v-if="!report.chartData.topProductsClosed.length" class="insight-empty">Nenhum produto registrado.</span>
        <template v-else>
          <div v-for="item in report.chartData.topProductsClosed" :key="item.label" class="dist-bar-row">
            <span class="dist-bar-row__label">{{ item.label }}</span>
            <div class="dist-bar-row__track"><div class="dist-bar-row__fill" :style="{ width: `${((item.count / report.chartData.topProductsClosed[0].count) * 100).toFixed(1)}%` }"></div></div>
            <span class="dist-bar-row__count">{{ item.count }}</span>
          </div>
        </template>
      </article>
      <article class="insight-card">
        <header class="intel-card__header"><h3 class="insight-card__title">Motivos de visita</h3></header>
        <span v-if="!report.chartData.topVisitReasons.length" class="insight-empty">Nenhum motivo registrado.</span>
        <template v-else>
          <div v-for="item in report.chartData.topVisitReasons" :key="item.label" class="dist-bar-row">
            <span class="dist-bar-row__label">{{ item.label }}</span>
            <div class="dist-bar-row__track"><div class="dist-bar-row__fill" :style="{ width: `${((item.count / report.chartData.topVisitReasons[0].count) * 100).toFixed(1)}%` }"></div></div>
            <span class="dist-bar-row__count">{{ item.count }}</span>
          </div>
        </template>
      </article>
      <article class="insight-card">
        <header class="intel-card__header"><h3 class="insight-card__title">Origem do cliente</h3></header>
        <span v-if="!report.chartData.topCustomerSources.length" class="insight-empty">Nenhuma origem registrada.</span>
        <template v-else>
          <div v-for="item in report.chartData.topCustomerSources" :key="item.label" class="dist-bar-row">
            <span class="dist-bar-row__label">{{ item.label }}</span>
            <div class="dist-bar-row__track"><div class="dist-bar-row__fill" :style="{ width: `${((item.count / report.chartData.topCustomerSources[0].count) * 100).toFixed(1)}%` }"></div></div>
            <span class="dist-bar-row__count">{{ item.count }}</span>
          </div>
        </template>
      </article>
    </div>

    <div class="insight-grid">
      <ReportsQualityTable :quality="report.quality" />
      <ReportsResultsTable :rows="report.rows" />
    </div>
  </section>
</template>
