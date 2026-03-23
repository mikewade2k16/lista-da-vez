import {
  buildOperationalIntelligence,
  formatCurrencyBRL,
  formatDurationMinutes,
  formatPercent
} from "../utils/admin-metrics.js";

function renderSeverityBadge(level) {
  const label =
    level === "critical"
      ? "Critico"
      : level === "attention"
        ? "Atencao"
        : "Saudavel";

  return `<span class="intel-badge intel-badge--${level}">${label}</span>`;
}

function renderDiagnosisCard(item) {
  return `
    <article class="insight-card intel-card">
      <header class="intel-card__header">
        <h3 class="insight-card__title">${item.title}</h3>
        ${renderSeverityBadge(item.level)}
      </header>
      <p class="intel-card__text"><strong>Leitura:</strong> ${item.reading}</p>
      <p class="intel-card__text"><strong>O que pode estar acontecendo:</strong> ${item.hypothesis}</p>
      <p class="intel-card__text"><strong>Acao recomendada:</strong> ${item.action}</p>
    </article>
  `;
}

function renderActionItems(items) {
  if (!items.length) {
    return '<li class="intel-list__item">Sem alerta relevante no momento.</li>';
  }

  return items
    .map((item) => `<li class="intel-list__item">${item}</li>`)
    .join("");
}

function renderContextRows(contextRows) {
  return contextRows
    .map(
      (row) => `
        <div class="intel-context__row">
          <span>${row.label}</span>
          <strong>${row.value}</strong>
        </div>
      `
    )
    .join("");
}

export function renderIntelligencePanel({
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
  const intelligence = buildOperationalIntelligence({
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
  });
  const summaryLevelClass =
    intelligence.healthScore >= 80 ? "healthy" : intelligence.healthScore >= 60 ? "attention" : "critical";
  const contextRows = [
    {
      label: "Tempo medio de espera na fila",
      value: formatDurationMinutes(intelligence.time.avgQueueWaitMs)
    },
    {
      label: "Taxa de atendimento fora da vez",
      value: formatPercent(intelligence.time.notUsingQueueRate)
    },
    {
      label: "Ticket medio (compra/reserva)",
      value: formatCurrencyBRL(intelligence.ticketAverage)
    },
    {
      label: "Conversao geral",
      value: formatPercent(intelligence.conversionRate)
    }
  ];

  return `
    <section class="admin-panel">
      <header class="admin-panel__header">
        <h2 class="admin-panel__title">Inteligencia operacional</h2>
        <p class="admin-panel__text">Leitura automatica dos dados para apoiar decisao de loja e gestao de equipe.</p>
      </header>

      <article class="insight-card intel-summary">
        <div class="intel-summary__score intel-summary__score--${summaryLevelClass}">
          <span class="intel-summary__label">Score operacional</span>
          <strong class="intel-summary__value">${Math.round(intelligence.healthScore)}</strong>
        </div>
        <div class="intel-summary__meta">
          <span class="insight-tag">Criticos: <strong>${intelligence.severityCounts.critical}</strong></span>
          <span class="insight-tag">Atencao: <strong>${intelligence.severityCounts.attention}</strong></span>
          <span class="insight-tag">Saudaveis: <strong>${intelligence.severityCounts.healthy}</strong></span>
          <span class="insight-tag">Atendimentos analisados: <strong>${intelligence.totalAttendances}</strong></span>
        </div>
      </article>

      <div class="insight-grid">
        ${intelligence.diagnosis.map((item) => renderDiagnosisCard(item)).join("")}
      </div>

      <div class="insight-grid">
        <article class="insight-card">
          <h3 class="insight-card__title">Acoes recomendadas agora</h3>
          <ul class="intel-list">
            ${renderActionItems(intelligence.recommendedActions)}
          </ul>
        </article>
        <article class="insight-card">
          <h3 class="insight-card__title">Contexto rapido</h3>
          <div class="intel-context">
            ${renderContextRows(contextRows)}
          </div>
        </article>
      </div>
    </section>
  `;
}
