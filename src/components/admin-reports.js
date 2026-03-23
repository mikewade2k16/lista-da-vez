import { buildReportData } from "../utils/reports.js";

const FILTER_GROUPS = [
  { id: "consultantIds", label: "Consultor" },
  { id: "outcomes", label: "Desfecho" },
  { id: "sourceIds", label: "Origem" },
  { id: "visitReasonIds", label: "Motivo" },
  { id: "startModes", label: "Tipo" },
  { id: "existingCustomerModes", label: "Cliente" },
  { id: "completionLevels", label: "Preenchimento" },
  { id: "advanced", label: "Periodo e busca" }
];

function formatPercent(value) {
  return `${Number(value || 0).toFixed(1)}%`;
}

function formatCurrency(value) {
  return new Intl.NumberFormat("pt-BR", {
    style: "currency",
    currency: "BRL"
  }).format(Number(value || 0));
}

function renderActiveFilterChip(chip) {
  return `
    <button
      type="button"
      class="report-active-chip"
      data-action="clear-report-filter"
      data-filter-id="${chip.filterId}"
      ${chip.filterValue ? `data-filter-value="${chip.filterValue}"` : ""}
      title="Remover filtro"
    >
      <span class="report-active-chip__label">${chip.label}</span>
      <span class="report-active-chip__remove material-icons-round">close</span>
    </button>
  `;
}

function renderIconActionButton({ action, icon, label, extraClass = "" }) {
  return `
    <button
      type="button"
      class="${`report-icon-action ${extraClass}`.trim()}"
      data-action="${action}"
      aria-label="${label}"
      title="${label}"
    >
      <span class="material-icons-round">${icon}</span>
    </button>
  `;
}

function hasActiveValue(value) {
  return Array.isArray(value) ? value.length > 0 : String(value || "").trim().length > 0;
}

function hasActiveGroup(groupId, filters) {
  if (groupId === "advanced") {
    return Boolean(filters.dateFrom || filters.dateTo || filters.minSaleAmount || filters.maxSaleAmount || filters.search);
  }

  return hasActiveValue(filters[groupId]);
}

function renderFilterOptionButtons(filterId, options, selectedValues = []) {
  return `
    <div class="report-option-cloud">
      ${options
        .map(
          (option) => `
            <button
              type="button"
              class="report-option-chip ${selectedValues.includes(option.value) ? "is-active" : ""}"
              data-action="toggle-report-filter-value"
              data-filter-id="${filterId}"
              data-filter-value="${option.value}"
            >
              ${option.label}
            </button>
          `
        )
        .join("")}
    </div>
  `;
}

function buildActiveFilterChips(filters, { roster, visitReasonOptions, customerSourceOptions }) {
  const consultantMap = new Map((roster || []).map((consultant) => [consultant.id, consultant.name]));
  const visitReasonMap = new Map((visitReasonOptions || []).map((item) => [item.id, item.label]));
  const customerSourceMap = new Map((customerSourceOptions || []).map((item) => [item.id, item.label]));
  const outcomeMap = new Map([
    ["compra", "Compra"],
    ["reserva", "Reserva"],
    ["nao-compra", "Nao compra"]
  ]);
  const startModeMap = new Map([
    ["queue", "Na vez"],
    ["queue-jump", "Fora da vez"]
  ]);
  const existingCustomerMap = new Map([
    ["yes", "Recorrente"],
    ["no", "Novo cliente"]
  ]);
  const completionMap = new Map([
    ["excellent", "Completo + observacao"],
    ["complete", "Completo"],
    ["incomplete", "Incompleto"]
  ]);
  const chips = [];

  (filters.consultantIds || []).forEach((value) => {
    chips.push({
      filterId: "consultantIds",
      filterValue: value,
      label: `Consultor: ${consultantMap.get(value) || value}`
    });
  });

  (filters.outcomes || []).forEach((value) => {
    chips.push({
      filterId: "outcomes",
      filterValue: value,
      label: `Desfecho: ${outcomeMap.get(value) || value}`
    });
  });

  (filters.sourceIds || []).forEach((value) => {
    chips.push({
      filterId: "sourceIds",
      filterValue: value,
      label: `Origem: ${customerSourceMap.get(value) || value}`
    });
  });

  (filters.visitReasonIds || []).forEach((value) => {
    chips.push({
      filterId: "visitReasonIds",
      filterValue: value,
      label: `Motivo: ${visitReasonMap.get(value) || value}`
    });
  });

  (filters.startModes || []).forEach((value) => {
    chips.push({
      filterId: "startModes",
      filterValue: value,
      label: `Tipo: ${startModeMap.get(value) || value}`
    });
  });

  (filters.existingCustomerModes || []).forEach((value) => {
    chips.push({
      filterId: "existingCustomerModes",
      filterValue: value,
      label: `Cliente: ${existingCustomerMap.get(value) || value}`
    });
  });

  (filters.completionLevels || []).forEach((value) => {
    chips.push({
      filterId: "completionLevels",
      filterValue: value,
      label: `Preenchimento: ${completionMap.get(value) || value}`
    });
  });

  if (filters.dateFrom) {
    chips.push({
      filterId: "dateFrom",
      label: `De: ${filters.dateFrom}`
    });
  }

  if (filters.dateTo) {
    chips.push({
      filterId: "dateTo",
      label: `Ate: ${filters.dateTo}`
    });
  }

  if (filters.minSaleAmount) {
    chips.push({
      filterId: "minSaleAmount",
      label: `Min: ${formatCurrency(filters.minSaleAmount)}`
    });
  }

  if (filters.maxSaleAmount) {
    chips.push({
      filterId: "maxSaleAmount",
      label: `Max: ${formatCurrency(filters.maxSaleAmount)}`
    });
  }

  if (filters.search) {
    chips.push({
      filterId: "search",
      label: `Busca: ${filters.search}`
    });
  }

  return chips;
}

