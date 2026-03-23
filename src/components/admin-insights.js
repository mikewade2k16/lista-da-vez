import {
  buildInsights,
  buildTimeIntelligence,
  formatCurrencyBRL,
  formatDurationMinutes,
  formatPercent
} from "../utils/admin-metrics.js";

function renderTagList(title, items) {
  return `
    <article class="insight-card">
      <h3 class="insight-card__title">${title}</h3>
      <div class="insight-tags">
        ${
          items.length
            ? items
                .map((item) => `<span class="insight-tag">${item.label} <strong>${item.count}</strong></span>`)
                .join("")
            : '<span class="insight-empty">Sem dados.</span>'
        }
      </div>
    </article>
  `;
}

function renderHourlyTable(items) {
  const rows = items
    .map(
      (item) => `
        <tr>
          <td>${item.label}</td>
          <td>${item.count}</td>
          <td>${formatCurrencyBRL(item.value)}</td>
        </tr>
      `
    )
    .join("");

  return `
    <article class="insight-card">
      <h3 class="insight-card__title">Horarios com mais venda</h3>
      <div class="insight-table-wrap">
        <table class="insight-table">
          <thead>
            <tr>
              <th>Horario</th>
              <th>Vendas</th>
              <th>Valor</th>
            </tr>
          </thead>
          <tbody>
            ${rows || '<tr><td colspan="3">Sem dados.</td></tr>'}
          </tbody>
        </table>
      </div>
    </article>
  `;
}

export function renderDataPanel({
  history,
  visitReasonOptions,
  customerSourceOptions,
  roster,
  waitingList,
  activeServices,
  pausedEmployees,
  consultantCurrentStatus,
  consultantActivitySessions,
  settings
}) {
  const insights = buildInsights({ history, visitReasonOptions, customerSourceOptions });
  const timeIntelligence = buildTimeIntelligence({
    history,
    roster,
    waitingList,
    activeServices,
    pausedEmployees,
    consultantCurrentStatus,
    consultantActivitySessions,
    settings
  });

  return `
    <section class="admin-panel">
      <header class="admin-panel__header">
        <h2 class="admin-panel__title">Dados operacionais</h2>
        <p class="admin-panel__text">Painel bruto de produto, motivo, origem, horario e tempo.</p>
      </header>

      <div class="insight-grid">
        <article class="insight-card insight-card--wide">
          <h3 class="insight-card__title">Inteligencia de tempo</h3>
          <div class="insight-time-grid">
            <span class="insight-tag">Fechou muito rapido: <strong>${timeIntelligence.quickHighPotentialCount}</strong></span>
            <span class="insight-tag">Demorou e vendeu baixo: <strong>${timeIntelligence.longLowSaleCount}</strong></span>
            <span class="insight-tag">Demorou e nao vendeu: <strong>${timeIntelligence.longNoSaleCount}</strong></span>
            <span class="insight-tag">Rapido e nao vendeu: <strong>${timeIntelligence.quickNoSaleCount}</strong></span>
            <span class="insight-tag">Espera media na fila: <strong>${formatDurationMinutes(timeIntelligence.avgQueueWaitMs)}</strong></span>
            <span class="insight-tag">Atendimento fora da vez: <strong>${formatPercent(timeIntelligence.notUsingQueueRate)}</strong></span>
          </div>
          <div class="insight-time-grid">
            <span class="insight-tag">Tempo historico em fila: <strong>${formatDurationMinutes(timeIntelligence.totalsByStatus.queue)}</strong></span>
            <span class="insight-tag">Tempo historico ocioso: <strong>${formatDurationMinutes(timeIntelligence.totalsByStatus.available)}</strong></span>
            <span class="insight-tag">Tempo historico em pausa: <strong>${formatDurationMinutes(timeIntelligence.totalsByStatus.paused)}</strong></span>
            <span class="insight-tag">Tempo historico atendendo: <strong>${formatDurationMinutes(timeIntelligence.totalsByStatus.service)}</strong></span>
          </div>
          <div class="insight-time-grid">
            <span class="insight-tag">Fila atual sem atender: <strong>${formatDurationMinutes(timeIntelligence.consultantsInQueueMs)}</strong></span>
            <span class="insight-tag">Pausa atual acumulada: <strong>${formatDurationMinutes(timeIntelligence.consultantsPausedMs)}</strong></span>
            <span class="insight-tag">Atendimento atual acumulado: <strong>${formatDurationMinutes(timeIntelligence.consultantsInServiceMs)}</strong></span>
          </div>
        </article>
        ${renderTagList("Produtos mais vendidos", insights.soldProducts)}
        ${renderTagList("Produtos mais procurados", insights.requestedProducts)}
        ${renderTagList("Motivos de visita", insights.visitReasons)}
        ${renderTagList("Origem do cliente", insights.customerSources)}
        ${renderTagList("Profissoes mais atendidas", insights.professions)}
        ${renderTagList("Desfecho dos atendimentos", insights.outcomeSummary)}
        ${renderHourlyTable(insights.hourlySales)}
      </div>
    </section>
  `;
}
