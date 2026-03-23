import { buildRankingRows, formatCurrencyBRL, formatPercent } from "../utils/admin-metrics.js";

function renderRankingTable(title, rows) {
  const body = rows
    .map(
      (row, index) => `
        <tr>
          <td>${index + 1}</td>
          <td>${row.consultantName}</td>
          <td>${formatCurrencyBRL(row.soldValue)}</td>
          <td>${row.conversions}/${row.attendances}</td>
          <td>${formatPercent(row.conversionRate)}</td>
          <td>${row.nonClientConversions}</td>
          <td>${row.queueJumpServices}</td>
        </tr>
      `
    )
    .join("");

  return `
    <article class="ranking-card">
      <header class="ranking-card__header">
        <h3 class="ranking-card__title">${title}</h3>
      </header>
      <div class="ranking-card__table-wrap">
        <table class="ranking-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Consultor</th>
              <th>Vendas</th>
              <th>Conversoes</th>
              <th>Taxa</th>
              <th>Nao clientes</th>
              <th>Fora da vez</th>
            </tr>
          </thead>
          <tbody>
            ${body || '<tr><td colspan="7">Sem dados no periodo.</td></tr>'}
          </tbody>
        </table>
      </div>
    </article>
  `;
}

export function renderRankingPanel({ history, roster }) {
  const monthly = buildRankingRows({ history, roster, scope: "month" });
  const today = buildRankingRows({ history, roster, scope: "today" });

  return `
    <section class="admin-panel">
      <header class="admin-panel__header">
        <h2 class="admin-panel__title">Ranking de vendedores</h2>
        <p class="admin-panel__text">Comparativo mensal e diario para acompanhar consistencia e bonificacao.</p>
      </header>

      <div class="ranking-grid">
        ${renderRankingTable("Ranking do mes", monthly)}
        ${renderRankingTable("Ranking de hoje", today)}
      </div>
    </section>
  `;
}