function renderFilterPanelContent(groupId, report, { roster, visitReasonOptions, customerSourceOptions }) {
  if (groupId === "consultantIds") {
    return renderFilterOptionButtons(
      "consultantIds",
      (roster || []).map((consultant) => ({
        value: consultant.id,
        label: consultant.name
      })),
      report.filters.consultantIds
    );
  }

  if (groupId === "outcomes") {
    return renderFilterOptionButtons(
      "outcomes",
      [
        { value: "compra", label: "Compra" },
        { value: "reserva", label: "Reserva" },
        { value: "nao-compra", label: "Nao compra" }
      ],
      report.filters.outcomes
    );
  }

  if (groupId === "sourceIds") {
    return renderFilterOptionButtons(
      "sourceIds",
      (customerSourceOptions || []).map((option) => ({
        value: option.id,
        label: option.label
      })),
      report.filters.sourceIds
    );
  }

  if (groupId === "visitReasonIds") {
    return renderFilterOptionButtons(
      "visitReasonIds",
      (visitReasonOptions || []).map((option) => ({
        value: option.id,
        label: option.label
      })),
      report.filters.visitReasonIds
    );
  }

  if (groupId === "startModes") {
    return renderFilterOptionButtons(
      "startModes",
      [
        { value: "queue", label: "Na vez" },
        { value: "queue-jump", label: "Fora da vez" }
      ],
      report.filters.startModes
    );
  }

  if (groupId === "existingCustomerModes") {
    return renderFilterOptionButtons(
      "existingCustomerModes",
      [
        { value: "yes", label: "Recorrente" },
        { value: "no", label: "Novo cliente" }
      ],
      report.filters.existingCustomerModes
    );
  }

  if (groupId === "completionLevels") {
    return renderFilterOptionButtons(
      "completionLevels",
      [
        { value: "excellent", label: "Completo + observacao" },
        { value: "complete", label: "Completo" },
        { value: "incomplete", label: "Incompleto" }
      ],
      report.filters.completionLevels
    );
  }

  return `
    <div class="report-filter-grid">
      <label class="settings-field">
        <span>Data inicial</span>
        <input type="date" value="${report.filters.dateFrom}" data-action="set-report-filter" data-filter-id="dateFrom">
      </label>
      <label class="settings-field">
        <span>Data final</span>
        <input type="date" value="${report.filters.dateTo}" data-action="set-report-filter" data-filter-id="dateTo">
      </label>
      <label class="settings-field">
        <span>Valor minimo (R$)</span>
        <input
          type="number"
          min="0"
          step="1"
          value="${report.filters.minSaleAmount}"
          data-action="set-report-filter"
          data-filter-id="minSaleAmount"
        >
      </label>
      <label class="settings-field">
        <span>Valor maximo (R$)</span>
        <input
          type="number"
          min="0"
          step="1"
          value="${report.filters.maxSaleAmount}"
          data-action="set-report-filter"
          data-filter-id="maxSaleAmount"
        >
      </label>
      <label class="settings-field report-filter-grid__search">
        <span>Busca livre</span>
        <input
          type="text"
          value="${report.filters.search}"
          placeholder="ID, cliente, telefone, produto..."
          data-action="set-report-filter"
          data-filter-id="search"
        >
      </label>
    </div>
  `;
}

