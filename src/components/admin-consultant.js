import {
  buildConsultantStats,
  formatCurrencyBRL,
  formatDurationMinutes,
  formatPercent
} from "../utils/admin-metrics.js";

function renderConsultantSelector(roster, selectedConsultantId) {
  return `
    <div class="admin-selector">
      ${roster
        .map((consultant) => {
          const isSelected = consultant.id === selectedConsultantId;

          return `
            <button
              type="button"
              class="admin-selector__button ${isSelected ? "admin-selector__button--active" : ""}"
              data-action="select-consultant"
              data-person-id="${consultant.id}"
            >
              ${consultant.name}
            </button>
          `;
        })
        .join("")}
    </div>
  `;
}

export function renderConsultantPanel({ roster, selectedConsultantId, history, simulationAdditionalSales }) {
  const selectedConsultant = roster.find((consultant) => consultant.id === selectedConsultantId) || roster[0];

  if (!selectedConsultant) {
    return "";
  }

  const stats = buildConsultantStats({
    history,
    consultantId: selectedConsultant.id,
    monthlyGoal: Number(selectedConsultant.monthlyGoal || 0),
    commissionRate: Number(selectedConsultant.commissionRate || 0)
  });
  const simulation = Math.max(0, Number(simulationAdditionalSales || 0));
  const projectedSales = stats.soldValue + simulation;
  const goalPercent = stats.monthlyGoal ? (stats.soldValue / stats.monthlyGoal) * 100 : 0;
  const projectedGoalPercent = stats.monthlyGoal ? (projectedSales / stats.monthlyGoal) * 100 : 0;
  const projectedCommission = projectedSales * stats.commissionRate;

  return `
    <section class="admin-panel">
      <header class="admin-panel__header">
        <h2 class="admin-panel__title">Perfil do consultor</h2>
        <p class="admin-panel__text">Meta mensal, desempenho e simulacao de venda.</p>
      </header>

      ${renderConsultantSelector(roster, selectedConsultant.id)}

      <div class="metric-grid">
        <article class="metric-card">
          <span class="metric-card__label">Meta mensal</span>
          <strong class="metric-card__value">${formatCurrencyBRL(stats.monthlyGoal)}</strong>
          <span class="metric-card__text">Faltam ${formatCurrencyBRL(stats.remainingToGoal)} para fechar a meta.</span>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Vendido no mes</span>
          <strong class="metric-card__value">${formatCurrencyBRL(stats.soldValue)}</strong>
          <span class="metric-card__text">${formatPercent(goalPercent)} da meta.</span>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Comissao estimada</span>
          <strong class="metric-card__value">${formatCurrencyBRL(stats.estimatedCommission)}</strong>
          <span class="metric-card__text">Taxa atual: ${formatPercent(stats.commissionRate * 100)}.</span>
        </article>
      </div>

      <div class="progress-block">
        <span class="progress-block__label">Progresso da meta</span>
        <div class="progress-bar">
          <span class="progress-bar__fill" style="--progress: ${Math.min(goalPercent, 100)}%"></span>
        </div>
        <span class="progress-block__text">${formatPercent(goalPercent)} concluido</span>
      </div>

      <section class="simulator">
        <h3 class="simulator__title">Simulador de fechamento</h3>
        <label class="simulator__field">
          <span>Venda adicional para simular (R$)</span>
          <input
            class="simulator__input"
            type="number"
            min="0"
            step="100"
            value="${simulation}"
            data-action="set-simulation-value"
          >
        </label>
        <div class="metric-grid metric-grid--tight">
          <article class="metric-card metric-card--soft">
            <span class="metric-card__label">Vendido projetado</span>
            <strong class="metric-card__value">${formatCurrencyBRL(projectedSales)}</strong>
            <span class="metric-card__text">${formatPercent(projectedGoalPercent)} da meta.</span>
          </article>
          <article class="metric-card metric-card--soft">
            <span class="metric-card__label">Comissao projetada</span>
            <strong class="metric-card__value">${formatCurrencyBRL(projectedCommission)}</strong>
            <span class="metric-card__text">Com base na taxa atual.</span>
          </article>
        </div>
      </section>

      <div class="metric-grid metric-grid--tight">
        <article class="metric-card metric-card--soft">
          <span class="metric-card__label">Atendimentos no mes</span>
          <strong class="metric-card__value">${stats.monthEntries.length}</strong>
        </article>
        <article class="metric-card metric-card--soft">
          <span class="metric-card__label">Taxa de conversao</span>
          <strong class="metric-card__value">${formatPercent(stats.conversionRate)}</strong>
        </article>
        <article class="metric-card metric-card--soft">
          <span class="metric-card__label">Tempo medio por atendimento</span>
          <strong class="metric-card__value">${formatDurationMinutes(stats.averageDurationMs)}</strong>
        </article>
        <article class="metric-card metric-card--soft">
          <span class="metric-card__label">Nao clientes convertidos</span>
          <strong class="metric-card__value">${stats.nonClientConversions}</strong>
        </article>
        <article class="metric-card metric-card--soft">
          <span class="metric-card__label">Dias com venda</span>
          <strong class="metric-card__value">${stats.daysWithSales}</strong>
        </article>
        <article class="metric-card metric-card--soft">
          <span class="metric-card__label">Atendimentos fora da vez</span>
          <strong class="metric-card__value">${stats.queueJumpServices}</strong>
        </article>
      </div>
    </section>
  `;
}