function renderFilterToolbar(report, reportUiState, sources) {
  const activeChips = buildActiveFilterChips(report.filters, sources);
  const filtersExpanded = Boolean(reportUiState?.filtersExpanded);
  const expandedGroup = reportUiState?.expandedGroup || null;

  return `
    <article class="settings-card report-filters-card">
      <header class="settings-card__header report-filters-card__header">
        <div class="report-filters-card__title-row">
          <h3 class="settings-card__title">Filtros</h3>
          <button
            type="button"
            class="report-filter-toggle"
            data-action="toggle-report-filters"
            aria-expanded="${filtersExpanded ? "true" : "false"}"
            aria-label="${filtersExpanded ? "Esconder filtros" : "Abrir filtros"}"
            title="${filtersExpanded ? "Esconder filtros" : "Abrir filtros"}"
          >
            <span class="material-icons-round">filter_alt</span>
          </button>
        </div>
      </header>

      ${
        activeChips.length
          ? `
            <div class="report-active-filters">
              <div class="report-active-filters__list">
                ${activeChips.map(renderActiveFilterChip).join("")}
              </div>
              ${renderIconActionButton({
                action: "reset-report-filters",
                icon: "filter_alt_off",
                label: "Limpar filtros",
                extraClass: "report-icon-action--subtle"
              })}
            </div>
          `
          : ""
      }

      ${
        filtersExpanded
          ? `
            <div class="report-filter-groups">
              ${FILTER_GROUPS
                .map((group) => `
                  <button
                    type="button"
                    class="report-filter-group-btn ${expandedGroup === group.id ? "is-active" : ""} ${hasActiveGroup(group.id, report.filters) ? "has-value" : ""}"
                    data-action="toggle-report-filter-group"
                    data-filter-group="${group.id}"
                  >
                    ${group.label}
                  </button>
                `)
                .join("")}
            </div>
            ${
              expandedGroup
                ? `
                  <div class="report-filter-panel">
                    ${renderFilterPanelContent(expandedGroup, report, sources)}
                  </div>
                `
                : ""
            }
          `
          : ""
      }

      <div class="report-actions">
        ${renderIconActionButton({
          action: "export-report-csv",
          icon: "table_view",
          label: "Exportar CSV"
        })}
        ${renderIconActionButton({
          action: "export-report-pdf",
          icon: "picture_as_pdf",
          label: "Exportar PDF"
        })}
      </div>
    </article>
  `;
}

function renderReportTable(rows) {
  const limitedRows = rows.slice(0, 200);
  const tableRows = limitedRows
    .map(
      (row) => `
        <tr>
          <td>${row.storeName}</td>
          <td>${row.finishedAtLabel}</td>
          <td>${row.consultantName}</td>
          <td>${row.outcomeLabel}</td>
          <td>${row.saleAmountLabel}</td>
          <td>${row.durationLabel}</td>
          <td>${row.queueWaitLabel}</td>
          <td>${row.completionLabel}</td>
          <td>${row.startModeLabel}</td>
          <td>${row.customerName}</td>
          <td>${row.customerSourcesLabel}</td>
          <td>${row.campaignNamesLabel}</td>
        </tr>
      `
    )
    .join("");

  return `
    <article class="insight-card insight-card--wide">
      <header class="intel-card__header">
        <h3 class="insight-card__title">Atendimentos filtrados</h3>
        <span class="insight-tag">${rows.length} registros</span>
      </header>
      <div class="insight-table-wrap">
        <table class="insight-table">
          <thead>
            <tr>
              <th>Loja</th>
              <th>Data/Hora</th>
              <th>Consultor</th>
              <th>Desfecho</th>
              <th>Valor</th>
              <th>Duracao</th>
              <th>Espera fila</th>
              <th>Preenchimento</th>
              <th>Modo</th>
              <th>Cliente</th>
              <th>Origem</th>
              <th>Campanhas</th>
            </tr>
          </thead>
          <tbody>
            ${tableRows || '<tr><td colspan="12">Sem dados para os filtros selecionados.</td></tr>'}
          </tbody>
        </table>
      </div>
      ${
        rows.length > limitedRows.length
          ? `<p class="settings-card__text">Mostrando os primeiros ${limitedRows.length} registros na tela.</p>`
          : ""
      }
    </article>
  `;
}

function getInitials(name) {
  return String(name || "")
    .split(" ")
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0].toUpperCase())
    .join("");
}

function renderOutcomeChart(chartData, total) {
  const t = total || 1;
  const items = [
    { label: "Compra", count: chartData.outcomeCounts.compra, color: "#22c55e" },
    { label: "Reserva", count: chartData.outcomeCounts.reserva, color: "#38bdf8" },
    { label: "Nao compra", count: chartData.outcomeCounts["nao-compra"], color: "#475569" }
  ];

  return items
    .map(
      (item) => `
      <div class="dist-bar-row">
        <span class="dist-bar-row__label" style="color: ${item.color}">${item.label}</span>
        <div class="dist-bar-row__track">
          <div class="dist-bar-row__fill" style="width: ${((item.count / t) * 100).toFixed(1)}%; background: ${item.color}"></div>
        </div>
        <span class="dist-bar-row__count">${item.count}</span>
      </div>
    `
    )
    .join("");
}

function renderHourlyChart(chartData) {
  const { hourlyData } = chartData;

  if (!hourlyData.length) {
    return '<span class="insight-empty">Sem dados para o periodo.</span>';
  }

  const W = 480;
  const H = 72;
  const allHours = Array.from({ length: 24 }, (_, i) => String(i).padStart(2, "0"));
  const maxVal = Math.max(...hourlyData.map((d) => d.attendances), 1);
  const barW = W / 24;

  const bars = allHours
    .map((hour, i) => {
      const d = hourlyData.find((x) => x.hour === hour);
      const count = d ? d.attendances : 0;
      const convs = d ? d.conversions : 0;
      const height = count > 0 ? Math.max(3, (count / maxVal) * H) : 0;
      const convHeight = convs > 0 ? Math.max(2, (convs / maxVal) * H) : 0;
      const x = i * barW;

      return `<g>
        <rect x="${(x + 1).toFixed(1)}" y="${(H - height).toFixed(1)}" width="${(barW - 2).toFixed(1)}" height="${height.toFixed(1)}" fill="#1e293b" rx="2"/>
        ${convs > 0 ? `<rect x="${(x + 1).toFixed(1)}" y="${(H - convHeight).toFixed(1)}" width="${(barW - 2).toFixed(1)}" height="${convHeight.toFixed(1)}" fill="#22c55e" rx="2"/>` : ""}
      </g>`;
    })
    .join("");

  const hourLabels = [0, 6, 12, 18]
    .map(
      (i) =>
        `<text x="${(i * barW + barW / 2).toFixed(1)}" y="${(H + 13).toFixed(1)}" font-size="9" fill="#94a3b8" text-anchor="middle">${String(i).padStart(2, "0")}h</text>`
    )
    .join("");

  return `
    <div class="chart-hourly-wrap">
      <svg viewBox="0 0 ${W} ${H + 18}" width="100%">
        ${bars}
        ${hourLabels}
      </svg>
      <div class="chart-legend">
        <span class="chart-legend__item chart-legend__item--base">Atendimentos</span>
        <span class="chart-legend__item chart-legend__item--success">Conversoes</span>
      </div>
    </div>
  `;
}

function renderDistBars(items, emptyText) {
  if (!items || !items.length) {
    return `<span class="insight-empty">${emptyText || "Sem dados."}</span>`;
  }

  const max = items[0].count || 1;

  return items
    .map(
      (item) => `
      <div class="dist-bar-row">
        <span class="dist-bar-row__label">${item.label}</span>
        <div class="dist-bar-row__track">
          <div class="dist-bar-row__fill" style="width: ${((item.count / max) * 100).toFixed(1)}%"></div>
        </div>
        <span class="dist-bar-row__count">${item.count}</span>
      </div>
    `
    )
    .join("");
}

function renderConsultantGoals(chartData, roster) {
  const rosterWithGoals = (roster || []).filter((c) => Number(c.monthlyGoal || 0) > 0);

  if (!rosterWithGoals.length) {
    return '<span class="insight-empty">Nenhum consultor com meta definida. Configure metas em Configuracoes > Consultores.</span>';
  }

  const teamGoal = rosterWithGoals.reduce((sum, c) => sum + Number(c.monthlyGoal || 0), 0);
  const teamSold = chartData.consultantAgg.reduce((sum, a) => sum + a.saleAmount, 0);
  const teamProgress = teamGoal > 0 ? Math.min(100, (teamSold / teamGoal) * 100) : 0;

  const consultantRows = rosterWithGoals
    .map((c) => {
      const agg = chartData.consultantAgg.find((a) => a.consultantId === c.id) || { attendances: 0, conversions: 0, saleAmount: 0 };
      const goal = Number(c.monthlyGoal || 0);
      const progress = goal > 0 ? Math.min(100, (agg.saleAmount / goal) * 100) : 0;
      const convRate = agg.attendances > 0 ? ((agg.conversions / agg.attendances) * 100).toFixed(0) : 0;
      const remaining = Math.max(0, goal - agg.saleAmount);

      return `
        <div class="consultant-goal-row">
          <span class="consultant-goal-row__avatar" style="--avatar-accent: ${c.color}">${getInitials(c.name)}</span>
          <div class="consultant-goal-row__body">
            <div class="consultant-goal-row__header">
              <strong class="consultant-goal-row__name">${c.name}</strong>
              <span class="insight-tag">${convRate}% conv</span>
              <span class="insight-tag">${agg.attendances} atend</span>
              ${progress >= 100 ? '<span class="insight-tag insight-tag--success">Meta atingida</span>' : ""}
            </div>
            <div class="progress-bar">
              <span class="progress-bar__fill" style="--progress: ${progress.toFixed(1)}%; background: linear-gradient(90deg, ${c.color}88, ${c.color})"></span>
            </div>
            <div class="consultant-goal-row__footer">
              <span class="metric-card__text">${formatCurrency(agg.saleAmount)} vendido</span>
              <span class="metric-card__text">Meta: ${formatCurrency(goal)}</span>
              ${remaining > 0 ? `<span class="metric-card__text">Falta: ${formatCurrency(remaining)}</span>` : ""}
            </div>
          </div>
        </div>
      `;
    })
    .join("");

  return `
    <div class="team-goal-summary">
      <div class="team-goal-summary__header">
        <span class="metric-card__label">Meta da equipe</span>
        <span class="metric-card__text">${formatCurrency(teamSold)} de ${formatCurrency(teamGoal)}</span>
      </div>
      <div class="progress-bar progress-bar--team">
        <span class="progress-bar__fill" style="--progress: ${teamProgress.toFixed(1)}%"></span>
      </div>
    </div>
    ${consultantRows}
  `;
}

function renderConsultantQuality(report) {
  const rows = report.quality.byConsultant
    .map(
      (item) => `
        <tr>
          <td>${item.consultantName}</td>
          <td>${item.totalAttendances}</td>
          <td>${formatPercent(item.completeRate)}</td>
          <td>${formatPercent(item.excellentRate)}</td>
          <td>${formatPercent(item.incompleteRate)}</td>
          <td>${formatPercent(item.notesRate)}</td>
          <td>
            <span class="report-quality-badge report-quality-badge--${item.qualityLevelKey}">
              ${item.qualityLevelLabel}
            </span>
          </td>
        </tr>
      `
    )
    .join("");

  return `
    <article class="settings-card">
      <header class="settings-card__header">
        <h3 class="settings-card__title">Qualidade do preenchimento</h3>
        <p class="settings-card__text">Quem preenche bem, quem adiciona observacoes e quem ainda deixa o fechamento incompleto.</p>
      </header>

      <section class="metric-grid metric-grid--tight">
        <article class="metric-card metric-card--soft">
          <span class="metric-card__label">Preenchimento correto</span>
          <strong class="metric-card__value">${formatPercent(report.quality.completeRate)}</strong>
          <span class="metric-card__text">${report.quality.completeCount} atendimentos completos</span>
        </article>
        <article class="metric-card metric-card--soft">
          <span class="metric-card__label">Completo + observacao</span>
          <strong class="metric-card__value">${formatPercent(report.quality.excellentRate)}</strong>
          <span class="metric-card__text">${report.quality.excellentCount} atendimentos com observacoes</span>
        </article>
        <article class="metric-card metric-card--soft">
          <span class="metric-card__label">Incompletos</span>
          <strong class="metric-card__value">${formatPercent(report.quality.incompleteRate)}</strong>
          <span class="metric-card__text">${report.quality.incompleteCount} atendimentos com falhas de preenchimento</span>
        </article>
        <article class="metric-card metric-card--soft">
          <span class="metric-card__label">Observacoes</span>
          <strong class="metric-card__value">${formatPercent(report.quality.notesRate)}</strong>
          <span class="metric-card__text">${report.quality.notesCount} atendimentos com anotacoes</span>
        </article>
      </section>

      <div class="insight-table-wrap">
        <table class="insight-table">
          <thead>
            <tr>
              <th>Consultor</th>
              <th>Atendimentos</th>
              <th>Completo</th>
              <th>Completo + obs</th>
              <th>Incompleto</th>
              <th>Observacoes</th>
              <th>Nivel</th>
            </tr>
          </thead>
          <tbody>
            ${rows || '<tr><td colspan="7">Sem dados suficientes para avaliar preenchimento.</td></tr>'}
          </tbody>
        </table>
      </div>
    </article>
  `;
}

export function renderReportsPanel({
  history,
  roster,
  visitReasonOptions,
  customerSourceOptions,
  reportFilters,
  reportUiState
}) {
  const report = buildReportData({
    history,
    roster,
    visitReasonOptions,
    customerSourceOptions,
    filters: reportFilters
  });
  const filterSources = {
    roster,
    visitReasonOptions,
    customerSourceOptions
  };

  return `
    <section class="admin-panel">
      <header class="admin-panel__header">
        <h2 class="admin-panel__title">Relatorios operacionais</h2>
        <p class="admin-panel__text">Leitura de performance, tempo medio e qualidade de preenchimento do fechamento.</p>
      </header>

      ${renderFilterToolbar(report, reportUiState, filterSources)}

      <section class="metric-grid">
        <article class="metric-card">
          <span class="metric-card__label">Atendimentos</span>
          <strong class="metric-card__value">${report.metrics.totalAttendances}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Conversao</span>
          <strong class="metric-card__value">${formatPercent(report.metrics.conversionRate)}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Valor vendido</span>
          <strong class="metric-card__value">${report.metrics.soldValueLabel}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Ticket medio</span>
          <strong class="metric-card__value">${report.metrics.averageTicketLabel}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Media de atendimento</span>
          <strong class="metric-card__value">${report.metrics.averageDurationLabel}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Media de espera</span>
          <strong class="metric-card__value">${report.metrics.averageQueueWaitLabel}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Fora da vez</span>
          <strong class="metric-card__value">${formatPercent(report.metrics.queueJumpRate)}</strong>
        </article>
        <article class="metric-card">
          <span class="metric-card__label">Bonus campanhas</span>
          <strong class="metric-card__value">${report.metrics.campaignBonusTotalLabel}</strong>
        </article>
      </section>

      <!-- Charts: desfecho + horario -->
      <div class="report-chart-grid">
        <article class="insight-card">
          <header class="intel-card__header">
            <h3 class="insight-card__title">Desfecho dos atendimentos</h3>
            <span class="insight-tag">${report.metrics.totalAttendances} total</span>
          </header>
          ${renderOutcomeChart(report.chartData, report.metrics.totalAttendances)}
        </article>
        <article class="insight-card">
          <header class="intel-card__header">
            <h3 class="insight-card__title">Atendimentos por hora</h3>
          </header>
          ${renderHourlyChart(report.chartData)}
        </article>
      </div>

      <!-- Meta por consultor -->
      <article class="insight-card insight-card--wide">
        <header class="intel-card__header">
          <h3 class="insight-card__title">Meta mensal dos consultores</h3>
        </header>
        ${renderConsultantGoals(report.chartData, roster)}
      </article>

      <!-- Distribuicoes: produtos, motivos, origens -->
      <div class="report-dist-grid">
        <article class="insight-card">
          <header class="intel-card__header">
            <h3 class="insight-card__title">Produtos fechados</h3>
          </header>
          ${renderDistBars(report.chartData.topProductsClosed, "Nenhum produto registrado.")}
        </article>
        <article class="insight-card">
          <header class="intel-card__header">
            <h3 class="insight-card__title">Motivos de visita</h3>
          </header>
          ${renderDistBars(report.chartData.topVisitReasons, "Nenhum motivo registrado.")}
        </article>
        <article class="insight-card">
          <header class="intel-card__header">
            <h3 class="insight-card__title">Origem do cliente</h3>
          </header>
          ${renderDistBars(report.chartData.topCustomerSources, "Nenhuma origem registrada.")}
        </article>
      </div>

      <!-- Qualidade + tabela -->
      <div class="insight-grid">
        ${renderConsultantQuality(report)}
        ${renderReportTable(report.rows)}
      </div>
    </section>
  `;
}
